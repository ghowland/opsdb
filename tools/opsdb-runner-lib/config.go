//# tools/opsdb-runner-lib/config.go

go
package runnerlib

import (
	"fmt"
	"os"
	"time"
)

// LoadRunnerConfig authenticates to the OpsDB API, reads the runner spec,
// parses runner_data_json, reads bounds and report key declarations,
// and returns a fully populated RunnerConfig.
func LoadRunnerConfig(specName string, apiEndpoint string) (*RunnerConfig, error) {
	// TODO: read OPSDB_AUTH_TOKEN from environment
	// TODO: create APIClient with endpoint and token
	// TODO: call client.GetEntityByName("runner_spec", specName)
	//   if not found: return error "runner spec not found: {specName}"
	// TODO: extract runner_spec_id from response
	// TODO: call client.Search("runner_spec_version", filters: runner_spec_id, is_active_version=true)
	//   to get current active version
	// TODO: parse runner_data_json from version row:
	//   cycle_interval_seconds -> time.Duration
	//   max_cycles -> int (0 = unlimited)
	//   dry_run -> bool
	//   bounds: max_retry_count, max_cycle_duration_seconds, max_items_per_cycle
	//   resource-specific config (varies per runner type)
	// TODO: call client.Search("runner_report_key", filters: runner_spec_id, is_active=true)
	//   cache as []ReportKeyDecl
	// TODO: read runner_machine_id from OPSDB_RUNNER_MACHINE_ID env var
	// TODO: read site_id from OPSDB_SITE_ID env var
	// TODO: build RunnerConfig with all parsed values
	// TODO: return config
	return nil, fmt.Errorf("not implemented")
}

// RefreshConfig re-reads the runner spec from OpsDB. Called at the start
// of each cycle for long-running runners to pick up configuration changes
// without restart.
func RefreshConfig(config *RunnerConfig) error {
	// TODO: call config.Client.GetEntityByName("runner_spec", config.SpecName)
	// TODO: call config.Client.Search("runner_spec_version", ...) for current active version
	// TODO: compare version serial against config.SpecVersion
	// TODO: if same version: no-op, return nil
	// TODO: if different version:
	//   re-parse runner_data_json
	//   update config.SpecVersion, config.SpecDataJSON, config.CycleInterval, config.MaxCycles, config.DryRun
	//   refresh report key declarations
	//   log "runner config refreshed" with old and new version serials
	return nil
}

// GetSpecData reads a typed value from the runner spec's runner_data_json.
// Returns the value and true if present, nil and false if absent.
func GetSpecData(config *RunnerConfig, key string) (interface{}, bool) {
	// TODO: look up key in config.SpecDataJSON
	// TODO: return value and presence boolean
	return nil, false
}

// GetSpecDataString reads a string value from runner_data_json.
// Returns empty string and false if absent or not a string.
func GetSpecDataString(config *RunnerConfig, key string) (string, bool) {
	// TODO: call GetSpecData, type-assert to string
	return "", false
}

// GetSpecDataInt reads an integer value from runner_data_json.
// Returns 0 and false if absent or not numeric.
func GetSpecDataInt(config *RunnerConfig, key string) (int, bool) {
	// TODO: call GetSpecData, type-assert to float64 (JSON numbers), convert to int
	return 0, false
}

// GetSpecDataStringSlice reads a string slice from runner_data_json.
// Returns nil and false if absent or not a list of strings.
func GetSpecDataStringSlice(config *RunnerConfig, key string) ([]string, bool) {
	// TODO: call GetSpecData, type-assert to []interface{}, convert each to string
	return nil, false
}

// GetSpecDataDuration reads a duration in seconds from runner_data_json.
// Returns zero duration and false if absent or not numeric.
func GetSpecDataDuration(config *RunnerConfig, key string) (time.Duration, bool) {
	// TODO: call GetSpecDataInt, convert to time.Duration * time.Second
	return 0, false
}

// envOrError reads an environment variable and returns error if empty.
func envOrError(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return val, nil
}


