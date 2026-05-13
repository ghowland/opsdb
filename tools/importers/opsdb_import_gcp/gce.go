package gcp

import (
	"context"
	"fmt"
	"path"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
)

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
	MachineType       string
	Status            string
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
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating compute client: %w", err)
	}
	defer client.Close()

	var instances []gceInstance

	req := &computepb.AggregatedListInstancesRequest{
		Project: project,
	}

	it := client.AggregatedList(ctx, req)
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return instances, fmt.Errorf("iterating aggregated instances: %w", err)
		}

		// pair.Value is the InstancesScopedList for one zone.
		if pair.Value == nil || pair.Value.Instances == nil {
			continue
		}

		for _, inst := range pair.Value.Instances {
			if inst == nil {
				continue
			}

			g := gceInstance{
				Name:   derefStr(inst.Name),
				Zone:   shortName(derefStr(inst.Zone)),
				Status: derefStr(inst.Status),
				Labels: inst.Labels,
			}

			// Machine type: extract short name from full URL.
			// e.g. "zones/us-central1-a/machineTypes/e2-medium" -> "e2-medium"
			g.MachineType = shortName(derefStr(inst.MachineType))

			// Network interfaces.
			g.NetworkInterfaces = len(inst.NetworkInterfaces)
			if len(inst.NetworkInterfaces) > 0 {
				ni := inst.NetworkInterfaces[0]
				g.InternalIP = derefStr(ni.NetworkIP)
				if len(ni.AccessConfigs) > 0 {
					g.ExternalIP = derefStr(ni.AccessConfigs[0].NatIP)
				}
			}

			// Disks: count and total size.
			g.DiskCount = len(inst.Disks)
			var totalDiskGB int64
			for _, disk := range inst.Disks {
				if disk != nil && disk.DiskSizeGb != nil {
					totalDiskGB += *disk.DiskSizeGb
				}
			}
			g.DiskSizeGB = totalDiskGB

			// Scheduling.
			if inst.Scheduling != nil && inst.Scheduling.Preemptible != nil {
				g.Preemptible = *inst.Scheduling.Preemptible
			}

			// Service accounts.
			for _, sa := range inst.ServiceAccounts {
				if sa != nil {
					g.ServiceAccounts = append(g.ServiceAccounts, derefStr(sa.Email))
				}
			}

			g.CreationTimestamp = derefStr(inst.CreationTimestamp)
			g.SelfLink = derefStr(inst.SelfLink)

			instances = append(instances, g)
		}
	}

	return instances, nil
}

// MapGCEInstance transforms a raw GCP Compute Engine instance into an OpsDB
// observation. Flattens per-instance metadata into cloud_data_json.
// Disk details and network interface lists are summarized (counts + totals)
// rather than fully expanded — individual disks are separate cloud_resources
// if the org needs that granularity.
func MapGCEInstance(project string, inst gceInstance) Observation {
	dataJSON := map[string]interface{}{
		"project":                   project,
		"instance_name":             inst.Name,
		"zone":                      inst.Zone,
		"machine_type":              inst.MachineType,
		"status":                    inst.Status,
		"internal_ip":               inst.InternalIP,
		"network_interface_count":   inst.NetworkInterfaces,
		"disk_count":                inst.DiskCount,
		"total_disk_size_gb":        inst.DiskSizeGB,
		"is_preemptible":            inst.Preemptible,
		"creation_timestamp":        inst.CreationTimestamp,
		"self_link":                 inst.SelfLink,
	}

	if inst.ExternalIP != "" {
		dataJSON["external_ip"] = inst.ExternalIP
	}

	if len(inst.Labels) > 0 {
		dataJSON["labels"] = inst.Labels
	}

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

// shortName extracts the last path component from a GCP resource URL.
// "zones/us-central1-a/machineTypes/e2-medium" -> "e2-medium"
// "projects/my-proj/zones/us-central1-a" -> "us-central1-a"
func shortName(resourceURL string) string {
	return path.Base(resourceURL)
}

// derefStr safely dereferences a *string, returning empty string if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
