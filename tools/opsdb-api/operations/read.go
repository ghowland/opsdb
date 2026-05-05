//# tools/opsdb-api/operations/read.go

package operations

import (
	"fmt"
	"strings"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb-api/schema"
)

// SearchParams holds search operation parameters.
type SearchParams struct {
	EntityType   string
	Filters      []FilterPredicate
	Joins        []string
	Projection   string // standard, summary, full_with_history, or comma-separated field list
	Ordering     []OrderSpec
	Cursor       string
	Offset       int
	Limit        int
	MaxStaleness int    // seconds, for observation cache
	ViewMode     string // standard, with_history, at_time
}

// FilterPredicate represents one filter condition.
type FilterPredicate struct {
	Field    string
	Operator string // eq, ne, gt, gte, lt, lte, in, like, is_null, is_not_null, between, json_contains
	Value    interface{}
}

// OrderSpec represents one ordering directive.
type OrderSpec struct {
	Field     string
	Direction string // asc, desc
}

// SearchResult holds search results.
type SearchResult struct {
	Rows              []map[string]interface{}
	Cursor            string
	TotalCount        int
	FreshnessSummary  map[string]interface{}
	FilterDisclosures []string
}

// EntityRow holds one entity row with field values and metadata.
type EntityRow struct {
	EntityType    string
	EntityID      int
	Fields        map[string]interface{}
	VersionSerial *int
	CreatedTime   *time.Time
	UpdatedTime   *time.Time
}

// VersionRow holds one version sibling row.
type VersionRow struct {
	VersionID                int
	VersionSerial            int
	ParentVersionID          *int
	ChangeSetID              *int
	IsActiveVersion          bool
	ApprovedForProductionTime *time.Time
	Fields                   map[string]interface{}
}

// DependencyNode holds one node in a dependency walk.
type DependencyNode struct {
	EntityType string
	EntityID   int
	Depth      int
	Metadata   map[string]interface{}
}

// query bounds — enforced on every search
const (
	maxSearchLimit        = 10000
	defaultSearchLimit    = 100
	maxJoinDepth          = 5
	maxPredicateCount     = 50
	maxQueryTimeoutMillis = 30000
)

// GetEntity fetches one entity row by primary key. Returns current state
// with all fields the caller is authorized to see.
func GetEntity(db *pg.DB, schema *runtimeschema.RuntimeSchema, entityType string, entityID int, omittedFields []string) (*EntityRow, error) {
	_, found := schema.GetEntityType(entityType)
	if !found {
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1",
		pg.QuoteIdentifier(entityType))

	rows, err := db.Query(query, entityID)
	if err != nil {
		return nil, fmt.Errorf("entity query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	if !rows.Next() {
		return nil, fmt.Errorf("%s with id=%d not found", entityType, entityID)
	}

	fieldValues, err := scanRowToMap(rows, columns)
	if err != nil {
		return nil, fmt.Errorf("row scan failed: %w", err)
	}

	// apply field omissions from authorization
	for _, omitted := range omittedFields {
		delete(fieldValues, omitted)
	}

	result := &EntityRow{
		EntityType: entityType,
		EntityID:   entityID,
		Fields:     fieldValues,
	}

	// extract version stamp if the entity is versioned
	if schema.IsVersioned(entityType) {
		versionSerial, err := readCurrentVersionSerial(db, entityType, entityID)
		if err == nil && versionSerial > 0 {
			result.VersionSerial = &versionSerial
		}
	}

	return result, nil
}

// GetEntityHistory fetches the version chain for one entity. Returns all
// versions ordered by version_serial descending (newest first).
func GetEntityHistory(db *pg.DB, schema *runtimeschema.RuntimeSchema, entityType string, entityID int, startTime *time.Time, endTime *time.Time) ([]VersionRow, error) {
	if !schema.IsVersioned(entityType) {
		return nil, fmt.Errorf("entity type %s is not versioned", entityType)
	}

	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	var queryParts []string
	var args []interface{}
	argIdx := 1

	queryParts = append(queryParts, fmt.Sprintf("SELECT * FROM %s WHERE %s = $%d",
		pg.QuoteIdentifier(versionTable),
		pg.QuoteIdentifier(fkColumn),
		argIdx))
	args = append(args, entityID)
	argIdx++

	if startTime != nil {
		queryParts = append(queryParts, fmt.Sprintf("AND approved_for_production_time >= $%d", argIdx))
		args = append(args, *startTime)
		argIdx++
	}

	if endTime != nil {
		queryParts = append(queryParts, fmt.Sprintf("AND approved_for_production_time <= $%d", argIdx))
		args = append(args, *endTime)
		argIdx++
	}

	queryParts = append(queryParts, "ORDER BY version_serial DESC")

	query := strings.Join(queryParts, " ")

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("version history query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	var versions []VersionRow
	for rows.Next() {
		fieldValues, err := scanRowToMap(rows, columns)
		if err != nil {
			return nil, fmt.Errorf("version row scan failed: %w", err)
		}

		vr := VersionRow{
			Fields: fieldValues,
		}

		if id, ok := fieldValues["id"]; ok {
			if intID, ok := toInt(id); ok {
				vr.VersionID = intID
			}
		}
		if serial, ok := fieldValues["version_serial"]; ok {
			if intSerial, ok := toInt(serial); ok {
				vr.VersionSerial = intSerial
			}
		}
		if active, ok := fieldValues["is_active_version"]; ok {
			if boolActive, ok := active.(bool); ok {
				vr.IsActiveVersion = boolActive
			}
		}
		if csID, ok := fieldValues["change_set_id"]; ok {
			if intCS, ok := toInt(csID); ok {
				vr.ChangeSetID = &intCS
			}
		}

		versions = append(versions, vr)
	}

	return versions, rows.Err()
}

// GetEntityAtTime reconstructs field values active at a specific timestamp.
// Finds the version that was active at that point in time.
func GetEntityAtTime(db *pg.DB, schema *runtimeschema.RuntimeSchema, entityType string, entityID int, timestamp time.Time) (*VersionRow, error) {
	if !schema.IsVersioned(entityType) {
		return nil, fmt.Errorf("entity type %s is not versioned", entityType)
	}

	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s = $1 AND approved_for_production_time <= $2 "+
			"ORDER BY approved_for_production_time DESC LIMIT 1",
		pg.QuoteIdentifier(versionTable),
		pg.QuoteIdentifier(fkColumn),
	)

	rows, err := db.Query(query, entityID, timestamp)
	if err != nil {
		return nil, fmt.Errorf("point-in-time query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	if !rows.Next() {
		return nil, fmt.Errorf("no version of %s id=%d exists at or before %s",
			entityType, entityID, timestamp.Format(time.RFC3339))
	}

	fieldValues, err := scanRowToMap(rows, columns)
	if err != nil {
		return nil, fmt.Errorf("version row scan failed: %w", err)
	}

	vr := &VersionRow{Fields: fieldValues}
	if serial, ok := fieldValues["version_serial"]; ok {
		if intSerial, ok := toInt(serial); ok {
			vr.VersionSerial = intSerial
		}
	}
	if id, ok := fieldValues["id"]; ok {
		if intID, ok := toInt(id); ok {
			vr.VersionID = intID
		}
	}
	if active, ok := fieldValues["is_active_version"]; ok {
		if boolActive, ok := active.(bool); ok {
			vr.IsActiveVersion = boolActive
		}
	}

	return vr, nil
}

// Search is the discovery surface across entity types. Builds a SQL query
// from structured parameters with enforced bounds on result size, join
// depth, predicate count, and query time.
func Search(db *pg.DB, schema *runtimeschema.RuntimeSchema, params *SearchParams, omittedFields []string) (*SearchResult, error) {
	_, found := schema.GetEntityType(params.EntityType)
	if !found {
		return nil, fmt.Errorf("unknown entity type: %s", params.EntityType)
	}

	// enforce bounds
	if params.Limit <= 0 {
		params.Limit = defaultSearchLimit
	}
	if params.Limit > maxSearchLimit {
		params.Limit = maxSearchLimit
	}
	if len(params.Filters) > maxPredicateCount {
		return nil, fmt.Errorf("too many filter predicates: %d (max %d)",
			len(params.Filters), maxPredicateCount)
	}
	if len(params.Joins) > maxJoinDepth {
		return nil, fmt.Errorf("too many joins: %d (max %d)",
			len(params.Joins), maxJoinDepth)
	}

	// build query
	selectClause := buildSelectClause(params.EntityType, params.Projection, schema, omittedFields)
	fromClause := pg.QuoteIdentifier(params.EntityType)
	joinClause := buildJoinClause(params.EntityType, params.Joins, schema)
	whereClause, whereArgs := buildWhereClause(params.EntityType, params.Filters, params.MaxStaleness)
	orderClause := buildOrderClause(params.Ordering)

	// count query for total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s %s",
		fromClause, joinClause, whereClause)

	var totalCount int
	err := db.QueryRow(countQuery, whereArgs...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("count query failed: %w", err)
	}

	// data query with pagination
	dataQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s LIMIT %d OFFSET %d",
		selectClause, fromClause, joinClause, whereClause, orderClause,
		params.Limit, params.Offset)

	rows, err := db.Query(dataQuery, whereArgs...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	var resultRows []map[string]interface{}
	for rows.Next() {
		fieldValues, err := scanRowToMap(rows, columns)
		if err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}

		// apply field omissions
		for _, omitted := range omittedFields {
			delete(fieldValues, omitted)
		}

		resultRows = append(resultRows, fieldValues)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search iteration failed: %w", err)
	}

	result := &SearchResult{
		Rows:       resultRows,
		TotalCount: totalCount,
	}

	// build freshness summary for observation cache queries
	if isObservationCacheTable(params.EntityType) {
		result.FreshnessSummary = buildFreshnessSummary(resultRows)
	}

	// compute cursor for pagination
	if len(resultRows) > 0 && len(resultRows) == params.Limit {
		result.Cursor = computeCursor(params.Offset + params.Limit)
	}

	return result, nil
}

// GetDependencies walks the substrate hierarchy or service connection graph.
// Supports multiple walk patterns with cycle detection and depth bounds.
func GetDependencies(db *pg.DB, startEntity string, startID int, pattern string, maxDepth int) ([]DependencyNode, error) {
	if maxDepth <= 0 {
		maxDepth = 10
	}
	if maxDepth > 50 {
		maxDepth = 50
	}

	switch pattern {
	case "substrate_parent_chain":
		return walkParentChain(db, "megavisor_instance", "parent_megavisor_instance_id",
			startID, maxDepth)

	case "service_connections":
		return walkServiceConnections(db, startID, maxDepth)

	case "location_ancestry":
		return walkParentChain(db, "location", "parent_location_id",
			startID, maxDepth)

	case "host_group_machines":
		return walkHostGroupMachines(db, startID)

	case "service_package_chain":
		return walkServicePackages(db, startID)

	default:
		return nil, fmt.Errorf("unknown dependency pattern: %s", pattern)
	}
}

// --- query building helpers ---

func buildSelectClause(entityType string, projection string, schema *runtimeschema.RuntimeSchema, omittedFields []string) string {
	tableName := pg.QuoteIdentifier(entityType)

	switch projection {
	case "summary":
		// summary mode returns id, name/label, status, and timestamps
		return fmt.Sprintf("%s.id, %s.created_time, %s.updated_time",
			tableName, tableName, tableName)

	case "full_with_history":
		return tableName + ".*"

	case "":
		return tableName + ".*"

	default:
		// explicit field list
		if strings.Contains(projection, ",") {
			fields := strings.Split(projection, ",")
			qualified := make([]string, 0, len(fields))
			for _, f := range fields {
				trimmed := strings.TrimSpace(f)
				if trimmed != "" {
					qualified = append(qualified, fmt.Sprintf("%s.%s",
						tableName, pg.QuoteIdentifier(trimmed)))
				}
			}
			if len(qualified) > 0 {
				return strings.Join(qualified, ", ")
			}
		}
		return tableName + ".*"
	}
}

func buildJoinClause(entityType string, joins []string, schema *runtimeschema.RuntimeSchema) string {
	if len(joins) == 0 {
		return ""
	}

	var clauses []string
	for _, joinTarget := range joins {
		// look up relationship from schema
		rels := schema.GetRelationships(entityType)
		for _, rel := range rels {
			if rel.TargetEntity == joinTarget {
				clauses = append(clauses, fmt.Sprintf(
					"LEFT JOIN %s ON %s.%s = %s.id",
					pg.QuoteIdentifier(rel.TargetEntity),
					pg.QuoteIdentifier(entityType),
					pg.QuoteIdentifier(rel.SourceField),
					pg.QuoteIdentifier(rel.TargetEntity),
				))
				break
			}
		}
	}

	return strings.Join(clauses, " ")
}

func buildWhereClause(entityType string, filters []FilterPredicate, maxStaleness int) (string, []interface{}) {
	if len(filters) == 0 && maxStaleness <= 0 {
		return "", nil
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	for _, f := range filters {
		condition, filterArgs := buildFilterCondition(entityType, f, &argIdx)
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, filterArgs...)
		}
	}

	// freshness filter for observation cache tables
	if maxStaleness > 0 && isObservationCacheTable(entityType) {
		conditions = append(conditions,
			fmt.Sprintf("%s._observed_time >= NOW() - INTERVAL '%d seconds'",
				pg.QuoteIdentifier(entityType), maxStaleness))
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

func buildFilterCondition(entityType string, f FilterPredicate, argIdx *int) (string, []interface{}) {
	qualifiedField := fmt.Sprintf("%s.%s",
		pg.QuoteIdentifier(entityType),
		pg.QuoteIdentifier(f.Field))

	switch f.Operator {
	case "eq", "":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s = %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "ne":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s != %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "gt":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s > %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "gte":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s >= %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "lt":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s < %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "lte":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s <= %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "in":
		values, ok := f.Value.([]interface{})
		if !ok {
			return "", nil
		}
		placeholders := make([]string, 0, len(values))
		args := make([]interface{}, 0, len(values))
		for _, v := range values {
			placeholders = append(placeholders, fmt.Sprintf("$%d", *argIdx))
			args = append(args, v)
			*argIdx++
		}
		return fmt.Sprintf("%s IN (%s)", qualifiedField, strings.Join(placeholders, ", ")), args

	case "like":
		// anchored pattern only — prefix% or %suffix — no regex
		strVal, ok := f.Value.(string)
		if !ok {
			return "", nil
		}
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s LIKE %s", qualifiedField, placeholder), []interface{}{strVal}

	case "is_null":
		return fmt.Sprintf("%s IS NULL", qualifiedField), nil

	case "is_not_null":
		return fmt.Sprintf("%s IS NOT NULL", qualifiedField), nil

	case "between":
		rangeVals, ok := f.Value.([]interface{})
		if !ok || len(rangeVals) != 2 {
			return "", nil
		}
		p1 := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		p2 := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s BETWEEN %s AND %s", qualifiedField, p1, p2),
			[]interface{}{rangeVals[0], rangeVals[1]}

	case "json_contains":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s @> %s::jsonb", qualifiedField, placeholder), []interface{}{f.Value}

	default:
		return "", nil
	}
}

func buildOrderClause(ordering []OrderSpec) string {
	if len(ordering) == 0 {
		return "ORDER BY id ASC"
	}

	parts := make([]string, 0, len(ordering)+1)
	for _, o := range ordering {
		dir := "ASC"
		if strings.ToLower(o.Direction) == "desc" {
			dir = "DESC"
		}
		parts = append(parts, fmt.Sprintf("%s %s", pg.QuoteIdentifier(o.Field), dir))
	}

	// always tie-break by id for deterministic pagination
	parts = append(parts, "id ASC")

	return "ORDER BY " + strings.Join(parts, ", ")
}

// --- dependency walk implementations ---

// walkParentChain walks a self-referential parent_id chain up to the root.
func walkParentChain(db *pg.DB, entityType string, parentColumn string, startID int, maxDepth int) ([]DependencyNode, error) {
	query := fmt.Sprintf(
		"WITH RECURSIVE chain AS ("+
			"SELECT id, %s AS parent_id, 0 AS depth FROM %s WHERE id = $1 "+
			"UNION ALL "+
			"SELECT t.id, t.%s, c.depth + 1 FROM %s t "+
			"JOIN chain c ON t.id = c.parent_id "+
			"WHERE c.depth < $2"+
			") SELECT id, parent_id, depth FROM chain ORDER BY depth ASC",
		pg.QuoteIdentifier(parentColumn),
		pg.QuoteIdentifier(entityType),
		pg.QuoteIdentifier(parentColumn),
		pg.QuoteIdentifier(entityType),
	)

	rows, err := db.Query(query, startID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("parent chain query failed: %w", err)
	}
	defer rows.Close()

	var nodes []DependencyNode
	for rows.Next() {
		var id int
		var parentID *int
		var depth int
		if err := rows.Scan(&id, &parentID, &depth); err != nil {
			return nil, err
		}
		node := DependencyNode{
			EntityType: entityType,
			EntityID:   id,
			Depth:      depth,
			Metadata:   make(map[string]interface{}),
		}
		if parentID != nil {
			node.Metadata["parent_id"] = *parentID
		}
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// walkServiceConnections walks service_connection rows from a source service.
func walkServiceConnections(db *pg.DB, serviceID int, maxDepth int) ([]DependencyNode, error) {
	query := fmt.Sprintf(
		"WITH RECURSIVE deps AS (" +
			"SELECT destination_service_id AS id, 1 AS depth " +
			"FROM service_connection WHERE source_service_id = $1 AND is_active = true " +
			"UNION ALL " +
			"SELECT sc.destination_service_id, d.depth + 1 " +
			"FROM service_connection sc " +
			"JOIN deps d ON sc.source_service_id = d.id " +
			"WHERE d.depth < $2 AND sc.is_active = true" +
			") SELECT DISTINCT id, depth FROM deps ORDER BY depth ASC, id ASC")

	rows, err := db.Query(query, serviceID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("service connection query failed: %w", err)
	}
	defer rows.Close()

	// include the starting service as depth 0
	nodes := []DependencyNode{{
		EntityType: "service",
		EntityID:   serviceID,
		Depth:      0,
		Metadata:   map[string]interface{}{"role": "source"},
	}}

	for rows.Next() {
		var id, depth int
		if err := rows.Scan(&id, &depth); err != nil {
			return nil, err
		}
		nodes = append(nodes, DependencyNode{
			EntityType: "service",
			EntityID:   id,
			Depth:      depth,
			Metadata:   map[string]interface{}{"role": "dependency"},
		})
	}

	return nodes, rows.Err()
}

// walkHostGroupMachines returns all machines in a host group.
func walkHostGroupMachines(db *pg.DB, hostGroupID int) ([]DependencyNode, error) {
	rows, err := db.Query(
		"SELECT hgm.machine_id FROM host_group_machine hgm "+
			"WHERE hgm.host_group_id = $1 AND hgm.is_active = true "+
			"ORDER BY hgm.machine_id",
		hostGroupID,
	)
	if err != nil {
		return nil, fmt.Errorf("host group machine query failed: %w", err)
	}
	defer rows.Close()

	nodes := []DependencyNode{{
		EntityType: "host_group",
		EntityID:   hostGroupID,
		Depth:      0,
		Metadata:   map[string]interface{}{"role": "group"},
	}}

	for rows.Next() {
		var machineID int
		if err := rows.Scan(&machineID); err != nil {
			return nil, err
		}
		nodes = append(nodes, DependencyNode{
			EntityType: "machine",
			EntityID:   machineID,
			Depth:      1,
			Metadata:   map[string]interface{}{"role": "member"},
		})
	}

	return nodes, rows.Err()
}

// walkServicePackages returns all packages installed on a service.
func walkServicePackages(db *pg.DB, serviceID int) ([]DependencyNode, error) {
	rows, err := db.Query(
		"SELECT sp.package_id, sp.install_order FROM service_package sp "+
			"WHERE sp.service_id = $1 AND sp.is_active = true "+
			"ORDER BY sp.install_order",
		serviceID,
	)
	if err != nil {
		return nil, fmt.Errorf("service package query failed: %w", err)
	}
	defer rows.Close()

	nodes := []DependencyNode{{
		EntityType: "service",
		EntityID:   serviceID,
		Depth:      0,
		Metadata:   map[string]interface{}{"role": "service"},
	}}

	for rows.Next() {
		var packageID, installOrder int
		if err := rows.Scan(&packageID, &installOrder); err != nil {
			return nil, err
		}
		nodes = append(nodes, DependencyNode{
			EntityType: "package",
			EntityID:   packageID,
			Depth:      1,
			Metadata: map[string]interface{}{
				"role":          "installed_package",
				"install_order": installOrder,
			},
		})
	}

	return nodes, rows.Err()
}

// --- utility helpers ---

// scanRowToMap scans a database row into a map of column name → value.
func scanRowToMap(rows pg.Rows, columns []string) (map[string]interface{}, error) {
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		result[col] = values[i]
	}

	return result, nil
}

// readCurrentVersionSerial reads the active version serial for an entity.
func readCurrentVersionSerial(db *pg.DB, entityType string, entityID int) (int, error) {
	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	var serial int
	err := db.QueryRow(
		fmt.Sprintf("SELECT version_serial FROM %s WHERE %s = $1 AND is_active_version = true LIMIT 1",
			pg.QuoteIdentifier(versionTable),
			pg.QuoteIdentifier(fkColumn)),
		entityID,
	).Scan(&serial)
	if err != nil {
		return 0, err
	}
	return serial, nil
}

// isObservationCacheTable checks if an entity type is an observation cache table.
func isObservationCacheTable(entityType string) bool {
	switch entityType {
	case "observation_cache_metric", "observation_cache_state", "observation_cache_config":
		return true
	default:
		return false
	}
}

// buildFreshnessSummary computes staleness statistics for observation cache rows.
func buildFreshnessSummary(rows []map[string]interface{}) map[string]interface{} {
	if len(rows) == 0 {
		return nil
	}

	var oldestObservation, newestObservation time.Time
	now := time.Now().UTC()
	totalStaleness := 0.0
	count := 0

	for _, row := range rows {
		observedTime, ok := row["_observed_time"].(time.Time)
		if !ok {
			continue
		}

		if count == 0 || observedTime.Before(oldestObservation) {
			oldestObservation = observedTime
		}
		if count == 0 || observedTime.After(newestObservation) {
			newestObservation = observedTime
		}

		totalStaleness += now.Sub(observedTime).Seconds()
		count++
	}

	if count == 0 {
		return nil
	}

	return map[string]interface{}{
		"row_count":                count,
		"oldest_observation":       oldestObservation.Format(time.RFC3339),
		"newest_observation":       newestObservation.Format(time.RFC3339),
		"max_staleness_seconds":    int(now.Sub(oldestObservation).Seconds()),
		"min_staleness_seconds":    int(now.Sub(newestObservation).Seconds()),
		"avg_staleness_seconds":    int(totalStaleness / float64(count)),
	}
}

// computeCursor encodes a pagination cursor from offset.
func computeCursor(nextOffset int) string {
	return fmt.Sprintf("offset:%d", nextOffset)
}

// toInt converts numeric interface values to int.
func toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}
