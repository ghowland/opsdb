package notification

import (
	"encoding/json"
	"fmt"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

// NotificationTrigger represents one state transition that requires notification.
type NotificationTrigger struct {
	TriggerType  string
	EntityType   string
	EntityID     int
	ChangeSetID  *int
	Severity     string
	Title        string
	Description  string
	DetectedTime time.Time
	SourceJobID  *int
}

// NotificationRecipient represents one person to notify for a trigger.
type NotificationRecipient struct {
	OpsUserID        int
	Username         string
	Email            string
	Role             string
	PreferredBackend string
}

// NotificationMessage holds a rendered message ready to dispatch.
type NotificationMessage struct {
	Trigger     *NotificationTrigger
	Recipient   *NotificationRecipient
	Subject     string
	Body        string
	BackendType string
	Metadata    map[string]interface{}
}

// DeliveryResult records the outcome of one notification dispatch attempt.
type DeliveryResult struct {
	MessageID string
	Backend   string
	Recipient string
	Success   bool
	ErrorMsg  string
	SentTime  time.Time
}

// NotificationSummary holds the results of one notification cycle.
type NotificationSummary struct {
	TriggersFound        int
	NotificationsSent    int
	NotificationsFailed  int
	NotificationsDeduped int
	BackendsUsed         []string
	Errors               []string
}

// Backend is the interface that notification delivery backends implement.
type Backend interface {
	Send(msg *NotificationMessage) (*DeliveryResult, error)
	Type() string
	Healthy() bool
}

// GetNotificationTriggers reads state transitions since lastCycleTime
// that require notification, filtered by enabled notification types.
func GetNotificationTriggers(client *runner.APIClient, lastCycleTime time.Time, enabledTypes map[string]bool, batchSize int) ([]NotificationTrigger, error) {
	var triggers []NotificationTrigger
	lastCycleStr := lastCycleTime.UTC().Format(time.RFC3339Nano)

	// Change sets entering pending_approval.
	if enabledTypes["change_set_pending_approval"] {
		result, err := client.Search("change_set",
			[]runner.SearchFilter{
				{Field: "status", Operator: "eq", Value: "pending_approval"},
				{Field: "updated_time", Operator: "gt", Value: lastCycleStr},
			},
			[]runner.OrderSpec{{Field: "updated_time", Direction: "asc"}},
			batchSize-len(triggers), "")
		if err == nil {
			for _, row := range result.Rows {
				triggers = append(triggers, changeSetTrigger(row, "change_set_pending_approval", "info"))
			}
		}
	}

	// Change sets approved.
	if enabledTypes["change_set_approved"] && len(triggers) < batchSize {
		result, err := client.Search("change_set",
			[]runner.SearchFilter{
				{Field: "status", Operator: "eq", Value: "approved"},
				{Field: "updated_time", Operator: "gt", Value: lastCycleStr},
			},
			[]runner.OrderSpec{{Field: "updated_time", Direction: "asc"}},
			batchSize-len(triggers), "")
		if err == nil {
			for _, row := range result.Rows {
				triggers = append(triggers, changeSetTrigger(row, "change_set_approved", "info"))
			}
		}
	}

	// Change sets rejected.
	if enabledTypes["change_set_rejected"] && len(triggers) < batchSize {
		result, err := client.Search("change_set",
			[]runner.SearchFilter{
				{Field: "status", Operator: "eq", Value: "rejected"},
				{Field: "updated_time", Operator: "gt", Value: lastCycleStr},
			},
			[]runner.OrderSpec{{Field: "updated_time", Direction: "asc"}},
			batchSize-len(triggers), "")
		if err == nil {
			for _, row := range result.Rows {
				triggers = append(triggers, changeSetTrigger(row, "change_set_rejected", "warning"))
			}
		}
	}

	// Emergency changes filed.
	if enabledTypes["emergency_change_filed"] && len(triggers) < batchSize {
		result, err := client.Search("change_set",
			[]runner.SearchFilter{
				{Field: "is_emergency", Operator: "eq", Value: true},
				{Field: "created_time", Operator: "gt", Value: lastCycleStr},
			},
			[]runner.OrderSpec{{Field: "created_time", Direction: "asc"}},
			batchSize-len(triggers), "")
		if err == nil {
			for _, row := range result.Rows {
				triggers = append(triggers, changeSetTrigger(row, "emergency_change_filed", "high"))
			}
		}
	}

	// Compliance findings created.
	if enabledTypes["compliance_finding_created"] && len(triggers) < batchSize {
		result, err := client.Search("compliance_finding",
			[]runner.SearchFilter{
				{Field: "created_time", Operator: "gt", Value: lastCycleStr},
			},
			[]runner.OrderSpec{{Field: "created_time", Direction: "asc"}},
			batchSize-len(triggers), "")
		if err == nil {
			for _, row := range result.Rows {
				id, _ := extractInt(row, "id")
				severity, _ := row["severity"].(string)
				if severity == "" {
					severity = "warning"
				}
				title, _ := row["title"].(string)
				description, _ := row["description"].(string)

				triggers = append(triggers, NotificationTrigger{
					TriggerType:  "compliance_finding_created",
					EntityType:   "compliance_finding",
					EntityID:     id,
					Severity:     severity,
					Title:        title,
					Description:  description,
					DetectedTime: time.Now(),
				})
			}
		}
	}

	// Escalation notifications from emergency review monitor.
	if enabledTypes["escalation_overdue"] && len(triggers) < batchSize {
		result, err := client.Search("runner_job_output_var",
			[]runner.SearchFilter{
				{Field: "var_name", Operator: "eq", Value: "escalation_notification"},
				{Field: "created_time", Operator: "gt", Value: lastCycleStr},
			},
			[]runner.OrderSpec{{Field: "created_time", Direction: "asc"}},
			batchSize-len(triggers), "")
		if err == nil {
			for _, row := range result.Rows {
				id, _ := extractInt(row, "id")
				varValue, _ := row["var_value"].(string)
				jobID, _ := extractInt(row, "runner_job_id")

				var escalationData map[string]interface{}
				if varValue != "" {
					json.Unmarshal([]byte(varValue), &escalationData)
				}

				csID := 0
				severity := "high"
				if escalationData != nil {
					if v, ok := escalationData["change_set_id"].(float64); ok {
						csID = int(v)
					}
					if v, ok := escalationData["severity"].(string); ok {
						severity = v
					}
				}

				trigger := NotificationTrigger{
					TriggerType:  "escalation_overdue",
					EntityType:   "runner_job_output_var",
					EntityID:     id,
					Severity:     severity,
					Title:        fmt.Sprintf("Emergency review escalation for change set %d", csID),
					DetectedTime: time.Now(),
					SourceJobID:  &jobID,
				}
				if csID > 0 {
					trigger.ChangeSetID = &csID
				}
				triggers = append(triggers, trigger)
			}
		}
	}

	return triggers, nil
}

// ResolveRecipients determines who should receive notification for a trigger
// by walking ownership and stakeholder bridges.
func ResolveRecipients(client *runner.APIClient, trigger *NotificationTrigger) ([]NotificationRecipient, error) {
	var recipients []NotificationRecipient
	seen := make(map[int]bool) // deduplicate by ops_user_id

	switch trigger.TriggerType {
	case "change_set_pending_approval":
		if trigger.ChangeSetID == nil {
			break
		}
		// Find required approver groups for this change set.
		approvalResult, err := client.Search("change_set_approval_required",
			[]runner.SearchFilter{
				{Field: "change_set_id", Operator: "eq", Value: *trigger.ChangeSetID},
				{Field: "is_fulfilled", Operator: "eq", Value: false},
			}, nil, 100, "")
		if err != nil {
			return nil, fmt.Errorf("searching approval requirements: %w", err)
		}

		for _, row := range approvalResult.Rows {
			groupID, _ := extractInt(row, "ops_group_required_id")
			if groupID == 0 {
				continue
			}

			members, err := resolveGroupMembers(client, groupID)
			if err != nil {
				continue
			}
			for _, member := range members {
				if !seen[member.OpsUserID] {
					member.Role = "approver"
					recipients = append(recipients, member)
					seen[member.OpsUserID] = true
				}
			}
		}

	case "change_set_approved", "change_set_rejected":
		if trigger.ChangeSetID == nil {
			break
		}
		csRow, err := client.GetEntity("change_set", *trigger.ChangeSetID)
		if err != nil {
			break
		}
		submitterID, _ := extractInt(csRow, "submitted_by_ops_user_id")
		if submitterID > 0 {
			user, err := resolveUser(client, submitterID)
			if err == nil {
				user.Role = "submitter"
				recipients = append(recipients, user)
			}
		}

	case "emergency_change_filed":
		if trigger.ChangeSetID == nil {
			break
		}
		// Notify submitter.
		csRow, err := client.GetEntity("change_set", *trigger.ChangeSetID)
		if err != nil {
			break
		}
		submitterID, _ := extractInt(csRow, "submitted_by_ops_user_id")
		if submitterID > 0 {
			user, err := resolveUser(client, submitterID)
			if err == nil {
				user.Role = "submitter"
				recipients = append(recipients, user)
				seen[user.OpsUserID] = true
			}
		}

		// Notify owners of affected entities via ownership bridges.
		owners, _ := resolveEntityOwners(client, trigger.EntityType, trigger.EntityID)
		for _, owner := range owners {
			if !seen[owner.OpsUserID] {
				owner.Role = "owner"
				recipients = append(recipients, owner)
				seen[owner.OpsUserID] = true
			}
		}

	case "compliance_finding_created":
		owners, _ := resolveEntityOwners(client, trigger.EntityType, trigger.EntityID)
		for _, owner := range owners {
			if !seen[owner.OpsUserID] {
				owner.Role = "owner"
				recipients = append(recipients, owner)
				seen[owner.OpsUserID] = true
			}
		}

	case "escalation_overdue":
		if trigger.ChangeSetID != nil {
			owners, _ := resolveEntityOwners(client, "change_set", *trigger.ChangeSetID)
			for _, owner := range owners {
				if !seen[owner.OpsUserID] {
					owner.Role = "escalation_target"
					recipients = append(recipients, owner)
					seen[owner.OpsUserID] = true
				}
			}
		}
	}

	return recipients, nil
}

// RenderMessage creates a notification message from a trigger and recipient.
func RenderMessage(trigger *NotificationTrigger, recipient *NotificationRecipient) *NotificationMessage {
	var subject, body string

	switch trigger.TriggerType {
	case "change_set_pending_approval":
		subject = fmt.Sprintf("[OpsDB] Approval needed: %s", trigger.Title)
		body = fmt.Sprintf(
			"A change set requires your approval.\n\n"+
				"Title: %s\n"+
				"Description: %s\n"+
				"Severity: %s\n\n"+
				"You are receiving this because you are an %s.\n"+
				"Please review and approve or reject this change set.",
			trigger.Title, trigger.Description, trigger.Severity, recipient.Role)

	case "change_set_approved":
		subject = fmt.Sprintf("[OpsDB] Approved: %s", trigger.Title)
		body = fmt.Sprintf(
			"Your change set has been approved.\n\n"+
				"Title: %s\n"+
				"The change set executor will apply the changes shortly.",
			trigger.Title)

	case "change_set_rejected":
		subject = fmt.Sprintf("[OpsDB] Rejected: %s", trigger.Title)
		body = fmt.Sprintf(
			"Your change set has been rejected.\n\n"+
				"Title: %s\n"+
				"Description: %s\n\n"+
				"Please review the rejection reason and submit a revised change set if appropriate.",
			trigger.Title, trigger.Description)

	case "emergency_change_filed":
		subject = fmt.Sprintf("[OpsDB] EMERGENCY: %s", trigger.Title)
		body = fmt.Sprintf(
			"An emergency change has been filed.\n\n"+
				"Title: %s\n"+
				"Description: %s\n"+
				"Severity: %s\n\n"+
				"This change was applied with reduced approvals and must be reviewed within the review window.\n"+
				"You are receiving this as %s.",
			trigger.Title, trigger.Description, trigger.Severity, recipient.Role)

	case "compliance_finding_created":
		subject = fmt.Sprintf("[OpsDB] Finding: %s", trigger.Title)
		body = fmt.Sprintf(
			"A compliance finding has been created.\n\n"+
				"Title: %s\n"+
				"Description: %s\n"+
				"Severity: %s\n\n"+
				"You are receiving this as %s of the affected entity.",
			trigger.Title, trigger.Description, trigger.Severity, recipient.Role)

	case "escalation_overdue":
		subject = fmt.Sprintf("[OpsDB] ESCALATION: %s", trigger.Title)
		body = fmt.Sprintf(
			"An emergency change review is overdue and has been escalated.\n\n"+
				"Title: %s\n"+
				"Description: %s\n"+
				"Severity: %s\n\n"+
				"Immediate review is required.",
			trigger.Title, trigger.Description, trigger.Severity)

	default:
		subject = fmt.Sprintf("[OpsDB] %s", trigger.Title)
		body = fmt.Sprintf("%s\n\nSeverity: %s\n%s",
			trigger.Title, trigger.Severity, trigger.Description)
	}

	backendType := recipient.PreferredBackend
	if backendType == "" {
		backendType = "email"
	}

	return &NotificationMessage{
		Trigger:     trigger,
		Recipient:   recipient,
		Subject:     subject,
		Body:        body,
		BackendType: backendType,
		Metadata: map[string]interface{}{
			"trigger_type": trigger.TriggerType,
			"entity_type":  trigger.EntityType,
			"entity_id":    trigger.EntityID,
			"recipient_id": recipient.OpsUserID,
		},
	}
}

// IsDuplicate checks if an identical notification was sent within the
// deduplication window. Prevents notification storms.
func IsDuplicate(client *runner.APIClient, trigger *NotificationTrigger, recipientUserID int, dedupWindowSeconds int) bool {
	cutoff := time.Now().Add(-time.Duration(dedupWindowSeconds) * time.Second)
	cutoffStr := cutoff.UTC().Format(time.RFC3339Nano)

	dedupKey := fmt.Sprintf("%s:%s:%d:%d", trigger.TriggerType, trigger.EntityType, trigger.EntityID, recipientUserID)

	result, err := client.Search("runner_job_output_var",
		[]runner.SearchFilter{
			{Field: "var_name", Operator: "eq", Value: "notification_sent"},
			{Field: "var_value", Operator: "like", Value: dedupKey + "%"},
			{Field: "created_time", Operator: "gt", Value: cutoffStr},
		}, nil, 1, "")
	if err != nil {
		return false // on error, don't suppress — better to duplicate than miss
	}

	return len(result.Rows) > 0
}

// RecordDelivery writes a runner_job_output_var recording that a notification
// was sent, for deduplication and audit trail.
func RecordDelivery(client *runner.APIClient, result *DeliveryResult, trigger *NotificationTrigger, recipientUserID int, runnerJobID int) error {
	dedupKey := fmt.Sprintf("%s:%s:%d:%d", trigger.TriggerType, trigger.EntityType, trigger.EntityID, recipientUserID)

	deliveryData := map[string]interface{}{
		"trigger_type":      trigger.TriggerType,
		"entity_type":       trigger.EntityType,
		"entity_id":         trigger.EntityID,
		"recipient_user_id": recipientUserID,
		"backend":           result.Backend,
		"is_success":        result.Success,
		"message_id":        result.MessageID,
		"sent_time":         result.SentTime.UTC().Format(time.RFC3339Nano),
		"dedup_key":         dedupKey,
	}
	if result.ErrorMsg != "" {
		deliveryData["error_message"] = result.ErrorMsg
	}

	_, err := client.WriteObservation(&runner.WriteObservationParams{
		TargetTable:  "runner_job_output_var",
		Key:          "notification_sent",
		Value:        dedupKey,
		DataJSON:     deliveryData,
		RunnerJobID:  runnerJobID,
		ObservedTime: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("recording delivery for %s: %w", dedupKey, err)
	}

	return nil
}

// ProcessCycle runs one complete get/act/set cycle for the notification runner.
func ProcessCycle(client *runner.APIClient, notifBackends []Backend, lastCycleTime time.Time, enabledTypes map[string]bool, batchSize int, dedupWindowSeconds int, dryRun bool, runnerJobID int) (*NotificationSummary, error) {
	summary := &NotificationSummary{}

	// GET phase.
	triggers, err := GetNotificationTriggers(client, lastCycleTime, enabledTypes, batchSize)
	if err != nil {
		return nil, fmt.Errorf("get phase: %w", err)
	}
	summary.TriggersFound = len(triggers)

	if dryRun {
		return summary, nil
	}

	// ACT phase.
	backendMap := make(map[string]Backend)
	for _, b := range notifBackends {
		backendMap[b.Type()] = b
	}
	var defaultBackend Backend
	if len(notifBackends) > 0 {
		defaultBackend = notifBackends[0]
	}

	backendsUsedSet := make(map[string]bool)

	for i := range triggers {
		trigger := &triggers[i]

		recipients, err := ResolveRecipients(client, trigger)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("resolve recipients: %v", err))
			continue
		}

		for _, recipient := range recipients {
			if IsDuplicate(client, trigger, recipient.OpsUserID, dedupWindowSeconds) {
				summary.NotificationsDeduped++
				continue
			}

			msg := RenderMessage(trigger, &recipient)

			backend := defaultBackend
			if b, ok := backendMap[msg.BackendType]; ok {
				backend = b
			}
			if backend == nil {
				summary.NotificationsFailed++
				summary.Errors = append(summary.Errors, fmt.Sprintf("no backend for %s", recipient.Username))
				continue
			}

			deliveryResult, sendErr := backend.Send(msg)
			backendsUsedSet[backend.Type()] = true

			if sendErr != nil {
				summary.NotificationsFailed++
				summary.Errors = append(summary.Errors, fmt.Sprintf("send to %s: %v", recipient.Username, sendErr))
				deliveryResult = &DeliveryResult{
					Backend:   backend.Type(),
					Recipient: recipient.Username,
					Success:   false,
					ErrorMsg:  sendErr.Error(),
					SentTime:  time.Now(),
				}
			} else if !deliveryResult.Success {
				summary.NotificationsFailed++
				summary.Errors = append(summary.Errors, fmt.Sprintf("send to %s: %s", recipient.Username, deliveryResult.ErrorMsg))
			} else {
				summary.NotificationsSent++
			}

			RecordDelivery(client, deliveryResult, trigger, recipient.OpsUserID, runnerJobID)
		}
	}

	for b := range backendsUsedSet {
		summary.BackendsUsed = append(summary.BackendsUsed, b)
	}

	return summary, nil
}

// --- internal helpers ---

// changeSetTrigger builds a NotificationTrigger from a change_set row.
func changeSetTrigger(row map[string]interface{}, triggerType string, severity string) NotificationTrigger {
	id, _ := extractInt(row, "id")
	name, _ := row["name"].(string)
	description, _ := row["description"].(string)

	trigger := NotificationTrigger{
		TriggerType:  triggerType,
		EntityType:   "change_set",
		EntityID:     id,
		Severity:     severity,
		Title:        name,
		Description:  description,
		DetectedTime: time.Now(),
	}
	trigger.ChangeSetID = &id
	return trigger
}

// resolveGroupMembers fetches members of an ops_group and returns them as recipients.
func resolveGroupMembers(client *runner.APIClient, groupID int) ([]NotificationRecipient, error) {
	memberResult, err := client.Search("ops_group_member",
		[]runner.SearchFilter{
			{Field: "ops_group_id", Operator: "eq", Value: groupID},
		}, nil, 100, "")
	if err != nil {
		return nil, err
	}

	var recipients []NotificationRecipient
	for _, row := range memberResult.Rows {
		userID, _ := extractInt(row, "ops_user_id")
		if userID == 0 {
			continue
		}
		user, err := resolveUser(client, userID)
		if err != nil {
			continue
		}
		recipients = append(recipients, user)
	}
	return recipients, nil
}

// resolveUser fetches an ops_user and returns a NotificationRecipient.
func resolveUser(client *runner.APIClient, userID int) (NotificationRecipient, error) {
	userRow, err := client.GetEntity("ops_user", userID)
	if err != nil {
		return NotificationRecipient{}, err
	}

	return NotificationRecipient{
		OpsUserID: userID,
		Username:  stringOrEmpty(userRow, "username"),
		Email:     stringOrEmpty(userRow, "email"),
	}, nil
}

// resolveEntityOwners looks up ownership bridge rows for an entity and
// returns the owning users as recipients.
func resolveEntityOwners(client *runner.APIClient, entityType string, entityID int) ([]NotificationRecipient, error) {
	// Determine which ownership bridge to query.
	ownershipTable := ""
	fkField := ""
	switch entityType {
	case "service", "change_set":
		ownershipTable = "service_ownership"
		fkField = "service_id"
	case "machine":
		ownershipTable = "machine_ownership"
		fkField = "machine_id"
	case "k8s_cluster":
		ownershipTable = "k8s_cluster_ownership"
		fkField = "k8s_cluster_id"
	case "cloud_resource":
		ownershipTable = "cloud_resource_ownership"
		fkField = "cloud_resource_id"
	default:
		return nil, nil // no ownership bridge for this entity type
	}

	result, err := client.Search(ownershipTable,
		[]runner.SearchFilter{
			{Field: fkField, Operator: "eq", Value: entityID},
		}, nil, 50, "")
	if err != nil {
		return nil, err
	}

	var recipients []NotificationRecipient
	for _, row := range result.Rows {
		roleID, _ := extractInt(row, "ops_user_role_id")
		if roleID == 0 {
			continue
		}

		// Resolve role to user via ops_user_role_member.
		memberResult, err := client.Search("ops_user_role_member",
			[]runner.SearchFilter{
				{Field: "ops_user_role_id", Operator: "eq", Value: roleID},
			}, nil, 10, "")
		if err != nil {
			continue
		}

		for _, memberRow := range memberResult.Rows {
			userID, _ := extractInt(memberRow, "ops_user_id")
			if userID == 0 {
				continue
			}
			user, err := resolveUser(client, userID)
			if err != nil {
				continue
			}
			recipients = append(recipients, user)
		}
	}

	return recipients, nil
}

// stringOrEmpty reads a string from a row map, returning empty string if absent.
func stringOrEmpty(row map[string]interface{}, key string) string {
	v, _ := row[key].(string)
	return v
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
