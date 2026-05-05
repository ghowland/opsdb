
// === internal/vocabulary/forbidden.go ===
package vocabulary

// ForbiddenViolation records a detected forbidden pattern.
type ForbiddenViolation struct {
	Pattern     string // regex, embedded_logic, inheritance, etc.
	Location    string // YAML path where violation found
	Rationale   string // why this is forbidden
	Alternative string // what to do instead
}

// ScanForForbiddenPatterns scans the raw parsed YAML map recursively
// for forbidden keys and value patterns.
func ScanForForbiddenPatterns(rawYAML map[string]interface{}) []ForbiddenViolation {
	// TODO: walk all keys and values recursively
	// TODO: call each Check* function at each node
	// TODO: accumulate violations
	return nil
}

// CheckForRegex checks if a string value contains regex metacharacters
// used as constraints (not as data).
func CheckForRegex(value string) bool {
	// TODO: check for common regex metacharacters in constraint context:
	//       ^, $, *, +, ?, {, }, [, ], |, \ used as pattern syntax
	// TODO: distinguish from legitimate data values containing these chars
	return false
}

// CheckForEmbeddedLogic checks for function calls, arithmetic operators,
// NOW(), CURRENT_TIMESTAMP, computed values in a YAML value.
func CheckForEmbeddedLogic(value interface{}) bool {
	// TODO: check string values for:
	//       parentheses as function calls: "NOW()", "COALESCE(", etc.
	//       arithmetic: "previous_value + 1", "field_a * 2"
	//       known SQL functions: CURRENT_TIMESTAMP, CURRENT_DATE
	// TODO: only applies to default values and constraint values, not descriptions
	return false
}

// CheckForInheritance checks for inheritance-related keys.
func CheckForInheritance(keys []string) bool {
	// TODO: check for: extends, inherits, parent_entity, base_class,
	//       abstract, mixin, trait
	return false
}

// CheckForTemplating checks for template syntax in a string value.
func CheckForTemplating(value string) bool {
	// TODO: check for: {{, }}, {%, %}, ${, $(
	return false
}

// CheckForImports checks for import-related keys.
func CheckForImports(keys []string) bool {
	// TODO: check for: import, include, require, $ref (in schema context),
	//       source (for file inclusion)
	return false
}

// CheckForDeletionMarkers checks for field/entity deletion marker keys.
func CheckForDeletionMarkers(keys []string) bool {
	// TODO: check for: deleted, removed, drop, remove_field
	return false
}

// CheckForTypeChangeMarkers checks for type change marker keys.
func CheckForTypeChangeMarkers(keys []string) bool {
	// TODO: check for: migrate_type, change_type, convert_to, cast_to
	return false
}

// CheckForRenameMarkers checks for rename marker keys.
func CheckForRenameMarkers(keys []string) bool {
	// TODO: check for: rename_to, renamed_from, alias, previous_name
	return false
}

