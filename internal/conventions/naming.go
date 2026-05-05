
// === internal/conventions/naming.go ===
package conventions

import "fmt"

// ValidateEntityName checks that an entity name follows conventions:
// lowercase_underscore, singular, min 2 chars, max 128 chars,
// no leading underscore unless _schema_ prefix.
func ValidateEntityName(name string) error {
	// TODO: check lowercase + underscores only (no uppercase, hyphens, spaces)
	// TODO: check length 2-128
	// TODO: check singular (no trailing 's' heuristic, known exceptions list)
	// TODO: check no leading underscore unless name starts with _schema_
	// TODO: check no double underscores
	// TODO: check no trailing underscore
	return nil
}

// ValidateFieldName checks that a field name follows conventions:
// lowercase_underscore, datetime fields end in _time, date fields end in _date,
// present-state booleans start with is_, past-event booleans start with was_,
// governance fields start with underscore.
func ValidateFieldName(name string, fieldType string) error {
	// TODO: check lowercase + underscores only
	// TODO: check length 2-128
	// TODO: if fieldType == "datetime", check name ends with _time
	// TODO: if fieldType == "boolean", check name starts with is_ or was_
	// TODO: check underscore prefix only on governance field names
	// TODO: check no double underscores, no trailing underscore
	return nil
}

// ValidateFKName checks that a foreign key field follows {referenced_table}_id
// pattern, allowing role prefixes for disambiguation.
func ValidateFKName(fieldName string, referencedEntity string) error {
	// TODO: check fieldName ends with _id
	// TODO: check fieldName == referencedEntity + "_id" OR has role prefix before referenced entity
	// TODO: examples: cloud_account_id (standard), source_service_version_id (role prefixed)
	return nil
}

// ValidateCompositeName checks hierarchical prefix naming pattern.
func ValidateCompositeName(name string) error {
	// TODO: check parent_concept_subconcept pattern
	// TODO: check no double underscores, no trailing underscore
	return nil
}


