// === internal/model/entity.go ===
package model

// Entity represents a parsed entity definition from a YAML file.
// Central data structure consumed by every stage of the schema engine pipeline.
type Entity struct {
	Name         string
	Description  string
	Category     string
	Versioned    bool
	SoftDelete   bool
	Hierarchical bool
	AppendOnly   bool
	Fields       []Field
	Indexes      []Index
	Governance   map[string]bool

	// Set by injector after parsing
	IsSibling    bool   // true if this entity was generated as a versioning sibling
	ParentEntity string // if IsSibling, name of the parent entity
}

// Index represents an index declaration on an entity.
type Index struct {
	Fields      []string
	Unique      bool
	Description string
}

// SchemaError represents a validation error with context.
type SchemaError struct {
	Entity   string
	Field    string
	Message  string
	Severity string // "error" or "warning"
}

