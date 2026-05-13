//# tools/opsdb-api/reportkeys/enforcer.go

package reportkeys

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/ghowland/opsdb/internal/pg"
)

// ReportKey represents one declared report key for a runner spec.
// Each declaration authorizes the runner to write a specific key to
// a specific observation table, optionally with value constraints.
type ReportKey struct {
	Key            string
	TargetTable    string
	ConstraintJSON map[string]interface{}
}

// Enforcer validates runner write_observation calls against declared
// report keys. This is the OPSDB-6 §8 mechanism: each runner declares
// which keys it may write to which observation tables, and the enforcer
// rejects writes outside that declared scope.
//
// Fail-closed: an undeclared key is always rejected. A runner that has
// not declared any report keys cannot write any observation data.
//
// Declarations are cached per runner spec for fast lookups. The cache
// is populated on first access and can be invalidated when
// runner_report_key rows change.
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

// Enforce validates a runner's observation write against declared report
// keys. Returns nil if the write is authorized, or a structured error
// if the key is undeclared or the value violates declared constraints.
//
// The check sequence:
//  1. Load declarations for the runner spec (from cache or database)
//  2. Find a declaration matching the submitted key and target table
//  3. If no match: reject with UndeclaredKeyError (fail closed)
//  4. If match: validate the value against the declaration's constraints
//  5. If value violates constraints: reject with InvalidKeyValueError
func Enforce(e *Enforcer, runnerSpecID int, targetTable string, key string, value interface{}) error {
	declarations, err := e.getDeclarations(runnerSpecID, targetTable)
	if err != nil {
		return fmt.Errorf("report key enforcement failed: could not load declarations for runner spec %d: %w",
			runnerSpecID, err)
	}

	// Find matching declaration — fail closed if not found
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

	// Validate value against declared constraints
	if len(matched.ConstraintJSON) > 0 {
		constraintErr := validateValueConstraints(key, value, matched.ConstraintJSON)
		if constraintErr != nil {
			return &InvalidKeyValueError{
				RunnerSpecID: runnerSpecID,
				TargetTable:  targetTable,
				Key:          key,
				Value:        value,
				Detail:       constraintErr.Error(),
			}
		}
	}

	return nil
}

// CacheDeclarations loads report key declarations for a runner spec from
// the runner_report_key table and stores them in the in-memory cache.
// Called automatically on first Enforce call for a runner spec, or
// explicitly to pre-warm the cache.
func (e *Enforcer) CacheDeclarations(runnerSpecID int) error {
	rows, err := e.db.Query(
		"SELECT report_key, report_target_table, report_key_data_json "+
			"FROM runner_report_key "+
			"WHERE runner_spec_id = $1 AND is_active = true",
		runnerSpecID,
	)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			// runner_report_key table doesn't exist yet — during early
			// bootstrap. Cache an empty map so subsequent calls don't
			// keep querying.
			e.mu.Lock()
			e.cache[runnerSpecID] = make(map[string][]ReportKey)
			e.mu.Unlock()
			return nil
		}
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
			if err := json.Unmarshal(constraintJSON, &constraints); err == nil {
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
// Called when runner_report_key rows are modified through a change set.
func (e *Enforcer) InvalidateCache(runnerSpecID int) {
	e.mu.Lock()
	delete(e.cache, runnerSpecID)
	e.mu.Unlock()
}

// InvalidateAll clears the entire cache. Called on schema refresh to
// ensure stale declarations don't persist after schema evolution.
func (e *Enforcer) InvalidateAll() {
	e.mu.Lock()
	e.cache = make(map[int]map[string][]ReportKey)
	e.mu.Unlock()
}

// getDeclarations returns cached declarations for a runner spec and target
// table, loading from the database on first access.
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

// ---------------------------------------------------------------------------
// Value constraint validation
// ---------------------------------------------------------------------------

// validateValueConstraints checks a submitted value against the constraints
// declared in report_key_data_json. Constraints are the same closed
// vocabulary as the schema — type, enum, numeric range, string length,
// required fields within JSON structure.
func validateValueConstraints(key string, value interface{}, constraints map[string]interface{}) error {
	// Type constraint
	if expectedType, ok := constraints["type"].(string); ok {
		if err := checkValueType(value, expectedType); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	// Enum constraint
	if enumVals, ok := constraints["enum_values"]; ok {
		if err := checkEnumValue(value, enumVals); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	// Numeric range constraints
	if minVal, ok := constraints["min"]; ok {
		if err := checkMinValue(value, minVal); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}
	if minVal, ok := constraints["min_value"]; ok {
		if err := checkMinValue(value, minVal); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}
	if maxVal, ok := constraints["max"]; ok {
		if err := checkMaxValue(value, maxVal); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}
	if maxVal, ok := constraints["max_value"]; ok {
		if err := checkMaxValue(value, maxVal); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	// String length constraint
	if maxLen, ok := constraints["max_length"]; ok {
		if err := checkMaxLength(value, maxLen); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	// Required fields within JSON structure
	if requiredFields, ok := constraints["required_fields"]; ok {
		if err := checkRequiredFields(value, requiredFields); err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
	}

	return nil
}

// checkValueType validates a value matches the expected type.
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

// checkEnumValue validates a string value is in the allowed set.
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
		return fmt.Errorf("value %q not in allowed set: [%s]",
			strVal, strings.Join(ev, ", "))

	case []interface{}:
		allowed := make([]string, 0, len(ev))
		for _, item := range ev {
			if s, ok := item.(string); ok {
				if s == strVal {
					return nil
				}
				allowed = append(allowed, s)
			}
		}
		return fmt.Errorf("value %q not in allowed set: [%s]",
			strVal, strings.Join(allowed, ", "))

	default:
		// Can't interpret enum values — pass through
		return nil
	}
}

// checkMinValue validates a numeric value is at or above the minimum.
func checkMinValue(value interface{}, minVal interface{}) error {
	valNum, ok := toFloat64(value)
	if !ok {
		return nil // non-numeric values skip numeric range checks
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

// checkMaxValue validates a numeric value is at or below the maximum.
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

// checkMaxLength validates a string value doesn't exceed the maximum length.
func checkMaxLength(value interface{}, maxLen interface{}) error {
	strVal, ok := value.(string)
	if !ok {
		return nil
	}
	maxNum, ok := toNumeric(maxLen)
	if !ok {
		return nil
	}
	strLen := len([]rune(strVal)) // character count, not byte count
	if strLen > maxNum {
		return fmt.Errorf("string length %d exceeds maximum %d", strLen, maxNum)
	}
	return nil
}

// checkRequiredFields validates that a JSON object value contains all
// required fields.
func checkRequiredFields(value interface{}, requiredFields interface{}) error {
	jsonMap, ok := value.(map[string]interface{})
	if !ok {
		return nil // non-object values skip required field checks
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

// ---------------------------------------------------------------------------
// Numeric conversion helpers (local to reportkeys package)
// ---------------------------------------------------------------------------

// toNumeric converts an interface{} to int. Handles the numeric types
// that JSON unmarshaling and Go code produce.
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

// toFloat64 converts an interface{} to float64. Accepts all numeric types.
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

// ---------------------------------------------------------------------------
// Error types
// ---------------------------------------------------------------------------

// UndeclaredKeyError indicates a runner submitted a report key that is
// not in its declared set. This is a fail-closed rejection — the write
// is not performed, and the rejection is recorded in the audit log.
type UndeclaredKeyError struct {
	RunnerSpecID int
	TargetTable  string
	SubmittedKey string
	DeclaredKeys []string
}

func (e *UndeclaredKeyError) Error() string {
	return fmt.Sprintf(
		"undeclared_report_key: runner spec %d submitted key %q to %s; declared keys: [%s]",
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
	return fmt.Sprintf(
		"invalid_report_key_value: runner spec %d key %q on %s: %s",
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
