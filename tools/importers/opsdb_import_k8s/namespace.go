// === importers/opsdb_import_k8s/namespace.go ===
package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ImportNamespaces reads Kubernetes namespaces and maps to k8s_namespace observations.
func ImportNamespaces(config *K8sImportConfig) ([]Observation, error) {
	var results []Observation

	clientset, _, err := buildClient(config)
	if err != nil {
		return results, fmt.Errorf("building k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	continueToken := ""
	for {
		nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
			Limit:    int64(config.BatchSize),
			Continue: continueToken,
		})
		if err != nil {
			return results, fmt.Errorf("listing namespaces: %w", err)
		}

		for _, ns := range nsList.Items {
			if len(config.Namespaces) > 0 && !isNamespaceInScope(ns.Name, config.Namespaces) {
				continue
			}

			nsID := fmt.Sprintf("%s_%s", config.ClusterName, ns.Name)

			quotaSummary := readNamespaceQuotas(clientset, ctx, ns.Name)
			limitRangeSummary := readNamespaceLimitRanges(clientset, ctx, ns.Name)

			obs := Observation{
				EntityType: "k8s_namespace",
				EntityID:   nsID,
				StateKey:   "k8s_namespace",
				Value:      ns.Name,
				DataJSON: map[string]interface{}{
					"name":             ns.Name,
					"cluster_name":     config.ClusterName,
					"status":           string(ns.Status.Phase),
					"labels":           ns.Labels,
					"annotations":      filterAnnotations(ns.Annotations),
					"resource_version": ns.ResourceVersion,
					"uid":              string(ns.UID),
					"created_time":     ns.CreationTimestamp.Format(time.RFC3339),
					"resource_quotas":  quotaSummary,
					"limit_ranges":     limitRangeSummary,
				},
			}
			results = append(results, obs)
		}

		continueToken = nsList.Continue
		if continueToken == "" {
			break
		}
	}

	return results, nil
}

// readNamespaceQuotas reads ResourceQuota objects for a namespace and returns
// a summary of hard limits and current usage.
func readNamespaceQuotas(clientset *kubernetes.Clientset, ctx context.Context, namespace string) map[string]interface{} {
	quotaList, err := clientset.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	if len(quotaList.Items) == 0 {
		return map[string]interface{}{"quota_count": 0}
	}

	quotas := make([]map[string]interface{}, 0, len(quotaList.Items))
	for _, quota := range quotaList.Items {
		hard := make(map[string]string, len(quota.Status.Hard))
		for resource, quantity := range quota.Status.Hard {
			hard[string(resource)] = quantity.String()
		}

		used := make(map[string]string, len(quota.Status.Used))
		for resource, quantity := range quota.Status.Used {
			used[string(resource)] = quantity.String()
		}

		quotas = append(quotas, map[string]interface{}{
			"name": quota.Name,
			"hard": hard,
			"used": used,
		})
	}

	return map[string]interface{}{
		"quota_count": len(quotas),
		"quotas":      quotas,
	}
}

// readNamespaceLimitRanges reads LimitRange objects for a namespace and returns
// a summary of default limits and requests.
func readNamespaceLimitRanges(clientset *kubernetes.Clientset, ctx context.Context, namespace string) map[string]interface{} {
	lrList, err := clientset.CoreV1().LimitRanges(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	if len(lrList.Items) == 0 {
		return map[string]interface{}{"limit_range_count": 0}
	}

	ranges := make([]map[string]interface{}, 0, len(lrList.Items))
	for _, lr := range lrList.Items {
		for _, item := range lr.Spec.Limits {
			limEntry := map[string]interface{}{
				"name": lr.Name,
				"type": string(item.Type),
			}

			if len(item.Default) > 0 {
				defaults := make(map[string]string, len(item.Default))
				for resource, quantity := range item.Default {
					defaults[string(resource)] = quantity.String()
				}
				limEntry["default"] = defaults
			}

			if len(item.DefaultRequest) > 0 {
				defaultRequests := make(map[string]string, len(item.DefaultRequest))
				for resource, quantity := range item.DefaultRequest {
					defaultRequests[string(resource)] = quantity.String()
				}
				limEntry["default_request"] = defaultRequests
			}

			if len(item.Max) > 0 {
				max := make(map[string]string, len(item.Max))
				for resource, quantity := range item.Max {
					max[string(resource)] = quantity.String()
				}
				limEntry["max"] = max
			}

			if len(item.Min) > 0 {
				min := make(map[string]string, len(item.Min))
				for resource, quantity := range item.Min {
					min[string(resource)] = quantity.String()
				}
				limEntry["min"] = min
			}

			ranges = append(ranges, limEntry)
		}
	}

	return map[string]interface{}{
		"limit_range_count": len(ranges),
		"limit_ranges":      ranges,
	}
}

// isNamespaceInScope checks if a namespace name is in the configured scope list.
func isNamespaceInScope(name string, scoped []string) bool {
	for _, s := range scoped {
		if s == name {
			return true
		}
	}
	return false
}
