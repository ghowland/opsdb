//# tools/opsdb-api/gate/step_versioning.go

package gate

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/pg"
)

// stepVersioningPrepare is gate step 6: Versioning Preparation.
// Prepares version sibling row data for change-managed entities.
// Only runs for write operations against versioned entities.
// This step never rejects — it only prepares data for step 9.
func stepVersioningPrepare(ctx *GateContext) {
	if !isWriteOperation(ctx.Request.OperationClass) {
		return
	}

	// check if target entity is versioned
	entityMeta, found := ctx.Schema.GetEntityType(ctx.Request.TargetEntity)
	if !found || !entityMeta.Versioned {
		return
	}

	// for creates (no target ID yet), prepare initial version
	if ctx.Request.TargetEntityID == 0 {
		ctx.VersionInfo = &VersionPrepResult{
			NextSerial: 1,
			ParentVID:  0,
		}
		return
	}

	// read current active version for existing entity
	currentSerial, currentVID, err := readActiveVersion(
		ctx.DB, ctx.Request.TargetEntity, ctx.Request.TargetEntityID)
	if err != nil {
		// if we can't read version info, log a warning but don't reject —
		// the entity may not have a version row yet (first change-managed
		// write after dev-to-operational cutover)
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
// row ID for an entity from its versioning sibling table.
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
		if pg.IsNoRows(err) {
			return 0, 0, nil
		}
		if pg.IsUndefinedTable(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("version query failed for %s id=%d: %w",
			entityType, entityID, err)
	}

	return serial, versionID, nil
}
