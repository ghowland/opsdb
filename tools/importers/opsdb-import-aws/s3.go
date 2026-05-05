
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


