//# tools/opsdb_api/gate/step_versioning.go

package gate

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/pg"
)

// stepVersioningPrepare is gate step 6: Versioning Preparation.
// For write operations against versioned entities, computes the next
// version_serial and reads the current active version row ID. These
// values are stored in ctx.VersionInfo for step 9 (execution) to use
// when inserting the version sibling row.
//
// This step never rejects. If versioning data can't be read (e.g., the
// version table doesn't exist yet, or the entity has no version rows),
// it warns and provides sensible defaults. The entity update in step 9
// still applies — just without a version history row until the issue
// is resolved.
//
// Skips entirely for read operations and for entities that are not
// versioned per the runtime schema.
func stepVersioningPrepare(ctx *GateContext) {
	// Reads and streams don't produce version rows
	if !isWriteOperation(ctx.Request.OperationClass) {
		return
	}

	// Check if the target entity type is versioned. Non-versioned entities
	// (observation tables, runner_job, evidence_record, audit_log_entry)
	// skip this step entirely.
	entityMeta, found := ctx.Schema.GetEntityType(ctx.Request.TargetEntity)
	if !found {
		// Unknown entity type — step 3 (schema validation) should have
		// caught this already, but if we're running with stubs, just skip.
		return
	}
	if !entityMeta.Versioned {
		return
	}

	// For creates (no target entity ID yet), this is the first version.
	// Serial starts at 1, no parent version.
	if ctx.Request.TargetEntityID == 0 {
		ctx.VersionInfo = &VersionPrepResult{
			NextSerial: 1,
			ParentVID:  0,
		}
		return
	}

	// For updates to existing entities, read the current active version
	// to determine the next serial and the parent pointer.
	currentSerial, currentVID, err := readActiveVersion(
		ctx.DB, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
	if err != nil {
		// Can't read version info. This happens when:
		// - The version sibling table doesn't exist yet (schema not fully applied)
		// - The entity has no version rows yet (first change-managed write
		//   after the dev-to-operational cutover from OPSDB-3 §10)
		// - A transient database error
		//
		// In all cases, warn and provide defaults. The entity update in
		// step 9 still works; it just won't have a version history row
		// for this change.
		warn(ctx, fmt.Sprintf("could not read version for %s id=%d: %v",
			ctx.Request.TargetEntity, ctx.Request.TargetEntityID, err))

		ctx.VersionInfo = &VersionPrepResult{
			NextSerial: 1,
			ParentVID:  0,
		}
		return
	}

	ctx.VersionInfo = &VersionPrepResult{
		NextSerial: currentSerial + 1,
		ParentVID:  currentVID,
	}
}

// readActiveVersion reads the current active version_serial and version
// row ID for an entity from its versioning sibling table. The sibling
// table follows the naming convention {entity_type}_version with a
// foreign key column {entity_type}_id.
//
// Returns (0, 0, nil) when no active version row exists — this is not
// an error, it means the entity hasn't been versioned yet. Returns
// (0, 0, nil) when the version table doesn't exist — this happens
// during bootstrap before the schema is fully applied.
func readActiveVersion(db *pg.DB, entityType string, entityID int) (int, int, error) {
	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	query := fmt.Sprintf(
		"SELECT version_serial, id FROM %s WHERE %s = $1 AND is_active_version = true LIMIT 1",
		pg.QuoteIdentifier(versionTable),
		pg.QuoteIdentifier(fkColumn),
	)

	var serial, versionID int
	err := db.QueryRow(query, entityID).Scan(&serial, &versionID)
	if err != nil {
		// No rows — entity exists but has no version history yet.
		// This is the normal case for the first change-managed write.
		if pg.IsNoRows(err) {
			return 0, 0, nil
		}

		// Undefined table — the version sibling table doesn't exist.
		// This happens during early bootstrap when the schema hasn't
		// been fully applied yet. Not an error from the caller's
		// perspective; versioning just isn't available yet.
		if pg.IsUndefinedTable(err) {
			return 0, 0, nil
		}

		// Anything else is a real error.
		return 0, 0, fmt.Errorf("version query failed for %s id=%d: %w",
			entityType, entityID, err)
	}

	return serial, versionID, nil
}
