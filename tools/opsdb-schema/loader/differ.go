package loader

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/internal/vocabulary"
)

// SchemaState represents the current state of the database schema.
// Read from _schema_* tables (preferred) or information_schema (bootstrap).
type SchemaState struct {
	Entities      map[string]*EntityState
	Relationships []RelationshipState
	Version       int
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
	Type         string
	IsNullable   bool
	Default      *string
	MaxLength    *int
	NumericScale *int
}

// IndexState represents one index's current state.
type IndexState struct {
	Name   string
	Fields []string
	Unique bool
}

// ConstraintState represents one constraint's current state.
type ConstraintState struct {
	Name            string
	Type            string
	Fields          []string
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
	NewEntities        []string
	NewFields          []DiffItem
	ChangedConstraints []DiffItem
	NewIndexes         []DiffItem
	RemovedFields      []DiffItem
	RemovedEntities    []string
	TypeChanges        []DiffItem
	Other              []DiffItem
}

// DiffItem represents one specific difference between desired and current.
type DiffItem struct {
	Entity       string
	Field        string
	ChangeType   string
	DesiredValue interface{}
	CurrentValue interface{}
	Description  string
}

// Diff compares desired state (Schema from YAML) against current state
// (from database). Classifies each difference by type.
func Diff(desired *model.Schema, current *SchemaState) (*SchemaDiff, error) {
	diff := &SchemaDiff{}

	// Find new entities: in desired but not in current.
	for name := range desired.Entities {
		if _, exists := current.Entities[name]; !exists {
			diff.NewEntities = append(diff.NewEntities, name)
		}
	}

	// Find removed entities: in current but not in desired.
	for name := range current.Entities {
		if _, exists := desired.Entities[name]; !exists {
			diff.RemovedEntities = append(diff.RemovedEntities, name)
		}
	}

	// Compare fields for entities that exist in both.
	for name, desiredEntity := range desired.Entities {
		currentEntity, exists := current.Entities[name]
		if !exists {
			continue // new entity, already recorded above
		}

		// Check each desired field.
		for _, desiredField := range desiredEntity.Fields {
			currentField, fieldExists := currentEntity.Fields[desiredField.Name]

			if !fieldExists {
				// New field.
				diff.NewFields = append(diff.NewFields, DiffItem{
					Entity:       name,
					Field:        desiredField.Name,
					ChangeType:   "new_field",
					DesiredValue: desiredField.Type,
					Description:  fmt.Sprintf("add %s %s", desiredField.Name, desiredField.Type),
				})
				continue
			}

			// Field exists in both — compare types.
			desiredPGType := vocabulary.GetPostgresType(desiredField.Type, buildConstraintMap(desiredField))
			if !pgTypesMatch(desiredPGType, currentField.Type) {
				diff.TypeChanges = append(diff.TypeChanges, DiffItem{
					Entity:       name,
					Field:        desiredField.Name,
					ChangeType:   "type_change",
					DesiredValue: desiredPGType,
					CurrentValue: currentField.Type,
					Description:  fmt.Sprintf("type change %s -> %s", currentField.Type, desiredPGType),
				})
				continue
			}

			// Compare constraints.
			constraintDiffs := compareFieldConstraints(name, desiredField, currentField)
			diff.ChangedConstraints = append(diff.ChangedConstraints, constraintDiffs...)
		}

		// Fields in current but not in desired — removals.
		for fieldName := range currentEntity.Fields {
			found := false
			for _, df := range desiredEntity.Fields {
				if df.Name == fieldName {
					found = true
					break
				}
			}
			if !found {
				diff.RemovedFields = append(diff.RemovedFields, DiffItem{
					Entity:       name,
					Field:        fieldName,
					ChangeType:   "removed_field",
					CurrentValue: currentEntity.Fields[fieldName].Type,
					Description:  fmt.Sprintf("field %s exists in database but not in schema", fieldName),
				})
			}
		}

		// Compare indexes.
		currentIndexSet := make(map[string]bool)
		for _, idx := range currentEntity.Indexes {
			currentIndexSet[idx.Name] = true
		}
		for _, desiredIdx := range desiredEntity.Indexes {
			idxName := buildIndexName(name, desiredIdx)
			if !currentIndexSet[idxName] {
				diff.NewIndexes = append(diff.NewIndexes, DiffItem{
					Entity:       name,
					ChangeType:   "new_index",
					DesiredValue: desiredIdx,
					Description:  fmt.Sprintf("index %s on (%s)", idxName, strings.Join(desiredIdx.Fields, ", ")),
				})
			}
		}
	}

	return diff, nil
}

// ReadCurrentState reads the current database schema from _schema_* tables.
// Falls back to information_schema if _schema_* tables don't exist yet.
func ReadCurrentState(db *pg.DB) (*SchemaState, error) {
	// Check if _schema_entity_type table exists.
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = '_schema_entity_type'
		)`).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("checking for schema metadata tables: %w", err)
	}

	if exists {
		return readFromSchemaMetaTables(db)
	}
	return ReadFromInformationSchema(db)
}

// readFromSchemaMetaTables reads schema from _schema_entity_type, _schema_field, _schema_relationship.
func readFromSchemaMetaTables(db *pg.DB) (*SchemaState, error) {
	state := &SchemaState{
		Entities: make(map[string]*EntityState),
	}

	// Read current schema version.
	err := db.QueryRow(
		`SELECT COALESCE(
			(SELECT version_serial FROM _schema_version WHERE is_current = true LIMIT 1),
			0
		)`).Scan(&state.Version)
	if err != nil {
		state.Version = 0
	}

	// Read entity types.
	entityRows, err := db.Query(
		`SELECT id, table_name FROM _schema_entity_type WHERE _schema_version_deprecated_id IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("reading _schema_entity_type: %w", err)
	}
	defer entityRows.Close()

	entityIDMap := make(map[int]string) // id -> table_name
	for entityRows.Next() {
		var id int
		var tableName string
		if err := entityRows.Scan(&id, &tableName); err != nil {
			return nil, fmt.Errorf("scanning entity type: %w", err)
		}
		entityIDMap[id] = tableName
		state.Entities[tableName] = &EntityState{
			Name:   tableName,
			Fields: make(map[string]*FieldState),
		}
	}
	if err := entityRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating entity types: %w", err)
	}

	// Read fields.
	fieldRows, err := db.Query(
		`SELECT schema_entity_type_id, field_name, field_type, is_nullable, default_value, constraint_data_json
		 FROM _schema_field WHERE _schema_version_deprecated_id IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("reading _schema_field: %w", err)
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var entityTypeID int
		var fieldName, fieldType string
		var isNullable bool
		var defaultValue *string
		var constraintJSON *string

		if err := fieldRows.Scan(&entityTypeID, &fieldName, &fieldType, &isNullable, &defaultValue, &constraintJSON); err != nil {
			return nil, fmt.Errorf("scanning field: %w", err)
		}

		tableName, ok := entityIDMap[entityTypeID]
		if !ok {
			continue
		}

		entityState := state.Entities[tableName]
		fs := &FieldState{
			Name:       fieldName,
			Type:       fieldType,
			IsNullable: isNullable,
			Default:    defaultValue,
		}

		// Parse constraints from JSON.
		if constraintJSON != nil && *constraintJSON != "" {
			var constraints map[string]interface{}
			if err := json.Unmarshal([]byte(*constraintJSON), &constraints); err == nil {
				if ml, ok := constraints["max_length"]; ok {
					if mlf, ok := ml.(float64); ok {
						mli := int(mlf)
						fs.MaxLength = &mli
					}
				}
				if ns, ok := constraints["precision_decimal_places"]; ok {
					if nsf, ok := ns.(float64); ok {
						nsi := int(nsf)
						fs.NumericScale = &nsi
					}
				}
			}
		}

		entityState.Fields[fieldName] = fs
	}
	if err := fieldRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating fields: %w", err)
	}

	// Read relationships.
	relRows, err := db.Query(
		`SELECT source_entity_name, source_field_name, target_entity_name
		 FROM _schema_relationship`)
	if err != nil {
		return nil, fmt.Errorf("reading _schema_relationship: %w", err)
	}
	defer relRows.Close()

	for relRows.Next() {
		var rs RelationshipState
		if err := relRows.Scan(&rs.SourceTable, &rs.SourceField, &rs.TargetTable); err != nil {
			return nil, fmt.Errorf("scanning relationship: %w", err)
		}
		rs.TargetField = "id"
		state.Relationships = append(state.Relationships, rs)
	}
	if err := relRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating relationships: %w", err)
	}

	return state, nil
}

// ReadFromInformationSchema reads current schema from Postgres information_schema.
// Used during bootstrap when _schema_* tables don't exist yet.
func ReadFromInformationSchema(db *pg.DB) (*SchemaState, error) {
	state := &SchemaState{
		Entities: make(map[string]*EntityState),
		Version:  0,
	}

	// Read all user tables in public schema.
	tableRows, err := db.Query(
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		 ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("reading tables from information_schema: %w", err)
	}
	defer tableRows.Close()

	var tableNames []string
	for tableRows.Next() {
		var name string
		if err := tableRows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning table name: %w", err)
		}
		tableNames = append(tableNames, name)
		state.Entities[name] = &EntityState{
			Name:   name,
			Fields: make(map[string]*FieldState),
		}
	}
	if err := tableRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tables: %w", err)
	}

	// Read columns for each table.
	for _, tableName := range tableNames {
		colRows, err := db.Query(
			`SELECT column_name, data_type, is_nullable, column_default,
			        character_maximum_length, numeric_scale
			 FROM information_schema.columns
			 WHERE table_schema = 'public' AND table_name = $1
			 ORDER BY ordinal_position`, tableName)
		if err != nil {
			return nil, fmt.Errorf("reading columns for %s: %w", tableName, err)
		}

		for colRows.Next() {
			var colName, dataType, isNullableStr string
			var colDefault *string
			var maxLength, numScale *int

			if err := colRows.Scan(&colName, &dataType, &isNullableStr, &colDefault, &maxLength, &numScale); err != nil {
				colRows.Close()
				return nil, fmt.Errorf("scanning column %s.%s: %w", tableName, colName, err)
			}

			state.Entities[tableName].Fields[colName] = &FieldState{
				Name:         colName,
				Type:         dataType,
				IsNullable:   isNullableStr == "YES",
				Default:      colDefault,
				MaxLength:    maxLength,
				NumericScale: numScale,
			}
		}
		colRows.Close()
		if err := colRows.Err(); err != nil {
			return nil, fmt.Errorf("iterating columns for %s: %w", tableName, err)
		}
	}

	// Read indexes.
	for _, tableName := range tableNames {
		idxRows, err := db.Query(
			`SELECT indexname, indexdef FROM pg_indexes
			 WHERE schemaname = 'public' AND tablename = $1`, tableName)
		if err != nil {
			return nil, fmt.Errorf("reading indexes for %s: %w", tableName, err)
		}

		for idxRows.Next() {
			var idxName, idxDef string
			if err := idxRows.Scan(&idxName, &idxDef); err != nil {
				idxRows.Close()
				return nil, fmt.Errorf("scanning index for %s: %w", tableName, err)
			}

			isUnique := strings.Contains(strings.ToUpper(idxDef), "UNIQUE")
			fields := parseIndexFields(idxDef)

			state.Entities[tableName].Indexes = append(state.Entities[tableName].Indexes, IndexState{
				Name:   idxName,
				Fields: fields,
				Unique: isUnique,
			})
		}
		idxRows.Close()
	}

	// Read constraints.
	for _, tableName := range tableNames {
		conRows, err := db.Query(
			`SELECT tc.constraint_name, tc.constraint_type,
			        kcu.column_name,
			        ccu.table_name AS references_table,
			        ccu.column_name AS references_column
			 FROM information_schema.table_constraints tc
			 JOIN information_schema.key_column_usage kcu
			   ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
			 LEFT JOIN information_schema.constraint_column_usage ccu
			   ON tc.constraint_name = ccu.constraint_name AND tc.table_schema = ccu.table_schema
			 WHERE tc.table_schema = 'public' AND tc.table_name = $1
			 ORDER BY tc.constraint_name, kcu.ordinal_position`, tableName)
		if err != nil {
			return nil, fmt.Errorf("reading constraints for %s: %w", tableName, err)
		}

		for conRows.Next() {
			var conName, conType, colName string
			var refTable, refColumn *string

			if err := conRows.Scan(&conName, &conType, &colName, &refTable, &refColumn); err != nil {
				conRows.Close()
				return nil, fmt.Errorf("scanning constraint for %s: %w", tableName, err)
			}

			cs := ConstraintState{
				Name:            conName,
				Type:            strings.ToLower(strings.ReplaceAll(conType, " ", "_")),
				Fields:          []string{colName},
				ReferencesTable: refTable,
				ReferencesField: refColumn,
			}
			state.Entities[tableName].Constraints = append(state.Entities[tableName].Constraints, cs)
		}
		conRows.Close()
	}

	// Build relationships from FK constraints.
	for tableName, entity := range state.Entities {
		for _, con := range entity.Constraints {
			if con.Type == "foreign_key" && con.ReferencesTable != nil {
				state.Relationships = append(state.Relationships, RelationshipState{
					SourceTable: tableName,
					SourceField: con.Fields[0],
					TargetTable: *con.ReferencesTable,
					TargetField: "id",
				})
			}
		}
	}

	return state, nil
}

// --- helpers ---

// pgTypesMatch compares a desired Postgres type with the type reported by
// information_schema or _schema_field. Handles common aliases.
func pgTypesMatch(desired string, current string) bool {
	desired = strings.ToLower(strings.TrimSpace(desired))
	current = strings.ToLower(strings.TrimSpace(current))

	if desired == current {
		return true
	}

	// Normalize common Postgres type aliases.
	aliases := map[string]string{
		"integer":                     "integer",
		"int":                         "integer",
		"int4":                        "integer",
		"double precision":            "double precision",
		"float8":                      "double precision",
		"boolean":                     "boolean",
		"bool":                        "boolean",
		"timestamp without time zone": "timestamp without time zone",
		"timestamp":                   "timestamp without time zone",
		"text":                        "text",
		"jsonb":                       "jsonb",
		"date":                        "date",
	}

	normalizedDesired := desired
	if norm, ok := aliases[desired]; ok {
		normalizedDesired = norm
	}
	normalizedCurrent := current
	if norm, ok := aliases[current]; ok {
		normalizedCurrent = norm
	}

	if normalizedDesired == normalizedCurrent {
		return true
	}

	// VARCHAR comparison: "character varying" vs "varchar(N)".
	if strings.HasPrefix(desired, "varchar(") && strings.HasPrefix(current, "character varying") {
		return true // length checked separately in constraint comparison
	}
	if strings.HasPrefix(current, "varchar(") && strings.HasPrefix(desired, "character varying") {
		return true
	}
	if strings.HasPrefix(desired, "varchar(") && current == "character varying" {
		return true
	}
	if desired == "character varying" && strings.HasPrefix(current, "varchar(") {
		return true
	}

	return false
}

// compareFieldConstraints compares constraints between a desired field and
// the current field state. Returns diff items for changed constraints.
func compareFieldConstraints(entityName string, desired model.Field, current *FieldState) []DiffItem {
	var diffs []DiffItem

	// Compare max_length for varchar/text. MaxLength is plain int, 0 = unset.
	if desired.MaxLength > 0 && current.MaxLength != nil {
		if desired.MaxLength != *current.MaxLength {
			diffs = append(diffs, DiffItem{
				Entity:       entityName,
				Field:        desired.Name,
				ChangeType:   "changed_constraint",
				DesiredValue: desired.MaxLength,
				CurrentValue: *current.MaxLength,
				Description:  fmt.Sprintf("max_length: %d -> %d", *current.MaxLength, desired.MaxLength),
			})
		}
	}

	// Compare nullable.
	if desired.Nullable != current.IsNullable {
		diffs = append(diffs, DiffItem{
			Entity:       entityName,
			Field:        desired.Name,
			ChangeType:   "changed_constraint",
			DesiredValue: desired.Nullable,
			CurrentValue: current.IsNullable,
			Description:  fmt.Sprintf("nullable: %v -> %v", current.IsNullable, desired.Nullable),
		})
	}

	return diffs
}

// buildIndexName constructs the expected index name from entity and index definition.
// model.Index has no Name field — an optional user-provided name is stored
// in Description. If Description is empty, the name is computed from fields.
func buildIndexName(entityName string, idx model.Index) string {
	if idx.Description != "" {
		return idx.Description
	}
	prefix := "idx"
	if idx.Unique {
		prefix = "uq"
	}
	return fmt.Sprintf("%s_%s_%s", prefix, entityName, strings.Join(idx.Fields, "_"))
}

// parseIndexFields extracts column names from a CREATE INDEX definition string.
// Example: "CREATE INDEX idx_foo ON bar (col1, col2)" -> ["col1", "col2"]
func parseIndexFields(indexDef string) []string {
	parenStart := strings.Index(indexDef, "(")
	parenEnd := strings.LastIndex(indexDef, ")")
	if parenStart == -1 || parenEnd == -1 || parenEnd <= parenStart {
		return nil
	}
	inner := indexDef[parenStart+1 : parenEnd]
	parts := strings.Split(inner, ",")
	var fields []string
	for _, p := range parts {
		field := strings.TrimSpace(p)
		// Remove any ordering suffix (ASC, DESC).
		field = strings.TrimSuffix(field, " ASC")
		field = strings.TrimSuffix(field, " DESC")
		field = strings.TrimSuffix(field, " asc")
		field = strings.TrimSuffix(field, " desc")
		if field != "" {
			fields = append(fields, field)
		}
	}
	return fields
}
