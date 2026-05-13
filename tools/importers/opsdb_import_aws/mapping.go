// === importers/opsdb_import_aws/mapping.go ===
package main

// mapping.go defines the mapping from AWS API data structures to OpsDB schema
// entities and observation cache keys. Central place for all DSNC flattening decisions.
//
// Flattening rules per DSNC:
//   - Per-row metadata of parent (instance_type, ami_id, engine, version) → flat fields in DataJSON
//   - N-per-resource items with independent identity (security groups, EBS volumes) → separate observations
//   - Lists of simple values (tags, policy names) → kept as lists in DataJSON

// MapRDSInstance maps an AWS RDS instance API response to an Observation.
// Per-row metadata: engine, version, class, storage, multi-az → flat fields.
func MapRDSInstance(
	identifier string,
	engine string,
	engineVersion string,
	instanceClass string,
	allocatedStorageGB int,
	multiAZ bool,
	status string,
	endpointAddress string,
	endpointPort int,
	vpcID string,
	subnetGroupName string,
	availabilityZone string,
	region string,
	arn string,
	storageType string,
	storageEncrypted bool,
	kmsKeyID string,
	publiclyAccessible bool,
	autoMinorUpgrade bool,
	backupRetentionDays int,
	preferredBackupWindow string,
	preferredMaintenanceWindow string,
	parameterGroupName string,
	optionGroupName string,
	caCertificateID string,
	iamAuthEnabled bool,
	deletionProtection bool,
	performanceInsightsEnabled bool,
	monitoringInterval int,
	sgIDs []string,
	sgNames []string,
	tags map[string]string,
	createTime string,
) Observation {
	name := ""
	if v, ok := tags["Name"]; ok {
		name = v
	}
	if name == "" {
		name = identifier
	}

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   identifier,
		StateKey:   "aws_rds_instance",
		Value:      name,
		DataJSON: map[string]interface{}{
			"cloud_resource_type":          "rds_database",
			"db_instance_identifier":       identifier,
			"name":                         name,
			"engine":                       engine,
			"engine_version":               engineVersion,
			"instance_class":               instanceClass,
			"allocated_storage_gb":         allocatedStorageGB,
			"storage_type":                 storageType,
			"storage_encrypted":            storageEncrypted,
			"kms_key_id":                   kmsKeyID,
			"multi_az":                     multiAZ,
			"status":                       status,
			"endpoint_address":             endpointAddress,
			"endpoint_port":                endpointPort,
			"vpc_id":                       vpcID,
			"subnet_group_name":            subnetGroupName,
			"availability_zone":            availabilityZone,
			"region":                       region,
			"arn":                          arn,
			"publicly_accessible":          publiclyAccessible,
			"auto_minor_version_upgrade":   autoMinorUpgrade,
			"backup_retention_days":        backupRetentionDays,
			"preferred_backup_window":      preferredBackupWindow,
			"preferred_maintenance_window": preferredMaintenanceWindow,
			"parameter_group_name":         parameterGroupName,
			"option_group_name":            optionGroupName,
			"ca_certificate_id":            caCertificateID,
			"iam_auth_enabled":             iamAuthEnabled,
			"deletion_protection":          deletionProtection,
			"performance_insights_enabled": performanceInsightsEnabled,
			"monitoring_interval":          monitoringInterval,
			"security_group_ids":           sgIDs,
			"security_group_names":         sgNames,
			"security_group_count":         len(sgIDs),
			"tags":                         tags,
			"created_time":                 createTime,
		},
	}
}

// MapS3Bucket maps an AWS S3 bucket to an Observation.
// Per-row metadata: region, versioning, encryption → flat fields.
func MapS3Bucket(
	name string,
	region string,
	versioningEnabled bool,
	versioningStatus string,
	encryptionAlgorithm string,
	encryptionKMSKeyID string,
	publicAccessBlockEnabled bool,
	blockPublicAcls bool,
	blockPublicPolicy bool,
	ignorePublicAcls bool,
	restrictPublicBuckets bool,
	lifecycleRuleCount int,
	createTime string,
	tags map[string]string,
) Observation {
	return Observation{
		EntityType: "cloud_resource",
		EntityID:   name,
		StateKey:   "aws_s3_bucket",
		Value:      name,
		DataJSON: map[string]interface{}{
			"cloud_resource_type":     "s3_bucket",
			"bucket_name":             name,
			"region":                  region,
			"versioning_enabled":      versioningEnabled,
			"versioning_status":       versioningStatus,
			"encryption_algorithm":    encryptionAlgorithm,
			"encryption_kms_key_id":   encryptionKMSKeyID,
			"public_access_block":     publicAccessBlockEnabled,
			"block_public_acls":       blockPublicAcls,
			"block_public_policy":     blockPublicPolicy,
			"ignore_public_acls":      ignorePublicAcls,
			"restrict_public_buckets": restrictPublicBuckets,
			"lifecycle_rule_count":    lifecycleRuleCount,
			"tags":                    tags,
			"created_time":            createTime,
		},
	}
}

// MapVPC maps an AWS VPC to an Observation.
func MapVPC(
	vpcID string,
	cidrBlock string,
	additionalCIDRs []string,
	isDefault bool,
	state string,
	dnsSupport bool,
	dnsHostnames bool,
	tenancy string,
	region string,
	ownerID string,
	subnetCount int,
	subnetIDs []string,
	tags map[string]string,
) Observation {
	name := ""
	if v, ok := tags["Name"]; ok {
		name = v
	}
	if name == "" {
		name = vpcID
	}

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   vpcID,
		StateKey:   "aws_vpc",
		Value:      name,
		DataJSON: map[string]interface{}{
			"cloud_resource_type": "vpc",
			"vpc_id":              vpcID,
			"name":                name,
			"cidr_block":          cidrBlock,
			"additional_cidrs":    additionalCIDRs,
			"is_default":          isDefault,
			"state":               state,
			"dns_support":         dnsSupport,
			"dns_hostnames":       dnsHostnames,
			"tenancy":             tenancy,
			"region":              region,
			"owner_id":            ownerID,
			"subnet_count":        subnetCount,
			"subnet_ids":          subnetIDs,
			"tags":                tags,
		},
	}
}

// MapRoute53Zone maps an AWS Route53 hosted zone to an Observation.
func MapRoute53Zone(
	zoneID string,
	name string,
	isPrivate bool,
	recordCount int,
	comment string,
	callerReference string,
	vpcAssociations []map[string]string,
	tags map[string]string,
) Observation {
	displayName := name
	if v, ok := tags["Name"]; ok && v != "" {
		displayName = v
	}

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   zoneID,
		StateKey:   "aws_route53_zone",
		Value:      displayName,
		DataJSON: map[string]interface{}{
			"cloud_resource_type": "route53_zone",
			"zone_id":             zoneID,
			"name":                name,
			"display_name":        displayName,
			"is_private":          isPrivate,
			"record_count":        recordCount,
			"comment":             comment,
			"caller_reference":    callerReference,
			"vpc_associations":    vpcAssociations,
			"vpc_count":           len(vpcAssociations),
			"tags":                tags,
		},
	}
}

// CloudResourceType returns the OpsDB cloud_resource_type discriminator value
// for an AWS resource type string.
func CloudResourceType(awsResourceType string) string {
	switch awsResourceType {
	case "AWS::EC2::Instance":
		return "ec2_instance"
	case "AWS::RDS::DBInstance":
		return "rds_database"
	case "AWS::S3::Bucket":
		return "s3_bucket"
	case "AWS::IAM::Role":
		return "iam_role"
	case "AWS::EC2::VPC":
		return "vpc"
	case "AWS::Route53::HostedZone":
		return "route53_zone"
	case "AWS::EC2::SecurityGroup":
		return "vpc"
	case "AWS::EC2::Subnet":
		return "vpc"
	case "AWS::Lambda::Function":
		return "lambda_function"
	case "AWS::CloudFront::Distribution":
		return "cloudfront_distribution"
	case "AWS::CloudWatch::LogGroup":
		return "cloudwatch_log_group"
	case "AWS::ElasticLoadBalancingV2::LoadBalancer":
		return "load_balancer"
	default:
		return "ec2_instance"
	}
}
