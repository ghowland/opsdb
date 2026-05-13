// === importers/opsdb_import_k8s/watcher.go ===
package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

// WatchConfig holds configuration for the watcher system.
type WatchConfig struct {
	ImportConfig  *K8sImportConfig
	ResourceTypes []string
	OnObservation func(obs Observation)
	OnError       func(resourceType string, err error)
	Logger        *runner.Logger
}

// Watcher manages Kubernetes watch API subscriptions with level-triggered backstop.
type Watcher struct {
	config    *WatchConfig
	clientset *kubernetes.Clientset
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// StartWatcher creates and starts a watcher for all configured resource types.
func StartWatcher(config *WatchConfig) (*Watcher, error) {
	clientset, _, err := buildClient(config.ImportConfig)
	if err != nil {
		return nil, fmt.Errorf("building k8s client for watcher: %w", err)
	}

	w := &Watcher{
		config:    config,
		clientset: clientset,
		stopCh:    make(chan struct{}),
	}

	for _, resourceType := range config.ResourceTypes {
		w.wg.Add(1)
		go w.watchResource(resourceType)
	}

	return w, nil
}

// Stop gracefully stops all watch goroutines and waits for them to exit.
func (w *Watcher) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// watchResource runs the list-then-watch loop for a single resource type.
// On disconnect or watch error, it re-lists from scratch (level-triggered backstop)
// then resumes watching.
func (w *Watcher) watchResource(resourceType string) {
	defer w.wg.Done()

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-w.stopCh:
			return
		default:
		}

		resourceVersion, err := w.listAndEmit(resourceType)
		if err != nil {
			w.config.OnError(resourceType, fmt.Errorf("initial list: %w", err))
			sleepOrStop(w.stopCh, backoff)
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}

		// reset backoff after successful list
		backoff = time.Second

		err = w.watchFromVersion(resourceType, resourceVersion)
		if err != nil {
			w.config.OnError(resourceType, fmt.Errorf("watch: %w", err))
		}

		// on any watch exit, re-list (level-triggered backstop)
		w.config.Logger.Info("watch ended, will re-list",
			runner.Field{Key: "resource_type", Value: resourceType},
		)
		sleepOrStop(w.stopCh, time.Second)
	}
}

// listAndEmit performs a full list of the resource type, emitting each item
// as an observation. Returns the resourceVersion from the list for watch resume.
func (w *Watcher) listAndEmit(resourceType string) (string, error) {
	namespaces, err := resolveNamespaces(w.clientset, context.Background(), w.config.ImportConfig.Namespaces)
	if err != nil {
		return "", fmt.Errorf("resolving namespaces: %w", err)
	}

	var resourceVersion string

	for _, ns := range namespaces {
		rv, err := w.listResourceInNamespace(resourceType, ns)
		if err != nil {
			return "", fmt.Errorf("listing %s in %s: %w", resourceType, ns, err)
		}
		if rv != "" {
			resourceVersion = rv
		}
	}

	return resourceVersion, nil
}

// listResourceInNamespace lists all items of a resource type in one namespace
// and emits observations for each.
func (w *Watcher) listResourceInNamespace(resourceType string, namespace string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var resourceVersion string

	switch resourceType {
	case "pod":
		continueToken := ""
		for {
			list, err := w.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				Limit:    int64(w.config.ImportConfig.BatchSize),
				Continue: continueToken,
			})
			if err != nil {
				return "", err
			}
			resourceVersion = list.ResourceVersion
			for i := range list.Items {
				w.emitPodObservation(&list.Items[i])
			}
			continueToken = list.Continue
			if continueToken == "" {
				break
			}
		}

	case "service":
		list, err := w.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", err
		}
		resourceVersion = list.ResourceVersion
		for i := range list.Items {
			w.emitServiceObservation(&list.Items[i])
		}

	case "configmap":
		continueToken := ""
		for {
			list, err := w.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
				Limit:    int64(w.config.ImportConfig.BatchSize),
				Continue: continueToken,
			})
			if err != nil {
				return "", err
			}
			resourceVersion = list.ResourceVersion
			for i := range list.Items {
				w.emitConfigMapObservation(&list.Items[i])
			}
			continueToken = list.Continue
			if continueToken == "" {
				break
			}
		}

	case "secret":
		continueToken := ""
		for {
			list, err := w.clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
				Limit:    int64(w.config.ImportConfig.BatchSize),
				Continue: continueToken,
			})
			if err != nil {
				return "", err
			}
			resourceVersion = list.ResourceVersion
			for i := range list.Items {
				w.emitSecretObservation(&list.Items[i])
			}
			continueToken = list.Continue
			if continueToken == "" {
				break
			}
		}

	case "namespace":
		list, err := w.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", err
		}
		resourceVersion = list.ResourceVersion
		for i := range list.Items {
			w.emitNamespaceObservation(&list.Items[i])
		}

	case "workload":
		// watch deployments as the primary workload type
		continueToken := ""
		for {
			list, err := w.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
				Limit:    int64(w.config.ImportConfig.BatchSize),
				Continue: continueToken,
			})
			if err != nil {
				return "", err
			}
			resourceVersion = list.ResourceVersion
			for i := range list.Items {
				w.emitDeploymentObservation(&list.Items[i])
			}
			continueToken = list.Continue
			if continueToken == "" {
				break
			}
		}

	default:
		return "", fmt.Errorf("unsupported watch resource type: %s", resourceType)
	}

	return resourceVersion, nil
}

// watchFromVersion starts a watch from the given resourceVersion and processes
// events until the watch ends or the watcher is stopped.
func (w *Watcher) watchFromVersion(resourceType string, resourceVersion string) error {
	namespaces, err := resolveNamespaces(w.clientset, context.Background(), w.config.ImportConfig.Namespaces)
	if err != nil {
		return fmt.Errorf("resolving namespaces: %w", err)
	}

	// for simplicity, watch the first namespace or all-namespaces
	// multi-namespace watching uses one goroutine per namespace
	if len(namespaces) == 1 {
		return w.watchNamespace(resourceType, namespaces[0], resourceVersion)
	}

	// all-namespaces watch
	return w.watchNamespace(resourceType, "", resourceVersion)
}

// watchNamespace watches a single resource type in a single namespace.
func (w *Watcher) watchNamespace(resourceType string, namespace string, resourceVersion string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// stop watch when watcher is stopped
	go func() {
		<-w.stopCh
		cancel()
	}()

	opts := metav1.ListOptions{
		ResourceVersion: resourceVersion,
		Watch:           true,
	}

	var watcher watch.Interface
	var err error

	switch resourceType {
	case "pod":
		watcher, err = w.clientset.CoreV1().Pods(namespace).Watch(ctx, opts)
	case "service":
		watcher, err = w.clientset.CoreV1().Services(namespace).Watch(ctx, opts)
	case "configmap":
		watcher, err = w.clientset.CoreV1().ConfigMaps(namespace).Watch(ctx, opts)
	case "secret":
		watcher, err = w.clientset.CoreV1().Secrets(namespace).Watch(ctx, opts)
	case "namespace":
		watcher, err = w.clientset.CoreV1().Namespaces().Watch(ctx, opts)
	case "workload":
		watcher, err = w.clientset.AppsV1().Deployments(namespace).Watch(ctx, opts)
	default:
		return fmt.Errorf("unsupported watch resource type: %s", resourceType)
	}

	if err != nil {
		return fmt.Errorf("starting watch for %s: %w", resourceType, err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-w.stopCh:
			return nil

		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed for %s", resourceType)
			}

			if event.Type == watch.Error {
				return fmt.Errorf("watch error event for %s", resourceType)
			}

			if event.Type == watch.Deleted {
				// for deleted resources, emit with a deleted marker
				// the observation will still be written; the reaper handles cleanup
			}

			w.emitWatchEvent(resourceType, event)
		}
	}
}

// emitWatchEvent dispatches a watch event to the appropriate observation emitter.
func (w *Watcher) emitWatchEvent(resourceType string, event watch.Event) {
	switch resourceType {
	case "pod":
		if pod, ok := event.Object.(*corev1.Pod); ok {
			w.emitPodObservation(pod)
		}
	case "service":
		if svc, ok := event.Object.(*corev1.Service); ok {
			w.emitServiceObservation(svc)
		}
	case "configmap":
		if cm, ok := event.Object.(*corev1.ConfigMap); ok {
			w.emitConfigMapObservation(cm)
		}
	case "secret":
		if secret, ok := event.Object.(*corev1.Secret); ok {
			w.emitSecretObservation(secret)
		}
	case "namespace":
		if ns, ok := event.Object.(*corev1.Namespace); ok {
			w.emitNamespaceObservation(ns)
		}
	case "workload":
		if deploy, ok := event.Object.(*appsv1.Deployment); ok {
			w.emitDeploymentObservation(deploy)
		}
	}
}

// emitPodObservation builds and emits a k8s_pod observation from a Pod object.
func (w *Watcher) emitPodObservation(pod *corev1.Pod) {
	clusterName := w.config.ImportConfig.ClusterName
	podID := fmt.Sprintf("%s_%s_%s", clusterName, pod.Namespace, pod.Name)

	containers := summarizeContainerStatuses(pod.Status.ContainerStatuses)
	totalRestarts := countTotalRestarts(pod.Status.ContainerStatuses)
	ownerKind, ownerName := extractOwnerReference(pod.OwnerReferences)

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
			"name":             pod.Name,
			"namespace":        pod.Namespace,
			"cluster_name":     clusterName,
			"uid":              string(pod.UID),
			"phase":            string(pod.Status.Phase),
			"node_name":        pod.Spec.NodeName,
			"pod_ip":           pod.Status.PodIP,
			"start_time":       startTime,
			"total_restarts":   totalRestarts,
			"container_count":  len(pod.Spec.Containers),
			"containers":       containers,
			"owner_kind":       ownerKind,
			"owner_name":       ownerName,
			"labels":           pod.Labels,
			"resource_version": pod.ResourceVersion,
		},
	}
	w.config.OnObservation(obs)
}

// emitServiceObservation builds and emits a k8s_service observation.
func (w *Watcher) emitServiceObservation(svc *corev1.Service) {
	clusterName := w.config.ImportConfig.ClusterName
	svcID := fmt.Sprintf("%s_%s_%s", clusterName, svc.Namespace, svc.Name)

	svcType := "ClusterIP"
	if svc.Spec.ClusterIP == "None" {
		svcType = "Headless"
	} else {
		svcType = string(svc.Spec.Type)
	}

	obs := Observation{
		EntityType: "k8s_service",
		EntityID:   svcID,
		StateKey:   "k8s_service",
		Value:      svc.Name,
		DataJSON: map[string]interface{}{
			"name":             svc.Name,
			"namespace":        svc.Namespace,
			"cluster_name":     clusterName,
			"service_type":     svcType,
			"cluster_ip":       svc.Spec.ClusterIP,
			"port_count":       len(svc.Spec.Ports),
			"labels":           svc.Labels,
			"resource_version": svc.ResourceVersion,
		},
	}
	w.config.OnObservation(obs)
}

// emitConfigMapObservation builds and emits a k8s_config_map observation.
func (w *Watcher) emitConfigMapObservation(cm *corev1.ConfigMap) {
	clusterName := w.config.ImportConfig.ClusterName
	cmID := fmt.Sprintf("%s_%s_%s", clusterName, cm.Namespace, cm.Name)

	dataKeys := make([]string, 0, len(cm.Data))
	for key := range cm.Data {
		dataKeys = append(dataKeys, key)
	}

	obs := Observation{
		EntityType: "k8s_config_map",
		EntityID:   cmID,
		StateKey:   "k8s_configmap",
		Value:      cm.Name,
		DataJSON: map[string]interface{}{
			"name":             cm.Name,
			"namespace":        cm.Namespace,
			"cluster_name":     clusterName,
			"data_key_count":   len(cm.Data),
			"data_keys":        dataKeys,
			"labels":           cm.Labels,
			"resource_version": cm.ResourceVersion,
		},
	}
	w.config.OnObservation(obs)
}

// emitSecretObservation builds and emits a k8s_secret_reference observation.
// Never reads or emits secret values.
func (w *Watcher) emitSecretObservation(secret *corev1.Secret) {
	clusterName := w.config.ImportConfig.ClusterName
	secretID := fmt.Sprintf("%s_%s_%s", clusterName, secret.Namespace, secret.Name)

	dataKeys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		dataKeys = append(dataKeys, key)
	}

	obs := Observation{
		EntityType: "k8s_secret_reference",
		EntityID:   secretID,
		StateKey:   "k8s_secret_reference",
		Value:      secret.Name,
		DataJSON: map[string]interface{}{
			"name":             secret.Name,
			"namespace":        secret.Namespace,
			"cluster_name":     clusterName,
			"secret_type":      string(secret.Type),
			"data_key_count":   len(secret.Data),
			"data_keys":        dataKeys,
			"labels":           secret.Labels,
			"resource_version": secret.ResourceVersion,
		},
	}
	w.config.OnObservation(obs)
}

// emitNamespaceObservation builds and emits a k8s_namespace observation.
func (w *Watcher) emitNamespaceObservation(ns *corev1.Namespace) {
	clusterName := w.config.ImportConfig.ClusterName
	nsID := fmt.Sprintf("%s_%s", clusterName, ns.Name)

	obs := Observation{
		EntityType: "k8s_namespace",
		EntityID:   nsID,
		StateKey:   "k8s_namespace",
		Value:      ns.Name,
		DataJSON: map[string]interface{}{
			"name":             ns.Name,
			"cluster_name":     clusterName,
			"status":           string(ns.Status.Phase),
			"labels":           ns.Labels,
			"resource_version": ns.ResourceVersion,
		},
	}
	w.config.OnObservation(obs)
}

// emitDeploymentObservation builds and emits a k8s_workload observation from a Deployment.
func (w *Watcher) emitDeploymentObservation(deploy *appsv1.Deployment) {
	clusterName := w.config.ImportConfig.ClusterName
	workloadID := fmt.Sprintf("%s_%s_%s_deployment", clusterName, deploy.Namespace, deploy.Name)

	var replicas, readyReplicas, updatedReplicas, availableReplicas int
	if deploy.Spec.Replicas != nil {
		replicas = int(*deploy.Spec.Replicas)
	}
	readyReplicas = int(deploy.Status.ReadyReplicas)
	updatedReplicas = int(deploy.Status.UpdatedReplicas)
	availableReplicas = int(deploy.Status.AvailableReplicas)

	obs := Observation{
		EntityType: "k8s_workload",
		EntityID:   workloadID,
		StateKey:   "k8s_workload",
		Value:      deploy.Name,
		DataJSON: map[string]interface{}{
			"name":               deploy.Name,
			"namespace":          deploy.Namespace,
			"cluster_name":       clusterName,
			"workload_type":      "deployment",
			"replicas":           replicas,
			"ready_replicas":     readyReplicas,
			"updated_replicas":   updatedReplicas,
			"available_replicas": availableReplicas,
			"labels":             deploy.Labels,
			"resource_version":   deploy.ResourceVersion,
		},
	}
	w.config.OnObservation(obs)
}

// sleepOrStop sleeps for the given duration or returns early if stopCh is closed.
func sleepOrStop(stopCh <-chan struct{}, d time.Duration) {
	select {
	case <-stopCh:
	case <-time.After(d):
	}
}

// nextBackoff doubles the backoff duration up to a maximum.
func nextBackoff(current time.Duration, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}
