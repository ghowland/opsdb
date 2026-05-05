
// === importers/opsdb-import-oncall/cmd/main.go ===
package main

import "os"

// main is the CLI entrypoint for the on-call importer.
func main() {
	// TODO: parse --dos flag
	// TODO: runner.Init("opsdb-import-oncall")
	// TODO: determine backend from runner_spec config: pagerduty or opsgenie
	// TODO: loop while runner.ShouldRun():
	//   GET: read runner_spec_version, authority
	//   ACT: call appropriate backend importer
	//   SET: write observations (on_call_schedule, on_call_assignment, escalation_path, escalation_step)
	//         write runner_job
	//   runner.WaitForNextCycle()
	os.Exit(0)
}
