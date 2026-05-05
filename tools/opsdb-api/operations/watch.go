// === opsdb-api/operations/watch.go ===
package operations

// Watch implements the streaming watch operation.
// Long-poll or WebSocket subscription to entity changes.
// On reconnect: fetches current state first, then streams from resume token
// (level-triggered backstop).
func Watch(params *WatchParams, callback func(event WatchEvent)) error {
	// TODO: if params.ResumeToken provided:
	//   validate token, extract last-seen state
	//   full list of matching entities to establish current state
	//   send SYNC event with current state
	//   then stream changes from the resume point
	//
	// TODO: if no resume token:
	//   full list of matching entities
	//   send initial SNAPSHOT events for each
	//   begin streaming changes
	//
	// TODO: change detection:
	//   poll updated_time on target entity type at configured interval
	//   OR listen on Postgres NOTIFY channel (if configured)
	//   for each change: send WatchEvent to callback
	//
	// TODO: generate opaque resume token encoding current position
	// TODO: handle client disconnect gracefully
	return nil
}

// WatchParams holds watch subscription parameters.
type WatchParams struct {
	EntityType  string
	Filters     []FilterPredicate
	ResumeToken string
}

// WatchEvent represents one change event in a watch stream.
type WatchEvent struct {
	Type       string                 // ADDED, MODIFIED, DELETED, SYNC
	EntityType string
	EntityID   int
	Data       map[string]interface{}
	Version    int
	Timestamp  interface{} // time.Time
}


