

// === importers/opsdb-import-oncall/pagerduty.go ===
package oncall

// OnCallObservation is the observation structure for on-call importers.
type OnCallObservation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// ImportPagerDuty reads schedules, assignments, and escalation policies from PagerDuty.
func ImportPagerDuty(config *OnCallImportConfig) ([]OnCallObservation, error) {
	// TODO: paginate list schedules
	// TODO: for each schedule:
	//   extract name, timezone, rotation details
	//   create on_call_schedule observation
	//   query on-calls for schedule (current + upcoming window)
	//   for each on-call slot: create on_call_assignment observation
	// TODO: paginate list escalation policies
	// TODO: for each policy:
	//   extract name, description
	//   create escalation_path observation
	//   for each rule in policy:
	//     create escalation_step observation with step_order and target
	// TODO: map PD services to OpsDB services for service_escalation_path
	return nil, nil
}

// OnCallImportConfig holds on-call importer configuration.
type OnCallImportConfig struct {
	BackendType         string // pagerduty, opsgenie
	AssignmentWindowDays int   // how far ahead to import assignments
	BatchSize           int
	MaxRetries          int
}

