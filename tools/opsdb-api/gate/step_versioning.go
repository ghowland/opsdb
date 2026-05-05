

// === opsdb-api/gate/step_versioning.go ===
package gate

// StepVersioningPrepare is gate step 6: Versioning Preparation.
// Prepares version sibling row for change-managed entities.
// Only runs for write operations against versioned entities.
func StepVersioningPrepare(ctx *GateContext) error {
	// TODO: check if target entity is versioned (from runtime schema)
	//   if not versioned: skip (ctx.VersionInfo = nil)
	//
	// TODO: read current active version for the entity:
	//   SELECT version_serial, id FROM {entity}_version
	//   WHERE {entity}_id = target_id AND is_active_version = true
	//
	// TODO: compute next version_serial = current + 1
	// TODO: set parent version ID = current version ID
	// TODO: change_set_id will be set when change_set is created (step 9)
	//
	// TODO: store in ctx.VersionInfo for step 9 to write
	// TODO: this step never rejects; it only prepares data
	return nil
}

