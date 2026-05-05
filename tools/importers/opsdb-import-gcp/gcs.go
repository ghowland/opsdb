
// === importers/opsdb-import-gcp/gcs.go ===
package gcp

// ImportGCS reads GCS buckets from GCP API.
func ImportGCS(config *GCPImportConfig) ([]Observation, error) {
	// TODO: for each project:
	//   list buckets
	//   for each bucket: get IAM, versioning, lifecycle, encryption
	//   MapGCSBucket for each
	return nil, nil
}
