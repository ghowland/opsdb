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
	ChangeType  string
	Description string
	DiffItem    DiffItem
}

// ForbiddenChange represents one change that violates evolution rules.
type ForbiddenChange struct {
	Entity      string
	Field       string
	Rule        string
	Reason      string
	Alternative string
	DiffItem    DiffItem
}

// CheckEvolution classifies each change in a SchemaDiff as allowed or forbidden.
// Returns the full classification so the caller can present allowed changes
// for plan/apply and forbidden changes as errors with alternatives.
func CheckEvolution(diff *SchemaDiff) (*EvolutionResult, error) {
	result := &EvolutionResult{}

	// New entities are always allowed.
	for _, entity := range diff.NewEntities {
		result.Allowed = append(result.Allowed, AllowedChange{
			Entity:      entity,
			ChangeType:  "new_entity",
			Description: fmt.Sprintf("add new entity type %s", entity),
		})
	}

	// New fields — allowed. The validator already enforced that new fields
	// must be nullable (evolution rule A1). If a non-nullable non-default
	// field slipped through, the database ALTER will fail at apply time,
	// which is the correct enforcement point.
	for _, item := range diff.NewFields {
		desiredType, _ := item.DesiredValue.(string)
		result.Allowed = append(result.Allowed, AllowedChange{
			Entity:      item.Entity,
			Field:       item.Field,
			ChangeType:  "new_field",
			Description: fmt.Sprintf("add field %s.%s (%s)", item.Entity, item.Field, desiredType),
			DiffItem:    item,
		})
	}

	// Changed constraints — classify each.
	for _, item := range diff.ChangedConstraints {
		classifyConstraintChange(result, item)
	}

	// New indexes are always allowed.
	for _, item := range diff.NewIndexes {
		result.Allowed = append(result.Allowed, AllowedChange{
			Entity:      item.Entity,
			ChangeType:  "new_index",
			Description: item.Description,
			DiffItem:    item,
		})
	}

	// Removed fields are always forbidden.
	for _, item := range diff.RemovedFields {
		result.Forbidden = append(result.Forbidden, ForbiddenChange{
			Entity:      item.Entity,
			Field:       item.Field,
			Rule:        "delete_field",
			Reason:      "deleting fields breaks history, version rows, and audit log field change references",
			Alternative: "deprecate the field: mark _schema_field deprecated; column and data remain as tombstone forever",
			DiffItem:    item,
		})
	}

	// Removed entities are always forbidden.
	for _, entity := range diff.RemovedEntities {
		result.Forbidden = append(result.Forbidden, ForbiddenChange{
			Entity:      entity,
			Rule:        "delete_entity",
			Reason:      "deleting entities breaks all consumers and audit references",
			Alternative: "deprecate: mark all fields deprecated; table remains as empty tombstone",
		})
	}

	// Type changes are always forbidden.
	for _, item := range diff.TypeChanges {
		if isFieldRename(item, diff) {
			result.Forbidden = append(result.Forbidden, ForbiddenChange{
				Entity:      item.Entity,
				Field:       item.Field,
				Rule:        "rename_field",
				Reason:      "this appears to be a field rename (field removed + new field with same type added); renames break every consumer",
				Alternative: "add new field with new name, deprecate old; both coexist indefinitely (duplication pattern)",
				DiffItem:    item,
			})
		} else {
			result.Forbidden = append(result.Forbidden, ForbiddenChange{
				Entity:      item.Entity,
				Field:       item.Field,
				Rule:        "type_change",
				Reason:      fmt.Sprintf("changing field type from %v to %v breaks consumers and stored data", item.CurrentValue, item.DesiredValue),
				Alternative: "add new field with new type, begin double-writing, migrate readers, deprecate old, never remove (duplication pattern)",
				DiffItem:    item,
			})
		}
	}

	return result, nil
}

// classifyConstraintChange determines whether a constraint change is
// allowed (widening) or forbidden (narrowing).
func classifyConstraintChange(result *EvolutionResult, item DiffItem) {
	desc := strings.ToLower(item.Description)

	// Max length changes.
	if strings.Contains(desc, "max_length") {
		desiredVal, desiredOK := toNumeric(item.DesiredValue)
		currentVal, currentOK := toNumeric(item.CurrentValue)

		if desiredOK && currentOK {
			if desiredVal >= currentVal {
				result.Allowed = append(result.Allowed, AllowedChange{
					Entity:      item.Entity,
					Field:       item.Field,
					ChangeType:  "widen_length",
					Description: fmt.Sprintf("widen max_length on %s.%s: %v -> %v", item.Entity, item.Field, item.CurrentValue, item.DesiredValue),
					DiffItem:    item,
				})
				return
			}
			result.Forbidden = append(result.Forbidden, ForbiddenChange{
				Entity:      item.Entity,
				Field:       item.Field,
				Rule:        "narrow_length",
				Reason:      fmt.Sprintf("narrowing max_length from %v to %v may invalidate existing strings", item.CurrentValue, item.DesiredValue),
				Alternative: "use duplication pattern: add new field with shorter max_length, migrate, deprecate old",
				DiffItem:    item,
			})
			return
		}
	}

	// Numeric range changes (min_value, max_value).
	if strings.Contains(desc, "min_value") || strings.Contains(desc, "max_value") {
		desiredVal, desiredOK := toNumeric(item.DesiredValue)
		currentVal, currentOK := toNumeric(item.CurrentValue)

		if desiredOK && currentOK {
			if isRangeWidening(desc, desiredVal, currentVal) {
				result.Allowed = append(result.Allowed, AllowedChange{
					Entity:      item.Entity,
					Field:       item.Field,
					ChangeType:  "widen_range",
					Description: fmt.Sprintf("widen range on %s.%s: %v -> %v", item.Entity, item.Field, item.CurrentValue, item.DesiredValue),
					DiffItem:    item,
				})
				return
			}
			result.Forbidden = append(result.Forbidden, ForbiddenChange{
				Entity:      item.Entity,
				Field:       item.Field,
				Rule:        "narrow_range",
				Reason:      fmt.Sprintf("narrowing numeric range from %v to %v may invalidate existing values", item.CurrentValue, item.DesiredValue),
				Alternative: "use duplication pattern: add new field with tighter range, migrate, deprecate old",
				DiffItem:    item,
			})
			return
		}
	}

	// Nullable changes.
	if strings.Contains(desc, "nullable") {
		desiredNullable, dOK := item.DesiredValue.(bool)
		currentNullable, cOK := item.CurrentValue.(bool)

		if dOK && cOK {
			if desiredNullable && !currentNullable {
				// Making nullable: always safe (widening).
				result.Allowed = append(result.Allowed, AllowedChange{
					Entity:      item.Entity,
					Field:       item.Field,
					ChangeType:  "make_nullable",
					Description: fmt.Sprintf("make %s.%s nullable", item.Entity, item.Field),
					DiffItem:    item,
				})
				return
			}
			if !desiredNullable && currentNullable {
				// Making non-nullable: forbidden (existing NULLs would violate).
				result.Forbidden = append(result.Forbidden, ForbiddenChange{
					Entity:      item.Entity,
					Field:       item.Field,
					Rule:        "tighten_nullable",
					Reason:      "making a nullable field non-nullable may break existing rows with NULL values",
					Alternative: "add new non-nullable field with default, migrate data, deprecate old nullable field",
					DiffItem:    item,
				})
				return
			}
		}
	}

	// If we can't classify the constraint change, treat as allowed with a note.
	// This covers cases like adding enum values where the description doesn't
	// match the specific patterns above.
	result.Allowed = append(result.Allowed, AllowedChange{
		Entity:      item.Entity,
		Field:       item.Field,
		ChangeType:  "changed_constraint",
		Description: item.Description,
		DiffItem:    item,
	})
}

// isFieldRename detects probable renames: a field disappeared from an entity
// and a new field with the same type appeared in the same entity.
func isFieldRename(removed DiffItem, diff *SchemaDiff) bool {
	removedType := fmt.Sprintf("%v", removed.CurrentValue)

	for _, newField := range diff.NewFields {
		if newField.Entity == removed.Entity {
			newType := fmt.Sprintf("%v", newField.DesiredValue)
			if newType == removedType {
				return true
			}
		}
	}
	return false
}

// isRangeWidening determines if a numeric constraint change is widening.
// For min_value: widening means desired <= current (lower minimum).
// For max_value: widening means desired >= current (higher maximum).
func isRangeWidening(desc string, desired float64, current float64) bool {
	if strings.Contains(desc, "min") {
		return desired <= current
	}
	if strings.Contains(desc, "max") {
		return desired >= current
	}
	return false
}

// isRangeNarrowing checks if a numeric constraint change narrows the allowed range.
func isRangeNarrowing(item DiffItem) bool {
	desc := strings.ToLower(item.Description)
	desiredVal, desiredOK := toNumeric(item.DesiredValue)
	currentVal, currentOK := toNumeric(item.CurrentValue)

	if !desiredOK || !currentOK {
		return false
	}

	if strings.Contains(desc, "min") {
		return desiredVal > currentVal // raising minimum narrows range
	}
	if strings.Contains(desc, "max") {
		return desiredVal < currentVal // lowering maximum narrows range
	}
	return false
}

// isEnumValueRemoval checks if an enum constraint change removes values.
func isEnumValueRemoval(item DiffItem) bool {
	currentValues, cOK := toStringSet(item.CurrentValue)
	desiredValues, dOK := toStringSet(item.DesiredValue)

	if !cOK || !dOK {
		return false
	}

	for _, cv := range currentValues {
		found := false
		for _, dv := range desiredValues {
			if cv == dv {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
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

// toNumeric converts an interface{} to float64.
func toNumeric(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

// toStringSet converts an interface{} to a string slice.
// Handles []string and []interface{}.
func toStringSet(v interface{}) ([]string, bool) {
	switch s := v.(type) {
	case []string:
		return s, true
	case []interface{}:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, len(result) == len(s)
	default:
		return nil, false
	}
}
