package executor

import (
	"fmt"
	"sort"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

// ChangeSetBatch holds the data read during the get phase for one cycle.
type ChangeSetBatch struct {
	ChangeSets []ChangeSetWork
	TotalFound int
}

// ChangeSetWork holds one approved change set and its field changes.
type ChangeSetWork struct {
	ChangeSetID   int
	Name          string
	SubmittedTime interface{}
	IsEmergency   bool
	IsBulk        bool
	FieldChanges  []FieldChangeWork
}

// FieldChangeWork holds one field change to apply.
type FieldChangeWork struct {
	FieldChangeID int
	EntityType    string
	EntityID      int
	FieldName     string
	AfterValue    interface{}
	ApplyOrder    int
	AppliedStatus string
}

// CycleSummary holds the results of one executor cycle.
type CycleSummary struct {
	ChangeSetsProcessed    int
	ChangeSetsFullyApplied int
	ChangeSetsFailed       int
	FieldChangesApplied    int
	FieldChangesFailed     int
	Errors                 []string
}

// GetApprovedChangeSets reads approved change sets from OpsDB.
// Returns them ordered by submitted_time ascending (oldest first).
func GetApprovedChangeSets(client *runner.APIClient, batchSize int, retryFailed bool) (*ChangeSetBatch, error) {
	filters := []runner.SearchFilter{
		{Field: "status", Operator: "eq", Value: "approved"},
	}

	result, err := client.Search("change_set", filters,
		[]runner.OrderSpec{{Field: "submitted_time", Direction: "asc"}},
		batchSize, "")
	if err != nil {
		return nil, fmt.Errorf("searching approved change sets: %w", err)
	}

	batch := &ChangeSetBatch{
		TotalFound: result.TotalCount,
	}

	for _, row := range result.Rows {
		csID, _ := extractInt(row, "id")
		name, _ := row["name"].(string)
		isEmergency, _ := row["is_emergency"].(bool)
		isBulk, _ := row["is_bulk"].(bool)
		submittedTime := row["submitted_time"]

		cs := ChangeSetWork{
			ChangeSetID:   csID,
			Name:          name,
			SubmittedTime: submittedTime,
			IsEmergency:   isEmergency,
			IsBulk:        isBulk,
		}

		// Load field changes for this change set.
		fcFilters := []runner.SearchFilter{
			{Field: "change_set_id", Operator: "eq", Value: csID},
		}

		// Unless retrying failed, only load pending field changes.
		if !retryFailed {
			fcFilters = append(fcFilters, runner.SearchFilter{
				Field: "applied_status", Operator: "eq", Value: "pending",
			})
		} else {
			// Load both pending and failed for retry.
			fcFilters = append(fcFilters, runner.SearchFilter{
				Field: "applied_status", Operator: "in", Value: []string{"pending", "failed"},
			})
		}

		fcResult, err := client.Search("change_set_field_change", fcFilters,
			[]runner.OrderSpec{{Field: "apply_order", Direction: "asc"}},
			1000, "")
		if err != nil {
			return nil, fmt.Errorf("loading field changes for change_set %d: %w", csID, err)
		}

		for _, fcRow := range fcResult.Rows {
			fcID, _ := extractInt(fcRow, "id")
			entityType, _ := fcRow["target_entity_type"].(string)
			entityID, _ := extractInt(fcRow, "target_entity_id")
			fieldName, _ := fcRow["field_name"].(string)
			afterValue := fcRow["after_value"]
			applyOrder, _ := extractInt(fcRow, "apply_order")
			appliedStatus, _ := fcRow["applied_status"].(string)

			cs.FieldChanges = append(cs.FieldChanges, FieldChangeWork{
				FieldChangeID: fcID,
				EntityType:    entityType,
				EntityID:      entityID,
				FieldName:     fieldName,
				AfterValue:    afterValue,
				ApplyOrder:    applyOrder,
				AppliedStatus: appliedStatus,
			})
		}

		// Ensure field changes are sorted by apply order.
		sort.Slice(cs.FieldChanges, func(i, j int) bool {
			return cs.FieldChanges[i].ApplyOrder < cs.FieldChanges[j].ApplyOrder
		})

		batch.ChangeSets = append(batch.ChangeSets, cs)
	}

	return batch, nil
}

// ApplyChangeSet processes one approved change set by applying each field
// change in order via the API. Stops on first failure within a change set.
// Returns the count of successfully applied field changes and any error.
func ApplyChangeSet(client *runner.APIClient, cs *ChangeSetWork) (int, error) {
	appliedCount := 0

	for _, fc := range cs.FieldChanges {
		if fc.AppliedStatus != "pending" && fc.AppliedStatus != "failed" {
			continue // already applied
		}

		err := client.ApplyFieldChange(cs.ChangeSetID, fc.FieldChangeID)
		if err != nil {
			return appliedCount, fmt.Errorf("field_change %d (%s.%s on %s/%d): %w",
				fc.FieldChangeID, fc.EntityType, fc.FieldName, fc.EntityType, fc.EntityID, err)
		}

		appliedCount++
	}

	return appliedCount, nil
}

// FinalizeChangeSet marks a change set as fully applied after all its
// field changes have been successfully applied.
func FinalizeChangeSet(client *runner.APIClient, changeSetID int) error {
	err := client.MarkChangeSetApplied(changeSetID)
	if err != nil {
		return fmt.Errorf("finalizing change_set %d: %w", changeSetID, err)
	}
	return nil
}

// ProcessCycle runs one complete get/act/set cycle for the executor.
// Reads approved change sets, applies field changes, finalizes, reports.
func ProcessCycle(client *runner.APIClient, batchSize int, fieldChangeBatchSize int, retryFailed bool, dryRun bool) (*CycleSummary, error) {
	summary := &CycleSummary{}

	// GET phase.
	batch, err := GetApprovedChangeSets(client, batchSize, retryFailed)
	if err != nil {
		return nil, fmt.Errorf("get phase: %w", err)
	}

	if dryRun {
		summary.ChangeSetsProcessed = len(batch.ChangeSets)
		return summary, nil
	}

	// ACT phase.
	for i := range batch.ChangeSets {
		cs := &batch.ChangeSets[i]
		summary.ChangeSetsProcessed++

		applied, applyErr := ApplyChangeSet(client, cs)
		summary.FieldChangesApplied += applied

		if applyErr != nil {
			summary.ChangeSetsFailed++
			summary.FieldChangesFailed += len(cs.FieldChanges) - applied
			summary.Errors = append(summary.Errors, applyErr.Error())
			continue
		}

		if applied == len(cs.FieldChanges) {
			finalizeErr := FinalizeChangeSet(client, cs.ChangeSetID)
			if finalizeErr != nil {
				summary.Errors = append(summary.Errors, finalizeErr.Error())
				continue
			}
			summary.ChangeSetsFullyApplied++
		}
	}

	return summary, nil
}

// extractInt reads an integer from a row map, handling JSON float64 numbers.
func extractInt(row map[string]interface{}, field string) (int, bool) {
	val, ok := row[field]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}
