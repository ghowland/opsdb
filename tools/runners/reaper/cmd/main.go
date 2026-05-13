//# tools/runners/reaper/cmd/main.go

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS config directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "error: --dos flag is required\n")
		os.Exit(2)
	}

	config, err := runner.Init("reaper", *dosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize reaper: %v\n", err)
		os.Exit(2)
	}
	defer runner.Shutdown(config)

	logger := runner.NewLogger(config)
	logger.Info("reaper starting",
		runner.Field{Key: "dos_path", Value: *dosPath},
		runner.Field{Key: "batch_size", Value: config.SpecData.IntOrDefault("batch_size", 10000)},
		runner.Field{Key: "tables_per_cycle", Value: config.SpecData.IntOrDefault("tables_per_cycle", 10)},
		runner.Field{Key: "max_cycle_duration_seconds", Value: config.SpecData.IntOrDefault("max_cycle_duration_seconds", 300)},
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", runner.Field{Key: "signal", Value: sig.String()})
		runner.RequestShutdown(config)
	}()

	for runner.ShouldRun(config) {
		err := runner.RefreshConfig(config)
		if err != nil {
			logger.Warn("failed to refresh config, using cached", runner.Field{Key: "error", Value: err.Error()})
		}

		jobID, err := runner.StartCycle(config)
		if err != nil {
			logger.Error("failed to start cycle", runner.Field{Key: "error", Value: err.Error()})
			runner.WaitForNextCycle(config)
			continue
		}
		cycleLogger := logger.WithJobID(jobID)

		summary, err := runCycle(config, cycleLogger)
		if err != nil {
			cycleLogger.Error("cycle failed", runner.Field{Key: "error", Value: err.Error()})
			runner.FinishCycle(config, jobID, "failed", summary)
		} else {
			cycleLogger.Info("cycle complete",
				runner.Field{Key: "policies_evaluated", Value: summary.PoliciesEvaluated},
				runner.Field{Key: "tables_processed", Value: summary.TablesProcessed},
				runner.Field{Key: "rows_deleted", Value: summary.RowsDeleted},
				runner.Field{Key: "rows_soft_deleted", Value: summary.RowsSoftDeleted},
				runner.Field{Key: "tables_skipped", Value: summary.TablesSkipped},
			)
			runner.FinishCycle(config, jobID, "succeeded", summary)
		}

		runner.WaitForNextCycle(config)
	}

	logger.Info("reaper stopped")
	os.Exit(0)
}

func runCycle(config *runner.RunnerConfig, logger *runner.Logger) (*ReaperSummary, error) {
	summary := &ReaperSummary{}
	client := config.APIClient
	batchSize := config.SpecData.IntOrDefault("batch_size", 10000)
	tablesPerCycle := config.SpecData.IntOrDefault("tables_per_cycle", 10)
	maxCycleDuration := config.SpecData.IntOrDefault("max_cycle_duration_seconds", 300)
	dryRunLogCount := config.SpecData.IntOrDefault("dry_run_log_count", 100)
	dryRun := runner.IsDryRun(config)
	deadline := runner.NewDeadline(maxCycleDuration)

	// GET: read all active retention policies
	policies, err := client.Search("retention_policy", map[string]interface{}{
		"is_active": true,
	}, nil, 0)
	if err != nil {
		return summary, fmt.Errorf("failed to search retention policies: %w", err)
	}

	logger.Info("loaded retention policies", runner.Field{Key: "count", Value: len(policies.Rows)})
	summary.PoliciesEvaluated = len(policies.Rows)

	tablesProcessed := 0

	for _, policyRow := range policies.Rows {
		if deadline.Exceeded() {
			logger.Warn("max cycle duration exceeded, stopping early",
				runner.Field{Key: "tables_processed", Value: tablesProcessed},
			)
			runner.RecordBoundHit(config, "max_cycle_duration_seconds", maxCycleDuration)
			break
		}
		if tablesProcessed >= tablesPerCycle {
			logger.Info("tables_per_cycle limit reached",
				runner.Field{Key: "tables_per_cycle", Value: tablesPerCycle},
			)
			runner.RecordBoundHit(config, "tables_per_cycle", tablesPerCycle)
			break
		}

		targetEntityType, _ := policyRow.StringField("target_entity_type")
		retentionDays, _ := policyRow.IntField("retention_days")
		policyID, _ := policyRow.IntField("id")

		if targetEntityType == "" || retentionDays <= 0 {
			logger.Warn("skipping invalid retention policy",
				runner.Field{Key: "policy_id", Value: policyID},
				runner.Field{Key: "target_entity_type", Value: targetEntityType},
				runner.Field{Key: "retention_days", Value: retentionDays},
			)
			continue
		}

		horizon := runner.DaysAgo(retentionDays)
		tableKind := classifyTable(targetEntityType)

		logger.Info("evaluating retention policy",
			runner.Field{Key: "policy_id", Value: policyID},
			runner.Field{Key: "target_entity_type", Value: targetEntityType},
			runner.Field{Key: "retention_days", Value: retentionDays},
			runner.Field{Key: "table_kind", Value: tableKind},
			runner.Field{Key: "horizon", Value: horizon.Format("2006-01-02T15:04:05Z")},
		)

		switch tableKind {
		case tableKindObservationCache:
			count, err := reapObservationCache(client, targetEntityType, horizon, batchSize, dryRun, dryRunLogCount, logger)
			if err != nil {
				logger.Error("failed to reap observation cache",
					runner.Field{Key: "target", Value: targetEntityType},
					runner.Field{Key: "error", Value: err.Error()},
				)
				summary.Errors = append(summary.Errors, err.Error())
			} else if count > 0 {
				summary.RowsDeleted += count
			} else {
				summary.TablesSkipped++
			}
			tablesProcessed++

		case tableKindSoftDelete:
			count, err := reapSoftDelete(client, targetEntityType, horizon, batchSize, dryRun, dryRunLogCount, logger)
			if err != nil {
				logger.Error("failed to soft-delete expired rows",
					runner.Field{Key: "target", Value: targetEntityType},
					runner.Field{Key: "error", Value: err.Error()},
				)
				summary.Errors = append(summary.Errors, err.Error())
			} else if count > 0 {
				summary.RowsSoftDeleted += count
			} else {
				summary.TablesSkipped++
			}
			tablesProcessed++

		case tableKindAppendOnly:
			allowAppendOnlyReap, _ := policyRow.BoolField("allow_append_only_reap")
			if !allowAppendOnlyReap {
				logger.Info("skipping append-only table without explicit reap permission",
					runner.Field{Key: "target", Value: targetEntityType},
					runner.Field{Key: "policy_id", Value: policyID},
				)
				summary.TablesSkipped++
				tablesProcessed++
				continue
			}
			count, err := reapAppendOnly(client, targetEntityType, horizon, batchSize, dryRun, dryRunLogCount, logger)
			if err != nil {
				logger.Error("failed to reap append-only table",
					runner.Field{Key: "target", Value: targetEntityType},
					runner.Field{Key: "error", Value: err.Error()},
				)
				summary.Errors = append(summary.Errors, err.Error())
			} else if count > 0 {
				summary.RowsDeleted += count
			} else {
				summary.TablesSkipped++
			}
			tablesProcessed++

		default:
			logger.Warn("unknown table kind for retention target",
				runner.Field{Key: "target", Value: targetEntityType},
				runner.Field{Key: "table_kind", Value: tableKind},
			)
			summary.TablesSkipped++
			tablesProcessed++
		}
	}

	summary.TablesProcessed = tablesProcessed
	return summary, nil
}

// reapObservationCache hard-deletes rows from observation cache tables
// where _observed_time is past the retention horizon. Cache data is not
// source of truth, so hard delete is safe.
func reapObservationCache(client *runner.APIClient, entityType string, horizon runner.Time, batchSize int, dryRun bool, dryRunLogCount int, logger *runner.Logger) (int, error) {
	filters := map[string]interface{}{
		"_observed_time__lt": horizon,
	}

	if dryRun {
		results, err := client.Search(entityType, filters, nil, dryRunLogCount)
		if err != nil {
			return 0, fmt.Errorf("dry run count query failed: %w", err)
		}
		logger.Info("dry run: would delete observation cache rows",
			runner.Field{Key: "target", Value: entityType},
			runner.Field{Key: "sample_count", Value: len(results.Rows)},
			runner.Field{Key: "total_matching", Value: results.TotalCount},
		)
		for i, row := range results.Rows {
			if i >= dryRunLogCount {
				break
			}
			observedTime, _ := row.StringField("_observed_time")
			stateKey, _ := row.StringField("state_key")
			logger.Debug("dry run: would delete",
				runner.Field{Key: "entity_type", Value: entityType},
				runner.Field{Key: "id", Value: row.ID},
				runner.Field{Key: "observed_time", Value: observedTime},
				runner.Field{Key: "state_key", Value: stateKey},
			)
		}
		return 0, nil
	}

	totalDeleted := 0
	for {
		results, err := client.Search(entityType, filters, []string{"_observed_time asc"}, batchSize)
		if err != nil {
			return totalDeleted, fmt.Errorf("batch query failed after %d deletions: %w", totalDeleted, err)
		}
		if len(results.Rows) == 0 {
			break
		}

		for _, row := range results.Rows {
			err := client.DeleteRow(entityType, row.ID)
			if err != nil {
				return totalDeleted, fmt.Errorf("delete failed for %s id=%d after %d deletions: %w",
					entityType, row.ID, totalDeleted, err)
			}
			totalDeleted++
		}

		logger.Info("deleted observation cache batch",
			runner.Field{Key: "target", Value: entityType},
			runner.Field{Key: "batch_deleted", Value: len(results.Rows)},
			runner.Field{Key: "total_deleted", Value: totalDeleted},
		)

		if len(results.Rows) < batchSize {
			break
		}
	}

	return totalDeleted, nil
}

// reapSoftDelete sets is_active=false on entity rows where created_time
// is past the retention horizon and is_active is currently true.
// Entity rows are never hard-deleted; soft delete preserves history.
func reapSoftDelete(client *runner.APIClient, entityType string, horizon runner.Time, batchSize int, dryRun bool, dryRunLogCount int, logger *runner.Logger) (int, error) {
	filters := map[string]interface{}{
		"created_time__lt": horizon,
		"is_active":        true,
	}

	if dryRun {
		results, err := client.Search(entityType, filters, nil, dryRunLogCount)
		if err != nil {
			return 0, fmt.Errorf("dry run count query failed: %w", err)
		}
		logger.Info("dry run: would soft-delete entity rows",
			runner.Field{Key: "target", Value: entityType},
			runner.Field{Key: "sample_count", Value: len(results.Rows)},
			runner.Field{Key: "total_matching", Value: results.TotalCount},
		)
		for i, row := range results.Rows {
			if i >= dryRunLogCount {
				break
			}
			createdTime, _ := row.StringField("created_time")
			logger.Debug("dry run: would soft-delete",
				runner.Field{Key: "entity_type", Value: entityType},
				runner.Field{Key: "id", Value: row.ID},
				runner.Field{Key: "created_time", Value: createdTime},
			)
		}
		return 0, nil
	}

	totalSoftDeleted := 0
	for {
		results, err := client.Search(entityType, filters, []string{"created_time asc"}, batchSize)
		if err != nil {
			return totalSoftDeleted, fmt.Errorf("batch query failed after %d soft-deletions: %w", totalSoftDeleted, err)
		}
		if len(results.Rows) == 0 {
			break
		}

		for _, row := range results.Rows {
			err := client.WriteObservation(entityType, row.ID, "is_active", false)
			if err != nil {
				return totalSoftDeleted, fmt.Errorf("soft-delete failed for %s id=%d after %d: %w",
					entityType, row.ID, totalSoftDeleted, err)
			}
			totalSoftDeleted++
		}

		logger.Info("soft-deleted entity batch",
			runner.Field{Key: "target", Value: entityType},
			runner.Field{Key: "batch_soft_deleted", Value: len(results.Rows)},
			runner.Field{Key: "total_soft_deleted", Value: totalSoftDeleted},
		)

		if len(results.Rows) < batchSize {
			break
		}
	}

	return totalSoftDeleted, nil
}

// reapAppendOnly hard-deletes rows from append-only tables when the
// retention policy explicitly allows it. This is rare — audit retention
// is typically 7+ years. Only used for tables like runner_job where
// retention is shorter by policy.
func reapAppendOnly(client *runner.APIClient, entityType string, horizon runner.Time, batchSize int, dryRun bool, dryRunLogCount int, logger *runner.Logger) (int, error) {
	filters := map[string]interface{}{
		"created_time__lt": horizon,
	}

	if dryRun {
		results, err := client.Search(entityType, filters, nil, dryRunLogCount)
		if err != nil {
			return 0, fmt.Errorf("dry run count query failed: %w", err)
		}
		logger.Info("dry run: would delete append-only rows",
			runner.Field{Key: "target", Value: entityType},
			runner.Field{Key: "sample_count", Value: len(results.Rows)},
			runner.Field{Key: "total_matching", Value: results.TotalCount},
		)
		return 0, nil
	}

	totalDeleted := 0
	for {
		results, err := client.Search(entityType, filters, []string{"created_time asc"}, batchSize)
		if err != nil {
			return totalDeleted, fmt.Errorf("batch query failed after %d deletions: %w", totalDeleted, err)
		}
		if len(results.Rows) == 0 {
			break
		}

		for _, row := range results.Rows {
			err := client.DeleteRow(entityType, row.ID)
			if err != nil {
				return totalDeleted, fmt.Errorf("delete failed for %s id=%d after %d: %w",
					entityType, row.ID, totalDeleted, err)
			}
			totalDeleted++
		}

		logger.Info("deleted append-only batch",
			runner.Field{Key: "target", Value: entityType},
			runner.Field{Key: "batch_deleted", Value: len(results.Rows)},
			runner.Field{Key: "total_deleted", Value: totalDeleted},
		)

		if len(results.Rows) < batchSize {
			break
		}
	}

	return totalDeleted, nil
}

const (
	tableKindObservationCache = "observation_cache"
	tableKindSoftDelete       = "soft_delete"
	tableKindAppendOnly       = "append_only"
	tableKindUnknown          = "unknown"
)

// classifyTable determines reaping strategy from entity type name.
// Observation cache tables get hard-deleted. Most entity tables get
// soft-deleted. Append-only tables (runner_job, audit-adjacent) require
// explicit policy permission.
func classifyTable(entityType string) string {
	switch entityType {
	case "observation_cache_metric", "observation_cache_state", "observation_cache_config":
		return tableKindObservationCache
	case "runner_job", "runner_job_output_var",
		"runner_job_target_machine", "runner_job_target_service",
		"runner_job_target_k8s_workload", "runner_job_target_cloud_resource":
		return tableKindAppendOnly
	case "audit_log_entry":
		return tableKindAppendOnly
	default:
		return tableKindSoftDelete
	}
}

// ReaperSummary holds the results of one reaper cycle for runner_job reporting.
type ReaperSummary struct {
	PoliciesEvaluated int
	TablesProcessed   int
	TablesSkipped     int
	RowsDeleted       int
	RowsSoftDeleted   int
	Errors            []string
}
