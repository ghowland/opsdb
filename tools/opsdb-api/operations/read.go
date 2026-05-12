//# tools/opsdb-api/operations/read.go

package operations

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/tools/opsdb-api/auth"
	"github.com/ghowland/opsdb/tools/opsdb-api/gate"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb-api/schema"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Handlers — the struct main.go creates, every handler method hangs off it
// ---------------------------------------------------------------------------

// Handlers owns HTTP request parsing and response writing for all 18 API
// operations. Each method constructs a GateRequest and delegates to the
// gate pipeline for processing. Read operations perform the actual DB
// query after the gate validates; write operations let the gate execute.
type Handlers struct {
	db     *pg.DB
	schema *runtimeschema.RuntimeSchema
	gate   *gate.Gate
}

// NewHandlers creates the operation handlers that main.go registers as
// HTTP routes. Matches the call site in cmd/main.go.
func NewHandlers(db *pg.DB, schema *runtimeschema.RuntimeSchema, g *gate.Gate) *Handlers {
	return &Handlers{
		db:     db,
		schema: schema,
		gate:   g,
	}
}

// ---------------------------------------------------------------------------
// Shared HTTP helpers — used by every handler across all 6 files
// ---------------------------------------------------------------------------

// parseCredentials extracts authentication material from the HTTP request.
// Supports Basic auth and Bearer tokens from the Authorization header.
func parseCredentials(r *http.Request) auth.Credentials {
	var creds auth.Credentials

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return creds
	}

	if strings.HasPrefix(authHeader, "Basic ") {
		username, password, ok := r.BasicAuth()
		if ok {
			creds.BasicUser = username
			creds.BasicPassword = password
		}
	} else if strings.HasPrefix(authHeader, "Bearer ") {
		creds.BearerToken = strings.TrimPrefix(authHeader, "Bearer ")
	}

	return creds
}

// parseJSONBody decodes the request body as JSON into the target struct.
func parseJSONBody(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(v)
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// writeJSON serializes data as JSON and writes it with the given HTTP
// status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a structured error response from a GateError.
func writeError(w http.ResponseWriter, status int, gateErr *gate.GateError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error": map[string]interface{}{
			"step":    gateErr.StepName,
			"code":    gateErr.Code,
			"message": gateErr.Message,
			"detail":  gateErr.Detail,
		},
	})
}

// writeGateResponse serializes a GateResponse as the HTTP response.
// Routes to writeError on failure, writeJSON on success.
func writeGateResponse(w http.ResponseWriter, resp *gate.GateResponse) {
	if !resp.Success {
		status := mapGateCodeToHTTPStatus(resp.Error.Code)
		writeError(w, status, resp.Error)
		return
	}

	result := map[string]interface{}{
		"success":        true,
		"data":           resp.Data,
		"audit_entry_id": resp.AuditEntryID,
	}
	if len(resp.Warnings) > 0 {
		result["warnings"] = resp.Warnings
	}
	if len(resp.Metadata) > 0 {
		result["metadata"] = resp.Metadata
	}

	writeJSON(w, http.StatusOK, result)
}

// newRequestID generates a UUID v4 for request correlation.
func newRequestID() string {
	return uuid.New().String()
}

// clientIP extracts the client IP from X-Forwarded-For or RemoteAddr.
func clientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain a comma-separated list; first is the client
		parts := strings.SplitN(forwarded, ",", 2)
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// requireMethod checks the HTTP method and writes 405 if it doesn't match.
// Returns true if the method matches and the handler should continue.
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("method %s not allowed, use %s", r.Method, method),
		})
		return false
	}
	return true
}

// mapGateCodeToHTTPStatus maps gate error codes to HTTP status codes.
func mapGateCodeToHTTPStatus(code string) int {
	switch code {
	case "auth_failed", "no_credentials", "invalid_credentials":
		return http.StatusUnauthorized
	case "forbidden", "denied", "authorization_denied":
		return http.StatusForbidden
	case "not_found", "entity_not_found":
		return http.StatusNotFound
	case "validation_failed", "schema_invalid", "bound_exceeded",
		"policy_violation", "stale_version":
		return http.StatusBadRequest
	case "conflict":
		return http.StatusConflict
	case "too_many_requests":
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// ---------------------------------------------------------------------------
// Read data types
// ---------------------------------------------------------------------------

// SearchParams holds search operation parameters.
type SearchParams struct {
	EntityType   string            `json:"entity_type"`
	Filters      []FilterPredicate `json:"filters"`
	Joins        []string          `json:"joins"`
	Projection   string            `json:"projection"`
	Ordering     []OrderSpec       `json:"ordering"`
	Cursor       string            `json:"cursor"`
	Offset       int               `json:"offset"`
	Limit        int               `json:"limit"`
	MaxStaleness int               `json:"max_staleness"`
	ViewMode     string            `json:"view_mode"`
}

// FilterPredicate represents one filter condition.
type FilterPredicate struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// OrderSpec represents one ordering directive.
type OrderSpec struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// SearchResult holds search results.
type SearchResult struct {
	Rows             []map[string]interface{} `json:"rows"`
	Cursor           string                   `json:"cursor,omitempty"`
	TotalCount       int                      `json:"total_count"`
	FreshnessSummary map[string]interface{}   `json:"freshness_summary,omitempty"`
}

// EntityRow holds one entity row with field values and metadata.
type EntityRow struct {
	EntityType    string                 `json:"entity_type"`
	EntityID      int                    `json:"entity_id"`
	Fields        map[string]interface{} `json:"fields"`
	VersionSerial *int                   `json:"version_serial,omitempty"`
	CreatedTime   *time.Time             `json:"created_time,omitempty"`
	UpdatedTime   *time.Time             `json:"updated_time,omitempty"`
}

// VersionRow holds one version sibling row.
type VersionRow struct {
	VersionID                 int                    `json:"version_id"`
	VersionSerial             int                    `json:"version_serial"`
	ParentVersionID           *int                   `json:"parent_version_id,omitempty"`
	ChangeSetID               *int                   `json:"change_set_id,omitempty"`
	IsActiveVersion           bool                   `json:"is_active_version"`
	ApprovedForProductionTime *time.Time             `json:"approved_for_production_time,omitempty"`
	Fields                    map[string]interface{} `json:"fields"`
}

// DependencyNode holds one node in a dependency walk.
type DependencyNode struct {
	EntityType string                 `json:"entity_type"`
	EntityID   int                    `json:"entity_id"`
	Depth      int                    `json:"depth"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ChangeSetViewResult holds the result of a change set view query.
type ChangeSetViewResult struct {
	ChangeSetID   int                      `json:"change_set_id"`
	Status        string                   `json:"status"`
	Name          string                   `json:"name"`
	Reason        string                   `json:"reason"`
	IsEmergency   bool                     `json:"is_emergency"`
	IsBulk        bool                     `json:"is_bulk"`
	SubmittedBy   *int                     `json:"submitted_by,omitempty"`
	SubmittedTime *time.Time               `json:"submitted_time,omitempty"`
	FieldChanges  []map[string]interface{} `json:"field_changes"`
	Approvals     []map[string]interface{} `json:"approvals"`
	Rejections    []map[string]interface{} `json:"rejections"`
	Requirements  []map[string]interface{} `json:"requirements"`
}

// query bounds — enforced on every search
const (
	maxSearchLimit        = 10000
	defaultSearchLimit    = 100
	maxJoinDepth          = 5
	maxPredicateCount     = 50
	maxQueryTimeoutMillis = 30000
)

// ---------------------------------------------------------------------------
// HTTP handler methods — read operations
// ---------------------------------------------------------------------------

// GetEntity handles GET /api/v1/entity/get
func (h *Handlers) GetEntity(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		EntityType string `json:"entity_type"`
		EntityID   int    `json:"entity_id"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "get_entity",
		OperationClass: "read",
		TargetEntity:   body.EntityType,
		TargetEntityID: body.EntityID,
		Params:         map[string]interface{}{"entity_type": body.EntityType, "entity_id": body.EntityID},
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

	// gate validated auth/authz/audit — now perform the read
	var omittedFields []string
	if resp.Metadata != nil {
		if of, ok := resp.Metadata["omitted_fields"].([]string); ok {
			omittedFields = of
		}
	}

	result, err := getEntity(h.db, h.schema, body.EntityType, body.EntityID, omittedFields)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false, "error": err.Error(), "audit_entry_id": resp.AuditEntryID,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "data": result, "audit_entry_id": resp.AuditEntryID,
		"warnings": resp.Warnings,
	})
}

// GetEntityHistory handles POST /api/v1/entity/history
func (h *Handlers) GetEntityHistory(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		EntityType string  `json:"entity_type"`
		EntityID   int     `json:"entity_id"`
		StartTime  *string `json:"start_time"`
		EndTime    *string `json:"end_time"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "get_entity_history",
		OperationClass: "read",
		TargetEntity:   body.EntityType,
		TargetEntityID: body.EntityID,
		Params:         map[string]interface{}{"entity_type": body.EntityType, "entity_id": body.EntityID},
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

	var startTime, endTime *time.Time
	if body.StartTime != nil {
		t, err := time.Parse(time.RFC3339, *body.StartTime)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false, "error": fmt.Sprintf("invalid start_time: %v", err),
			})
			return
		}
		startTime = &t
	}
	if body.EndTime != nil {
		t, err := time.Parse(time.RFC3339, *body.EndTime)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false, "error": fmt.Sprintf("invalid end_time: %v", err),
			})
			return
		}
		endTime = &t
	}

	versions, err := getEntityHistory(h.db, h.schema, body.EntityType, body.EntityID, startTime, endTime)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": err.Error(), "audit_entry_id": resp.AuditEntryID,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "data": versions, "audit_entry_id": resp.AuditEntryID,
		"warnings": resp.Warnings,
	})
}

// GetEntityAtTime handles POST /api/v1/entity/at-time
func (h *Handlers) GetEntityAtTime(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		EntityType string `json:"entity_type"`
		EntityID   int    `json:"entity_id"`
		Timestamp  string `json:"timestamp"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	timestamp, err := time.Parse(time.RFC3339, body.Timestamp)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": fmt.Sprintf("invalid timestamp: %v", err),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "get_entity_at_time",
		OperationClass: "read",
		TargetEntity:   body.EntityType,
		TargetEntityID: body.EntityID,
		Params:         map[string]interface{}{"entity_type": body.EntityType, "entity_id": body.EntityID, "timestamp": body.Timestamp},
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

	version, err := getEntityAtTime(h.db, h.schema, body.EntityType, body.EntityID, timestamp)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false, "error": err.Error(), "audit_entry_id": resp.AuditEntryID,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "data": version, "audit_entry_id": resp.AuditEntryID,
		"warnings": resp.Warnings,
	})
}

// Search handles POST /api/v1/search
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var params SearchParams
	if err := parseJSONBody(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "search",
		OperationClass: "read",
		TargetEntity:   params.EntityType,
		Params: map[string]interface{}{
			"entity_type":   params.EntityType,
			"filters":       params.Filters,
			"joins":         params.Joins,
			"projection":    params.Projection,
			"ordering":      params.Ordering,
			"cursor":        params.Cursor,
			"offset":        params.Offset,
			"limit":         params.Limit,
			"max_staleness": params.MaxStaleness,
			"view_mode":     params.ViewMode,
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

	var omittedFields []string
	if resp.Metadata != nil {
		if of, ok := resp.Metadata["omitted_fields"].([]string); ok {
			omittedFields = of
		}
	}

	result, err := search(h.db, h.schema, &params, omittedFields)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": err.Error(), "audit_entry_id": resp.AuditEntryID,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "data": result, "audit_entry_id": resp.AuditEntryID,
		"warnings": resp.Warnings,
	})
}

// GetDependencies handles POST /api/v1/dependencies
func (h *Handlers) GetDependencies(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		EntityType string `json:"entity_type"`
		EntityID   int    `json:"entity_id"`
		Pattern    string `json:"pattern"`
		MaxDepth   int    `json:"max_depth"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "get_dependencies",
		OperationClass: "read",
		TargetEntity:   body.EntityType,
		TargetEntityID: body.EntityID,
		Params: map[string]interface{}{
			"entity_type": body.EntityType,
			"entity_id":   body.EntityID,
			"pattern":     body.Pattern,
			"max_depth":   body.MaxDepth,
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

	nodes, err := getDependencies(h.db, body.EntityType, body.EntityID, body.Pattern, body.MaxDepth)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": err.Error(), "audit_entry_id": resp.AuditEntryID,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "data": nodes, "audit_entry_id": resp.AuditEntryID,
		"warnings": resp.Warnings,
	})
}

// ChangeSetView handles POST /api/v1/changeset/view
func (h *Handlers) ChangeSetView(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		ChangeSetID int    `json:"change_set_id"`
		ViewMode    string `json:"view_mode"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "changeset_view",
		OperationClass: "read",
		TargetEntity:   "change_set",
		TargetEntityID: body.ChangeSetID,
		Params:         map[string]interface{}{"change_set_id": body.ChangeSetID, "view_mode": body.ViewMode},
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

	result, err := queryChangeSetView(h.db, body.ChangeSetID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false, "error": err.Error(), "audit_entry_id": resp.AuditEntryID,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "data": result, "audit_entry_id": resp.AuditEntryID,
		"warnings": resp.Warnings,
	})
}

// ---------------------------------------------------------------------------
// Domain functions — read queries (called by handlers after gate validates)
// ---------------------------------------------------------------------------

// getEntity fetches one entity row by primary key. Returns current state
// with all fields the caller is authorized to see.
func getEntity(db *pg.DB, schema *runtimeschema.RuntimeSchema, entityType string, entityID int, omittedFields []string) (*EntityRow, error) {
	_, found := schema.GetEntityType(entityType)
	if !found {
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = $1",
		pg.QuoteIdentifier(entityType))

	rows, err := db.Query(query, entityID)
	if err != nil {
		return nil, fmt.Errorf("entity query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	if !rows.Next() {
		return nil, fmt.Errorf("%s with id=%d not found", entityType, entityID)
	}

	fieldValues, err := scanRowToMap(rows, columns)
	if err != nil {
		return nil, fmt.Errorf("row scan failed: %w", err)
	}

	for _, omitted := range omittedFields {
		delete(fieldValues, omitted)
	}

	result := &EntityRow{
		EntityType: entityType,
		EntityID:   entityID,
		Fields:     fieldValues,
	}

	if schema.IsVersioned(entityType) {
		versionSerial, err := readCurrentVersionSerial(db, entityType, entityID)
		if err == nil && versionSerial > 0 {
			result.VersionSerial = &versionSerial
		}
	}

	return result, nil
}

// getEntityHistory fetches the version chain for one entity. Returns all
// versions ordered by version_serial descending (newest first).
func getEntityHistory(db *pg.DB, schema *runtimeschema.RuntimeSchema, entityType string, entityID int, startTime *time.Time, endTime *time.Time) ([]VersionRow, error) {
	if !schema.IsVersioned(entityType) {
		return nil, fmt.Errorf("entity type %s is not versioned", entityType)
	}

	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	var queryParts []string
	var args []interface{}
	argIdx := 1

	queryParts = append(queryParts, fmt.Sprintf("SELECT * FROM %s WHERE %s = $%d",
		pg.QuoteIdentifier(versionTable),
		pg.QuoteIdentifier(fkColumn),
		argIdx))
	args = append(args, entityID)
	argIdx++

	if startTime != nil {
		queryParts = append(queryParts, fmt.Sprintf("AND approved_for_production_time >= $%d", argIdx))
		args = append(args, *startTime)
		argIdx++
	}

	if endTime != nil {
		queryParts = append(queryParts, fmt.Sprintf("AND approved_for_production_time <= $%d", argIdx))
		args = append(args, *endTime)
		argIdx++
	}

	queryParts = append(queryParts, "ORDER BY version_serial DESC")

	query := strings.Join(queryParts, " ")

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("version history query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	var versions []VersionRow
	for rows.Next() {
		fieldValues, err := scanRowToMap(rows, columns)
		if err != nil {
			return nil, fmt.Errorf("version row scan failed: %w", err)
		}

		vr := VersionRow{
			Fields: fieldValues,
		}

		if id, ok := fieldValues["id"]; ok {
			if intID, ok := toInt(id); ok {
				vr.VersionID = intID
			}
		}
		if serial, ok := fieldValues["version_serial"]; ok {
			if intSerial, ok := toInt(serial); ok {
				vr.VersionSerial = intSerial
			}
		}
		if active, ok := fieldValues["is_active_version"]; ok {
			if boolActive, ok := active.(bool); ok {
				vr.IsActiveVersion = boolActive
			}
		}
		if csID, ok := fieldValues["change_set_id"]; ok {
			if intCS, ok := toInt(csID); ok {
				vr.ChangeSetID = &intCS
			}
		}

		versions = append(versions, vr)
	}

	return versions, rows.Err()
}

// getEntityAtTime reconstructs field values active at a specific timestamp.
func getEntityAtTime(db *pg.DB, schema *runtimeschema.RuntimeSchema, entityType string, entityID int, timestamp time.Time) (*VersionRow, error) {
	if !schema.IsVersioned(entityType) {
		return nil, fmt.Errorf("entity type %s is not versioned", entityType)
	}

	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s = $1 AND approved_for_production_time <= $2 "+
			"ORDER BY approved_for_production_time DESC LIMIT 1",
		pg.QuoteIdentifier(versionTable),
		pg.QuoteIdentifier(fkColumn),
	)

	rows, err := db.Query(query, entityID, timestamp)
	if err != nil {
		return nil, fmt.Errorf("point-in-time query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	if !rows.Next() {
		return nil, fmt.Errorf("no version of %s id=%d exists at or before %s",
			entityType, entityID, timestamp.Format(time.RFC3339))
	}

	fieldValues, err := scanRowToMap(rows, columns)
	if err != nil {
		return nil, fmt.Errorf("version row scan failed: %w", err)
	}

	vr := &VersionRow{Fields: fieldValues}
	if serial, ok := fieldValues["version_serial"]; ok {
		if intSerial, ok := toInt(serial); ok {
			vr.VersionSerial = intSerial
		}
	}
	if id, ok := fieldValues["id"]; ok {
		if intID, ok := toInt(id); ok {
			vr.VersionID = intID
		}
	}
	if active, ok := fieldValues["is_active_version"]; ok {
		if boolActive, ok := active.(bool); ok {
			vr.IsActiveVersion = boolActive
		}
	}

	return vr, nil
}

// search is the discovery surface across entity types. Builds a SQL query
// from structured parameters with enforced bounds on result size, join
// depth, predicate count, and query time.
func search(db *pg.DB, schema *runtimeschema.RuntimeSchema, params *SearchParams, omittedFields []string) (*SearchResult, error) {
	_, found := schema.GetEntityType(params.EntityType)
	if !found {
		return nil, fmt.Errorf("unknown entity type: %s", params.EntityType)
	}

	if params.Limit <= 0 {
		params.Limit = defaultSearchLimit
	}
	if params.Limit > maxSearchLimit {
		params.Limit = maxSearchLimit
	}
	if len(params.Filters) > maxPredicateCount {
		return nil, fmt.Errorf("too many filter predicates: %d (max %d)",
			len(params.Filters), maxPredicateCount)
	}
	if len(params.Joins) > maxJoinDepth {
		return nil, fmt.Errorf("too many joins: %d (max %d)",
			len(params.Joins), maxJoinDepth)
	}

	selectClause := buildSelectClause(params.EntityType, params.Projection, schema, omittedFields)
	fromClause := pg.QuoteIdentifier(params.EntityType)
	joinClause := buildJoinClause(params.EntityType, params.Joins, schema)
	whereClause, whereArgs := buildWhereClause(params.EntityType, params.Filters, params.MaxStaleness)
	orderClause := buildOrderClause(params.Ordering)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s %s",
		fromClause, joinClause, whereClause)

	var totalCount int
	err := db.QueryRow(countQuery, whereArgs...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("count query failed: %w", err)
	}

	dataQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s LIMIT %d OFFSET %d",
		selectClause, fromClause, joinClause, whereClause, orderClause,
		params.Limit, params.Offset)

	rows, err := db.Query(dataQuery, whereArgs...)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to read columns: %w", err)
	}

	var resultRows []map[string]interface{}
	for rows.Next() {
		fieldValues, err := scanRowToMap(rows, columns)
		if err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}

		for _, omitted := range omittedFields {
			delete(fieldValues, omitted)
		}

		resultRows = append(resultRows, fieldValues)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search iteration failed: %w", err)
	}

	result := &SearchResult{
		Rows:       resultRows,
		TotalCount: totalCount,
	}

	if isObservationCacheTable(params.EntityType) {
		result.FreshnessSummary = buildFreshnessSummary(resultRows)
	}

	if len(resultRows) > 0 && len(resultRows) == params.Limit {
		result.Cursor = computeCursor(params.Offset + params.Limit)
	}

	return result, nil
}

// getDependencies walks the substrate hierarchy or service connection graph.
func getDependencies(db *pg.DB, startEntity string, startID int, pattern string, maxDepth int) ([]DependencyNode, error) {
	if maxDepth <= 0 {
		maxDepth = 10
	}
	if maxDepth > 50 {
		maxDepth = 50
	}

	switch pattern {
	case "substrate_parent_chain":
		return walkParentChain(db, "megavisor_instance", "parent_megavisor_instance_id",
			startID, maxDepth)
	case "service_connections":
		return walkServiceConnections(db, startID, maxDepth)
	case "location_ancestry":
		return walkParentChain(db, "location", "parent_location_id",
			startID, maxDepth)
	case "host_group_machines":
		return walkHostGroupMachines(db, startID)
	case "service_package_chain":
		return walkServicePackages(db, startID)
	default:
		return nil, fmt.Errorf("unknown dependency pattern: %s", pattern)
	}
}

// queryChangeSetView fetches a complete change set view including field
// changes, approvals, rejections, and requirements.
func queryChangeSetView(db *pg.DB, changeSetID int) (*ChangeSetViewResult, error) {
	// read the change set row
	var status, name, reason string
	var isEmergency, isBulk bool
	var submittedBy *int
	var submittedTime *time.Time

	err := db.QueryRow(
		"SELECT status, name, reason, is_emergency, is_bulk, "+
			"submitted_by_ops_user_id, submitted_time "+
			"FROM change_set WHERE id = $1",
		changeSetID,
	).Scan(&status, &name, &reason, &isEmergency, &isBulk, &submittedBy, &submittedTime)
	if err != nil {
		if pg.IsNoRows(err) {
			return nil, fmt.Errorf("change_set with id=%d not found", changeSetID)
		}
		return nil, fmt.Errorf("change_set query failed: %w", err)
	}

	result := &ChangeSetViewResult{
		ChangeSetID:   changeSetID,
		Status:        status,
		Name:          name,
		Reason:        reason,
		IsEmergency:   isEmergency,
		IsBulk:        isBulk,
		SubmittedBy:   submittedBy,
		SubmittedTime: submittedTime,
	}

	// read field changes
	fcRows, err := db.Query(
		"SELECT id, target_entity_type, target_entity_id, field_name, "+
			"before_value, after_value, change_type, applied_status, apply_order "+
			"FROM change_set_field_change WHERE change_set_id = $1 ORDER BY apply_order",
		changeSetID,
	)
	if err != nil {
		return nil, fmt.Errorf("field change query failed: %w", err)
	}
	defer fcRows.Close()

	for fcRows.Next() {
		var fcID, entityID, applyOrder int
		var entityType, fieldName, changeType, appliedStatus string
		var beforeValue, afterValue interface{}
		if err := fcRows.Scan(&fcID, &entityType, &entityID, &fieldName,
			&beforeValue, &afterValue, &changeType, &appliedStatus, &applyOrder); err != nil {
			return nil, fmt.Errorf("field change scan failed: %w", err)
		}
		result.FieldChanges = append(result.FieldChanges, map[string]interface{}{
			"id":                 fcID,
			"target_entity_type": entityType,
			"target_entity_id":   entityID,
			"field_name":         fieldName,
			"before_value":       beforeValue,
			"after_value":        afterValue,
			"change_type":        changeType,
			"applied_status":     appliedStatus,
			"apply_order":        applyOrder,
		})
	}
	if err := fcRows.Err(); err != nil {
		return nil, fmt.Errorf("field change iteration failed: %w", err)
	}

	// read approvals
	approvalRows, err := db.Query(
		"SELECT id, approved_by_ops_user_id, comment, created_time "+
			"FROM change_set_approval WHERE change_set_id = $1 ORDER BY created_time",
		changeSetID,
	)
	if err != nil {
		return nil, fmt.Errorf("approval query failed: %w", err)
	}
	defer approvalRows.Close()

	for approvalRows.Next() {
		var aID, approverID int
		var comment string
		var createdTime time.Time
		if err := approvalRows.Scan(&aID, &approverID, &comment, &createdTime); err != nil {
			return nil, fmt.Errorf("approval scan failed: %w", err)
		}
		result.Approvals = append(result.Approvals, map[string]interface{}{
			"id":           aID,
			"approver_id":  approverID,
			"comment":      comment,
			"created_time": createdTime,
		})
	}
	if err := approvalRows.Err(); err != nil {
		return nil, fmt.Errorf("approval iteration failed: %w", err)
	}

	// read rejections
	rejectionRows, err := db.Query(
		"SELECT id, rejected_by_ops_user_id, reason, created_time "+
			"FROM change_set_rejection WHERE change_set_id = $1 ORDER BY created_time",
		changeSetID,
	)
	if err != nil {
		return nil, fmt.Errorf("rejection query failed: %w", err)
	}
	defer rejectionRows.Close()

	for rejectionRows.Next() {
		var rID, rejectorID int
		var rejReason string
		var createdTime time.Time
		if err := rejectionRows.Scan(&rID, &rejectorID, &rejReason, &createdTime); err != nil {
			return nil, fmt.Errorf("rejection scan failed: %w", err)
		}
		result.Rejections = append(result.Rejections, map[string]interface{}{
			"id":           rID,
			"rejector_id":  rejectorID,
			"reason":       rejReason,
			"created_time": createdTime,
		})
	}
	if err := rejectionRows.Err(); err != nil {
		return nil, fmt.Errorf("rejection iteration failed: %w", err)
	}

	// read approval requirements
	reqRows, err := db.Query(
		"SELECT car.id, car.approval_rule_id, car.required_group_id, "+
			"car.required_count, car.fulfilled_count, car.is_fulfilled, "+
			"COALESCE(g.name, '') AS group_name "+
			"FROM change_set_approval_required car "+
			"LEFT JOIN ops_group g ON g.id = car.required_group_id "+
			"WHERE car.change_set_id = $1",
		changeSetID,
	)
	if err != nil {
		return nil, fmt.Errorf("requirements query failed: %w", err)
	}
	defer reqRows.Close()

	for reqRows.Next() {
		var reqID, ruleID, groupID, requiredCount, fulfilledCount int
		var isFulfilled bool
		var groupName string
		if err := reqRows.Scan(&reqID, &ruleID, &groupID, &requiredCount,
			&fulfilledCount, &isFulfilled, &groupName); err != nil {
			return nil, fmt.Errorf("requirement scan failed: %w", err)
		}
		result.Requirements = append(result.Requirements, map[string]interface{}{
			"id":              reqID,
			"rule_id":         ruleID,
			"group_id":        groupID,
			"group_name":      groupName,
			"required_count":  requiredCount,
			"fulfilled_count": fulfilledCount,
			"is_fulfilled":    isFulfilled,
		})
	}
	if err := reqRows.Err(); err != nil {
		return nil, fmt.Errorf("requirement iteration failed: %w", err)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Query building helpers
// ---------------------------------------------------------------------------

func buildSelectClause(entityType string, projection string, schema *runtimeschema.RuntimeSchema, omittedFields []string) string {
	tableName := pg.QuoteIdentifier(entityType)

	switch projection {
	case "summary":
		return fmt.Sprintf("%s.id, %s.created_time, %s.updated_time",
			tableName, tableName, tableName)
	case "full_with_history":
		return tableName + ".*"
	case "":
		return tableName + ".*"
	default:
		if strings.Contains(projection, ",") {
			fields := strings.Split(projection, ",")
			qualified := make([]string, 0, len(fields))
			for _, f := range fields {
				trimmed := strings.TrimSpace(f)
				if trimmed != "" {
					qualified = append(qualified, fmt.Sprintf("%s.%s",
						tableName, pg.QuoteIdentifier(trimmed)))
				}
			}
			if len(qualified) > 0 {
				return strings.Join(qualified, ", ")
			}
		}
		return tableName + ".*"
	}
}

func buildJoinClause(entityType string, joins []string, schema *runtimeschema.RuntimeSchema) string {
	if len(joins) == 0 {
		return ""
	}

	var clauses []string
	for _, joinTarget := range joins {
		rels := schema.GetRelationships(entityType)
		for _, rel := range rels {
			if rel.TargetEntity == joinTarget {
				clauses = append(clauses, fmt.Sprintf(
					"LEFT JOIN %s ON %s.%s = %s.id",
					pg.QuoteIdentifier(rel.TargetEntity),
					pg.QuoteIdentifier(entityType),
					pg.QuoteIdentifier(rel.SourceField),
					pg.QuoteIdentifier(rel.TargetEntity),
				))
				break
			}
		}
	}

	return strings.Join(clauses, " ")
}

func buildWhereClause(entityType string, filters []FilterPredicate, maxStaleness int) (string, []interface{}) {
	if len(filters) == 0 && maxStaleness <= 0 {
		return "", nil
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	for _, f := range filters {
		condition, filterArgs := buildFilterCondition(entityType, f, &argIdx)
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, filterArgs...)
		}
	}

	if maxStaleness > 0 && isObservationCacheTable(entityType) {
		conditions = append(conditions,
			fmt.Sprintf("%s._observed_time >= NOW() - INTERVAL '%d seconds'",
				pg.QuoteIdentifier(entityType), maxStaleness))
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

func buildFilterCondition(entityType string, f FilterPredicate, argIdx *int) (string, []interface{}) {
	qualifiedField := fmt.Sprintf("%s.%s",
		pg.QuoteIdentifier(entityType),
		pg.QuoteIdentifier(f.Field))

	switch f.Operator {
	case "eq", "":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s = %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "ne":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s != %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "gt":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s > %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "gte":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s >= %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "lt":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s < %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "lte":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s <= %s", qualifiedField, placeholder), []interface{}{f.Value}

	case "in":
		values, ok := f.Value.([]interface{})
		if !ok {
			return "", nil
		}
		placeholders := make([]string, 0, len(values))
		args := make([]interface{}, 0, len(values))
		for _, v := range values {
			placeholders = append(placeholders, fmt.Sprintf("$%d", *argIdx))
			args = append(args, v)
			*argIdx++
		}
		return fmt.Sprintf("%s IN (%s)", qualifiedField, strings.Join(placeholders, ", ")), args

	case "like":
		strVal, ok := f.Value.(string)
		if !ok {
			return "", nil
		}
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s LIKE %s", qualifiedField, placeholder), []interface{}{strVal}

	case "is_null":
		return fmt.Sprintf("%s IS NULL", qualifiedField), nil

	case "is_not_null":
		return fmt.Sprintf("%s IS NOT NULL", qualifiedField), nil

	case "between":
		rangeVals, ok := f.Value.([]interface{})
		if !ok || len(rangeVals) != 2 {
			return "", nil
		}
		p1 := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		p2 := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s BETWEEN %s AND %s", qualifiedField, p1, p2),
			[]interface{}{rangeVals[0], rangeVals[1]}

	case "json_contains":
		placeholder := fmt.Sprintf("$%d", *argIdx)
		*argIdx++
		return fmt.Sprintf("%s @> %s::jsonb", qualifiedField, placeholder), []interface{}{f.Value}

	default:
		return "", nil
	}
}

func buildOrderClause(ordering []OrderSpec) string {
	if len(ordering) == 0 {
		return "ORDER BY id ASC"
	}

	parts := make([]string, 0, len(ordering)+1)
	for _, o := range ordering {
		dir := "ASC"
		if strings.ToLower(o.Direction) == "desc" {
			dir = "DESC"
		}
		parts = append(parts, fmt.Sprintf("%s %s", pg.QuoteIdentifier(o.Field), dir))
	}

	parts = append(parts, "id ASC")

	return "ORDER BY " + strings.Join(parts, ", ")
}

// ---------------------------------------------------------------------------
// Dependency walk implementations
// ---------------------------------------------------------------------------

func walkParentChain(db *pg.DB, entityType string, parentColumn string, startID int, maxDepth int) ([]DependencyNode, error) {
	query := fmt.Sprintf(
		"WITH RECURSIVE chain AS ("+
			"SELECT id, %s AS parent_id, 0 AS depth FROM %s WHERE id = $1 "+
			"UNION ALL "+
			"SELECT t.id, t.%s, c.depth + 1 FROM %s t "+
			"JOIN chain c ON t.id = c.parent_id "+
			"WHERE c.depth < $2"+
			") SELECT id, parent_id, depth FROM chain ORDER BY depth ASC",
		pg.QuoteIdentifier(parentColumn),
		pg.QuoteIdentifier(entityType),
		pg.QuoteIdentifier(parentColumn),
		pg.QuoteIdentifier(entityType),
	)

	rows, err := db.Query(query, startID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("parent chain query failed: %w", err)
	}
	defer rows.Close()

	var nodes []DependencyNode
	for rows.Next() {
		var id int
		var parentID *int
		var depth int
		if err := rows.Scan(&id, &parentID, &depth); err != nil {
			return nil, err
		}
		node := DependencyNode{
			EntityType: entityType,
			EntityID:   id,
			Depth:      depth,
			Metadata:   make(map[string]interface{}),
		}
		if parentID != nil {
			node.Metadata["parent_id"] = *parentID
		}
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

func walkServiceConnections(db *pg.DB, serviceID int, maxDepth int) ([]DependencyNode, error) {
	query := "WITH RECURSIVE deps AS (" +
		"SELECT destination_service_id AS id, 1 AS depth " +
		"FROM service_connection WHERE source_service_id = $1 AND is_active = true " +
		"UNION ALL " +
		"SELECT sc.destination_service_id, d.depth + 1 " +
		"FROM service_connection sc " +
		"JOIN deps d ON sc.source_service_id = d.id " +
		"WHERE d.depth < $2 AND sc.is_active = true" +
		") SELECT DISTINCT id, depth FROM deps ORDER BY depth ASC, id ASC"

	rows, err := db.Query(query, serviceID, maxDepth)
	if err != nil {
		return nil, fmt.Errorf("service connection query failed: %w", err)
	}
	defer rows.Close()

	nodes := []DependencyNode{{
		EntityType: "service",
		EntityID:   serviceID,
		Depth:      0,
		Metadata:   map[string]interface{}{"role": "source"},
	}}

	for rows.Next() {
		var id, depth int
		if err := rows.Scan(&id, &depth); err != nil {
			return nil, err
		}
		nodes = append(nodes, DependencyNode{
			EntityType: "service",
			EntityID:   id,
			Depth:      depth,
			Metadata:   map[string]interface{}{"role": "dependency"},
		})
	}

	return nodes, rows.Err()
}

func walkHostGroupMachines(db *pg.DB, hostGroupID int) ([]DependencyNode, error) {
	rows, err := db.Query(
		"SELECT hgm.machine_id FROM host_group_machine hgm "+
			"WHERE hgm.host_group_id = $1 AND hgm.is_active = true "+
			"ORDER BY hgm.machine_id",
		hostGroupID,
	)
	if err != nil {
		return nil, fmt.Errorf("host group machine query failed: %w", err)
	}
	defer rows.Close()

	nodes := []DependencyNode{{
		EntityType: "host_group",
		EntityID:   hostGroupID,
		Depth:      0,
		Metadata:   map[string]interface{}{"role": "group"},
	}}

	for rows.Next() {
		var machineID int
		if err := rows.Scan(&machineID); err != nil {
			return nil, err
		}
		nodes = append(nodes, DependencyNode{
			EntityType: "machine",
			EntityID:   machineID,
			Depth:      1,
			Metadata:   map[string]interface{}{"role": "member"},
		})
	}

	return nodes, rows.Err()
}

func walkServicePackages(db *pg.DB, serviceID int) ([]DependencyNode, error) {
	rows, err := db.Query(
		"SELECT sp.package_id, sp.install_order FROM service_package sp "+
			"WHERE sp.service_id = $1 AND sp.is_active = true "+
			"ORDER BY sp.install_order",
		serviceID,
	)
	if err != nil {
		return nil, fmt.Errorf("service package query failed: %w", err)
	}
	defer rows.Close()

	nodes := []DependencyNode{{
		EntityType: "service",
		EntityID:   serviceID,
		Depth:      0,
		Metadata:   map[string]interface{}{"role": "service"},
	}}

	for rows.Next() {
		var packageID, installOrder int
		if err := rows.Scan(&packageID, &installOrder); err != nil {
			return nil, err
		}
		nodes = append(nodes, DependencyNode{
			EntityType: "package",
			EntityID:   packageID,
			Depth:      1,
			Metadata: map[string]interface{}{
				"role":          "installed_package",
				"install_order": installOrder,
			},
		})
	}

	return nodes, rows.Err()
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

// scanRowToMap scans a database row into a map of column name to value.
func scanRowToMap(rows pg.Rows, columns []string) (map[string]interface{}, error) {
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	result := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		result[col] = values[i]
	}

	return result, nil
}

// readCurrentVersionSerial reads the active version serial for an entity.
func readCurrentVersionSerial(db *pg.DB, entityType string, entityID int) (int, error) {
	versionTable := entityType + "_version"
	fkColumn := entityType + "_id"

	var serial int
	err := db.QueryRow(
		fmt.Sprintf("SELECT version_serial FROM %s WHERE %s = $1 AND is_active_version = true LIMIT 1",
			pg.QuoteIdentifier(versionTable),
			pg.QuoteIdentifier(fkColumn)),
		entityID,
	).Scan(&serial)
	if err != nil {
		return 0, err
	}
	return serial, nil
}

// isObservationCacheTable checks if an entity type is an observation cache table.
func isObservationCacheTable(entityType string) bool {
	switch entityType {
	case "observation_cache_metric", "observation_cache_state", "observation_cache_config":
		return true
	default:
		return false
	}
}

// buildFreshnessSummary computes staleness statistics for observation cache rows.
func buildFreshnessSummary(rows []map[string]interface{}) map[string]interface{} {
	if len(rows) == 0 {
		return nil
	}

	var oldestObservation, newestObservation time.Time
	now := time.Now().UTC()
	totalStaleness := 0.0
	count := 0

	for _, row := range rows {
		observedTime, ok := row["_observed_time"].(time.Time)
		if !ok {
			continue
		}

		if count == 0 || observedTime.Before(oldestObservation) {
			oldestObservation = observedTime
		}
		if count == 0 || observedTime.After(newestObservation) {
			newestObservation = observedTime
		}

		totalStaleness += now.Sub(observedTime).Seconds()
		count++
	}

	if count == 0 {
		return nil
	}

	return map[string]interface{}{
		"row_count":             count,
		"oldest_observation":    oldestObservation.Format(time.RFC3339),
		"newest_observation":    newestObservation.Format(time.RFC3339),
		"max_staleness_seconds": int(now.Sub(oldestObservation).Seconds()),
		"min_staleness_seconds": int(now.Sub(newestObservation).Seconds()),
		"avg_staleness_seconds": int(totalStaleness / float64(count)),
	}
}

// computeCursor encodes a pagination cursor from offset.
func computeCursor(nextOffset int) string {
	return fmt.Sprintf("offset:%d", nextOffset)
}

// toInt converts numeric interface values to int.
func toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}
