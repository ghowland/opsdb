// === importers/opsdb_import_aws/rds.go ===
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// ImportRDS reads RDS instances from AWS API for all configured regions.
func ImportRDS(importCfg *ImportConfig) ([]Observation, error) {
	var results []Observation

	for _, region := range importCfg.Regions {
		regionObs, err := importRDSRegion(importCfg, region)
		if err != nil {
			return results, fmt.Errorf("rds import region %s: %w", region, err)
		}
		results = append(results, regionObs...)
	}

	return results, nil
}

func importRDSRegion(importCfg *ImportConfig, region string) ([]Observation, error) {
	var results []Observation

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return results, fmt.Errorf("loading aws config for %s: %w", region, err)
	}

	client := rds.NewFromConfig(cfg)

	paginator := rds.NewDescribeDBInstancesPaginator(client, &rds.DescribeDBInstancesInput{
		MaxRecords: aws.Int32(int32(importCfg.BatchSize)),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return results, fmt.Errorf("rds DescribeDBInstances page in %s: %w", region, err)
		}

		for _, instance := range page.DBInstances {
			identifier := aws.ToString(instance.DBInstanceIdentifier)

			var endpointAddress string
			var endpointPort int
			if instance.Endpoint != nil {
				endpointAddress = aws.ToString(instance.Endpoint.Address)
				endpointPort = int(instance.Endpoint.Port)
			}

			sgIDs := make([]string, 0, len(instance.VpcSecurityGroups))
			sgStatuses := make([]string, 0, len(instance.VpcSecurityGroups))
			for _, sg := range instance.VpcSecurityGroups {
				sgIDs = append(sgIDs, aws.ToString(sg.VpcSecurityGroupId))
				sgStatuses = append(sgStatuses, aws.ToString(sg.Status))
			}

			var vpcID string
			var subnetGroupName string
			if instance.DBSubnetGroup != nil {
				vpcID = aws.ToString(instance.DBSubnetGroup.VpcId)
				subnetGroupName = aws.ToString(instance.DBSubnetGroup.DBSubnetGroupName)
			}

			var parameterGroupName string
			if len(instance.DBParameterGroups) > 0 {
				parameterGroupName = aws.ToString(instance.DBParameterGroups[0].DBParameterGroupName)
			}

			var optionGroupName string
			if len(instance.OptionGroupMemberships) > 0 {
				optionGroupName = aws.ToString(instance.OptionGroupMemberships[0].OptionGroupName)
			}

			tags := make(map[string]string, len(instance.TagList))
			for _, tag := range instance.TagList {
				tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}

			var createTime string
			if instance.InstanceCreateTime != nil {
				createTime = instance.InstanceCreateTime.Format(time.RFC3339)
			}

			var allocatedStorage int
			if instance.AllocatedStorage != nil {
				allocatedStorage = int(*instance.AllocatedStorage)
			}

			var backupRetention int
			if instance.BackupRetentionPeriod != nil {
				backupRetention = int(*instance.BackupRetentionPeriod)
			}

			var monitoringInterval int
			if instance.MonitoringInterval != nil {
				monitoringInterval = int(*instance.MonitoringInterval)
			}

			obs := MapRDSInstance(
				identifier,
				aws.ToString(instance.Engine),
				aws.ToString(instance.EngineVersion),
				aws.ToString(instance.DBInstanceClass),
				allocatedStorage,
				aws.ToBool(&instance.MultiAZ),
				aws.ToString(instance.DBInstanceStatus),
				endpointAddress,
				endpointPort,
				vpcID,
				subnetGroupName,
				aws.ToString(instance.AvailabilityZone),
				region,
				aws.ToString(instance.DBInstanceArn),
				aws.ToString(instance.StorageType),
				instance.StorageEncrypted,
				aws.ToString(instance.KmsKeyId),
				instance.PubliclyAccessible,
				instance.AutoMinorVersionUpgrade,
				backupRetention,
				aws.ToString(instance.PreferredBackupWindow),
				aws.ToString(instance.PreferredMaintenanceWindow),
				parameterGroupName,
				optionGroupName,
				aws.ToString(instance.CACertificateIdentifier),
				instance.IAMDatabaseAuthenticationEnabled,
				instance.DeletionProtection,
				instance.PerformanceInsightsEnabled,
				monitoringInterval,
				sgIDs,
				sgStatuses,
				tags,
				createTime,
			)

			// if this is a read replica, add linkage to primary
			if instance.ReadReplicaSourceDBInstanceIdentifier != nil {
				primaryID := aws.ToString(instance.ReadReplicaSourceDBInstanceIdentifier)
				obs.DataJSON["is_read_replica"] = true
				obs.DataJSON["read_replica_source"] = primaryID
			} else {
				obs.DataJSON["is_read_replica"] = false
				// list read replicas of this primary
				replicaIDs := make([]string, 0, len(instance.ReadReplicaDBInstanceIdentifiers))
				for _, rid := range instance.ReadReplicaDBInstanceIdentifiers {
					replicaIDs = append(replicaIDs, rid)
				}
				if len(replicaIDs) > 0 {
					obs.DataJSON["read_replica_ids"] = replicaIDs
					obs.DataJSON["read_replica_count"] = len(replicaIDs)
				}
			}

			results = append(results, obs)
		}
	}

	return results, nil
}
