//# tools/runners/reaper/reaper.go

go
package reaper

import (
	"time"
)

// RetentionTarget holds one retention policy and its computed deletion scope.
type RetentionTarget struct {
	PolicyID         int
	PolicyName       string
	TargetEntityType string
	RetentionDays    int
	Horizon          time.Time // now - retention_days
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

// GetRetentionTargets reads all active retention policies from OpsDB,
// computes the retention horizon for each, and counts expired rows.
func GetRetentionTargets(client interface{}) ([]RetentionTarget, error) {
	// TODO: search retention_policy where is_active=true
	// TODO: for each policy:
	//   extract target_entity_type from policy_data_json
	//   extract retention_days from policy_data_json
	//   compute horizon = time.Now().AddDate(0, 0, -retentionDays)
	//   determine deletion mode:
	//     if target is observation_cache_metric/state/config: hard_delete
	//     if target is entity with soft_delete: soft_delete
	//     if target is append_only (audit_log_entry): skip unless policy.force_audit_reap=true
	//     if target is runner_job: hard_delete (operational data, not audit)
	//   count expired rows:
	//     for hard_delete cache tables: count where _observed_time < horizon
	//     for soft_delete entities: count where created_time < horizon and is_active=true
	//     for runner_job: count where started_time < horizon
	//   build RetentionTarget
	// TODO: return targets sorted by expired row count descending (busiest first)
	return nil, nil
}

// ReapTable deletes or soft-deletes expired rows from one table up to batchSize.
// Returns the number of rows affected.
func ReapTable(client interface{}, target *RetentionTarget, batchSize int) (int, error) {
	// TODO: switch on target.DeletionMode:
	//   hard_delete:
	//     for observation cache tables:
	//       DELETE FROM {target.TargetEntityType}
	//       WHERE _observed_time < {target.Horizon}
	//       LIMIT {batchSize}
	//       (executed via API bulk delete operation or multiple individual deletes)
	//     for runner_job:
	//       DELETE FROM runner_job WHERE started_time < {target.Horizon} LIMIT {batchSize}
	//   soft_delete:
	//     search target entity where created_time < horizon and is_active=true
	//     limit to batchSize
	//     for each row: submit change_set with is_active=false
	//       (these go through change management as auto-approved per reaper policy)
	//   skip:
	//     return 0, nil (table explicitly skipped by policy)
	// TODO: return rows affected
	return 0, nil
}

// ProcessCycle runs one complete get/act/set cycle for the reaper.
func ProcessCycle(client interface{}, batchSize int, tablesPerCycle int, maxDuration time.Duration, dryRun bool) (*ReaperSummary, error) {
	startTime := time.Now()
	summary := &ReaperSummary{}

	// TODO: GET phase
	//   targets, err := GetRetentionTargets(client)
	//   summary.PoliciesEvaluated = len(targets)

	// TODO: if dryRun:
	//   for each target: log policy name, table, retention days, horizon, expired count, mode
	//   return summary

	// TODO: ACT phase
	//   tablesProcessed := 0
	//   for each target in targets:
	//     if tablesProcessed >= tablesPerCycle:
	//       summary.BoundHits = append(..., "tables_per_cycle")
	//       break
	//     if time.Since(startTime) > maxDuration:
	//       summary.BoundHits = append(..., "max_cycle_duration")
	//       break
	//     if target.ExpiredRowCount == 0:
	//       summary.TablesSkipped++
	//       continue
	//     if target.DeletionMode == "skip":
	//       summary.TablesSkipped++
	//       continue
	//     affected, err := ReapTable(client, &target, batchSize)
	//     if err:
	//       summary.Errors = append(...)
	//       continue
	//     tablesProcessed++
	//     summary.TablesProcessed++
	//     if target.DeletionMode == "hard_delete":
	//       summary.RowsDeleted += affected
	//     else:
	//       summary.RowsSoftDeleted += affected

	// TODO: return summary, nil
	_ = startTime
	return summary, nil
}


