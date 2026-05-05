//# tools/opsdb-schema/loader/validator.go

go
package loader

import (
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
	// TODO: initialize errors slice
	// TODO: validate entity name via conventions.ValidateEntityName
	// TODO: validate category is in metaSchema.AllowedCategories
	// TODO: validate fields via validateFields
	// TODO: validate indexes via validateIndexes
	// TODO: validate governance keys via validateGovernance
	// TODO: scan raw YAML for forbidden patterns via vocabulary.ScanForForbiddenPatterns
	//   convert each ForbiddenViolation to SchemaError
	// TODO: check for reserved field name collisions:
	//   if any declared field has name matching a reserved field (id, created_time, etc.)
	//   error: "field {name} is reserved and must not be declared in entity files"
	// TODO: validate versioned + append_only are mutually exclusive
	// TODO: return accumulated errors
	_ = conventions.ValidateEntityName
	_ = vocabulary.ScanForForbiddenPatterns
	return nil
}

// validateFields checks each field's type, constraints, modifiers, and naming.
func validateFields(fields []model.Field, metaSchema *MetaSchema, knownEntities map[string]bool) []SchemaError {
	// TODO: initialize errors slice
	// TODO: for each field:
	//   validate field name via conventions.ValidateFieldName(field.Name, field.Type)
	//   validate type via vocabulary.IsValidType(field.Type)
	//     if invalid: error "unknown field type: {type}"
	//   validate constraints via vocabulary.ValidateConstraints(field.Type, constraints)
	//     constraints built from field's MaxLength, MinLength, MaxValue, MinValue,
	//     PrecisionDecimalPlaces, EnumValues
	//   validate modifiers:
	//     if field.Default set: vocabulary.ValidateDefault(field.Type, field.Default)
	//     if field.Unique: vocabulary.ValidateUnique(field.Type)
	//     if field.MustBeUniqueWithin set: vocabulary.ValidateMustBeUniqueWithin(field.MustBeUniqueWithin, all field names)
	//   if field.Type == "foreign_key":
	//     conventions.ValidateFKName(field.Name, field.References)
	//     vocabulary.ValidateReferences(field.References, knownEntities)
	//   if field.Type == "json":
	//     vocabulary.ValidateJsonDiscriminator(field.JsonTypeDiscriminator, all field names)
	// TODO: check for duplicate field names
	// TODO: return accumulated errors
	return nil
}

// validateIndexes checks that index field references exist on the entity.
func validateIndexes(indexes []model.Index, fields []model.Field) []SchemaError {
	// TODO: build set of field names from fields
	// TODO: for each index:
	//   for each field name in index.Fields:
	//     if not in field name set: error "index references unknown field: {name}"
	//   validate index has at least one field
	// TODO: return accumulated errors
	return nil
}

// validateGovernance checks that governance keys are recognized by the meta-schema.
func validateGovernance(governance map[string]bool, metaSchema *MetaSchema) []SchemaError {
	// TODO: for each key in governance:
	//   if key not in metaSchema.AllowedGovernanceKeys:
	//     error "unknown governance key: {key}"
	// TODO: return accumulated errors
	return nil
}


