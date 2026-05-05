
// === importers/opsdb-import-k8s/node.go ===
package k8s

// ImportNodes reads Kubernetes nodes and maps to k8s_cluster_node observations.
func ImportNodes(config *K8sImportConfig) ([]Observation, error) {
	// TODO: list nodes
	// TODO: for each node:
	//   extract name, role (from labels), schedulable, capacity, allocatable
	//   extract conditions, addresses, instance ID (from provider ID for cloud linkage)
	//   create k8s_cluster_node observation
	return nil, nil
}
