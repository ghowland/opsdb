// === importers/opsdb-import-k8s/workload.go ===
package k8s

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ImportWorkloads reads Kubernetes workloads and maps to k8s_workload observations.
func ImportWorkloads(config *K8sImportConfig) ([]Observation, error) {
	var results []Observation

	clientset, _, err := buildClient(config)
	if err != nil {
		return results, fmt.Errorf("building k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	namespaces, err := resolveNamespaces(clientset, ctx, config.Namespaces)
	if err != nil {
		return results, fmt.Errorf("resolving namespaces: %w", err)
	}

	for _, ns := range namespaces {
		deployments, err := importDeployments(clientset, ctx, ns, config)
		if err != nil {
			return results, fmt.Errorf("deployments in %s: %w", ns, err)
		}
		results = append(results, deployments...)

		statefulsets, err := importStatefulSets(clientset, ctx, ns, config)
		if err != nil {
			return results, fmt.Errorf("statefulsets in %s: %w", ns, err)
		}
		results = append(results, statefulsets...)

		daemonsets, err := importDaemonSets(clientset, ctx, ns, config)
		if err != nil {
			return results, fmt.Errorf("daemonsets in %s: %w", ns, err)
		}
		results = append(results, daemonsets...)

		jobs, err := importJobs(clientset, ctx, ns, config)
		if err != nil {
			return results, fmt.Errorf("jobs in %s: %w", ns, err)
		}
		results = append(results, jobs...)

		cronjobs, err := importCronJobs(clientset, ctx, ns, config)
		if err != nil {
			return results, fmt.Errorf("cronjobs in %s: %w", ns, err)
		}
		results = append(results, cronjobs...)

		replicasets, err := importReplicaSets(clientset, ctx, ns, config)
		if err != nil {
			return results, fmt.Errorf("replicasets in %s: %w", ns, err)
		}
		results = append(results, replicasets...)
	}

	return results, nil
}

func importDeployments(clientset *kubernetes.Clientset, ctx context.Context, namespace string, config *K8sImportConfig) ([]Observation, error) {
	var results []Observation
	continueToken := ""

	for {
		list, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
			Limit:    int64(config.BatchSize),
			Continue: continueToken,
		})
		if err != nil {
			return results, err
		}

		for _, deploy := range list.Items {
			workloadID := fmt.Sprintf("%s_%s_%s_deployment", config.ClusterName, deploy.Namespace, deploy.Name)

			var replicas int
			if deploy.Spec.Replicas != nil {
				replicas = int(*deploy.Spec.Replicas)
			}

			images, envKeys := extractPodTemplateInfo(deploy.Spec.Template)
			resources := aggregatePodTemplateResources(deploy.Spec.Template)
			ownerKind, ownerName := extractOwnerReference(deploy.OwnerReferences)

			obs := Observation{
				EntityType: "k8s_workload",
				EntityID:   workloadID,
				StateKey:   "k8s_workload",
				Value:      deploy.Name,
				DataJSON: map[string]interface{}{
					"name":                deploy.Name,
					"namespace":           deploy.Namespace,
					"cluster_name":        config.ClusterName,
					"workload_type":       "deployment",
					"replicas":            replicas,
					"ready_replicas":      int(deploy.Status.ReadyReplicas),
					"updated_replicas":    int(deploy.Status.UpdatedReplicas),
					"available_replicas":  int(deploy.Status.AvailableReplicas),
					"images":              images,
					"env_keys":            envKeys,
					"resources":           resources,
					"strategy":            string(deploy.Spec.Strategy.Type),
					"selector":            labelSelectorToMap(deploy.Spec.Selector),
					"owner_kind":          ownerKind,
					"owner_name":          ownerName,
					"labels":              deploy.Labels,
					"annotations":         filterAnnotations(deploy.Annotations),
					"resource_version":    deploy.ResourceVersion,
					"uid":                 string(deploy.UID),
					"created_time":        deploy.CreationTimestamp.Format(time.RFC3339),
				},
			}
			results = append(results, obs)
		}

		continueToken = list.Continue
		if continueToken == "" {
			break
		}
	}

	return results, nil
}

func importStatefulSets(clientset *kubernetes.Clientset, ctx context.Context, namespace string, config *K8sImportConfig) ([]Observation, error) {
	var results []Observation
	continueToken := ""

	for {
		list, err := clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{
			Limit:    int64(config.BatchSize),
			Continue: continueToken,
		})
		if err != nil {
			return results, err
		}

		for _, sts := range list.Items {
			workloadID := fmt.Sprintf("%s_%s_%s_statefulset", config.ClusterName, sts.Namespace, sts.Name)

			var replicas int
			if sts.Spec.Replicas != nil {
				replicas = int(*sts.Spec.Replicas)
			}

			images, envKeys := extractPodTemplateInfo(sts.Spec.Template)
			resources := aggregatePodTemplateResources(sts.Spec.Template)

			pvcNames := make([]string, 0, len(sts.Spec.VolumeClaimTemplates))
			for _, pvc := range sts.Spec.VolumeClaimTemplates {
				pvcNames = append(pvcNames, pvc.Name)
			}

			obs := Observation{
				EntityType: "k8s_workload",
				EntityID:   workloadID,
				StateKey:   "k8s_workload",
				Value:      sts.Name,
				DataJSON: map[string]interface{}{
					"name":                  sts.Name,
					"namespace":             sts.Namespace,
					"cluster_name":          config.ClusterName,
					"workload_type":         "statefulset",
					"replicas":              replicas,
					"ready_replicas":        int(sts.Status.ReadyReplicas),
					"updated_replicas":      int(sts.Status.UpdatedReplicas),
					"current_replicas":      int(sts.Status.CurrentReplicas),
					"images":                images,
					"env_keys":              envKeys,
					"resources":             resources,
					"service_name":          sts.Spec.ServiceName,
					"pvc_template_names":    pvcNames,
					"pvc_template_count":    len(pvcNames),
					"pod_management_policy": string(sts.Spec.PodManagementPolicy),
					"selector":              labelSelectorToMap(sts.Spec.Selector),
					"labels":                sts.Labels,
					"annotations":           filterAnnotations(sts.Annotations),
					"resource_version":      sts.ResourceVersion,
					"uid":                   string(sts.UID),
					"created_time":          sts.CreationTimestamp.Format(time.RFC3339),
				},
			}
			results = append(results, obs)
		}

		continueToken = list.Continue
		if continueToken == "" {
			break
		}
	}

	return results, nil
}

func importDaemonSets(clientset *kubernetes.Clientset, ctx context.Context, namespace string, config *K8sImportConfig) ([]Observation, error) {
	var results []Observation
	continueToken := ""

	for {
		list, err := clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{
			Limit:    int64(config.BatchSize),
			Continue: continueToken,
		})
		if err != nil {
			return results, err
		}

		for _, ds := range list.Items {
			workloadID := fmt.Sprintf("%s_%s_%s_daemonset", config.ClusterName, ds.Namespace, ds.Name)

			images, envKeys := extractPodTemplateInfo(ds.Spec.Template)
			resources := aggregatePodTemplateResources(ds.Spec.Template)

			obs := Observation{
				EntityType: "k8s_workload",
				EntityID:   workloadID,
				StateKey:   "k8s_workload",
				Value:      ds.Name,
				DataJSON: map[string]interface{}{
					"name":                      ds.Name,
					"namespace":                 ds.Namespace,
					"cluster_name":              config.ClusterName,
					"workload_type":             "daemonset",
					"desired_number_scheduled":  int(ds.Status.DesiredNumberScheduled),
					"current_number_scheduled":  int(ds.Status.CurrentNumberScheduled),
					"number_ready":              int(ds.Status.NumberReady),
					"number_available":          int(ds.Status.NumberAvailable),
					"number_unavailable":        int(ds.Status.NumberUnavailable),
					"updated_number_scheduled":  int(ds.Status.UpdatedNumberScheduled),
					"images":                    images,
					"env_keys":                  envKeys,
					"resources":                 resources,
					"update_strategy":           string(ds.Spec.UpdateStrategy.Type),
					"selector":                  labelSelectorToMap(ds.Spec.Selector),
					"labels":                    ds.Labels,
					"annotations":               filterAnnotations(ds.Annotations),
					"resource_version":          ds.ResourceVersion,
					"uid":                       string(ds.UID),
					"created_time":              ds.CreationTimestamp.Format(time.RFC3339),
				},
			}
			results = append(results, obs)
		}

		continueToken = list.Continue
		if continueToken == "" {
			break
		}
	}

	return results, nil
}

func importJobs(clientset *kubernetes.Clientset, ctx context.Context, namespace string, config *K8sImportConfig) ([]Observation, error) {
	var results []Observation
	continueToken := ""

	for {
		list, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
			Limit:    int64(config.BatchSize),
			Continue: continueToken,
		})
		if err != nil {
			return results, err
		}

		for _, job := range list.Items {
			workloadID := fmt.Sprintf("%s_%s_%s_job", config.ClusterName, job.Namespace, job.Name)

			images, envKeys := extractPodTemplateInfo(job.Spec.Template)
			resources := aggregatePodTemplateResources(job.Spec.Template)
			ownerKind, ownerName := extractOwnerReference(job.OwnerReferences)

			var completions, parallelism, backoffLimit int
			if job.Spec.Completions != nil {
				completions = int(*job.Spec.Completions)
			}
			if job.Spec.Parallelism != nil {
				parallelism = int(*job.Spec.Parallelism)
			}
			if job.Spec.BackoffLimit != nil {
				backoffLimit = int(*job.Spec.BackoffLimit)
			}

			var startTime, completionTime string
			if job.Status.StartTime != nil {
				startTime = job.Status.StartTime.Format(time.RFC3339)
			}
			if job.Status.CompletionTime != nil {
				completionTime = job.Status.CompletionTime.Format(time.RFC3339)
			}

			obs := Observation{
				EntityType: "k8s_workload",
				EntityID:   workloadID,
				StateKey:   "k8s_workload",
				Value:      job.Name,
				DataJSON: map[string]interface{}{
					"name":             job.Name,
					"namespace":        job.Namespace,
					"cluster_name":     config.ClusterName,
					"workload_type":    "job",
					"completions":      completions,
					"parallelism":      parallelism,
					"backoff_limit":    backoffLimit,
					"active":           int(job.Status.Active),
					"succeeded":        int(job.Status.Succeeded),
					"failed":           int(job.Status.Failed),
					"start_time":       startTime,
					"completion_time":  completionTime,
					"images":           images,
					"env_keys":         envKeys,
					"resources":        resources,
					"owner_kind":       ownerKind,
					"owner_name":       ownerName,
					"labels":           job.Labels,
					"annotations":      filterAnnotations(job.Annotations),
					"resource_version": job.ResourceVersion,
					"uid":              string(job.UID),
					"created_time":     job.CreationTimestamp.Format(time.RFC3339),
				},
			}
			results = append(results, obs)
		}

		continueToken = list.Continue
		if continueToken == "" {
			break
		}
	}

	return results, nil
}

func importCronJobs(clientset *kubernetes.Clientset, ctx context.Context, namespace string, config *K8sImportConfig) ([]Observation, error) {
	var results []Observation
	continueToken := ""

	for {
		list, err := clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{
			Limit:    int64(config.BatchSize),
			Continue: continueToken,
		})
		if err != nil {
			return results, err
		}

		for _, cj := range list.Items {
			workloadID := fmt.Sprintf("%s_%s_%s_cronjob", config.ClusterName, cj.Namespace, cj.Name)

			images, envKeys := extractPodTemplateInfo(cj.Spec.JobTemplate.Spec.Template)
			resources := aggregatePodTemplateResources(cj.Spec.JobTemplate.Spec.Template)

			var suspend bool
			if cj.Spec.Suspend != nil {
				suspend = *cj.Spec.Suspend
			}

			var lastScheduleTime string
			if cj.Status.LastScheduleTime != nil {
				lastScheduleTime = cj.Status.LastScheduleTime.Format(time.RFC3339)
			}

			var lastSuccessfulTime string
			if cj.Status.LastSuccessfulTime != nil {
				lastSuccessfulTime = cj.Status.LastSuccessfulTime.Format(time.RFC3339)
			}

			obs := Observation{
				EntityType: "k8s_workload",
				EntityID:   workloadID,
				StateKey:   "k8s_workload",
				Value:      cj.Name,
				DataJSON: map[string]interface{}{
					"name":                   cj.Name,
					"namespace":              cj.Namespace,
					"cluster_name":           config.ClusterName,
					"workload_type":          "cronjob",
					"schedule":               cj.Spec.Schedule,
					"suspend":                suspend,
					"concurrency_policy":     string(cj.Spec.ConcurrencyPolicy),
					"active_count":           len(cj.Status.Active),
					"last_schedule_time":     lastScheduleTime,
					"last_successful_time":   lastSuccessfulTime,
					"images":                 images,
					"env_keys":               envKeys,
					"resources":              resources,
					"labels":                 cj.Labels,
					"annotations":            filterAnnotations(cj.Annotations),
					"resource_version":       cj.ResourceVersion,
					"uid":                    string(cj.UID),
					"created_time":           cj.CreationTimestamp.Format(time.RFC3339),
				},
			}
			results = append(results, obs)
		}

		continueToken = list.Continue
		if continueToken == "" {
			break
		}
	}

	return results, nil
}

func importReplicaSets(clientset *kubernetes.Clientset, ctx context.Context, namespace string, config *K8sImportConfig) ([]Observation, error) {
	var results []Observation
	continueToken := ""

	for {
		list, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
			Limit:    int64(config.BatchSize),
			Continue: continueToken,
		})
		if err != nil {
			return results, err
		}

		for _, rs := range list.Items {
			// skip replicasets owned by deployments — they're implementation details
			ownerKind, ownerName := extractOwnerReference(rs.OwnerReferences)
			if ownerKind == "Deployment" {
				continue
			}

			workloadID := fmt.Sprintf("%s_%s_%s_replicaset", config.ClusterName, rs.Namespace, rs.Name)

			var replicas int
			if rs.Spec.Replicas != nil {
				replicas = int(*rs.Spec.Replicas)
			}

			images, envKeys := extractPodTemplateInfo(corev1.PodTemplateSpec{
				Spec: rs.Spec.Template.Spec,
			})
			resources := aggregatePodTemplateResources(corev1.PodTemplateSpec{
				Spec: rs.Spec.Template.Spec,
			})

			obs := Observation{
				EntityType: "k8s_workload",
				EntityID:   workloadID,
				StateKey:   "k8s_workload",
				Value:      rs.Name,
				DataJSON: map[string]interface{}{
					"name":              rs.Name,
					"namespace":         rs.Namespace,
					"cluster_name":      config.ClusterName,
					"workload_type":     "replicaset",
					"replicas":          replicas,
					"ready_replicas":    int(rs.Status.ReadyReplicas),
					"available_replicas": int(rs.Status.AvailableReplicas),
					"images":            images,
					"env_keys":          envKeys,
					"resources":         resources,
					"owner_kind":        ownerKind,
					"owner_name":        ownerName,
					"selector":          labelSelectorToMap(rs.Spec.Selector),
					"labels":            rs.Labels,
					"annotations":       filterAnnotations(rs.Annotations),
					"resource_version":  rs.ResourceVersion,
					"uid":               string(rs.UID),
					"created_time":      rs.CreationTimestamp.Format(time.RFC3339),
				},
			}
			results = append(results, obs)
		}

		continueToken = list.Continue
		if continueToken == "" {
			break
		}
	}

	return results, nil
}

// extractPodTemplateInfo extracts container images and non-secret environment
// variable key names from a pod template spec.
func extractPodTemplateInfo(template corev1.PodTemplateSpec) ([]string, []string) {
	imageSet := make(map[string]bool)
	envKeySet := make(map[string]bool)

	for _, container := range template.Spec.Containers {
		if container.Image != "" {
			imageSet[container.Image] = true
		}
		for _, env := range container.Env {
			// record key names only, never values
			// skip secret-sourced env vars entirely
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				continue
			}
			envKeySet[env.Name] = true
		}
	}

	for _, container := range template.Spec.InitContainers {
		if container.Image != "" {
			imageSet[container.Image] = true
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				continue
			}
			envKeySet[env.Name] = true
		}
	}

	images := make([]string, 0, len(imageSet))
	for img := range imageSet {
		images = append(images, img)
	}

	envKeys := make([]string, 0, len(envKeySet))
	for key := range envKeySet {
		envKeys = append(envKeys, key)
	}

	return images, envKeys
}

// aggregatePodTemplateResources summarizes resource requests and limits
// across all containers in a pod template.
func aggregatePodTemplateResources(template corev1.PodTemplateSpec) map[string]interface{} {
	requests := make(map[string]int64)
	limits := make(map[string]int64)

	for _, container := range template.Spec.Containers {
		for resource, quantity := range container.Resources.Requests {
			requests[string(resource)] += quantity.MilliValue()
		}
		for resource, quantity := range container.Resources.Limits {
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

	return map[string]interface{}{
		"requests": requestStrs,
		"limits":   limitStrs,
	}
}

// labelSelectorToMap converts a LabelSelector to a flat map for observation storage.
func labelSelectorToMap(selector *metav1.LabelSelector) map[string]string {
	if selector == nil {
		return nil
	}
	result := make(map[string]string, len(selector.MatchLabels))
	for k, v := range selector.MatchLabels {
		result[k] = v
	}
	return result
}
