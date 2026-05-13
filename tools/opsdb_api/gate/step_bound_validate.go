//# tools/opsdb_api/gate/step_bound_validate.go

package gate

import (
	"fmt"
	"strings"

	"github.com/ghowland/opsdb/internal/pg"
)

// boundViolation records one field value that failed bound validation.
type boundViolation struct {
	Field      string
	Constraint string
	Value      interface{}
	Bound      interface{}
	Message    string
}

// stepBoundValidate is gate step 4: Bound Validation.
// Checks field values against declared constraints from the runtime
// schema metadata. The constraint vocabulary is closed per OPSDB-7 §6:
// numeric ranges (min_value, max_value), string lengths (min_length,
// max_length), enum membership (enum_values), FK existence (references),
// precision (precision_decimal_places), and JSON payload validation
// against registered schemas. No regex — ever.
//
// This step runs after schema validation (step 3) which already verified
// that field names exist and value types are compatible. Step 4 checks
// that values are within the declared bounds for their type.
//
// Skips entirely for read operations.
func stepBoundValidate(ctx *GateContext) {
	if !isWriteOperation(ctx.Request.OperationClass) {
		ctx.BoundsValid = true
		return
	}

	fieldValues := extractFieldValues(ctx.Request)
	if fieldValues == nil || len(fieldValues) == 0 {
		ctx.BoundsValid = true
		return
	}

	var violations []boundViolation

	for fieldName, value := range fieldValues {
		if value == nil {
			// Null values are checked by step 3 (nullable validation).
			// Step 4 only checks non-null values against bounds.
			continue
		}

		fieldMeta, found := ctx.Schema.GetField(ctx.Request.TargetEntity, fieldName)
		if !found {
			// Unknown fields are caught by step 3. Skip here.
			continue
		}

		fieldViolations := validateFieldBounds(ctx, fieldMeta, fieldName, value)
		violations = append(violations, fieldViolations...)
	}

	if len(violations) > 0 {
		violationDetails := make([]map[string]interface{}, 0, len(violations))
		for _, v := range violations {
			violationDetails = append(violationDetails, map[string]interface{}{
				"field":      v.Field,
				"constraint": v.Constraint,
				"value":      v.Value,
				"bound":      v.Bound,
				"message":    v.Message,
			})
		}

		reject(ctx, 4, "validation_failed",
			fmt.Sprintf("bound validation failed: %d field(s) out of bounds", len(violations)),
			map[string]interface{}{
				"entity_type": ctx.Request.TargetEntity,
				"violations":  violationDetails,
			})
		return
	}

	ctx.BoundsValid = true
}

// validateFieldBounds dispatches to the appropriate per-type bound
// validator based on the field's declared type.
func validateFieldBounds(ctx *GateContext, meta *RuntimeFieldMeta, fieldName string, value interface{}) []boundViolation {
	switch meta.Type {
	case "int":
		return validateIntBounds(fieldName, value, meta)
	case "float":
		return validateFloatBounds(fieldName, value, meta)
	case "varchar":
		return validateVarcharBounds(fieldName, value, meta)
	case "text":
		return validateTextBounds(fieldName, value, meta)
	case "enum":
		return validateEnumBounds(fieldName, value, meta)
	case "foreign_key":
		return validateFKExists(ctx.DB, fieldName, value, meta)
	case "json":
		return validateJSONPayload(ctx, fieldName, value, meta)
	case "boolean":
		return validateBoolean(fieldName, value)
	case "datetime", "date":
		// Datetime and date are validated at the type level by step 3.
		// No additional bound constraints are defined for these types
		// in the closed vocabulary.
		return nil
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Integer bounds
// ---------------------------------------------------------------------------

// validateIntBounds checks an int value against min_value and max_value.
func validateIntBounds(fieldName string, value interface{}, meta *RuntimeFieldMeta) []boundViolation {
	intVal, ok := toInt(value)
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "value is not a valid integer",
		}}
	}

	var violations []boundViolation

	if meta.MinValue != nil {
		minVal, ok := toInt(*meta.MinValue)
		if ok && intVal < minVal {
			violations = append(violations, boundViolation{
				Field:      fieldName,
				Constraint: "min_value",
				Value:      intVal,
				Bound:      minVal,
				Message:    fmt.Sprintf("value %d is below minimum %d", intVal, minVal),
			})
		}
	}

	if meta.MaxValue != nil {
		maxVal, ok := toInt(*meta.MaxValue)
		if ok && intVal > maxVal {
			violations = append(violations, boundViolation{
				Field:      fieldName,
				Constraint: "max_value",
				Value:      intVal,
				Bound:      maxVal,
				Message:    fmt.Sprintf("value %d exceeds maximum %d", intVal, maxVal),
			})
		}
	}

	return violations
}

// ---------------------------------------------------------------------------
// Float bounds
// ---------------------------------------------------------------------------

// validateFloatBounds checks a float value against min_value, max_value,
// and precision_decimal_places.
func validateFloatBounds(fieldName string, value interface{}, meta *RuntimeFieldMeta) []boundViolation {
	floatVal, ok := toFloat(value)
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "value is not a valid number",
		}}
	}

	var violations []boundViolation

	if meta.MinValue != nil {
		minVal, ok := toFloat(*meta.MinValue)
		if ok && floatVal < minVal {
			violations = append(violations, boundViolation{
				Field:      fieldName,
				Constraint: "min_value",
				Value:      floatVal,
				Bound:      minVal,
				Message:    fmt.Sprintf("value %g is below minimum %g", floatVal, minVal),
			})
		}
	}

	if meta.MaxValue != nil {
		maxVal, ok := toFloat(*meta.MaxValue)
		if ok && floatVal > maxVal {
			violations = append(violations, boundViolation{
				Field:      fieldName,
				Constraint: "max_value",
				Value:      floatVal,
				Bound:      maxVal,
				Message:    fmt.Sprintf("value %g exceeds maximum %g", floatVal, maxVal),
			})
		}
	}

	if meta.PrecisionDecimalPlaces != nil {
		maxPlaces := *meta.PrecisionDecimalPlaces
		actualPlaces := countDecimalPlaces(floatVal)
		if actualPlaces > maxPlaces {
			violations = append(violations, boundViolation{
				Field:      fieldName,
				Constraint: "precision_decimal_places",
				Value:      floatVal,
				Bound:      maxPlaces,
				Message: fmt.Sprintf("value %g has %d decimal places, maximum is %d",
					floatVal, actualPlaces, maxPlaces),
			})
		}
	}

	return violations
}

// ---------------------------------------------------------------------------
// String bounds (varchar and text)
// ---------------------------------------------------------------------------

// validateVarcharBounds checks a string value against min_length and
// max_length. Both constraints are character-counted.
func validateVarcharBounds(fieldName string, value interface{}, meta *RuntimeFieldMeta) []boundViolation {
	strVal, ok := value.(string)
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "value is not a string",
		}}
	}

	var violations []boundViolation
	strLen := len([]rune(strVal)) // character count, not byte count

	if meta.MinLength != nil && strLen < *meta.MinLength {
		violations = append(violations, boundViolation{
			Field:      fieldName,
			Constraint: "min_length",
			Value:      strLen,
			Bound:      *meta.MinLength,
			Message:    fmt.Sprintf("string length %d is below minimum %d", strLen, *meta.MinLength),
		})
	}

	if meta.MaxLength != nil && strLen > *meta.MaxLength {
		violations = append(violations, boundViolation{
			Field:      fieldName,
			Constraint: "max_length",
			Value:      strLen,
			Bound:      *meta.MaxLength,
			Message:    fmt.Sprintf("string length %d exceeds maximum %d", strLen, *meta.MaxLength),
		})
	}

	return violations
}

// validateTextBounds checks a text value against max_length if declared.
// Text fields may not have a max_length (the schema allows omitting it
// for text fields), in which case no bound is checked.
func validateTextBounds(fieldName string, value interface{}, meta *RuntimeFieldMeta) []boundViolation {
	strVal, ok := value.(string)
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "value is not a string",
		}}
	}

	if meta.MaxLength != nil {
		strLen := len([]rune(strVal))
		if strLen > *meta.MaxLength {
			return []boundViolation{{
				Field:      fieldName,
				Constraint: "max_length",
				Value:      strLen,
				Bound:      *meta.MaxLength,
				Message:    fmt.Sprintf("text length %d exceeds maximum %d", strLen, *meta.MaxLength),
			}}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Enum bounds
// ---------------------------------------------------------------------------

// validateEnumBounds checks that the value is in the declared enum_values
// set. The enum_values list is loaded from the runtime schema (originally
// declared in the entity YAML file per OPSDB-7 §6.3).
func validateEnumBounds(fieldName string, value interface{}, meta *RuntimeFieldMeta) []boundViolation {
	strVal, ok := value.(string)
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "enum value must be a string",
		}}
	}

	if len(meta.EnumValues) == 0 {
		// No enum values declared — nothing to check. This shouldn't
		// happen for a properly-declared enum field, but if the schema
		// metadata is incomplete, pass through rather than reject.
		return nil
	}

	for _, allowed := range meta.EnumValues {
		if strVal == allowed {
			return nil
		}
	}

	return []boundViolation{{
		Field:      fieldName,
		Constraint: "enum_values",
		Value:      strVal,
		Bound:      meta.EnumValues,
		Message: fmt.Sprintf("value %q is not in allowed set: [%s]",
			strVal, strings.Join(meta.EnumValues, ", ")),
	}}
}

// ---------------------------------------------------------------------------
// Foreign key existence
// ---------------------------------------------------------------------------

// validateFKExists checks that the referenced foreign key row exists in
// the target table. The target table name comes from the field's
// References metadata (set from the `references` property in the
// entity YAML file).
func validateFKExists(db *pg.DB, fieldName string, value interface{}, meta *RuntimeFieldMeta) []boundViolation {
	if meta.References == "" {
		return nil
	}

	intVal, ok := toInt(value)
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "foreign key value must be an integer",
		}}
	}

	query := fmt.Sprintf(
		"SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)",
		pg.QuoteIdentifier(meta.References),
	)

	var exists bool
	err := db.QueryRow(query, intVal).Scan(&exists)
	if err != nil {
		// FK check failure — could be the referenced table doesn't
		// exist (during bootstrap), or a transient error. Report as
		// a violation with the error detail.
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "foreign_key",
			Value:      intVal,
			Bound:      meta.References,
			Message:    fmt.Sprintf("failed to verify reference to %s: %v", meta.References, err),
		}}
	}

	if !exists {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "foreign_key",
			Value:      intVal,
			Bound:      meta.References,
			Message:    fmt.Sprintf("referenced %s with id=%d does not exist", meta.References, intVal),
		}}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Boolean validation
// ---------------------------------------------------------------------------

// validateBoolean checks that a boolean value is actually a boolean.
// This is a type check more than a bound check, but it lives here
// because step 3 does structural type compatibility and step 4 does
// the detailed per-type validation.
func validateBoolean(fieldName string, value interface{}) []boundViolation {
	if _, ok := value.(bool); !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "value must be a boolean (true or false)",
		}}
	}
	return nil
}

// ---------------------------------------------------------------------------
// JSON payload validation
// ---------------------------------------------------------------------------

// validateJSONPayload validates a JSON payload against the registered
// schema for its discriminator value. Looks up the discriminator field
// value from the request, finds the matching JSON schema in the runtime
// schema, and validates the payload's structure and field bounds.
//
// Per OPSDB-7 §9.4, JSON payload schemas are one level deep. Lists
// may contain primitives, maps may contain primitives, but nested
// objects are not validated recursively — deeper structure is a signal
// to factor into separate entity types.
func validateJSONPayload(ctx *GateContext, fieldName string, value interface{}, meta *RuntimeFieldMeta) []boundViolation {
	if meta.JsonTypeDiscriminator == "" {
		// No discriminator declared — the JSON field accepts any valid
		// JSON without schema validation.
		return nil
	}

	// Read the discriminator value from the request's field values.
	fieldValues := extractFieldValues(ctx.Request)
	if fieldValues == nil {
		return nil
	}

	discriminatorValue, ok := fieldValues[meta.JsonTypeDiscriminator]
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "json_type_discriminator",
			Bound:      meta.JsonTypeDiscriminator,
			Message: fmt.Sprintf("JSON field %s requires discriminator field %s to be set",
				fieldName, meta.JsonTypeDiscriminator),
		}}
	}

	discStr, ok := discriminatorValue.(string)
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "json_type_discriminator",
			Value:      discriminatorValue,
			Message:    "discriminator value must be a string",
		}}
	}

	// Look up the registered JSON schema for this discriminator value.
	jsonSchema, found := ctx.Schema.GetJSONSchema(
		ctx.Request.TargetEntity, fieldName, discStr)
	if !found {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "json_schema",
			Value:      discStr,
			Message: fmt.Sprintf("no JSON schema registered for %s.%s with type %s",
				ctx.Request.TargetEntity, fieldName, discStr),
		}}
	}

	return validateJSONAgainstSchema(fieldName, value, jsonSchema)
}

// validateJSONAgainstSchema checks a JSON value against a registered
// schema. Validates that the value is an object, checks required fields
// are present, and validates each declared field's type and bounds.
func validateJSONAgainstSchema(fieldName string, value interface{}, schema *RuntimeJSONSchemaMeta) []boundViolation {
	jsonMap, ok := value.(map[string]interface{})
	if !ok {
		// Accept JSON-as-string — try to note it but don't reject.
		// The actual parsing happens downstream when the value is
		// written to the database's JSONB column.
		if _, isStr := value.(string); isStr {
			return nil
		}
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "json_structure",
			Value:      value,
			Message:    "JSON payload must be an object or a JSON string",
		}}
	}

	var violations []boundViolation

	// Check required fields
	for _, required := range schema.RequiredFields {
		if _, exists := jsonMap[required]; !exists {
			violations = append(violations, boundViolation{
				Field:      fmt.Sprintf("%s.%s", fieldName, required),
				Constraint: "required",
				Message:    fmt.Sprintf("required field %s missing in JSON payload", required),
			})
		}
	}

	// Check each declared field's type and bounds
	for key, val := range jsonMap {
		fieldSchema, declared := schema.Fields[key]
		if !declared {
			// Undeclared fields in JSON payloads are allowed for forward
			// compatibility — the schema may not cover every field the
			// source API returns. No violation.
			continue
		}

		// Type check
		if !jsonFieldTypeMatches(val, fieldSchema.Type) {
			violations = append(violations, boundViolation{
				Field:      fmt.Sprintf("%s.%s", fieldName, key),
				Constraint: "type",
				Value:      val,
				Bound:      fieldSchema.Type,
				Message:    fmt.Sprintf("field %s expected type %s, got %T", key, fieldSchema.Type, val),
			})
			continue
		}

		// String length bound
		if fieldSchema.MaxLength != nil {
			if strVal, ok := val.(string); ok {
				strLen := len([]rune(strVal))
				if strLen > *fieldSchema.MaxLength {
					violations = append(violations, boundViolation{
						Field:      fmt.Sprintf("%s.%s", fieldName, key),
						Constraint: "max_length",
						Value:      strLen,
						Bound:      *fieldSchema.MaxLength,
						Message: fmt.Sprintf("field %s length %d exceeds maximum %d",
							key, strLen, *fieldSchema.MaxLength),
					})
				}
			}
		}

		// Enum membership
		if len(fieldSchema.EnumValues) > 0 {
			if strVal, ok := val.(string); ok {
				found := false
				for _, allowed := range fieldSchema.EnumValues {
					if strVal == allowed {
						found = true
						break
					}
				}
				if !found {
					violations = append(violations, boundViolation{
						Field:      fmt.Sprintf("%s.%s", fieldName, key),
						Constraint: "enum_values",
						Value:      strVal,
						Bound:      fieldSchema.EnumValues,
						Message: fmt.Sprintf("field %s value %q not in allowed set: [%s]",
							key, strVal, strings.Join(fieldSchema.EnumValues, ", ")),
					})
				}
			}
		}

		// Numeric range on JSON fields
		if fieldSchema.MinValue != nil || fieldSchema.MaxValue != nil {
			if numVal, ok := toFloat(val); ok {
				if fieldSchema.MinValue != nil {
					if minVal, ok := toFloat(*fieldSchema.MinValue); ok && numVal < minVal {
						violations = append(violations, boundViolation{
							Field:      fmt.Sprintf("%s.%s", fieldName, key),
							Constraint: "min_value",
							Value:      numVal,
							Bound:      minVal,
							Message: fmt.Sprintf("field %s value %g below minimum %g",
								key, numVal, minVal),
						})
					}
				}
				if fieldSchema.MaxValue != nil {
					if maxVal, ok := toFloat(*fieldSchema.MaxValue); ok && numVal > maxVal {
						violations = append(violations, boundViolation{
							Field:      fmt.Sprintf("%s.%s", fieldName, key),
							Constraint: "max_value",
							Value:      numVal,
							Bound:      maxVal,
							Message: fmt.Sprintf("field %s value %g exceeds maximum %g",
								key, numVal, maxVal),
						})
					}
				}
			}
		}

		// List count bounds
		if fieldSchema.MinCount != nil || fieldSchema.MaxCount != nil {
			if listVal, ok := val.([]interface{}); ok {
				listLen := len(listVal)
				if fieldSchema.MinCount != nil && listLen < *fieldSchema.MinCount {
					violations = append(violations, boundViolation{
						Field:      fmt.Sprintf("%s.%s", fieldName, key),
						Constraint: "min_count",
						Value:      listLen,
						Bound:      *fieldSchema.MinCount,
						Message: fmt.Sprintf("field %s has %d items, minimum is %d",
							key, listLen, *fieldSchema.MinCount),
					})
				}
				if fieldSchema.MaxCount != nil && listLen > *fieldSchema.MaxCount {
					violations = append(violations, boundViolation{
						Field:      fmt.Sprintf("%s.%s", fieldName, key),
						Constraint: "max_count",
						Value:      listLen,
						Bound:      *fieldSchema.MaxCount,
						Message: fmt.Sprintf("field %s has %d items, maximum is %d",
							key, listLen, *fieldSchema.MaxCount),
					})
				}
			}
		}

		// Map entry count bounds
		if fieldSchema.MaxEntries != nil {
			if mapVal, ok := val.(map[string]interface{}); ok {
				if len(mapVal) > *fieldSchema.MaxEntries {
					violations = append(violations, boundViolation{
						Field:      fmt.Sprintf("%s.%s", fieldName, key),
						Constraint: "max_entries",
						Value:      len(mapVal),
						Bound:      *fieldSchema.MaxEntries,
						Message: fmt.Sprintf("field %s has %d entries, maximum is %d",
							key, len(mapVal), *fieldSchema.MaxEntries),
					})
				}
			}
		}
	}

	return violations
}

// jsonFieldTypeMatches checks if a JSON value matches the expected type.
func jsonFieldTypeMatches(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string", "varchar":
		_, ok := value.(string)
		return ok
	case "int", "integer":
		_, ok := toInt(value)
		return ok
	case "float", "number":
		_, ok := toFloat(value)
		return ok
	case "bool", "boolean":
		_, ok := value.(bool)
		return ok
	case "list", "array":
		_, ok := value.([]interface{})
		return ok
	case "map", "object":
		_, ok := value.(map[string]interface{})
		return ok
	default:
		// Unknown type — pass through. The schema registered an
		// unrecognized type string; this is a schema issue, not a
		// value issue.
		return true
	}
}

// ---------------------------------------------------------------------------
// Runtime schema type references
// ---------------------------------------------------------------------------

// RuntimeFieldMeta is a type alias for the field metadata struct returned
// by the runtime schema package. When the runtime schema package is
// finalized, this will reference the actual exported type. For now it
// declares the fields step_bound_validate needs.
//
// The runtime schema package contract (from IOSE):
//
//	Name, Type, Nullable, HasDefault, IsReserved, IsGovernance,
//	IsDeprecated, DeprecatedAlternative, References,
//	MinValue, MaxValue, MinLength, MaxLength,
//	PrecisionDecimalPlaces, EnumValues, JsonTypeDiscriminator,
//	AccessClassification
type RuntimeFieldMeta = interface {
	// This alias will be replaced with a concrete struct import when
	// tools/opsdb_api/schema is written. For now, step_bound_validate
	// accesses field metadata through ctx.Schema.GetField() which
	// returns whatever the runtime schema package defines.
}

// RuntimeJSONSchemaMeta represents the registered JSON schema for a
// typed payload field. Loaded from the runtime schema's JSON schema
// registry. Contains the fields and constraints declared in the
// schema/json_schemas/ YAML files.
type RuntimeJSONSchemaMeta struct {
	RequiredFields []string
	Fields         map[string]RuntimeJSONFieldSchema
}

// RuntimeJSONFieldSchema represents one field within a JSON payload schema.
type RuntimeJSONFieldSchema struct {
	Type       string
	EnumValues []string
	MinValue   *interface{}
	MaxValue   *interface{}
	MinLength  *int
	MaxLength  *int
	MinCount   *int
	MaxCount   *int
	MaxEntries *int
}

// ---------------------------------------------------------------------------
// Numeric conversion helpers (value, bool) signatures
// ---------------------------------------------------------------------------

// toInt converts an interface{} to int. Returns (value, true) on success,
// (0, false) on failure. Handles int, int32, int64, float64 (from JSON
// unmarshaling where whole numbers arrive as float64).
//
// This is the (value, bool) variant used by all validation steps.
// step_execute.go uses toIntErr which returns (value, error) for its
// error message needs.
func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case float64:
		// Only accept float64 values that are whole numbers
		if val == float64(int64(val)) {
			return int(val), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// toFloat converts an interface{} to float64. Returns (value, true) on
// success, (0, false) on failure. Accepts all numeric types since any
// number is a valid float.
func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

// countDecimalPlaces counts the number of decimal places in a float value
// by converting to string and counting digits after the decimal point.
func countDecimalPlaces(f float64) int {
	s := fmt.Sprintf("%g", f)
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}
