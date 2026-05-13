// === importers/opsdb_import_k8s/node.go ===
package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ImportNodes reads Kubernetes nodes and maps to k8s_cluster_node observations.
func ImportNodes(config *K8sImportConfig) ([]Observation, error) {
	var results []Observation

	clientset, _, err := buildClient(config)
	if err != nil {
		return results, fmt.Errorf("building k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	continueToken := ""
	for {
		nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
			Limit:    int64(config.BatchSize),
			Continue: continueToken,
		})
		if err != nil {
			return results, fmt.Errorf("listing nodes: %w", err)
		}

		for _, node := range nodeList.Items {
			nodeID := fmt.Sprintf("%s_%s", config.ClusterName, node.Name)

			roles := extractNodeRoles(node.Labels)
			conditions := summarizeNodeConditions(node.Status.Conditions)
			addresses := summarizeNodeAddresses(node.Status.Addresses)
			capacity := summarizeResources(node.Status.Capacity)
			allocatable := summarizeResources(node.Status.Allocatable)
			providerID, instanceID := parseProviderID(node.Spec.ProviderID)

			obs := Observation{
				EntityType: "k8s_cluster_node",
				EntityID:   nodeID,
				StateKey:   "k8s_node",
				Value:      node.Name,
				DataJSON: map[string]interface{}{
					"name":               node.Name,
					"cluster_name":       config.ClusterName,
					"roles":              roles,
					"schedulable":        !node.Spec.Unschedulable,
					"provider_id":        node.Spec.ProviderID,
					"cloud_provider":     providerID,
					"cloud_instance_id":  instanceID,
					"capacity":           capacity,
					"allocatable":        allocatable,
					"conditions":         conditions,
					"addresses":          addresses,
					"os_image":           node.Status.NodeInfo.OSImage,
					"os":                 node.Status.NodeInfo.OperatingSystem,
					"architecture":       node.Status.NodeInfo.Architecture,
					"kernel_version":     node.Status.NodeInfo.KernelVersion,
					"container_runtime":  node.Status.NodeInfo.ContainerRuntimeVersion,
					"kubelet_version":    node.Status.NodeInfo.KubeletVersion,
					"kube_proxy_version": node.Status.NodeInfo.KubeProxyVersion,
					"machine_id":         node.Status.NodeInfo.MachineID,
					"system_uuid":        node.Status.NodeInfo.SystemUUID,
					"pod_cidr":           node.Spec.PodCIDR,
					"labels":             node.Labels,
					"annotations":        filterAnnotations(node.Annotations),
					"taints":             summarizeTaints(node.Spec.Taints),
					"resource_version":   node.ResourceVersion,
					"uid":                string(node.UID),
					"created_time":       node.CreationTimestamp.Format(time.RFC3339),
				},
			}
			results = append(results, obs)
		}

		continueToken = nodeList.Continue
		if continueToken == "" {
			break
		}
	}

	return results, nil
}

// extractNodeRoles determines node roles from the standard role labels.
func extractNodeRoles(labels map[string]string) []string {
	var roles []string
	for key, value := range labels {
		if key == "node-role.kubernetes.io/master" || key == "node-role.kubernetes.io/control-plane" {
			roles = append(roles, "control-plane")
		} else if strings.HasPrefix(key, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			} else if value != "" {
				roles = append(roles, value)
			}
		}
	}
	if len(roles) == 0 {
		roles = append(roles, "worker")
	}
	return roles
}

// summarizeNodeConditions converts node conditions into a map of condition type to status.
func summarizeNodeConditions(conditions []corev1.NodeCondition) map[string]interface{} {
	result := make(map[string]interface{}, len(conditions))
	for _, c := range conditions {
		result[string(c.Type)] = map[string]interface{}{
			"status":               string(c.Status),
			"reason":               c.Reason,
			"message":              truncateValue(c.Message, 256),
			"last_heartbeat_time":  formatTimePtr(c.LastHeartbeatTime),
			"last_transition_time": formatTimePtr(c.LastTransitionTime),
		}
	}
	return result
}

// summarizeNodeAddresses converts node addresses into a map of address type to address.
func summarizeNodeAddresses(addresses []corev1.NodeAddress) map[string]string {
	result := make(map[string]string, len(addresses))
	for _, addr := range addresses {
		result[string(addr.Type)] = addr.Address
	}
	return result
}

// summarizeResources converts a resource list into a map of resource name to string quantity.
func summarizeResources(resources corev1.ResourceList) map[string]string {
	result := make(map[string]string, len(resources))
	for resource, quantity := range resources {
		result[string(resource)] = quantity.String()
	}
	return result
}

// summarizeTaints converts taints into a slice of summary maps.
func summarizeTaints(taints []corev1.Taint) []map[string]string {
	if len(taints) == 0 {
		return nil
	}
	result := make([]map[string]string, 0, len(taints))
	for _, taint := range taints {
		entry := map[string]string{
			"key":    taint.Key,
			"effect": string(taint.Effect),
		}
		if taint.Value != "" {
			entry["value"] = taint.Value
		}
		result = append(result, entry)
	}
	return result
}

// parseProviderID extracts cloud provider and instance ID from the node's provider ID.
// Format varies by provider:
//
//	aws:///us-east-1a/i-0abcdef1234567890
//	gce://project-id/zone/instance-name
//	azure:///subscriptions/.../virtualMachines/vm-name
func parseProviderID(providerID string) (string, string) {
	if providerID == "" {
		return "", ""
	}

	if strings.HasPrefix(providerID, "aws://") {
		parts := strings.Split(providerID, "/")
		if len(parts) > 0 {
			return "aws", parts[len(parts)-1]
		}
		return "aws", ""
	}

	if strings.HasPrefix(providerID, "gce://") {
		parts := strings.Split(providerID, "/")
		if len(parts) > 0 {
			return "gcp", parts[len(parts)-1]
		}
		return "gcp", ""
	}

	if strings.HasPrefix(providerID, "azure://") {
		parts := strings.Split(providerID, "/")
		if len(parts) > 0 {
			return "azure", parts[len(parts)-1]
		}
		return "azure", ""
	}

	// unknown provider, return the scheme portion if present
	if idx := strings.Index(providerID, "://"); idx >= 0 {
		provider := providerID[:idx]
		parts := strings.Split(providerID, "/")
		return provider, parts[len(parts)-1]
	}

	return "unknown", providerID
}

// formatTimePtr formats a metav1.Time as RFC3339, returning empty string for zero time.
func formatTimePtr(t metav1.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
