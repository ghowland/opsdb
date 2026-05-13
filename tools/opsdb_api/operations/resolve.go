//# tools/opsdb_api/operations/resolve.go

package operations

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/tools/opsdb_api/gate"
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// ResolveResult holds the result of an authority pointer resolution.
// Contains everything a caller needs to fetch the resource from the
// authority directly — the API does not fetch from the authority itself.
type ResolveResult struct {
	AuthorityID      int                    `json:"authority_id"`
	AuthorityName    string                 `json:"authority_name"`
	AuthorityType    string                 `json:"authority_type"`
	BaseURL          string                 `json:"base_url"`
	PointerID        int                    `json:"pointer_id"`
	PointerType      string                 `json:"pointer_type"`
	Locator          string                 `json:"locator"`
	PointerDataJSON  map[string]interface{} `json:"pointer_data_json,omitempty"`
	LastVerifiedTime *time.Time             `json:"last_verified_time,omitempty"`
}

// ---------------------------------------------------------------------------
// HTTP handler
// ---------------------------------------------------------------------------

// ResolveAuthorityPointer handles POST /api/v1/authority/resolve
func (h *Handlers) ResolveAuthorityPointer(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body struct {
		PointerID     int    `json:"pointer_id"`
		AuthorityName string `json:"authority_name"`
		Locator       string `json:"locator"`
	}
	if err := parseJSONBody(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	// determine target entity ID: pointer_id if given, otherwise we resolve
	// by name+locator after the gate validates
	targetID := body.PointerID

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "resolve_authority_pointer",
		OperationClass: "read",
		TargetEntity:   "authority_pointer",
		TargetEntityID: targetID,
		Params: map[string]interface{}{
			"pointer_id":     body.PointerID,
			"authority_name": body.AuthorityName,
			"locator":        body.Locator,
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

	// gate validated auth/authz/audit — now perform the read
	var result *ResolveResult
	var err error

	if body.PointerID > 0 {
		result, err = resolveAuthorityPointer(h.db, body.PointerID)
	} else if body.AuthorityName != "" && body.Locator != "" {
		result, err = resolveAuthorityPointerByPath(h.db, body.AuthorityName, body.Locator)
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "either pointer_id or both authority_name and locator are required",
		})
		return
	}

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

// ---------------------------------------------------------------------------
// Domain functions
// ---------------------------------------------------------------------------

// resolveAuthorityPointer performs a where-is-X lookup by pointer ID.
// Returns authority connection details and the locator for the specific
// resource within that authority.
func resolveAuthorityPointer(db *pg.DB, pointerID int) (*ResolveResult, error) {
	var authorityID int
	var pointerType, locator string
	var pointerDataJSON []byte
	var lastVerifiedTime *time.Time

	err := db.QueryRow(
		"SELECT authority_id, pointer_type, locator, pointer_data_json, last_verified_time "+
			"FROM authority_pointer WHERE id = $1 AND is_active = true",
		pointerID,
	).Scan(&authorityID, &pointerType, &locator, &pointerDataJSON, &lastVerifiedTime)
	if err != nil {
		if pg.IsNoRows(err) {
			return nil, fmt.Errorf("authority_pointer with id=%d not found or inactive", pointerID)
		}
		return nil, fmt.Errorf("authority_pointer query failed: %w", err)
	}

	var authorityName, authorityType string
	var authorityDataJSON []byte

	err = db.QueryRow(
		"SELECT name, authority_type, authority_data_json "+
			"FROM authority WHERE id = $1 AND is_active = true",
		authorityID,
	).Scan(&authorityName, &authorityType, &authorityDataJSON)
	if err != nil {
		if pg.IsNoRows(err) {
			return nil, fmt.Errorf("authority with id=%d not found or inactive (referenced by pointer %d)",
				authorityID, pointerID)
		}
		return nil, fmt.Errorf("authority query failed: %w", err)
	}

	baseURL := extractBaseURL(authorityDataJSON, authorityType)

	var pointerData map[string]interface{}
	if len(pointerDataJSON) > 0 {
		pg.UnmarshalJSON(pointerDataJSON, &pointerData)
	}

	return &ResolveResult{
		AuthorityID:      authorityID,
		AuthorityName:    authorityName,
		AuthorityType:    authorityType,
		BaseURL:          baseURL,
		PointerID:        pointerID,
		PointerType:      pointerType,
		Locator:          locator,
		PointerDataJSON:  pointerData,
		LastVerifiedTime: lastVerifiedTime,
	}, nil
}

// resolveAuthorityPointerByPath resolves an authority pointer by authority
// name and locator path instead of by ID.
func resolveAuthorityPointerByPath(db *pg.DB, authorityName string, locator string) (*ResolveResult, error) {
	var pointerID int
	err := db.QueryRow(
		"SELECT ap.id FROM authority_pointer ap "+
			"JOIN authority a ON a.id = ap.authority_id "+
			"WHERE a.name = $1 AND ap.locator = $2 "+
			"AND ap.is_active = true AND a.is_active = true "+
			"LIMIT 1",
		authorityName, locator,
	).Scan(&pointerID)
	if err != nil {
		if pg.IsNoRows(err) {
			return nil, fmt.Errorf("authority_pointer not found for authority=%s locator=%s",
				authorityName, locator)
		}
		return nil, fmt.Errorf("authority_pointer lookup failed: %w", err)
	}

	return resolveAuthorityPointer(db, pointerID)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractBaseURL pulls the base_url or equivalent connection string from
// the authority's typed data JSON. Different authority types store their
// connection details in different fields.
func extractBaseURL(authorityDataJSON []byte, authorityType string) string {
	if len(authorityDataJSON) == 0 {
		return ""
	}

	var data map[string]interface{}
	if err := pg.UnmarshalJSON(authorityDataJSON, &data); err != nil {
		return ""
	}

	for _, field := range []string{"base_url", "url", "endpoint", "address", "server_url"} {
		if val, ok := data[field].(string); ok && val != "" {
			return val
		}
	}

	switch authorityType {
	case "prometheus_server":
		if val, ok := data["prometheus_url"].(string); ok {
			return val
		}
	case "secret_vault":
		if val, ok := data["vault_addr"].(string); ok {
			return val
		}
	case "identity_provider":
		if val, ok := data["issuer_url"].(string); ok {
			return val
		}
	case "code_repository":
		if val, ok := data["repo_url"].(string); ok {
			return val
		}
	case "container_registry":
		if val, ok := data["registry_url"].(string); ok {
			return val
		}
	}

	return ""
}
