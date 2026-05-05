package gcp

import "fmt"

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
	Name                  string
	Location              string // zone or region
	LocationType          string // zonal or regional
	ClusterVersion        string // Kubernetes master version
	NodePoolCount         int
	TotalNodeCount        int
	Status                string // RUNNING, PROVISIONING, STOPPING, ERROR, etc.
	Endpoint              string // API server endpoint
	Network               string
	Subnetwork            string
	PodCIDR               string
	ServiceCIDR           string
	MasterAuthorizedNets  bool
	NetworkPolicy         bool
	PrivateCluster        bool
	AutopilotEnabled      bool
	ReleaseChannel        string // RAPID, REGULAR, STABLE, unspecified
	AutoUpgradeEnabled    bool
	ShieldedNodesEnabled  bool
	WorkloadIdentity      bool
	BinaryAuthorization   bool
	LoggingService        string
	MonitoringService     string
	CreationTime          string
	SelfLink              string
}

// listGKEClusters reads all GKE clusters in a project across all locations.
func listGKEClusters(project string) ([]gkeCluster, error) {
	// TODO: create container service client using application default credentials
	// TODO: call projects.locations.clusters.list with location="-" for all locations
	// TODO: for each cluster in response:
	//   extract Name, Location, determine LocationType from location format
	//     (zone format: us-central1-a, region format: us-central1),
	//   CurrentMasterVersion, len(NodePools), sum of NodePools[*].InitialNodeCount
	//     or CurrentNodeCount,
	//   Status, Endpoint, Network, Subnetwork,
	//   ClusterIpv4Cidr, ServicesIpv4Cidr,
	//   MasterAuthorizedNetworksConfig.Enabled,
	//   NetworkPolicy.Enabled,
	//   PrivateClusterConfig != nil && PrivateClusterConfig.EnablePrivateNodes,
	//   Autopilot.Enabled,
	//   ReleaseChannel.Channel,
	//   NodePools[0].Management.AutoUpgrade (as general indicator),
	//   ShieldedNodes.Enabled,
	//   WorkloadIdentityConfig.WorkloadPool != "",
	//   BinaryAuthorization.Enabled,
	//   LoggingService, MonitoringService,
	//   CreateTime, SelfLink
	// TODO: handle API errors (permission denied, quota)
	// TODO: return clusters
	return nil, nil
}

// MapGKECluster transforms a raw GKE cluster into two OpsDB observations:
// one for cloud_resource (infrastructure perspective) and one for k8s_cluster
// (Kubernetes operational perspective).
func MapGKECluster(project string, cluster gkeCluster) (Observation, Observation) {
	// Cloud resource observation — how GCP sees this cluster.
	cloudDataJSON := map[string]interface{}{
		"project":                    project,
		"cluster_name":               cluster.Name,
		"location":                   cluster.Location,
		"location_type":              cluster.LocationType,
		"cluster_version":            cluster.ClusterVersion,
		"node_pool_count":            cluster.NodePoolCount,
		"total_node_count":           cluster.TotalNodeCount,
		"status":                     cluster.Status,
		"network":                    cluster.Network,
		"subnetwork":                 cluster.Subnetwork,
		"is_autopilot":               cluster.AutopilotEnabled,
		"is_private_cluster":         cluster.PrivateCluster,
		"release_channel":            cluster.ReleaseChannel,
		"is_auto_upgrade_enabled":    cluster.AutoUpgradeEnabled,
		"is_shielded_nodes":          cluster.ShieldedNodesEnabled,
		"is_workload_identity":       cluster.WorkloadIdentity,
		"is_binary_authorization":    cluster.BinaryAuthorization,
		"logging_service":            cluster.LoggingService,
		"monitoring_service":         cluster.MonitoringService,
		"creation_time":              cluster.CreationTime,
		"self_link":                  cluster.SelfLink,
	}

	cloudObs := Observation{
		EntityType: "cloud_resource",
		EntityID:   fmt.Sprintf("gcp:%s:gke:%s:%s", project, cluster.Location, cluster.Name),
		StateKey:   "gke_cluster",
		Value:      cluster.Status,
		DataJSON:   cloudDataJSON,
	}

	// K8s cluster observation — how Kubernetes operations sees this cluster.
	k8sDataJSON := map[string]interface{}{
		"cluster_name":                cluster.Name,
		"distribution":               "gke",
		"kubernetes_version":          cluster.ClusterVersion,
		"api_server_endpoint":         cluster.Endpoint,
		"node_count":                  cluster.TotalNodeCount,
		"pod_cidr":                    cluster.PodCIDR,
		"service_cidr":               cluster.ServiceCIDR,
		"is_master_authorized_nets":   cluster.MasterAuthorizedNets,
		"is_network_policy":           cluster.NetworkPolicy,
		"is_private":                  cluster.PrivateCluster,
		"is_autopilot":               cluster.AutopilotEnabled,
		"cloud_provider":             "gcp",
		"cloud_project":              project,
		"cloud_location":             cluster.Location,
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