package monitor

import (
	"encoding/json"
	"fmt"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

// OverdueReview holds one emergency review that has passed its review window.
type OverdueReview struct {
	EmergencyReviewID  int
	ChangeSetID        int
	ChangeSetName      string
	SubmitterUserID    int
	SubmittedTime      time.Time
	ReviewWindowHours  int
	ElapsedHours       float64
	LastEscalationTime *time.Time
	ExistingFindingID  *int
}

// MonitorSummary holds the results of one monitor cycle.
type MonitorSummary struct {
	ReviewsChecked  int
	OverdueFound    int
	FindingsCreated int
	EscalationsSent int
	Errors          []string
}

// GetPendingEmergencyReviews reads all pending emergency reviews from OpsDB
// and determines which are overdue.
func GetPendingEmergencyReviews(client *runner.APIClient, defaultWindowHours int) ([]OverdueReview, int, error) {
	// Search for pending emergency reviews.
	reviewResult, err := client.Search("change_set_emergency_review",
		[]runner.SearchFilter{
			{Field: "review_status", Operator: "eq", Value: "pending"},
		},
		[]runner.OrderSpec{{Field: "created_time", Direction: "asc"}},
		1000, "")
	if err != nil {
		return nil, 0, fmt.Errorf("searching pending emergency reviews: %w", err)
	}

	totalChecked := len(reviewResult.Rows)
	var overdue []OverdueReview

	for _, row := range reviewResult.Rows {
		reviewID, _ := extractInt(row, "id")
		changeSetID, _ := extractInt(row, "change_set_id")

		// Load parent change set for submitted_time and metadata.
		csRow, err := client.GetEntity("change_set", changeSetID)
		if err != nil {
			continue // skip if change set not accessible
		}

		submittedTimeStr, _ := csRow["submitted_time"].(string)
		submittedTime, err := time.Parse(time.RFC3339Nano, submittedTimeStr)
		if err != nil {
			submittedTime, err = time.Parse(time.RFC3339, submittedTimeStr)
			if err != nil {
				continue // skip unparseable timestamps
			}
		}

		csName, _ := csRow["name"].(string)
		submitterID, _ := extractInt(csRow, "submitted_by_ops_user_id")

		// Determine review window: check for per-policy override, fall back to default.
		windowHours := defaultWindowHours
		// Look for change management rule that might override the window.
		cmRuleResult, err := client.Search("policy",
			[]runner.SearchFilter{
				{Field: "policy_type", Operator: "eq", Value: "change_management"},
				{Field: "is_active", Operator: "eq", Value: true},
			},
			nil, 1, "")
		if err == nil && len(cmRuleResult.Rows) > 0 {
			if policyData, ok := cmRuleResult.Rows[0]["policy_data_json"].(map[string]interface{}); ok {
				if overrideHours, ok := policyData["emergency_review_window_hours"].(float64); ok && overrideHours > 0 {
					windowHours = int(overrideHours)
				}
			}
		}

		elapsed := time.Since(submittedTime).Hours()

		// Only include if actually overdue.
		if elapsed <= float64(windowHours) {
			continue
		}

		review := OverdueReview{
			EmergencyReviewID: reviewID,
			ChangeSetID:       changeSetID,
			ChangeSetName:     csName,
			SubmitterUserID:   submitterID,
			SubmittedTime:     submittedTime,
			ReviewWindowHours: windowHours,
			ElapsedHours:      elapsed,
		}

		// Check for existing compliance finding for this change set.
		findingResult, err := client.Search("compliance_finding",
			[]runner.SearchFilter{
				{Field: "finding_type", Operator: "eq", Value: "emergency_review_overdue"},
				{Field: "target_entity_type", Operator: "eq", Value: "change_set"},
				{Field: "target_entity_id", Operator: "eq", Value: changeSetID},
			},
			[]runner.OrderSpec{{Field: "created_time", Direction: "desc"}},
			1, "")
		if err == nil && len(findingResult.Rows) > 0 {
			findingID, _ := extractInt(findingResult.Rows[0], "id")
			review.ExistingFindingID = &findingID

			// Extract last escalation time from the most recent finding.
			if createdStr, ok := findingResult.Rows[0]["created_time"].(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, createdStr); err == nil {
					review.LastEscalationTime = &t
				} else if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
					review.LastEscalationTime = &t
				}
			}
		}

		overdue = append(overdue, review)
	}

	return overdue, totalChecked, nil
}

// FileOverdueFinding creates a compliance_finding for an overdue emergency review.
func FileOverdueFinding(client *runner.APIClient, review *OverdueReview, runnerJobID int) (int, error) {
	severity := "high"
	if review.ElapsedHours > float64(review.ReviewWindowHours*2) {
		severity = "critical"
	}

	findingData := map[string]interface{}{
		"finding_type":            "emergency_review_overdue",
		"severity":                severity,
		"title":                   fmt.Sprintf("Emergency change set %d overdue for review", review.ChangeSetID),
		"description":             fmt.Sprintf("Change set '%s' was submitted as emergency %.0f hours ago and has not been reviewed. Review window is %d hours.", review.ChangeSetName, review.ElapsedHours, review.ReviewWindowHours),
		"target_entity_type":      "change_set",
		"target_entity_id":        review.ChangeSetID,
		"submitter_ops_user_id":   review.SubmitterUserID,
		"detected_time":           time.Now().UTC().Format(time.RFC3339Nano),
		"emergency_review_id":     review.EmergencyReviewID,
		"elapsed_hours":           review.ElapsedHours,
		"review_window_hours":     review.ReviewWindowHours,
	}

	result, err := client.WriteObservation(&runner.WriteObservationParams{
		TargetTable:  "compliance_finding",
		Key:          fmt.Sprintf("emergency_review_overdue:%d", review.ChangeSetID),
		Value:        severity,
		DataJSON:     findingData,
		RunnerJobID:  runnerJobID,
		ObservedTime: time.Now(),
	})
	if err != nil {
		return 0, fmt.Errorf("writing compliance finding for change_set %d: %w", review.ChangeSetID, err)
	}

	return result.RowID, nil
}

// ShouldReescalate determines if an overdue review needs another escalation
// notification based on the escalation interval.
func ShouldReescalate(review *OverdueReview, escalationIntervalHours int) bool {
	if review.LastEscalationTime == nil {
		return true // no prior escalation — first one
	}

	elapsed := time.Since(*review.LastEscalationTime).Hours()
	return elapsed >= float64(escalationIntervalHours)
}

// SignalNotification writes a runner_job_output_var that the notification
// runner picks up to dispatch the escalation.
func SignalNotification(client *runner.APIClient, review *OverdueReview, findingID int, runnerJobID int) error {
	severity := "high"
	if review.ElapsedHours > float64(review.ReviewWindowHours*2) {
		severity = "critical"
	}

	notificationData := map[string]interface{}{
		"change_set_id":       review.ChangeSetID,
		"change_set_name":     review.ChangeSetName,
		"finding_id":          findingID,
		"severity":            severity,
		"elapsed_hours":       review.ElapsedHours,
		"review_window_hours": review.ReviewWindowHours,
		"submitter_user_id":   review.SubmitterUserID,
	}

	notificationJSON, err := json.Marshal(notificationData)
	if err != nil {
		return fmt.Errorf("marshaling notification data: %w", err)
	}

	_, err = client.WriteObservation(&runner.WriteObservationParams{
		TargetTable:  "runner_job_output_var",
		Key:          "escalation_notification",
		Value:        string(notificationJSON),
		DataJSON:     notificationData,
		RunnerJobID:  runnerJobID,
		ObservedTime: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("writing escalation notification for change_set %d: %w", review.ChangeSetID, err)
	}

	return nil
}

// ProcessCycle runs one complete get/act/set cycle for the monitor.
func ProcessCycle(client *runner.APIClient, defaultWindowHours int, escalationIntervalHours int, maxFindings int, notifyOnOverdue bool, dryRun bool, runnerJobID int) (*MonitorSummary, error) {
	summary := &MonitorSummary{}

	// GET phase.
	overdue, totalChecked, err := GetPendingEmergencyReviews(client, defaultWindowHours)
	if err != nil {
		return nil, fmt.Errorf("get phase: %w", err)
	}
	summary.ReviewsChecked = totalChecked
	summary.OverdueFound = len(overdue)

	if dryRun {
		return summary, nil
	}

	// ACT phase.
	findingsCreated := 0
	for i := range overdue {
		review := &overdue[i]

		if findingsCreated >= maxFindings {
			break
		}

		if review.ExistingFindingID == nil {
			findingID, err := FileOverdueFinding(client, review, runnerJobID)
			if err != nil {
				summary.Errors = append(summary.Errors, err.Error())
				continue
			}
			summary.FindingsCreated++
			findingsCreated++

			if notifyOnOverdue {
				err = SignalNotification(client, review, findingID, runnerJobID)
				if err != nil {
					summary.Errors = append(summary.Errors, err.Error())
				} else {
					summary.EscalationsSent++
				}
			}
		} else if ShouldReescalate(review, escalationIntervalHours) {
			findingID, err := FileOverdueFinding(client, review, runnerJobID)
			if err != nil {
				summary.Errors = append(summary.Errors, err.Error())
				continue
			}
			summary.FindingsCreated++
			findingsCreated++

			if notifyOnOverdue {
				err = SignalNotification(client, review, findingID, runnerJobID)
				if err != nil {
					summary.Errors = append(summary.Errors, err.Error())
				} else {
					summary.EscalationsSent++
				}
			}
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
