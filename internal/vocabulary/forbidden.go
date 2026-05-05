package vocabulary

import (
	"fmt"
	"strings"
)

// ForbiddenViolation records a detected forbidden pattern in entity YAML.
type ForbiddenViolation struct {
	Pattern     string // regex, embedded_logic, inheritance, templating, import, deletion, type_change, rename
	Location    string // YAML path where violation found (e.g. "fields[2].default")
	Rationale   string // why this is forbidden
	Alternative string // what to do instead
}

// inheritanceKeys are top-level or nested keys that indicate inheritance.
var inheritanceKeys = map[string]bool{
	"extends":       true,
	"inherits":      true,
	"parent_entity": true,
	"base_class":    true,
	"abstract":      true,
	"mixin":         true,
	"trait":         true,
	"mixins":        true,
	"traits":        true,
}

// importKeys are keys that indicate file-level imports.
var importKeys = map[string]bool{
	"import":  true,
	"imports": false, // "imports" at top level in directory.yaml is allowed, but not in entity files
	"include": true,
	"require": true,
	"$ref":    true,
	"source":  true,
}

// deletionKeys are keys that indicate field or entity deletion intent.
var deletionKeys = map[string]bool{
	"deleted":      true,
	"removed":      true,
	"drop":         true,
	"remove_field": true,
	"drop_field":   true,
	"delete_field": true,
	"drop_entity":  true,
}

// typeChangeKeys are keys that indicate type migration intent.
var typeChangeKeys = map[string]bool{
	"migrate_type": true,
	"change_type":  true,
	"convert_to":   true,
	"cast_to":      true,
	"type_change":  true,
}

// renameKeys are keys that indicate rename intent.
var renameKeys = map[string]bool{
	"rename_to":     true,
	"renamed_from":  true,
	"alias":         true,
	"previous_name": true,
	"old_name":      true,
}

// embeddedLogicFunctions are SQL or programming functions forbidden in defaults.
var embeddedLogicFunctions = []string{
	"NOW(", "now(",
	"CURRENT_TIMESTAMP", "current_timestamp",
	"CURRENT_DATE", "current_date",
	"CURRENT_TIME", "current_time",
	"COALESCE(", "coalesce(",
	"NULLIF(", "nullif(",
	"GREATEST(", "greatest(",
	"LEAST(", "least(",
	"CONCAT(", "concat(",
	"UPPER(", "upper(",
	"LOWER(", "lower(",
	"TRIM(", "trim(",
	"SUBSTRING(", "substring(",
	"REPLACE(", "replace(",
	"UUID_GENERATE", "uuid_generate",
	"GEN_RANDOM_UUID", "gen_random_uuid",
	"NEXTVAL(", "nextval(",
	"SETVAL(", "setval(",
	"RANDOM(", "random(",
}

// regexMetachars are characters that indicate regex syntax when found in
// constraint values (not in descriptions or free-form text).
var regexMetachars = []string{
	"^[", "[a-z]", "[A-Z]", "[0-9]",
	"]+$", "]+", ".*", ".+",
	"\\d", "\\w", "\\s", "\\b",
	"(?:", "(?=", "(?!",
}

// ScanForForbiddenPatterns scans the raw parsed YAML map recursively
// for forbidden keys and value patterns. Returns all violations found.
func ScanForForbiddenPatterns(rawYAML map[string]interface{}) []ForbiddenViolation {
	var violations []ForbiddenViolation

	// Check top-level keys for forbidden patterns.
	topKeys := collectKeys(rawYAML)

	if CheckForInheritance(topKeys) {
		for _, key := range topKeys {
			if inheritanceKeys[key] {
				violations = append(violations, ForbiddenViolation{
					Pattern:     "inheritance",
					Location:    key,
					Rationale:   "no inheritance, extends, or base class — each entity declares its fields independently",
					Alternative: "declare fields on each entity independently; reserved fields are the only controlled exception",
				})
			}
		}
	}

	if CheckForImports(topKeys) {
		for _, key := range topKeys {
			if importKeys[key] {
				violations = append(violations, ForbiddenViolation{
					Pattern:     "import",
					Location:    key,
					Rationale:   "entity files do not import other files — only directory.yaml controls loading",
					Alternative: "use directory.yaml to control entity loading order; shared structure uses reserved field injection",
				})
			}
		}
	}

	if CheckForDeletionMarkers(topKeys) {
		for _, key := range topKeys {
			if deletionKeys[key] {
				violations = append(violations, ForbiddenViolation{
					Pattern:     "deletion",
					Location:    key,
					Rationale:   "fields and entities cannot be deleted — deletion breaks history, version rows, and audit log",
					Alternative: "deprecate: mark _schema_version_deprecated_id; column and data remain as tombstone",
				})
			}
		}
	}

	if CheckForTypeChangeMarkers(topKeys) {
		for _, key := range topKeys {
			if typeChangeKeys[key] {
				violations = append(violations, ForbiddenViolation{
					Pattern:     "type_change",
					Location:    key,
					Rationale:   "field types cannot be changed — type changes break consumers and stored data",
					Alternative: "use duplication pattern: add new field with new type, double-write, migrate readers, deprecate old",
				})
			}
		}
	}

	if CheckForRenameMarkers(topKeys) {
		for _, key := range topKeys {
			if renameKeys[key] {
				violations = append(violations, ForbiddenViolation{
					Pattern:     "rename",
					Location:    key,
					Rationale:   "fields and entities cannot be renamed — renames break every consumer",
					Alternative: "add new field with new name, deprecate old; both coexist",
				})
			}
		}
	}

	// Scan fields for value-level forbidden patterns.
	if fieldsRaw, ok := rawYAML["fields"]; ok {
		if fieldsList, ok := fieldsRaw.([]interface{}); ok {
			for i, fieldRaw := range fieldsList {
				if fieldMap, ok := fieldRaw.(map[string]interface{}); ok {
					fieldViolations := scanFieldMap(fieldMap, i)
					violations = append(violations, fieldViolations...)
				}
			}
		}
	}

	// Deep scan all values for templating syntax.
	templateViolations := scanValuesRecursive(rawYAML, "")
	violations = append(violations, templateViolations...)

	return violations
}

// scanFieldMap checks a single field's map for forbidden patterns.
func scanFieldMap(fieldMap map[string]interface{}, fieldIndex int) []ForbiddenViolation {
	var violations []ForbiddenViolation

	fieldName := ""
	if name, ok := fieldMap["name"].(string); ok {
		fieldName = name
	}
	location := func(key string) string {
		if fieldName != "" {
			return fmt.Sprintf("fields[%d] (%s).%s", fieldIndex, fieldName, key)
		}
		return fmt.Sprintf("fields[%d].%s", fieldIndex, key)
	}

	// Check field-level keys for forbidden markers.
	fieldKeys := collectKeys(fieldMap)
	for _, key := range fieldKeys {
		if deletionKeys[key] {
			violations = append(violations, ForbiddenViolation{
				Pattern:     "deletion",
				Location:    location(key),
				Rationale:   "fields cannot be deleted",
				Alternative: "deprecate the field",
			})
		}
		if typeChangeKeys[key] {
			violations = append(violations, ForbiddenViolation{
				Pattern:     "type_change",
				Location:    location(key),
				Rationale:   "field types cannot be changed",
				Alternative: "use duplication pattern",
			})
		}
		if renameKeys[key] {
			violations = append(violations, ForbiddenViolation{
				Pattern:     "rename",
				Location:    location(key),
				Rationale:   "fields cannot be renamed",
				Alternative: "add new field with new name, deprecate old",
			})
		}
	}

	// Check default value for embedded logic.
	if defaultVal, ok := fieldMap["default"]; ok {
		if CheckForEmbeddedLogic(defaultVal) {
			violations = append(violations, ForbiddenViolation{
				Pattern:     "embedded_logic",
				Location:    location("default"),
				Rationale:   "default values must be literals — no function calls, expressions, or computed values",
				Alternative: "use a literal default value; computed defaults belong in the API or runner logic",
			})
		}
	}

	// Check constraint values for regex patterns.
	constraintKeys := []string{"pattern", "format", "regex", "match", "validation"}
	for _, key := range constraintKeys {
		if val, ok := fieldMap[key]; ok {
			if strVal, ok := val.(string); ok {
				if CheckForRegex(strVal) {
					violations = append(violations, ForbiddenViolation{
						Pattern:     "regex",
						Location:    location(key),
						Rationale:   "regex is forbidden — DoS vector, dialect variation, unpredictable edge cases",
						Alternative: "use enum sets, length bounds, or prefix/suffix matching; richer validation at API semantic-validation step",
					})
				}
			}
		}
	}

	// The presence of a "pattern" key itself is a strong signal.
	if _, ok := fieldMap["pattern"]; ok {
		violations = append(violations, ForbiddenViolation{
			Pattern:     "regex",
			Location:    location("pattern"),
			Rationale:   "the 'pattern' key implies regex validation, which is forbidden",
			Alternative: "use enum sets, length bounds, or prefix/suffix matching",
		})
	}

	return violations
}

// scanValuesRecursive walks all values in a YAML map looking for
// templating syntax in string values.
func scanValuesRecursive(m map[string]interface{}, path string) []ForbiddenViolation {
	var violations []ForbiddenViolation

	for key, val := range m {
		currentPath := key
		if path != "" {
			currentPath = path + "." + key
		}

		switch v := val.(type) {
		case string:
			if CheckForTemplating(v) {
				violations = append(violations, ForbiddenViolation{
					Pattern:     "templating",
					Location:    currentPath,
					Rationale:   "template syntax is forbidden in schema files — no variable substitution or expressions",
					Alternative: "variation across environments via OpsDB runtime config, not different schemas",
				})
			}
		case map[string]interface{}:
			nested := scanValuesRecursive(v, currentPath)
			violations = append(violations, nested...)
		case []interface{}:
			for i, item := range v {
				itemPath := fmt.Sprintf("%s[%d]", currentPath, i)
				if strItem, ok := item.(string); ok {
					if CheckForTemplating(strItem) {
						violations = append(violations, ForbiddenViolation{
							Pattern:     "templating",
							Location:    itemPath,
							Rationale:   "template syntax is forbidden in schema files",
							Alternative: "variation across environments via OpsDB runtime config",
						})
					}
				}
				if mapItem, ok := item.(map[string]interface{}); ok {
					nested := scanValuesRecursive(mapItem, itemPath)
					violations = append(violations, nested...)
				}
			}
		}
	}

	return violations
}

// CheckForRegex checks if a string value contains regex metacharacter
// patterns used as constraints.
func CheckForRegex(value string) bool {
	for _, pattern := range regexMetachars {
		if strings.Contains(value, pattern) {
			return true
		}
	}

	// Check for common regex delimiters: starts with ^ and ends with $
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "^") && strings.HasSuffix(trimmed, "$") {
		return true
	}

	// Check for character class syntax with quantifiers.
	if strings.Contains(value, "[") && strings.Contains(value, "]") {
		afterBracket := value[strings.Index(value, "]")+1:]
		if strings.HasPrefix(afterBracket, "+") ||
			strings.HasPrefix(afterBracket, "*") ||
			strings.HasPrefix(afterBracket, "{") {
			return true
		}
	}

	return false
}

// CheckForEmbeddedLogic checks for function calls, arithmetic operators,
// SQL functions, and computed values in a YAML value.
func CheckForEmbeddedLogic(value interface{}) bool {
	strVal, ok := value.(string)
	if !ok {
		return false
	}

	// Check for known SQL and programming functions.
	for _, fn := range embeddedLogicFunctions {
		if strings.Contains(strVal, fn) {
			return true
		}
	}

	// Check for arithmetic expressions in default-like contexts.
	// Look for patterns like "previous_value + 1", "field * 2", etc.
	arithmeticOps := []string{" + ", " - ", " * ", " / ", " % "}
	for _, op := range arithmeticOps {
		if strings.Contains(strVal, op) {
			return true
		}
	}

	// Check for explicit expression markers.
	if strings.HasPrefix(strVal, "=") {
		return true
	}

	return false
}

// CheckForInheritance checks for inheritance-related keys in a key list.
func CheckForInheritance(keys []string) bool {
	for _, key := range keys {
		if inheritanceKeys[key] {
			return true
		}
	}
	return false
}

// CheckForTemplating checks for template syntax in a string value.
func CheckForTemplating(value string) bool {
	templateMarkers := []string{
		"{{", "}}",
		"{%", "%}",
		"${",
		"$(",
	}
	for _, marker := range templateMarkers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

// CheckForImports checks for import-related keys in a key list.
// Note: "imports" is only forbidden in entity files, not in directory.yaml.
func CheckForImports(keys []string) bool {
	for _, key := range keys {
		if importKeys[key] {
			return true
		}
	}
	return false
}

// CheckForDeletionMarkers checks for field/entity deletion marker keys.
func CheckForDeletionMarkers(keys []string) bool {
	for _, key := range keys {
		if deletionKeys[key] {
			return true
		}
	}
	return false
}

// CheckForTypeChangeMarkers checks for type change marker keys.
func CheckForTypeChangeMarkers(keys []string) bool {
	for _, key := range keys {
		if typeChangeKeys[key] {
			return true
		}
	}
	return false
}

// CheckForRenameMarkers checks for rename marker keys.
func CheckForRenameMarkers(keys []string) bool {
	for _, key := range keys {
		if renameKeys[key] {
			return true
		}
	}
	return false
}

// collectKeys extracts all keys from a map as a string slice.
func collectKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
