package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
	"github.com/ghowland/opsdb/tools/runners/reaper"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "usage: reaper --dos <dos-directory>\n")
		os.Exit(2)
	}
	_ = dosPath

	config, err := runner.Init("reaper")
	if err != nil {
		fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
		os.Exit(1)
	}

	for runner.ShouldRun(config) {
		if err := runner.RefreshConfig(config); err != nil {
			config.Logger.Warn("failed to refresh config, using cached",
				runner.Field("error", err.Error()),
			)
		}

		jobID, err := runner.StartCycle(config)
		if err != nil {
			config.Logger.Error("failed to start cycle", runner.Field("error", err.Error()))
			runner.WaitForNextCycle(config)
			continue
		}

		// --- GET ---
		batchSize, _ := runner.GetSpecDataInt(config, "batch_size")
		if batchSize == 0 {
			batchSize = 10000
		}
		tablesPerCycle, _ := runner.GetSpecDataInt(config, "tables_per_cycle")
		if tablesPerCycle == 0 {
			tablesPerCycle = 10
		}
		maxDurationSec, _ := runner.GetSpecDataInt(config, "max_cycle_duration_seconds")
		if maxDurationSec == 0 {
			maxDurationSec = 300
		}

		targets, err := reaper.GetRetentionTargets(config.Client, config.Logger)
		if err != nil {
			config.Logger.Error("failed to get retention targets",
				runner.Field("error", err.Error()),
			)
			runner.FinishCycle(config, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		config.Logger.Info("loaded retention targets",
			runner.Field("policies_evaluated", len(targets)),
			runner.Field("total_expired_rows", reaper.TotalExpiredRows(targets)),
		)

		// --- ACT ---
		if runner.IsDryRun(config) {
			planData := make([]map[string]interface{}, 0, len(targets))
			for _, t := range targets {
				planData = append(planData, map[string]interface{}{
					"policy_id":      t.PolicyID,
					"policy_name":    t.PolicyName,
					"target":         t.TargetEntityType,
					"retention_days": t.RetentionDays,
					"expired_rows":   t.ExpiredRowCount,
					"deletion_mode":  t.DeletionMode,
				})
			}
			runner.LogPlan(config.Logger, "retention targets", planData)
			runner.SkipActPhase(config.Logger)
			runner.SkipSetPhase(config.Logger)
			runner.FinishCycle(config, "completed", map[string]interface{}{
				"dry_run":            true,
				"policies_evaluated": len(targets),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		summary := &reaper.ReaperSummary{
			PoliciesEvaluated: len(targets),
		}
		cycleStart := time.Now()

		for _, target := range targets {
			if summary.TablesProcessed >= tablesPerCycle {
				config.Logger.Info("tables_per_cycle bound reached",
					runner.Field("limit", tablesPerCycle),
				)
				runner.RecordBoundHit(config, "tables_per_cycle", tablesPerCycle)
				summary.BoundHits = append(summary.BoundHits, "tables_per_cycle")
				break
			}

			elapsed := time.Since(cycleStart)
			if elapsed.Seconds() > float64(maxDurationSec) {
				config.Logger.Info("max_cycle_duration bound reached",
					runner.Field("elapsed_seconds", int(elapsed.Seconds())),
					runner.Field("limit_seconds", maxDurationSec),
				)
				runner.RecordBoundHit(config, "max_cycle_duration_seconds", maxDurationSec)
				summary.BoundHits = append(summary.BoundHits, "max_cycle_duration")
				break
			}

			if target.ExpiredRowCount == 0 || target.DeletionMode == "skip" {
				summary.TablesSkipped++
				continue
			}

			config.Logger.Info("reaping table",
				runner.Field("target", target.TargetEntityType),
				runner.Field("deletion_mode", target.DeletionMode),
				runner.Field("expired_rows", target.ExpiredRowCount),
			)

			affected, reapErr := reaper.ReapTable(config.Client, &target, batchSize, jobID, config.Logger)
			if reapErr != nil {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("%s: %v", target.TargetEntityType, reapErr))
				config.Logger.Error("reap failed",
					runner.Field("target", target.TargetEntityType),
					runner.Field("rows_before_failure", affected),
					runner.Field("error", reapErr.Error()),
				)
			}

			summary.TablesProcessed++

			switch target.DeletionMode {
			case "hard_delete":
				summary.RowsDeleted += affected
			case "soft_delete":
				summary.RowsSoftDeleted += affected
			}

			config.Logger.Info("reap complete for table",
				runner.Field("target", target.TargetEntityType),
				runner.Field("rows_affected", affected),
				runner.Field("deletion_mode", target.DeletionMode),
			)
		}

		// --- SET ---
		status := "completed"
		if len(summary.Errors) > 0 {
			status = "completed_with_errors"
		}

		runner.FinishCycle(config, status, map[string]interface{}{
			"policies_evaluated": summary.PoliciesEvaluated,
			"tables_processed":   summary.TablesProcessed,
			"tables_skipped":     summary.TablesSkipped,
			"rows_deleted":       summary.RowsDeleted,
			"rows_soft_deleted":  summary.RowsSoftDeleted,
			"bound_hits":         summary.BoundHits,
			"errors":             summary.Errors,
		})

		runner.WaitForNextCycle(config)
	}

	config.Logger.Info("reaper shutting down")
	os.Exit(0)
}
