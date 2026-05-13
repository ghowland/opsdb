// === importers/opsdb-import-k8s/cluster.go ===
package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sImportConfig holds Kubernetes importer cycle configuration.
type K8sImportConfig struct {
	Kubeconfig  string
	ClusterName string
	Namespaces  []string
	BatchSize   int
	MaxRetries  int
	UseWatchAPI bool
}

// Observation is the K8s importer observation structure.
type Observation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// ImportCluster reads cluster-level metadata and maps to k8s_cluster observations.
func ImportCluster(config *K8sImportConfig) ([]Observation, error) {
	var results []Observation

	clientset, restConfig, err := buildClient(config)
	if err != nil {
		return results, fmt.Errorf("building k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// read server version
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return results, fmt.Errorf("reading server version: %w", err)
	}

	// count nodes
	nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return results, fmt.Errorf("listing nodes for count: %w", err)
	}
	// Limit:1 gives us the total via remaining items count
	nodeCount := len(nodeList.Items)
	if nodeList.RemainingItemCount != nil {
		nodeCount += int(*nodeList.RemainingItemCount)
	}

	// count namespaces
	nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return results, fmt.Errorf("listing namespaces for count: %w", err)
	}
	nsCount := len(nsList.Items)
	if nsList.RemainingItemCount != nil {
		nsCount += int(*nsList.RemainingItemCount)
	}

	// determine distribution from node labels if available
	distribution := detectDistribution(clientset, ctx)

	// determine API endpoint
	apiEndpoint := restConfig.Host

	clusterName := config.ClusterName
	if clusterName == "" {
		clusterName = sanitizeK8sID(apiEndpoint)
	}

	obs := Observation{
		EntityType: "k8s_cluster",
		EntityID:   clusterName,
		StateKey:   "k8s_cluster_metadata",
		Value:      clusterName,
		DataJSON: map[string]interface{}{
			"name":             clusterName,
			"api_endpoint":     apiEndpoint,
			"server_version":   serverVersion.GitVersion,
			"major_version":    serverVersion.Major,
			"minor_version":    serverVersion.Minor,
			"platform":         serverVersion.Platform,
			"go_version":       serverVersion.GoVersion,
			"build_date":       serverVersion.BuildDate,
			"git_commit":       serverVersion.GitCommit,
			"compiler":         serverVersion.Compiler,
			"distribution":     distribution,
			"node_count":       nodeCount,
			"namespace_count":  nsCount,
		},
	}
	results = append(results, obs)

	return results, nil
}

// detectDistribution inspects node labels to determine the K8s distribution.
func detectDistribution(clientset *kubernetes.Clientset, ctx context.Context) string {
	nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil || len(nodeList.Items) == 0 {
		return "unknown"
	}

	node := nodeList.Items[0]
	labels := node.Labels

	// check for well-known distribution labels
	if _, ok := labels["eks.amazonaws.com/nodegroup"]; ok {
		return "eks"
	}
	if _, ok := labels["cloud.google.com/gke-nodepool"]; ok {
		return "gke"
	}
	if v, ok := labels["kubernetes.azure.com/cluster"]; ok && v != "" {
		return "aks"
	}
	if _, ok := labels["node.openshift.io/os_id"]; ok {
		return "openshift"
	}
	if _, ok := labels["microk8s.io/cluster"]; ok {
		return "microk8s"
	}
	if _, ok := labels["minikube.k8s.io/name"]; ok {
		return "minikube"
	}

	// check kubelet version string for hints
	kubeletVersion := node.Status.NodeInfo.KubeletVersion
	if strings.Contains(kubeletVersion, "-eks-") {
		return "eks"
	}
	if strings.Contains(kubeletVersion, "-gke.") {
		return "gke"
	}
	if strings.Contains(kubeletVersion, "+k3s") {
		return "k3s"
	}
	if strings.Contains(kubeletVersion, "+rke2") {
		return "rke2"
	}

	// check container runtime for distribution hints
	containerRuntime := node.Status.NodeInfo.ContainerRuntimeVersion
	if strings.Contains(containerRuntime, "k3s") {
		return "k3s"
	}

	return "upstream"
}

// buildClient creates a Kubernetes clientset from the import config.
func buildClient(config *K8sImportConfig) (*kubernetes.Clientset, *rest.Config, error) {
	var restConfig *rest.Config
	var err error

	if config.Kubeconfig == "" || config.Kubeconfig == "in-cluster" {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("building in-cluster config: %w", err)
		}
	} else {
		restConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig)
		if err != nil {
			return nil, nil, fmt.Errorf("building config from kubeconfig %s: %w", config.Kubeconfig, err)
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("creating clientset: %w", err)
	}

	return clientset, restConfig, nil
}

// sanitizeK8sID replaces characters unsuitable for entity IDs with underscores.
func sanitizeK8sID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
