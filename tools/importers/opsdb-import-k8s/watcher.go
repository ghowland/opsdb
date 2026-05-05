
// === importers/opsdb-import-k8s/watcher.go ===
package k8s

// Watcher manages Kubernetes watch API subscriptions with level-triggered backstop.
// On connect: full list to establish current state.
// While connected: incremental watch events.
// On disconnect: re-list (level-triggered backstop), then resume watch.
type Watcher struct {
	// TODO: kubeconfig or in-cluster config
	// TODO: resource type being watched
	// TODO: namespace filter (empty = all namespaces)
	// TODO: last resource version for resume
	// TODO: callback for processing events
}

// Start begins watching a resource type. Performs initial list, then watches.
func (w *Watcher) Start(resourceType string, namespace string, callback func(event WatchEvent)) error {
	// TODO: full list of resource type (level-triggered baseline)
	// TODO: call callback for each existing resource as Added event
	// TODO: record resourceVersion from list response
	// TODO: start watch from that resourceVersion
	// TODO: on each event: call callback
	// TODO: on watch error/disconnect: re-list from scratch (level-triggered backstop)
	//       then resume watch from new resourceVersion
	return nil
}

// Stop gracefully stops the watcher.
func (w *Watcher) Stop() error {
	// TODO: signal watch goroutine to stop
	// TODO: wait for clean exit
	return nil
}

// WatchEvent represents a single Kubernetes watch event.
type WatchEvent struct {
	Type     string      // ADDED, MODIFIED, DELETED
	Resource interface{} // typed K8s resource object
}

