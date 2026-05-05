//# tools/opsdb-api/gate/step_execute.go

package gate

import (
	"fmt"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
)

// stepExecute is gate step 9: Execution.
// Performs the actual database write. Only runs if not rejected in prior steps.
// All writes within a single operation are atomic (single transaction).
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
			return fmt.Errorf("unhandled operation for execution: %s", ctx.Request.Operation)
		}
	})

	if err != nil {
		reject(ctx, 9, "execution_failed", err.Error(), nil)
	}
}

// executeRead handles read operations by delegating to the query path.
// Reads don't need a transaction for writes but do populate ExecutionResult
// for the response step.
func executeRead(ctx *GateContext) {
	ctx.ExecutionResult = &ExecutionResult{}
}

// executeWriteObservation inserts or upserts into observation cache tables,
// runner_job_output_var, or evidence_record.
func executeWriteObservation(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	targetTable, _ := params["target_table"].(string)
	if targetTable == "" {
		targetTable = ctx.Request.TargetEntity
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

	case "runner_job_output_var", "evidence_record":
		return insertObservationRow(ctx, tx, targetTable, params)

	default:
		// direct entity field write (e.g., soft-delete via is_active update)
		if ctx.Request.TargetEntityID > 0 && key != "" {
			return updateEntityField(ctx, tx, targetTable, ctx.Request.TargetEntityID, key, value)
		}
		return fmt.Errorf("unsupported observation target: %s", targetTable)
	}
}

// upsertObservationCache performs an INSERT ON CONFLICT UPDATE for cache tables.
func upsertObservationCache(ctx *GateContext, tx *pg.Tx, table string, conflictKeys []string, params map[string]interface{}, key string, value interface{}) error {
	columns := make(map[string]interface{})

	// populate conflict key columns from params
	for _, ck := range conflictKeys {
		if v, ok := params[ck]; ok {
			columns[ck] = v
		}
	}

	columns["value_json"] = value
	columns["_observed_time"] = time.Now().UTC()

	if authorityID, ok := params["authority_id"]; ok {
		columns["_authority_id"] = authorityID
	}
	if runnerJobID, ok := params["runner_job_id"]; ok {
		columns["_puller_runner_job_id"] = runnerJobID
	}

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

		// update non-key columns on conflict
		isConflictKey := false
		for _, ck := range conflictKeys {
			if col == ck {
				isConflictKey = true
				break
			}
		}
		if !isConflictKey {
			updateClauses = append(updateClauses, fmt.Sprintf("%s = $%d", quotedCol, i))
		}
		i++
	}

	conflictCols := make([]string, 0, len(conflictKeys))
	for _, ck := range conflictKeys {
		conflictCols = append(conflictCols, pg.QuoteIdentifier(ck))
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s RETURNING id",
		pg.QuoteIdentifier(table),
		joinStrings(colNames, ", "),
		joinStrings(placeholders, ", "),
		joinStrings(conflictCols, ", "),
		joinStrings(updateClauses, ", "),
	)

	var rowID int
	err := pg.QueryInTx(tx, query, values...).Scan(&rowID)
	if err != nil {
		return fmt.Errorf("upsert into %s failed: %w", table, err)
	}

	ctx.ExecutionResult = &ExecutionResult{AffectedRowIDs: []int{rowID}}
	return nil
}

// insertObservationRow inserts a new row into runner_job_output_var or
// evidence_record tables.
func insertObservationRow(ctx *GateContext, tx *pg.Tx, table string, params map[string]interface{}) error {
	// filter params to only include valid column names
	columns := filterToColumns(ctx, table, params)
	columns["created_time"] = time.Now().UTC()

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
		joinStrings(colNames, ", "),
		joinStrings(placeholders, ", "),
	)

	var rowID int
	err := pg.QueryInTx(tx, query, values...).Scan(&rowID)
	if err != nil {
		return fmt.Errorf("insert into %s failed: %w", table, err)
	}

	ctx.ExecutionResult = &ExecutionResult{AffectedRowIDs: []int{rowID}}
	return nil
}

// updateEntityField updates a single field on an entity row.
func updateEntityField(ctx *GateContext, tx *pg.Tx, entityType string, entityID int, fieldName string, value interface{}) error {
	query := fmt.Sprintf(
		"UPDATE %s SET %s = $1, updated_time = $2 WHERE id = $3",
		pg.QuoteIdentifier(entityType),
		pg.QuoteIdentifier(fieldName),
	)

	result, err := pg.ExecInTx(tx, query, value, time.Now().UTC(), entityID)
	if err != nil {
		return fmt.Errorf("update %s.%s failed: %w", entityType, fieldName, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("%s with id=%d not found", entityType, entityID)
	}

	ctx.ExecutionResult = &ExecutionResult{AffectedRowIDs: []int{entityID}}
	return nil
}

// executeSubmitChangeSet creates change_set, field_change, and approval_required rows.
func executeSubmitChangeSet(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	// insert change_set
	name, _ := params["name"].(string)
	reason, _ := params["reason"].(string)
	isBulk := ctx.Request.Operation == "bulk_submit_change_set"

	status := "pending_approval"
	if ctx.CMRouting != nil && ctx.CMRouting.AutoApproved {
		status = "approved"
	}

	var changeSetID int
	err := pg.QueryInTx(tx,
		"INSERT INTO change_set (name, reason, status, is_bulk, is_emergency, "+
			"submitted_by_ops_user_id, submitted_time, created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, false, $5, $6, $6, $6) RETURNING id",
		name, reason, status, isBulk, safeUserID(ctx.Identity), now,
	).Scan(&changeSetID)
	if err != nil {
		return fmt.Errorf("change_set insert failed: %w", err)
	}

	// insert field changes
	fieldChangeIDs, err := insertFieldChanges(tx, changeSetID, params, now)
	if err != nil {
		return fmt.Errorf("field_change insert failed: %w", err)
	}

	// insert approval requirements from step 7
	if ctx.CMRouting != nil {
		for _, req := range ctx.CMRouting.ApprovalRequired {
			_, err := pg.ExecInTx(tx,
				"INSERT INTO change_set_approval_required "+
					"(change_set_id, approval_rule_id, required_group_id, required_count, "+
					"fulfilled_count, is_fulfilled, created_time, updated_time) "+
					"VALUES ($1, $2, $3, $4, 0, false, $5, $5)",
				changeSetID, req.RuleID, req.GroupID, req.CountRequired, now,
			)
			if err != nil {
				return fmt.Errorf("approval_required insert failed: %w", err)
			}
		}
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: fieldChangeIDs,
		ChangeSetID:    changeSetID,
	}
	return nil
}

// executeEmergencyApply creates change_set with is_emergency=true and
// an emergency_review row.
func executeEmergencyApply(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	name, _ := params["name"].(string)
	reason, _ := params["reason"].(string)

	var changeSetID int
	err := pg.QueryInTx(tx,
		"INSERT INTO change_set (name, reason, status, is_bulk, is_emergency, "+
			"submitted_by_ops_user_id, submitted_time, created_time, updated_time) "+
			"VALUES ($1, $2, 'approved', false, true, $3, $4, $4, $4) RETURNING id",
		name, reason, safeUserID(ctx.Identity), now,
	).Scan(&changeSetID)
	if err != nil {
		return fmt.Errorf("emergency change_set insert failed: %w", err)
	}

	fieldChangeIDs, err := insertFieldChanges(tx, changeSetID, params, now)
	if err != nil {
		return fmt.Errorf("emergency field_change insert failed: %w", err)
	}

	// create emergency review row — must be reviewed within 72 hours
	_, err = pg.ExecInTx(tx,
		"INSERT INTO change_set_emergency_review "+
			"(change_set_id, status, review_deadline_time, created_time, updated_time) "+
			"VALUES ($1, 'pending', $2, $3, $3)",
		changeSetID, now.Add(72*time.Hour), now,
	)
	if err != nil {
		return fmt.Errorf("emergency_review insert failed: %w", err)
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: fieldChangeIDs,
		ChangeSetID:    changeSetID,
	}
	return nil
}

// executeApplyFieldChange applies one field change from an approved change set.
// Called by the change-set-executor runner.
func executeApplyFieldChange(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	changeSetID, _ := toInt(params["change_set_id"])
	fieldChangeID, _ := toInt(params["field_change_id"])

	if changeSetID == 0 || fieldChangeID == 0 {
		return fmt.Errorf("change_set_id and field_change_id are required")
	}

	// read the field change to get target entity, field, and value
	var entityType, fieldName string
	var entityID int
	var afterValue interface{}
	err := pg.QueryInTx(tx,
		"SELECT target_entity_type, target_entity_id, field_name, after_value "+
			"FROM change_set_field_change "+
			"WHERE id = $1 AND change_set_id = $2 AND applied_status = 'pending'",
		fieldChangeID, changeSetID,
	).Scan(&entityType, &entityID, &fieldName, &afterValue)
	if err != nil {
		return fmt.Errorf("field change %d not found or not pending: %w", fieldChangeID, err)
	}

	// apply the field value to the target entity
	_, err = pg.ExecInTx(tx,
		fmt.Sprintf("UPDATE %s SET %s = $1, updated_time = $2 WHERE id = $3",
			pg.QuoteIdentifier(entityType),
			pg.QuoteIdentifier(fieldName),
		),
		afterValue, now, entityID,
	)
	if err != nil {
		// mark field change as failed
		pg.ExecInTx(tx,
			"UPDATE change_set_field_change SET applied_status = 'failed', "+
				"applied_time = $1, updated_time = $1 WHERE id = $2",
			now, fieldChangeID,
		)
		return fmt.Errorf("failed to apply %s.%s on %s id=%d: %w",
			entityType, fieldName, entityType, entityID, err)
	}

	// mark field change as applied
	_, err = pg.ExecInTx(tx,
		"UPDATE change_set_field_change SET applied_status = 'applied', "+
			"applied_time = $1, updated_time = $1 WHERE id = $2",
		now, fieldChangeID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark field change %d as applied: %w", fieldChangeID, err)
	}

	// insert version sibling row if entity is versioned and step 6 prepared it
	var versionRowID int
	if ctx.VersionInfo != nil {
		err = pg.QueryInTx(tx,
			fmt.Sprintf(
				"INSERT INTO %s (%s, version_serial, parent_%s_version_id, "+
					"change_set_id, is_active_version, approved_for_production_time, "+
					"created_time, updated_time) "+
					"VALUES ($1, $2, $3, $4, true, $5, $5, $5) RETURNING id",
				pg.QuoteIdentifier(entityType+"_version"),
				pg.QuoteIdentifier(entityType+"_id"),
				entityType,
			),
			entityID, ctx.VersionInfo.NextSerial, ctx.VersionInfo.ParentVID,
			changeSetID, now,
		).Scan(&versionRowID)
		if err != nil {
			// version insert failure is not fatal to the field change apply
			warn(ctx, fmt.Sprintf("version row insert failed for %s id=%d: %v", entityType, entityID, err))
		} else {
			// deactivate previous version
			pg.ExecInTx(tx,
				fmt.Sprintf(
					"UPDATE %s SET is_active_version = false, updated_time = $1 "+
						"WHERE %s = $2 AND is_active_version = true AND id != $3",
					pg.QuoteIdentifier(entityType+"_version"),
					pg.QuoteIdentifier(entityType+"_id"),
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

// executeApproveChangeSet records an approval and checks if all requirements
// are now fulfilled.
func executeApproveChangeSet(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	changeSetID, _ := toInt(params["change_set_id"])
	comment, _ := params["comment"].(string)

	if changeSetID == 0 {
		return fmt.Errorf("change_set_id is required")
	}

	// insert approval record
	var approvalID int
	err := pg.QueryInTx(tx,
		"INSERT INTO change_set_approval "+
			"(change_set_id, approved_by_ops_user_id, comment, created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, $4) RETURNING id",
		changeSetID, safeUserID(ctx.Identity), comment, now,
	).Scan(&approvalID)
	if err != nil {
		return fmt.Errorf("approval insert failed: %w", err)
	}

	// update fulfillment counts on matching approval requirements
	// match by group membership: if approver is in the required group,
	// increment fulfilled_count
	if ctx.Identity.OpsUserID != nil {
		_, err = pg.ExecInTx(tx,
			"UPDATE change_set_approval_required car "+
				"SET fulfilled_count = fulfilled_count + 1, "+
				"is_fulfilled = (fulfilled_count + 1 >= required_count), "+
				"updated_time = $1 "+
				"WHERE car.change_set_id = $2 "+
				"AND car.is_fulfilled = false "+
				"AND EXISTS (SELECT 1 FROM ops_group_member gm "+
				"WHERE gm.ops_group_id = car.required_group_id "+
				"AND gm.ops_user_id = $3 AND gm.is_active = true)",
			now, changeSetID, *ctx.Identity.OpsUserID,
		)
		if err != nil {
			return fmt.Errorf("approval fulfillment update failed: %w", err)
		}
	}

	// check if all requirements are now fulfilled
	var unfulfilled int
	err = pg.QueryInTx(tx,
		"SELECT COUNT(*) FROM change_set_approval_required "+
			"WHERE change_set_id = $1 AND is_fulfilled = false",
		changeSetID,
	).Scan(&unfulfilled)
	if err != nil {
		return fmt.Errorf("fulfillment check failed: %w", err)
	}

	if unfulfilled == 0 {
		_, err = pg.ExecInTx(tx,
			"UPDATE change_set SET status = 'approved', updated_time = $1 "+
				"WHERE id = $2 AND status = 'pending_approval'",
			now, changeSetID,
		)
		if err != nil {
			return fmt.Errorf("change_set status transition to approved failed: %w", err)
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

	changeSetID, _ := toInt(params["change_set_id"])
	reason, _ := params["reason"].(string)

	if changeSetID == 0 {
		return fmt.Errorf("change_set_id is required")
	}

	var rejectionID int
	err := pg.QueryInTx(tx,
		"INSERT INTO change_set_rejection "+
			"(change_set_id, rejected_by_ops_user_id, reason, created_time, updated_time) "+
			"VALUES ($1, $2, $3, $4, $4) RETURNING id",
		changeSetID, safeUserID(ctx.Identity), reason, now,
	).Scan(&rejectionID)
	if err != nil {
		return fmt.Errorf("rejection insert failed: %w", err)
	}

	_, err = pg.ExecInTx(tx,
		"UPDATE change_set SET status = 'rejected', updated_time = $1 "+
			"WHERE id = $2 AND status = 'pending_approval'",
		now, changeSetID,
	)
	if err != nil {
		return fmt.Errorf("change_set status transition to rejected failed: %w", err)
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

	changeSetID, _ := toInt(params["change_set_id"])
	if changeSetID == 0 {
		return fmt.Errorf("change_set_id is required")
	}

	result, err := pg.ExecInTx(tx,
		"UPDATE change_set SET status = 'cancelled', updated_time = $1 "+
			"WHERE id = $2 AND status IN ('draft', 'pending_approval')",
		now, changeSetID,
	)
	if err != nil {
		return fmt.Errorf("change_set cancellation failed: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("change_set %d not found or not in cancellable state", changeSetID)
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: []int{changeSetID},
		ChangeSetID:    changeSetID,
	}
	return nil
}

// executeMarkApplied verifies all field changes are applied and transitions
// the change set to applied status.
func executeMarkApplied(ctx *GateContext, tx *pg.Tx) error {
	params := ctx.Request.Params
	now := time.Now().UTC()

	changeSetID, _ := toInt(params["change_set_id"])
	if changeSetID == 0 {
		return fmt.Errorf("change_set_id is required")
	}

	// verify all field changes have been applied
	var pending int
	err := pg.QueryInTx(tx,
		"SELECT COUNT(*) FROM change_set_field_change "+
			"WHERE change_set_id = $1 AND applied_status != 'applied'",
		changeSetID,
	).Scan(&pending)
	if err != nil {
		return fmt.Errorf("field change status check failed: %w", err)
	}

	if pending > 0 {
		return fmt.Errorf("cannot mark change_set %d as applied: %d field changes not yet applied",
			changeSetID, pending)
	}

	result, err := pg.ExecInTx(tx,
		"UPDATE change_set SET status = 'applied', applied_time = $1, updated_time = $1 "+
			"WHERE id = $2 AND status = 'approved'",
		now, changeSetID,
	)
	if err != nil {
		return fmt.Errorf("change_set status transition to applied failed: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("change_set %d not found or not in approved state", changeSetID)
	}

	ctx.ExecutionResult = &ExecutionResult{
		AffectedRowIDs: []int{changeSetID},
		ChangeSetID:    changeSetID,
	}
	return nil
}

// --- helpers ---

// insertFieldChanges creates change_set_field_change rows from request params.
func insertFieldChanges(tx *pg.Tx, changeSetID int, params map[string]interface{}, now time.Time) ([]int, error) {
	rawChanges, ok := params["field_changes"]
	if !ok {
		return nil, nil
	}

	changeList, ok := rawChanges.([]interface{})
	if !ok {
		return nil, fmt.Errorf("field_changes must be an array")
	}

	var ids []int

	for order, item := range changeList {
		changeMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		entityType, _ := changeMap["entity_type"].(string)
		entityID, _ := toInt(changeMap["entity_id"])
		fieldName, _ := changeMap["field_name"].(string)
		beforeValue := changeMap["before_value"]
		afterValue := changeMap["after_value"]
		versionStamp, _ := toInt(changeMap["version_stamp"])

		var fieldChangeID int
		err := pg.QueryInTx(tx,
			"INSERT INTO change_set_field_change "+
				"(change_set_id, target_entity_type, target_entity_id, field_name, "+
				"before_value, after_value, version_stamp, apply_order, "+
				"applied_status, created_time, updated_time) "+
				"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9, $9) RETURNING id",
			changeSetID, entityType, entityID, fieldName,
			beforeValue, afterValue, versionStamp, order+1, now,
		).Scan(&fieldChangeID)
		if err != nil {
			return ids, fmt.Errorf("field_change insert for %s.%s failed: %w", entityType, fieldName, err)
		}

		ids = append(ids, fieldChangeID)
	}

	return ids, nil
}

// safeUserID returns the ops_user_id from an identity, or nil if not present.
func safeUserID(identity *auth.Identity) interface{} {
	if identity != nil && identity.OpsUserID != nil {
		return *identity.OpsUserID
	}
	return nil
}

// filterToColumns filters params to only include keys that are valid columns
// for the target table per the runtime schema.
func filterToColumns(ctx *GateContext, table string, params map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	for key, val := range params {
		// skip meta keys that aren't columns
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
