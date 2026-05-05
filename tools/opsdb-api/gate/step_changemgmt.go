//# tools/opsdb-api/gate/step_changemgmt.go

package gate

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/pg"
)

// fieldChangeTuple identifies one field being changed in a change set.
type fieldChangeTuple struct {
	EntityType string
	EntityID   int
	FieldName  string
}

// ownershipRecord identifies a responsible role for an entity.
type ownershipRecord struct {
	EntityType    string
	EntityID      int
	OpsUserRoleID int
	RoleName      string
	OwnershipRole string // owner, operator, security_reviewer, etc.
}

// approvalRule represents a loaded approval_rule policy row.
type approvalRule struct {
	RuleID               int
	RuleName             string
	TargetEntityTypes    []string
	TargetFields         []string
	TargetClassifications []string
	TargetSecurityZones  []string
	RequiredGroupID      int
	RequiredGroupName    string
	RequiredCount        int
	AutoApprovable       bool
}

// stepChangeMgmtRoute is gate step 7: Change Management Routing.
// Evaluates approval rules, walks ownership and stakeholder bridges,
// computes required approvals, determines auto-approve vs human approval.
func stepChangeMgmtRoute(ctx *GateContext) {
	if !isChangeManaged(ctx.Request.OperationClass) {
		ctx.CMRouting = &CMRoutingResult{AutoApproved: true}
		return
	}

	// SR1: enumerate field changes from the proposed change set
	fieldChanges := enumerateFieldChanges(ctx.Request)
	if len(fieldChanges) == 0 {
		ctx.CMRouting = &CMRoutingResult{AutoApproved: true}
		return
	}

	// SR2: walk ownership bridges for all touched entities
	owners, err := walkOwnershipBridges(ctx.DB, fieldChanges)
	if err != nil {
		reject(ctx, 7, "change_management_error",
			fmt.Sprintf("failed to resolve ownership: %v", err), nil)
		return
	}

	// SR3: walk stakeholder bridges for additional interested roles
	stakeholders, err := walkStakeholderBridges(ctx.DB, fieldChanges)
	if err != nil {
		warn(ctx, fmt.Sprintf("stakeholder resolution failed: %v", err))
		// stakeholder failure is a warning, not a rejection — ownership
		// is sufficient to route approvals
	}
	_ = stakeholders // stakeholders inform notification routing, not approval requirements

	// SR4: evaluate approval rules against touched entities and fields
	rules, err := loadApprovalRules(ctx.DB)
	if err != nil {
		reject(ctx, 7, "change_management_error",
			fmt.Sprintf("failed to load approval rules: %v", err), nil)
		return
	}

	matchingRules := matchApprovalRules(ctx, rules, fieldChanges, owners)

	// SR5: compute requirements — one per matching rule
	requirements := make([]ApprovalRequirement, 0, len(matchingRules))
	allAutoApprovable := true

	for _, rule := range matchingRules {
		requirements = append(requirements, ApprovalRequirement{
			RuleID:        rule.RuleID,
			GroupID:       rule.RequiredGroupID,
			GroupName:     rule.RequiredGroupName,
			CountRequired: rule.RequiredCount,
		})

		if !rule.AutoApprovable {
			allAutoApprovable = false
		}
	}

	// determine auto-approval
	autoApproved := false
	if len(requirements) == 0 {
		// no rules matched — auto-approve (low-risk change)
		autoApproved = true
	} else if allAutoApprovable {
		// all matching rules allow auto-approval
		autoApproved = true
	}

	ctx.CMRouting = &CMRoutingResult{
		AutoApproved:     autoApproved,
		ApprovalRequired: requirements,
	}
}

// enumerateFieldChanges extracts the list of (entity_type, entity_id, field_name)
// tuples from the request parameters.
func enumerateFieldChanges(req *GateRequest) []fieldChangeTuple {
	if req.Params == nil {
		return nil
	}

	rawChanges, ok := req.Params["field_changes"]
	if !ok {
		// single-entity write: construct from request target and fields
		fields := extractRequestedFields(req)
		if len(fields) == 0 {
			return nil
		}
		tuples := make([]fieldChangeTuple, 0, len(fields))
		for _, f := range fields {
			tuples = append(tuples, fieldChangeTuple{
				EntityType: req.TargetEntity,
				EntityID:   req.TargetEntityID,
				FieldName:  f,
			})
		}
		return tuples
	}

	changeList, ok := rawChanges.([]interface{})
	if !ok {
		return nil
	}

	var tuples []fieldChangeTuple
	for _, item := range changeList {
		changeMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		entityType, _ := changeMap["entity_type"].(string)
		entityID, _ := toInt(changeMap["entity_id"])
		fieldName, _ := changeMap["field_name"].(string)

		if entityType != "" && fieldName != "" {
			tuples = append(tuples, fieldChangeTuple{
				EntityType: entityType,
				EntityID:   entityID,
				FieldName:  fieldName,
			})
		}
	}

	return tuples
}

// walkOwnershipBridges reads ownership bridge tables for all entities
// touched by the change set. Returns the responsible roles.
func walkOwnershipBridges(db *pg.DB, changes []fieldChangeTuple) ([]ownershipRecord, error) {
	// deduplicate entities
	type entityRef struct {
		EntityType string
		EntityID   int
	}
	seen := make(map[entityRef]bool)
	var entities []entityRef
	for _, fc := range changes {
		ref := entityRef{fc.EntityType, fc.EntityID}
		if !seen[ref] {
			seen[ref] = true
			entities = append(entities, ref)
		}
	}

	var allOwners []ownershipRecord

	for _, entity := range entities {
		bridgeTable, fkColumn := ownershipBridgeFor(entity.EntityType)
		if bridgeTable == "" {
			continue
		}

		rows, err := db.Query(
			fmt.Sprintf(
				"SELECT o.%s, o.ops_user_role_id, r.name, o.ownership_role "+
					"FROM %s o "+
					"JOIN ops_user_role r ON r.id = o.ops_user_role_id "+
					"WHERE o.%s = $1 AND o.is_active = true AND r.is_active = true",
				pg.QuoteIdentifier(fkColumn),
				pg.QuoteIdentifier(bridgeTable),
				pg.QuoteIdentifier(fkColumn),
			),
			entity.EntityID,
		)
		if err != nil {
			if pg.IsUndefinedTable(err) {
				continue
			}
			return nil, fmt.Errorf("ownership query failed for %s: %w", entity.EntityType, err)
		}

		for rows.Next() {
			var record ownershipRecord
			var entityID int
			err := rows.Scan(&entityID, &record.OpsUserRoleID, &record.RoleName, &record.OwnershipRole)
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("ownership scan failed: %w", err)
			}
			record.EntityType = entity.EntityType
			record.EntityID = entityID
			allOwners = append(allOwners, record)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("ownership iteration failed: %w", err)
		}
	}

	return allOwners, nil
}

// ownershipBridgeFor returns the ownership bridge table and FK column
// for a given entity type.
func ownershipBridgeFor(entityType string) (string, string) {
	switch entityType {
	case "service":
		return "service_ownership", "service_id"
	case "machine":
		return "machine_ownership", "machine_id"
	case "k8s_cluster":
		return "k8s_cluster_ownership", "k8s_cluster_id"
	case "cloud_resource":
		return "cloud_resource_ownership", "cloud_resource_id"
	default:
		return "", ""
	}
}

// walkStakeholderBridges reads stakeholder bridge tables for touched entities.
// Stakeholders are interested parties who may receive notifications but
// are not necessarily required approvers.
func walkStakeholderBridges(db *pg.DB, changes []fieldChangeTuple) ([]ownershipRecord, error) {
	// deduplicate entities
	type entityRef struct {
		EntityType string
		EntityID   int
	}
	seen := make(map[entityRef]bool)
	var entities []entityRef
	for _, fc := range changes {
		ref := entityRef{fc.EntityType, fc.EntityID}
		if !seen[ref] {
			seen[ref] = true
			entities = append(entities, ref)
		}
	}

	var allStakeholders []ownershipRecord

	for _, entity := range entities {
		bridgeTable, fkColumn := stakeholderBridgeFor(entity.EntityType)
		if bridgeTable == "" {
			continue
		}

		rows, err := db.Query(
			fmt.Sprintf(
				"SELECT s.%s, s.ops_user_role_id, r.name, s.stakeholder_role "+
					"FROM %s s "+
					"JOIN ops_user_role r ON r.id = s.ops_user_role_id "+
					"WHERE s.%s = $1 AND s.is_active = true AND r.is_active = true",
				pg.QuoteIdentifier(fkColumn),
				pg.QuoteIdentifier(bridgeTable),
				pg.QuoteIdentifier(fkColumn),
			),
			entity.EntityID,
		)
		if err != nil {
			if pg.IsUndefinedTable(err) {
				continue
			}
			return nil, fmt.Errorf("stakeholder query failed for %s: %w", entity.EntityType, err)
		}

		for rows.Next() {
			var record ownershipRecord
			var entityID int
			err := rows.Scan(&entityID, &record.OpsUserRoleID, &record.RoleName, &record.OwnershipRole)
			if err != nil {
				rows.Close()
				return nil, err
			}
			record.EntityType = entity.EntityType
			record.EntityID = entityID
			allStakeholders = append(allStakeholders, record)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return allStakeholders, nil
}

// stakeholderBridgeFor returns the stakeholder bridge table and FK column
// for a given entity type.
func stakeholderBridgeFor(entityType string) (string, string) {
	switch entityType {
	case "service":
		return "service_stakeholder", "service_id"
	default:
		return "", ""
	}
}

// loadApprovalRules reads all active approval_rule policy rows.
func loadApprovalRules(db *pg.DB) ([]approvalRule, error) {
	rows, err := db.Query(
		"SELECT p.id, p.name, p.policy_data_json FROM policy p " +
			"WHERE p.policy_type = 'approval_rule' AND p.is_active = true",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query approval rules: %w", err)
	}
	defer rows.Close()

	var rules []approvalRule
	for rows.Next() {
		var ruleID int
		var ruleName string
		var dataJSON []byte
		if err := rows.Scan(&ruleID, &ruleName, &dataJSON); err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := pg.UnmarshalJSON(dataJSON, &data); err != nil {
			continue
		}

		rule := approvalRule{
			RuleID:            ruleID,
			RuleName:          ruleName,
			TargetEntityTypes: extractStringList(data, "target_entity_types"),
			TargetFields:      extractStringList(data, "target_fields"),
			TargetClassifications: extractStringList(data, "target_classifications"),
			TargetSecurityZones:   extractStringList(data, "target_security_zones"),
			RequiredCount:     extractIntOrDefault(data, "required_count", 1),
			AutoApprovable:    extractBoolOrDefault(data, "auto_approvable", false),
		}

		// resolve required group
		if groupName, ok := data["required_group"].(string); ok {
			rule.RequiredGroupName = groupName
			groupID, err := resolveGroupID(db, groupName)
			if err == nil {
				rule.RequiredGroupID = groupID
			}
		}

		rules = append(rules, rule)
	}

	return rules, rows.Err()
}

// matchApprovalRules finds which approval rules match the current change set.
func matchApprovalRules(ctx *GateContext, rules []approvalRule, changes []fieldChangeTuple, owners []ownershipRecord) []approvalRule {
	// collect unique entity types and field names from changes
	entityTypes := make(map[string]bool)
	fieldNames := make(map[string]bool)
	for _, fc := range changes {
		entityTypes[fc.EntityType] = true
		fieldNames[fc.FieldName] = true
	}

	var matched []approvalRule

	for _, rule := range rules {
		if ruleMatches(ctx, rule, entityTypes, fieldNames) {
			matched = append(matched, rule)
		}
	}

	return matched
}

// ruleMatches checks if a single approval rule matches the current change set.
func ruleMatches(ctx *GateContext, rule approvalRule, entityTypes map[string]bool, fieldNames map[string]bool) bool {
	// if rule specifies entity types, at least one must match
	if len(rule.TargetEntityTypes) > 0 {
		entityMatch := false
		for _, ruleEntity := range rule.TargetEntityTypes {
			if ruleEntity == "all" || entityTypes[ruleEntity] {
				entityMatch = true
				break
			}
		}
		if !entityMatch {
			return false
		}
	}

	// if rule specifies fields, at least one must match
	if len(rule.TargetFields) > 0 {
		fieldMatch := false
		for _, ruleField := range rule.TargetFields {
			if ruleField == "all" || fieldNames[ruleField] {
				fieldMatch = true
				break
			}
		}
		if !fieldMatch {
			return false
		}
	}

	// if rule specifies security zones, check entity membership
	if len(rule.TargetSecurityZones) > 0 {
		zoneMatch := checkSecurityZoneMembership(ctx.DB, ctx.Request.TargetEntity,
			ctx.Request.TargetEntityID, rule.TargetSecurityZones)
		if !zoneMatch {
			return false
		}
	}

	// if rule specifies classifications, check field classifications
	if len(rule.TargetClassifications) > 0 {
		classMatch := checkFieldClassificationMatch(ctx.DB, ctx.Request.TargetEntity,
			fieldNames, rule.TargetClassifications)
		if !classMatch {
			return false
		}
	}

	return true
}

// checkSecurityZoneMembership checks if an entity is a member of any of
// the specified security zones.
func checkSecurityZoneMembership(db *pg.DB, entityType string, entityID int, zones []string) bool {
	if entityID == 0 {
		return false
	}

	membershipTable := ""
	fkColumn := ""
	switch entityType {
	case "service":
		membershipTable = "security_zone_membership_service"
		fkColumn = "service_id"
	case "machine":
		membershipTable = "security_zone_membership_machine"
		fkColumn = "machine_id"
	case "k8s_namespace":
		membershipTable = "security_zone_membership_k8s_namespace"
		fkColumn = "k8s_namespace_id"
	default:
		return false
	}

	for _, zoneName := range zones {
		var exists bool
		err := db.QueryRow(
			fmt.Sprintf(
				"SELECT EXISTS(SELECT 1 FROM %s szm "+
					"JOIN security_zone sz ON sz.id = szm.security_zone_id "+
					"WHERE szm.%s = $1 AND sz.name = $2 "+
					"AND szm.is_active = true AND sz.is_active = true)",
				pg.QuoteIdentifier(membershipTable),
				pg.QuoteIdentifier(fkColumn),
			),
			entityID, zoneName,
		).Scan(&exists)
		if err == nil && exists {
			return true
		}
	}

	return false
}

// checkFieldClassificationMatch checks if any of the touched fields
// have a classification matching the rule's target classifications.
func checkFieldClassificationMatch(db *pg.DB, entityType string, fieldNames map[string]bool, classifications []string) bool {
	for _, classification := range classifications {
		for fieldName := range fieldNames {
			var fieldClassification string
			err := db.QueryRow(
				"SELECT _access_classification FROM _schema_field "+
					"WHERE entity_type = $1 AND field_name = $2",
				entityType, fieldName,
			).Scan(&fieldClassification)
			if err == nil && fieldClassification == classification {
				return true
			}
		}
	}
	return false
}

// resolveGroupID looks up an ops_group ID by name.
func resolveGroupID(db *pg.DB, groupName string) (int, error) {
	var groupID int
	err := db.QueryRow(
		"SELECT id FROM ops_group WHERE name = $1 AND is_active = true",
		groupName,
	).Scan(&groupID)
	return groupID, err
}

// --- data extraction helpers ---

func extractStringList(data map[string]interface{}, key string) []string {
	val, ok := data[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func extractIntOrDefault(data map[string]interface{}, key string, defaultVal int) int {
	val, ok := data[key]
	if !ok {
		return defaultVal
	}
	if f, ok := val.(float64); ok {
		return int(f)
	}
	if i, ok := val.(int); ok {
		return i
	}
	return defaultVal
}

func extractBoolOrDefault(data map[string]interface{}, key string, defaultVal bool) bool {
	val, ok := data[key]
	if !ok {
		return defaultVal
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return defaultVal
}
