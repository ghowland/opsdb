
// === importers/opsdb-import-k8s/cluster.go ===
package k8s

// ImportCluster reads cluster-level metadata and maps to k8s_cluster observations.
func ImportCluster(config *K8sImportConfig) ([]Observation, error) {
	// TODO: read server version via /version endpoint
	// TODO: determine distribution from server headers or node labels
	// TODO: read API endpoint from kubeconfig
	// TODO: count nodes
	// TODO: create k8s_cluster observation
	return nil, nil
}

// K8sImportConfig holds Kubernetes importer cycle configuration.
type K8sImportConfig struct {
	Kubeconfig     string // path or in-cluster
	ClusterName    string
	Namespaces     []string // empty = all
	BatchSize      int
	MaxRetries     int
	UseWatchAPI    bool
}

// Observation is the K8s importer observation structure.
type Observation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

