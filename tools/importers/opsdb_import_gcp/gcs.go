package gcp

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

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
	Name                   string
	Location               string
	LocationType           string
	StorageClass           string
	VersioningEnabled      bool
	LifecycleRuleCount     int
	EncryptionType         string
	CMEKKeyName            string
	UniformBucketAccess    bool
	PublicAccessPrevention  string
	RetentionPolicyDays    int
	LoggingEnabled         bool
	CreationTime           string
	SelfLink               string
}

// listGCSBuckets reads all GCS buckets in a project and extracts per-bucket details.
func listGCSBuckets(project string) ([]gcsBucket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating storage client: %w", err)
	}
	defer client.Close()

	var buckets []gcsBucket

	it := client.Buckets(ctx, project)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return buckets, fmt.Errorf("iterating buckets: %w", err)
		}

		b := gcsBucket{
			Name:         attrs.Name,
			Location:     attrs.Location,
			LocationType: attrs.LocationType,
			StorageClass: attrs.StorageClass,
			CreationTime: attrs.Created.Format(time.RFC3339),
		}

		// Versioning.
		b.VersioningEnabled = attrs.VersioningEnabled

		// Lifecycle rules — count only, individual rules are bucket config detail.
		b.LifecycleRuleCount = len(attrs.Lifecycle.Rules)

		// Encryption type.
		if attrs.Encryption != nil && attrs.Encryption.DefaultKMSKeyName != "" {
			b.EncryptionType = "cmek"
			b.CMEKKeyName = attrs.Encryption.DefaultKMSKeyName
		} else {
			b.EncryptionType = "google_managed"
		}

		// Uniform bucket-level access.
		b.UniformBucketAccess = attrs.UniformBucketLevelAccess.Enabled

		// Public access prevention.
		switch attrs.PublicAccessPrevention {
		case storage.PublicAccessPreventionEnforced:
			b.PublicAccessPrevention = "enforced"
		case storage.PublicAccessPreventionInherited:
			b.PublicAccessPrevention = "inherited"
		default:
			b.PublicAccessPrevention = "unspecified"
		}

		// Retention policy — convert duration to days.
		if attrs.RetentionPolicy != nil && attrs.RetentionPolicy.RetentionPeriod > 0 {
			b.RetentionPolicyDays = int(attrs.RetentionPolicy.RetentionPeriod.Hours() / 24)
		}

		// Logging.
		b.LoggingEnabled = attrs.Logging != nil && attrs.Logging.LogBucket != ""

		// SelfLink is not directly on BucketAttrs; construct from name.
		b.SelfLink = fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s", attrs.Name)

		buckets = append(buckets, b)
	}

	return buckets, nil
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
		"encryption_type":         bucket.EncryptionType,
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
