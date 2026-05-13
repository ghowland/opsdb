package gcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	container "cloud.google.com/go/container/apiv1"
	containerpb "cloud.google.com/go/container/apiv1/containerpb"
)

// ImportGKE reads GKE clusters from the GCP API across all configured projects.
// Produces dual observations per cluster: one targeting cloud_resource (the
// infrastructure view) and one targeting k8s_cluster (the Kubernetes view).
// This reflects the fact that a GKE cluster is both a cloud resource managed
// by GCP and a Kubernetes cluster with its own operational identity.
func ImportGKE(config *GCPImportConfig) ([]Observation, error) {
	var observations []Observation

	for _, project := range config.Projects {
		clusters, err := listGKEClusters(project)
		if err != nil {
			return observations, fmt.Errorf("listing GKE clusters in project %s: %w", project, err)
		}

		for _, cluster := range clusters {
			cloudObs, k8sObs := MapGKECluster(project, cluster)
			observations = append(observations, cloudObs, k8sObs)

			if len(observations) >= config.BatchSize {
				return observations, nil
			}
		}
	}

	return observations, nil
}

// gkeCluster holds the raw fields read from the GCP Container API.
type gkeCluster struct {
	Name                 string
	Location             string
	LocationType         string
	ClusterVersion       string
	NodePoolCount        int
	TotalNodeCount       int
	Status               string
	Endpoint             string
	Network              string
	Subnetwork           string
	PodCIDR              string
	ServiceCIDR          string
	MasterAuthorizedNets bool
	NetworkPolicy        bool
	PrivateCluster       bool
	AutopilotEnabled     bool
	ReleaseChannel       string
	AutoUpgradeEnabled   bool
	ShieldedNodesEnabled bool
	WorkloadIdentity     bool
	BinaryAuthorization  bool
	LoggingService       string
	MonitoringService    string
	CreationTime         string
	SelfLink             string
}

// listGKEClusters reads all GKE clusters in a project across all locations.
// Uses location="-" to get clusters from every region and zone in one call.
func listGKEClusters(project string) ([]gkeCluster, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := container.NewClusterManagerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating container client: %w", err)
	}
	defer client.Close()

	// location "-" returns clusters from all regions and zones.
	parent := fmt.Sprintf("projects/%s/locations/-", project)

	resp, err := client.ListClusters(ctx, &containerpb.ListClustersRequest{
		Parent: parent,
	})
	if err != nil {
		return nil, fmt.Errorf("listing GKE clusters: %w", err)
	}

	var clusters []gkeCluster

	for _, c := range resp.Clusters {
		if c == nil {
			continue
		}

		g := gkeCluster{
			Name:           c.Name,
			Location:       c.Location,
			ClusterVersion: c.CurrentMasterVersion,
			NodePoolCount:  len(c.NodePools),
			Endpoint:       c.Endpoint,
			Network:        c.Network,
			Subnetwork:     c.Subnetwork,
			PodCIDR:        c.ClusterIpv4Cidr,
			ServiceCIDR:    c.ServicesIpv4Cidr,
			LoggingService:  c.LoggingService,
			MonitoringService: c.MonitoringService,
			SelfLink:       c.SelfLink,
			CreationTime:   c.CreateTime,
		}

		// Status: convert enum to string.
		g.Status = c.Status.String()

		// Location type: zone names have 3 dash-separated parts (us-central1-a),
		// region names have 2 (us-central1).
		parts := strings.Split(c.Location, "-")
		if len(parts) >= 3 {
			g.LocationType = "zonal"
		} else {
			g.LocationType = "regional"
		}

		// Total node count: sum InitialNodeCount across pools, or use
		// CurrentNodeCount on each pool if available.
		totalNodes := 0
		for _, np := range c.NodePools {
			if np == nil {
				continue
			}
			if np.InitialNodeCount > 0 {
				totalNodes += int(np.InitialNodeCount)
			}
		}
		g.TotalNodeCount = totalNodes

		// Master authorized networks.
		if c.MasterAuthorizedNetworksConfig != nil {
			g.MasterAuthorizedNets = c.MasterAuthorizedNetworksConfig.Enabled
		}

		// Network policy.
		if c.NetworkPolicy != nil {
			g.NetworkPolicy = c.NetworkPolicy.Enabled
		}

		// Private cluster.
		if c.PrivateClusterConfig != nil {
			g.PrivateCluster = c.PrivateClusterConfig.EnablePrivateNodes
		}

		// Autopilot.
		if c.Autopilot != nil {
			g.AutopilotEnabled = c.Autopilot.Enabled
		}

		// Release channel.
		if c.ReleaseChannel != nil {
			g.ReleaseChannel = c.ReleaseChannel.Channel.String()
		} else {
			g.ReleaseChannel = "UNSPECIFIED"
		}

		// Auto-upgrade: check first node pool as general indicator.
		if len(c.NodePools) > 0 && c.NodePools[0].Management != nil {
			g.AutoUpgradeEnabled = c.NodePools[0].Management.AutoUpgrade
		}

		// Shielded nodes.
		if c.ShieldedNodes != nil {
			g.ShieldedNodesEnabled = c.ShieldedNodes.Enabled
		}

		// Workload identity: enabled when WorkloadPool is set.
		if c.WorkloadIdentityConfig != nil {
			g.WorkloadIdentity = c.WorkloadIdentityConfig.WorkloadPool != ""
		}

		// Binary authorization.
		if c.BinaryAuthorization != nil {
			g.BinaryAuthorization = c.BinaryAuthorization.Enabled
		}

		clusters = append(clusters, g)
	}

	return clusters, nil
}

// MapGKECluster transforms a raw GKE cluster into two OpsDB observations:
// one for cloud_resource (infrastructure perspective) and one for k8s_cluster
// (Kubernetes operational perspective).
func MapGKECluster(project string, cluster gkeCluster) (Observation, Observation) {
	cloudDataJSON := map[string]interface{}{
		"project":                 project,
		"cluster_name":            cluster.Name,
		"location":                cluster.Location,
		"location_type":           cluster.LocationType,
		"cluster_version":         cluster.ClusterVersion,
		"node_pool_count":         cluster.NodePoolCount,
		"total_node_count":        cluster.TotalNodeCount,
		"status":                  cluster.Status,
		"network":                 cluster.Network,
		"subnetwork":              cluster.Subnetwork,
		"is_autopilot":            cluster.AutopilotEnabled,
		"is_private_cluster":      cluster.PrivateCluster,
		"release_channel":         cluster.ReleaseChannel,
		"is_auto_upgrade_enabled": cluster.AutoUpgradeEnabled,
		"is_shielded_nodes":       cluster.ShieldedNodesEnabled,
		"is_workload_identity":    cluster.WorkloadIdentity,
		"is_binary_authorization": cluster.BinaryAuthorization,
		"logging_service":         cluster.LoggingService,
		"monitoring_service":      cluster.MonitoringService,
		"creation_time":           cluster.CreationTime,
		"self_link":               cluster.SelfLink,
	}

	cloudObs := Observation{
		EntityType: "cloud_resource",
		EntityID:   fmt.Sprintf("gcp:%s:gke:%s:%s", project, cluster.Location, cluster.Name),
		StateKey:   "gke_cluster",
		Value:      cluster.Status,
		DataJSON:   cloudDataJSON,
	}

	k8sDataJSON := map[string]interface{}{
		"cluster_name":              cluster.Name,
		"distribution":              "gke",
		"kubernetes_version":        cluster.ClusterVersion,
		"api_server_endpoint":       cluster.Endpoint,
		"node_count":                cluster.TotalNodeCount,
		"pod_cidr":                  cluster.PodCIDR,
		"service_cidr":              cluster.ServiceCIDR,
		"is_master_authorized_nets": cluster.MasterAuthorizedNets,
		"is_network_policy":         cluster.NetworkPolicy,
		"is_private":                cluster.PrivateCluster,
		"is_autopilot":              cluster.AutopilotEnabled,
		"cloud_provider":            "gcp",
		"cloud_project":             project,
		"cloud_location":            cluster.Location,
	}

	k8sObs := Observation{
		EntityType: "k8s_cluster",
		EntityID:   fmt.Sprintf("gke:%s:%s:%s", project, cluster.Location, cluster.Name),
		StateKey:   "cluster_metadata",
		Value:      cluster.Status,
		DataJSON:   k8sDataJSON,
	}

	return cloudObs, k8sObs
}