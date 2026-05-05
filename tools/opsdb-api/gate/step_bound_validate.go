
// === opsdb-api/gate/step_bound_validate.go ===
package gate

// StepBoundValidate is gate step 4: Bound Validation.
// Checks field values against declared constraints from the schema.
// No regex. Declarative bounds only.
func StepBoundValidate(ctx *GateContext) error {
	// TODO: for each field value in the operation:
	//   int/float: check against min_value, max_value if declared
	//   varchar: check against min_length, max_length
	//   enum: check value is in enum_values list
	//   foreign_key: check referenced row exists (SELECT 1 FROM target WHERE id = value)
	//   json: validate structure against registered JSON schema for discriminator value
	//         (look up discriminator field value, find matching json_schema, validate)
	//
	// TODO: no regex evaluation at any point
	// TODO: anchored pattern matching (prefix%, %suffix) for fields that declare it
	//       implemented as string HasPrefix/HasSuffix, not regex
	//
	// TODO: on failure: reject with step=4, list of (field, constraint, submitted value, bound)
	// TODO: set ctx.BoundsValid = true on success
	return nil
}

