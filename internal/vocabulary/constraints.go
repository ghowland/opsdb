
// === internal/vocabulary/constraints.go ===
package vocabulary

// ValidateConstraints is the master constraint validator for a field.
// Checks that only allowed constraints for the type are present,
// required constraints are present, and values are valid.
func ValidateConstraints(fieldType string, constraints map[string]interface{}) error {
	// TODO: get allowed and required constraints for fieldType
	// TODO: check no unrecognized constraints present
	// TODO: check all required constraints present
	// TODO: dispatch to per-constraint validators
	return nil
}

// ValidateNumericRange checks that min_value <= max_value.
func ValidateNumericRange(minValue, maxValue interface{}) error {
	// TODO: convert to float64, compare
	// TODO: error if min > max
	return nil
}

// ValidateStringLength checks that min_length <= max_length, max_length >= 1.
func ValidateStringLength(minLength, maxLength interface{}) error {
	// TODO: convert to int, compare
	// TODO: error if min > max or max < 1
	return nil
}

// ValidateEnumValues checks enum values list: non-empty, no duplicates,
// all lowercase_underscore.
func ValidateEnumValues(values []string) error {
	// TODO: check len > 0 and len <= 256
	// TODO: check no duplicates
	// TODO: check each value is lowercase + underscores only
	return nil
}

// ValidateReferences checks that the referenced entity exists in the known set.
func ValidateReferences(targetEntity string, knownEntities map[string]bool) error {
	// TODO: check targetEntity in knownEntities
	// TODO: error message: "referenced entity {target} not found or not yet loaded"
	return nil
}

// ValidateJsonDiscriminator checks that the discriminator field exists in
// the same entity and is an enum type.
func ValidateJsonDiscriminator(discriminatorField string, entityFields []Field) error {
	// TODO: find field by name in entityFields
	// TODO: check it exists
	// TODO: check its type is "enum"
	return nil
}

// ValidatePrecision checks precision_decimal_places is in 0-15 range.
func ValidatePrecision(places interface{}) error {
	// TODO: convert to int, check 0 <= places <= 15
	return nil
}

// Field is imported from model package; redeclared here for function signatures.
// In actual code, use model.Field.
type Field = struct {
	Name string
	Type string
}

