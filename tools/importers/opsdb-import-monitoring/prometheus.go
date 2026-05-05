
// === importers/opsdb-import-monitoring/prometheus.go ===
package monitoring

// MonitoringObservation is the observation structure for monitoring importers.
type MonitoringObservation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// ImportPrometheus reads configuration, scrape targets, alert rules, and metric metadata
// from a Prometheus server.
func ImportPrometheus(config *MonitoringImportConfig) ([]MonitoringObservation, error) {
	// TODO: query /api/v1/status/config for scrape configuration
	//   create prometheus_config observations
	//   extract scrape targets → prometheus_scrape_target observations
	// TODO: query /api/v1/rules for alert rules
	//   for each alerting rule:
	//     create monitor observation (type prometheus_query)
	//     create alert observation with severity from labels
	// TODO: query /api/v1/alerts for currently firing alerts
	//   for each firing alert: create alert_fire observation
	// TODO: query /api/v1/metadata for metric metadata
	//   for each metric: create observation_cache_metric observation
	//   (metric name, type, help text, labels — not values)
	return nil, nil
}

// MonitoringImportConfig holds monitoring importer configuration.
type MonitoringImportConfig struct {
	BackendType string // prometheus, datadog
	BatchSize   int
	MaxRetries  int
}

