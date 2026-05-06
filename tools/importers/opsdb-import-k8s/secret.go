// === importers/opsdb-import-k8s/secret.go ===
package k8s

import (
	"context"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ImportSecrets reads Kubernetes Secret metadata and maps to k8s_secret_reference
// observations. NEVER reads secret values. Only metadata: name, namespace, type,
// creation time, and data key names.
func ImportSecrets(config *K8sImportConfig) ([]Observation, error) {
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
			secretList, err := clientset.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{
				Limit:    int64(config.BatchSize),
				Continue: continueToken,
			})
			if err != nil {
				return results, fmt.Errorf("listing secrets in namespace %s: %w", ns, err)
			}

			for _, secret := range secretList.Items {
				secretID := fmt.Sprintf("%s_%s_%s", config.ClusterName, secret.Namespace, secret.Name)

				// extract key names only — never read values
				dataKeys := make([]string, 0, len(secret.Data))
				for key := range secret.Data {
					dataKeys = append(dataKeys, key)
				}
				sort.Strings(dataKeys)

				ownerKind, ownerName := extractOwnerReference(secret.OwnerReferences)
				secretType := string(secret.Type)

				// determine if this is a helm release secret (skip if desired,
				// but still record metadata)
				isHelmRelease := secret.Type == "helm.sh/release.v1"

				obs := Observation{
					EntityType: "k8s_secret_reference",
					EntityID:   secretID,
					StateKey:   "k8s_secret_reference",
					Value:      secret.Name,
					DataJSON: map[string]interface{}{
						"name":             secret.Name,
						"namespace":        secret.Namespace,
						"cluster_name":     config.ClusterName,
						"secret_type":      secretType,
						"data_key_count":   len(secret.Data),
						"data_keys":        dataKeys,
						"is_helm_release":  isHelmRelease,
						"owner_kind":       ownerKind,
						"owner_name":       ownerName,
						"labels":           secret.Labels,
						"annotations":      filterAnnotations(secret.Annotations),
						"resource_version": secret.ResourceVersion,
						"uid":              string(secret.UID),
						"created_time":     secret.CreationTimestamp.Format(time.RFC3339),
					},
				}
				results = append(results, obs)
			}

			continueToken = secretList.Continue
			if continueToken == "" {
				break
			}
		}
	}

	return results, nil
}
