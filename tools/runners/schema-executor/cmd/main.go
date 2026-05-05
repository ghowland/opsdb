//# tools/runners/schema-executor/cmd/main.go

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS config directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "error: --dos flag is required\n")
		os.Exit(2)
	}

	config, err := runner.Init("schema-executor", *dosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize schema-executor: %v\n", err)
		os.Exit(2)
	}
	defer runner.Shutdown(config)

	logger := runner.NewLogger(config)
	logger.Info("schema-executor starting",
		runner.Field{Key: "dos_path", Value: *dosPath},
		runner.Field{Key: "schema_repo_path", Value: config.SpecData.StringOrDefault("schema_repo_path", "")},
		runner.Field{Key: "max_changes_per_cycle", Value: config.SpecData.IntOrDefault("max_changes_per_cycle", 1)},
		runner.Field{Key: "require_git_clean", Value: config.SpecData.BoolOrDefault("require_git_clean", true)},
		runner.Field{Key: "auto_pull", Value: config.SpecData.BoolOrDefault("auto_pull", true)},
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
		} else if len(summary.Errors) > 0 {
			cycleLogger.Warn("cycle completed with errors",
				runner.Field{Key: "changes_processed", Value: summary.ChangesProcessed},
				runner.Field{Key: "changes_applied", Value: summary.ChangesApplied},
				runner.Field{Key: "changes_failed", Value: summary.ChangesFailed},
				runner.Field{Key: "error_count", Value: len(summary.Errors)},
			)
			runner.FinishCycle(config, jobID, "partial", summary)
		} else {
			cycleLogger.Info("cycle complete",
				runner.Field{Key: "changes_processed", Value: summary.ChangesProcessed},
				runner.Field{Key: "changes_applied", Value: summary.ChangesApplied},
				runner.Field{Key: "tables_created", Value: summary.TablesCreated},
				runner.Field{Key: "fields_added", Value: summary.FieldsAdded},
				runner.Field{Key: "constraints_modified", Value: summary.ConstraintsModified},
			)
			runner.FinishCycle(config, jobID, "succeeded", summary)
		}

		runner.WaitForNextCycle(config)
	}

	logger.Info("schema-executor stopped")
	os.Exit(0)
}

func runCycle(config *runner.RunnerConfig, logger *runner.Logger) (*SchemaExecutorSummary, error) {
	summary := &SchemaExecutorSummary{}
	client := config.APIClient
	dryRun := runner.IsDryRun(config)

	schemaRepoPath := config.SpecData.StringOrDefault("schema_repo_path", "")
	if schemaRepoPath == "" {
		return summary, fmt.Errorf("schema_repo_path not configured in runner spec")
	}
	maxChanges := config.SpecData.IntOrDefault("max_changes_per_cycle", 1)
	requireGitClean := config.SpecData.BoolOrDefault("require_git_clean", true)
	autoPull := config.SpecData.BoolOrDefault("auto_pull", true)

	// GET: search for approved schema change sets, oldest first
	results, err := client.Search("_schema_change_set", map[string]interface{}{
		"status": "approved",
	}, []string{"created_time asc"}, maxChanges)
	if err != nil {
		return summary, fmt.Errorf("failed to search approved schema change sets: %w", err)
	}

	if len(results.Rows) == 0 {
		logger.Info("no approved schema change sets to process")
		return summary, nil
	}

	logger.Info("found approved schema change sets",
		runner.Field{Key: "count", Value: len(results.Rows)},
		runner.Field{Key: "max_per_cycle", Value: maxChanges},
	)

	if dryRun {
		for _, row := range results.Rows {
			changeSetID, _ := row.IntField("id")
			description, _ := row.StringField("description")
			commitHash, _ := row.StringField("target_commit_hash")
			createdTime, _ := row.StringField("created_time")
			logger.Info("dry run: would apply schema change set",
				runner.Field{Key: "change_set_id", Value: changeSetID},
				runner.Field{Key: "description", Value: description},
				runner.Field{Key: "commit_hash", Value: commitHash},
				runner.Field{Key: "created_time", Value: createdTime},
			)
		}
		summary.ChangesProcessed = len(results.Rows)
		return summary, nil
	}

	// ACT: apply each approved schema change set
	for _, row := range results.Rows {
		changeSetID, _ := row.IntField("id")
		description, _ := row.StringField("description")
		commitHash, _ := row.StringField("target_commit_hash")
		submittedBy, _ := row.StringField("submitted_by")

		changeLogger := logger.With(
			runner.Field{Key: "schema_change_set_id", Value: changeSetID},
			runner.Field{Key: "description", Value: description},
		)

		summary.ChangesProcessed++

		changeLogger.Info("processing schema change set",
			runner.Field{Key: "commit_hash", Value: commitHash},
			runner.Field{Key: "submitted_by", Value: submittedBy},
		)

		result, err := applySchemaChange(client, changeLogger, schemaRepoPath, changeSetID, commitHash, requireGitClean, autoPull)
		if err != nil {
			summary.ChangesFailed++
			summary.Errors = append(summary.Errors, fmt.Sprintf("change_set %d: %v", changeSetID, err))

			changeLogger.Error("schema change set failed",
				runner.Field{Key: "error", Value: err.Error()},
			)

			// mark the change set as failed in OpsDB
			markErr := client.WriteObservation("_schema_change_set", changeSetID, "status", "failed")
			if markErr != nil {
				changeLogger.Error("failed to mark schema change set as failed",
					runner.Field{Key: "error", Value: markErr.Error()},
				)
			}
			markErr = client.WriteObservation("_schema_change_set", changeSetID, "error_detail", err.Error())
			if markErr != nil {
				changeLogger.Error("failed to write error detail",
					runner.Field{Key: "error", Value: markErr.Error()},
				)
			}
			continue
		}

		summary.ChangesApplied++
		summary.TablesCreated += result.TablesCreated
		summary.FieldsAdded += result.FieldsAdded
		summary.ConstraintsModified += result.ConstraintsModified

		// mark the change set as applied
		markErr := client.WriteObservation("_schema_change_set", changeSetID, "status", "applied")
		if markErr != nil {
			changeLogger.Error("failed to mark schema change set as applied",
				runner.Field{Key: "error", Value: markErr.Error()},
			)
			summary.Errors = append(summary.Errors, fmt.Sprintf("change_set %d: applied but failed to update status: %v", changeSetID, markErr))
			continue
		}

		// write evidence record for schema evolution
		evidence := map[string]interface{}{
			"evidence_record_type": "schema_evolution",
			"description":         fmt.Sprintf("Applied schema change set %d: %s", changeSetID, description),
			"outcome":             "pass",
			"evidence_record_data_json": map[string]interface{}{
				"schema_change_set_id": changeSetID,
				"commit_hash":         commitHash,
				"tables_created":      result.TablesCreated,
				"fields_added":        result.FieldsAdded,
				"constraints_modified": result.ConstraintsModified,
				"ddl_statement_count": result.DDLStatements,
			},
		}
		_, evidenceErr := client.CreateEntity("evidence_record", evidence)
		if evidenceErr != nil {
			changeLogger.Warn("failed to write evidence record for schema change",
				runner.Field{Key: "error", Value: evidenceErr.Error()},
			)
		}

		changeLogger.Info("schema change set applied",
			runner.Field{Key: "tables_created", Value: result.TablesCreated},
			runner.Field{Key: "fields_added", Value: result.FieldsAdded},
			runner.Field{Key: "constraints_modified", Value: result.ConstraintsModified},
			runner.Field{Key: "ddl_statements", Value: result.DDLStatements},
		)
	}

	return summary, nil
}

// applySchemaChange runs the full schema loader pipeline for one approved
// change set: git operations, load, diff, evolution check, DDL generation,
// apply, and meta population.
func applySchemaChange(client *runner.APIClient, logger *runner.Logger, repoPath string, changeSetID int, commitHash string, requireGitClean bool, autoPull bool) (*SchemaApplyResult, error) {
	result := &SchemaApplyResult{}

	// git safety checks
	if requireGitClean {
		clean, err := gitIsClean(repoPath)
		if err != nil {
			return result, fmt.Errorf("failed to check git status: %w", err)
		}
		if !clean {
			return result, fmt.Errorf("schema repo has uncommitted changes at %s; refusing to apply (require_git_clean=true)", repoPath)
		}
	}

	if autoPull {
		logger.Info("pulling latest schema repo")
		err := gitPull(repoPath)
		if err != nil {
			return result, fmt.Errorf("git pull failed: %w", err)
		}
	}

	// checkout specific commit if specified
	if commitHash != "" {
		logger.Info("checking out target commit", runner.Field{Key: "commit", Value: commitHash})
		err := gitCheckout(repoPath, commitHash)
		if err != nil {
			return result, fmt.Errorf("git checkout %s failed: %w", commitHash, err)
		}
	}

	// run the schema loader pipeline
	logger.Info("loading schema from repository", runner.Field{Key: "repo_path", Value: repoPath})

	desiredSchema, err := schemaLoad(repoPath)
	if err != nil {
		return result, fmt.Errorf("schema load failed: %w", err)
	}

	logger.Info("schema loaded",
		runner.Field{Key: "entity_count", Value: len(desiredSchema.Entities)},
	)

	currentState, err := schemaReadCurrentState(client)
	if err != nil {
		return result, fmt.Errorf("failed to read current schema state: %w", err)
	}

	diff, err := schemaDiff(desiredSchema, currentState)
	if err != nil {
		return result, fmt.Errorf("schema diff failed: %w", err)
	}

	if diff.IsEmpty() {
		logger.Info("schema is already up to date, no changes needed")
		return result, nil
	}

	logger.Info("schema diff computed",
		runner.Field{Key: "new_entities", Value: diff.NewEntityCount},
		runner.Field{Key: "new_fields", Value: diff.NewFieldCount},
		runner.Field{Key: "changed_constraints", Value: diff.ChangedConstraintCount},
		runner.Field{Key: "potentially_forbidden", Value: diff.ForbiddenCount},
	)

	evolutionResult, err := schemaCheckEvolution(diff)
	if err != nil {
		return result, fmt.Errorf("evolution check failed: %w", err)
	}

	if len(evolutionResult.Forbidden) > 0 {
		for _, forbidden := range evolutionResult.Forbidden {
			logger.Error("forbidden schema change detected",
				runner.Field{Key: "entity", Value: forbidden.Entity},
				runner.Field{Key: "field", Value: forbidden.Field},
				runner.Field{Key: "rule", Value: forbidden.Rule},
				runner.Field{Key: "alternative", Value: forbidden.Alternative},
			)
		}
		return result, fmt.Errorf("schema change set contains %d forbidden changes; see logs for details", len(evolutionResult.Forbidden))
	}

	ddlStatements, err := schemaGenerateDDL(desiredSchema, evolutionResult.Allowed)
	if err != nil {
		return result, fmt.Errorf("DDL generation failed: %w", err)
	}

	result.DDLStatements = len(ddlStatements)

	logger.Info("applying DDL",
		runner.Field{Key: "statement_count", Value: len(ddlStatements)},
	)

	applyResult, err := schemaApply(client, ddlStatements)
	if err != nil {
		return result, fmt.Errorf("DDL apply failed: %w", err)
	}

	result.TablesCreated = applyResult.EntitiesCreated
	result.FieldsAdded = applyResult.FieldsAdded
	result.ConstraintsModified = applyResult.ConstraintsModified

	// populate _schema_* meta tables within the same logical operation
	label := fmt.Sprintf("schema_change_set_%d", changeSetID)
	err = schemaPopulateMeta(client, desiredSchema, evolutionResult.Allowed, label)
	if err != nil {
		return result, fmt.Errorf("meta population failed after successful DDL apply: %w", err)
	}

	logger.Info("schema meta tables updated", runner.Field{Key: "version_label", Value: label})

	return result, nil
}

// SchemaExecutorSummary holds the results of one schema executor cycle.
type SchemaExecutorSummary struct {
	ChangesProcessed     int
	ChangesApplied       int
	ChangesFailed        int
	TablesCreated        int
	FieldsAdded          int
	ConstraintsModified  int
	Errors               []string
}

// SchemaApplyResult holds the outcome of applying one schema change set.
type SchemaApplyResult struct {
	TablesCreated       int
	FieldsAdded         int
	ConstraintsModified int
	DDLStatements       int
}

// The functions below delegate to the schema loader and git packages.
// They are defined here as thin wrappers so the runner does not import
// the loader package directly — the runner calls through these boundaries
// which can be replaced for testing.

// gitIsClean checks whether the schema repo has uncommitted changes.
func gitIsClean(repoPath string) (bool, error) {
	// delegates to: opsdb.git or os/exec "git status --porcelain"
	output, err := runner.ExecCommand(repoPath, "git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(output) == 0, nil
}

// gitPull runs git pull in the schema repo.
func gitPull(repoPath string) error {
	_, err := runner.ExecCommand(repoPath, "git", "pull", "--ff-only")
	return err
}

// gitCheckout checks out a specific commit in the schema repo.
func gitCheckout(repoPath string, commitHash string) error {
	_, err := runner.ExecCommand(repoPath, "git", "checkout", commitHash)
	return err
}

// schemaLoad delegates to the loader package to parse, validate, resolve,
// and inject the schema from YAML files.
func schemaLoad(repoPath string) (*SchemaResult, error) {
	output, err := runner.ExecCommand(repoPath, "opsdb-schema", "validate", "--repo", repoPath, "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("schema validation failed: %w (output: %s)", err, string(output))
	}
	schema, err := parseSchemaResult(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema validation output: %w", err)
	}
	return schema, nil
}

// schemaReadCurrentState reads the current database schema state via the
// opsdb-schema tool.
func schemaReadCurrentState(client *runner.APIClient) (*SchemaState, error) {
	// reads _schema_entity_type, _schema_field, _schema_relationship
	// via the API client to build current state
	entityTypes, err := client.Search("_schema_entity_type", map[string]interface{}{
		"is_active": true,
	}, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema entity types: %w", err)
	}

	fields, err := client.Search("_schema_field", map[string]interface{}{
		"is_active": true,
	}, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema fields: %w", err)
	}

	relationships, err := client.Search("_schema_relationship", nil, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema relationships: %w", err)
	}

	return &SchemaState{
		EntityTypes:   entityTypes.Rows,
		Fields:        fields.Rows,
		Relationships: relationships.Rows,
	}, nil
}

// schemaDiff compares desired schema against current state.
func schemaDiff(desired *SchemaResult, current *SchemaState) (*SchemaDiffResult, error) {
	output, err := runner.ExecCommand(".", "opsdb-schema", "diff",
		"--repo", desired.RepoPath,
		"--format", "json",
	)
	if err != nil {
		return nil, fmt.Errorf("schema diff command failed: %w (output: %s)", err, string(output))
	}
	diff, err := parseSchemaDiffResult(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff output: %w", err)
	}
	return diff, nil
}

// schemaCheckEvolution validates diff changes against evolution rules.
func schemaCheckEvolution(diff *SchemaDiffResult) (*EvolutionResult, error) {
	allowed := make([]AllowedChange, 0, diff.NewEntityCount+diff.NewFieldCount+diff.ChangedConstraintCount)
	var forbidden []ForbiddenChange

	for _, change := range diff.Changes {
		if change.IsForbidden {
			forbidden = append(forbidden, ForbiddenChange{
				Entity:      change.Entity,
				Field:       change.Field,
				Rule:        change.ForbiddenRule,
				Alternative: change.ForbiddenAlternative,
			})
		} else {
			allowed = append(allowed, AllowedChange{
				Entity:     change.Entity,
				Field:      change.Field,
				ChangeType: change.ChangeType,
			})
		}
	}

	return &EvolutionResult{
		Allowed:   allowed,
		Forbidden: forbidden,
	}, nil
}

// schemaGenerateDDL generates Postgres DDL from allowed changes.
func schemaGenerateDDL(schema *SchemaResult, allowed []AllowedChange) ([]DDLStatement, error) {
	output, err := runner.ExecCommand(".", "opsdb-schema", "plan",
		"--repo", schema.RepoPath,
		"--format", "json",
	)
	if err != nil {
		return nil, fmt.Errorf("schema plan command failed: %w (output: %s)", err, string(output))
	}
	statements, err := parseDDLStatements(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan output: %w", err)
	}
	return statements, nil
}

// schemaApply executes DDL statements against the database via the
// opsdb-schema apply command.
func schemaApply(client *runner.APIClient, statements []DDLStatement) (*ApplyResult, error) {
	// the actual apply uses opsdb-schema apply which handles advisory
	// locks, transactions, and atomic commit internally
	output, err := runner.ExecCommand(".", "opsdb-schema", "apply",
		"--format", "json",
	)
	if err != nil {
		return nil, fmt.Errorf("schema apply command failed: %w (output: %s)", err, string(output))
	}
	result, err := parseApplyResult(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse apply output: %w", err)
	}
	return result, nil
}

// schemaPopulateMeta updates _schema_* tables after DDL apply.
func schemaPopulateMeta(client *runner.APIClient, schema *SchemaResult, allowed []AllowedChange, label string) error {
	// meta population is handled as part of opsdb-schema apply;
	// this records the label for the version row via API
	return client.WriteObservation("_schema_version", 0, "label", label)
}

// Supporting types for schema pipeline results. These mirror the
// structures returned by the opsdb-schema tool in JSON format.

type SchemaResult struct {
	RepoPath string
	Entities map[string]interface{}
}

type SchemaState struct {
	EntityTypes   []runner.Row
	Fields        []runner.Row
	Relationships []runner.Row
}

type SchemaDiffResult struct {
	NewEntityCount       int
	NewFieldCount        int
	ChangedConstraintCount int
	ForbiddenCount       int
	Changes              []DiffChange
}

func (d *SchemaDiffResult) IsEmpty() bool {
	return d.NewEntityCount == 0 && d.NewFieldCount == 0 &&
		d.ChangedConstraintCount == 0 && d.ForbiddenCount == 0
}

type DiffChange struct {
	Entity              string
	Field               string
	ChangeType          string
	IsForbidden         bool
	ForbiddenRule       string
	ForbiddenAlternative string
}

type AllowedChange struct {
	Entity     string
	Field      string
	ChangeType string
}

type ForbiddenChange struct {
	Entity      string
	Field       string
	Rule        string
	Alternative string
}

type DDLStatement struct {
	SQL         string
	Entity      string
	Description string
}

type ApplyResult struct {
	EntitiesCreated    int
	FieldsAdded        int
	ConstraintsModified int
}

// Parse helpers for opsdb-schema JSON output. These will deserialize
// the structured output from the opsdb-schema tool commands.

func parseSchemaResult(output []byte) (*SchemaResult, error) {
	// TODO: unmarshal JSON output from opsdb-schema validate --format json
	return &SchemaResult{
		Entities: make(map[string]interface{}),
	}, nil
}

func parseSchemaDiffResult(output []byte) (*SchemaDiffResult, error) {
	// TODO: unmarshal JSON output from opsdb-schema diff --format json
	return &SchemaDiffResult{}, nil
}

func parseDDLStatements(output []byte) ([]DDLStatement, error) {
	// TODO: unmarshal JSON output from opsdb-schema plan --format json
	return nil, nil
}

func parseApplyResult(output []byte) (*ApplyResult, error) {
	// TODO: unmarshal JSON output from opsdb-schema apply --format json
	return &ApplyResult{}, nil
}
