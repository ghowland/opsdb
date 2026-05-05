
// === importers/opsdb-import-aws/vpc.go ===
package aws

// ImportVPC reads VPCs, subnets, and security groups from AWS API.
// Security groups are separate observations (not flattened into VPC or EC2)
// because they have independent lifecycle and are N-per-resource.
func ImportVPC(config *ImportConfig) ([]Observation, error) {
	// TODO: for each region in config.Regions:
	//   DescribeVpcs → map each to VPC observation
	//   DescribeSubnets → map each to subnet observation (linked to VPC)
	//   DescribeSecurityGroups → map each to security group observation
	// TODO: security groups are their own cloud_resource observations,
	//       not flattened into EC2 or VPC per DSNC list-of-N test
	return nil, nil
}

