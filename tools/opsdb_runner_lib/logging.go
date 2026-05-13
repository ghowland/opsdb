package runnerlib

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

// severityLevel maps severity names to numeric levels for filtering.
var severityLevel = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
}

// Logger provides structured logging with runner context. Every log line
// includes timestamp, severity, runner_job_id, correlation_id, runner spec
// name and version, runner_machine_id, and source location.
type Logger struct {
	SpecName        string
	SpecVersion     int
	RunnerMachineID int
	JobID           int
	CorrelationID   string
	minLevel        int
	output          io.Writer
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
	level := "info"
	if envLevel := os.Getenv("OPSDB_LOG_LEVEL"); envLevel != "" {
		lower := strings.ToLower(envLevel)
		if _, ok := severityLevel[lower]; ok {
			level = lower
		}
	}

	return &Logger{
		SpecName:        config.SpecName,
		SpecVersion:     config.SpecVersion,
		RunnerMachineID: config.RunnerMachineID,
		JobID:           0,
		CorrelationID:   uuid.New().String(),
		minLevel:        severityLevel[level],
		output:          os.Stdout,
	}
}

// NewTestLogger creates a logger that writes to the provided writer.
// Used in tests to capture log output.
func NewTestLogger(w io.Writer) *Logger {
	return &Logger{
		SpecName:      "test",
		CorrelationID: "test-correlation",
		minLevel:      severityLevel["debug"],
		output:        w,
	}
}

// WithJobID returns a new logger with the runner_job_id set for the
// current cycle. Called at cycle start after runner_job row is created.
func (l *Logger) WithJobID(jobID int) *Logger {
	return &Logger{
		SpecName:        l.SpecName,
		SpecVersion:     l.SpecVersion,
		RunnerMachineID: l.RunnerMachineID,
		JobID:           jobID,
		CorrelationID:   l.CorrelationID,
		minLevel:        l.minLevel,
		output:          l.output,
	}
}

// Info logs at info severity.
func (l *Logger) Info(msg string, fields ...LogField) {
	l.emit("info", msg, fields)
}

// Warn logs at warn severity.
func (l *Logger) Warn(msg string, fields ...LogField) {
	l.emit("warn", msg, fields)
}

// Error logs at error severity.
func (l *Logger) Error(msg string, fields ...LogField) {
	l.emit("error", msg, fields)
}

// Debug logs at debug severity.
func (l *Logger) Debug(msg string, fields ...LogField) {
	l.emit("debug", msg, fields)
}

// emit formats and writes one structured log line as JSON.
// Each line is a complete JSON object terminated by newline.
func (l *Logger) emit(severity string, msg string, fields []LogField) {
	level, ok := severityLevel[severity]
	if !ok {
		level = severityLevel["info"]
	}
	if level < l.minLevel {
		return
	}

	entry := make(map[string]interface{}, 8+len(fields))

	entry["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	entry["severity"] = severity
	entry["message"] = msg
	entry["runner_spec"] = l.SpecName

	if l.SpecVersion > 0 {
		entry["runner_spec_version"] = l.SpecVersion
	}
	if l.RunnerMachineID > 0 {
		entry["runner_machine_id"] = l.RunnerMachineID
	}
	if l.JobID > 0 {
		entry["runner_job_id"] = l.JobID
	}
	if l.CorrelationID != "" {
		entry["correlation_id"] = l.CorrelationID
	}

	// Append caller-provided fields.
	for _, f := range fields {
		entry[f.Key] = f.Value
	}

	b, err := json.Marshal(entry)
	if err != nil {
		// Fallback to unstructured if JSON marshal fails.
		fallback := fmt.Sprintf("%s %s %s runner_spec=%s",
			time.Now().UTC().Format(time.RFC3339), severity, msg, l.SpecName)
		for _, f := range fields {
			fallback += fmt.Sprintf(" %s=%v", f.Key, f.Value)
		}
		fmt.Fprintln(l.output, fallback)
		return
	}

	fmt.Fprintln(l.output, string(b))
}
