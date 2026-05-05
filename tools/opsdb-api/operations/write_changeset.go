//# tools/opsdb-api/operations/write_changeset.go

package operations

import (
	"fmt"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/tools/opsdb-api/concurrency"
)

// SubmitChangeSetParams holds change set submission parameters.
type SubmitChangeSetParams struct {
	SiteID       int
	Name         string
	Description  string
	Reason       string
	FieldChanges []FieldChange
	TicketRef    *int // authority_pointer_id
	IsEmergency  bool
	IsBulk       bool
	DryRun       bool
	ProposerUser *int // ops_user_id
	ProposerJob  *int // runner_job_id
}

// FieldChange represents one field change in a change set submission.
type FieldChange struct {
	EntityType   string
	EntityID     int
	FieldName    string
	BeforeValue  interface{}
	AfterValue   interface{}
	ChangeType   string // create, update, delete
	VersionStamp int
}

// ChangeSetResult holds the result of a change set operation.
type ChangeSetResult struct {
	ChangeSetID      int
	Status           string
	FieldChangeIDs   []int
	ApprovalRequired []ApprovalRequirementResult
	ValidationErrors []ValidationError
	DryRunResult     *DryRunResult
}

// ApprovalRequirementResult describes one computed approval requirement.
type ApprovalRequirementResult struct {
	RuleID        int
	GroupID       int
	GroupName     string
	CountRequired int
	AutoApproved  bool
}

// ValidationError describes one validation failure.
type ValidationError struct {
	EntityType string
	EntityID   int
	FieldName  string
	ErrorType  string // schema, bound, semantic, policy, lint, dependency
	Message    string
	Severity   string // error, warning
}

// DryRunResult holds the output of a dry-run submission.
type DryRunResult struct {
	WouldCreate       int
	WouldUpdate       int
	WouldRequireApproval []ApprovalRequirementResult
	ValidationErrors  []ValidationError
	ValidationWarnings []ValidationError
}

const (
	defaultBulkChunkSize = 1000
)

// SubmitChangeSet handles the submit_change_set operation. Creates the
// change set row, field change rows, runs validation, computes approval
// requirements, and transitions to the appropriate status.
func SubmitChangeSet(db *pg.DB, params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	if len(params.FieldChanges) == 0 {
		return nil, fmt.Errorf("change set must contain at least one field change")
	}

	// validate optimistic concurrency: check version stamps against current state
	stamps := buildVersionStamps(params.FieldChanges)
	if len(stamps) > 0 {
		err := concurrency.ValidateVersionStamps(stamps, db)
		if err != nil {
			if concurrency.IsStaleVersionError(err) {
				return &ChangeSetResult{
					Status:           "stale_version",
					ValidationErrors: staleVersionToErrors(err),
				}, err
			}
			return nil, fmt.Errorf("version stamp validation failed: %w", err)
		}
	}

	// run validation pipeline on field changes
	validationErrors, validationWarnings := validateFieldChanges(db, params.FieldChanges)

	// dry run: return validation and computed approvals without writing
	if params.DryRun {
		approvals := computeApprovalRequirements(db, params.FieldChanges)
		return &ChangeSetResult{
			Status:           "dry_run",
			ValidationErrors: validationErrors,
			DryRunResult: &DryRunResult{
				WouldCreate:          countByChangeType(params.FieldChanges, "create"),
				WouldUpdate:          countByChangeType(params.FieldChanges, "update"),
				WouldRequireApproval: approvals,
				ValidationErrors:     validationErrors,
				ValidationWarnings:   validationWarnings,
			},
		}, nil
	}

	// check for blocking validation errors
	blockingErrors := filterBySeverity(validationErrors, "error")
	if len(blockingErrors) > 0 {
		return &ChangeSetResult{
			Status:           "validation_failed",
			ValidationErrors: blockingErrors,
		}, fmt.Errorf("change set validation failed with %d error(s)", len(blockingErrors))
	}

	// write change set within a transaction
	result := &ChangeSetResult{}

	err := pg.WithTransaction(db, func(tx *pg.Tx) error {
		now := time.Now().UTC()

		// insert change_set row
		var changeSetID int
		err := pg.QueryInTx(tx,
			"INSERT INTO change_set "+
				"(site_id, name, description, reason, status, "+
				"is_bulk, is_emergency, submitted_by_ops_user_id, "+
				"submitted_time, ticket_authority_pointer_id, "+
				"created_time, updated_time) "+
				"VALUES ($1, $2, $3, $4, 'submitted', "+
				"$5, $6, $7, $8, $9, $8, $8) RETURNING id",
			params.SiteID, params.Name, params.Description, params.Reason,
			params.IsBulk, params.IsEmergency, params.ProposerUser,
			now, params.TicketRef,
		).Scan(&changeSetID)
		if err != nil {
			return fmt.Errorf("change_set insert failed: %w", err)
		}
		result.ChangeSetID = changeSetID

		// insert field change rows
		fieldChangeIDs, err := insertFieldChangeRows(tx, changeSetID, params.FieldChanges, now)
		if err != nil {
			return fmt.Errorf("field change insert failed: %w", err)
		}
		result.FieldChangeIDs = fieldChangeIDs

		// compute and write approval requirements
		approvals := computeApprovalRequirements(db, params.FieldChanges)
		result.ApprovalRequired = approvals

		allAutoApproved := true
		for _, approval := range approvals {
			_, err := pg.ExecInTx(tx,
				"INSERT INTO change_set_approval_required "+
					"(change_set_id, approval_rule_id, required_group_id, "+
					"required_count, fulfilled_count, is_fulfilled, "+
					"created_time, updated_time) "+
					"VALUES ($1, $2, $3, $4, 0, false, $5, $5)",
				changeSetID, approval.RuleID, approval.GroupID,
				approval.CountRequired, now,
			)
			if err != nil {
				return fmt.Errorf("approval_required insert failed: %w", err)
			}
			if !approval.AutoApproved {
				allAutoApproved = false
			}
		}

		// determine status based on approvals
		if len(approvals) == 0 || allAutoApproved {
			// no approval required or all auto-approvable: transition to approved
			_, err = pg.ExecInTx(tx,
				"UPDATE change_set SET status = 'approved', updated_time = $1 WHERE id = $2",
				now, changeSetID,
			)
			if err != nil {
				return fmt.Errorf("auto-approve transition failed: %w", err)
			}

			// if auto-approved, mark all approval requirements as fulfilled
			_, err = pg.ExecInTx(tx,
				"UPDATE change_set_approval_required "+
					"SET is_fulfilled = true, fulfilled_count = required_count, updated_time = $1 "+
					"WHERE change_set_id = $2",
				now, changeSetID,
			)
			if err != nil {
				return fmt.Errorf("auto-approve fulfillment update failed: %w", err)
			}

			result.Status = "approved"
		} else {
			// needs human approval
			_, err = pg.ExecInTx(tx,
				"UPDATE change_set SET status = 'pending_approval', updated_time = $1 WHERE id = $2",
				now, changeSetID,
			)
			if err != nil {
				return fmt.Errorf("pending_approval transition failed: %w", err)
			}
			result.Status = "pending_approval"
		}

		return nil
	})

	if err != nil {
		return result, err
	}

	return result, nil
}

// EmergencyApply handles the emergency_apply operation. Same as submit
// but with is_emergency=true, reduced approvals, and a mandatory
// post-hoc emergency review.
func EmergencyApply(db *pg.DB, params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	params.IsEmergency = true

	if len(params.FieldChanges) == 0 {
		return nil, fmt.Errorf("emergency change set must contain at least one field change")
	}

	// validate version stamps
	stamps := buildVersionStamps(params.FieldChanges)
	if len(stamps) > 0 {
		err := concurrency.ValidateVersionStamps(stamps, db)
		if err != nil {
			if concurrency.IsStaleVersionError(err) {
				return &ChangeSetResult{
					Status:           "stale_version",
					ValidationErrors: staleVersionToErrors(err),
				}, err
			}
			return nil, fmt.Errorf("version stamp validation failed: %w", err)
		}
	}

	result := &ChangeSetResult{}

	err := pg.WithTransaction(db, func(tx *pg.Tx) error {
		now := time.Now().UTC()

		// insert change_set with emergency flag and immediate approved status
		var changeSetID int
		err := pg.QueryInTx(tx,
			"INSERT INTO change_set "+
				"(site_id, name, description, reason, status, "+
				"is_bulk, is_emergency, submitted_by_ops_user_id, "+
				"submitted_time, ticket_authority_pointer_id, "+
				"created_time, updated_time) "+
				"VALUES ($1, $2, $3, $4, 'approved', "+
				"false, true, $5, $6, $7, $6, $6) RETURNING id",
			params.SiteID, params.Name, params.Description, params.Reason,
			params.ProposerUser, now, params.TicketRef,
		).Scan(&changeSetID)
		if err != nil {
			return fmt.Errorf("emergency change_set insert failed: %w", err)
		}
		result.ChangeSetID = changeSetID

		// insert field changes
		fieldChangeIDs, err := insertFieldChangeRows(tx, changeSetID, params.FieldChanges, now)
		if err != nil {
			return fmt.Errorf("emergency field change insert failed: %w", err)
		}
		result.FieldChangeIDs = fieldChangeIDs

		// create emergency review row — must be reviewed within 72 hours
		reviewDeadline := now.Add(72 * time.Hour)
		_, err = pg.ExecInTx(tx,
			"INSERT INTO change_set_emergency_review "+
				"(change_set_id, status, review_deadline_time, created_time, updated_time) "+
				"VALUES ($1, 'pending', $2, $3, $3)",
			changeSetID, reviewDeadline, now,
		)
		if err != nil {
			return fmt.Errorf("emergency_review insert failed: %w", err)
		}

		result.Status = "approved"
		return nil
	})

	if err != nil {
		return result, err
	}

	return result, nil
}

// BulkSubmit handles the bulk_submit_change_set operation. Validates
// field changes in chunks and writes atomically.
func BulkSubmit(db *pg.DB, params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	params.IsBulk = true

	if len(params.FieldChanges) == 0 {
		return nil, fmt.Errorf("bulk change set must contain at least one field change")
	}

	// validate version stamps for all changes
	stamps := buildVersionStamps(params.FieldChanges)
	if len(stamps) > 0 {
		err := concurrency.ValidateVersionStamps(stamps, db)
		if err != nil {
			if concurrency.IsStaleVersionError(err) {
				return &ChangeSetResult{
					Status:           "stale_version",
					ValidationErrors: staleVersionToErrors(err),
				}, err
			}
			return nil, fmt.Errorf("version stamp validation failed: %w", err)
		}
	}

	// validate in chunks with interim feedback
	chunkSize := defaultBulkChunkSize
	var allErrors []ValidationError
	var allWarnings []ValidationError

	for i := 0; i < len(params.FieldChanges); i += chunkSize {
		end := i + chunkSize
		if end > len(params.FieldChanges) {
			end = len(params.FieldChanges)
		}
		chunk := params.FieldChanges[i:end]

		chunkErrors, chunkWarnings := validateFieldChanges(db, chunk)
		allErrors = append(allErrors, chunkErrors...)
		allWarnings = append(allWarnings, chunkWarnings...)

		// any chunk failure means the entire change set fails
		blockingErrors := filterBySeverity(chunkErrors, "error")
		if len(blockingErrors) > 0 {
			return &ChangeSetResult{
				Status:           "validation_failed",
				ValidationErrors: allErrors,
			}, fmt.Errorf("bulk validation failed at chunk starting at index %d: %d error(s)",
				i, len(blockingErrors))
		}
	}

	if params.DryRun {
		approvals := computeApprovalRequirements(db, params.FieldChanges)
		return &ChangeSetResult{
			Status: "dry_run",
			DryRunResult: &DryRunResult{
				WouldCreate:          countByChangeType(params.FieldChanges, "create"),
				WouldUpdate:          countByChangeType(params.FieldChanges, "update"),
				WouldRequireApproval: approvals,
				ValidationErrors:     allErrors,
				ValidationWarnings:   allWarnings,
			},
		}, nil
	}

	// write all atomically — same as regular submit from here
	return SubmitChangeSet(db, params)
}

// --- internal helpers ---

// buildVersionStamps converts field changes to version stamp structs
// for optimistic concurrency checking.
func buildVersionStamps(changes []FieldChange) []concurrency.FieldChangeStamp {
	var stamps []concurrency.FieldChangeStamp
	for _, fc := range changes {
		if fc.VersionStamp > 0 && fc.EntityID > 0 {
			stamps = append(stamps, concurrency.FieldChangeStamp{
				EntityType:   fc.EntityType,
				EntityID:     fc.EntityID,
				VersionStamp: fc.VersionStamp,
			})
		}
	}
	return stamps
}

// insertFieldChangeRows creates change_set_field_change rows within a transaction.
func insertFieldChangeRows(tx *pg.Tx, changeSetID int, changes []FieldChange, now time.Time) ([]int, error) {
	ids := make([]int, 0, len(changes))

	for order, fc := range changes {
		var fieldChangeID int
		err := pg.QueryInTx(tx,
			"INSERT INTO change_set_field_change "+
				"(change_set_id, target_entity_type, target_entity_id, "+
				"field_name, before_value, after_value, "+
				"change_type, version_stamp, apply_order, "+
				"applied_status, created_time, updated_time) "+
				"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', $10, $10) RETURNING id",
			changeSetID, fc.EntityType, fc.EntityID,
			fc.FieldName, fc.BeforeValue, fc.AfterValue,
			fc.ChangeType, fc.VersionStamp, order+1,
			now,
		).Scan(&fieldChangeID)
		if err != nil {
			return ids, fmt.Errorf("field change insert failed for %s.%s on %s id=%d: %w",
				fc.EntityType, fc.FieldName, fc.EntityType, fc.EntityID, err)
		}
		ids = append(ids, fieldChangeID)
	}

	return ids, nil
}

// validateFieldChanges runs the validation pipeline on a set of field changes.
// Returns blocking errors and non-blocking warnings separately.
func validateFieldChanges(db *pg.DB, changes []FieldChange) ([]ValidationError, []ValidationError) {
	var errors, warnings []ValidationError

	for _, fc := range changes {
		// schema validation: entity type and field exist
		if fc.EntityType == "" {
			errors = append(errors, ValidationError{
				FieldName: fc.FieldName,
				ErrorType: "schema",
				Message:   "entity_type is required",
				Severity:  "error",
			})
			continue
		}
		if fc.FieldName == "" {
			errors = append(errors, ValidationError{
				EntityType: fc.EntityType,
				EntityID:   fc.EntityID,
				ErrorType:  "schema",
				Message:    "field_name is required",
				Severity:   "error",
			})
			continue
		}

		// check entity type exists
		var entityExists bool
		err := db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM _schema_entity_type WHERE table_name = $1 AND is_active = true)",
			fc.EntityType,
		).Scan(&entityExists)
		if err != nil || !entityExists {
			errors = append(errors, ValidationError{
				EntityType: fc.EntityType,
				FieldName:  fc.FieldName,
				ErrorType:  "schema",
				Message:    fmt.Sprintf("unknown entity type: %s", fc.EntityType),
				Severity:   "error",
			})
			continue
		}

		// check field exists
		var fieldExists bool
		err = db.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM _schema_field sf "+
				"JOIN _schema_entity_type et ON et.id = sf.entity_type_id "+
				"WHERE et.table_name = $1 AND sf.field_name = $2 AND sf.is_active = true)",
			fc.EntityType, fc.FieldName,
		).Scan(&fieldExists)
		if err != nil || !fieldExists {
			errors = append(errors, ValidationError{
				EntityType: fc.EntityType,
				EntityID:   fc.EntityID,
				FieldName:  fc.FieldName,
				ErrorType:  "schema",
				Message:    fmt.Sprintf("unknown field %s on entity %s", fc.FieldName, fc.EntityType),
				Severity:   "error",
			})
			continue
		}

		// for updates: verify entity exists
		if fc.ChangeType == "update" && fc.EntityID > 0 {
			var rowExists bool
			err = db.QueryRow(
				fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)",
					pg.QuoteIdentifier(fc.EntityType)),
				fc.EntityID,
			).Scan(&rowExists)
			if err != nil || !rowExists {
				errors = append(errors, ValidationError{
					EntityType: fc.EntityType,
					EntityID:   fc.EntityID,
					FieldName:  fc.FieldName,
					ErrorType:  "schema",
					Message:    fmt.Sprintf("%s with id=%d not found", fc.EntityType, fc.EntityID),
					Severity:   "error",
				})
			}
		}
	}

	return errors, warnings
}

// computeApprovalRequirements determines what approvals are needed for
// the given field changes based on active approval rules.
func computeApprovalRequirements(db *pg.DB, changes []FieldChange) []ApprovalRequirementResult {
	// collect unique entity types and fields
	entityTypes := make(map[string]bool)
	fieldNames := make(map[string]bool)
	for _, fc := range changes {
		entityTypes[fc.EntityType] = true
		fieldNames[fc.FieldName] = true
	}

	rows, err := db.Query(
		"SELECT p.id, p.name, p.policy_data_json FROM policy p " +
			"WHERE p.policy_type = 'approval_rule' AND p.is_active = true",
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var approvals []ApprovalRequirementResult

	for rows.Next() {
		var ruleID int
		var ruleName string
		var dataJSON []byte
		if err := rows.Scan(&ruleID, &ruleName, &dataJSON); err != nil {
			continue
		}

		var data map[string]interface{}
		if err := pg.UnmarshalJSON(dataJSON, &data); err != nil {
			continue
		}

		// check if rule targets any of our entity types
		targetTypes := extractStringListFromMap(data, "target_entity_types")
		if len(targetTypes) > 0 {
			matched := false
			for _, t := range targetTypes {
				if t == "all" || entityTypes[t] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// check if rule targets any of our fields
		targetFields := extractStringListFromMap(data, "target_fields")
		if len(targetFields) > 0 {
			matched := false
			for _, f := range targetFields {
				if f == "all" || fieldNames[f] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// matched: extract approval requirements
		groupName, _ := data["required_group"].(string)
		requiredCount := 1
		if rc, ok := data["required_count"].(float64); ok {
			requiredCount = int(rc)
		}
		autoApprovable := false
		if aa, ok := data["auto_approvable"].(bool); ok {
			autoApprovable = aa
		}

		groupID := resolveGroupIDByName(db, groupName)

		approvals = append(approvals, ApprovalRequirementResult{
			RuleID:        ruleID,
			GroupID:       groupID,
			GroupName:     groupName,
			CountRequired: requiredCount,
			AutoApproved:  autoApprovable,
		})
	}

	return approvals
}

// staleVersionToErrors converts a stale version error to validation errors.
func staleVersionToErrors(err error) []ValidationError {
	staleEntities := concurrency.GetStaleEntities(err)
	if staleEntities == nil {
		return []ValidationError{{
			ErrorType: "stale_version",
			Message:   err.Error(),
			Severity:  "error",
		}}
	}

	errors := make([]ValidationError, 0, len(staleEntities))
	for _, s := range staleEntities {
		errors = append(errors, ValidationError{
			EntityType: s.EntityType,
			EntityID:   s.EntityID,
			ErrorType:  "stale_version",
			Message: fmt.Sprintf("entity was at version %d when drafted but is now at version %d",
				s.DraftedVersion, s.CurrentVersion),
			Severity: "error",
		})
	}
	return errors
}

// countByChangeType counts field changes with a specific change type.
func countByChangeType(changes []FieldChange, changeType string) int {
	count := 0
	for _, fc := range changes {
		if fc.ChangeType == changeType {
			count++
		}
	}
	return count
}

// filterBySeverity returns only validation errors matching the given severity.
func filterBySeverity(errors []ValidationError, severity string) []ValidationError {
	var filtered []ValidationError
	for _, e := range errors {
		if e.Severity == severity {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// extractStringListFromMap extracts a string list from a map value.
func extractStringListFromMap(data map[string]interface{}, key string) []string {
	val, ok := data[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// resolveGroupIDByName looks up an ops_group ID by name.
func resolveGroupIDByName(db *pg.DB, name string) int {
	if name == "" {
		return 0
	}
	var groupID int
	err := db.QueryRow(
		"SELECT id FROM ops_group WHERE name = $1 AND is_active = true",
		name,
	).Scan(&groupID)
	if err != nil {
		return 0
	}
	return groupID
}