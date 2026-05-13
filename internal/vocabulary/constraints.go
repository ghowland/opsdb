//# internal/vocabulary/constraints.go

package vocabulary

import (
	"fmt"
	"strings"
	"unicode"
)

// ValidateConstraints is the master constraint validator for a field.
// Checks that only allowed constraints for the type are present,
// required constraints are present, and values are valid.
// Uses typeAllowedConstraints and typeRequiredConstraints from types.go
// as the single source of truth.
func ValidateConstraints(fieldType string, constraints map[string]interface{}) error {
	allowed := GetAllowedConstraints(fieldType)
	if allowed == nil {
		return fmt.Errorf("unknown field type %q: cannot validate constraints", fieldType)
	}

	allowedSet := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = true
	}

	for key := range constraints {
		if !allowedSet[key] {
			return fmt.Errorf("constraint %q is not allowed on field type %q (allowed: %s)",
				key, fieldType, strings.Join(allowed, ", "))
		}
	}

	required := GetRequiredConstraints(fieldType)
	for _, key := range required {
		if _, present := constraints[key]; !present {
			return fmt.Errorf("constraint %q is required for field type %q", key, fieldType)
		}
	}

	// Dispatch to per-constraint validators.
	if fieldType == "int" || fieldType == "float" {
		minVal, hasMin := constraints["min_value"]
		maxVal, hasMax := constraints["max_value"]
		if hasMin && hasMax {
			if err := ValidateNumericRange(minVal, maxVal); err != nil {
				return err
			}
		}
		if hasMin {
			if _, err := toFloat64(minVal); err != nil {
				return fmt.Errorf("min_value: %w", err)
			}
		}
		if hasMax {
			if _, err := toFloat64(maxVal); err != nil {
				return fmt.Errorf("max_value: %w", err)
			}
		}
	}

	if fieldType == "float" {
		if places, ok := constraints["precision_decimal_places"]; ok {
			if err := ValidatePrecision(places); err != nil {
				return err
			}
		}
	}

	if fieldType == "varchar" || fieldType == "text" {
		maxLen, hasMax := constraints["max_length"]
		minLen, hasMin := constraints["min_length"]
		if hasMax {
			if err := validatePositiveInt(maxLen, "max_length"); err != nil {
				return err
			}
		}
		if hasMin {
			if err := validateNonNegativeInt(minLen, "min_length"); err != nil {
				return err
			}
		}
		if hasMin && hasMax {
			if err := ValidateStringLength(minLen, maxLen); err != nil {
				return err
			}
		}
	}

	if fieldType == "enum" {
		if rawValues, ok := constraints["enum_values"]; ok {
			values, err := toStringSlice(rawValues)
			if err != nil {
				return fmt.Errorf("enum_values: %w", err)
			}
			if err := ValidateEnumValues(values); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateNumericRange checks that min_value <= max_value.
func ValidateNumericRange(minValue, maxValue interface{}) error {
	minF, err := toFloat64(minValue)
	if err != nil {
		return fmt.Errorf("min_value is not numeric: %w", err)
	}
	maxF, err := toFloat64(maxValue)
	if err != nil {
		return fmt.Errorf("max_value is not numeric: %w", err)
	}
	if minF > maxF {
		return fmt.Errorf("min_value (%v) is greater than max_value (%v)", minValue, maxValue)
	}
	return nil
}

// ValidateStringLength checks that min_length <= max_length and max_length >= 1.
func ValidateStringLength(minLength, maxLength interface{}) error {
	minI, err := toInt(minLength)
	if err != nil {
		return fmt.Errorf("min_length is not an integer: %w", err)
	}
	maxI, err := toInt(maxLength)
	if err != nil {
		return fmt.Errorf("max_length is not an integer: %w", err)
	}
	if maxI < 1 {
		return fmt.Errorf("max_length must be at least 1 (got %d)", maxI)
	}
	if minI < 0 {
		return fmt.Errorf("min_length must be non-negative (got %d)", minI)
	}
	if minI > maxI {
		return fmt.Errorf("min_length (%d) is greater than max_length (%d)", minI, maxI)
	}
	return nil
}

// ValidateEnumValues checks the enum values list: non-empty, max 256 values,
// no duplicates, all lowercase_underscore.
func ValidateEnumValues(values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("enum_values must not be empty")
	}
	if len(values) > 256 {
		return fmt.Errorf("enum_values has %d entries (max 256)", len(values))
	}

	seen := make(map[string]bool, len(values))
	for i, v := range values {
		if v == "" {
			return fmt.Errorf("enum_values[%d] is empty string", i)
		}
		if seen[v] {
			return fmt.Errorf("enum_values contains duplicate: %q", v)
		}
		seen[v] = true

		if err := checkEnumValueFormat(v); err != nil {
			return fmt.Errorf("enum_values[%d] %q: %w", i, v, err)
		}
	}
	return nil
}

// ValidateReferences checks that the referenced entity exists in the known set.
func ValidateReferences(targetEntity string, knownEntities map[string]bool) error {
	if targetEntity == "" {
		return fmt.Errorf("foreign_key references is empty")
	}
	if !knownEntities[targetEntity] {
		return fmt.Errorf("referenced entity %q not found or not yet loaded (check directory.yaml order)", targetEntity)
	}
	return nil
}

// ValidateJsonDiscriminator checks that the discriminator field exists in
// the same entity and is an enum type. entityFieldNames maps field names
// to their types.
func ValidateJsonDiscriminator(discriminatorField string, entityFieldNames map[string]string) error {
	if discriminatorField == "" {
		return fmt.Errorf("json_type_discriminator is empty")
	}
	fieldType, exists := entityFieldNames[discriminatorField]
	if !exists {
		return fmt.Errorf("json_type_discriminator references field %q which does not exist on this entity", discriminatorField)
	}
	if fieldType != "enum" {
		return fmt.Errorf("json_type_discriminator field %q has type %q but must be enum", discriminatorField, fieldType)
	}
	return nil
}

// ValidatePrecision checks that precision_decimal_places is in the 0-15 range.
func ValidatePrecision(places interface{}) error {
	p, err := toInt(places)
	if err != nil {
		return fmt.Errorf("precision_decimal_places is not an integer: %w", err)
	}
	if p < 0 || p > 15 {
		return fmt.Errorf("precision_decimal_places must be 0-15 (got %d)", p)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Type conversion helpers — used by constraints and by other packages
// that validate values from parsed YAML
// ---------------------------------------------------------------------------

func toFloat64(v interface{}) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case int32:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func toInt(v interface{}) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case int32:
		return int(n), nil
	case float64:
		if n != float64(int(n)) {
			return 0, fmt.Errorf("value %v is not a whole number", n)
		}
		return int(n), nil
	case float32:
		if n != float32(int(n)) {
			return 0, fmt.Errorf("value %v is not a whole number", n)
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func toStringSlice(v interface{}) ([]string, error) {
	switch s := v.(type) {
	case []string:
		return s, nil
	case []interface{}:
		result := make([]string, len(s))
		for i, item := range s {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("element %d is %T, not string", i, item)
			}
			result[i] = str
		}
		return result, nil
	default:
		return nil, fmt.Errorf("expected string list, got %T", v)
	}
}

func checkEnumValueFormat(v string) error {
	for i, r := range v {
		if r == '_' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if unicode.IsUpper(r) {
			return fmt.Errorf("contains uppercase character at position %d (use lowercase)", i)
		}
		return fmt.Errorf("contains invalid character %q at position %d (only lowercase, digits, underscore)", string(r), i)
	}
	return nil
}

func validatePositiveInt(v interface{}, name string) error {
	i, err := toInt(v)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if i < 1 {
		return fmt.Errorf("%s must be at least 1 (got %d)", name, i)
	}
	return nil
}

func validateNonNegativeInt(v interface{}, name string) error {
	i, err := toInt(v)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if i < 0 {
		return fmt.Errorf("%s must be non-negative (got %d)", name, i)
	}
	return nil
}
