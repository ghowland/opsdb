// === opsdb-api/reportkeys/enforcer.go ===
package reportkeys

// Enforcer validates runner write_observation calls against declared report keys.
// Caches declarations per runner spec for fast lookups.
type Enforcer struct {
	// TODO: cache map[int]map[string][]ReportKey — runner_spec_id → target_table → declared keys
}

// ReportKey represents one declared report key for a runner.
type ReportKey struct {
	Key            string
	TargetTable    string
	ConstraintJSON map[string]interface{} // report_key_data_json constraints
}

// NewEnforcer creates a report key enforcer.
func NewEnforcer() *Enforcer {
	// TODO: initialize empty cache
	return nil
}

// Enforce validates a runner's observation write against declared report keys.
// Returns nil on pass, structured error on rejection.
// Fail-closed: undeclared keys are always rejected.
func (e *Enforcer) Enforce(runnerSpecID int, targetTable string, key string, value interface{}) error {
	// TODO: look up cached declarations for runner_spec_id + target_table
	// TODO: if not cached: call CacheDeclarations to load
	//
	// TODO: check submitted key is in declared set
	//   if not: return undeclared_report_key error with runner identity + submitted key
	//
	// TODO: find matching declaration
	// TODO: validate submitted value against report_key_data_json constraints:
	//   numeric range, enum membership, structural shape
	//   if invalid: return invalid_report_key_value error with detail
	//
	// TODO: return nil on pass
	return nil
}

// CacheDeclarations loads report key declarations for a runner spec from OpsDB.
func (e *Enforcer) CacheDeclarations(runnerSpecID int) error {
	// TODO: SELECT * FROM runner_report_key
	//       WHERE runner_spec_id = runnerSpecID AND is_active = true
	// TODO: group by report_target_table
	// TODO: store in cache keyed by runner_spec_id → target_table → []ReportKey
	return nil
}

// InvalidateCache clears cached declarations for a runner spec.
// Called when runner_report_key rows are modified.
func (e *Enforcer) InvalidateCache(runnerSpecID int) {
	// TODO: delete cache entry for runnerSpecID
}


