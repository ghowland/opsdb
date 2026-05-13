//# internal/conventions/reserved.go

package conventions

import (
	"fmt"
	"os"
	"strings"

	"github.com/ghowland/opsdb/internal/model"
	"gopkg.in/yaml.v3"
)

// reservedFileSchema mirrors the YAML structure of conventions/reserved.yaml.
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
		Fields      []reservedFieldDef   `yaml:"fields"`
		Constraints []reservedConstraint `yaml:"constraints"`
	} `yaml:"versioning_sibling"`
	Governance struct {
		Fields []reservedGovernanceDef `yaml:"fields"`
	} `yaml:"governance"`
	Observation struct {
		Fields []reservedGovernanceDef `yaml:"fields"`
	} `yaml:"observation"`
	SchemaMetadata struct {
		Fields []reservedGovernanceDef `yaml:"fields"`
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
	MinValue    *float64    `yaml:"min_value"`
}

type reservedGovernanceDef struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Nullable    bool        `yaml:"nullable"`
	Default     interface{} `yaml:"default"`
	Description string      `yaml:"description"`
	References  string      `yaml:"references"`
	EnumValues  []string    `yaml:"enum_values"`
	MaxLength   int         `yaml:"max_length"`
	EnabledBy   string      `yaml:"enabled_by"`
}

type reservedConstraint struct {
	Type   string   `yaml:"type"`
	Fields []string `yaml:"fields"`
}

type reservedRoleDef struct {
	Name        string   `yaml:"name"`
	Grants      []string `yaml:"grants"`
	Description string   `yaml:"description"`
}

// staticReservedNames is the set of field names that are always reserved.
var staticReservedNames = map[string]bool{
	"id":           true,
	"created_time": true,
	"updated_time": true,
	"is_active":    true,
}

// versioningSiblingStaticNames is the set of field names reserved on
// version sibling tables.
var versioningSiblingStaticNames = map[string]bool{
	"version_serial":               true,
	"is_active_version":            true,
	"approved_for_production_time": true,
	"change_set_id":                true,
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

	config := &model.ReservedConfig{}

	// Universal fields: id, created_time, updated_time.
	for _, fd := range raw.Universal.Fields {
		config.Universal = append(config.Universal, toModelField(fd))
	}
	if len(config.Universal) == 0 {
		config.Universal = hardcodedUniversalFields()
	}

	// Soft delete fields.
	for _, fd := range raw.SoftDelete.Fields {
		config.SoftDelete = append(config.SoftDelete, toModelField(fd))
	}
	if len(config.SoftDelete) == 0 {
		config.SoftDelete = hardcodedSoftDeleteFields()
	}

	// Hierarchical fields (templates — {entity} substituted at injection time).
	for _, fd := range raw.Hierarchical.Fields {
		config.Hierarchical = append(config.Hierarchical, toModelField(fd))
	}

	// Versioning sibling fields and constraints.
	for _, fd := range raw.VersioningSibling.Fields {
		config.VersioningSibling = append(config.VersioningSibling, toModelField(fd))
	}
	if len(config.VersioningSibling) == 0 {
		config.VersioningSibling = hardcodedVersionSiblingFields()
	}
	for _, c := range raw.VersioningSibling.Constraints {
		config.VersioningConstraints = append(config.VersioningConstraints, model.ConstraintDef{
			Type:   c.Type,
			Fields: c.Fields,
		})
	}

	// Governance fields.
	for _, gd := range raw.Governance.Fields {
		config.Governance = append(config.Governance, model.GovernanceFieldDef{
			Field:     governanceToModelField(gd),
			EnabledBy: gd.EnabledBy,
		})
	}

	// Observation fields.
	for _, gd := range raw.Observation.Fields {
		config.Observation = append(config.Observation, model.GovernanceFieldDef{
			Field:     governanceToModelField(gd),
			EnabledBy: gd.EnabledBy,
		})
	}

	// Schema metadata fields.
	for _, gd := range raw.SchemaMetadata.Fields {
		config.SchemaMetadata = append(config.SchemaMetadata, model.GovernanceFieldDef{
			Field:     governanceToModelField(gd),
			EnabledBy: gd.EnabledBy,
		})
	}

	// Ensure hardcoded governance/observation/schema metadata defaults exist.
	ensureGovernanceDefaults(config)

	// Append-only config.
	config.AppendOnly = model.AppendOnlyConfig{
		PostgresRevokeRoles: raw.AppendOnly.RevokeFromRoles,
	}
	if len(config.AppendOnly.PostgresRevokeRoles) == 0 {
		config.AppendOnly.PostgresRevokeRoles = []string{
			"opsdb_app_role", "opsdb_runner_role", "opsdb_readonly_role",
		}
	}

	// Database roles.
	for _, rd := range raw.DatabaseRoles {
		config.DatabaseRoles = append(config.DatabaseRoles, model.RoleDefinition{
			Name:        rd.Name,
			Grants:      rd.Grants,
			Description: rd.Description,
		})
	}
	if len(config.DatabaseRoles) == 0 {
		config.DatabaseRoles = hardcodedDatabaseRoles()
	}

	return config, nil
}

// IsReservedFieldName checks if a field name is reserved.
// Accounts for entity-name-templated names like parent_{entity}_id
// and versioning sibling names.
func IsReservedFieldName(name string, entityName string) bool {
	if staticReservedNames[name] {
		return true
	}

	if GovernanceFieldNames()[name] {
		return true
	}

	if versioningSiblingStaticNames[name] {
		return true
	}

	// Templated hierarchical: parent_{entityName}_id
	if name == "parent_"+entityName+"_id" {
		return true
	}

	// Templated versioning sibling FK: {entityName}_id
	if name == entityName+"_id" {
		return true
	}

	// Templated versioning sibling self-FK: parent_{entityName}_version_id
	if name == "parent_"+entityName+"_version_id" {
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
			MinValue:    float64Ptr(1),
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
func GetGovernanceFields(enabled map[string]bool) []model.Field {
	var fields []model.Field

	if enabled["_requires_group"] {
		fields = append(fields, model.Field{
			Name: "_requires_group", Type: "varchar", Nullable: true,
			MaxLength: intPtr(255), Description: "group required for access beyond standard role",
			IsReserved: true, IsGovernance: true,
		})
	}
	if enabled["_access_classification"] {
		fields = append(fields, model.Field{
			Name: "_access_classification", Type: "enum", Nullable: true,
			EnumValues:  []string{"public", "internal", "confidential", "restricted", "regulated"},
			Description: "data sensitivity level for access decisions and logging",
			IsReserved:  true, IsGovernance: true,
		})
	}
	if enabled["_audit_chain_hash"] {
		fields = append(fields, model.Field{
			Name: "_audit_chain_hash", Type: "varchar", Nullable: true,
			MaxLength: intPtr(128), Description: "cryptographic chain hash over prior entry",
			IsReserved: true, IsGovernance: true,
		})
	}
	if enabled["_retention_policy_id"] {
		fields = append(fields, model.Field{
			Name: "_retention_policy_id", Type: "foreign_key", Nullable: true,
			References: "retention_policy", Description: "override of default retention policy",
			IsReserved: true, IsGovernance: true,
		})
	}
	if enabled["_schema_version_introduced_id"] {
		fields = append(fields, model.Field{
			Name: "_schema_version_introduced_id", Type: "foreign_key", Nullable: true,
			References: "_schema_version", Description: "schema version that introduced this entity or field",
			IsReserved: true, IsGovernance: true,
		})
	}
	if enabled["_schema_version_deprecated_id"] {
		fields = append(fields, model.Field{
			Name: "_schema_version_deprecated_id", Type: "foreign_key", Nullable: true,
			References: "_schema_version", Description: "schema version that deprecated this entity or field",
			IsReserved: true, IsGovernance: true,
		})
	}
	if enabled["_observed_time"] {
		fields = append(fields, model.Field{
			Name: "_observed_time", Type: "datetime", Nullable: true,
			Description: "when observation was sampled from authority",
			IsReserved:  true, IsGovernance: true,
		})
	}
	if enabled["_authority_id"] {
		fields = append(fields, model.Field{
			Name: "_authority_id", Type: "foreign_key", Nullable: true,
			References: "authority", Description: "source authority of observation",
			IsReserved: true, IsGovernance: true,
		})
	}
	if enabled["_puller_runner_job_id"] {
		fields = append(fields, model.Field{
			Name: "_puller_runner_job_id", Type: "foreign_key", Nullable: true,
			References: "runner_job", Description: "runner job that wrote this observation",
			IsReserved: true, IsGovernance: true,
		})
	}

	return fields
}

// GetDatabaseRoles returns the database role definitions for DDL generation.
func GetDatabaseRoles() []model.RoleDefinition {
	return hardcodedDatabaseRoles()
}

// --- hardcoded defaults ---

func hardcodedUniversalFields() []model.Field {
	return []model.Field{
		{
			Name: "id", Type: "int", Nullable: false,
			Description: "primary key auto-increment", IsReserved: true,
		},
		{
			Name: "created_time", Type: "datetime", Nullable: false,
			Description: "set on insert", IsReserved: true,
		},
		{
			Name: "updated_time", Type: "datetime", Nullable: false,
			Description: "set on insert and update", IsReserved: true,
		},
	}
}

func hardcodedSoftDeleteFields() []model.Field {
	return []model.Field{
		{
			Name: "is_active", Type: "boolean", Nullable: false,
			Default: true, Description: "soft delete state; false means logically deleted",
			IsReserved: true,
		},
	}
}

func hardcodedVersionSiblingFields() []model.Field {
	return []model.Field{
		{
			Name: "version_serial", Type: "int", Nullable: false,
			Description: "monotonic version number per entity instance",
			MinValue:    float64Ptr(1), IsReserved: true,
		},
		{
			Name: "is_active_version", Type: "boolean", Nullable: false,
			Default: false, Description: "true for current active version only",
			IsReserved: true,
		},
		{
			Name: "approved_for_production_time", Type: "datetime", Nullable: true,
			Description: "when this version went live", IsReserved: true,
		},
	}
}

func hardcodedDatabaseRoles() []model.RoleDefinition {
	return []model.RoleDefinition{
		{
			Name:        "opsdb_app_role",
			Grants:      []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
			Description: "application role for API — full CRUD on non-append-only tables",
		},
		{
			Name:        "opsdb_admin_role",
			Grants:      []string{"ALL"},
			Description: "admin role for substrate operators — DDL and data, under SoD",
		},
		{
			Name:        "opsdb_readonly_role",
			Grants:      []string{"SELECT"},
			Description: "read-only role for auditors and dashboards",
		},
		{
			Name:        "opsdb_runner_role",
			Grants:      []string{"SELECT", "INSERT", "UPDATE"},
			Description: "runner role — read and write but not delete (soft-delete via UPDATE)",
		},
	}
}

// ensureGovernanceDefaults fills in governance/observation/schema metadata
// definitions that were not present in the YAML file.
func ensureGovernanceDefaults(config *model.ReservedConfig) {
	governanceDefaults := []model.GovernanceFieldDef{
		{EnabledBy: "_requires_group", Field: model.Field{
			Name: "_requires_group", Type: "varchar", Nullable: true,
			MaxLength: intPtr(255), Description: "group required for access beyond standard role",
			IsReserved: true, IsGovernance: true,
		}},
		{EnabledBy: "_access_classification", Field: model.Field{
			Name: "_access_classification", Type: "enum", Nullable: true,
			EnumValues:  []string{"public", "internal", "confidential", "restricted", "regulated"},
			Description: "data sensitivity level", IsReserved: true, IsGovernance: true,
		}},
		{EnabledBy: "_audit_chain_hash", Field: model.Field{
			Name: "_audit_chain_hash", Type: "varchar", Nullable: true,
			MaxLength: intPtr(128), Description: "cryptographic chain hash",
			IsReserved: true, IsGovernance: true,
		}},
		{EnabledBy: "_retention_policy_id", Field: model.Field{
			Name: "_retention_policy_id", Type: "foreign_key", Nullable: true,
			References: "retention_policy", Description: "retention policy override",
			IsReserved: true, IsGovernance: true,
		}},
	}
	observationDefaults := []model.GovernanceFieldDef{
		{EnabledBy: "_observed_time", Field: model.Field{
			Name: "_observed_time", Type: "datetime", Nullable: true,
			Description: "when observation was sampled", IsReserved: true, IsGovernance: true,
		}},
		{EnabledBy: "_authority_id", Field: model.Field{
			Name: "_authority_id", Type: "foreign_key", Nullable: true,
			References: "authority", Description: "source authority of observation",
			IsReserved: true, IsGovernance: true,
		}},
		{EnabledBy: "_puller_runner_job_id", Field: model.Field{
			Name: "_puller_runner_job_id", Type: "foreign_key", Nullable: true,
			References: "runner_job", Description: "runner job that wrote observation",
			IsReserved: true, IsGovernance: true,
		}},
	}
	schemaDefaults := []model.GovernanceFieldDef{
		{EnabledBy: "_schema_version_introduced_id", Field: model.Field{
			Name: "_schema_version_introduced_id", Type: "foreign_key", Nullable: true,
			References: "_schema_version", Description: "schema version introduced",
			IsReserved: true, IsGovernance: true,
		}},
		{EnabledBy: "_schema_version_deprecated_id", Field: model.Field{
			Name: "_schema_version_deprecated_id", Type: "foreign_key", Nullable: true,
			References: "_schema_version", Description: "schema version deprecated",
			IsReserved: true, IsGovernance: true,
		}},
	}

	config.Governance = mergeGovernanceDefs(config.Governance, governanceDefaults)
	config.Observation = mergeGovernanceDefs(config.Observation, observationDefaults)
	config.SchemaMetadata = mergeGovernanceDefs(config.SchemaMetadata, schemaDefaults)
}

// mergeGovernanceDefs adds defaults that aren't already present by EnabledBy key.
func mergeGovernanceDefs(existing []model.GovernanceFieldDef, defaults []model.GovernanceFieldDef) []model.GovernanceFieldDef {
	have := make(map[string]bool, len(existing))
	for _, gd := range existing {
		have[gd.EnabledBy] = true
	}
	for _, gd := range defaults {
		if !have[gd.EnabledBy] {
			existing = append(existing, gd)
		}
	}
	return existing
}

// --- conversion helpers ---

// toModelField converts a parsed YAML field definition to a model.Field.
func toModelField(fd reservedFieldDef) model.Field {
	f := model.Field{
		Name:        fd.Name,
		Type:        fd.Type,
		Nullable:    fd.Nullable,
		Default:     fd.Default,
		Description: fd.Description,
		References:  fd.References,
		Unique:      fd.Unique,
		EnumValues:  fd.EnumValues,
		IsReserved:  true,
		MinValue:    fd.MinValue,
	}
	if fd.MaxLength > 0 {
		f.MaxLength = intPtr(fd.MaxLength)
	}
	return f
}

// governanceToModelField converts a governance YAML definition to a model.Field.
func governanceToModelField(gd reservedGovernanceDef) model.Field {
	f := model.Field{
		Name:         gd.Name,
		Type:         gd.Type,
		Nullable:     gd.Nullable,
		Default:      gd.Default,
		Description:  gd.Description,
		References:   gd.References,
		EnumValues:   gd.EnumValues,
		IsReserved:   true,
		IsGovernance: true,
	}
	if gd.MaxLength > 0 {
		f.MaxLength = intPtr(gd.MaxLength)
	}
	return f
}

// intPtr returns a pointer to an int value.
func intPtr(v int) *int {
	return &v
}

// float64Ptr returns a pointer to a float64 value.
func float64Ptr(v float64) *float64 {
	return &v
}

// Ensure the import is used.
var _ = strings.HasPrefix
