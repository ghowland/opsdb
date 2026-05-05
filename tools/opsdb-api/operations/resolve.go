// === opsdb-api/operations/resolve.go ===
package operations

// ResolveAuthorityPointer performs a where-is-X lookup.
// Returns authority connection details and locator. Does NOT fetch from authority.
func ResolveAuthorityPointer(pointerID int) (*ResolveResult, error) {
	// TODO: read authority_pointer row by ID
	// TODO: read parent authority row for connection details
	// TODO: return ResolveResult with:
	//   authority base_url
	//   authority_type
	//   pointer_type
	//   locator (path/identifier within authority)
	//   pointer_data_json
	//   last_verified_time
	return nil, nil
}

// ResolveResult holds the result of an authority pointer resolution.
type ResolveResult struct {
	AuthorityID       int
	AuthorityName     string
	AuthorityType     string
	BaseURL           string
	PointerType       string
	Locator           string
	PointerDataJSON   map[string]interface{}
	LastVerifiedTime  interface{} // time.Time or nil
}


