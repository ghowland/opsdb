// === importers/opsdb_import_aws/vpc.go ===
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ImportVPC reads VPCs, subnets, and security groups from AWS API for all configured regions.
func ImportVPC(importCfg *ImportConfig) ([]Observation, error) {
	var results []Observation

	for _, region := range importCfg.Regions {
		regionObs, err := importVPCRegion(importCfg, region)
		if err != nil {
			return results, fmt.Errorf("vpc import region %s: %w", region, err)
		}
		results = append(results, regionObs...)
	}

	return results, nil
}

func importVPCRegion(importCfg *ImportConfig, region string) ([]Observation, error) {
	var results []Observation

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return results, fmt.Errorf("loading aws config for %s: %w", region, err)
	}

	client := ec2.NewFromConfig(cfg)

	// import VPCs
	vpcs, err := importVPCs(client, ctx, region, importCfg.BatchSize)
	if err != nil {
		return results, fmt.Errorf("vpcs in %s: %w", region, err)
	}
	results = append(results, vpcs...)

	// import subnets
	subnets, err := importSubnets(client, ctx, region, importCfg.BatchSize)
	if err != nil {
		return results, fmt.Errorf("subnets in %s: %w", region, err)
	}
	results = append(results, subnets...)

	// import security groups as independent cloud_resource observations
	securityGroups, err := importSecurityGroups(client, ctx, region, importCfg.BatchSize)
	if err != nil {
		return results, fmt.Errorf("security groups in %s: %w", region, err)
	}
	results = append(results, securityGroups...)

	return results, nil
}

func importVPCs(client *ec2.Client, ctx context.Context, region string, batchSize int) ([]Observation, error) {
	var results []Observation

	paginator := ec2.NewDescribeVpcsPaginator(client, &ec2.DescribeVpcsInput{
		MaxResults: aws.Int32(int32(batchSize)),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return results, fmt.Errorf("DescribeVpcs page: %w", err)
		}

		for _, vpc := range page.Vpcs {
			vpcID := aws.ToString(vpc.VpcId)
			tags := extractEC2Tags(vpc.Tags)

			additionalCIDRs := make([]string, 0)
			primaryCIDR := aws.ToString(vpc.CidrBlock)
			for _, assoc := range vpc.CidrBlockAssociationSet {
				cidr := aws.ToString(assoc.CidrBlock)
				if cidr != primaryCIDR {
					additionalCIDRs = append(additionalCIDRs, cidr)
				}
			}

			// count subnets for this VPC
			subnetResp, err := client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
				Filters: []ec2types.Filter{
					{Name: aws.String("vpc-id"), Values: []string{vpcID}},
				},
			})
			var subnetCount int
			var subnetIDs []string
			if err == nil {
				subnetCount = len(subnetResp.Subnets)
				subnetIDs = make([]string, 0, subnetCount)
				for _, s := range subnetResp.Subnets {
					subnetIDs = append(subnetIDs, aws.ToString(s.SubnetId))
				}
			}

			// read DNS attributes
			dnsSupport := getVPCAttribute(client, ctx, vpcID, ec2types.VpcAttributeNameEnableDnsSupport)
			dnsHostnames := getVPCAttribute(client, ctx, vpcID, ec2types.VpcAttributeNameEnableDnsHostnames)

			obs := MapVPC(
				vpcID,
				primaryCIDR,
				additionalCIDRs,
				aws.ToBool(vpc.IsDefault),
				string(vpc.State),
				dnsSupport,
				dnsHostnames,
				string(vpc.InstanceTenancy),
				region,
				aws.ToString(vpc.OwnerId),
				subnetCount,
				subnetIDs,
				tags,
			)
			results = append(results, obs)
		}
	}

	return results, nil
}

func importSubnets(client *ec2.Client, ctx context.Context, region string, batchSize int) ([]Observation, error) {
	var results []Observation

	paginator := ec2.NewDescribeSubnetsPaginator(client, &ec2.DescribeSubnetsInput{
		MaxResults: aws.Int32(int32(batchSize)),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return results, fmt.Errorf("DescribeSubnets page: %w", err)
		}

		for _, subnet := range page.Subnets {
			subnetID := aws.ToString(subnet.SubnetId)
			tags := extractEC2Tags(subnet.Tags)

			name := ""
			if v, ok := tags["Name"]; ok {
				name = v
			}
			if name == "" {
				name = subnetID
			}

			var availableIPs int
			if subnet.AvailableIpAddressCount != nil {
				availableIPs = int(*subnet.AvailableIpAddressCount)
			}

			obs := Observation{
				EntityType: "cloud_resource",
				EntityID:   subnetID,
				StateKey:   "aws_subnet",
				Value:      name,
				DataJSON: map[string]interface{}{
					"cloud_resource_type":     "cloud_network",
					"subnet_id":               subnetID,
					"name":                    name,
					"vpc_id":                  aws.ToString(subnet.VpcId),
					"cidr_block":              aws.ToString(subnet.CidrBlock),
					"availability_zone":       aws.ToString(subnet.AvailabilityZone),
					"availability_zone_id":    aws.ToString(subnet.AvailabilityZoneId),
					"state":                   string(subnet.State),
					"available_ip_count":      availableIPs,
					"default_for_az":          aws.ToBool(subnet.DefaultForAz),
					"map_public_ip_on_launch": aws.ToBool(subnet.MapPublicIpOnLaunch),
					"assign_ipv6_on_creation": aws.ToBool(subnet.AssignIpv6AddressOnCreation),
					"owner_id":                aws.ToString(subnet.OwnerId),
					"region":                  region,
					"tags":                    tags,
				},
			}
			results = append(results, obs)
		}
	}

	return results, nil
}

func importSecurityGroups(client *ec2.Client, ctx context.Context, region string, batchSize int) ([]Observation, error) {
	var results []Observation

	paginator := ec2.NewDescribeSecurityGroupsPaginator(client, &ec2.DescribeSecurityGroupsInput{
		MaxResults: aws.Int32(int32(batchSize)),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return results, fmt.Errorf("DescribeSecurityGroups page: %w", err)
		}

		for _, sg := range page.SecurityGroups {
			sgID := aws.ToString(sg.GroupId)
			sgName := aws.ToString(sg.GroupName)
			tags := extractEC2Tags(sg.Tags)

			ingressRules := summarizeSGRules(sg.IpPermissions, "ingress")
			egressRules := summarizeSGRules(sg.IpPermissionsEgress, "egress")

			displayName := sgName
			if v, ok := tags["Name"]; ok && v != "" {
				displayName = v
			}

			obs := Observation{
				EntityType: "cloud_resource",
				EntityID:   sgID,
				StateKey:   "aws_security_group",
				Value:      displayName,
				DataJSON: map[string]interface{}{
					"cloud_resource_type": "vpc",
					"security_group_id":   sgID,
					"name":                sgName,
					"display_name":        displayName,
					"description":         aws.ToString(sg.Description),
					"vpc_id":              aws.ToString(sg.VpcId),
					"owner_id":            aws.ToString(sg.OwnerId),
					"region":              region,
					"ingress_rule_count":  len(ingressRules),
					"egress_rule_count":   len(egressRules),
					"ingress_rules":       ingressRules,
					"egress_rules":        egressRules,
					"tags":                tags,
				},
			}
			results = append(results, obs)
		}
	}

	return results, nil
}

// summarizeSGRules converts EC2 IP permissions into a slice of rule summaries.
func summarizeSGRules(permissions []ec2types.IpPermission, direction string) []map[string]interface{} {
	if len(permissions) == 0 {
		return nil
	}

	rules := make([]map[string]interface{}, 0, len(permissions))
	for _, perm := range permissions {
		rule := map[string]interface{}{
			"direction": direction,
			"protocol":  aws.ToString(perm.IpProtocol),
		}

		if perm.FromPort != nil {
			rule["from_port"] = int(*perm.FromPort)
		}
		if perm.ToPort != nil {
			rule["to_port"] = int(*perm.ToPort)
		}

		cidrs := make([]string, 0)
		for _, r := range perm.IpRanges {
			cidrs = append(cidrs, aws.ToString(r.CidrIp))
		}
		for _, r := range perm.Ipv6Ranges {
			cidrs = append(cidrs, aws.ToString(r.CidrIpv6))
		}
		if len(cidrs) > 0 {
			rule["cidrs"] = cidrs
		}

		sgRefs := make([]string, 0)
		for _, ref := range perm.UserIdGroupPairs {
			sgRefs = append(sgRefs, aws.ToString(ref.GroupId))
		}
		if len(sgRefs) > 0 {
			rule["security_group_refs"] = sgRefs
		}

		prefixLists := make([]string, 0)
		for _, pl := range perm.PrefixListIds {
			prefixLists = append(prefixLists, aws.ToString(pl.PrefixListId))
		}
		if len(prefixLists) > 0 {
			rule["prefix_lists"] = prefixLists
		}

		rules = append(rules, rule)
	}

	return rules
}

// getVPCAttribute reads a single VPC attribute (DNS support or DNS hostnames).
func getVPCAttribute(client *ec2.Client, ctx context.Context, vpcID string, attr ec2types.VpcAttributeName) bool {
	resp, err := client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
		VpcId:     aws.String(vpcID),
		Attribute: attr,
	})
	if err != nil {
		return false
	}

	switch attr {
	case ec2types.VpcAttributeNameEnableDnsSupport:
		if resp.EnableDnsSupport != nil {
			return aws.ToBool(resp.EnableDnsSupport.Value)
		}
	case ec2types.VpcAttributeNameEnableDnsHostnames:
		if resp.EnableDnsHostnames != nil {
			return aws.ToBool(resp.EnableDnsHostnames.Value)
		}
	}
	return false
}

// extractEC2Tags converts EC2 tags to a string map.
func extractEC2Tags(tags []ec2types.Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	result := make(map[string]string, len(tags))
	for _, tag := range tags {
		result[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return result
}
