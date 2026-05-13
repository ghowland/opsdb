// === importers/opsdb_import_k8s/service.go ===
package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ImportServices reads Kubernetes Service objects and maps to k8s_service observations.
func ImportServices(config *K8sImportConfig) ([]Observation, error) {
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
			svcList, err := clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{
				Limit:    int64(config.BatchSize),
				Continue: continueToken,
			})
			if err != nil {
				return results, fmt.Errorf("listing services in namespace %s: %w", ns, err)
			}

			for _, svc := range svcList.Items {
				svcID := fmt.Sprintf("%s_%s_%s", config.ClusterName, svc.Namespace, svc.Name)

				svcType := classifyServiceType(svc)
				ports := summarizeServicePorts(svc.Spec.Ports)
				externalIPs := svc.Spec.ExternalIPs

				var loadBalancerIPs []string
				for _, ingress := range svc.Status.LoadBalancer.Ingress {
					if ingress.IP != "" {
						loadBalancerIPs = append(loadBalancerIPs, ingress.IP)
					} else if ingress.Hostname != "" {
						loadBalancerIPs = append(loadBalancerIPs, ingress.Hostname)
					}
				}

				selector := make(map[string]string)
				for k, v := range svc.Spec.Selector {
					selector[k] = v
				}

				obs := Observation{
					EntityType: "k8s_service",
					EntityID:   svcID,
					StateKey:   "k8s_service",
					Value:      svc.Name,
					DataJSON: map[string]interface{}{
						"name":              svc.Name,
						"namespace":         svc.Namespace,
						"cluster_name":      config.ClusterName,
						"service_type":      svcType,
						"cluster_ip":        svc.Spec.ClusterIP,
						"cluster_ips":       svc.Spec.ClusterIPs,
						"external_ips":      externalIPs,
						"load_balancer_ips": loadBalancerIPs,
						"external_name":     svc.Spec.ExternalName,
						"ports":             ports,
						"port_count":        len(svc.Spec.Ports),
						"selector":          selector,
						"session_affinity":  string(svc.Spec.SessionAffinity),
						"ip_families":       ipFamiliesToStrings(svc.Spec.IPFamilies),
						"ip_family_policy":  ipFamilyPolicyToString(svc.Spec.IPFamilyPolicy),
						"labels":            svc.Labels,
						"annotations":       filterAnnotations(svc.Annotations),
						"resource_version":  svc.ResourceVersion,
						"uid":               string(svc.UID),
						"created_time":      svc.CreationTimestamp.Format(time.RFC3339),
					},
				}
				results = append(results, obs)
			}

			continueToken = svcList.Continue
			if continueToken == "" {
				break
			}
		}
	}

	return results, nil
}

// classifyServiceType returns the service type, identifying headless services
// (ClusterIP=None) as a distinct type.
func classifyServiceType(svc corev1.Service) string {
	if svc.Spec.ClusterIP == "None" {
		return "Headless"
	}
	return string(svc.Spec.Type)
}

// summarizeServicePorts converts service port specs into a slice of summary maps.
func summarizeServicePorts(ports []corev1.ServicePort) []map[string]interface{} {
	if len(ports) == 0 {
		return nil
	}

	result := make([]map[string]interface{}, 0, len(ports))
	for _, port := range ports {
		entry := map[string]interface{}{
			"name":        port.Name,
			"protocol":    string(port.Protocol),
			"port":        int(port.Port),
			"target_port": port.TargetPort.String(),
		}
		if port.NodePort > 0 {
			entry["node_port"] = int(port.NodePort)
		}
		if port.AppProtocol != nil {
			entry["app_protocol"] = *port.AppProtocol
		}
		result = append(result, entry)
	}
	return result
}

// ipFamiliesToStrings converts IPFamily slice to string slice.
func ipFamiliesToStrings(families []corev1.IPFamily) []string {
	if len(families) == 0 {
		return nil
	}
	result := make([]string, 0, len(families))
	for _, f := range families {
		result = append(result, string(f))
	}
	return result
}

// ipFamilyPolicyToString safely converts an IPFamilyPolicy pointer to string.
func ipFamilyPolicyToString(policy *corev1.IPFamilyPolicy) string {
	if policy == nil {
		return ""
	}
	return string(*policy)
}
