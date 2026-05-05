package conventions

import (
	"fmt"
	"os"
	"strings"

	"github.com/ghowland/opsdb/internal/model"
	"gopkg.in/yaml.v3"
)

// reservedFileSchema mirrors the YAML structure of conventions/reserved.yaml
// for parsing. Internal to this file — callers use the model types.
type reservedFileSchema struct {
	Universal struct {
		Fields []reservedFieldDef `yaml:"fields"`
	} `yaml:"universal"`
	SoftDelete struct {
		Fields []reservedFieldDef `yaml:"fields"`
	} `yaml:"soft_delete"`
	Hierarchical struct {
		Fields []reservedFieldDef `yaml:"fields"`
	} `yaml:"hierarchical"`
	VersioningSibling struct {
		Fields []reservedFieldDef `yaml:"fields"`
	} `yaml:"versioning_sibling"`
	Governance struct {
		Fields []reservedFieldDef `yaml:"fields"`
	} `yaml:"governance"`
	Observation struct {
		Fields []reservedFieldDef `yaml:"fields"`
	} `yaml:"observation"`
	SchemaMetadata struct {
		Fields []reservedFieldDef `yaml:"fields"`
	} `yaml:"schema_metadata"`
	AppendOnly struct {
		RevokeOperations []string `yaml:"revoke_operations"`
		RevokeFromRoles  []string `yaml:"revoke_from_roles"`
	} `yaml:"append_only"`
	DatabaseRoles []reservedRoleDef `yaml:"database_roles"`
}

type reservedFieldDef struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Nullable    bool        `yaml:"nullable"`
	Default     interface{} `yaml:"default"`
	Description string      `yaml:"description"`
	References  string      `yaml:"references"`
	Unique      bool        `yaml:"unique"`
	EnumValues  []string    `yaml:"enum_values"`
	MaxLength   int         `yaml:"max_length"`
}

type reservedRoleDef struct {
	Name        string   `yaml:"name"`
	Permissions []string `yaml:"permissions"`
	AppliesTo   string   `yaml:"applies_to"`
	Description string   `yaml:"description"`
}

// staticReservedNames is the set of field names that are always reserved,
// regardless of entity. Checked without needing to load the YAML file.
var staticReservedNames = map[string]bool{
	"id":           true,
	"created_time": true,
	"updated_time": true,
	"is_active":    true,
}

// versioningSiblingStaticNames is the set of field names reserved on
// version sibling tables (not templated — always the same name).
var versioningSiblingStaticNames = map[string]bool{
	"version_serial":              true,
	"is_active_version":           true,
	"approved_for_production_time": true,
}

// allGovernanceNames collects every governance, observation, and schema
// metadata field name. Used by IsReservedFieldName.
var allGovernanceNames = map[string]bool{
	"_requires_group":               true,
	"_access_classification":        true,
	"_audit_chain_hash":             true,
	"_retention_policy_id":          true,
	"_schema_version_introduced_id": true,
	"_schema_version_deprecated_id": true,
	"_observed_time":                true,
	"_authority_id":                 true,
	"_puller_runner_job_id":         true,
}

// LoadReserved reads and parses conventions/reserved.yaml into a ReservedConfig.
func LoadReserved(path string) (*model.ReservedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading reserved conventions file: %w", err)
	}

	var raw reservedFileSchema
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing reserved conventions YAML: %w", err)
	}

	config := &model.ReservedConfig{
		GovernanceFields: make(map[string]model.Field),
	}

	// Universal fields: id, created_time, updated_time.
	for _, fd := range raw.Universal.Fields {
		config.UniversalFields = append(config.UniversalFields, toModelField(fd, true, false))
	}
	// If YAML was empty or missing, provide hardcoded defaults.
	if len(config.UniversalFields) == 0 {
		config.UniversalFields = hardcodedUniversalFields()
	}

	// Soft delete fields.
	for _, fd := range raw.SoftDelete.Fields {
		config.SoftDeleteFields = append(config.SoftDeleteFields, toModelField(fd, true, false))
	}
	if len(config.SoftDeleteFields) == 0 {
		config.SoftDeleteFields = hardcodedSoftDeleteFields()
	}

	// Versioning sibling fields (templates — {entity} substituted at injection time).
	for _, fd := range raw.VersioningSibling.Fields {
		config.VersionSiblingFields = append(config.VersionSiblingFields, toModelField(fd, true, false))
	}
	if len(config.VersionSiblingFields) == 0 {
		config.VersionSiblingFields = hardcodedVersionSiblingFields()
	}

	// Governance fields — keyed by a logical flag name.
	for _, fd := range raw.Governance.Fields {
		config.GovernanceFields[fd.Name] = toModelField(fd, false, true)
	}
	for _, fd := range raw.Observation.Fields {
		config.GovernanceFields[fd.Name] = toModelField(fd, false, true)
	}
	for _, fd := range raw.SchemaMetadata.Fields {
		config.GovernanceFields[fd.Name] = toModelField(fd, false, true)
	}
	// Ensure hardcoded governance fields exist even if YAML was sparse.
	ensureGovernanceDefaults(config)

	// Database roles.
	for _, rd := range raw.DatabaseRoles {
		config.DatabaseRoles = append(config.DatabaseRoles, model.RoleDefinition{
			Name:        rd.Name,
			Permissions: rd.Permissions,
			AppliesTo:   rd.AppliesTo,
			Description: rd.Description,
		})
	}
	if len(config.DatabaseRoles) == 0 {
		config.DatabaseRoles = hardcodedDatabaseRoles()
	}

	// Append-only config.
	config.AppendOnlyRevokeOps = raw.AppendOnly.RevokeOperations
	config.AppendOnlyRevokeRoles = raw.AppendOnly.RevokeFromRoles
	if len(config.AppendOnlyRevokeOps) == 0 {
		config.AppendOnlyRevokeOps = []string{"UPDATE", "DELETE"}
	}
	if len(config.AppendOnlyRevokeRoles) == 0 {
		config.AppendOnlyRevokeRoles = []string{"opsdb_app_role", "opsdb_runner_role", "opsdb_readonly_role"}
	}

	return config, nil
}

// IsReservedFieldName checks if a field name is reserved.
// Accounts for entity-name-templated names like parent_{entity}_id
// and versioning sibling names like {entity}_id, parent_{entity}_version_id.
func IsReservedFieldName(name string, entityName string) bool {
	// Static universal reserved names.
	if staticReservedNames[name] {
		return true
	}

	// Governance fields.
	if allGovernanceNames[name] {
		return true
	}

	// Versioning sibling static names.
	if versioningSiblingStaticNames[name] {
		return true
	}

	// Templated hierarchical: parent_{entityName}_id
	if name == "parent_"+entityName+"_id" {
		return true
	}

	// Templated versioning sibling FK to parent: {entityName}_id
	// (on the sibling table, this is the FK back to the parent entity)
	if name == entityName+"_id" {
		return true
	}

	// Templated versioning sibling self-FK: parent_{entityName}_version_id
	if name == "parent_"+entityName+"_version_id" {
		return true
	}

	// change_set_id is reserved on version sibling tables.
	if name == "change_set_id" {
		return true
	}

	return false
}

// GetUniversalFields returns the fields injected into every table:
// id, created_time, updated_time.
func GetUniversalFields() []model.Field {
	return hardcodedUniversalFields()
}

// GetSoftDeleteFields returns is_active field for soft-deletable entities.
func GetSoftDeleteFields() []model.Field {
	return hardcodedSoftDeleteFields()
}

// GetHierarchicalFields returns parent_{entityName}_id self-FK field.
func GetHierarchicalFields(entityName string) []model.Field {
	return []model.Field{
		{
			Name:        "parent_" + entityName + "_id",
			Type:        "foreign_key",
			Nullable:    true,
			Description: "self-referential hierarchy traversal",
			References:  entityName,
			IsReserved:  true,
		},
	}
}

// GetVersioningSiblingFields returns fields for the {entityName}_version table.
// These are the structural versioning fields, not copies of the parent's data fields.
func GetVersioningSiblingFields(entityName string) []model.Field {
	return []model.Field{
		{
			Name:        entityName + "_id",
			Type:        "foreign_key",
			Nullable:    false,
			Description: "FK to parent entity",
			References:  entityName,
			IsReserved:  true,
		},
		{
			Name:        "version_serial",
			Type:        "int",
			Nullable:    false,
			Description: "monotonic version number per entity instance",
			MinValue:    intPtr(1),
			IsReserved:  true,
		},
		{
			Name:        "parent_" + entityName + "_version_id",
			Type:        "foreign_key",
			Nullable:    true,
			Description: "prior version in chain (null for first version)",
			References:  entityName + "_version",
			IsReserved:  true,
		},
		{
			Name:        "change_set_id",
			Type:        "foreign_key",
			Nullable:    true,
			Description: "change set that produced this version",
			References:  "change_set",
			IsReserved:  true,
		},
		{
			Name:        "is_active_version",
			Type:        "boolean",
			Nullable:    false,
			Description: "true for current active version only",
			Default:     false,
			IsReserved:  true,
		},
		{
			Name:        "approved_for_production_time",
			Type:        "datetime",
			Nullable:    true,
			Description: "when this version went live",
			IsReserved:  true,
		},
	}
}

// GetGovernanceFields returns governance field definitions for enabled flags.
// The enabled map keys correspond to governance field names.
func GetGovernanceFields(enabled map[string]bool) []model.Field {
	var fields []model.Field

	// Governance fields proper.
	if enabled["_requires_group"] {
		fields = append(fields, model.Field{
			Name:        "_requires_group",
			Type:        "varchar",
			Nullable:    true,
			MaxLength:   intPtr(255),
			Description: "group required for access beyond standard role",
			IsReserved:  true,
			IsGovernance: true,
		})
	}
	if enabled["_access_classification"] {
		fields = append(fields, model.Field{
			Name:        "_access_classification",
			Type:        "enum",
			Nullable:    true,
			EnumValues:  []string{"public", "internal", "confidential", "restricted", "regulated"},
			Description: "data sensitivity level for access decisions and logging",
			IsReserved:  true,
			IsGovernance: true,
		})
	}
	if enabled["_audit_chain_hash"] {
		fields = append(fields, model.Field{
			Name:        "_audit_chain_hash",
			Type:        "varchar",
			Nullable:    true,
			MaxLength:   intPtr(128),
			Description: "cryptographic chain hash over prior entry",
			IsReserved:  true,
			IsGovernance: true,
		})
	}
	if enabled["_retention_policy_id"] {
		fields = append(fields, model.Field{
			Name:        "_retention_policy_id",
			Type:        "foreign_key",
			Nullable:    true,
			References:  "retention_policy",
			Description: "override of default retention policy",
			IsReserved:  true,
			IsGovernance: true,
		})
	}

	// Schema metadata fields.
	if enabled["_schema_version_introduced_id"] {
		fields = append(fields, model.Field{
			Name:        "_schema_version_introduced_id",
			Type:        "foreign_key",
			Nullable:    true,
			References:  "_schema_version",
			Description: "schema version that introduced this entity or field",
			IsReserved:  true,
			IsGovernance: true,
		})
	}
	if enabled["_schema_version_deprecated_id"] {
		fields = append(fields, model.Field{
			Name:        "_schema_version_deprecated_id",
			Type:        "foreign_key",
			Nullable:    true,
			References:  "_schema_version",
			Description: "schema version that deprecated this entity or field",
			IsReserved:  true,
			IsGovernance: true,
		})
	}

	// Observation fields.
	if enabled["_observed_time"] {
		fields = append(fields, model.Field{
			Name:        "_observed_time",
			Type:        "datetime",
			Nullable:    true,
			Description: "when observation was sampled from authority",
			IsReserved:  true,
			IsGovernance: true,
		})
	}
	if enabled["_authority_id"] {
		fields = append(fields, model.Field{
			Name:        "_authority_id",
			Type:        "foreign_key",
			Nullable:    true,
			References:  "authority",
			Description: "source authority of observation",
			IsReserved:  true,
			IsGovernance: true,
		})
	}
	if enabled["_puller_runner_job_id"] {
		fields = append(fields, model.Field{
			Name:        "_puller_runner_job_id",
			Type:        "foreign_key",
			Nullable:    true,
			References:  "runner_job",
			Description: "runner job that wrote this observation",
			IsReserved:  true,
			IsGovernance: true,
		})
	}

	return fields
}

// GetDatabaseRoles returns the database role definitions for DDL generation.
func GetDatabaseRoles() []model.RoleDefinition {
	return hardcodedDatabaseRoles()
}

// --- hardcoded defaults (used when YAML file is empty or missing sections) ---

func hardcodedUniversalFields() []model.Field {
	return []model.Field{
		{
			Name:        "id",
			Type:        "int",
			Nullable:    false,
			Description: "primary key auto-increment",
			IsReserved:  true,
		},
		{
			Name:        "created_time",
			Type:        "datetime",
			Nullable:    false,
			Description: "set on insert",
			IsReserved:  true,
		},
		{
			Name:        "updated_time",
			Type:        "datetime",
			Nullable:    false,
			Description: "set on insert and update",
			IsReserved:  true,
		},
	}
}

func hardcodedSoftDeleteFields() []model.Field {
	return []model.Field{
		{
			Name:        "is_active",
			Type:        "boolean",
			Nullable:    false,
			Default:     true,
			Description: "soft delete state; false means logically deleted",
			IsReserved:  true,
		},
	}
}

func hardcodedVersionSiblingFields() []model.Field {
	// These are templates — {entity} is substituted at injection time.
	// Stored here as the non-templated structural fields only.
	return []model.Field{
		{
			Name:        "version_serial",
			Type:        "int",
			Nullable:    false,
			Description: "monotonic version number per entity instance",
			MinValue:    intPtr(1),
			IsReserved:  true,
		},
		{
			Name:        "is_active_version",
			Type:        "boolean",
			Nullable:    false,
			Default:     false,
			Description: "true for current active version only",
			IsReserved:  true,
		},
		{
			Name:        "approved_for_production_time",
			Type:        "datetime",
			Nullable:    true,
			Description: "when this version went live",
			IsReserved:  true,
		},
	}
}

func hardcodedDatabaseRoles() []model.RoleDefinition {
	return []model.RoleDefinition{
		{
			Name:        "opsdb_app_role",
			Permissions: []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
			AppliesTo:   "all",
			Description: "application role for API — full CRUD on non-append-only tables",
		},
		{
			Name:        "opsdb_admin_role",
			Permissions: []string{"ALL"},
			AppliesTo:   "all",
			Description: "admin role for substrate operators — DDL and data, under SoD",
		},
		{
			Name:        "opsdb_readonly_role",
			Permissions: []string{"SELECT"},
			AppliesTo:   "all",
			Description: "read-only role for auditors and dashboards",
		},
		{
			Name:        "opsdb_runner_role",
			Permissions: []string{"SELECT", "INSERT", "UPDATE"},
			AppliesTo:   "all",
			Description: "runner role — read and write but not delete (soft-delete via UPDATE)",
		},
	}
}

// ensureGovernanceDefaults fills in governance field definitions that were
// not present in the YAML file, using hardcoded values.
func ensureGovernanceDefaults(config *model.ReservedConfig) {
	defaults := map[string]model.Field{
		"_requires_group": {
			Name: "_requires_group", Type: "varchar", Nullable: true,
			MaxLength: intPtr(255), Description: "group required for access beyond standard role",
			IsReserved: true, IsGovernance: true,
		},
		"_access_classification": {
			Name: "_access_classification", Type: "enum", Nullable: true,
			EnumValues: []string{"public", "internal", "confidential", "restricted", "regulated"},
			Description: "data sensitivity level", IsReserved: true, IsGovernance: true,
		},
		"_audit_chain_hash": {
			Name: "_audit_chain_hash", Type: "varchar", Nullable: true,
			MaxLength: intPtr(128), Description: "cryptographic chain hash",
			IsReserved: true, IsGovernance: true,
		},
		"_retention_policy_id": {
			Name: "_retention_policy_id", Type: "foreign_key", Nullable: true,
			References: "retention_policy", Description: "retention policy override",
			IsReserved: true, IsGovernance: true,
		},
		"_schema_version_introduced_id": {
			Name: "_schema_version_introduced_id", Type: "foreign_key", Nullable: true,
			References: "_schema_version", Description: "schema version introduced",
			IsReserved: true, IsGovernance: true,
		},
		"_schema_version_deprecated_id": {
			Name: "_schema_version_deprecated_id", Type: "foreign_key", Nullable: true,
			References: "_schema_version", Description: "schema version deprecated",
			IsReserved: true, IsGovernance: true,
		},
		"_observed_time": {
			Name: "_observed_time", Type: "datetime", Nullable: true,
			Description: "when observation was sampled", IsReserved: true, IsGovernance: true,
		},
		"_authority_id": {
			Name: "_authority_id", Type: "foreign_key", Nullable: true,
			References: "authority", Description: "source authority of observation",
			IsReserved: true, IsGovernance: true,
		},
		"_puller_runner_job_id": {
			Name: "_puller_runner_job_id", Type: "foreign_key", Nullable: true,
			References: "runner_job", Description: "runner job that wrote observation",
			IsReserved: true, IsGovernance: true,
		},
	}
	for key, field := range defaults {
		if _, exists := config.GovernanceFields[key]; !exists {
			config.GovernanceFields[key] = field
		}
	}
}

// toModelField converts a parsed YAML field definition to a model.Field.
func toModelField(fd reservedFieldDef, isReserved bool, isGovernance bool) model.Field {
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
	if fd.MaxLength > 0 {
		f.MaxLength = intPtr(fd.MaxLength)
	}
	return f
}

// intPtr returns a pointer to an int value. Utility for optional int fields.
func intPtr(v int) *int {
	return &v
}

// Ensure the import is used.
var _ = strings.HasPrefix
