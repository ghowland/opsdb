//# tools/opsdb-api/gate/step_schema_validate.go

package gate

import (
	"fmt"

	runtimeschema "github.com/ghowland/opsdb/tools/opsdb-api/schema"
)

// schemaViolation records one field that failed schema validation.
type schemaViolation struct {
	Field   string
	Message string
}

// stepSchemaValidate is gate step 3: Schema Validation.
// Checks operation shape against registered schema metadata loaded
// from _schema_* tables at startup.
func stepSchemaValidate(ctx *GateContext) {
	// verify the target entity type exists in the schema
	entityMeta, found := ctx.Schema.GetEntityType(ctx.Request.TargetEntity)
	if !found {
		reject(ctx, 3, "validation_failed",
			fmt.Sprintf("unknown entity type: %s", ctx.Request.TargetEntity),
			map[string]interface{}{
				"entity_type": ctx.Request.TargetEntity,
			})
		return
	}

	// reads don't need field-level validation
	if isReadOnly(ctx.Request.OperationClass) {
		ctx.SchemaValid = true
		return
	}

	fieldValues := extractFieldValues(ctx.Request)
	if fieldValues == nil {
		// write with no fields — valid for operations like approve/reject
		// that don't carry field data
		ctx.SchemaValid = true
		return
	}

	var violations []schemaViolation

	// check each submitted field exists and has compatible type
	for fieldName, value := range fieldValues {
		fieldMeta, fieldFound := ctx.Schema.GetField(ctx.Request.TargetEntity, fieldName)
		if !fieldFound {
			violations = append(violations, schemaViolation{
				Field:   fieldName,
				Message: fmt.Sprintf("unknown field %q on entity %s", fieldName, ctx.Request.TargetEntity),
			})
			continue
		}

		// reject writes to reserved fields — these are system-managed
		if fieldMeta.IsReserved && !isReservedFieldWritable(fieldName, ctx.Request.Operation) {
			violations = append(violations, schemaViolation{
				Field:   fieldName,
				Message: fmt.Sprintf("field %q is reserved and cannot be set directly", fieldName),
			})
			continue
		}

		// reject writes to deprecated fields
		if fieldMeta.IsDeprecated {
			violations = append(violations, schemaViolation{
				Field:   fieldName,
				Message: fmt.Sprintf("field %q is deprecated; use %s instead", fieldName, fieldMeta.DeprecatedAlternative),
			})
			continue
		}

		// check value type compatibility
		typeViolation := checkTypeCompatibility(fieldName, value, fieldMeta)
		if typeViolation != "" {
			violations = append(violations, schemaViolation{
				Field:   fieldName,
				Message: typeViolation,
			})
		}
	}

	// for creates: check all required fields are present
	if isCreateOperation(ctx.Request) {
		missingFields := checkRequiredFields(ctx, entityMeta, fieldValues)
		violations = append(violations, missingFields...)
	}

	if len(violations) > 0 {
		violationList := make([]map[string]interface{}, 0, len(violations))
		for _, v := range violations {
			violationList = append(violationList, map[string]interface{}{
				"field":   v.Field,
				"message": v.Message,
			})
		}

		reject(ctx, 3, "validation_failed",
			fmt.Sprintf("schema validation failed: %d field error(s)", len(violations)),
			map[string]interface{}{
				"entity_type": ctx.Request.TargetEntity,
				"violations":  violationList,
			})
		return
	}

	ctx.SchemaValid = true
}

// checkTypeCompatibility verifies a submitted value is compatible with
// the declared field type. Returns empty string on success or an error
// message on mismatch.
func checkTypeCompatibility(fieldName string, value interface{}, meta *runtimeschema.FieldMeta) string {
	if value == nil {
		if !meta.Nullable {
			return fmt.Sprintf("field %q does not accept null", fieldName)
		}
		return ""
	}

	switch meta.Type {
	case "int":
		if _, ok := toInt(value); !ok {
			return fmt.Sprintf("field %q expects int, got %T", fieldName, value)
		}

	case "float":
		if _, ok := toFloat(value); !ok {
			return fmt.Sprintf("field %q expects float, got %T", fieldName, value)
		}

	case "varchar", "text":
		if _, ok := value.(string); !ok {
			return fmt.Sprintf("field %q expects string, got %T", fieldName, value)
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Sprintf("field %q expects boolean, got %T", fieldName, value)
		}

	case "enum":
		if _, ok := value.(string); !ok {
			return fmt.Sprintf("field %q expects string (enum value), got %T", fieldName, value)
		}

	case "foreign_key":
		if _, ok := toInt(value); !ok {
			return fmt.Sprintf("field %q expects integer (foreign key), got %T", fieldName, value)
		}

	case "json":
		switch value.(type) {
		case map[string]interface{}, []interface{}:
			// valid JSON structures
		case string:
			// JSON passed as string is acceptable — will be parsed downstream
		default:
			return fmt.Sprintf("field %q expects JSON object or array, got %T", fieldName, value)
		}

	case "datetime":
		if _, ok := value.(string); !ok {
			return fmt.Sprintf("field %q expects datetime string, got %T", fieldName, value)
		}

	case "date":
		if _, ok := value.(string); !ok {
			return fmt.Sprintf("field %q expects date string, got %T", fieldName, value)
		}
	}

	return ""
}

// checkRequiredFields verifies that all non-nullable fields without defaults
// are present in a create operation.
func checkRequiredFields(ctx *GateContext, entityMeta *runtimeschema.EntityTypeMeta, submittedFields map[string]interface{}) []schemaViolation {
	var violations []schemaViolation

	allFields := ctx.Schema.GetAllFields(ctx.Request.TargetEntity)
	for _, fieldMeta := range allFields {
		// skip fields that are optional
		if fieldMeta.Nullable {
			continue
		}
		// skip fields with defaults — the database will fill them
		if fieldMeta.HasDefault {
			continue
		}
		// skip reserved fields — they are system-managed
		if fieldMeta.IsReserved {
			continue
		}
		// skip governance fields — they are injected
		if fieldMeta.IsGovernance {
			continue
		}

		if _, present := submittedFields[fieldMeta.Name]; !present {
			violations = append(violations, schemaViolation{
				Field: fieldMeta.Name,
				Message: fmt.Sprintf("required field %q is missing (non-nullable, no default)",
					fieldMeta.Name),
			})
		}
	}

	return violations
}

// isCreateOperation determines if the request is creating a new entity
// (as opposed to updating an existing one).
func isCreateOperation(req *GateRequest) bool {
	// creates have no target entity ID — the row doesn't exist yet
	if req.TargetEntityID > 0 {
		return false
	}

	// change set operations are not direct creates
	if req.OperationClass == "write-cs" || req.OperationClass == "cm-action" {
		return false
	}

	return isWriteOperation(req.OperationClass)
}

// isReservedFieldWritable checks whether a reserved field can be written
// in the context of a specific operation. Most reserved fields are never
// writable, but some operations need to set specific reserved fields.
func isReservedFieldWritable(fieldName string, operation string) bool {
	switch fieldName {
	case "is_active":
		// soft-delete: the reaper and direct observation writes can set is_active
		return operation == "write_observation" || operation == "apply_change_set_field_change"
	case "created_time", "updated_time", "id":
		// never writable by callers
		return false
	default:
		// governance fields with underscore prefix may be writable
		// through change management
		if len(fieldName) > 0 && fieldName[0] == '_' {
			return operation == "apply_change_set_field_change"
		}
		return false
	}
}
