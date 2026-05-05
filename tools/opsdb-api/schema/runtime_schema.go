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
// fields, constraints, and relationships. Thread-safe for concurrent reads
// with periodic refresh.
type RuntimeSchema struct {
	mu                   sync.RWMutex
	entityTypes          map[string]*EntityTypeMeta
	fields               map[string]map[string]*FieldMeta // entity → field name → meta
	allFields            map[string][]*FieldMeta          // entity → ordered field list
	relationships        map[string][]RelationshipMeta    // entity → relationships
	jsonSchemas          map[string]map[string]map[string]*JSONSchemaMeta // entity → field → disc_value → schema
	currentVersionSerial int
	lastRefreshed        time.Time
}

// EntityTypeMeta holds metadata about one entity type.
type EntityTypeMeta struct {
	ID          int
	TableName   string
	Description string
	Category    string
	Versioned   bool
	SoftDelete  bool
	AppendOnly  bool
	Introduced  int
	Deprecated  *int
}

// FieldMeta holds metadata about one field.
type FieldMeta struct {
	ID                    int
	EntityType            string
	Name                  string
	Type                  string
	Nullable              bool
	HasDefault            bool
	DefaultValue          *string
	Unique                bool
	References            string // target entity for FK, empty otherwise
	MinValue              *interface{}
	MaxValue              *interface{}
	MinLength             *int
	MaxLength             *int
	PrecisionDecimalPlaces *int
	EnumValues            []string
	JsonTypeDiscriminator string
	MustBeUniqueWithin    []string
	AccessClassification  string
	IsReserved            bool
	IsGovernance          bool
	IsDeprecated          bool
	DeprecatedAlternative string
	Description           string
	Introduced            int
	DeprecatedVersion     *int
}

// RelationshipMeta holds metadata about one relationship.
type RelationshipMeta struct {
	SourceEntity   string
	SourceField    string
	TargetEntity   string
	Cardinality    string
	OnDeleteAction string
}

// JSONSchemaMeta holds the registered JSON payload schema for a
// discriminator value. Used by bound validation to check JSON payloads.
type JSONSchemaMeta struct {
	RequiredFields []string
	Fields         map[string]*JSONFieldMeta
}

// JSONFieldMeta holds constraints for one field within a JSON payload schema.
type JSONFieldMeta struct {
	Type       string
	MaxLength  *int
	MinLength  *int
	EnumValues []string
	MinValue   *float64
	MaxValue   *float64
}

// LoadRuntimeSchema reads _schema_entity_type, _schema_field, and
// _schema_relationship from the database and builds lookup maps.
func LoadRuntimeSchema(db *pg.DB) (*RuntimeSchema, error) {
	rs := &RuntimeSchema{
		entityTypes:   make(map[string]*EntityTypeMeta),
		fields:        make(map[string]map[string]*FieldMeta),
		allFields:     make(map[string][]*FieldMeta),
		relationships: make(map[string][]RelationshipMeta),
		jsonSchemas:   make(map[string]map[string]map[string]*JSONSchemaMeta),
	}

	// read current schema version
	err := db.QueryRow(
		"SELECT version_serial FROM _schema_version WHERE is_current = true LIMIT 1",
	).Scan(&rs.currentVersionSerial)
	if err != nil {
		return nil, fmt.Errorf("failed to read current schema version: %w", err)
	}

	// load entity types
	err = loadEntityTypes(db, rs)
	if err != nil {
		return nil, fmt.Errorf("failed to load entity types: %w", err)
	}

	// load fields
	err = loadFields(db, rs)
	if err != nil {
		return nil, fmt.Errorf("failed to load fields: %w", err)
	}

	// load relationships
	err = loadRelationships(db, rs)
	if err != nil {
		return nil, fmt.Errorf("failed to load relationships: %w", err)
	}

	rs.lastRefreshed = time.Now()

	return rs, nil
}

// Refresh checks if the schema version has changed and reloads if so.
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

	newSchema, err := LoadRuntimeSchema(db)
	if err != nil {
		return fmt.Errorf("schema refresh failed: %w", err)
	}

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

// EntityCount returns the number of loaded entity types.
func (rs *RuntimeSchema) EntityCount() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return len(rs.entityTypes)
}

// GetEntityType looks up entity type metadata by table name.
func (rs *RuntimeSchema) GetEntityType(name string) (*EntityTypeMeta, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	meta, ok := rs.entityTypes[name]
	return meta, ok
}

// GetField looks up field metadata by entity type and field name.
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
func (rs *RuntimeSchema) GetAllFields(entityType string) []*FieldMeta {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.allFields[entityType]
}

// GetRelationships returns all relationships for an entity type.
func (rs *RuntimeSchema) GetRelationships(entityType string) []RelationshipMeta {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.relationships[entityType]
}

// GetAllEntityTypes returns all registered entity type names, sorted.
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

// IsVersioned checks if an entity type has a versioning sibling table.
func (rs *RuntimeSchema) IsVersioned(entityType string) bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	_, ok := rs.entityTypes[entityType+"_version"]
	return ok
}

// GetJSONSchema returns the registered JSON payload schema for a
// specific entity type, field, and discriminator value.
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

// --- data loading functions ---

func loadEntityTypes(db *pg.DB, rs *RuntimeSchema) error {
	rows, err := db.Query(
		"SELECT id, table_name, description, category, " +
			"is_versioned, is_soft_delete, is_append_only, " +
			"_schema_version_introduced_id, _schema_version_deprecated_id " +
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

func loadFields(db *pg.DB, rs *RuntimeSchema) error {
	rows, err := db.Query(
		"SELECT sf.id, et.table_name, sf.field_name, sf.field_type, " +
			"sf.is_nullable, sf.has_default, sf.default_value, sf.is_unique, " +
			"sf.references_entity, sf.min_value, sf.max_value, " +
			"sf.min_length, sf.max_length, sf.precision_decimal_places, " +
			"sf.enum_values, sf.json_type_discriminator, " +
			"sf.must_be_unique_within, sf._access_classification, " +
			"sf.is_reserved, sf.is_governance, sf.description, " +
			"sf._schema_version_introduced_id, sf._schema_version_deprecated_id, " +
			"sf.deprecated_alternative " +
			"FROM _schema_field sf " +
			"JOIN _schema_entity_type et ON et.id = sf.entity_type_id " +
			"WHERE sf.is_active = true",
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		meta := &FieldMeta{}
		var referencesEntity, jsonDiscriminator, accessClassification *string
		var enumValuesStr, uniqueWithinStr, deprecatedAlt *string
		var minVal, maxVal *string
		var minLen, maxLen, precision *int
		var deprecated *int

		err := rows.Scan(
			&meta.ID, &meta.EntityType, &meta.Name, &meta.Type,
			&meta.Nullable, &meta.HasDefault, &meta.DefaultValue, &meta.Unique,
			&referencesEntity, &minVal, &maxVal,
			&minLen, &maxLen, &precision,
			&enumValuesStr, &jsonDiscriminator,
			&uniqueWithinStr, &accessClassification,
			&meta.IsReserved, &meta.IsGovernance, &meta.Description,
			&meta.Introduced, &deprecated,
			&deprecatedAlt,
		)
		if err != nil {
			return fmt.Errorf("field scan failed: %w", err)
		}

		if referencesEntity != nil {
			meta.References = *referencesEntity
		}
		if jsonDiscriminator != nil {
			meta.JsonTypeDiscriminator = *jsonDiscriminator
		}
		if accessClassification != nil {
			meta.AccessClassification = *accessClassification
		}
		if minLen != nil {
			meta.MinLength = minLen
		}
		if maxLen != nil {
			meta.MaxLength = maxLen
		}
		if precision != nil {
			meta.PrecisionDecimalPlaces = precision
		}
		if deprecated != nil {
			meta.IsDeprecated = true
			meta.DeprecatedVersion = deprecated
		}
		if deprecatedAlt != nil {
			meta.DeprecatedAlternative = *deprecatedAlt
		}

		// parse min/max values as interface pointers
		if minVal != nil {
			var v interface{} = *minVal
			meta.MinValue = &v
		}
		if maxVal != nil {
			var v interface{} = *maxVal
			meta.MaxValue = &v
		}

		// parse enum values from comma-separated or JSON array
		if enumValuesStr != nil && *enumValuesStr != "" {
			meta.EnumValues = parseStringList(*enumValuesStr)
		}

		// parse must_be_unique_within
		if uniqueWithinStr != nil && *uniqueWithinStr != "" {
			meta.MustBeUniqueWithin = parseStringList(*uniqueWithinStr)
		}

		// add to field maps
		if rs.fields[meta.EntityType] == nil {
			rs.fields[meta.EntityType] = make(map[string]*FieldMeta)
		}
		rs.fields[meta.EntityType][meta.Name] = meta
		rs.allFields[meta.EntityType] = append(rs.allFields[meta.EntityType], meta)
	}

	return rows.Err()
}

func loadRelationships(db *pg.DB, rs *RuntimeSchema) error {
	rows, err := db.Query(
		"SELECT source_entity, source_field, target_entity, " +
			"cardinality, on_delete_action " +
			"FROM _schema_relationship",
	)
	if err != nil {
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

// parseStringList parses a string that may be a JSON array or
// comma-separated values into a string slice.
func parseStringList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// try JSON array first
	if strings.HasPrefix(s, "[") {
		var result []string
		if err := json.Unmarshal([]byte(s), &result); err == nil {
			return result
		}
	}

	// fall back to comma-separated
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
