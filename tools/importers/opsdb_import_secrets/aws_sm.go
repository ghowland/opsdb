//# tools/importers/opsdb_import_secrets/aws_sm.go

package secrets

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

// AWSSecretsManagerConfig holds configuration for the AWS Secrets Manager importer.
type AWSSecretsManagerConfig struct {
	Region     string
	Regions    []string // import from multiple regions; empty = use Region only
	MaxResults int
}

// awsSecretEntry represents one secret's metadata from the ListSecrets response.
type awsSecretEntry struct {
	Name                 string
	ARN                  string
	Description          string
	CreatedDate          *time.Time
	LastChangedDate      *time.Time
	LastAccessedDate     *time.Time
	LastRotatedDate      *time.Time
	RotationEnabled      bool
	RotationLambdaARN    string
	RotationIntervalDays int
	Tags                 map[string]string
}

// ImportAWSSecretsManager reads secret metadata from AWS Secrets Manager.
// NEVER reads secret values — only names, ARNs, creation dates, rotation
// configuration, and last rotated timestamps.
func ImportAWSSecretsManager(config *AWSSecretsManagerConfig) ([]Observation, error) {
	if config.Region == "" && len(config.Regions) == 0 {
		return nil, fmt.Errorf("at least one region must be configured")
	}

	regions := config.Regions
	if len(regions) == 0 {
		regions = []string{config.Region}
	}

	maxResults := config.MaxResults
	if maxResults <= 0 {
		maxResults = 1000
	}

	var allObservations []Observation

	for _, region := range regions {
		observations, err := importSecretsFromRegion(region, maxResults)
		if err != nil {
			return allObservations, fmt.Errorf("AWS Secrets Manager import failed for region %s: %w", region, err)
		}
		allObservations = append(allObservations, observations...)
	}

	return allObservations, nil
}

// importSecretsFromRegion reads secret metadata from one AWS region.
func importSecretsFromRegion(region string, maxResults int) ([]Observation, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session for region %s: %w", region, err)
	}

	svc := secretsmanager.New(sess)

	entries, err := listSecrets(svc, maxResults)
	if err != nil {
		return nil, fmt.Errorf("ListSecrets failed in %s: %w", region, err)
	}

	observations := make([]Observation, 0, len(entries))
	now := time.Now().UTC()

	for _, entry := range entries {
		obs := mapAWSSecretToObservation(entry, region, now)
		observations = append(observations, obs)
	}

	return observations, nil
}

// listSecrets paginates through AWS Secrets Manager ListSecrets.
// NEVER calls GetSecretValue.
func listSecrets(svc *secretsmanager.SecretsManager, maxResults int) ([]awsSecretEntry, error) {
	var entries []awsSecretEntry

	input := &secretsmanager.ListSecretsInput{
		MaxResults: aws.Int64(100),
	}

	for {
		output, err := svc.ListSecrets(input)
		if err != nil {
			return entries, fmt.Errorf("ListSecrets API call failed: %w", err)
		}

		for _, secret := range output.SecretList {
			entry := awsSecretEntry{
				Name:             aws.StringValue(secret.Name),
				ARN:              aws.StringValue(secret.ARN),
				Description:      aws.StringValue(secret.Description),
				CreatedDate:      secret.CreatedDate,
				LastChangedDate:  secret.LastChangedDate,
				LastAccessedDate: secret.LastAccessedDate,
				LastRotatedDate:  secret.LastRotatedDate,
				RotationEnabled:  aws.BoolValue(secret.RotationEnabled),
			}

			if secret.RotationRules != nil && secret.RotationRules.AutomaticallyAfterDays != nil {
				entry.RotationIntervalDays = int(aws.Int64Value(secret.RotationRules.AutomaticallyAfterDays))
			}

			if secret.RotationLambdaARN != nil {
				entry.RotationLambdaARN = aws.StringValue(secret.RotationLambdaARN)
			}

			entry.Tags = awsTagsToMap(secret.Tags)

			entries = append(entries, entry)
		}

		if output.NextToken == nil || len(entries) >= maxResults {
			break
		}
		input.NextToken = output.NextToken
	}

	return entries, nil
}

// mapAWSSecretToObservation converts one AWS secret's metadata into an
// Observation. Computes rotation compliance from rotation config and
// last rotated timestamp.
func mapAWSSecretToObservation(entry awsSecretEntry, region string, now time.Time) Observation {
	obs := Observation{
		SecretPath:      entry.ARN,
		Engine:          "aws_secretsmanager",
		RotationEnabled: entry.RotationEnabled,
		Tags:            entry.Tags,
	}

	if obs.Tags == nil {
		obs.Tags = make(map[string]string)
	}
	obs.Tags["aws_region"] = region
	obs.Tags["aws_name"] = entry.Name

	// version: use last changed date as version indicator
	if entry.LastChangedDate != nil {
		obs.Version = entry.LastChangedDate.Format(time.RFC3339)
	}

	// last rotated time
	if entry.LastRotatedDate != nil {
		obs.LastRotatedTime = entry.LastRotatedDate.Format(time.RFC3339)
	}

	// expiration: compute from rotation interval if rotation is enabled
	if entry.RotationEnabled && entry.RotationIntervalDays > 0 && entry.LastRotatedDate != nil {
		nextRotation := entry.LastRotatedDate.Add(
			time.Duration(entry.RotationIntervalDays) * 24 * time.Hour)
		obs.ExpirationTime = nextRotation.Format(time.RFC3339)
	}

	// rotation compliance
	obs.RotationCompliance = computeAWSRotationCompliance(entry, now)

	return obs
}

// computeAWSRotationCompliance determines whether an AWS secret is in
// compliance with its rotation policy.
func computeAWSRotationCompliance(entry awsSecretEntry, now time.Time) string {
	if !entry.RotationEnabled {
		return "rotation_not_enabled"
	}

	if entry.RotationIntervalDays <= 0 {
		return "no_rotation_interval"
	}

	if entry.LastRotatedDate == nil {
		return "never_rotated"
	}

	daysSinceRotation := int(now.Sub(*entry.LastRotatedDate).Hours() / 24)

	if daysSinceRotation > entry.RotationIntervalDays {
		return fmt.Sprintf("overdue_by_%d_days", daysSinceRotation-entry.RotationIntervalDays)
	}

	if daysSinceRotation > entry.RotationIntervalDays-7 {
		return "rotation_due_soon"
	}

	return "compliant"
}

// awsTagsToMap converts AWS SDK tag slices to a simple string map.
func awsTagsToMap(tags []*secretsmanager.Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}

	result := make(map[string]string, len(tags))
	for _, tag := range tags {
		if tag.Key != nil && tag.Value != nil {
			result[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
		}
	}
	return result
}
