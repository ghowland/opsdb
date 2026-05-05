package vocabulary

import (
	"fmt"
	"strings"
)

// validModifiers is the set of recognized modifier names.
var validModifiers = map[string]bool{
	"nullable":              true,
	"default":               true,
	"unique":                true,
	"must_be_unique_within": true,
}

// forbiddenDefaultTypes lists field types that cannot have default values.
var forbiddenDefaultTypes = map[string]bool{
	"foreign_key": true,
	"datetime":    true,
	"json":        true,
}

// forbiddenUniqueTypes lists field types where the unique modifier is not allowed.
var forbiddenUniqueTypes = map[string]bool{
	"text": true,
	"json": true,
}

// IsValidModifier checks if modName is one of the recognized modifiers.
func IsValidModifier(modName string) bool {
	return validModifiers[modName]
}

// ValidateDefault validates that a default value is a literal appropriate
// for the field type. Rejects expressions, function calls, computed values,
// and type mismatches.
func ValidateDefault(fieldType string, defaultValue interface{}) error {
	if defaultValue == nil {
		return nil
	}

	// Certain types cannot have defaults at all.
	if forbiddenDefaultTypes[fieldType] {
		return fmt.Errorf("default values are not allowed on %s fields", fieldType)
	}

	// Check for embedded logic in string defaults.
	if strVal, ok := defaultValue.(string); ok {
		if CheckForEmbeddedLogic(strVal) {
			return fmt.Errorf("default value %q contains embedded logic (function call or expression); defaults must be literals", strVal)
		}
		if CheckForTemplating(strVal) {
			return fmt.Errorf("default value %q contains template syntax; defaults must be literals", strVal)
		}
	}

	// Type-specific validation.
	switch fieldType {
	case "boolean":
		if err := validateBooleanDefault(defaultValue); err != nil {
			return err
		}
	case "int":
		if err := validateIntDefault(defaultValue); err != nil {
			return err
		}
	case "float":
		if err := validateFloatDefault(defaultValue); err != nil {
			return err
		}
	case "varchar", "text":
		if err := validateStringDefault(defaultValue); err != nil {
			return err
		}
	case "enum":
		if err := validateStringDefault(defaultValue); err != nil {
			return err
		}
	case "date":
		if err := validateStringDefault(defaultValue); err != nil {
			return err
		}
		// Date defaults must look like a date literal, not a function.
		if strVal, ok := defaultValue.(string); ok {
			if strings.Contains(strVal, "(") {
				return fmt.Errorf("default value %q for date field looks like a function call; use a literal date", strVal)
			}
		}
	}

	return nil
}

// ValidateUnique checks if the unique modifier is permitted for this field type.
func ValidateUnique(fieldType string) error {
	if forbiddenUniqueTypes[fieldType] {
		return fmt.Errorf("unique modifier is not allowed on %s fields (use composite index for text, not applicable for json)", fieldType)
	}
	return nil
}

// ValidateMustBeUniqueWithin validates that composite uniqueness scope field
// names reference real fields in the same entity. The field list must be
// non-empty and every name must exist.
func ValidateMustBeUniqueWithin(fieldNames []string, entityFields []string) error {
	if len(fieldNames) == 0 {
		return fmt.Errorf("must_be_unique_within is empty; provide at least one field name")
	}

	// Build lookup set of entity field names.
	fieldSet := make(map[string]bool, len(entityFields))
	for _, f := range entityFields {
		fieldSet[f] = true
	}

	// Check each referenced field exists.
	for _, name := range fieldNames {
		if name == "" {
			return fmt.Errorf("must_be_unique_within contains empty field name")
		}
		if !fieldSet[name] {
			return fmt.Errorf("must_be_unique_within references field %q which does not exist on this entity", name)
		}
	}

	// Check for duplicates in the scope list.
	seen := make(map[string]bool, len(fieldNames))
	for _, name := range fieldNames {
		if seen[name] {
			return fmt.Errorf("must_be_unique_within contains duplicate field %q", name)
		}
		seen[name] = true
	}

	return nil
}

// validateBooleanDefault checks that a boolean field's default is true or false.
func validateBooleanDefault(value interface{}) error {
	switch v := value.(type) {
	case bool:
		return nil
	case string:
		lower := strings.ToLower(v)
		if lower == "true" || lower == "false" {
			return nil
		}
		return fmt.Errorf("boolean default must be true or false, got string %q", v)
	default:
		return fmt.Errorf("boolean default must be true or false, got %T", value)
	}
}

// validateIntDefault checks that an int field's default is a whole number.
func validateIntDefault(value interface{}) error {
	switch v := value.(type) {
	case int:
		return nil
	case int32:
		return nil
	case int64:
		return nil
	case float64:
		if v != float64(int64(v)) {
			return fmt.Errorf("int default must be a whole number, got %v", v)
		}
		return nil
	case float32:
		if v != float32(int32(v)) {
			return fmt.Errorf("int default must be a whole number, got %v", v)
		}
		return nil
	case string:
		return fmt.Errorf("int default must be a number, got string %q (use a numeric literal, not a string)", v)
	default:
		return fmt.Errorf("int default must be a number, got %T", value)
	}
}

// validateFloatDefault checks that a float field's default is numeric.
func validateFloatDefault(value interface{}) error {
	switch value.(type) {
	case float64, float32, int, int32, int64:
		return nil
	case string:
		return fmt.Errorf("float default must be a number, got string %q (use a numeric literal, not a string)", value)
	default:
		return fmt.Errorf("float default must be a number, got %T", value)
	}
}

// validateStringDefault checks that a string field's default is a string literal.
func validateStringDefault(value interface{}) error {
	switch value.(type) {
	case string:
		return nil
	default:
		return fmt.Errorf("default for string-typed field must be a string, got %T", value)
	}
}
