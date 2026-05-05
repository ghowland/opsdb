// === opsdb-api/operations/write_changeset.go ===
package operations

// SubmitChangeSet handles the submit_change_set operation.
// Creates change_set, change_set_field_change, and change_set_approval_required rows.
// Supports dry_run mode.
func SubmitChangeSet(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	// TODO: validate optimistic concurrency (each field change's version stamp vs current)
	//       via concurrency.ValidateVersionStamps()
	//       if stale: return stale_version error
	//
	// TODO: if params.DryRun:
	//   run full validation pipeline
	//   compute approval requirements
	//   return result without writing any rows
	//
	// TODO: INSERT change_set row (status=submitted)
	// TODO: INSERT change_set_field_change rows (one per field change, apply_order set)
	// TODO: run validation pipeline (schema, bound, semantic, policy, lint, dependency)
	//   on recoverable error: update change_set status to draft, return errors
	//   on unrecoverable error: update change_set status to rejected, return errors
	//   on success: update change_set status to pending_approval (or approved if auto)
	// TODO: INSERT change_set_approval_required rows from CM routing
	// TODO: return ChangeSetResult
	return nil, nil
}

// EmergencyApply handles the emergency_apply operation.
// Same as submit but with is_emergency=true and reduced approvals.
func EmergencyApply(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	// TODO: verify caller has emergency authority per policy
	// TODO: submit with is_emergency=true
	// TODO: INSERT change_set_emergency_review row with status=pending
	// TODO: approval requirements reduced per emergency path policy
	// TODO: return ChangeSetResult
	return nil, nil
}

// BulkSubmit handles the bulk_submit_change_set operation.
// Chunked validation and atomic submission.
func BulkSubmit(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	// TODO: set is_bulk=true on change_set
	// TODO: chunk field changes (default 1000 per chunk)
	// TODO: validate each chunk, providing interim feedback
	//   on any chunk failure: entire change_set fails
	// TODO: if all chunks valid: write all rows atomically
	// TODO: approval routing may produce bundle-level approval (not per-entity)
	// TODO: return ChangeSetResult
	return nil, nil
}

// SubmitChangeSetParams holds change set submission parameters.
type SubmitChangeSetParams struct {
	SiteID        int
	Name          string
	Description   string
	Reason        string
	FieldChanges  []FieldChange
	TicketRef     *int // authority_pointer_id
	IsEmergency   bool
	IsBulk        bool
	DryRun        bool
	ProposerUser  *int // ops_user_id
	ProposerJob   *int // runner_job_id
}

// FieldChange represents one field change in a change set submission.
type FieldChange struct {
	EntityType      string
	EntityID        int
	FieldName       string
	BeforeValue     interface{}
	AfterValue      interface{}
	ChangeType      string // create, update, delete
	VersionStamp    int    // version of entity this change was drafted against
}

// ChangeSetResult holds the result of a change set operation.
type ChangeSetResult struct {
	ChangeSetID       int
	Status            string
	ApprovalRequired  []interface{} // computed approval requirements
	ValidationErrors  []interface{} // if validation failed
	DryRunResult      interface{}   // populated only for dry_run
}

