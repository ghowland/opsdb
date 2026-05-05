
// === internal/vocabulary/modifiers.go ===
package vocabulary

// IsValidModifier checks if modName is one of the three modifiers:
// nullable, default, unique.
func IsValidModifier(modName string) bool {
	// TODO: check against [nullable, default, unique, must_be_unique_within]
	return false
}

// ValidateDefault validates that a default value is a literal appropriate for the field type.
// Rejects expressions, function calls, NOW(), CURRENT_TIMESTAMP, computed values.
func ValidateDefault(fieldType string, defaultValue interface{}) error {
	// TODO: check defaultValue is not a string containing "(" (function call)
	// TODO: check not NOW(), CURRENT_TIMESTAMP, or arithmetic
	// TODO: check type compatibility: boolean default must be true/false,
	//       int default must be integer, float default must be number,
	//       varchar/text/enum default must be string
	return nil
}

// ValidateUnique checks if the unique modifier is permitted for this field type.
func ValidateUnique(fieldType string) error {
	// TODO: reject if fieldType == "foreign_key" (uniqueness on FK via composite index instead)
	return nil
}

// ValidateMustBeUniqueWithin validates that composite uniqueness scope field names
// reference real fields in the same entity.
func ValidateMustBeUniqueWithin(fieldNames []string, entityFields []string) error {
	// TODO: for each name in fieldNames, check it exists in entityFields
	// TODO: check fieldNames is non-empty
	return nil
}

