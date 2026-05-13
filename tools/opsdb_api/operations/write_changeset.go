//# tools/opsdb_api/operations/write_changeset.go

package operations

import (
	"net/http"
	"time"

	"github.com/ghowland/opsdb/tools/opsdb_api/gate"
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// SubmitChangeSetParams holds change set submission parameters.
type SubmitChangeSetParams struct {
	SiteID       int           `json:"site_id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Reason       string        `json:"reason"`
	FieldChanges []FieldChange `json:"field_changes"`
	TicketRef    *int          `json:"ticket_authority_pointer_id"`
	IsEmergency  bool          `json:"is_emergency"`
	IsBulk       bool          `json:"is_bulk"`
	DryRun       bool          `json:"dry_run"`
}

// FieldChange represents one field change in a change set submission.
type FieldChange struct {
	EntityType   string      `json:"entity_type"`
	EntityID     int         `json:"entity_id"`
	FieldName    string      `json:"field_name"`
	BeforeValue  interface{} `json:"before_value"`
	AfterValue   interface{} `json:"after_value"`
	ChangeType   string      `json:"change_type"`
	VersionStamp int         `json:"version_stamp"`
}

// ChangeSetResult holds the result of a change set operation.
type ChangeSetResult struct {
	ChangeSetID      int                         `json:"change_set_id"`
	Status           string                      `json:"status"`
	FieldChangeIDs   []int                       `json:"field_change_ids,omitempty"`
	ApprovalRequired []ApprovalRequirementResult `json:"approval_required,omitempty"`
	ValidationErrors []ValidationError           `json:"validation_errors,omitempty"`
	DryRunResult     *DryRunResult               `json:"dry_run_result,omitempty"`
}

// ApprovalRequirementResult describes one computed approval requirement.
type ApprovalRequirementResult struct {
	RuleID        int    `json:"rule_id"`
	GroupID       int    `json:"group_id"`
	GroupName     string `json:"group_name"`
	CountRequired int    `json:"count_required"`
	AutoApproved  bool   `json:"auto_approved"`
}

// ValidationError describes one validation failure.
type ValidationError struct {
	EntityType string `json:"entity_type,omitempty"`
	EntityID   int    `json:"entity_id,omitempty"`
	FieldName  string `json:"field_name,omitempty"`
	ErrorType  string `json:"error_type"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

// DryRunResult holds the output of a dry-run submission.
type DryRunResult struct {
	WouldCreate          int                         `json:"would_create"`
	WouldUpdate          int                         `json:"would_update"`
	WouldRequireApproval []ApprovalRequirementResult `json:"would_require_approval,omitempty"`
	ValidationErrors     []ValidationError           `json:"validation_errors,omitempty"`
	ValidationWarnings   []ValidationError           `json:"validation_warnings,omitempty"`
}

// ---------------------------------------------------------------------------
// HTTP handlers
// ---------------------------------------------------------------------------

// SubmitChangeSet handles POST /api/v1/changeset/submit
// Parses the submission, packs field changes and metadata into a GateRequest,
// and lets the gate pipeline handle validation (steps 3-5), change management
// routing (step 7), and execution (step 9).
func (h *Handlers) SubmitChangeSet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var params SubmitChangeSetParams
	if err := parseJSONBody(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	// determine the primary target entity from the first field change
	// for the gate's entity-level auth/policy checks
	targetEntity := ""
	if len(params.FieldChanges) > 0 {
		targetEntity = params.FieldChanges[0].EntityType
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "submit_change_set",
		OperationClass: "write-cs",
		TargetEntity:   targetEntity,
		Params:         buildChangeSetParams(&params),
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}

// BulkSubmitChangeSet handles POST /api/v1/changeset/bulk-submit
func (h *Handlers) BulkSubmitChangeSet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var params SubmitChangeSetParams
	if err := parseJSONBody(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	params.IsBulk = true

	targetEntity := ""
	if len(params.FieldChanges) > 0 {
		targetEntity = params.FieldChanges[0].EntityType
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "bulk_submit_change_set",
		OperationClass: "write-cs",
		TargetEntity:   targetEntity,
		Params:         buildChangeSetParams(&params),
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}

// EmergencyApply handles POST /api/v1/changeset/emergency-apply
func (h *Handlers) EmergencyApply(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var params SubmitChangeSetParams
	if err := parseJSONBody(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	params.IsEmergency = true

	targetEntity := ""
	if len(params.FieldChanges) > 0 {
		targetEntity = params.FieldChanges[0].EntityType
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "emergency_apply",
		OperationClass: "write-cs",
		TargetEntity:   targetEntity,
		Params:         buildChangeSetParams(&params),
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildChangeSetParams converts the parsed submission into the params map
// that the gate pipeline and step_execute consume.
func buildChangeSetParams(params *SubmitChangeSetParams) map[string]interface{} {
	// convert field changes to the []interface{} form step_execute expects
	fieldChanges := make([]interface{}, 0, len(params.FieldChanges))
	for _, fc := range params.FieldChanges {
		fieldChanges = append(fieldChanges, map[string]interface{}{
			"entity_type":   fc.EntityType,
			"entity_id":     fc.EntityID,
			"field_name":    fc.FieldName,
			"before_value":  fc.BeforeValue,
			"after_value":   fc.AfterValue,
			"change_type":   fc.ChangeType,
			"version_stamp": fc.VersionStamp,
		})
	}

	gateParams := map[string]interface{}{
		"site_id":       params.SiteID,
		"name":          params.Name,
		"description":   params.Description,
		"reason":        params.Reason,
		"field_changes": fieldChanges,
		"is_emergency":  params.IsEmergency,
		"is_bulk":       params.IsBulk,
		"dry_run":       params.DryRun,
	}

	if params.TicketRef != nil {
		gateParams["ticket_authority_pointer_id"] = *params.TicketRef
	}

	return gateParams
}
