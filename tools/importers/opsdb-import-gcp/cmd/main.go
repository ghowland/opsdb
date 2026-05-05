
// === importers/opsdb-import-gcp/cmd/main.go ===
package main

import "os"

// main is the CLI entrypoint for the GCP importer.
func main() {
	// TODO: parse --dos flag
	// TODO: runner.Init("opsdb-import-gcp")
	// TODO: loop while runner.ShouldRun():
	//   GET: read runner_spec_version, authority, report keys
	//   ACT: call each resource importer (gce, cloudsql, gcs, gke, iam)
	//   SET: write observations, write runner_job
	//   runner.WaitForNextCycle()
	os.Exit(0)
}

