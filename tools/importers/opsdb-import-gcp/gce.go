package gcp

import "fmt"

// ImportGCE reads GCE instances from the GCP API across all configured
// projects. Uses aggregated list to get instances across all zones in
// one call per project. Maps each instance to an observation targeting
// the cloud_resource entity with gce_instance discriminator type.
func ImportGCE(config *GCPImportConfig) ([]Observation, error) {
	var observations []Observation

	for _, project := range config.Projects {
		instances, err := listGCEInstances(project)
		if err != nil {
			return observations, fmt.Errorf("listing GCE instances in project %s: %w", project, err)
		}

		for _, inst := range instances {
			obs := MapGCEInstance(project, inst)
			observations = append(observations, obs)

			if len(observations) >= config.BatchSize {
				return observations, nil
			}
		}
	}

	return observations, nil
}

// gceInstance holds the raw fields read from the GCP Compute Engine API.
type gceInstance struct {
	Name              string
	Zone              string
	MachineType       string // short name extracted from full URL
	Status            string // RUNNING, STOPPED, TERMINATED, etc.
	InternalIP        string
	ExternalIP        string
	NetworkInterfaces int
	DiskCount         int
	DiskSizeGB        int64
	Preemptible       bool
	Labels            map[string]string
	ServiceAccounts   []string
	CreationTimestamp  string
	SelfLink          string
}

// listGCEInstances reads all GCE instances in a project via aggregated list.
// Aggregated list returns instances across all zones in one paginated call.
func listGCEInstances(project string) ([]gceInstance, error) {
	// TODO: create compute service client using application default credentials
	//   or credentials from environment (GOOGLE_APPLICATION_CREDENTIALS)
	// TODO: call instances.AggregatedList(project)
	// TODO: handle pagination via NextPageToken
	// TODO: for each zone's instance list in response:
	//   for each instance:
	//     extract Name, Zone (short name from full URL),
	//     MachineType (short name from full URL),
	//     Status, NetworkInterfaces[0].NetworkIP (internal),
	//     NetworkInterfaces[0].AccessConfigs[0].NatIP (external, if present),
	//     len(NetworkInterfaces), len(Disks),
	//     sum of Disks[*].DiskSizeGb,
	//     Scheduling.Preemptible, Labels,
	//     ServiceAccounts[*].Email,
	//     CreationTimestamp, SelfLink
	// TODO: handle API errors (permission denied, quota exceeded)
	// TODO: return instances
	return nil, nil
}

// MapGCEInstance transforms a raw GCP Compute Engine instance into an OpsDB
// observation. Flattens per-instance metadata into cloud_data_json.
// Disk details and network interface lists are summarized (counts + totals)
// rather than fully expanded — individual disks are separate cloud_resources
// if the org needs that granularity.
func MapGCEInstance(project string, inst gceInstance) Observation {
	dataJSON := map[string]interface{}{
		"project":             project,
		"instance_name":       inst.Name,
		"zone":                inst.Zone,
		"machine_type":        inst.MachineType,
		"status":              inst.Status,
		"internal_ip":         inst.InternalIP,
		"network_interface_count": inst.NetworkInterfaces,
		"disk_count":          inst.DiskCount,
		"total_disk_size_gb":  inst.DiskSizeGB,
		"is_preemptible":      inst.Preemptible,
		"creation_timestamp":  inst.CreationTimestamp,
		"self_link":           inst.SelfLink,
	}

	// External IP is optional — not all instances have one.
	if inst.ExternalIP != "" {
		dataJSON["external_ip"] = inst.ExternalIP
	}

	// Labels flattened into the JSON payload as a nested map.
	// Labels are per-instance metadata, not N-of independent entities.
	if len(inst.Labels) > 0 {
		dataJSON["labels"] = inst.Labels
	}

	// Service account count and first account for quick reference.
	dataJSON["service_account_count"] = len(inst.ServiceAccounts)
	if len(inst.ServiceAccounts) > 0 {
		dataJSON["primary_service_account"] = inst.ServiceAccounts[0]
	}

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   fmt.Sprintf("gcp:%s:gce:%s:%s", project, inst.Zone, inst.Name),
		StateKey:   "gce_instance",
		Value:      inst.Status,
		DataJSON:   dataJSON,
	}
}
