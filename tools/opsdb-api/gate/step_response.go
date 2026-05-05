//# tools/opsdb-api/gate/step_response.go

package gate

// stepResponse is gate step 10: Response Construction.
// Assembles the API response from GateContext. Always runs regardless
// of whether prior steps rejected.
func stepResponse(ctx *GateContext) *GateResponse {
	if ctx.Rejected {
		return buildRejectionResponse(ctx)
	}
	return buildSuccessResponse(ctx)
}

// buildRejectionResponse constructs the response for a rejected request.
func buildRejectionResponse(ctx *GateContext) *GateResponse {
	resp := &GateResponse{
		Success:      false,
		Error:        ctx.RejectionError,
		AuditEntryID: ctx.AuditEntryID,
	}

	// include warnings even on rejection — they may help diagnose
	// or provide additional context
	if len(ctx.Warnings) > 0 {
		resp.Warnings = ctx.Warnings
	}

	return resp
}

// buildSuccessResponse constructs the response for a successful request.
func buildSuccessResponse(ctx *GateContext) *GateResponse {
	resp := &GateResponse{
		Success:      true,
		AuditEntryID: ctx.AuditEntryID,
		Warnings:     ctx.Warnings,
		Metadata:     make(map[string]interface{}),
	}

	// populate result data from execution
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

	// include computed approvals for change set submissions so the
	// caller knows what approval requirements were created
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

	// include version info for versioned entity writes
	if ctx.VersionInfo != nil {
		resp.Metadata["version_serial"] = ctx.VersionInfo.NextSerial
		if ctx.VersionInfo.ParentVID > 0 {
			resp.Metadata["parent_version_id"] = ctx.VersionInfo.ParentVID
		}
	}

	// include omitted fields for reads where classification filtered results
	if ctx.AuthzResult != nil && len(ctx.AuthzResult.OmittedFields) > 0 {
		resp.Metadata["omitted_fields"] = ctx.AuthzResult.OmittedFields
	}

	return resp
}
