package main

import (
	"flag"
	"fmt"
	"os"

	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
	"github.com/ghowland/opsdb/tools/runners/emergency-review-monitor"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "usage: emergency-review-monitor --dos <dos-directory>\n")
		os.Exit(2)
	}
	_ = dosPath

	config, err := runner.Init("emergency-review-monitor")
	if err != nil {
		fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
		os.Exit(1)
	}

	for runner.ShouldRun(config) {
		jobID, err := runner.StartCycle(config)
		if err != nil {
			config.Logger.Error("failed to start cycle", runner.Field("error", err.Error()))
			runner.WaitForNextCycle(config)
			continue
		}
		client := config.Client.WithCorrelation(jobID, "")

		// --- GET ---
		reviewWindowHours, _ := runner.GetSpecDataInt(config, "review_window_hours")
		if reviewWindowHours == 0 {
			reviewWindowHours = 72
		}
		escalationIntervalHours, _ := runner.GetSpecDataInt(config, "escalation_interval_hours")
		if escalationIntervalHours == 0 {
			escalationIntervalHours = 24
		}
		maxFindings, _ := runner.GetSpecDataInt(config, "max_findings_per_cycle")
		if maxFindings == 0 {
			maxFindings = 100
		}
		notifyOnOverdue, ok := runner.GetSpecDataBool(config, "notify_on_overdue")
		if !ok {
			notifyOnOverdue = true
		}

		overdue, totalChecked, err := monitor.GetPendingEmergencyReviews(client, reviewWindowHours)
		if err != nil {
			config.Logger.Error("failed to get pending reviews",
				runner.Field("error", err.Error()))
			runner.FinishCycle(config, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		config.Logger.Info("checked emergency reviews",
			runner.Field("total_checked", totalChecked),
			runner.Field("overdue_found", len(overdue)),
		)

		// --- ACT ---
		if runner.IsDryRun(config) {
			planData := make([]map[string]interface{}, 0, len(overdue))
			for _, r := range overdue {
				planData = append(planData, map[string]interface{}{
					"change_set_id":  r.ChangeSetID,
					"change_set_name": r.ChangeSetName,
					"elapsed_hours":  r.ElapsedHours,
					"has_finding":    r.ExistingFindingID != nil,
				})
			}
			runner.LogPlan(config.Logger, "overdue emergency reviews", planData)
			runner.SkipActPhase(config.Logger)
			runner.SkipSetPhase(config.Logger)
			runner.FinishCycle(config, "completed", map[string]interface{}{
				"dry_run":        true,
				"reviews_checked": totalChecked,
				"overdue_found":  len(overdue),
			})
			runner.WaitForNextCycle(config)
			continue
		}

		findingsCreated := 0
		escalationsSent := 0
		var errors []string

		for _, rev := range overdue {
			if findingsCreated >= maxFindings {
				config.Logger.Warn("max findings per cycle reached")
				runner.RecordBoundHit(config, "max_findings_per_cycle", maxFindings)
				break
			}

			if rev.ExistingFindingID == nil {
				// No existing finding — file one.
				findingID, err := monitor.FileOverdueFinding(client, &rev, jobID)
				if err != nil {
					errors = append(errors, fmt.Sprintf("finding for cs %d: %v", rev.ChangeSetID, err))
					config.Logger.Error("failed to file finding",
						runner.Field("change_set_id", rev.ChangeSetID),
						runner.Field("error", err.Error()),
					)
					continue
				}
				findingsCreated++

				config.Logger.Info("filed overdue finding",
					runner.Field("change_set_id", rev.ChangeSetID),
					runner.Field("finding_id", findingID),
					runner.Field("elapsed_hours", rev.ElapsedHours),
				)

				if notifyOnOverdue {
					err = monitor.SignalNotification(client, &rev, findingID, jobID)
					if err != nil {
						errors = append(errors, fmt.Sprintf("notification for cs %d: %v", rev.ChangeSetID, err))
					} else {
						escalationsSent++
					}
				}
			} else if monitor.ShouldReescalate(&rev, escalationIntervalHours) {
				// Existing finding but re-escalation interval has passed.
				findingID, err := monitor.FileOverdueFinding(client, &rev, jobID)
				if err != nil {
					errors = append(errors, fmt.Sprintf("re-escalation for cs %d: %v", rev.ChangeSetID, err))
					continue
				}
				findingsCreated++

				config.Logger.Info("filed re-escalation finding",
					runner.Field("change_set_id", rev.ChangeSetID),
					runner.Field("finding_id", findingID),
					runner.Field("elapsed_hours", rev.ElapsedHours),
				)

				if notifyOnOverdue {
					err = monitor.SignalNotification(client, &rev, findingID, jobID)
					if err != nil {
						errors = append(errors, fmt.Sprintf("re-escalation notification for cs %d: %v", rev.ChangeSetID, err))
					} else {
						escalationsSent++
					}
				}
			}
		}

		// --- SET ---
		status := "completed"
		if len(errors) > 0 {
			status = "completed_with_errors"
		}

		runner.FinishCycle(config, status, map[string]interface{}{
			"reviews_checked":  totalChecked,
			"overdue_found":    len(overdue),
			"findings_created": findingsCreated,
			"escalations_sent": escalationsSent,
			"errors":           errors,
		})

		runner.WaitForNextCycle(config)
	}

	config.Logger.Info("emergency review monitor shutting down")
	os.Exit(0)
}
