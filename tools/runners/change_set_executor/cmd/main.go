package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
	executor "github.com/ghowland/opsdb/tools/runners/change_set_executor"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "usage: change-set-executor --dos <dos-directory>\n")
		os.Exit(2)
	}
	_ = dosPath

	config, err := runner.Init("change-set-executor")
	if err != nil {
		fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
		os.Exit(1)
	}

	for runner.ShouldRun(config) {
		jobID, err := runner.StartCycle(config)
		if err != nil {
			config.Logger.Error("failed to start cycle", runner.Field("error", err.Error()))
			runner.WaitForNextCycle(config)
			continue
		}
		client := config.Client.WithCorrelation(jobID, "")

		// --- GET ---
		batchSize, _ := runner.GetSpecDataInt(config, "batch_size")
		if batchSize == 0 {
			batchSize = 50
		}
		fieldChangeBatchSize, _ := runner.GetSpecDataInt(config, "field_change_batch_size")
		if fieldChangeBatchSize == 0 {
			fieldChangeBatchSize = 500
		}
		retryFailed, _ := runner.GetSpecDataBool(config, "retry_failed")
		maxDuration, _ := runner.GetSpecDataInt(config, "max_cycle_duration_seconds")
		if maxDuration == 0 {
			maxDuration = 120
		}

		batch, err := executor.GetApprovedChangeSets(client, batchSize, retryFailed)
		if err != nil {
			config.Logger.Error("failed to get approved change sets",
				runner.Field("error", err.Error()))
			runner.FinishCycle(config, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		config.Logger.Info("found approved change sets",
			runner.Field("total_found", batch.TotalFound),
			runner.Field("processing", len(batch.ChangeSets)),
		)

		// --- ACT ---
		if runner.IsDryRun(config) {
			planData := make([]map[string]interface{}, 0, len(batch.ChangeSets))
			for _, cs := range batch.ChangeSets {
				planData = append(planData, map[string]interface{}{
					"change_set_id":      cs.ChangeSetID,
					"name":               cs.Name,
					"field_change_count": len(cs.FieldChanges),
					"is_emergency":       cs.IsEmergency,
				})
			}
			runner.LogPlan(config.Logger, "change sets to apply", planData)
			runner.SkipActPhase(config.Logger)
			runner.SkipSetPhase(config.Logger)
			runner.FinishCycle(config, "completed", map[string]interface{}{
				"dry_run":           true,
				"change_sets_found": len(batch.ChangeSets),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		summary := &executor.CycleSummary{}
		cycleStart := time.Now()

		for _, cs := range batch.ChangeSets {
			if time.Since(cycleStart).Seconds() > float64(maxDuration) {
				config.Logger.Warn("max cycle duration reached, stopping")
				runner.RecordBoundHit(config, "max_cycle_duration", maxDuration)
				break
			}

			summary.ChangeSetsProcessed++

			config.Logger.Info("applying change set",
				runner.Field("change_set_id", cs.ChangeSetID),
				runner.Field("name", cs.Name),
				runner.Field("field_changes", len(cs.FieldChanges)),
			)

			applied, applyErr := executor.ApplyChangeSet(client, &cs)
			summary.FieldChangesApplied += applied

			if applyErr != nil {
				summary.ChangeSetsFailed++
				summary.FieldChangesFailed += len(cs.FieldChanges) - applied
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("change_set %d: %v", cs.ChangeSetID, applyErr))
				config.Logger.Error("change set apply failed",
					runner.Field("change_set_id", cs.ChangeSetID),
					runner.Field("applied", applied),
					runner.Field("total", len(cs.FieldChanges)),
					runner.Field("error", applyErr.Error()),
				)
				continue
			}

			if applied == len(cs.FieldChanges) {
				finalizeErr := executor.FinalizeChangeSet(client, cs.ChangeSetID)
				if finalizeErr != nil {
					summary.Errors = append(summary.Errors,
						fmt.Sprintf("finalize change_set %d: %v", cs.ChangeSetID, finalizeErr))
					config.Logger.Error("change set finalize failed",
						runner.Field("change_set_id", cs.ChangeSetID),
						runner.Field("error", finalizeErr.Error()),
					)
					continue
				}
				summary.ChangeSetsFullyApplied++
				config.Logger.Info("change set fully applied",
					runner.Field("change_set_id", cs.ChangeSetID),
					runner.Field("field_changes_applied", applied),
				)
			} else {
				config.Logger.Warn("change set partially applied",
					runner.Field("change_set_id", cs.ChangeSetID),
					runner.Field("applied", applied),
					runner.Field("total", len(cs.FieldChanges)),
				)
			}
		}

		// --- SET ---
		status := "completed"
		if len(summary.Errors) > 0 {
			status = "completed_with_errors"
		}
		if summary.ChangeSetsProcessed == 0 && batch.TotalFound == 0 {
			status = "completed"
		}

		runner.FinishCycle(config, status, map[string]interface{}{
			"change_sets_found":         batch.TotalFound,
			"change_sets_processed":     summary.ChangeSetsProcessed,
			"change_sets_fully_applied": summary.ChangeSetsFullyApplied,
			"change_sets_failed":        summary.ChangeSetsFailed,
			"field_changes_applied":     summary.FieldChangesApplied,
			"field_changes_failed":      summary.FieldChangesFailed,
			"errors":                    summary.Errors,
		})

		runner.WaitForNextCycle(config)
	}

	config.Logger.Info("change-set executor shutting down")
	os.Exit(0)
}
