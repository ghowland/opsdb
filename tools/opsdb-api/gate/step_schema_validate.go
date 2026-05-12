//# tools/opsdb-api/gate/step_schema_validate.go

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
	// Verify the target entity type exists in the schema. An unknown
	// entity type is always a rejection regardless of operation class.
	if ctx.Request.TargetEntity == "" {
		// Some operations (health checks, watch subscriptions) may not
		// target a specific entity. Allow them through.
		ctx.SchemaValid = true
		return
	}

	entityMeta, found := ctx.Schema.GetEntityType(ctx.Request.TargetEntity)
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

	// Validate each submitted field: exists, not reserved, not deprecated,
	// type compatible.
	for fieldName, value := range fieldValues {
		fieldViolations := validateOneField(ctx, fieldName, value)
		violations = append(violations, fieldViolations...)
	}

	// For creates: check that all required fields (non-nullable, no default,
	// not reserved, not governance) are present in the submission.
	if isCreateOperation(ctx.Request) {
		missing := checkRequiredFields(ctx, entityMeta, fieldValues)
		violations = append(violations, missing...)
	}

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
// Per-field validation
// ---------------------------------------------------------------------------

// validateOneField checks a single submitted field against the runtime
// schema. Returns zero or more violations.
func validateOneField(ctx *GateContext, fieldName string, value interface{}) []schemaViolation {
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

	// Deprecated fields should not receive new writes. The deprecation
	// message tells the caller what to use instead.
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

	// Check value type compatibility with the declared field type.
	typeErr := checkTypeCompatibility(fieldName, value, fieldMeta)
	if typeErr != "" {
		violations = append(violations, schemaViolation{
			Field:   fieldName,
			Message: typeErr,
		})
	}

	return violations
}

// ---------------------------------------------------------------------------
// Type compatibility checks
// ---------------------------------------------------------------------------

// checkTypeCompatibility verifies a submitted value is compatible with
// the declared field type from the runtime schema. Returns empty string
// on success or an error message describing the mismatch.
//
// This is structural type checking only — not bound validation. Whether
// an int value is within the declared min/max range is step 4's job.
// This step checks whether the value is an int at all.
func checkTypeCompatibility(fieldName string, value interface{}, meta *runtimeSchemaFieldMeta) string {
	if value == nil {
		if !meta.Nullable {
			return fmt.Sprintf("field %q does not accept null", fieldName)
		}
		return ""
	}

	switch meta.Type {
	case "int":
		if !isIntLike(value) {
			return fmt.Sprintf("field %q expects int, got %T", fieldName, value)
		}

	case "float":
		if !isFloatLike(value) {
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
		if !isIntLike(value) {
			return fmt.Sprintf("field %q expects integer (foreign key), got %T", fieldName, value)
		}

	case "json":
		switch value.(type) {
		case map[string]interface{}, []interface{}, string:
			// Valid: JSON object, JSON array, or JSON-as-string
		default:
			return fmt.Sprintf("field %q expects JSON object, array, or string, got %T", fieldName, value)
		}

	case "datetime":
		if _, ok := value.(string); !ok {
			return fmt.Sprintf("field %q expects datetime string (ISO 8601), got %T", fieldName, value)
		}

	case "date":
		if _, ok := value.(string); !ok {
			return fmt.Sprintf("field %q expects date string (YYYY-MM-DD), got %T", fieldName, value)
		}
	}

	return ""
}

// ---------------------------------------------------------------------------
// Required field checks
// ---------------------------------------------------------------------------

// checkRequiredFields verifies that all required fields are present in a
// create operation. A field is required if it is non-nullable, has no
// default value, is not reserved (system-managed), and is not a
// governance field (injected by the system).
func checkRequiredFields(ctx *GateContext, entityMeta *runtimeSchemaEntityTypeMeta, submittedFields map[string]interface{}) []schemaViolation {
	var violations []schemaViolation

	allFields := ctx.Schema.GetAllFields(ctx.Request.TargetEntity)
	for _, fieldMeta := range allFields {
		if fieldMeta.Nullable {
			continue
		}
		if fieldMeta.HasDefault {
			continue
		}
		if fieldMeta.IsReserved {
			continue
		}
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
//     array, not at the top level. We don't validate individual field
//     change values at the schema step — the per-field validation happens
//     when the change-set executor applies each field change. Return nil
//     to skip field validation.
//
//   - Change management actions (approve, reject, cancel, mark_applied):
//     these carry action parameters (change_set_id, comment, reason),
//     not entity field values. Return nil to skip field validation.
//
// Returns nil when there are no field values to validate, which is
// a valid state (not an error).
func extractFieldValues(req *GateRequest) map[string]interface{} {
	switch req.OperationClass {
	case "write-cs":
		// Change set submissions carry field values inside field_changes
		// array. Individual field validation happens at apply time, not
		// at submission time. Skip here.
		return nil

	case "cm-action":
		// Change management actions don't carry entity field values.
		return nil

	case "write-direct":
		// Direct writes: extract field values from params, excluding
		// routing parameters that aren't column names.
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
// Operation classification helpers
// ---------------------------------------------------------------------------

// isCreateOperation determines if the request is creating a new entity
// (as opposed to updating an existing one). Creates have no target
// entity ID because the row doesn't exist yet. Change set operations
// and change management actions are not direct creates.
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
// in the context of a specific operation. Most reserved fields are never
// writable by callers. Exceptions:
//
//   - is_active: writable through observation writes (soft-delete by runners)
//     and through field change application (change-managed soft-delete).
//
//   - Governance fields (underscore prefix): writable through change
//     management field change application only.
//
// - id, created_time, updated_time: never writable by callers.
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

// ---------------------------------------------------------------------------
// Type checking helpers
// ---------------------------------------------------------------------------

// runtimeSchemaFieldMeta is a type alias for the field metadata returned
// by the runtime schema package. Defined here to avoid importing the
// schema package's types directly into type-check logic — the gate
// package accesses these through the RuntimeSchema interface methods.
//
// When the runtime schema package is finalized, this alias will reference
// the actual type. For now it matches the contract from the IOSE:
// Name, Type, Nullable, HasDefault, IsReserved, IsGovernance,
// IsDeprecated, DeprecatedAlternative.
type runtimeSchemaFieldMeta = interface {
	GetName() string
	GetType() string
	GetNullable() bool
	GetHasDefault() bool
	GetIsReserved() bool
	GetIsGovernance() bool
	GetIsDeprecated() bool
	GetDeprecatedAlternative() string
}

// runtimeSchemaEntityTypeMeta is a type alias for entity type metadata.
type runtimeSchemaEntityTypeMeta = interface {
	GetVersioned() bool
}

// isIntLike returns true if the value can be interpreted as an integer.
// Handles int, int64, float64 (from JSON unmarshaling where all numbers
// are float64), and numeric strings.
func isIntLike(v interface{}) bool {
	switch v.(type) {
	case int, int64:
		return true
	case float64:
		f := v.(float64)
		return f == float64(int64(f))
	default:
		return false
	}
}

// isFloatLike returns true if the value can be interpreted as a float.
// Accepts int, int64, float64 (any number is a valid float).
func isFloatLike(v interface{}) bool {
	switch v.(type) {
	case float64, int, int64:
		return true
	default:
		return false
	}
}
