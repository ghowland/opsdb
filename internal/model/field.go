
// === internal/model/field.go ===
package model

// Field represents a single field within an entity definition.
// Carries type, constraints, modifiers, and metadata.
type Field struct {
	Name                   string
	Type                   string // one of nine: int, float, varchar, text, boolean, datetime, json, enum, foreign_key
	Nullable               bool
	Description            string
	Default                interface{} // literal value only; nil if no default
	Unique                 bool
	References             string   // target entity name for foreign_key type
	MaxLength              int      // required for varchar, optional for text
	MinLength              int      // optional for varchar
	MaxValue               *float64 // inclusive bound for int/float; pointer to distinguish unset from zero
	MinValue               *float64 // inclusive bound for int/float
	PrecisionDecimalPlaces *int     // optional for float
	EnumValues             []string // required for enum type
	JsonTypeDiscriminator  string   // required for json type; names sibling enum field
	MustBeUniqueWithin     []string // field names forming composite uniqueness scope

	// Set by injector, not parsed from YAML
	IsReserved   bool // true for id, created_time, updated_time, is_active, parent_*_id, versioning fields
	IsGovernance bool // true for _requires_group, _access_classification, etc.
}
