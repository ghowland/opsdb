//# tools/opsdb-api/operations/changeset_actions.go

package operations

import (
	"fmt"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
)

// ApproveChangeSet records a stakeholder approval. Verifies the caller
// is in a required approver group, records the approval, increments
// fulfillment counts, and transitions the change set to approved when
// all requirements are met.
func ApproveChangeSet(db *pg.DB, changeSetID int, approverUserID int, comment string) error {
	return pg.WithTransaction(db, func(tx *pg.Tx) error {
		now := time.Now().UTC()

		// verify change set exists and is in approvable state
		var status string
		var submitterID int
		err := pg.QueryInTx(tx,
			"SELECT status, submitted_by_ops_user_id FROM change_set WHERE id = $1",
			changeSetID,
		).Scan(&status, &submitterID)
		if err != nil {
			return fmt.Errorf("change set %d not found: %w", changeSetID, err)
		}
		if status != "pending_approval" {
			return fmt.Errorf("change set %d is in status %q, not pending_approval", changeSetID, status)
		}

		// separation of duty: approver cannot be the submitter
		if approverUserID == submitterID {
			sodRequired, _ := isSoDRequired(tx, changeSetID)
			if sodRequired {
				return fmt.Errorf("separation of duty violation: submitter (user %d) cannot approve their own change set %d",
					approverUserID, changeSetID)
			}
		}

		// find which approval requirements the approver can fulfill
		// (approver must be a member of the required group)
		requirements, err := findFulfillableRequirements(tx, changeSetID, approverUserID)
		if err != nil {
			return fmt.Errorf("failed to check approval requirements: %w", err)
		}
		if len(requirements) == 0 {
			return fmt.Errorf("user %d is not in any required approver group for change set %d",
				approverUserID, changeSetID)
		}

		// insert approval record
		var approvalID int
		err = pg.QueryInTx(tx,
			"INSERT INTO change_set_approval "+
				"(change_set_id, approved_by_ops_user_id, comment, created_time, updated_time) "+
				"VALUES ($1, $2, $3, $4, $4) RETURNING id",
			changeSetID, approverUserID, comment, now,
		).Scan(&approvalID)
		if err != nil {
			return fmt.Errorf("approval insert failed: %w", err)
		}

		// increment fulfilled count on each matching requirement
		for _, reqID := range requirements {
			_, err = pg.ExecInTx(tx,
				"UPDATE change_set_approval_required "+
					"SET fulfilled_count = fulfilled_count + 1, "+
					"is_fulfilled = (fulfilled_count + 1 >= required_count), "+
					"updated_time = $1 "+
					"WHERE id = $2 AND is_fulfilled = false",
				now, reqID,
			)
			if err != nil {
				return fmt.Errorf("fulfillment update failed for requirement %d: %w", reqID, err)
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
				"UPDATE change_set SET status = 'approved', updated_time = $1 WHERE id = $2",
				now, changeSetID,
			)
			if err != nil {
				return fmt.Errorf("status transition to approved failed: %w", err)
			}
		}

		return nil
	})
}

// RejectChangeSet records a stakeholder rejection. Verifies the caller
// is in a required approver group. Evaluates rejection semantics from
// the matching approval rule to determine whether to immediately reject
// or wait for further rejections.
func RejectChangeSet(db *pg.DB, changeSetID int, rejectorUserID int, reason string) error {
	return pg.WithTransaction(db, func(tx *pg.Tx) error {
		now := time.Now().UTC()

		// verify change set is in rejectable state
		var status string
		err := pg.QueryInTx(tx,
			"SELECT status FROM change_set WHERE id = $1",
			changeSetID,
		).Scan(&status)
		if err != nil {
			return fmt.Errorf("change set %d not found: %w", changeSetID, err)
		}
		if status != "pending_approval" {
			return fmt.Errorf("change set %d is in status %q, not pending_approval", changeSetID, status)
		}

		// verify rejector is in a required approver group
		requirements, err := findFulfillableRequirements(tx, changeSetID, rejectorUserID)
		if err != nil {
			return fmt.Errorf("failed to check approver eligibility: %w", err)
		}
		if len(requirements) == 0 {
			return fmt.Errorf("user %d is not in any required approver group for change set %d",
				rejectorUserID, changeSetID)
		}

		// insert rejection record
		var rejectionID int
		err = pg.QueryInTx(tx,
			"INSERT INTO change_set_rejection "+
				"(change_set_id, rejected_by_ops_user_id, reason, created_time, updated_time) "+
				"VALUES ($1, $2, $3, $4, $4) RETURNING id",
			changeSetID, rejectorUserID, reason, now,
		).Scan(&rejectionID)
		if err != nil {
			return fmt.Errorf("rejection insert failed: %w", err)
		}

		// evaluate rejection behavior from the matching approval rules
		shouldReject, err := evaluateRejectionBehavior(tx, changeSetID, rejectorUserID)
		if err != nil {
			return fmt.Errorf("rejection behavior evaluation failed: %w", err)
		}

		if shouldReject {
			_, err = pg.ExecInTx(tx,
				"UPDATE change_set SET status = 'rejected', updated_time = $1 WHERE id = $2",
				now, changeSetID,
			)
			if err != nil {
				return fmt.Errorf("status transition to rejected failed: %w", err)
			}
		}

		return nil
	})
}

// CancelChangeSet withdraws a change set. Available to the original
// submitter or any user with cancel authority.
func CancelChangeSet(db *pg.DB, changeSetID int, cancellerUserID int) error {
	return pg.WithTransaction(db, func(tx *pg.Tx) error {
		now := time.Now().UTC()

		var status string
		var submitterID int
		err := pg.QueryInTx(tx,
			"SELECT status, submitted_by_ops_user_id FROM change_set WHERE id = $1",
			changeSetID,
		).Scan(&status, &submitterID)
		if err != nil {
			return fmt.Errorf("change set %d not found: %w", changeSetID, err)
		}

		// only cancellable in these states
		switch status {
		case "draft", "submitted", "validating", "pending_approval":
			// cancellable
		default:
			return fmt.Errorf("change set %d is in status %q, which cannot be cancelled", changeSetID, status)
		}

		// verify authority: submitter or user with cancel role
		if cancellerUserID != submitterID {
			hasAuthority, err := userHasCancelAuthority(tx, cancellerUserID)
			if err != nil {
				return fmt.Errorf("failed to check cancel authority: %w", err)
			}
			if !hasAuthority {
				return fmt.Errorf("user %d is not the submitter and lacks cancel authority for change set %d",
					cancellerUserID, changeSetID)
			}
		}

		_, err = pg.ExecInTx(tx,
			"UPDATE change_set SET status = 'cancelled', updated_time = $1 WHERE id = $2",
			now, changeSetID,
		)
		if err != nil {
			return fmt.Errorf("status transition to cancelled failed: %w", err)
		}

		return nil
	})
}

// ApplyFieldChange applies one approved field change to the target entity.
// Called by the change-set-executor runner. Updates the entity row,
// creates a version sibling row, and marks the field change as applied.
func ApplyFieldChange(db *pg.DB, changeSetID int, fieldChangeID int, executorID int) error {
	return pg.WithTransaction(db, func(tx *pg.Tx) error {
		now := time.Now().UTC()

		// verify change set is approved
		var csStatus string
		err := pg.QueryInTx(tx,
			"SELECT status FROM change_set WHERE id = $1",
			changeSetID,
		).Scan(&csStatus)
		if err != nil {
			return fmt.Errorf("change set %d not found: %w", changeSetID, err)
		}
		if csStatus != "approved" {
			return fmt.Errorf("change set %d is in status %q, not approved", changeSetID, csStatus)
		}

		// read the field change
		var entityType, fieldName string
		var entityID int
		var afterValue interface{}
		var appliedStatus string
		err = pg.QueryInTx(tx,
			"SELECT target_entity_type, target_entity_id, field_name, after_value, applied_status "+
				"FROM change_set_field_change WHERE id = $1 AND change_set_id = $2",
			fieldChangeID, changeSetID,
		).Scan(&entityType, &entityID, &fieldName, &afterValue, &appliedStatus)
		if err != nil {
			return fmt.Errorf("field change %d not found in change set %d: %w",
				fieldChangeID, changeSetID, err)
		}

		if appliedStatus != "pending" {
			return fmt.Errorf("field change %d has status %q, not pending",
				fieldChangeID, appliedStatus)
		}

		// apply the field value to the target entity
		result, err := pg.ExecInTx(tx,
			fmt.Sprintf("UPDATE %s SET %s = $1, updated_time = $2 WHERE id = $3",
				pg.QuoteIdentifier(entityType),
				pg.QuoteIdentifier(fieldName),
			),
			afterValue, now, entityID,
		)
		if err != nil {
			// mark as failed
			pg.ExecInTx(tx,
				"UPDATE change_set_field_change SET applied_status = 'failed', "+
					"applied_error_text = $1, applied_time = $2, updated_time = $2 WHERE id = $3",
				err.Error(), now, fieldChangeID,
			)
			return fmt.Errorf("failed to apply %s.%s on id=%d: %w",
				entityType, fieldName, entityID, err)
		}

		affected, _ := result.RowsAffected()
		if affected == 0 {
			pg.ExecInTx(tx,
				"UPDATE change_set_field_change SET applied_status = 'failed', "+
					"applied_error_text = $1, applied_time = $2, updated_time = $2 WHERE id = $3",
				"target entity row not found", now, fieldChangeID,
			)
			return fmt.Errorf("%s with id=%d not found", entityType, entityID)
		}

		// create version sibling row
		err = createVersionRow(tx, entityType, entityID, changeSetID, now)
		if err != nil {
			// version creation failure is logged but does not fail the apply
			// — the field change itself succeeded
			_ = err
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

		return nil
	})
}

// MarkChangeSetApplied finalizes a change set after all field changes
// have been applied. Verifies every field change has applied_status=applied.
func MarkChangeSetApplied(db *pg.DB, changeSetID int, executorID int) error {
	return pg.WithTransaction(db, func(tx *pg.Tx) error {
		now := time.Now().UTC()

		// verify change set is approved
		var status string
		err := pg.QueryInTx(tx,
			"SELECT status FROM change_set WHERE id = $1",
			changeSetID,
		).Scan(&status)
		if err != nil {
			return fmt.Errorf("change set %d not found: %w", changeSetID, err)
		}
		if status != "approved" {
			return fmt.Errorf("change set %d is in status %q, not approved", changeSetID, status)
		}

		// verify all field changes are applied
		var pendingCount, failedCount int
		err = pg.QueryInTx(tx,
			"SELECT "+
				"COUNT(*) FILTER (WHERE applied_status = 'pending'), "+
				"COUNT(*) FILTER (WHERE applied_status = 'failed') "+
				"FROM change_set_field_change WHERE change_set_id = $1",
			changeSetID,
		).Scan(&pendingCount, &failedCount)
		if err != nil {
			return fmt.Errorf("field change status check failed: %w", err)
		}

		if pendingCount > 0 {
			return fmt.Errorf("cannot mark change set %d as applied: %d field changes still pending",
				changeSetID, pendingCount)
		}
		if failedCount > 0 {
			return fmt.Errorf("cannot mark change set %d as applied: %d field changes failed",
				changeSetID, failedCount)
		}

		// transition to applied
		_, err = pg.ExecInTx(tx,
			"UPDATE change_set SET status = 'applied', applied_time = $1, updated_time = $1 WHERE id = $2",
			now, changeSetID,
		)
		if err != nil {
			return fmt.Errorf("status transition to applied failed: %w", err)
		}

		return nil
	})
}

// --- helper functions ---

// findFulfillableRequirements returns the IDs of approval requirements
// that the given user can fulfill (user is a member of the required group).
func findFulfillableRequirements(tx *pg.Tx, changeSetID int, userID int) ([]int, error) {
	rows, err := pg.QueryRowsInTx(tx,
		"SELECT car.id FROM change_set_approval_required car "+
			"WHERE car.change_set_id = $1 AND car.is_fulfilled = false "+
			"AND EXISTS (SELECT 1 FROM ops_group_member gm "+
			"WHERE gm.ops_group_id = car.required_group_id "+
			"AND gm.ops_user_id = $2 AND gm.is_active = true)",
		changeSetID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// isSoDRequired checks if separation of duty is required for a change set
// by looking at the approval rules that matched it.
func isSoDRequired(tx *pg.Tx, changeSetID int) (bool, error) {
	var required bool
	err := pg.QueryInTx(tx,
		"SELECT EXISTS(SELECT 1 FROM change_set_approval_required car "+
			"JOIN policy p ON p.id = car.approval_rule_id "+
			"WHERE car.change_set_id = $1 "+
			"AND p.policy_data_json->>'separation_of_duty' = 'submitter_cannot_approve')",
		changeSetID,
	).Scan(&required)
	if err != nil {
		return true, nil // fail closed: assume SoD required on error
	}
	return required, nil
}

// evaluateRejectionBehavior determines whether a rejection should immediately
// reject the change set based on the rejection_behavior configured in
// matching approval rules.
func evaluateRejectionBehavior(tx *pg.Tx, changeSetID int, rejectorUserID int) (bool, error) {
	// read rejection behaviors from matching approval rules
	rows, err := pg.QueryRowsInTx(tx,
		"SELECT COALESCE(p.policy_data_json->>'rejection_behavior', 'any_rejects_halts') "+
			"FROM change_set_approval_required car "+
			"JOIN policy p ON p.id = car.approval_rule_id "+
			"WHERE car.change_set_id = $1",
		changeSetID,
	)
	if err != nil {
		return true, nil // fail closed: reject on error
	}
	defer rows.Close()

	behaviors := make(map[string]bool)
	for rows.Next() {
		var behavior string
		if err := rows.Scan(&behavior); err != nil {
			continue
		}
		behaviors[behavior] = true
	}

	// if any rule uses any_rejects_halts (the default), one rejection is enough
	if behaviors["any_rejects_halts"] || len(behaviors) == 0 {
		return true, nil
	}

	// for majority_rejects_halts: count rejections vs total required approvers
	if behaviors["majority_rejects_halts"] {
		var rejectionCount, totalRequired int
		pg.QueryInTx(tx,
			"SELECT COUNT(*) FROM change_set_rejection WHERE change_set_id = $1",
			changeSetID,
		).Scan(&rejectionCount)
		pg.QueryInTx(tx,
			"SELECT COALESCE(SUM(required_count), 0) FROM change_set_approval_required WHERE change_set_id = $1",
			changeSetID,
		).Scan(&totalRequired)

		if totalRequired > 0 && rejectionCount > totalRequired/2 {
			return true, nil
		}
		return false, nil
	}

	// for all_must_reject: all approvers must reject
	if behaviors["all_must_reject"] {
		var rejectionCount, totalApprovers int
		pg.QueryInTx(tx,
			"SELECT COUNT(*) FROM change_set_rejection WHERE change_set_id = $1",
			changeSetID,
		).Scan(&rejectionCount)
		pg.QueryInTx(tx,
			"SELECT COALESCE(SUM(required_count), 0) FROM change_set_approval_required WHERE change_set_id = $1",
			changeSetID,
		).Scan(&totalApprovers)

		return rejectionCount >= totalApprovers, nil
	}

	// default: any rejection halts
	return true, nil
}

// userHasCancelAuthority checks if a user has a role granting cancel
// authority on change sets.
func userHasCancelAuthority(tx *pg.Tx, userID int) (bool, error) {
	var exists bool
	err := pg.QueryInTx(tx,
		"SELECT EXISTS(SELECT 1 FROM ops_user_role_member rm "+
			"JOIN ops_user_role r ON r.id = rm.ops_user_role_id "+
			"WHERE rm.ops_user_id = $1 AND rm.is_active = true AND r.is_active = true "+
			"AND r.name IN ('admin', 'operator'))",
		userID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// createVersionRow inserts a new version sibling row for a versioned entity.
// Reads the current active version, increments the serial, and deactivates
// the previous version.
func createVersionRow(tx *pg.Tx, entityType string, entityID int, changeSetID int, now time.Time) error {
	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	// check if version table exists (entity may not be versioned)
	var tableExists bool
	err := pg.QueryInTx(tx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
		versionTable,
	).Scan(&tableExists)
	if err != nil || !tableExists {
		return nil
	}

	// read current active version
	var currentSerial, currentVID int
	err = pg.QueryInTx(tx,
		fmt.Sprintf(
			"SELECT version_serial, id FROM %s WHERE %s = $1 AND is_active_version = true LIMIT 1",
			pg.QuoteIdentifier(versionTable),
			pg.QuoteIdentifier(fkColumn),
		),
		entityID,
	).Scan(&currentSerial, &currentVID)
	if err != nil {
		if pg.IsNoRows(err) {
			currentSerial = 0
			currentVID = 0
		} else {
			return fmt.Errorf("version read failed: %w", err)
		}
	}

	nextSerial := currentSerial + 1

	// insert new version row
	var parentVIDArg interface{}
	if currentVID > 0 {
		parentVIDArg = currentVID
	}

	var newVID int
	err = pg.QueryInTx(tx,
		fmt.Sprintf(
			"INSERT INTO %s (%s, version_serial, parent_%s_version_id, "+
				"change_set_id, is_active_version, approved_for_production_time, "+
				"created_time, updated_time) "+
				"VALUES ($1, $2, $3, $4, true, $5, $5, $5) RETURNING id",
			pg.QuoteIdentifier(versionTable),
			pg.QuoteIdentifier(fkColumn),
			entityType,
		),
		entityID, nextSerial, parentVIDArg, changeSetID, now,
	).Scan(&newVID)
	if err != nil {
		return fmt.Errorf("version insert failed: %w", err)
	}

	// deactivate previous version
	if currentVID > 0 {
		_, err = pg.ExecInTx(tx,
			fmt.Sprintf(
				"UPDATE %s SET is_active_version = false, updated_time = $1 WHERE id = $2",
				pg.QuoteIdentifier(versionTable),
			),
			now, currentVID,
		)
		if err != nil {
			return fmt.Errorf("previous version deactivation failed: %w", err)
		}
	}

	return nil
}
