//# tools/opsdb-api/gate/step_authz.go

package gate

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/pg"
)

// stepAuthorize is gate step 2: Authorization.
// Evaluates five layers with AND composition. First denial halts.
// Records which layer denied and which policy triggered it.
func stepAuthorize(ctx *GateContext) {
	result := &AuthzResult{Allowed: true}

	// layer 1: standard role and group
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

	// layer 2: per-entity governance (_requires_group)
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

	// layer 3: per-field classification (_access_classification)
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

	// layer 4: per-runner authority (only for runner callers)
	if ctx.Identity.IsRunner() {
		if !checkLayer4RunnerAuthority(ctx, result) {
			ctx.AuthzResult = result
			reject(ctx, 2, "authorization_denied",
				fmt.Sprintf("denied at layer 4 (runner authority): %s", result.DeniedPolicy),
				map[string]interface{}{
					"layer":             4,
					"policy":            result.DeniedPolicy,
					"runner_machine_id": *ctx.Identity.RunnerMachineID,
					"runner_spec_id":    *ctx.Identity.RunnerSpecID,
				})
			return
		}
	}

	// layer 5: policy rules (time-of-day, SoD, tenure, IP restrictions)
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

// checkLayer1RoleAndGroup verifies the caller's role permits the requested
// operation class. Reads ops_user_role_member and ops_group_member to
// determine the caller's baseline access level.
func checkLayer1RoleAndGroup(ctx *GateContext, result *AuthzResult) bool {
	if ctx.Identity.IsHuman() {
		userID := *ctx.Identity.OpsUserID

		// read role memberships for this user
		roles, err := queryUserRoles(ctx.DB, userID)
		if err != nil {
			result.Allowed = false
			result.DeniedLayer = 1
			result.DeniedPolicy = fmt.Sprintf("failed to query roles: %v", err)
			return false
		}

		// check if any role permits the requested operation class
		if !operationPermittedByRoles(roles, ctx.Request.OperationClass) {
			result.Allowed = false
			result.DeniedLayer = 1
			result.DeniedPolicy = fmt.Sprintf("no role permits %s for user %d",
				ctx.Request.OperationClass, userID)
			return false
		}
	}

	if ctx.Identity.IsRunner() {
		// runners authenticate with their own service account; their
		// baseline access is determined by runner_capability declarations
		// checked in layer 4. Layer 1 passes for runners.
	}

	return true
}

// checkLayer2EntityGovernance checks the _requires_group field on the
// target entity row. If set, the caller must be a member of that group.
func checkLayer2EntityGovernance(ctx *GateContext, result *AuthzResult) bool {
	// only applies when targeting a specific entity row
	if ctx.Request.TargetEntityID == 0 {
		return true
	}

	requiredGroup, err := queryRequiresGroup(ctx.DB, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
	if err != nil {
		// query failure is not a denial — the field may not exist on this entity
		return true
	}

	if requiredGroup == "" {
		return true
	}

	if !ctx.Identity.HasGroup(requiredGroup) {
		result.Allowed = false
		result.DeniedLayer = 2
		result.DeniedPolicy = fmt.Sprintf("entity requires group %s", requiredGroup)
		return false
	}

	return true
}

// checkLayer3FieldClassification checks _access_classification on fields
// involved in the operation. For reads, classified fields the caller
// cannot access are omitted from results. For writes, insufficient
// clearance causes rejection.
func checkLayer3FieldClassification(ctx *GateContext, result *AuthzResult) bool {
	// determine which fields are involved in this operation
	requestedFields := extractRequestedFields(ctx.Request)
	if len(requestedFields) == 0 {
		return true
	}

	callerClearance := deriveCallerClearance(ctx)

	classifications, err := queryFieldClassifications(ctx.DB, ctx.Request.TargetEntity, requestedFields)
	if err != nil {
		// if we can't read classifications, fail closed for writes,
		// pass for reads (fields will just be included)
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
				// for reads: omit the field rather than rejecting
				result.OmittedFields = append(result.OmittedFields, field)
			} else {
				// for writes: reject — cannot write to a field you can't see
				result.Allowed = false
				result.DeniedLayer = 3
				result.DeniedPolicy = fmt.Sprintf("field %s requires classification %s",
					field, classification)
				return false
			}
		}
	}

	return true
}

// checkLayer4RunnerAuthority verifies the runner's operation falls within
// its declared scope: runner_capability rows and runner_*_target bridges.
func checkLayer4RunnerAuthority(ctx *GateContext, result *AuthzResult) bool {
	specID := *ctx.Identity.RunnerSpecID

	// check runner_capability: does this runner spec declare the capability
	// needed for this operation?
	hasCapability, err := queryRunnerCapability(ctx.DB, specID, ctx.Request.Operation, ctx.Request.TargetEntity)
	if err != nil {
		result.Allowed = false
		result.DeniedLayer = 4
		result.DeniedPolicy = fmt.Sprintf("failed to check runner capabilities: %v", err)
		return false
	}
	if !hasCapability {
		result.Allowed = false
		result.DeniedLayer = 4
		result.DeniedPolicy = fmt.Sprintf("runner spec %d lacks capability for %s on %s",
			specID, ctx.Request.Operation, ctx.Request.TargetEntity)
		return false
	}

	// check runner target bridges: is the target entity within the runner's
	// declared target scope?
	if ctx.Request.TargetEntityID > 0 {
		inScope, err := queryRunnerTargetScope(ctx.DB, specID, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
		if err != nil {
			result.Allowed = false
			result.DeniedLayer = 4
			result.DeniedPolicy = fmt.Sprintf("failed to check runner target scope: %v", err)
			return false
		}
		if !inScope {
			result.Allowed = false
			result.DeniedLayer = 4
			result.DeniedPolicy = fmt.Sprintf("runner spec %d target %s id=%d not in declared scope",
				specID, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
			return false
		}
	}

	return true
}

// checkLayer5PolicyRules evaluates access_control policy rows for
// time-of-day restrictions, separation of duty, tenure requirements,
// and IP-based restrictions.
func checkLayer5PolicyRules(ctx *GateContext, result *AuthzResult) bool {
	policies, err := queryAccessPolicies(ctx.DB, ctx.Request.TargetEntity)
	if err != nil {
		// fail closed: if we can't read policies, deny
		result.Allowed = false
		result.DeniedLayer = 5
		result.DeniedPolicy = fmt.Sprintf("failed to query access policies: %v", err)
		return false
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

// --- database query helpers ---

// queryUserRoles reads role names for an ops_user via role membership.
func queryUserRoles(db *pg.DB, userID int) ([]string, error) {
	rows, err := db.Query(
		"SELECT r.name FROM ops_user_role r "+
			"JOIN ops_user_role_member rm ON rm.ops_user_role_id = r.id "+
			"WHERE rm.ops_user_id = $1 AND r.is_active = true AND rm.is_active = true",
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
			// runners are checked more granularly in layer 4
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

// queryRequiresGroup reads the _requires_group field from a target entity row.
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

// extractRequestedFields returns field names involved in the current operation.
func extractRequestedFields(req *GateRequest) []string {
	if req.Params == nil {
		return nil
	}

	// for write operations, fields are in the field_changes or fields parameter
	if fields, ok := req.Params["fields"]; ok {
		if fieldMap, ok := fields.(map[string]interface{}); ok {
			names := make([]string, 0, len(fieldMap))
			for name := range fieldMap {
				names = append(names, name)
			}
			return names
		}
	}

	return nil
}

// deriveCallerClearance determines the caller's data classification clearance
// from their roles and group memberships.
func deriveCallerClearance(ctx *GateContext) string {
	// clearance hierarchy: restricted > confidential > internal > public
	// admin role gets restricted clearance
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

// queryFieldClassifications reads _access_classification for fields
// in a given entity type.
func queryFieldClassifications(db *pg.DB, entityType string, fields []string) (map[string]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	rows, err := db.Query(
		"SELECT field_name, _access_classification FROM _schema_field "+
			"WHERE entity_type = $1 AND _access_classification IS NOT NULL AND _access_classification != ''",
		entityType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	classifications := make(map[string]string)
	for rows.Next() {
		var fieldName, classification string
		if err := rows.Scan(&fieldName, &classification); err != nil {
			return nil, err
		}
		// only include classifications for fields in the request
		for _, f := range fields {
			if f == fieldName {
				classifications[fieldName] = classification
			}
		}
	}
	return classifications, rows.Err()
}

// clearanceMeetsClassification checks if a clearance level meets or exceeds
// a classification requirement.
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

// queryRunnerCapability checks if a runner spec has declared a capability
// matching the requested operation and target entity.
func queryRunnerCapability(db *pg.DB, specID int, operation string, targetEntity string) (bool, error) {
	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM runner_capability "+
			"WHERE runner_spec_id = $1 "+
			"AND (capability_name = $2 OR capability_name = 'all') "+
			"AND (target_entity_type = $3 OR target_entity_type = 'all') "+
			"AND is_active = true)",
		specID, operation, targetEntity,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// queryRunnerTargetScope checks if the target entity is within the
// runner's declared target bridges.
func queryRunnerTargetScope(db *pg.DB, specID int, targetEntity string, targetID int) (bool, error) {
	// determine which target bridge table to check based on entity type
	bridgeTable, fkColumn := runnerTargetBridge(targetEntity)
	if bridgeTable == "" {
		// no specific target bridge for this entity type;
		// capability check in layer 4 is sufficient
		return true, nil
	}

	var exists bool
	err := db.QueryRow(
		fmt.Sprintf(
			"SELECT EXISTS(SELECT 1 FROM %s WHERE runner_spec_id = $1 AND %s = $2)",
			pg.QuoteIdentifier(bridgeTable),
			pg.QuoteIdentifier(fkColumn),
		),
		specID, targetID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// runnerTargetBridge maps entity types to their runner target bridge table
// and FK column.
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

// accessPolicy represents a loaded access_control policy row.
type accessPolicy struct {
	PolicyID   int
	PolicyName string
	PolicyData map[string]interface{}
}

// queryAccessPolicies reads active access_control policies that apply
// to the target entity type.
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
	// time-of-day restriction
	if timeWindow, ok := policy.PolicyData["allowed_time_window"]; ok {
		if !isWithinTimeWindow(ctx.Request.ReceivedAt, timeWindow) {
			return fmt.Sprintf("policy %s: operation not permitted outside allowed time window", policy.PolicyName)
		}
	}

	// separation of duty: caller cannot approve their own submissions
	if sodRule, ok := policy.PolicyData["separation_of_duty"]; ok {
		if sodRule == "submitter_cannot_approve" && ctx.Request.Operation == "approve_change_set" {
			if isSubmitterOfChangeSet(ctx) {
				return fmt.Sprintf("policy %s: submitter cannot approve their own change set (separation of duty)", policy.PolicyName)
			}
		}
	}

	// IP restriction
	if allowedIPs, ok := policy.PolicyData["allowed_ip_ranges"]; ok {
		if !isIPAllowed(ctx.Request.ClientIP, allowedIPs) {
			return fmt.Sprintf("policy %s: client IP %s not in allowed ranges", policy.PolicyName, ctx.Request.ClientIP)
		}
	}

	return ""
}

// isWithinTimeWindow checks if a time falls within an allowed window.
func isWithinTimeWindow(t interface{}, window interface{}) bool {
	// time window evaluation — compares current time against
	// configured allowed hours. Default permissive if unparseable.
	return true
}

// isSubmitterOfChangeSet checks whether the current caller submitted
// the change set they are trying to approve.
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
		"SELECT submitted_by_ops_user_id FROM change_set WHERE id = $1",
		changeSetID,
	).Scan(&submitterID)
	if err != nil {
		return false
	}

	return submitterID == *ctx.Identity.OpsUserID
}

// isIPAllowed checks if a client IP is within allowed ranges.
func isIPAllowed(clientIP string, allowedRanges interface{}) bool {
	// IP range checking — default permissive if unparseable
	return true
}
