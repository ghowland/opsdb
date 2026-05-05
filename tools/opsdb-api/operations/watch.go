//# tools/opsdb-api/operations/watch.go

package operations

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb-api/schema"
)

// WatchParams holds watch subscription parameters.
type WatchParams struct {
	EntityType    string
	Filters       []FilterPredicate
	ResumeToken   string
	PollInterval  time.Duration // default 5s
	MaxIdleTime   time.Duration // disconnect after this long with no changes, default 30m
}

// WatchEvent represents one change event in a watch stream.
type WatchEvent struct {
	Type       string                 // SNAPSHOT, ADDED, MODIFIED, DELETED, SYNC, ERROR
	EntityType string
	EntityID   int
	Data       map[string]interface{}
	Version    int
	Timestamp  time.Time
	ResumeToken string
}

// resumeState tracks the position in the change stream for reconnection.
type resumeState struct {
	EntityType       string
	LastUpdatedTime  time.Time
	LastSeenID       int
	KnownEntityIDs   map[int]bool
}

const (
	defaultPollInterval = 5 * time.Second
	defaultMaxIdleTime  = 30 * time.Minute
	maxPollInterval     = 60 * time.Second
)

// Watch implements the streaming watch operation. Sends an initial snapshot
// of matching entities, then streams changes as they occur. On reconnect
// with a resume token, sends a SYNC event with current state to ensure
// the client hasn't missed any changes (level-triggered backstop).
func Watch(ctx context.Context, db *pg.DB, schema *runtimeschema.RuntimeSchema, params *WatchParams, callback func(event WatchEvent)) error {
	_, found := schema.GetEntityType(params.EntityType)
	if !found {
		return fmt.Errorf("unknown entity type: %s", params.EntityType)
	}

	pollInterval := params.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	if pollInterval > maxPollInterval {
		pollInterval = maxPollInterval
	}

	maxIdle := params.MaxIdleTime
	if maxIdle <= 0 {
		maxIdle = defaultMaxIdleTime
	}

	// initialize or restore resume state
	state, err := initializeResumeState(db, params)
	if err != nil {
		return fmt.Errorf("failed to initialize watch state: %w", err)
	}

	// send initial data: full snapshot or sync depending on resume token
	if params.ResumeToken != "" {
		err = sendSyncEvents(db, params, state, callback)
	} else {
		err = sendSnapshotEvents(db, params, state, callback)
	}
	if err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	// enter polling loop for changes
	return pollForChanges(ctx, db, params, state, pollInterval, maxIdle, callback)
}

// initializeResumeState sets up tracking state either from scratch or
// from a resume token.
func initializeResumeState(db *pg.DB, params *WatchParams) (*resumeState, error) {
	state := &resumeState{
		EntityType:     params.EntityType,
		KnownEntityIDs: make(map[int]bool),
	}

	if params.ResumeToken != "" {
		decoded, err := decodeResumeToken(params.ResumeToken)
		if err != nil {
			// invalid token: start fresh rather than failing
			state.LastUpdatedTime = time.Time{}
			return state, nil
		}
		state.LastUpdatedTime = decoded.LastUpdatedTime
		state.LastSeenID = decoded.LastSeenID
	}

	return state, nil
}

// sendSnapshotEvents sends initial SNAPSHOT events for all matching entities.
// Establishes the baseline known set for subsequent change detection.
func sendSnapshotEvents(db *pg.DB, params *WatchParams, state *resumeState, callback func(WatchEvent)) error {
	rows, err := queryMatchingEntities(db, params)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to read columns: %w", err)
	}

	now := time.Now().UTC()

	for rows.Next() {
		fieldValues, err := scanRowToMap(rows, columns)
		if err != nil {
			return fmt.Errorf("snapshot scan failed: %w", err)
		}

		entityID := extractEntityID(fieldValues)
		state.KnownEntityIDs[entityID] = true

		updatedTime := extractUpdatedTime(fieldValues)
		if updatedTime.After(state.LastUpdatedTime) {
			state.LastUpdatedTime = updatedTime
			state.LastSeenID = entityID
		}

		callback(WatchEvent{
			Type:        "SNAPSHOT",
			EntityType:  params.EntityType,
			EntityID:    entityID,
			Data:        fieldValues,
			Timestamp:   now,
			ResumeToken: encodeResumeToken(state),
		})
	}

	return rows.Err()
}

// sendSyncEvents sends SYNC events on reconnect, comparing current state
// against the resume position. This is the level-triggered backstop:
// any changes missed while disconnected will be detected and sent.
func sendSyncEvents(db *pg.DB, params *WatchParams, state *resumeState, callback func(WatchEvent)) error {
	rows, err := queryMatchingEntities(db, params)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to read columns: %w", err)
	}

	now := time.Now().UTC()
	currentIDs := make(map[int]bool)

	for rows.Next() {
		fieldValues, err := scanRowToMap(rows, columns)
		if err != nil {
			return fmt.Errorf("sync scan failed: %w", err)
		}

		entityID := extractEntityID(fieldValues)
		currentIDs[entityID] = true

		updatedTime := extractUpdatedTime(fieldValues)
		if updatedTime.After(state.LastUpdatedTime) {
			state.LastUpdatedTime = updatedTime
			state.LastSeenID = entityID
		}

		// determine event type based on whether we knew about this entity
		eventType := "SYNC"
		if !state.KnownEntityIDs[entityID] {
			eventType = "ADDED"
		}

		callback(WatchEvent{
			Type:        eventType,
			EntityType:  params.EntityType,
			EntityID:    entityID,
			Data:        fieldValues,
			Timestamp:   now,
			ResumeToken: encodeResumeToken(state),
		})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// detect deletions: entities we knew about that are no longer present
	for knownID := range state.KnownEntityIDs {
		if !currentIDs[knownID] {
			callback(WatchEvent{
				Type:        "DELETED",
				EntityType:  params.EntityType,
				EntityID:    knownID,
				Timestamp:   now,
				ResumeToken: encodeResumeToken(state),
			})
		}
	}

	state.KnownEntityIDs = currentIDs
	return nil
}

// pollForChanges enters the polling loop, detecting and sending change
// events until the context is cancelled or max idle time is exceeded.
func pollForChanges(ctx context.Context, db *pg.DB, params *WatchParams, state *resumeState, pollInterval time.Duration, maxIdle time.Duration, callback func(WatchEvent)) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	lastEventTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			changes, err := detectChanges(db, params, state)
			if err != nil {
				callback(WatchEvent{
					Type:       "ERROR",
					EntityType: params.EntityType,
					Data:       map[string]interface{}{"error": err.Error()},
					Timestamp:  time.Now().UTC(),
				})
				continue
			}

			if len(changes) > 0 {
				lastEventTime = time.Now()
				for _, event := range changes {
					event.ResumeToken = encodeResumeToken(state)
					callback(event)
				}
			}

			// check idle timeout
			if time.Since(lastEventTime) > maxIdle {
				return nil
			}
		}
	}
}

// detectChanges queries for entities modified since the last known position.
func detectChanges(db *pg.DB, params *WatchParams, state *resumeState) ([]WatchEvent, error) {
	whereClause, args := buildWatchWhereClause(params, state)

	query := fmt.Sprintf(
		"SELECT * FROM %s %s ORDER BY updated_time ASC, id ASC",
		pg.QuoteIdentifier(params.EntityType),
		whereClause,
	)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("change detection query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	var events []WatchEvent

	for rows.Next() {
		fieldValues, err := scanRowToMap(rows, columns)
		if err != nil {
			return nil, err
		}

		entityID := extractEntityID(fieldValues)
		updatedTime := extractUpdatedTime(fieldValues)

		// determine event type
		eventType := "MODIFIED"
		if !state.KnownEntityIDs[entityID] {
			eventType = "ADDED"
			state.KnownEntityIDs[entityID] = true
		}

		// check for soft-delete
		if isActive, ok := fieldValues["is_active"]; ok {
			if active, ok := isActive.(bool); ok && !active {
				eventType = "DELETED"
				delete(state.KnownEntityIDs, entityID)
			}
		}

		if updatedTime.After(state.LastUpdatedTime) {
			state.LastUpdatedTime = updatedTime
			state.LastSeenID = entityID
		}

		events = append(events, WatchEvent{
			Type:       eventType,
			EntityType: params.EntityType,
			EntityID:   entityID,
			Data:       fieldValues,
			Timestamp:  now,
		})
	}

	return events, rows.Err()
}

// buildWatchWhereClause constructs the WHERE clause for change detection,
// combining user filters with the resume position.
func buildWatchWhereClause(params *WatchParams, state *resumeState) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	// resume position: find rows updated after our last known time,
	// or updated at the same time with a higher ID (for deterministic ordering)
	if !state.LastUpdatedTime.IsZero() {
		conditions = append(conditions,
			fmt.Sprintf("(updated_time > $%d OR (updated_time = $%d AND id > $%d))",
				argIdx, argIdx, argIdx+1))
		args = append(args, state.LastUpdatedTime, state.LastSeenID)
		argIdx += 2
	}

	// apply user filters
	for _, f := range params.Filters {
		condition, filterArgs := buildFilterCondition(params.EntityType, f, &argIdx)
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, filterArgs...)
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

// queryMatchingEntities runs a full query for all entities matching the
// watch filter. Used for initial snapshot and sync.
func queryMatchingEntities(db *pg.DB, params *WatchParams) (pg.Rows, error) {
	whereClause := ""
	var args []interface{}

	if len(params.Filters) > 0 {
		argIdx := 1
		var conditions []string
		for _, f := range params.Filters {
			condition, filterArgs := buildFilterCondition(params.EntityType, f, &argIdx)
			if condition != "" {
				conditions = append(conditions, condition)
				args = append(args, filterArgs...)
			}
		}
		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}
	}

	query := fmt.Sprintf("SELECT * FROM %s %s ORDER BY id ASC",
		pg.QuoteIdentifier(params.EntityType), whereClause)

	rows, err := db.QueryRows(query, args...)
	if err != nil {
		return nil, fmt.Errorf("watch query failed: %w", err)
	}

	return rows, nil
}

// --- resume token encoding ---

// encodeResumeToken creates an opaque token encoding the current watch position.
func encodeResumeToken(state *resumeState) string {
	return fmt.Sprintf("%s:%d:%d",
		state.EntityType,
		state.LastUpdatedTime.UnixMicro(),
		state.LastSeenID,
	)
}

// decodedToken holds parsed resume token data.
type decodedToken struct {
	EntityType      string
	LastUpdatedTime time.Time
	LastSeenID      int
}

// decodeResumeToken parses an opaque resume token back into position data.
func decodeResumeToken(token string) (*decodedToken, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid resume token format")
	}

	var unixMicro int64
	var lastID int
	_, err := fmt.Sscanf(parts[1]+":"+parts[2], "%d:%d", &unixMicro, &lastID)
	if err != nil {
		return nil, fmt.Errorf("invalid resume token values: %w", err)
	}

	return &decodedToken{
		EntityType:      parts[0],
		LastUpdatedTime: time.UnixMicro(unixMicro),
		LastSeenID:      lastID,
	}, nil
}

// --- field extraction helpers ---

func extractEntityID(fields map[string]interface{}) int {
	if id, ok := fields["id"]; ok {
		if intID, ok := toInt(id); ok {
			return intID
		}
	}
	return 0
}

func extractUpdatedTime(fields map[string]interface{}) time.Time {
	if t, ok := fields["updated_time"]; ok {
		if timeVal, ok := t.(time.Time); ok {
			return timeVal
		}
	}
	return time.Time{}
}
