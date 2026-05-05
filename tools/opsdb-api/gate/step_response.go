// === opsdb-api/gate/step_response.go ===
package gate

// StepResponse is gate step 10: Response Construction.
// Assembles the API response from GateContext. Always runs.
func StepResponse(ctx *GateContext) *GateResponse {
	// TODO: if ctx.Rejected:
	//   return GateResponse{
	//     Success: false,
	//     Error: ctx.RejectionError,
	//     AuditEntryID: ctx.AuditEntryID,
	//   }
	//
	// TODO: if success:
	//   return GateResponse{
	//     Success: true,
	//     Data: ctx.ExecutionResult (or read result),
	//     AuditEntryID: ctx.AuditEntryID,
	//     Warnings: ctx.Warnings,
	//     Metadata: {
	//       affected_row_ids: from execution,
	//       computed_approvals: from CM routing (for change set submissions),
	//       version_info: from versioning step,
	//     },
	//   }
	return nil
}

