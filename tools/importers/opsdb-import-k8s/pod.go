
// === importers/opsdb-import-k8s/pod.go ===
package k8s

// ImportPods reads Kubernetes pods and maps to k8s_pod observations.
// High-frequency updates — pods change state often.
func ImportPods(config *K8sImportConfig) ([]Observation, error) {
	// TODO: for each namespace in scope:
	//   list pods
	// TODO: for each pod:
	//   extract name, namespace, UID, phase, node, IP, start time
	//   extract container statuses (running, waiting, terminated)
	//   extract restart count
	//   extract owner reference (links to workload)
	//   create k8s_pod observation
	return nil, nil
}

