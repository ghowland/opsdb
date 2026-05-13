package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/tools/opsdb_schema/loader"
)

// Version and BuildTime are set via ldflags at build time.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	repoPath := flag.String("repo", ".", "path to schema repository root")
	dsn := flag.String("dsn", "", "postgres connection string (or set OPSDB_DSN)")
	scope := flag.String("scope", "", "limit to entity or entity/field")
	verbose := flag.Bool("verbose", false, "verbose output")
	dryRun := flag.Bool("dry-run", false, "apply: rollback instead of commit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("opsdb_schema %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: opsdb_schema <command> [flags]\n")
		fmt.Fprintf(os.Stderr, "commands: validate, plan, apply, diff, export, init\n")
		os.Exit(2)
	}

	command := flag.Arg(0)

	// Resolve DSN: flag takes precedence, then environment variable.
	resolvedDSN := *dsn
	if resolvedDSN == "" {
		resolvedDSN = os.Getenv("OPSDB_DSN")
	}

	// Commands that require a database connection.
	dbRequired := map[string]bool{
		"plan":   true,
		"apply":  true,
		"diff":   true,
		"export": true,
	}

	if dbRequired[command] && resolvedDSN == "" {
		fmt.Fprintf(os.Stderr, "error: database connection required for %q (use --dsn or set OPSDB_DSN)\n", command)
		os.Exit(2)
	}

	switch command {
	case "validate":
		exitCode := cmdValidate(*repoPath, *scope)
		os.Exit(exitCode)

	case "plan":
		exitCode := cmdPlan(*repoPath, resolvedDSN, *scope, *verbose)
		os.Exit(exitCode)

	case "apply":
		exitCode := cmdApply(*repoPath, resolvedDSN, *scope, *verbose, *dryRun)
		os.Exit(exitCode)

	case "diff":
		exitCode := cmdDiff(*repoPath, resolvedDSN, *scope)
		os.Exit(exitCode)

	case "export":
		exitCode := cmdExport(resolvedDSN)
		os.Exit(exitCode)

	case "init":
		exitCode := cmdInit(*repoPath)
		os.Exit(exitCode)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		os.Exit(2)
	}
}

// cmdValidate parses and validates schema YAML files. No database required.
func cmdValidate(repoPath string, scope string) int {
	fmt.Println("validating schema...")

	schema, err := loader.Load(repoPath)
	if err != nil {
		if schema != nil && len(schema.Errors) > 0 {
			printSchemaErrors(schema.Errors)
			fmt.Fprintf(os.Stderr, "\n%d validation error(s) found\n", len(schema.Errors))
			return 1
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	if len(schema.Errors) > 0 {
		printSchemaErrors(schema.Errors)
		fmt.Fprintf(os.Stderr, "\n%d validation error(s) found\n", len(schema.Errors))
		return 1
	}

	entityCount := len(schema.Entities)
	fieldCount := 0
	for _, entity := range schema.Entities {
		fieldCount += len(entity.Fields)
	}
	relCount := len(schema.Relationships)

	fmt.Printf("schema valid: %d entities, %d fields, %d relationships\n", entityCount, fieldCount, relCount)
	_ = scope
	return 0
}

// cmdPlan loads schema, diffs against database, checks evolution rules,
// and prints the DDL that would be generated.
func cmdPlan(repoPath string, dsn string, scope string, verbose bool) int {
	schema, err := loader.Load(repoPath)
	if err != nil {
		if schema != nil && len(schema.Errors) > 0 {
			printSchemaErrors(schema.Errors)
		}
		fmt.Fprintf(os.Stderr, "error loading schema: %v\n", err)
		return 1
	}

	db, err := pg.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to database: %v\n", err)
		return 2
	}
	defer db.Close()

	current, err := loader.ReadCurrentState(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading current state: %v\n", err)
		return 2
	}

	diff, err := loader.Diff(schema, current)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error computing diff: %v\n", err)
		return 2
	}

	evolution, err := loader.CheckEvolution(diff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error checking evolution: %v\n", err)
		return 2
	}

	if len(evolution.Forbidden) > 0 {
		fmt.Println("\n=== Forbidden Changes ===")
		for _, fc := range evolution.Forbidden {
			fmt.Printf("  ! %s", fc.Entity)
			if fc.Field != "" {
				fmt.Printf(".%s", fc.Field)
			}
			fmt.Printf(": %s\n", fc.Rule)
			fmt.Printf("    reason: %s\n", fc.Reason)
			fmt.Printf("    alternative: %s\n", fc.Alternative)
		}
		fmt.Fprintf(os.Stderr, "\n%d forbidden change(s) — cannot apply\n", len(evolution.Forbidden))
		return 1
	}

	if len(evolution.Allowed) == 0 {
		fmt.Println("no changes needed — schema matches database")
		return 0
	}

	statements, err := loader.GenerateDDL(schema, evolution.Allowed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating DDL: %v\n", err)
		return 2
	}

	fmt.Println("\n=== Plan ===")
	for _, stmt := range statements {
		fmt.Printf("  [%s] %s\n", stmt.Entity, stmt.Description)
		if verbose {
			fmt.Printf("    %s\n", stmt.SQL)
		}
	}

	tables, columns, constraints, indexes := countChanges(evolution.Allowed)
	fmt.Printf("\nplan: %d tables to create, %d columns to add, %d constraints to modify, %d indexes to create\n",
		tables, columns, constraints, indexes)
	fmt.Printf("total: %d DDL statements\n", len(statements))

	_ = scope
	return 0
}

// cmdApply loads schema, diffs, generates DDL, and applies to the database.
func cmdApply(repoPath string, dsn string, scope string, verbose bool, dryRun bool) int {
	schema, err := loader.Load(repoPath)
	if err != nil {
		if schema != nil && len(schema.Errors) > 0 {
			printSchemaErrors(schema.Errors)
		}
		fmt.Fprintf(os.Stderr, "error loading schema: %v\n", err)
		return 1
	}

	db, err := pg.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to database: %v\n", err)
		return 2
	}
	defer db.Close()

	current, err := loader.ReadCurrentState(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading current state: %v\n", err)
		return 2
	}

	diff, err := loader.Diff(schema, current)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error computing diff: %v\n", err)
		return 2
	}

	evolution, err := loader.CheckEvolution(diff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error checking evolution: %v\n", err)
		return 2
	}

	if len(evolution.Forbidden) > 0 {
		fmt.Println("\n=== Forbidden Changes ===")
		for _, fc := range evolution.Forbidden {
			fmt.Printf("  ! %s", fc.Entity)
			if fc.Field != "" {
				fmt.Printf(".%s", fc.Field)
			}
			fmt.Printf(": %s\n", fc.Rule)
			fmt.Printf("    reason: %s\n", fc.Reason)
			fmt.Printf("    alternative: %s\n", fc.Alternative)
		}
		fmt.Fprintf(os.Stderr, "\n%d forbidden change(s) — refusing to apply\n", len(evolution.Forbidden))
		return 1
	}

	if len(evolution.Allowed) == 0 {
		fmt.Println("no changes needed — schema matches database")
		return 0
	}

	statements, err := loader.GenerateDDL(schema, evolution.Allowed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error generating DDL: %v\n", err)
		return 2
	}

	if dryRun {
		fmt.Println("=== Dry Run ===")
		result, err := loader.DryRun(db, statements)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dry run failed: %v\n", err)
			return 1
		}
		fmt.Printf("dry run successful: %d statements validated in %s\n",
			result.StatementsExecuted, result.Duration)
		fmt.Println("(no changes persisted — rolled back)")
		return 0
	}

	fmt.Println("=== Applying ===")
	result, err := loader.Apply(db, statements, verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "apply failed: %v\n", err)
		return 1
	}

	fmt.Printf("applied: %d statements in %s\n", result.StatementsExecuted, result.Duration)
	fmt.Printf("  entities created: %d\n", result.EntitiesCreated)
	fmt.Printf("  fields added: %d\n", result.FieldsAdded)
	fmt.Printf("  constraints modified: %d\n", result.ConstraintsModified)
	fmt.Printf("  indexes created: %d\n", result.IndexesCreated)

	// Populate _schema_* metadata tables.
	fmt.Println("\npopulating schema metadata...")
	err = pg.WithTransaction(db, func(tx *pg.Tx) error {
		return loader.PopulateMeta(tx, schema, evolution.Allowed, fmt.Sprintf("apply-%s", Version))
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: schema applied but metadata population failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "  tables are created but _schema_* tables may be incomplete\n")
		return 1
	}

	fmt.Println("schema metadata populated")
	_ = scope
	return 0
}

// cmdDiff shows differences between YAML and current database schema.
func cmdDiff(repoPath string, dsn string, scope string) int {
	schema, err := loader.Load(repoPath)
	if err != nil {
		if schema != nil && len(schema.Errors) > 0 {
			printSchemaErrors(schema.Errors)
		}
		fmt.Fprintf(os.Stderr, "error loading schema: %v\n", err)
		return 1
	}

	db, err := pg.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to database: %v\n", err)
		return 2
	}
	defer db.Close()

	current, err := loader.ReadCurrentState(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading current state: %v\n", err)
		return 2
	}

	diff, err := loader.Diff(schema, current)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error computing diff: %v\n", err)
		return 2
	}

	hasChanges := false

	if len(diff.NewEntities) > 0 {
		hasChanges = true
		for _, name := range diff.NewEntities {
			fmt.Printf("+ new entity: %s\n", name)
		}
	}

	if len(diff.NewFields) > 0 {
		hasChanges = true
		for _, item := range diff.NewFields {
			fmt.Printf("+ new field: %s.%s (%v)\n", item.Entity, item.Field, item.DesiredValue)
		}
	}

	if len(diff.ChangedConstraints) > 0 {
		hasChanges = true
		for _, item := range diff.ChangedConstraints {
			fmt.Printf("~ changed constraint: %s.%s: %v -> %v\n",
				item.Entity, item.Field, item.CurrentValue, item.DesiredValue)
		}
	}

	if len(diff.NewIndexes) > 0 {
		hasChanges = true
		for _, item := range diff.NewIndexes {
			fmt.Printf("+ new index: %s.%s\n", item.Entity, item.Description)
		}
	}

	if len(diff.RemovedFields) > 0 {
		hasChanges = true
		for _, item := range diff.RemovedFields {
			fmt.Printf("! forbidden removal: %s.%s\n", item.Entity, item.Field)
		}
	}

	if len(diff.RemovedEntities) > 0 {
		hasChanges = true
		for _, name := range diff.RemovedEntities {
			fmt.Printf("! forbidden removal: entity %s\n", name)
		}
	}

	if len(diff.TypeChanges) > 0 {
		hasChanges = true
		for _, item := range diff.TypeChanges {
			fmt.Printf("! forbidden type change: %s.%s: %v -> %v\n",
				item.Entity, item.Field, item.CurrentValue, item.DesiredValue)
		}
	}

	if !hasChanges {
		fmt.Println("no changes — schema matches database")
	}

	_ = scope
	return 0
}

// cmdExport dumps current database schema to stdout.
func cmdExport(dsn string) int {
	db, err := pg.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error connecting to database: %v\n", err)
		return 2
	}
	defer db.Close()

	current, err := loader.ReadCurrentState(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading current state: %v\n", err)
		return 2
	}

	fmt.Printf("# Exported schema — %d entities\n", len(current.Entities))
	fmt.Printf("# Schema version: %d\n\n", current.Version)

	for name, entity := range current.Entities {
		fmt.Printf("--- %s ---\n", name)
		for fieldName, field := range entity.Fields {
			nullable := ""
			if field.IsNullable {
				nullable = " nullable"
			}
			fmt.Printf("  %s: %s%s\n", fieldName, field.Type, nullable)
		}
		fmt.Println()
	}

	return 0
}

// cmdInit creates a new empty schema repository directory structure.
// The user populates it with the actual YAML files from the schema/
// directory or copies from an existing deployment.
func cmdInit(repoPath string) int {
	schemaDir := filepath.Join(repoPath, "schema")

	dirs := []string{
		filepath.Join(schemaDir, "meta"),
		filepath.Join(schemaDir, "conventions"),
		filepath.Join(schemaDir, "domains"),
		filepath.Join(schemaDir, "json_schemas"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating directory %s: %v\n", dir, err)
			return 2
		}
	}

	fmt.Printf("initialized schema repository at %s\n", schemaDir)
	fmt.Println("  schema/meta/          — place _schema_meta.yaml here")
	fmt.Println("  schema/conventions/   — place reserved.yaml here")
	fmt.Println("  schema/domains/       — place entity YAML files here")
	fmt.Println("  schema/json_schemas/  — place JSON payload schemas here")
	fmt.Println("  schema/directory.yaml — create with ordered imports list")
	return 0
}

// printSchemaErrors prints validation errors in a human-readable format.
func printSchemaErrors(errors []model.SchemaError) {
	for _, e := range errors {
		severity := strings.ToUpper(e.Severity)
		if e.Field != "" {
			fmt.Fprintf(os.Stderr, "  [%s] %s.%s: %s\n", severity, e.Entity, e.Field, e.Message)
		} else {
			fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", severity, e.Entity, e.Message)
		}
	}
}

// countChanges summarizes allowed changes by type.
func countChanges(allowed []loader.AllowedChange) (tables, columns, constraints, indexes int) {
	for _, c := range allowed {
		switch c.ChangeType {
		case "new_entity":
			tables++
		case "new_field":
			columns++
		case "widen_range", "add_enum", "widen_length":
			constraints++
		case "new_index":
			indexes++
		}
	}
	return
}
