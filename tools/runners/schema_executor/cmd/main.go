package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ghowland/opsdb/internal/pg"
	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
	executor "github.com/ghowland/opsdb/tools/runners/schema_executor"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "usage: schema_executor --dos <dos-directory>\n")
		os.Exit(2)
	}
	_ = dosPath

	config, err := runner.Init("schema_executor")
	if err != nil {
		fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
		os.Exit(1)
	}

	// Schema executor needs direct database access for DDL execution.
	// Read DSN from environment — same variable the opsdb_schema CLI uses.
	dsn := os.Getenv("OPSDB_DSN")
	if dsn == "" {
		fmt.Fprintf(os.Stderr, "error: OPSDB_DSN environment variable required for schema executor\n")
		os.Exit(2)
	}

	db, err := pg.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

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
		schemaRepoPath, ok := runner.GetSpecDataString(config, "schema_repo_path")
		if !ok || schemaRepoPath == "" {
			config.Logger.Error("schema_repo_path not configured in runner spec")
			runner.FinishCycle(config, "failed", map[string]interface{}{
				"error": "schema_repo_path not configured",
			})
			runner.WaitForNextCycle(config)
			continue
		}

		maxChanges, _ := runner.GetSpecDataInt(config, "max_changes_per_cycle")
		if maxChanges == 0 {
			maxChanges = 1
		}
		requireGitClean, ok := runner.GetSpecDataBool(config, "require_git_clean")
		if !ok {
			requireGitClean = true
		}
		autoPull, ok := runner.GetSpecDataBool(config, "auto_pull")
		if !ok {
			autoPull = true
		}

		changes, err := executor.GetApprovedSchemaChanges(config.Client, maxChanges)
		if err != nil {
			config.Logger.Error("failed to get approved schema changes",
				runner.Field("error", err.Error()),
			)
			runner.FinishCycle(config, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		if len(changes) == 0 {
			config.Logger.Info("no approved schema change sets to process")
			runner.FinishCycle(config, "completed", map[string]interface{}{
				"changes_found": 0,
			})
			runner.WaitForNextCycle(config)
			continue
		}

		config.Logger.Info("found approved schema change sets",
			runner.Field("count", len(changes)),
		)

		// --- ACT ---
		if runner.IsDryRun(config) {
			planData := make([]map[string]interface{}, 0, len(changes))
			for _, ch := range changes {
				planData = append(planData, map[string]interface{}{
					"change_set_id": ch.ChangeSetID,
					"label":         ch.Label,
					"description":   ch.Description,
					"target_commit": ch.TargetCommit,
					"created_time":  ch.CreatedTime,
				})
			}
			runner.LogPlan(config.Logger, "schema changes to apply", planData)
			runner.SkipActPhase(config.Logger)
			runner.SkipSetPhase(config.Logger)
			runner.FinishCycle(config, "completed", map[string]interface{}{
				"dry_run":       true,
				"changes_found": len(changes),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		// Validate git state once before processing any changes.
		if err := executor.ValidateGitState(schemaRepoPath, requireGitClean, autoPull); err != nil {
			config.Logger.Error("git state validation failed",
				runner.Field("error", err.Error()),
			)
			runner.FinishCycle(config, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		summary := &executor.SchemaExecutorSummary{}

		for _, change := range changes {
			summary.ChangesProcessed++

			config.Logger.Info("processing schema change set",
				runner.Field("change_set_id", change.ChangeSetID),
				runner.Field("label", change.Label),
				runner.Field("description", change.Description),
				runner.Field("target_commit", change.TargetCommit),
			)

			// Checkout target commit for this change.
			if err := executor.CheckoutCommit(schemaRepoPath, change.TargetCommit); err != nil {
				config.Logger.Error("failed to checkout target commit",
					runner.Field("change_set_id", change.ChangeSetID),
					runner.Field("commit", change.TargetCommit),
					runner.Field("error", err.Error()),
				)
				summary.ChangesFailed++
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("change_set %d: checkout failed: %v", change.ChangeSetID, err))

				// Mark as failed via API.
				failResult := &executor.SchemaApplyResult{
					ChangeSetID:  change.ChangeSetID,
					Success:      false,
					ErrorMessage: fmt.Sprintf("checkout failed: %v", err),
				}
				executor.FinalizeSchemaChange(config.Client, change.ChangeSetID, failResult, jobID)
				continue
			}

			// Run the schema loader pipeline.
			result, err := executor.ApplySchemaChange(db, schemaRepoPath, &change, true)
			if err != nil {
				config.Logger.Error("unexpected error during schema apply",
					runner.Field("change_set_id", change.ChangeSetID),
					runner.Field("error", err.Error()),
				)
				summary.ChangesFailed++
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("change_set %d: unexpected error: %v", change.ChangeSetID, err))

				failResult := &executor.SchemaApplyResult{
					ChangeSetID:  change.ChangeSetID,
					Success:      false,
					ErrorMessage: fmt.Sprintf("unexpected error: %v", err),
				}
				executor.FinalizeSchemaChange(config.Client, change.ChangeSetID, failResult, jobID)
				continue
			}

			summary.Results = append(summary.Results, *result)

			if result.Success {
				summary.ChangesApplied++
				config.Logger.Info("schema change set applied",
					runner.Field("change_set_id", change.ChangeSetID),
					runner.Field("tables_created", result.TablesCreated),
					runner.Field("fields_added", result.FieldsAdded),
					runner.Field("constraints_modified", result.ConstraintsModified),
					runner.Field("indexes_created", result.IndexesCreated),
					runner.Field("statements_executed", result.StatementsExecuted),
				)
			} else {
				summary.ChangesFailed++
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("change_set %d: %s", change.ChangeSetID, result.ErrorMessage))

				if len(result.ForbiddenChanges) > 0 {
					for _, forbidden := range result.ForbiddenChanges {
						config.Logger.Error("forbidden schema change",
							runner.Field("change_set_id", change.ChangeSetID),
							runner.Field("detail", forbidden),
						)
					}
				}
				config.Logger.Error("schema change set failed",
					runner.Field("change_set_id", change.ChangeSetID),
					runner.Field("error", result.ErrorMessage),
				)
			}

			// --- SET per change: finalize status and write evidence ---
			finalizeErr := executor.FinalizeSchemaChange(config.Client, change.ChangeSetID, result, jobID)
			if finalizeErr != nil {
				config.Logger.Error("failed to finalize schema change set status",
					runner.Field("change_set_id", change.ChangeSetID),
					runner.Field("error", finalizeErr.Error()),
				)
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("change_set %d: finalize failed: %v", change.ChangeSetID, finalizeErr))
			}

			evidenceErr := executor.WriteSchemaEvidence(config.Client, result, jobID)
			if evidenceErr != nil {
				config.Logger.Warn("failed to write schema evidence record",
					runner.Field("change_set_id", change.ChangeSetID),
					runner.Field("error", evidenceErr.Error()),
				)
			}
		}

		// --- SET ---
		status := "completed"
		if summary.ChangesFailed > 0 && summary.ChangesApplied > 0 {
			status = "completed_with_errors"
		} else if summary.ChangesFailed > 0 && summary.ChangesApplied == 0 {
			status = "failed"
		}

		runner.FinishCycle(config, status, map[string]interface{}{
			"changes_found":     len(changes),
			"changes_processed": summary.ChangesProcessed,
			"changes_applied":   summary.ChangesApplied,
			"changes_failed":    summary.ChangesFailed,
			"errors":            summary.Errors,
		})

		runner.WaitForNextCycle(config)
	}

	config.Logger.Info("schema executor shutting down")
	os.Exit(0)
}
