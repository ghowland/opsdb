//# tools/opsdb-api/gate/step_changemgmt.go

package gate

import (
	"encoding/json"
	"fmt"

	"github.com/ghowland/opsdb/internal/pg"
)

// fieldChangeTuple identifies one field being changed in a change set.
// Extracted from the request params to drive ownership resolution and
// approval rule matching.
type fieldChangeTuple struct {
	EntityType string
	EntityID   int
	FieldName  string
}

// ownershipRecord identifies a responsible role for an entity, loaded
// from ownership bridge tables (service_ownership, machine_ownership,
// k8s_cluster_ownership, cloud_resource_ownership).
type ownershipRecord struct {
	EntityType    string
	EntityID      int
	OpsUserRoleID int
	RoleName      string
	OwnershipRole string // owner, technical_owner, business_owner, support_owner
}

// approvalRule represents a loaded approval_rule policy row. Rules are
// matched against change sets by entity type, field name, security zone
// membership, and field classification. When a rule matches, it produces
// an ApprovalRequirement that step 9 writes as a
// change_set_approval_required row.
type approvalRule struct {
	RuleID                int
	RuleName              string
	TargetEntityTypes     []string
	TargetFields          []string
	TargetClassifications []string
	TargetSecurityZones   []string
	RequiredGroupID       int
	RequiredGroupName     string
	RequiredCount         int
	AutoApprovable        bool
}

// stepChangeMgmtRoute is gate step 7: Change Management Routing.
// For change set submissions (write-cs operations), evaluates approval
// rules to determine who must approve, walks ownership and stakeholder
// bridges to find responsible roles, computes approval requirements,
// and determines whether the change set can auto-approve.
//
// Skips for operations that don't go through change management: reads,
// streams, direct observation writes, and change management actions
// (approve, reject, cancel, apply, mark_applied — these act on
// existing change sets, they don't create new ones).
//
// The five-step sequence within this step:
//
//	SR1: Enumerate field changes from the request
//	SR2: Walk ownership bridges for all touched entities
//	SR3: Walk stakeholder bridges (for notification routing, not approval)
//	SR4: Load and match approval rules
//	SR5: Compute requirements and determine auto-approval
func stepChangeMgmtRoute(ctx *GateContext) {
	// Non-change-managed operations skip entirely. Direct writes
	// (write_observation) and change management actions operate
	// outside the approval pipeline.
	if !isChangeManaged(ctx.Request.OperationClass) {
		return
	}

	// SR1: Enumerate the field changes from the proposed change set.
	fieldChanges := enumerateFieldChanges(ctx.Request)
	if len(fieldChanges) == 0 {
		// A change set with no field changes auto-approves — there's
		// nothing to govern.
		ctx.CMRouting = &CMRoutingResult{AutoApproved: true}
		return
	}

	// SR2: Walk ownership bridges for all touched entities to find
	// the responsible roles. These inform approval routing — who
	// must approve changes to their entities.
	owners, err := walkOwnershipBridges(ctx.DB, fieldChanges)
	if err != nil {
		reject(ctx, 7, "change_management_error",
			fmt.Sprintf("failed to resolve ownership: %v", err), nil)
		return
	}

	// SR3: Walk stakeholder bridges for notification routing.
	// Stakeholders are interested parties who receive notifications
	// but are not necessarily required approvers. Failure here is
	// a warning, not a rejection — ownership is sufficient for
	// approval routing.
	_, stakeholderErr := walkStakeholderBridges(ctx.DB, fieldChanges)
	if stakeholderErr != nil {
		warn(ctx, fmt.Sprintf("stakeholder resolution failed: %v", stakeholderErr))
	}

	// SR4: Load all active approval rules and match them against
	// the change set's entity types, fields, security zones, and
	// classifications.
	rules, err := loadApprovalRules(ctx.DB)
	if err != nil {
		reject(ctx, 7, "change_management_error",
			fmt.Sprintf("failed to load approval rules: %v", err), nil)
		return
	}

	matchingRules := matchApprovalRules(ctx, rules, fieldChanges, owners)

	// SR5: Compute requirements — one per matching rule.
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

	// Determine auto-approval. A change set auto-approves when:
	// - No rules matched (low-risk change with no governance rules), OR
	// - All matching rules have auto_approvable=true in their policy data
	autoApproved := false
	if len(requirements) == 0 {
		autoApproved = true
	} else if allAutoApprovable {
		autoApproved = true
	}

	ctx.CMRouting = &CMRoutingResult{
		AutoApproved:     autoApproved,
		ApprovalRequired: requirements,
	}
}

// ---------------------------------------------------------------------------
// SR1: Field change enumeration
// ---------------------------------------------------------------------------

// enumerateFieldChanges extracts the list of (entity_type, entity_id,
// field_name) tuples from the request parameters. For change set
// submissions, the tuples come from the field_changes array. For
// single-entity writes that are change-managed, the tuple is
// constructed from the request target and fields.
func enumerateFieldChanges(req *GateRequest) []fieldChangeTuple {
	if req.Params == nil {
		return nil
	}

	// Change set submissions carry field changes as an array
	rawChanges, ok := req.Params["field_changes"]
	if ok {
		return enumerateFromFieldChangesArray(rawChanges, req)
	}

	// Single-entity change-managed write: construct tuples from the
	// request target and requested fields
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

// enumerateFromFieldChangesArray parses the field_changes array from
// request params into fieldChangeTuple structs.
func enumerateFromFieldChangesArray(rawChanges interface{}, req *GateRequest) []fieldChangeTuple {
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
		fieldName, _ := changeMap["field_name"].(string)

		if entityType == "" {
			entityType = req.TargetEntity
		}
		if entityType == "" || fieldName == "" {
			continue
		}

		entityID, _ := toInt(changeMap["entity_id"])

		tuples = append(tuples, fieldChangeTuple{
			EntityType: entityType,
			EntityID:   entityID,
			FieldName:  fieldName,
		})
	}

	return tuples
}

// ---------------------------------------------------------------------------
// SR2: Ownership bridge walking
// ---------------------------------------------------------------------------

// entityRef identifies a unique entity being changed, for deduplication
// when walking bridges.
type entityRef struct {
	EntityType string
	EntityID   int
}

// walkOwnershipBridges reads ownership bridge tables for all entities
// touched by the change set. Returns the responsible roles which drive
// approval routing.
func walkOwnershipBridges(db *pg.DB, changes []fieldChangeTuple) ([]ownershipRecord, error) {
	entities := deduplicateEntities(changes)

	var allOwners []ownershipRecord

	for _, entity := range entities {
		if entity.EntityID == 0 {
			// Creates don't have entity IDs yet — can't look up ownership.
			// Approval rules handle creates through entity-type matching
			// rather than ownership matching.
			continue
		}

		bridgeTable, fkColumn := ownershipBridgeFor(entity.EntityType)
		if bridgeTable == "" {
			continue
		}

		owners, err := queryOwnershipBridge(db, bridgeTable, fkColumn, entity)
		if err != nil {
			return nil, err
		}

		allOwners = append(allOwners, owners...)
	}

	return allOwners, nil
}

// queryOwnershipBridge reads one ownership bridge table for one entity.
func queryOwnershipBridge(db *pg.DB, bridgeTable string, fkColumn string, entity entityRef) ([]ownershipRecord, error) {
	rows, err := db.Query(
		fmt.Sprintf(
			"SELECT o.%s, o.ops_user_role_id, r.name, o.ownership_role "+
				"FROM %s o "+
				"JOIN ops_user_role r ON r.id = o.ops_user_role_id "+
				"WHERE o.%s = $1 AND r.is_active = true",
			pg.QuoteIdentifier(fkColumn),
			pg.QuoteIdentifier(bridgeTable),
			pg.QuoteIdentifier(fkColumn),
		),
		entity.EntityID,
	)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ownership query failed for %s: %w",
			entity.EntityType, err)
	}
	defer rows.Close()

	var owners []ownershipRecord
	for rows.Next() {
		var record ownershipRecord
		var entityID int
		err := rows.Scan(&entityID, &record.OpsUserRoleID,
			&record.RoleName, &record.OwnershipRole)
		if err != nil {
			return nil, fmt.Errorf("ownership scan failed: %w", err)
		}
		record.EntityType = entity.EntityType
		record.EntityID = entityID
		owners = append(owners, record)
	}

	return owners, rows.Err()
}

// ownershipBridgeFor returns the ownership bridge table and FK column
// for a given entity type. Returns empty strings for entity types that
// don't have ownership bridges.
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

// ---------------------------------------------------------------------------
// SR3: Stakeholder bridge walking
// ---------------------------------------------------------------------------

// walkStakeholderBridges reads stakeholder bridge tables for touched
// entities. Stakeholders are interested parties who receive notifications
// about changes to entities they care about, but are not necessarily
// required approvers. The returned records inform the notification
// runner's dispatch, not the approval requirements.
func walkStakeholderBridges(db *pg.DB, changes []fieldChangeTuple) ([]ownershipRecord, error) {
	entities := deduplicateEntities(changes)

	var allStakeholders []ownershipRecord

	for _, entity := range entities {
		if entity.EntityID == 0 {
			continue
		}

		bridgeTable, fkColumn := stakeholderBridgeFor(entity.EntityType)
		if bridgeTable == "" {
			continue
		}

		rows, err := db.Query(
			fmt.Sprintf(
				"SELECT s.%s, s.ops_user_role_id, r.name, s.stakeholder_role "+
					"FROM %s s "+
					"JOIN ops_user_role r ON r.id = s.ops_user_role_id "+
					"WHERE s.%s = $1 AND r.is_active = true",
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
			return nil, fmt.Errorf("stakeholder query failed for %s: %w",
				entity.EntityType, err)
		}

		for rows.Next() {
			var record ownershipRecord
			var entityID int
			err := rows.Scan(&entityID, &record.OpsUserRoleID,
				&record.RoleName, &record.OwnershipRole)
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("stakeholder scan failed: %w", err)
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
// for a given entity type. Currently only services have a stakeholder
// bridge; other entity types will get theirs as the schema grows.
func stakeholderBridgeFor(entityType string) (string, string) {
	switch entityType {
	case "service":
		return "service_stakeholder", "service_id"
	default:
		return "", ""
	}
}

// ---------------------------------------------------------------------------
// SR4: Approval rule loading and matching
// ---------------------------------------------------------------------------

// loadApprovalRules reads all active approval_rule policy rows. Each
// rule's policy_data_json carries the matching predicates (entity types,
// fields, security zones, classifications) and the approval requirement
// (required group, count, auto-approvable flag).
func loadApprovalRules(db *pg.DB) ([]approvalRule, error) {
	rows, err := db.Query(
		"SELECT p.id, p.name, p.policy_data_json FROM policy p " +
			"WHERE p.policy_type = 'approval_rule' AND p.is_active = true",
	)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			return nil, nil
		}
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
		if err := json.Unmarshal(dataJSON, &data); err != nil {
			continue
		}

		rule := approvalRule{
			RuleID:                ruleID,
			RuleName:              ruleName,
			TargetEntityTypes:     extractStringList(data, "target_entity_types"),
			TargetFields:          extractStringList(data, "target_fields"),
			TargetClassifications: extractStringList(data, "target_classifications"),
			TargetSecurityZones:   extractStringList(data, "target_security_zones"),
			RequiredCount:         extractIntOrDefault(data, "required_count", 1),
			AutoApprovable:        extractBoolOrDefault(data, "auto_approvable", false),
		}

		// Resolve the required approver group by name
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

// matchApprovalRules finds which approval rules match the current change
// set. A rule matches if ALL of its specified predicates are satisfied
// (AND composition). Predicates that are empty (no target_entity_types
// specified) match anything.
func matchApprovalRules(ctx *GateContext, rules []approvalRule, changes []fieldChangeTuple, owners []ownershipRecord) []approvalRule {
	// Collect unique entity types and field names from the change set
	entityTypes := make(map[string]bool)
	fieldNames := make(map[string]bool)
	for _, fc := range changes {
		entityTypes[fc.EntityType] = true
		fieldNames[fc.FieldName] = true
	}

	var matched []approvalRule
	for _, rule := range rules {
		if ruleMatchesChangeSet(ctx, rule, entityTypes, fieldNames) {
			matched = append(matched, rule)
		}
	}

	return matched
}

// ruleMatchesChangeSet checks if a single approval rule matches the
// current change set. Each predicate (entity types, fields, security
// zones, classifications) is checked independently; the rule matches
// only if ALL specified predicates are satisfied.
func ruleMatchesChangeSet(ctx *GateContext, rule approvalRule, entityTypes map[string]bool, fieldNames map[string]bool) bool {
	// Entity type predicate: if specified, at least one changed entity
	// type must match. "all" is a wildcard.
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

	// Field name predicate: if specified, at least one changed field
	// must match. "all" is a wildcard.
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

	// Security zone predicate: if specified, the target entity must
	// be a member of at least one of the specified zones.
	if len(rule.TargetSecurityZones) > 0 {
		zoneMatch := checkSecurityZoneMembership(
			ctx.DB, ctx.Request.TargetEntity,
			ctx.Request.TargetEntityID, rule.TargetSecurityZones)
		if !zoneMatch {
			return false
		}
	}

	// Classification predicate: if specified, at least one changed
	// field must have a classification matching the rule's targets.
	if len(rule.TargetClassifications) > 0 {
		classMatch := checkFieldClassificationMatch(
			ctx.DB, ctx.Request.TargetEntity,
			fieldNames, rule.TargetClassifications)
		if !classMatch {
			return false
		}
	}

	return true
}

// ---------------------------------------------------------------------------
// Security zone and classification matching
// ---------------------------------------------------------------------------

// checkSecurityZoneMembership checks if an entity is a member of any
// of the specified security zones via the appropriate zone membership
// bridge table.
func checkSecurityZoneMembership(db *pg.DB, entityType string, entityID int, zones []string) bool {
	if entityID == 0 {
		return false
	}

	membershipTable, fkColumn := securityZoneBridgeFor(entityType)
	if membershipTable == "" {
		return false
	}

	for _, zoneName := range zones {
		var exists bool
		err := db.QueryRow(
			fmt.Sprintf(
				"SELECT EXISTS("+
					"SELECT 1 FROM %s szm "+
					"JOIN security_zone sz ON sz.id = szm.security_zone_id "+
					"WHERE szm.%s = $1 AND sz.name = $2 "+
					"AND sz.is_active = true"+
					")",
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

// securityZoneBridgeFor returns the security zone membership bridge table
// and FK column for a given entity type.
func securityZoneBridgeFor(entityType string) (string, string) {
	switch entityType {
	case "service":
		return "security_zone_membership_service", "service_id"
	case "machine":
		return "security_zone_membership_machine", "machine_id"
	case "k8s_namespace":
		return "security_zone_membership_k8s_namespace", "k8s_namespace_id"
	default:
		return "", ""
	}
}

// checkFieldClassificationMatch checks if any of the changed fields
// have a classification matching the rule's target classifications.
// Reads from the _schema_field metadata table.
func checkFieldClassificationMatch(db *pg.DB, entityType string, fieldNames map[string]bool, classifications []string) bool {
	for _, classification := range classifications {
		for fieldName := range fieldNames {
			var fieldClassification string
			err := db.QueryRow(
				"SELECT constraint_data_json->>'access_classification' "+
					"FROM _schema_field "+
					"WHERE _schema_entity_type_id = ("+
					"  SELECT id FROM _schema_entity_type WHERE table_name = $1 LIMIT 1"+
					") AND field_name = $2",
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

// ---------------------------------------------------------------------------
// Entity deduplication
// ---------------------------------------------------------------------------

// deduplicateEntities extracts unique (entity_type, entity_id) pairs
// from a list of field change tuples. Used before walking bridges to
// avoid querying the same entity's ownership multiple times.
func deduplicateEntities(changes []fieldChangeTuple) []entityRef {
	seen := make(map[entityRef]bool)
	var entities []entityRef
	for _, fc := range changes {
		ref := entityRef{fc.EntityType, fc.EntityID}
		if !seen[ref] {
			seen[ref] = true
			entities = append(entities, ref)
		}
	}
	return entities
}

// ---------------------------------------------------------------------------
// Policy data extraction helpers
// ---------------------------------------------------------------------------

// extractStringList reads a string list from a parsed JSON map. Handles
// both []string and []interface{} representations (the latter is what
// json.Unmarshal produces).
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

// extractIntOrDefault reads an int from a parsed JSON map with a default
// value if the key is missing or not numeric.
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

// extractBoolOrDefault reads a bool from a parsed JSON map with a default
// value if the key is missing or not boolean.
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
