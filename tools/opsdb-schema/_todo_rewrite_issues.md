# OpsDB Schema Engine — Technical Specification

## Package: `tools/opsdb-schema/loader/` (package `loader`) + `tools/opsdb-schema/cmd/` (package `main`)

### Overview

Twelve files: one CLI entrypoint and eleven loader package files implementing the schema engine pipeline. Depends on `internal/model`, `internal/conventions`, `internal/vocabulary`, `internal/pg`, `internal/testutil`. External dependency: `gopkg.in/yaml.v3`.

---

## File: cmd/main.go

### Purpose
CLI entrypoint for the `opsdb-schema` binary. Parses flags, dispatches to six commands.

### Commands
- `validate` — Parse and validate schema YAML. No database required.
- `plan` — Load, diff, check evolution, generate DDL, print plan.
- `apply` — Load, diff, check evolution, generate DDL, execute, populate metadata.
- `diff` — Load, diff, print differences.
- `export` — Read current database schema, print to stdout.
- `init` — Create empty schema directory structure (no embedded YAML).

### Flags
- `--repo` — path to schema repository root (default `.`)
- `--dsn` — Postgres connection string (or `OPSDB_DSN` env var)
- `--scope` — limit to entity or entity/field (accepted but unused currently)
- `--verbose` — verbose DDL output in plan/apply
- `--dry-run` — apply with rollback instead of commit
- `--version` — print version and exit

### Exit Codes
- `0` — success
- `1` — validation or evolution error
- `2` — runtime error (connection, parse, etc.)

### Cross-References
- Uses `model.SchemaError` for error display (not `loader.SchemaError`)
- Uses `loader.Load`, `loader.ReadCurrentState`, `loader.Diff`, `loader.CheckEvolution`, `loader.GenerateDDL`, `loader.Apply`, `loader.DryRun`, `loader.PopulateMeta`
- Uses `loader.AllowedChange`, `loader.DDLStatement`, `loader.ApplyResult`
- Uses `pg.Connect`, `pg.WithTransaction`, `pg.Tx`

### IOSE Deviations
- **`cmdInit` does not embed YAML content.** The IOSE describes init writing minimal meta-schema and reserved.yaml. The rewrite creates directory structure only and instructs the user to populate files. This is intentional — the actual YAML files already exist in the repo at `schema/`.
- **`scope` flag is accepted but unused.** The IOSE mentions `--scope entity or entity/field`. The flag is parsed and passed to command functions but all commands currently ignore it with `_ = scope`. Noted for future implementation.

---

## File: loader/loader.go

### Purpose
Pipeline orchestrator. Entry point for the loading half before database interaction.

### Functions

- `Load(repoPath string) (*model.Schema, error)` — Full pipeline: parse meta-schema → parse reserved → parse directory.yaml → parse each entity file → validate → resolve FK dependencies → inject reserved fields. Returns complete `Schema` with `schema.Reserved` populated. Returns schema even on validation errors (caller inspects `schema.Errors`).
- `LoadAndValidateOnly(repoPath string) (*model.Schema, error)` — Stops before resolution and injection. Used by validate command when no database is needed.

### Pipeline Steps (in Load)
1. `ParseMetaSchema(metaPath)` → `*MetaSchema`
2. `ParseReserved(reservedPath)` → `*model.ReservedConfig`
3. `ParseDirectoryYAML(directoryPath)` → `[]string` (entity file paths)
4. Initialize `model.Schema` with `Reserved: reserved`
5. For each entity path: `ParseEntityFile` → `Validate` → add to schema
6. `Resolve(schema)` — FK dependency graph and topo sort
7. `Inject(schema, reserved)` — reserved fields and versioning siblings

### Cross-References
- Calls `ParseMetaSchema`, `ParseReserved`, `ParseDirectoryYAML`, `ParseEntityFile` (parser.go)
- Calls `Validate` (validator.go)
- Calls `Resolve` (resolver.go)
- Calls `Inject` (injector.go)
- Uses `model.Schema`, `model.SchemaError`

### IOSE Deviations
- **`LoadAndValidateOnly` not explicitly in IOSE.** The IOSE describes `Load` as the single entry point. `LoadAndValidateOnly` is a subset that skips resolution and injection, useful for offline validation. Should be retained.
- **`schema.Reserved` populated.** The IOSE doesn't mention storing `reserved` on the schema struct. Added so downstream consumers (meta populator) can access it without re-parsing. `model.Schema` has a `Reserved *ReservedConfig` field.

---

## File: loader/parser.go

### Purpose
YAML parsing for all schema configuration files. Produces structured types from raw YAML.

### Types Defined (loader-local)

**`MetaSchema`** — Simplified parse target for meta-schema validation.
- `AllowedTopLevelKeys []string`
- `AllowedFieldKeys []string`
- `AllowedIndexKeys []string`
- `AllowedGovernanceKeys []string`
- `AllowedCategories []string`
- `Version string`

Note: This is intentionally simpler than `model.MetaSchema`. The validator only needs the string slices for validation. The full `model.MetaSchema` carries richer structure for runtime use by the API server.

**`JSONPayloadSchema`** — Parsed JSON payload schema for typed payload validation.
- `Name string`
- `Description string`
- `Fields []JSONPayloadField`

**`JSONPayloadField`** — One field in a JSON payload schema.
- `Name string`
- `Type string`
- `Required bool`
- `Description string`
- `Constraints map[string]interface{}`

**YAML parse structs (unexported):**
- `reservedYAML` — mirrors `conventions/reserved.yaml` structure
- `reservedFieldYAML` — includes `EnabledBy string` for governance/observation/schema_metadata fields
- `reservedRoleYAML` — uses `Grants []string` (not `Permissions`), no `AppliesTo`

### Functions

- `ParseEntityFile(path string) (*model.Entity, map[string]interface{}, error)` — Returns structured Entity AND raw YAML map. Raw map consumed by forbidden pattern scanner. Validates top-level keys. Parses fields, indexes, governance map.
- `parseField(fieldMap, index, filePath) (*model.Field, error)` — Extracts all field properties. `MaxLength`/`MinLength` assigned as plain `int` (0 = unset). `MaxValue`/`MinValue` converted via `yamlFloat64Ptr` to `*float64`.
- `parseIndex(idxMap, index, filePath) (*model.Index, error)` — Optional user-provided name stored in `Index.Description` (model has no `Name` field). Fields list required.
- `ParseDirectoryYAML(path string) ([]string, error)` — Reads `imports` key, checks for duplicates.
- `ParseMetaSchema(path string) (*MetaSchema, error)` — Reads allowed keys, types, categories.
- `ParseReserved(path string) (*model.ReservedConfig, error)` — Returns `*model.ReservedConfig` (not a local type). Populates:
  - `Universal []model.Field` from `universal.fields`
  - `SoftDelete []model.Field` from `soft_delete.fields`
  - `Hierarchical []model.Field` from `hierarchical.fields`
  - `VersioningSibling []model.Field` from `versioning_sibling.fields`
  - `Governance []model.GovernanceFieldDef` from `governance.fields` with `EnabledBy`
  - `Observation []model.GovernanceFieldDef` from `observation.fields` with `EnabledBy`
  - `SchemaMetadata []model.GovernanceFieldDef` from `schema_metadata.fields` with `EnabledBy`
  - `AppendOnly model.AppendOnlyConfig` with `PostgresRevokeRoles`
  - `DatabaseRoles []model.RoleDefinition` with `Grants` (not `Permissions`)
- `ParseJSONSchema(path string) (*JSONPayloadSchema, error)` — Validates depth restrictions (no lists-of-lists, no maps-of-lists).

### YAML Helpers (unexported)
- `yamlFieldToModel(fd reservedFieldYAML, isReserved, isGovernance bool) model.Field` — Converts YAML field def to model. `MaxLength` set only if `> 0`.
- `yamlBool(m, key) bool` — Bool from map with false default.
- `yamlBoolValue(val interface{}) bool` — Handles `bool`, `string` ("true"/"yes").
- `yamlInt(val interface{}) (int, error)` — Handles `int`, `int64`, `float64`.
- `yamlFloat64Ptr(val interface{}) *float64` — Handles `float64`, `float32`, `int`, `int64`, `int32`. Returns nil on unconvertible input.
- `yamlStringSlice(val interface{}) ([]string, error)` — Handles `[]string`, `[]interface{}`.

### Cross-References
- Produces `*model.Entity`, `*model.ReservedConfig`, `model.Field`, `model.Index`
- Uses `model.GovernanceFieldDef`, `model.RoleDefinition`, `model.AppendOnlyConfig`

### IOSE Deviations
- **`ParseReserved` returns `*model.ReservedConfig` directly.** The IOSE describes `ParseReserved` delegating to `conventions.LoadReserved`. The rewrite parses directly from YAML into model types, bypassing conventions package for reserved loading. This avoids a circular dependency (parser producing types that conventions would also produce) and ensures exact alignment with model struct. The conventions package is still used for field generation (`GetUniversalFields`, etc.) in the injector.
- **`ParseJSONSchema` not explicitly listed in IOSE for parser.** The IOSE mentions JSON schema validation but doesn't list a specific parse function. Retained because the schema directory contains `json_schemas/` subdirectories and the schema engine needs to validate them.
- **`yamlFloat64Ptr` is new.** Not in the naive version — added to correctly convert YAML numeric values to `*float64` for `model.Field.MaxValue`/`MinValue`.

---

## File: loader/validator.go

### Purpose
Validates parsed entities against meta-schema. Accumulates errors rather than failing on first.

### Functions

- `Validate(entity *model.Entity, rawYAML map[string]interface{}, metaSchema *MetaSchema, knownEntities map[string]bool) []model.SchemaError` — Runs all validation checks on one entity:
  1. Entity name via `conventions.ValidateEntityName`
  2. Category in `metaSchema.AllowedCategories`
  3. Versioned + AppendOnly mutual exclusion
  4. Reserved field name collision via `conventions.IsReservedFieldName`
  5. Field validation via `validateFields`
  6. Index validation via `validateIndexes`
  7. Governance key validation via `validateGovernance`
  8. Forbidden pattern scan via `vocabulary.ScanForForbiddenPatterns`

- `validateFields(fields, entityName, metaSchema, knownEntities) []model.SchemaError` — Per-field checks:
  - Duplicate field names
  - Field name via `conventions.ValidateFieldName`
  - Type via `vocabulary.IsValidType`
  - Constraints via `vocabulary.ValidateConstraints` using `buildConstraintMap`
  - Default via `vocabulary.ValidateDefault`
  - Unique via `vocabulary.ValidateUnique`
  - Composite uniqueness via `vocabulary.ValidateMustBeUniqueWithin`
  - FK: requires `references`, validates name via `conventions.ValidateFKName`, checks target exists via `vocabulary.ValidateReferences`
  - JSON: requires `json_type_discriminator`, validates via `vocabulary.ValidateJsonDiscriminator(field, []string)`

- `validateIndexes(indexes, fields, entityName) []model.SchemaError` — Checks index field references exist. Pre-includes reserved field names in the lookup set.

- `validateGovernance(governance, metaSchema, entityName) []model.SchemaError` — Checks governance keys against `metaSchema.AllowedGovernanceKeys`.

- `buildConstraintMap(field model.Field) map[string]interface{}` — **Canonical copy for the entire loader package.** Converts `model.Field` properties to the constraint map format expected by `vocabulary.ValidateConstraints` and `vocabulary.GetPostgresType`. `MaxLength`/`MinLength` checked with `> 0`. `MaxValue`/`MinValue` dereferenced from `*float64`.

- `isInList(s string, list []string) bool` — Simple string-in-slice check.

### Cross-References
- Uses `conventions.ValidateEntityName`, `conventions.ValidateFieldName`, `conventions.ValidateFKName`, `conventions.IsReservedFieldName`
- Uses `vocabulary.IsValidType`, `vocabulary.ValidateConstraints`, `vocabulary.ValidateDefault`, `vocabulary.ValidateUnique`, `vocabulary.ValidateMustBeUniqueWithin`, `vocabulary.ValidateReferences`, `vocabulary.ValidateJsonDiscriminator`, `vocabulary.ScanForForbiddenPatterns`
- `buildConstraintMap` called by differ.go, generator.go, and this file

### IOSE Deviations
- **`vocabulary.ValidateModifiersForType` call removed.** The IOSE lists this function but it does not exist in the vocabulary package. The individual modifier validation functions (`ValidateDefault`, `ValidateUnique`, `ValidateMustBeUniqueWithin`) cover the same ground. If `ValidateModifiersForType` is added to vocabulary later, the call should be restored here.
- **`ValidateJsonDiscriminator` called with `[]string` not `map[string]string`.** The naive version passed a `map[string]string` (field name → type). The IOSE signature is `(discriminatorField string, entityFields []string)`. Fixed to pass `allFieldNames` slice. If the vocabulary function's actual signature differs, this needs revisiting.
- **Local `SchemaError` type removed.** Naive version defined `SchemaError` in this file. Removed — uses `model.SchemaError` everywhere. `loader.go` and `cmd/main.go` both reference `model.SchemaError`.

---

## File: loader/resolver.go

### Purpose
FK dependency resolution, cycle detection, topological sort.

### Functions

- `Resolve(schema *model.Schema) error` — Builds dependency graph → detects cycles → topological sort → builds `model.Relationship` structs. Populates `schema.LoadOrder` and `schema.Relationships`.
- `BuildDependencyGraph(entities map[string]*model.Entity) (map[string][]string, error)` — Adjacency list from FK references. Self-referential edges excluded. Validates referenced entities exist. Deduplicates multiple FKs to same target.
- `TopologicalSort(graph map[string][]string) ([]string, error)` — Kahn's algorithm. Deterministic output via sorted queues. Returns error if not all nodes visited (cycle).
- `DetectCycles(graph map[string][]string) [][]string` — DFS-based cycle detection with deterministic ordering. Returns list of cycles for error reporting.
- `extractCycle(from, to string, parent map[string]string) []string` — Traces parent chain to build cycle path. Safety-bounded to prevent infinite loops.

### Cross-References
- Uses `model.Schema`, `model.Entity`, `model.Relationship`
- Writes `schema.LoadOrder`, `schema.Relationships`

### IOSE Deviations
- **Dead loop removed from `TopologicalSort`.** The naive version had an empty loop `for _, deps := range graph { for _, dep := range deps { } _ = dep }` that did nothing. Removed in rewrite.
- None otherwise. Matches IOSE exactly.

---

## File: loader/injector.go

### Purpose
Post-validation field injection. Adds reserved fields, governance fields, versioning siblings.

### Functions

- `Inject(schema *model.Schema, reserved *model.ReservedConfig) error` — Two-pass process:
  1. **First pass:** For each entity, inject universal fields, soft delete, hierarchical, governance fields.
  2. **Second pass:** For versioned entities, generate versioning sibling entity, add relationships (sibling→parent, sibling→self, sibling→change_set), insert into `LoadOrder` after parent.

- `injectUniversalFields(entity *model.Entity)` — Prepends `id`, `created_time`, `updated_time` via `conventions.GetUniversalFields()`.
- `injectSoftDelete(entity *model.Entity)` — Appends `is_active` via `conventions.GetSoftDeleteFields()`.
- `injectHierarchical(entity *model.Entity)` — Appends `parent_{entity}_id` via `conventions.GetHierarchicalFields(entity.Name)`.
- `injectGovernanceFields(entity, enabled map[string]bool, reserved *model.ReservedConfig)` — Searches across three `GovernanceFieldDef` slices: `reserved.Governance`, `reserved.Observation`, `reserved.SchemaMetadata`. Matches on `gfd.EnabledBy == flagName`. Falls back to `conventions.GetGovernanceFields` if not found in any slice.
- `findAndInjectGovernanceField(entity, flagName string, defs []model.GovernanceFieldDef) bool` — Helper: searches one slice, injects if found, returns whether matched.
- `generateVersioningSibling(entity *model.Entity) *model.Entity` — Creates `{name}_version` entity with:
  - Universal fields (injected directly since siblings are created in second pass)
  - FK to parent entity
  - `version_serial` (int, `MinValue` = `float64(1)`)
  - Self-referential FK for version chain
  - FK to `change_set`
  - `is_active_version` boolean
  - `approved_for_production_time` datetime
  - Snapshot copies of all parent fields (nullable, no defaults, no uniqueness)
  - Three indexes (composite unique on entity+serial, index on change_set_id, index on is_active_version)
  - Sets `IsSibling = true`, `ParentEntity = entity.Name`
  - Index definitions use `Description` for explicit names (model has no `Index.Name`)

### Cross-References
- Takes `*model.ReservedConfig` (from parser.go via loader.go)
- Uses `conventions.GetUniversalFields`, `conventions.GetSoftDeleteFields`, `conventions.GetHierarchicalFields`, `conventions.GetGovernanceFields`
- Uses `model.Entity`, `model.Field`, `model.Index`, `model.Relationship`, `model.GovernanceFieldDef`

### IOSE Deviations
- **`findAndInjectGovernanceField` is new.** Not in IOSE — extracted as helper to avoid tripling governance search code across three slices. Should be retained.
- **Governance lookup changed from map to slice iteration.** The naive version used `reserved.GovernanceFields[flagName]` (a map). The model uses `[]GovernanceFieldDef` with `EnabledBy` field. The rewrite searches across all three slices (Governance, Observation, SchemaMetadata) matching on `EnabledBy`.
- **Sibling structural field skip uses a set.** The naive version used a chain of `||` comparisons. The rewrite builds `siblingStructuralNames` map for cleaner readability. Functionally identical.

---

## File: loader/differ.go

### Purpose
Compares desired state (Schema from YAML) against current state (from database). Produces a classified diff.

### Types Defined

**`SchemaState`** — Current database schema state.
- `Entities map[string]*EntityState`
- `Relationships []RelationshipState`
- `Version int`

**`EntityState`** — One table's current schema.
- `Name string`
- `Fields map[string]*FieldState`
- `Indexes []IndexState`
- `Constraints []ConstraintState`

**`FieldState`** — One column's current state.
- `Name`, `Type string`, `IsNullable bool`, `Default *string`, `MaxLength *int`, `NumericScale *int`

**`IndexState`** — `Name`, `Fields []string`, `Unique bool`

**`ConstraintState`** — `Name`, `Type`, `Fields []string`, `ReferencesTable *string`, `ReferencesField *string`, `CheckExpression *string`

**`RelationshipState`** — `SourceTable`, `SourceField`, `TargetTable`, `TargetField`

**`SchemaDiff`** — Classified differences.
- `NewEntities []string`
- `NewFields []DiffItem`
- `ChangedConstraints []DiffItem`
- `NewIndexes []DiffItem`
- `RemovedFields []DiffItem`
- `RemovedEntities []string`
- `TypeChanges []DiffItem`
- `Other []DiffItem`

**`DiffItem`** — One specific difference.
- `Entity`, `Field`, `ChangeType string`, `DesiredValue`, `CurrentValue interface{}`, `Description string`

### Functions

- `Diff(desired *model.Schema, current *SchemaState) (*SchemaDiff, error)` — Compares entities, fields, types, constraints, indexes. Uses `buildConstraintMap` (from validator.go) and `vocabulary.GetPostgresType` for type comparison.
- `ReadCurrentState(db *pg.DB) (*SchemaState, error)` — Checks for `_schema_entity_type` table existence. Delegates to `readFromSchemaMetaTables` or `ReadFromInformationSchema`.
- `readFromSchemaMetaTables(db *pg.DB) (*SchemaState, error)` — Reads from `_schema_version`, `_schema_entity_type`, `_schema_field`, `_schema_relationship`. Uses `db.Query`/`db.QueryRow` wrappers.
- `ReadFromInformationSchema(db *pg.DB) (*SchemaState, error)` — Reads from `information_schema.tables`, `information_schema.columns`, `pg_indexes`, `information_schema.table_constraints`/`key_column_usage`/`constraint_column_usage`. Builds relationships from FK constraints.
- `pgTypesMatch(desired, current string) bool` — Normalizes Postgres type aliases (int/integer, bool/boolean, timestamp variants, varchar/character varying).
- `compareFieldConstraints(entityName string, desired model.Field, current *FieldState) []DiffItem` — Compares `MaxLength` (with `> 0` check) and nullable.
- `buildIndexName(entityName string, idx model.Index) string` — Uses `idx.Description` for explicit name, computes from fields otherwise.
- `parseIndexFields(indexDef string) []string` — Extracts column names from `CREATE INDEX` DDL string.

### Cross-References
- Uses `buildConstraintMap` (validator.go — canonical copy)
- Uses `vocabulary.GetPostgresType`
- Uses `pg.DB`, `db.Query`, `db.QueryRow` (not `db.Pool` directly)
- Uses `model.Schema`, `model.Entity`, `model.Field`, `model.Index`

### IOSE Deviations
- **Uses `db.Query`/`db.QueryRow` wrappers instead of `db.Pool.Query`.** The naive version used `db.Pool.Query`/`db.Pool.QueryRow` with manual context management. The rewrite uses the `pg.DB` wrapper methods which handle context internally. Functionally equivalent.
- **`indexFieldNames` helper removed.** Was just `return idx.Fields`. Inlined.
- **Duplicate `buildConstraintMap` removed.** Uses validator.go's canonical copy.

---

## File: loader/evolution.go

### Purpose
Classifies schema diff changes as allowed or forbidden per evolution rules.

### Types Defined

**`EvolutionResult`** — `Allowed []AllowedChange`, `Forbidden []ForbiddenChange`

**`AllowedChange`** — `Entity`, `Field`, `ChangeType`, `Description string`, `DiffItem DiffItem`

**`ForbiddenChange`** — `Entity`, `Field`, `Rule`, `Reason`, `Alternative string`, `DiffItem DiffItem`

### Functions

- `CheckEvolution(diff *SchemaDiff) (*EvolutionResult, error)` — Classifies each diff item:
  - New entities → always allowed
  - New fields → always allowed (database ALTER enforces nullable requirement)
  - Changed constraints → `classifyConstraintChange`
  - New indexes → always allowed
  - Removed fields → always forbidden (deprecation alternative)
  - Removed entities → always forbidden (deprecation alternative)
  - Type changes → always forbidden (detects renames via `isFieldRename`)

- `classifyConstraintChange(result, item)` — Classifies by description content:
  - `max_length`: widening (≥) allowed, narrowing forbidden
  - `min_value`/`max_value`: widening allowed via `isRangeWidening`, narrowing forbidden
  - `nullable`: making nullable (widening) allowed, making non-nullable (tightening) forbidden
  - Unclassifiable: treated as allowed with description passthrough

- `isFieldRename(removed DiffItem, diff *SchemaDiff) bool` — Heuristic: field disappeared + new field with same type in same entity.
- `isRangeWidening(desc string, desired, current float64) bool` — Min: desired ≤ current. Max: desired ≥ current.
- `isRangeNarrowing(item DiffItem) bool` — Inverse of widening. Currently unused but retained as useful helper.
- `isEnumValueRemoval(item DiffItem) bool` — Checks if any current enum value missing from desired. Currently unused but retained.
- `formatForbiddenMessage(change ForbiddenChange) string` — Multi-line human-readable error.
- `toNumeric(v interface{}) (float64, bool)` — Converts `float64`, `float32`, `int`, `int64`, `int32`.
- `toStringSet(v interface{}) ([]string, bool)` — Converts `[]string`, `[]interface{}`.

### Cross-References
- Uses `SchemaDiff`, `DiffItem` (differ.go)
- Consumed by cmd/main.go (`evolution.Allowed`, `evolution.Forbidden`)
- `AllowedChange` consumed by generator.go and applier.go

### IOSE Deviations
- None. Matches IOSE. `isRangeNarrowing` and `isEnumValueRemoval` are unused but described in the IOSE as classification helpers.

---

## File: loader/generator.go

### Purpose
Generates Postgres DDL from allowed changes. Pure generation — no database interaction.

### Types Defined

**`DDLStatement`** — `SQL`, `Entity`, `Description string`, `Phase int`
- Phase 1: CREATE TABLE, ALTER TABLE ADD COLUMN
- Phase 2: FK constraints, UNIQUE constraints, CHECK constraints
- Phase 3: CREATE INDEX
- Phase 4: REVOKE for append-only

### Functions

- `GenerateDDL(schema *model.Schema, changes []AllowedChange) ([]DDLStatement, error)` — Orchestrates generation in phase order. New entities get full CREATE TABLE + constraints + indexes + REVOKE. New fields on existing entities get ALTER TABLE ADD COLUMN + constraints. Final `OrderByDependency` sorts by phase then load order.
- `generateCreateTable(entity *model.Entity) DDLStatement` — Full CREATE TABLE with columns, NOT NULL, DEFAULT, IDENTITY for `id`, inline CHECK for enums. Uses `buildConstraintMap` (validator.go) and `vocabulary.GetPostgresType`.
- `generateAlterAddColumn(entityName string, field *model.Field) DDLStatement`
- `generateFKConstraint(entityName string, field *model.Field) DDLStatement` — `fk_{entity}_{field}` naming.
- `generateUniqueConstraint(entityName string, field *model.Field) DDLStatement` — `uq_{entity}_{field}` naming.
- `generateCompositeUnique(entityName string, fields []string) DDLStatement` — `uq_{entity}_{fields joined}` naming.
- `generateCheckConstraint(entityName string, field *model.Field) DDLStatement` — Numeric ranges. Dereferences `*field.MinValue`/`*field.MaxValue`. Returns empty `DDLStatement` if no conditions.
- `generateIndex(entityName string, index *model.Index) DDLStatement` — Uses `index.Description` for explicit name, computes from fields otherwise. `idx_` or `uq_` prefix.
- `generateRevokeAppendOnly(entityName string, roles []string) []DDLStatement` — REVOKE UPDATE, DELETE for each role.
- `OrderByDependency(statements []DDLStatement, loadOrder []string) []DDLStatement` — Stable sort by (phase, position in load order).
- `formatDefault(fieldType string, value interface{}) string` — Converts Go values to SQL literals. Boolean TRUE/FALSE, numeric passthrough, string/enum/date quoted with escaping.
- `escapeSQLString(s string) string` — Single quote escaping.

### Cross-References
- Uses `buildConstraintMap` (validator.go — canonical copy)
- Uses `vocabulary.GetPostgresType`
- Uses `model.Schema`, `model.Entity`, `model.Field`, `model.Index`
- Uses `AllowedChange` (evolution.go)
- `DDLStatement` consumed by applier.go

### IOSE Deviations
- **Duplicate `buildConstraintMapFromField` removed.** Was identical to `buildConstraintMap` in validator.go. All call sites now use the canonical copy.
- **`generateCheckConstraint` dereferences pointers.** The naive version used `%v` on `field.MinValue` (a `*float64`), which would print the pointer address. Fixed to dereference with `*field.MinValue`.
- **`index.Name` → `index.Description`.** Model has no `Name` field on `Index`. All index name lookups use `Description`.

---

## File: loader/applier.go

### Purpose
Executes DDL against Postgres. Advisory lock → transaction → execute → commit/rollback.

### Types Defined

**`ApplyResult`** — `StatementsExecuted`, `EntitiesCreated`, `FieldsAdded`, `ConstraintsModified`, `IndexesCreated int`, `Duration time.Duration`, `DryRun bool`

### Functions

- `Apply(db *pg.DB, statements []DDLStatement, verbose bool) (*ApplyResult, error)` — Advisory lock → serializable transaction → execute each statement → commit. Verbose mode prints DDL.
- `DryRun(db *pg.DB, statements []DDLStatement) (*ApplyResult, error)` — Advisory lock → transaction → execute → rollback. Validates DDL without persisting.
- `classifyStatement(result *ApplyResult, stmt DDLStatement)` — Increments counters by phase and description prefix.

### Cross-References
- Uses `pg.WaitForAdvisoryLock`, `pg.SchemaApplyLockID`, `pg.ReleaseAdvisoryLock`
- Uses `pg.WithSerializableTransaction`, `pg.RollbackOnly`
- Uses `pg.ExecInTx`
- Uses `DDLStatement` (generator.go)

### IOSE Deviations
- None. Clean file — no changes from naive. All `pg` signatures match.

---

## File: loader/meta.go

### Purpose
Populates `_schema_*` tables after DDL apply. Runs within transaction.

### Functions

- `PopulateMeta(tx *pg.Tx, schema *model.Schema, changes []AllowedChange, label string) error` — Five steps: insert version → mark previous non-current → upsert entity types → upsert fields → upsert relationships.
- `insertSchemaVersion(tx *pg.Tx, label string) (int, error)` — Finds current parent version (if any), inserts new row with `is_current = true`.
- `upsertEntityTypes(tx *pg.Tx, schema *model.Schema, versionID int) error` — Insert or update `_schema_entity_type` rows for all entities.
- `upsertFields(tx *pg.Tx, schema *model.Schema, versionID int) error` — Looks up entity type ID, builds constraint JSON via `buildFieldConstraintJSON`, insert or update `_schema_field` rows.
- `upsertRelationships(tx *pg.Tx, schema *model.Schema, versionID int) error` — Insert or update `_schema_relationship` rows.
- `markDeprecated(tx *pg.Tx, fieldName, entityName string, versionID int) error` — Sets `_schema_version_deprecated_id`. Handles whole entity (fieldName empty) or single field. Defined but not yet called from `PopulateMeta`.
- `buildFieldConstraintJSON(field model.Field) (string, error)` — Serializes field constraints to JSON string. `MaxLength`/`MinLength` checked with `> 0`. `MaxValue`/`MinValue` dereferenced from `*float64`.

### Cross-References
- Uses `pg.ExecInTx`, `pg.QueryRowInTx`
- Uses `model.Schema`, `model.Entity`, `model.Field`, `model.Relationship`
- Uses `AllowedChange` (evolution.go)

### IOSE Deviations
- **`markDeprecated` defined but never called.** The IOSE describes deprecation as part of the meta population flow. The function exists and is correct but `PopulateMeta` doesn't call it — it only handles adds and updates, not deprecations. Wiring it in would require passing the `SchemaDiff` (specifically `RemovedFields` and `RemovedEntities`) into `PopulateMeta`. Noted for future integration.

---

## File: loader/loader_test.go

### Purpose
Unit and integration tests for the loader package.

### Test Categories

**Parser tests:** `TestParseMinimalEntity`, `TestParseEntityWithAllTypes`, `TestParseDirectoryYAML`

**Validation tests:** `TestValidateRejectsReservedFieldNames`, `TestForbiddenRegex`, `TestForbiddenInheritance`, `TestForbiddenLogic`, `TestForbiddenTemplating`

**Resolver tests:** `TestResolverTopologicalSort`, `TestResolverDetectsCycles`, `TestResolverThreeEntityChain`

**Injector tests:** `TestInjectorUniversalFields`, `TestInjectorVersioningSibling` (checks `IsSibling`, `ParentEntity`), `TestInjectorHierarchical`, `TestInjectorGovernanceFields`

**Evolution tests:** `TestEvolutionAllowsNewEntity`, `TestEvolutionAllowsNewNullableField`, `TestEvolutionForbidsFieldDeletion`, `TestEvolutionForbidsEntityDeletion`, `TestEvolutionForbidsTypeChange`, `TestEvolutionDetectsRename`, `TestEvolutionAllowsWideningRange`, `TestEvolutionForbidsNarrowingRange`

**Generator tests:** `TestGeneratorCreateTable` (MaxLength as plain int), `TestGeneratorFKConstraint`, `TestGeneratorAppendOnlyRevoke`, `TestGeneratorOrderByDependency`

**Integration tests** (require Postgres, skipped with `-short`): `TestFullLoadPipeline`, `TestApplyToPostgres`, `TestDryRunDoesNotPersist`, `TestMetaPopulation`, `TestFullApplyAndDiffCycle`

### Cross-References
- Uses `model.Schema`, `model.Entity`, `model.Field`
- Uses `pg.Connect`, `pg.WithTransaction`
- Uses `testutil.SchemaRepoDir`, `testutil.SchemaRepoDirOrdered`, `testutil.StartTestPostgres`, `testutil.ResetTestDB`, `testutil.TableExists`, `testutil.QueryScalarInt`
- Uses `testutil.MinimalValidEntity`, `testutil.EntityWithAllTypes`, `testutil.EntityWithReservedFieldCollision`, `testutil.EntityWithForbiddenRegex`, `testutil.EntityWithForbiddenInheritance`, `testutil.EntityWithForbiddenLogic`, `testutil.EntityWithForbiddenTemplating`, `testutil.VersionedEntity`, `testutil.HierarchicalEntity`, `testutil.EntityWithGovernance`, `testutil.TwoEntitiesWithFK`, `testutil.CyclicEntities`, `testutil.ThreeEntityChain`
- Uses `vocabulary.ScanForForbiddenPatterns`

### IOSE Deviations / Known Issues
- **`testutil` functions may not exist.** The following are called but may need to be added to `internal/testutil/`:
  - `SchemaRepoDirOrdered(t, map[string]string, []string) string` — creates temp repo with multiple entities in specified order
  - `EntityWithReservedFieldCollision() string` — entity YAML with a field named `id` or similar
  - `EntityWithForbiddenTemplating() string` — entity YAML with `{{ }}` syntax
  - `ThreeEntityChain() (string, string, string)` — grandparent → parent → child
  - `TableExists(t, tdb, tableName) bool` — checks if table exists in test DB
  - `QueryScalarInt(t, tdb, query) int` — executes query returning single int
- **`MaxLength` in test fixtures uses plain `int`.** `TestGeneratorCreateTable` uses `MaxLength: 255` (not `&maxLen`). Matches the model.

---

## Integration Notes for Future Sessions

1. **`go.mod` must include `gopkg.in/yaml.v3`** — used by parser.go.
2. **`buildConstraintMap` is the single canonical copy** in validator.go. differ.go and generator.go call it without import (same package).
3. **`model.Index` has no `Name` field.** All code uses `Description` for optional explicit index names. Generator computes names from entity + fields when `Description` is empty.
4. **`model.Field.MaxLength` and `MinLength` are plain `int`** (0 = unset). All code checks `> 0`, never `!= nil`.
5. **`model.Field.MaxValue` and `MinValue` are `*float64`**. Parser converts via `yamlFloat64Ptr`. Generator dereferences in format strings. Meta serializer dereferences for JSON.
6. **`model.ReservedConfig` uses `Grants` not `Permissions`** on `RoleDefinition`. No `AppliesTo` field. Governance fields use `[]GovernanceFieldDef` with `EnabledBy`, not a map.
7. **`model.SchemaError` used everywhere.** No local `SchemaError` type in the loader package.