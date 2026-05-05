
// === importers/opsdb-import-k8s/service.go ===
package k8s

// ImportServices reads Kubernetes Service objects and maps to k8s_service observations.
func ImportServices(config *K8sImportConfig) ([]Observation, error) {
	// TODO: for each namespace in scope:
	//   list Services
	// TODO: for each service:
	//   extract name, namespace, type (ClusterIP, NodePort, LoadBalancer, ExternalName, Headless)
	//   extract cluster IP, external IPs, ports, selector
	//   create k8s_service observation
	return nil, nil
}


// === importers/opsdb-import-identity/cmd/main.go ===
package main

import "os"

// main is the CLI entrypoint for the identity provider importer.
func main() {
	// TODO: parse --dos flag
	// TODO: runner.Init("opsdb-import-identity")
	// TODO: determine backend from runner_spec config: okta, azuread, or ldap
	// TODO: loop while runner.ShouldRun():
	//   GET: read runner_spec_version, authority (IdP connection details)
	//   ACT: call appropriate backend importer (okta, azuread, ldap)
	//   SET: write observations (ops_user, ops_group, ops_group_member)
	//         write runner_job
	//   runner.WaitForNextCycle()
	os.Exit(0)
}

