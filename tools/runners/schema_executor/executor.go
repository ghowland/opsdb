package executor

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
	"github.com/ghowland/opsdb/tools/opsdb_schema/loader"
)

// SchemaChangeWork holds one approved schema change set to process.
type SchemaChangeWork struct {
	ChangeSetID  int
	Label        string
	Description  string
	TargetCommit string
	CreatedTime  string
}

// SchemaApplyResult holds the outcome of applying one schema change.
type SchemaApplyResult struct {
	ChangeSetID         int
	Success             bool
	TablesCreated       int
	FieldsAdded         int
	ConstraintsModified int
	IndexesCreated      int
	StatementsExecuted  int
	ForbiddenChanges    []string
	ErrorMessage        string
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
// Returns them ordered by created_time ascending (oldest first).
func GetApprovedSchemaChanges(client *runner.APIClient, maxChanges int) ([]SchemaChangeWork, error) {
	results, err := client.Search("_schema_change_set",
		[]runner.SearchFilter{
			{Field: "status", Operator: "eq", Value: "approved"},
		},
		[]runner.OrderSpec{{Field: "created_time", Direction: "asc"}},
		maxChanges, "")
	if err != nil {
		return nil, fmt.Errorf("searching approved schema change sets: %w", err)
	}

	changes := make([]SchemaChangeWork, 0, len(results.Rows))
	for _, row := range results.Rows {
		csID, _ := extractInt(row, "id")
		label, _ := row["label"].(string)
		description, _ := row["description"].(string)
		targetCommit, _ := row["target_commit_hash"].(string)
		createdTime, _ := row["created_time"].(string)

		changes = append(changes, SchemaChangeWork{
			ChangeSetID:  csID,
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
// This is the one runner that needs direct database access because it
// executes DDL — the runner-lib API client cannot run DDL statements.
// Returns the apply result with counts and any errors.
func ApplySchemaChange(db *pg.DB, repoPath string, change *SchemaChangeWork, verbose bool) (*SchemaApplyResult, error) {
	result := &SchemaApplyResult{ChangeSetID: change.ChangeSetID}

	// Step 1: load schema from repo.
	schema, err := loader.Load(repoPath)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("schema load failed: %v", err)
		return result, nil
	}

	// Step 2: read current database state.
	current, err := loader.ReadCurrentState(db)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("read current state failed: %v", err)
		return result, nil
	}

	// Step 3: diff desired vs current.
	diff, err := loader.Diff(schema, current)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("schema diff failed: %v", err)
		return result, nil
	}

	// Step 4: check evolution rules.
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

	// Step 5: generate DDL.
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

	// Step 6: apply DDL within transaction with advisory lock.
	applyResult, err := loader.Apply(db, statements, verbose)
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

	// Step 7: populate meta tables within a separate transaction.
	label := change.Label
	if label == "" {
		label = fmt.Sprintf("schema_change_set_%d", change.ChangeSetID)
	}
	err = pg.WithTransaction(db, func(tx *pg.Tx) error {
		return loader.PopulateMeta(tx, schema, evolution.Allowed, label)
	})
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
func FinalizeSchemaChange(client *runner.APIClient, changeSetID int, applyResult *SchemaApplyResult, runnerJobID int) error {
	status := "applied"
	if !applyResult.Success {
		status = "failed"
	}

	finalizeData := map[string]interface{}{
		"change_set_id":  changeSetID,
		"status":         status,
		"finalized_time": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if applyResult.Success {
		finalizeData["tables_created"] = applyResult.TablesCreated
		finalizeData["fields_added"] = applyResult.FieldsAdded
		finalizeData["constraints_modified"] = applyResult.ConstraintsModified
		finalizeData["statements_executed"] = applyResult.StatementsExecuted
	} else {
		finalizeData["error_message"] = applyResult.ErrorMessage
		if len(applyResult.ForbiddenChanges) > 0 {
			finalizeData["forbidden_changes"] = applyResult.ForbiddenChanges
		}
	}

	_, err := client.WriteObservation(&runner.WriteObservationParams{
		TargetTable:  "_schema_change_set",
		Key:          fmt.Sprintf("finalize:%d", changeSetID),
		Value:        status,
		DataJSON:     finalizeData,
		RunnerJobID:  runnerJobID,
		ObservedTime: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("finalizing schema change set %d as %s: %w", changeSetID, status, err)
	}

	return nil
}

// WriteSchemaEvidence creates an evidence_record documenting the schema change.
func WriteSchemaEvidence(client *runner.APIClient, applyResult *SchemaApplyResult, runnerJobID int) error {
	outcome := "pass"
	if !applyResult.Success {
		outcome = "fail"
	}

	description := fmt.Sprintf("Schema change set %d", applyResult.ChangeSetID)
	if applyResult.Success {
		description = fmt.Sprintf("Schema change set %d applied: %d tables created, %d fields added, %d constraints modified",
			applyResult.ChangeSetID, applyResult.TablesCreated, applyResult.FieldsAdded, applyResult.ConstraintsModified)
	} else {
		description = fmt.Sprintf("Schema change set %d failed: %s",
			applyResult.ChangeSetID, applyResult.ErrorMessage)
	}

	evidenceData := map[string]interface{}{
		"evidence_record_type": "schema_evolution",
		"description":          description,
		"outcome":              outcome,
		"change_set_id":        applyResult.ChangeSetID,
		"tables_created":       applyResult.TablesCreated,
		"fields_added":         applyResult.FieldsAdded,
		"constraints_modified": applyResult.ConstraintsModified,
		"indexes_created":      applyResult.IndexesCreated,
		"statements_executed":  applyResult.StatementsExecuted,
	}

	if len(applyResult.ForbiddenChanges) > 0 {
		evidenceData["forbidden_changes"] = applyResult.ForbiddenChanges
	}
	if applyResult.ErrorMessage != "" {
		evidenceData["error_message"] = applyResult.ErrorMessage
	}

	_, err := client.WriteObservation(&runner.WriteObservationParams{
		TargetTable:  "evidence_record",
		Key:          fmt.Sprintf("schema_evolution:%d", applyResult.ChangeSetID),
		Value:        outcome,
		DataJSON:     evidenceData,
		RunnerJobID:  runnerJobID,
		ObservedTime: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("writing schema evidence record for change set %d: %w", applyResult.ChangeSetID, err)
	}

	return nil
}

// --- internal helpers ---

// runGit executes a git command in the given directory and returns
// combined output.
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
