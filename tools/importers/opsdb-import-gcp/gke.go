

// === importers/opsdb-import-gcp/gke.go ===
package gcp

// ImportGKE reads GKE clusters from GCP API.
// Produces both cloud_resource and k8s_cluster observations.
func ImportGKE(config *GCPImportConfig) ([]Observation, error) {
	// TODO: for each project:
	//   for each location (region or zone):
	//     list clusters
	//     MapGKECluster for each (produces dual observations)
	return nil, nil
}

