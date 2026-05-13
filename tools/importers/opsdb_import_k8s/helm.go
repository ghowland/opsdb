// === importers/opsdb_import_k8s/helm.go ===
package k8s

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ImportHelm reads Helm releases from the cluster by inspecting Helm release
// secrets (the default storage backend since Helm 3) and maps them to
// k8s_helm_release observations.
func ImportHelm(config *K8sImportConfig) ([]Observation, error) {
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
		releases, err := discoverHelmReleases(clientset, ctx, ns, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("discovering helm releases in namespace %s: %w", ns, err)
		}

		for _, rel := range releases {
			releaseID := fmt.Sprintf("%s_%s_%s", config.ClusterName, rel.Namespace, rel.Name)

			valueKeys := make([]string, 0, len(rel.Values))
			for k := range rel.Values {
				valueKeys = append(valueKeys, k)
			}
			sort.Strings(valueKeys)

			obs := Observation{
				EntityType: "k8s_helm_release",
				EntityID:   releaseID,
				StateKey:   "k8s_helm_release",
				Value:      rel.Name,
				DataJSON: map[string]interface{}{
					"name":           rel.Name,
					"namespace":      rel.Namespace,
					"cluster_name":   config.ClusterName,
					"chart_name":     rel.ChartName,
					"chart_version":  rel.ChartVersion,
					"app_version":    rel.AppVersion,
					"status":         rel.Status,
					"revision":       rel.Revision,
					"first_deployed": rel.FirstDeployed,
					"last_deployed":  rel.LastDeployed,
					"description":    rel.Description,
					"value_count":    len(rel.Values),
					"value_keys":     valueKeys,
				},
			}
			results = append(results, obs)
		}
	}

	return results, nil
}

// helmRelease holds parsed Helm release metadata from cluster storage.
type helmRelease struct {
	Name          string
	Namespace     string
	ChartName     string
	ChartVersion  string
	AppVersion    string
	Status        string
	Revision      int
	FirstDeployed string
	LastDeployed  string
	Description   string
	Values        map[string]interface{}
}

// discoverHelmReleases finds Helm 3 releases by reading secrets with the
// owner=helm label in the given namespace.
func discoverHelmReleases(clientset *kubernetes.Clientset, ctx context.Context, namespace string, batchSize int) ([]helmRelease, error) {
	// Helm 3 stores releases as secrets with type=helm.sh/release.v1
	// and label owner=helm
	releaseMap := make(map[string]*helmRelease)

	continueToken := ""
	for {
		secretList, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "owner=helm",
			Limit:         int64(batchSize),
			Continue:      continueToken,
		})
		if err != nil {
			// fall back to configmap storage if secrets fail
			return discoverHelmReleasesFromConfigMaps(clientset, ctx, namespace, batchSize)
		}

		for _, secret := range secretList.Items {
			releaseName := secret.Labels["name"]
			if releaseName == "" {
				continue
			}

			rel, err := decodeHelmRelease(secret.Data["release"])
			if err != nil {
				continue
			}
			rel.Namespace = namespace

			// keep only the latest revision per release name
			existing, exists := releaseMap[releaseName]
			if !exists || rel.Revision > existing.Revision {
				releaseMap[releaseName] = rel
			}
		}

		continueToken = secretList.Continue
		if continueToken == "" {
			break
		}
	}

	releases := make([]helmRelease, 0, len(releaseMap))
	for _, rel := range releaseMap {
		releases = append(releases, *rel)
	}
	return releases, nil
}

// discoverHelmReleasesFromConfigMaps is the fallback for Helm installations
// using configmap storage backend (HELM_DRIVER=configmap).
func discoverHelmReleasesFromConfigMaps(clientset *kubernetes.Clientset, ctx context.Context, namespace string, batchSize int) ([]helmRelease, error) {
	releaseMap := make(map[string]*helmRelease)

	continueToken := ""
	for {
		cmList, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "owner=helm",
			Limit:         int64(batchSize),
			Continue:      continueToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing helm configmaps: %w", err)
		}

		for _, cm := range cmList.Items {
			releaseName := cm.Labels["name"]
			if releaseName == "" {
				continue
			}

			releaseData, ok := cm.Data["release"]
			if !ok {
				continue
			}

			rel, err := decodeHelmRelease([]byte(releaseData))
			if err != nil {
				continue
			}
			rel.Namespace = namespace

			existing, exists := releaseMap[releaseName]
			if !exists || rel.Revision > existing.Revision {
				releaseMap[releaseName] = rel
			}
		}

		continueToken = cmList.Continue
		if continueToken == "" {
			break
		}
	}

	releases := make([]helmRelease, 0, len(releaseMap))
	for _, rel := range releaseMap {
		releases = append(releases, *rel)
	}
	return releases, nil
}

// decodeHelmRelease decodes a Helm release payload from its stored format.
// Helm 3 encodes releases as base64(gzip(json)).
func decodeHelmRelease(data []byte) (*helmRelease, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty release data")
	}

	// try base64 decode first
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		// might already be raw bytes
		decoded = data
	}

	// try gzip decompress
	gzReader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		// might not be gzipped, try parsing directly
		return parseHelmJSON(decoded)
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("decompressing release: %w", err)
	}

	return parseHelmJSON(decompressed)
}

// parseHelmJSON parses the JSON structure of a Helm release.
func parseHelmJSON(data []byte) (*helmRelease, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing release json: %w", err)
	}

	rel := &helmRelease{
		Name:   helmGetString(raw, "name"),
		Status: helmGetNestedString(raw, "info", "status"),
	}

	if v, ok := raw["version"].(float64); ok {
		rel.Revision = int(v)
	}

	// chart metadata
	if chart, ok := raw["chart"].(map[string]interface{}); ok {
		if metadata, ok := chart["metadata"].(map[string]interface{}); ok {
			rel.ChartName = helmGetString(metadata, "name")
			rel.ChartVersion = helmGetString(metadata, "version")
			rel.AppVersion = helmGetString(metadata, "appVersion")
			rel.Description = helmGetString(metadata, "description")
		}
	}

	// timestamps
	if info, ok := raw["info"].(map[string]interface{}); ok {
		rel.FirstDeployed = helmFormatTime(info, "first_deployed")
		rel.LastDeployed = helmFormatTime(info, "last_deployed")
		if rel.Description == "" {
			rel.Description = helmGetString(info, "description")
		}
	}

	// values — flatten top-level keys only, skip deeply nested structures
	if config, ok := raw["config"].(map[string]interface{}); ok {
		rel.Values = flattenValues(config, "", 2)
	}

	return rel, nil
}

// flattenValues flattens a nested map into dot-separated keys up to maxDepth.
func flattenValues(m map[string]interface{}, prefix string, maxDepth int) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			if maxDepth > 1 {
				for fk, fv := range flattenValues(val, key, maxDepth-1) {
					result[fk] = fv
				}
			} else {
				result[key] = fmt.Sprintf("{%d keys}", len(val))
			}
		case []interface{}:
			result[key] = fmt.Sprintf("[%d items]", len(val))
		default:
			result[key] = v
		}
	}
	return result
}

// helmGetString safely extracts a string from a map.
func helmGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// helmGetNestedString safely extracts a string from a nested map.
func helmGetNestedString(m map[string]interface{}, outerKey string, innerKey string) string {
	if outer, ok := m[outerKey].(map[string]interface{}); ok {
		return helmGetString(outer, innerKey)
	}
	return ""
}

// helmFormatTime extracts a timestamp from a Helm info map and formats as RFC3339.
func helmFormatTime(info map[string]interface{}, key string) string {
	v, ok := info[key]
	if !ok {
		return ""
	}

	switch t := v.(type) {
	case string:
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed.Format(time.RFC3339)
		}
		return t
	case float64:
		return time.Unix(int64(t), 0).UTC().Format(time.RFC3339)
	default:
		s := fmt.Sprintf("%v", v)
		if !strings.Contains(s, "T") {
			return s
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed.Format(time.RFC3339)
		}
		return s
	}
}
