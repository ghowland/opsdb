package loader

import (
	"encoding/json"
	"fmt"

	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/pg"
)

// PopulateMeta writes schema metadata to _schema_* tables after DDL apply.
// Creates a new _schema_version row, upserts entity type rows, field rows,
// and relationship rows. Runs within the same transaction as the DDL apply.
func PopulateMeta(tx *pg.Tx, schema *model.Schema, changes []AllowedChange, label string) error {
	// Step 1: insert new _schema_version row.
	versionID, err := insertSchemaVersion(tx, label)
	if err != nil {
		return fmt.Errorf("inserting schema version: %w", err)
	}

	// Step 2: mark previous versions as not current.
	_, err = pg.ExecInTx(tx,
		`UPDATE _schema_version SET is_current = false, updated_time = NOW()
		 WHERE is_current = true AND id != $1`, versionID)
	if err != nil {
		return fmt.Errorf("marking previous versions non-current: %w", err)
	}

	// Step 3: upsert entity types.
	err = upsertEntityTypes(tx, schema, versionID)
	if err != nil {
		return fmt.Errorf("upserting entity types: %w", err)
	}

	// Step 4: upsert fields.
	err = upsertFields(tx, schema, versionID)
	if err != nil {
		return fmt.Errorf("upserting fields: %w", err)
	}

	// Step 5: upsert relationships.
	err = upsertRelationships(tx, schema, versionID)
	if err != nil {
		return fmt.Errorf("upserting relationships: %w", err)
	}

	return nil
}

// insertSchemaVersion creates a new _schema_version row with is_current=true.
// Links to the previous current version as parent. Returns the new version ID.
func insertSchemaVersion(tx *pg.Tx, label string) (int, error) {
	// Find the current parent version (may not exist on first apply).
	var parentID *int
	row := pg.QueryRowInTx(tx,
		`SELECT id FROM _schema_version WHERE is_current = true LIMIT 1`)
	var pid int
	err := row.Scan(&pid)
	if err == nil {
		parentID = &pid
	}
	// No parent on first ever apply — that's fine.

	var newID int
	err = pg.QueryRowInTx(tx,
		`INSERT INTO _schema_version (
			label, is_current, parent__schema_version_id,
			created_time, updated_time
		) VALUES ($1, true, $2, NOW(), NOW())
		RETURNING id`,
		label, parentID).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("inserting _schema_version: %w", err)
	}

	return newID, nil
}

// upsertEntityTypes inserts or updates _schema_entity_type rows for
// all entities in the schema.
func upsertEntityTypes(tx *pg.Tx, schema *model.Schema, versionID int) error {
	for _, entity := range schema.Entities {
		// Check if entity already exists.
		var existingID int
		err := pg.QueryRowInTx(tx,
			`SELECT id FROM _schema_entity_type WHERE table_name = $1`,
			entity.Name).Scan(&existingID)

		if err != nil {
			// Entity does not exist — insert.
			_, err = pg.ExecInTx(tx,
				`INSERT INTO _schema_entity_type (
					table_name, description, category,
					is_versioned, is_soft_delete, is_hierarchical, is_append_only,
					_schema_version_introduced_id,
					created_time, updated_time
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`,
				entity.Name, entity.Description, entity.Category,
				entity.Versioned, entity.SoftDelete, entity.Hierarchical, entity.AppendOnly,
				versionID)
			if err != nil {
				return fmt.Errorf("inserting entity type %s: %w", entity.Name, err)
			}
		} else {
			// Entity exists — update description and timestamp.
			_, err = pg.ExecInTx(tx,
				`UPDATE _schema_entity_type SET
					description = $1,
					category = $2,
					is_versioned = $3,
					is_soft_delete = $4,
					is_hierarchical = $5,
					is_append_only = $6,
					updated_time = NOW()
				WHERE table_name = $7`,
				entity.Description, entity.Category,
				entity.Versioned, entity.SoftDelete, entity.Hierarchical, entity.AppendOnly,
				entity.Name)
			if err != nil {
				return fmt.Errorf("updating entity type %s: %w", entity.Name, err)
			}
		}
	}
	return nil
}

// upsertFields inserts or updates _schema_field rows for all fields
// across all entities.
func upsertFields(tx *pg.Tx, schema *model.Schema, versionID int) error {
	for _, entity := range schema.Entities {
		// Look up entity type ID.
		var entityTypeID int
		err := pg.QueryRowInTx(tx,
			`SELECT id FROM _schema_entity_type WHERE table_name = $1`,
			entity.Name).Scan(&entityTypeID)
		if err != nil {
			return fmt.Errorf("looking up entity type ID for %s: %w", entity.Name, err)
		}

		for _, field := range entity.Fields {
			constraintJSON, err := buildFieldConstraintJSON(field)
			if err != nil {
				return fmt.Errorf("building constraint JSON for %s.%s: %w", entity.Name, field.Name, err)
			}

			isPK := field.Name == "id"
			isFK := field.Type == "foreign_key"
			fkTarget := field.References

			var defaultStr *string
			if field.Default != nil {
				s := fmt.Sprintf("%v", field.Default)
				defaultStr = &s
			}

			// Check if field already exists.
			var existingID int
			err = pg.QueryRowInTx(tx,
				`SELECT id FROM _schema_field
				 WHERE schema_entity_type_id = $1 AND field_name = $2`,
				entityTypeID, field.Name).Scan(&existingID)

			if err != nil {
				// Field does not exist — insert.
				_, err = pg.ExecInTx(tx,
					`INSERT INTO _schema_field (
						schema_entity_type_id, field_name, field_type, description,
						is_nullable, is_primary_key, is_foreign_key, fk_target_entity,
						default_value, constraint_data_json,
						is_reserved, is_governance,
						_schema_version_introduced_id,
						created_time, updated_time
					) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW())`,
					entityTypeID, field.Name, field.Type, field.Description,
					field.Nullable, isPK, isFK, fkTarget,
					defaultStr, constraintJSON,
					field.IsReserved, field.IsGovernance,
					versionID)
				if err != nil {
					return fmt.Errorf("inserting field %s.%s: %w", entity.Name, field.Name, err)
				}
			} else {
				// Field exists — update.
				_, err = pg.ExecInTx(tx,
					`UPDATE _schema_field SET
						description = $1,
						is_nullable = $2,
						default_value = $3,
						constraint_data_json = $4,
						updated_time = NOW()
					WHERE schema_entity_type_id = $5 AND field_name = $6`,
					field.Description, field.Nullable, defaultStr, constraintJSON,
					entityTypeID, field.Name)
				if err != nil {
					return fmt.Errorf("updating field %s.%s: %w", entity.Name, field.Name, err)
				}
			}
		}
	}
	return nil
}

// upsertRelationships inserts or updates _schema_relationship rows.
func upsertRelationships(tx *pg.Tx, schema *model.Schema, versionID int) error {
	for _, rel := range schema.Relationships {
		// Check if relationship already exists.
		var existingID int
		err := pg.QueryRowInTx(tx,
			`SELECT id FROM _schema_relationship
			 WHERE source_entity_name = $1 AND source_field_name = $2 AND target_entity_name = $3`,
			rel.SourceEntity, rel.SourceField, rel.TargetEntity).Scan(&existingID)

		if err != nil {
			// Relationship does not exist — insert.
			_, err = pg.ExecInTx(tx,
				`INSERT INTO _schema_relationship (
					source_entity_name, source_field_name, target_entity_name,
					cardinality, on_delete_action, is_self_referential,
					_schema_version_introduced_id,
					created_time, updated_time
				) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`,
				rel.SourceEntity, rel.SourceField, rel.TargetEntity,
				rel.Cardinality, rel.OnDeleteAction, rel.IsSelfReferential,
				versionID)
			if err != nil {
				return fmt.Errorf("inserting relationship %s.%s -> %s: %w",
					rel.SourceEntity, rel.SourceField, rel.TargetEntity, err)
			}
		} else {
			// Relationship exists — update timestamp.
			_, err = pg.ExecInTx(tx,
				`UPDATE _schema_relationship SET
					cardinality = $1,
					on_delete_action = $2,
					updated_time = NOW()
				WHERE source_entity_name = $3 AND source_field_name = $4 AND target_entity_name = $5`,
				rel.Cardinality, rel.OnDeleteAction,
				rel.SourceEntity, rel.SourceField, rel.TargetEntity)
			if err != nil {
				return fmt.Errorf("updating relationship %s.%s -> %s: %w",
					rel.SourceEntity, rel.SourceField, rel.TargetEntity, err)
			}
		}
	}
	return nil
}

// markDeprecated sets _schema_version_deprecated_id on entities or fields
// that exist in the database but not in the desired schema.
func markDeprecated(tx *pg.Tx, fieldName string, entityName string, versionID int) error {
	if fieldName == "" {
		// Deprecate entire entity.
		_, err := pg.ExecInTx(tx,
			`UPDATE _schema_entity_type SET
				_schema_version_deprecated_id = $1, updated_time = NOW()
			WHERE table_name = $2 AND _schema_version_deprecated_id IS NULL`,
			versionID, entityName)
		if err != nil {
			return fmt.Errorf("deprecating entity %s: %w", entityName, err)
		}
	} else {
		// Deprecate single field.
		_, err := pg.ExecInTx(tx,
			`UPDATE _schema_field SET
				_schema_version_deprecated_id = $1, updated_time = NOW()
			WHERE field_name = $2
				AND schema_entity_type_id = (
					SELECT id FROM _schema_entity_type WHERE table_name = $3
				)
				AND _schema_version_deprecated_id IS NULL`,
			versionID, fieldName, entityName)
		if err != nil {
			return fmt.Errorf("deprecating field %s.%s: %w", entityName, fieldName, err)
		}
	}
	return nil
}

// buildFieldConstraintJSON serializes a field's constraints into a JSON string
// for storage in _schema_field.constraint_data_json.
func buildFieldConstraintJSON(field model.Field) (string, error) {
	constraints := make(map[string]interface{})

	// MaxLength and MinLength are plain int; 0 means unset.
	if field.MaxLength > 0 {
		constraints["max_length"] = field.MaxLength
	}
	if field.MinLength > 0 {
		constraints["min_length"] = field.MinLength
	}

	// MaxValue and MinValue are *float64.
	if field.MaxValue != nil {
		constraints["max_value"] = *field.MaxValue
	}
	if field.MinValue != nil {
		constraints["min_value"] = *field.MinValue
	}

	if field.PrecisionDecimalPlaces != nil {
		constraints["precision_decimal_places"] = *field.PrecisionDecimalPlaces
	}
	if len(field.EnumValues) > 0 {
		constraints["enum_values"] = field.EnumValues
	}
	if field.References != "" {
		constraints["references"] = field.References
	}
	if field.JsonTypeDiscriminator != "" {
		constraints["json_type_discriminator"] = field.JsonTypeDiscriminator
	}
	if len(field.MustBeUniqueWithin) > 0 {
		constraints["must_be_unique_within"] = field.MustBeUniqueWithin
	}
	if field.Unique {
		constraints["unique"] = true
	}

	if len(constraints) == 0 {
		return "{}", nil
	}

	b, err := json.Marshal(constraints)
	if err != nil {
		return "", fmt.Errorf("marshaling constraints: %w", err)
	}
	return string(b), nil
}
