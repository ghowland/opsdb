# OpsDB Schema Engine

## What This Document Covers

The schema engine is the first component built and the foundation everything else depends on. It reads YAML files describing the operational schema, validates them against a closed vocabulary, and idempotently applies them to Postgres. It enforces the spec's evolution rules mechanically — forbidden changes are rejected before they reach the database, allowed changes are applied in a transaction, and the schema self-description tables are updated so the API can discover the schema at runtime.

This document specifies the YAML format, the closed vocabulary, the validation rules, the DDL generation, the idempotent application process, the evolution rules, and the CLI interface.

---

## Design Principles

The schema is data, not code. Entity definitions live in YAML files in a git repository. The schema engine reads those files and produces a database. Changing the schema means changing YAML files and running the engine — no hand-written DDL, no migration scripts, no ORM-generated SQL. The YAML is the source of truth. The database is the derivative. The `_schema_*` tables inside the database are a runtime cache of what the YAML declared.

The vocabulary is closed. Nine types, three modifiers, six-plus constraints. That's the complete set of primitives available for describing fields. No regex. No embedded logic. No conditional constraints. No inheritance. No templating. No imports within entity files. Each of these is a refusal that prevents complexity from entering the schema layer. The engine enforces these refusals mechanically — a YAML file containing forbidden vocabulary is rejected with a clear error, not silently accepted.

Schema evolution is additive only. Fields can be added. Enum values can be added. Numeric ranges can be widened. Length bounds can be widened. New entities can be added. New indexes can be added. Nothing can be removed, renamed, or narrowed. This is the price of decade-scale stability — every consumer of the schema (every runner, every query, every audit trail reference) can trust that the fields they depend on will still exist with the same names and types indefinitely. The engine enforces this by comparing desired state (YAML) against current state (database) and rejecting any change that violates the evolution rules.

Storage engine independence is a goal. The schema declarations are engine-independent — the nine types map to standard SQL features available in every major relational database. The current implementation targets Postgres. Future implementations could target MySQL, SQLite, or others by changing only the DDL generation layer. The YAML files, the validation, the evolution rules, and the `_schema_*` population are all engine-independent.

---

## Repository Layout

The schema repository is a directory tree with a defined structure. The engine processes files in a specific order so that foreign key references resolve correctly.

At the root: `directory.yaml` lists domain directories in dependency order. Domains that are referenced by other domains come first. Identity before substrate, substrate before service, service before Kubernetes, and so on through to schema metadata at the end.

Within each domain directory, entity files are loaded in alphabetical order. File names are chosen so that alphabetical order respects intra-domain foreign key dependencies — entities referenced by others sort first alphabetically.

A `meta/` directory holds `_schema_meta.yaml`, which defines what constitutes a valid entity file. This is the one structural definition hardcoded into the engine. A `conventions/` directory holds `reserved.yaml`, which defines universal reserved fields and governance fields.

A `json_schemas/` directory holds registered JSON schemas for typed payloads, organized by discriminator domain. Each discriminator value (ec2_instance, gce_instance, prometheus_server, approval_rule, etc.) has a YAML file defining the expected structure of its `*_data_json` payload. The API validates typed payloads against these registered schemas at write time.

---

## Entity File Format

Each entity is one YAML file. Every key in the file must come from the recognized set — unrecognized keys cause the engine to reject the file.

The top-level keys are: `name`, `description`, `category`, `versioned`, `soft_delete`, `hierarchical`, `append_only`, `fields`, `indexes`, and `governance`.

`name` is the entity name in singular lower_case_with_underscores form. This becomes the table name. The engine validates that the name follows the naming conventions — lowercase, underscores only, no trailing underscores, no double underscores.

`versioned` when true causes the engine to generate a version sibling table (`{name}_version`) with the standard versioning fields. The entity must also reference `change_set` through the sibling, so `versioned: true` is only valid when the change management entities exist in the schema.

`soft_delete` when true causes the engine to inject an `is_active` boolean field with a default of true. Rows are never physically deleted — they are marked inactive.

`hierarchical` when true causes the engine to inject a `parent_{name}_id` self-referencing foreign key. This enables tree structures like location hierarchies, service hierarchies, and escalation step chains.

`append_only` when true causes the engine to generate REVOKE statements that remove UPDATE and DELETE permissions for all database roles on this table. Currently only `audit_log_entry` uses this, but the mechanism is general.

---

## The Closed Vocabulary

### The Nine Types

**int** maps to Postgres INTEGER. Optional constraints: `min_value` and `max_value` (inclusive). The engine generates CHECK constraints for declared bounds.

**float** maps to Postgres DOUBLE PRECISION. Optional constraints: `min_value`, `max_value`, and `precision_decimal_places`. CHECK constraints generated for bounds.

**varchar** maps to Postgres VARCHAR with a required `max_length`. Optional `min_length` (default 0). The engine generates the column as VARCHAR(max_length) and a CHECK constraint for min_length if declared.

**text** maps to Postgres TEXT. Optional `max_length`. No minimum length constraint — text fields are for long-form content where minimum length is not meaningful.

**boolean** maps to Postgres BOOLEAN. No constraints beyond nullability.

**datetime** maps to Postgres TIMESTAMP WITHOUT TIME ZONE. No additional constraints. Used for all time fields, which must be named with a `_time` suffix per naming conventions.

**json** maps to Postgres JSONB. Requires `json_type_discriminator`, which names the sibling field that determines which JSON schema validates the payload. The engine does not validate JSON content at the DDL level — JSON validation is the API's responsibility at write time, using the registered schemas in `json_schemas/`.

**enum** maps to Postgres VARCHAR(255) with a CHECK constraint against the declared `enum_values` list. The engine generates `CHECK (field_name IN ('value1', 'value2', ...))`. Adding new enum values is an allowed evolution. Removing values is forbidden.

**foreign_key** maps to Postgres INTEGER with a FOREIGN KEY constraint referencing the target entity's `id` column. Requires `references` naming the target entity. The engine generates `FOREIGN KEY (field_name) REFERENCES target_table(id)`.

### The Three Modifiers

**nullable** defaults to false. When true, the NOT NULL constraint is omitted. When false (the default), the column is NOT NULL. Explicit declaration is encouraged even for the default to make intent clear in the YAML.

**default** specifies a literal default value. The engine generates `DEFAULT {value}`. Boolean defaults render as TRUE or FALSE. String defaults are quoted. Numeric defaults are bare. Default is not permitted on foreign_key fields (referential integrity should be explicit), datetime fields (computed defaults like NOW() are injected by the engine for reserved fields only, not user fields), or json fields (no sensible default for typed payloads).

**unique** declares that the field's values must be unique across all rows. The engine generates a UNIQUE constraint. For composite uniqueness across multiple fields, use `must_be_unique_within` which names the other fields in the uniqueness scope — the engine generates a composite unique index.

### What Is Forbidden

**No regex.** Regex is a DoS vector (catastrophic backtracking), introduces dialect variation across implementations, produces unpredictable edge cases, and adds an embedded mini-language into the schema. The engine rejects any file containing regex patterns. Validation that would require regex belongs at the API's semantic validation layer using policy data, not in the schema.

**No embedded logic.** Every value in a schema file is a literal. The engine does not evaluate expressions, functions, or computed values. Defaults are literal values only — no `NOW()`, no `previous_value + 1`, no conditional defaults. Computed defaults for reserved fields (created_time, updated_time) are handled by the engine's injection logic, not by the YAML.

**No conditional constraints.** Cross-field invariants like "if status is X then field Y must be set" or "min_replicas must be less than or equal to max_replicas" do not belong in the schema. They belong in policy data evaluated at the API's semantic validation step. The schema defines per-field bounds (mechanical, rarely changing). Policy data defines cross-field invariants (organizational, changing more frequently).

**No inheritance.** No `extends`, no parent entity, no shared base class. Two entities with similar fields each declare their fields independently. Reserved fields are the controlled exception — they are injected mechanically by the engine based on entity-level flags, not through an inheritance mechanism.

**No templating.** No template variables, no parameterized files, no macros. Variation across environments is handled by OpsDB runtime configuration (different data in different DOS substrates), not by different schemas. One schema per OpsDB.

**No imports within entity files.** Entity files do not import or reference other entity files. Only `directory.yaml` imports domain directories. This keeps each entity file self-contained and independently readable.

**No deletion of fields or entities.** Deletion breaks history — version rows reference the deleted field, audit log entries reference it, runners depend on it. The engine rejects any YAML change that removes a previously existing field or entity. The alternative is deprecation: mark the field with `_schema_version_deprecated_id`. The column remains. The data remains. New code stops using it. Old code continues working.

**No renames.** Renames break every consumer. The engine detects potential renames (a field disappears and a new field with the same type appears in the same entity) and rejects them, suggesting the duplication pattern: add the new field with the new name, begin double-writing, migrate readers, deprecate the old field.

**No type changes.** Changing a field's type breaks consumers that depend on the old type. The engine rejects type changes and suggests the duplication pattern: add a new field with the new type, double-write, migrate readers, deprecate the old field.

---

## Reserved Field Injection

The engine injects reserved fields mechanically based on entity-level declarations. The entity YAML never declares these fields — they appear in the generated DDL and in the `_schema_field` metadata but not in the entity file.

Every table receives `id` (SERIAL PRIMARY KEY), `created_time` (TIMESTAMP NOT NULL DEFAULT NOW()), and `updated_time` (TIMESTAMP NOT NULL DEFAULT NOW()). The engine generates a trigger or application-level convention to update `updated_time` on every row modification.

When `soft_delete: true`, the table receives `is_active` (BOOLEAN NOT NULL DEFAULT TRUE).

When `hierarchical: true`, the table receives `parent_{entity_name}_id` (INTEGER REFERENCES {entity_name}(id), nullable). The self-FK is nullable because root nodes in the hierarchy have no parent.

When `versioned: true`, the engine generates a sibling table named `{entity_name}_version` with fields: `id` (SERIAL PRIMARY KEY), `{entity_name}_id` (INTEGER NOT NULL REFERENCES {entity_name}(id)), `version_serial` (INTEGER NOT NULL), `parent_{entity_name}_version_id` (INTEGER REFERENCES {entity_name}_version(id), nullable — first version has no parent), `change_set_id` (INTEGER NOT NULL REFERENCES change_set(id)), `is_active_version` (BOOLEAN NOT NULL DEFAULT FALSE), `approved_for_production_time` (TIMESTAMP, nullable), `created_time` (TIMESTAMP NOT NULL DEFAULT NOW()), and `updated_time` (TIMESTAMP NOT NULL DEFAULT NOW()). A composite unique constraint is generated on `({entity_name}_id, version_serial)` to enforce monotonic version numbering per entity.

When governance fields are enabled in the `governance` section, the corresponding columns are injected: `_requires_group` (VARCHAR(255), nullable), `_access_classification` (VARCHAR(50), nullable), `_retention_policy_id` (INTEGER REFERENCES retention_policy(id), nullable), `_audit_chain_hash` (VARCHAR(128), nullable). For observation cache entities, `_observed_time` (TIMESTAMP NOT NULL), `_authority_id` (INTEGER NOT NULL REFERENCES authority(id)), and `_puller_runner_job_id` (INTEGER NOT NULL REFERENCES runner_job(id)) are injected.

---

## DDL Generation

The engine translates the validated internal representation into Postgres DDL.

### Type Mapping

| Schema Type | Postgres Type |
|-------------|--------------|
| int | INTEGER |
| float | DOUBLE PRECISION |
| varchar | VARCHAR(max_length) |
| text | TEXT |
| boolean | BOOLEAN |
| datetime | TIMESTAMP WITHOUT TIME ZONE |
| json | JSONB |
| enum | VARCHAR(255) |
| foreign_key | INTEGER |

### Constraint Generation

Numeric bounds produce CHECK constraints: `CONSTRAINT chk_{table}_{field}_range CHECK ({field} >= {min} AND {field} <= {max})`.

String minimum length produces a CHECK constraint: `CONSTRAINT chk_{table}_{field}_minlen CHECK (LENGTH({field}) >= {min_length})`.

Enum values produce a CHECK constraint: `CONSTRAINT chk_{table}_{field}_enum CHECK ({field} IN ('v1', 'v2', ...))`.

Foreign keys produce referential constraints: `CONSTRAINT fk_{table}_{field} FOREIGN KEY ({field}) REFERENCES {target}(id)`.

Unique fields produce unique constraints: `CONSTRAINT uq_{table}_{field} UNIQUE ({field})`.

Composite uniqueness produces unique indexes: `CREATE UNIQUE INDEX uix_{table}_{f1}_{f2} ON {table} ({f1}, {f2})`.

Declared indexes produce indexes: `CREATE INDEX ix_{table}_{f1}_{f2} ON {table} ({f1}, {f2})`, or unique indexes if `unique: true`.

### Dependency Resolution

The engine builds a directed acyclic graph from foreign key references. Each entity is a node. Each foreign_key field creates an edge from the referencing entity to the referenced entity. Self-referential foreign keys (hierarchical entities) do not create ordering edges — they are handled by making the self-FK nullable.

The graph is topologically sorted to produce the creation order. Entities with no foreign key dependencies are created first, then entities referencing only already-created entities, and so on.

If the graph contains a cycle (which indicates a schema design error), the engine rejects the schema with an error identifying the participating entities.

Version sibling tables depend on both their parent entity and the change_set entity. The engine places sibling creation after both dependencies are satisfied.

### Append-Only Enforcement

When an entity is declared `append_only: true`, the engine generates REVOKE statements after creating the table:

```sql
REVOKE UPDATE, DELETE ON {table} FROM PUBLIC;
REVOKE UPDATE, DELETE ON {table} FROM opsdb_app_role;
REVOKE UPDATE, DELETE ON {table} FROM opsdb_admin_role;
```

This enforces append-only at the DDL level. No role can modify or delete rows in the table. This is not a convention — it is a mechanical constraint that survives any application-layer changes.

---

## Idempotent Application

The engine's core capability is running against an existing database with data and making it match the YAML without destroying anything.

### Scope Control

The engine accepts a scope argument limiting which entities are processed:

No scope processes the full database. An entity name processes that entity and its version sibling. An entity/field pair processes that specific field only.

### State Comparison

The engine reads current database state from two sources. If `_schema_entity_type`, `_schema_field`, and `_schema_relationship` tables exist and are populated, the engine reads from them (preferred — this is the fast path that avoids querying information_schema). If the `_schema_*` tables don't exist (first-time bootstrap), the engine reads from Postgres `information_schema` to discover existing tables, columns, types, and constraints.

The engine compares current state against desired state from YAML and produces a diff: new entities, new fields, changed constraints, new indexes, and any forbidden changes.

### Allowed Changes

New entities generate CREATE TABLE with all columns, constraints, indexes, and reserved fields. If versioned, the sibling table is also created.

New nullable fields generate ALTER TABLE ADD COLUMN. New non-nullable fields with defaults generate ALTER TABLE ADD COLUMN with DEFAULT, followed by a backfill of existing rows.

New enum values update the CHECK constraint — the old constraint is dropped and recreated with the expanded value list.

Widened numeric ranges (decreased min_value or increased max_value) update the CHECK constraint with new bounds.

Widened length bounds (increased max_length for varchar) alter the column type to the new length. Decreased min_length updates the CHECK constraint.

New indexes generate CREATE INDEX.

New governance fields generate ALTER TABLE ADD COLUMN for each newly enabled governance field.

Entities changed to `versioned: true` generate the sibling table.

### Forbidden Changes

The engine rejects any change that violates the evolution rules. Each rejection produces a clear error message identifying the specific rule violated, the entity and field involved, and the alternative approach.

Field deletion: the engine identifies fields present in the database but absent from the YAML and rejects with guidance to deprecate instead.

Field rename: the engine detects when a field disappears and a new field with the same type appears in the same entity. It flags this as a potential rename and rejects with guidance to use the duplication pattern. This heuristic may false-positive on coincidental add-plus-deprecate — the user confirms intent by adding a deprecated annotation to the old field.

Type change: the engine detects when a field's type in the YAML differs from its type in the database and rejects with guidance to use the duplication pattern.

Range narrowing: the engine detects decreased max_value, increased min_value, decreased max_length, or increased min_length and rejects because existing rows may hold values outside the new bounds.

Enum value removal: the engine detects enum values present in the database constraint but absent from the YAML and rejects because existing rows may hold the removed value.

Uniqueness tightening: adding a unique constraint to an existing field is rejected because existing rows may violate it. The duplication pattern applies.

Removing versioning: changing `versioned` from true to false is rejected because the sibling table contains historical data.

Removing soft delete: changing `soft_delete` from true to false is rejected because existing rows may use `is_active=false`.

### Transaction Safety

All DDL changes are executed within a single Postgres transaction. If any statement fails, the entire transaction rolls back and the database is unchanged. The engine takes a Postgres advisory lock at the start of the apply to prevent concurrent applies — a second concurrent invocation waits for the first to complete, then runs as a no-op because the database already matches the YAML.

---

## Schema Self-Description Tables

After applying DDL changes, the engine updates the `_schema_*` tables so the API can discover the schema at runtime.

### _schema_version

A new row is inserted with an incremented version_serial, `is_current` set to true, and the previous current row's `is_current` set to false. The version_label is generated from the current date and a sequence number (for example, "2026.05.05.01"). The parent pointer chains versions into a linear history.

### _schema_entity_type

One row per entity. On first load, all entities are inserted with `_schema_version_introduced_id` pointing to the initial version. On subsequent loads, new entities get rows with the new version. Deprecated entities (marked in YAML) get `_schema_version_deprecated_id` set. No rows are deleted — the entity type registry only grows.

### _schema_field

One row per field per entity, including injected reserved fields and governance fields. Same insert and deprecation logic as entity types. Each row records the field name, type, nullability, whether it's a primary key, whether it's a foreign key (and if so, which entity it references), default value, constraint details, and the schema version that introduced and optionally deprecated it.

### _schema_relationship

One row per foreign key relationship. Source entity, source field, target entity, cardinality (one_to_many for standard FK, many_to_many for bridge tables — detected by the entity having exactly two non-self foreign keys and no other required fields beyond reserved ones), and on_delete_action.

These tables make the schema queryable through the API like any other data. "Show me all fields added in the last schema version" is a standard search query. "Show me all deprecated fields" filters on `_schema_version_deprecated_id IS NOT NULL`. The API's schema validation step reads these tables to know what fields exist and what their types and constraints are — no hardcoded entity knowledge in the API.

---

## CLI Interface

The engine ships as a single binary, `opsdb-schema`, with six commands.

### opsdb-schema init

Creates a new schema repository with the directory structure, meta-schema file, conventions file, and an empty directory.yaml. Starting point for a new OpsDB or for organizations extending the standard schema.

### opsdb-schema validate

Validates all YAML files against the meta-schema without connecting to a database. Checks that every key is from the recognized vocabulary, every type is one of the nine allowed types, every constraint is valid for its type, every foreign key reference points to an entity that exists (and is loaded before the referencing entity), every name follows conventions, and no forbidden patterns are present.

This command runs in CI as a schema PR check. No database needed — pure YAML validation.

### opsdb-schema plan

Does everything `apply` does except execute the DDL. Connects to the database, reads current state, computes the diff, checks evolution rules, generates the DDL, and prints it. If any evolution violations would block the apply, those are printed instead.

This is the "what would happen" command. Run it before apply to see the exact DDL that will execute, or to see why a proposed schema change is forbidden.

### opsdb-schema apply

The full pipeline. Validate YAML. Connect to database. Acquire advisory lock. Read current state from `_schema_*` tables (or information_schema on bootstrap). Compute diff. Check evolution rules — if any forbidden change is detected, print the error and exit without modifying the database. Generate DDL for allowed changes. Execute DDL in a transaction. Update `_schema_*` tables. Commit. Release advisory lock.

If any step fails, the transaction rolls back. The database is either fully updated or completely unchanged.

### opsdb-schema diff

Compares YAML against current database state and shows differences in a human-readable format. New entities, new fields, changed constraints, deprecated fields. No DDL generated, no evolution rules checked — just the diff. Useful for understanding what has changed between the YAML and the database without the context of whether those changes are allowed.

### opsdb-schema export

Reads an existing Postgres database and generates YAML entity files from its schema. This enables adopting the engine against an existing database that was created through other means. The exported files follow the naming conventions and vocabulary but may need manual cleanup — inferred types might not perfectly match intent, and relationships might need annotation.

---

## Testing Strategy

### Unit Tests

Tests on the closed vocabulary enforcer verify that valid entity files are accepted and that files containing each forbidden pattern (regex, embedded logic, inheritance, conditionals, imports, templates) are rejected with the correct error.

Tests on type mapping verify each of the nine types generates the correct Postgres DDL.

Tests on reserved field injection verify that an entity with various combinations of `versioned`, `soft_delete`, `hierarchical`, and governance fields gets all the correct injected fields and, for versioned entities, a correct sibling table.

Tests on dependency resolution verify that entities with various foreign key relationships produce the correct topological order and that cycles are detected and rejected.

### Integration Tests

Integration tests run against a real Postgres instance (via testcontainers or a dedicated test database).

**Fresh apply.** Load the full 138-entity schema into an empty database. Verify all tables exist with correct columns, types, constraints, indexes, and foreign key relationships. Verify `_schema_*` tables are populated correctly. Verify `audit_log_entry` has REVOKE applied.

**Idempotent re-apply.** Run apply again with no YAML changes. Verify zero DDL executed. Verify `_schema_*` tables unchanged.

**Every allowed evolution type.** Add a new nullable field, a new entity, a new enum value, widen a numeric range, widen a length bound, add a new index. Verify each generates the correct DDL and updates `_schema_*` tables with the correct version references.

**Every forbidden evolution type.** Attempt field deletion, rename, type change, range narrowing, length narrowing, enum removal, uniqueness tightening, versioning removal, soft-delete removal. Verify each is rejected with the correct error message and zero database modifications.

**Data preservation.** Insert rows into tables. Run apply with additive changes. Verify existing data is intact and accessible.

**Concurrent safety.** Two concurrent applies — the second waits for the advisory lock, then runs as a no-op because the first already applied the changes.
