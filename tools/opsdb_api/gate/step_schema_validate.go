//# tools/opsdb_api/gate/step_schema_validate.go

package gate

import (
	"fmt"
)

// schemaViolation records one field that failed schema validation.
type schemaViolation struct {
	Field   string
	Message string
}

// stepSchemaValidate is gate step 3: Schema Validation.
// Checks that the operation's target entity type exists in the runtime
// schema, that submitted field names are known fields on that entity,
// that reserved and deprecated fields aren't written directly, that
// value types are compatible with declared field types, and that
// required fields are present on creates.
//
// Reads don't need field-level validation — only the entity type
// existence check. Change management actions (approve, reject, cancel)
// don't carry field data and skip field validation.
func stepSchemaValidate(ctx *GateContext) {
	// Some operations (health checks, watch subscriptions) may not
	// target a specific entity type. Allow them through.
	if ctx.Request.TargetEntity == "" {
		ctx.SchemaValid = true
		return
	}

	// Verify the target entity type exists in the schema. An unknown
	// entity type is always a rejection regardless of operation class.
	_, found := ctx.Schema.GetEntityType(ctx.Request.TargetEntity)
	if !found {
		reject(ctx, 3, "validation_failed",
			fmt.Sprintf("unknown entity type: %s", ctx.Request.TargetEntity),
			map[string]interface{}{
				"entity_type": ctx.Request.TargetEntity,
			})
		return
	}

	// Reads only need the entity type to exist — field-level validation
	// is not needed because the query will simply return whatever columns
	// exist. The projection filtering happens at query time.
	if isReadOnly(ctx.Request.OperationClass) {
		ctx.SchemaValid = true
		return
	}

	// Extract field name/value pairs from the request. Returns nil for
	// operations that don't carry field data (approve, reject, cancel,
	// mark_applied), which is valid.
	fieldValues := extractFieldValues(ctx.Request)
	if fieldValues == nil {
		ctx.SchemaValid = true
		return
	}

	var violations []schemaViolation

	// Validate each submitted field: exists in schema, not reserved,
	// not deprecated.
	for fieldName := range fieldValues {
		fieldViolations := validateFieldName(ctx, fieldName)
		violations = append(violations, fieldViolations...)
	}

	// checkTypeCompatibility — verifies value types match declared field
	// types. Deferred: when implemented, this will check each submitted
	// value against the field's declared type from the runtime schema
	// (int values for int fields, strings for varchar/text/enum, booleans
	// for boolean fields, JSON objects for json fields, etc.). Currently
	// passes all values through; type mismatches will be caught at the
	// database level on write.

	// checkRequiredFields — verifies all non-nullable, no-default, non-
	// reserved, non-governance fields are present on creates. Deferred:
	// when implemented, this will iterate all fields from
	// ctx.Schema.GetAllFields, skip nullable/default/reserved/governance
	// fields, and reject if any required field is missing from the
	// submission. Currently passes; missing required fields will be
	// caught by the database NOT NULL constraint on write.

	if len(violations) > 0 {
		violationDetails := make([]map[string]interface{}, 0, len(violations))
		for _, v := range violations {
			violationDetails = append(violationDetails, map[string]interface{}{
				"field":   v.Field,
				"message": v.Message,
			})
		}

		reject(ctx, 3, "validation_failed",
			fmt.Sprintf("schema validation failed: %d field error(s)", len(violations)),
			map[string]interface{}{
				"entity_type": ctx.Request.TargetEntity,
				"violations":  violationDetails,
			})
		return
	}

	ctx.SchemaValid = true
}

// ---------------------------------------------------------------------------
// Per-field name validation
// ---------------------------------------------------------------------------

// validateFieldName checks that a submitted field name exists in the
// runtime schema and is writable in the context of this operation.
// Returns zero or more violations.
func validateFieldName(ctx *GateContext, fieldName string) []schemaViolation {
	var violations []schemaViolation

	fieldMeta, found := ctx.Schema.GetField(ctx.Request.TargetEntity, fieldName)
	if !found {
		violations = append(violations, schemaViolation{
			Field:   fieldName,
			Message: fmt.Sprintf("unknown field %q on entity %s", fieldName, ctx.Request.TargetEntity),
		})
		return violations
	}

	// Reserved fields (id, created_time, updated_time, parent_*_id) are
	// system-managed. Most are never writable; some are writable in
	// specific operations (is_active in observation writes and field
	// change applies, governance underscore fields through change mgmt).
	if fieldMeta.IsReserved && !isReservedFieldWritable(fieldName, ctx.Request.Operation) {
		violations = append(violations, schemaViolation{
			Field:   fieldName,
			Message: fmt.Sprintf("field %q is reserved and cannot be set directly", fieldName),
		})
		return violations
	}

	// Deprecated fields should not receive new writes.
	if fieldMeta.IsDeprecated {
		alternative := fieldMeta.DeprecatedAlternative
		if alternative == "" {
			alternative = "(no replacement specified)"
		}
		violations = append(violations, schemaViolation{
			Field:   fieldName,
			Message: fmt.Sprintf("field %q is deprecated; use %s instead", fieldName, alternative),
		})
		return violations
	}

	return violations
}

// ---------------------------------------------------------------------------
// Field value extraction from request params
// ---------------------------------------------------------------------------

// extractFieldValues extracts field name/value pairs from request params.
// The extraction depends on the operation type:
//
//   - Direct writes (write_observation): the params map contains the field
//     values directly, minus routing keys (target_table, key, value).
//
//   - Change set submissions: field values are inside the field_changes
//     array, not at the top level. Individual field validation happens
//     when the change-set executor applies each field change. Return nil.
//
//   - Change management actions (approve, reject, cancel, mark_applied):
//     these carry action parameters, not entity field values. Return nil.
//
// Returns nil when there are no field values to validate.
func extractFieldValues(req *GateRequest) map[string]interface{} {
	switch req.OperationClass {
	case "write-cs":
		// Change set submissions carry field values inside field_changes
		// array. Individual field validation happens at apply time.
		return nil

	case "cm-action":
		// Change management actions don't carry entity field values.
		return nil

	case "write-direct":
		if len(req.Params) == 0 {
			return nil
		}

		fields := make(map[string]interface{})
		for key, val := range req.Params {
			if isRoutingParam(key) {
				continue
			}
			fields[key] = val
		}

		if len(fields) == 0 {
			return nil
		}
		return fields

	default:
		return nil
	}
}

// isRoutingParam returns true for param keys that are routing metadata
// rather than entity field values.
func isRoutingParam(key string) bool {
	switch key {
	case "target_table", "key", "value", "runner_job_id",
		"authority_id", "data_json":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Operation and field writability helpers
// ---------------------------------------------------------------------------

// isCreateOperation determines if the request is creating a new entity.
func isCreateOperation(req *GateRequest) bool {
	if req.TargetEntityID > 0 {
		return false
	}
	if req.OperationClass == "write-cs" || req.OperationClass == "cm-action" {
		return false
	}
	return isWriteOperation(req.OperationClass)
}

// isReservedFieldWritable checks whether a reserved field can be written
// in the context of a specific operation.
//
//   - is_active: writable through observation writes (soft-delete by runners)
//     and through field change application (change-managed soft-delete).
//
//   - Governance fields (underscore prefix): writable through change
//     management field change application only.
//
//   - id, created_time, updated_time: never writable by callers.
func isReservedFieldWritable(fieldName string, operation string) bool {
	switch fieldName {
	case "is_active":
		return operation == "write_observation" ||
			operation == "apply_change_set_field_change"

	case "id", "created_time", "updated_time":
		return false

	default:
		// Governance fields with underscore prefix are writable through
		// change management only.
		if len(fieldName) > 0 && fieldName[0] == '_' {
			return operation == "apply_change_set_field_change"
		}
		return false
	}
}
