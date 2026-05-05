

// === importers/opsdb-import-aws/iam.go ===
package aws

// ImportIAM reads IAM roles from AWS API.
// IAM is global (not regional). Handles pagination.
func ImportIAM(config *ImportConfig) ([]Observation, error) {
	// TODO: paginate ListRoles
	// TODO: for each role:
	//   ListAttachedRolePolicies for attached policy count
	//   GetRole for trust policy document
	//   summarize trust policy (principals, not full JSON)
	//   call MapIAMRole
	//   append to results
	// TODO: optionally import IAM users if configured
	return nil, nil
}

