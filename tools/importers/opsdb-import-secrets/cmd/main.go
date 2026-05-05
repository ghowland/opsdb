
// === importers/opsdb-import-secrets/cmd/main.go ===
package main

import "os"

// main is the CLI entrypoint for the secret metadata importer.
// Imports metadata only — NEVER reads or records secret values.
func main() {
	// TODO: parse --dos flag
	// TODO: runner.Init("opsdb-import-secrets")
	// TODO: determine backend from runner_spec config: vault or aws_sm
	// TODO: loop while runner.ShouldRun():
	//   GET: read runner_spec_version, authority
	//   ACT: call appropriate backend importer
	//   SET: write observations (authority_pointer with type=secret, rotation metadata)
	//         write runner_job
	//   runner.WaitForNextCycle()
	os.Exit(0)
}

