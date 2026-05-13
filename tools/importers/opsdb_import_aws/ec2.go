// === importers/opsdb-import-aws/ec2.go ===
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// ImportEC2 reads EC2 instances from AWS API for all configured regions.
func ImportEC2(importCfg *ImportConfig) ([]Observation, error) {
	var results []Observation

	for _, region := range importCfg.Regions {
		regionObs, err := importEC2Region(importCfg, region)
		if err != nil {
			return results, fmt.Errorf("ec2 import region %s: %w", region, err)
		}
		results = append(results, regionObs...)
	}

	return results, nil
}

func importEC2Region(importCfg *ImportConfig, region string) ([]Observation, error) {
	var results []Observation

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return results, fmt.Errorf("loading aws config for %s: %w", region, err)
	}

	client := ec2.NewFromConfig(cfg)

	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		MaxResults: aws.Int32(int32(importCfg.BatchSize)),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return results, fmt.Errorf("ec2 DescribeInstances page in %s: %w", region, err)
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				obs := mapEC2Instance(instance, region)
				results = append(results, obs)
			}
		}
	}

	return results, nil
}

func mapEC2Instance(instance types.Instance, region string) Observation {
	instanceID := aws.ToString(instance.InstanceId)

	name := ""
	tags := make(map[string]string, len(instance.Tags))
	for _, tag := range instance.Tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		tags[key] = value
		if key == "Name" {
			name = value
		}
	}

	var privateIP, publicIP, privateDNS, publicDNS string
	if instance.PrivateIpAddress != nil {
		privateIP = aws.ToString(instance.PrivateIpAddress)
	}
	if instance.PublicIpAddress != nil {
		publicIP = aws.ToString(instance.PublicIpAddress)
	}
	if instance.PrivateDnsName != nil {
		privateDNS = aws.ToString(instance.PrivateDnsName)
	}
	if instance.PublicDnsName != nil {
		publicDNS = aws.ToString(instance.PublicDnsName)
	}

	sgNames := make([]string, 0, len(instance.SecurityGroups))
	sgIDs := make([]string, 0, len(instance.SecurityGroups))
	for _, sg := range instance.SecurityGroups {
		sgNames = append(sgNames, aws.ToString(sg.GroupName))
		sgIDs = append(sgIDs, aws.ToString(sg.GroupId))
	}

	var launchTime string
	if instance.LaunchTime != nil {
		launchTime = instance.LaunchTime.Format(time.RFC3339)
	}

	var iamRole string
	if instance.IamInstanceProfile != nil {
		arn := aws.ToString(instance.IamInstanceProfile.Arn)
		// extract role name from arn:aws:iam::123456:instance-profile/role-name
		if idx := lastIndex(arn, '/'); idx >= 0 {
			iamRole = arn[idx+1:]
		} else {
			iamRole = arn
		}
	}

	var stateReason string
	if instance.StateReason != nil {
		stateReason = aws.ToString(instance.StateReason.Message)
	}

	ebsVolumes := make([]map[string]interface{}, 0, len(instance.BlockDeviceMappings))
	for _, bdm := range instance.BlockDeviceMappings {
		volEntry := map[string]interface{}{
			"device_name": aws.ToString(bdm.DeviceName),
		}
		if bdm.Ebs != nil {
			volEntry["volume_id"] = aws.ToString(bdm.Ebs.VolumeId)
			volEntry["status"] = string(bdm.Ebs.Status)
			volEntry["delete_on_termination"] = aws.ToBool(bdm.Ebs.DeleteOnTermination)
			if bdm.Ebs.AttachTime != nil {
				volEntry["attach_time"] = bdm.Ebs.AttachTime.Format(time.RFC3339)
			}
		}
		ebsVolumes = append(ebsVolumes, volEntry)
	}

	var eniCount int
	var eniIDs []string
	for _, eni := range instance.NetworkInterfaces {
		eniCount++
		eniIDs = append(eniIDs, aws.ToString(eni.NetworkInterfaceId))
	}

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   instanceID,
		StateKey:   "aws_ec2_instance",
		Value:      name,
		DataJSON: map[string]interface{}{
			"cloud_resource_type": "ec2_instance",
			"instance_id":        instanceID,
			"name":               name,
			"instance_type":      string(instance.InstanceType),
			"state":              string(instance.State.Name),
			"state_reason":       stateReason,
			"region":             region,
			"availability_zone":  aws.ToString(instance.Placement.AvailabilityZone),
			"vpc_id":             aws.ToString(instance.VpcId),
			"subnet_id":          aws.ToString(instance.SubnetId),
			"private_ip":         privateIP,
			"public_ip":          publicIP,
			"private_dns":        privateDNS,
			"public_dns":         publicDNS,
			"ami_id":             aws.ToString(instance.ImageId),
			"key_name":           aws.ToString(instance.KeyName),
			"platform":           string(instance.PlatformDetails),
			"architecture":       string(instance.Architecture),
			"hypervisor":         string(instance.Hypervisor),
			"virtualization_type": string(instance.VirtualizationType),
			"root_device_type":   string(instance.RootDeviceType),
			"root_device_name":   aws.ToString(instance.RootDeviceName),
			"iam_instance_profile": iamRole,
			"launch_time":        launchTime,
			"security_group_names": sgNames,
			"security_group_ids":   sgIDs,
			"security_group_count": len(sgNames),
			"ebs_volumes":          ebsVolumes,
			"ebs_volume_count":     len(ebsVolumes),
			"eni_count":            eniCount,
			"eni_ids":              eniIDs,
			"tags":                 tags,
			"monitoring_state":     string(instance.Monitoring.State),
			"ebs_optimized":        aws.ToBool(instance.EbsOptimized),
			"ena_support":          aws.ToBool(instance.EnaSupport),
			"source_dest_check":    aws.ToBool(instance.SourceDestCheck),
		},
	}
}

// lastIndex returns the index of the last occurrence of sep in s, or -1.
func lastIndex(s string, sep byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == sep {
			return i
		}
	}
	return -1
}
