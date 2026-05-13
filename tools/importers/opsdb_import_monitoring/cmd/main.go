// === importers/opsdb_import_monitoring/cmd/main.go ===
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	monitoring "github.com/ghowland/opsdb/tools/importers/opsdb_import_monitoring"
	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

type CycleSummary struct {
	PrometheusConfigsWritten int
	MonitorsWritten          int
	AlertsWritten            int
	MetricsWritten           int
	Errors                   []string
}

func main() {
	dosPath := flag.String("dos", "", "path to DOS config directory")
	flag.Parse()
	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "error: --dos flag is required\n")
		os.Exit(2)
	}

	config, err := runner.Init("opsdb-import-monitoring", *dosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize: %v\n", err)
		os.Exit(2)
	}
	defer runner.Shutdown(config)

	logger := runner.NewLogger(config)
	logger.Info("monitoring importer starting", runner.Field{Key: "dos_path", Value: *dosPath})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", runner.Field{Key: "signal", Value: sig.String()})
		runner.RequestShutdown(config)
	}()

	for runner.ShouldRun(config) {
		runner.RefreshConfig(config)
		jobID, err := runner.StartCycle(config)
		if err != nil {
			logger.Error("failed to start cycle", runner.Field{Key: "error", Value: err.Error()})
			runner.WaitForNextCycle(config)
			continue
		}
		cycleLogger := logger.WithJobID(jobID)

		summary, err := runCycle(config, cycleLogger)
		if err != nil {
			cycleLogger.Error("cycle failed", runner.Field{Key: "error", Value: err.Error()})
			runner.FinishCycle(config, jobID, "failed", summary)
		} else {
			cycleLogger.Info("cycle completed",
				runner.Field{Key: "prometheus_configs_written", Value: summary.PrometheusConfigsWritten},
				runner.Field{Key: "monitors_written", Value: summary.MonitorsWritten},
				runner.Field{Key: "alerts_written", Value: summary.AlertsWritten},
				runner.Field{Key: "metrics_written", Value: summary.MetricsWritten},
				runner.Field{Key: "error_count", Value: len(summary.Errors)},
			)
			runner.FinishCycle(config, jobID, "succeeded", summary)
		}
		runner.WaitForNextCycle(config)
	}

	logger.Info("monitoring importer stopped")
	os.Exit(0)
}

func runCycle(config *runner.RunnerConfig, logger *runner.Logger) (*CycleSummary, error) {
	summary := &CycleSummary{}
	client := config.APIClient
	dryRun := runner.IsDryRun(config)

	// GET phase: determine backend and build import config
	backend := config.SpecData.StringOrDefault("backend", "")
	if backend == "" {
		return summary, fmt.Errorf("get phase failed: backend not configured in runner_spec (expected prometheus or datadog)")
	}

	logger.Info("get phase complete",
		runner.Field{Key: "backend", Value: backend},
	)

	importConfig := &monitoring.ImportConfig{
		Backend:        backend,
		APIToken:       config.ResolveCredential("api_token"),
		BaseURL:        config.SpecData.StringOrDefault("base_url", ""),
		BatchSize:      config.SpecData.IntOrDefault("batch_size", 100),
		MaxRetries:     config.SpecData.IntOrDefault("max_retries", 3),
		MetricPrefixes: config.SpecData.StringListOrDefault("metric_prefixes", nil),
		ScrapeInterval: config.SpecData.IntOrDefault("scrape_interval_seconds", 60),
		AppKey:         config.ResolveCredential("app_key"),
	}

	// ACT phase: call the appropriate backend importer
	var promConfigs []monitoring.Observation
	var monitors []monitoring.Observation
	var alerts []monitoring.Observation
	var metrics []monitoring.Observation
	var err error

	switch backend {
	case "prometheus":
		promConfigs, err = monitoring.ImportPrometheusConfigs(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: prometheus configs: %w", err)
		}
		alerts, err = monitoring.ImportPrometheusAlerts(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: prometheus alerts: %w", err)
		}
		metrics, err = monitoring.ImportPrometheusMetrics(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: prometheus metrics: %w", err)
		}

	case "datadog":
		monitors, err = monitoring.ImportDatadogMonitors(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: datadog monitors: %w", err)
		}
		alerts, err = monitoring.ImportDatadogAlerts(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: datadog alerts: %w", err)
		}
		metrics, err = monitoring.ImportDatadogMetrics(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: datadog metrics: %w", err)
		}

	default:
		return summary, fmt.Errorf("act phase failed: unsupported backend %q (expected prometheus or datadog)", backend)
	}

	if dryRun {
		logger.Info("dry run: would write observations",
			runner.Field{Key: "prometheus_configs", Value: len(promConfigs)},
			runner.Field{Key: "monitors", Value: len(monitors)},
			runner.Field{Key: "alerts", Value: len(alerts)},
			runner.Field{Key: "metrics", Value: len(metrics)},
		)
		summary.PrometheusConfigsWritten = len(promConfigs)
		summary.MonitorsWritten = len(monitors)
		summary.AlertsWritten = len(alerts)
		summary.MetricsWritten = len(metrics)
		return summary, nil
	}

	// SET phase: write all observations to OpsDB via API
	for _, obs := range promConfigs {
		writeErr := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
		if writeErr != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("prometheus_config %s: %v", obs.EntityID, writeErr))
			logger.Warn("failed to write prometheus_config observation",
				runner.Field{Key: "entity_id", Value: obs.EntityID},
				runner.Field{Key: "error", Value: writeErr.Error()},
			)
			continue
		}
		summary.PrometheusConfigsWritten++
	}

	for _, obs := range monitors {
		writeErr := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
		if writeErr != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("monitor %s: %v", obs.EntityID, writeErr))
			logger.Warn("failed to write monitor observation",
				runner.Field{Key: "entity_id", Value: obs.EntityID},
				runner.Field{Key: "error", Value: writeErr.Error()},
			)
			continue
		}
		summary.MonitorsWritten++
	}

	for _, obs := range alerts {
		writeErr := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
		if writeErr != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("alert %s: %v", obs.EntityID, writeErr))
			logger.Warn("failed to write alert observation",
				runner.Field{Key: "entity_id", Value: obs.EntityID},
				runner.Field{Key: "error", Value: writeErr.Error()},
			)
			continue
		}
		summary.AlertsWritten++
	}

	for _, obs := range metrics {
		writeErr := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
		if writeErr != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("metric %s: %v", obs.EntityID, writeErr))
			logger.Warn("failed to write metric observation",
				runner.Field{Key: "entity_id", Value: obs.EntityID},
				runner.Field{Key: "error", Value: writeErr.Error()},
			)
			continue
		}
		summary.MetricsWritten++
	}

	return summary, nil
}
