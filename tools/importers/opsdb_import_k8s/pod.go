// === importers/opsdb-import-k8s/pod.go ===
package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ImportPods reads Kubernetes pods and maps to k8s_pod observations.
func ImportPods(config *K8sImportConfig) ([]Observation, error) {
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
			podList, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
				Limit:    int64(config.BatchSize),
				Continue: continueToken,
			})
			if err != nil {
				return results, fmt.Errorf("listing pods in namespace %s: %w", ns, err)
			}

			for _, pod := range podList.Items {
				podID := fmt.Sprintf("%s_%s_%s", config.ClusterName, pod.Namespace, pod.Name)

				containers := summarizeContainerStatuses(pod.Status.ContainerStatuses)
				initContainers := summarizeContainerStatuses(pod.Status.InitContainerStatuses)
				totalRestarts := countTotalRestarts(pod.Status.ContainerStatuses)
				ownerKind, ownerName := extractOwnerReference(pod.OwnerReferences)
				conditions := summarizePodConditions(pod.Status.Conditions)

				requestsTotal, limitsTotal := aggregateContainerResources(pod.Spec.Containers)

				var startTime string
				if pod.Status.StartTime != nil {
					startTime = pod.Status.StartTime.Format(time.RFC3339)
				}

				obs := Observation{
					EntityType: "k8s_pod",
					EntityID:   podID,
					StateKey:   "k8s_pod",
					Value:      pod.Name,
					DataJSON: map[string]interface{}{
						"name":                pod.Name,
						"namespace":           pod.Namespace,
						"cluster_name":        config.ClusterName,
						"uid":                 string(pod.UID),
						"phase":               string(pod.Status.Phase),
						"node_name":           pod.Spec.NodeName,
						"pod_ip":              pod.Status.PodIP,
						"host_ip":             pod.Status.HostIP,
						"start_time":          startTime,
						"qos_class":           string(pod.Status.QOSClass),
						"service_account":     pod.Spec.ServiceAccountName,
						"restart_policy":      string(pod.Spec.RestartPolicy),
						"total_restarts":      totalRestarts,
						"container_count":     len(pod.Spec.Containers),
						"containers":          containers,
						"init_container_count": len(pod.Spec.InitContainers),
						"init_containers":     initContainers,
						"conditions":          conditions,
						"owner_kind":          ownerKind,
						"owner_name":          ownerName,
						"requests":            requestsTotal,
						"limits":              limitsTotal,
						"labels":              pod.Labels,
						"annotations":         filterAnnotations(pod.Annotations),
						"resource_version":    pod.ResourceVersion,
						"created_time":        pod.CreationTimestamp.Format(time.RFC3339),
					},
				}
				results = append(results, obs)
			}

			continueToken = podList.Continue
			if continueToken == "" {
				break
			}
		}
	}

	return results, nil
}

// summarizeContainerStatuses converts container statuses into a slice of summary maps.
func summarizeContainerStatuses(statuses []corev1.ContainerStatus) []map[string]interface{} {
	if len(statuses) == 0 {
		return nil
	}

	result := make([]map[string]interface{}, 0, len(statuses))
	for _, cs := range statuses {
		entry := map[string]interface{}{
			"name":          cs.Name,
			"ready":         cs.Ready,
			"started":       cs.Started != nil && *cs.Started,
			"restart_count": int(cs.RestartCount),
			"image":         cs.Image,
			"image_id":      cs.ImageID,
			"container_id":  cs.ContainerID,
		}

		if cs.State.Running != nil {
			entry["state"] = "running"
			entry["started_at"] = cs.State.Running.StartedAt.Format(time.RFC3339)
		} else if cs.State.Waiting != nil {
			entry["state"] = "waiting"
			entry["waiting_reason"] = cs.State.Waiting.Reason
			entry["waiting_message"] = truncateValue(cs.State.Waiting.Message, 256)
		} else if cs.State.Terminated != nil {
			entry["state"] = "terminated"
			entry["terminated_reason"] = cs.State.Terminated.Reason
			entry["terminated_message"] = truncateValue(cs.State.Terminated.Message, 256)
			entry["exit_code"] = int(cs.State.Terminated.ExitCode)
			if !cs.State.Terminated.FinishedAt.IsZero() {
				entry["finished_at"] = cs.State.Terminated.FinishedAt.Format(time.RFC3339)
			}
		} else {
			entry["state"] = "unknown"
		}

		if cs.LastTerminationState.Terminated != nil {
			last := cs.LastTerminationState.Terminated
			entry["last_terminated_reason"] = last.Reason
			entry["last_terminated_exit_code"] = int(last.ExitCode)
			if !last.FinishedAt.IsZero() {
				entry["last_terminated_at"] = last.FinishedAt.Format(time.RFC3339)
			}
		}

		result = append(result, entry)
	}
	return result
}

// countTotalRestarts sums restart counts across all container statuses.
func countTotalRestarts(statuses []corev1.ContainerStatus) int {
	total := 0
	for _, cs := range statuses {
		total += int(cs.RestartCount)
	}
	return total
}

// extractOwnerReference returns the kind and name of the first owner reference,
// which links a pod to its controlling workload.
func extractOwnerReference(owners []metav1.OwnerReference) (string, string) {
	if len(owners) == 0 {
		return "", ""
	}
	// prefer the controller owner
	for _, owner := range owners {
		if owner.Controller != nil && *owner.Controller {
			return owner.Kind, owner.Name
		}
	}
	return owners[0].Kind, owners[0].Name
}

// summarizePodConditions converts pod conditions into a map of condition type to status detail.
func summarizePodConditions(conditions []corev1.PodCondition) map[string]interface{} {
	if len(conditions) == 0 {
		return nil
	}

	result := make(map[string]interface{}, len(conditions))
	for _, c := range conditions {
		entry := map[string]interface{}{
			"status": string(c.Status),
		}
		if c.Reason != "" {
			entry["reason"] = c.Reason
		}
		if c.Message != "" {
			entry["message"] = truncateValue(c.Message, 256)
		}
		if !c.LastTransitionTime.IsZero() {
			entry["last_transition_time"] = c.LastTransitionTime.Format(time.RFC3339)
		}
		result[string(c.Type)] = entry
	}
	return result
}

// aggregateContainerResources sums resource requests and limits across all containers in a pod.
func aggregateContainerResources(containers []corev1.Container) (map[string]string, map[string]string) {
	requests := make(map[string]int64)
	limits := make(map[string]int64)

	for _, c := range containers {
		for resource, quantity := range c.Resources.Requests {
			requests[string(resource)] += quantity.MilliValue()
		}
		for resource, quantity := range c.Resources.Limits {
			limits[string(resource)] += quantity.MilliValue()
		}
	}

	requestStrs := make(map[string]string, len(requests))
	for k, v := range requests {
		if k == "cpu" {
			requestStrs[k] = fmt.Sprintf("%dm", v)
		} else {
			requestStrs[k] = fmt.Sprintf("%d", v)
		}
	}

	limitStrs := make(map[string]string, len(limits))
	for k, v := range limits {
		if k == "cpu" {
			limitStrs[k] = fmt.Sprintf("%dm", v)
		} else {
			limitStrs[k] = fmt.Sprintf("%d", v)
		}
	}

	return requestStrs, limitStrs
}
