// === importers/opsdb_import_aws/s3.go ===
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// ImportS3 reads S3 buckets from AWS API. S3 is global but buckets have regions.
func ImportS3(importCfg *ImportConfig) ([]Observation, error) {
	var results []Observation

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return results, fmt.Errorf("loading aws config for s3: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	listResp, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return results, fmt.Errorf("s3 ListBuckets: %w", err)
	}

	regionFilter := make(map[string]bool, len(importCfg.Regions))
	for _, r := range importCfg.Regions {
		regionFilter[r] = true
	}
	filterByRegion := len(importCfg.Regions) > 0

	for _, bucket := range listResp.Buckets {
		bucketName := aws.ToString(bucket.Name)

		// determine bucket region
		bucketRegion := getBucketRegion(client, ctx, bucketName)
		if filterByRegion && !regionFilter[bucketRegion] {
			continue
		}

		// create a region-specific client for per-bucket API calls
		regionCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(bucketRegion))
		if err != nil {
			results = append(results, bucketErrorObservation(bucketName, bucketRegion, fmt.Sprintf("config for region %s: %v", bucketRegion, err)))
			continue
		}
		regionClient := s3.NewFromConfig(regionCfg)

		// get versioning
		versioningEnabled, versioningStatus := getBucketVersioning(regionClient, ctx, bucketName)

		// get encryption
		encryptionAlgorithm, encryptionKMSKey := getBucketEncryption(regionClient, ctx, bucketName)

		// get public access block
		publicAccessEnabled, blockPublicAcls, blockPublicPolicy, ignorePublicAcls, restrictPublicBuckets := getBucketPublicAccess(regionClient, ctx, bucketName)

		// get lifecycle rule count
		lifecycleRuleCount := getBucketLifecycleRuleCount(regionClient, ctx, bucketName)

		// get tags
		tags := getBucketTags(regionClient, ctx, bucketName)

		var createTime string
		if bucket.CreationDate != nil {
			createTime = bucket.CreationDate.Format(time.RFC3339)
		}

		obs := MapS3Bucket(
			bucketName,
			bucketRegion,
			versioningEnabled,
			versioningStatus,
			encryptionAlgorithm,
			encryptionKMSKey,
			publicAccessEnabled,
			blockPublicAcls,
			blockPublicPolicy,
			ignorePublicAcls,
			restrictPublicBuckets,
			lifecycleRuleCount,
			createTime,
			tags,
		)
		results = append(results, obs)
	}

	return results, nil
}

func getBucketRegion(client *s3.Client, ctx context.Context, bucketName string) string {
	resp, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return "us-east-1"
	}

	location := string(resp.LocationConstraint)
	if location == "" {
		// empty location means us-east-1
		return "us-east-1"
	}
	return location
}

func getBucketVersioning(client *s3.Client, ctx context.Context, bucketName string) (bool, string) {
	resp, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return false, ""
	}

	status := string(resp.Status)
	enabled := resp.Status == s3types.BucketVersioningStatusEnabled
	return enabled, status
}

func getBucketEncryption(client *s3.Client, ctx context.Context, bucketName string) (string, string) {
	resp, err := client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		if isS3NotFound(err) || isS3AccessDenied(err) {
			return "", ""
		}
		return "", ""
	}

	if resp.ServerSideEncryptionConfiguration == nil || len(resp.ServerSideEncryptionConfiguration.Rules) == 0 {
		return "", ""
	}

	rule := resp.ServerSideEncryptionConfiguration.Rules[0]
	if rule.ApplyServerSideEncryptionByDefault == nil {
		return "", ""
	}

	algorithm := string(rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm)
	kmsKey := aws.ToString(rule.ApplyServerSideEncryptionByDefault.KMSMasterKeyID)
	return algorithm, kmsKey
}

func getBucketPublicAccess(client *s3.Client, ctx context.Context, bucketName string) (bool, bool, bool, bool, bool) {
	resp, err := client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		if isS3NotFound(err) || isS3AccessDenied(err) {
			return false, false, false, false, false
		}
		return false, false, false, false, false
	}

	if resp.PublicAccessBlockConfiguration == nil {
		return false, false, false, false, false
	}

	cfg := resp.PublicAccessBlockConfiguration
	blockAcls := aws.ToBool(cfg.BlockPublicAcls)
	blockPolicy := aws.ToBool(cfg.BlockPublicPolicy)
	ignoreAcls := aws.ToBool(cfg.IgnorePublicAcls)
	restrictBuckets := aws.ToBool(cfg.RestrictPublicBuckets)

	allEnabled := blockAcls && blockPolicy && ignoreAcls && restrictBuckets
	return allEnabled, blockAcls, blockPolicy, ignoreAcls, restrictBuckets
}

func getBucketLifecycleRuleCount(client *s3.Client, ctx context.Context, bucketName string) int {
	resp, err := client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return 0
	}
	return len(resp.Rules)
}

func getBucketTags(client *s3.Client, ctx context.Context, bucketName string) map[string]string {
	resp, err := client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return nil
	}

	tags := make(map[string]string, len(resp.TagSet))
	for _, tag := range resp.TagSet {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tags
}

func bucketErrorObservation(bucketName string, region string, errMsg string) Observation {
	return Observation{
		EntityType: "cloud_resource",
		EntityID:   bucketName,
		StateKey:   "aws_s3_bucket_error",
		Value:      fmt.Sprintf("failed to import: %s", errMsg),
		DataJSON: map[string]interface{}{
			"cloud_resource_type": "s3_bucket",
			"bucket_name":         bucketName,
			"region":              region,
			"error":               errMsg,
		},
	}
}

// isS3AccessDenied checks if an error is an S3 access denied response.
func isS3AccessDenied(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		return ae.ErrorCode() == "AccessDenied" || ae.ErrorCode() == "Forbidden"
	}
	return false
}

// isS3NotFound checks if an error is an S3 not found response
// (NoSuchBucketPolicy, NoSuchConfiguration, etc).
func isS3NotFound(err error) bool {
	var ae smithy.APIError
	if errors.As(err, &ae) {
		code := ae.ErrorCode()
		return code == "NoSuchBucketPolicy" ||
			code == "NoSuchConfiguration" ||
			code == "NoSuchLifecycleConfiguration" ||
			code == "NoSuchTagSet" ||
			code == "NoSuchPublicAccessBlockConfiguration" ||
			code == "ServerSideEncryptionConfigurationNotFoundError"
	}
	return false
}
