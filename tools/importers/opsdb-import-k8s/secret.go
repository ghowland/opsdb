
// === importers/opsdb-import-k8s/secret.go ===
package k8s

// ImportSecrets reads Kubernetes Secret metadata and maps to k8s_secret_reference observations.
// NEVER reads secret values. Only metadata: name, namespace, type, creation time.
func ImportSecrets(config *K8sImportConfig) ([]Observation, error) {
	// TODO: for each namespace in scope:
	//   list Secrets
	// TODO: for each secret:
	//   extract name, namespace, type (opaque, tls, dockerconfigjson, etc.)
	//   extract creation timestamp
	//   extract data key names ONLY (not values)
	//   DO NOT read .data or .stringData fields
	//   create k8s_secret_reference observation
	return nil, nil
}

