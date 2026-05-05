//# tools/runners/reaper/cmd/main.go

go
package main

import (
	"os"
	// runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

// main is the CLI entrypoint for the reaper runner.
// Enforces retention policies by finding rows past their retention horizon
// and deleting (observation cache) or soft-deleting (entities) them.
//
// Without this runner, observation cache tables grow unboundedly,
// violating the "bound everything" principle.
func main() {
	// TODO: parse --dos flag for DOS config directory path
	// TODO: runner.Init("reaper") to load runner spec
	//   spec runner_data_json contains:
	//     batch_size: max rows to delete per table per cycle (default 10000)
	//     tables_per_cycle: max tables to process per cycle (default 10)
	//     max_cycle_duration_seconds: hard bound on cycle time (default 300)
	//     dry_run_log_count: in dry run, how many rows to log (default 100)
	// TODO: loop while runner.ShouldRun():
	//   jobID := runner.StartCycle(config)
	//
	//   GET:
	//     search retention_policy rows (all active policies)
	//     for each policy:
	//       determine target table from policy's target_entity_type
	//       determine retention horizon: now - retention_days
	//       count rows past horizon in target table
	//
	//   ACT (skip if dry run):
	//     for each policy with expired rows:
	//       if target is observation cache table:
	//         DELETE rows where _observed_time < retention_horizon
	//         (hard delete — cache data, not source of truth)
	//       if target is entity table with soft delete:
	//         UPDATE SET is_active=false where created_time < retention_horizon
	//         and is_active=true
	//       if target is append-only (runner_job, audit related):
	//         only if policy explicitly allows (audit retention is typically 7+ years)
	//       respect batch_size per table
	//
	//   SET:
	//     write runner_job with cycle summary:
	//       policies_evaluated, tables_processed,
	//       rows_deleted, rows_soft_deleted, tables_skipped (no expired rows)
	//     runner.FinishCycle(config, status, summary)
	//
	//   runner.WaitForNextCycle(config)
	os.Exit(0)
}


