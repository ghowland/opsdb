package runnerlib

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// LoadRunnerConfig authenticates to the OpsDB API, reads the runner spec,
// parses runner_data_json, reads bounds and report key declarations,
// and returns a fully populated RunnerConfig.
func LoadRunnerConfig(specName string, apiEndpoint string) (*RunnerConfig, error) {
	authToken, err := envOrError("OPSDB_AUTH_TOKEN")
	if err != nil {
		return nil, fmt.Errorf("runner config: %w", err)
	}

	client := NewAPIClient(apiEndpoint, authToken)

	// Look up runner spec by name.
	specRow, err := client.GetEntityByName("runner_spec", specName)
	if err != nil {
		return nil, fmt.Errorf("runner spec %q not found: %w", specName, err)
	}

	specID, err := extractIntField(specRow, "id")
	if err != nil {
		return nil, fmt.Errorf("runner spec %q missing id: %w", specName, err)
	}

	// Find the current active version of this spec.
	versionResult, err := client.Search("runner_spec_version", []SearchFilter{
		{Field: "runner_spec_id", Operator: "eq", Value: specID},
		{Field: "is_active_version", Operator: "eq", Value: true},
	}, []OrderSpec{
		{Field: "version_serial", Direction: "desc"},
	}, 1, "")
	if err != nil {
		return nil, fmt.Errorf("searching runner spec version for %q: %w", specName, err)
	}
	if len(versionResult.Rows) == 0 {
		return nil, fmt.Errorf("no active version found for runner spec %q", specName)
	}

	versionRow := versionResult.Rows[0]
	versionSerial, _ := extractIntField(versionRow, "version_serial")

	// Parse runner_data_json from version row.
	specDataJSON, err := extractJSONField(versionRow, "runner_data_json")
	if err != nil {
		return nil, fmt.Errorf("parsing runner_data_json for %q: %w", specName, err)
	}

	// Extract standard config values from runner_data_json.
	cycleIntervalSec := jsonIntOrDefault(specDataJSON, "cycle_interval_seconds", 60)
	maxCycles := jsonIntOrDefault(specDataJSON, "max_cycles", 0)
	dryRun := jsonBoolOrDefault(specDataJSON, "dry_run", false)

	// Load report key declarations for this runner spec.
	reportKeys, err := loadReportKeys(client, specID)
	if err != nil {
		// Non-fatal: runner can operate without report keys if it only reads.
		reportKeys = nil
	}

	// Read runner identity from environment.
	runnerMachineID := envIntOrDefault("OPSDB_RUNNER_MACHINE_ID", 0)
	siteID := envIntOrDefault("OPSDB_SITE_ID", 1)

	config := &RunnerConfig{
		SpecName:        specName,
		SpecVersion:     versionSerial,
		RunnerSpecID:    specID,
		RunnerMachineID: runnerMachineID,
		APIEndpoint:     apiEndpoint,
		AuthToken:       authToken,
		SiteID:          siteID,
		DryRun:          dryRun,
		MaxCycles:       maxCycles,
		CycleInterval:   time.Duration(cycleIntervalSec) * time.Second,
		CycleCount:      0,
		SpecDataJSON:    specDataJSON,
		ReportKeys:      reportKeys,
		Client:          client,
		shutdownCh:      make(chan struct{}),
	}

	// Pass report keys to the API client for local fail-fast.
	client.ReportKeys = reportKeys

	config.Logger = NewLogger(config)

	return config, nil
}

// RefreshConfig re-reads the runner spec from OpsDB. Called at the start
// of each cycle for long-running runners to pick up configuration changes
// without restart.
func RefreshConfig(config *RunnerConfig) error {
	versionResult, err := config.Client.Search("runner_spec_version", []SearchFilter{
		{Field: "runner_spec_id", Operator: "eq", Value: config.RunnerSpecID},
		{Field: "is_active_version", Operator: "eq", Value: true},
	}, []OrderSpec{
		{Field: "version_serial", Direction: "desc"},
	}, 1, "")
	if err != nil {
		return fmt.Errorf("refreshing runner spec version: %w", err)
	}
	if len(versionResult.Rows) == 0 {
		return fmt.Errorf("no active version found during refresh for %q", config.SpecName)
	}

	versionRow := versionResult.Rows[0]
	newVersionSerial, _ := extractIntField(versionRow, "version_serial")

	if newVersionSerial == config.SpecVersion {
		return nil // no change
	}

	oldVersion := config.SpecVersion

	newSpecDataJSON, err := extractJSONField(versionRow, "runner_data_json")
	if err != nil {
		return fmt.Errorf("parsing updated runner_data_json: %w", err)
	}

	config.mu.Lock()
	config.SpecVersion = newVersionSerial
	config.SpecDataJSON = newSpecDataJSON
	config.CycleInterval = time.Duration(jsonIntOrDefault(newSpecDataJSON, "cycle_interval_seconds", 60)) * time.Second
	config.MaxCycles = jsonIntOrDefault(newSpecDataJSON, "max_cycles", 0)
	config.DryRun = jsonBoolOrDefault(newSpecDataJSON, "dry_run", false)
	config.mu.Unlock()

	// Refresh report keys.
	reportKeys, err := loadReportKeys(config.Client, config.RunnerSpecID)
	if err == nil {
		config.mu.Lock()
		config.ReportKeys = reportKeys
		config.Client.ReportKeys = reportKeys
		config.mu.Unlock()
	}

	config.Logger.Info("runner config refreshed",
		Field("old_version", oldVersion),
		Field("new_version", newVersionSerial),
	)

	return nil
}

// GetSpecData reads a typed value from the runner spec's runner_data_json.
// Returns the value and true if present, nil and false if absent.
func GetSpecData(config *RunnerConfig, key string) (interface{}, bool) {
	config.mu.Lock()
	defer config.mu.Unlock()

	if config.SpecDataJSON == nil {
		return nil, false
	}
	val, ok := config.SpecDataJSON[key]
	return val, ok
}

// GetSpecDataString reads a string value from runner_data_json.
// Returns empty string and false if absent or not a string.
func GetSpecDataString(config *RunnerConfig, key string) (string, bool) {
	val, ok := GetSpecData(config, key)
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

// GetSpecDataInt reads an integer value from runner_data_json.
// Returns 0 and false if absent or not numeric.
// Handles JSON numbers (float64) and converts to int.
func GetSpecDataInt(config *RunnerConfig, key string) (int, bool) {
	val, ok := GetSpecData(config, key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

// GetSpecDataBool reads a boolean value from runner_data_json.
// Returns false and false if absent or not a bool.
func GetSpecDataBool(config *RunnerConfig, key string) (bool, bool) {
	val, ok := GetSpecData(config, key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetSpecDataFloat reads a float64 value from runner_data_json.
// Returns 0 and false if absent or not numeric.
func GetSpecDataFloat(config *RunnerConfig, key string) (float64, bool) {
	val, ok := GetSpecData(config, key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// GetSpecDataStringSlice reads a string slice from runner_data_json.
// Returns nil and false if absent or not a list of strings.
func GetSpecDataStringSlice(config *RunnerConfig, key string) ([]string, bool) {
	val, ok := GetSpecData(config, key)
	if !ok {
		return nil, false
	}
	switch v := val.(type) {
	case []string:
		return v, true
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, s)
		}
		return result, true
	default:
		return nil, false
	}
}

// GetSpecDataDuration reads a duration in seconds from runner_data_json.
// Returns zero duration and false if absent or not numeric.
func GetSpecDataDuration(config *RunnerConfig, key string) (time.Duration, bool) {
	seconds, ok := GetSpecDataInt(config, key)
	if !ok {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

// GetSpecDataMap reads a nested map from runner_data_json.
// Returns nil and false if absent or not a map.
func GetSpecDataMap(config *RunnerConfig, key string) (map[string]interface{}, bool) {
	val, ok := GetSpecData(config, key)
	if !ok {
		return nil, false
	}
	m, ok := val.(map[string]interface{})
	return m, ok
}

// --- internal helpers ---

// loadReportKeys fetches all active report key declarations for a runner spec.
func loadReportKeys(client *APIClient, runnerSpecID int) ([]ReportKeyDecl, error) {
	result, err := client.Search("runner_report_key", []SearchFilter{
		{Field: "runner_spec_id", Operator: "eq", Value: runnerSpecID},
		{Field: "is_active", Operator: "eq", Value: true},
	}, nil, 1000, "")
	if err != nil {
		return nil, fmt.Errorf("loading report keys: %w", err)
	}

	var keys []ReportKeyDecl
	for _, row := range result.Rows {
		key := ReportKeyDecl{}
		if v, ok := row["report_key"].(string); ok {
			key.Key = v
		}
		if v, ok := row["report_target_table"].(string); ok {
			key.TargetTable = v
		}
		if v, ok := row["report_key_data_json"].(map[string]interface{}); ok {
			key.ConstraintJSON = v
		}
		if key.Key != "" && key.TargetTable != "" {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// extractIntField reads an integer field from a row map.
// Handles JSON float64 numbers.
func extractIntField(row map[string]interface{}, field string) (int, error) {
	val, ok := row[field]
	if !ok {
		return 0, fmt.Errorf("field %q not found", field)
	}
	switch v := val.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("field %q: %w", field, err)
		}
		return int(i), nil
	default:
		return 0, fmt.Errorf("field %q is %T, not numeric", field, val)
	}
}

// extractJSONField reads a JSON object field from a row map.
// If the field is a string, it's parsed as JSON. If already a map, returned directly.
func extractJSONField(row map[string]interface{}, field string) (map[string]interface{}, error) {
	val, ok := row[field]
	if !ok {
		return make(map[string]interface{}), nil
	}
	switch v := val.(type) {
	case map[string]interface{}:
		return v, nil
	case string:
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, fmt.Errorf("parsing %q as JSON: %w", field, err)
		}
		return m, nil
	default:
		return nil, fmt.Errorf("field %q is %T, not map or string", field, val)
	}
}

// jsonIntOrDefault reads an int from a JSON map with a default fallback.
func jsonIntOrDefault(m map[string]interface{}, key string, defaultVal int) int {
	val, ok := m[key]
	if !ok {
		return defaultVal
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return defaultVal
		}
		return int(i)
	default:
		return defaultVal
	}
}

// jsonBoolOrDefault reads a bool from a JSON map with a default fallback.
func jsonBoolOrDefault(m map[string]interface{}, key string, defaultVal bool) bool {
	val, ok := m[key]
	if !ok {
		return defaultVal
	}
	b, ok := val.(bool)
	if !ok {
		return defaultVal
	}
	return b
}

// envOrError reads an environment variable and returns error if empty.
func envOrError(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return val, nil
}

// envIntOrDefault reads an environment variable as an integer.
// Returns the default value if the variable is not set or not parseable.
func envIntOrDefault(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}