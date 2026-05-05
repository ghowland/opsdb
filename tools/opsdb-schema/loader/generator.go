
//# tools/opsdb-schema/loader/generator.go

go
package loader

import (
	"fmt"
	"strings"

	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/vocabulary"
)

// DDLStatement represents one SQL DDL statement to execute.
type DDLStatement struct {
	SQL         string
	Entity      string
	Description string
	Phase       int // 1=create tables, 2=add constraints, 3=create indexes, 4=revoke permissions
}

// GenerateDDL generates ordered Postgres DDL statements from allowed changes.
// Produces CREATE TABLE, ALTER TABLE, CHECK/FK/UNIQUE constraints, CREATE INDEX,
// and REVOKE statements for append-only tables.
func GenerateDDL(schema *model.Schema, changes []AllowedChange) ([]DDLStatement, error) {
	// TODO: initialize statements slice
	// TODO: group changes by type:
	//   new entities -> CREATE TABLE statements (phase 1)
	//   new fields on existing entities -> ALTER TABLE ADD COLUMN (phase 1)
	//   constraint changes -> ALTER TABLE (phase 2)
	//   new indexes -> CREATE INDEX (phase 3)
	//   append-only entities -> REVOKE statements (phase 4)

	// TODO: for each new entity in schema.LoadOrder (respects FK dependencies):
	//   if entity is in new entities set:
	//     generate CREATE TABLE with all columns and inline constraints
	//     generate FK constraints as separate ALTER TABLE statements (phase 2)
	//     generate indexes as CREATE INDEX statements (phase 3)
	//     if entity.AppendOnly: generate REVOKE UPDATE, DELETE (phase 4)

	// TODO: for each new field on existing entity:
	//   generate ALTER TABLE ADD COLUMN
	//   if field has FK: generate ALTER TABLE ADD FOREIGN KEY (phase 2)

	// TODO: for each constraint change:
	//   generate ALTER TABLE to modify constraint

	// TODO: order all statements by phase, then by schema.LoadOrder within phase
	// TODO: return ordered statements

	_ = schema
	_ = changes
	return nil, fmt.Errorf("not implemented")
}

// generateCreateTable produces a CREATE TABLE statement with all columns,
// inline CHECK constraints, NOT NULL constraints, and DEFAULT values.
// FK constraints are generated separately to handle dependency ordering.
func generateCreateTable(entity *model.Entity) DDLStatement {
	// TODO: build column definitions:
	//   for each field in entity.Fields:
	//     columnType := vocabulary.GetPostgresType(field.Type, field constraints)
	//     line = "  {field.Name} {columnType}"
	//     if not field.Nullable: line += " NOT NULL"
	//     if field.Default != nil: line += " DEFAULT {literal value}"
	//     if field.Name == "id": line += " GENERATED ALWAYS AS IDENTITY PRIMARY KEY"
	//     if field.Type == "enum":
	//       line += " CHECK ({field.Name} IN ({quoted enum values}))"
	//     if field.Type == "int" or "float" and has range:
	//       generate CHECK ({field.Name} >= {min} AND {field.Name} <= {max})

	// TODO: SQL = "CREATE TABLE IF NOT EXISTS {entity.Name} (\n{columns joined by ,\n}\n);"
	// TODO: return DDLStatement{SQL, entity.Name, "create table", phase=1}
	_ = vocabulary.GetPostgresType
	return DDLStatement{}
}

// generateAlterAddColumn produces ALTER TABLE ADD COLUMN for a new field
// on an existing entity.
func generateAlterAddColumn(entityName string, field *model.Field) DDLStatement {
	// TODO: columnType := vocabulary.GetPostgresType(field.Type, constraints)
	// TODO: SQL = "ALTER TABLE {entityName} ADD COLUMN {field.Name} {columnType}"
	// TODO: add NOT NULL, DEFAULT, CHECK as appropriate
	// TODO: return DDLStatement{SQL, entityName, "add column {field.Name}", phase=1}
	return DDLStatement{}
}

// generateFKConstraint produces ALTER TABLE ADD CONSTRAINT for a foreign key.
func generateFKConstraint(entityName string, field *model.Field) DDLStatement {
	// TODO: constraintName = "fk_{entityName}_{field.Name}"
	// TODO: SQL = "ALTER TABLE {entityName} ADD CONSTRAINT {constraintName} FOREIGN KEY ({field.Name}) REFERENCES {field.References}(id);"
	// TODO: return DDLStatement{SQL, entityName, "FK {field.Name} -> {field.References}", phase=2}
	return DDLStatement{}
}

// generateUniqueConstraint produces ALTER TABLE ADD CONSTRAINT for a unique field.
func generateUniqueConstraint(entityName string, field *model.Field) DDLStatement {
	// TODO: constraintName = "uq_{entityName}_{field.Name}"
	// TODO: SQL = "ALTER TABLE {entityName} ADD CONSTRAINT {constraintName} UNIQUE ({field.Name});"
	// TODO: return DDLStatement{SQL, entityName, "unique {field.Name}", phase=2}
	return DDLStatement{}
}

// generateCompositeUnique produces a composite unique constraint from
// a field's must_be_unique_within declaration.
func generateCompositeUnique(entityName string, fields []string) DDLStatement {
	// TODO: constraintName = "uq_{entityName}_{fields joined by _}"
	// TODO: SQL = "ALTER TABLE {entityName} ADD CONSTRAINT {constraintName} UNIQUE ({fields joined by , });"
	// TODO: return DDLStatement{SQL, entityName, "composite unique ({fields})", phase=2}
	_ = strings.Join(fields, ", ")
	return DDLStatement{}
}

// generateCheckConstraint produces a CHECK constraint for numeric ranges
// or enum values on an existing column.
func generateCheckConstraint(entityName string, field *model.Field) DDLStatement {
	// TODO: if field has numeric range (MinValue or MaxValue):
	//   constraintName = "ck_{entityName}_{field.Name}_range"
	//   conditions := []string{}
	//   if MinValue set: "{field.Name} >= {MinValue}"
	//   if MaxValue set: "{field.Name} <= {MaxValue}"
	//   SQL = "ALTER TABLE {entityName} ADD CONSTRAINT {constraintName} CHECK ({conditions joined by AND});"
	// TODO: if field has enum values:
	//   constraintName = "ck_{entityName}_{field.Name}_enum"
	//   SQL = "ALTER TABLE {entityName} ADD CONSTRAINT {constraintName} CHECK ({field.Name} IN ({quoted values}));"
	return DDLStatement{}
}

// generateIndex produces a CREATE INDEX statement.
func generateIndex(entityName string, index *model.Index) DDLStatement {
	// TODO: indexName = "idx_{entityName}_{index fields joined by _}"
	// TODO: unique prefix = "UNIQUE " if index.Unique else ""
	// TODO: SQL = "CREATE {unique}INDEX IF NOT EXISTS {indexName} ON {entityName} ({fields joined by , });"
	// TODO: return DDLStatement{SQL, entityName, "index on ({fields})", phase=3}
	return DDLStatement{}
}

// generateRevokeAppendOnly produces REVOKE statements for append-only tables.
// Removes UPDATE and DELETE permissions for all application roles.
func generateRevokeAppendOnly(entityName string, roles []string) []DDLStatement {
	// TODO: for each role in roles (except admin):
	//   SQL = "REVOKE UPDATE, DELETE ON {entityName} FROM {role};"
	//   append DDLStatement{SQL, entityName, "revoke UPDATE/DELETE for {role}", phase=4}
	// TODO: return statements
	return nil
}

// OrderByDependency reorders DDL statements to respect FK dependencies
// within each phase. Uses the schema's topological load order.
func OrderByDependency(statements []DDLStatement, loadOrder []string) []DDLStatement {
	// TODO: build position map: entity name -> position in loadOrder
	// TODO: stable sort statements by (phase, position in loadOrder)
	// TODO: return sorted statements
	return statements
}


