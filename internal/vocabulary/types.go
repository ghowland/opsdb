// === internal/vocabulary/types.go ===
package vocabulary

// The nine allowed field types.
var allowedTypes = []string{
	"int", "float", "varchar", "text", "boolean",
	"datetime", "json", "enum", "foreign_key",
}

// IsValidType checks if typeName is one of the nine allowed types.
func IsValidType(typeName string) bool {
	// TODO: lookup in allowedTypes slice
	return false
}

// GetPostgresType returns the Postgres DDL type string for a schema type.
// VARCHAR requires max_length from constraints. Others are fixed mappings.
func GetPostgresType(typeName string, constraints map[string]interface{}) string {
	// TODO: switch on typeName
	// int → "INTEGER"
	// float → "DOUBLE PRECISION"
	// varchar → "VARCHAR({max_length})" from constraints
	// text → "TEXT"
	// boolean → "BOOLEAN"
	// datetime → "TIMESTAMP WITHOUT TIME ZONE"
	// json → "JSONB"
	// enum → "VARCHAR(255)"
	// foreign_key → "INTEGER"
	return ""
}

// GetAllowedConstraints returns which constraints are permitted for this type.
func GetAllowedConstraints(typeName string) []string {
	// TODO: int → [min_value, max_value]
	// float → [min_value, max_value, precision_decimal_places]
	// varchar → [min_length] (max_length is required, handled separately)
	// text → [max_length]
	// boolean → []
	// datetime → []
	// json → [] (json_type_discriminator is required, handled separately)
	// enum → [] (enum_values is required, handled separately)
	// foreign_key → [] (references is required, handled separately)
	return nil
}

// GetRequiredConstraints returns which constraints must be present for this type.
func GetRequiredConstraints(typeName string) []string {
	// TODO: varchar → [max_length]
	// json → [json_type_discriminator]
	// enum → [enum_values]
	// foreign_key → [references]
	// all others → []
	return nil
}

// GetAllowedModifiers returns which modifiers are permitted for this type.
func GetAllowedModifiers(typeName string) []string {
	// TODO: int → [nullable, default, unique, must_be_unique_within]
	// float → [nullable, default, unique, must_be_unique_within]
	// varchar → [nullable, default, unique, must_be_unique_within]
	// text → [nullable, default, unique]
	// boolean → [nullable, default]
	// datetime → [nullable]
	// json → [nullable]
	// enum → [nullable, default, unique, must_be_unique_within]
	// foreign_key → [nullable]
	return nil
}

// GetForbiddenModifiers returns which modifiers are explicitly forbidden for this type.
func GetForbiddenModifiers(typeName string) []string {
	// TODO: datetime → [default] (computed defaults handled by engine for reserved fields)
	// json → [default] (no sensible default for typed payloads)
	// foreign_key → [default, unique] (referential integrity explicit, uniqueness via index)
	// all others → []
	return nil
}

