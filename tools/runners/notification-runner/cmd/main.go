//# tools/runners/notification-runner/cmd/main.go

go
package main

import (
	"os"
	// runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

// main is the CLI entrypoint for the notification runner.
// Reads state transitions that require stakeholder notification and
// dispatches through configured backends (email, webhook).
//
// The API does not communicate with stakeholders — that boundary is
// enforced by the spec. This runner reads what changed and decides
// who to tell and how to tell them.
func main() {
	// TODO: parse --dos flag for DOS config directory path
	// TODO: runner.Init("notification-runner") to load runner spec
	//   spec runner_data_json contains:
	//     backends: list of enabled backends [{type: "email", config: {...}}, {type: "webhook", config: {...}}]
	//     batch_size: max notifications per cycle (default 200)
	//     dedup_window_seconds: suppress duplicate notifications within window (default 300)
	//     notification_types: which state transitions to notify on:
	//       change_set_pending_approval: true
	//       change_set_approved: true
	//       change_set_rejected: true
	//       emergency_change_filed: true
	//       compliance_finding_created: true
	//       escalation_overdue: true
	// TODO: loop while runner.ShouldRun():
	//   jobID := runner.StartCycle(config)
	//
	//   GET:
	//     search for notification triggers since last cycle:
	//       change_set rows where status=pending_approval and updated_time > last_cycle_time
	//       change_set rows where status=approved and updated_time > last_cycle_time
	//       change_set rows where status=rejected and updated_time > last_cycle_time
	//       change_set rows where is_emergency=true and created_time > last_cycle_time
	//       compliance_finding rows where created_time > last_cycle_time
	//       runner_job_output_var rows where var_name=escalation_notification
	//         and runner_job.started_time > last_cycle_time
	//     for each trigger: resolve recipients via ownership/stakeholder bridges
	//     deduplicate: skip if same recipient+entity+type within dedup window
	//
	//   ACT (skip if dry run):
	//     for each notification to send:
	//       render message from trigger data + recipient context
	//       dispatch through configured backend (email, webhook)
	//       record delivery attempt
	//
	//   SET:
	//     write runner_job with cycle summary:
	//       triggers_found, notifications_sent, notifications_failed,
	//       notifications_deduped, backends_used
	//     runner.FinishCycle(config, status, summary)
	//
	//   runner.WaitForNextCycle(config)
	os.Exit(0)
}


