
// === importers/opsdb-import-gcp/iam.go ===
package gcp

// ImportGCPIAM reads GCP IAM service accounts from GCP API.
func ImportGCPIAM(config *GCPImportConfig) ([]Observation, error) {
	// TODO: for each project:
	//   list service accounts
	//   for each: get keys count, bindings count
	//   MapGCPServiceAccount for each
	return nil, nil
}

