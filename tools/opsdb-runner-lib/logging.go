//# tools/opsdb-runner-lib/logging.go

go
package runnerlib

import (
	"fmt"
	"time"
)

// Logger provides structured logging with runner context. Every log line
// includes timestamp, severity, runner_job_id, correlation_id, runner spec
// name and version, runner_machine_id, and source location.
type Logger struct {
	SpecName        string
	SpecVersion     int
	RunnerMachineID int
	JobID           int
	CorrelationID   string
	// TODO: output destination (os.Stdout, syslog, etc.)
	// TODO: minimum severity level
}

// LogField is a key-value pair for structured log data.
type LogField struct {
	Key   string
	Value interface{}
}

// Field creates a structured log field.
func Field(key string, value interface{}) LogField {
	return LogField{Key: key, Value: value}
}

// NewLogger creates a logger with runner context fields populated
// from the RunnerConfig.
func NewLogger(config *RunnerConfig) *Logger {
	// TODO: create logger with:
	//   SpecName = config.SpecName
	//   SpecVersion = config.SpecVersion
	//   RunnerMachineID = config.RunnerMachineID
	//   JobID = 0 (set per cycle via WithJobID)
	//   CorrelationID = generated UUID
	// TODO: read log level from OPSDB_LOG_LEVEL env var, default "info"
	// TODO: return logger
	return nil
}

// WithJobID returns a new logger with the runner_job_id set for the
// current cycle. Called at cycle start after runner_job row is created.
func (l *Logger) WithJobID(jobID int) *Logger {
	// TODO: shallow copy logger
	// TODO: set JobID = jobID
	// TODO: return copy
	return nil
}

// Info logs at info severity.
func (l *Logger) Info(msg string, fields ...LogField) {
	// TODO: call emit("info", msg, fields)
}

// Warn logs at warn severity.
func (l *Logger) Warn(msg string, fields ...LogField) {
	// TODO: call emit("warn", msg, fields)
}

// Error logs at error severity.
func (l *Logger) Error(msg string, fields ...LogField) {
	// TODO: call emit("error", msg, fields)
}

// Debug logs at debug severity.
func (l *Logger) Debug(msg string, fields ...LogField) {
	// TODO: call emit("debug", msg, fields)
}

// emit formats and writes one structured log line.
func (l *Logger) emit(severity string, msg string, fields []LogField) {
	// TODO: build structured log entry as JSON:
	//   "timestamp": time.Now().UTC().Format(time.RFC3339Nano)
	//   "severity": severity
	//   "message": msg
	//   "runner_spec": l.SpecName
	//   "runner_spec_version": l.SpecVersion
	//   "runner_machine_id": l.RunnerMachineID
	//   "runner_job_id": l.JobID (if > 0)
	//   "correlation_id": l.CorrelationID
	//   plus all extra fields from fields parameter
	// TODO: serialize to JSON
	// TODO: write to output destination (one line, newline terminated)
	_ = fmt.Sprintf("%s %s %s", time.Now().Format(time.RFC3339), severity, msg)
}


