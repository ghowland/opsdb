
// === importers/opsdb-import-k8s/cmd/main.go ===
package main

import "os"

// main is the CLI entrypoint for the Kubernetes importer.
func main() {
	// TODO: parse --dos flag
	// TODO: runner.Init("opsdb-import-k8s")
	// TODO: determine mode: poll or watch (from runner_spec config use_watch_api)
	// TODO: if watch mode:
	//   start watcher goroutines per resource type
	//   block on signal or error
	// TODO: if poll mode:
	//   loop while runner.ShouldRun():
	//     GET: read runner_spec_version, authority (kubeconfig source)
	//     ACT: call each resource importer (cluster, node, namespace, workload, pod, helm, configmap, secret, service)
	//     SET: write observations, write runner_job
	//     runner.WaitForNextCycle()
	os.Exit(0)
}

