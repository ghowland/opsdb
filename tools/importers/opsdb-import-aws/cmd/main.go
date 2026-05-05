// === importers/opsdb-import-aws/cmd/main.go ===
package main

import (
	"os"
	// runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

// main is the CLI entrypoint for the AWS importer.
// Reads --dos flag for DOS config path, initializes runner via opsdb-runner-lib,
// starts the get/act/set cycle loop.
func main() {
	// TODO: parse --dos flag for DOS config directory path
	// TODO: runner.Init("opsdb-import-aws") to load runner spec from OpsDB
	// TODO: loop while runner.ShouldRun():
	//   GET: read runner_spec_version for config (regions, resource types, batch size)
	//         read authority row for AWS credentials source
	//         read report key declarations
	//   ACT: call each resource importer (ec2, rds, s3, iam, vpc, route53)
	//         based on configured resource_types
	//         each returns []Observation
	//   SET: write observations via runner.WriteObservation()
	//         write runner_job with cycle summary
	//   runner.WaitForNextCycle()
	os.Exit(0)
}

