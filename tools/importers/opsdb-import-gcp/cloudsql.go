

// === importers/opsdb-import-gcp/cloudsql.go ===
package gcp

// ImportCloudSQL reads Cloud SQL instances from GCP API.
func ImportCloudSQL(config *GCPImportConfig) ([]Observation, error) {
	// TODO: for each project:
	//   paginate instances.list
	//   MapCloudSQLInstance for each
	return nil, nil
}

