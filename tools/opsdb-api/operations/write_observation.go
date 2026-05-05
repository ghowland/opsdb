// === opsdb-api/operations/write_observation.go ===
package operations

// WriteObservation handles the write_observation operation.
// Runner writes to observation cache tables, runner_job_output_var, or evidence_record.
// Validates report key before writing.
func WriteObservation(params *WriteObservationParams) (*WriteResult, error) {
	// TODO: report key enforcement is done by reportkeys.Enforce() called from gate step
	//       (fail-fast before this function is called)
	//
	// TODO: switch on params.TargetTable:
	//   observation_cache_metric:
	//     UPSERT keyed by (authority_id, hostname, metric_key)
	//     set metric_value, metric_data_json, _observed_time, _puller_runner_job_id
	//   observation_cache_state:
	//     UPSERT keyed by (entity_type, entity_id, state_key)
	//     set state_value, state_data_json, _observed_time, _puller_runner_job_id
	//   observation_cache_config:
	//     UPSERT keyed by (authority_id, hostname, config_key)
	//     set config_value, config_data_json, _observed_time, _puller_runner_job_id
	//   runner_job_output_var:
	//     INSERT (runner_job_id, var_name, var_value, var_type)
	//   evidence_record:
	//     INSERT with all evidence fields
	//
	// TODO: return WriteResult with written row ID
	return nil, nil
}

// WriteObservationParams holds write_observation parameters.
type WriteObservationParams struct {
	TargetTable    string
	Key            string
	Value          interface{}
	DataJSON       map[string]interface{}
	RunnerJobID    int
	AuthorityID    int
	ObservedTime   interface{} // time.Time
}

// WriteResult holds the result of a write operation.
type WriteResult struct {
	RowID int
}


