package loader

import (
	"github.com/ghowland/opsdb/internal/conventions"
	"github.com/ghowland/opsdb/internal/model"
)

// Inject adds reserved fields, governance fields, and versioning sibling
// entity definitions to the schema based on entity flags. Runs after
// validation and resolution — entities are known-good at this point.
func Inject(schema *model.Schema, reserved *ReservedConfig) error {
	// Snapshot entity names to avoid mutation during iteration.
	entityNames := make([]string, 0, len(schema.Entities))
	for name := range schema.Entities {
		entityNames = append(entityNames, name)
	}

	// First pass: inject reserved fields into all entities.
	for _, name := range entityNames {
		entity := schema.Entities[name]

		injectUniversalFields(entity)

		if entity.SoftDelete {
			injectSoftDelete(entity)
		}

		if entity.Hierarchical {
			injectHierarchical(entity)
		}

		if len(entity.Governance) > 0 {
			injectGovernanceFields(entity, entity.Governance, reserved)
		}
	}

	// Second pass: generate versioning siblings after all entities have
	// their fields injected (siblings copy parent fields).
	var siblingNames []string
	for _, name := range entityNames {
		entity := schema.Entities[name]
		if !entity.Versioned {
			continue
		}

		sibling := generateVersioningSibling(entity)
		schema.Entities[sibling.Name] = sibling
		siblingNames = append(siblingNames, sibling.Name)

		// Add FK relationships for the sibling.
		// Sibling -> parent entity.
		schema.Relationships = append(schema.Relationships, model.Relationship{
			SourceEntity:      sibling.Name,
			SourceField:       entity.Name + "_id",
			TargetEntity:      entity.Name,
			Cardinality:       "many_to_one",
			OnDeleteAction:    "restrict",
			IsSelfReferential: false,
		})

		// Sibling -> self (version chain).
		schema.Relationships = append(schema.Relationships, model.Relationship{
			SourceEntity:      sibling.Name,
			SourceField:       "parent_" + sibling.Name + "_id",
			TargetEntity:      sibling.Name,
			Cardinality:       "many_to_one",
			OnDeleteAction:    "restrict",
			IsSelfReferential: true,
		})

		// Sibling -> change_set.
		schema.Relationships = append(schema.Relationships, model.Relationship{
			SourceEntity:      sibling.Name,
			SourceField:       "change_set_id",
			TargetEntity:      "change_set",
			Cardinality:       "many_to_one",
			OnDeleteAction:    "restrict",
			IsSelfReferential: false,
		})
	}

	// Insert sibling names into LoadOrder immediately after their parent entities.
	if len(siblingNames) > 0 {
		siblingParent := make(map[string]string) // sibling name -> parent name
		for _, name := range entityNames {
			entity := schema.Entities[name]
			if entity.Versioned {
				siblingParent[entity.Name+"_version"] = entity.Name
			}
		}

		var newLoadOrder []string
		for _, name := range schema.LoadOrder {
			newLoadOrder = append(newLoadOrder, name)
			sibName := name + "_version"
			if _, isSiblingParent := siblingParent[sibName]; isSiblingParent {
				newLoadOrder = append(newLoadOrder, sibName)
			}
		}

		// Append any siblings whose parents weren't in LoadOrder (shouldn't happen, safety).
		inOrder := make(map[string]bool)
		for _, name := range newLoadOrder {
			inOrder[name] = true
		}
		for _, sn := range siblingNames {
			if !inOrder[sn] {
				newLoadOrder = append(newLoadOrder, sn)
			}
		}

		schema.LoadOrder = newLoadOrder
	}

	return nil
}

// injectUniversalFields prepends id, created_time, updated_time to an entity.
// These fields are present on every table per the schema conventions.
func injectUniversalFields(entity *model.Entity) {
	universals := conventions.GetUniversalFields()

	// Prepend: universal fields appear first in the table definition.
	entity.Fields = append(universals, entity.Fields...)
}

// injectSoftDelete appends the is_active boolean field for soft-delete entities.
func injectSoftDelete(entity *model.Entity) {
	softDeleteFields := conventions.GetSoftDeleteFields()
	entity.Fields = append(entity.Fields, softDeleteFields...)
}

// injectHierarchical appends parent_{entity_name}_id self-referential FK.
func injectHierarchical(entity *model.Entity) {
	hierarchicalFields := conventions.GetHierarchicalFields(entity.Name)
	entity.Fields = append(entity.Fields, hierarchicalFields...)
}

// injectGovernanceFields appends enabled governance fields with underscore prefix.
func injectGovernanceFields(entity *model.Entity, enabled map[string]bool, reserved *ReservedConfig) {
	for flagName, isEnabled := range enabled {
		if !isEnabled {
			continue
		}

		// Look up the field definition from the reserved config.
		fieldDef, ok := reserved.GovernanceFields[flagName]
		if !ok {
			// Fall back to conventions package hardcoded defaults.
			govFields := conventions.GetGovernanceFields(map[string]bool{flagName: true})
			for _, gf := range govFields {
				entity.Fields = append(entity.Fields, gf)
			}
			continue
		}

		// Ensure reserved and governance flags are set.
		fieldDef.IsReserved = true
		fieldDef.IsGovernance = true
		entity.Fields = append(entity.Fields, fieldDef)
	}
}

// generateVersioningSibling creates the {entity_name}_version entity
// definition for a versioned entity. The sibling holds the full version
// history with per-version state snapshots.
func generateVersioningSibling(entity *model.Entity) *model.Entity {
	siblingName := entity.Name + "_version"

	sibling := &model.Entity{
		Name:        siblingName,
		Description: "Version history for " + entity.Name,
		Category:    entity.Category,
		Versioned:   false,  // no recursive versioning
		SoftDelete:  false,  // versions are permanent
		Hierarchical: false,
		AppendOnly:  false,
		Governance:  nil,
	}

	// Versioning structural fields (after universal fields which will be
	// injected by the caller in the first pass — but siblings are created
	// in the second pass, so we inject universals here directly).
	sibling.Fields = append(sibling.Fields, conventions.GetUniversalFields()...)

	// FK to parent entity.
	sibling.Fields = append(sibling.Fields, model.Field{
		Name:        entity.Name + "_id",
		Type:        "foreign_key",
		Nullable:    false,
		References:  entity.Name,
		IsReserved:  true,
		Description: "FK to parent entity",
	})

	// Version serial.
	minOne := 1
	sibling.Fields = append(sibling.Fields, model.Field{
		Name:        "version_serial",
		Type:        "int",
		Nullable:    false,
		MinValue:    &minOne,
		IsReserved:  true,
		Description: "monotonic version number per entity instance",
	})

	// Prior version chain (self-referential FK).
	sibling.Fields = append(sibling.Fields, model.Field{
		Name:        "parent_" + siblingName + "_id",
		Type:        "foreign_key",
		Nullable:    true,
		References:  siblingName,
		IsReserved:  true,
		Description: "prior version in chain (null for first version)",
	})

	// Change set FK.
	sibling.Fields = append(sibling.Fields, model.Field{
		Name:        "change_set_id",
		Type:        "foreign_key",
		Nullable:    true,
		References:  "change_set",
		IsReserved:  true,
		Description: "change set that produced this version",
	})

	// Active version flag.
	sibling.Fields = append(sibling.Fields, model.Field{
		Name:        "is_active_version",
		Type:        "boolean",
		Nullable:    false,
		Default:     false,
		IsReserved:  true,
		Description: "true for current active version only",
	})

	// Approved for production time.
	sibling.Fields = append(sibling.Fields, model.Field{
		Name:        "approved_for_production_time",
		Type:        "datetime",
		Nullable:    true,
		IsReserved:  true,
		Description: "when this version went live",
	})

	// Copy ALL fields from parent entity as snapshot fields.
	// Skip reserved universal fields (id, created_time, updated_time) —
	// the sibling has its own. Copy everything else as nullable
	// (historical values may be null for fields added after initial creation).
	universalNames := map[string]bool{
		"id":           true,
		"created_time": true,
		"updated_time": true,
	}
	for _, parentField := range entity.Fields {
		if universalNames[parentField.Name] {
			continue
		}
		// Skip fields that are already on the sibling (versioning structural fields).
		if parentField.Name == entity.Name+"_id" ||
			parentField.Name == "version_serial" ||
			parentField.Name == "parent_"+siblingName+"_id" ||
			parentField.Name == "change_set_id" ||
			parentField.Name == "is_active_version" ||
			parentField.Name == "approved_for_production_time" {
			continue
		}

		snapshotField := parentField
		snapshotField.Nullable = true    // historical snapshots may have nulls
		snapshotField.IsReserved = false // these are data fields, not reserved
		snapshotField.Default = nil      // no defaults on snapshot fields
		snapshotField.Unique = false     // uniqueness not enforced on snapshots

		// Clear composite uniqueness — doesn't apply to version snapshots.
		snapshotField.MustBeUniqueWithin = nil

		sibling.Fields = append(sibling.Fields, snapshotField)
	}

	// Indexes for the sibling.
	sibling.Indexes = []model.Index{
		{
			Name:   "uq_" + siblingName + "_entity_serial",
			Fields: []string{entity.Name + "_id", "version_serial"},
			Unique: true,
		},
		{
			Name:   "idx_" + siblingName + "_change_set_id",
			Fields: []string{"change_set_id"},
			Unique: false,
		},
		{
			Name:   "idx_" + siblingName + "_is_active_version",
			Fields: []string{"is_active_version"},
			Unique: false,
		},
	}

	return sibling
}
