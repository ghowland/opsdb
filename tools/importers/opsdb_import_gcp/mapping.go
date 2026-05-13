package gcp

import "fmt"

// GCPImportConfig holds GCP importer cycle configuration.
// Shared across all resource type importers in this package.
type GCPImportConfig struct {
	Projects      []string
	ResourceTypes []string
	BatchSize     int
	MaxRetries    int
}

// Observation is the GCP importer observation structure.
// Each resource importer returns a slice of these, which cmd/main.go
// writes to OpsDB via the runner library.
type Observation struct {
	EntityType  string
	EntityID    string
	StateKey    string
	Value       string
	DataJSON    map[string]interface{}
	AuthorityID int
}

// buildEntityID constructs a globally unique entity ID for a GCP resource.
// Format: gcp:{project}:{resource_type}:{identifiers...}
func buildEntityID(project string, resourceType string, identifiers ...string) string {
	id := fmt.Sprintf("gcp:%s:%s", project, resourceType)
	for _, part := range identifiers {
		id += ":" + part
	}
	return id
}

// truncateString truncates a string to maxLen characters for safe storage
// in bounded varchar fields.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}