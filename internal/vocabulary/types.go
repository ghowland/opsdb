package vocabulary

import (
	"fmt"
)

// allowedTypeSet is the lookup set for the nine permitted field types.
var allowedTypeSet = map[string]bool{
	"int":         true,
	"float":       true,
	"varchar":     true,
	"text":        true,
	"boolean":     true,
	"datetime":    true,
	"date":        true,
	"json":        true,
	"enum":        true,
	"foreign_key": true,
}

// AllAllowedTypes returns the complete list of allowed field types.
func AllAllowedTypes() []string {
	return []string{
		"int", "float", "varchar", "text", "boolean",
		"datetime", "date", "json", "enum", "foreign_key",
	}
}

// IsValidType checks if typeName is one of the allowed field types.
func IsValidType(typeName string) bool {
	return allowedTypeSet[typeName]
}

// postgresTypeMap maps schema types to their Postgres DDL type strings.
// VARCHAR is handled separately because it requires max_length.
var postgresTypeMap = map[string]string{
	"int":         "INTEGER",
	"float":       "DOUBLE PRECISION",
	"text":        "TEXT",
	"boolean":     "BOOLEAN",
	"datetime":    "TIMESTAMP WITHOUT TIME ZONE",
	"date":        "DATE",
	"json":        "JSONB",
	"enum":        "VARCHAR(255)",
	"foreign_key": "INTEGER",
}

// GetPostgresType returns the Postgres DDL type string for a schema type.
// VARCHAR requires max_length from the constraints map. All others are
// fixed mappings. Returns an error-indicating string if type is unknown.
func GetPostgresType(typeName string, constraints map[string]interface{}) string {
	if typeName == "varchar" {
		maxLen := 255 // fallback, though validator should have caught missing max_length
		if ml, ok := constraints["max_length"]; ok {
			if mlInt, err := toInt(ml); err == nil {
				maxLen = mlInt
			}
		}
		return fmt.Sprintf("VARCHAR(%d)", maxLen)
	}

	if pgType, ok := postgresTypeMap[typeName]; ok {
		return pgType
	}

	return fmt.Sprintf("UNKNOWN_TYPE(%s)", typeName)
}

// typeAllowedConstraints maps field types to their optional constraints.
// Required constraints (max_length for varchar, enum_values for enum, etc.)
// are listed in typeRequiredConstraints.
var typeAllowedConstraints = map[string][]string{
	"int":         {"min_value", "max_value"},
	"float":       {"min_value", "max_value", "precision_decimal_places"},
	"varchar":     {"min_length", "max_length"},
	"text":        {"max_length"},
	"boolean":     {},
	"datetime":    {},
	"date":        {},
	"json":        {"json_type_discriminator"},
	"enum":        {"enum_values"},
	"foreign_key": {"references"},
}

// typeRequiredConstraints maps field types to constraints that must be present.
var typeRequiredConstraints = map[string][]string{
	"int":         {},
	"float":       {},
	"varchar":     {"max_length"},
	"text":        {},
	"boolean":     {},
	"datetime":    {},
	"date":        {},
	"json":        {"json_type_discriminator"},
	"enum":        {"enum_values"},
	"foreign_key": {"references"},
}

// GetAllowedConstraints returns which constraints are permitted for this type.
// Includes both optional and required constraints.
func GetAllowedConstraints(typeName string) []string {
	if constraints, ok := typeAllowedConstraints[typeName]; ok {
		return constraints
	}
	return nil
}

// GetRequiredConstraints returns which constraints must be present for this type.
func GetRequiredConstraints(typeName string) []string {
	if constraints, ok := typeRequiredConstraints[typeName]; ok {
		return constraints
	}
	return nil
}

// typeAllowedModifiers maps field types to their permitted modifiers.
var typeAllowedModifiers = map[string][]string{
	"int":         {"nullable", "default", "unique", "must_be_unique_within"},
	"float":       {"nullable", "default", "unique", "must_be_unique_within"},
	"varchar":     {"nullable", "default", "unique", "must_be_unique_within"},
	"text":        {"nullable", "default"},
	"boolean":     {"nullable", "default"},
	"datetime":    {"nullable"},
	"date":        {"nullable"},
	"json":        {"nullable"},
	"enum":        {"nullable", "default", "unique", "must_be_unique_within"},
	"foreign_key": {"nullable"},
}

// GetAllowedModifiers returns which modifiers are permitted for this type.
func GetAllowedModifiers(typeName string) []string {
	if modifiers, ok := typeAllowedModifiers[typeName]; ok {
		return modifiers
	}
	return nil
}

// typeForbiddenModifiers maps field types to modifiers that are explicitly
// not allowed. This is the inverse of allowed — used for clearer error messages.
var typeForbiddenModifiers = map[string][]string{
	"int":         {},
	"float":       {},
	"varchar":     {},
	"text":        {"unique", "must_be_unique_within"},
	"boolean":     {"unique", "must_be_unique_within"},
	"datetime":    {"default", "unique", "must_be_unique_within"},
	"date":        {"default", "unique", "must_be_unique_within"},
	"json":        {"default", "unique", "must_be_unique_within"},
	"enum":        {},
	"foreign_key": {"default", "unique", "must_be_unique_within"},
}

// GetForbiddenModifiers returns which modifiers are explicitly forbidden
// for this type.
func GetForbiddenModifiers(typeName string) []string {
	if modifiers, ok := typeForbiddenModifiers[typeName]; ok {
		return modifiers
	}
	return nil
}

// IsModifierAllowed checks whether a specific modifier is allowed on a
// specific field type.
func IsModifierAllowed(typeName string, modifierName string) bool {
	allowed := GetAllowedModifiers(typeName)
	for _, m := range allowed {
		if m == modifierName {
			return true
		}
	}
	return false
}

// IsModifierForbidden checks whether a specific modifier is explicitly
// forbidden on a specific field type.
func IsModifierForbidden(typeName string, modifierName string) bool {
	forbidden := GetForbiddenModifiers(typeName)
	for _, m := range forbidden {
		if m == modifierName {
			return true
		}
	}
	return false
}

// ValidateModifiersForType checks all modifiers present on a field against
// the allowed and forbidden lists for its type. Returns an error describing
// the first invalid modifier found.
func ValidateModifiersForType(typeName string, hasDefault bool, defaultValue interface{}, hasUnique bool, hasMustBeUniqueWithin bool) error {
	if hasDefault {
		if IsModifierForbidden(typeName, "default") {
			return fmt.Errorf("default modifier is not allowed on %s fields", typeName)
		}
		if !IsModifierAllowed(typeName, "default") {
			return fmt.Errorf("default modifier is not allowed on %s fields", typeName)
		}
		if defaultValue != nil {
			if err := ValidateDefault(typeName, defaultValue); err != nil {
				return err
			}
		}
	}

	if hasUnique {
		if IsModifierForbidden(typeName, "unique") {
			return fmt.Errorf("unique modifier is not allowed on %s fields", typeName)
		}
		if !IsModifierAllowed(typeName, "unique") {
			return fmt.Errorf("unique modifier is not allowed on %s fields", typeName)
		}
		if err := ValidateUnique(typeName); err != nil {
			return err
		}
	}

	if hasMustBeUniqueWithin {
		if IsModifierForbidden(typeName, "must_be_unique_within") {
			return fmt.Errorf("must_be_unique_within modifier is not allowed on %s fields", typeName)
		}
		if !IsModifierAllowed(typeName, "must_be_unique_within") {
			return fmt.Errorf("must_be_unique_within modifier is not allowed on %s fields", typeName)
		}
	}

	return nil
}
