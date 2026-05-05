

// === internal/conventions/reserved.go ===
package conventions

import "github.com/ghowland/opsdb/internal/model"

// LoadReserved reads and parses conventions/reserved.yaml into ReservedConfig.
func LoadReserved(path string) (*model.ReservedConfig, error) {
	// TODO: read YAML file
	// TODO: parse universal, soft_delete, hierarchical, versioning_sibling sections
	// TODO: parse governance, observation, schema_metadata sections
	// TODO: parse naming_conventions, append_only, database_roles sections
	// TODO: return structured ReservedConfig
	return nil, nil
}

// IsReservedFieldName checks if a field name is reserved.
// Accounts for entity-name-templated names like parent_{entity}_id.
func IsReservedFieldName(name string, entityName string) bool {
	// TODO: check against static reserved names: id, created_time, updated_time, is_active
	// TODO: check against templated names: parent_{entityName}_id
	// TODO: check against versioning sibling names: version_serial, parent_{entityName}_version_id, etc.
	// TODO: check against all governance field names
	return false
}

// GetUniversalFields returns the fields injected into every table:
// id, created_time, updated_time.
func GetUniversalFields() []model.Field {
	// TODO: return id (int, not null, primary key), created_time (datetime, not null),
	//       updated_time (datetime, not null)
	return nil
}

// GetSoftDeleteFields returns is_active field for soft-deletable entities.
func GetSoftDeleteFields() []model.Field {
	// TODO: return is_active (boolean, not null, default true)
	return nil
}

// GetHierarchicalFields returns parent_{entityName}_id self-FK field.
func GetHierarchicalFields(entityName string) []model.Field {
	// TODO: substitute entityName into template
	// TODO: return parent_{entityName}_id (foreign_key, nullable, references entityName)
	return nil
}

// GetVersioningSiblingFields returns fields for the {entityName}_version table.
func GetVersioningSiblingFields(entityName string) []model.Field {
	// TODO: return {entityName}_id, version_serial, parent_{entityName}_version_id,
	//       change_set_id, is_active_version, approved_for_production_time
	return nil
}

// GetGovernanceFields returns governance field definitions for enabled flags.
func GetGovernanceFields(enabled map[string]bool) []model.Field {
	// TODO: for each key in enabled that is true, look up matching governance/observation/schema field
	// TODO: return matching field definitions
	return nil
}

// GetDatabaseRoles returns the database role definitions for bootstrap.
func GetDatabaseRoles() []model.RoleDefinition {
	// TODO: return opsdb_app_role, opsdb_admin_role, opsdb_readonly_role, opsdb_runner_role
	//       with their grant lists
	return nil
}
