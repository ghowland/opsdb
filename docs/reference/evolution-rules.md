# Evolution Rules Reference

Complete list of what the schema loader allows and forbids when comparing
desired state (YAML) against current state (database).

## Allowed Changes

| ID | Change | Condition | Why safe |
|----|--------|-----------|----------|
| A1 | Add new field (nullable) | `nullable: true` | Existing rows have NULL |
| A2 | Add new enum values | Values appended to enum_values list | Existing rows hold previous values |
| A3 | Widen numeric range | `min_value` decreased OR `max_value` increased | Existing values still in range |
| A4 | Widen string length | `max_length` increased | Existing strings still fit |
| A5 | Add new entity type | Entirely new table | No impact on existing entities |
| A6 | Add new index | New CREATE INDEX | Performance improvement only |
| A7 | Add new approval rule reference | Tightens governance for new changes | Existing rows unaffected |

## Forbidden Changes

| ID | Change | Why forbidden | Alternative |
|----|--------|--------------|-------------|
| F1 | Delete field | Breaks history, version rows, audit log field change references | Deprecate: set `_schema_version_deprecated_id`. Column and data remain as tombstone forever. |
| F2 | Rename field or entity | Breaks every consumer | Add new field with new name. Deprecate old. Both coexist. |
| F3 | Change field type | Breaks consumers and stored data | Duplication pattern: add new field with new type, double-write, migrate readers, deprecate old, never remove. |
| F4 | Narrow numeric range | Existing rows may hold values now out of range | Add new field with tighter range via duplication pattern. |
| F5 | Narrow string length | Existing strings may exceed new bound | Add new field with shorter max_length. |
| F6 | Remove enum values | Existing rows may hold removed value | Add new field with narrower enum set. |
| F7 | Add uniqueness constraint | Existing rows may violate constraint | Add new unique field via duplication pattern. |
| F8 | Remove index | Usually a mistake | Schema steward must verify manually. Loader does not enforce. |

## Duplication Pattern Steps

Used for type changes, renames, range narrowing, and enum narrowing.

| Step | Action | Duration |
|------|--------|----------|
| 1 | Add new field with desired type/constraints | Both fields exist |
| 2 | Begin double-writing to both fields | 3-5 release cycles |
| 3 | Migrate readers from old field to new | Until reads of old field are zero |
| 4 | Mark old field deprecated | Immediate |
| 5 | Continue double-writing as safety measure | Additional cycles |
| 6 | Old field never removed | Indefinitely — tombstone |

## Detection Heuristics

The evolution checker uses heuristics for some classifications:

- **Rename detection:** field disappears from entity AND new field with same type appears in same entity. Flagged as probable rename with alternative guidance.
- **Range narrowing:** current min < desired min OR current max > desired max.
- **Enum removal:** any value in current enum_values not present in desired enum_values.

## Loader Behavior

When the loader encounters forbidden changes:

1. The `plan` command prints each forbidden change with rule ID, entity, field, reason, and alternative.
2. The `apply` command refuses to execute. No DDL is generated for forbidden changes. Exit code 1.
3. Allowed and forbidden changes are reported separately. Allowed changes can proceed in a subsequent run after forbidden changes are resolved.

## Schema Steward Review

All schema changes (allowed or forbidden) are visible in the `plan` output.
The schema steward reviews for comprehensive coherence beyond what the loader
checks mechanically: naming consistency, pie-slicing correctness, category
assignment, unnecessary complexity.
