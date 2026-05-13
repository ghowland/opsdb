//# tools/opsdb_api/operations/changeset_actions.go

package operations

import (
	"net/http"
	"time"

	"github.com/ghowland/opsdb/tools/opsdb_api/gate"
)

// ---------------------------------------------------------------------------
// HTTP handlers — change management actions
// ---------------------------------------------------------------------------

// ApproveChangeSet handles POST /api/v1/changeset/approve
func (h *Handlers) ApproveChangeSet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		ChangeSetID int    `json:"change_set_id"`
		Comment     string `json:"comment"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "approve_change_set",
		OperationClass: "cm-action",
		TargetEntity:   "change_set",
		TargetEntityID: body.ChangeSetID,
		Params: map[string]interface{}{
			"change_set_id": body.ChangeSetID,
			"comment":       body.Comment,
		},
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}

// RejectChangeSet handles POST /api/v1/changeset/reject
func (h *Handlers) RejectChangeSet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		ChangeSetID int    `json:"change_set_id"`
		Reason      string `json:"reason"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "reject_change_set",
		OperationClass: "cm-action",
		TargetEntity:   "change_set",
		TargetEntityID: body.ChangeSetID,
		Params: map[string]interface{}{
			"change_set_id": body.ChangeSetID,
			"reason":        body.Reason,
		},
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}

// CancelChangeSet handles POST /api/v1/changeset/cancel
func (h *Handlers) CancelChangeSet(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		ChangeSetID int `json:"change_set_id"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "cancel_change_set",
		OperationClass: "cm-action",
		TargetEntity:   "change_set",
		TargetEntityID: body.ChangeSetID,
		Params: map[string]interface{}{
			"change_set_id": body.ChangeSetID,
		},
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}

// ApplyFieldChange handles POST /api/v1/changeset/apply-field-change
func (h *Handlers) ApplyFieldChange(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		ChangeSetID   int `json:"change_set_id"`
		FieldChangeID int `json:"field_change_id"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "apply_change_set_field_change",
		OperationClass: "cm-action",
		TargetEntity:   "change_set",
		TargetEntityID: body.ChangeSetID,
		Params: map[string]interface{}{
			"change_set_id":   body.ChangeSetID,
			"field_change_id": body.FieldChangeID,
		},
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}

// MarkApplied handles POST /api/v1/changeset/mark-applied
func (h *Handlers) MarkApplied(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		ChangeSetID int `json:"change_set_id"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "mark_change_set_applied",
		OperationClass: "cm-action",
		TargetEntity:   "change_set",
		TargetEntityID: body.ChangeSetID,
		Params: map[string]interface{}{
			"change_set_id": body.ChangeSetID,
		},
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}
