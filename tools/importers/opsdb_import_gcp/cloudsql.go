package gcp

import (
	"context"
	"fmt"
	"time"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// ImportCloudSQL reads Cloud SQL instances from the GCP API across all
// configured projects. Maps each instance to an observation targeting
// the cloud_resource entity with cloud_sql_instance discriminator type.
func ImportCloudSQL(config *GCPImportConfig) ([]Observation, error) {
	var observations []Observation

	for _, project := range config.Projects {
		instances, err := listCloudSQLInstances(project)
		if err != nil {
			return observations, fmt.Errorf("listing Cloud SQL instances in project %s: %w", project, err)
		}

		for _, inst := range instances {
			obs := MapCloudSQLInstance(project, inst)
			observations = append(observations, obs)

			if len(observations) >= config.BatchSize {
				return observations, nil
			}
		}
	}

	return observations, nil
}

// cloudSQLInstance holds the raw fields read from the GCP Cloud SQL API.
type cloudSQLInstance struct {
	Name              string
	DatabaseVersion   string
	Tier              string
	Region            string
	State             string
	IPAddresses       []string
	StorageSizeGB     int
	AutoResize        bool
	BackupEnabled     bool
	HAEnabled         bool
	MaintenanceWindow string
	SelfLink          string
}

// listCloudSQLInstances reads all Cloud SQL instances in a project via the GCP API.
func listCloudSQLInstances(project string) ([]cloudSQLInstance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	svc, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating sqladmin client: %w", err)
	}

	var instances []cloudSQLInstance
	pageToken := ""

	for {
		call := svc.Instances.List(project)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Context(ctx).Do()
		if err != nil {
			return instances, fmt.Errorf("listing Cloud SQL instances: %w", err)
		}

		for _, dbInst := range resp.Items {
			inst := cloudSQLInstance{
				Name:            dbInst.Name,
				DatabaseVersion: dbInst.DatabaseVersion,
				Region:          dbInst.Region,
				State:           dbInst.State,
				SelfLink:        dbInst.SelfLink,
			}

			// Settings fields.
			if dbInst.Settings != nil {
				inst.Tier = dbInst.Settings.Tier
				inst.StorageSizeGB = int(dbInst.Settings.DataDiskSizeGb)
				inst.AutoResize = dbInst.Settings.StorageAutoResize

				// Backup configuration.
				if dbInst.Settings.BackupConfiguration != nil {
					inst.BackupEnabled = dbInst.Settings.BackupConfiguration.Enabled
				}

				// HA: AvailabilityType "REGIONAL" means HA is enabled.
				inst.HAEnabled = dbInst.Settings.AvailabilityType == "REGIONAL"

				// Maintenance window: combine day and hour if set.
				if dbInst.Settings.MaintenanceWindow != nil {
					inst.MaintenanceWindow = fmt.Sprintf("day=%d hour=%d",
						dbInst.Settings.MaintenanceWindow.Day,
						dbInst.Settings.MaintenanceWindow.Hour)
				}
			}

			// IP addresses.
			for _, ipMapping := range dbInst.IpAddresses {
				if ipMapping.IpAddress != "" {
					inst.IPAddresses = append(inst.IPAddresses, ipMapping.IpAddress)
				}
			}

			instances = append(instances, inst)
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return instances, nil
}

// MapCloudSQLInstance transforms a raw GCP Cloud SQL instance into an OpsDB observation.
// Flattens per-instance metadata into cloud_data_json. Connection details
// and database-level info are flat fields; replica lists would be separate entities.
func MapCloudSQLInstance(project string, inst cloudSQLInstance) Observation {
	dataJSON := map[string]interface{}{
		"project":            project,
		"instance_name":      inst.Name,
		"database_version":   inst.DatabaseVersion,
		"tier":               inst.Tier,
		"region":             inst.Region,
		"state":              inst.State,
		"storage_size_gb":    inst.StorageSizeGB,
		"is_auto_resize":     inst.AutoResize,
		"is_backup_enabled":  inst.BackupEnabled,
		"is_ha_enabled":      inst.HAEnabled,
		"maintenance_window": inst.MaintenanceWindow,
		"self_link":          inst.SelfLink,
	}

	if len(inst.IPAddresses) > 0 {
		dataJSON["primary_ip_address"] = inst.IPAddresses[0]
	}
	dataJSON["ip_address_count"] = len(inst.IPAddresses)

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   fmt.Sprintf("gcp:%s:cloudsql:%s", project, inst.Name),
		StateKey:   "cloud_sql_instance",
		Value:      inst.State,
		DataJSON:   dataJSON,
	}
}