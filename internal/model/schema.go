

// === internal/model/schema.go ===
package model

// Schema is the complete in-memory representation of the full OpsDB schema.
// Built incrementally by the pipeline: parser → validator → resolver → injector.
// Consumed by differ, evolution checker, generator, applier, meta populator.
type Schema struct {
	Entities      map[string]*Entity    // keyed by entity name
	Relationships []Relationship
	LoadOrder     []string              // topologically sorted entity names
	MetaSchema    *MetaSchema           // parsed from _schema_meta.yaml
	Reserved      *ReservedConfig       // parsed from conventions/reserved.yaml
	Version       SchemaVersionInfo
	Errors        []SchemaError         // accumulated across pipeline stages
}

// SchemaVersionInfo holds version metadata for a schema apply.
type SchemaVersionInfo struct {
	Serial int
	Label  string // "2026.05.05.01" format
}

// MetaSchema represents the parsed meta-schema definition.
// Defines what constitutes a valid entity file.
type MetaSchema struct {
	AllowedTopLevelKeys    []string
	AllowedTypes           []TypeDefinition
	FieldDefinition        FieldSchemaDefinition
	IndexDefinition        IndexSchemaDefinition
	GovernanceDefinition   GovernanceSchemaDefinition
	ForbiddenPatterns      []ForbiddenPattern
	EvolutionRules         EvolutionRuleSet
	ReservedFieldNames     []string
}

// TypeDefinition defines one of the nine allowed types.
type TypeDefinition struct {
	Name                string
	PostgresType        string
	AllowedConstraints  []string
	RequiredConstraints []string
	AllowedModifiers    []string
	ForbiddenModifiers  []string
}

// FieldSchemaDefinition defines what keys are valid in a field definition.
type FieldSchemaDefinition struct {
	AllowedKeys  []string
	RequiredKeys []string
}

// IndexSchemaDefinition defines what keys are valid in an index definition.
type IndexSchemaDefinition struct {
	AllowedKeys  []string
	RequiredKeys []string
}

// GovernanceSchemaDefinition defines valid governance keys and groups.
type GovernanceSchemaDefinition struct {
	AllowedKeys []string
	FieldGroups map[string][]string // group name → required-together fields
}

// ForbiddenPattern defines a pattern the engine scans for and rejects.
type ForbiddenPattern struct {
	Name        string
	Description string
	Rationale   string
	Alternative string
}

// EvolutionRuleSet holds allowed and forbidden evolution rules.
type EvolutionRuleSet struct {
	Allowed   []EvolutionRule
	Forbidden []ForbiddenEvolutionRule
}

// EvolutionRule defines an allowed schema change.
type EvolutionRule struct {
	Name        string
	Description string
}

// ForbiddenEvolutionRule defines a forbidden schema change with error template.
type ForbiddenEvolutionRule struct {
	Name          string
	ErrorTemplate string
	Alternative   string
}

// ReservedConfig holds the parsed conventions/reserved.yaml content.
type ReservedConfig struct {
	Universal             []Field
	SoftDelete            []Field
	Hierarchical          []Field // contains {entity_name} placeholders
	VersioningSibling     []Field // contains {entity_name} placeholders
	VersioningConstraints []ConstraintDef
	Governance            []GovernanceFieldDef
	Observation           []GovernanceFieldDef
	SchemaMetadata        []GovernanceFieldDef
	NamingConventions     NamingConventionConfig
	AppendOnly            AppendOnlyConfig
	DatabaseRoles         []RoleDefinition
}

// GovernanceFieldDef is a governance field with its enable key.
type GovernanceFieldDef struct {
	Field     Field
	EnabledBy string // key in entity governance section that enables this field
}

// ConstraintDef defines a constraint to generate on sibling tables.
type ConstraintDef struct {
	Type   string   // "unique_composite"
	Fields []string // may contain {entity_name} placeholders
}

// NamingConventionConfig holds naming convention rules.
type NamingConventionConfig struct {
	// Loaded from YAML; used by conventions/naming.go
}

// AppendOnlyConfig holds append-only enforcement config.
type AppendOnlyConfig struct {
	PostgresRevokeRoles []string
}

// RoleDefinition defines a database role to create.
type RoleDefinition struct {
	Name        string
	Description string
	Grants      []string
}

