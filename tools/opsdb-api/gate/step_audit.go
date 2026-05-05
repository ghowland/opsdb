
// === opsdb-api/gate/step_audit.go ===
package gate

// StepAuditLog is gate step 8: Audit Logging.
// Constructs and writes audit_log_entry. Runs on BOTH success and rejection paths.
// Atomic with the operation outcome (within the same transaction).
func StepAuditLog(ctx *GateContext) error {
	// TODO: construct audit_log_entry row:
	//   site_id: from context
	//   acting_ops_user_id: from ctx.Identity (nil for runner-only)
	//   acting_service_account_id: from ctx.Identity (nil for human-only)
	//   api_endpoint: from ctx.Request.Operation
	//   http_method: derived from operation class
	//   action_type: from operation class (read, create, update, delete, approve, reject, etc.)
	//   target_entity_type: from ctx.Request.TargetEntity
	//   target_entity_id: from ctx.Request.TargetEntityID
	//   request_data_summary: summarize request params (not full payload)
	//   response_status: HTTP status code (200, 400, 401, 403, etc.)
	//   response_data_summary: summarize response (affected IDs or error)
	//   client_ip_address: from ctx.Request.ClientIP
	//   client_user_agent: from ctx.Request.UserAgent
	//   acted_time: NOW() from database (API-supplied, not client-supplied)
	//
	// TODO: if tamper evidence enabled:
	//   compute _audit_chain_hash = hash(entry contents + previous entry hash)
	//
	// TODO: INSERT into audit_log_entry (append-only table)
	// TODO: set ctx.AuditEntryID for correlation in response
	//
	// TODO: this step NEVER rejects — it records what happened
	return nil
}

