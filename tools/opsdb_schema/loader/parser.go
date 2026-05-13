package loader

import (
	"fmt"
	"os"
	"strings"

	"github.com/ghowland/opsdb/internal/model"
	"gopkg.in/yaml.v3"
)

// MetaSchema holds the parsed meta-schema that validates entity YAML files.
// This is a simplified parse target for the loader — not the full
// model.MetaSchema which carries richer structure for runtime use.
type MetaSchema struct {
	AllowedTopLevelKeys   []string
	AllowedFieldKeys      []string
	AllowedIndexKeys      []string
	AllowedGovernanceKeys []string
	AllowedCategories     []string
	Version               string
}

// allowedTopLevelKeySet for fast lookup during entity parsing.
var defaultAllowedTopLevelKeys = map[string]bool{
	"name": true, "description": true, "category": true,
	"versioned": true, "soft_delete": true, "hierarchical": true,
	"append_only": true, "fields": true, "indexes": true, "governance": true,
}

// ParseEntityFile reads and parses a single entity YAML file.
// Returns both the structured Entity and the raw YAML map.
// The raw map is consumed by the forbidden pattern scanner.
func ParseEntityFile(path string) (*model.Entity, map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading entity file %s: %w", path, err)
	}

	var rawYAML map[string]interface{}
	if err := yaml.Unmarshal(data, &rawYAML); err != nil {
		return nil, nil, fmt.Errorf("parsing YAML in %s: %w", path, err)
	}

	// Validate top-level keys.
	for key := range rawYAML {
		if !defaultAllowedTopLevelKeys[key] {
			return nil, rawYAML, fmt.Errorf("unknown top-level key %q in %s", key, path)
		}
	}

	// Extract entity metadata.
	entity := &model.Entity{}

	name, ok := rawYAML["name"].(string)
	if !ok || name == "" {
		return nil, rawYAML, fmt.Errorf("missing or empty 'name' in %s", path)
	}
	entity.Name = name

	if desc, ok := rawYAML["description"].(string); ok {
		entity.Description = desc
	}

	category, ok := rawYAML["category"].(string)
	if !ok || category == "" {
		return nil, rawYAML, fmt.Errorf("missing or empty 'category' in %s", path)
	}
	entity.Category = category

	entity.Versioned = yamlBool(rawYAML, "versioned")
	entity.SoftDelete = yamlBool(rawYAML, "soft_delete")
	entity.Hierarchical = yamlBool(rawYAML, "hierarchical")
	entity.AppendOnly = yamlBool(rawYAML, "append_only")

	// Parse fields list.
	if fieldsRaw, ok := rawYAML["fields"]; ok {
		fieldsList, ok := fieldsRaw.([]interface{})
		if !ok {
			return nil, rawYAML, fmt.Errorf("'fields' is not a list in %s", path)
		}

		for i, fieldRaw := range fieldsList {
			fieldMap, ok := fieldRaw.(map[string]interface{})
			if !ok {
				return nil, rawYAML, fmt.Errorf("field %d is not a map in %s", i, path)
			}

			field, err := parseField(fieldMap, i, path)
			if err != nil {
				return nil, rawYAML, err
			}
			entity.Fields = append(entity.Fields, *field)
		}
	}

	// Parse indexes list.
	if indexesRaw, ok := rawYAML["indexes"]; ok {
		indexesList, ok := indexesRaw.([]interface{})
		if !ok {
			return nil, rawYAML, fmt.Errorf("'indexes' is not a list in %s", path)
		}

		for i, idxRaw := range indexesList {
			idxMap, ok := idxRaw.(map[string]interface{})
			if !ok {
				return nil, rawYAML, fmt.Errorf("index %d is not a map in %s", i, path)
			}

			idx, err := parseIndex(idxMap, i, path)
			if err != nil {
				return nil, rawYAML, err
			}
			entity.Indexes = append(entity.Indexes, *idx)
		}
	}

	// Parse governance map.
	if govRaw, ok := rawYAML["governance"]; ok {
		govMap, ok := govRaw.(map[string]interface{})
		if !ok {
			return nil, rawYAML, fmt.Errorf("'governance' is not a map in %s", path)
		}

		entity.Governance = make(map[string]bool)
		for key, val := range govMap {
			entity.Governance[key] = yamlBoolValue(val)
		}
	}

	return entity, rawYAML, nil
}

// parseField extracts a model.Field from a parsed YAML field map.
func parseField(fieldMap map[string]interface{}, index int, filePath string) (*model.Field, error) {
	field := &model.Field{}

	name, ok := fieldMap["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("field %d missing 'name' in %s", index, filePath)
	}
	field.Name = name

	fieldType, ok := fieldMap["type"].(string)
	if !ok || fieldType == "" {
		return nil, fmt.Errorf("field %q missing 'type' in %s", name, filePath)
	}
	field.Type = fieldType

	// Nullable defaults to true if not specified (safe default for evolution).
	if nullable, ok := fieldMap["nullable"]; ok {
		field.Nullable = yamlBoolValue(nullable)
	} else {
		field.Nullable = true
	}

	if desc, ok := fieldMap["description"].(string); ok {
		field.Description = desc
	}

	if def, ok := fieldMap["default"]; ok {
		field.Default = def
	}

	if unique, ok := fieldMap["unique"]; ok {
		field.Unique = yamlBoolValue(unique)
	}

	if ref, ok := fieldMap["references"].(string); ok {
		field.References = ref
	}

	// MaxLength and MinLength are plain int on model.Field (0 = unset).
	if ml, ok := fieldMap["max_length"]; ok {
		if v, err := yamlInt(ml); err == nil {
			field.MaxLength = v
		}
	}

	if ml, ok := fieldMap["min_length"]; ok {
		if v, err := yamlInt(ml); err == nil {
			field.MinLength = v
		}
	}

	// MaxValue and MinValue are *float64 on model.Field.
	if mv, ok := fieldMap["max_value"]; ok {
		field.MaxValue = yamlFloat64Ptr(mv)
	}

	if mv, ok := fieldMap["min_value"]; ok {
		field.MinValue = yamlFloat64Ptr(mv)
	}

	if pdp, ok := fieldMap["precision_decimal_places"]; ok {
		if v, err := yamlInt(pdp); err == nil {
			field.PrecisionDecimalPlaces = &v
		}
	}

	if ev, ok := fieldMap["enum_values"]; ok {
		values, err := yamlStringSlice(ev)
		if err != nil {
			return nil, fmt.Errorf("field %q enum_values in %s: %w", name, filePath, err)
		}
		field.EnumValues = values
	}

	if jtd, ok := fieldMap["json_type_discriminator"].(string); ok {
		field.JsonTypeDiscriminator = jtd
	}

	if mbuw, ok := fieldMap["must_be_unique_within"]; ok {
		values, err := yamlStringSlice(mbuw)
		if err != nil {
			return nil, fmt.Errorf("field %q must_be_unique_within in %s: %w", name, filePath, err)
		}
		field.MustBeUniqueWithin = values
	}

	return field, nil
}

// parseIndex extracts a model.Index from a parsed YAML index map.
// The model.Index has no Name field — an optional user-provided name
// is stored in Description. Index names for DDL are computed from
// entity name and field list by the generator.
func parseIndex(idxMap map[string]interface{}, index int, filePath string) (*model.Index, error) {
	idx := &model.Index{}

	// Optional name stored in Description (model.Index has no Name field).
	if name, ok := idxMap["name"].(string); ok {
		idx.Description = name
	}

	if fieldsRaw, ok := idxMap["fields"]; ok {
		fields, err := yamlStringSlice(fieldsRaw)
		if err != nil {
			return nil, fmt.Errorf("index %d 'fields' in %s: %w", index, filePath, err)
		}
		idx.Fields = fields
	} else {
		return nil, fmt.Errorf("index %d missing 'fields' in %s", index, filePath)
	}

	if len(idx.Fields) == 0 {
		return nil, fmt.Errorf("index %d has empty 'fields' in %s", index, filePath)
	}

	if unique, ok := idxMap["unique"]; ok {
		idx.Unique = yamlBoolValue(unique)
	}

	return idx, nil
}

// ParseDirectoryYAML reads directory.yaml and returns the ordered list
// of entity file paths to process. Paths are relative to the schema directory.
func ParseDirectoryYAML(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory.yaml: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing directory.yaml: %w", err)
	}

	importsRaw, ok := raw["imports"]
	if !ok {
		return nil, fmt.Errorf("directory.yaml missing 'imports' key")
	}

	imports, err := yamlStringSlice(importsRaw)
	if err != nil {
		return nil, fmt.Errorf("directory.yaml 'imports': %w", err)
	}

	// Check for duplicates.
	seen := make(map[string]bool, len(imports))
	for _, p := range imports {
		if seen[p] {
			return nil, fmt.Errorf("directory.yaml contains duplicate import: %s", p)
		}
		seen[p] = true
	}

	return imports, nil
}

// ParseMetaSchema reads and parses the meta-schema file.
// The meta-schema defines what keys and values are permitted in entity YAML files.
func ParseMetaSchema(path string) (*MetaSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading meta-schema: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing meta-schema YAML: %w", err)
	}

	meta := &MetaSchema{}

	if v, ok := raw["version"].(string); ok {
		meta.Version = v
	}

	var extractErr error

	meta.AllowedTopLevelKeys, extractErr = yamlStringSlice(raw["allowed_top_level_keys"])
	if extractErr != nil {
		return nil, fmt.Errorf("meta-schema allowed_top_level_keys: %w", extractErr)
	}

	meta.AllowedFieldKeys, extractErr = yamlStringSlice(raw["allowed_field_keys"])
	if extractErr != nil {
		return nil, fmt.Errorf("meta-schema allowed_field_keys: %w", extractErr)
	}

	meta.AllowedIndexKeys, extractErr = yamlStringSlice(raw["allowed_index_keys"])
	if extractErr != nil {
		return nil, fmt.Errorf("meta-schema allowed_index_keys: %w", extractErr)
	}

	meta.AllowedGovernanceKeys, extractErr = yamlStringSlice(raw["allowed_governance_keys"])
	if extractErr != nil {
		return nil, fmt.Errorf("meta-schema allowed_governance_keys: %w", extractErr)
	}

	meta.AllowedCategories, extractErr = yamlStringSlice(raw["allowed_categories"])
	if extractErr != nil {
		return nil, fmt.Errorf("meta-schema allowed_categories: %w", extractErr)
	}

	return meta, nil
}

// ParseReserved reads and parses the reserved field conventions file.
// Returns *model.ReservedConfig aligned to the model's structure.
func ParseReserved(path string) (*model.ReservedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading reserved conventions: %w", err)
	}

	var raw reservedYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing reserved conventions: %w", err)
	}

	config := &model.ReservedConfig{}

	// Universal fields (id, created_time, updated_time).
	for _, fd := range raw.Universal.Fields {
		config.Universal = append(config.Universal, yamlFieldToModel(fd, true, false))
	}

	// Soft delete fields (is_active).
	for _, fd := range raw.SoftDelete.Fields {
		config.SoftDelete = append(config.SoftDelete, yamlFieldToModel(fd, true, false))
	}

	// Hierarchical fields (parent_{entity}_id placeholders).
	for _, fd := range raw.Hierarchical.Fields {
		config.Hierarchical = append(config.Hierarchical, yamlFieldToModel(fd, true, false))
	}

	// Versioning sibling fields.
	for _, fd := range raw.VersioningSibling.Fields {
		config.VersioningSibling = append(config.VersioningSibling, yamlFieldToModel(fd, true, false))
	}

	// Governance fields — stored as []GovernanceFieldDef with EnabledBy.
	for _, fd := range raw.Governance.Fields {
		config.Governance = append(config.Governance, model.GovernanceFieldDef{
			Field:     yamlFieldToModel(fd, true, true),
			EnabledBy: fd.EnabledBy,
		})
	}

	// Observation fields — also governance-style with EnabledBy.
	for _, fd := range raw.Observation.Fields {
		config.Observation = append(config.Observation, model.GovernanceFieldDef{
			Field:     yamlFieldToModel(fd, true, true),
			EnabledBy: fd.EnabledBy,
		})
	}

	// Schema metadata fields — also governance-style with EnabledBy.
	for _, fd := range raw.SchemaMetadata.Fields {
		config.SchemaMetadata = append(config.SchemaMetadata, model.GovernanceFieldDef{
			Field:     yamlFieldToModel(fd, true, true),
			EnabledBy: fd.EnabledBy,
		})
	}

	// Append-only config.
	config.AppendOnly = model.AppendOnlyConfig{
		PostgresRevokeRoles: raw.AppendOnly.RevokeFromRoles,
	}

	// Database roles — model uses Grants, not Permissions.
	for _, rd := range raw.DatabaseRoles {
		config.DatabaseRoles = append(config.DatabaseRoles, model.RoleDefinition{
			Name:        rd.Name,
			Description: rd.Description,
			Grants:      rd.Grants,
		})
	}

	return config, nil
}

// reservedYAML mirrors the YAML structure of conventions/reserved.yaml.
type reservedYAML struct {
	Universal struct {
		Fields []reservedFieldYAML `yaml:"fields"`
	} `yaml:"universal"`
	SoftDelete struct {
		Fields []reservedFieldYAML `yaml:"fields"`
	} `yaml:"soft_delete"`
	Hierarchical struct {
		Fields []reservedFieldYAML `yaml:"fields"`
	} `yaml:"hierarchical"`
	VersioningSibling struct {
		Fields []reservedFieldYAML `yaml:"fields"`
	} `yaml:"versioning_sibling"`
	Governance struct {
		Fields []reservedFieldYAML `yaml:"fields"`
	} `yaml:"governance"`
	Observation struct {
		Fields []reservedFieldYAML `yaml:"fields"`
	} `yaml:"observation"`
	SchemaMetadata struct {
		Fields []reservedFieldYAML `yaml:"fields"`
	} `yaml:"schema_metadata"`
	AppendOnly struct {
		RevokeOperations []string `yaml:"revoke_operations"`
		RevokeFromRoles  []string `yaml:"revoke_from_roles"`
	} `yaml:"append_only"`
	DatabaseRoles []reservedRoleYAML `yaml:"database_roles"`
}

type reservedFieldYAML struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Nullable    bool        `yaml:"nullable"`
	Default     interface{} `yaml:"default"`
	Description string      `yaml:"description"`
	References  string      `yaml:"references"`
	Unique      bool        `yaml:"unique"`
	EnumValues  []string    `yaml:"enum_values"`
	MaxLength   int         `yaml:"max_length"`
	EnabledBy   string      `yaml:"enabled_by"`
}

// reservedRoleYAML matches model.RoleDefinition: uses Grants, no AppliesTo.
type reservedRoleYAML struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Grants      []string `yaml:"grants"`
}

// yamlFieldToModel converts a parsed YAML field definition to a model.Field.
func yamlFieldToModel(fd reservedFieldYAML, isReserved bool, isGovernance bool) model.Field {
	f := model.Field{
		Name:         fd.Name,
		Type:         fd.Type,
		Nullable:     fd.Nullable,
		Default:      fd.Default,
		Description:  fd.Description,
		References:   fd.References,
		Unique:       fd.Unique,
		EnumValues:   fd.EnumValues,
		IsReserved:   isReserved,
		IsGovernance: isGovernance,
	}
	// MaxLength is plain int on model.Field (0 = unset).
	if fd.MaxLength > 0 {
		f.MaxLength = fd.MaxLength
	}
	return f
}

// ParseJSONSchema reads and parses a JSON payload schema file.
// Used to validate typed payloads (cloud_resource, authority, policy, etc.).
func ParseJSONSchema(path string) (*JSONPayloadSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading JSON schema file: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing JSON schema YAML: %w", err)
	}

	schema := &JSONPayloadSchema{}

	if name, ok := raw["name"].(string); ok {
		schema.Name = name
	}
	if desc, ok := raw["description"].(string); ok {
		schema.Description = desc
	}

	if fieldsRaw, ok := raw["fields"]; ok {
		fieldsList, ok := fieldsRaw.([]interface{})
		if !ok {
			return nil, fmt.Errorf("JSON schema 'fields' is not a list")
		}

		for i, fieldRaw := range fieldsList {
			fieldMap, ok := fieldRaw.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("JSON schema field %d is not a map", i)
			}

			jpf := JSONPayloadField{}
			if name, ok := fieldMap["name"].(string); ok {
				jpf.Name = name
			}
			if ft, ok := fieldMap["type"].(string); ok {
				jpf.Type = ft
			}
			if req, ok := fieldMap["required"]; ok {
				jpf.Required = yamlBoolValue(req)
			}
			if desc, ok := fieldMap["description"].(string); ok {
				jpf.Description = desc
			}

			// Collect remaining keys as constraints.
			jpf.Constraints = make(map[string]interface{})
			for k, v := range fieldMap {
				if k == "name" || k == "type" || k == "required" || k == "description" {
					continue
				}
				jpf.Constraints[k] = v
			}

			// Validate depth: no nested lists-of-lists or maps-of-lists.
			if jpf.Type == "list" {
				if elemType, ok := jpf.Constraints["element_type"].(string); ok {
					if elemType == "list" || elemType == "map" {
						return nil, fmt.Errorf("JSON schema field %q: lists of %s are forbidden (factor into separate entity)", jpf.Name, elemType)
					}
				}
			}
			if jpf.Type == "map" {
				if valType, ok := jpf.Constraints["value_type"].(string); ok {
					if valType == "list" || valType == "map" {
						return nil, fmt.Errorf("JSON schema field %q: maps of %s are forbidden (factor into separate entity)", jpf.Name, valType)
					}
				}
			}

			schema.Fields = append(schema.Fields, jpf)
		}
	}

	return schema, nil
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
	Type        string
	Required    bool
	Description string
	Constraints map[string]interface{}
}

// --- YAML helpers ---

// yamlBool reads a boolean from a raw YAML map with false as default.
func yamlBool(m map[string]interface{}, key string) bool {
	val, ok := m[key]
	if !ok {
		return false
	}
	return yamlBoolValue(val)
}

// yamlBoolValue converts a YAML value to bool.
func yamlBoolValue(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(v)
		return lower == "true" || lower == "yes"
	default:
		return false
	}
}

// yamlInt converts a YAML numeric value to int.
func yamlInt(val interface{}) (int, error) {
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", val)
	}
}

// yamlFloat64Ptr converts a YAML numeric value to *float64.
// Returns nil if the value cannot be converted.
func yamlFloat64Ptr(val interface{}) *float64 {
	switch v := val.(type) {
	case float64:
		return &v
	case float32:
		f := float64(v)
		return &f
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case int32:
		f := float64(v)
		return &f
	default:
		return nil
	}
}

// yamlStringSlice converts a YAML value to []string.
func yamlStringSlice(val interface{}) ([]string, error) {
	if val == nil {
		return nil, fmt.Errorf("value is nil")
	}

	switch v := val.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("element %d is %T, not string", i, item)
			}
			result = append(result, s)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("expected list, got %T", val)
	}
}
