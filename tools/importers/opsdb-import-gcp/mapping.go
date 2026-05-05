
// === importers/opsdb-import-gcp/mapping.go ===
package gcp

// Observation is the same structure as AWS mapping; could extract to shared package.
// Keeping per-importer for now to avoid premature coupling.
type Observation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// MapGCEInstance maps a GCP Compute Engine instance to observations.
func MapGCEInstance(instance interface{}) []Observation {
	// TODO: extract name, zone, machine type, network, IPs, status, image
	// TODO: flat per-row metadata in DataJSON per DSNC
	return nil
}

// MapCloudSQLInstance maps a GCP Cloud SQL instance to observations.
func MapCloudSQLInstance(instance interface{}) []Observation {
	// TODO: extract database version, tier, disk size, availability, IP, connection name
	return nil
}

// MapGCSBucket maps a GCP Cloud Storage bucket to observations.
func MapGCSBucket(bucket interface{}) []Observation {
	// TODO: extract storage class, versioning, encryption, uniform access, lifecycle
	return nil
}

// MapGKECluster maps a GKE cluster to both cloud_resource and k8s_cluster observations.
func MapGKECluster(cluster interface{}) []Observation {
	// TODO: cloud_resource observation with type gce_instance (cluster is also cloud resource)
	// TODO: k8s_cluster observation with distribution=gke, version, endpoint
	// TODO: return both
	return nil
}

// MapGCPServiceAccount maps a GCP IAM service account to observations.
func MapGCPServiceAccount(sa interface{}) []Observation {
	// TODO: extract email, display name, disabled status, key count
	return nil
}

