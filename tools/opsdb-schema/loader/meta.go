//# tools/opsdb-schema/loader/meta.go

go
package loader

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/pg"
)

// PopulateMeta writes schema metadata to _schema_* tables after DDL apply.
// Creates a new _schema_version row, upserts entity type rows, field rows,
// and relationship rows. Runs within the same transaction as the DDL apply.
func PopulateMeta(tx *pg.Tx, schema *model.Schema, changes []AllowedChange, label string) error {
	// TODO: step 1: insert new _schema_version row
	//   versionID, err := insertSchemaVersion(tx, label)
	//   if err: return err

	// TODO: step 2: mark previous version as not current
	//   UPDATE _schema_version SET is_current = false WHERE is_current = true AND id != versionID

	// TODO: step 3: upsert entity types
	//   err = upsertEntityTypes(tx, schema, versionID)
	//   if err: return err

	// TODO: step 4: upsert fields
	//   err = upsertFields(tx, schema, versionID)
	//   if err: return err

	// TODO: step 5: upsert relationships
	//   err = upsertRelationships(tx, schema, versionID)
	//   if err: return err

	// TODO: step 6: mark deprecated entities/fields for removed-from-desired
	//   (these are entities/fields that exist in DB but not in desired schema;
	//    they can't be deleted per evolution rules, but we mark them deprecated)

	return fmt.Errorf("not implemented")
}

// insertSchemaVersion creates a new _schema_version row with is_current=true.
// Returns the new version ID.
func insertSchemaVersion(tx *pg.Tx, label string) (int, error) {
	// TODO: INSERT INTO _schema_version (
	//     label, applied_time, is_current, parent_schema_version_id,
	//     created_time, updated_time
	//   ) VALUES (
	//     label, NOW(), true,
	//     (SELECT id FROM _schema_version WHERE is_current = true LIMIT 1),
	//     NOW(), NOW()
	//   ) RETURNING id
	// TODO: scan returned ID
	// TODO: return id, nil
	return 0, fmt.Errorf("not implemented")
}

// upsertEntityTypes inserts or updates _schema_entity_type rows for
// all entities in the schema.
func upsertEntityTypes(tx *pg.Tx, schema *model.Schema, versionID int) error {
	// TODO: for each entity in schema.Entities:
	//   check if entity already exists in _schema_entity_type (by table_name)
	//   if new:
	//     INSERT INTO _schema_entity_type (
	//       table_name, description, category, is_versioned, is_soft_delete,
	//       is_hierarchical, is_append_only,
	//       _schema_version_introduced_id,
	//       created_time, updated_time
	//     ) VALUES (entity.Name, entity.Description, entity.Category,
	//       entity.Versioned, entity.SoftDelete, entity.Hierarchical, entity.AppendOnly,
	//       versionID, NOW(), NOW())
	//   if existing:
	//     UPDATE _schema_entity_type SET
	//       description = entity.Description,
	//       updated_time = NOW()
	//     WHERE table_name = entity.Name
	return nil
}

// upsertFields inserts or updates _schema_field rows for all fields
// across all entities.
func upsertFields(tx *pg.Tx, schema *model.Schema, versionID int) error {
	// TODO: for each entity in schema.Entities:
	//   look up entity_type_id from _schema_entity_type WHERE table_name = entity.Name
	//   for each field in entity.Fields:
	//     build constraint_data_json from field constraints:
	//       {max_length, min_length, max_value, min_value, precision_decimal_places,
	//        enum_values, references, json_type_discriminator, must_be_unique_within}
	//     check if field exists in _schema_field (by entity_type_id + field_name)
	//     if new:
	//       INSERT INTO _schema_field (
	//         schema_entity_type_id, field_name, field_type, description,
	//         is_nullable, is_primary_key, is_foreign_key, fk_target_entity,
	//         default_value, constraint_data_json,
	//         is_reserved, is_governance,
	//         _schema_version_introduced_id,
	//         created_time, updated_time
	//       ) VALUES (...)
	//     if existing:
	//       UPDATE _schema_field SET
	//         description = field.Description,
	//         constraint_data_json = updated constraints,
	//         updated_time = NOW()
	//       WHERE schema_entity_type_id = entityTypeID AND field_name = field.Name
	return nil
}

// upsertRelationships inserts or updates _schema_relationship rows.
func upsertRelationships(tx *pg.Tx, schema *model.Schema, versionID int) error {
	// TODO: for each relationship in schema.Relationships:
	//   check if relationship exists (by source_entity + source_field + target_entity)
	//   if new:
	//     INSERT INTO _schema_relationship (
	//       source_entity_name, source_field_name, target_entity_name,
	//       cardinality, on_delete_action, is_self_referential,
	//       _schema_version_introduced_id,
	//       created_time, updated_time
	//     ) VALUES (...)
	//   if existing: update updated_time
	return nil
}

// markDeprecated sets _schema_version_deprecated_id on entities or fields
// that exist in the database but not in the desired schema.
func markDeprecated(tx *pg.Tx, entityOrFieldName string, entityName string, versionID int) error {
	// TODO: if entityOrFieldName is empty (whole entity deprecated):
	//   UPDATE _schema_entity_type SET
	//     _schema_version_deprecated_id = versionID, updated_time = NOW()
	//   WHERE table_name = entityName AND _schema_version_deprecated_id IS NULL
	// TODO: if entityOrFieldName is set (single field deprecated):
	//   UPDATE _schema_field SET
	//     _schema_version_deprecated_id = versionID, updated_time = NOW()
	//   WHERE field_name = entityOrFieldName
	//     AND schema_entity_type_id = (SELECT id FROM _schema_entity_type WHERE table_name = entityName)
	//     AND _schema_version_deprecated_id IS NULL
	return nil
}


