//# tools/runners/schema-executor/executor.go

package executor

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
	"github.com/ghowland/opsdb/tools/opsdb-schema/loader"
)

// SchemaChangeWork holds one approved schema change set to process.
type SchemaChangeWork struct {
	ChangeSetID  int
	Label        string
	Description  string
	TargetCommit string
	CreatedTime  time.Time
}

// SchemaApplyResult holds the outcome of applying one schema change.
type SchemaApplyResult struct {
	ChangeSetID        int
	Success            bool
	TablesCreated      int
	FieldsAdded        int
	ConstraintsModified int
	IndexesCreated     int
	StatementsExecuted int
	ForbiddenChanges   []string
	ErrorMessage       string
}

// SchemaExecutorSummary holds the results of one executor cycle.
type SchemaExecutorSummary struct {
	ChangesProcessed int
	ChangesApplied   int
	ChangesFailed    int
	Results          []SchemaApplyResult
	Errors           []string
}

// GetApprovedSchemaChanges reads approved schema change sets from OpsDB.
func GetApprovedSchemaChanges(client *runner.APIClient, maxChanges int) ([]SchemaChangeWork, error) {
	results, err := client.Search("_schema_change_set", map[string]interface{}{
		"status": "approved",
	}, []string{"created_time asc"}, maxChanges)
	if err != nil {
		return nil, fmt.Errorf("failed to search approved schema change sets: %w", err)
	}

	changes := make([]SchemaChangeWork, 0, len(results.Rows))
	for _, row := range results.Rows {
		changeSetID, _ := row.IntField("id")
		label, _ := row.StringField("label")
		description, _ := row.StringField("description")
		targetCommit, _ := row.StringField("target_commit_hash")
		createdTime, _ := row.TimeField("created_time")

		changes = append(changes, SchemaChangeWork{
			ChangeSetID:  changeSetID,
			Label:        label,
			Description:  description,
			TargetCommit: targetCommit,
			CreatedTime:  createdTime,
		})
	}

	return changes, nil
}

// ValidateGitState checks that the schema repository is in a clean state
// suitable for apply. Optionally pulls latest changes.
func ValidateGitState(repoPath string, requireClean bool, autoPull bool) error {
	if requireClean {
		output, err := runGit(repoPath, "status", "--porcelain")
		if err != nil {
			return fmt.Errorf("git status failed: %w", err)
		}
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("schema repo has uncommitted changes at %s:\n%s", repoPath, trimmed)
		}
	}

	if autoPull {
		_, err := runGit(repoPath, "pull", "--ff-only")
		if err != nil {
			return fmt.Errorf("git pull --ff-only failed: %w", err)
		}
	}

	return nil
}

// CheckoutCommit switches the schema repo to a specific commit.
// If commit is empty, stays at current HEAD.
func CheckoutCommit(repoPath string, commit string) error {
	if commit == "" {
		return nil
	}

	_, err := runGit(repoPath, "checkout", commit)
	if err != nil {
		return fmt.Errorf("git checkout %s failed: %w", commit, err)
	}

	return nil
}

// ApplySchemaChange runs the full schema loader pipeline for one change.
// Returns the apply result with counts and any errors.
func ApplySchemaChange(repoPath string, db *runner.DBHandle, change *SchemaChangeWork, verbose bool) (*SchemaApplyResult, error) {
	result := &SchemaApplyResult{ChangeSetID: change.ChangeSetID}

	// step 1: load schema from repo
	schema, err := loader.Load(repoPath)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("schema load failed: %v", err)
		return result, nil
	}

	// step 2: read current database state
	current, err := loader.ReadCurrentState(db.DB)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("read current state failed: %v", err)
		return result, nil
	}

	// step 3: diff desired vs current
	diff, err := loader.Diff(schema, current)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("schema diff failed: %v", err)
		return result, nil
	}

	// step 4: check evolution rules
	evolution, err := loader.CheckEvolution(diff)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("evolution check failed: %v", err)
		return result, nil
	}

	if len(evolution.Forbidden) > 0 {
		result.Success = false
		for _, f := range evolution.Forbidden {
			result.ForbiddenChanges = append(result.ForbiddenChanges,
				fmt.Sprintf("%s.%s: %s (alternative: %s)", f.Entity, f.Field, f.Rule, f.Alternative))
		}
		result.ErrorMessage = fmt.Sprintf("schema change contains %d forbidden modifications", len(evolution.Forbidden))
		return result, nil
	}

	// step 5: generate DDL
	statements, err := loader.GenerateDDL(schema, evolution.Allowed)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("DDL generation failed: %v", err)
		return result, nil
	}

	if len(statements) == 0 {
		result.Success = true
		result.StatementsExecuted = 0
		return result, nil
	}

	// step 6: apply DDL within transaction with advisory lock
	applyResult, err := loader.Apply(db.DB, statements, verbose)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("DDL apply failed: %v", err)
		return result, nil
	}

	result.TablesCreated = applyResult.EntitiesCreated
	result.FieldsAdded = applyResult.FieldsAdded
	result.ConstraintsModified = applyResult.ConstraintsModified
	result.IndexesCreated = applyResult.IndexesCreated
	result.StatementsExecuted = applyResult.StatementsExecuted

	// step 7: populate meta tables
	// meta population happens within the same transaction that apply
	// used, so the DDL and meta updates are atomic
	label := change.Label
	if label == "" {
		label = fmt.Sprintf("schema_change_set_%d", change.ChangeSetID)
	}
	err = loader.PopulateMeta(db.DB, schema, evolution.Allowed, label)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("meta population failed after DDL apply: %v", err)
		return result, nil
	}

	result.Success = true
	return result, nil
}

// FinalizeSchemaChange updates the _schema_change_set status to applied
// or failed based on the apply result.
func FinalizeSchemaChange(client *runner.APIClient, changeSetID int, success bool, errorMessage string) error {
	if success {
		err := client.WriteObservation("_schema_change_set", changeSetID, "status", "applied")
		if err != nil {
			return fmt.Errorf("failed to mark schema change set %d as applied: %w", changeSetID, err)
		}
		err = client.WriteObservation("_schema_change_set", changeSetID, "applied_time", time.Now().UTC())
		if err != nil {
			return fmt.Errorf("failed to set applied_time on schema change set %d: %w", changeSetID, err)
		}
	} else {
		err := client.WriteObservation("_schema_change_set", changeSetID, "status", "failed")
		if err != nil {
			return fmt.Errorf("failed to mark schema change set %d as failed: %w", changeSetID, err)
		}
		err = client.WriteObservation("_schema_change_set", changeSetID, "error_message", errorMessage)
		if err != nil {
			return fmt.Errorf("failed to set error_message on schema change set %d: %w", changeSetID, err)
		}
	}

	return nil
}

// WriteSchemaEvidence creates an evidence_record documenting the schema change.
func WriteSchemaEvidence(client *runner.APIClient, result *SchemaApplyResult, runnerJobID int) error {
	evidenceData := map[string]interface{}{
		"change_set_id":        result.ChangeSetID,
		"success":              result.Success,
		"tables_created":       result.TablesCreated,
		"fields_added":         result.FieldsAdded,
		"constraints_modified": result.ConstraintsModified,
		"indexes_created":      result.IndexesCreated,
		"statements_executed":  result.StatementsExecuted,
	}

	if len(result.ForbiddenChanges) > 0 {
		evidenceData["forbidden_changes"] = result.ForbiddenChanges
	}
	if result.ErrorMessage != "" {
		evidenceData["error_message"] = result.ErrorMessage
	}

	outcome := "pass"
	if !result.Success {
		outcome = "fail"
	}

	description := fmt.Sprintf("Schema change set %d", result.ChangeSetID)
	if result.Success {
		description = fmt.Sprintf("Schema change set %d applied: %d tables created, %d fields added, %d constraints modified",
			result.ChangeSetID, result.TablesCreated, result.FieldsAdded, result.ConstraintsModified)
	} else {
		description = fmt.Sprintf("Schema change set %d failed: %s", result.ChangeSetID, result.ErrorMessage)
	}

	evidence := map[string]interface{}{
		"evidence_record_type":      "schema_evolution",
		"description":               description,
		"outcome":                   outcome,
		"runner_job_id":             runnerJobID,
		"evidence_record_data_json": evidenceData,
	}

	_, err := client.CreateEntity("evidence_record", evidence)
	if err != nil {
		return fmt.Errorf("failed to write schema evidence record for change set %d: %w", result.ChangeSetID, err)
	}

	return nil
}

// ProcessCycle runs one complete get/act/set cycle for the schema executor.
func ProcessCycle(client *runner.APIClient, repoPath string, db *runner.DBHandle, maxChanges int, requireClean bool, autoPull bool, dryRun bool, runnerJobID int, logger *runner.Logger) (*SchemaExecutorSummary, error) {
	summary := &SchemaExecutorSummary{}

	// GET phase: read approved schema change sets
	changes, err := GetApprovedSchemaChanges(client, maxChanges)
	if err != nil {
		return summary, fmt.Errorf("get phase failed: %w", err)
	}
	summary.ChangesProcessed = len(changes)

	if len(changes) == 0 {
		logger.Info("no approved schema change sets to process")
		return summary, nil
	}

	logger.Info("found approved schema change sets",
		runner.Field{Key: "count", Value: len(changes)},
	)

	// dry run: log what would happen, then return
	if dryRun {
		for _, change := range changes {
			logger.Info("dry run: would apply schema change set",
				runner.Field{Key: "change_set_id", Value: change.ChangeSetID},
				runner.Field{Key: "label", Value: change.Label},
				runner.Field{Key: "description", Value: change.Description},
				runner.Field{Key: "target_commit", Value: change.TargetCommit},
				runner.Field{Key: "created_time", Value: change.CreatedTime.Format(time.RFC3339)},
			)
		}
		return summary, nil
	}

	// validate git state once before processing any changes
	err = ValidateGitState(repoPath, requireClean, autoPull)
	if err != nil {
		return summary, fmt.Errorf("git state validation failed: %w", err)
	}

	// ACT phase: apply each schema change set in order
	for _, change := range changes {
		changeLogger := logger.With(
			runner.Field{Key: "schema_change_set_id", Value: change.ChangeSetID},
			runner.Field{Key: "label", Value: change.Label},
		)

		// checkout target commit for this change
		err := CheckoutCommit(repoPath, change.TargetCommit)
		if err != nil {
			changeLogger.Error("failed to checkout target commit",
				runner.Field{Key: "commit", Value: change.TargetCommit},
				runner.Field{Key: "error", Value: err.Error()},
			)
			summary.ChangesFailed++
			summary.Errors = append(summary.Errors,
				fmt.Sprintf("change_set %d: checkout failed: %v", change.ChangeSetID, err))

			finalizeErr := FinalizeSchemaChange(client, change.ChangeSetID, false, err.Error())
			if finalizeErr != nil {
				changeLogger.Error("failed to finalize schema change set",
					runner.Field{Key: "error", Value: finalizeErr.Error()},
				)
			}
			continue
		}

		changeLogger.Info("applying schema change set",
			runner.Field{Key: "description", Value: change.Description},
			runner.Field{Key: "target_commit", Value: change.TargetCommit},
		)

		result, err := ApplySchemaChange(repoPath, db, &change, true)
		if err != nil {
			// unexpected error from the apply function itself (not a schema error)
			changeLogger.Error("unexpected error during schema apply",
				runner.Field{Key: "error", Value: err.Error()},
			)
			summary.ChangesFailed++
			summary.Errors = append(summary.Errors,
				fmt.Sprintf("change_set %d: unexpected error: %v", change.ChangeSetID, err))

			finalizeErr := FinalizeSchemaChange(client, change.ChangeSetID, false, err.Error())
			if finalizeErr != nil {
				changeLogger.Error("failed to finalize schema change set",
					runner.Field{Key: "error", Value: finalizeErr.Error()},
				)
			}
			continue
		}

		summary.Results = append(summary.Results, *result)

		if result.Success {
			summary.ChangesApplied++
			changeLogger.Info("schema change set applied",
				runner.Field{Key: "tables_created", Value: result.TablesCreated},
				runner.Field{Key: "fields_added", Value: result.FieldsAdded},
				runner.Field{Key: "constraints_modified", Value: result.ConstraintsModified},
				runner.Field{Key: "indexes_created", Value: result.IndexesCreated},
				runner.Field{Key: "statements_executed", Value: result.StatementsExecuted},
			)
		} else {
			summary.ChangesFailed++
			summary.Errors = append(summary.Errors,
				fmt.Sprintf("change_set %d: %s", change.ChangeSetID, result.ErrorMessage))

			if len(result.ForbiddenChanges) > 0 {
				for _, forbidden := range result.ForbiddenChanges {
					changeLogger.Error("forbidden schema change",
						runner.Field{Key: "detail", Value: forbidden},
					)
				}
			}
			changeLogger.Error("schema change set failed",
				runner.Field{Key: "error", Value: result.ErrorMessage},
			)
		}

		// SET phase per change: finalize status and write evidence
		finalizeErr := FinalizeSchemaChange(client, change.ChangeSetID, result.Success, result.ErrorMessage)
		if finalizeErr != nil {
			changeLogger.Error("failed to finalize schema change set status",
				runner.Field{Key: "error", Value: finalizeErr.Error()},
			)
			summary.Errors = append(summary.Errors,
				fmt.Sprintf("change_set %d: finalize failed: %v", change.ChangeSetID, finalizeErr))
		}

		evidenceErr := WriteSchemaEvidence(client, result, runnerJobID)
		if evidenceErr != nil {
			changeLogger.Warn("failed to write schema evidence record",
				runner.Field{Key: "error", Value: evidenceErr.Error()},
			)
		}
	}

	return summary, nil
}

// runGit executes a git command in the given directory and returns
// combined output. Thin wrapper over os/exec.
func runGit(repoPath string, args ...string) ([]byte, error) {
	fullArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("git %s: %w (output: %s)",
			strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
