//# tools/opsdb-schema/loader/loader_test.go

go
package loader

import (
	"testing"

	"github.com/ghowland/opsdb/internal/testutil"
)

// TestParseMinimalEntity tests parsing of the simplest valid entity YAML.
func TestParseMinimalEntity(t *testing.T) {
	// TODO: get minimal entity YAML from testutil.MinimalValidEntity()
	// TODO: write to temp file
	// TODO: call ParseEntityFile(tempFile)
	// TODO: assert no error
	// TODO: assert entity.Name is set
	// TODO: assert entity has at least one field
	// TODO: assert raw YAML map is non-nil
	_ = testutil.MinimalValidEntity
}

// TestParseEntityWithAllTypes tests parsing of an entity with all nine field types.
func TestParseEntityWithAllTypes(t *testing.T) {
	// TODO: get YAML from testutil.EntityWithAllTypes()
	// TODO: parse
	// TODO: assert 9 fields parsed
	// TODO: assert each field type is correctly identified
}

// TestValidateRejectsReservedFieldNames tests that declaring reserved
// field names in entity YAML produces validation errors.
func TestValidateRejectsReservedFieldNames(t *testing.T) {
	// TODO: create entity YAML that declares a field named "id" or "created_time"
	// TODO: parse
	// TODO: call Validate with the entity
	// TODO: assert at least one error mentioning "reserved"
}

// TestForbiddenRegex tests detection of regex patterns in entity YAML.
func TestForbiddenRegex(t *testing.T) {
	// TODO: get YAML from testutil.EntityWithForbiddenRegex()
	// TODO: parse
	// TODO: call Validate
	// TODO: assert forbidden violation for regex
}

// TestForbiddenInheritance tests detection of extends/inherits keywords.
func TestForbiddenInheritance(t *testing.T) {
	// TODO: get YAML from testutil.EntityWithForbiddenInheritance()
	// TODO: parse, validate
	// TODO: assert forbidden violation for inheritance
}

// TestForbiddenLogic tests detection of embedded logic like NOW() in defaults.
func TestForbiddenLogic(t *testing.T) {
	// TODO: get YAML from testutil.EntityWithForbiddenLogic()
	// TODO: parse, validate
	// TODO: assert forbidden violation for embedded logic
}

// TestResolverTopologicalSort tests that FK dependencies produce correct ordering.
func TestResolverTopologicalSort(t *testing.T) {
	// TODO: get parent and child YAML from testutil.TwoEntitiesWithFK()
	// TODO: create schema with both entities
	// TODO: call Resolve(schema)
	// TODO: assert no error
	// TODO: assert schema.LoadOrder has parent before child
}

// TestResolverDetectsCycles tests that circular FK references produce an error.
func TestResolverDetectsCycles(t *testing.T) {
	// TODO: get cyclic entities from testutil.CyclicEntities()
	// TODO: create schema with both entities
	// TODO: call Resolve(schema)
	// TODO: assert error mentioning "cycle"
}

// TestInjectorUniversalFields tests that id, created_time, updated_time are injected.
func TestInjectorUniversalFields(t *testing.T) {
	// TODO: create minimal entity
	// TODO: create minimal schema
	// TODO: call Inject(schema, reserved)
	// TODO: assert entity now has "id", "created_time", "updated_time" fields
	// TODO: assert these fields have IsReserved = true
}

// TestInjectorVersioningSibling tests that versioned entities get a sibling.
func TestInjectorVersioningSibling(t *testing.T) {
	// TODO: get YAML from testutil.VersionedEntity()
	// TODO: parse, create schema, resolve, inject
	// TODO: assert schema.Entities contains "{entity_name}_version"
	// TODO: assert sibling has version_serial, parent version FK, change_set FK,
	//   is_active_version, approved_for_production_time fields
	// TODO: assert sibling has copies of parent entity's fields
}

// TestInjectorHierarchical tests that hierarchical entities get parent FK.
func TestInjectorHierarchical(t *testing.T) {
	// TODO: get YAML from testutil.HierarchicalEntity()
	// TODO: parse, create schema, inject
	// TODO: assert entity has "parent_{name}_id" field
	// TODO: assert field References points to same entity (self-referential)
}

// TestInjectorGovernanceFields tests that enabled governance fields are injected.
func TestInjectorGovernanceFields(t *testing.T) {
	// TODO: get YAML from testutil.EntityWithGovernance()
	// TODO: parse, create schema, inject
	// TODO: assert entity has underscore-prefixed governance fields
	// TODO: assert these fields have IsGovernance = true
}

// TestEvolutionAllowsNewNullableField tests that adding a nullable field passes.
func TestEvolutionAllowsNewNullableField(t *testing.T) {
	// TODO: create SchemaDiff with one new nullable field
	// TODO: call CheckEvolution(diff)
	// TODO: assert len(result.Allowed) == 1 and len(result.Forbidden) == 0
}

// TestEvolutionForbidsFieldDeletion tests that removing a field is forbidden.
func TestEvolutionForbidsFieldDeletion(t *testing.T) {
	// TODO: create SchemaDiff with one removed field
	// TODO: call CheckEvolution(diff)
	// TODO: assert len(result.Forbidden) == 1
	// TODO: assert forbidden change mentions "deprecate" as alternative
}

// TestEvolutionForbidsTypeChange tests that changing a field's type is forbidden.
func TestEvolutionForbidsTypeChange(t *testing.T) {
	// TODO: create SchemaDiff with one type change
	// TODO: call CheckEvolution(diff)
	// TODO: assert forbidden
	// TODO: assert alternative mentions "duplication pattern"
}

// TestEvolutionAllowsWideningRange tests that increasing max_value passes.
func TestEvolutionAllowsWideningRange(t *testing.T) {
	// TODO: create SchemaDiff with widened numeric range
	// TODO: call CheckEvolution
	// TODO: assert allowed
}

// TestEvolutionForbidsNarrowingRange tests that decreasing max_value is forbidden.
func TestEvolutionForbidsNarrowingRange(t *testing.T) {
	// TODO: create SchemaDiff with narrowed range
	// TODO: call CheckEvolution
	// TODO: assert forbidden
}

// TestGeneratorCreateTable tests DDL generation for a new entity.
func TestGeneratorCreateTable(t *testing.T) {
	// TODO: create entity with int, varchar, enum, FK fields
	// TODO: call generateCreateTable(entity)
	// TODO: assert SQL contains CREATE TABLE
	// TODO: assert SQL contains column definitions with correct Postgres types
	// TODO: assert SQL contains NOT NULL where applicable
	// TODO: assert SQL contains CHECK for enum values
}

// TestGeneratorFKConstraint tests DDL generation for FK constraints.
func TestGeneratorFKConstraint(t *testing.T) {
	// TODO: create field with References set
	// TODO: call generateFKConstraint(entityName, field)
	// TODO: assert SQL contains ALTER TABLE ADD CONSTRAINT
	// TODO: assert SQL contains FOREIGN KEY and REFERENCES
}

// TestGeneratorAppendOnlyRevoke tests REVOKE generation for append-only tables.
func TestGeneratorAppendOnlyRevoke(t *testing.T) {
	// TODO: call generateRevokeAppendOnly("audit_log_entry", roles)
	// TODO: assert at least one REVOKE statement
	// TODO: assert SQL contains "REVOKE UPDATE, DELETE"
}

// --- Integration Tests ---
// These require a running Postgres instance via testcontainers.

// TestFullLoadPipeline tests the complete load pipeline against actual YAML files.
func TestFullLoadPipeline(t *testing.T) {
	// TODO: if testing.Short(): t.Skip("skipping integration test")
	// TODO: create temp schema repo via testutil.SchemaRepoDir
	// TODO: call Load(repoDir)
	// TODO: assert no error
	// TODO: assert schema.Entities populated
	// TODO: assert schema.LoadOrder non-empty
	// TODO: assert schema.Relationships populated
}

// TestApplyToPostgres tests DDL application against a real Postgres instance.
func TestApplyToPostgres(t *testing.T) {
	// TODO: if testing.Short(): t.Skip("skipping integration test")
	// TODO: start test Postgres via testutil.StartTestPostgres(t)
	// TODO: defer stop
	// TODO: load schema from test fixtures
	// TODO: generate DDL
	// TODO: call Apply(db, statements, false)
	// TODO: assert no error
	// TODO: assert result.EntitiesCreated > 0
	// TODO: verify tables exist via information_schema query
	_ = testutil.StartTestPostgres
}

// TestDryRunDoesNotPersist tests that dry run rolls back all changes.
func TestDryRunDoesNotPersist(t *testing.T) {
	// TODO: if testing.Short(): t.Skip
	// TODO: start test Postgres
	// TODO: load schema, generate DDL
	// TODO: call DryRun(db, statements)
	// TODO: assert no error
	// TODO: verify tables do NOT exist (rollback worked)
}

// TestMetaPopulation tests that _schema_* tables are correctly populated.
func TestMetaPopulation(t *testing.T) {
	// TODO: if testing.Short(): t.Skip
	// TODO: start test Postgres
	// TODO: apply schema
	// TODO: call PopulateMeta
	// TODO: query _schema_version: assert one row with is_current=true
	// TODO: query _schema_entity_type: assert row count matches entities
	// TODO: query _schema_field: assert fields populated
	// TODO: query _schema_relationship: assert relationships populated
}

// TestFullApplyAndDiffCycle tests apply followed by diff shows no changes.
func TestFullApplyAndDiffCycle(t *testing.T) {
	// TODO: if testing.Short(): t.Skip
	// TODO: start test Postgres
	// TODO: load schema, generate DDL, apply, populate meta
	// TODO: read current state from DB
	// TODO: diff desired vs current
	// TODO: assert diff has no new entities, no new fields, no changes
	//   (everything desired is now in the database)
}
