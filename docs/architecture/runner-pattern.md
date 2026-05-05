# OpsDB Runner Pattern

## What This Document Covers

Runners are the active layer of OpsDB. The API gate is passive — it answers queries and accepts writes but never initiates work. Runners are what make things happen. They read from OpsDB, act in the world, and write results back. Every piece of operational automation in an OpsDB-coordinated environment is a runner.

This document specifies the runner pattern, the lifecycle every runner follows, the ten kinds of runners, the three disciplines every runner must satisfy, how runners coordinate without orchestration, and how to design new runners.

---

## The Three-Phase Pattern

Every runner follows the same shape: get, act, set.

**Get:** Read from OpsDB through the API. Fetch the runner's own spec, the entities it needs to examine, the policies that govern its behavior, the observations it needs to compare against. This phase produces no side effects. Nothing in the world changes during get. The runner is gathering the information it needs to decide what to do.

**Act:** Execute planned actions in the world through shared libraries. Apply a Kubernetes manifest. Rotate a credential in Vault. SSH to a host and update a configuration file. Query a cloud API to read current state. This phase produces side effects — things change in the world. Every action is bounded by the runner's declared limits and mediated through shared libraries that provide retry, backoff, idempotency keys, and circuit breaking.

**Set:** Write results back to OpsDB through the API. Create a runner_job row recording what happened. Write per-target status rows. Write observation cache entries. Write evidence records. Propose change sets. Every write goes through the API gate. Every write produces an audit log entry. Nothing is recorded out of band.

This three-phase shape is not a suggestion. It is the structural contract every runner implements. A runner that mixes phases — reading OpsDB state during its act phase, or producing side effects during its get phase — is harder to reason about, harder to test, and harder to debug when something goes wrong.

---

## Why OpsDB Is the Runner's Only Stable Interface

Runners interact with many systems — cloud APIs, Kubernetes clusters, monitoring platforms, identity providers, secret backends, SSH endpoints. All of these are transient. Cloud APIs change versions. Kubernetes deprecates API groups. Monitoring platforms get replaced. SSH endpoints get decommissioned.

OpsDB's schema is versioned and evolves only additively — no field deletions, no renames, no type changes. A runner written against the OpsDB schema today will still work against the same schema five years from now because the fields it reads will still exist with the same names and types. The world-side interfaces it touches through shared libraries may change, but the libraries absorb that change behind stable contracts.

This means a runner's persistent inputs and outputs are exclusively OpsDB rows. The runner reads its configuration, targets, policies, and observed state from OpsDB. It writes its results, evidence, observations, and proposals back to OpsDB. The world is read from and acted upon during the act phase, but the world is not where the runner's persistent state lives.

---

## Runner Lifecycle

A runner's execution follows seven phases within each cycle.

### Invocation

An external trigger starts the runner instance. This could be a cron schedule, a systemd timer, a Kubernetes CronJob, an event watch, or the runner's own long-running loop hitting its next cycle interval. The runner does not invoke itself — something outside starts it or it wakes from sleep within a long-running process.

### Get

The runner reads its own `runner_spec_version` from OpsDB to get its current configuration. It reads whatever additional data it needs — target entities, cached observations, policies, schedules, prior evidence records. Each row carries freshness metadata (_observed_time for cached data, updated_time for configuration). The runner checks freshness before acting on cached data — hour-old observation cache treated as minute-fresh is an anti-pattern that leads to acting on stale information.

No side effects occur during get. The runner is assembling a picture of current state, not changing anything.

### Internal Computation

The runner computes its planned action set. For a reconciler, this is the diff between desired and observed state. For a verifier, this is the evaluation of a condition against the target. For a drift detector, this is the set of entities that have drifted. For a puller, this is the list of keys to fetch from the authority.

The output of this phase is a concrete, inspectable plan — a data structure describing exactly what actions the runner intends to take. No side effects. The plan could be logged, reviewed, or compared against expectations.

### Dry-Run Output

If the runner is invoked with `dry_run=true`, it renders the planned action set to its log output and exits without executing. The plan is deterministic given the same input state — running in dry-run mode against the same OpsDB state produces the same plan every time. This is essential for testing and for building confidence in a new runner before letting it act.

### Act

The runner executes its planned actions through shared libraries. Each action is bounded by the limits declared in `runner_data_json` — maximum retry count, maximum execution time per action, maximum total execution time per cycle, maximum scope (number of targets to process per cycle). If a bound is hit, the runner records which bound stopped execution in the runner_job and terminates the cycle cleanly rather than continuing unbounded.

Every action mediated through shared libraries gets retry with exponential backoff and jitter. Actions that are not naturally idempotent carry uniqueness keys so that retries don't produce duplicates. The shared libraries handle this mechanically — the runner provides the keys where needed, and the libraries handle the retry logic.

### Set

The runner writes its results back to OpsDB through the API. It creates a `runner_job` row with the job's status (succeeded, failed, timeout), timing information, and input/output summaries. It creates per-target rows (`runner_job_target_machine`, `runner_job_target_service`, etc.) with per-target status and detail. It writes observation cache entries, evidence records, runner job output variables, or change-set proposals as appropriate for what it did.

Every write goes through the API gate. Every write is authenticated with the runner's service account credentials. Every write is authorized against the runner's declared scope. Every write is validated against the schema. Every write produces an audit log entry.

### Recorded Outcome

At the end of the cycle, the runner's work is fully recorded in OpsDB. The runner_job row captures what happened at a summary level. The per-target rows capture what happened to each entity the runner touched. The evidence records, observations, and change-set proposals capture the operational outcomes. The audit log captures every individual API write with full attribution.

The runner then either exits (for scheduled runners) or sleeps until the next cycle interval (for long-running runners). A long-running runner checks its OpsDB spec at the beginning of each cycle — if the spec has been updated through a change set, the runner picks up the new configuration on the next cycle. In-memory state does not persist across cycles. Anything the runner needs to remember must be written to OpsDB.

---

## The Ten Kinds of Runners

### Puller

Pullers read from external authorities and write to OpsDB's observation cache tables. A Prometheus puller reads metric metadata and writes to `observation_cache_metric`. A Kubernetes puller reads pod state and writes to `observation_cache_state`. A cloud control plane puller reads resource state and writes to `observation_cache_state` and `observation_cache_config`. An identity provider puller reads group memberships and writes to observation cache.

Pullers are naturally idempotent — re-running writes the current value, overwriting the previous one. They use the direct write gating mode — no change set, just authenticated writes to observation tables. They are scheduled, typically on short intervals (every few minutes for fast-changing state, every hour for slow-changing metadata).

A puller never writes secret values. A Vault metadata puller writes the existence and version of secrets — paths, rotation timestamps, metadata — but never the secret content itself. Secret values stay in the secret backend. OpsDB holds pointers.

### Reconciler

Reconcilers compare desired state (configuration in OpsDB) against observed state (cached observations from pullers) and take action to close the gap. A Kubernetes manifest reconciler compares OpsDB's workload spec against the cluster's actual spec. A configuration drift corrector compares OpsDB's host configuration against the host's actual state. A certificate renewer reads approaching expiration dates and initiates renewal.

Reconcilers are level-triggered — they read current state on every cycle and act on it, rather than reacting to events. If a reconciler misses a cycle, the next cycle catches up because it compares the full current state, not incremental changes. This is the most important property of reconcilers and the primary reason the spec prefers them over reactors.

When the desired and observed state match, the reconciler takes no action. This is convergence — repeated application drives toward the desired state and stays there. A reconciler that runs against an already-converged system produces no side effects beyond recording that it ran.

Reconcilers vary in gating mode depending on what they touch and where. The same reconciler code might auto-approve drift corrections in staging but require human approval for production changes to sensitive fields. This is per-target gating driven by policy data, not hardcoded in the runner.

### Verifier

Verifiers check that scheduled work happened or that state is correct and produce evidence records. A backup verifier confirms today's backup completed and is restorable. A certificate validity scanner checks expiration dates across the fleet. A compliance scanner evaluates policies against entity configurations. A credential rotation verifier confirms credentials were rotated on schedule. A manual operation verifier confirms that tape rotations, vendor reviews, or keycard revocations happened as scheduled.

Each verification cycle produces a new evidence record — verifiers never modify prior evidence. The evidence accumulates over time, creating a continuous queryable compliance trail. An auditor can query evidence records for any control over any time period and see the verification history without anyone having assembled it manually.

Verifiers use the direct write gating mode. They record observations, not configuration changes.

### Scheduler

Schedulers enforce runner schedules on target substrates. A cron entry deployer ensures host-level cron entries match the schedules declared in OpsDB. A Kubernetes CronJob reconciler ensures cluster CronJobs match. A systemd timer manager ensures timer units match.

Schedulers reconcile desired schedule state (OpsDB runner_schedule rows) against observed schedule state (what's actually configured on the substrate). They are long-running and use auto-approved gating for routine schedule management, with approval required for high-stakes schedules.

### Reactor

Reactors respond to events in an edge-triggered fashion. A webhook receiver writes incoming events to OpsDB. A Kubernetes event watcher writes alert_fire rows. A ticketing system change handler updates entity metadata when ticket states transition.

Reactors are the exception to the level-triggered preference. Some situations genuinely require event-driven response — a webhook arrives and must be processed, an event occurs that can't be reconstructed from state comparison. The spec's mitigation is that reactors are always paired with reconciler backstops. The reactor handles the immediate event, and a reconciler on a scheduled cycle catches anything the reactor missed. Missed events don't mean missed work because the reconciler's next cycle detects the gap.

Shared libraries provide idempotency keys for reactor replays. If the same event arrives twice (which happens with webhooks), the second processing is a no-op because the idempotency key matches the first.

### Drift Detector

Drift detectors have the same shape as reconcilers — they read desired and observed state and compute diffs. The difference is that drift detectors propose rather than act. Instead of correcting drift directly, they submit change sets describing the correction needed. The change set then flows through the normal approval pipeline.

Drift detectors are appropriate for sensitive systems where automatic correction is undesirable — production databases, security configurations, compliance-scoped entities. The drift is detected within one cycle (no accumulation), but the correction is governed.

If a drift detector finds drift that it has already proposed a change set for (and the change set is still pending), it does not re-propose. It waits for the existing proposal to be resolved before creating a new one.

### Change-Set Executor

The change-set executor is the runner that closes the change management loop. It reads change sets in approved status, reads their field changes, and applies each one through the API's `apply_change_set_field_change` operation. For each field change, it updates the target entity row and writes the version sibling row. When all field changes are applied, it marks the change set as applied.

There is typically one change-set executor per OpsDB substrate. It polls for approved change sets on a short interval. If a field change fails to apply (constraint violation, concurrent modification), the executor records the failure on the field change row and marks the change set as failed. Failed change sets produce compliance findings.

The executor is the only runner that modifies change-managed entity rows (other than through the submission of new change sets). Its authority comes from the change-set approval — it doesn't have blanket write access, it has the authority to apply changes that have been through the approval pipeline.

### Reaper

Reapers enforce retention policies. They read `retention_policy` rows, query target tables for rows older than the retention horizon, and remove them. For observation cache tables, reapers delete directly (cached observations are not change-managed). For change-managed entities, reapers soft-delete by setting `is_active=false`, following the policy's governance requirements.

Reapers are naturally idempotent — if nothing exists past the retention horizon, there's nothing to reap. They run on a scheduled basis, typically daily.

### Bootstrapper

Bootstrappers set up new machines from minimal state. A cloud-init script pulls templated configuration, applies it, and registers the new VM in OpsDB. A Kubernetes node bootstrap joins a cluster and registers in OpsDB. A PXE boot finalizer completes bare-metal host setup.

Bootstrappers are unique in that they may need to operate with minimal or cached OpsDB configuration — the machine being bootstrapped may not have full network connectivity to the OpsDB API yet. The runner framework supports templated configuration (baked into the deployment artifact) for this case, with runtime polling for updates once connectivity is established.

### Failover Handler

Failover handlers detect primary failures, perform failover, verify the result, and update OpsDB. A database primary-replica failover handler detects primary failure through observation cache state, initiates promotion of the replica, verifies the new primary is serving, and updates OpsDB with the new topology.

Failover handlers use stack-walking queries to make safety decisions. Before failing over, the handler walks the substrate hierarchy to verify that the primary and replica are in different failure domains (different racks, different datacenters). If they share a failure domain, failover offers no resilience and the handler refuses, logging the reason and filing a finding.

Failover handlers use emergency change gating or fast-track approval, depending on the urgency. The change set records the failover as an emergency with mandatory post-hoc review.

---

## The Three Disciplines

Every runner, regardless of kind, must satisfy three disciplines. These are non-negotiable load-bearing properties of the runner population.

### Idempotency

Every runner action must be safely retryable. Running the same runner with the same inputs and starting state must produce the same end state as running it once. If a runner crashes halfway through its act phase and restarts, re-running the cycle must not create duplicates, double-apply changes, or leave the system in an inconsistent state.

Idempotency operates at two levels. At the runner level, re-running the full cycle against the same state produces no additional side effects beyond the first converging run. A reconciler that finds desired and observed state already matching takes no action. A verifier produces a new evidence record (which is correct — it's verifying again) but doesn't modify prior records.

At the action level, each individual call through shared libraries is either naturally idempotent (writing a value to a cache key overwrites the previous value) or carries a uniqueness key (creating a change set with a deduplication token prevents duplicate proposals). The shared libraries handle the mechanics — the runner provides keys where needed.

Some operations are not naturally idempotent — sending an email, processing a payment, triggering a one-shot external action. These are flagged in the runner spec's `runner_data_json` as requiring special handling. Uniqueness keys are applied where possible. Where they can't be (truly one-shot external calls), the runner documents the non-idempotent boundary and the recovery procedure.

Idempotency is enforced through schema fields like `change_set_field_change.applied_status` (already-applied field changes are skipped on re-run) and through uniqueness markers in the shared libraries.

### Level-Triggered Over Edge-Triggered

Runners should react to current state, not event streams. A reconciler reads the current desired and observed state on every cycle and acts on the difference. If an event was missed — a webhook didn't fire, a message was dropped, the runner was down during a state change — the next cycle catches it because the state comparison reveals the gap.

Edge-triggered systems lose work when events are missed. Level-triggered systems lose nothing because they compare against current reality, not against a stream of changes that may be incomplete.

Reactors (edge-triggered runners) exist because some situations require event-driven response, but they are always paired with reconciler backstops. The reactor handles the immediate event. The reconciler catches anything the reactor missed on its next cycle. The combination provides both responsiveness and reliability.

This discipline is enforced through the architectural pattern itself — reconcilers re-evaluate on every cycle. The spec doesn't provide a mechanism for "react to this event and trust that the event arrived" because the mechanism would undermine the discipline.

### Bound Everything

Every runner has explicit limits on retry count, execution time per action, total execution time per cycle, scope (number of targets per cycle), queue depth, and memory consumption. These limits are declared in `runner_data_json` and enforced by the runner framework library.

When a bound is hit, the runner records which bound stopped execution in the `runner_job` row and terminates the cycle cleanly. The next cycle picks up where this one left off (because the runner reads current state, not a continuation token from the previous cycle — level-triggered).

Unbounded runners are the source of cascading failures. An unbounded retry loop holds connections open indefinitely. An unbounded queue consumes memory until the process crashes. An unbounded scope means the runner tries to process ten thousand targets in a cycle designed for a hundred. Bounding everything means failures are contained and predictable.

---

## Coordination Without Orchestration

No runner directs another runner. There is no orchestrator. There is no message bus between runners. There is no runner-to-runner RPC. Runners coordinate through shared data in OpsDB.

Runner A writes a row to OpsDB. Runner B reads that row on its next cycle and acts on it. The coordination is implicit through the shared substrate. The change-set executor reads approved change sets written by drift detectors. The notification runner reads state transitions written by the approval pipeline. The deploy watcher reads output variables written by the helm executor.

This pattern has several consequences. A crashed runner blocks no other runner — Runner B doesn't wait for Runner A to complete because Runner B reads data, not messages. The next instance of Runner A picks up where the crashed instance left off because it reads current state. There is no orchestration state to recover.

Runners are independently deployable, independently restartable, and independently retirable. Adding a new runner doesn't require modifying any existing runner. Removing a runner doesn't break any other runner — the data it used to write stops being updated, which downstream runners detect through freshness checks.

---

## Change Management Gating

Runners interact with the change management system in three modes.

**Direct write** is for observation-only data that never goes through change management. Pullers writing to observation cache tables, verifiers writing evidence records, reapers trimming cached data — these are authenticated and audited but not governed through change sets. The data is observational, not intentional. It records what is, not what should be.

**Auto-approved change set** is for changes that should be recorded and audited but don't need human approval. Drift corrections in non-production environments, routine credential rotations within declared schedules, reconciler corrections to fields within declared safe bounds. The change set is created, validated, and automatically approved by policy. The audit trail exists. A human can review after the fact but didn't need to approve in advance.

**Approval-required change set** is for changes that route to human approvers. Production database changes, security policy modifications, compliance scope changes, schema evolution, high-severity alert configuration. The change set is created, validated, and enters pending_approval status. The notification runner dispatches notifications. Stakeholders review and approve or reject. Only after all required approvals are the changes applied by the executor.

The same runner code can operate in all three modes for different targets. A drift correction runner might auto-approve in staging, auto-approve for low-risk production fields (timeouts, replica counts within declared bounds), require approval for production fields outside the low-risk set, and refuse to act entirely on compliance-restricted entities (filing a finding instead). The gating mode is determined by policy data evaluated per target, not hardcoded in the runner.

---

## Stack-Walking

Runners make decisions by walking OpsDB's relational structure. These walks use the `get_dependencies` API operation with recursive queries through the substrate hierarchy, service connections, and location ancestry.

**Decommission awareness.** Before acting on a service, a reconciler walks from the service through its host group, machines, and megavisor instance parent chain, checking whether any node in the chain is decommissioned or has a pending decommission change set. If the underlying infrastructure is being retired, the reconciler skips the action and logs why.

**Failure domain analysis.** Before performing a failover, the failover handler walks both the primary and replica to their location ancestry (rack, datacenter, region). If they share a rack, failover offers no resilience — the same rack failure that took down the primary will take down the replica. The handler refuses the failover and files a finding about the topology gap.

**Capacity awareness.** A Kubernetes workload scheduler walks from cluster nodes through underlying machines to hardware sets to see actual hardware capacity, not just what the cluster reports. The cluster might report available CPU, but the underlying hardware might be shared with non-Kubernetes workloads not visible to the cluster.

**Dependency-aware change validation.** Before applying a change to a service, the validator walks `service_connection` rows to find downstream dependents. If the change breaks a downstream contract (removing an interface that another service connects to), the validator rejects or warns unless the dependent is also being modified in the same change set.

**Locality-aware deployment.** A deployment runner walks from the service to its preferred locations (via `site_location` with precedence order) and then to available capacity at each location, scheduling workloads where capacity exists and locality preferences are satisfied.

---

## GitOps Integration

OpsDB integrates with GitOps workflows through a cast of specialized runners that form a complete deployment trail.

The cast has six roles. The **Helm Change-Set Executor** reads approved change sets targeting Helm release versions and configuration variables, applies the changes to OpsDB entity rows, and produces an output variable signaling the new version is ready. The **Helm Git Exporter** reads the output variable, resolves secret references through Vault at render time, commits the rendered values to the git repository with a structured message linking the change set ID, and tags the commit. **Argo CD or Flux** (external, not an OpsDB runner) reconciles the git repository to the cluster. The **Kubernetes Deploy Watcher** watches the cluster for pod transitions and records the rollout outcome — success or failure, pod count, deployed image digests, errors — as observation cache state and output variables. The **Image Digest Verifier** compares the intended digests from the change set against the actually deployed digests from the deploy watcher and writes an evidence record. If they don't match, it files a compliance finding. The **Drift Detector** continuously compares OpsDB's known Helm release version against what's observed in the cluster, auto-correcting or filing findings per policy.

The source of truth varies by domain. Intent lives in OpsDB (change sets and Helm release versions). What's checked into git to be applied lives in git (Argo CD reconciles git to cluster). Live state lives in the cluster (pulled into OpsDB cache by the deploy watcher). The trail connecting all of these lives in OpsDB — every step linked by IDs through change sets, runner jobs, output variables, observation rows, evidence records, and audit log entries.

This trail means "show me the complete history of this deployment from proposal through approval through git commit through cluster application through verification" is a set of joins across OpsDB tables, not a tour of five different tools.

The same cast handles variations without code changes. Fixed-version deployment pins exact image digests in the change set. Tag-tracking deployment resolves current tags from an artifact registry and submits change sets with the resolved digests. Promotion from staging to production reads staging's deployed digest from cache and submits a production change set with that digest. Rollback submits a change set restoring the prior Helm release version's values — the cast processes it like any other change set.

---

## Designing a New Runner

Building a new runner follows a nine-step process.

**Step 1: Identify the inputs.** What OpsDB rows does the runner read? What external authorities does it consult? This shapes the get phase. A certificate verifier reads `certificate_expiration_schedule`, target certificate entities, and prior `evidence_record` rows. It consults the certificate authority or endpoint to check actual validity.

**Step 2: Identify the outputs.** What OpsDB rows does the runner write? What side effects does it produce in the world? This shapes the set phase and determines the gating mode. A certificate verifier writes `evidence_record` rows and `compliance_finding` rows when certificates are invalid. It produces no world-side effects — it only observes.

**Step 3: Choose the gating.** Direct write for observation-only output. Auto-approved change set for routine corrections. Approval-required change set for sensitive changes. A certificate verifier uses direct write — it records evidence, not configuration changes.

**Step 4: Choose the trigger.** Scheduled for periodic work. Event-triggered for reactive work. Long-running for continuous reconciliation. Invoked by other runner data for downstream processing. A certificate verifier is scheduled — it runs daily or weekly against the certificate inventory.

**Step 5: Specify the bounds.** Maximum retry count, maximum execution time, maximum scope per cycle, maximum queue depth, maximum memory. These go into `runner_data_json`. A certificate verifier might bound at 1000 certificates per cycle, 30 minutes total execution, and 3 retries per certificate check.

**Step 6: Define idempotency.** What does "same end state" mean for this runner? What uniqueness keys does it use? A certificate verifier is naturally idempotent — each cycle produces a new evidence record. Running it twice against the same certificate state produces two evidence records, both correct.

**Step 7: Write the runner spec.** Create a `runner_spec` row with the appropriate `runner_spec_type` and `runner_data_json` schema. Register the JSON schema with the API for validation. Declare `runner_report_key` rows for every observation key the runner writes.

**Step 8: Build the runner.** Small, single-purpose, using shared libraries. The runner-specific logic is 200-500 lines. The get phase uses the API client library. The act phase uses the appropriate world-side library (Kubernetes, cloud, SSH, secret access). The set phase uses the API client library. Test the get-act-set shape. Test idempotency by running the same inputs twice and verifying the same end state.

**Step 9: Deploy through change management.** The runner's code goes to a container registry or binary repository. The runner's spec goes to OpsDB through a change set. The deployment creating a `runner_machine` row is itself a change set. The runner is operational once the change sets are approved and applied.

---

## Anti-Patterns

These are the patterns that violate the runner design and produce operational problems.

**Orchestrating other runners.** A runner that invokes other runners directly creates an orchestrator — a single point of failure, a coupling point, and a coordination bottleneck. Runners coordinate through shared data. If Runner A needs Runner B to act, Runner A writes data that Runner B reads on its next cycle.

**State outside OpsDB.** A runner that persists state in local files, in-memory caches that survive across cycles, or external databases that OpsDB doesn't know about creates invisible state. Other runners can't see it. Queries can't find it. "What did this runner do last time?" is unanswerable. Persistent state lives in OpsDB.

**Reinventing shared libraries.** A runner that implements its own retry logic, its own Kubernetes client wrapper, its own logging format creates divergence. Two runners with different retry strategies behave differently under the same failure conditions. The shared library suite exists to prevent this. Use it.

**Acting on stale cache without freshness check.** Reading `observation_cache_state` without checking `_observed_time` means acting on data that might be hours old as if it were current. The get phase must check freshness and either skip stale data or refresh it.

**Logic in template variables.** Templates are deliberately simple — substitution and inclusion only. If a template needs conditional logic, the logic belongs in the upstream runner that produces concrete values as configuration variables. The template substitutes the values. Complex templates are opaque code embedded in supposed data.

**Skipping the audit trail.** Writing directly to the database, bypassing the API, to avoid audit logging. There is no legitimate reason to do this. The emergency path exists for genuine emergencies. Skipping audit is not faster — it is unaccountable.

**In-memory state across cycles.** A long-running runner that accumulates state in memory between cycles loses that state on crash. The next instance starts fresh with no knowledge of what the previous instance was tracking. Persistent state goes to OpsDB. In-memory state is for one cycle only.

**Multi-domain runners.** A runner that handles deployment and monitoring and alerting and capacity is too big to understand. Split it into single-purpose runners. Each one is 200-500 lines. Each one does one thing. The composition happens through shared data in OpsDB, not through a monolithic runner that does everything.

**Privileged authority not expressed as policy.** A runner with admin rights hardcoded into its runtime configuration or environment variables bypasses the authorization model. Runner authority is data — `runner_capability` rows and `runner_*_target` bridge rows declare what the runner is allowed to do. The API enforces these declarations. Hardcoded privileges are invisible to the governance system.

**Treating OpsDB as a queue.** Polling OpsDB for new rows and treating each row as a job to process is a misuse of the pattern. OpsDB is a database, not a message queue. The runner pattern is read current state, decide what to do, act, record results. The change-set executor is the one exception — it reads approved change sets and applies them — but even it reads state (approved, not yet applied) rather than consuming a queue.

**Bypassing change management for speed.** Routing around the change-set pipeline because approval takes too long. The emergency path exists for genuine emergencies with break-glass authority and mandatory post-hoc review. "Faster" is not a reason to bypass governance. The cost of unaudited changes compounds over time.

---

## Tool Mapping

Existing operational tools map to runner-kind slots in the OpsDB architecture. The tools continue to do what they do — OpsDB adds the governance, audit, and coordination layer around them.

**Argo CD / Flux** remains the external cluster reconciler between git and the cluster. OpsDB adds the intent (change sets), the observed outcome (deploy watcher), and the trail connecting them.

**Crossplane** remains the Kubernetes-API-based cloud resource reconciler. OpsDB adds cloud resource state tracking and change-set governance.

**Pulumi / Terraform** remains the apply engine for infrastructure. OpsDB adds change-set submission for proposed changes and applied-state recording.

**Salt / Ansible / Puppet** remains the host reconciler. OpsDB adds host group definitions as the desired state source and applied-state recording.

**cert-manager** remains the certificate reconciler in Kubernetes. OpsDB adds certificate inventory, evidence records per renewal cycle, and verification runners.

**Prometheus** remains the metric collection authority. OpsDB adds configuration tracking (`prometheus_config` rows) and summary observation caching via puller runners.

**PagerDuty / Opsgenie** remains the page delivery authority. OpsDB adds alert tracking (`alert_fire` rows), escalation runners reading OpsDB's escalation paths, and on-call assignment tracking.

The pattern is consistent. The existing tool handles the world-side work. OpsDB handles the data, governance, and trail. Runners bridge the two.
