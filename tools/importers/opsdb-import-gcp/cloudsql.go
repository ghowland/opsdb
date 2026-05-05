package gcp

import "fmt"

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
	Name             string
	DatabaseVersion  string
	Tier             string
	Region           string
	State            string
	IPAddresses      []string
	StorageSizeGB    int
	AutoResize       bool
	BackupEnabled    bool
	HAEnabled        bool
	MaintenanceWindow string
	SelfLink         string
}

// listCloudSQLInstances reads all Cloud SQL instances in a project via the GCP API.
func listCloudSQLInstances(project string) ([]cloudSQLInstance, error) {
	// TODO: create sqladmin service client using application default credentials
	//   or credentials from environment (GOOGLE_APPLICATION_CREDENTIALS)
	// TODO: call instances.List(project)
	// TODO: handle pagination via NextPageToken
	// TODO: for each instance in response:
	//   extract Name, DatabaseVersion, Settings.Tier, Region,
	//   State, IpAddresses, Settings.DataDiskSizeGb,
	//   Settings.StorageAutoResize, Settings.BackupConfiguration.Enabled,
	//   Settings.AvailabilityType (REGIONAL = HA), Settings.MaintenanceWindow,
	//   SelfLink
	// TODO: handle API errors (permission denied, quota, not found)
	// TODO: return instances
	return nil, nil
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

	// IP addresses flattened as primary only; multiple IPs would use
	// a list in the JSON payload since they are per-instance metadata.
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
