

// === internal/model/relationship.go ===
package model

// Relationship represents a foreign key relationship between two entities.
// Derived by the resolver from FK fields. Used for dependency ordering
// and _schema_relationship population.
type Relationship struct {
	SourceEntity      string
	SourceField       string
	TargetEntity      string
	Cardinality       string // one_to_one, one_to_many, many_to_many
	OnDeleteAction    string // cascade, restrict, set_null
	IsSelfReferential bool   // true when SourceEntity == TargetEntity
}
