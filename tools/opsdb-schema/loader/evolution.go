//# tools/opsdb-schema/loader/evolution.go

go
package loader

import (
	"fmt"
	"strings"
)

// EvolutionResult holds the outcome of evolution rule checking.
// Allowed changes can proceed to DDL generation. Forbidden changes
// block the apply and require alternative approaches.
type EvolutionResult struct {
	Allowed   []AllowedChange
	Forbidden []ForbiddenChange
}

// AllowedChange represents one change that passes evolution rules.
type AllowedChange struct {
	Entity      string
	Field       string
	ChangeType  string // new_entity, new_field, widen_range, add_enum, new_index, add_approval_rule
	Description string
	DiffItem    DiffItem
}

// ForbiddenChange represents one change that violates evolution rules.
type ForbiddenChange struct {
	Entity      string
	Field       string
	Rule        string // delete_field, rename_field, type_change, narrow_range, remove_enum, add_uniqueness
	Reason      string
	Alternative string
	DiffItem    DiffItem
}

// CheckEvolution classifies each change in a SchemaDiff as allowed or forbidden.
// Returns the full classification so the caller can present allowed changes
// for plan/apply and forbidden changes as errors with alternatives.
func CheckEvolution(diff *SchemaDiff) (*EvolutionResult, error) {
	result := &EvolutionResult{}

	// TODO: new entities are always allowed
	//   for each entity in diff.NewEntities:
	//     result.Allowed = append(result.Allowed, AllowedChange{
	//       Entity: entity, ChangeType: "new_entity", Description: "add new entity type"})

	// TODO: new fields — allowed if nullable
	//   for each item in diff.NewFields:
	//     check if the field is nullable (from desired schema)
	//     if nullable: allowed (new_field)
	//     if not nullable and has default: allowed (new_field with default)
	//     if not nullable and no default: forbidden (would break existing rows)
	//       alternative: "add as nullable first, backfill, then consider tightening via new field"

	// TODO: changed constraints — classify each
	//   for each item in diff.ChangedConstraints:
	//     if numeric range widened (min decreased OR max increased): allowed (widen_range)
	//     if numeric range narrowed (min increased OR max decreased): forbidden (narrow_range)
	//       alternative: "use duplication pattern: add new field with tighter range"
	//     if string length widened (max_length increased): allowed (widen_length)
	//     if string length narrowed: forbidden
	//       alternative: "use duplication pattern: add new field with shorter length"
	//     if enum values added: allowed (add_enum)
	//     if enum values removed: forbidden (remove_enum)
	//       alternative: "use duplication pattern: add new field with narrower enum set"
	//     if uniqueness added: forbidden (add_uniqueness)
	//       alternative: "use duplication pattern: add new unique field"

	// TODO: new indexes are always allowed
	//   for each item in diff.NewIndexes:
	//     result.Allowed = append(...)

	// TODO: removed fields are always forbidden
	//   for each item in diff.RemovedFields:
	//     result.Forbidden = append(result.Forbidden, ForbiddenChange{
	//       Entity: item.Entity, Field: item.Field,
	//       Rule: "delete_field",
	//       Reason: "deleting fields breaks history, version rows, and audit log references",
	//       Alternative: "deprecate the field: mark _schema_field deprecated; column and data remain as tombstone"})

	// TODO: removed entities are always forbidden
	//   for each entity in diff.RemovedEntities:
	//     result.Forbidden = append(result.Forbidden, ForbiddenChange{
	//       Entity: entity, Rule: "delete_entity",
	//       Reason: "deleting entities breaks all consumers and audit references",
	//       Alternative: "deprecate: mark all fields deprecated; table remains empty"})

	// TODO: type changes are always forbidden
	//   for each item in diff.TypeChanges:
	//     check for rename heuristic: isFieldRename(item, diff)
	//     if rename detected:
	//       rule = "rename_field"
	//       alternative = "add new field with new name, deprecate old (duplication pattern)"
	//     else:
	//       rule = "type_change"
	//       alternative = "add new field with new type, double-write, migrate readers, deprecate old"
	//     result.Forbidden = append(...)

	_ = diff
	return result, fmt.Errorf("not implemented")
}

// isFieldRename detects probable renames: a field disappeared from an entity
// and a new field with the same type appeared in the same entity.
func isFieldRename(removed DiffItem, diff *SchemaDiff) bool {
	// TODO: for each new field in diff.NewFields:
	//   if same entity as removed field:
	//     if same type as removed field:
	//       return true (probable rename)
	// TODO: return false
	_ = removed
	_ = diff
	return false
}

// isRangeNarrowing checks if a numeric constraint change narrows the allowed range.
func isRangeNarrowing(item DiffItem) bool {
	// TODO: compare desired vs current min_value and max_value
	// TODO: narrowing = desired min > current min OR desired max < current max
	_ = item
	return false
}

// isEnumValueRemoval checks if an enum constraint change removes values.
func isEnumValueRemoval(item DiffItem) bool {
	// TODO: get current enum values and desired enum values
	// TODO: for each current value: if not in desired set, return true
	_ = item
	return false
}

// formatForbiddenMessage creates a human-readable error message for a forbidden change.
func formatForbiddenMessage(change ForbiddenChange) string {
	parts := []string{
		fmt.Sprintf("FORBIDDEN: %s", change.Rule),
	}
	if change.Entity != "" {
		parts = append(parts, fmt.Sprintf("entity: %s", change.Entity))
	}
	if change.Field != "" {
		parts = append(parts, fmt.Sprintf("field: %s", change.Field))
	}
	parts = append(parts, fmt.Sprintf("reason: %s", change.Reason))
	parts = append(parts, fmt.Sprintf("alternative: %s", change.Alternative))
	return strings.Join(parts, "\n  ")
}

