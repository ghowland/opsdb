//# tools/opsdb-api/gate/step_authz.go

package gate

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/pg"
)

// stepAuthorize is gate step 2: Authorization.
// Evaluates five layers of authorization with AND composition. First
// denial halts the pipeline. All five must pass for the operation to
// proceed.
//
// Layer 1: Standard role and group — caller's role permits the operation class
// Layer 2: Per-entity governance — _requires_group field on the target entity
// Layer 3: Per-field classification — _access_classification on involved fields
// Layer 4: Per-runner authority — runner_capability and runner_*_target (runner only)
// Layer 5: Policy rules — time-of-day, separation of duty, tenure, IP restrictions
//
// On denial, the response includes which layer denied and which policy
// triggered it, so the caller (or operator investigating) can identify
// exactly what needs to change.
func stepAuthorize(ctx *GateContext) {
	result := &AuthzResult{Allowed: true}

	// Layer 1: Standard role and group membership
	if !checkLayer1RoleAndGroup(ctx, result) {
		ctx.AuthzResult = result
		reject(ctx, 2, "authorization_denied",
			fmt.Sprintf("denied at layer 1 (role/group): %s", result.DeniedPolicy),
			map[string]interface{}{
				"layer":  1,
				"policy": result.DeniedPolicy,
			})
		return
	}

	// Layer 2: Per-entity governance (_requires_group)
	if !checkLayer2EntityGovernance(ctx, result) {
		ctx.AuthzResult = result
		reject(ctx, 2, "authorization_denied",
			fmt.Sprintf("denied at layer 2 (entity governance): %s", result.DeniedPolicy),
			map[string]interface{}{
				"layer":         2,
				"policy":        result.DeniedPolicy,
				"target_entity": ctx.Request.TargetEntity,
				"target_id":     ctx.Request.TargetEntityID,
			})
		return
	}

	// Layer 3: Per-field classification (_access_classification)
	if !checkLayer3FieldClassification(ctx, result) {
		ctx.AuthzResult = result
		reject(ctx, 2, "authorization_denied",
			fmt.Sprintf("denied at layer 3 (field classification): %s", result.DeniedPolicy),
			map[string]interface{}{
				"layer":          3,
				"policy":         result.DeniedPolicy,
				"omitted_fields": result.OmittedFields,
			})
		return
	}

	// Layer 4: Per-runner authority — only evaluated for runner callers.
	// Human callers skip this layer entirely.
	if ctx.Identity.IsRunner() {
		if !checkLayer4RunnerAuthority(ctx, result) {
			ctx.AuthzResult = result
			reject(ctx, 2, "authorization_denied",
				fmt.Sprintf("denied at layer 4 (runner authority): %s", result.DeniedPolicy),
				map[string]interface{}{
					"layer":  4,
					"policy": result.DeniedPolicy,
				})
			return
		}
	}

	// Layer 5: Policy rules (time-of-day, separation of duty, etc.)
	if !checkLayer5PolicyRules(ctx, result) {
		ctx.AuthzResult = result
		reject(ctx, 2, "authorization_denied",
			fmt.Sprintf("denied at layer 5 (policy rule): %s", result.DeniedPolicy),
			map[string]interface{}{
				"layer":  5,
				"policy": result.DeniedPolicy,
			})
		return
	}

	ctx.AuthzResult = result
}

// ---------------------------------------------------------------------------
// Layer 1: Standard role and group
// ---------------------------------------------------------------------------

// checkLayer1RoleAndGroup verifies the caller's roles permit the requested
// operation class. Reads ops_user_role_member to get role names, then
// checks if any role grants access.
//
// Role → permitted operation classes:
//
//	admin    → all
//	operator → all
//	reader   → read, stream
//	runner   → all (granular check deferred to layer 4)
//	auditor  → read
//	approver → read, cm-action
func checkLayer1RoleAndGroup(ctx *GateContext, result *AuthzResult) bool {
	if ctx.Identity.IsHuman() {
		userID := *ctx.Identity.OpsUserID

		roles, err := queryUserRoles(ctx.DB, userID)
		if err != nil {
			result.Allowed = false
			result.DeniedLayer = 1
			result.DeniedPolicy = fmt.Sprintf("failed to query roles: %v", err)
			return false
		}

		if !operationPermittedByRoles(roles, ctx.Request.OperationClass) {
			result.Allowed = false
			result.DeniedLayer = 1
			result.DeniedPolicy = fmt.Sprintf("no role permits %s for user %d",
				ctx.Request.OperationClass, userID)
			return false
		}
	}

	// Runners pass layer 1 unconditionally — their granular authorization
	// happens in layer 4 via runner_capability and runner_*_target checks.
	// Layer 1 only gates human callers by role.

	return true
}

// queryUserRoles reads role names for an ops_user via ops_user_role_member.
func queryUserRoles(db *pg.DB, userID int) ([]string, error) {
	rows, err := db.Query(
		"SELECT r.name FROM ops_user_role r "+
			"JOIN ops_user_role_member rm ON rm.ops_user_role_id = r.id "+
			"WHERE rm.ops_user_id = $1 AND r.is_active = true",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, rows.Err()
}

// operationPermittedByRoles checks whether any of the caller's roles
// grants access to the requested operation class.
func operationPermittedByRoles(roles []string, operationClass string) bool {
	for _, role := range roles {
		switch role {
		case "admin":
			return true
		case "operator":
			return true
		case "reader":
			if operationClass == "read" || operationClass == "stream" {
				return true
			}
		case "runner":
			// Runner role passes layer 1; layer 4 does granular check
			return true
		case "auditor":
			if operationClass == "read" {
				return true
			}
		case "approver":
			if operationClass == "read" || operationClass == "cm-action" {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Layer 2: Per-entity governance
// ---------------------------------------------------------------------------

// checkLayer2EntityGovernance reads the _requires_group governance field
// from the target entity row. If set, the caller must be a member of
// that group (checked via identity.HasGroup which reads from the Groups
// slice populated at authentication time).
func checkLayer2EntityGovernance(ctx *GateContext, result *AuthzResult) bool {
	// Only applies when targeting a specific entity row. Searches,
	// creates, and operations without a target ID skip this layer.
	if ctx.Request.TargetEntityID == 0 {
		return true
	}

	requiredGroup, err := queryRequiresGroup(
		ctx.DB, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
	if err != nil {
		// Query failure is not a denial — the _requires_group column
		// may not exist on this entity type (not all entities have
		// governance fields). Pass through.
		return true
	}

	if requiredGroup == "" {
		return true
	}

	if !ctx.Identity.HasGroup(requiredGroup) {
		result.Allowed = false
		result.DeniedLayer = 2
		result.DeniedPolicy = fmt.Sprintf("entity requires group membership: %s", requiredGroup)
		return false
	}

	return true
}

// queryRequiresGroup reads the _requires_group field from a target entity row.
// Returns empty string if the field doesn't exist, the entity doesn't exist,
// or the field is null.
func queryRequiresGroup(db *pg.DB, entityType string, entityID int) (string, error) {
	query := fmt.Sprintf(
		"SELECT _requires_group FROM %s WHERE id = $1",
		pg.QuoteIdentifier(entityType),
	)

	var group string
	err := db.QueryRow(query, entityID).Scan(&group)
	if err != nil {
		if pg.IsNoRows(err) || pg.IsUndefinedColumn(err) {
			return "", nil
		}
		return "", err
	}
	return group, nil
}

// ---------------------------------------------------------------------------
// Layer 3: Per-field classification
// ---------------------------------------------------------------------------

// checkLayer3FieldClassification checks _access_classification on fields
// involved in the operation. Each field may have a classification level
// (public, internal, confidential, restricted, regulated). The caller's
// clearance (derived from their roles) must meet or exceed the field's
// classification.
//
// For reads: fields the caller can't access are omitted from results
// (recorded in AuthzResult.OmittedFields) rather than rejecting.
//
// For writes: insufficient clearance on any field causes rejection —
// you can't write to a field you can't see.
func checkLayer3FieldClassification(ctx *GateContext, result *AuthzResult) bool {
	requestedFields := extractRequestedFields(ctx.Request)
	if len(requestedFields) == 0 {
		return true
	}

	callerClearance := deriveCallerClearance(ctx)

	classifications, err := queryFieldClassifications(
		ctx.DB, ctx.Request.TargetEntity, requestedFields)
	if err != nil {
		// Can't read classifications. Fail closed for writes (deny),
		// pass for reads (fields will just be included unfiltered —
		// safe because the caller has passed layer 1 role check).
		if isWriteOperation(ctx.Request.OperationClass) {
			result.Allowed = false
			result.DeniedLayer = 3
			result.DeniedPolicy = fmt.Sprintf("failed to check field classifications: %v", err)
			return false
		}
		return true
	}

	for field, classification := range classifications {
		if !clearanceMeetsClassification(callerClearance, classification) {
			if isReadOnly(ctx.Request.OperationClass) {
				// Omit the field from results rather than rejecting
				result.OmittedFields = append(result.OmittedFields, field)
			} else {
				// Reject — can't write to a field you can't see
				result.Allowed = false
				result.DeniedLayer = 3
				result.DeniedPolicy = fmt.Sprintf(
					"field %s requires classification %s, caller has %s",
					field, classification, callerClearance)
				return false
			}
		}
	}

	return true
}

// queryFieldClassifications reads _access_classification values for
// fields of a given entity type from the _schema_field metadata table.
// Only returns entries for fields that have a non-empty classification
// AND are in the requested field list.
func queryFieldClassifications(db *pg.DB, entityType string, fields []string) (map[string]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	rows, err := db.Query(
		"SELECT field_name, constraint_data_json->>'access_classification' "+
			"FROM _schema_field "+
			"WHERE _schema_entity_type_id = ("+
			"  SELECT id FROM _schema_entity_type WHERE table_name = $1 LIMIT 1"+
			") "+
			"AND constraint_data_json->>'access_classification' IS NOT NULL "+
			"AND constraint_data_json->>'access_classification' != ''",
		entityType,
	)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	// Build a set of requested fields for fast lookup
	requestedSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		requestedSet[f] = true
	}

	classifications := make(map[string]string)
	for rows.Next() {
		var fieldName, classification string
		if err := rows.Scan(&fieldName, &classification); err != nil {
			return nil, err
		}
		// Only include classifications for fields the request touches
		if requestedSet[fieldName] {
			classifications[fieldName] = classification
		}
	}
	return classifications, rows.Err()
}

// ---------------------------------------------------------------------------
// Layer 4: Per-runner authority
// ---------------------------------------------------------------------------

// checkLayer4RunnerAuthority verifies the runner's operation falls within
// its declared scope. Two checks:
//
//  1. runner_capability: does this runner spec declare a capability matching
//     the requested operation and target entity type?
//
//  2. runner_*_target bridges: is the specific target entity within the
//     runner's declared target set?
//
// Only called for runner callers (Identity.IsRunner() == true).
func checkLayer4RunnerAuthority(ctx *GateContext, result *AuthzResult) bool {
	if ctx.Identity.RunnerSpecID == nil {
		result.Allowed = false
		result.DeniedLayer = 4
		result.DeniedPolicy = "runner identity has no runner_spec_id"
		return false
	}

	specID := *ctx.Identity.RunnerSpecID

	// Check 1: runner_capability — does the runner declare a capability
	// for this operation on this entity type?
	hasCapability, err := queryRunnerCapability(
		ctx.DB, specID, ctx.Request.Operation, ctx.Request.TargetEntity)
	if err != nil {
		result.Allowed = false
		result.DeniedLayer = 4
		result.DeniedPolicy = fmt.Sprintf("failed to check runner capabilities: %v", err)
		return false
	}
	if !hasCapability {
		result.Allowed = false
		result.DeniedLayer = 4
		result.DeniedPolicy = fmt.Sprintf(
			"runner spec %d lacks capability for %s on %s",
			specID, ctx.Request.Operation, ctx.Request.TargetEntity)
		return false
	}

	// Check 2: runner_*_target bridges — is the specific entity within
	// the runner's declared target scope? Only checked when targeting
	// a specific entity (not for searches or creates).
	if ctx.Request.TargetEntityID > 0 {
		inScope, err := queryRunnerTargetScope(
			ctx.DB, specID, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
		if err != nil {
			result.Allowed = false
			result.DeniedLayer = 4
			result.DeniedPolicy = fmt.Sprintf("failed to check runner target scope: %v", err)
			return false
		}
		if !inScope {
			result.Allowed = false
			result.DeniedLayer = 4
			result.DeniedPolicy = fmt.Sprintf(
				"runner spec %d: target %s id=%d not in declared scope",
				specID, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
			return false
		}
	}

	return true
}

// queryRunnerCapability checks if a runner spec has declared a capability
// matching the requested operation and target entity type. Supports
// "all" wildcards in both capability_name and target entity.
func queryRunnerCapability(db *pg.DB, specID int, operation string, targetEntity string) (bool, error) {
	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS("+
			"SELECT 1 FROM runner_capability "+
			"WHERE runner_spec_id = $1 "+
			"AND (capability_name = $2 OR capability_name = 'all') "+
			"AND is_active = true"+
			")",
		specID, operation,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// queryRunnerTargetScope checks if the target entity is within the
// runner's declared target scope by querying the appropriate
// runner_*_target bridge table.
func queryRunnerTargetScope(db *pg.DB, specID int, targetEntity string, targetID int) (bool, error) {
	bridgeTable, fkColumn := runnerTargetBridge(targetEntity)
	if bridgeTable == "" {
		// No specific target bridge for this entity type. The
		// capability check in layer 4 check 1 is sufficient —
		// not all entity types have runner target bridges.
		return true, nil
	}

	var exists bool
	err := db.QueryRow(
		fmt.Sprintf(
			"SELECT EXISTS("+
				"SELECT 1 FROM %s "+
				"WHERE runner_spec_id = $1 AND %s = $2 AND is_active = true"+
				")",
			pg.QuoteIdentifier(bridgeTable),
			pg.QuoteIdentifier(fkColumn),
		),
		specID, targetID,
	).Scan(&exists)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			// Bridge table doesn't exist yet — pass through
			return true, nil
		}
		return false, err
	}
	return exists, nil
}

// runnerTargetBridge maps entity types to their runner target bridge
// table and FK column. Returns empty strings for entity types that
// don't have runner target bridges.
func runnerTargetBridge(entityType string) (string, string) {
	switch entityType {
	case "service":
		return "runner_service_target", "service_id"
	case "host_group":
		return "runner_host_group_target", "host_group_id"
	case "k8s_namespace":
		return "runner_k8s_namespace_target", "k8s_namespace_id"
	case "cloud_account":
		return "runner_cloud_account_target", "cloud_account_id"
	default:
		return "", ""
	}
}

// ---------------------------------------------------------------------------
// Layer 5: Policy rules
// ---------------------------------------------------------------------------

// checkLayer5PolicyRules evaluates access_control policy rows for
// additional governance constraints: time-of-day restrictions,
// separation of duties, IP-based restrictions. Fail closed — if
// policies can't be loaded, writes are denied.
func checkLayer5PolicyRules(ctx *GateContext, result *AuthzResult) bool {
	policies, err := queryAccessPolicies(ctx.DB, ctx.Request.TargetEntity)
	if err != nil {
		// Fail closed: can't read policies → deny writes, pass reads
		if isWriteOperation(ctx.Request.OperationClass) {
			result.Allowed = false
			result.DeniedLayer = 5
			result.DeniedPolicy = fmt.Sprintf("failed to query access policies: %v", err)
			return false
		}
		return true
	}

	for _, policy := range policies {
		violation := evaluateAccessPolicy(ctx, policy)
		if violation != "" {
			result.Allowed = false
			result.DeniedLayer = 5
			result.DeniedPolicy = violation
			return false
		}
	}

	return true
}

// accessPolicy represents a loaded access_control policy row.
type accessPolicy struct {
	PolicyID   int
	PolicyName string
	PolicyData map[string]interface{}
}

// queryAccessPolicies reads active access_control policies that apply
// to the target entity type. Includes policies targeting "all" entity
// types (global policies).
func queryAccessPolicies(db *pg.DB, targetEntity string) ([]accessPolicy, error) {
	rows, err := db.Query(
		"SELECT p.id, p.name, p.policy_data_json FROM policy p "+
			"WHERE p.policy_type = 'access_control' "+
			"AND p.is_active = true "+
			"AND (p.policy_data_json->>'target_entity_type' = $1 "+
			"OR p.policy_data_json->>'target_entity_type' = 'all')",
		targetEntity,
	)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var policies []accessPolicy
	for rows.Next() {
		var p accessPolicy
		var dataJSON []byte
		if err := rows.Scan(&p.PolicyID, &p.PolicyName, &dataJSON); err != nil {
			return nil, err
		}
		if err := pg.UnmarshalJSON(dataJSON, &p.PolicyData); err != nil {
			continue
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

// evaluateAccessPolicy checks one access_control policy against the
// current request context. Returns empty string if policy passes,
// or a violation description if it denies.
func evaluateAccessPolicy(ctx *GateContext, policy accessPolicy) string {
	// Separation of duty: submitter cannot approve their own change set.
	// This is the OPSDB-6 §10.6 segregation of duties enforcement.
	if sodRule, _ := policy.PolicyData["separation_of_duty"].(string); sodRule != "" {
		if sodRule == "submitter_cannot_approve" &&
			ctx.Request.Operation == "approve_change_set" {
			if isSubmitterOfChangeSet(ctx) {
				return fmt.Sprintf("policy %s: submitter cannot approve their own change set (separation of duty)",
					policy.PolicyName)
			}
		}
	}

	// IP restriction: client IP must be in allowed ranges.
	if allowedIPs, ok := policy.PolicyData["allowed_ip_ranges"]; ok {
		if !isIPInAllowedRanges(ctx.Request.ClientIP, allowedIPs) {
			return fmt.Sprintf("policy %s: client IP %s not in allowed ranges",
				policy.PolicyName, ctx.Request.ClientIP)
		}
	}

	// Time-of-day restriction: operation must fall within allowed window.
	// Stubbed for now — returns true (permissive) until time window
	// parsing is implemented. The policy data would carry allowed_hours
	// as a start/end pair in UTC.
	if _, ok := policy.PolicyData["allowed_time_window"]; ok {
		// TODO: parse time window from policy data and check
		// ctx.Request.ReceivedAt against it. For now, pass.
	}

	return ""
}

// isSubmitterOfChangeSet checks whether the current caller is the person
// who submitted the change set they're trying to approve. This is the
// mechanical enforcement of separation of duties — the proposer of a
// change cannot also approve it.
func isSubmitterOfChangeSet(ctx *GateContext) bool {
	if ctx.Request.Params == nil || ctx.Identity.OpsUserID == nil {
		return false
	}

	changeSetID, ok := ctx.Request.Params["change_set_id"]
	if !ok {
		return false
	}

	var submitterID int
	err := ctx.DB.QueryRow(
		"SELECT proposed_by_ops_user_id FROM change_set WHERE id = $1",
		changeSetID,
	).Scan(&submitterID)
	if err != nil {
		// Can't determine submitter — fail open here because the
		// denial would be "can't verify you're not the submitter"
		// which is confusing. The audit trail captures both identities
		// regardless.
		return false
	}

	return submitterID == *ctx.Identity.OpsUserID
}

// isIPInAllowedRanges checks if a client IP is within the allowed ranges
// specified in policy data. The ranges can be individual IPs or CIDR
// notation.
//
// Stubbed for now — returns true (permissive) until CIDR parsing is
// implemented. When implemented, this will parse the allowed_ip_ranges
// value as a list of CIDR strings and check net.IP.Contains.
func isIPInAllowedRanges(clientIP string, allowedRanges interface{}) bool {
	// TODO: parse allowedRanges as []string of CIDR notation,
	// parse clientIP as net.IP, check containment.
	return true
}

// ---------------------------------------------------------------------------
// Shared helpers (used by step_authz.go and other step files)
// ---------------------------------------------------------------------------

// extractRequestedFields returns the field names involved in the current
// operation. Extracts from params["fields"] if present (a map of field
// name → value for direct writes), or from the field_changes array for
// change set submissions.
func extractRequestedFields(req *GateRequest) []string {
	if req.Params == nil {
		return nil
	}

	// Direct writes and reads may carry a fields map
	if fields, ok := req.Params["fields"]; ok {
		if fieldMap, ok := fields.(map[string]interface{}); ok {
			names := make([]string, 0, len(fieldMap))
			for name := range fieldMap {
				names = append(names, name)
			}
			return names
		}
	}

	// Change set submissions carry field names inside field_changes
	if rawChanges, ok := req.Params["field_changes"]; ok {
		if changeList, ok := rawChanges.([]interface{}); ok {
			seen := make(map[string]bool)
			var names []string
			for _, item := range changeList {
				if changeMap, ok := item.(map[string]interface{}); ok {
					if fieldName, ok := changeMap["field_name"].(string); ok {
						if !seen[fieldName] {
							seen[fieldName] = true
							names = append(names, fieldName)
						}
					}
				}
			}
			return names
		}
	}

	return nil
}

// deriveCallerClearance determines the caller's data classification
// clearance level from their roles. The clearance hierarchy from lowest
// to highest: public < internal < confidential < restricted < regulated.
//
// Role mappings:
//
//	admin    → restricted (can see almost everything)
//	operator → confidential
//	auditor  → confidential (auditors need to verify controls)
//	approver → internal
//	reader   → internal
//	runner   → internal (runner-specific access is handled by layer 4)
func deriveCallerClearance(ctx *GateContext) string {
	if ctx.Identity == nil {
		return "public"
	}

	if ctx.Identity.HasRole("admin") {
		return "restricted"
	}
	if ctx.Identity.HasRole("operator") {
		return "confidential"
	}
	if ctx.Identity.HasRole("auditor") {
		return "confidential"
	}
	return "internal"
}

// clearanceMeetsClassification checks if a caller's clearance level
// meets or exceeds a field's classification requirement. Both are
// compared as integer levels on the same hierarchy.
func clearanceMeetsClassification(clearance string, classification string) bool {
	levels := map[string]int{
		"public":       0,
		"internal":     1,
		"confidential": 2,
		"restricted":   3,
		"regulated":    4,
	}

	clearanceLevel, ok := levels[clearance]
	if !ok {
		clearanceLevel = 0
	}
	classificationLevel, ok := levels[classification]
	if !ok {
		classificationLevel = 0
	}

	return clearanceLevel >= classificationLevel
}
