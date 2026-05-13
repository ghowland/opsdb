//# tools/opsdb_api/gate/step_response.go

package gate

// stepResponse is gate step 10: Response Construction.
// Assembles the GateResponse from the accumulated GateContext. Always
// runs as the final step regardless of whether prior steps rejected.
// Dispatches to rejection or success builder based on ctx.Rejected.
func stepResponse(ctx *GateContext) *GateResponse {
	if ctx.Rejected {
		return buildRejectionResponse(ctx)
	}
	return buildSuccessResponse(ctx)
}

// buildRejectionResponse constructs the response for a rejected request.
// The rejection error from whichever step halted the pipeline is included
// as the structured error. Warnings are included even on rejection — they
// may help diagnose or provide additional context (e.g., an audit log
// insert failure warning alongside an authentication rejection).
func buildRejectionResponse(ctx *GateContext) *GateResponse {
	resp := &GateResponse{
		Success:      false,
		Error:        ctx.RejectionError,
		AuditEntryID: ctx.AuditEntryID,
	}

	if len(ctx.Warnings) > 0 {
		resp.Warnings = ctx.Warnings
	}

	return resp
}

// buildSuccessResponse constructs the response for a successful request.
// Populates metadata from execution results, change management routing,
// versioning info, and authorization filtering — giving the caller full
// visibility into what happened and what governance was applied.
func buildSuccessResponse(ctx *GateContext) *GateResponse {
	resp := &GateResponse{
		Success:      true,
		AuditEntryID: ctx.AuditEntryID,
		Warnings:     ctx.Warnings,
		Metadata:     make(map[string]interface{}),
	}

	// Execution result — what was written to the database
	if ctx.ExecutionResult != nil {
		resp.Data = ctx.ExecutionResult

		if len(ctx.ExecutionResult.AffectedRowIDs) > 0 {
			resp.Metadata["affected_row_ids"] = ctx.ExecutionResult.AffectedRowIDs
		}
		if len(ctx.ExecutionResult.VersionRowIDs) > 0 {
			resp.Metadata["version_row_ids"] = ctx.ExecutionResult.VersionRowIDs
		}
		if ctx.ExecutionResult.ChangeSetID > 0 {
			resp.Metadata["change_set_id"] = ctx.ExecutionResult.ChangeSetID
		}
	}

	// Change management routing — included for change set submissions so
	// the caller knows what approval requirements were created and whether
	// the change set was auto-approved.
	if ctx.CMRouting != nil {
		if ctx.CMRouting.AutoApproved {
			resp.Metadata["auto_approved"] = true
		}
		if len(ctx.CMRouting.ApprovalRequired) > 0 {
			approvals := make([]map[string]interface{}, 0, len(ctx.CMRouting.ApprovalRequired))
			for _, req := range ctx.CMRouting.ApprovalRequired {
				approvals = append(approvals, map[string]interface{}{
					"rule_id":        req.RuleID,
					"group_id":       req.GroupID,
					"group_name":     req.GroupName,
					"count_required": req.CountRequired,
				})
			}
			resp.Metadata["approval_requirements"] = approvals
		}
	}

	// Version info — included for writes to versioned entities so the
	// caller knows what version serial was assigned and what the parent
	// version was.
	if ctx.VersionInfo != nil {
		resp.Metadata["version_serial"] = ctx.VersionInfo.NextSerial
		if ctx.VersionInfo.ParentVID > 0 {
			resp.Metadata["parent_version_id"] = ctx.VersionInfo.ParentVID
		}
	}

	// Omitted fields — included for reads where authorization layer 3
	// (per-field access classification) filtered fields from the result.
	// The caller sees which fields were omitted by access policy versus
	// which were genuinely absent, per OPSDB-6 §4.4.
	if ctx.AuthzResult != nil && len(ctx.AuthzResult.OmittedFields) > 0 {
		resp.Metadata["omitted_fields"] = ctx.AuthzResult.OmittedFields
	}

	return resp
}
