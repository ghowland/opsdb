//# tools/opsdb-api/reportkeys/enforcer.go

package reportkeys

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ghowland/opsdb/internal/pg"
)

// ReportKey represents one declared report key for a runner.
type ReportKey struct {
	Key            string
	TargetTable    string
	ConstraintJSON map[string]interface{}
}

// Enforcer validates runner write_observation calls against declared
// report keys. Fail-closed: undeclared keys are always rejected.
// Caches declarations per runner spec for fast lookups.
type Enforcer struct {
	db    *pg.DB
	mu    sync.RWMutex
	cache map[int]map[string][]ReportKey // runner_spec_id → target_table → keys
}

// NewEnforcer creates a report key enforcer backed by the given database.
func NewEnforcer(db *pg.DB) *Enforcer {
	return &Enforcer{
		db:    db,
		cache: make(map[int]map[string][]ReportKey),
	}
}

// Enforce validates a runner's observation write against declared report keys.
// Returns nil on pass, structured error on rejection.
func (e *Enforcer) Enforce(runnerSpecID int, targetTable string, key string, value interface{}) error {
	declarations, err := e.getDeclarations(runnerSpecID, targetTable)
	if err != nil {
		return fmt.Errorf("report key enforcement failed: could not load declarations for runner spec %d: %w",
			runnerSpecID, err)
	}

	// find matching declaration — fail closed if not found
	var matched *ReportKey
	for i := range declarations {
		if declarations[i].Key == key {
			matched = &declarations[i]
			break
		}
	}

	if matched == nil {
		declaredKeys := make([]string, 0, len(declarations))
		for _, d := range declarations {
			declaredKeys = append(declaredKeys, d.Key)
		}
		return &UndeclaredKeyError{
			RunnerSpecID: runnerSpecID,
			TargetTable:  targetTable,
			SubmittedKey: key,
			DeclaredKeys: declaredKeys,
		}
	}

	// validate value against declared constraints
	if len(matched.ConstraintJSON) > 0 {
		err := validateValueConstraints(key, value, matched.ConstraintJSON)
		if err != nil {
			return &InvalidKeyValueError{
				RunnerSpecID: runnerSpecID,
				TargetTable:  targetTable,
				Key:          key,
				Value:        value,
				Detail:       err.Error(),
			}
		}
	}

	return nil
}

// CacheDeclarations loads report key declarations for a runner spec from OpsDB.
func (e *Enforcer) CacheDeclarations(runnerSpecID int) error {
	rows, err := e.db.Query(
		"SELECT report_key, report_target_table, report_key_data_json "+
			"FROM runner_report_key "+
			"WHERE runner_spec_id = $1 AND is_active = true",
		runnerSpecID,
	)
	if err != nil {
		return fmt.Errorf("failed to query report keys for spec %d: %w", runnerSpecID, err)
	}
	defer rows.Close()

	byTable := make(map[string][]ReportKey)

	for rows.Next() {
		var key, targetTable string
		var constraintJSON []byte

		if err := rows.Scan(&key, &targetTable, &constraintJSON); err != nil {
			return fmt.Errorf("failed to scan report key row: %w", err)
		}

		rk := ReportKey{
			Key:         key,
			TargetTable: targetTable,
		}

		if len(constraintJSON) > 0 {
			constraints := make(map[string]interface{})
			if err := pg.UnmarshalJSON(constraintJSON, &constraints); err == nil {
				rk.ConstraintJSON = constraints
			}
		}

		byTable[targetTable] = append(byTable[targetTable], rk)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("report key query iteration failed: %w", err)
	}

	e.mu.Lock()
	e.cache[runnerSpecID] = byTable
	e.mu.Unlock()

	return nil
}

// InvalidateCache clears cached declarations for a runner spec.
// Called when runner_report_key rows are modified.
func (e *Enforcer) InvalidateCache(runnerSpecID int) {
	e.mu.Lock()
	delete(e.cache, runnerSpecID)
	e.mu.Unlock()
}

// InvalidateAll clears the entire cache. Called on schema refresh.
func (e *Enforcer) InvalidateAll() {
	e.mu.Lock()
	e.cache = make(map[int]map[string][]ReportKey)
	e.mu.Unlock()
}

// getDeclarations returns cached declarations for a runner spec and target
// table, loading from the database if not yet cached.
func (e *Enforcer) getDeclarations(runnerSpecID int, targetTable string) ([]ReportKey, error) {
	e.mu.RLock()
	specCache, specCached := e.cache[runnerSpecID]
	e.mu.RUnlock()

	if !specCached {
		err := e.CacheDeclarations(runnerSpecID)
		if err != nil {
			return nil, err
		}

		e.mu.RLock()
		specCache = e.cache[runnerSpecID]
		e.mu.RUnlock()
	}

	if specCache == nil {
		return nil, nil
	}

	return specCache[targetTable], nil
}

// validateValueConstraints checks a submitted value against the constraints
// declared in report_key_data_json.
func validateValueConstraints(key string, value interface{}, constraints map[string]interface{}) error {
	// type constraint
	if expectedType, ok := constraints["type"].(string); ok {
		if err := checkValueType(value, expectedType); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	// enum constraint
	if enumVals, ok := constraints["enum_values"]; ok {
		if err := checkEnumValue(value, enumVals); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	// numeric range constraints
	if minVal, ok := constraints["min_value"]; ok {
		if err := checkMinValue(value, minVal); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}
	if maxVal, ok := constraints["max_value"]; ok {
		if err := checkMaxValue(value, maxVal); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	// string length constraints
	if maxLen, ok := constraints["max_length"]; ok {
		if err := checkMaxLength(value, maxLen); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	// required fields within JSON structure
	if requiredFields, ok := constraints["required_fields"]; ok {
		if err := checkRequiredFields(value, requiredFields); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	return nil
}

func checkValueType(value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "int", "integer":
		if _, ok := toNumeric(value); !ok {
			return fmt.Errorf("expected integer, got %T", value)
		}
	case "float", "number":
		if _, ok := toFloat64(value); !ok {
			return fmt.Errorf("expected number, got %T", value)
		}
	case "bool", "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "json", "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected JSON object, got %T", value)
		}
	}
	return nil
}

func checkEnumValue(value interface{}, enumVals interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		return fmt.Errorf("enum check requires string value, got %T", value)
	}

	switch ev := enumVals.(type) {
	case []string:
		for _, allowed := range ev {
			if strVal == allowed {
				return nil
			}
		}
		return fmt.Errorf("value %q not in allowed set: %s", strVal, strings.Join(ev, ", "))
	case []interface{}:
		for _, item := range ev {
			if s, ok := item.(string); ok && s == strVal {
				return nil
			}
		}
		return fmt.Errorf("value %q not in allowed set", strVal)
	default:
		return nil
	}
}

func checkMinValue(value interface{}, minVal interface{}) error {
	valNum, ok := toFloat64(value)
	if !ok {
		return nil
	}
	minNum, ok := toFloat64(minVal)
	if !ok {
		return nil
	}
	if valNum < minNum {
		return fmt.Errorf("value %g is below minimum %g", valNum, minNum)
	}
	return nil
}

func checkMaxValue(value interface{}, maxVal interface{}) error {
	valNum, ok := toFloat64(value)
	if !ok {
		return nil
	}
	maxNum, ok := toFloat64(maxVal)
	if !ok {
		return nil
	}
	if valNum > maxNum {
		return fmt.Errorf("value %g exceeds maximum %g", valNum, maxNum)
	}
	return nil
}

func checkMaxLength(value interface{}, maxLen interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		return nil
	}
	maxNum, ok := toNumeric(maxLen)
	if !ok {
		return nil
	}
	if len(strVal) > maxNum {
		return fmt.Errorf("string length %d exceeds maximum %d", len(strVal), maxNum)
	}
	return nil
}

func checkRequiredFields(value interface{}, requiredFields interface{}) error {
	jsonMap, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	var fields []string
	switch rf := requiredFields.(type) {
	case []string:
		fields = rf
	case []interface{}:
		for _, item := range rf {
			if s, ok := item.(string); ok {
				fields = append(fields, s)
			}
		}
	default:
		return nil
	}

	var missing []string
	for _, field := range fields {
		if _, exists := jsonMap[field]; !exists {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}

	return nil
}

// --- numeric helpers ---

func toNumeric(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// --- error types ---

// UndeclaredKeyError indicates a runner submitted a report key that is
// not in its declared set. This is a fail-closed rejection.
type UndeclaredKeyError struct {
	RunnerSpecID int
	TargetTable  string
	SubmittedKey string
	DeclaredKeys []string
}

func (e *UndeclaredKeyError) Error() string {
	return fmt.Sprintf("undeclared_report_key: runner spec %d submitted key %q to %s; declared keys: [%s]",
		e.RunnerSpecID, e.SubmittedKey, e.TargetTable,
		strings.Join(e.DeclaredKeys, ", "))
}

// InvalidKeyValueError indicates a runner submitted a value that violates
// the constraints declared for the report key.
type InvalidKeyValueError struct {
	RunnerSpecID int
	TargetTable  string
	Key          string
	Value        interface{}
	Detail       string
}

func (e *InvalidKeyValueError) Error() string {
	return fmt.Sprintf("invalid_report_key_value: runner spec %d key %q on %s: %s",
		e.RunnerSpecID, e.Key, e.TargetTable, e.Detail)
}

// IsUndeclaredKeyError checks whether an error is an UndeclaredKeyError.
func IsUndeclaredKeyError(err error) bool {
	_, ok := err.(*UndeclaredKeyError)
	return ok
}

// IsInvalidKeyValueError checks whether an error is an InvalidKeyValueError.
func IsInvalidKeyValueError(err error) bool {
	_, ok := err.(*InvalidKeyValueError)
	return ok
}
