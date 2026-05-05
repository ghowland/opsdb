
// === importers/opsdb-import-identity/azuread.go ===
package identity

// ImportAzureAD reads users, groups, and memberships from Azure AD / Entra ID.
func ImportAzureAD(config *IdentityImportConfig) ([]IdentityObservation, error) {
	// TODO: paginate list users via Microsoft Graph API
	// TODO: for each user:
	//   extract userPrincipalName, displayName, mail, accountEnabled
	//   create ops_user observation
	// TODO: paginate list groups
	// TODO: for each group:
	//   extract displayName, description
	//   create ops_group observation
	//   paginate group members
	//   for each member: create ops_group_member observation
	// TODO: handle guest users vs member users
	return nil, nil
}

