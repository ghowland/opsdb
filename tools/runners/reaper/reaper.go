package reaper

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

// RetentionTarget holds one retention policy and its computed deletion scope.
type RetentionTarget struct {
	PolicyID         int
	PolicyName       string
	TargetEntityType string
	RetentionDays    int
	Horizon          time.Time
	ExpiredRowCount  int
	DeletionMode     string // hard_delete, soft_delete, skip
}

// ReaperSummary holds the results of one reaper cycle.
type ReaperSummary struct {
	PoliciesEvaluated int
	TablesProcessed   int
	TablesSkipped     int
	RowsDeleted       int
	RowsSoftDeleted   int
	BoundHits         []string
	Errors            []string
}

// observationCacheTables are hard-deleted because they are cached copies
// whose source of truth lives in external authorities.
var observationCacheTables = map[string]bool{
	"observation_cache_metric": true,
	"observation_cache_state":  true,
	"observation_cache_config": true,
}

// runnerJobTables hold operational execution records that can be hard-deleted
// after retention.
var runnerJobTables = map[string]bool{
	"runner_job":                       true,
	"runner_job_output_var":            true,
	"runner_job_target_machine":        true,
	"runner_job_target_service":        true,
	"runner_job_target_k8s_workload":   true,
	"runner_job_target_cloud_resource": true,
}

// appendOnlyTables require explicit policy permission to reap.
var appendOnlyTables = map[string]bool{
	"audit_log_entry": true,
}

// classifyDeletionMode determines the reaping strategy for an entity type.
func classifyDeletionMode(entityType string, forceAuditReap bool) string {
	if observationCacheTables[entityType] {
		return "hard_delete"
	}
	if runnerJobTables[entityType] {
		return "hard_delete"
	}
	if appendOnlyTables[entityType] {
		if forceAuditReap {
			return "hard_delete"
		}
		return "skip"
	}
	return "soft_delete"
}

// timeColumnForEntity returns the column name used to determine row age.
func timeColumnForEntity(entityType string) string {
	if observationCacheTables[entityType] {
		return "_observed_time"
	}
	if entityType == "runner_job" {
		return "started_time"
	}
	return "created_time"
}

// GetRetentionTargets reads all active retention policies from OpsDB,
// computes the retention horizon for each, and counts expired rows.
func GetRetentionTargets(client *runner.APIClient, logger *runner.Logger) ([]RetentionTarget, error) {
	results, err := client.Search("retention_policy",
		[]runner.SearchFilter{
			{Field: "is_active", Operator: "eq", Value: true},
		},
		nil, 1000, "")
	if err != nil {
		return nil, fmt.Errorf("searching retention policies: %w", err)
	}

	now := time.Now().UTC()
	var targets []RetentionTarget

	for _, row := range results.Rows {
		policyID, _ := extractInt(row, "id")
		policyName, _ := row["name"].(string)

		policyData, err := extractJSONField(row, "policy_data_json")
		if err != nil {
			logger.Warn("skipping retention policy with unparseable data",
				runner.Field("policy_id", policyID),
				runner.Field("error", err.Error()),
			)
			continue
		}

		targetEntityType, _ := policyData["target_entity_type"].(string)
		retentionDays := jsonIntOrDefault(policyData, "retention_days", 0)
		forceAuditReap := jsonBoolOrDefault(policyData, "force_audit_reap", false)

		if targetEntityType == "" {
			logger.Warn("skipping retention policy with no target_entity_type",
				runner.Field("policy_id", policyID),
			)
			continue
		}
		if retentionDays <= 0 {
			logger.Warn("skipping retention policy with invalid retention_days",
				runner.Field("policy_id", policyID),
				runner.Field("retention_days", retentionDays),
			)
			continue
		}

		horizon := now.AddDate(0, 0, -retentionDays)
		deletionMode := classifyDeletionMode(targetEntityType, forceAuditReap)

		expiredCount := 0
		if deletionMode != "skip" {
			timeCol := timeColumnForEntity(targetEntityType)
			filters := []runner.SearchFilter{
				{Field: timeCol, Operator: "lt", Value: horizon.Format(time.RFC3339Nano)},
			}
			if deletionMode == "soft_delete" {
				filters = append(filters, runner.SearchFilter{
					Field: "is_active", Operator: "eq", Value: true,
				})
			}
			countResult, err := client.Search(targetEntityType, filters, nil, 1, "")
			if err != nil {
				logger.Warn("failed to count expired rows, skipping target",
					runner.Field("policy_id", policyID),
					runner.Field("target", targetEntityType),
					runner.Field("error", err.Error()),
				)
				continue
			}
			expiredCount = countResult.TotalCount
		}

		targets = append(targets, RetentionTarget{
			PolicyID:         policyID,
			PolicyName:       policyName,
			TargetEntityType: targetEntityType,
			RetentionDays:    retentionDays,
			Horizon:          horizon,
			ExpiredRowCount:  expiredCount,
			DeletionMode:     deletionMode,
		})
	}

	// Busiest tables first so the most impactful reaping happens
	// before any bound is hit.
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].ExpiredRowCount > targets[j].ExpiredRowCount
	})

	return targets, nil
}

// ReapTable deletes or soft-deletes expired rows from one table up to batchSize.
// Returns the number of rows affected.
func ReapTable(client *runner.APIClient, target *RetentionTarget, batchSize int, runnerJobID int, logger *runner.Logger) (int, error) {
	switch target.DeletionMode {
	case "hard_delete":
		return reapHardDelete(client, target, batchSize, runnerJobID, logger)
	case "soft_delete":
		return reapSoftDelete(client, target, batchSize, runnerJobID, logger)
	case "skip":
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown deletion mode %q for %s", target.DeletionMode, target.TargetEntityType)
	}
}

// reapHardDelete signals deletion of rows from observation cache or runner job tables.
// Uses WriteObservation with a delete marker — the API server's step_execute
// handles the actual row removal for reaper-initiated deletions.
func reapHardDelete(client *runner.APIClient, target *RetentionTarget, batchSize int, runnerJobID int, logger *runner.Logger) (int, error) {
	timeCol := timeColumnForEntity(target.TargetEntityType)
	filters := []runner.SearchFilter{
		{Field: timeCol, Operator: "lt", Value: target.Horizon.Format(time.RFC3339Nano)},
	}

	totalDeleted := 0

	for {
		results, err := client.Search(target.TargetEntityType, filters,
			[]runner.OrderSpec{{Field: timeCol, Direction: "asc"}},
			batchSize, "")
		if err != nil {
			return totalDeleted, fmt.Errorf("query failed for %s after %d deletions: %w",
				target.TargetEntityType, totalDeleted, err)
		}
		if len(results.Rows) == 0 {
			break
		}

		for _, row := range results.Rows {
			rowID, _ := extractInt(row, "id")

			_, err := client.WriteObservation(&runner.WriteObservationParams{
				TargetTable: target.TargetEntityType,
				Key:         fmt.Sprintf("reaper_delete:%d", rowID),
				Value:       "deleted",
				DataJSON: map[string]interface{}{
					"action":    "hard_delete",
					"entity_id": rowID,
					"policy_id": target.PolicyID,
					"horizon":   target.Horizon.Format(time.RFC3339Nano),
				},
				RunnerJobID:  runnerJobID,
				ObservedTime: time.Now(),
			})
			if err != nil {
				return totalDeleted, fmt.Errorf("delete failed for %s id=%d after %d deletions: %w",
					target.TargetEntityType, rowID, totalDeleted, err)
			}
			totalDeleted++
		}

		logger.Info("hard-deleted batch",
			runner.Field("target", target.TargetEntityType),
			runner.Field("batch_size", len(results.Rows)),
			runner.Field("total_deleted", totalDeleted),
		)

		if len(results.Rows) < batchSize {
			break
		}
	}

	return totalDeleted, nil
}

// reapSoftDelete sets is_active=false on entity rows past the retention horizon.
func reapSoftDelete(client *runner.APIClient, target *RetentionTarget, batchSize int, runnerJobID int, logger *runner.Logger) (int, error) {
	timeCol := timeColumnForEntity(target.TargetEntityType)
	filters := []runner.SearchFilter{
		{Field: timeCol, Operator: "lt", Value: target.Horizon.Format(time.RFC3339Nano)},
		{Field: "is_active", Operator: "eq", Value: true},
	}

	totalSoftDeleted := 0

	for {
		results, err := client.Search(target.TargetEntityType, filters,
			[]runner.OrderSpec{{Field: timeCol, Direction: "asc"}},
			batchSize, "")
		if err != nil {
			return totalSoftDeleted, fmt.Errorf("query failed for %s after %d soft-deletions: %w",
				target.TargetEntityType, totalSoftDeleted, err)
		}
		if len(results.Rows) == 0 {
			break
		}

		for _, row := range results.Rows {
			rowID, _ := extractInt(row, "id")

			_, err := client.WriteObservation(&runner.WriteObservationParams{
				TargetTable: target.TargetEntityType,
				Key:         fmt.Sprintf("reaper_soft_delete:%d", rowID),
				Value:       "soft_deleted",
				DataJSON: map[string]interface{}{
					"action":    "soft_delete",
					"entity_id": rowID,
					"policy_id": target.PolicyID,
					"is_active": false,
					"horizon":   target.Horizon.Format(time.RFC3339Nano),
				},
				RunnerJobID:  runnerJobID,
				ObservedTime: time.Now(),
			})
			if err != nil {
				return totalSoftDeleted, fmt.Errorf("soft-delete failed for %s id=%d after %d: %w",
					target.TargetEntityType, rowID, totalSoftDeleted, err)
			}
			totalSoftDeleted++
		}

		logger.Info("soft-deleted batch",
			runner.Field("target", target.TargetEntityType),
			runner.Field("batch_size", len(results.Rows)),
			runner.Field("total_soft_deleted", totalSoftDeleted),
		)

		if len(results.Rows) < batchSize {
			break
		}
	}

	return totalSoftDeleted, nil
}

// totalExpiredRows sums expired row counts across all targets for logging.
func totalExpiredRows(targets []RetentionTarget) int {
	total := 0
	for _, t := range targets {
		total += t.ExpiredRowCount
	}
	return total
}

// --- internal helpers ---

// extractInt reads an integer from a row map, handling JSON float64 numbers.
func extractInt(row map[string]interface{}, field string) (int, bool) {
	val, ok := row[field]
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
	default:
		return 0, false
	}
}

// extractJSONField reads a JSON object field from a row map.
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
