//# tools/runners/emergency-review-monitor/monitor.go

go
package monitor

import (
	"time"
)

// OverdueReview holds one emergency review that has passed its review window.
type OverdueReview struct {
	EmergencyReviewID int
	ChangeSetID       int
	ChangeSetName     string
	SubmitterUserID   int
	SubmittedTime     time.Time
	ReviewWindowHours int
	ElapsedHours      float64
	LastEscalationTime *time.Time
	ExistingFindingID  *int
}

// MonitorSummary holds the results of one monitor cycle.
type MonitorSummary struct {
	ReviewsChecked   int
	OverdueFound     int
	FindingsCreated  int
	EscalationsSent  int
	Errors           []string
}

// GetPendingEmergencyReviews reads all pending emergency reviews from OpsDB
// and determines which are overdue.
func GetPendingEmergencyReviews(client interface{}, defaultWindowHours int) ([]OverdueReview, int, error) {
	// TODO: search change_set_emergency_review where status=pending
	// TODO: for each review:
	//   load parent change_set for submitted_time
	//   load change_management_rule for review_window_hours override
	//     (per-entity or per-policy configured window, falling back to default)
	//   compute elapsed = time.Since(submitted_time).Hours()
	//   if elapsed > review_window:
	//     check for existing compliance_finding with:
	//       finding_type=emergency_review_overdue, target change_set_id
	//     check for last escalation time from prior findings
	//     add to overdue list
	// TODO: return overdue list, total reviews checked, nil
	return nil, 0, nil
}

// FileOverdueFinding creates a compliance_finding for an overdue emergency review.
func FileOverdueFinding(client interface{}, review *OverdueReview, runnerJobID int) (int, error) {
	// TODO: build evidence/finding data:
	//   finding_type: "emergency_review_overdue"
	//   severity: "high" (escalate to "critical" if > 2x window)
	//   title: "Emergency change set {review.ChangeSetID} overdue for review"
	//   description: "Change set '{review.ChangeSetName}' was submitted as emergency
	//     {review.ElapsedHours:.0f} hours ago and has not been reviewed.
	//     Review window is {review.ReviewWindowHours} hours."
	//   target_change_set_id: review.ChangeSetID
	//   submitter_ops_user_id: review.SubmitterUserID
	//   detected_time: now
	//   runner_job_id: runnerJobID
	// TODO: call client.WriteObservation to create compliance_finding row
	// TODO: return finding ID
	return 0, nil
}

// ShouldReescalate determines if an overdue review needs another escalation
// notification based on the escalation interval.
func ShouldReescalate(review *OverdueReview, escalationIntervalHours int) bool {
	// TODO: if review.LastEscalationTime is nil: return true (first escalation)
	// TODO: elapsed since last escalation = time.Since(*review.LastEscalationTime).Hours()
	// TODO: return elapsed >= float64(escalationIntervalHours)
	return false
}

// SignalNotification writes a runner_job_output_var that the notification
// runner picks up to dispatch the escalation.
func SignalNotification(client interface{}, review *OverdueReview, findingID int, runnerJobID int) error {
	// TODO: call client.WriteObservation with:
	//   target_table: runner_job_output_var
	//   var_name: "escalation_notification"
	//   var_value: JSON with change_set_id, finding_id, severity, elapsed_hours
	//   runner_job_id: runnerJobID
	return nil
}

// ProcessCycle runs one complete get/act/set cycle for the monitor.
func ProcessCycle(client interface{}, defaultWindowHours int, escalationIntervalHours int, maxFindings int, notifyOnOverdue bool, dryRun bool) (*MonitorSummary, error) {
	summary := &MonitorSummary{}

	// TODO: GET phase
	//   overdue, totalChecked, err := GetPendingEmergencyReviews(client, defaultWindowHours)
	//   summary.ReviewsChecked = totalChecked
	//   summary.OverdueFound = len(overdue)

	// TODO: if dryRun:
	//   log plan listing each overdue review with elapsed hours
	//   return summary

	// TODO: ACT phase
	//   findingsCreated := 0
	//   for each review in overdue:
	//     if findingsCreated >= maxFindings:
	//       runner.RecordBoundHit("max_findings_per_cycle", maxFindings)
	//       break
	//     if review.ExistingFindingID == nil:
	//       findingID, err := FileOverdueFinding(client, &review, runnerJobID)
	//       if err: summary.Errors = append(...); continue
	//       summary.FindingsCreated++
	//       findingsCreated++
	//       if notifyOnOverdue:
	//         SignalNotification(client, &review, findingID, runnerJobID)
	//         summary.EscalationsSent++
	//     else if ShouldReescalate(&review, escalationIntervalHours):
	//       findingID, err := FileOverdueFinding(client, &review, runnerJobID)
	//       if err: summary.Errors = append(...); continue
	//       summary.FindingsCreated++
	//       findingsCreated++
	//       if notifyOnOverdue:
	//         SignalNotification(client, &review, findingID, runnerJobID)
	//         summary.EscalationsSent++

	// TODO: return summary, nil
	return summary, nil
}


