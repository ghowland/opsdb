//# tools/opsdb-runner-lib/lifecycle.go

go
package runnerlib

import (
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
	SpecName         string
	SpecVersion      int
	RunnerSpecID     int
	RunnerMachineID  int
	APIEndpoint      string
	AuthToken        string
	SiteID           int
	DryRun           bool
	MaxCycles        int    // 0 = unlimited
	CycleInterval    time.Duration
	CycleCount       int
	CurrentJobID     int
	BoundHits        []BoundHit
	ReportKeys       []ReportKeyDecl
	SpecDataJSON     map[string]interface{}
	Logger           *Logger
	Client           *APIClient
	shutdownCh       chan struct{}
	shutdownOnce     sync.Once
	mu               sync.Mutex
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
	// TODO: read OPSDB_API_ENDPOINT from environment, error if empty
	// TODO: read OPSDB_AUTH_TOKEN from environment, error if empty
	// TODO: create APIClient with endpoint and token
	// TODO: call client.GetEntity("runner_spec", name) to load spec
	// TODO: parse runner_data_json from spec for cycle_interval, max_cycles, dry_run, bounds
	// TODO: read runner_machine_id from environment or from spec lookup
	// TODO: call client.Search("runner_report_key", filters for this spec) to cache report keys
	// TODO: create Logger with runner context fields
	// TODO: set up shutdown channel and signal handler (SIGTERM, SIGINT)
	// TODO: log "runner initialized" with spec name and version
	// TODO: return populated RunnerConfig
	return nil, fmt.Errorf("not implemented")
}

// ShouldRun checks whether the runner should execute another cycle.
// Returns false if shutdown signal received or max cycles reached.
func ShouldRun(config *RunnerConfig) bool {
	// TODO: check shutdown channel (non-blocking select)
	// TODO: if max_cycles > 0 and cycle_count >= max_cycles, return false
	// TODO: return true
	return false
}

// WaitForNextCycle sleeps for the configured cycle interval or until
// a shutdown signal is received, whichever comes first.
func WaitForNextCycle(config *RunnerConfig) error {
	// TODO: select on:
	//   time.After(config.CycleInterval) -> return nil
	//   config.shutdownCh -> return shutdown error
	return nil
}

// StartCycle begins a new runner cycle. Creates a runner_job row via
// the API, increments cycle count, resets bound hits, returns the job ID.
func StartCycle(config *RunnerConfig) (int, error) {
	// TODO: increment config.CycleCount
	// TODO: reset config.BoundHits to empty
	// TODO: call client.WriteObservation to create runner_job row with:
	//   runner_spec_id, runner_machine_id, started_time=now, status=running
	// TODO: store job ID in config.CurrentJobID
	// TODO: update logger with new job ID via WithJobID
	// TODO: log "cycle started" with cycle number
	// TODO: return job ID
	return 0, nil
}

// FinishCycle completes a runner cycle. Updates the runner_job row with
// final status, duration, bound hits, and cycle summary.
func FinishCycle(config *RunnerConfig, status string, summary map[string]interface{}) error {
	// TODO: build runner_job update with:
	//   finished_time=now, status (completed/failed/bound_hit),
	//   duration, bound_hits as JSON, summary as JSON
	// TODO: call client.WriteObservation to update runner_job row
	// TODO: log "cycle finished" with status and summary
	return nil
}

// RecordBoundHit records that a specific bound was reached during the
// current cycle. Written to the runner_job row at cycle end.
func RecordBoundHit(config *RunnerConfig, boundName string, boundValue interface{}) {
	// TODO: lock config.mu
	// TODO: append BoundHit{boundName, boundValue, time.Now()} to config.BoundHits
	// TODO: log "bound hit" with name and value at warn level
	// TODO: unlock
}

// Shutdown initiates graceful shutdown. Signals the run loop to stop
// after the current cycle completes. Safe to call multiple times.
func Shutdown(config *RunnerConfig) {
	// TODO: config.shutdownOnce.Do: close config.shutdownCh
	// TODO: log "shutdown initiated"
}

// setupSignalHandler listens for SIGTERM and SIGINT and calls Shutdown.
func setupSignalHandler(config *RunnerConfig) {
	// TODO: make signal channel, buffered 1
	// TODO: signal.Notify for syscall.SIGTERM, syscall.SIGINT
	// TODO: goroutine: wait on signal channel, call Shutdown(config)
	_ = os.Getpid()
	_ = signal.Notify
	_ = syscall.SIGTERM
}

