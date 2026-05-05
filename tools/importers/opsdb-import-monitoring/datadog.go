
// === importers/opsdb-import-monitoring/datadog.go ===
package monitoring

// ImportDatadog reads monitors, alert definitions, and alert state from Datadog API.
func ImportDatadog(config *MonitoringImportConfig) ([]MonitoringObservation, error) {
	// TODO: paginate list monitors via Datadog API
	// TODO: for each monitor:
	//   extract name, type, query, message, tags
	//   map to monitor observation (type depends on monitor type)
	//   extract thresholds → map to alert observation
	// TODO: query monitor state for currently triggered monitors
	//   for each triggered monitor: create alert_fire observation
	// TODO: optionally import dashboard list (metadata only, not full dashboard JSON)
	return nil, nil
}

