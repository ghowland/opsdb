// === importers/opsdb_import_k8s/cmd/main.go ===
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	k8s "github.com/ghowland/opsdb/tools/importers/opsdb_import_k8s"
	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

type CycleSummary struct {
	ClustersWritten   int
	NodesWritten      int
	NamespacesWritten int
	WorkloadsWritten  int
	PodsWritten       int
	HelmWritten       int
	ConfigMapsWritten int
	SecretsWritten    int
	ServicesWritten   int
	Errors            []string
}

func main() {
	dosPath := flag.String("dos", "", "path to DOS config directory")
	flag.Parse()
	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "error: --dos flag is required\n")
		os.Exit(2)
	}

	config, err := runner.Init("opsdb-import-k8s", *dosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize: %v\n", err)
		os.Exit(2)
	}
	defer runner.Shutdown(config)

	logger := runner.NewLogger(config)
	logger.Info("kubernetes importer starting", runner.Field{Key: "dos_path", Value: *dosPath})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", runner.Field{Key: "signal", Value: sig.String()})
		runner.RequestShutdown(config)
	}()

	useWatch := config.SpecData.BoolOrDefault("use_watch_api", false)

	if useWatch {
		runWatchMode(config, logger)
	} else {
		runPollMode(config, logger)
	}

	logger.Info("kubernetes importer stopped")
	os.Exit(0)
}

func runPollMode(config *runner.RunnerConfig, logger *runner.Logger) {
	for runner.ShouldRun(config) {
		runner.RefreshConfig(config)
		jobID, err := runner.StartCycle(config)
		if err != nil {
			logger.Error("failed to start cycle", runner.Field{Key: "error", Value: err.Error()})
			runner.WaitForNextCycle(config)
			continue
		}
		cycleLogger := logger.WithJobID(jobID)

		summary, err := runPollCycle(config, cycleLogger)
		if err != nil {
			cycleLogger.Error("cycle failed", runner.Field{Key: "error", Value: err.Error()})
			runner.FinishCycle(config, jobID, "failed", summary)
		} else {
			cycleLogger.Info("cycle completed",
				runner.Field{Key: "clusters_written", Value: summary.ClustersWritten},
				runner.Field{Key: "nodes_written", Value: summary.NodesWritten},
				runner.Field{Key: "namespaces_written", Value: summary.NamespacesWritten},
				runner.Field{Key: "workloads_written", Value: summary.WorkloadsWritten},
				runner.Field{Key: "pods_written", Value: summary.PodsWritten},
				runner.Field{Key: "helm_written", Value: summary.HelmWritten},
				runner.Field{Key: "configmaps_written", Value: summary.ConfigMapsWritten},
				runner.Field{Key: "secrets_written", Value: summary.SecretsWritten},
				runner.Field{Key: "services_written", Value: summary.ServicesWritten},
				runner.Field{Key: "error_count", Value: len(summary.Errors)},
			)
			runner.FinishCycle(config, jobID, "succeeded", summary)
		}
		runner.WaitForNextCycle(config)
	}
}

func runPollCycle(config *runner.RunnerConfig, logger *runner.Logger) (*CycleSummary, error) {
	summary := &CycleSummary{}
	client := config.APIClient
	dryRun := runner.IsDryRun(config)

	// GET phase: build K8s import config from runner spec
	importConfig := buildImportConfig(config)

	logger.Info("get phase complete",
		runner.Field{Key: "kubeconfig", Value: importConfig.Kubeconfig},
		runner.Field{Key: "cluster_name", Value: importConfig.ClusterName},
		runner.Field{Key: "namespace_count", Value: len(importConfig.Namespaces)},
	)

	resourceTypes := config.SpecData.StringListOrDefault("resource_types", []string{
		"cluster", "node", "namespace", "workload", "pod", "helm", "configmap", "secret", "service",
	})
	enabled := make(map[string]bool, len(resourceTypes))
	for _, rt := range resourceTypes {
		enabled[rt] = true
	}

	// ACT phase: call each enabled resource importer
	type importResult struct {
		name    string
		obs     []k8s.Observation
		counter *int
	}

	var allImports []importResult

	if enabled["cluster"] {
		obs, err := k8s.ImportCluster(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: cluster: %w", err)
		}
		allImports = append(allImports, importResult{"cluster", obs, &summary.ClustersWritten})
	}

	if enabled["node"] {
		obs, err := k8s.ImportNodes(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: nodes: %w", err)
		}
		allImports = append(allImports, importResult{"node", obs, &summary.NodesWritten})
	}

	if enabled["namespace"] {
		obs, err := k8s.ImportNamespaces(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: namespaces: %w", err)
		}
		allImports = append(allImports, importResult{"namespace", obs, &summary.NamespacesWritten})
	}

	if enabled["workload"] {
		obs, err := k8s.ImportWorkloads(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: workloads: %w", err)
		}
		allImports = append(allImports, importResult{"workload", obs, &summary.WorkloadsWritten})
	}

	if enabled["pod"] {
		obs, err := k8s.ImportPods(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: pods: %w", err)
		}
		allImports = append(allImports, importResult{"pod", obs, &summary.PodsWritten})
	}

	if enabled["helm"] {
		obs, err := k8s.ImportHelm(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: helm: %w", err)
		}
		allImports = append(allImports, importResult{"helm", obs, &summary.HelmWritten})
	}

	if enabled["configmap"] {
		obs, err := k8s.ImportConfigMaps(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: configmaps: %w", err)
		}
		allImports = append(allImports, importResult{"configmap", obs, &summary.ConfigMapsWritten})
	}

	if enabled["secret"] {
		obs, err := k8s.ImportSecrets(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: secrets: %w", err)
		}
		allImports = append(allImports, importResult{"secret", obs, &summary.SecretsWritten})
	}

	if enabled["service"] {
		obs, err := k8s.ImportServices(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: services: %w", err)
		}
		allImports = append(allImports, importResult{"service", obs, &summary.ServicesWritten})
	}

	if dryRun {
		for _, ir := range allImports {
			logger.Info("dry run: would write observations",
				runner.Field{Key: "resource_type", Value: ir.name},
				runner.Field{Key: "count", Value: len(ir.obs)},
			)
			*ir.counter = len(ir.obs)
		}
		return summary, nil
	}

	// SET phase: write all observations to OpsDB via API
	for _, ir := range allImports {
		for _, obs := range ir.obs {
			writeErr := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
			if writeErr != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s %s: %v", ir.name, obs.EntityID, writeErr))
				logger.Warn("failed to write observation",
					runner.Field{Key: "resource_type", Value: ir.name},
					runner.Field{Key: "entity_id", Value: obs.EntityID},
					runner.Field{Key: "error", Value: writeErr.Error()},
				)
				continue
			}
			*ir.counter++
		}
	}

	return summary, nil
}

func runWatchMode(config *runner.RunnerConfig, logger *runner.Logger) {
	importConfig := buildImportConfig(config)

	jobID, err := runner.StartCycle(config)
	if err != nil {
		logger.Error("failed to start watch cycle", runner.Field{Key: "error", Value: err.Error()})
		return
	}
	cycleLogger := logger.WithJobID(jobID)

	resourceTypes := config.SpecData.StringListOrDefault("resource_types", []string{
		"namespace", "workload", "pod", "configmap", "secret", "service",
	})

	watchConfig := &k8s.WatchConfig{
		ImportConfig:  importConfig,
		ResourceTypes: resourceTypes,
		OnObservation: func(obs k8s.Observation) {
			writeErr := config.APIClient.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
			if writeErr != nil {
				cycleLogger.Warn("failed to write watch observation",
					runner.Field{Key: "entity_type", Value: obs.EntityType},
					runner.Field{Key: "entity_id", Value: obs.EntityID},
					runner.Field{Key: "error", Value: writeErr.Error()},
				)
			}
		},
		OnError: func(resourceType string, err error) {
			cycleLogger.Error("watch error",
				runner.Field{Key: "resource_type", Value: resourceType},
				runner.Field{Key: "error", Value: err.Error()},
			)
		},
		Logger: cycleLogger,
	}

	cycleLogger.Info("starting watch mode",
		runner.Field{Key: "resource_types", Value: len(resourceTypes)},
		runner.Field{Key: "cluster_name", Value: importConfig.ClusterName},
	)

	watcher, err := k8s.StartWatcher(watchConfig)
	if err != nil {
		cycleLogger.Error("failed to start watcher", runner.Field{Key: "error", Value: err.Error()})
		runner.FinishCycle(config, jobID, "failed", nil)
		return
	}

	// block until shutdown is requested
	for runner.ShouldRun(config) {
		runner.RefreshConfig(config)
		newConfig := buildImportConfig(config)
		if newConfig.Kubeconfig != importConfig.Kubeconfig || newConfig.ClusterName != importConfig.ClusterName {
			cycleLogger.Info("config changed, restarting watcher")
			watcher.Stop()
			importConfig = newConfig
			watchConfig.ImportConfig = importConfig
			watcher, err = k8s.StartWatcher(watchConfig)
			if err != nil {
				cycleLogger.Error("failed to restart watcher", runner.Field{Key: "error", Value: err.Error()})
				runner.FinishCycle(config, jobID, "failed", nil)
				return
			}
		}
		runner.WaitForNextCycle(config)
	}

	watcher.Stop()
	cycleLogger.Info("watch mode stopped")
	runner.FinishCycle(config, jobID, "succeeded", nil)
}

func buildImportConfig(config *runner.RunnerConfig) *k8s.K8sImportConfig {
	return &k8s.K8sImportConfig{
		Kubeconfig:  config.SpecData.StringOrDefault("kubeconfig", ""),
		ClusterName: config.SpecData.StringOrDefault("cluster_name", ""),
		Namespaces:  config.SpecData.StringListOrDefault("namespaces", nil),
		BatchSize:   config.SpecData.IntOrDefault("batch_size", 500),
		MaxRetries:  config.SpecData.IntOrDefault("max_retries", 3),
		UseWatchAPI: config.SpecData.BoolOrDefault("use_watch_api", false),
	}
}
