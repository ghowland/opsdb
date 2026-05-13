//# tools/opsdb_api/gate/step_execute.go

package gate

import (
	"fmt"
	"strings"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
)

// stepExecute is gate step 9: Execution.
// Performs the actual database write. Only runs if not rejected in prior
// steps. All writes within a single operation are atomic — wrapped in a
// single Postgres transaction. If any part fails, the entire transaction
// rolls back and the request is rejected with execution_failed.
//
// Read operations pass through with an empty ExecutionResult — the actual
// read data is returned through the operations package, not through the
// gate execute step.
func stepExecute(ctx *GateContext) {
	if ctx.Rejected {
		return
	}

	if isReadOnly(ctx.Request.OperationClass) {
		executeRead(ctx)
		return
	}

	err := pg.WithTransaction(ctx.DB, func(tx *pg.Tx) error {
		switch ctx.Request.Operation {
		case "write_observation":
			return executeWriteObservation(ctx, tx)
		case "submit_change_set":
			return executeSubmitChangeSet(ctx, tx)
		case "bulk_submit_change_set":
			return executeSubmitChangeSet(ctx, tx)
		case "emergency_apply":
			return executeEmergencyApply(ctx, tx)
		case "apply_change_set_field_change":
			return executeApplyFieldChange(ctx, tx)
		case "approve_change_set":
			return executeApproveChangeSet(ctx, tx)
		case "reject_change_set":
			return executeRejectChangeSet(ctx, tx)
		case "cancel_change_set":
			return executeCancelChangeSet(ctx, tx)
		case "mark_change_set_applied":
			return executeMarkApplied(ctx, tx)
		default:
			return fmt.Errorf("unhandled write operation: %s", ctx.Request.Operation)
		}
	})

	if err != nil {
		reject(ctx, 9, "execution_failed", err.Error(), nil)
	}
}

// executeRead handles read operations. Reads don't write anything — the
// actual query is performed by the operations package. We set an empty
// ExecutionResult so the response step has something to inspect.
func executeRead(ctx *GateContext) {
	ctx.ExecutionResult = &ExecutionResult{}
}

// ---------------------------------------------------------------------------
// Observation writes
// ---------------------------------------------------------------------------

// executeWriteObservation inserts or upserts into observation cache tables,
// runner_job_output_var, or evidence_record. Routes by target table name.
func executeWriteObservation(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params

	targetTable, _ := params["target_table"].(string)
	if targetTable == "" {
		targetTable = ctx.Request.TargetEntity
	}
	if targetTable == "" {
		return fmt.Errorf("write_observation requires target_table or target entity")
	}

	key, _ := params["key"].(string)
	value := params["value"]

	switch targetTable {
	case "observation_cache_metric":
		return upsertObservationCache(ctx, tx, targetTable,
			[]string{"authority_id", "hostname", "metric_key"},
			params, key, value)

	case "observation_cache_state":
		return upsertObservationCache(ctx, tx, targetTable,
			[]string{"entity_type", "entity_id", "state_key"},
			params, key, value)

	case "observation_cache_config":
		return upsertObservationCache(ctx, tx, targetTable,
			[]string{"authority_id", "hostname", "config_key"},
			params, key, value)

	case "runner_job_output_var":
		return insertObservationRow(ctx, tx, targetTable, params)

	case "evidence_record":
		return insertObservationRow(ctx, tx, targetTable, params)

	default:
		return fmt.Errorf("unsupported observation target table: %s", targetTable)
	}
}

// upsertObservationCache performs an INSERT ... ON CONFLICT ... DO UPDATE
// for the three observation cache tables. The conflict keys identify the
// unique observation being updated (e.g., authority+hostname+metric_key
// for metrics). Non-key columns are updated on conflict. Governance
// metadata columns (_observed_time, _authority_id, _puller_runner_job_id)
// are populated from params.
func upsertObservationCache(ctx *GateContext, tx *pg.Tx, table string, conflictKeys []string, params map[string]interface{}, key string, value interface{}) error {
	now := time.Now().UTC()

	// Build the column map. Start with conflict key values from params,
	// then add the observation value and governance metadata.
	columns := make(map[string]interface{})

	for _, ck := range conflictKeys {
		if v, ok := params[ck]; ok {
			columns[ck] = v
		}
	}

	// The value column name varies by table.
	if value != nil {
		switch table {
		case "observation_cache_metric":
			columns["metric_value"] = value
		case "observation_cache_state":
			columns["state_value"] = value
		case "observation_cache_config":
			columns["config_value"] = value
		}
	}

	// Data JSON column for additional structured payload
	if dataJSON, ok := params["data_json"]; ok {
		switch table {
		case "observation_cache_metric":
			columns["metric_data_json"] = dataJSON
		case "observation_cache_state":
			columns["state_data_json"] = dataJSON
		case "observation_cache_config":
			columns["config_data_json"] = dataJSON
		}
	}

	// Governance metadata — always set on observation writes
	columns["_observed_time"] = now
	if authorityID, ok := params["authority_id"]; ok {
		columns["_authority_id"] = authorityID
	}
	if runnerJobID, ok := params["runner_job_id"]; ok {
		columns["_puller_runner_job_id"] = runnerJobID
	}

	columns["created_time"] = now

	// Build the INSERT ... ON CONFLICT ... DO UPDATE statement.
	colNames := make([]string, 0, len(columns))
	placeholders := make([]string, 0, len(columns))
	values := make([]interface{}, 0, len(columns))
	updateClauses := make([]string, 0)

	i := 1
	for col, val := range columns {
		quotedCol := pg.QuoteIdentifier(col)
		colNames = append(colNames, quotedCol)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)

		// Non-key columns get updated on conflict
		if !isInSlice(col, conflictKeys) {
			updateClauses = append(updateClauses, fmt.Sprintf("%s = $%d", quotedCol, i))
		}

		i++
	}

	quotedConflictCols := make([]string, 0, len(conflictKeys))
	for _, ck := range conflictKeys {
		quotedConflictCols = append(quotedConflictCols, pg.QuoteIdentifier(ck))
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s RETURNING id",
		pg.QuoteIdentifier(table),
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(quotedConflictCols, ", "),
		strings.Join(updateClauses, ", "),
	)

	var rowID int
	err := pg.QueryRowInTx(tx, query, values...).Scan(&rowID)
	if err != nil {
		return fmt.Errorf("upsert into %s failed: %w", table, err)
	}

	ctx.ExecutionResult = &ExecutionResult{AffectedRowIDs: []int{rowID}}
	return nil
}

// insertObservationRow inserts a new row into runner_job_output_var or
// evidence_record. These are append-only tables — no upsert, just insert.
// Params are filtered to only include keys that are valid column names
// for the target table per the runtime schema.
func insertObservationRow(ctx *GateContext, tx *pg.Tx, table string, params map[string]interface{}) error {
	now := time.Now().UTC()

	columns := filterToColumns(ctx, table, params)
	columns["created_time"] = now

	colNames := make([]string, 0, len(columns))
	placeholders := make([]string, 0, len(columns))
	values := make([]interface{}, 0, len(columns))

	i := 1
	for col, val := range columns {
		colNames = append(colNames, pg.QuoteIdentifier(col))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING id",
		pg.QuoteIdentifier(table),
		strings.Join(colNames, ", "),
		strings.Join(placeholders, ", "),
	)

	var rowID int
	err := pg.QueryRowInTx(tx, query, values...).Scan(&rowID)
	if err != nil {
		return fmt.Errorf("insert into %s failed: %w", table, err)
	}

	ctx.ExecutionResult = &ExecutionResult{AffectedRowIDs: []int{rowID}}
	return nil
}

// ---------------------------------------------------------------------------
// Change set submission
// ---------------------------------------------------------------------------

// executeSubmitChangeSet creates a change_set row, its change_set_field_change
// rows, and any change_set_approval_required rows computed by step 7.
// Handles both regular and bulk submissions — the difference is only the
// is_bulk flag on the change_set row.
func executeSubmitChangeSet(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	name, _ := params["name"].(string)
	reason, _ := params["reason"].(string)
	isBulk := ctx.Request.Operation == "bulk_submit_change_set"

	// If step 7 (change management routing) determined auto-approval,
	// the change set goes directly to approved status. Otherwise it
	// enters pending_approval and waits for human approvers.
	// When step 7 is stubbed (CMRouting is nil), default to pending_approval.
	status := "pending_approval"
	if ctx.CMRouting != nil && ctx.CMRouting.AutoApproved {
		status = "approved"
	}

	var changeSetID int
	err := pg.QueryRowInTx(tx,
		"INSERT INTO change_set "+
			"(name, reason_text, change_set_status, is_bulk, is_emergency, "+
			"proposed_by_ops_user_id, created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, false, $5, $6, $6) RETURNING id",
		name, reason, status, isBulk, safeUserID(ctx.Identity), now,
	).Scan(&changeSetID)
	if err != nil {
		return fmt.Errorf("change_set insert failed: %w", err)
	}

	// Insert the individual field changes
	fieldChangeIDs, err := insertFieldChanges(tx, changeSetID, params, now)
	if err != nil {
		return fmt.Errorf("change_set_field_change insert failed: %w", err)
	}

	// Insert approval requirements from step 7's computation.
	// When step 7 is stubbed, CMRouting is nil and no requirements are written.
	if ctx.CMRouting != nil {
		for _, req := range ctx.CMRouting.ApprovalRequired {
			_, err := pg.ExecInTx(tx,
				"INSERT INTO change_set_approval_required "+
					"(change_set_id, approval_rule_id, ops_group_required_id, "+
					"approver_count_required, fulfilled_count, is_fulfilled, "+
					"created_time, updated_time) "+
					"VALUES ($1, $2, $3, $4, 0, false, $5, $5)",
				changeSetID, req.RuleID, req.GroupID, req.CountRequired, now,
			)
			if err != nil {
				return fmt.Errorf("change_set_approval_required insert failed: %w", err)
			}
		}
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: fieldChangeIDs,
		ChangeSetID:    changeSetID,
	}
	return nil
}

// ---------------------------------------------------------------------------
// Emergency apply
// ---------------------------------------------------------------------------

// executeEmergencyApply creates a change_set with is_emergency=true and
// status=approved (bypassing normal approval), plus a
// change_set_emergency_review row with a review deadline.
func executeEmergencyApply(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	name, _ := params["name"].(string)
	reason, _ := params["reason"].(string)

	var changeSetID int
	err := pg.QueryRowInTx(tx,
		"INSERT INTO change_set "+
			"(name, reason_text, change_set_status, is_bulk, is_emergency, "+
			"proposed_by_ops_user_id, created_time, updated_time) "+
			"VALUES ($1, $2, 'approved', false, true, $3, $4, $4) RETURNING id",
		name, reason, safeUserID(ctx.Identity), now,
	).Scan(&changeSetID)
	if err != nil {
		return fmt.Errorf("emergency change_set insert failed: %w", err)
	}

	fieldChangeIDs, err := insertFieldChanges(tx, changeSetID, params, now)
	if err != nil {
		return fmt.Errorf("emergency change_set_field_change insert failed: %w", err)
	}

	// Create the emergency review row. The emergency_review_monitor runner
	// watches for these and escalates when the deadline passes without review.
	_, err = pg.ExecInTx(tx,
		"INSERT INTO change_set_emergency_review "+
			"(change_set_id, review_status, created_time, updated_time) "+
			"VALUES ($1, 'pending', $2, $2)",
		changeSetID, now,
	)
	if err != nil {
		return fmt.Errorf("change_set_emergency_review insert failed: %w", err)
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: fieldChangeIDs,
		ChangeSetID:    changeSetID,
	}
	return nil
}

// ---------------------------------------------------------------------------
// Change set field change application (called by executor runner)
// ---------------------------------------------------------------------------

// executeApplyFieldChange applies one field change from an approved change
// set. Called by the change_set_executor runner. Reads the pending field
// change to get the target entity, field, and value; updates the entity
// row; marks the field change as applied; and optionally inserts a version
// sibling row if step 6 prepared versioning info.
func executeApplyFieldChange(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	changeSetID, err := toIntErr(params["change_set_id"])
	if err != nil || changeSetID == 0 {
		return fmt.Errorf("change_set_id is required and must be a positive integer")
	}

	fieldChangeID, err := toIntErr(params["field_change_id"])
	if err != nil || fieldChangeID == 0 {
		return fmt.Errorf("field_change_id is required and must be a positive integer")
	}

	// Read the field change to get what we're applying
	var entityType, fieldName string
	var entityID int
	var afterValue interface{}
	err = pg.QueryRowInTx(tx,
		"SELECT target_entity_type, target_entity_id, target_field_name, after_value_text "+
			"FROM change_set_field_change "+
			"WHERE id = $1 AND change_set_id = $2 AND applied_status = 'pending'",
		fieldChangeID, changeSetID,
	).Scan(&entityType, &entityID, &fieldName, &afterValue)
	if err != nil {
		return fmt.Errorf("field change %d not found or not in pending status: %w", fieldChangeID, err)
	}

	// Apply the field value to the target entity row
	result, err := pg.ExecInTx(tx,
		fmt.Sprintf("UPDATE %s SET %s = $1, updated_time = $2 WHERE id = $3",
			pg.QuoteIdentifier(entityType),
			pg.QuoteIdentifier(fieldName),
		),
		afterValue, now, entityID,
	)
	if err != nil {
		// Mark the field change as failed before returning
		pg.ExecInTx(tx,
			"UPDATE change_set_field_change "+
				"SET applied_status = 'failed', applied_error_text = $1, updated_time = $2 "+
				"WHERE id = $3",
			err.Error(), now, fieldChangeID,
		)
		return fmt.Errorf("failed to apply %s.%s on id=%d: %w", entityType, fieldName, entityID, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%s with id=%d not found", entityType, entityID)
	}

	// Mark the field change as applied
	_, err = pg.ExecInTx(tx,
		"UPDATE change_set_field_change "+
			"SET applied_status = 'applied', updated_time = $1 "+
			"WHERE id = $2",
		now, fieldChangeID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark field change %d as applied: %w", fieldChangeID, err)
	}

	// Insert version sibling row if the entity is versioned and step 6
	// prepared versioning info. When step 6 leaves VersionInfo nil,
	// we skip this — the entity update still applies, just without
	// version history until step 6 is implemented.
	var versionRowID int
	if ctx.VersionInfo != nil {
		versionTable := entityType + "_version"
		entityFKCol := entityType + "_id"
		parentVIDCol := "parent_" + entityType + "_version_id"

		err = pg.QueryRowInTx(tx,
			fmt.Sprintf(
				"INSERT INTO %s (%s, version_serial, %s, "+
					"change_set_id, is_active_version, created_time, updated_time) "+
					"VALUES ($1, $2, $3, $4, true, $5, $5) RETURNING id",
				pg.QuoteIdentifier(versionTable),
				pg.QuoteIdentifier(entityFKCol),
				pg.QuoteIdentifier(parentVIDCol),
			),
			entityID, ctx.VersionInfo.NextSerial, ctx.VersionInfo.ParentVID,
			changeSetID, now,
		).Scan(&versionRowID)
		if err != nil {
			// Version insert failure is not fatal to the field change apply.
			// The entity was updated; we warn and continue.
			warn(ctx, fmt.Sprintf("version row insert failed for %s id=%d: %v", entityType, entityID, err))
		} else {
			// Deactivate the previous active version for this entity
			_, _ = pg.ExecInTx(tx,
				fmt.Sprintf(
					"UPDATE %s SET is_active_version = false, updated_time = $1 "+
						"WHERE %s = $2 AND is_active_version = true AND id != $3",
					pg.QuoteIdentifier(versionTable),
					pg.QuoteIdentifier(entityFKCol),
				),
				now, entityID, versionRowID,
			)
		}
	}

	affectedIDs := []int{entityID}
	var versionIDs []int
	if versionRowID > 0 {
		versionIDs = []int{versionRowID}
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: affectedIDs,
		VersionRowIDs:  versionIDs,
		ChangeSetID:    changeSetID,
	}
	return nil
}

// ---------------------------------------------------------------------------
// Change management actions
// ---------------------------------------------------------------------------

// executeApproveChangeSet records an approval on a change set.
func executeApproveChangeSet(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	changeSetID, err := toIntErr(params["change_set_id"])
	if err != nil || changeSetID == 0 {
		return fmt.Errorf("change_set_id is required")
	}

	comment, _ := params["comment"].(string)

	// Insert the approval record
	var approvalID int
	err = pg.QueryRowInTx(tx,
		"INSERT INTO change_set_approval "+
			"(change_set_id, approving_ops_user_id, approval_data_json, "+
			"approved_time, created_time) "+
			"VALUES ($1, $2, $3, $4, $4) RETURNING id",
		changeSetID, safeUserID(ctx.Identity), comment, now,
	).Scan(&approvalID)
	if err != nil {
		return fmt.Errorf("change_set_approval insert failed: %w", err)
	}

	// Increment fulfilled_count on matching approval requirements
	if ctx.Identity != nil && ctx.Identity.OpsUserID != nil {
		_, err = pg.ExecInTx(tx,
			"UPDATE change_set_approval_required "+
				"SET fulfilled_count = fulfilled_count + 1, "+
				"is_fulfilled = (fulfilled_count + 1 >= approver_count_required), "+
				"updated_time = $1 "+
				"WHERE change_set_id = $2 "+
				"AND is_fulfilled = false "+
				"AND EXISTS ("+
				"SELECT 1 FROM ops_group_member "+
				"WHERE ops_group_id = change_set_approval_required.ops_group_required_id "+
				"AND ops_user_id = $3)",
			now, changeSetID, *ctx.Identity.OpsUserID,
		)
		if err != nil {
			return fmt.Errorf("approval fulfillment update failed: %w", err)
		}
	}

	// Check if all approval requirements are now fulfilled
	var unfulfilled int
	err = pg.QueryRowInTx(tx,
		"SELECT COUNT(*) FROM change_set_approval_required "+
			"WHERE change_set_id = $1 AND is_fulfilled = false",
		changeSetID,
	).Scan(&unfulfilled)
	if err != nil {
		return fmt.Errorf("fulfillment check query failed: %w", err)
	}

	// If all requirements are fulfilled, transition to approved
	if unfulfilled == 0 {
		_, err = pg.ExecInTx(tx,
			"UPDATE change_set SET change_set_status = 'approved', updated_time = $1 "+
				"WHERE id = $2 AND change_set_status = 'pending_approval'",
			now, changeSetID,
		)
		if err != nil {
			return fmt.Errorf("change_set transition to approved failed: %w", err)
		}
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: []int{approvalID},
		ChangeSetID:    changeSetID,
	}
	return nil
}

// executeRejectChangeSet records a rejection and transitions the change set.
func executeRejectChangeSet(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	changeSetID, err := toIntErr(params["change_set_id"])
	if err != nil || changeSetID == 0 {
		return fmt.Errorf("change_set_id is required")
	}

	reason, _ := params["rejection_reason"].(string)
	if reason == "" {
		reason, _ = params["reason"].(string)
	}

	var rejectionID int
	err = pg.QueryRowInTx(tx,
		"INSERT INTO change_set_rejection "+
			"(change_set_id, rejecting_ops_user_id, rejection_reason_text, "+
			"rejected_time, created_time) "+
			"VALUES ($1, $2, $3, $4, $4) RETURNING id",
		changeSetID, safeUserID(ctx.Identity), reason, now,
	).Scan(&rejectionID)
	if err != nil {
		return fmt.Errorf("change_set_rejection insert failed: %w", err)
	}

	_, err = pg.ExecInTx(tx,
		"UPDATE change_set SET change_set_status = 'rejected', updated_time = $1 "+
			"WHERE id = $2 AND change_set_status = 'pending_approval'",
		now, changeSetID,
	)
	if err != nil {
		return fmt.Errorf("change_set transition to rejected failed: %w", err)
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: []int{rejectionID},
		ChangeSetID:    changeSetID,
	}
	return nil
}

// executeCancelChangeSet transitions a change set to cancelled.
func executeCancelChangeSet(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	changeSetID, err := toIntErr(params["change_set_id"])
	if err != nil || changeSetID == 0 {
		return fmt.Errorf("change_set_id is required")
	}

	result, err := pg.ExecInTx(tx,
		"UPDATE change_set SET change_set_status = 'cancelled', updated_time = $1 "+
			"WHERE id = $2 AND change_set_status IN ('draft', 'pending_approval')",
		now, changeSetID,
	)
	if err != nil {
		return fmt.Errorf("change_set cancellation failed: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("change_set %d not found or not in a cancellable state (must be draft or pending_approval)",
			changeSetID)
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: []int{changeSetID},
		ChangeSetID:    changeSetID,
	}
	return nil
}

// executeMarkApplied verifies all field changes are applied, then transitions
// the change set to applied status.
func executeMarkApplied(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	changeSetID, err := toIntErr(params["change_set_id"])
	if err != nil || changeSetID == 0 {
		return fmt.Errorf("change_set_id is required")
	}

	var pendingCount int
	err = pg.QueryRowInTx(tx,
		"SELECT COUNT(*) FROM change_set_field_change "+
			"WHERE change_set_id = $1 AND applied_status != 'applied'",
		changeSetID,
	).Scan(&pendingCount)
	if err != nil {
		return fmt.Errorf("field change status check failed: %w", err)
	}

	if pendingCount > 0 {
		return fmt.Errorf("cannot mark change_set %d as applied: %d field changes not yet applied",
			changeSetID, pendingCount)
	}

	result, err := pg.ExecInTx(tx,
		"UPDATE change_set "+
			"SET change_set_status = 'applied', applied_time = $1, updated_time = $1 "+
			"WHERE id = $2 AND change_set_status = 'approved'",
		now, changeSetID,
	)
	if err != nil {
		return fmt.Errorf("change_set transition to applied failed: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("change_set %d not found or not in approved status", changeSetID)
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: []int{changeSetID},
		ChangeSetID:    changeSetID,
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// insertFieldChanges creates change_set_field_change rows from the
// field_changes array in request params.
func insertFieldChanges(tx *pg.Tx, changeSetID int, params map[string]interface{}, now time.Time) ([]int, error) {
	rawChanges, ok := params["field_changes"]
	if !ok {
		return nil, nil
	}

	changeList, ok := rawChanges.([]interface{})
	if !ok {
		return nil, fmt.Errorf("field_changes must be an array")
	}

	ids := make([]int, 0, len(changeList))

	for order, item := range changeList {
		changeMap, ok := item.(map[string]interface{})
		if !ok {
			return ids, fmt.Errorf("field_changes[%d] must be an object", order)
		}

		entityType, _ := changeMap["entity_type"].(string)
		fieldName, _ := changeMap["field_name"].(string)

		if entityType == "" || fieldName == "" {
			return ids, fmt.Errorf("field_changes[%d] requires entity_type and field_name", order)
		}

		entityID, _ := toIntErr(changeMap["entity_id"])
		beforeValue := changeMap["before_value"]
		afterValue := changeMap["after_value"]

		var fieldChangeID int
		err := pg.QueryRowInTx(tx,
			"INSERT INTO change_set_field_change "+
				"(change_set_id, target_entity_type, target_entity_id, target_field_name, "+
				"before_value_text, after_value_text, apply_order, "+
				"applied_status, created_time, updated_time) "+
				"VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8, $8) RETURNING id",
			changeSetID, entityType, entityID, fieldName,
			beforeValue, afterValue, order+1, now,
		).Scan(&fieldChangeID)
		if err != nil {
			return ids, fmt.Errorf("field_change insert for %s.%s failed: %w", entityType, fieldName, err)
		}

		ids = append(ids, fieldChangeID)
	}

	return ids, nil
}

// safeUserID returns the ops_user_id from an identity as an interface{}
// suitable for use as a SQL parameter. Returns nil if the identity is nil
// or has no OpsUserID, which becomes a SQL NULL.
func safeUserID(identity *Identity) interface{} {
	if identity != nil && identity.OpsUserID != nil {
		return *identity.OpsUserID
	}
	return nil
}

// filterToColumns filters request params to only include keys that are
// valid column names for the target table per the runtime schema.
func filterToColumns(ctx *GateContext, table string, params map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})

	for key, val := range params {
		if key == "target_table" || key == "key" || key == "value" {
			continue
		}

		_, isField := ctx.Schema.GetField(table, key)
		if isField {
			filtered[key] = val
		}
	}

	return filtered
}

// toIntErr converts an interface{} to int, returning an error on failure.
// This is the (int, error) variant used by step_execute.go for cases where
// the error message matters. The validation steps use toInt from
// step_bound_validate.go which returns (int, bool).
func toIntErr(v interface{}) (int, error) {
	if v == nil {
		return 0, fmt.Errorf("nil value")
	}

	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	case string:
		var n int
		_, err := fmt.Sscanf(val, "%d", &n)
		if err != nil {
			return 0, fmt.Errorf("cannot convert %q to int: %w", val, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

// isInSlice returns true if needle is in the haystack slice.
func isInSlice(needle string, haystack []string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
