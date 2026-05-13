package runnerlib

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// RunnerConfig holds all runtime state for a runner instance.
// Populated by Init, read by every phase of the runner cycle.
type RunnerConfig struct {
	SpecName        string
	SpecVersion     int
	RunnerSpecID    int
	RunnerMachineID int
	APIEndpoint     string
	AuthToken       string
	SiteID          int
	DryRun          bool
	MaxCycles       int // 0 = unlimited
	CycleInterval   time.Duration
	CycleCount      int
	CurrentJobID    int
	BoundHits       []BoundHit
	ReportKeys      []ReportKeyDecl
	SpecDataJSON    map[string]interface{}
	Logger          *Logger
	Client          *APIClient
	shutdownCh      chan struct{}
	shutdownOnce    sync.Once
	mu              sync.Mutex
}

// BoundHit records a bound that was reached during a runner cycle.
type BoundHit struct {
	BoundName  string
	BoundValue interface{}
	HitTime    time.Time
}

// ReportKeyDecl holds a cached report key declaration for fail-fast validation.
type ReportKeyDecl struct {
	Key            string
	TargetTable    string
	ConstraintJSON map[string]interface{}
}

// Init loads the runner spec from OpsDB via the API client, initializes
// logging with runner context, sets up bound tracking and shutdown signal
// handling. Returns a fully populated RunnerConfig or error.
func Init(runnerSpecName string) (*RunnerConfig, error) {
	apiEndpoint, err := envOrError("OPSDB_API_ENDPOINT")
	if err != nil {
		return nil, fmt.Errorf("runner init: %w", err)
	}

	config, err := LoadRunnerConfig(runnerSpecName, apiEndpoint)
	if err != nil {
		return nil, fmt.Errorf("runner init: %w", err)
	}

	setupSignalHandler(config)

	config.Logger.Info("runner initialized",
		Field("spec_name", config.SpecName),
		Field("spec_version", config.SpecVersion),
		Field("runner_spec_id", config.RunnerSpecID),
		Field("runner_machine_id", config.RunnerMachineID),
		Field("site_id", config.SiteID),
		Field("cycle_interval", config.CycleInterval.String()),
		Field("max_cycles", config.MaxCycles),
		Field("is_dry_run", config.DryRun),
		Field("report_key_count", len(config.ReportKeys)),
	)

	return config, nil
}

// ShouldRun checks whether the runner should execute another cycle.
// Returns false if shutdown signal received or max cycles reached.
func ShouldRun(config *RunnerConfig) bool {
	// Non-blocking check on shutdown channel.
	select {
	case <-config.shutdownCh:
		return false
	default:
	}

	config.mu.Lock()
	defer config.mu.Unlock()

	if config.MaxCycles > 0 && config.CycleCount >= config.MaxCycles {
		return false
	}

	return true
}

// WaitForNextCycle sleeps for the configured cycle interval or until
// a shutdown signal is received, whichever comes first.
func WaitForNextCycle(config *RunnerConfig) error {
	config.Logger.Debug("waiting for next cycle",
		Field("interval", config.CycleInterval.String()),
	)

	select {
	case <-time.After(config.CycleInterval):
		return nil
	case <-config.shutdownCh:
		return fmt.Errorf("shutdown during wait")
	}
}

// StartCycle begins a new runner cycle. Creates a runner_job row via
// the API, increments cycle count, resets bound hits, returns the job ID.
func StartCycle(config *RunnerConfig) (int, error) {
	config.mu.Lock()
	config.CycleCount++
	config.BoundHits = nil
	cycleNum := config.CycleCount
	config.mu.Unlock()

	// Create runner_job row via API.
	jobData := map[string]interface{}{
		"runner_spec_id":    config.RunnerSpecID,
		"runner_machine_id": config.RunnerMachineID,
		"site_id":           config.SiteID,
		"started_time":      time.Now().UTC().Format(time.RFC3339Nano),
		"status":            "running",
		"cycle_number":      cycleNum,
	}

	result, err := config.Client.WriteObservation(&WriteObservationParams{
		TargetTable:  "runner_job",
		Key:          fmt.Sprintf("cycle:%d:%d", config.RunnerSpecID, cycleNum),
		Value:        "running",
		DataJSON:     jobData,
		RunnerJobID:  0, // not yet assigned — this creates the job
		AuthorityID:  0,
		ObservedTime: time.Now(),
	})
	if err != nil {
		return 0, fmt.Errorf("creating runner_job for cycle %d: %w", cycleNum, err)
	}

	jobID := result.RowID

	config.mu.Lock()
	config.CurrentJobID = jobID
	config.mu.Unlock()

	// Update logger and client with new job ID for this cycle.
	config.Logger = config.Logger.WithJobID(jobID)
	config.Client = config.Client.WithCorrelation(jobID, "")

	config.Logger.Info("cycle started",
		Field("cycle_number", cycleNum),
		Field("runner_job_id", jobID),
	)

	return jobID, nil
}

// FinishCycle completes a runner cycle. Updates the runner_job row with
// final status, duration, bound hits, and cycle summary.
func FinishCycle(config *RunnerConfig, status string, summary map[string]interface{}) error {
	config.mu.Lock()
	boundHits := make([]BoundHit, len(config.BoundHits))
	copy(boundHits, config.BoundHits)
	jobID := config.CurrentJobID
	config.mu.Unlock()

	// Serialize bound hits.
	var boundHitData []map[string]interface{}
	for _, bh := range boundHits {
		boundHitData = append(boundHitData, map[string]interface{}{
			"bound_name":  bh.BoundName,
			"bound_value": bh.BoundValue,
			"hit_time":    bh.HitTime.UTC().Format(time.RFC3339Nano),
		})
	}

	// If any bounds were hit and status is "completed", upgrade to "completed_with_bounds".
	if len(boundHits) > 0 && status == "completed" {
		status = "completed_with_bounds"
	}

	finishData := map[string]interface{}{
		"runner_job_id": jobID,
		"finished_time": time.Now().UTC().Format(time.RFC3339Nano),
		"status":        status,
	}
	if len(boundHitData) > 0 {
		finishData["bound_hits"] = boundHitData
	}
	if summary != nil {
		finishData["summary"] = summary
	}

	_, err := config.Client.WriteObservation(&WriteObservationParams{
		TargetTable:  "runner_job",
		Key:          fmt.Sprintf("finish:%d", jobID),
		Value:        status,
		DataJSON:     finishData,
		RunnerJobID:  jobID,
		AuthorityID:  0,
		ObservedTime: time.Now(),
	})
	if err != nil {
		config.Logger.Error("failed to write cycle finish",
			Field("runner_job_id", jobID),
			Field("error", err.Error()),
		)
		return fmt.Errorf("finishing cycle %d: %w", jobID, err)
	}

	// Log summary.
	summaryJSON := "{}"
	if summary != nil {
		if b, merr := json.Marshal(summary); merr == nil {
			summaryJSON = string(b)
		}
	}

	config.Logger.Info("cycle finished",
		Field("status", status),
		Field("runner_job_id", jobID),
		Field("bound_hits", len(boundHits)),
		Field("summary", summaryJSON),
	)

	return nil
}

// RecordBoundHit records that a specific bound was reached during the
// current cycle. Written to the runner_job row at cycle end.
func RecordBoundHit(config *RunnerConfig, boundName string, boundValue interface{}) {
	config.mu.Lock()
	config.BoundHits = append(config.BoundHits, BoundHit{
		BoundName:  boundName,
		BoundValue: boundValue,
		HitTime:    time.Now(),
	})
	config.mu.Unlock()

	config.Logger.Warn("bound hit",
		Field("bound_name", boundName),
		Field("bound_value", boundValue),
	)
}

// Shutdown initiates graceful shutdown. Signals the run loop to stop
// after the current cycle completes. Safe to call multiple times.
func Shutdown(config *RunnerConfig) {
	config.shutdownOnce.Do(func() {
		close(config.shutdownCh)
		config.Logger.Info("shutdown initiated")
	})
}

// setupSignalHandler listens for SIGTERM and SIGINT and calls Shutdown.
func setupSignalHandler(config *RunnerConfig) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		config.Logger.Info("received signal",
			Field("signal", sig.String()),
		)
		Shutdown(config)
	}()
}
