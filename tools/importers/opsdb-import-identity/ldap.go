
// === importers/opsdb-import-identity/ldap.go ===
package identity

// ImportLDAP reads users, groups, and memberships from LDAP directory.
func ImportLDAP(config *IdentityImportConfig) ([]IdentityObservation, error) {
	// TODO: connect to LDAP server (TLS)
	// TODO: search user base DN for user entries
	// TODO: for each user:
	//   extract uid, cn, mail, accountStatus
	//   create ops_user observation
	// TODO: search group base DN for group entries
	// TODO: for each group:
	//   extract cn, description
	//   create ops_group observation
	//   extract member/memberUid attributes
	//   for each member: create ops_group_member observation
	// TODO: handle pagination via paged results control
	return nil, nil
}

