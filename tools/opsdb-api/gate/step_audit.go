//# tools/opsdb-api/gate/step_audit.go

package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/ghowland/opsdb/internal/pg"
)

// stepAuditLog is gate step 8: Audit Logging.
// Constructs and writes an audit_log_entry. Runs on BOTH success and
// rejection paths. This step never rejects — its job is to record
// what happened regardless of outcome.
func stepAuditLog(ctx *GateContext) {
	entry := buildAuditEntry(ctx)

	// compute tamper-evidence hash if enabled
	chainHash := computeChainHash(ctx.DB, entry)
	if chainHash != "" {
		entry["_audit_chain_hash"] = chainHash
	}

	auditID, err := insertAuditEntry(ctx.DB, entry)
	if err != nil {
		// audit logging failure is serious but must not prevent the
		// response from reaching the caller. Record as a warning.
		warn(ctx, fmt.Sprintf("audit logging failed: %v", err))
		return
	}

	ctx.AuditEntryID = auditID
}

// buildAuditEntry constructs the audit_log_entry field map from gate context.
func buildAuditEntry(ctx *GateContext) map[string]interface{} {
	entry := map[string]interface{}{
		"api_endpoint":       ctx.Request.Operation,
		"action_type":        deriveActionType(ctx.Request.Operation, ctx.Request.OperationClass),
		"target_entity_type": ctx.Request.TargetEntity,
		"client_ip_address":  ctx.Request.ClientIP,
		"client_user_agent":  ctx.Request.UserAgent,
		"request_id":         ctx.Request.RequestID,
	}

	// target entity ID — 0 for creates and searches
	if ctx.Request.TargetEntityID > 0 {
		entry["target_entity_id"] = ctx.Request.TargetEntityID
	}

	// caller identity — set whichever fields are available
	if ctx.Identity != nil {
		if ctx.Identity.OpsUserID != nil {
			entry["acting_ops_user_id"] = *ctx.Identity.OpsUserID
		}
		if ctx.Identity.RunnerMachineID != nil {
			entry["acting_service_account_id"] = *ctx.Identity.RunnerMachineID
		}
	}

	// HTTP method derived from operation class
	entry["http_method"] = deriveHTTPMethod(ctx.Request.OperationClass)

	// request data summary — summarize params, not full payload
	requestSummary := summarizeRequest(ctx.Request)
	if requestSummary != "" {
		entry["request_data_summary"] = requestSummary
	}

	// response status and summary depend on whether we rejected
	if ctx.Rejected {
		entry["response_status"] = deriveErrorStatus(ctx.RejectionError)
		entry["response_data_summary"] = summarizeRejection(ctx.RejectionError)
	} else if ctx.ExecutionResult != nil {
		entry["response_status"] = "success"
		entry["response_data_summary"] = summarizeExecution(ctx.ExecutionResult)
	} else {
		// pipeline hasn't reached execution yet (audit runs before execute
		// on the success path); status will be updated post-execute if needed
		entry["response_status"] = "pending"
	}

	// change set ID for correlation when applicable
	if ctx.CMRouting != nil && ctx.ExecutionResult != nil && ctx.ExecutionResult.ChangeSetID > 0 {
		entry["change_set_id"] = ctx.ExecutionResult.ChangeSetID
	}

	return entry
}

// insertAuditEntry writes one row to audit_log_entry. The table is
// append-only: no role has UPDATE or DELETE permission.
func insertAuditEntry(db *pg.DB, entry map[string]interface{}) (int, error) {
	columns := make([]string, 0, len(entry))
	placeholders := make([]string, 0, len(entry))
	values := make([]interface{}, 0, len(entry))

	i := 1
	for col, val := range entry {
		columns = append(columns, pg.QuoteIdentifier(col))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	query := fmt.Sprintf(
		"INSERT INTO audit_log_entry (%s) VALUES (%s) RETURNING id",
		joinStrings(columns, ", "),
		joinStrings(placeholders, ", "),
	)

	var auditID int
	err := db.QueryRow(query, values...).Scan(&auditID)
	if err != nil {
		return 0, fmt.Errorf("audit_log_entry INSERT failed: %w", err)
	}

	return auditID, nil
}

// computeChainHash computes the tamper-evidence hash for an audit entry.
// Each entry's hash covers its own contents plus the previous entry's hash,
// forming a chain where modifying any historical entry breaks all subsequent
// hashes. Returns empty string if chain hashing is not enabled or if this
// is the first entry.
func computeChainHash(db *pg.DB, entry map[string]interface{}) string {
	// read the most recent audit entry's chain hash
	var previousHash string
	err := db.QueryRow(
		"SELECT _audit_chain_hash FROM audit_log_entry " +
			"WHERE _audit_chain_hash IS NOT NULL " +
			"ORDER BY id DESC LIMIT 1",
	).Scan(&previousHash)
	if err != nil {
		// no previous entry with chain hash, or chain hashing not enabled;
		// check if we should start a new chain
		if !isChainHashEnabled(db) {
			return ""
		}
		previousHash = ""
	}

	// serialize entry contents deterministically for hashing
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return ""
	}

	hasher := sha256.New()
	hasher.Write(entryJSON)
	hasher.Write([]byte(previousHash))
	return hex.EncodeToString(hasher.Sum(nil))
}

// isChainHashEnabled checks whether tamper-evidence chain hashing is
// enabled via compliance regime policy.
func isChainHashEnabled(db *pg.DB) bool {
	var enabled bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM policy "+
			"WHERE policy_type = 'compliance_scope' "+
			"AND is_active = true "+
			"AND policy_data_json->>'audit_chain_hash_enabled' = 'true')",
	).Scan(&enabled)
	if err != nil {
		return false
	}
	return enabled
}

// deriveActionType maps operation name and class to a structured action type
// for the audit log.
func deriveActionType(operation string, operationClass string) string {
	switch operation {
	case "get_entity", "get_entity_history", "get_entity_at_time",
		"search", "get_dependencies", "resolve_authority_pointer",
		"change_set_view":
		return "read"
	case "write_observation":
		return "write_observation"
	case "submit_change_set":
		return "submit_change_set"
	case "bulk_submit_change_set":
		return "bulk_submit_change_set"
	case "emergency_apply":
		return "emergency_apply"
	case "approve_change_set":
		return "approve"
	case "reject_change_set":
		return "reject"
	case "cancel_change_set":
		return "cancel"
	case "apply_change_set_field_change":
		return "apply_field_change"
	case "mark_change_set_applied":
		return "mark_applied"
	case "watch":
		return "watch_subscribe"
	default:
		return operationClass
	}
}

// deriveHTTPMethod maps operation class to the expected HTTP method.
func deriveHTTPMethod(operationClass string) string {
	switch operationClass {
	case "read", "stream":
		return "GET"
	case "write-direct", "write-cs":
		return "POST"
	case "cm-action":
		return "POST"
	default:
		return "POST"
	}
}

// deriveErrorStatus maps a gate rejection to a status string for the audit log.
func deriveErrorStatus(err *GateError) string {
	if err == nil {
		return "unknown_error"
	}
	switch err.Code {
	case "authentication_failed":
		return "authentication_failed"
	case "authorization_denied":
		return "authorization_denied"
	case "validation_failed":
		return "validation_failed"
	case "stale_version":
		return "stale_version"
	case "policy_violation":
		return "policy_violation"
	case "rate_limited":
		return "rate_limited"
	default:
		return "rejected"
	}
}

// summarizeRequest creates a brief summary of request parameters for the
// audit log. Does not include full payloads — only keys, entity types,
// and counts to keep audit entries bounded.
func summarizeRequest(req *GateRequest) string {
	summary := map[string]interface{}{
		"operation":     req.Operation,
		"target_entity": req.TargetEntity,
	}
	if req.TargetEntityID > 0 {
		summary["target_id"] = req.TargetEntityID
	}

	// include param keys but not values to avoid storing sensitive data
	if len(req.Params) > 0 {
		keys := make([]string, 0, len(req.Params))
		for k := range req.Params {
			keys = append(keys, k)
		}
		summary["param_keys"] = keys
	}

	data, err := json.Marshal(summary)
	if err != nil {
		return fmt.Sprintf("operation=%s target=%s", req.Operation, req.TargetEntity)
	}
	return string(data)
}

// summarizeRejection creates a brief summary of a rejection for the audit log.
func summarizeRejection(err *GateError) string {
	if err == nil {
		return "rejected: unknown"
	}
	summary := map[string]interface{}{
		"step":      err.StepName,
		"code":      err.Code,
		"message":   err.Message,
	}
	data, jsonErr := json.Marshal(summary)
	if jsonErr != nil {
		return fmt.Sprintf("rejected at step %d (%s): %s", err.Step, err.StepName, err.Code)
	}
	return string(data)
}

// summarizeExecution creates a brief summary of execution results for
// the audit log.
func summarizeExecution(result *ExecutionResult) string {
	summary := map[string]interface{}{
		"affected_rows": len(result.AffectedRowIDs),
		"version_rows":  len(result.VersionRowIDs),
	}
	if result.ChangeSetID > 0 {
		summary["change_set_id"] = result.ChangeSetID
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return fmt.Sprintf("affected=%d", len(result.AffectedRowIDs))
	}
	return string(data)
}

// joinStrings joins a string slice with a separator.
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}
