package loader

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/vocabulary"
)

// DDLStatement represents one SQL DDL statement to execute.
type DDLStatement struct {
	SQL         string
	Entity      string
	Description string
	Phase       int // 1=create tables/columns, 2=constraints, 3=indexes, 4=revoke
}

// GenerateDDL generates ordered Postgres DDL statements from allowed changes.
// Produces CREATE TABLE, ALTER TABLE ADD COLUMN, FK/UNIQUE/CHECK constraints,
// CREATE INDEX, and REVOKE statements for append-only tables.
func GenerateDDL(schema *model.Schema, changes []AllowedChange) ([]DDLStatement, error) {
	var statements []DDLStatement

	// Build sets for quick lookup.
	newEntitySet := make(map[string]bool)
	newFieldsByEntity := make(map[string][]string) // entity -> field names
	for _, c := range changes {
		switch c.ChangeType {
		case "new_entity":
			newEntitySet[c.Entity] = true
		case "new_field":
			newFieldsByEntity[c.Entity] = append(newFieldsByEntity[c.Entity], c.Field)
		}
	}

	// Phase 1: CREATE TABLE for new entities, in load order.
	for _, entityName := range schema.LoadOrder {
		if !newEntitySet[entityName] {
			continue
		}
		entity, ok := schema.Entities[entityName]
		if !ok {
			continue
		}

		// CREATE TABLE with all columns.
		stmt := generateCreateTable(entity)
		statements = append(statements, stmt)

		// Phase 2: FK constraints (separate for dependency ordering).
		for i := range entity.Fields {
			field := &entity.Fields[i]
			if field.Type == "foreign_key" && field.References != "" {
				statements = append(statements, generateFKConstraint(entity.Name, field))
			}
			if field.Unique {
				statements = append(statements, generateUniqueConstraint(entity.Name, field))
			}
			if len(field.MustBeUniqueWithin) > 0 {
				allFields := append([]string{field.Name}, field.MustBeUniqueWithin...)
				statements = append(statements, generateCompositeUnique(entity.Name, allFields))
			}
			// CHECK constraints for numeric ranges (not inlined in CREATE TABLE for existing fields).
			if (field.Type == "int" || field.Type == "float") && (field.MinValue != nil || field.MaxValue != nil) {
				stmt := generateCheckConstraint(entity.Name, field)
				if stmt.SQL != "" {
					statements = append(statements, stmt)
				}
			}
		}

		// Phase 3: Indexes.
		for i := range entity.Indexes {
			idx := &entity.Indexes[i]
			statements = append(statements, generateIndex(entity.Name, idx))
		}

		// Phase 4: REVOKE for append-only tables.
		if entity.AppendOnly {
			revokeRoles := []string{"opsdb_app_role", "opsdb_runner_role", "opsdb_readonly_role"}
			revokeStmts := generateRevokeAppendOnly(entity.Name, revokeRoles)
			statements = append(statements, revokeStmts...)
		}
	}

	// Phase 1: ALTER TABLE ADD COLUMN for new fields on existing entities.
	for entityName, fieldNames := range newFieldsByEntity {
		if newEntitySet[entityName] {
			continue // already handled in CREATE TABLE above
		}
		entity, ok := schema.Entities[entityName]
		if !ok {
			continue
		}

		fieldNameSet := make(map[string]bool)
		for _, fn := range fieldNames {
			fieldNameSet[fn] = true
		}

		for i := range entity.Fields {
			field := &entity.Fields[i]
			if !fieldNameSet[field.Name] {
				continue
			}
			statements = append(statements, generateAlterAddColumn(entityName, field))

			if field.Type == "foreign_key" && field.References != "" {
				statements = append(statements, generateFKConstraint(entityName, field))
			}
			if field.Unique {
				statements = append(statements, generateUniqueConstraint(entityName, field))
			}
		}
	}

	// Phase 3: New indexes on existing entities (from allowed changes).
	for _, c := range changes {
		if c.ChangeType == "new_index" && !newEntitySet[c.Entity] {
			entity, ok := schema.Entities[c.Entity]
			if !ok {
				continue
			}
			for i := range entity.Indexes {
				idx := &entity.Indexes[i]
				statements = append(statements, generateIndex(c.Entity, idx))
			}
		}
	}

	// Order by phase, then by load order within phase.
	statements = OrderByDependency(statements, schema.LoadOrder)

	return statements, nil
}

// generateCreateTable produces a CREATE TABLE statement with all columns,
// inline CHECK constraints for enums, NOT NULL, DEFAULT, and IDENTITY for id.
// FK constraints are generated separately.
func generateCreateTable(entity *model.Entity) DDLStatement {
	var columns []string

	for _, field := range entity.Fields {
		constraints := buildConstraintMap(field)
		colType := vocabulary.GetPostgresType(field.Type, constraints)

		line := fmt.Sprintf("  %s %s", field.Name, colType)

		// Primary key with identity for id field.
		if field.Name == "id" {
			line = fmt.Sprintf("  %s %s GENERATED ALWAYS AS IDENTITY PRIMARY KEY", field.Name, colType)
			columns = append(columns, line)
			continue
		}

		if !field.Nullable {
			line += " NOT NULL"
		}

		if field.Default != nil {
			line += fmt.Sprintf(" DEFAULT %s", formatDefault(field.Type, field.Default))
		}

		// Inline CHECK for enum values.
		if field.Type == "enum" && len(field.EnumValues) > 0 {
			quoted := make([]string, len(field.EnumValues))
			for i, v := range field.EnumValues {
				quoted[i] = fmt.Sprintf("'%s'", v)
			}
			line += fmt.Sprintf(" CHECK (%s IN (%s))", field.Name, strings.Join(quoted, ", "))
		}

		columns = append(columns, line)
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n%s\n);", entity.Name, strings.Join(columns, ",\n"))

	return DDLStatement{
		SQL:         sql,
		Entity:      entity.Name,
		Description: fmt.Sprintf("create table %s", entity.Name),
		Phase:       1,
	}
}

// generateAlterAddColumn produces ALTER TABLE ADD COLUMN for a new field
// on an existing entity.
func generateAlterAddColumn(entityName string, field *model.Field) DDLStatement {
	constraints := buildConstraintMap(*field)
	colType := vocabulary.GetPostgresType(field.Type, constraints)

	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", entityName, field.Name, colType)

	if !field.Nullable {
		sql += " NOT NULL"
	}
	if field.Default != nil {
		sql += fmt.Sprintf(" DEFAULT %s", formatDefault(field.Type, field.Default))
	}
	if field.Type == "enum" && len(field.EnumValues) > 0 {
		quoted := make([]string, len(field.EnumValues))
		for i, v := range field.EnumValues {
			quoted[i] = fmt.Sprintf("'%s'", v)
		}
		sql += fmt.Sprintf(" CHECK (%s IN (%s))", field.Name, strings.Join(quoted, ", "))
	}

	sql += ";"

	return DDLStatement{
		SQL:         sql,
		Entity:      entityName,
		Description: fmt.Sprintf("add column %s (%s)", field.Name, field.Type),
		Phase:       1,
	}
}

// generateFKConstraint produces ALTER TABLE ADD CONSTRAINT for a foreign key.
func generateFKConstraint(entityName string, field *model.Field) DDLStatement {
	constraintName := fmt.Sprintf("fk_%s_%s", entityName, field.Name)
	sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(id);",
		entityName, constraintName, field.Name, field.References)

	return DDLStatement{
		SQL:         sql,
		Entity:      entityName,
		Description: fmt.Sprintf("FK %s -> %s", field.Name, field.References),
		Phase:       2,
	}
}

// generateUniqueConstraint produces ALTER TABLE ADD CONSTRAINT for a unique field.
func generateUniqueConstraint(entityName string, field *model.Field) DDLStatement {
	constraintName := fmt.Sprintf("uq_%s_%s", entityName, field.Name)
	sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s);",
		entityName, constraintName, field.Name)

	return DDLStatement{
		SQL:         sql,
		Entity:      entityName,
		Description: fmt.Sprintf("unique %s", field.Name),
		Phase:       2,
	}
}

// generateCompositeUnique produces a composite unique constraint from
// a field's must_be_unique_within declaration.
func generateCompositeUnique(entityName string, fields []string) DDLStatement {
	constraintName := fmt.Sprintf("uq_%s_%s", entityName, strings.Join(fields, "_"))
	fieldList := strings.Join(fields, ", ")
	sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s);",
		entityName, constraintName, fieldList)

	return DDLStatement{
		SQL:         sql,
		Entity:      entityName,
		Description: fmt.Sprintf("composite unique (%s)", fieldList),
		Phase:       2,
	}
}

// generateCheckConstraint produces a CHECK constraint for numeric ranges
// on an existing or new column.
func generateCheckConstraint(entityName string, field *model.Field) DDLStatement {
	var conditions []string

	if field.MinValue != nil {
		conditions = append(conditions, fmt.Sprintf("%s >= %v", field.Name, field.MinValue))
	}
	if field.MaxValue != nil {
		conditions = append(conditions, fmt.Sprintf("%s <= %v", field.Name, field.MaxValue))
	}

	if len(conditions) == 0 {
		return DDLStatement{}
	}

	constraintName := fmt.Sprintf("ck_%s_%s_range", entityName, field.Name)
	sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s);",
		entityName, constraintName, strings.Join(conditions, " AND "))

	return DDLStatement{
		SQL:         sql,
		Entity:      entityName,
		Description: fmt.Sprintf("check range on %s", field.Name),
		Phase:       2,
	}
}

// generateIndex produces a CREATE INDEX statement.
func generateIndex(entityName string, index *model.Index) DDLStatement {
	fieldList := strings.Join(index.Fields, ", ")
	indexName := index.Name
	if indexName == "" {
		prefix := "idx"
		if index.Unique {
			prefix = "uq"
		}
		indexName = fmt.Sprintf("%s_%s_%s", prefix, entityName, strings.Join(index.Fields, "_"))
	}

	unique := ""
	if index.Unique {
		unique = "UNIQUE "
	}

	sql := fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s);",
		unique, indexName, entityName, fieldList)

	return DDLStatement{
		SQL:         sql,
		Entity:      entityName,
		Description: fmt.Sprintf("index %s on (%s)", indexName, fieldList),
		Phase:       3,
	}
}

// generateRevokeAppendOnly produces REVOKE statements for append-only tables.
// Removes UPDATE and DELETE permissions for all non-admin roles.
func generateRevokeAppendOnly(entityName string, roles []string) []DDLStatement {
	var statements []DDLStatement
	for _, role := range roles {
		sql := fmt.Sprintf("REVOKE UPDATE, DELETE ON %s FROM %s;", entityName, role)
		statements = append(statements, DDLStatement{
			SQL:         sql,
			Entity:      entityName,
			Description: fmt.Sprintf("revoke UPDATE/DELETE for %s", role),
			Phase:       4,
		})
	}
	return statements
}

// OrderByDependency reorders DDL statements to respect FK dependencies
// within each phase. Uses the schema's topological load order.
func OrderByDependency(statements []DDLStatement, loadOrder []string) []DDLStatement {
	// Build position map: entity name -> position in load order.
	posMap := make(map[string]int, len(loadOrder))
	for i, name := range loadOrder {
		posMap[name] = i
	}

	// Stable sort by (phase, position in load order).
	sort.SliceStable(statements, func(i, j int) bool {
		if statements[i].Phase != statements[j].Phase {
			return statements[i].Phase < statements[j].Phase
		}
		posI, okI := posMap[statements[i].Entity]
		posJ, okJ := posMap[statements[j].Entity]
		if !okI {
			posI = len(loadOrder)
		}
		if !okJ {
			posJ = len(loadOrder)
		}
		return posI < posJ
	})

	return statements
}

// formatDefault converts a Go default value to a SQL literal string.
func formatDefault(fieldType string, value interface{}) string {
	if value == nil {
		return "NULL"
	}

	switch fieldType {
	case "boolean":
		switch v := value.(type) {
		case bool:
			if v {
				return "TRUE"
			}
			return "FALSE"
		case string:
			lower := strings.ToLower(v)
			if lower == "true" {
				return "TRUE"
			}
			return "FALSE"
		}
	case "int":
		return fmt.Sprintf("%v", value)
	case "float":
		return fmt.Sprintf("%v", value)
	case "varchar", "text", "enum":
		return fmt.Sprintf("'%s'", escapeSQLString(fmt.Sprintf("%v", value)))
	case "date":
		return fmt.Sprintf("'%s'", escapeSQLString(fmt.Sprintf("%v", value)))
	}

	return fmt.Sprintf("'%s'", escapeSQLString(fmt.Sprintf("%v", value)))
}

// escapeSQLString escapes single quotes in a SQL string literal.
func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// buildConstraintMap builds a constraint map from a model.Field for use
// with vocabulary.GetPostgresType. (Duplicated from differ.go for package locality.)
func buildConstraintMapFromField(f *model.Field) map[string]interface{} {
	m := make(map[string]interface{})
	if f.MaxLength != nil {
		m["max_length"] = *f.MaxLength
	}
	if f.MinLength != nil {
		m["min_length"] = *f.MinLength
	}
	if f.MaxValue != nil {
		m["max_value"] = f.MaxValue
	}
	if f.MinValue != nil {
		m["min_value"] = f.MinValue
	}
	if f.PrecisionDecimalPlaces != nil {
		m["precision_decimal_places"] = *f.PrecisionDecimalPlaces
	}
	if len(f.EnumValues) > 0 {
		m["enum_values"] = f.EnumValues
	}
	if f.References != "" {
		m["references"] = f.References
	}
	return m
}
