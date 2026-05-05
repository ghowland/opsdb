//# tools/opsdb-schema/cmd/main.go

go
package main

import (
	"flag"
	"fmt"
	"os"
)

// main is the CLI entrypoint for the opsdb-schema binary.
// Parses command-line flags and dispatches to the appropriate loader function.
//
// Commands:
//   validate  - parse and validate schema YAML files, report errors
//   plan      - show what DDL would be generated (diff against database)
//   apply     - execute DDL against database within advisory-locked transaction
//   diff      - show differences between YAML and current database schema
//   export    - dump current database schema as YAML (reverse engineering)
//   init      - create a new schema repository with directory.yaml and meta-schema
func main() {
	// TODO: define subcommand flags
	//   --repo     string  path to schema repository root (default ".")
	//   --dsn      string  postgres connection string (or OPSDB_DSN env var)
	//   --scope    string  limit to specific entity or entity/field
	//   --verbose  bool    print DDL statements as they execute
	//   --dry-run  bool    for apply: execute in transaction then rollback
	//   --version  bool    print version and exit

	repoPath := flag.String("repo", ".", "path to schema repository root")
	dsn := flag.String("dsn", "", "postgres connection string (or set OPSDB_DSN)")
	scope := flag.String("scope", "", "limit to entity or entity/field")
	verbose := flag.Bool("verbose", false, "verbose output")
	dryRun := flag.Bool("dry-run", false, "apply: rollback instead of commit")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: opsdb-schema <command> [flags]\n")
		fmt.Fprintf(os.Stderr, "commands: validate, plan, apply, diff, export, init\n")
		os.Exit(2)
	}

	command := flag.Arg(0)

	// TODO: resolve DSN: if --dsn empty, read OPSDB_DSN env var
	// TODO: for commands that need database (plan, apply, diff, export):
	//   if DSN still empty: error "database connection required for {command}"

	switch command {
	case "validate":
		// TODO: loader.Load(repoPath) to parse and validate all YAML files
		// TODO: if errors: print each error with file, line, field, message
		// TODO: exit 1 if errors, 0 if clean
		_ = repoPath

	case "plan":
		// TODO: loader.Load(repoPath) to get desired schema
		// TODO: pg.Connect(dsn) to open database
		// TODO: differ.ReadCurrentState(db) to get current schema
		// TODO: differ.Diff(desired, current) to get changes
		// TODO: evolution.CheckEvolution(diff) to classify changes
		// TODO: if forbidden changes: print each with error and alternative, exit 1
		// TODO: generator.GenerateDDL(schema, allowedChanges) to get DDL
		// TODO: print each DDL statement with entity and description
		// TODO: print summary: N tables to create, N columns to add, N constraints to modify
		_ = dsn

	case "apply":
		// TODO: same as plan through DDL generation
		// TODO: if dryRun: applier.DryRun(db, statements)
		// TODO: else: applier.Apply(db, statements, verbose)
		// TODO: if apply succeeds: meta.PopulateMeta(db, schema, changes)
		// TODO: print result summary
		// TODO: exit 0 on success, 1 on error
		_ = dryRun
		_ = verbose

	case "diff":
		// TODO: loader.Load(repoPath) to get desired
		// TODO: pg.Connect(dsn)
		// TODO: differ.Diff(desired, current)
		// TODO: print human-readable diff:
		//   + new entity: {name}
		//   + new field: {entity}.{field} ({type})
		//   ~ changed constraint: {entity}.{field}.{constraint}: {old} -> {new}
		//   ! forbidden: {entity}.{field}: {reason}

	case "export":
		// TODO: pg.Connect(dsn)
		// TODO: differ.ReadCurrentState(db)
		// TODO: convert SchemaState to YAML format
		// TODO: write to stdout or --output path

	case "init":
		// TODO: create directory structure:
		//   {repo}/schema/meta/_schema_meta.yaml (copy template)
		//   {repo}/schema/conventions/reserved.yaml (copy template)
		//   {repo}/schema/directory.yaml (empty imports list)
		//   {repo}/schema/domains/ (empty directory)
		// TODO: print "initialized schema repository at {repo}"

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		os.Exit(2)
	}

	_ = scope
}


