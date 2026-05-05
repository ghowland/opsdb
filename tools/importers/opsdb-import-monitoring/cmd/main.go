
// === importers/opsdb-import-monitoring/cmd/main.go ===
package main

import "os"

// main is the CLI entrypoint for the monitoring importer.
func main() {
	// TODO: parse --dos flag
	// TODO: runner.Init("opsdb-import-monitoring")
	// TODO: determine backend from runner_spec config: prometheus or datadog
	// TODO: loop while runner.ShouldRun():
	//   GET: read runner_spec_version, authority
	//   ACT: call appropriate backend importer
	//   SET: write observations (prometheus_config, monitor, alert, observation_cache_metric)
	//         write runner_job
	//   runner.WaitForNextCycle()
	os.Exit(0)
}

