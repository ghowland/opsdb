//# tools/opsdb-api/gate/step_policy.go

package gate

import (
	"fmt"
	"strings"

	"github.com/ghowland/opsdb/internal/pg"
)

// semanticInvariant represents one cross-field constraint loaded from
// policy data. Evaluated at API time against proposed field values.
type semanticInvariant struct {
	PolicyID    int
	PolicyName  string
	EntityType  string
	Operator    string // lte, gte, eq, neq, requires_if, requires_unless
	LeftField   string
	RightField  string
	RightValue  interface{} // literal value for comparison (when not comparing two fields)
	FailMode    string      // block, warn
	Message     string
}

// stepPolicyEvaluate is gate step 5: Policy Evaluation.
// Consults policy rows for semantic invariants, data classification
// consistency, and additional governance rules.
func stepPolicyEvaluate(ctx *GateContext) {
	result := &PolicyResult{Passed: true}

	if !isWriteOperation(ctx.Request.OperationClass) {
		ctx.PolicyResult = result
		return
	}

	// evaluate semantic invariants (cross-field constraints from policy data)
	evaluateSemanticInvariants(ctx, result)

	// evaluate entity-linked policies
	evaluateEntityPolicies(ctx, result)

	// evaluate data classification consistency
	evaluateClassificationConsistency(ctx, result)

	// check if any blocking violations accumulated
	if len(result.Blocks) > 0 {
		result.Passed = false

		detail := map[string]interface{}{
			"violations": result.Blocks,
		}
		if len(result.Warnings) > 0 {
			detail["warnings"] = result.Warnings
		}

		reject(ctx, 5, "policy_violation",
			fmt.Sprintf("policy evaluation failed: %d violation(s)", len(result.Blocks)),
			detail)
		return
	}

	// pass warnings through to the response
	if len(result.Warnings) > 0 {
		for _, w := range result.Warnings {
			warn(ctx, w)
		}
	}

	ctx.PolicyResult = result
}

// evaluateSemanticInvariants loads and evaluates cross-field constraints
// defined as policy rows of type semantic_invariant.
func evaluateSemanticInvariants(ctx *GateContext, result *PolicyResult) {
	invariants, err := loadSemanticInvariants(ctx.DB, ctx.Request.TargetEntity)
	if err != nil {
		// fail closed: if we can't read policies, block writes
		result.Blocks = append(result.Blocks,
			fmt.Sprintf("failed to load semantic invariants: %v", err))
		return
	}

	if len(invariants) == 0 {
		return
	}

	// build merged field values: current entity state + proposed changes
	currentValues := loadCurrentEntityValues(ctx)
	proposedValues := extractFieldValues(ctx.Request)
	merged := mergeFieldValues(currentValues, proposedValues)

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
			// block is the default (fail closed)
			result.Blocks = append(result.Blocks,
				fmt.Sprintf("policy %s: %s", inv.PolicyName, violation))
		}
	}
}

// evaluateOneInvariant checks a single semantic invariant against merged
// field values. Returns empty string if the invariant holds, or a
// violation message if it doesn't.
func evaluateOneInvariant(inv semanticInvariant, values map[string]interface{}) string {
	leftVal, leftExists := values[inv.LeftField]

	switch inv.Operator {
	case "lte":
		// left field must be <= right field (e.g., min_replicas <= max_replicas)
		rightVal, rightExists := values[inv.RightField]
		if !leftExists || !rightExists {
			return ""
		}
		leftNum, leftOk := toFloat(leftVal)
		rightNum, rightOk := toFloat(rightVal)
		if !leftOk || !rightOk {
			return ""
		}
		if leftNum > rightNum {
			msg := inv.Message
			if msg == "" {
				msg = fmt.Sprintf("%s (%g) must be <= %s (%g)",
					inv.LeftField, leftNum, inv.RightField, rightNum)
			}
			return msg
		}

	case "gte":
		rightVal, rightExists := values[inv.RightField]
		if !leftExists || !rightExists {
			return ""
		}
		leftNum, leftOk := toFloat(leftVal)
		rightNum, rightOk := toFloat(rightVal)
		if !leftOk || !rightOk {
			return ""
		}
		if leftNum < rightNum {
			msg := inv.Message
			if msg == "" {
				msg = fmt.Sprintf("%s (%g) must be >= %s (%g)",
					inv.LeftField, leftNum, inv.RightField, rightNum)
			}
			return msg
		}

	case "requires_if":
		// if left field equals a specific value, right field must be set
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
			msg := inv.Message
			if msg == "" {
				msg = fmt.Sprintf("when %s = %q, %s must be set",
					inv.LeftField, triggerStr, inv.RightField)
			}
			return msg
		}

	case "requires_unless":
		// right field must be set unless left field equals a specific value
		if leftExists {
			leftStr, _ := leftVal.(string)
			exemptStr, _ := inv.RightValue.(string)
			if leftStr == exemptStr {
				return ""
			}
		}
		rightVal, rightExists := values[inv.RightField]
		if !rightExists || rightVal == nil || rightVal == "" {
			msg := inv.Message
			if msg == "" {
				msg = fmt.Sprintf("%s is required unless %s = %v",
					inv.RightField, inv.LeftField, inv.RightValue)
			}
			return msg
		}

	case "eq":
		// left field must equal right value
		if !leftExists {
			return ""
		}
		if fmt.Sprintf("%v", leftVal) != fmt.Sprintf("%v", inv.RightValue) {
			msg := inv.Message
			if msg == "" {
				msg = fmt.Sprintf("%s must equal %v", inv.LeftField, inv.RightValue)
			}
			return msg
		}

	case "neq":
		// left field must not equal right value
		if !leftExists {
			return ""
		}
		if fmt.Sprintf("%v", leftVal) == fmt.Sprintf("%v", inv.RightValue) {
			msg := inv.Message
			if msg == "" {
				msg = fmt.Sprintf("%s must not equal %v", inv.LeftField, inv.RightValue)
			}
			return msg
		}
	}

	return ""
}

// evaluateEntityPolicies loads policies linked to the target entity
// via bridge tables and evaluates them.
func evaluateEntityPolicies(ctx *GateContext, result *PolicyResult) {
	if ctx.Request.TargetEntityID == 0 {
		return
	}

	policies, err := loadLinkedPolicies(ctx.DB, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
	if err != nil {
		// warn but don't block — linked policy lookup failure shouldn't
		// prevent all writes
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("failed to load linked policies: %v", err))
		return
	}

	for _, policy := range policies {
		evaluateLinkedPolicy(ctx, policy, result)
	}
}

// evaluateClassificationConsistency checks that proposed field values
// don't create classification inconsistencies — for example, writing
// restricted data into a field classified as internal.
func evaluateClassificationConsistency(ctx *GateContext, result *PolicyResult) {
	if ctx.Request.TargetEntityID == 0 {
		return
	}

	// read entity-level classification
	entityClassification := queryEntityClassification(ctx.DB, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
	if entityClassification == "" {
		return
	}

	// check that no proposed field writes target fields with a lower
	// classification than the entity itself
	requestedFields := extractRequestedFields(ctx.Request)
	for _, fieldName := range requestedFields {
		fieldMeta, found := ctx.Schema.GetField(ctx.Request.TargetEntity, fieldName)
		if !found {
			continue
		}
		if fieldMeta.AccessClassification != "" {
			if !classificationAtLeast(fieldMeta.AccessClassification, entityClassification) {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("field %s classification (%s) is lower than entity classification (%s)",
						fieldName, fieldMeta.AccessClassification, entityClassification))
			}
		}
	}
}

// --- data loading helpers ---

// loadSemanticInvariants reads policy rows of type semantic_invariant
// matching the target entity type.
func loadSemanticInvariants(db *pg.DB, entityType string) ([]semanticInvariant, error) {
	rows, err := db.Query(
		"SELECT p.id, p.name, p.policy_data_json FROM policy p "+
			"WHERE p.policy_type = 'semantic_invariant' AND p.is_active = true "+
			"AND (p.policy_data_json->>'target_entity_type' = $1 "+
			"OR p.policy_data_json->>'target_entity_type' = 'all')",
		entityType,
	)
	if err != nil {
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
		if err := pg.UnmarshalJSON(dataJSON, &data); err != nil {
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

		if inv.Operator == "" || inv.LeftField == "" {
			continue
		}

		invariants = append(invariants, inv)
	}

	return invariants, rows.Err()
}

// loadCurrentEntityValues reads the current field values for the target entity.
func loadCurrentEntityValues(ctx *GateContext) map[string]interface{} {
	if ctx.Request.TargetEntityID == 0 {
		return nil
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1",
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
// Proposed values override current values.
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

// linkedPolicy represents a policy linked to an entity via a bridge table.
type linkedPolicy struct {
	PolicyID   int
	PolicyName string
	PolicyType string
	PolicyData map[string]interface{}
}

// loadLinkedPolicies reads policies linked to a specific entity via
// the appropriate policy bridge table.
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
				"WHERE bp.%s = $1 AND p.is_active = true AND bp.is_active = true",
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
		pg.UnmarshalJSON(dataJSON, &p.PolicyData)
		policies = append(policies, p)
	}

	return policies, rows.Err()
}

// policyBridgeFor returns the policy bridge table and FK column for
// a given entity type.
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

// evaluateLinkedPolicy evaluates a single entity-linked policy.
func evaluateLinkedPolicy(ctx *GateContext, policy linkedPolicy, result *PolicyResult) {
	switch policy.PolicyType {
	case "change_management":
		// change management policies are handled in step 7
		return

	case "retention":
		// retention policies are enforced by the reaper runner, not at write time
		return

	case "security_zone":
		evaluateSecurityZonePolicy(ctx, policy, result)

	case "data_classification":
		evaluateDataClassificationPolicy(ctx, policy, result)
	}
}

// evaluateSecurityZonePolicy checks security zone constraints on the
// proposed change.
func evaluateSecurityZonePolicy(ctx *GateContext, policy linkedPolicy, result *PolicyResult) {
	// security zone policies may restrict which operations are allowed,
	// which callers can modify entities in the zone, etc.
	restrictedOps, _ := policy.PolicyData["restricted_operations"].([]interface{})
	for _, op := range restrictedOps {
		if opStr, ok := op.(string); ok && opStr == ctx.Request.Operation {
			result.Blocks = append(result.Blocks,
				fmt.Sprintf("policy %s: operation %s restricted in security zone",
					policy.PolicyName, ctx.Request.Operation))
			return
		}
	}
}

// evaluateDataClassificationPolicy checks data classification constraints.
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

// queryEntityClassification reads the data classification for an entity.
func queryEntityClassification(db *pg.DB, entityType string, entityID int) string {
	query := fmt.Sprintf(
		"SELECT _access_classification FROM %s WHERE id = $1",
		pg.QuoteIdentifier(entityType),
	)

	var classification string
	err := db.QueryRow(query, entityID).Scan(&classification)
	if err != nil {
		return ""
	}
	return classification
}

// classificationAtLeast checks if classification a is at least as high as b.
func classificationAtLeast(a string, b string) bool {
	levels := map[string]int{
		"public":       0,
		"internal":     1,
		"confidential": 2,
		"restricted":   3,
		"regulated":    4,
	}

	aLevel := levels[strings.ToLower(a)]
	bLevel := levels[strings.ToLower(b)]
	return aLevel >= bLevel
}
