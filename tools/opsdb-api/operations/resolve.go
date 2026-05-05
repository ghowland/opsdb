//# tools/opsdb-api/operations/resolve.go

package operations

import (
	"fmt"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
)

// ResolveResult holds the result of an authority pointer resolution.
// Contains everything a caller needs to fetch the resource from the
// authority directly — the API does not fetch from the authority itself.
type ResolveResult struct {
	AuthorityID      int
	AuthorityName    string
	AuthorityType    string
	BaseURL          string
	PointerID        int
	PointerType      string
	Locator          string
	PointerDataJSON  map[string]interface{}
	LastVerifiedTime *time.Time
}

// ResolveAuthorityPointer performs a where-is-X lookup. Returns authority
// connection details and the locator for the specific resource within
// that authority. Does NOT fetch from the authority — the caller uses
// the returned coordinates to query the authority directly.
func ResolveAuthorityPointer(db *pg.DB, pointerID int) (*ResolveResult, error) {
	// read the authority_pointer row
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

	// read the parent authority row for connection details
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

	// extract base_url from authority_data_json
	baseURL := extractBaseURL(authorityDataJSON, authorityType)

	// parse pointer data
	var pointerData map[string]interface{}
	if len(pointerDataJSON) > 0 {
		pg.UnmarshalJSON(pointerDataJSON, &pointerData)
	}

	result := &ResolveResult{
		AuthorityID:      authorityID,
		AuthorityName:    authorityName,
		AuthorityType:    authorityType,
		BaseURL:          baseURL,
		PointerID:        pointerID,
		PointerType:      pointerType,
		Locator:          locator,
		PointerDataJSON:  pointerData,
		LastVerifiedTime: lastVerifiedTime,
	}

	return result, nil
}

// ResolveAuthorityPointerByPath resolves an authority pointer by authority
// name and locator path instead of by ID. Useful when the caller knows
// the authority and resource path but not the pointer row ID.
func ResolveAuthorityPointerByPath(db *pg.DB, authorityName string, locator string) (*ResolveResult, error) {
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

	return ResolveAuthorityPointer(db, pointerID)
}

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

	// try common field names in priority order
	for _, field := range []string{"base_url", "url", "endpoint", "address", "server_url"} {
		if val, ok := data[field].(string); ok && val != "" {
			return val
		}
	}

	// authority-type-specific fields
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
