
// === importers/opsdb-import-aws/s3.go ===
package aws

// ImportS3 reads S3 buckets from AWS API.
// S3 is global but buckets have regions. Lists all buckets then
// fetches per-bucket details (versioning, encryption, lifecycle, public access).
func ImportS3(config *ImportConfig) ([]Observation, error) {
	// TODO: ListBuckets (global call)
	// TODO: for each bucket:
	//   GetBucketLocation to determine region
	//   if region not in config.Regions, skip (or include all if Regions empty)
	//   GetBucketVersioning
	//   GetBucketEncryption
	//   GetPublicAccessBlock
	//   GetBucketLifecycleConfiguration (count rules)
	//   call MapS3Bucket
	//   append to results
	// TODO: handle AccessDenied gracefully for per-bucket calls
	return nil, nil
}


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

