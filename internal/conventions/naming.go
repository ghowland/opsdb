package conventions

import (
	"fmt"
	"strings"
	"unicode"
)

// knownPluralExceptions lists entity names that legitimately end in 's'
// but are singular. Checked before the trailing-s heuristic rejects a name.
var knownPluralExceptions = map[string]bool{
	"dns":          true,
	"kubernetes":   true,
	"k8s":          true,
	"prometheus":   true,
	"atlas":        true,
	"status":       true,
	"alias":        true,
	"chassis":      true,
	"bus":          true,
	"address":      true,
	"access":       true,
	"process":      true,
	"class":        true,
	"loss":         true,
	"pass":         true,
	"analysis":     true,
	"basis":        true,
	"diagnosis":    true,
	"oss":          true,
	"tls":          true,
	"https":        true,
	"cors":         true,
	"metrics":      true, // used as uncountable noun in ops context
	"credentials":  true,
	"contents":     true,
	"headquarters": true,
	"series":       true,
	"species":      true,
}

// knownGovernanceFields lists field names that are allowed to start with underscore.
var knownGovernanceFields = map[string]bool{
	"_requires_group":                true,
	"_access_classification":         true,
	"_audit_chain_hash":              true,
	"_retention_policy_id":           true,
	"_schema_version_introduced_id":  true,
	"_schema_version_deprecated_id":  true,
	"_observed_time":                 true,
	"_authority_id":                  true,
	"_puller_runner_job_id":          true,
}

// ValidateEntityName checks that an entity name follows conventions:
// lowercase_underscore, singular, min 2 chars, max 128 chars,
// no leading underscore unless _schema_ prefix.
func ValidateEntityName(name string) error {
	if len(name) < 2 {
		return fmt.Errorf("entity name %q is too short (min 2 characters)", name)
	}
	if len(name) > 128 {
		return fmt.Errorf("entity name %q is too long (max 128 characters, got %d)", name, len(name))
	}

	if err := checkLowercaseUnderscoreOnly(name); err != nil {
		return fmt.Errorf("entity name %q: %w", name, err)
	}

	if strings.Contains(name, "__") {
		return fmt.Errorf("entity name %q contains double underscore", name)
	}
	if strings.HasSuffix(name, "_") {
		return fmt.Errorf("entity name %q has trailing underscore", name)
	}

	// Leading underscore only allowed for _schema_ prefixed entities.
	if strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "_schema_") {
		return fmt.Errorf("entity name %q has leading underscore but is not a _schema_ entity", name)
	}

	// Singular check: reject trailing 's' unless the final component is a known exception.
	if err := checkSingular(name); err != nil {
		return fmt.Errorf("entity name %q: %w", name, err)
	}

	return nil
}

// ValidateFieldName checks that a field name follows conventions:
// lowercase_underscore, datetime fields end in _time, date fields end in _date,
// present-state booleans start with is_ or was_,
// governance fields start with underscore.
func ValidateFieldName(name string, fieldType string) error {
	if len(name) < 2 {
		return fmt.Errorf("field name %q is too short (min 2 characters)", name)
	}
	if len(name) > 128 {
		return fmt.Errorf("field name %q is too long (max 128 characters, got %d)", name, len(name))
	}

	// Governance fields start with underscore — validate against known set.
	if strings.HasPrefix(name, "_") {
		if !knownGovernanceFields[name] {
			return fmt.Errorf("field name %q starts with underscore but is not a recognized governance field", name)
		}
		// Governance fields skip remaining naming checks since they follow
		// their own conventions defined in the reserved config.
		return nil
	}

	if err := checkLowercaseUnderscoreOnly(name); err != nil {
		return fmt.Errorf("field name %q: %w", name, err)
	}

	if strings.Contains(name, "__") {
		return fmt.Errorf("field name %q contains double underscore", name)
	}
	if strings.HasSuffix(name, "_") {
		return fmt.Errorf("field name %q has trailing underscore", name)
	}

	// Type-specific suffix/prefix checks.
	switch fieldType {
	case "datetime":
		if !strings.HasSuffix(name, "_time") {
			return fmt.Errorf("field name %q has type datetime but does not end with _time", name)
		}
	case "date":
		if !strings.HasSuffix(name, "_date") {
			return fmt.Errorf("field name %q has type date but does not end with _date", name)
		}
	case "boolean":
		if !strings.HasPrefix(name, "is_") && !strings.HasPrefix(name, "was_") {
			return fmt.Errorf("field name %q has type boolean but does not start with is_ or was_", name)
		}
	}

	return nil
}

// ValidateFKName checks that a foreign key field follows {referenced_table}_id
// pattern, allowing role prefixes for disambiguation.
//
// Valid patterns:
//
//	service_id                    (standard: references service)
//	source_service_id             (role prefix "source_" + service_id)
//	parent_location_id            (role prefix "parent_" for self-ref)
//	parent_service_version_id     (role prefix for version chain self-ref)
func ValidateFKName(fieldName string, referencedEntity string) error {
	if !strings.HasSuffix(fieldName, "_id") {
		return fmt.Errorf("FK field %q does not end with _id", fieldName)
	}

	// Standard case: field is exactly {referenced_entity}_id.
	expectedStandard := referencedEntity + "_id"
	if fieldName == expectedStandard {
		return nil
	}

	// Role-prefixed case: field ends with _{referenced_entity}_id
	// and has a non-empty prefix before that.
	expectedSuffix := "_" + referencedEntity + "_id"
	if strings.HasSuffix(fieldName, expectedSuffix) {
		prefix := fieldName[:len(fieldName)-len(expectedSuffix)]
		if len(prefix) > 0 {
			if err := checkLowercaseUnderscoreOnly(prefix); err != nil {
				return fmt.Errorf("FK field %q role prefix: %w", fieldName, err)
			}
			return nil
		}
	}

	// Special case for self-referential version chain:
	// parent_{entity}_version_id references {entity}_version
	if strings.HasPrefix(fieldName, "parent_") && strings.HasSuffix(fieldName, "_id") {
		// Extract what's between "parent_" and "_id"
		inner := fieldName[7 : len(fieldName)-3]
		if inner == referencedEntity {
			return nil
		}
	}

	return fmt.Errorf(
		"FK field %q does not follow naming convention for reference to %q: expected %q or {role_prefix}_%s",
		fieldName, referencedEntity, expectedStandard, expectedStandard,
	)
}

// ValidateCompositeName checks hierarchical prefix naming pattern.
// Composite names use parent_concept_subconcept with no double underscores
// and no trailing underscores.
func ValidateCompositeName(name string) error {
	if len(name) < 2 {
		return fmt.Errorf("composite name %q is too short", name)
	}
	if len(name) > 128 {
		return fmt.Errorf("composite name %q is too long (max 128)", name)
	}

	if err := checkLowercaseUnderscoreOnly(name); err != nil {
		return fmt.Errorf("composite name %q: %w", name, err)
	}
	if strings.Contains(name, "__") {
		return fmt.Errorf("composite name %q contains double underscore", name)
	}
	if strings.HasSuffix(name, "_") {
		return fmt.Errorf("composite name %q has trailing underscore", name)
	}
	if strings.HasPrefix(name, "_") {
		return fmt.Errorf("composite name %q has leading underscore", name)
	}

	// Must contain at least one underscore to be composite.
	if !strings.Contains(name, "_") {
		return fmt.Errorf("composite name %q has no underscore (not composite)", name)
	}

	return nil
}

// checkLowercaseUnderscoreOnly validates that a string contains only
// lowercase ASCII letters, digits, and underscores.
func checkLowercaseUnderscoreOnly(s string) error {
	for i, r := range s {
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
			return fmt.Errorf("contains uppercase character %q at position %d (use lowercase_underscore)", string(r), i)
		}
		if r == '-' {
			return fmt.Errorf("contains hyphen at position %d (use underscore, not hyphen)", i)
		}
		if r == ' ' {
			return fmt.Errorf("contains space at position %d", i)
		}
		return fmt.Errorf("contains invalid character %q at position %d (only lowercase, digits, underscore allowed)", string(r), i)
	}
	return nil
}

// checkSingular checks that a name appears to be singular, not plural.
// Uses a trailing-s heuristic with known exception handling.
// Only checks the last component of underscore-separated names.
func checkSingular(name string) error {
	// Split on underscore, check only the last component.
	parts := strings.Split(name, "_")
	last := parts[len(parts)-1]

	// Skip very short components — can't meaningfully pluralize.
	if len(last) < 3 {
		return nil
	}

	// Check if the entire name or last component is a known exception.
	if knownPluralExceptions[name] {
		return nil
	}
	if knownPluralExceptions[last] {
		return nil
	}

	// Heuristic: reject if last component ends in 's' but not 'ss', 'us', 'is'.
	// These suffixes are common singular endings (process, status, basis).
	if strings.HasSuffix(last, "s") &&
		!strings.HasSuffix(last, "ss") &&
		!strings.HasSuffix(last, "us") &&
		!strings.HasSuffix(last, "is") &&
		!strings.HasSuffix(last, "ics") {
		return fmt.Errorf("appears plural (ends with 's'): use singular form (e.g., %q not %q)",
			strings.TrimSuffix(last, "s"), last)
	}

	return nil
}
