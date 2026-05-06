// === importers/opsdb-import-k8s/configmap.go ===
package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ImportConfigMaps reads Kubernetes ConfigMaps and maps to k8s_config_map observations.
func ImportConfigMaps(config *K8sImportConfig) ([]Observation, error) {
	var results []Observation

	clientset, _, err := buildClient(config)
	if err != nil {
		return results, fmt.Errorf("building k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	namespaces, err := resolveNamespaces(clientset, ctx, config.Namespaces)
	if err != nil {
		return results, fmt.Errorf("resolving namespaces: %w", err)
	}

	for _, ns := range namespaces {
		continueToken := ""
		for {
			cmList, err := clientset.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{
				Limit:    int64(config.BatchSize),
				Continue: continueToken,
			})
			if err != nil {
				return results, fmt.Errorf("listing configmaps in namespace %s: %w", ns, err)
			}

			for _, cm := range cmList.Items {
				cmID := fmt.Sprintf("%s_%s_%s", config.ClusterName, cm.Namespace, cm.Name)

				dataKeys := make([]string, 0, len(cm.Data))
				dataSizes := make(map[string]int, len(cm.Data))
				totalBytes := 0
				for key, value := range cm.Data {
					dataKeys = append(dataKeys, key)
					dataSizes[key] = len(value)
					totalBytes += len(value)
				}

				binaryKeys := make([]string, 0, len(cm.BinaryData))
				for key, value := range cm.BinaryData {
					binaryKeys = append(binaryKeys, key)
					dataSizes[key] = len(value)
					totalBytes += len(value)
				}

				obs := Observation{
					EntityType: "k8s_config_map",
					EntityID:   cmID,
					StateKey:   "k8s_configmap",
					Value:      cm.Name,
					DataJSON: map[string]interface{}{
						"name":             cm.Name,
						"namespace":        cm.Namespace,
						"cluster_name":     config.ClusterName,
						"data_key_count":   len(cm.Data),
						"data_keys":        dataKeys,
						"data_sizes":       dataSizes,
						"binary_key_count": len(cm.BinaryData),
						"binary_keys":      binaryKeys,
						"total_bytes":      totalBytes,
						"labels":           cm.Labels,
						"annotations":      filterAnnotations(cm.Annotations),
						"resource_version": cm.ResourceVersion,
						"uid":              string(cm.UID),
						"created_time":     cm.CreationTimestamp.Format(time.RFC3339),
					},
				}
				results = append(results, obs)

				for key, value := range cm.Data {
					if len(value) > 8192 {
						continue
					}
					varID := fmt.Sprintf("%s_%s", cmID, sanitizeK8sID(key))
					varObs := Observation{
						EntityType: "observation_cache_config",
						EntityID:   varID,
						StateKey:   fmt.Sprintf("k8s_configmap_%s_%s_%s", cm.Namespace, cm.Name, key),
						Value:      truncateValue(value, 512),
						DataJSON: map[string]interface{}{
							"source":         "k8s_configmap",
							"cluster_name":   config.ClusterName,
							"namespace":      cm.Namespace,
							"configmap_name": cm.Name,
							"key":            key,
							"value_bytes":    len(value),
							"value":          truncateValue(value, 4096),
						},
					}
					results = append(results, varObs)
				}
			}

			continueToken = cmList.Continue
			if continueToken == "" {
				break
			}
		}
	}

	return results, nil
}

// resolveNamespaces returns the list of namespaces to import from.
// If the configured list is empty, all namespaces are discovered from the cluster.
func resolveNamespaces(clientset *kubernetes.Clientset, ctx context.Context, configured []string) ([]string, error) {
	if len(configured) > 0 {
		return configured, nil
	}

	nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}

	namespaces := make([]string, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

// filterAnnotations removes kubectl and k8s internal annotations that are
// high-volume and low-value for operational tracking.
func filterAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		return nil
	}
	filtered := make(map[string]string, len(annotations))
	for k, v := range annotations {
		if k == "kubectl.kubernetes.io/last-applied-configuration" {
			continue
		}
		filtered[k] = truncateValue(v, 1024)
	}
	return filtered
}

// truncateValue returns the first maxLen characters of s, appending "..." if truncated.
func truncateValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
