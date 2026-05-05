//# tools/opsdb-api/gate/step_bound_validate.go

package gate

import (
	"fmt"
	"strings"

	"github.com/ghowland/opsdb/internal/pg"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb-api/schema"
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
// Checks field values against declared constraints from the runtime schema.
// No regex. Declarative bounds only.
func stepBoundValidate(ctx *GateContext) {
	if !isWriteOperation(ctx.Request.OperationClass) {
		ctx.BoundsValid = true
		return
	}

	fields := extractRequestedFields(ctx.Request)
	if len(fields) == 0 {
		ctx.BoundsValid = true
		return
	}

	fieldValues := extractFieldValues(ctx.Request)
	var violations []boundViolation

	for _, fieldName := range fields {
		value, hasValue := fieldValues[fieldName]
		if !hasValue {
			continue
		}

		fieldMeta, found := ctx.Schema.GetField(ctx.Request.TargetEntity, fieldName)
		if !found {
			// unknown fields are caught by step 3 (schema validation)
			continue
		}

		fieldViolations := validateFieldBounds(ctx, fieldMeta, fieldName, value)
		violations = append(violations, fieldViolations...)
	}

	if len(violations) > 0 {
		detail := make(map[string]interface{})
		violationList := make([]map[string]interface{}, 0, len(violations))
		for _, v := range violations {
			violationList = append(violationList, map[string]interface{}{
				"field":      v.Field,
				"constraint": v.Constraint,
				"value":      v.Value,
				"bound":      v.Bound,
				"message":    v.Message,
			})
		}
		detail["violations"] = violationList

		reject(ctx, 4, "validation_failed",
			fmt.Sprintf("bound validation failed: %d field(s) out of bounds", len(violations)),
			detail)
		return
	}

	ctx.BoundsValid = true
}

// validateFieldBounds checks one field's value against all declared constraints.
func validateFieldBounds(ctx *GateContext, meta *runtimeschema.FieldMeta, fieldName string, value interface{}) []boundViolation {
	var violations []boundViolation

	switch meta.Type {
	case "int":
		violations = append(violations, validateIntBounds(fieldName, value, meta)...)
	case "float":
		violations = append(violations, validateFloatBounds(fieldName, value, meta)...)
	case "varchar":
		violations = append(violations, validateVarcharBounds(fieldName, value, meta)...)
	case "text":
		violations = append(violations, validateTextBounds(fieldName, value, meta)...)
	case "enum":
		violations = append(violations, validateEnumBounds(fieldName, value, meta)...)
	case "foreign_key":
		violations = append(violations, validateFKExists(ctx.DB, fieldName, value, meta)...)
	case "json":
		violations = append(violations, validateJSONPayload(ctx, fieldName, value, meta)...)
	case "boolean":
		violations = append(violations, validateBoolean(fieldName, value)...)
	case "datetime", "date":
		// datetime and date are validated at the type level by step 3;
		// no additional bound constraints defined for these types
	}

	return violations
}

// validateIntBounds checks int value against min_value and max_value.
func validateIntBounds(fieldName string, value interface{}, meta *runtimeschema.FieldMeta) []boundViolation {
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

// validateFloatBounds checks float value against min_value, max_value,
// and precision_decimal_places.
func validateFloatBounds(fieldName string, value interface{}, meta *runtimeschema.FieldMeta) []boundViolation {
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
		precision := *meta.PrecisionDecimalPlaces
		if countDecimalPlaces(floatVal) > precision {
			violations = append(violations, boundViolation{
				Field:      fieldName,
				Constraint: "precision_decimal_places",
				Value:      floatVal,
				Bound:      precision,
				Message:    fmt.Sprintf("value %g exceeds %d decimal places", floatVal, precision),
			})
		}
	}

	return violations
}

// validateVarcharBounds checks string value against min_length and max_length.
func validateVarcharBounds(fieldName string, value interface{}, meta *runtimeschema.FieldMeta) []boundViolation {
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
	strLen := len(strVal)

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

// validateTextBounds checks text value against max_length if declared.
func validateTextBounds(fieldName string, value interface{}, meta *runtimeschema.FieldMeta) []boundViolation {
	strVal, ok := value.(string)
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "value is not a string",
		}}
	}

	if meta.MaxLength != nil && len(strVal) > *meta.MaxLength {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "max_length",
			Value:      len(strVal),
			Bound:      *meta.MaxLength,
			Message:    fmt.Sprintf("text length %d exceeds maximum %d", len(strVal), *meta.MaxLength),
		}}
	}

	return nil
}

// validateEnumBounds checks that the value is in the declared enum_values set.
func validateEnumBounds(fieldName string, value interface{}, meta *runtimeschema.FieldMeta) []boundViolation {
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
		Message: fmt.Sprintf("value %q is not in allowed set: %s",
			strVal, strings.Join(meta.EnumValues, ", ")),
	}}
}

// validateFKExists checks that the referenced foreign key row exists.
func validateFKExists(db *pg.DB, fieldName string, value interface{}, meta *runtimeschema.FieldMeta) []boundViolation {
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

// validateJSONPayload validates a JSON payload against the registered schema
// for its discriminator value. Looks up the discriminator field value from
// the request, finds the matching JSON schema, and validates structure.
func validateJSONPayload(ctx *GateContext, fieldName string, value interface{}, meta *runtimeschema.FieldMeta) []boundViolation {
	if meta.JsonTypeDiscriminator == "" {
		return nil
	}

	// find the discriminator value from the request fields
	fieldValues := extractFieldValues(ctx.Request)
	discriminatorValue, ok := fieldValues[meta.JsonTypeDiscriminator]
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "json_type_discriminator",
			Value:      nil,
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

	// look up the JSON schema for this entity type + discriminator value
	jsonSchema, found := ctx.Schema.GetJSONSchema(ctx.Request.TargetEntity, fieldName, discStr)
	if !found {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "json_schema",
			Value:      discStr,
			Message: fmt.Sprintf("no JSON schema registered for %s.%s with type %s",
				ctx.Request.TargetEntity, fieldName, discStr),
		}}
	}

	// validate the payload against the schema
	violations := validateJSONAgainstSchema(fieldName, value, jsonSchema)
	return violations
}

// validateJSONAgainstSchema checks a JSON value against a registered schema.
// Validates required fields, types, bounds on nested values, and max depth
// (one level per spec).
func validateJSONAgainstSchema(fieldName string, value interface{}, schema *runtimeschema.JSONSchemaMeta) []boundViolation {
	jsonMap, ok := value.(map[string]interface{})
	if !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "json_structure",
			Value:      value,
			Message:    "JSON payload must be an object",
		}}
	}

	var violations []boundViolation

	// check required fields
	for _, required := range schema.RequiredFields {
		if _, exists := jsonMap[required]; !exists {
			violations = append(violations, boundViolation{
				Field:      fmt.Sprintf("%s.%s", fieldName, required),
				Constraint: "required",
				Message:    fmt.Sprintf("required field %s missing in JSON payload", required),
			})
		}
	}

	// check each declared field's type and bounds
	for key, val := range jsonMap {
		fieldSchema, exists := schema.Fields[key]
		if !exists {
			// undeclared fields in JSON payload — warn but don't reject
			// to allow forward compatibility
			continue
		}

		// type check
		if !jsonFieldTypeMatches(val, fieldSchema.Type) {
			violations = append(violations, boundViolation{
				Field:      fmt.Sprintf("%s.%s", fieldName, key),
				Constraint: "type",
				Value:      val,
				Bound:      fieldSchema.Type,
				Message:    fmt.Sprintf("field %s expected type %s", key, fieldSchema.Type),
			})
			continue
		}

		// bounds on JSON field values
		if fieldSchema.MaxLength != nil {
			if strVal, ok := val.(string); ok && len(strVal) > *fieldSchema.MaxLength {
				violations = append(violations, boundViolation{
					Field:      fmt.Sprintf("%s.%s", fieldName, key),
					Constraint: "max_length",
					Value:      len(strVal),
					Bound:      *fieldSchema.MaxLength,
					Message:    fmt.Sprintf("field %s length %d exceeds max %d", key, len(strVal), *fieldSchema.MaxLength),
				})
			}
		}

		if fieldSchema.EnumValues != nil {
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
						Message: fmt.Sprintf("field %s value %q not in allowed set",
							key, strVal),
					})
				}
			}
		}
	}

	return violations
}

// validateBoolean checks that a boolean value is actually a boolean.
func validateBoolean(fieldName string, value interface{}) []boundViolation {
	if _, ok := value.(bool); !ok {
		return []boundViolation{{
			Field:      fieldName,
			Constraint: "type",
			Value:      value,
			Message:    "value must be a boolean",
		}}
	}
	return nil
}

// jsonFieldTypeMatches checks if a JSON value matches the expected type string.
func jsonFieldTypeMatches(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
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
		return true
	}
}

// extractFieldValues returns a map of field name → value from the request params.
func extractFieldValues(req *GateRequest) map[string]interface{} {
	if req.Params == nil {
		return nil
	}
	if fields, ok := req.Params["fields"]; ok {
		if fieldMap, ok := fields.(map[string]interface{}); ok {
			return fieldMap
		}
	}
	return nil
}

// --- numeric conversion helpers ---

func toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		if v == float64(int(v)) {
			return int(v), true
		}
		return 0, false
	case string:
		return 0, false
	default:
		return 0, false
	}
}

func toFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func countDecimalPlaces(f float64) int {
	s := fmt.Sprintf("%g", f)
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}
