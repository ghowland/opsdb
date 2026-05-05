package gcp

import "fmt"

// ImportGCS reads GCS buckets from the GCP API across all configured projects.
// For each bucket, fetches versioning, lifecycle, encryption, and access
// configuration. Maps each to an observation targeting cloud_resource with
// gcs_bucket discriminator type.
func ImportGCS(config *GCPImportConfig) ([]Observation, error) {
	var observations []Observation

	for _, project := range config.Projects {
		buckets, err := listGCSBuckets(project)
		if err != nil {
			return observations, fmt.Errorf("listing GCS buckets in project %s: %w", project, err)
		}

		for _, bucket := range buckets {
			obs := MapGCSBucket(project, bucket)
			observations = append(observations, obs)

			if len(observations) >= config.BatchSize {
				return observations, nil
			}
		}
	}

	return observations, nil
}

// gcsBucket holds the raw fields read from the GCP Cloud Storage API.
type gcsBucket struct {
	Name                  string
	Location              string
	LocationType          string // region, dual-region, multi-region
	StorageClass          string // STANDARD, NEARLINE, COLDLINE, ARCHIVE
	VersioningEnabled     bool
	LifecycleRuleCount    int
	EncryptionType        string // google-managed, cmek, csek
	CMEKKeyName           string // KMS key name if CMEK
	UniformBucketAccess   bool
	PublicAccessPrevention string // enforced, inherited
	RetentionPolicyDays   int    // 0 if no retention policy
	LoggingEnabled        bool
	CreationTime          string
	SelfLink              string
}

// listGCSBuckets reads all GCS buckets in a project and fetches per-bucket details.
func listGCSBuckets(project string) ([]gcsBucket, error) {
	// TODO: create storage service client using application default credentials
	// TODO: call buckets.List(project)
	// TODO: handle pagination via NextPageToken
	// TODO: for each bucket in response:
	//   extract Name, Location, LocationType, StorageClass,
	//   Versioning.Enabled,
	//   len(Lifecycle.Rule) for rule count,
	//   Encryption (determine type: default = google-managed,
	//     DefaultKmsKeyName set = cmek),
	//   IamConfiguration.UniformBucketLevelAccess.Enabled,
	//   IamConfiguration.PublicAccessPrevention,
	//   RetentionPolicy.RetentionPeriod (convert seconds to days),
	//   Logging != nil for logging enabled,
	//   TimeCreated, SelfLink
	// TODO: handle AccessDenied gracefully per bucket
	// TODO: return buckets
	return nil, nil
}

// MapGCSBucket transforms a raw GCP Cloud Storage bucket into an OpsDB observation.
// Flattens per-bucket metadata into cloud_data_json. IAM bindings and lifecycle
// rules are summarized (counts) — individual rules would be separate entities
// if the org needs that granularity.
func MapGCSBucket(project string, bucket gcsBucket) Observation {
	dataJSON := map[string]interface{}{
		"project":                  project,
		"bucket_name":              bucket.Name,
		"location":                 bucket.Location,
		"location_type":            bucket.LocationType,
		"storage_class":            bucket.StorageClass,
		"is_versioning_enabled":    bucket.VersioningEnabled,
		"lifecycle_rule_count":     bucket.LifecycleRuleCount,
		"encryption_type":          bucket.EncryptionType,
		"is_uniform_bucket_access": bucket.UniformBucketAccess,
		"public_access_prevention": bucket.PublicAccessPrevention,
		"is_logging_enabled":       bucket.LoggingEnabled,
		"creation_time":            bucket.CreationTime,
		"self_link":                bucket.SelfLink,
	}

	if bucket.CMEKKeyName != "" {
		dataJSON["cmek_key_name"] = bucket.CMEKKeyName
	}

	if bucket.RetentionPolicyDays > 0 {
		dataJSON["retention_policy_days"] = bucket.RetentionPolicyDays
	}

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   fmt.Sprintf("gcp:%s:gcs:%s", project, bucket.Name),
		StateKey:   "gcs_bucket",
		Value:      bucket.StorageClass,
		DataJSON:   dataJSON,
	}
}
