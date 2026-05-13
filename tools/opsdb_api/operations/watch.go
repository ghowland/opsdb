//# tools/opsdb-api/operations/watch.go

package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/tools/opsdb-api/gate"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb-api/schema"
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// WatchParams holds watch subscription parameters.
type WatchParams struct {
	EntityType   string            `json:"entity_type"`
	Filters      []FilterPredicate `json:"filters"`
	ResumeToken  string            `json:"resume_token"`
	PollInterval int               `json:"poll_interval_seconds"`
	MaxIdleTime  int               `json:"max_idle_time_seconds"`
}

// WatchEvent represents one change event in a watch stream.
type WatchEvent struct {
	Type        string                 `json:"type"`
	EntityType  string                 `json:"entity_type"`
	EntityID    int                    `json:"entity_id"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Version     int                    `json:"version,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	ResumeToken string                 `json:"resume_token"`
}

// resumeState tracks the position in the change stream for reconnection.
type resumeState struct {
	EntityType      string
	LastUpdatedTime time.Time
	LastSeenID      int
	KnownEntityIDs  map[int]bool
}

// decodedToken holds parsed resume token data.
type decodedToken struct {
	EntityType      string
	LastUpdatedTime time.Time
	LastSeenID      int
}

const (
	defaultPollInterval = 5 * time.Second
	defaultMaxIdleTime  = 30 * time.Minute
	maxPollInterval     = 60 * time.Second
)

// ---------------------------------------------------------------------------
// HTTP handler
// ---------------------------------------------------------------------------

// Watch handles POST /api/v1/watch
// After gate validates auth/authz, sets SSE headers and streams change
// events until the client disconnects or max idle time is exceeded.
func (h *Handlers) Watch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var params WatchParams
	if err := parseJSONBody(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "watch",
		OperationClass: "stream",
		TargetEntity:   params.EntityType,
		Params: map[string]interface{}{
			"entity_type":  params.EntityType,
			"filters":      params.Filters,
			"resume_token": params.ResumeToken,
		},
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	if !resp.Success {
		writeGateResponse(w, resp)
		return
	}

	// verify the response writer supports flushing for SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": "streaming not supported",
		})
		return
	}

	// set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Audit-Entry-ID", fmt.Sprintf("%d", resp.AuditEntryID))
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// callback that writes each event as an SSE data line
	callback := func(event WatchEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
		flusher.Flush()
	}

	// run the streaming loop — blocks until context cancels or idle timeout
	err := watchStream(r.Context(), h.db, h.schema, &params, callback)
	if err != nil {
		// write a final error event before closing
		errEvent, _ := json.Marshal(WatchEvent{
			Type:       "ERROR",
			EntityType: params.EntityType,
			Data:       map[string]interface{}{"error": err.Error()},
			Timestamp:  time.Now().UTC(),
		})
		fmt.Fprintf(w, "event: ERROR\ndata: %s\n\n", errEvent)
		flusher.Flush()
	}
}

// ---------------------------------------------------------------------------
// Domain functions
// ---------------------------------------------------------------------------

// watchStream implements the streaming watch operation. Sends an initial
// snapshot of matching entities, then streams changes as they occur. On
// reconnect with a resume token, sends SYNC events with current state to
// ensure the client hasn't missed any changes (level-triggered backstop).
func watchStream(ctx context.Context, db *pg.DB, schema *runtimeschema.RuntimeSchema, params *WatchParams, callback func(event WatchEvent)) error {
	_, found := schema.GetEntityType(params.EntityType)
	if !found {
		return fmt.Errorf("unknown entity type: %s", params.EntityType)
	}

	pollInterval := time.Duration(params.PollInterval) * time.Second
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	if pollInterval > maxPollInterval {
		pollInterval = maxPollInterval
	}

	maxIdle := time.Duration(params.MaxIdleTime) * time.Second
	if maxIdle <= 0 {
		maxIdle = defaultMaxIdleTime
	}

	state, err := initializeResumeState(params)
	if err != nil {
		return fmt.Errorf("failed to initialize watch state: %w", err)
	}

	if params.ResumeToken != "" {
		err = sendSyncEvents(db, params, state, callback)
	} else {
		err = sendSnapshotEvents(db, params, state, callback)
	}
	if err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	return pollForChanges(ctx, db, params, state, pollInterval, maxIdle, callback)
}

// initializeResumeState sets up tracking state either from scratch or
// from a resume token.
func initializeResumeState(params *WatchParams) (*resumeState, error) {
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

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// buildWatchWhereClause constructs the WHERE clause for change detection,
// combining user filters with the resume position.
func buildWatchWhereClause(params *WatchParams, state *resumeState) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	// resume position: find rows updated after our last known time,
	// or updated at the same time with a higher ID (deterministic ordering)
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

// ---------------------------------------------------------------------------
// Resume token encoding
// ---------------------------------------------------------------------------

// encodeResumeToken creates an opaque token encoding the current watch position.
func encodeResumeToken(state *resumeState) string {
	return fmt.Sprintf("%s:%d:%d",
		state.EntityType,
		state.LastUpdatedTime.UnixMicro(),
		state.LastSeenID,
	)
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

// ---------------------------------------------------------------------------
// Field extraction helpers
// ---------------------------------------------------------------------------

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
