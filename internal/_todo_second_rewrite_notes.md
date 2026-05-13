# Internal Package Technical Specification

## Document Purpose

This document describes the current state of the `internal/` directory: what has been written, what each file does, how the packages compose, what types are exported, what functions are available, and how consuming code should use them. A developer (or LLM) reading this document should be able to use any internal package correctly without reading the source files.

---

## 1. Package Overview

```
internal/
├── model/
│   ├── entity.go          — Entity, Index, SchemaError structs
│   ├── field.go           — Field struct (all constraint/modifier fields)
│   ├── relationship.go    — Relationship struct (FK edges)
│   └── schema.go          — Schema, MetaSchema, ReservedConfig, and all sub-structs
├── conventions/
│   ├── naming.go          — Name validation (entities, fields, FKs, composites)
│   └── reserved.go        — Reserved field definitions, YAML loading, injection helpers
├── vocabulary/
│   ├── types.go           — Ten allowed field types, Postgres mappings, constraint/modifier rules
│   ├── constraints.go     — Constraint validation (numeric, string, enum, FK, JSON, precision)
│   ├── modifiers.go       — Modifier validation (default, unique, must_be_unique_within)
│   └── forbidden.go       — Forbidden pattern detection (regex, logic, inheritance, templating, etc.)
├── pg/
│   ├── conn.go            — DB struct, connection pool, Query, QueryRow, error classifiers, JSON helpers
│   ├── tx.go              — Tx struct, transaction helpers, ExecInTx, QueryInTx, QuoteIdentifier
│   └── advisory_lock.go   — Advisory lock acquire/wait/release for schema apply
└── testutil/
    ├── pg.go              — Test Postgres container management
    └── fixtures.go        — Test entity YAML fragments and pre-built structs
```

### Dependency graph between internal packages

```
model          — no internal dependencies (leaf)
conventions    — depends on model
vocabulary     — no internal dependencies (leaf, but modifiers.go calls forbidden.go within same package)
pg             — no internal dependencies (leaf)
testutil       — depends on pg
```

No circular dependencies exist. `conventions` imports `model` for struct types. All other packages are independent.

### External consumers

- `tools/opsdb-schema/` — uses `model`, `conventions`, `vocabulary`, `pg`, `testutil`
- `tools/opsdb-api/` — uses `pg` only (API uses its own `schema/runtime_schema.go` for runtime type data, not `model` or `vocabulary`)
- `tools/opsdb-runner-lib/` — does not use internal packages directly (communicates via API)

---

## 2. `internal/model/` — Data Structures

### Purpose

Pure data structures with no behavior. These are the types that flow through the schema engine pipeline. No methods, no validation logic, no I/O.

### 2.1 `entity.go`

**Entity** — one parsed entity definition from a YAML file.

| Field | Type | Description |
|---|---|---|
| Name | string | Entity name (lowercase_underscore, singular) |
| Description | string | Human-readable description |
| Category | string | Grouping category |
| Versioned | bool | Whether this entity has a version sibling table |
| SoftDelete | bool | Whether is_active field is injected |
| Hierarchical | bool | Whether parent_{name}_id self-FK is injected |
| AppendOnly | bool | Whether UPDATE/DELETE are revoked |
| Fields | []Field | All fields including injected reserved fields |
| Indexes | []Index | Declared indexes |
| Governance | map[string]bool | Which governance fields are enabled |
| IsSibling | bool | True if generated as a versioning sibling (set by injector) |
| ParentEntity | string | If IsSibling, the parent entity name |

**Index** — an index declaration.

| Field | Type | Description |
|---|---|---|
| Fields | []string | Column names in the index |
| Unique | bool | Whether this is a unique index |
| Description | string | Human-readable description |

**SchemaError** — a validation error with context.

| Field | Type | Description |
|---|---|---|
| Entity | string | Entity where error occurred |
| Field | string | Field where error occurred (empty for entity-level) |
| Message | string | Error description |
| Severity | string | "error" or "warning" |

### 2.2 `field.go`

**Field** — a single field within an entity.

| Field | Type | Description |
|---|---|---|
| Name | string | Field name |
| Type | string | One of ten types (see vocabulary/types.go §4.1) |
| Nullable | bool | Whether NULL is allowed |
| Description | string | Human-readable description |
| Default | interface{} | Literal default value; nil if none |
| Unique | bool | Whether a unique constraint is added |
| References | string | Target entity name for foreign_key type |
| MaxLength | *int | Required for varchar, optional for text |
| MinLength | *int | Optional for varchar |
| MaxValue | *float64 | Inclusive upper bound for int/float |
| MinValue | *float64 | Inclusive lower bound for int/float |
| PrecisionDecimalPlaces | *int | Optional for float |
| EnumValues | []string | Required for enum type |
| JsonTypeDiscriminator | string | Required for json type; names sibling enum field |
| MustBeUniqueWithin | []string | Field names forming composite uniqueness scope |
| IsReserved | bool | Set by injector for id, created_time, etc. |
| IsGovernance | bool | Set by injector for _requires_group, etc. |

Pointer fields (`*int`, `*float64`) distinguish "not set" from "set to zero." This matters for constraint validation: `MinValue == nil` means no lower bound, `MinValue == &0.0` means lower bound is zero.

### 2.3 `relationship.go`

**Relationship** — a foreign key edge between two entities.

| Field | Type | Description |
|---|---|---|
| SourceEntity | string | Entity containing the FK field |
| SourceField | string | FK field name |
| TargetEntity | string | Referenced entity |
| Cardinality | string | one_to_one, one_to_many, many_to_many |
| OnDeleteAction | string | cascade, restrict, set_null |
| IsSelfReferential | bool | True when SourceEntity == TargetEntity |

### 2.4 `schema.go`

**Schema** — the complete in-memory schema, built incrementally by the pipeline.

| Field | Type | Description |
|---|---|---|
| Entities | map[string]*Entity | All entities keyed by name |
| Relationships | []Relationship | All FK relationships |
| LoadOrder | []string | Topologically sorted entity names |
| MetaSchema | *MetaSchema | Parsed from _schema_meta.yaml |
| Reserved | *ReservedConfig | Parsed from conventions/reserved.yaml |
| Version | SchemaVersionInfo | Version metadata |
| Errors | []SchemaError | Accumulated across pipeline stages |

**ReservedConfig** — parsed reserved field definitions.

| Field | Type | Description |
|---|---|---|
| Universal | []Field | id, created_time, updated_time |
| SoftDelete | []Field | is_active |
| Hierarchical | []Field | Template fields with {entity} placeholders |
| VersioningSibling | []Field | Non-templated structural versioning fields |
| VersioningConstraints | []ConstraintDef | Constraints for sibling tables |
| Governance | []GovernanceFieldDef | Governance fields with enable keys |
| Observation | []GovernanceFieldDef | Observation fields with enable keys |
| SchemaMetadata | []GovernanceFieldDef | Schema metadata fields with enable keys |
| NamingConventions | NamingConventionConfig | Reserved for future use |
| AppendOnly | AppendOnlyConfig | Revoke config for append-only tables |
| DatabaseRoles | []RoleDefinition | Database role definitions for DDL |

**RoleDefinition** — a database role to create.

| Field | Type | Description |
|---|---|---|
| Name | string | Role name (e.g. opsdb_app_role) |
| Description | string | Human-readable description |
| Grants | []string | SQL grant keywords (SELECT, INSERT, etc.) |

**GovernanceFieldDef** — a governance field paired with its enable key.

| Field | Type | Description |
|---|---|---|
| Field | Field | The field definition |
| EnabledBy | string | Key in entity governance section that enables this field |

**ConstraintDef** — a constraint for sibling tables.

| Field | Type | Description |
|---|---|---|
| Type | string | Constraint type (e.g. "unique_composite") |
| Fields | []string | Column names, may contain {entity_name} placeholders |

**AppendOnlyConfig** — append-only enforcement config.

| Field | Type | Description |
|---|---|---|
| PostgresRevokeRoles | []string | Roles to revoke UPDATE/DELETE from |

**SchemaVersionInfo**, **MetaSchema**, **TypeDefinition**, **FieldSchemaDefinition**, **IndexSchemaDefinition**, **GovernanceSchemaDefinition**, **ForbiddenPattern**, **EvolutionRuleSet**, **EvolutionRule**, **ForbiddenEvolutionRule** — supporting structs for meta-schema and evolution rules. See source for field definitions; these are consumed by `tools/opsdb-schema/loader/` and not by the API server.

---

## 3. `internal/conventions/` — Naming and Reserved Fields

### 3.1 `naming.go`

**Purpose:** Validates names against the spec's naming conventions at schema-define time.

**Exported functions:**

`ValidateEntityName(name string) error` — checks lowercase_underscore, singular form, 2-128 characters, no double underscores, no trailing underscore, leading underscore only for `_schema_` prefix. Uses `checkSingular` heuristic: rejects trailing 's' unless final component is in `knownPluralExceptions`.

`ValidateFieldName(name string, fieldType string) error` — checks lowercase_underscore, 2-128 characters, no double underscores. Type-specific rules: datetime fields must end in `_time`, date fields must end in `_date`, boolean fields must start with `is_` or `was_`. Fields starting with `_` must be recognized governance fields (validated via `IsGovernanceFieldName`).

`ValidateFKName(fieldName string, referencedEntity string) error` — checks `_id` suffix, then accepts either `{entity}_id` (standard) or `{prefix}_{entity}_id` (role-prefixed). Also accepts `parent_{entity}_version_id` for self-referential version chains.

`ValidateCompositeName(name string) error` — checks lowercase_underscore, 2-128 characters, must contain at least one underscore, no leading underscore.

`IsGovernanceFieldName(name string) bool` — returns true if the name is in the governance field set. Delegates to `GovernanceFieldNames()`.

`GovernanceFieldNames() map[string]bool` — returns the canonical set of all nine governance/observation/schema metadata field names. This is the single source of truth for underscore-prefixed field name validation. The nine names are: `_requires_group`, `_access_classification`, `_audit_chain_hash`, `_retention_policy_id`, `_schema_version_introduced_id`, `_schema_version_deprecated_id`, `_observed_time`, `_authority_id`, `_puller_runner_job_id`.

**Unexported helpers:** `checkLowercaseUnderscoreOnly(s string) error`, `checkSingular(name string) error`.

**Usage by schema engine:**
```go
if err := conventions.ValidateEntityName(entity.Name); err != nil {
    // reject entity
}
for _, field := range entity.Fields {
    if err := conventions.ValidateFieldName(field.Name, field.Type); err != nil {
        // reject field
    }
    if field.Type == "foreign_key" {
        if err := conventions.ValidateFKName(field.Name, field.References); err != nil {
            // reject FK
        }
    }
}
```

### 3.2 `reserved.go`

**Purpose:** Loads `conventions/reserved.yaml`, provides reserved field definitions for injection into entities, and checks whether field names are reserved.

**Exported functions:**

`LoadReserved(path string) (*model.ReservedConfig, error)` — reads and parses the YAML file. Populates all sections of `ReservedConfig`: Universal, SoftDelete, Hierarchical, VersioningSibling, VersioningConstraints, Governance, Observation, SchemaMetadata, AppendOnly, DatabaseRoles. Falls back to hardcoded defaults for any missing section. Calls `ensureGovernanceDefaults` to fill gaps.

`IsReservedFieldName(name string, entityName string) bool` — checks if a field name is reserved. Covers: static universal names (`id`, `created_time`, `updated_time`, `is_active`), governance field names, versioning sibling static names (`version_serial`, `is_active_version`, `approved_for_production_time`, `change_set_id`), and entity-templated names (`parent_{entity}_id`, `{entity}_id`, `parent_{entity}_version_id`).

`GetUniversalFields() []model.Field` — returns `id`, `created_time`, `updated_time` with `IsReserved=true`.

`GetSoftDeleteFields() []model.Field` — returns `is_active` with `IsReserved=true`, `Default=true`.

`GetHierarchicalFields(entityName string) []model.Field` — returns `parent_{entityName}_id` as a nullable self-referential FK with `IsReserved=true`.

`GetVersioningSiblingFields(entityName string) []model.Field` — returns the six versioning sibling fields: `{entity}_id` (FK), `version_serial` (int, min 1), `parent_{entity}_version_id` (nullable FK), `change_set_id` (nullable FK), `is_active_version` (boolean, default false), `approved_for_production_time` (nullable datetime). All have `IsReserved=true`.

`GetGovernanceFields(enabled map[string]bool) []model.Field` — returns governance field definitions for enabled flags. The `enabled` map keys are governance field names (e.g. `"_requires_group": true`). Only fields whose key is true in the map are returned. All returned fields have both `IsReserved=true` and `IsGovernance=true`.

`GetDatabaseRoles() []model.RoleDefinition` — returns four role definitions: `opsdb_app_role` (CRUD), `opsdb_admin_role` (ALL), `opsdb_readonly_role` (SELECT), `opsdb_runner_role` (SELECT/INSERT/UPDATE).

**Usage by schema engine injector:**
```go
reserved, err := conventions.LoadReserved("conventions/reserved.yaml")
// ...
for _, entity := range schema.Entities {
    entity.Fields = append(entity.Fields, conventions.GetUniversalFields()...)
    if entity.SoftDelete {
        entity.Fields = append(entity.Fields, conventions.GetSoftDeleteFields()...)
    }
    if entity.Hierarchical {
        entity.Fields = append(entity.Fields, conventions.GetHierarchicalFields(entity.Name)...)
    }
    if entity.Versioned {
        siblingFields := conventions.GetVersioningSiblingFields(entity.Name)
        // create sibling entity with these fields
    }
    if len(entity.Governance) > 0 {
        entity.Fields = append(entity.Fields, conventions.GetGovernanceFields(entity.Governance)...)
    }
}
```

**Usage by schema engine validator:**
```go
if conventions.IsReservedFieldName(field.Name, entity.Name) {
    // reject: user-defined field uses a reserved name
}
```

---

## 4. `internal/vocabulary/` — Type System and Pattern Detection

### 4.1 `types.go`

**Purpose:** Single source of truth for the ten allowed field types, their Postgres mappings, and per-type constraint/modifier rules.

**The ten allowed types:** `int`, `float`, `varchar`, `text`, `boolean`, `datetime`, `date`, `json`, `enum`, `foreign_key`.

**Exported functions:**

`AllAllowedTypes() []string` — returns all ten type names.

`IsValidType(typeName string) bool` — checks membership.

`GetPostgresType(typeName string, constraints map[string]interface{}) string` — returns the Postgres DDL type string. VARCHAR uses `max_length` from constraints to produce `VARCHAR(N)`. All others are fixed mappings: int→INTEGER, float→DOUBLE PRECISION, text→TEXT, boolean→BOOLEAN, datetime→TIMESTAMP WITHOUT TIME ZONE, date→DATE, json→JSONB, enum→VARCHAR(255), foreign_key→INTEGER.

`GetAllowedConstraints(typeName string) []string` — returns constraint keys the type accepts. int/float: min_value, max_value (float also: precision_decimal_places). varchar: min_length, max_length. text: max_length. json: json_type_discriminator. enum: enum_values. foreign_key: references. boolean/datetime/date: none.

`GetRequiredConstraints(typeName string) []string` — returns constraint keys that must be present. varchar: max_length. json: json_type_discriminator. enum: enum_values. foreign_key: references. All others: none.

`GetAllowedModifiers(typeName string) []string` — returns modifier names the type accepts. int/float/varchar/enum: nullable, default, unique, must_be_unique_within. text: nullable, default. boolean: nullable, default. datetime/date: nullable only. json: nullable only. foreign_key: nullable only.

`GetForbiddenModifiers(typeName string) []string` — returns explicitly forbidden modifiers. text: unique, must_be_unique_within. boolean: unique, must_be_unique_within. datetime/date/json: default, unique, must_be_unique_within. foreign_key: default, unique, must_be_unique_within.

`IsModifierAllowed(typeName, modifierName string) bool` — checks allowed list.

`IsModifierForbidden(typeName, modifierName string) bool` — checks forbidden list.

`ValidateModifiersForType(typeName string, hasDefault bool, defaultValue interface{}, hasUnique bool, hasMustBeUniqueWithin bool) error` — validates all modifiers present on a field against rules. Calls `ValidateDefault` (from modifiers.go) and `ValidateUnique` (from modifiers.go) as needed.

**Important:** `constraints.go` and `modifiers.go` within the same package reference the maps defined here (`typeAllowedConstraints`, `typeRequiredConstraints`) directly. These maps are the single source of truth — they are not duplicated.

### 4.2 `constraints.go`

**Purpose:** Validates constraint values against the closed constraint vocabulary.

**Exported functions:**

`ValidateConstraints(fieldType string, constraints map[string]interface{}) error` — master validator. Reads allowed/required constraint lists from types.go. Rejects unknown constraints, verifies required ones are present, dispatches to per-constraint validators. This is the entry point for constraint validation.

`ValidateNumericRange(minValue, maxValue interface{}) error` — checks min <= max. Converts via `toFloat64`.

`ValidateStringLength(minLength, maxLength interface{}) error` — checks min <= max, max >= 1, min >= 0. Converts via `toInt`.

`ValidateEnumValues(values []string) error` — checks non-empty, max 256 values, no duplicates, all lowercase_underscore format.

`ValidateReferences(targetEntity string, knownEntities map[string]bool) error` — checks referenced entity exists in the known set.

`ValidateJsonDiscriminator(discriminatorField string, entityFieldNames map[string]string) error` — checks discriminator field exists in same entity and is enum type. The `entityFieldNames` map is field_name→field_type.

`ValidatePrecision(places interface{}) error` — checks 0-15 range.

**Unexported helpers:** `toFloat64(v interface{}) (float64, error)`, `toInt(v interface{}) (int, error)`, `toStringSlice(v interface{}) ([]string, error)`, `checkEnumValueFormat(v string) error`, `validatePositiveInt(v interface{}, name string) error`, `validateNonNegativeInt(v interface{}, name string) error`.

**Usage by schema engine validator:**
```go
constraintMap := map[string]interface{}{
    "max_length": field.MaxLength,
    "min_length": field.MinLength,
}
if err := vocabulary.ValidateConstraints(field.Type, constraintMap); err != nil {
    // reject field
}
```

### 4.3 `modifiers.go`

**Purpose:** Validates modifier values (default literals, unique applicability, composite uniqueness scope).

**Exported functions:**

`IsValidModifier(modName string) bool` — checks if modifier is one of: nullable, default, unique, must_be_unique_within.

`ValidateDefault(fieldType string, defaultValue interface{}) error` — validates a default value is an appropriate literal. Rejects defaults on forbidden types (foreign_key, datetime, date, json). Checks for embedded logic and template syntax in string defaults. Dispatches to per-type validators: boolean must be true/false, int must be whole number, float must be numeric, varchar/text/enum must be string.

`ValidateUnique(fieldType string) error` — rejects unique on text and json types.

`ValidateMustBeUniqueWithin(fieldNames []string, entityFields []string) error` — validates composite uniqueness scope. Field list must be non-empty, every referenced field must exist in the entity, no duplicates.

**Note:** `ValidateDefault` calls `CheckForEmbeddedLogic` and `CheckForTemplating` from `forbidden.go` within the same package. This is an intentional cross-file dependency within `vocabulary`.

### 4.4 `forbidden.go`

**Purpose:** Detects forbidden patterns in raw entity YAML maps. Scans for inheritance, imports, deletion markers, type change markers, rename markers, regex, embedded logic, and templating.

**Exported types:**

`ForbiddenViolation` — Pattern (string), Location (string), Rationale (string), Alternative (string).

**Exported functions:**

`ScanForForbiddenPatterns(rawYAML map[string]interface{}) []ForbiddenViolation` — master scanner. Checks top-level keys against all forbidden key categories, scans field-level maps for forbidden markers and constraint patterns, recursively scans all values for templating syntax. Returns all violations found (does not stop at first).

`CheckForRegex(value string) bool` — detects regex metacharacters in constraint values. Checks for character class patterns (`[a-z]`, `\\d`, etc.), anchored patterns (`^...$`), character class quantifiers (`]+`, `]*`).

`CheckForEmbeddedLogic(value interface{}) bool` — detects SQL functions (`NOW()`, `CURRENT_TIMESTAMP`, `gen_random_uuid`, etc.), arithmetic operators (` + `, ` - `, etc.), and expression markers (leading `=`). Only operates on string values; returns false for non-strings.

`CheckForInheritance(keys []string) bool` — checks for: extends, inherits, parent_entity, base_class, abstract, mixin, trait, mixins, traits.

`CheckForTemplating(value string) bool` — checks for: `{{`, `}}`, `{%`, `%}`, `${`, `$(`.

`CheckForImports(keys []string) bool` — checks for: import, imports, include, require, $ref, source.

`CheckForDeletionMarkers(keys []string) bool` — checks for: deleted, removed, drop, remove_field, drop_field, delete_field, drop_entity.

`CheckForTypeChangeMarkers(keys []string) bool` — checks for: migrate_type, change_type, convert_to, cast_to, type_change.

`CheckForRenameMarkers(keys []string) bool` — checks for: rename_to, renamed_from, alias, previous_name, old_name.

**Usage by schema engine validator:**
```go
violations := vocabulary.ScanForForbiddenPatterns(rawYAMLMap)
for _, v := range violations {
    schema.Errors = append(schema.Errors, model.SchemaError{
        Entity:  entityName,
        Message: fmt.Sprintf("%s at %s: %s", v.Pattern, v.Location, v.Rationale),
        Severity: "error",
    })
}
```

---

## 5. `internal/pg/` — Postgres Helpers

### 5.1 `conn.go`

**Purpose:** Connection pool management, query execution, error classification, and JSON helpers.

**Exported types:**

`DB` — wraps `*pgxpool.Pool`. Fields: `Pool *pgxpool.Pool` (exported for direct access when needed), `dsn string` (unexported).

`Row` — wraps `pgx.Row`. Method: `Scan(dest ...interface{}) error`.

`Rows` — wraps `pgx.Rows`. Methods: `Next() bool`, `Scan(dest ...interface{}) error`, `Close()`, `Err() error`, `Columns() ([]string, error)`. The `Columns` method reads `FieldDescriptions()` from the underlying pgx rows and returns column names as strings. This is critical for `scanRowToMap` in the operations package which needs column names to build `map[string]interface{}` results.

**Connection functions:**

`Connect(dsn string) (*DB, error)` — opens a pool with default settings (25 max conns, 2 min conns, 5m max lifetime, 1m max idle), pings to verify.

`ConnectWithPoolSize(dsn string, maxConns int, minConns int, maxLifetime time.Duration) (*DB, error)` — same with explicit pool sizing.

`DSNFromEnv(envVar string) (string, error)` — reads DSN from named env var, validates format (accepts `postgres://` URI or `host=` keyword format).

**DB methods:**

`(db *DB) Close()` — closes pool.

`(db *DB) Ping() error` — tests liveness with 5s timeout.

`(db *DB) Query(query string, args ...interface{}) (*Rows, error)` — executes query returning multiple rows. 30s timeout. Returns wrapped `*Rows`.

`(db *DB) QueryRow(query string, args ...interface{}) *Row` — executes query returning at most one row. 30s timeout. Returns wrapped `*Row`. Call `.Scan()` on the result.

`(db *DB) QueryRows(query string, args ...interface{}) (*Rows, error)` — alias for `Query`. Exists because some call sites (watch.go) use this name.

`(db *DB) DSN() string` — returns redacted DSN for logging.

**Error classification:**

`IsNoRows(err error) bool` — checks `errors.Is(err, pgx.ErrNoRows)`. Use after `Row.Scan()` to distinguish "not found" from other errors.

`IsUndefinedTable(err error) bool` — checks Postgres SQLSTATE `42P01`. Use when querying tables that may not exist (e.g. version sibling tables during bootstrap).

`IsUndefinedColumn(err error) bool` — checks Postgres SQLSTATE `42703`. Use when querying columns that may not exist (e.g. governance fields on entities that don't enable them).

**JSON helpers:**

`MarshalJSON(v interface{}) ([]byte, error)` — wraps `json.Marshal`. Exists so callers don't import `encoding/json`.

`UnmarshalJSON(data []byte, v interface{}) error` — wraps `json.Unmarshal`.

**Usage patterns:**

```go
// Single row lookup
row := db.QueryRow("SELECT name FROM service WHERE id = $1", serviceID)
var name string
err := row.Scan(&name)
if err != nil {
    if pg.IsNoRows(err) {
        // not found
    }
    // other error
}

// Multiple rows
rows, err := db.Query("SELECT id, name FROM service WHERE is_active = true")
if err != nil {
    return err
}
defer rows.Close()
columns, _ := rows.Columns()
for rows.Next() {
    // rows.Scan(...) or scanRowToMap(rows, columns)
}
if err := rows.Err(); err != nil {
    return err
}

// Table existence check
_, err := db.Query("SELECT 1 FROM " + pg.QuoteIdentifier(tableName) + " LIMIT 0")
if err != nil && pg.IsUndefinedTable(err) {
    // table does not exist
}
```

### 5.2 `tx.go`

**Purpose:** Transaction management and in-transaction query execution.

**Exported types:**

`Tx` — wraps `pgx.Tx`. Unexported fields: `tx pgx.Tx`, `ctx context.Context`.

**Transaction functions:**

`WithTransaction(db *DB, fn func(tx *Tx) error) error` — begins a read-committed transaction, calls fn, commits on nil error, rolls back on error or panic. 60s timeout.

`WithTransactionContext(ctx context.Context, db *DB, fn func(tx *Tx) error) error` — same with explicit parent context.

`WithSerializableTransaction(db *DB, fn func(tx *Tx) error) error` — serializable isolation. Used for schema DDL operations.

`RollbackOnly(db *DB, fn func(tx *Tx) error) error` — begins transaction, calls fn, always rolls back. Used for dry-run DDL validation.

**In-transaction execution:**

`ExecInTx(tx *Tx, query string, args ...interface{}) (pgconn.CommandTag, error)` — executes a statement. Returns `CommandTag` which has `.RowsAffected() int64`.

`QueryInTx(tx *Tx, query string, args ...interface{}) (*Rows, error)` — executes a query returning wrapped `*Rows`.

`QueryRowInTx(tx *Tx, query string, args ...interface{}) *Row` — executes a query returning wrapped `*Row`.

`QueryRowsInTx(tx *Tx, query string, args ...interface{}) (*Rows, error)` — alias for `QueryInTx`. Exists because some call sites use this name.

**Tx methods:**

`(tx *Tx) Context() context.Context` — returns the transaction's context.

`(tx *Tx) Underlying() pgx.Tx` — returns raw pgx.Tx for direct access.

**Utility:**

`QuoteIdentifier(name string) string` — wraps a SQL identifier in double quotes with escaping (doubles internal quotes). Prevents SQL injection when table or column names come from schema metadata. Example: `QuoteIdentifier("service")` returns `"service"`.

**Usage patterns:**

```go
err := pg.WithTransaction(db, func(tx *pg.Tx) error {
    // insert parent row
    var id int
    err := pg.QueryRowInTx(tx,
        "INSERT INTO change_set (name, status) VALUES ($1, $2) RETURNING id",
        name, "submitted",
    ).Scan(&id)
    if err != nil {
        return err // triggers rollback
    }

    // insert child rows
    _, err = pg.ExecInTx(tx,
        "INSERT INTO change_set_field_change (change_set_id, field_name) VALUES ($1, $2)",
        id, fieldName,
    )
    if err != nil {
        return err // triggers rollback
    }

    return nil // triggers commit
})
```

### 5.3 `advisory_lock.go`

**Purpose:** Postgres advisory locks for concurrent schema apply safety.

**Exported functions:**

`SchemaApplyLockID() int64` — returns the fixed lock ID `7283946501` used for all schema apply operations.

`AcquireAdvisoryLock(db *DB, lockID int64) (bool, error)` — non-blocking attempt. Returns true if acquired, false if held by another session. 5s timeout on the query itself.

`WaitForAdvisoryLock(db *DB, lockID int64, timeout time.Duration) error` — blocking. Polls every 250ms. Returns nil on acquisition, error on timeout.

`ReleaseAdvisoryLock(db *DB, lockID int64) error` — releases the lock. Errors if the lock was not held by this session.

**Usage by schema engine applier:**
```go
lockID := pg.SchemaApplyLockID()
err := pg.WaitForAdvisoryLock(db, lockID, 30*time.Second)
if err != nil {
    return fmt.Errorf("another schema apply is in progress: %w", err)
}
defer pg.ReleaseAdvisoryLock(db, lockID)
// proceed with DDL
```

---

## 6. `internal/testutil/` — Test Helpers

### 6.1 `pg.go`

Provides test Postgres instance management via testcontainers. Functions: `StartTestPostgres`, `StopTestPostgres`, `ResetTestDB`, `DSN`. These are NOT rewritten in this pass — documented here for completeness.

### 6.2 `fixtures.go`

Provides test schema fragments. Functions return YAML strings and pre-built Entity structs for unit tests. NOT rewritten in this pass.

---

## 7. Cross-Package Integration Points

### 7.1 Schema engine pipeline (`tools/opsdb-schema/`)

The schema engine calls internal packages in this order:

1. `conventions.LoadReserved()` — load reserved field config
2. Parser reads entity YAML files
3. `vocabulary.ScanForForbiddenPatterns()` — reject forbidden patterns in raw YAML
4. `conventions.ValidateEntityName()` — validate entity name
5. For each field:
   - `vocabulary.IsValidType()` — check type is one of ten
   - `conventions.ValidateFieldName()` — check naming conventions
   - `conventions.IsReservedFieldName()` — reject user fields using reserved names
   - `vocabulary.ValidateConstraints()` — validate constraint keys and values
   - `vocabulary.ValidateModifiersForType()` — validate modifiers
   - For FK fields: `conventions.ValidateFKName()`, `vocabulary.ValidateReferences()`
   - For JSON fields: `vocabulary.ValidateJsonDiscriminator()`
6. Resolver builds dependency graph, runs topological sort
7. Injector calls `conventions.GetUniversalFields()`, `GetSoftDeleteFields()`, etc.
8. Generator calls `vocabulary.GetPostgresType()` for DDL
9. Applier uses `pg.WithSerializableTransaction()`, `pg.WaitForAdvisoryLock()`
10. Meta populator uses `pg.ExecInTx()`, `pg.QueryRowInTx()`

### 7.2 API server (`tools/opsdb-api/`)

The API server uses only `internal/pg/`. It does NOT use `model`, `conventions`, or `vocabulary` at runtime. The API's runtime type information comes from `schema/runtime_schema.go` which reads `_schema_entity_type`, `_schema_field`, and `_schema_relationship` tables populated by the schema engine.

API usage of `pg`:
- `cmd/main.go`: `pg.Connect()` for database connection
- `gate/step_*.go`: `db.Query()`, `db.QueryRow()`, `pg.QueryRowInTx()`, `pg.ExecInTx()`, `pg.WithTransaction()`, `pg.QuoteIdentifier()`, `pg.IsNoRows()`, `pg.IsUndefinedTable()`, `pg.IsUndefinedColumn()`, `pg.UnmarshalJSON()`
- `operations/*.go`: `db.Query()`, `db.QueryRow()`, `db.QueryRows()` for read operations; `pg.QuoteIdentifier()` for dynamic SQL; `pg.IsNoRows()` for not-found handling; `pg.MarshalJSON()`, `pg.UnmarshalJSON()` for JSON column handling
- `schema/runtime_schema.go`: `db.Query()`, `db.QueryRow()` for schema metadata loading
- `reportkeys/enforcer.go`: `db.Query()` for report key lookups

### 7.3 The `pg.Rows` interface contract

The `pg.Rows` type is used across the API server for generic row scanning. The key method is `Columns() ([]string, error)` which returns column names. This enables `scanRowToMap` in `operations/read.go` to build `map[string]interface{}` results without knowing the schema at compile time:

```go
func scanRowToMap(rows pg.Rows, columns []string) (map[string]interface{}, error) {
    values := make([]interface{}, len(columns))
    valuePtrs := make([]interface{}, len(columns))
    for i := range values {
        valuePtrs[i] = &values[i]
    }
    if err := rows.Scan(valuePtrs...); err != nil {
        return nil, err
    }
    result := make(map[string]interface{}, len(columns))
    for i, col := range columns {
        result[col] = values[i]
    }
    return result, nil
}
```

Note: `scanRowToMap` accepts `pg.Rows` (the struct type, not an interface). The `Columns` method is defined on the `*Rows` struct and reads `FieldDescriptions()` from the underlying `pgx.Rows` to extract column names.

---

## 8. Known Conflicts with IOSE

### 8.1 Field type count

The IOSE's entry for `model/field.go` states the Type field is "one of nine" types. The actual implementation supports ten types: the nine listed (int, float, varchar, text, boolean, datetime, json, enum, foreign_key) plus `date`. The `date` type is fully supported in `vocabulary/types.go` (maps to Postgres `DATE`), in `conventions/naming.go` (validates `_date` suffix), and in `vocabulary/modifiers.go` (forbids default values). The IOSE comment is simply missing `date` from its list.

### 8.2 `model/field.go` MaxLength/MinLength type

The IOSE describes `MaxLength` and `MinLength` as `int` (non-pointer). The actual `model/field.go` defines them as `*int` (pointer). The pointer form is correct — it distinguishes "not set" from "set to zero" which matters for constraint validation. Code that accesses these fields must handle nil.

### 8.3 `model/schema.go` ReservedConfig structure

The IOSE describes `ReservedConfig` with a `GovernanceFields map[string]Field` field. The actual struct uses `Governance []GovernanceFieldDef`, `Observation []GovernanceFieldDef`, `SchemaMetadata []GovernanceFieldDef` — three separate slices of `GovernanceFieldDef` (which pairs a `Field` with an `EnabledBy` string). The IOSE description is outdated. `reserved.go` conforms to the actual struct.

### 8.4 `model/schema.go` RoleDefinition fields

The IOSE describes `RoleDefinition` with `Permissions []string` and `AppliesTo string`. The actual struct has `Grants []string` and no `AppliesTo`. `reserved.go` uses `Grants` to match the actual struct. The IOSE description should be updated.

### 8.5 `pg/tx.go` QueryInTx return type

The IOSE describes `QueryInTx` as returning `(*sql.Rows, error)`. The actual implementation returns `(*pg.Rows, error)` — the wrapped type. This is correct for the pgx-native implementation. The IOSE assumed `database/sql`; the codebase uses `pgx/v5` throughout.

### 8.6 `pg/tx.go` ExecInTx return type

The IOSE describes `ExecInTx` as returning `(sql.Result, error)`. The actual implementation returns `(pgconn.CommandTag, error)`. Both expose `.RowsAffected()` but the types differ. `CommandTag.RowsAffected()` returns `int64` directly (no error return). Callers using `result.RowsAffected()` work correctly.

### 8.7 `pg/tx.go` QueryRowInDB removed

The IOSE lists `QueryRowInDB(db *DB, query string, args ...interface{}) *sql.Row`. This function is removed. Its purpose is served by `db.QueryRow()` on the `DB` struct, which returns `*pg.Row` (not `*sql.Row`).

### 8.8 `pg/conn.go` Query/QueryRow timeout

The `db.Query()` and `db.QueryRow()` methods create a 30-second context timeout internally. This means long-running queries will be cancelled after 30 seconds. The watch operation's `queryMatchingEntities` calls `db.QueryRows()` (alias for `db.Query()`) which has this same 30-second limit. For large entity sets this may need adjustment. The gate step functions also call `db.Query()`/`db.QueryRow()` with the same timeout. The IOSE does not specify query timeouts; this is an implementation decision that consuming code should be aware of.

### 8.9 `vocabulary/forbidden.go` importKeys includes "imports"

The naive version had `"imports": false` (allowed at top level in directory.yaml). The rewritten version has `"imports": true`. This means `ScanForForbiddenPatterns` will flag `imports` as a forbidden key in entity files. This is correct behavior — entity files should not have an `imports` key. The `directory.yaml` file is not validated through this scanner; it is parsed by `loader/parser.go` directly.

---

## 9. Testing Guidance

### Unit tests for vocabulary and conventions

These packages are pure functions with no I/O (except `LoadReserved` which reads a YAML file). Unit tests should:

- Test each `Validate*` function with valid inputs, boundary cases, and known-bad inputs
- Test `ScanForForbiddenPatterns` with fixtures from `testutil/fixtures.go` (known-good entities return empty violations, known-bad entities return the expected violation)
- Test `IsReservedFieldName` with all static names, templated names for various entity names, and non-reserved names
- Test `GovernanceFieldNames` returns exactly nine entries

### Integration tests for pg

Use `testutil.StartTestPostgres` to get a real Postgres instance:

- Test `Connect`, `Ping`, `Close`
- Test `Query`/`QueryRow` with simple SELECT
- Test `Rows.Columns()` returns correct column names
- Test `WithTransaction` commit and rollback paths
- Test `ExecInTx` with INSERT/UPDATE and verify `RowsAffected()`
- Test `IsNoRows` after scanning a non-existent row
- Test `IsUndefinedTable` after querying a non-existent table
- Test `AcquireAdvisoryLock`/`ReleaseAdvisoryLock` including contention
