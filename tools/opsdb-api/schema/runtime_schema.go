//# tools/opsdb-api/schema/runtime_schema.go

package schema

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
)

// RuntimeSchema holds the in-memory representation of the OpsDB schema
// loaded from _schema_* tables. Provides fast lookups for entity types,
// fields, constraints, relationships, and JSON payload schemas. Thread-safe
// for concurrent reads; refreshed periodically by the schema refresh loop
// in main.go when the schema version changes.
//
// This is the runtime schema cache the API gate consults during the
// 10-step pipeline. Every validation, authorization, and execution step
// reads from this cache rather than querying _schema_* tables per request.
type RuntimeSchema struct {
	mu sync.RWMutex

	// Entity type metadata, keyed by table name
	entityTypes map[string]*EntityTypeMeta

	// Field metadata, keyed by entity type → field name
	fields map[string]map[string]*FieldMeta

	// All fields per entity in declaration order, keyed by entity type
	allFields map[string][]*FieldMeta

	// Relationships, keyed by source entity type
	relationships map[string][]RelationshipMeta

	// JSON payload schemas, keyed by entity type → field name → discriminator value
	jsonSchemas map[string]map[string]map[string]*JSONSchemaMeta

	// Current schema version serial from _schema_version
	currentVersionSerial int

	// When the schema was last loaded or refreshed
	lastRefreshed time.Time
}

// EntityTypeMeta holds metadata about one entity type loaded from
// _schema_entity_type.
type EntityTypeMeta struct {
	ID          int
	TableName   string
	Description string
	Category    string // change_managed, observation_only, append_only, computed
	Versioned   bool
	SoftDelete  bool
	AppendOnly  bool
	Introduced  int  // _schema_version_introduced_id
	Deprecated  *int // _schema_version_deprecated_id, nil if active
}

// FieldMeta holds metadata about one field loaded from _schema_field.
// This is the primary type consumed by gate steps for validation,
// authorization, and execution decisions.
type FieldMeta struct {
	ID         int
	EntityType string
	Name       string
	Type       string // int, float, varchar, text, boolean, datetime, date, json, enum, foreign_key
	Nullable   bool
	HasDefault bool
	Unique     bool

	// FK target entity type name. Empty for non-FK fields.
	References string

	// Numeric range constraints. Stored as *interface{} because the
	// underlying value may be int or float depending on the field type.
	// Gate steps use toInt/toFloat to extract the concrete value.
	MinValue *interface{}
	MaxValue *interface{}

	// String length constraints. Nil when not declared.
	MinLength *int
	MaxLength *int

	// Float precision constraint. Nil when not declared.
	PrecisionDecimalPlaces *int

	// Enum membership constraint. Empty for non-enum fields.
	EnumValues []string

	// JSON payload discriminator — the sibling field whose value selects
	// the JSON schema for validation. Empty for non-json fields.
	JsonTypeDiscriminator string

	// Composite uniqueness scope — field names that define the
	// uniqueness group this field participates in.
	MustBeUniqueWithin []string

	// Data classification level (public, internal, confidential,
	// restricted, regulated). Empty when no classification is set.
	// Read by step 2 (authorization layer 3) and step 5 (policy).
	AccessClassification string

	// Whether this field is a system-managed reserved field (id,
	// created_time, updated_time, parent_*_id, is_active).
	IsReserved bool

	// Whether this field is an underscore-prefixed governance field
	// (_requires_group, _access_classification, _retention_policy_id, etc.).
	IsGovernance bool

	// Whether this field has been deprecated via schema evolution.
	IsDeprecated          bool
	DeprecatedAlternative string // replacement field name, if any
	DeprecatedVersion     *int   // schema version that deprecated it

	Description string
	Introduced  int // _schema_version_introduced_id
}

// RelationshipMeta holds metadata about one FK relationship loaded from
// _schema_relationship.
type RelationshipMeta struct {
	SourceEntity   string
	SourceField    string
	TargetEntity   string
	Cardinality    string // one_to_one, one_to_many, many_to_many
	OnDeleteAction string // cascade, restrict, set_null
}

// JSONSchemaMeta holds the registered JSON payload schema for a
// discriminator value. Used by step 4 (bound validation) to validate
// JSON payloads against declared structure.
type JSONSchemaMeta struct {
	RequiredFields []string
	Fields         map[string]*JSONFieldMeta
}

// JSONFieldMeta holds constraints for one field within a JSON payload
// schema. Loaded from the schema/json_schemas/ YAML files via the
// _schema_field metadata.
type JSONFieldMeta struct {
	Type       string
	EnumValues []string
	MinValue   *float64
	MaxValue   *float64
	MinLength  *int
	MaxLength  *int
	MinCount   *int
	MaxCount   *int
	MaxEntries *int
}

// ---------------------------------------------------------------------------
// Loading
// ---------------------------------------------------------------------------

// LoadRuntimeSchema reads _schema_entity_type, _schema_field, and
// _schema_relationship from the database and builds the in-memory
// lookup maps. Called once at startup and again on schema refresh
// when the version serial changes.
func LoadRuntimeSchema(db *pg.DB) (*RuntimeSchema, error) {
	rs := &RuntimeSchema{
		entityTypes:   make(map[string]*EntityTypeMeta),
		fields:        make(map[string]map[string]*FieldMeta),
		allFields:     make(map[string][]*FieldMeta),
		relationships: make(map[string][]RelationshipMeta),
		jsonSchemas:   make(map[string]map[string]map[string]*JSONSchemaMeta),
	}

	// Read current schema version serial
	err := db.QueryRow(
		"SELECT version_serial FROM _schema_version WHERE is_current = true LIMIT 1",
	).Scan(&rs.currentVersionSerial)
	if err != nil {
		return nil, fmt.Errorf("failed to read current schema version: %w", err)
	}

	// Load entity types
	err = loadEntityTypes(db, rs)
	if err != nil {
		return nil, fmt.Errorf("failed to load entity types: %w", err)
	}

	// Load fields
	err = loadFields(db, rs)
	if err != nil {
		return nil, fmt.Errorf("failed to load fields: %w", err)
	}

	// Load relationships
	err = loadRelationships(db, rs)
	if err != nil {
		return nil, fmt.Errorf("failed to load relationships: %w", err)
	}

	rs.lastRefreshed = time.Now()

	return rs, nil
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

// Refresh checks if the schema version has changed since the last load
// and reloads the entire schema if so. Called periodically by the
// refreshSchemaLoop goroutine in main.go (default every 30 seconds).
//
// The reload creates a completely new RuntimeSchema and swaps all maps
// under the write lock. Readers holding pointers to *EntityTypeMeta or
// *FieldMeta from before the refresh still see the old data — which is
// correct, because each request gets its metadata at the start of the
// gate pipeline and uses it consistently throughout.
func (rs *RuntimeSchema) Refresh(db *pg.DB) error {
	var currentSerial int
	err := db.QueryRow(
		"SELECT version_serial FROM _schema_version WHERE is_current = true LIMIT 1",
	).Scan(&currentSerial)
	if err != nil {
		return fmt.Errorf("failed to check schema version: %w", err)
	}

	rs.mu.RLock()
	needsRefresh := currentSerial != rs.currentVersionSerial
	rs.mu.RUnlock()

	if !needsRefresh {
		return nil
	}

	// Build a completely new schema from the database
	newSchema, err := LoadRuntimeSchema(db)
	if err != nil {
		return fmt.Errorf("schema refresh failed: %w", err)
	}

	// Swap all maps under the write lock
	rs.mu.Lock()
	rs.entityTypes = newSchema.entityTypes
	rs.fields = newSchema.fields
	rs.allFields = newSchema.allFields
	rs.relationships = newSchema.relationships
	rs.jsonSchemas = newSchema.jsonSchemas
	rs.currentVersionSerial = newSchema.currentVersionSerial
	rs.lastRefreshed = time.Now()
	rs.mu.Unlock()

	return nil
}

// ---------------------------------------------------------------------------
// Lookup methods
// ---------------------------------------------------------------------------

// EntityCount returns the number of loaded entity types.
func (rs *RuntimeSchema) EntityCount() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return len(rs.entityTypes)
}

// GetEntityType looks up entity type metadata by table name.
// Returns nil, false if the entity type doesn't exist.
func (rs *RuntimeSchema) GetEntityType(name string) (*EntityTypeMeta, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	meta, ok := rs.entityTypes[name]
	return meta, ok
}

// GetField looks up field metadata by entity type and field name.
// Returns nil, false if the entity type or field doesn't exist.
func (rs *RuntimeSchema) GetField(entityType string, fieldName string) (*FieldMeta, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	entityFields, ok := rs.fields[entityType]
	if !ok {
		return nil, false
	}
	field, ok := entityFields[fieldName]
	return field, ok
}

// GetAllFields returns all fields for an entity type in declaration order.
// Returns nil if the entity type doesn't exist.
func (rs *RuntimeSchema) GetAllFields(entityType string) []*FieldMeta {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.allFields[entityType]
}

// GetRelationships returns all FK relationships where the given entity
// type is the source (i.e., the entity that holds the FK column).
func (rs *RuntimeSchema) GetRelationships(entityType string) []RelationshipMeta {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.relationships[entityType]
}

// GetAllEntityTypes returns all registered entity type names, sorted
// alphabetically.
func (rs *RuntimeSchema) GetAllEntityTypes() []string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	names := make([]string, 0, len(rs.entityTypes))
	for name := range rs.entityTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetJSONSchema returns the registered JSON payload schema for a
// specific entity type, field, and discriminator value. Used by
// step 4 (bound validation) to validate JSON payloads.
// Returns nil, false if no schema is registered for the combination.
func (rs *RuntimeSchema) GetJSONSchema(entityType string, fieldName string, discriminatorValue string) (*JSONSchemaMeta, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	byField, ok := rs.jsonSchemas[entityType]
	if !ok {
		return nil, false
	}
	byDisc, ok := byField[fieldName]
	if !ok {
		return nil, false
	}
	schema, ok := byDisc[discriminatorValue]
	return schema, ok
}

// IsVersioned checks if an entity type has versioning enabled.
func (rs *RuntimeSchema) IsVersioned(entityType string) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	meta, ok := rs.entityTypes[entityType]
	if !ok {
		return false
	}
	return meta.Versioned
}

// VersionSerial returns the current schema version serial.
func (rs *RuntimeSchema) VersionSerial() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.currentVersionSerial
}

// LastRefreshed returns the time of the last schema load or refresh.
func (rs *RuntimeSchema) LastRefreshed() time.Time {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.lastRefreshed
}

// ---------------------------------------------------------------------------
// Data loading from _schema_* tables
// ---------------------------------------------------------------------------

// loadEntityTypes reads all active entity types from _schema_entity_type.
func loadEntityTypes(db *pg.DB, rs *RuntimeSchema) error {
	rows, err := db.Query(
		"SELECT id, table_name, description, " +
			"COALESCE(category, 'change_managed'), " +
			"COALESCE(is_versioned, false), " +
			"COALESCE(is_soft_delete, false), " +
			"COALESCE(is_append_only, false), " +
			"_schema_version_introduced_id, " +
			"_schema_version_deprecated_id " +
			"FROM _schema_entity_type WHERE is_active = true",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		meta := &EntityTypeMeta{}
		var deprecated *int
		err := rows.Scan(
			&meta.ID, &meta.TableName, &meta.Description, &meta.Category,
			&meta.Versioned, &meta.SoftDelete, &meta.AppendOnly,
			&meta.Introduced, &deprecated,
		)
		if err != nil {
			return fmt.Errorf("entity type scan failed: %w", err)
		}
		meta.Deprecated = deprecated
		rs.entityTypes[meta.TableName] = meta
	}

	return rows.Err()
}

// loadFields reads all active fields from _schema_field, joined to
// _schema_entity_type for the entity table name. Parses constraint
// values, enum lists, and deprecation metadata.
func loadFields(db *pg.DB, rs *RuntimeSchema) error {
	rows, err := db.Query(
		"SELECT sf.id, et.table_name, sf.field_name, sf.field_type, " +
			"COALESCE(sf.is_nullable, false), " +
			"COALESCE(sf.is_primary_key, false), " +
			"COALESCE(sf.is_foreign_key, false), " +
			"sf.foreign_key_target_entity, " +
			"sf.default_value_text, " +
			"sf.constraint_data_json, " +
			"COALESCE(sf.is_reserved, false), " +
			"COALESCE(sf.is_governance, false), " +
			"sf.description, " +
			"sf._schema_version_introduced_id, " +
			"sf._schema_version_deprecated_id, " +
			"sf.deprecated_alternative " +
			"FROM _schema_field sf " +
			"JOIN _schema_entity_type et ON et.id = sf._schema_entity_type_id " +
			"WHERE sf.is_active = true",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		meta := &FieldMeta{}
		var fkTarget, defaultValue, constraintJSON, deprecatedAlt *string
		var isPK, isFK bool
		var deprecated *int

		err := rows.Scan(
			&meta.ID, &meta.EntityType, &meta.Name, &meta.Type,
			&meta.Nullable,
			&isPK, &isFK,
			&fkTarget,
			&defaultValue,
			&constraintJSON,
			&meta.IsReserved, &meta.IsGovernance,
			&meta.Description,
			&meta.Introduced, &deprecated,
			&deprecatedAlt,
		)
		if err != nil {
			return fmt.Errorf("field scan failed: %w", err)
		}

		// FK reference target
		if fkTarget != nil {
			meta.References = *fkTarget
		}

		// Default value
		if defaultValue != nil {
			meta.HasDefault = true
		}

		// Deprecation
		if deprecated != nil {
			meta.IsDeprecated = true
			meta.DeprecatedVersion = deprecated
		}
		if deprecatedAlt != nil {
			meta.DeprecatedAlternative = *deprecatedAlt
		}

		// Parse constraints from JSON
		if constraintJSON != nil && *constraintJSON != "" {
			parseFieldConstraints(meta, *constraintJSON)
		}

		// Primary key fields are always reserved
		if isPK {
			meta.IsReserved = true
		}

		// Add to field lookup maps
		if rs.fields[meta.EntityType] == nil {
			rs.fields[meta.EntityType] = make(map[string]*FieldMeta)
		}
		rs.fields[meta.EntityType][meta.Name] = meta
		rs.allFields[meta.EntityType] = append(rs.allFields[meta.EntityType], meta)
	}

	return rows.Err()
}

// parseFieldConstraints extracts constraint values from the
// constraint_data_json column into the FieldMeta struct fields.
func parseFieldConstraints(meta *FieldMeta, constraintJSON string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(constraintJSON), &data); err != nil {
		return
	}

	// Numeric range
	if v, ok := data["min_value"]; ok {
		meta.MinValue = &v
	}
	if v, ok := data["max_value"]; ok {
		meta.MaxValue = &v
	}

	// String length
	if v, ok := data["min_length"]; ok {
		if n, ok := jsonToInt(v); ok {
			meta.MinLength = &n
		}
	}
	if v, ok := data["max_length"]; ok {
		if n, ok := jsonToInt(v); ok {
			meta.MaxLength = &n
		}
	}

	// Precision
	if v, ok := data["precision_decimal_places"]; ok {
		if n, ok := jsonToInt(v); ok {
			meta.PrecisionDecimalPlaces = &n
		}
	}

	// Enum values
	if v, ok := data["enum_values"]; ok {
		meta.EnumValues = jsonToStringList(v)
	}

	// JSON discriminator
	if v, ok := data["json_type_discriminator"].(string); ok {
		meta.JsonTypeDiscriminator = v
	}

	// Composite uniqueness
	if v, ok := data["must_be_unique_within"]; ok {
		meta.MustBeUniqueWithin = jsonToStringList(v)
	}

	// Access classification
	if v, ok := data["access_classification"].(string); ok {
		meta.AccessClassification = v
	}

	// Unique flag from constraints (supplements the column-level unique)
	if v, ok := data["unique"].(bool); ok && v {
		meta.Unique = true
	}
}

// loadRelationships reads all relationships from _schema_relationship.
func loadRelationships(db *pg.DB, rs *RuntimeSchema) error {
	rows, err := db.Query(
		"SELECT " +
			"src.table_name, sf.field_name, tgt.table_name, " +
			"sr.cardinality, sr.on_delete_action " +
			"FROM _schema_relationship sr " +
			"JOIN _schema_entity_type src ON src.id = sr.source__schema_entity_type_id " +
			"JOIN _schema_field sf ON sf.id = sr.source__schema_field_id " +
			"JOIN _schema_entity_type tgt ON tgt.id = sr.target__schema_entity_type_id " +
			"WHERE sr.is_active = true",
	)
	if err != nil {
		if pg.IsUndefinedTable(err) {
			// _schema_relationship may not exist yet during early bootstrap
			return nil
		}
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rel RelationshipMeta
		err := rows.Scan(
			&rel.SourceEntity, &rel.SourceField, &rel.TargetEntity,
			&rel.Cardinality, &rel.OnDeleteAction,
		)
		if err != nil {
			return fmt.Errorf("relationship scan failed: %w", err)
		}

		rs.relationships[rel.SourceEntity] = append(
			rs.relationships[rel.SourceEntity], rel)
	}

	return rows.Err()
}

// ---------------------------------------------------------------------------
// JSON helpers for parsing constraint data
// ---------------------------------------------------------------------------

// jsonToInt extracts an int from a JSON-unmarshaled interface value.
// JSON numbers unmarshal as float64 in Go.
func jsonToInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case int64:
		return int(val), true
	default:
		return 0, false
	}
}

// jsonToFloat extracts a float64 from a JSON-unmarshaled interface value.
func jsonToFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

// jsonToStringList extracts a string slice from a JSON-unmarshaled value.
// Handles []interface{} (from json.Unmarshal), []string (direct), and
// comma-separated strings.
func jsonToStringList(v interface{}) []string {
	switch val := v.(type) {
	case []string:
		return val

	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result

	case string:
		return parseCommaSeparated(val)

	default:
		return nil
	}
}

// parseCommaSeparated splits a string on commas, trims whitespace,
// and filters empty entries. Also handles JSON array strings by
// trying json.Unmarshal first.
func parseCommaSeparated(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Try JSON array first
	if strings.HasPrefix(s, "[") {
		var result []string
		if err := json.Unmarshal([]byte(s), &result); err == nil {
			return result
		}
	}

	// Fall back to comma-separated
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
