// === opsdb-api/concurrency/optimistic.go ===
package concurrency

// ValidateVersionStamps checks each field change's version stamp against
// the current version of the target entity. Returns stale_version error
// with details of which entities are stale.
func ValidateVersionStamps(fieldChanges []FieldChangeStamp, db interface{}) error {
	// TODO: for each unique (entity_type, entity_id) in fieldChanges:
	//   read current version_serial from {entity_type}_version
	//     WHERE {entity_type}_id = entity_id AND is_active_version = true
	//   compare against field change's VersionStamp
	//   if current > drafted-against: add to stale list
	//
	// TODO: if stale list non-empty:
	//   return StaleVersionError with list of (entity_type, entity_id,
	//     drafted_version, current_version) for each stale entity
	//
	// TODO: return nil if all stamps current
	return nil
}

// FieldChangeStamp holds the minimum info needed for version stamp validation.
type FieldChangeStamp struct {
	EntityType   string
	EntityID     int
	VersionStamp int // version_serial this change was drafted against
}

// StaleVersionError indicates one or more entities have advanced since drafting.
type StaleVersionError struct {
	StaleEntities []StaleEntity
}

func (e *StaleVersionError) Error() string {
	// TODO: format message listing stale entities
	return "stale_version"
}

// StaleEntity records one entity that is stale.
type StaleEntity struct {
	EntityType     string
	EntityID       int
	DraftedVersion int
	CurrentVersion int
}

