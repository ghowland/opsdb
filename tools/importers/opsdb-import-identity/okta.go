
// === importers/opsdb-import-identity/okta.go ===
package identity

// IdentityObservation is the observation structure for identity importers.
type IdentityObservation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// ImportOkta reads users, groups, and memberships from Okta API.
func ImportOkta(config *IdentityImportConfig) ([]IdentityObservation, error) {
	// TODO: paginate list users
	// TODO: for each user:
	//   extract login, email, display name, status (active, suspended, deprovisioned)
	//   extract last login time
	//   create ops_user observation
	// TODO: paginate list groups
	// TODO: for each group:
	//   extract name, description
	//   create ops_group observation
	//   paginate group members
	//   for each member: create ops_group_member observation
	return nil, nil
}

// IdentityImportConfig holds identity importer configuration.
type IdentityImportConfig struct {
	ProviderType string // okta, azuread, ldap
	BatchSize    int
	MaxRetries   int
}

