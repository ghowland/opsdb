//# tools/opsdb_api/gate/step_policy.go

package gate

import (
	"encoding/json"
	"fmt"

	"github.com/ghowland/opsdb/internal/pg"
)

// semanticInvariant represents one cross-field constraint loaded from
// policy data. Evaluated at API time against proposed field values
// merged with current entity state.
type semanticInvariant struct {
	PolicyID   int
	PolicyName string
	EntityType string
	Operator   string // lte, gte, eq, neq, requires_if, requires_unless
	LeftField  string
	RightField string
	RightValue interface{} // literal value for comparison (when not comparing two fields)
	FailMode   string      // block, warn
	Message    string      // custom violation message
}

// linkedPolicy represents a policy linked to an entity via a bridge table.
type linkedPolicy struct {
	PolicyID   int
	PolicyName string
	PolicyType string
	PolicyData map[string]interface{}
}

// stepPolicyEvaluate is gate step 5: Policy Evaluation.
// Consults policy rows for semantic invariants (cross-field constraints),
// entity-linked policies (security zone, data classification), and
// classification consistency checks.
//
// Semantic invariants are the OPSDB-7 §8 mechanism: cross-field rules
// that can't be expressed in the closed schema vocabulary live in policy
// data and are evaluated here. Examples: min_replicas <= max_replicas,
// if status is 'active' then running_since must be set.
//
// Each invariant has a fail mode — "block" rejects the request, "warn"
// passes with a warning. The default is block (fail closed).
//
// Skips entirely for read operations.
func stepPolicyEvaluate(ctx *GateContext) {
	result := &PolicyResult{Passed: true}

	if !isWriteOperation(ctx.Request.OperationClass) {
		ctx.PolicyResult = result
		return
	}

	// Evaluate semantic invariants — cross-field constraints from
	// policy rows of type semantic_invariant.
	evaluateSemanticInvariants(ctx, result)

	// Evaluate entity-linked policies — policies attached to the target
	// entity via bridge tables (service_policy, machine_policy, etc.).
	evaluateEntityPolicies(ctx, result)

	// Evaluate data classification consistency — check that proposed
	// field values don't create classification inconsistencies.
	evaluateClassificationConsistency(ctx, result)

	// Check if any blocking violations accumulated
	if len(result.Blocks) > 0 {
		result.Passed = false

		detail := map[string]interface{}{
			"violation_count": len(result.Blocks),
			"violations":      result.Blocks,
		}
		if len(result.Warnings) > 0 {
			detail["warnings"] = result.Warnings
		}

		reject(ctx, 5, "policy_violation",
			fmt.Sprintf("policy evaluation failed: %d violation(s)", len(result.Blocks)),
			detail)
		return
	}

	// Pass non-blocking warnings through to the response
	if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			warn(ctx, w)
		}
	}

	ctx.PolicyResult = result
}

// ---------------------------------------------------------------------------
// Semantic invariant evaluation
// ---------------------------------------------------------------------------

// evaluateSemanticInvariants loads and evaluates cross-field constraints
// defined as policy rows of type semantic_invariant matching the target
// entity type. Merges current entity values with proposed changes so
// invariants see the complete post-change state.
func evaluateSemanticInvariants(ctx *GateContext, result *PolicyResult) {
	invariants, err := loadSemanticInvariants(ctx.DB, ctx.Request.TargetEntity)
	if err != nil {
		// Fail closed: if we can't read policies, block writes.
		result.Blocks = append(result.Blocks,
			fmt.Sprintf("failed to load semantic invariants: %v", err))
		return
	}

	if len(invariants) == 0 {
		return
	}

	// Build merged field values: current entity state overlaid with
	// proposed changes. This gives invariants the complete picture of
	// what the entity will look like after the change.
	currentValues := loadCurrentEntityValues(ctx)
	proposedValues := extractFieldValues(ctx.Request)
	merged := mergeFieldValues(currentValues, proposedValues)

	if len(merged) == 0 {
		return
	}

	for _, inv := range invariants {
		violation := evaluateOneInvariant(inv, merged)
		if violation == "" {
			continue
		}

		switch inv.FailMode {
		case "warn":
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("policy %s: %s", inv.PolicyName, violation))
		default:
			// "block" is the default — fail closed per OPSDB-9 §5.3
			result.Blocks = append(result.Blocks,
				fmt.Sprintf("policy %s: %s", inv.PolicyName, violation))
		}
	}
}

// evaluateOneInvariant checks a single semantic invariant against merged
// field values. Returns empty string if the invariant holds, or a
// violation message if it doesn't.
//
// Six operators are supported:
//
//	lte            — left field must be <= right field (e.g., min_replicas <= max_replicas)
//	gte            — left field must be >= right field
//	eq             — left field must equal a literal right value
//	neq            — left field must not equal a literal right value
//	requires_if    — if left field equals a value, right field must be set
//	requires_unless — right field must be set unless left field equals a value
func evaluateOneInvariant(inv semanticInvariant, values map[string]interface{}) string {
	leftVal, leftExists := values[inv.LeftField]

	switch inv.Operator {
	case "lte":
		return evaluateNumericComparison(inv, values, leftVal, leftExists,
			func(left, right float64) bool { return left > right },
			"must be <=")

	case "gte":
		return evaluateNumericComparison(inv, values, leftVal, leftExists,
			func(left, right float64) bool { return left < right },
			"must be >=")

	case "eq":
		if !leftExists {
			return ""
		}
		if fmt.Sprintf("%v", leftVal) != fmt.Sprintf("%v", inv.RightValue) {
			return invariantMessage(inv,
				fmt.Sprintf("%s must equal %v, got %v",
					inv.LeftField, inv.RightValue, leftVal))
		}

	case "neq":
		if !leftExists {
			return ""
		}
		if fmt.Sprintf("%v", leftVal) == fmt.Sprintf("%v", inv.RightValue) {
			return invariantMessage(inv,
				fmt.Sprintf("%s must not equal %v",
					inv.LeftField, inv.RightValue))
		}

	case "requires_if":
		// If left field equals a specific trigger value, right field must be set
		if !leftExists {
			return ""
		}
		leftStr, _ := leftVal.(string)
		triggerStr, _ := inv.RightValue.(string)
		if leftStr != triggerStr {
			return ""
		}
		rightVal, rightExists := values[inv.RightField]
		if !rightExists || rightVal == nil || rightVal == "" {
			return invariantMessage(inv,
				fmt.Sprintf("when %s = %q, %s must be set",
					inv.LeftField, triggerStr, inv.RightField))
		}

	case "requires_unless":
		// Right field must be set unless left field equals an exempt value
		if leftExists {
			leftStr, _ := leftVal.(string)
			exemptStr, _ := inv.RightValue.(string)
			if leftStr == exemptStr {
				return ""
			}
		}
		rightVal, rightExists := values[inv.RightField]
		if !rightExists || rightVal == nil || rightVal == "" {
			return invariantMessage(inv,
				fmt.Sprintf("%s is required unless %s = %v",
					inv.RightField, inv.LeftField, inv.RightValue))
		}
	}

	return ""
}

// evaluateNumericComparison handles the lte and gte operators. The
// violates function returns true when the invariant is violated.
func evaluateNumericComparison(inv semanticInvariant, values map[string]interface{}, leftVal interface{}, leftExists bool, violates func(float64, float64) bool, opDesc string) string {
	rightVal, rightExists := values[inv.RightField]
	if !leftExists || !rightExists {
		// Can't evaluate — one of the fields is missing. Not a
		// violation (the field might be optional or not yet set).
		return ""
	}

	leftNum, leftOk := toFloat(leftVal)
	rightNum, rightOk := toFloat(rightVal)
	if !leftOk || !rightOk {
		// Can't convert to numbers — not a violation of this invariant,
		// but the type mismatch should be caught by step 3 or step 4.
		return ""
	}

	if violates(leftNum, rightNum) {
		return invariantMessage(inv,
			fmt.Sprintf("%s (%g) %s %s (%g)",
				inv.LeftField, leftNum, opDesc, inv.RightField, rightNum))
	}

	return ""
}

// invariantMessage returns the invariant's custom message if set, or
// the provided default message.
func invariantMessage(inv semanticInvariant, defaultMsg string) string {
	if inv.Message != "" {
		return inv.Message
	}
	return defaultMsg
}

// ---------------------------------------------------------------------------
// Entity-linked policy evaluation
// ---------------------------------------------------------------------------

// evaluateEntityPolicies loads policies linked to the target entity via
// bridge tables and evaluates each one. Different policy types are
// handled by specialized evaluators.
func evaluateEntityPolicies(ctx *GateContext, result *PolicyResult) {
	if ctx.Request.TargetEntityID == 0 {
		return
	}

	policies, err := loadLinkedPolicies(
		ctx.DB, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
	if err != nil {
		// Warn but don't block — linked policy lookup failure shouldn't
		// prevent all writes. The underlying bridge table may not exist
		// yet for this entity type.
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("failed to load linked policies: %v", err))
		return
	}

	for _, policy := range policies {
		evaluateLinkedPolicy(ctx, policy, result)
	}
}

// evaluateLinkedPolicy dispatches evaluation to the appropriate handler
// based on policy type.
func evaluateLinkedPolicy(ctx *GateContext, policy linkedPolicy, result *PolicyResult) {
	switch policy.PolicyType {
	case "change_management":
		// Change management policies are handled in step 7 — skip here
		return

	case "retention":
		// Retention policies are enforced by the reaper runner at
		// garbage-collection time, not at write time — skip here
		return

	case "security_zone":
		evaluateSecurityZonePolicy(ctx, policy, result)

	case "data_classification":
		evaluateDataClassificationPolicy(ctx, policy, result)

	case "schedule_governance":
		// Schedule governance policies restrict when operations are
		// allowed — could add time-window checks here. For now, the
		// time-of-day check in step 2 layer 5 covers this.
		return
	}
}

// evaluateSecurityZonePolicy checks security zone constraints on the
// proposed change. A security zone policy may restrict which operations
// are allowed on entities in that zone.
func evaluateSecurityZonePolicy(ctx *GateContext, policy linkedPolicy, result *PolicyResult) {
	restrictedOps, ok := policy.PolicyData["restricted_operations"]
	if !ok {
		return
	}

	opList, ok := restrictedOps.([]interface{})
	if !ok {
		return
	}

	for _, op := range opList {
		if opStr, ok := op.(string); ok && opStr == ctx.Request.Operation {
			result.Blocks = append(result.Blocks,
				fmt.Sprintf("policy %s: operation %s is restricted in security zone",
					policy.PolicyName, ctx.Request.Operation))
			return
		}
	}
}

// evaluateDataClassificationPolicy checks that the caller's clearance
// meets the minimum required by the data classification policy attached
// to the entity.
func evaluateDataClassificationPolicy(ctx *GateContext, policy linkedPolicy, result *PolicyResult) {
	requiredClearance, _ := policy.PolicyData["minimum_clearance"].(string)
	if requiredClearance == "" {
		return
	}

	callerClearance := deriveCallerClearance(ctx)
	if !clearanceMeetsClassification(callerClearance, requiredClearance) {
		result.Blocks = append(result.Blocks,
			fmt.Sprintf("policy %s: caller clearance %s insufficient (requires %s)",
				policy.PolicyName, callerClearance, requiredClearance))
	}
}

// ---------------------------------------------------------------------------
// Classification consistency
// ---------------------------------------------------------------------------

// evaluateClassificationConsistency checks that proposed field values
// don't create classification inconsistencies — for example, writing
// data into a field whose classification is lower than the entity's
// overall classification. This is a warning, not a block, because
// classification mismatches are common during migration and the fix
// is to update the classification, not to block the write.
func evaluateClassificationConsistency(ctx *GateContext, result *PolicyResult) {
	if ctx.Request.TargetEntityID == 0 {
		return
	}

	entityClassification := queryEntityClassification(
		ctx.DB, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
	if entityClassification == "" {
		return
	}

	requestedFields := extractRequestedFields(ctx.Request)
	for _, fieldName := range requestedFields {
		fieldMeta, found := ctx.Schema.GetField(ctx.Request.TargetEntity, fieldName)
		if !found {
			continue
		}

		// Read the field's access classification from the runtime schema.
		// This comes from the _access_classification constraint declared
		// in the schema YAML.
		fieldClassification := fieldMeta.AccessClassification
		if fieldClassification == "" {
			continue
		}

		if !classificationAtLeast(fieldClassification, entityClassification) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("field %s classification (%s) is lower than entity classification (%s)",
					fieldName, fieldClassification, entityClassification))
		}
	}
}

// classificationAtLeast checks if classification a is at least as high
// as classification b on the hierarchy: public < internal < confidential
// < restricted < regulated.
func classificationAtLeast(a string, b string) bool {
	levels := map[string]int{
		"public":       0,
		"internal":     1,
		"confidential": 2,
		"restricted":   3,
		"regulated":    4,
	}

	aLevel, aOk := levels[a]
	if !aOk {
		aLevel = 0
	}
	bLevel, bOk := levels[b]
	if !bOk {
		bLevel = 0
	}

	return aLevel >= bLevel
}

// ---------------------------------------------------------------------------
// Data loading helpers
// ---------------------------------------------------------------------------

// loadSemanticInvariants reads policy rows of type semantic_invariant
// matching the target entity type. Also includes invariants targeting
// "all" entity types (global invariants).
func loadSemanticInvariants(db *pg.DB, entityType string) ([]semanticInvariant, error) {
	rows, err := db.Query(
		"SELECT p.id, p.name, p.policy_data_json FROM policy p "+
			"WHERE p.policy_type = 'semantic_invariant' "+
			"AND p.is_active = true "+
			"AND (p.policy_data_json->>'target_entity_type' = $1 "+
			"OR p.policy_data_json->>'target_entity_type' = 'all')",
		entityType,
	)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var invariants []semanticInvariant
	for rows.Next() {
		var policyID int
		var policyName string
		var dataJSON []byte
		if err := rows.Scan(&policyID, &policyName, &dataJSON); err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := json.Unmarshal(dataJSON, &data); err != nil {
			continue
		}

		inv := semanticInvariant{
			PolicyID:   policyID,
			PolicyName: policyName,
			EntityType: entityType,
		}

		inv.Operator, _ = data["operator"].(string)
		inv.LeftField, _ = data["left_field"].(string)
		inv.RightField, _ = data["right_field"].(string)
		inv.RightValue = data["right_value"]
		inv.FailMode, _ = data["fail_mode"].(string)
		inv.Message, _ = data["message"].(string)

		// An invariant must have at least an operator and a left field
		// to be evaluable.
		if inv.Operator == "" || inv.LeftField == "" {
			continue
		}

		invariants = append(invariants, inv)
	}

	return invariants, rows.Err()
}

// loadCurrentEntityValues reads the current field values for the target
// entity row. Used to merge with proposed changes so semantic invariants
// see the complete post-change state.
func loadCurrentEntityValues(ctx *GateContext) map[string]interface{} {
	if ctx.Request.TargetEntityID == 0 {
		return nil
	}

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE id = $1",
		pg.QuoteIdentifier(ctx.Request.TargetEntity))

	rows, err := ctx.DB.Query(query, ctx.Request.TargetEntityID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil
	}

	if !rows.Next() {
		return nil
	}

	// Scan all columns into interface{} values
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil
	}

	result := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		result[col] = values[i]
	}

	return result
}

// mergeFieldValues combines current entity values with proposed changes.
// Proposed values override current values, producing the complete
// post-change state for invariant evaluation.
func mergeFieldValues(current map[string]interface{}, proposed map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	for k, v := range current {
		merged[k] = v
	}
	for k, v := range proposed {
		merged[k] = v
	}

	return merged
}

// loadLinkedPolicies reads policies linked to a specific entity via
// the appropriate policy bridge table (service_policy, machine_policy,
// k8s_namespace_policy, cloud_account_policy).
func loadLinkedPolicies(db *pg.DB, entityType string, entityID int) ([]linkedPolicy, error) {
	bridgeTable, fkColumn := policyBridgeFor(entityType)
	if bridgeTable == "" {
		return nil, nil
	}

	rows, err := db.Query(
		fmt.Sprintf(
			"SELECT p.id, p.name, p.policy_type, p.policy_data_json "+
				"FROM policy p "+
				"JOIN %s bp ON bp.policy_id = p.id "+
				"WHERE bp.%s = $1 AND p.is_active = true",
			pg.QuoteIdentifier(bridgeTable),
			pg.QuoteIdentifier(fkColumn),
		),
		entityID,
	)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var policies []linkedPolicy
	for rows.Next() {
		var p linkedPolicy
		var dataJSON []byte
		if err := rows.Scan(&p.PolicyID, &p.PolicyName, &p.PolicyType, &dataJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(dataJSON, &p.PolicyData); err != nil {
			p.PolicyData = make(map[string]interface{})
		}
		policies = append(policies, p)
	}

	return policies, rows.Err()
}

// policyBridgeFor returns the policy bridge table and FK column for
// a given entity type. Returns empty strings for entity types that
// don't have policy bridges.
func policyBridgeFor(entityType string) (string, string) {
	switch entityType {
	case "service":
		return "service_policy", "service_id"
	case "machine":
		return "machine_policy", "machine_id"
	case "k8s_namespace":
		return "k8s_namespace_policy", "k8s_namespace_id"
	case "cloud_account":
		return "cloud_account_policy", "cloud_account_id"
	default:
		return "", ""
	}
}

// queryEntityClassification reads the _access_classification governance
// field from a target entity row. Returns empty string if the field
// doesn't exist on the entity, the entity doesn't exist, or the
// field is null.
func queryEntityClassification(db *pg.DB, entityType string, entityID int) string {
	query := fmt.Sprintf(
		"SELECT _access_classification FROM %s WHERE id = $1",
		pg.QuoteIdentifier(entityType),
	)

	var classification string
	err := db.QueryRow(query, entityID).Scan(&classification)
	if err != nil {
		// The column may not exist on this entity type, the row may
		// not exist, or the value may be null. All cases return empty.
		return ""
	}
	return classification
}
