
// === opsdb-api/operations/changeset_actions.go ===
package operations

// ApproveChangeSet records a stakeholder approval.
// Verifies caller is in a required approver group.
// Increments fulfilled count. May transition status to approved.
func ApproveChangeSet(changeSetID int, approverUserID int, comments string) error {
	// TODO: read change_set_approval_required rows for this change set
	// TODO: determine which requirement(s) the approver can fulfill
	//       (approver must be in ops_group_required_id for at least one requirement)
	// TODO: check SoD: approver != submitter (if policy requires)
	// TODO: INSERT change_set_approval row
	// TODO: UPDATE change_set_approval_required: increment fulfilled_count, set is_fulfilled if met
	// TODO: if ALL requirements now is_fulfilled=true:
	//       UPDATE change_set status to approved
	return nil
}

// RejectChangeSet records a stakeholder rejection.
// Verifies caller is in a required approver group.
// Transitions change set to rejected per rejection semantics.
func RejectChangeSet(changeSetID int, rejectorUserID int, reason string) error {
	// TODO: verify rejector is in a required approver group
	// TODO: INSERT change_set_rejection row
	// TODO: evaluate rejection_behavior from matching approval_rule:
	//       any_rejects_halts → immediately reject
	//       majority_rejects_halts → count rejections vs required, reject if majority
	//       all_must_reject → reject only if all approvers reject
	// TODO: if rejected: UPDATE change_set status to rejected
	return nil
}

// CancelChangeSet withdraws a change set.
// Available to original submitter or user with sufficient authority.
func CancelChangeSet(changeSetID int, cancellerUserID int) error {
	// TODO: read change_set to verify status is cancellable (draft, submitted, pending_approval)
	// TODO: verify canceller is submitter OR has cancel authority
	// TODO: UPDATE change_set status to cancelled
	return nil
}

// ApplyFieldChange applies one approved field change.
// Used by the change-set executor runner.
func ApplyFieldChange(changeSetID int, fieldChangeID int, executorID int) error {
	// TODO: verify change_set status is approved
	// TODO: verify field change applied_status is pending (not already applied)
	// TODO: verify caller has executor authority
	// TODO: read the field change (entity_type, entity_id, field_name, after_value)
	// TODO: UPDATE target entity row: SET field_name = after_value
	// TODO: INSERT {entity}_version row with full entity state after update
	//       (version_serial, parent version, change_set_id, is_active_version=true)
	// TODO: UPDATE previous version: is_active_version = false
	// TODO: UPDATE change_set_field_change: applied_status = applied
	// TODO: on failure: UPDATE applied_status = failed, set applied_error_text
	return nil
}

// MarkChangeSetApplied finalizes change set status after all field changes applied.
func MarkChangeSetApplied(changeSetID int, executorID int) error {
	// TODO: read all change_set_field_change rows for this change set
	// TODO: verify ALL have applied_status = applied
	//       if any are pending or failed: return error
	// TODO: UPDATE change_set status = applied, applied_time = NOW()
	return nil
}


