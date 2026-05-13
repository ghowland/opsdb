# Writing a Runner

How to write a new OpsDB runner.

## The pattern

Every runner follows the same three-phase shape:

1. **Get** — read from OpsDB. No side effects.
2. **Act** — do work in the world through shared libraries. Side effects happen here.
3. **Set** — write results back to OpsDB. Every write through the API.

The runner library (`opsdb-runner-lib`) handles lifecycle, API client, logging,
retry, and dry-run support. Your runner code is 200-500 lines of domain-specific
logic. The library does the heavy lifting.

## Skeleton

```go
package main

import (
    "os"
    runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

func main() {
    config, err := runner.Init("my-runner-name")
    if err != nil {
        os.Exit(1)
    }

    for runner.ShouldRun(config) {
        jobID, _ := runner.StartCycle(config)
        client := config.Client.WithCorrelation(jobID, "")

        // --- GET ---
        // Read what you need from OpsDB.
        // No side effects in this phase.
        data, err := client.Search("service", filters, nil, 100, "")

        // --- ACT ---
        if runner.IsDryRun(config) {
            runner.LogPlan(config.Logger, "what I would do", data)
        } else {
            // Do work through shared libraries.
            // Each action bounded per runner_data_json.
        }

        // --- SET ---
        if !runner.IsDryRun(config) {
            // Write results via API.
            client.WriteObservation(&runner.WriteObservationParams{...})
        }

        runner.FinishCycle(config, "completed", summary)
        runner.WaitForNextCycle(config)
    }
}
```

## Disciplines

Three disciplines are non-negotiable:

### Idempotent

Running the same cycle twice with the same inputs produces the same end state.
Use version stamps, uniqueness keys, and upsert semantics. The library's
`WithIdempotencyKey` helps for API calls.

### Level-triggered

React to current state, not events. Read desired state, read observed state,
compute the diff, act on the diff. If you miss a cycle, the next cycle sees
the same state difference and acts on it. Reactors (edge-triggered) must be
paired with a reconciler backstop.

### Bounded

Every resource consumption has an explicit limit. Set these in `runner_data_json`:
- `max_cycle_duration_seconds` — hard time bound per cycle
- `batch_size` — max items to process per cycle
- `max_retry_count` — max retries per action

When a bound is hit, call `runner.RecordBoundHit()` and stop cleanly.
The bound hit is recorded in the `runner_job` row so you can query
"which runners are hitting bounds?"

## Registering the runner

Before deploying, create a `runner_spec` row (via change set or seed):

```yaml
entity_type: runner_spec
records:
  - name: my-runner-name
    description: "What this runner does"
    runner_spec_type: reconcile  # or puller, verifier, drift_detect, etc.
    is_active: true
    runner_data_json:
      cycle_interval_seconds: 60
      batch_size: 100
      max_cycle_duration_seconds: 120
      dry_run: false
      # domain-specific config here
```

Create a service account, declare runner capabilities and target scope,
and declare report keys for any observation writes.

## Anti-patterns to avoid

| Anti-pattern | What's wrong | Instead |
|-------------|-------------|---------|
| Orchestrating other runners | Violates "no runner directs another" | Coordinate through shared data in OpsDB |
| State outside OpsDB | Local files OpsDB doesn't reflect | All persistent state in OpsDB rows |
| Reimplementing shared libraries | Own retry/K8s client/logging | Use the library suite |
| In-memory state across cycles | Accumulates and drifts | Fresh read from OpsDB each cycle |
| Skipping audit trail | Bypasses API to avoid audit | Every write through the API |
| Acting on stale cache | Reads observation without checking `_observed_time` | Check freshness before acting |
