
// === importers/opsdb-import-aws/mapping.go ===
package aws

// mapping.go defines the mapping from AWS API data structures to OpsDB schema
// entities and observation cache keys. Central place for all DSNC flattening decisions.

// ObservationType identifies which observation cache table to write to.
type ObservationType string

const (
	ObsState  ObservationType = "observation_cache_state"
	ObsConfig ObservationType = "observation_cache_config"
)

// Observation represents one observation ready to write to OpsDB.
type Observation struct {
	EntityType string          // "cloud_resource", "cloud_account", etc.
	EntityID   string          // external_id from AWS (ARN or resource ID)
	StateKey   string          // observation key: "ec2_instance_state", "rds_status", etc.
	Value      string          // scalar value
	DataJSON   map[string]interface{} // structured detail
	ObsType    ObservationType
}

// MapEC2Instance maps an AWS EC2 instance API response to an Observation.
// Flattens per-row metadata into cloud_data_json per DSNC rules:
// instance_type, ami_id, vpc_id, subnet_id are per-row metadata → flat fields.
// Security group memberships are N-per-instance → separate observations.
func MapEC2Instance(instance interface{}) []Observation {
	// TODO: extract instance ID, state, type, ami, vpc, subnet, IPs, launch time
	// TODO: create main instance observation with flat fields in DataJSON
	// TODO: create separate observations for each security group membership
	// TODO: create separate observations for each attached EBS volume (they are own cloud_resources)
	// TODO: return slice of observations
	return nil
}

// MapRDSInstance maps an AWS RDS instance to observations.
func MapRDSInstance(instance interface{}) []Observation {
	// TODO: extract identifier, engine, version, class, storage, multi-az, endpoint, params
	// TODO: flat fields per DSNC: engine, version, class, storage are per-row metadata
	// TODO: return observations
	return nil
}

// MapS3Bucket maps an AWS S3 bucket to observations.
func MapS3Bucket(bucket interface{}) []Observation {
	// TODO: extract name, region, versioning, encryption, public access, lifecycle rules count
	// TODO: return observations
	return nil
}

// MapIAMRole maps an AWS IAM role to observations.
func MapIAMRole(role interface{}) []Observation {
	// TODO: extract name, ARN, path, attached policies count, trust policy summary
	// TODO: trust policy stays in AWS; we store summary only
	// TODO: return observations
	return nil
}

// MapVPC maps an AWS VPC to observations.
func MapVPC(vpc interface{}) []Observation {
	// TODO: extract VPC ID, CIDR, default flag, DNS settings, tenancy
	// TODO: return observations
	return nil
}

// MapRoute53Zone maps an AWS Route53 hosted zone to observations.
func MapRoute53Zone(zone interface{}) []Observation {
	// TODO: extract zone ID, name, private flag, record count
	// TODO: return observations
	return nil
}

// CloudResourceType returns the OpsDB cloud_resource_type discriminator value
// for an AWS resource type string.
func CloudResourceType(awsResourceType string) string {
	// TODO: map "AWS::EC2::Instance" → "ec2_instance"
	// TODO: map "AWS::RDS::DBInstance" → "rds_database"
	// TODO: map "AWS::S3::Bucket" → "s3_bucket"
	// TODO: map "AWS::IAM::Role" → "iam_role"
	// TODO: map "AWS::EC2::VPC" → "vpc"
	// TODO: map "AWS::Route53::HostedZone" → "route53_zone"
	return ""
}

