package loader

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/conventions"
	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/vocabulary"
)

// SchemaError represents one validation error with location and context.
type SchemaError struct {
	Entity   string
	Field    string
	Message  string
	Severity string // error, warning
}

// Validate runs all validation checks on one parsed entity.
// Accumulates errors rather than failing on first — returns the full
// picture of what is wrong. Called once per entity during the loading pipeline.
func Validate(entity *model.Entity, rawYAML map[string]interface{}, metaSchema *MetaSchema, knownEntities map[string]bool) []SchemaError {
	var errors []SchemaError

	// Validate entity name.
	if err := conventions.ValidateEntityName(entity.Name); err != nil {
		errors = append(errors, SchemaError{
			Entity:   entity.Name,
			Message:  fmt.Sprintf("invalid entity name: %v", err),
			Severity: "error",
		})
	}

	// Validate category is in allowed set.
	if !isInList(entity.Category, metaSchema.AllowedCategories) {
		errors = append(errors, SchemaError{
			Entity:   entity.Name,
			Message:  fmt.Sprintf("unknown category %q (allowed: %v)", entity.Category, metaSchema.AllowedCategories),
			Severity: "error",
		})
	}

	// Validate versioned + append_only are mutually exclusive.
	if entity.Versioned && entity.AppendOnly {
		errors = append(errors, SchemaError{
			Entity:   entity.Name,
			Message:  "versioned and append_only are mutually exclusive: an append-only entity cannot have version history (versions require updates)",
			Severity: "error",
		})
	}

	// Check for reserved field name collisions.
	for _, field := range entity.Fields {
		if conventions.IsReservedFieldName(field.Name, entity.Name) {
			errors = append(errors, SchemaError{
				Entity:   entity.Name,
				Field:    field.Name,
				Message:  fmt.Sprintf("field %q is reserved and must not be declared in entity files (it is injected automatically)", field.Name),
				Severity: "error",
			})
		}
	}

	// Validate fields.
	fieldErrors := validateFields(entity.Fields, entity.Name, metaSchema, knownEntities)
	errors = append(errors, fieldErrors...)

	// Validate indexes.
	indexErrors := validateIndexes(entity.Indexes, entity.Fields, entity.Name)
	errors = append(errors, indexErrors...)

	// Validate governance keys.
	if len(entity.Governance) > 0 {
		govErrors := validateGovernance(entity.Governance, metaSchema, entity.Name)
		errors = append(errors, govErrors...)
	}

	// Scan raw YAML for forbidden patterns.
	violations := vocabulary.ScanForForbiddenPatterns(rawYAML)
	for _, v := range violations {
		errors = append(errors, SchemaError{
			Entity:   entity.Name,
			Field:    v.Location,
			Message:  fmt.Sprintf("forbidden pattern '%s': %s (alternative: %s)", v.Pattern, v.Rationale, v.Alternative),
			Severity: "error",
		})
	}

	return errors
}

// validateFields checks each field's type, constraints, modifiers, and naming.
func validateFields(fields []model.Field, entityName string, metaSchema *MetaSchema, knownEntities map[string]bool) []SchemaError {
	var errors []SchemaError

	// Collect all field names for duplicate detection and cross-references.
	allFieldNames := make([]string, 0, len(fields))
	fieldNameMap := make(map[string]string) // name -> type for JSON discriminator validation
	nameCount := make(map[string]int)

	for _, f := range fields {
		allFieldNames = append(allFieldNames, f.Name)
		fieldNameMap[f.Name] = f.Type
		nameCount[f.Name]++
	}

	// Check for duplicate field names.
	for name, count := range nameCount {
		if count > 1 {
			errors = append(errors, SchemaError{
				Entity:   entityName,
				Field:    name,
				Message:  fmt.Sprintf("duplicate field name %q (appears %d times)", name, count),
				Severity: "error",
			})
		}
	}

	for _, field := range fields {
		// Validate field name.
		if err := conventions.ValidateFieldName(field.Name, field.Type); err != nil {
			errors = append(errors, SchemaError{
				Entity:   entityName,
				Field:    field.Name,
				Message:  fmt.Sprintf("invalid field name: %v", err),
				Severity: "error",
			})
		}

		// Validate type is one of the nine allowed types.
		if !vocabulary.IsValidType(field.Type) {
			errors = append(errors, SchemaError{
				Entity:   entityName,
				Field:    field.Name,
				Message:  fmt.Sprintf("unknown field type %q", field.Type),
				Severity: "error",
			})
			continue // can't validate constraints for unknown type
		}

		// Build constraint map from field properties.
		constraintMap := buildFieldConstraintMap(field)

		// Validate constraints against the type's allowed/required constraints.
		if err := vocabulary.ValidateConstraints(field.Type, constraintMap); err != nil {
			errors = append(errors, SchemaError{
				Entity:   entityName,
				Field:    field.Name,
				Message:  fmt.Sprintf("constraint error: %v", err),
				Severity: "error",
			})
		}

		// Validate modifiers.
		if field.Default != nil {
			if err := vocabulary.ValidateDefault(field.Type, field.Default); err != nil {
				errors = append(errors, SchemaError{
					Entity:   entityName,
					Field:    field.Name,
					Message:  fmt.Sprintf("invalid default: %v", err),
					Severity: "error",
				})
			}
		}

		if field.Unique {
			if err := vocabulary.ValidateUnique(field.Type); err != nil {
				errors = append(errors, SchemaError{
					Entity:   entityName,
					Field:    field.Name,
					Message:  fmt.Sprintf("invalid unique modifier: %v", err),
					Severity: "error",
				})
			}
		}

		if len(field.MustBeUniqueWithin) > 0 {
			if err := vocabulary.ValidateMustBeUniqueWithin(field.MustBeUniqueWithin, allFieldNames); err != nil {
				errors = append(errors, SchemaError{
					Entity:   entityName,
					Field:    field.Name,
					Message:  fmt.Sprintf("invalid must_be_unique_within: %v", err),
					Severity: "error",
				})
			}
		}

		// Validate modifier compatibility with type.
		if err := vocabulary.ValidateModifiersForType(
			field.Type,
			field.Default != nil, field.Default,
			field.Unique,
			len(field.MustBeUniqueWithin) > 0,
		); err != nil {
			errors = append(errors, SchemaError{
				Entity:   entityName,
				Field:    field.Name,
				Message:  fmt.Sprintf("modifier error: %v", err),
				Severity: "error",
			})
		}

		// FK-specific validation.
		if field.Type == "foreign_key" {
			if field.References == "" {
				errors = append(errors, SchemaError{
					Entity:   entityName,
					Field:    field.Name,
					Message:  "foreign_key field missing 'references'",
					Severity: "error",
				})
			} else {
				if err := conventions.ValidateFKName(field.Name, field.References); err != nil {
					errors = append(errors, SchemaError{
						Entity:   entityName,
						Field:    field.Name,
						Message:  fmt.Sprintf("FK naming: %v", err),
						Severity: "error",
					})
				}

				// Check referenced entity exists — but only if it's not self-referential
				// (self-referential is always valid) and not the current entity
				// (which isn't in knownEntities yet).
				if field.References != entityName {
					if err := vocabulary.ValidateReferences(field.References, knownEntities); err != nil {
						errors = append(errors, SchemaError{
							Entity:   entityName,
							Field:    field.Name,
							Message:  fmt.Sprintf("FK reference: %v", err),
							Severity: "error",
						})
					}
				}
			}
		}

		// JSON-specific validation.
		if field.Type == "json" {
			if field.JsonTypeDiscriminator == "" {
				errors = append(errors, SchemaError{
					Entity:   entityName,
					Field:    field.Name,
					Message:  "json field missing 'json_type_discriminator'",
					Severity: "error",
				})
			} else {
				if err := vocabulary.ValidateJsonDiscriminator(field.JsonTypeDiscriminator, fieldNameMap); err != nil {
					errors = append(errors, SchemaError{
						Entity:   entityName,
						Field:    field.Name,
						Message:  fmt.Sprintf("JSON discriminator: %v", err),
						Severity: "error",
					})
				}
			}
		}
	}

	return errors
}

// validateIndexes checks that index field references exist on the entity.
func validateIndexes(indexes []model.Index, fields []model.Field, entityName string) []SchemaError {
	var errors []SchemaError

	// Build set of field names.
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f.Name] = true
	}

	// Also include reserved fields that will be injected.
	reservedNames := []string{"id", "created_time", "updated_time", "is_active",
		"parent_" + entityName + "_id"}
	for _, rn := range reservedNames {
		fieldSet[rn] = true
	}

	for i, idx := range indexes {
		if len(idx.Fields) == 0 {
			errors = append(errors, SchemaError{
				Entity:   entityName,
				Message:  fmt.Sprintf("index %d has no fields", i),
				Severity: "error",
			})
			continue
		}

		for _, fieldName := range idx.Fields {
			if !fieldSet[fieldName] {
				errors = append(errors, SchemaError{
					Entity:   entityName,
					Field:    fieldName,
					Message:  fmt.Sprintf("index %d references unknown field %q", i, fieldName),
					Severity: "error",
				})
			}
		}
	}

	return errors
}

// validateGovernance checks that governance keys are recognized by the meta-schema.
func validateGovernance(governance map[string]bool, metaSchema *MetaSchema, entityName string) []SchemaError {
	var errors []SchemaError

	allowedSet := make(map[string]bool, len(metaSchema.AllowedGovernanceKeys))
	for _, key := range metaSchema.AllowedGovernanceKeys {
		allowedSet[key] = true
	}

	for key := range governance {
		if !allowedSet[key] {
			errors = append(errors, SchemaError{
				Entity:   entityName,
				Message:  fmt.Sprintf("unknown governance key %q (allowed: %v)", key, metaSchema.AllowedGovernanceKeys),
				Severity: "error",
			})
		}
	}

	return errors
}

// buildFieldConstraintMap builds a constraint map from a model.Field
// for validation by vocabulary.ValidateConstraints.
func buildFieldConstraintMap(field model.Field) map[string]interface{} {
	m := make(map[string]interface{})

	if field.MaxLength != nil {
		m["max_length"] = *field.MaxLength
	}
	if field.MinLength != nil {
		m["min_length"] = *field.MinLength
	}
	if field.MaxValue != nil {
		m["max_value"] = field.MaxValue
	}
	if field.MinValue != nil {
		m["min_value"] = field.MinValue
	}
	if field.PrecisionDecimalPlaces != nil {
		m["precision_decimal_places"] = *field.PrecisionDecimalPlaces
	}
	if len(field.EnumValues) > 0 {
		m["enum_values"] = field.EnumValues
	}
	if field.References != "" {
		m["references"] = field.References
	}
	if field.JsonTypeDiscriminator != "" {
		m["json_type_discriminator"] = field.JsonTypeDiscriminator
	}

	return m
}

// isInList checks if a string is in a string slice.
func isInList(s string, list []string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
