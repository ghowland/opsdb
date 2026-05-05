
// === opsdb-api/gate/step_changemgmt.go ===
package gate

// StepChangeManagementRoute is gate step 7: Change Management Routing.
// Evaluates approval rules, walks ownership and stakeholder bridges,
// computes required approvals, determines auto-approve vs human approval.
// Only runs for write-change-set operations.
func StepChangeManagementRoute(ctx *GateContext) error {
	// TODO: if operation class is not write-change-set: skip
	//
	// TODO: step SR1: enumerate field changes from the proposed change set
	//   list of (entity_type, entity_id, field_name) tuples
	//
	// TODO: step SR2: walk ownership bridges
	//   for each touched entity:
	//     read service_ownership / machine_ownership / k8s_cluster_ownership / cloud_resource_ownership
	//     collect responsible ops_user_role rows
	//
	// TODO: step SR3: walk stakeholder bridges
	//   read service_stakeholder + other stakeholder bridges
	//   collect additional interested roles
	//
	// TODO: step SR4: evaluate approval rules
	//   read approval_rule policy rows
	//   match against entity types, namespaces, fields, metadata
	//     (data classification, security zone, compliance scope)
	//   each matching rule produces an approval requirement
	//
	// TODO: step SR5: compute requirements
	//   create change_set_approval_required entries:
	//     one per matching rule with group_id, approver_count_required
	//
	// TODO: check auto-approval policies:
	//   if all requirements satisfiable by auto-approval → set AutoApproved = true
	//   otherwise → ApprovalRequired with the requirement list
	//
	// TODO: set ctx.CMRouting
	return nil
}

