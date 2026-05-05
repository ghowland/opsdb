
// === importers/opsdb-import-aws/rds.go ===
package aws

// ImportRDS reads RDS instances from AWS API for configured regions.
// Handles pagination and rate limiting.
func ImportRDS(config *ImportConfig) ([]Observation, error) {
	// TODO: for each region in config.Regions:
	//   create RDS client for region
	//   paginate DescribeDBInstances
	//   for each instance:
	//     call MapRDSInstance to get observations
	//     append to results
	// TODO: also import read replicas and their primary linkage
	// TODO: record per-region counts
	return nil, nil
}

