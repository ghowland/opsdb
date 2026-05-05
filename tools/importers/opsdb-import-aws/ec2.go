
// === importers/opsdb-import-aws/ec2.go ===
package aws

// ImportEC2 reads EC2 instances from AWS API for configured regions.
// Handles pagination, rate limiting, and multi-region scanning.
// Returns observations ready for OpsDB write.
func ImportEC2(config *ImportConfig) ([]Observation, error) {
	// TODO: for each region in config.Regions:
	//   create EC2 client for region
	//   paginate DescribeInstances
	//   for each reservation, for each instance:
	//     call MapEC2Instance to get observations
	//     append to results
	// TODO: respect rate limits via backoff
	// TODO: record per-region counts for runner_job summary
	return nil, nil
}

// ImportConfig holds the configuration for an AWS import cycle.
type ImportConfig struct {
	Regions      []string
	BatchSize    int
	MaxRetries   int
	// TODO: AWS credentials resolved from secret backend at runtime
	// TODO: authority_id for observation attribution
}

