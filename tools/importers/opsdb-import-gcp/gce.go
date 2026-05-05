
// === importers/opsdb-import-gcp/gce.go ===
package gcp

// ImportGCE reads GCE instances from GCP API for configured projects and zones.
func ImportGCE(config *GCPImportConfig) ([]Observation, error) {
	// TODO: for each project in config:
	//   for each zone (or aggregated list):
	//     paginate instances.list
	//     MapGCEInstance for each
	// TODO: handle pagination, rate limits
	return nil, nil
}

// GCPImportConfig holds GCP importer cycle configuration.
type GCPImportConfig struct {
	Projects   []string
	Regions    []string
	BatchSize  int
	MaxRetries int
}
