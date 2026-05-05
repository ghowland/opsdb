
// === importers/opsdb-import-aws/route53.go ===
package aws

// ImportRoute53 reads Route53 hosted zones from AWS API.
// Route53 is global. Handles pagination.
func ImportRoute53(config *ImportConfig) ([]Observation, error) {
	// TODO: paginate ListHostedZones
	// TODO: for each zone:
	//   GetHostedZone for detail
	//   ListResourceRecordSets to count records
	//   call MapRoute53Zone
	//   append to results
	return nil, nil
}

