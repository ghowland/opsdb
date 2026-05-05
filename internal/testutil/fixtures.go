package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// MinimalValidEntity returns YAML for the simplest valid entity:
// name, category, one varchar field.
func MinimalValidEntity() string {
	return `name: test_entity
description: "Minimal valid entity for testing"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "display label"
`
}

// EntityWithAllTypes returns YAML with one field of each of the nine types.
func EntityWithAllTypes() string {
	return `name: all_types_entity
description: "Entity with one field of each type"
category: identity

fields:
  - name: count_value
    type: int
    nullable: false
    min_value: 0
    max_value: 1000000
    description: "integer field"

  - name: ratio_value
    type: float
    nullable: true
    min_value: 0.0
    max_value: 1.0
    precision_decimal_places: 4
    description: "float field"

  - name: short_name
    type: varchar
    max_length: 128
    nullable: false
    description: "varchar field"

  - name: long_description
    type: text
    nullable: true
    description: "text field"

  - name: is_enabled
    type: boolean
    nullable: false
    default: true
    description: "boolean field"

  - name: observed_time
    type: datetime
    nullable: true
    description: "datetime field"

  - name: expiration_date
    type: date
    nullable: true
    description: "date field"

  - name: status
    type: enum
    nullable: false
    enum_values:
      - active
      - inactive
      - pending
    description: "enum field"

  - name: payload_type
    type: enum
    nullable: false
    enum_values:
      - type_a
      - type_b
    description: "discriminator for JSON payload"

  - name: payload_data_json
    type: json
    nullable: true
    json_type_discriminator: payload_type
    description: "json typed payload field"
`
}

// EntityWithFK returns YAML for an entity with a foreign key to test_entity.
// Requires MinimalValidEntity to be loaded first.
func EntityWithFK() string {
	return `name: child_entity
description: "Entity with FK to test_entity"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "display label"

  - name: test_entity_id
    type: foreign_key
    nullable: false
    references: test_entity
    description: "FK to parent test_entity"
`
}

// EntityWithForbiddenRegex returns YAML containing a regex pattern
// in a constraint value. Should trigger the forbidden pattern scanner.
func EntityWithForbiddenRegex() string {
	return `name: regex_entity
description: "Entity with forbidden regex pattern"
category: identity

fields:
  - name: code
    type: varchar
    max_length: 50
    nullable: false
    pattern: "^[A-Z]{2}-[0-9]{4}$"
    description: "field with regex pattern constraint"
`
}

// EntityWithForbiddenInheritance returns YAML containing the extends keyword.
// Should trigger the forbidden pattern scanner.
func EntityWithForbiddenInheritance() string {
	return `name: inherited_entity
description: "Entity with forbidden inheritance"
category: identity
extends: base_entity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "display label"
`
}

// EntityWithForbiddenLogic returns YAML containing NOW() in a default value.
// Should trigger the forbidden pattern scanner for embedded logic.
func EntityWithForbiddenLogic() string {
	return `name: logic_entity
description: "Entity with forbidden embedded logic"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "display label"

  - name: recorded_time
    type: datetime
    nullable: false
    default: "NOW()"
    description: "field with computed default"
`
}

// EntityWithForbiddenTemplating returns YAML containing template syntax.
func EntityWithForbiddenTemplating() string {
	return `name: template_entity
description: "Entity with forbidden templating"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    default: "{{ env.HOSTNAME }}"
    description: "field with template variable"
`
}

// EntityWithForbiddenImport returns YAML containing an import directive.
func EntityWithForbiddenImport() string {
	return `name: import_entity
description: "Entity with forbidden import"
category: identity
import: shared/base_fields.yaml

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "display label"
`
}

// VersionedEntity returns YAML with versioned: true and soft_delete: true.
func VersionedEntity() string {
	return `name: versioned_thing
description: "Versioned entity for testing sibling generation"
category: identity
versioned: true
soft_delete: true

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "display label"

  - name: config_value
    type: varchar
    max_length: 1024
    nullable: true
    description: "configuration value that gets versioned"
`
}

// HierarchicalEntity returns YAML with hierarchical: true.
func HierarchicalEntity() string {
	return `name: tree_node
description: "Hierarchical entity for testing self-FK injection"
category: identity
hierarchical: true
soft_delete: true

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "node label"

  - name: depth_level
    type: int
    nullable: false
    min_value: 0
    max_value: 100
    description: "depth in tree"
`
}

// AppendOnlyEntity returns YAML with append_only: true.
func AppendOnlyEntity() string {
	return `name: event_log
description: "Append-only entity for testing revoke generation"
category: identity
append_only: true

fields:
  - name: event_type
    type: varchar
    max_length: 128
    nullable: false
    description: "type of event"

  - name: event_data
    type: text
    nullable: true
    description: "event payload"
`
}

// EntityWithGovernance returns YAML with governance fields enabled.
func EntityWithGovernance() string {
	return `name: governed_entity
description: "Entity with all governance fields enabled"
category: identity
soft_delete: true

governance:
  _requires_group: true
  _access_classification: true
  _retention_policy_id: true

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "display label"

  - name: sensitivity_note
    type: text
    nullable: true
    description: "note about data sensitivity"
`
}

// EntityWithObservationGovernance returns YAML with observation governance fields.
func EntityWithObservationGovernance() string {
	return `name: observed_entity
description: "Entity with observation governance fields"
category: identity

governance:
  _observed_time: true
  _authority_id: true
  _puller_runner_job_id: true

fields:
  - name: state_key
    type: varchar
    max_length: 255
    nullable: false
    description: "observation key"

  - name: state_value
    type: text
    nullable: true
    description: "observation value"
`
}

// EntityWithReservedFieldCollision returns YAML that declares a field
// named "id" which collides with the reserved universal field.
func EntityWithReservedFieldCollision() string {
	return `name: collision_entity
description: "Entity that declares a reserved field name"
category: identity

fields:
  - name: id
    type: int
    nullable: false
    description: "this collides with the reserved id field"

  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "display label"
`
}

// EntityWithDuplicateFields returns YAML with two fields having the same name.
func EntityWithDuplicateFields() string {
	return `name: duplicate_entity
description: "Entity with duplicate field names"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "first label"

  - name: label
    type: text
    nullable: true
    description: "duplicate label"
`
}

// TwoEntitiesWithFK returns parent and child YAML where the child has
// a foreign key referencing the parent. Load order: parent first.
func TwoEntitiesWithFK() (string, string) {
	parent := `name: parent_entity
description: "Parent entity for FK testing"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "parent label"
`

	child := `name: child_entity
description: "Child entity with FK to parent"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "child label"

  - name: parent_entity_id
    type: foreign_key
    nullable: false
    references: parent_entity
    description: "FK to parent_entity"
`

	return parent, child
}

// CyclicEntities returns two entities with circular FK references.
// Used to test that the resolver detects cycles and reports them.
func CyclicEntities() (string, string) {
	entityA := `name: cycle_a
description: "First entity in circular reference"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "label"

  - name: cycle_b_id
    type: foreign_key
    nullable: true
    references: cycle_b
    description: "FK to cycle_b (creates cycle)"
`

	entityB := `name: cycle_b
description: "Second entity in circular reference"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "label"

  - name: cycle_a_id
    type: foreign_key
    nullable: true
    references: cycle_a
    description: "FK to cycle_a (creates cycle)"
`

	return entityA, entityB
}

// ThreeEntityChain returns three entities forming a dependency chain:
// grandparent <- parent <- child. Tests topological sort ordering.
func ThreeEntityChain() (string, string, string) {
	gp := `name: grandparent
description: "Root of dependency chain"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "label"
`

	p := `name: parent_node
description: "Middle of dependency chain"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "label"

  - name: grandparent_id
    type: foreign_key
    nullable: false
    references: grandparent
    description: "FK to grandparent"
`

	c := `name: leaf_node
description: "End of dependency chain"
category: identity

fields:
  - name: label
    type: varchar
    max_length: 255
    nullable: false
    description: "label"

  - name: parent_node_id
    type: foreign_key
    nullable: false
    references: parent_node
    description: "FK to parent_node"
`

	return gp, p, c
}

// minimalMetaSchema returns embedded meta-schema YAML content for test repos.
func minimalMetaSchema() string {
	return `version: "1.0"

allowed_top_level_keys:
  - name
  - description
  - category
  - versioned
  - soft_delete
  - hierarchical
  - append_only
  - fields
  - indexes
  - governance

allowed_field_keys:
  - name
  - type
  - nullable
  - description
  - default
  - unique
  - references
  - max_length
  - min_length
  - max_value
  - min_value
  - precision_decimal_places
  - enum_values
  - json_type_discriminator
  - must_be_unique_within

allowed_index_keys:
  - fields
  - unique
  - name

allowed_governance_keys:
  - _requires_group
  - _access_classification
  - _audit_chain_hash
  - _retention_policy_id
  - _schema_version_introduced_id
  - _schema_version_deprecated_id
  - _observed_time
  - _authority_id
  - _puller_runner_job_id

allowed_categories:
  - identity
  - substrate
  - service
  - kubernetes
  - authority
  - schedule
  - policy
  - documentation
  - runner
  - monitoring
  - observation
  - config
  - change_mgmt
  - audit
  - schema_meta
`
}

// minimalReservedYAML returns embedded reserved.yaml content for test repos.
func minimalReservedYAML() string {
	return `universal:
  fields:
    - name: id
      type: int
      nullable: false
      description: "primary key auto-increment"
    - name: created_time
      type: datetime
      nullable: false
      description: "set on insert"
    - name: updated_time
      type: datetime
      nullable: false
      description: "set on insert and update"

soft_delete:
  fields:
    - name: is_active
      type: boolean
      nullable: false
      default: true
      description: "soft delete state"

hierarchical:
  fields: []

versioning_sibling:
  fields:
    - name: version_serial
      type: int
      nullable: false
      description: "monotonic version number"
    - name: is_active_version
      type: boolean
      nullable: false
      default: false
      description: "true for current version"
    - name: approved_for_production_time
      type: datetime
      nullable: true
      description: "when version went live"

governance:
  fields:
    - name: _requires_group
      type: varchar
      nullable: true
      max_length: 255
      description: "group required for access"
    - name: _access_classification
      type: enum
      nullable: true
      enum_values: [public, internal, confidential, restricted, regulated]
      description: "data sensitivity level"
    - name: _retention_policy_id
      type: foreign_key
      nullable: true
      references: retention_policy
      description: "retention policy override"

observation:
  fields:
    - name: _observed_time
      type: datetime
      nullable: true
      description: "when observation sampled"
    - name: _authority_id
      type: foreign_key
      nullable: true
      references: authority
      description: "source authority"
    - name: _puller_runner_job_id
      type: foreign_key
      nullable: true
      references: runner_job
      description: "runner job that wrote"

schema_metadata:
  fields:
    - name: _schema_version_introduced_id
      type: foreign_key
      nullable: true
      references: _schema_version
      description: "schema version introduced"
    - name: _schema_version_deprecated_id
      type: foreign_key
      nullable: true
      references: _schema_version
      description: "schema version deprecated"

append_only:
  revoke_operations: [UPDATE, DELETE]
  revoke_from_roles: [opsdb_app_role, opsdb_runner_role, opsdb_readonly_role]

database_roles:
  - name: opsdb_app_role
    permissions: [SELECT, INSERT, UPDATE, DELETE]
    applies_to: all
    description: "application role for API"
  - name: opsdb_admin_role
    permissions: [ALL]
    applies_to: all
    description: "admin role for substrate operators"
  - name: opsdb_readonly_role
    permissions: [SELECT]
    applies_to: all
    description: "read-only role for auditors"
  - name: opsdb_runner_role
    permissions: [SELECT, INSERT, UPDATE]
    applies_to: all
    description: "runner role"
`
}

// SchemaRepoDir creates a temporary directory with proper schema repo structure:
// meta-schema, reserved conventions, directory.yaml, and the provided entity
// YAML strings. Each entity is written as a numbered file in domains/test/.
// The directory.yaml imports list references each file in order.
// Returns the repo root path. Cleaned up automatically by testing.T.
func SchemaRepoDir(t *testing.T, entities ...string) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create directory structure.
	schemaDir := filepath.Join(tmpDir, "schema")
	dirs := []string{
		filepath.Join(schemaDir, "meta"),
		filepath.Join(schemaDir, "conventions"),
		filepath.Join(schemaDir, "domains", "test"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("creating test schema dir %s: %v", d, err)
		}
	}

	// Write meta-schema.
	metaPath := filepath.Join(schemaDir, "meta", "_schema_meta.yaml")
	if err := os.WriteFile(metaPath, []byte(minimalMetaSchema()), 0644); err != nil {
		t.Fatalf("writing meta-schema: %v", err)
	}

	// Write reserved conventions.
	reservedPath := filepath.Join(schemaDir, "conventions", "reserved.yaml")
	if err := os.WriteFile(reservedPath, []byte(minimalReservedYAML()), 0644); err != nil {
		t.Fatalf("writing reserved conventions: %v", err)
	}

	// Write entity files and build directory.yaml imports list.
	var imports []string
	for i, entityYAML := range entities {
		filename := fmt.Sprintf("entity_%03d.yaml", i)
		relPath := filepath.Join("domains", "test", filename)
		absPath := filepath.Join(schemaDir, relPath)

		if err := os.WriteFile(absPath, []byte(entityYAML), 0644); err != nil {
			t.Fatalf("writing entity file %s: %v", filename, err)
		}
		imports = append(imports, relPath)
	}

	// Write directory.yaml.
	var directoryContent string
	directoryContent = "# Auto-generated test directory\nimports:\n"
	for _, imp := range imports {
		directoryContent += fmt.Sprintf("  - %s\n", imp)
	}

	directoryPath := filepath.Join(schemaDir, "directory.yaml")
	if err := os.WriteFile(directoryPath, []byte(directoryContent), 0644); err != nil {
		t.Fatalf("writing directory.yaml: %v", err)
	}

	return tmpDir
}

// SchemaRepoDirOrdered creates a test schema repo where entity files
// are loaded in the exact order specified. Unlike SchemaRepoDir which
// numbers entities automatically, this lets tests control the import
// order explicitly (important for FK dependency testing).
func SchemaRepoDirOrdered(t *testing.T, namedEntities map[string]string, order []string) string {
	t.Helper()

	tmpDir := t.TempDir()

	schemaDir := filepath.Join(tmpDir, "schema")
	dirs := []string{
		filepath.Join(schemaDir, "meta"),
		filepath.Join(schemaDir, "conventions"),
		filepath.Join(schemaDir, "domains", "test"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("creating test schema dir %s: %v", d, err)
		}
	}

	metaPath := filepath.Join(schemaDir, "meta", "_schema_meta.yaml")
	if err := os.WriteFile(metaPath, []byte(minimalMetaSchema()), 0644); err != nil {
		t.Fatalf("writing meta-schema: %v", err)
	}

	reservedPath := filepath.Join(schemaDir, "conventions", "reserved.yaml")
	if err := os.WriteFile(reservedPath, []byte(minimalReservedYAML()), 0644); err != nil {
		t.Fatalf("writing reserved conventions: %v", err)
	}

	var imports []string
	for _, name := range order {
		entityYAML, ok := namedEntities[name]
		if !ok {
			t.Fatalf("entity %q listed in order but not in namedEntities map", name)
		}

		filename := name + ".yaml"
		relPath := filepath.Join("domains", "test", filename)
		absPath := filepath.Join(schemaDir, relPath)

		if err := os.WriteFile(absPath, []byte(entityYAML), 0644); err != nil {
			t.Fatalf("writing entity file %s: %v", filename, err)
		}
		imports = append(imports, relPath)
	}

	directoryContent := "# Auto-generated test directory\nimports:\n"
	for _, imp := range imports {
		directoryContent += fmt.Sprintf("  - %s\n", imp)
	}

	directoryPath := filepath.Join(schemaDir, "directory.yaml")
	if err := os.WriteFile(directoryPath, []byte(directoryContent), 0644); err != nil {
		t.Fatalf("writing directory.yaml: %v", err)
	}

	return tmpDir
}