# OpsDB Shared Library Suite

## What This Document Covers

The shared library suite is how runners interact with the world and with OpsDB without reimplementing common patterns. Every runner uses the API client library. Every runner uses the observation libraries. Runners that touch Kubernetes use the Kubernetes library. Runners that touch cloud providers use the cloud library. The libraries enforce policy, handle retry logic, propagate correlation IDs, and validate inputs — so that runners don't have to.

This document specifies what the library suite is, what each library does, how the libraries enforce two-sided policy, and what the suite deliberately refuses to become.

---

## Design Principles

The library suite exists so that runners stay small. A runner built against the suite is roughly 200 lines of runner-specific logic — 150 lines of decisions and transformations, 50 lines of library glue. The same runner without the suite would be roughly 1500 lines, because it would reimplement HTTP client handling, authentication, retry with backoff, structured logging, correlation ID propagation, schema validation, and world-side API access.

The suite is one suite. Within an organization there is one library suite. Multiple language implementations of the same contracts are valid — a Go implementation and a Python implementation of the same API client contract can coexist. Parallel suites in the same language are forbidden. A team wanting "their own version" of a library is solving a real problem, but the solution is absorbing their need into the standard library, not fragmenting.

Every library is a contract specification, not just an implementation. The contract defines operations (what can be called), inputs (typed parameters with bounds), outputs (typed return values with structured metadata), guarantees (idempotency, ordering, freshness, bounded execution time), and failure modes (auth failures, network errors, timeouts, bound exceeded). An implementation satisfies the contract by passing the contract's test suite. Multiple implementations of the same contract can coexist — different languages, different transports — as long as they pass the tests.

The boundary between library and runner is determined by a mechanical test: would two runners reimplement this? If yes, it belongs in the library. If no, it stays in the runner. Authentication mechanisms, world-side substrate access, resilience patterns, observation, notification, templating, and git operations all pass this test — every runner that touches Kubernetes would reimplement Kubernetes API access. Decision logic, target selection, threshold computation, and domain-specific interpretation fail this test — they are specific to one runner's purpose.

---

## Library Families

The suite is organized into seven families, each with a distinct role.

### API Access (opsdb.api)

The only path runners use to read from or write to OpsDB. Mandatory for every runner. No runner accesses the database directly, makes raw HTTP calls to the API, or uses an alternative client. This library is the enforcement of the one-way-to-do-each-thing principle applied to OpsDB access.

### World-Side Substrate (opsdb.world.*)

One library per external substrate — Kubernetes, cloud providers, hosts (SSH), container registries, secret backends, identity providers, monitoring systems. Each substrate gets its own library because substrates evolve at different cadences. A Kubernetes API change doesn't affect the cloud library. An AWS SDK update doesn't affect the SSH library. Isolation contains change.

### Coordination and Resilience (opsdb.coordination.*)

Retry with backoff and jitter, circuit breakers, hedging, bulkheads, failover routing. These patterns are non-trivial and have subtle correctness requirements. Reviewed once, tested once, used by every runner that makes outbound calls. Each pattern composes with the world-side libraries — retry wraps Kubernetes calls, circuit breakers wrap cloud calls, bulkheads isolate failure domains across targets.

### Observation (opsdb.observation.*)

Logging, metrics, and tracing. Mandatory for every runner. Consistent observation format is the precondition for operational visibility across the runner population. Different log formats break aggregation. Different metric naming breaks dashboards. Different trace context propagation breaks correlation. The observation libraries enforce uniformity.

### Notification (opsdb.notification)

Channel-agnostic notification operations — send email, post to chat, page on-call, create ticket. The library reads channel configuration from OpsDB (authority rows typed as chat_platform, ticketing_system, paging_provider) and dispatches through the configured backend. A runner says "page the database SRE on-call about this" and the library resolves the on-call assignment, finds the contact information, and dispatches through the configured paging provider.

### Templating and Rendering (opsdb.templating.*)

Deliberately simple. Variable substitution, file inclusion, and bounded iteration. No expressions, no conditionals over expressions, no function calls, no embedded code. If a template needs logic, the logic belongs in the upstream runner that produces concrete values as configuration variables. The template substitutes those values.

This restriction is deliberate and permanent. Template languages with logic accumulate complexity over time. Authors find it easier to add a conditional to a template than to write runner logic that produces a concrete value. Over months and years, templates become opaque programs embedded in supposed data. The restriction prevents this.

### Git Operations (opsdb.git)

Clone, commit, push, tag, create pull request. Used by GitOps integration runners and schema evolution tooling. Authentication through the secret backend (SSH keys, deploy tokens, OAuth tokens). The library provides a structured commit message helper that encodes change set IDs and references — consistent across all runners so that git history can be pivoted back to OpsDB's trail.

---

## The Libraries

### opsdb.api — API Client

**Mandatory.** Every runner uses this library. It is the only permitted path from a runner to OpsDB.

The client wraps all sixteen API operations: get_entity, get_entity_history, get_entity_at_time, search, get_dependencies, resolve_authority_pointer, change_set_view, write_observation, submit_change_set, approve_change_set, reject_change_set, cancel_change_set, emergency_apply, apply_change_set_field_change, mark_change_set_applied, and watch.

Beyond wrapping HTTP calls, the client provides several critical features.

**Authentication and credential management.** The client acquires credentials from the secret backend at startup, refreshes them before expiration, and includes them in every request. The runner never handles raw credentials.

**Correlation ID propagation.** Every request carries the runner_job_id as a correlation header. The API records this in the audit_log_entry. Any log line from the runner, any API audit entry, any downstream trace span can be joined through the correlation ID. "Show me everything that happened during this runner job" is a single query across logs, audit entries, and traces.

**Optimistic concurrency handling.** When submitting change sets, the client stamps each field change with the version of the entity it was drafted against. If the API returns a stale_version error, the client can optionally auto-refetch, re-reconcile, and retry (up to a configurable limit, default 3). This auto-retry only applies when the merge is straightforward — complex bundles with interdependent field changes are surfaced to the runner for manual reconciliation.

**Report key fail-fast.** Before sending a write_observation request, the client checks the submitted key against the runner's cached report key declarations. If the key isn't declared, the client rejects locally without making the round-trip. This catches misconfiguration immediately rather than after a network call.

**Structured failure surfacing.** Every API error is returned as a typed failure: validation_failed, authorization_denied, stale_version, not_found, bound_exceeded, network_error, internal_error. The runner can match on the failure type and respond appropriately — retry on network errors, record and skip on authorization denied, refetch and reconcile on stale version.

**Dry-run support.** The submit_change_set operation supports a dry_run flag. The full validation pipeline runs but no rows are written. The response shows what approval requirements would be computed. Runners use this during their dry-run mode to show what they would propose without proposing it.

**Watch with level-triggered backstop.** The watch operation provides a streaming subscription to entity changes. On reconnect (after network partition, API restart, or client restart), the client fetches current state first, then resumes streaming from the resume token. This ensures no state changes are missed — even if the stream was interrupted, the full current state is loaded before incremental updates resume.

Why is this library mandatory? Because alternate paths would make the API gate's disciplines advisory. If a runner could bypass the client and make raw HTTP calls, it could skip correlation IDs, skip report key validation, skip concurrency handling. Not building alternative paths is the discipline.

### opsdb.world.kubernetes — Kubernetes Operations

**Conditional.** Used by runners that interact with Kubernetes clusters.

Operations: apply_manifest, query_resource, query_resources, watch_resources, helm_render, helm_install, exec_in_pod, get_pod_logs.

Before executing any operation, the library extracts the target cluster and namespace from the operation parameters and validates them against the runner's declared `runner_k8s_namespace_target` rows. If the runner hasn't declared authority over the target cluster and namespace, the library rejects the call with a structured error before it reaches the Kubernetes API. Fail closed.

The library wraps the standard Kubernetes client with standardized retry logic, error classification (transient vs permanent, retryable vs not), and structured error mapping. A Kubernetes API error is translated into the same failure taxonomy the API client uses, so runners handle failures uniformly regardless of which substrate produced them.

### opsdb.world.cloud — Cloud Operations

**Conditional.** Used by runners that interact with cloud providers.

Operations: provision_resource, query_resource, query_resources, modify_resource, decommission_resource, watch_resource_events.

Provider-agnostic interface with per-provider backends. The runner calls `cloud.query_resource(account_id, resource_type, resource_id)` and the library routes to the appropriate AWS, GCP, or Azure backend based on the cloud account's provider. The response is normalized to a common structure with provider-specific details in a typed payload.

Policy validation checks the target cloud_account_id against the runner's declared `runner_cloud_account_target` rows. Undeclared accounts are rejected before the cloud API call is made.

### opsdb.world.host — Host Operations

**Conditional.** Used by runners managing hosts through SSH.

Operations: exec_command, copy_file, apply_config.

Policy validation checks the target host against the runner's declared machine target or host_group target. The library handles SSH connection management, key-based authentication through the secret backend, command timeout enforcement, and structured output capture.

This library exists for legacy substrates without richer APIs. It is used sparingly — Kubernetes and cloud API access are preferred when available.

### opsdb.world.registry — Registry Operations

**Conditional.** Used by runners that interact with container or artifact registries.

Operations: pull_image, query_image_metadata, list_tags, verify_signature.

Supports ECR, GCR, Docker Hub, Harbor, and Artifactory. Policy validation checks the target registry and repository against the runner's declared registry access scope.

### opsdb.world.secret — Secret Access

**Conditional.** Used by runners that need secret values at runtime.

Operations: fetch_secret, store_secret, rotate_credential, list_secrets.

Supports Vault, AWS Secrets Manager, GCP Secret Manager, and Azure Key Vault.

This library enforces the strictest data handling discipline in the suite. Secret values exist in memory only during the call. They are never logged — the logging library records the secret path, the caller, the timestamp, and the result (success/failure), but never the value. They are never written to OpsDB — OpsDB holds pointers to secrets, not secrets themselves. They are never persisted to disk by the library. When the call completes, the value is available to the runner's in-memory logic and nowhere else.

Policy validation checks the target secret path against the runner's declared secret access target.

### opsdb.world.identity — Identity Provider Operations

**Conditional.** Used by runners that query identity providers.

Operations: get_user, get_group_members, check_membership, watch_identity_changes.

Read-only — this library does not mutate identity provider state. It queries Okta, Azure AD, or LDAP for operational purposes: syncing group memberships, checking whether a user is still active, watching for identity changes that affect access control.

### opsdb.world.monitoring — Monitoring Operations

**Conditional.** Used by runners that query monitoring systems.

Operations: prometheus_query, prometheus_query_range, datadog_query, log_query, fetch_recent_alerts.

Used by puller runners to read metric metadata and alert state from monitoring authorities. The library handles query construction, pagination, rate limiting, and response normalization.

### opsdb.world.pointer — Authority Pointer Resolution

**Convenience library.** Composes the API client and the appropriate world-side library to resolve and fetch from an authority pointer in one call.

The API client's resolve_authority_pointer operation returns the authority coordinates (base URL, pointer type, locator). This library takes the additional step of using the appropriate world-side library to fetch the actual data from the authority. A runner calls `pointer.resolve_and_fetch(authority_pointer_id)` and gets back the resolved data without knowing which authority type or world-side library is involved.

### opsdb.coordination.retry — Retry with Backoff

**Optional but used by nearly every runner through composition.**

Operations: with_retry, with_idempotency_key.

Provides exponential backoff with jitter, bounded by a retry budget (maximum attempts, maximum total duration). Operations can be tagged with idempotency keys so that retried operations are recognized by the target system as repeats rather than new requests.

This library composes inside every outbound library — the Kubernetes library uses it internally for retryable API calls, the cloud library uses it for transient cloud API failures, the API client uses it for network errors. Runners can also wrap their own logic in retry when appropriate.

### opsdb.coordination.circuit_breaker — Circuit Breaker

**Optional.**

Operations: call_with_breaker.

Prevents cascading failure by tracking per-target error rates and opening the circuit (refusing calls) when the error rate exceeds a threshold. Half-open probes test recovery before fully closing. State is per-runner-instance by default, or synchronized across instances via `observation_cache_state` for targets where coordinated circuit breaking matters.

The circuit breaker can pre-trip based on cached health observations — if the observation cache shows a target is unhealthy, the breaker opens before the runner even attempts the call.

### opsdb.coordination.hedger — Hedging

**Optional.**

Operations: hedge_call.

Reduces tail latency by issuing redundant requests to multiple targets. The library validates that the operation is marked as idempotent before allowing hedging — non-idempotent operations cannot be hedged because the second request would produce duplicate side effects.

### opsdb.coordination.bulkhead — Bulkhead Isolation

**Optional.**

Operations: with_bulkhead.

Isolates failure domains by maintaining bounded resource pools keyed by a domain identifier. Each call specifies its domain key, and the bulkhead ensures that one failing domain's calls don't consume resources needed by healthy domains. Pool sizing, queueing depth, and timeout are configured per policy.

### opsdb.coordination.failover — Failover Routing

**Optional.**

Operations: call_with_failover.

Routes calls through a primary target, falling back to replicas in order on failure. Hides the topology from the runner — the runner calls the logical service and the library handles primary detection, failure detection, and failover sequencing.

### opsdb.observation.logging — Structured Logging

**Mandatory.** Every runner uses this library.

Emits structured log lines (JSON or logfmt) with mandatory fields: timestamp, severity, runner_job_id, correlation_id, runner spec name, runner spec version, runner_machine_id, and source location. The destination is configured through the runtime environment — stdout for containerized runners, syslog for host-based runners, direct-to-aggregator for high-volume runners.

Runners that bypass this library and emit their own log format produce lines that don't correlate with anything — they can't be joined to runner jobs, audit entries, or traces. The logging library exists to prevent this.

### opsdb.observation.metrics — Metrics Emission

**Mandatory.** Every runner uses this library.

Operations: counter_increment, gauge_set, histogram_observe, timer.

Emits metrics in standard format (Prometheus, statsd, or Datadog depending on configuration). Labels on metrics are validated against the runner's `runner_capability` declarations. Metrics the runner hasn't declared are rejected — this is the outbound analog of report keys for inbound observations. A runner can't emit arbitrary metric names and pollute the metric namespace.

### opsdb.observation.tracing — Distributed Tracing

**Mandatory.** Every runner uses this library.

Operations: start_span, with_span, inject_trace_context, extract_trace_context.

Propagates trace context (OpenTelemetry or vendor-native) across runner operations. Trace IDs are correlated with audit_log_entry rows and runner_job rows, enabling pivots from a trace span to the OpsDB structural data — "this span corresponds to this runner job which produced this change set which was approved by this person."

---

## Two-Sided Policy Enforcement

The library suite and the API gate form two enforcement surfaces that together prevent any runner from acting outside its declared scope.

**The API gate** enforces policy on OpsDB writes. When a runner writes an observation, submits a change set, or applies a field change, the API's ten-step pipeline validates the write against the runner's declarations (report keys, target scope, capabilities). Unauthorized writes are rejected before they reach the database.

**The library suite** enforces policy on world-side actions. When a runner attempts a Kubernetes operation, a cloud API call, an SSH command, a secret fetch, or a notification dispatch, the library validates the target against the runner's declarations (namespace targets, cloud account targets, machine targets, secret access targets, notification scope). Unauthorized actions are rejected before they reach the external substrate.

Both surfaces use the same input — runner declaration rows stored in OpsDB. Both produce the same outcome — fail-closed rejection with structured error and audit logging. Together they create comprehensive coverage: a runner cannot write unauthorized data to OpsDB (caught by the gate) and cannot perform unauthorized actions in the world (caught by the library).

This is what "runner authority is data" means in practice. Every action a runner can take — reading, writing, acting — is governed by data rows in OpsDB. Those data rows are themselves change-managed through the standard approval pipeline. Modifying a runner's scope requires a change set approved by the appropriate authority. The runner's code doesn't determine what it's allowed to do. The data does.

### How Library Policy Validation Works

Every world-side library follows the same pattern when validating policy.

First, extract the target from the operation parameters. For a Kubernetes call, the target is the cluster and namespace. For a cloud call, the target is the cloud account ID. For an SSH call, the target is the machine or host group. For a secret fetch, the target is the secret path. For a notification, the target is the channel and, for pages, the paging authority.

Second, look up the runner's declarations from the cached OpsDB data. The library caches the runner's target scope declarations at startup and refreshes them periodically. For Kubernetes, this means `runner_k8s_namespace_target` rows. For cloud, `runner_cloud_account_target` rows. For notifications, the runner's declared notification scope and paging authority.

Third, check coverage. Does the runner have a declaration covering this target? If yes, proceed. If no, reject with a structured error identifying the target and the missing declaration.

Fourth, log the check. Every policy validation — pass or fail — is logged through the observation library and emitted as a metric. Failed validations are surfaced alongside API gate denials in unified queries. "Show me every authorization denial for runner X in the last hour" returns both gate denials (from audit_log_entry) and library denials (from observation logs and metrics).

### Fail Closed Under Partition

If the library cannot reach OpsDB to refresh its declaration cache, it uses the last known good cache. This provides partition tolerance — the runner can continue operating with its previously cached declarations.

However, the cache has bounded staleness. After a threshold (configurable per library, shorter for security-sensitive libraries like secrets and paging, longer for less sensitive operations), the library refuses calls entirely because the declarations can no longer be trusted. A puller writing metric observations might tolerate a stale cache for an hour. A runner accessing secrets might tolerate only five minutes.

If the library cannot determine authorization — because the cache is empty, corrupted, or past its staleness threshold — it refuses rather than allows. This is the fail-closed principle applied consistently across every library.

---

## Library Versioning

Libraries follow semantic versioning: MAJOR.MINOR.PATCH.

**PATCH** versions are bug fixes with no contract change. Runners upgrade freely.

**MINOR** versions add new operations, new optional parameters, or new return fields. Backward compatible. Runners upgrade to use new features at their own pace.

**MAJOR** versions break the contract. These are rare and require deprecation cycles. The old version is maintained alongside the new one for a configurable number of release cycles (typically 3-5). Deprecation warnings are emitted when runners use the old version. Runners migrate at their own pace. Removal of the old version happens only after the library steward confirms no consumer depends on it. Most deprecated versions remain available indefinitely as legacy paths — the cost of supporting them is less than the cost of breaking consumers.

Every contract has a test suite. A new implementation (new language, new transport, new backend) is accepted only if it passes the contract's test suite. "Implements the contract" is operationally defined as "passes the suite."

Library changes that touch the schema coordinate with schema evolution. The schema change set lands first. The library version using the new fields is released after. Runners migrate to the new library version after that. The order matters — schema first, then library, then runners.

---

## The Library Steward

The library steward is a role parallel to the schema steward. A senior engineer or architect who reviews new library proposals (should this be a library or runner logic?), contract additions (right shape? composable?), contract removals (all consumers migrated?), cross-library coherence (do the libraries compose well?), and implementation quality (do all implementations pass the tests?).

The steward resists fragmentation. Every team wanting their own version of a library is solving a real problem — maybe the standard library doesn't handle their edge case, maybe the API is awkward for their use pattern. The steward's job is to absorb the real need into the standard library rather than allowing a fork. This is the same discipline the schema steward applies to schema fragmentation, applied at the library layer.

The investment in the steward role is proportional to suite maturity. A small suite (two mandatory libraries, a few world-side libraries) requires 10-25% of a senior engineer's time. A mature suite (twenty libraries, multiple language implementations, hundreds of consuming runners) may require a full-time steward with a small supporting team.

The investment compounds. A well-stewarded suite makes the next runner cheap. The team building the runner finds that the library calls already exist, are documented, tested, and integrated. Their runner is small because the suite is good. The steward's work is paid back by every runner that doesn't have to reimplement what the suite already provides.

---

## What the Library Suite Is Not

Each of these refusals preserves a boundary that keeps the suite focused and the architecture clean.

**Not a runner framework.** The library is callable, not controlling. A framework that owns the runner's main loop, event dispatch, and lifecycle would couple every runner's evolution to the framework's evolution. The runner owns its main loop. The runner calls library functions. The library does not call the runner. The `opsdb-runner-lib` in the FOSS project provides lifecycle helpers as composable functions, not as a controlling framework — the runner's main function calls the helpers, not the other way around.

**Not a workflow engine.** The library does not mediate runner-to-runner messaging. Runners coordinate through OpsDB rows. A library that routed messages between runners would be an orchestrator by another name, violating the passive substrate commitment.

**Not a code distribution system.** Library implementations are distributed through standard package mechanisms — Go modules, PyPI packages, container images, internal artifact stores. OpsDB holds operational data, not code.

**Not a secrets store.** The secret library accesses secret backends. It never persists values. This is a recursive boundary — just as OpsDB holds pointers to secrets and not secrets themselves, the library fetches secrets for in-memory use and never stores them.

**Not a service mesh.** The library makes outbound calls from runners. It does not intercept traffic between other components. Service mesh functionality (mutual TLS, traffic routing, observability injection at the network layer) belongs in service mesh products.

**Not a UI.** The suite has no user interface. Runners are observed through the observation libraries (logs, metrics, traces) and through OpsDB itself (runner_job rows, evidence records, audit entries). Dashboards and UIs are downstream consumers of this data, not part of the library suite.

**Not a database.** OpsDB is the database. Library state is either ephemeral (in-memory for the duration of a call or cycle) or written to OpsDB through the API client. The library does not maintain its own persistent storage.

---

## Adoption Path

The suite starts small and grows with the runner population.

**Start with two libraries.** The API client and structured logging. These are the mandatory minimum. Many simple runners — basic pullers, simple verifiers — need nothing more. A puller that reads from a Prometheus endpoint and writes to observation cache uses the API client for OpsDB access, the logging library for structured output, and standard HTTP for the Prometheus query. No other libraries needed.

**Add world-side libraries as domains arrive.** When Kubernetes coordination is added to the OpsDB, the Kubernetes library is built. When cloud governance is added, the cloud library is built. When monitoring integration is added, the monitoring library is built. Each library is built once and paid back many times by every subsequent runner that consumes it.

**Add coordination libraries as patterns emerge.** When three runners have independently implemented retry with backoff, extract the retry library. When three runners have implemented circuit breaker logic, extract the circuit breaker library. The library steward calibrates the timing — not premature (extracting before the pattern is stable) and not deferred (letting five more runners reimplement the same pattern).

**Cross-implementation portability follows a natural order.** The first language implementation is the canonical reference. Subsequent ports (Go to Python, Go to Rust) are gated by passing the contract test suite. The order of porting follows demand: API client first (every runner needs it), observation libraries next (every runner uses them), world-side libraries as needed (only runners in that language touching that substrate), coordination libraries last (patterns are language-specific in implementation but the contracts are portable).
