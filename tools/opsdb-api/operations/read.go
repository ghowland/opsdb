
// === opsdb-api/operations/read.go ===
package operations

// GetEntity fetches one entity row by primary key.
// Returns current state with all fields the caller is authorized to see.
func GetEntity(entityType string, entityID int, authzResult interface{}) (interface{}, error) {
	// TODO: SELECT * FROM {entityType} WHERE id = {entityID}
	// TODO: apply field omissions from authzResult.OmittedFields
	// TODO: return structured row data with version stamp and governance metadata
	return nil, nil
}

// GetEntityHistory fetches the version chain for one entity.
// Returns current state plus all prior versions, ordered by version_serial.
func GetEntityHistory(entityType string, entityID int, timeRange interface{}) (interface{}, error) {
	// TODO: SELECT * FROM {entityType}_version WHERE {entityType}_id = {entityID}
	//       ORDER BY version_serial DESC
	// TODO: optionally filter by time range on approved_for_production_time
	// TODO: return ordered version chain
	return nil, nil
}

// GetEntityAtTime reconstructs field values active at a specific timestamp.
// Single lookup against version sibling — O(1) because versions contain full state.
func GetEntityAtTime(entityType string, entityID int, timestamp interface{}) (interface{}, error) {
	// TODO: SELECT * FROM {entityType}_version
	//       WHERE {entityType}_id = {entityID}
	//       AND approved_for_production_time <= {timestamp}
	//       ORDER BY approved_for_production_time DESC LIMIT 1
	// TODO: return reconstructed row at that point in time
	return nil, nil
}

// Search is the discovery surface across entity types.
// Accepts filters, joins, projection, ordering, pagination, freshness, view mode.
func Search(params *SearchParams) (*SearchResult, error) {
	// TODO: build SQL query from params:
	//   WHERE clause from filter predicates (equality, inequality, comparison, IN, LIKE anchored, IS NULL, BETWEEN, JSON containment)
	//   JOIN clause from named join paths (registered in _schema_relationship)
	//   SELECT clause from projection mode (standard, summary, full_with_history, explicit fields)
	//   ORDER BY from ordering params (field + direction pairs, tie-break by id)
	//   cursor or offset pagination
	// TODO: apply bounds: max result size, max join depth, max query time, max predicate depth
	//   reject if bounds exceeded
	// TODO: apply freshness filter for observation cache rows (max_staleness_seconds)
	// TODO: apply field omissions from authorization
	// TODO: return SearchResult with rows, cursor, count, freshness summary, filtering disclosures
	return nil, nil
}

// SearchParams holds search operation parameters.
type SearchParams struct {
	EntityType    string
	Filters       []FilterPredicate
	Joins         []string
	Projection    string // standard, summary, full_with_history, or field list
	Ordering      []OrderSpec
	Cursor        string
	Offset        int
	Limit         int
	MaxStaleness  int // seconds, for observation cache
	ViewMode      string // standard, with_history, at_time
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

// GetDependencies walks the substrate hierarchy or service connection graph.
// Used for stack-walking queries (decommission awareness, failure domain analysis, etc.).
func GetDependencies(startEntity string, startID int, pattern string, maxDepth int) (interface{}, error) {
	// TODO: switch on pattern:
	//   "substrate_parent_chain": recursive CTE walking megavisor_instance.parent_megavisor_instance_id
	//   "service_connections": walk service_connection rows from source service
	//   "location_ancestry": walk location.parent_location_id up to root
	//   "host_group_machines": walk host_group → host_group_machine → machine
	// TODO: enforce maxDepth and cycle detection
	// TODO: return dependency chain as ordered list of (entity_type, entity_id, depth, metadata)
	return nil, nil
}


