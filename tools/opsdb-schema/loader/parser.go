//# tools/opsdb-schema/loader/parser.go

go
package loader

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/model"
)

// MetaSchema holds the parsed meta-schema that validates entity YAML files.
// Loaded from schema/meta/_schema_meta.yaml.
type MetaSchema struct {
	AllowedTopLevelKeys   []string
	AllowedFieldKeys      []string
	AllowedIndexKeys      []string
	AllowedGovernanceKeys []string
	AllowedCategories     []string
	Version               string
}

// ReservedConfig holds parsed reserved field conventions.
// Loaded from schema/conventions/reserved.yaml.
type ReservedConfig struct {
	UniversalFields      []model.Field
	SoftDeleteFields     []model.Field
	GovernanceFields     map[string]model.Field // keyed by governance flag name
	VersionSiblingFields []model.Field          // template fields for {entity}_version tables
	DatabaseRoles        []RoleDefinition
}

// RoleDefinition describes a database role for DDL generation.
type RoleDefinition struct {
	Name        string
	Permissions []string // SELECT, INSERT, UPDATE, DELETE per table category
	AppliesTo   string   // all, append_only, versioned, observation
}

// ParseEntityFile reads and parses a single entity YAML file.
// Returns both the structured Entity and the raw YAML map.
// The raw map is consumed by the forbidden pattern scanner.
func ParseEntityFile(path string) (*model.Entity, map[string]interface{}, error) {
	// TODO: read file contents from path
	// TODO: parse YAML into raw map[string]interface{}
	// TODO: validate top-level keys against allowed set:
	//   name, description, category, versioned, soft_delete, hierarchical,
	//   append_only, fields, indexes, governance
	//   reject unknown keys with error identifying key and file
	// TODO: extract entity metadata:
	//   Name from "name" key (required)
	//   Description from "description" key (optional)
	//   Category from "category" key (required)
	//   Versioned from "versioned" key (default false)
	//   SoftDelete from "soft_delete" key (default false)
	//   Hierarchical from "hierarchical" key (default false)
	//   AppendOnly from "append_only" key (default false)
	// TODO: parse fields list:
	//   for each field in "fields" list:
	//     extract Name, Type, Nullable, Description, Default, Unique,
	//     References, MaxLength, MinLength, MaxValue, MinValue,
	//     PrecisionDecimalPlaces, EnumValues, JsonTypeDiscriminator,
	//     MustBeUniqueWithin
	//     create model.Field struct
	// TODO: parse indexes list (if present):
	//   for each index in "indexes" list:
	//     extract fields, unique flag, name
	// TODO: parse governance map (if present):
	//   for each key in "governance" map:
	//     record which governance fields are enabled
	// TODO: return entity, raw map, nil
	return nil, nil, fmt.Errorf("not implemented")
}

// ParseDirectoryYAML reads directory.yaml and returns the ordered list
// of entity file paths to process. Paths are relative to the schema directory.
func ParseDirectoryYAML(path string) ([]string, error) {
	// TODO: read file contents from path
	// TODO: parse YAML
	// TODO: extract "imports" key as list of strings
	// TODO: each entry is a relative path to an entity YAML file
	//   e.g. "domains/01_identity/site.yaml"
	// TODO: validate no duplicates in import list
	// TODO: return ordered path list
	return nil, fmt.Errorf("not implemented")
}

// ParseMetaSchema reads and parses the meta-schema file.
// The meta-schema defines what keys and values are permitted in entity YAML files.
func ParseMetaSchema(path string) (*MetaSchema, error) {
	// TODO: read file contents from path
	// TODO: parse YAML
	// TODO: extract allowed_top_level_keys, allowed_field_keys, allowed_index_keys,
	//   allowed_governance_keys, allowed_categories, version
	// TODO: validate meta-schema has required sections
	// TODO: return MetaSchema
	return nil, fmt.Errorf("not implemented")
}

// ParseReserved reads and parses the reserved field conventions file.
func ParseReserved(path string) (*ReservedConfig, error) {
	// TODO: read file contents from path
	// TODO: parse YAML
	// TODO: extract universal_fields section -> []model.Field
	// TODO: extract soft_delete_fields section -> []model.Field
	// TODO: extract governance_fields section -> map[string]model.Field
	// TODO: extract version_sibling_fields section -> []model.Field
	// TODO: extract database_roles section -> []RoleDefinition
	// TODO: return ReservedConfig
	return nil, fmt.Errorf("not implemented")
}

// ParseJSONSchema reads and parses a JSON payload schema file.
// Used to validate typed payloads (cloud_resource, authority, policy, etc.).
func ParseJSONSchema(path string) (*JSONPayloadSchema, error) {
	// TODO: read file contents from path
	// TODO: parse YAML (JSON schemas are stored as YAML for consistency)
	// TODO: extract fields with their types and constraints
	// TODO: validate against JSON payload vocabulary rules:
	//   one level deep only, no lists-of-lists, no maps-of-lists
	// TODO: return JSONPayloadSchema
	return nil, fmt.Errorf("not implemented")
}

// JSONPayloadSchema holds the parsed structure of a JSON payload schema.
type JSONPayloadSchema struct {
	Name        string
	Description string
	Fields      []JSONPayloadField
}

// JSONPayloadField represents one field in a JSON payload schema.
type JSONPayloadField struct {
	Name        string
	Type        string // string, int, float, bool, list, map
	Required    bool
	Description string
	Constraints map[string]interface{}
}


