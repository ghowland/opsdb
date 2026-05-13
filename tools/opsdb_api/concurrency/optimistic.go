//# tools/opsdb_api/concurrency/optimistic.go

package concurrency

import (
	"fmt"
	"strings"

	"github.com/ghowland/opsdb/internal/pg"
)

// FieldChangeStamp holds the minimum info needed for version stamp validation.
type FieldChangeStamp struct {
	EntityType   string
	EntityID     int
	VersionStamp int // version_serial this change was drafted against
}

// StaleEntity records one entity whose version has advanced since drafting.
type StaleEntity struct {
	EntityType     string
	EntityID       int
	DraftedVersion int
	CurrentVersion int
}

// StaleVersionError indicates one or more entities have been modified
// since the change set was drafted. The submitter must retrieve current
// values, reconcile their proposed changes, and resubmit.
type StaleVersionError struct {
	StaleEntities []StaleEntity
}

func (e *StaleVersionError) Error() string {
	if len(e.StaleEntities) == 1 {
		s := e.StaleEntities[0]
		return fmt.Sprintf("stale_version: %s id=%d drafted against version %d but current is %d",
			s.EntityType, s.EntityID, s.DraftedVersion, s.CurrentVersion)
	}

	var parts []string
	for _, s := range e.StaleEntities {
		parts = append(parts, fmt.Sprintf("%s id=%d (drafted=%d current=%d)",
			s.EntityType, s.EntityID, s.DraftedVersion, s.CurrentVersion))
	}
	return fmt.Sprintf("stale_version: %d entities have changed since drafting: %s",
		len(e.StaleEntities), strings.Join(parts, ", "))
}

// entityKey is used to deduplicate entity lookups when multiple field
// changes target the same entity.
type entityKey struct {
	EntityType string
	EntityID   int
}

// ValidateVersionStamps checks each field change's version stamp against
// the current version of the target entity. If any entity has advanced
// past the drafted-against version, returns a StaleVersionError listing
// all stale entities so the submitter can reconcile in one pass.
func ValidateVersionStamps(fieldChanges []FieldChangeStamp, db *pg.DB) error {
	if len(fieldChanges) == 0 {
		return nil
	}

	// deduplicate: multiple field changes may target the same entity,
	// and we only need to check the version once per entity. If different
	// field changes for the same entity carry different version stamps,
	// use the lowest (most conservative) — if any of them is stale,
	// the whole change set for that entity is stale.
	byEntity := make(map[entityKey]int)
	for _, fc := range fieldChanges {
		key := entityKey{EntityType: fc.EntityType, EntityID: fc.EntityID}
		if existing, ok := byEntity[key]; ok {
			if fc.VersionStamp < existing {
				byEntity[key] = fc.VersionStamp
			}
		} else {
			byEntity[key] = fc.VersionStamp
		}
	}

	var stale []StaleEntity

	for key, draftedVersion := range byEntity {
		currentVersion, err := readCurrentVersion(db, key.EntityType, key.EntityID)
		if err != nil {
			return fmt.Errorf("failed to read current version for %s id=%d: %w",
				key.EntityType, key.EntityID, err)
		}

		// version 0 means the entity is new (no version row exists yet),
		// which is valid — the entity will be created by this change set
		if currentVersion == 0 {
			continue
		}

		if currentVersion > draftedVersion {
			stale = append(stale, StaleEntity{
				EntityType:     key.EntityType,
				EntityID:       key.EntityID,
				DraftedVersion: draftedVersion,
				CurrentVersion: currentVersion,
			})
		}
	}

	if len(stale) > 0 {
		return &StaleVersionError{StaleEntities: stale}
	}

	return nil
}

// readCurrentVersion reads the active version_serial for an entity from
// its versioning sibling table. Returns 0 if no version row exists
// (new entity) or if the entity type is not versioned.
func readCurrentVersion(db *pg.DB, entityType string, entityID int) (int, error) {
	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	query := fmt.Sprintf(
		"SELECT version_serial FROM %s WHERE %s = $1 AND is_active_version = true LIMIT 1",
		pg.QuoteIdentifier(versionTable),
		pg.QuoteIdentifier(fkColumn),
	)

	var versionSerial int
	err := db.QueryRow(query, entityID).Scan(&versionSerial)
	if err != nil {
		// if the version table doesn't exist or no rows match,
		// the entity is either new or not versioned — both are valid
		if pg.IsNoRows(err) || pg.IsUndefinedTable(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("version query failed: %w", err)
	}

	return versionSerial, nil
}

// IsStaleVersionError checks whether an error is a StaleVersionError.
// Used by callers to distinguish version conflicts from other failures.
func IsStaleVersionError(err error) bool {
	_, ok := err.(*StaleVersionError)
	return ok
}

// GetStaleEntities extracts the stale entity list from a StaleVersionError.
// Returns nil if the error is not a StaleVersionError.
func GetStaleEntities(err error) []StaleEntity {
	sve, ok := err.(*StaleVersionError)
	if !ok {
		return nil
	}
	return sve.StaleEntities
}
