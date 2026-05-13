//# tools/opsdb-api/gate/step_audit.go

package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghowland/opsdb/internal/pg"
)

// stepAuditLog is gate step 8: Audit Logging.
// Constructs and writes an audit_log_entry row. Runs on BOTH success
// and rejection paths — every API interaction must be recorded regardless
// of outcome. This is the architectural commitment from OPSDB-6 §9:
// the audit log is the system's queryable memory of who did what.
//
// The audit_log_entry table is append-only: no role has UPDATE or DELETE
// permission. The insert happens outside the operation transaction so
// that audit entries commit even if the operation transaction rolls back.
//
// This step never rejects. If the audit insert itself fails, the failure
// is recorded as a warning on the context — the operation may still
// succeed, but the audit gap must be investigated.
func stepAuditLog(ctx *GateContext) {
	entry := buildAuditEntry(ctx)

	// Compute tamper-evidence chain hash if enabled by compliance policy.
	// Each entry's hash covers its own contents plus the previous entry's
	// hash, forming a chain where modifying any historical entry breaks
	// all subsequent hashes.
	chainHash := computeChainHash(ctx.DB, entry)
	if chainHash != "" {
		entry["_audit_chain_hash"] = chainHash
	}

	auditID, err := insertAuditEntry(ctx.DB, entry)
	if err != nil {
		// Audit logging failure is serious but must not prevent the
		// response from reaching the caller. Record as a warning so
		// the response includes the gap signal and operators can
		// investigate.
		warn(ctx, fmt.Sprintf("audit log insert failed: %v", err))
		return
	}

	ctx.AuditEntryID = auditID
}

// buildAuditEntry constructs the audit_log_entry field map from the
// gate context. Captures: operation identity, caller identity, target
// identity, HTTP method, request summary, response status, and
// change_set correlation where applicable.
func buildAuditEntry(ctx *GateContext) map[string]interface{} {
	entry := map[string]interface{}{
		"api_endpoint":       ctx.Request.Operation,
		"action_type":        deriveActionType(ctx.Request.Operation, ctx.Request.OperationClass),
		"http_method":        deriveHTTPMethod(ctx.Request.OperationClass),
		"target_entity_type": ctx.Request.TargetEntity,
		"client_ip_address":  ctx.Request.ClientIP,
		"client_user_agent":  ctx.Request.UserAgent,
	}

	// Target entity ID — zero for creates and searches, which become
	// SQL NULL via the nil interface value.
	if ctx.Request.TargetEntityID > 0 {
		entry["target_entity_id"] = ctx.Request.TargetEntityID
	}

	// Caller identity — set whichever fields are available. Human
	// callers have OpsUserID. Runner callers have RunnerMachineID.
	// Web-mediated writes have both.
	if ctx.Identity != nil {
		if ctx.Identity.OpsUserID != nil {
			entry["acting_ops_user_id"] = *ctx.Identity.OpsUserID
		}
		if ctx.Identity.RunnerMachineID != nil {
			entry["acting_service_account_id"] = *ctx.Identity.RunnerMachineID
		}
	}

	// Request data summary — param keys and operation metadata, never
	// param values (which may contain sensitive data).
	requestSummary := summarizeRequest(ctx.Request)
	if requestSummary != "" {
		entry["request_data_summary"] = requestSummary
	}

	// Response status and summary depend on pipeline state at the time
	// audit runs. On the rejection path, we have the rejection error.
	// On the success path, audit runs before execute (step 8 before
	// step 9), so ExecutionResult may not be populated yet — we record
	// "pending" and the actual outcome is visible in the execution's
	// own data trail (runner_job rows, change_set status, etc).
	if ctx.Rejected {
		entry["response_status"] = deriveErrorStatus(ctx.RejectionError)
		entry["response_data_summary"] = summarizeRejection(ctx.RejectionError)
	} else if ctx.ExecutionResult != nil {
		entry["response_status"] = "success"
		entry["response_data_summary"] = summarizeExecution(ctx.ExecutionResult)
	} else {
		entry["response_status"] = "pending"
	}

	// Change set ID for correlation when the operation produced or
	// acted on a change set.
	if ctx.ExecutionResult != nil && ctx.ExecutionResult.ChangeSetID > 0 {
		entry["change_set_id"] = ctx.ExecutionResult.ChangeSetID
	}

	return entry
}

// insertAuditEntry writes one row to audit_log_entry. The table is
// append-only at the DDL level — no UPDATE or DELETE permission for
// any role including substrate operators. The insert uses the DB
// handle directly (not within the operation transaction) so the
// audit entry commits independently.
func insertAuditEntry(db *pg.DB, entry map[string]interface{}) (int, error) {
	columns := make([]string, 0, len(entry))
	placeholders := make([]string, 0, len(entry))
	values := make([]interface{}, 0, len(entry))

	i := 1
	for col, val := range columns {
		_ = val // suppress unused; real iteration below
		_ = col
		break
	}

	// Build column list, placeholder list, and value list in lockstep.
	i = 1
	for col, val := range entry {
		columns = append(columns, pg.QuoteIdentifier(col))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		values = append(values, val)
		i++
	}

	// acted_time and created_time are set server-side via NOW() so the
	// timestamp comes from the database clock, not the application clock.
	columns = append(columns, pg.QuoteIdentifier("acted_time"))
	placeholders = append(placeholders, "NOW()")

	columns = append(columns, pg.QuoteIdentifier("created_time"))
	placeholders = append(placeholders, "NOW()")

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING id",
		pg.QuoteIdentifier("audit_log_entry"),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	var auditID int
	err := db.QueryRow(query, values...).Scan(&auditID)
	if err != nil {
		return 0, fmt.Errorf("audit_log_entry INSERT failed: %w", err)
	}

	return auditID, nil
}

// ---------------------------------------------------------------------------
// Chain hashing for tamper evidence
// ---------------------------------------------------------------------------

// computeChainHash computes the tamper-evidence hash for an audit entry.
// Each entry's hash covers its own contents plus the previous entry's
// hash, forming a chain where modifying any historical entry breaks all
// subsequent hashes. Returns empty string if chain hashing is not enabled
// or if the computation fails (non-fatal — the entry is still inserted
// without a chain hash).
func computeChainHash(db *pg.DB, entry map[string]interface{}) string {
	if !isChainHashEnabled(db) {
		return ""
	}

	// Read the most recent audit entry's chain hash to link to.
	previousHash := readPreviousChainHash(db)

	// Serialize entry contents deterministically for hashing.
	// json.Marshal produces deterministic output for a given map
	// (sorted keys in Go 1.12+).
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return ""
	}

	hasher := sha256.New()
	hasher.Write(entryJSON)
	hasher.Write([]byte(previousHash))
	return hex.EncodeToString(hasher.Sum(nil))
}

// readPreviousChainHash reads the _audit_chain_hash from the most recent
// audit log entry that has one. Returns empty string if no previous
// entry has a chain hash (first entry in the chain).
func readPreviousChainHash(db *pg.DB) string {
	var previousHash string
	err := db.QueryRow(
		"SELECT _audit_chain_hash FROM audit_log_entry " +
			"WHERE _audit_chain_hash IS NOT NULL " +
			"ORDER BY id DESC LIMIT 1",
	).Scan(&previousHash)
	if err != nil {
		// No previous entry with chain hash — this will be the first
		// link in the chain. Not an error.
		return ""
	}
	return previousHash
}

// isChainHashEnabled checks whether tamper-evidence chain hashing is
// enabled. This is controlled by a compliance_scope policy row with
// audit_chain_hash_enabled set to true in its policy_data_json.
// Returns false if the policy doesn't exist or can't be read.
func isChainHashEnabled(db *pg.DB) bool {
	var enabled bool
	err := db.QueryRow(
		"SELECT EXISTS(" +
			"SELECT 1 FROM policy " +
			"WHERE policy_type = 'compliance_scope' " +
			"AND is_active = true " +
			"AND policy_data_json->>'audit_chain_hash_enabled' = 'true'" +
			")",
	).Scan(&enabled)
	if err != nil {
		return false
	}
	return enabled
}

// ---------------------------------------------------------------------------
// Action type and status derivation
// ---------------------------------------------------------------------------

// deriveActionType maps an operation name and class to the action_type
// value stored in audit_log_entry. Each operation gets a specific action
// type for precise audit querying.
func deriveActionType(operation string, operationClass string) string {
	switch operation {
	case "get_entity", "get_entity_history", "get_entity_at_time",
		"search", "get_dependencies", "resolve_authority_pointer",
		"change_set_view":
		return "read"
	case "write_observation":
		return "create"
	case "submit_change_set":
		return "change_set_submit"
	case "bulk_submit_change_set":
		return "change_set_submit"
	case "emergency_apply":
		return "change_set_submit"
	case "approve_change_set":
		return "approve"
	case "reject_change_set":
		return "reject"
	case "cancel_change_set":
		return "change_set_submit"
	case "apply_change_set_field_change":
		return "update"
	case "mark_change_set_applied":
		return "update"
	case "watch":
		return "read"
	default:
		return operationClass
	}
}

// deriveHTTPMethod maps an operation class to the expected HTTP method.
// Used for the http_method field in the audit log entry.
func deriveHTTPMethod(operationClass string) string {
	switch operationClass {
	case "read", "stream":
		return "GET"
	default:
		return "POST"
	}
}

// deriveErrorStatus maps a gate rejection error to a status string for
// the response_status field in the audit log entry.
func deriveErrorStatus(err *GateError) string {
	if err == nil {
		return "unknown_error"
	}
	switch err.Code {
	case "authentication_failed", "auth_not_configured":
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
	case "execution_failed":
		return "execution_failed"
	default:
		return "rejected"
	}
}

// ---------------------------------------------------------------------------
// Request and response summarization
// ---------------------------------------------------------------------------

// summarizeRequest creates a brief summary of request parameters for the
// audit log. Includes operation name, target entity, target ID, and param
// keys — but never param values, which may contain sensitive data.
func summarizeRequest(req *GateRequest) string {
	summary := map[string]interface{}{
		"operation":     req.Operation,
		"target_entity": req.TargetEntity,
	}

	if req.TargetEntityID > 0 {
		summary["target_id"] = req.TargetEntityID
	}

	// Include param keys but not values to avoid storing sensitive data
	// in the audit log's summary field.
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

// summarizeRejection creates a brief summary of a gate rejection for
// the response_data_summary field in the audit log entry.
func summarizeRejection(err *GateError) string {
	if err == nil {
		return "rejected: unknown"
	}
	summary := map[string]interface{}{
		"step":    err.StepName,
		"code":    err.Code,
		"message": err.Message,
	}
	data, jsonErr := json.Marshal(summary)
	if jsonErr != nil {
		return fmt.Sprintf("rejected at step %d (%s): %s",
			err.Step, err.StepName, err.Code)
	}
	return string(data)
}

// summarizeExecution creates a brief summary of execution results for
// the response_data_summary field in the audit log entry.
func summarizeExecution(result *ExecutionResult) string {
	if result == nil {
		return ""
	}
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
