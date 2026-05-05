//# tools/opsdb-schema/loader/differ.go

go
package loader

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/pg"
)

// SchemaState represents the current state of the database schema.
// Read from _schema_* tables (preferred) or information_schema (bootstrap).
type SchemaState struct {
	Entities      map[string]*EntityState
	Relationships []RelationshipState
	Version       int // current _schema_version serial, 0 if no schema metadata
}

// EntityState represents one table's current schema in the database.
type EntityState struct {
	Name        string
	Fields      map[string]*FieldState
	Indexes     []IndexState
	Constraints []ConstraintState
}

// FieldState represents one column's current state.
type FieldState struct {
	Name         string
	Type         string // postgres type as reported by information_schema
	IsNullable   bool
	Default      *string
	MaxLength    *int
	NumericScale *int
}

// IndexState represents one index's current state.
type IndexState struct {
	Name    string
	Fields  []string
	Unique  bool
}

// ConstraintState represents one constraint's current state.
type ConstraintState struct {
	Name           string
	Type           string // primary_key, foreign_key, unique, check
	Fields         []string
	ReferencesTable *string
	ReferencesField *string
	CheckExpression *string
}

// RelationshipState represents one FK relationship in the current database.
type RelationshipState struct {
	SourceTable string
	SourceField string
	TargetTable string
	TargetField string
}

// SchemaDiff holds the differences between desired state and current state.
type SchemaDiff struct {
	NewEntities       []string    // entities in desired but not in current
	NewFields         []DiffItem  // fields in desired but not in current
	ChangedConstraints []DiffItem // constraints that differ between desired and current
	NewIndexes        []DiffItem  // indexes in desired but not in current
	RemovedFields     []DiffItem  // fields in current but not in desired (forbidden)
	RemovedEntities   []string    // entities in current but not in desired (forbidden)
	TypeChanges       []DiffItem  // fields whose type changed (forbidden)
	Other             []DiffItem  // other changes
}

// DiffItem represents one specific difference between desired and current.
type DiffItem struct {
	Entity       string
	Field        string
	ChangeType   string // new_field, changed_constraint, removed_field, type_change, etc.
	DesiredValue interface{}
	CurrentValue interface{}
	Description  string
}

// Diff compares desired state (Schema from YAML) against current state
// (from database). Classifies each difference by type.
func Diff(desired *model.Schema, current *SchemaState) (*SchemaDiff, error) {
	// TODO: initialize SchemaDiff
	diff := &SchemaDiff{}

	// TODO: find new entities: in desired.Entities but not in current.Entities
	//   for each: add to diff.NewEntities

	// TODO: for each entity in both desired and current:
	//   compare fields:
	//     fields in desired but not current -> diff.NewFields
	//     fields in current but not desired -> diff.RemovedFields (forbidden)
	//     fields in both: compare types
	//       if type changed -> diff.TypeChanges (forbidden)
	//       if constraints changed (wider range, new enum values, etc.) -> diff.ChangedConstraints

	// TODO: compare indexes:
	//   indexes in desired but not current -> diff.NewIndexes
	//   indexes in current but not desired -> noted but not necessarily forbidden

	// TODO: entities in current but not desired -> diff.RemovedEntities (forbidden)

	_ = desired
	_ = current
	return diff, fmt.Errorf("not implemented")
}

// ReadCurrentState reads the current database schema from _schema_* tables.
// Falls back to information_schema if _schema_* tables don't exist yet (bootstrap).
func ReadCurrentState(db *pg.DB) (*SchemaState, error) {
	// TODO: check if _schema_entity_type table exists:
	//   SELECT 1 FROM information_schema.tables WHERE table_name = '_schema_entity_type'
	//   if exists: call readFromSchemaMetaTables(db)
	//   if not: call ReadFromInformationSchema(db)
	return nil, fmt.Errorf("not implemented")
}

// readFromSchemaMetaTables reads schema from _schema_entity_type, _schema_field, _schema_relationship.
func readFromSchemaMetaTables(db *pg.DB) (*SchemaState, error) {
	// TODO: read _schema_version WHERE is_current = true for version serial
	// TODO: read all _schema_entity_type rows
	//   for each: create EntityState with name
	// TODO: read all _schema_field rows
	//   for each: add FieldState to parent EntityState
	//   parse constraint_data_json for constraint details
	// TODO: read all _schema_relationship rows
	//   for each: add RelationshipState
	// TODO: return SchemaState
	return nil, fmt.Errorf("not implemented")
}

// ReadFromInformationSchema reads current schema from Postgres information_schema.
// Used during bootstrap when _schema_* tables don't exist yet.
func ReadFromInformationSchema(db *pg.DB) (*SchemaState, error) {
	// TODO: query information_schema.tables for all user tables (exclude pg_ and information_schema)
	// TODO: for each table:
	//   query information_schema.columns for field details
	//     column_name, data_type, is_nullable, column_default,
	//     character_maximum_length, numeric_scale
	//   query information_schema.table_constraints + key_column_usage for constraints
	//   query pg_indexes for index details
	//   build EntityState with FieldState entries
	// TODO: query information_schema.referential_constraints for FK relationships
	// TODO: return SchemaState with Version = 0
	return nil, fmt.Errorf("not implemented")
}


