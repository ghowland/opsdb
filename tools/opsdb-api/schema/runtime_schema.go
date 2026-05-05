// === opsdb-api/schema/runtime_schema.go ===
package schema

// RuntimeSchema holds the in-memory representation of the OpsDB schema
// loaded from _schema_* tables. Provides fast lookups for entity types,
// fields, constraints, and relationships. Refreshed when schema version changes.
type RuntimeSchema struct {
	// TODO: entityTypes map[string]*EntityTypeMeta keyed by table name
	// TODO: fields map[string]map[string]*FieldMeta keyed by entity → field name
	// TODO: relationships map[string][]RelationshipMeta keyed by entity name
	// TODO: currentVersionSerial int
	// TODO: lastRefreshed time.Time
}

// EntityTypeMeta holds metadata about one entity type.
type EntityTypeMeta struct {
	ID          int
	TableName   string
	Description string
	Introduced  int // schema version serial
	Deprecated  *int // nil if not deprecated
}

// FieldMeta holds metadata about one field.
type FieldMeta struct {
	ID              int
	EntityTypeID    int
	FieldName       string
	FieldType       string
	IsNullable      bool
	IsPrimaryKey    bool
	IsForeignKey    bool
	FKTargetEntity  string // empty if not FK
	DefaultValue    *string
	Constraints     map[string]interface{} // parsed from constraint_data_json
	Description     string
	Introduced      int
	Deprecated      *int
}

// RelationshipMeta holds metadata about one relationship.
type RelationshipMeta struct {
	SourceEntity   string
	SourceField    string
	TargetEntity   string
	Cardinality    string
	OnDeleteAction string
}

// LoadRuntimeSchema reads _schema_entity_type, _schema_field, _schema_relationship
// from the database and builds lookup maps.
func LoadRuntimeSchema(db interface{}) (*RuntimeSchema, error) {
	// TODO: SELECT * FROM _schema_version WHERE is_current = true
	//       get current version serial
	// TODO: SELECT * FROM _schema_entity_type
	//       build entityTypes map
	// TODO: SELECT * FROM _schema_field
	//       build fields map keyed by entity → field name
	//       parse constraint_data_json into structured constraints
	// TODO: SELECT * FROM _schema_relationship
	//       build relationships map
	// TODO: set currentVersionSerial and lastRefreshed
	return nil, nil
}

// Refresh checks if schema version has changed and reloads if so.
func (rs *RuntimeSchema) Refresh(db interface{}) error {
	// TODO: SELECT version_serial FROM _schema_version WHERE is_current = true
	// TODO: if different from rs.currentVersionSerial: call LoadRuntimeSchema
	// TODO: if same: no-op
	return nil
}

// GetEntityType looks up entity type metadata by table name.
func (rs *RuntimeSchema) GetEntityType(name string) (*EntityTypeMeta, bool) {
	// TODO: look up in entityTypes map
	return nil, false
}

// GetField looks up field metadata by entity type and field name.
func (rs *RuntimeSchema) GetField(entityType string, fieldName string) (*FieldMeta, bool) {
	// TODO: look up in fields[entityType][fieldName]
	return nil, false
}

// GetRelationships returns all relationships for an entity type.
func (rs *RuntimeSchema) GetRelationships(entityType string) []RelationshipMeta {
	// TODO: look up in relationships map
	return nil
}

// GetAllEntityTypes returns all registered entity type names.
func (rs *RuntimeSchema) GetAllEntityTypes() []string {
	// TODO: return sorted keys of entityTypes map
	return nil
}

// GetFieldsForEntity returns all fields for an entity type.
func (rs *RuntimeSchema) GetFieldsForEntity(entityType string) []*FieldMeta {
	// TODO: return all fields from fields[entityType]
	return nil
}

// IsVersioned checks if an entity type has a versioning sibling.
func (rs *RuntimeSchema) IsVersioned(entityType string) bool {
	// TODO: check if {entityType}_version exists in entityTypes map
	return false
}


