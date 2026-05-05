# Schema Evolution

How to change the OpsDB schema safely.

## The rules

### Always allowed

| Change | Why safe |
|--------|----------|
| Add new entity type | Entirely additive, no impact on existing entities |
| Add nullable field | Existing rows have NULL, no breakage |
| Add new enum values | Existing rows hold previous values, still valid |
| Widen numeric range (increase max, decrease min) | Existing values still in range |
| Widen string length (increase max_length) | Existing strings still fit |
| Add new index | Performance improvement, no data impact |
| Add new approval rule reference | Tightens governance for new changes only |

### Always forbidden

| Change | Why forbidden | Alternative |
|--------|--------------|-------------|
| Delete field | Breaks history, version rows, audit log | Deprecate: mark in `_schema_field`, column stays forever |
| Rename field | Breaks every consumer | Add new field with new name, deprecate old |
| Change field type | Breaks consumers and stored data | Duplication pattern (see below) |
| Narrow numeric range | Existing values may violate new bounds | Add new field with tighter range |
| Narrow string length | Existing strings may exceed new bound | Add new field with shorter length |
| Remove enum values | Existing rows may hold removed value | Add new field with narrower enum set |
| Add uniqueness constraint | Existing rows may violate constraint | Add new unique field |

## The duplication pattern

When you need to change a field's type, range, enum set, or name:

1. **Add new field** with the desired type/constraints (nullable).
2. **Begin double-writing** — all code writing the old field also writes the new.
3. **Migrate readers** — code reading from the old field switches to the new.
4. **Mark old field deprecated** — set `_schema_version_deprecated_id` on the field.
5. **Continue double-writing** — old field becomes a tombstone for safety.
6. **Never remove the old field** — it stays forever. Storage cost is the price of stable history.

Typical timeline: 3-5 successful release cycles for double-writing, then
migration, then deprecation.

## How to make a schema change

### 1. Edit the YAML

Modify the entity file under `schema/domains/`. Add a field, add an entity,
widen a constraint.

### 2. Validate locally

```bash
bin/opsdb-schema validate --repo .
```

This parses all YAML, checks naming conventions, validates types and constraints,
scans for forbidden patterns, and reports errors. No database needed.

### 3. Plan against your database

```bash
bin/opsdb-schema plan --repo . --dsn "$OPSDB_DSN"
```

This diffs the YAML against the current database, checks evolution rules, and
shows the DDL that would be generated. Forbidden changes are flagged with the
rule name and the alternative approach.

### 4. Apply (dev) or submit change set (operational)

In dev:

```bash
bin/opsdb-schema apply --repo . --dsn "$OPSDB_DSN" --verbose
```

In operational: commit the YAML change, create a `_schema_change_set` through
the API, wait for schema steward approval, and the schema executor runner
applies it.

### 5. Dry-run first

```bash
bin/opsdb-schema apply --repo . --dsn "$OPSDB_DSN" --dry-run
```

Executes all DDL inside a transaction then rolls back. Validates everything
is correct without persisting.

## Schema steward role

Every schema change set requires approval from a member of the `schema_stewards`
group. The steward reviews for comprehensive coherence: does this change fit the
whole schema, does it follow naming conventions, does it slice the pie correctly,
does it avoid aggregated-system patterns.

## JSON payload schema changes

JSON payload schemas (under `schema/json_schemas/`) follow the same rules.
Adding fields to a payload schema is allowed. Removing fields is forbidden.
Type changes use the duplication pattern at the payload level.
