
// === opsdb-api/gate/step_execute.go ===
package gate

// StepExecute is gate step 9: Execution.
// Performs the actual database write. Only runs if not rejected in prior steps.
// All writes within a single operation are atomic (single transaction).
func StepExecute(ctx *GateContext) error {
	// TODO: if ctx.Rejected: skip execution entirely
	//
	// TODO: switch on operation type:
	//
	// CASE write_observation:
	//   INSERT or UPSERT into target observation cache table
	//   (keyed by authority+entity_type+entity_id+state_key for state cache,
	//    authority+hostname+metric_key for metric cache)
	//
	// CASE submit_change_set:
	//   INSERT change_set row
	//   INSERT change_set_field_change rows (one per field change)
	//   INSERT change_set_approval_required rows (from step 7)
	//   if auto-approved: transition change_set status to approved
	//   if emergency: INSERT change_set_emergency_review row with status=pending
	//
	// CASE apply_change_set_field_change:
	//   UPDATE target entity row with new field value
	//   INSERT {entity}_version row with full entity state (step 6 prepared this)
	//   UPDATE change_set_field_change.applied_status = applied
	//
	// CASE approve_change_set:
	//   INSERT change_set_approval row
	//   UPDATE change_set_approval_required.fulfilled_count
	//   if all requirements fulfilled: transition change_set status to approved
	//
	// CASE reject_change_set:
	//   INSERT change_set_rejection row
	//   transition change_set status to rejected
	//
	// CASE cancel_change_set:
	//   transition change_set status to cancelled
	//
	// CASE mark_change_set_applied:
	//   verify all field changes have applied_status=applied
	//   transition change_set status to applied, set applied_time
	//
	// CASE get_entity / search / etc. (reads):
	//   delegate to operations/read.go (already executed before gate for reads)
	//
	// TODO: set ctx.ExecutionResult with affected row IDs
	return nil
}


