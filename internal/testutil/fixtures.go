// === internal/testutil/fixtures.go ===
package testutil

import "testing"

// MinimalValidEntity returns YAML for the simplest valid entity:
// name, category, one varchar field.
func MinimalValidEntity() string {
	// TODO: return YAML string:
	// name: test_entity
	// category: identity
	// fields:
	//   - name: label
	//     type: varchar
	//     max_length: 255
	//     nullable: false
	return ""
}

// EntityWithAllTypes returns YAML with one field of each of the nine types.
func EntityWithAllTypes() string {
	// TODO: return YAML with int, float, varchar, text, boolean, datetime,
	//       json (with discriminator), enum, foreign_key fields
	return ""
}

// EntityWithForbiddenRegex returns YAML containing a regex pattern.
func EntityWithForbiddenRegex() string {
	// TODO: return YAML with a field containing pattern: "^[a-z]+$"
	return ""
}

// EntityWithForbiddenInheritance returns YAML containing extends keyword.
func EntityWithForbiddenInheritance() string {
	// TODO: return YAML with extends: base_entity
	return ""
}

// EntityWithForbiddenLogic returns YAML containing NOW() default.
func EntityWithForbiddenLogic() string {
	// TODO: return YAML with a field having default: "NOW()"
	return ""
}

// VersionedEntity returns YAML with versioned: true.
func VersionedEntity() string {
	// TODO: return YAML with versioned: true and at least one field
	return ""
}

// HierarchicalEntity returns YAML with hierarchical: true.
func HierarchicalEntity() string {
	// TODO: return YAML with hierarchical: true
	return ""
}

// EntityWithGovernance returns YAML with all governance fields enabled.
func EntityWithGovernance() string {
	// TODO: return YAML with governance section enabling all fields
	return ""
}

// TwoEntitiesWithFK returns parent and child YAML where child has FK to parent.
func TwoEntitiesWithFK() (string, string) {
	// TODO: return (parent YAML, child YAML with foreign_key referencing parent)
	return "", ""
}

// CyclicEntities returns two entities with circular FK references.
func CyclicEntities() (string, string) {
	// TODO: return (entity_a with FK to entity_b, entity_b with FK to entity_a)
	//       used to test cycle detection
	return "", ""
}

// SchemaRepoDir creates a temporary directory with proper schema repo structure:
// directory.yaml, meta-schema, conventions, and the provided entity YAML strings.
// Returns the directory path. Cleaned up by testing.T.
func SchemaRepoDir(t *testing.T, entities ...string) string {
	// TODO: create temp dir
	// TODO: write meta/_schema_meta.yaml from embedded content
	// TODO: write conventions/reserved.yaml from embedded content
	// TODO: write each entity string as a .yaml file in domains/test/
	// TODO: write directory.yaml listing the entity files
	// TODO: register t.Cleanup to remove temp dir
	// TODO: return path
	return ""
}
