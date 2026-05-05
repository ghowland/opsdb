
// === opsdb-api/gate/step_authz.go ===
package gate

// StepAuthorize is gate step 2: Authorization.
// Evaluates five layers with AND composition. First denial halts.
// Records which layer denied and which policy triggered it.
func StepAuthorize(ctx *GateContext) error {
	// TODO: Layer 1: Standard Role and Group
	//   read ops_user_role_member + ops_group_member for caller
	//   check role permits operation class (read, write-direct, write-cs, cm-action)
	//   if denied: reject with layer=1
	//
	// TODO: Layer 2: Per-Entity Governance
	//   read _requires_group on target entity row (if it exists)
	//   check caller is member of required group
	//   if denied: reject with layer=2
	//
	// TODO: Layer 3: Per-Field Classification
	//   read _access_classification on target fields/table
	//   check caller clearance >= classification
	//   if insufficient for specific fields: add to OmittedFields (reads) or reject (writes)
	//   if denied: reject with layer=3
	//
	// TODO: Layer 4: Per-Runner Authority
	//   if caller is runner:
	//     read runner_capability rows
	//     read runner_*_target bridge rows (service, namespace, cloud_account, host_group)
	//     check operation target within declared scope
	//     if denied: reject with layer=4
	//
	// TODO: Layer 5: Policy Rules
	//   read policy rows of type access_control
	//   evaluate time-of-day, SoD, tenure, IP restrictions
	//   if denied: reject with layer=5
	//   if additional approval needed: add to ctx for step 7
	//
	// TODO: set ctx.AuthzResult
	return nil
}

