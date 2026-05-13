// === importers/opsdb_import_aws/route53.go ===
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// ImportRoute53 reads Route53 hosted zones from AWS API. Route53 is global.
func ImportRoute53(importCfg *ImportConfig) ([]Observation, error) {
	var results []Observation

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return results, fmt.Errorf("loading aws config for route53: %w", err)
	}

	client := route53.NewFromConfig(cfg)

	var marker *string
	for {
		input := &route53.ListHostedZonesInput{
			MaxItems: aws.Int32(int32(importCfg.BatchSize)),
		}
		if marker != nil {
			input.Marker = marker
		}

		resp, err := client.ListHostedZones(ctx, input)
		if err != nil {
			return results, fmt.Errorf("route53 ListHostedZones: %w", err)
		}

		for _, zone := range resp.HostedZones {
			zoneID := aws.ToString(zone.Id)
			// strip the /hostedzone/ prefix that AWS returns
			zoneID = strings.TrimPrefix(zoneID, "/hostedzone/")

			zoneName := aws.ToString(zone.Name)
			isPrivate := zone.Config != nil && zone.Config.PrivateZone

			var comment string
			if zone.Config != nil {
				comment = aws.ToString(zone.Config.Comment)
			}

			recordCount := int(aws.ToInt64(zone.ResourceRecordSetCount))

			// get VPC associations for private zones
			var vpcAssociations []map[string]string
			if isPrivate {
				vpcAssociations = getZoneVPCAssociations(client, ctx, zoneID)
			}

			// get tags
			tags := getZoneTags(client, ctx, zoneID)

			obs := MapRoute53Zone(
				zoneID,
				zoneName,
				isPrivate,
				recordCount,
				comment,
				aws.ToString(zone.CallerReference),
				vpcAssociations,
				tags,
			)
			results = append(results, obs)
		}

		if !resp.IsTruncated {
			break
		}
		marker = resp.NextMarker
	}

	return results, nil
}

// getZoneVPCAssociations retrieves VPC associations for a private hosted zone.
func getZoneVPCAssociations(client *route53.Client, ctx context.Context, zoneID string) []map[string]string {
	resp, err := client.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: aws.String(zoneID),
	})
	if err != nil {
		return nil
	}

	associations := make([]map[string]string, 0, len(resp.VPCs))
	for _, vpc := range resp.VPCs {
		associations = append(associations, map[string]string{
			"vpc_id":     aws.ToString(vpc.VPCId),
			"vpc_region": string(vpc.VPCRegion),
		})
	}
	return associations
}

// getZoneTags retrieves tags for a hosted zone.
func getZoneTags(client *route53.Client, ctx context.Context, zoneID string) map[string]string {
	resp, err := client.ListTagsForResource(ctx, &route53.ListTagsForResourceInput{
		ResourceId:   aws.String(zoneID),
		ResourceType: "hostedzone",
	})
	if err != nil {
		return nil
	}

	if resp.ResourceTagSet == nil {
		return nil
	}

	tags := make(map[string]string, len(resp.ResourceTagSet.Tags))
	for _, tag := range resp.ResourceTagSet.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tags
}
