//# tools/runners/notification-runner/runner.go

go
package notification

import (
	"time"
)

// NotificationTrigger represents one state transition that requires notification.
type NotificationTrigger struct {
	TriggerType   string // change_set_pending_approval, emergency_change_filed, etc.
	EntityType    string
	EntityID      int
	ChangeSetID   *int
	Severity      string // info, warning, high, critical
	Title         string
	Description   string
	DetectedTime  time.Time
	SourceJobID   *int // runner_job_id that produced this trigger
}

// NotificationRecipient represents one person to notify for a trigger.
type NotificationRecipient struct {
	OpsUserID     int
	Username      string
	Email         string
	Role          string // owner, stakeholder, approver, on_call
	PreferredBackend string // email, webhook, or empty for default
}

// NotificationMessage holds a rendered message ready to dispatch.
type NotificationMessage struct {
	Trigger    *NotificationTrigger
	Recipient  *NotificationRecipient
	Subject    string
	Body       string
	BackendType string
	Metadata   map[string]interface{}
}

// DeliveryResult records the outcome of one notification dispatch attempt.
type DeliveryResult struct {
	MessageID  string
	Backend    string
	Recipient  string
	Success    bool
	ErrorMsg   string
	SentTime   time.Time
}

// NotificationSummary holds the results of one notification cycle.
type NotificationSummary struct {
	TriggersFound       int
	NotificationsSent   int
	NotificationsFailed int
	NotificationsDeduped int
	BackendsUsed        []string
	Errors              []string
}

// Backend is the interface that notification delivery backends implement.
type Backend interface {
	// Send delivers one notification message. Returns a delivery result.
	Send(msg *NotificationMessage) (*DeliveryResult, error)
	// Type returns the backend type name: "email", "webhook".
	Type() string
	// Healthy returns true if the backend is reachable and ready.
	Healthy() bool
}

// GetNotificationTriggers reads state transitions since lastCycleTime
// that require notification, filtered by enabled notification types.
func GetNotificationTriggers(client interface{}, lastCycleTime time.Time, enabledTypes map[string]bool, batchSize int) ([]NotificationTrigger, error) {
	// TODO: initialize triggers slice
	// TODO: if enabledTypes["change_set_pending_approval"]:
	//   search change_set where status=pending_approval and updated_time > lastCycleTime
	//   for each: create trigger with type=change_set_pending_approval, severity=info
	// TODO: if enabledTypes["change_set_approved"]:
	//   search change_set where status=approved and updated_time > lastCycleTime
	//   for each: create trigger with type=change_set_approved, severity=info
	// TODO: if enabledTypes["change_set_rejected"]:
	//   search change_set where status=rejected and updated_time > lastCycleTime
	//   for each: create trigger with type=change_set_rejected, severity=warning
	// TODO: if enabledTypes["emergency_change_filed"]:
	//   search change_set where is_emergency=true and created_time > lastCycleTime
	//   for each: create trigger with type=emergency_change_filed, severity=high
	// TODO: if enabledTypes["compliance_finding_created"]:
	//   search compliance_finding where created_time > lastCycleTime
	//   for each: create trigger with finding severity
	// TODO: if enabledTypes["escalation_overdue"]:
	//   search runner_job_output_var where var_name=escalation_notification
	//     and runner_job.started_time > lastCycleTime
	//   for each: create trigger with type=escalation_overdue, severity=high
	// TODO: limit total triggers to batchSize
	// TODO: return triggers
	return nil, nil
}

// ResolveRecipients determines who should receive notification for a trigger
// by walking ownership and stakeholder bridges.
func ResolveRecipients(client interface{}, trigger *NotificationTrigger) ([]NotificationRecipient, error) {
	// TODO: switch on trigger.TriggerType:
	//   change_set_pending_approval:
	//     read change_set_approval_required rows for the change set
	//     for each required approver group: resolve group members
	//     return members as recipients with role=approver
	//   change_set_approved, change_set_rejected:
	//     read change_set.submitter_ops_user_id
	//     return submitter as recipient with role=submitter
	//   emergency_change_filed:
	//     read change_set submitter
	//     walk service_ownership for affected entities -> owners
	//     walk escalation_path for service -> on_call
	//     return all with appropriate roles
	//   compliance_finding_created:
	//     walk service_ownership for finding target -> owners
	//     walk compliance_regime stakeholders
	//     return all
	//   escalation_overdue:
	//     read escalation_path for affected service
	//     walk next escalation_step beyond current
	//     return escalation targets
	// TODO: deduplicate recipients by ops_user_id
	return nil, nil
}

// RenderMessage creates a notification message from a trigger and recipient.
func RenderMessage(trigger *NotificationTrigger, recipient *NotificationRecipient) *NotificationMessage {
	// TODO: build subject from trigger type and entity:
	//   change_set_pending_approval: "[OpsDB] Approval needed: {change_set.name}"
	//   emergency_change_filed: "[OpsDB] EMERGENCY: {change_set.name}"
	//   compliance_finding_created: "[OpsDB] Finding: {finding.title}"
	//   etc.
	// TODO: build body with:
	//   trigger description
	//   link to change set or finding (API URL)
	//   action needed (approve, review, acknowledge)
	//   who is being notified and why (role)
	// TODO: determine backend from recipient.PreferredBackend or default
	// TODO: return NotificationMessage
	return nil
}

// IsDuplicate checks if an identical notification was sent within the
// deduplication window. Prevents notification storms.
func IsDuplicate(client interface{}, trigger *NotificationTrigger, recipientUserID int, dedupWindowSeconds int) bool {
	// TODO: search runner_job_output_var for recent notification records:
	//   var_name = "notification_sent"
	//   var_value contains trigger_type + entity_id + recipient_user_id
	//   runner_job.started_time > now - dedupWindowSeconds
	// TODO: if found: return true (duplicate, skip)
	// TODO: return false
	return false
}

// RecordDelivery writes a runner_job_output_var recording that a notification
// was sent, for deduplication and audit trail.
func RecordDelivery(client interface{}, result *DeliveryResult, trigger *NotificationTrigger, recipientUserID int, runnerJobID int) error {
	// TODO: call client.WriteObservation with:
	//   target_table: runner_job_output_var
	//   var_name: "notification_sent"
	//   var_value: JSON with trigger_type, entity_id, recipient_user_id,
	//     backend, success, message_id, sent_time
	//   runner_job_id: runnerJobID
	return nil
}

// ProcessCycle runs one complete get/act/set cycle for the notification runner.
func ProcessCycle(client interface{}, backends []Backend, lastCycleTime time.Time, enabledTypes map[string]bool, batchSize int, dedupWindowSeconds int, dryRun bool) (*NotificationSummary, error) {
	summary := &NotificationSummary{}

	// TODO: GET phase
	//   triggers, err := GetNotificationTriggers(client, lastCycleTime, enabledTypes, batchSize)
	//   summary.TriggersFound = len(triggers)

	// TODO: if dryRun:
	//   for each trigger: log trigger type, entity, severity, would-notify recipients
	//   return summary

	// TODO: ACT phase
	//   backendMap := build map[string]Backend from backends list
	//   for each trigger:
	//     recipients, err := ResolveRecipients(client, &trigger)
	//     for each recipient:
	//       if IsDuplicate(...): summary.NotificationsDeduped++; continue
	//       msg := RenderMessage(&trigger, &recipient)
	//       backend := backendMap[msg.BackendType]
	//       if backend == nil: use first available backend
	//       result, err := backend.Send(msg)
	//       if err or !result.Success:
	//         summary.NotificationsFailed++
	//         summary.Errors = append(...)
	//       else:
	//         summary.NotificationsSent++
	//         track backend in summary.BackendsUsed
	//       RecordDelivery(client, result, &trigger, recipient.OpsUserID, runnerJobID)

	// TODO: return summary, nil
	return summary, nil
}


