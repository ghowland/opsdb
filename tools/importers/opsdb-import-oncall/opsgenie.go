
// === importers/opsdb-import-oncall/opsgenie.go ===
package oncall

// ImportOpsgenie reads schedules, assignments, and escalation policies from Opsgenie.
func ImportOpsgenie(config *OnCallImportConfig) ([]OnCallObservation, error) {
	// TODO: list schedules via Opsgenie API
	// TODO: for each schedule:
	//   extract name, timezone, rotations
	//   create on_call_schedule observation
	//   query on-call participants for schedule
	//   for each participant slot: create on_call_assignment observation
	// TODO: list escalation policies
	// TODO: for each policy:
	//   extract name, rules
	//   create escalation_path observation
	//   for each rule: create escalation_step observation
	return nil, nil
}

