//# tools/opsdb-runner-lib/dryrun.go

go
package runnerlib

// IsDryRun checks if the runner was invoked with dry_run=true in its
// runner spec configuration. When true, the runner should compute and
// log its planned actions but skip the act and set phases.
func IsDryRun(config *RunnerConfig) bool {
	// TODO: return config.DryRun
	return false
}

// LogPlan serializes a planned action set to structured log output.
// Called during dry-run mode after the get phase computes what would happen.
// The plan is logged at Info level with structured fields so it can be
// queried from log aggregation.
func LogPlan(logger *Logger, planDescription string, plan interface{}) {
	// TODO: serialize plan to JSON
	// TODO: logger.Info("dry run plan",
	//   Field("plan_description", planDescription),
	//   Field("plan_data", serialized plan),
	//   Field("dry_run", true),
	// )
}

// SkipActPhase logs that the act phase is being skipped due to dry-run mode.
// Convenience function for runners to call at the top of their act phase.
func SkipActPhase(logger *Logger) {
	// TODO: logger.Info("skipping act phase: dry run mode")
}

// SkipSetPhase logs that the set phase is being skipped due to dry-run mode.
// Convenience function for runners to call at the top of their set phase.
func SkipSetPhase(logger *Logger) {
	// TODO: logger.Info("skipping set phase: dry run mode")
}


