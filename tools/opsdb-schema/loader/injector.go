//# tools/opsdb-schema/loader/injector.go

go
package loader

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/model"
)

// Inject adds reserved fields, governance fields, and versioning sibling
// entity definitions to the schema based on entity flags. Runs after
// validation and resolution — entities are known-good at this point.
func Inject(schema *model.Schema, reserved *ReservedConfig) error {
	// TODO: collect entities to process (snapshot keys to avoid mutation during iteration)
	// TODO: for each entity in schema.Entities:
	//   injectUniversalFields(entity)
	//   if entity.SoftDelete: injectSoftDelete(entity)
	//   if entity.Hierarchical: injectHierarchical(entity)
	//   if entity.Governance has enabled flags: injectGovernanceFields(entity, entity.Governance, reserved)
	//   if entity.Versioned: generate and add versioning sibling

	// TODO: generate versioning siblings AFTER all entities processed:
	//   for each entity where entity.Versioned == true:
	//     sibling := generateVersioningSibling(entity)
	//     schema.Entities[sibling.Name] = sibling
	//     add sibling to schema.LoadOrder (after parent entity)
	//     add FK relationship from sibling to parent entity
	//     add self-referential FK for parent_version chain

	return fmt.Errorf("not implemented")
}

// injectUniversalFields adds id, created_time, updated_time to an entity.
// These fields are present on every table per the schema conventions.
func injectUniversalFields(entity *model.Entity) {
	// TODO: prepend to entity.Fields:
	//   {Name: "id", Type: "int", Nullable: false, IsReserved: true, Description: "primary key auto-increment"}
	//   {Name: "created_time", Type: "datetime", Nullable: false, IsReserved: true, Description: "set on insert"}
	//   {Name: "updated_time", Type: "datetime", Nullable: false, IsReserved: true, Description: "set on insert and update"}
	// NOTE: these are prepended so they appear first in the table definition
}

// injectSoftDelete adds the is_active boolean field for soft-delete entities.
func injectSoftDelete(entity *model.Entity) {
	// TODO: append to entity.Fields:
	//   {Name: "is_active", Type: "boolean", Nullable: false, Default: true, IsReserved: true, Description: "soft delete state"}
}

// injectHierarchical adds parent_{entity_name}_id self-referential FK.
func injectHierarchical(entity *model.Entity) {
	// TODO: field name = "parent_" + entity.Name + "_id"
	// TODO: append to entity.Fields:
	//   {Name: fieldName, Type: "foreign_key", Nullable: true, References: entity.Name, IsReserved: true, Description: "hierarchy traversal"}
	// NOTE: nullable because root nodes have no parent
}

// injectGovernanceFields adds enabled governance fields with underscore prefix.
func injectGovernanceFields(entity *model.Entity, enabled map[string]bool, reserved *ReservedConfig) {
	// TODO: for each governance flag in enabled:
	//   if not enabled: skip
	//   look up field definition from reserved.GovernanceFields[flag]
	//   append field to entity.Fields with IsGovernance: true
	//
	// Governance fields include (depending on what entity enables):
	//   _requires_group (varchar): group required for access beyond standard role
	//   _access_classification (enum): public/internal/confidential/restricted/regulated
	//   _audit_chain_hash (varchar): cryptographic chain hash
	//   _retention_policy_id (foreign_key -> retention_policy): override default retention
	//   _schema_version_introduced_id (foreign_key -> _schema_version): when entity/field appeared
	//   _schema_version_deprecated_id (foreign_key -> _schema_version): when deprecated
	//   _observed_time (datetime): when observation was sampled
	//   _authority_id (foreign_key -> authority): source authority of observation
	//   _puller_runner_job_id (foreign_key -> runner_job): runner job that wrote observation
}

// generateVersioningSibling creates the {entity_name}_version entity
// definition for a versioned entity. The sibling holds the full version
// history with per-version state snapshots.
func generateVersioningSibling(entity *model.Entity) *model.Entity {
	// TODO: sibling name = entity.Name + "_version"
	// TODO: sibling category = entity's category
	// TODO: sibling is NOT versioned itself (no recursive versioning)
	// TODO: sibling is NOT soft-delete (versions are permanent)
	//
	// TODO: sibling fields:
	//   id (int, PK) — injected by universal fields
	//   created_time, updated_time — injected by universal fields
	//   {entity.Name}_id (foreign_key -> entity) — link to parent entity
	//   version_serial (int, not null) — monotonic per entity, unique within entity scope
	//   parent_{entity.Name}_version_id (foreign_key -> self, nullable) — prior version chain
	//   change_set_id (foreign_key -> change_set, nullable) — change set that produced this version
	//   is_active_version (boolean, not null, default false) — true for current version only
	//   approved_for_production_time (datetime, nullable) — when this version went live
	//
	// TODO: copy ALL fields from parent entity into sibling
	//   these are the snapshot fields that record the entity's state at this version
	//   each copied field: same name, same type, but nullable=true (historical values may be null)
	//   copied fields do NOT include id, created_time, updated_time (sibling has its own)
	//
	// TODO: sibling indexes:
	//   composite unique on ({entity.Name}_id, version_serial)
	//   index on change_set_id
	//   index on is_active_version (partial: WHERE is_active_version = true)
	//
	// TODO: return sibling entity
	return nil
}


