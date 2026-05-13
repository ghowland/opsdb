package runnerlib

import (
	"encoding/json"
	"fmt"
)

// IsDryRun checks if the runner was invoked with dry_run=true in its
// runner spec configuration. When true, the runner should compute and
// log its planned actions but skip the act and set phases.
func IsDryRun(config *RunnerConfig) bool {
	config.mu.Lock()
	defer config.mu.Unlock()
	return config.DryRun
}

// LogPlan serializes a planned action set to structured log output.
// Called during dry-run mode after the get phase computes what would happen.
// The plan is logged at Info level with structured fields so it can be
// queried from log aggregation.
func LogPlan(logger *Logger, planDescription string, plan interface{}) {
	serialized := "{}"
	if plan != nil {
		if b, err := json.Marshal(plan); err == nil {
			serialized = string(b)
		} else {
			serialized = fmt.Sprintf("(serialization failed: %v)", err)
		}
	}

	logger.Info("dry run plan",
		Field("plan_description", planDescription),
		Field("plan_data", serialized),
		Field("is_dry_run", true),
	)
}

// SkipActPhase logs that the act phase is being skipped due to dry-run mode.
// Convenience function for runners to call at the top of their act phase.
func SkipActPhase(logger *Logger) {
	logger.Info("skipping act phase: dry run mode",
		Field("is_dry_run", true),
		Field("phase", "act"),
	)
}

// SkipSetPhase logs that the set phase is being skipped due to dry-run mode.
// Convenience function for runners to call at the top of their set phase.
func SkipSetPhase(logger *Logger) {
	logger.Info("skipping set phase: dry run mode",
		Field("is_dry_run", true),
		Field("phase", "set"),
	)
}
