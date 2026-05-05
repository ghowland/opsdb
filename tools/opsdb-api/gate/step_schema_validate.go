
// === opsdb-api/gate/step_schema_validate.go ===
package gate

// StepSchemaValidate is gate step 3: Schema Validation.
// Checks operation shape against registered schema metadata in _schema_* tables.
func StepSchemaValidate(ctx *GateContext) error {
	// TODO: read entity type from runtime schema cache
	//   if entity type not found: reject with "unknown entity type"
	//
	// TODO: for writes (create, update):
	//   for each field in the operation:
	//     check field exists in _schema_field for this entity type
	//     check value type matches field type
	//   for creates:
	//     check all required (non-nullable, no-default) fields are present
	//   for unknown fields:
	//     reject with "unknown field {name} on entity {type}"
	//
	// TODO: set ctx.SchemaValid = true on success
	// TODO: on failure: reject with step=3, structured error listing each failing field
	return nil
}

