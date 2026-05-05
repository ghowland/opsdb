# OpsDB Internal Package and Tool File Reference

## internal/

### internal/model/

#### entity.go
**Purpose:** In-memory representation of a parsed entity definition. The central data structure the schema engine works with.

**Struct fields:** Name, Description, Category, Versioned (bool), SoftDelete (bool), Hierarchical (bool), AppendOnly (bool), Fields ([]Field), Indexes ([]Index), Governance (map of enabled governance fields).

**Inputs:** Populated by the parser from YAML. Consumed by every subsequent stage.

**Outputs:** Read by validator, resolver, injector, generator, differ, evolution checker, meta populator.

**Side effects:** None. Pure data structure.

#### field.go
**Purpose:** In-memory representation of a single field within an entity. Carries type, constraints, modifiers, and metadata.

**Struct fields:** Name, Type (one of nine), Nullable (bool), Description, Default (literal value), Unique (bool), References (target entity name for FK), MaxLength, MinLength, MaxValue, MinValue, PrecisionDecimalPlaces, EnumValues ([]string), JsonTypeDiscriminator (sibling field name), MustBeUniqueWithin ([]string field names), IsReserved (bool, set by injector), IsGovernance (bool, set by injector).

**Inputs:** Populated by parser from YAML field definitions. Augmented by injector for reserved and governance fields.

**Outputs:** Read by validator, generator, differ, evolution checker.

**Side effects:** None. Pure data structure.

#### relationship.go
**Purpose:** In-memory representation of a foreign key relationship between two entities. Used for dependency resolution and _schema_relationship population.

**Struct fields:** SourceEntity (string), SourceField (string), TargetEntity (string), Cardinality (one_to_one, one_to_many, many_to_many), OnDeleteAction (cascade, restrict, set_null), IsSelfReferential (bool).

**Inputs:** Derived by resolver from FK fields in parsed entities.

**Outputs:** Read by resolver for topological sort, by generator for FK constraint DDL, by meta populator for _schema_relationship rows.

**Side effects:** None. Pure data structure.

#### schema.go
**Purpose:** Complete in-memory representation of the full schema. Container for all entities, relationships, and metadata. The top-level structure passed through the pipeline.

**Struct fields:** Entities (map[string]*Entity, keyed by name), Relationships ([]Relationship), LoadOrder ([]string, topologically sorted entity names), Version (schema version metadata), Errors ([]SchemaError, accumulated validation errors).

**Inputs:** Built incrementally by parser, validator, resolver, injector.

**Outputs:** Consumed by differ, evolution checker, generator, applier, meta populator.

**Side effects:** None. Pure data structure.

---

### internal/conventions/

#### naming.go
**Purpose:** Validates names against the spec's naming conventions. Lowercase with underscores, singular form, proper FK naming, prefix rules for governance fields, suffix rules for datetime and date fields, boolean prefix rules.

**Functions:**
- `ValidateEntityName(name string) error` — checks lowercase_underscore, singular, length, no leading underscore unless schema meta.
- `ValidateFieldName(name string, fieldType string) error` — checks lowercase_underscore, length, datetime suffix, date suffix, boolean prefix (is_/was_), governance underscore prefix.
- `ValidateFKName(fieldName string, referencedEntity string) error` — checks field follows `{referenced_table}_id` pattern, allows role prefix.
- `ValidateCompositeName(name string) error` — checks hierarchical prefix pattern, no double underscores, no trailing underscores.

**Inputs:** Name strings from parsed entities and fields.

**Outputs:** Error or nil.

**Side effects:** None. Pure validation.

#### reserved.go
**Purpose:** Defines the set of reserved field names and provides lookup functions. Loaded from conventions/reserved.yaml at startup. Used by the validator to reject entity files that declare reserved fields, and by the injector to know what to inject.

**Functions:**
- `LoadReserved(path string) (*ReservedConfig, error)` — parses conventions/reserved.yaml into structured config.
- `IsReservedFieldName(name string, entityName string) bool` — checks if a field name is reserved, accounting for entity-name-templated names like `parent_{entity}_id`.
- `GetUniversalFields() []Field` — returns id, created_time, updated_time field definitions.
- `GetSoftDeleteFields() []Field` — returns is_active.
- `GetHierarchicalFields(entityName string) []Field` — returns parent_{entity}_id.
- `GetVersioningSiblingFields(entityName string) []Field` — returns sibling table fields.
- `GetGovernanceFields(enabled map[string]bool) []Field` — returns governance fields for enabled flags.
- `GetDatabaseRoles() []RoleDefinition` — returns opsdb_app_role, opsdb_admin_role, opsdb_readonly_role, opsdb_runner_role definitions.

**Inputs:** YAML file path, entity names, governance enable flags.

**Outputs:** Field definitions, role definitions, boolean lookups.

**Side effects:** File I/O when loading YAML.

---

### internal/vocabulary/

#### types.go
**Purpose:** Defines the nine allowed field types and their properties. Mapping from schema type names to Postgres type strings, allowed constraints per type, allowed modifiers per type, required constraints per type.

**Functions:**
- `IsValidType(typeName string) bool` — checks if type is one of the nine.
- `GetPostgresType(typeName string, constraints map[string]interface{}) string` — returns Postgres DDL type string. VARCHAR needs max_length, others are fixed.
- `GetAllowedConstraints(typeName string) []string` — returns which constraints this type permits.
- `GetRequiredConstraints(typeName string) []string` — returns which constraints this type requires.
- `GetAllowedModifiers(typeName string) []string` — returns which modifiers this type permits.
- `GetForbiddenModifiers(typeName string) []string` — returns which modifiers this type forbids (default on FK/datetime/json).

**Inputs:** Type name strings, constraint maps.

**Outputs:** Postgres type strings, string slices of allowed/required/forbidden items, booleans.

**Side effects:** None. Pure lookup.

#### modifiers.go
**Purpose:** Defines the three modifiers (nullable, default, unique) and their validation rules.

**Functions:**
- `IsValidModifier(modName string) bool` — checks if modifier is one of three.
- `ValidateDefault(fieldType string, defaultValue interface{}) error` — validates that a default value is a literal appropriate for the field type. Rejects expressions, function calls, computed values.
- `ValidateUnique(fieldType string) error` — checks if unique is permitted for this type (forbidden on FK).
- `ValidateMustBeUniqueWithin(fieldNames []string, entityFields []string) error` — validates composite uniqueness scope references real fields.

**Inputs:** Modifier names, field types, default values, field name lists.

**Outputs:** Errors or nil.

**Side effects:** None. Pure validation.

#### constraints.go
**Purpose:** Defines the six-plus constraints and their validation rules per type.

**Functions:**
- `ValidateConstraints(fieldType string, constraints map[string]interface{}) error` — master validator. Checks that only allowed constraints for this type are present, required constraints are present, values are valid.
- `ValidateNumericRange(minValue, maxValue interface{}) error` — checks min <= max.
- `ValidateStringLength(minLength, maxLength interface{}) error` — checks min <= max, max >= 1.
- `ValidateEnumValues(values []string) error` — checks non-empty, no duplicates, all lowercase_underscore.
- `ValidateReferences(targetEntity string, knownEntities map[string]bool) error` — checks referenced entity exists.
- `ValidateJsonDiscriminator(discriminatorField string, entityFields []string) error` — checks discriminator field exists in same entity and is enum type.
- `ValidatePrecision(places interface{}) error` — checks 0-15 range.

**Inputs:** Field types, constraint maps, entity field lists, known entity maps.

**Outputs:** Errors or nil.

**Side effects:** None. Pure validation.

#### forbidden.go
**Purpose:** Detects forbidden patterns in entity YAML files. Scans for regex, embedded logic, conditional constraints, inheritance, templating, imports, deletion markers, type change markers, rename markers.

**Functions:**
- `ScanForForbiddenPatterns(rawYAML map[string]interface{}) []ForbiddenViolation` — scans the raw parsed YAML map recursively for forbidden keys and value patterns.
- `CheckForRegex(value string) bool` — checks if a string contains regex metacharacters used as constraints.
- `CheckForEmbeddedLogic(value interface{}) bool` — checks for function calls, arithmetic operators, NOW(), CURRENT_TIMESTAMP, computed values.
- `CheckForInheritance(keys []string) bool` — checks for extends, inherits, parent_entity, base_class, abstract, mixin.
- `CheckForTemplating(value string) bool` — checks for {{ }}, {% %}, ${, $(.
- `CheckForImports(keys []string) bool` — checks for import, include, require, $ref.

**Inputs:** Raw YAML map from parser.

**Outputs:** Slice of ForbiddenViolation structs (pattern name, location in YAML, rationale, alternative).

**Side effects:** None. Pure detection.

---

### internal/pg/

#### conn.go
**Purpose:** Postgres connection management. Opens connections, validates DSN, manages connection pool.

**Functions:**
- `Connect(dsn string) (*DB, error)` — opens connection pool to Postgres, validates connectivity with a ping.
- `Close() error` — closes connection pool.
- `Ping() error` — tests connection liveness.
- `DSNFromEnv(envVar string) (string, error)` — reads DSN from environment variable, validates format.

**Inputs:** DSN string or environment variable name.

**Outputs:** Database handle or error.

**Side effects:** Opens network connections to Postgres. Maintains connection pool.

#### tx.go
**Purpose:** Transaction helpers. Wraps database/sql transactions with begin/commit/rollback and error handling.

**Functions:**
- `WithTransaction(db *DB, fn func(tx *Tx) error) error` — begins transaction, calls fn, commits on success, rolls back on error or panic. Returns fn's error.
- `ExecInTx(tx *Tx, query string, args ...interface{}) (sql.Result, error)` — executes a statement within a transaction.
- `QueryInTx(tx *Tx, query string, args ...interface{}) (*sql.Rows, error)` — executes a query within a transaction.

**Inputs:** Database handle, function to execute, SQL strings, arguments.

**Outputs:** Transaction results or errors.

**Side effects:** Database reads and writes within transaction scope. Commit or rollback.

#### advisory_lock.go
**Purpose:** Postgres advisory locks for concurrent apply safety. Prevents two concurrent schema applies from colliding.

**Functions:**
- `AcquireAdvisoryLock(db *DB, lockID int64) (bool, error)` — attempts to acquire a session-level advisory lock. Returns true if acquired, false if already held by another session.
- `WaitForAdvisoryLock(db *DB, lockID int64, timeout time.Duration) error` — blocks until lock is acquired or timeout. Used by apply command.
- `ReleaseAdvisoryLock(db *DB, lockID int64) error` — releases the advisory lock.
- `SchemaApplyLockID() int64` — returns the fixed lock ID used for schema apply operations.

**Inputs:** Database handle, lock ID, timeout duration.

**Outputs:** Boolean acquired, errors.

**Side effects:** Acquires and releases Postgres advisory locks. Blocks on contention.

---

### internal/testutil/

#### pg.go
**Purpose:** Test Postgres instance management. Starts a Postgres container via testcontainers for integration tests, provides connection strings, handles cleanup.

**Functions:**
- `StartTestPostgres(t *testing.T) (*TestDB, error)` — starts a Postgres testcontainer, waits for readiness, returns a handle with DSN.
- `StopTestPostgres(tdb *TestDB) error` — stops and removes the container.
- `ResetTestDB(tdb *TestDB) error` — drops all tables and recreates from empty. Used between test cases.
- `DSN() string` — returns the connection string for the test instance.

**Inputs:** testing.T for cleanup registration.

**Outputs:** TestDB handle with DSN.

**Side effects:** Starts and stops Docker containers. Creates and drops databases.

#### fixtures.go
**Purpose:** Test schema fragments. Provides minimal valid entity YAML strings and pre-built Entity structs for unit tests. Includes known-good entities, known-bad entities (one per forbidden pattern), and edge cases.

**Functions:**
- `MinimalValidEntity() string` — returns YAML string for simplest valid entity (name, category, one varchar field).
- `EntityWithAllTypes() string` — returns YAML with one field of each of the nine types.
- `EntityWithForbiddenRegex() string` — returns YAML containing regex pattern.
- `EntityWithForbiddenInheritance() string` — returns YAML containing extends keyword.
- `EntityWithForbiddenLogic() string` — returns YAML containing NOW() default.
- `VersionedEntity() string` — returns YAML with versioned: true.
- `HierarchicalEntity() string` — returns YAML with hierarchical: true.
- `EntityWithGovernance() string` — returns YAML with all governance fields enabled.
- `TwoEntitiesWithFK() (string, string)` — returns parent and child YAML where child has FK to parent.
- `CyclicEntities() (string, string)` — returns two entities with circular FK references.
- `SchemaRepoDir(t *testing.T, entities ...string) string` — writes entity YAML strings to a temp directory with proper directory.yaml and meta-schema, returns path.

**Inputs:** None (factory functions) or testing.T and YAML strings for temp dir creation.

**Outputs:** YAML strings, Entity structs, temporary directory paths.

**Side effects:** File system writes for temp directories. Cleaned up by testing.T.

---

## tools/opsdb-schema/

### cmd/main.go
**Purpose:** CLI entrypoint for the opsdb-schema binary. Parses command-line flags (command, repo path, DSN, scope, verbose, dry-run), dispatches to the appropriate loader function.

**Inputs:** Command-line arguments: command (init, validate, plan, apply, diff, export), --repo path, --dsn connection string, --scope entity or entity/field, --verbose flag, --version flag.

**Outputs:** Stdout for plan/diff/validate output. Exit code 0 on success, 1 on validation/evolution error, 2 on runtime error.

**Side effects:** Depending on command — file creation (init), database modification (apply), stdout output (plan, diff, validate, export).

### loader/loader.go
**Purpose:** Orchestrates the full schema loading pipeline. Calls parser, validator, resolver, injector, and returns a complete Schema. Entry point for the loading half of the process (before database interaction).

**Functions:**
- `Load(repoPath string) (*Schema, error)` — reads directory.yaml, loads meta-schema and conventions, processes each entity file in order, runs validation, resolves dependencies, injects reserved fields, returns complete Schema.

**Inputs:** Repository directory path.

**Outputs:** Fully populated Schema struct or error.

**Side effects:** File I/O reading YAML files from the repository.

### loader/parser.go
**Purpose:** Parses entity YAML files into raw maps and then into Entity structs. Handles YAML syntax, key extraction, field list parsing.

**Functions:**
- `ParseEntityFile(path string) (*Entity, map[string]interface{}, error)` — reads and parses YAML file. Returns both the structured Entity and the raw map (raw map used by forbidden pattern scanner). Validates top-level keys against allowed set.
- `ParseDirectoryYAML(path string) ([]string, error)` — reads directory.yaml and returns ordered list of entity file paths.
- `ParseMetaSchema(path string) (*MetaSchema, error)` — reads and parses _schema_meta.yaml.
- `ParseReserved(path string) (*ReservedConfig, error)` — reads and parses reserved.yaml.

**Inputs:** File paths.

**Outputs:** Entity structs, raw YAML maps, file path lists, MetaSchema and ReservedConfig structs.

**Side effects:** File I/O.

### loader/validator.go
**Purpose:** Validates parsed entities against the meta-schema. Checks types, constraints, modifiers, naming conventions, forbidden patterns. Accumulates errors rather than failing on first.

**Functions:**
- `Validate(entity *Entity, rawYAML map[string]interface{}, metaSchema *MetaSchema, knownEntities map[string]bool) []SchemaError` — runs all validation checks on one entity. Returns accumulated errors.
- `validateFields(fields []Field, metaSchema *MetaSchema, knownEntities map[string]bool) []SchemaError` — validates each field's type, constraints, modifiers, naming.
- `validateIndexes(indexes []Index, fields []Field) []SchemaError` — validates index field references exist.
- `validateGovernance(governance map[string]bool, metaSchema *MetaSchema) []SchemaError` — validates governance keys are recognized.

**Inputs:** Entity struct, raw YAML map, meta-schema, set of already-loaded entity names.

**Outputs:** Slice of SchemaError (field, message, severity).

**Side effects:** None. Pure validation.

### loader/resolver.go
**Purpose:** Resolves foreign key references between entities and builds the dependency graph. Detects cycles. Produces topological sort order.

**Functions:**
- `Resolve(schema *Schema) error` — processes all FK fields across all entities, creates Relationship structs, builds adjacency graph, runs topological sort, stores result in schema.LoadOrder. Returns error on cycle.
- `BuildDependencyGraph(entities map[string]*Entity) (map[string][]string, error)` — creates adjacency list from FK references. Self-FK edges noted but excluded from sort graph.
- `TopologicalSort(graph map[string][]string) ([]string, error)` — Kahn's algorithm. Returns ordered list or error if cycle detected.
- `DetectCycles(graph map[string][]string) [][]string` — returns all cycles if any exist, for error reporting.

**Inputs:** Schema struct with parsed entities.

**Outputs:** Schema.LoadOrder populated, Schema.Relationships populated. Error on cycles.

**Side effects:** None. Modifies Schema struct in place.

### loader/injector.go
**Purpose:** Injects reserved fields, governance fields, and versioning sibling definitions into entities based on their flags. Runs after validation and resolution.

**Functions:**
- `Inject(schema *Schema, reserved *ReservedConfig) error` — processes each entity: injects universal fields, injects soft_delete/hierarchical/governance fields per flags, generates versioning sibling entity definitions for versioned entities, adds siblings to schema.
- `injectUniversalFields(entity *Entity) error` — adds id, created_time, updated_time.
- `injectSoftDelete(entity *Entity) error` — adds is_active.
- `injectHierarchical(entity *Entity) error` — adds parent_{name}_id.
- `injectGovernanceFields(entity *Entity, enabled map[string]bool) error` — adds enabled governance fields.
- `generateVersioningSibling(entity *Entity) *Entity` — creates the {name}_version entity with versioning fields.

**Inputs:** Schema struct, ReservedConfig.

**Outputs:** Schema modified in place — entities have additional fields, new sibling entities added.

**Side effects:** None beyond Schema mutation.

### loader/differ.go
**Purpose:** Compares desired state (Schema from YAML) against current state (from _schema_* tables or information_schema). Produces a diff of changes needed.

**Functions:**
- `Diff(desired *Schema, current *SchemaState) (*SchemaDiff, error)` — compares entities, fields, constraints, indexes. Classifies each difference as new entity, new field, changed constraint, new index, or potentially forbidden change.
- `ReadCurrentState(db *DB) (*SchemaState, error)` — reads current database schema from _schema_entity_type, _schema_field, _schema_relationship tables. Falls back to information_schema if _schema_* tables don't exist.
- `ReadFromInformationSchema(db *DB) (*SchemaState, error)` — reads current schema from Postgres information_schema. Bootstrap path.

**Inputs:** Desired Schema struct, database handle.

**Outputs:** SchemaDiff struct containing lists of additions, modifications, and potentially forbidden changes.

**Side effects:** Database reads (SELECT queries against _schema_* or information_schema).

### loader/evolution.go
**Purpose:** Checks a SchemaDiff against the evolution rules. Classifies each change as allowed or forbidden. Produces clear error messages for forbidden changes with alternative guidance.

**Functions:**
- `CheckEvolution(diff *SchemaDiff) (*EvolutionResult, error)` — iterates through diff items, checks each against allowed and forbidden rules. Returns result containing allowed changes (to apply) and forbidden changes (to reject).
- `isFieldDeletion(change DiffItem) bool`
- `isFieldRename(change DiffItem, diff *SchemaDiff) bool` — heuristic: field gone + new field with same type in same entity.
- `isTypeChange(change DiffItem) bool`
- `isRangeNarrowing(change DiffItem) bool`
- `isEnumValueRemoval(change DiffItem) bool`
- `isUniquenessAdded(change DiffItem) bool`
- `formatForbiddenError(change DiffItem, rule ForbiddenRule) string` — produces the error message with entity, field, rule name, and alternative approach.

**Inputs:** SchemaDiff struct.

**Outputs:** EvolutionResult struct with allowed and forbidden change lists.

**Side effects:** None. Pure classification.

### loader/generator.go
**Purpose:** Generates Postgres DDL from the allowed changes in an EvolutionResult. Produces CREATE TABLE, ALTER TABLE, CREATE INDEX, CHECK constraints, FK constraints, REVOKE statements.

**Functions:**
- `GenerateDDL(schema *Schema, changes []AllowedChange) ([]DDLStatement, error)` — generates ordered DDL statements for all allowed changes.
- `generateCreateTable(entity *Entity) DDLStatement` — full CREATE TABLE with all columns, constraints, defaults.
- `generateAlterAddColumn(entity *Entity, field *Field) DDLStatement`
- `generateAlterColumnType(entity string, field string, newType string) DDLStatement`
- `generateCheckConstraint(entity string, field *Field) DDLStatement`
- `generateFKConstraint(entity string, field *Field) DDLStatement`
- `generateUniqueConstraint(entity string, field *Field) DDLStatement`
- `generateCompositeUnique(entity string, fields []string) DDLStatement`
- `generateIndex(entity string, index *Index) DDLStatement`
- `generateRevokeAppendOnly(entity string, roles []string) []DDLStatement`
- `OrderByDependency(statements []DDLStatement, loadOrder []string) []DDLStatement` — reorders DDL to respect FK dependencies.

**Inputs:** Schema struct, list of allowed changes.

**Outputs:** Ordered slice of DDLStatement structs (SQL string, entity name, change description).

**Side effects:** None. Pure generation.

### loader/applier.go
**Purpose:** Executes generated DDL against Postgres within a transaction. Acquires advisory lock, begins transaction, executes statements in order, commits or rolls back.

**Functions:**
- `Apply(db *DB, statements []DDLStatement, verbose bool) (*ApplyResult, error)` — acquires advisory lock, begins transaction, executes each statement, commits. Prints DDL if verbose. Returns result with counts.
- `DryRun(db *DB, statements []DDLStatement) (*ApplyResult, error)` — acquires lock, begins transaction, executes each statement, rolls back. Validates DDL is executable without persisting.

**Inputs:** Database handle, ordered DDL statements, verbose flag.

**Outputs:** ApplyResult struct (statements executed count, entities created, fields added, constraints modified).

**Side effects:** Acquires and releases advisory lock. Executes DDL against Postgres. Commits transaction on apply, rolls back on dry-run or error.

### loader/meta.go
**Purpose:** Populates _schema_* tables after DDL apply. Creates _schema_version row, upserts _schema_entity_type rows, upserts _schema_field rows, upserts _schema_relationship rows.

**Functions:**
- `PopulateMeta(db *DB, schema *Schema, changes []AllowedChange) error` — runs within the same transaction as apply. Creates new _schema_version row, updates is_current flags, inserts/updates entity type, field, and relationship rows.
- `insertSchemaVersion(tx *Tx, label string) (int, error)` — inserts new version row, returns ID.
- `upsertEntityTypes(tx *Tx, schema *Schema, versionID int) error`
- `upsertFields(tx *Tx, schema *Schema, versionID int) error`
- `upsertRelationships(tx *Tx, schema *Schema, versionID int) error`
- `markDeprecated(tx *Tx, entityOrField string, versionID int) error` — sets _schema_version_deprecated_id on deprecated entities or fields.

**Inputs:** Database handle (within transaction), Schema struct, list of changes applied.

**Outputs:** Error or nil.

**Side effects:** Inserts and updates rows in _schema_version, _schema_entity_type, _schema_field, _schema_relationship tables.

### loader/loader_test.go
**Purpose:** Unit and integration tests for the loader package. Tests validation, parsing, resolution, injection, evolution checking, DDL generation, and full apply cycle.

**Inputs:** Test fixtures from internal/testutil/fixtures.go. Test Postgres from internal/testutil/pg.go for integration tests.

**Outputs:** Test pass/fail.

**Side effects:** Creates and destroys test databases and temporary directories.

---

## tools/opsdb-api/

### cmd/main.go
**Purpose:** CLI entrypoint for the opsdb-api binary. Loads configuration, initializes auth provider, connects to database, loads runtime schema from _schema_* tables, starts HTTP server.

**Inputs:** --config path to DOS config.yaml. Environment variables for DSN and secrets.

**Outputs:** HTTP server listening on configured address.

**Side effects:** Opens database connections, starts HTTP listener, logs startup.

### gate/gate.go
**Purpose:** Orchestrates the 10-step gate pipeline. Receives an API request, creates a gate context, runs each step in order, short-circuits on rejection.

**Functions:**
- `ProcessRequest(req *GateRequest) *GateResponse` — runs steps 1-10 in order. Each step receives and returns GateContext. First rejection stops pipeline. Audit logging (step 8) runs on both success and rejection paths.

**Inputs:** GateRequest struct (HTTP request data, operation type, target entity, caller identity placeholder).

**Outputs:** GateResponse struct (success/error, result data, audit entry ID, rejection detail if rejected).

**Side effects:** Delegates to individual steps which may read from database and write audit entries.

### gate/step_auth.go
**Purpose:** Step 1: Authentication. Validates caller credentials, resolves to ops_user or runner_machine.

**Inputs:** GateContext with raw credentials (token, assertion).

**Outputs:** GateContext populated with resolved caller identity (ops_user_id or runner_machine_id or both).

**Side effects:** Calls auth provider which may make external calls (IdP, secret backend). Database reads for user/runner mapping.

### gate/step_authz.go
**Purpose:** Step 2: Authorization. Evaluates five layers. First denial halts.

**Inputs:** GateContext with caller identity, operation class, target entity type and ID.

**Outputs:** GateContext with authorization result. Denial includes which layer and which policy.

**Side effects:** Database reads for role memberships, group memberships, _requires_group, _access_classification, runner capabilities, runner targets, policy rows.

### gate/step_schema_validate.go
**Purpose:** Step 3: Schema validation. Checks operation shape against _schema_* metadata.

**Inputs:** GateContext with target entity type, field names, field values.

**Outputs:** GateContext with validation result. Rejection includes which fields failed and why.

**Side effects:** Reads from runtime schema cache (loaded from _schema_* tables at startup).

### gate/step_bound_validate.go
**Purpose:** Step 4: Bound validation. Checks field values against declared constraints.

**Inputs:** GateContext with field names and values, constraint metadata from runtime schema.

**Outputs:** GateContext with bound validation result. Rejection includes which field, what bound, what value.

**Side effects:** Database reads for FK existence checks.

### gate/step_policy.go
**Purpose:** Step 5: Policy evaluation. Evaluates semantic invariants, data classification, retention, separation of duty.

**Inputs:** GateContext with target entity, proposed field values, caller identity.

**Outputs:** GateContext with policy evaluation result. Rejection or warnings.

**Side effects:** Database reads for policy rows.

### gate/step_versioning.go
**Purpose:** Step 6: Versioning preparation. Prepares version sibling row for change-managed entities.

**Inputs:** GateContext with target entity (if versioned), current version data.

**Outputs:** GateContext with prepared version row (next version_serial, parent pointer, change_set reference).

**Side effects:** Database reads for current version.

### gate/step_changemgmt.go
**Purpose:** Step 7: Change management routing. Evaluates approval rules, walks ownership and stakeholder bridges, computes required approvals, determines auto-approve vs human approval.

**Inputs:** GateContext with change_set_field_change details, target entities, caller identity.

**Outputs:** GateContext with computed approval requirements (change_set_approval_required rows to write), auto-approve determination.

**Side effects:** Database reads for approval rules, ownership bridges, stakeholder bridges, auto-approval policies.

### gate/step_audit.go
**Purpose:** Step 8: Audit logging. Constructs and writes audit_log_entry. Atomic with operation outcome.

**Inputs:** GateContext with full operation details — caller, target, operation, result, timestamps.

**Outputs:** GateContext with audit_entry_id for correlation.

**Side effects:** INSERT into audit_log_entry table. This is the only write in the gate that occurs on both success and rejection paths.

### gate/step_execute.go
**Purpose:** Step 9: Execution. Performs the actual database write — entity insert/update, change_set creation, approval/rejection recording, observation write.

**Inputs:** GateContext with validated, authorized, routed operation.

**Outputs:** GateContext with execution result — affected row IDs, written version rows.

**Side effects:** Database writes — INSERT/UPDATE on target tables, change_set tables, version sibling tables. Within transaction started by gate orchestrator.

### gate/step_response.go
**Purpose:** Step 10: Response construction. Assembles the API response from GateContext.

**Inputs:** GateContext with all accumulated results from prior steps.

**Outputs:** GateResponse struct with result data, metadata, audit_entry_id, any warnings.

**Side effects:** None. Pure construction.

### auth/provider.go
**Purpose:** Auth provider interface. Defines the contract all auth backends implement.

**Interface:**
- `Authenticate(credentials Credentials) (*Identity, error)` — validates credentials, returns resolved identity.
- `RefreshToken(token string) (*Identity, error)` — refreshes an expiring token.
- `Type() string` — returns provider type name.

**Inputs/Outputs:** Defined by interface.

**Side effects:** Defined by implementation.

### auth/yaml_provider.go
**Purpose:** YAML file auth backend. Reads users.yaml, validates bcrypt-hashed passwords, resolves to ops_user mappings.

**Inputs:** Path to users.yaml file. Username and password from request.

**Outputs:** Identity struct with ops_user_id and role memberships.

**Side effects:** File I/O on startup to load users.yaml.

### auth/oidc_provider.go
**Purpose:** OIDC auth backend for production human authentication. Validates OIDC tokens, resolves claims to ops_user mappings.

**Inputs:** OIDC token from request. OIDC provider configuration (issuer, client_id, audience).

**Outputs:** Identity struct with ops_user_id and claims.

**Side effects:** HTTP calls to OIDC provider for token validation and JWKS retrieval. Caches JWKS.

### auth/serviceaccount_provider.go
**Purpose:** Service account token auth for runners. Validates tokens issued by secret backend, resolves to runner_machine mappings.

**Inputs:** Bearer token from request. Token validation configuration.

**Outputs:** Identity struct with runner_machine_id, runner_spec_id.

**Side effects:** May call secret backend for token validation depending on token type.

### operations/read.go
**Purpose:** Implements read operations: get_entity, get_entity_history, get_entity_at_time, search, get_dependencies.

**Functions:** One function per operation. Each constructs a database query from the gate context, executes it, and returns structured results. Search builds queries from filter predicates, join paths, projection, ordering, and pagination parameters.

**Inputs:** GateContext with operation parameters, runtime schema for field metadata.

**Outputs:** Result structs with entity rows, version chains, dependency walks, search results with cursor.

**Side effects:** Database reads.

### operations/write_observation.go
**Purpose:** Implements write_observation operation. Runner writes to observation cache tables, runner_job_output_var, or evidence_record.

**Functions:**
- `WriteObservation(ctx *GateContext) (*WriteResult, error)` — validates report key, writes to target table.

**Inputs:** GateContext with target table, key, value, runner identity.

**Outputs:** WriteResult with written row ID.

**Side effects:** Database INSERT or UPDATE (upsert for observation cache tables).

### operations/write_changeset.go
**Purpose:** Implements submit_change_set, emergency_apply, and bulk_submit_change_set operations.

**Functions:**
- `SubmitChangeSet(ctx *GateContext) (*ChangeSetResult, error)` — creates change_set, change_set_field_change, and change_set_approval_required rows. Handles dry-run mode.
- `EmergencyApply(ctx *GateContext) (*ChangeSetResult, error)` — submit with is_emergency=true and change_set_emergency_review creation.
- `BulkSubmit(ctx *GateContext) (*ChangeSetResult, error)` — chunked validation and submission.

**Inputs:** GateContext with field changes, reason, metadata, urgency.

**Outputs:** ChangeSetResult with change_set_id, computed approvals, dry-run results.

**Side effects:** Database writes for change_set, field_change, approval_required, emergency_review rows.

### operations/changeset_actions.go
**Purpose:** Implements approve, reject, cancel, apply_field_change, and mark_applied operations.

**Functions:**
- `ApproveChangeSet(ctx *GateContext) error` — records approval, increments fulfilled count, may transition status.
- `RejectChangeSet(ctx *GateContext) error` — records rejection, transitions status.
- `CancelChangeSet(ctx *GateContext) error` — transitions to cancelled.
- `ApplyFieldChange(ctx *GateContext) error` — executor applies one field change, writes version sibling.
- `MarkApplied(ctx *GateContext) error` — verifies all field changes applied, transitions to applied.

**Inputs:** GateContext with change_set_id, caller identity, optional comments/reason.

**Outputs:** Error or nil.

**Side effects:** Database writes for approval/rejection/status transitions, entity updates, version sibling inserts.

### operations/resolve.go
**Purpose:** Implements resolve_authority_pointer operation.

**Functions:**
- `ResolveAuthorityPointer(ctx *GateContext) (*ResolveResult, error)` — looks up authority_pointer row, returns authority connection details and locator.

**Inputs:** GateContext with authority_pointer_id.

**Outputs:** ResolveResult with authority base_url, pointer locator, metadata.

**Side effects:** Database reads.

### operations/watch.go
**Purpose:** Implements watch streaming operation. Long-poll or WebSocket subscription to entity changes.

**Functions:**
- `Watch(ctx *GateContext, callback func(event WatchEvent)) error` — establishes subscription, delivers events to callback. On reconnect, fetches current state then streams from resume token.

**Inputs:** GateContext with entity type, filter, resume token. Callback function.

**Outputs:** Stream of WatchEvent structs to callback.

**Side effects:** Maintains long-lived database connection or polling loop. Level-triggered backstop on reconnect.

### schema/runtime_schema.go
**Purpose:** Loads schema metadata from _schema_* tables at API startup. Provides runtime lookup for entity types, fields, constraints, relationships. Refreshes on schema version change.

**Functions:**
- `LoadRuntimeSchema(db *DB) (*RuntimeSchema, error)` — reads _schema_entity_type, _schema_field, _schema_relationship. Builds lookup maps.
- `Refresh(db *DB) error` — checks _schema_version.is_current against cached version. Reloads if changed.
- `GetEntityType(name string) (*EntityTypeMeta, bool)`
- `GetField(entityType string, fieldName string) (*FieldMeta, bool)`
- `GetRelationships(entityType string) []RelationshipMeta`

**Inputs:** Database handle.

**Outputs:** RuntimeSchema struct with lookup maps.

**Side effects:** Database reads. Caches in memory.

### reportkeys/enforcer.go
**Purpose:** Runner report key enforcement. Validates that a runner's write_observation call targets a declared key with valid values.

**Functions:**
- `Enforce(runnerSpecID int, targetTable string, key string, value interface{}, db *DB) error` — looks up runner_report_key rows for this runner+table, checks key is declared, validates value against report_key_data_json constraints. Returns nil on pass, structured error on rejection.
- `CacheDeclarations(runnerSpecID int, db *DB) error` — pre-loads report key declarations for a runner into memory cache.

**Inputs:** Runner spec ID, target table name, submitted key and value, database handle.

**Outputs:** Error or nil.

**Side effects:** Database reads for report key rows. Caches declarations.

### concurrency/optimistic.go
**Purpose:** Optimistic concurrency control for change set submission. Validates version stamps on field changes against current entity versions.

**Functions:**
- `ValidateVersionStamps(fieldChanges []FieldChange, db *DB) error` — for each field change, reads current version of target entity, compares against drafted-against version stamp. Returns stale_version error identifying which entities are stale.

**Inputs:** Slice of field changes with version stamps, database handle.

**Outputs:** Error (with stale entity details) or nil.

**Side effects:** Database reads for current entity versions.

### config/config.go
**Purpose:** API configuration loading. Reads DOS config.yaml, resolves DSN from environment, determines auth backend, sets listen address and TLS configuration.

**Functions:**
- `LoadConfig(path string) (*Config, error)` — reads config.yaml, resolves environment variables, validates required fields.

**Inputs:** Path to DOS config.yaml.

**Outputs:** Config struct with DSN, listen address, TLS paths, auth backend type and config path, schema repo path.

**Side effects:** File I/O. Environment variable reads.

---

## tools/opsdb-runner-lib/

### lifecycle.go
**Purpose:** Runner lifecycle helpers. Init, cycle management, shutdown, bound enforcement. Not a framework — the runner calls these, they don't call the runner.

**Functions:**
- `Init(runnerSpecName string) (*RunnerConfig, error)` — reads runner spec from OpsDB via API client, initializes logging with runner context, sets up bound tracking.
- `ShouldRun(config *RunnerConfig) bool` — checks if runner should continue (not shutting down, not past max cycles).
- `WaitForNextCycle(config *RunnerConfig) error` — sleeps for configured interval or until shutdown signal.
- `RecordBoundHit(config *RunnerConfig, boundName string, boundValue interface{})` — records which bound was hit for runner_job reporting.
- `Shutdown(config *RunnerConfig) error` — graceful shutdown. Completes current cycle, logs, exits.

**Inputs:** Runner spec name, RunnerConfig state.

**Outputs:** RunnerConfig struct, booleans, errors.

**Side effects:** API calls to read runner spec. Sleep. Signal handling.

### api_client.go
**Purpose:** OpsDB API client. Wraps all sixteen API operations with authentication, correlation ID propagation, retry, report key fail-fast, and structured error handling.

**Functions:** One function per API operation (GetEntity, GetEntityHistory, GetEntityAtTime, Search, GetDependencies, ResolveAuthorityPointer, ChangeSetView, WriteObservation, SubmitChangeSet, ApproveChangeSet, RejectChangeSet, CancelChangeSet, EmergencyApply, ApplyFieldChange, MarkApplied, Watch).

**Inputs:** Per-operation parameters. RunnerConfig for auth credentials and API endpoint.

**Outputs:** Per-operation result structs. Typed errors (ValidationFailed, AuthorizationDenied, StaleVersion, NotFound, BoundExceeded, NetworkError, InternalError).

**Side effects:** HTTP calls to OpsDB API. Report key cache lookups for fail-fast on WriteObservation.

### logging.go
**Purpose:** Structured logging with runner context. Every log line includes timestamp, severity, runner_job_id, correlation_id, runner spec name, runner spec version, runner_machine_id, source location.

**Functions:**
- `NewLogger(config *RunnerConfig) *Logger`
- `Info(msg string, fields ...Field)`
- `Warn(msg string, fields ...Field)`
- `Error(msg string, fields ...Field)`
- `Debug(msg string, fields ...Field)`
- `WithJobID(jobID int) *Logger` — returns logger with runner_job_id set for this cycle.

**Inputs:** RunnerConfig for context fields. Message and structured fields per log call.

**Outputs:** Structured log lines to configured destination (stdout, syslog, aggregator).

**Side effects:** I/O to log destination.

### retry.go
**Purpose:** Retry with exponential backoff, jitter, and idempotency key support. Composes with API client and world-side operations.

**Functions:**
- `WithRetry(config RetryConfig, fn func() error) error` — calls fn, retries on retryable errors with exponential backoff and jitter, up to max attempts or max duration.
- `WithIdempotencyKey(key string, fn func() error) error` — wraps fn with idempotency key header for API calls.
- `IsRetryable(err error) bool` — classifies error as retryable (network, 503, 429) or not (400, 401, 403, 404).
- `DefaultRetryConfig() RetryConfig` — returns default: 3 attempts, 1s base delay, 2x multiplier, 25% jitter, 30s max total.

**Inputs:** RetryConfig (max attempts, base delay, multiplier, jitter fraction, max total duration), function to retry.

**Outputs:** Error from last attempt or nil on success.

**Side effects:** Sleep between retries. Calls fn multiple times on retry.

### dryrun.go
**Purpose:** Dry-run mode support. Checks dry-run flag, logs planned actions instead of executing them.

**Functions:**
- `IsDryRun(config *RunnerConfig) bool` — checks if runner was invoked with dry_run=true.
- `LogPlan(logger *Logger, plan interface{})` — serializes planned action set to structured log output.

**Inputs:** RunnerConfig, planned action data.

**Outputs:** Boolean, log output.

**Side effects:** Log I/O in LogPlan.

### config.go
**Purpose:** Runner configuration from spec and environment. Reads runner_spec_version from OpsDB, merges with environment variables for credentials and endpoints.

**Functions:**
- `LoadRunnerConfig(specName string, apiEndpoint string) (*RunnerConfig, error)` — authenticates to API, reads runner_spec_version, parses runner_data_json, reads bounds, reads report key declarations, caches.
- `RefreshConfig(config *RunnerConfig) error` — re-reads spec from OpsDB. Called at start of each cycle for long-running runners.

**Inputs:** Runner spec name, API endpoint, environment variables for credentials.

**Outputs:** RunnerConfig struct with parsed spec, bounds, report keys, API client, logger.

**Side effects:** API calls. Environment variable reads.

---

## tools/runners/

### change-set-executor/executor.go
**Purpose:** Drains approved change sets. Reads change_sets with status=approved, applies each field change via API, marks change sets applied.

**Inputs (get):** change_set rows with status=approved. change_set_field_change rows per change set.

**Outputs (set):** Entity row updates via ApplyFieldChange. Change set status transitions via MarkApplied. runner_job row.

**Side effects:** API write calls for each field change application and status transition.

### schema-executor/executor.go
**Purpose:** Applies approved schema change sets. Reads _schema_change_set rows with approved status, runs opsdb-schema apply against the schema repo at the specified commit.

**Inputs (get):** _schema_change_set rows with approved status.

**Outputs (set):** Updated database schema via opsdb-schema apply. Updated _schema_* tables. runner_job row.

**Side effects:** Runs opsdb-schema binary or calls loader directly. Database DDL changes.

### reaper/reaper.go
**Purpose:** Enforces retention policies. Reads retention_policy rows, queries target tables for rows past retention horizon, deletes or soft-deletes.

**Inputs (get):** retention_policy rows. Target table rows with timestamps.

**Outputs (set):** Deleted rows from observation cache tables. Soft-deleted entity rows (is_active=false). runner_job row.

**Side effects:** API delete/update calls for expired rows.

### emergency-review-monitor/monitor.go
**Purpose:** Escalates overdue emergency reviews. Reads change_set_emergency_review rows where status=pending and review window elapsed.

**Inputs (get):** change_set_emergency_review rows. Change management rule for review window.

**Outputs (set):** compliance_finding rows for overdue reviews. Escalation notifications. runner_job row.

**Side effects:** API writes for findings. Notification dispatch through notification library.

### notification-runner/runner.go
**Purpose:** Reads state transitions requiring notification and dispatches through configured channels.

**Inputs (get):** Change sets entering pending_approval. Emergency changes filed. Compliance findings created. Escalation timeouts.

**Outputs (set):** Notification dispatch via backends. runner_job row recording what was sent.

**Side effects:** Email, webhook, or other notification delivery.

### notification-runner/backends/email.go
**Purpose:** Email notification backend. Sends email via SMTP.

**Inputs:** Recipient, subject, body, SMTP configuration.

**Outputs:** Delivery result.

**Side effects:** SMTP connection and email delivery.

### notification-runner/backends/webhook.go
**Purpose:** Webhook notification backend. Posts JSON payload to configured URL.

**Inputs:** URL, payload, optional headers.

**Outputs:** HTTP response status.

**Side effects:** HTTP POST to external URL.

---

## tools/importers/

All importers follow identical structure. Differences are in which authority they read from and how they map authority data to OpsDB schema.

### Common pattern per importer:

**cmd/main.go** — CLI entrypoint. Reads --dos flag for DOS config, initializes runner via opsdb-runner-lib, starts cycle loop.

**mapping.go** (where present) — Defines the mapping from authority data structures to OpsDB entity types, field names, and observation cache keys. Central place for DSNC flattening decisions.

**Per-resource files** (ec2.go, rds.go, pod.go, etc.) — Each file handles one resource type from the authority. Reads from authority API, transforms to schema shape, returns observation data ready to write.

### opsdb-import-aws/
- **ec2.go** — Reads EC2 instances. Maps to cloud_resource/ec2_instance observations.
- **rds.go** — Reads RDS instances. Maps to cloud_resource/rds_database observations.
- **s3.go** — Reads S3 buckets. Maps to cloud_resource/s3_bucket observations.
- **iam.go** — Reads IAM roles. Maps to cloud_resource/iam_role observations.
- **vpc.go** — Reads VPCs, subnets, security groups. Maps to cloud_resource/vpc observations.
- **route53.go** — Reads Route53 hosted zones. Maps to cloud_resource/route53_zone observations.

### opsdb-import-gcp/
- **gce.go** — GCE instances to cloud_resource/gce_instance.
- **cloudsql.go** — Cloud SQL to cloud_resource/cloud_sql_instance.
- **gcs.go** — GCS buckets to cloud_resource/gcs_bucket.
- **gke.go** — GKE clusters to both cloud_resource and k8s_cluster observations.
- **iam.go** — Service accounts to cloud_resource/service_account.

### opsdb-import-k8s/
- **cluster.go** — Cluster metadata to k8s_cluster observations.
- **node.go** — Nodes to k8s_cluster_node observations.
- **namespace.go** — Namespaces to k8s_namespace observations.
- **workload.go** — Deployments, StatefulSets, DaemonSets, Jobs, CronJobs to k8s_workload observations.
- **pod.go** — Pods to k8s_pod observations.
- **helm.go** — Helm releases to k8s_helm_release observations.
- **configmap.go** — ConfigMaps to k8s_config_map observations.
- **secret.go** — Secret metadata (never values) to k8s_secret_reference observations.
- **service.go** — K8s Service objects to k8s_service observations.
- **watcher.go** — Kubernetes watch API with level-triggered backstop. Full list on connect, incremental via watch, re-list on disconnect.

### opsdb-import-identity/
- **okta.go** — Okta users, groups, memberships to ops_user, ops_group, ops_group_member observations.
- **azuread.go** — Azure AD users, groups, memberships. Same target entities.
- **ldap.go** — LDAP directory users, groups, memberships. Same target entities.

### opsdb-import-monitoring/
- **prometheus.go** — Prometheus scrape configs, alert rules, metric metadata to prometheus_config, alert, observation_cache_metric.
- **datadog.go** — Datadog monitors, alert definitions to monitor, alert observations.

### opsdb-import-oncall/
- **pagerduty.go** — PagerDuty schedules, assignments, escalation policies to on_call_schedule, on_call_assignment, escalation_path, escalation_step.
- **opsgenie.go** — Opsgenie schedules and policies. Same target entities.

### opsdb-import-secrets/
- **vault.go** — Vault secret paths, metadata, rotation timestamps to authority_pointer observations. Never reads values.
- **aws_sm.go** — AWS Secrets Manager secret metadata. Same pattern. Never reads values.
