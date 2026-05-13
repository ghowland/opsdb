//# tools/runners/reaper/reaper.go

package reaper

import (
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
	RowsDeleted       int
	RowsSoftDeleted   int
	TablesSkipped     int
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
// after retention. These are not audit records.
var runnerJobTables = map[string]bool{
	"runner_job":                       true,
	"runner_job_output_var":            true,
	"runner_job_target_machine":        true,
	"runner_job_target_service":        true,
	"runner_job_target_k8s_workload":   true,
	"runner_job_target_cloud_resource": true,
}

// appendOnlyTables require explicit policy permission to reap because
// they carry audit and compliance weight. audit_log_entry retention is
// typically 7+ years per compliance regime.
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

// timeColumnForEntity returns the column name used to determine row age
// for the given entity type.
func timeColumnForEntity(entityType string) string {
	if observationCacheTables[entityType] {
		return "_observed_time"
	}
	if entityType == "runner_job" {
		return "started_time"
	}
	// runner_job child tables don't have their own time column;
	// they are reaped by joining to runner_job. For direct query
	// we fall back to created_time which every table has.
	return "created_time"
}

// GetRetentionTargets reads all active retention policies from OpsDB,
// computes the retention horizon for each, and counts expired rows.
func GetRetentionTargets(client *runner.APIClient, logger *runner.Logger) ([]RetentionTarget, error) {
	results, err := client.Search("retention_policy", map[string]interface{}{
		"is_active": true,
	}, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to search retention policies: %w", err)
	}

	now := time.Now().UTC()
	var targets []RetentionTarget

	for _, row := range results.Rows {
		policyID, _ := row.IntField("id")
		policyName, _ := row.StringField("name")

		policyData, err := row.JSONField("policy_data_json")
		if err != nil {
			logger.Warn("skipping retention policy with unparseable data",
				runner.Field{Key: "policy_id", Value: policyID},
				runner.Field{Key: "error", Value: err.Error()},
			)
			continue
		}

		targetEntityType, _ := policyData.String("target_entity_type")
		retentionDays, _ := policyData.Int("retention_days")
		forceAuditReap, _ := policyData.Bool("force_audit_reap")

		if targetEntityType == "" {
			logger.Warn("skipping retention policy with no target_entity_type",
				runner.Field{Key: "policy_id", Value: policyID},
			)
			continue
		}
		if retentionDays <= 0 {
			logger.Warn("skipping retention policy with invalid retention_days",
				runner.Field{Key: "policy_id", Value: policyID},
				runner.Field{Key: "retention_days", Value: retentionDays},
			)
			continue
		}

		horizon := now.AddDate(0, 0, -retentionDays)
		deletionMode := classifyDeletionMode(targetEntityType, forceAuditReap)

		expiredCount := 0
		if deletionMode != "skip" {
			timeCol := timeColumnForEntity(targetEntityType)
			filters := map[string]interface{}{
				timeCol + "__lt": horizon,
			}
			if deletionMode == "soft_delete" {
				filters["is_active"] = true
			}
			countResult, err := client.Search(targetEntityType, filters, nil, 1)
			if err != nil {
				logger.Warn("failed to count expired rows, skipping target",
					runner.Field{Key: "policy_id", Value: policyID},
					runner.Field{Key: "target", Value: targetEntityType},
					runner.Field{Key: "error", Value: err.Error()},
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

	// busiest tables first so the most impactful reaping happens
	// before any bound is hit
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].ExpiredRowCount > targets[j].ExpiredRowCount
	})

	return targets, nil
}

// ReapTable deletes or soft-deletes expired rows from one table up to batchSize.
// Returns the number of rows affected.
func ReapTable(client *runner.APIClient, target *RetentionTarget, batchSize int, logger *runner.Logger) (int, error) {
	switch target.DeletionMode {
	case "hard_delete":
		return reapHardDelete(client, target, batchSize, logger)
	case "soft_delete":
		return reapSoftDelete(client, target, batchSize, logger)
	case "skip":
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown deletion mode %q for %s", target.DeletionMode, target.TargetEntityType)
	}
}

// reapHardDelete removes rows from observation cache or runner job tables.
func reapHardDelete(client *runner.APIClient, target *RetentionTarget, batchSize int, logger *runner.Logger) (int, error) {
	timeCol := timeColumnForEntity(target.TargetEntityType)
	filters := map[string]interface{}{
		timeCol + "__lt": target.Horizon,
	}

	totalDeleted := 0

	for {
		results, err := client.Search(target.TargetEntityType, filters, []string{timeCol + " asc"}, batchSize)
		if err != nil {
			return totalDeleted, fmt.Errorf("query failed for %s after %d deletions: %w",
				target.TargetEntityType, totalDeleted, err)
		}
		if len(results.Rows) == 0 {
			break
		}

		for _, row := range results.Rows {
			err := client.DeleteRow(target.TargetEntityType, row.ID)
			if err != nil {
				return totalDeleted, fmt.Errorf("delete failed for %s id=%d after %d deletions: %w",
					target.TargetEntityType, row.ID, totalDeleted, err)
			}
			totalDeleted++
		}

		logger.Info("hard-deleted batch",
			runner.Field{Key: "target", Value: target.TargetEntityType},
			runner.Field{Key: "batch_size", Value: len(results.Rows)},
			runner.Field{Key: "total_deleted", Value: totalDeleted},
		)

		// fewer results than requested means we've exhausted the expired rows
		if len(results.Rows) < batchSize {
			break
		}
	}

	return totalDeleted, nil
}

// reapSoftDelete sets is_active=false on entity rows past the retention horizon.
// Each soft-delete goes through the API as an auto-approved change set per
// the reaper's gating policy (direct write for retention enforcement).
func reapSoftDelete(client *runner.APIClient, target *RetentionTarget, batchSize int, logger *runner.Logger) (int, error) {
	timeCol := timeColumnForEntity(target.TargetEntityType)
	filters := map[string]interface{}{
		timeCol + "__lt": target.Horizon,
		"is_active":      true,
	}

	totalSoftDeleted := 0

	for {
		results, err := client.Search(target.TargetEntityType, filters, []string{timeCol + " asc"}, batchSize)
		if err != nil {
			return totalSoftDeleted, fmt.Errorf("query failed for %s after %d soft-deletions: %w",
				target.TargetEntityType, totalSoftDeleted, err)
		}
		if len(results.Rows) == 0 {
			break
		}

		for _, row := range results.Rows {
			err := client.WriteObservation(target.TargetEntityType, row.ID, "is_active", false)
			if err != nil {
				return totalSoftDeleted, fmt.Errorf("soft-delete failed for %s id=%d after %d: %w",
					target.TargetEntityType, row.ID, totalSoftDeleted, err)
			}
			totalSoftDeleted++
		}

		logger.Info("soft-deleted batch",
			runner.Field{Key: "target", Value: target.TargetEntityType},
			runner.Field{Key: "batch_size", Value: len(results.Rows)},
			runner.Field{Key: "total_soft_deleted", Value: totalSoftDeleted},
		)

		if len(results.Rows) < batchSize {
			break
		}
	}

	return totalSoftDeleted, nil
}

// ProcessCycle runs one complete get/act/set cycle for the reaper.
func ProcessCycle(client *runner.APIClient, batchSize int, tablesPerCycle int, maxDuration time.Duration, dryRun bool, logger *runner.Logger) (*ReaperSummary, error) {
	startTime := time.Now()
	summary := &ReaperSummary{}

	// GET phase: load retention policies and count expired rows
	targets, err := GetRetentionTargets(client, logger)
	if err != nil {
		return summary, fmt.Errorf("get phase failed: %w", err)
	}
	summary.PoliciesEvaluated = len(targets)

	logger.Info("retention targets loaded",
		runner.Field{Key: "policies_evaluated", Value: summary.PoliciesEvaluated},
		runner.Field{Key: "total_expired_rows", Value: totalExpiredRows(targets)},
	)

	// dry run: log what would happen, then return
	if dryRun {
		for _, t := range targets {
			logger.Info("dry run: retention target",
				runner.Field{Key: "policy_id", Value: t.PolicyID},
				runner.Field{Key: "policy_name", Value: t.PolicyName},
				runner.Field{Key: "target", Value: t.TargetEntityType},
				runner.Field{Key: "retention_days", Value: t.RetentionDays},
				runner.Field{Key: "horizon", Value: t.Horizon.Format(time.RFC3339)},
				runner.Field{Key: "expired_rows", Value: t.ExpiredRowCount},
				runner.Field{Key: "deletion_mode", Value: t.DeletionMode},
			)
		}
		return summary, nil
	}

	// ACT phase: reap each table within bounds
	for _, target := range targets {
		if summary.TablesProcessed >= tablesPerCycle {
			summary.BoundHits = append(summary.BoundHits, "tables_per_cycle")
			logger.Info("tables_per_cycle bound reached",
				runner.Field{Key: "limit", Value: tablesPerCycle},
			)
			break
		}

		elapsed := time.Since(startTime)
		if elapsed > maxDuration {
			summary.BoundHits = append(summary.BoundHits, "max_cycle_duration")
			logger.Info("max_cycle_duration bound reached",
				runner.Field{Key: "elapsed_seconds", Value: int(elapsed.Seconds())},
				runner.Field{Key: "limit_seconds", Value: int(maxDuration.Seconds())},
			)
			break
		}

		if target.ExpiredRowCount == 0 {
			summary.TablesSkipped++
			continue
		}

		if target.DeletionMode == "skip" {
			logger.Info("skipping table per policy",
				runner.Field{Key: "target", Value: target.TargetEntityType},
				runner.Field{Key: "policy_id", Value: target.PolicyID},
			)
			summary.TablesSkipped++
			continue
		}

		logger.Info("reaping table",
			runner.Field{Key: "target", Value: target.TargetEntityType},
			runner.Field{Key: "deletion_mode", Value: target.DeletionMode},
			runner.Field{Key: "expired_rows", Value: target.ExpiredRowCount},
			runner.Field{Key: "batch_size", Value: batchSize},
		)

		affected, err := ReapTable(client, &target, batchSize, logger)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s: %v", target.TargetEntityType, err))
			logger.Error("reap failed",
				runner.Field{Key: "target", Value: target.TargetEntityType},
				runner.Field{Key: "rows_before_failure", Value: affected},
				runner.Field{Key: "error", Value: err.Error()},
			)
			continue
		}

		summary.TablesProcessed++

		switch target.DeletionMode {
		case "hard_delete":
			summary.RowsDeleted += affected
		case "soft_delete":
			summary.RowsSoftDeleted += affected
		}

		logger.Info("reap complete for table",
			runner.Field{Key: "target", Value: target.TargetEntityType},
			runner.Field{Key: "rows_affected", Value: affected},
			runner.Field{Key: "deletion_mode", Value: target.DeletionMode},
		)
	}

	return summary, nil
}

// totalExpiredRows sums expired row counts across all targets for logging.
func totalExpiredRows(targets []RetentionTarget) int {
	total := 0
	for _, t := range targets {
		total += t.ExpiredRowCount
	}
	return total
}
