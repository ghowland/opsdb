//# tools/opsdb-api/operations/write_observation.go

package operations

import (
	"fmt"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
)

// WriteObservationParams holds write_observation parameters.
type WriteObservationParams struct {
	TargetTable  string
	Key          string
	Value        interface{}
	DataJSON     map[string]interface{}
	RunnerJobID  int
	AuthorityID  int
	Hostname     string
	EntityType   string
	EntityID     int
	ObservedTime time.Time
}

// WriteResult holds the result of a write operation.
type WriteResult struct {
	RowID   int
	Upserted bool // true if existing row was updated, false if new row inserted
}

// WriteObservation handles the write_observation operation. Writes to
// observation cache tables, runner_job_output_var, or evidence_record.
// Report key enforcement is performed by the gate pipeline before this
// function is called — by the time we get here the write is authorized.
func WriteObservation(db *pg.DB, params *WriteObservationParams) (*WriteResult, error) {
	if params.TargetTable == "" {
		return nil, fmt.Errorf("target_table is required")
	}

	if params.ObservedTime.IsZero() {
		params.ObservedTime = time.Now().UTC()
	}

	switch params.TargetTable {
	case "observation_cache_metric":
		return writeMetricCache(db, params)
	case "observation_cache_state":
		return writeStateCache(db, params)
	case "observation_cache_config":
		return writeConfigCache(db, params)
	case "runner_job_output_var":
		return writeRunnerJobOutputVar(db, params)
	case "evidence_record":
		return writeEvidenceRecord(db, params)
	default:
		return nil, fmt.Errorf("unsupported observation target table: %s", params.TargetTable)
	}
}

// writeMetricCache upserts into observation_cache_metric keyed by
// (authority_id, hostname, metric_key).
func writeMetricCache(db *pg.DB, params *WriteObservationParams) (*WriteResult, error) {
	if params.AuthorityID == 0 {
		return nil, fmt.Errorf("authority_id is required for observation_cache_metric")
	}
	if params.Key == "" {
		return nil, fmt.Errorf("key (metric_key) is required for observation_cache_metric")
	}

	now := time.Now().UTC()

	var rowID int
	err := db.QueryRow(
		"INSERT INTO observation_cache_metric "+
			"(authority_id, hostname, metric_key, metric_value, metric_data_json, "+
			"_observed_time, _authority_id, _puller_runner_job_id, "+
			"created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, $5, $6, $1, $7, $8, $8) "+
			"ON CONFLICT (authority_id, hostname, metric_key) DO UPDATE SET "+
			"metric_value = EXCLUDED.metric_value, "+
			"metric_data_json = EXCLUDED.metric_data_json, "+
			"_observed_time = EXCLUDED._observed_time, "+
			"_puller_runner_job_id = EXCLUDED._puller_runner_job_id, "+
			"updated_time = EXCLUDED.updated_time "+
			"RETURNING id",
		params.AuthorityID, params.Hostname, params.Key,
		params.Value, marshalDataJSON(params.DataJSON),
		params.ObservedTime, params.RunnerJobID, now,
	).Scan(&rowID)
	if err != nil {
		return nil, fmt.Errorf("observation_cache_metric upsert failed: %w", err)
	}

	return &WriteResult{RowID: rowID}, nil
}

// writeStateCache upserts into observation_cache_state keyed by
// (entity_type, entity_id, state_key).
func writeStateCache(db *pg.DB, params *WriteObservationParams) (*WriteResult, error) {
	if params.EntityType == "" {
		return nil, fmt.Errorf("entity_type is required for observation_cache_state")
	}
	if params.EntityID == 0 {
		return nil, fmt.Errorf("entity_id is required for observation_cache_state")
	}
	if params.Key == "" {
		return nil, fmt.Errorf("key (state_key) is required for observation_cache_state")
	}

	now := time.Now().UTC()

	var rowID int
	err := db.QueryRow(
		"INSERT INTO observation_cache_state "+
			"(entity_type, entity_id, state_key, state_value, state_data_json, "+
			"_observed_time, _authority_id, _puller_runner_job_id, "+
			"created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9) "+
			"ON CONFLICT (entity_type, entity_id, state_key) DO UPDATE SET "+
			"state_value = EXCLUDED.state_value, "+
			"state_data_json = EXCLUDED.state_data_json, "+
			"_observed_time = EXCLUDED._observed_time, "+
			"_authority_id = EXCLUDED._authority_id, "+
			"_puller_runner_job_id = EXCLUDED._puller_runner_job_id, "+
			"updated_time = EXCLUDED.updated_time "+
			"RETURNING id",
		params.EntityType, params.EntityID, params.Key,
		params.Value, marshalDataJSON(params.DataJSON),
		params.ObservedTime, params.AuthorityID, params.RunnerJobID, now,
	).Scan(&rowID)
	if err != nil {
		return nil, fmt.Errorf("observation_cache_state upsert failed: %w", err)
	}

	return &WriteResult{RowID: rowID}, nil
}

// writeConfigCache upserts into observation_cache_config keyed by
// (authority_id, hostname, config_key).
func writeConfigCache(db *pg.DB, params *WriteObservationParams) (*WriteResult, error) {
	if params.AuthorityID == 0 {
		return nil, fmt.Errorf("authority_id is required for observation_cache_config")
	}
	if params.Key == "" {
		return nil, fmt.Errorf("key (config_key) is required for observation_cache_config")
	}

	now := time.Now().UTC()

	var rowID int
	err := db.QueryRow(
		"INSERT INTO observation_cache_config "+
			"(authority_id, hostname, config_key, config_value, config_data_json, "+
			"_observed_time, _authority_id, _puller_runner_job_id, "+
			"created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, $5, $6, $1, $7, $8, $8) "+
			"ON CONFLICT (authority_id, hostname, config_key) DO UPDATE SET "+
			"config_value = EXCLUDED.config_value, "+
			"config_data_json = EXCLUDED.config_data_json, "+
			"_observed_time = EXCLUDED._observed_time, "+
			"_puller_runner_job_id = EXCLUDED._puller_runner_job_id, "+
			"updated_time = EXCLUDED.updated_time "+
			"RETURNING id",
		params.AuthorityID, params.Hostname, params.Key,
		params.Value, marshalDataJSON(params.DataJSON),
		params.ObservedTime, params.RunnerJobID, now,
	).Scan(&rowID)
	if err != nil {
		return nil, fmt.Errorf("observation_cache_config upsert failed: %w", err)
	}

	return &WriteResult{RowID: rowID}, nil
}

// writeRunnerJobOutputVar inserts a discrete output variable from a runner job.
func writeRunnerJobOutputVar(db *pg.DB, params *WriteObservationParams) (*WriteResult, error) {
	if params.RunnerJobID == 0 {
		return nil, fmt.Errorf("runner_job_id is required for runner_job_output_var")
	}
	if params.Key == "" {
		return nil, fmt.Errorf("key (var_name) is required for runner_job_output_var")
	}

	now := time.Now().UTC()

	// determine var_type from the value
	varType := inferVarType(params.Value)

	var rowID int
	err := db.QueryRow(
		"INSERT INTO runner_job_output_var "+
			"(runner_job_id, var_name, var_value, var_type, var_data_json, "+
			"created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, $5, $6, $6) RETURNING id",
		params.RunnerJobID, params.Key, fmt.Sprintf("%v", params.Value),
		varType, marshalDataJSON(params.DataJSON), now,
	).Scan(&rowID)
	if err != nil {
		return nil, fmt.Errorf("runner_job_output_var insert failed: %w", err)
	}

	return &WriteResult{RowID: rowID}, nil
}

// writeEvidenceRecord inserts a verification outcome from a runner or human.
func writeEvidenceRecord(db *pg.DB, params *WriteObservationParams) (*WriteResult, error) {
	if params.DataJSON == nil {
		return nil, fmt.Errorf("data_json is required for evidence_record")
	}

	now := time.Now().UTC()

	// extract required fields from data_json
	evidenceType, _ := params.DataJSON["evidence_record_type"].(string)
	if evidenceType == "" {
		return nil, fmt.Errorf("evidence_record_type is required in data_json")
	}

	description, _ := params.DataJSON["description"].(string)
	outcome, _ := params.DataJSON["outcome"].(string)
	if outcome == "" {
		outcome = "pending"
	}

	var rowID int
	err := db.QueryRow(
		"INSERT INTO evidence_record "+
			"(evidence_record_type, description, outcome, "+
			"evidence_record_data_json, runner_job_id, "+
			"_observed_time, created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, $5, $6, $7, $7) RETURNING id",
		evidenceType, description, outcome,
		marshalDataJSON(params.DataJSON), params.RunnerJobID,
		params.ObservedTime, now,
	).Scan(&rowID)
	if err != nil {
		return nil, fmt.Errorf("evidence_record insert failed: %w", err)
	}

	return &WriteResult{RowID: rowID}, nil
}

// marshalDataJSON safely converts a map to a JSON byte slice for storage.
// Returns nil if the input is nil or empty.
func marshalDataJSON(data map[string]interface{}) interface{} {
	if len(data) == 0 {
		return nil
	}
	bytes, err := pg.MarshalJSON(data)
	if err != nil {
		return nil
	}
	return bytes
}

// inferVarType determines the type string for a runner_job_output_var
// based on the Go type of the value.
func inferVarType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int32, int64:
		return "int"
	case float32, float64:
		return "float"
	case bool:
		return "bool"
	case map[string]interface{}:
		return "json"
	case []interface{}:
		return "json"
	default:
		return "string"
	}
}