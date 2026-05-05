
// === opsdb-api/gate/step_policy.go ===
package gate

// StepPolicyEvaluate is gate step 5: Policy Evaluation.
// Consults policy rows for semantic invariants, data classification,
// retention, separation of duty, and other governance rules.
func StepPolicyEvaluate(ctx *GateContext) error {
	// TODO: read policy rows relevant to the target entity:
	//   policies linked via service_policy, machine_policy, k8s_namespace_policy, cloud_account_policy
	//   policies of type semantic_invariant matching entity type
	//
	// TODO: evaluate semantic invariants (cross-field constraints):
	//   "min_replicas <= max_replicas"
	//   "if status = decommissioned then decommissioned_time must be set"
	//   these are data rows, not hardcoded checks
	//
	// TODO: check data classification consistency:
	//   field classification not higher than entity classification
	//
	// TODO: check retention policy compatibility
	//
	// TODO: check SoD if relevant (submitter != approver checked at approval time,
	//       but SoD policy may restrict other combinations)
	//
	// TODO: policy violations either block (fail-closed) or produce warnings
	//       based on policy configuration
	// TODO: set ctx.PolicyResult
	return nil
}
