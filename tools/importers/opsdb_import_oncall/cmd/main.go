// === importers/opsdb_import_oncall/cmd/main.go ===
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	oncall "github.com/ghowland/opsdb/tools/importers/opsdb_import_oncall"
	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

type CycleSummary struct {
	SchedulesWritten   int
	AssignmentsWritten int
	EscalationsWritten int
	StepsWritten       int
	Errors             []string
}

func main() {
	dosPath := flag.String("dos", "", "path to DOS config directory")
	flag.Parse()
	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "error: --dos flag is required\n")
		os.Exit(2)
	}

	config, err := runner.Init("opsdb-import-oncall", *dosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize: %v\n", err)
		os.Exit(2)
	}
	defer runner.Shutdown(config)

	logger := runner.NewLogger(config)
	logger.Info("on-call importer starting", runner.Field{Key: "dos_path", Value: *dosPath})

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
				runner.Field{Key: "schedules_written", Value: summary.SchedulesWritten},
				runner.Field{Key: "assignments_written", Value: summary.AssignmentsWritten},
				runner.Field{Key: "escalations_written", Value: summary.EscalationsWritten},
				runner.Field{Key: "steps_written", Value: summary.StepsWritten},
				runner.Field{Key: "error_count", Value: len(summary.Errors)},
			)
			runner.FinishCycle(config, jobID, "succeeded", summary)
		}
		runner.WaitForNextCycle(config)
	}

	logger.Info("on-call importer stopped")
	os.Exit(0)
}

func runCycle(config *runner.RunnerConfig, logger *runner.Logger) (*CycleSummary, error) {
	summary := &CycleSummary{}
	client := config.APIClient
	dryRun := runner.IsDryRun(config)

	// GET phase: determine backend and build import config
	backend := config.SpecData.StringOrDefault("backend", "")
	if backend == "" {
		return summary, fmt.Errorf("get phase failed: backend not configured in runner_spec (expected pagerduty or opsgenie)")
	}

	logger.Info("get phase complete",
		runner.Field{Key: "backend", Value: backend},
	)

	importConfig := &oncall.ImportConfig{
		Backend:    backend,
		APIToken:   config.ResolveCredential("api_token"),
		BaseURL:    config.SpecData.StringOrDefault("base_url", ""),
		BatchSize:  config.SpecData.IntOrDefault("batch_size", 100),
		MaxRetries: config.SpecData.IntOrDefault("max_retries", 3),
	}

	// ACT phase: call the appropriate backend importer
	var schedules []oncall.Observation
	var assignments []oncall.Observation
	var escalations []oncall.Observation
	var steps []oncall.Observation
	var err error

	switch backend {
	case "pagerduty":
		schedules, err = oncall.ImportPagerDutySchedules(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: pagerduty schedules: %w", err)
		}
		assignments, err = oncall.ImportPagerDutyAssignments(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: pagerduty assignments: %w", err)
		}
		escalations, err = oncall.ImportPagerDutyEscalations(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: pagerduty escalations: %w", err)
		}
		steps, err = oncall.ImportPagerDutyEscalationSteps(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: pagerduty escalation steps: %w", err)
		}

	case "opsgenie":
		schedules, err = oncall.ImportOpsgenieSchedules(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: opsgenie schedules: %w", err)
		}
		assignments, err = oncall.ImportOpsgenieAssignments(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: opsgenie assignments: %w", err)
		}
		escalations, err = oncall.ImportOpsgenieEscalations(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: opsgenie escalations: %w", err)
		}
		steps, err = oncall.ImportOpsgenieEscalationSteps(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: opsgenie escalation steps: %w", err)
		}

	default:
		return summary, fmt.Errorf("act phase failed: unsupported backend %q (expected pagerduty or opsgenie)", backend)
	}

	if dryRun {
		logger.Info("dry run: would write observations",
			runner.Field{Key: "schedules", Value: len(schedules)},
			runner.Field{Key: "assignments", Value: len(assignments)},
			runner.Field{Key: "escalations", Value: len(escalations)},
			runner.Field{Key: "steps", Value: len(steps)},
		)
		summary.SchedulesWritten = len(schedules)
		summary.AssignmentsWritten = len(assignments)
		summary.EscalationsWritten = len(escalations)
		summary.StepsWritten = len(steps)
		return summary, nil
	}

	// SET phase: write all observations to OpsDB via API
	for _, obs := range schedules {
		err := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("schedule %s: %v", obs.EntityID, err))
			logger.Warn("failed to write schedule observation",
				runner.Field{Key: "entity_id", Value: obs.EntityID},
				runner.Field{Key: "error", Value: err.Error()},
			)
			continue
		}
		summary.SchedulesWritten++
	}

	for _, obs := range assignments {
		err := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("assignment %s: %v", obs.EntityID, err))
			logger.Warn("failed to write assignment observation",
				runner.Field{Key: "entity_id", Value: obs.EntityID},
				runner.Field{Key: "error", Value: err.Error()},
			)
			continue
		}
		summary.AssignmentsWritten++
	}

	for _, obs := range escalations {
		err := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("escalation %s: %v", obs.EntityID, err))
			logger.Warn("failed to write escalation observation",
				runner.Field{Key: "entity_id", Value: obs.EntityID},
				runner.Field{Key: "error", Value: err.Error()},
			)
			continue
		}
		summary.EscalationsWritten++
	}

	for _, obs := range steps {
		err := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("step %s: %v", obs.EntityID, err))
			logger.Warn("failed to write step observation",
				runner.Field{Key: "entity_id", Value: obs.EntityID},
				runner.Field{Key: "error", Value: err.Error()},
			)
			continue
		}
		summary.StepsWritten++
	}

	return summary, nil
}
