# OpsDB Architecture Overview

## What OpsDB Is

OpsDB is a centralized data substrate that holds the full operational reality of an organization. It is not a wiki, not a monitoring system, not a code repository, not an orchestrator, not a ticketing system, not a secrets manager. It is a passive, governed, auditable data store with a single API gate through which every interaction passes.

OpsDB holds two kinds of data: configuration it is authoritative for (service definitions, deployment specs, on-call schedules, escalation paths, security policies, approval rules, retention policies, compliance scopes, runner specifications) and cached observations from external authorities (cloud resource state, Kubernetes pod status, metric summaries, identity provider group memberships, alert states). For the data it holds authoritatively, changes flow through a governed change management pipeline with validation, approval routing, and full audit trails. For cached observations, pullers continuously read from external authorities and write through the API with report key enforcement and audit logging.

The schema covers 138 entity types spanning seventeen domains: sites and locations, identity (users, groups, roles), substrate (hardware through cloud through containers), services (packages, interfaces, connections, host groups), Kubernetes (clusters, nodes, namespaces, workloads, pods, Helm releases, ConfigMaps, secret references), cloud resources (generic resource modeling with provider-specific typed payloads), authority directory (typed pointers to monitoring, logs, secrets, docs, identity, code repos), schedules, policy (security zones, classifications, retention, approval rules, escalation, compliance regimes), documentation metadata (ownership, runbooks, dashboards), runners (specs, capabilities, jobs, output, targets), monitoring and alerting (monitors, alerts, on-call, suppression), cached observations, configuration variables, change management (change sets, approvals, approval rules), audit and evidence (audit log, evidence records, compliance findings), and schema metadata (OpsDB's record of its own schema).

---

## Core Architectural Commitments

### Passive Substrate

OpsDB answers queries and accepts writes. It never invokes work. No internal scheduler fires when a change set is approved. No trigger sends a notification when an alert fires. No background process applies approved changes. The API records state transitions, and runners observe those transitions and act.

This commitment means OpsDB cannot become an orchestrator. Orchestrators accumulate state, couple to timing, and become single points of failure. OpsDB is a database with governance. Runners are independent processes that read from it, act in the world, and write back. A crashed runner blocks no other runner. A slow runner delays no other runner.

### API as Only Path

Every read and every write passes through the API gate. No direct database access for any consumer — not humans, not runners, not auditors, not substrate operators during emergencies. The API enforces authentication, authorization, schema validation, bound validation, policy evaluation, change management routing, and audit logging uniformly on every operation.

This commitment means governance is not advisory. If every path to the data goes through the gate, every interaction is validated and audited. If an out-of-band path existed (SSH to the database, direct SQL), governance would be bypassable and the audit trail would be incomplete. The API is the only path because it must be the only path for the governance claims to hold.

### Storage Engine Independence

The schema is the design. The storage engine is an implementation choice. The nine field types, the three modifiers, the six-plus constraints — all map to standard SQL features available in every major relational database. The current implementation targets Postgres. The schema YAML files, the validation rules, the evolution constraints, and the API's data-driven validation are all engine-independent.

This commitment means OpsDB survives storage engine changes. If the organization needs to move from Postgres to MySQL, or from self-hosted to managed, or from one cloud provider's managed database to another, the schema repo and the API code transfer. The DDL generation layer is the only engine-specific component.

### Configuration as Data

Every governance decision the API enforces is evaluated against data rows in OpsDB. Authorization policies, approval rules, auto-approval conditions, emergency authority grants, validation constraints, search bounds, rate limits — all of them are rows in policy tables, themselves change-managed. Changing what the API enforces means submitting a change set that modifies policy data.

This commitment means governance changes are governed. Approval rules for modifying approval rules go through the approval pipeline. The policy that determines who can change security zones is itself a security-zone-governed policy row. No config file edits, no code changes, no redeployments to change governance behavior.

---

## The Four Components

### Schema Engine

The schema engine reads YAML files describing the operational schema, validates them against a closed vocabulary, and idempotently applies them to Postgres. It enforces the spec's evolution rules mechanically — forbidden changes (field deletion, rename, type change, range narrowing, enum value removal) are rejected before they reach the database. Allowed changes (new fields, new entities, new enum values, widened ranges, new indexes) are applied in a transaction with the schema self-description tables updated so the API discovers the new schema at runtime.

The vocabulary is closed: nine types (int, float, varchar, text, boolean, datetime, json, enum, foreign_key), three modifiers (nullable, default, unique), and six-plus constraints (numeric ranges, length bounds, enum value lists, foreign key references, precision, composite uniqueness). No regex. No embedded logic. No conditional constraints. No inheritance. No templating. Each refusal prevents complexity from entering the schema layer.

Schema evolution is additive only. Fields can be added but never removed. Types can never be changed. Ranges can be widened but never narrowed. Enum values can be added but never removed. This is the price of decade-scale stability — every consumer of the schema can trust that the fields they depend on will still exist with the same names and types indefinitely. The engine enforces this mechanically. A YAML change that violates an evolution rule produces a clear error citing the specific rule and the alternative approach (typically the duplication pattern: add new field, double-write, migrate readers, deprecate old).

Reserved fields are injected automatically based on entity-level flags. Every table gets id, created_time, and updated_time. Soft-deletable entities get is_active. Hierarchical entities get a self-referencing parent foreign key. Versioned entities get a sibling table with version serial, parent version pointer, change set reference, and active version flag. The entity YAML never declares these — the engine handles them.

The engine ships as a single Go binary with six commands: init (create a new schema repo), validate (check YAML against meta-schema without a database), plan (show what DDL would execute), apply (execute the full pipeline), diff (compare YAML against database state), and export (reverse-engineer an existing database into YAML files).

See [Schema Engine](schema-engine.md) for the full specification.

### API Gate

The API gate implements sixteen operations with a ten-step enforcement pipeline applied uniformly to every request.

The ten steps are: authentication (verify caller identity against IdP or secret backend), authorization (five-layer model — role and group, per-entity governance, per-field classification, per-runner authority, policy rules — first denial halts), schema validation (operation shape matches registered schema), bound validation (field values within declared bounds), policy evaluation (cross-field invariants, data classification, retention, separation of duty), versioning preparation (prepare version sibling row for change-managed entities), change management routing (evaluate approval rules, compute required approvers, determine auto-approve vs human approval), audit logging (append-only record with full attribution, atomic with operation outcome), execution (apply the write or record the change set), and response (return result with metadata).

The sixteen operations span five classes. Read operations: get_entity, get_entity_history, get_entity_at_time, search, get_dependencies, resolve_authority_pointer, change_set_view. Direct write operations: write_observation (for runners writing cached state and evidence), apply_change_set_field_change (for the executor applying approved changes). Change set write operations: submit_change_set, emergency_apply, bulk_submit_change_set. Change management actions: approve_change_set, reject_change_set, cancel_change_set, mark_change_set_applied.

The API is data-driven. It reads entity structures from _schema_* tables, authorization policies from policy rows, approval rules from approval_rule rows, and report key declarations from runner_report_key rows. Adding a new entity type, changing an approval rule, or modifying a runner's authorized scope means changing data — no API code change, no redeployment.

The API delegates exactly two concerns externally: human identity verification (to an IdP) and credential values (to a secret backend). Everything else is evaluated locally. The API does not invoke runners, send notifications, apply approved change sets, orchestrate workflows, provide full-text search, render dashboards, store secrets, or distribute code. Each of these refusals preserves the API's simplicity and the passive substrate commitment.

See [API Gate](api-gate.md) for the full specification.

### Runners

Runners are the active layer. Every piece of operational automation follows the same three-phase pattern: get (read from OpsDB through the API — no side effects), act (execute planned actions in the world through shared libraries — bounded, idempotent, library-mediated), set (write results back to OpsDB through the API — every write audited).

Ten kinds of runners cover the operational surface. **Pullers** read from external authorities and write to observation cache. **Reconcilers** compare desired state against observed state and act to close the gap. **Verifiers** check that scheduled work happened or state is correct and produce evidence records. **Schedulers** enforce runner schedules on target substrates. **Reactors** respond to events (edge-triggered, always paired with reconciler backstops). **Drift detectors** have the reconciler shape but propose change sets rather than acting directly. **Change-set executors** apply approved change sets. **Reapers** enforce retention policies. **Bootstrappers** set up new machines from minimal state. **Failover handlers** detect primary failures and perform failover.

Three disciplines are non-negotiable for every runner. **Idempotency**: every action safely retryable, same inputs and starting state produce the same end state as running once. **Level-triggered over edge-triggered**: react to current state, not event streams, because missed events are inevitable and missed state is not. **Bound everything**: explicit limits on retry count, execution time, scope per cycle, queue depth, and memory, declared in runner_data_json and enforced by the runner framework library.

Runners coordinate through shared data, not orchestration. No runner directs another runner. Runner B reads what Runner A wrote on B's next cycle. A crashed runner blocks no other runner. Runners are independently deployable, independently restartable, and independently retirable.

Runners are small — 200-500 lines of runner-specific logic. The shared library suite does the heavy lifting: API access, Kubernetes operations, cloud operations, SSH, secret access, retry with backoff, circuit breaking, structured logging, metrics, tracing, notification dispatch, templating, and git operations. The runner contains the decision logic. The libraries contain everything else.

See [Runner Pattern](runner-pattern.md) for the full specification.

### Shared Library Suite

The library suite is organized into seven families. **API access** (mandatory — the only path to OpsDB), **world-side substrate** (one library per external substrate — Kubernetes, cloud, host, registry, secret, identity, monitoring), **coordination and resilience** (retry, circuit breaker, hedger, bulkhead, failover), **observation** (logging, metrics, tracing — all mandatory), **notification** (channel-agnostic dispatch), **templating** (deliberately simple — substitution and inclusion only, no expressions or conditionals), and **git operations**.

The suite enforces two-sided policy. The API gate validates writes to OpsDB against the runner's declarations (report keys, target scope, capabilities). The library suite validates world-side actions against the same declarations (namespace targets, cloud account targets, machine targets, secret access targets). Both surfaces fail closed. Together they prevent any runner from acting outside its declared scope through any path.

Libraries are contracts, not just implementations. Each library defines operations, typed inputs with bounds, typed outputs with structured metadata, guarantees (idempotency, ordering, freshness, bounded execution time), and failure modes. Multiple implementations of the same contract can coexist (different languages, different transports) as long as they pass the contract's test suite.

The boundary between library and runner is determined by a mechanical test: would two runners reimplement this? If yes, it belongs in the library. If no, it stays in the runner. The library steward (a role parallel to the schema steward) maintains coherence across the suite, reviews proposals, resists fragmentation, and ensures cross-library composability.

The suite is not a runner framework (the library is callable, not controlling — the runner owns its main loop), not a workflow engine (runners coordinate through OpsDB rows, not library-mediated messaging), not a secrets store (secret values exist in memory only during calls, never persisted), and not a code distribution system (libraries are distributed through standard package mechanisms).

See [Library Contracts](library-contracts.md) for the full specification.

---

## Authority Importers

Importers bridge the world as it exists today and OpsDB as the system of record. They are runners — standard pullers following the get/act/set pattern — that read from live infrastructure and write to OpsDB.

Importers operate in two phases. In the observation phase, they write to observation cache tables, recording what exists without claiming authority over it. OpsDB watches but doesn't govern. In the promotion phase, observed data is promoted to governed entity rows through change sets. OpsDB transitions from watching to coordinating — changes to promoted entities flow through change management with validation, approval, and audit.

The FOSS project ships importers for AWS (EC2, RDS, S3, IAM, VPC, Route53), GCP (GCE, Cloud SQL, GCS, GKE, IAM), Kubernetes (clusters, nodes, namespaces, workloads, pods, Helm releases, ConfigMaps, secret references, services), identity providers (Okta, Azure AD, LDAP), monitoring systems (Prometheus, Datadog), on-call (PagerDuty, Opsgenie), and secret metadata (Vault, AWS Secrets Manager).

The quickstart experience: clone the repo, build the binaries, stand up Postgres, apply the schema, start the API, configure credentials for your authorities, run the importers. Within an hour, the full operational infrastructure is queryable in OpsDB. The team hasn't committed to governance yet — just visibility. Promotion to governed entities happens later, per domain, at whatever pace the organization chooses.

See [Authority Importers](importer-pattern.md) for the full specification.

---

## N-Substrate Architecture

Most organizations need one OpsDB. Some need more than one — when security perimeters require physical separation, when legal or regulatory requirements mandate data residency, when independently operating business units have no shared processes or personnel, or when air-gapped systems require physical isolation.

The cardinality rule is 0, 1, or N. There is no 2. If you need more than one, the architecture handles any number. Technical fragility, convenience, premature optimization, and performance are not valid reasons to split substrates — they indicate governance or scaling problems solvable within a single substrate.

The N-pipeline separates shared components from diverged components. Shared: schema repository (one repo deployed to all substrates), library suite (one set of contracts and implementations), API code (one codebase deployed N times), change management discipline (same mechanisms, parameterized by per-substrate policy data), tool binaries (built once, deployed everywhere). Diverged: data (each substrate is its own write authority), users authorized (per substrate), audit log (independent per substrate), runners deployed (per substrate with per-substrate configuration), policies and approval rules (per-substrate data).

The master repository demonstrates the N pattern from day one with two DOS configurations — production and staging. Organizations that need only one substrate ignore staging. Organizations that need three or more copy a DOS directory, edit the configuration, create a database, apply the schema, seed, and start. Zero code changes to go from N to N+1.

See [N-Substrate Architecture](n-substrate.md) for the full specification.

---

## Change Management

Changes to governed entities flow through a defined lifecycle: draft, submitted, validating, pending_approval, approved, applied. Terminal states include rejected, expired, cancelled, and failed.

At submission, the API evaluates the field changes against the schema, checks bounds, evaluates cross-field invariants from policy data, walks ownership and stakeholder bridges to find responsible parties, evaluates approval rules, and computes the required approvals. The change set either auto-approves (for low-risk changes matching auto-approval policies) or enters pending_approval status for human review.

Optimistic concurrency prevents silent overwrites. Each field change carries the version stamp of the entity it was drafted against. At submit time, the API checks each entity's current version against the drafted-against version. Stale submissions fail loudly — the submitter retrieves current values, reconciles, and resubmits. No change reaches the approval stage against stale state.

The emergency path provides break-glass authority for genuine emergencies. Emergency changes have reduced approval requirements (often single approver, sometimes self-approved), are permanently flagged as emergencies, and require post-hoc review within a configurable window (default 72 hours). A monitor runner escalates overdue reviews.

Rollback is itself a change set. There is no side-channel rollback mechanism. Restoring a prior version means submitting a change set with the prior version's field values. The rollback goes through the same validation, approval, and audit pipeline as any other change. The audit trail records the rollback with full attribution.

---

## Audit Trail

Every API operation produces an audit log entry with full attribution: who (human identity and/or runner service account), what (API endpoint, method, action type), targeting what (entity type and ID), with what data (request and response summaries), with what result (success, validation failed, authorization denied, rate limited, internal error), when (API-supplied high-precision timestamp), and from where (client IP, user agent, request ID, correlation ID).

The audit log table has no UPDATE or DELETE permissions for any database role, enforced at DDL level. This is the strictest append-only guarantee in the schema. Optional cryptographic chaining (each entry hashes its contents plus the prior entry's hash) provides tamper evidence — modifying any historical entry breaks the chain, and verification tooling detects the break.

The audit trail composes with the library suite's observation logging. API gate denials and library-layer denials (world-side policy enforcement) are both queryable. "Show me every authorization denial for runner X in the last hour" returns results from both surfaces.

---

## SOC2 and Compliance

OpsDB produces compliance evidence as a byproduct of normal operation, not as a periodic collection exercise.

Every governed change has a change set with validation, approval routing, and a recorded approval trail. Every entity has version history reconstructible to any point in time. Every runner that performs verification writes structured evidence records. Every emergency change is flagged with mandatory post-hoc review. Every API operation is audit-logged with full attribution.

When an auditor arrives, they receive read-only scoped access to the OpsDB API and query the same data the team queries. "Show me every production configuration change in the last twelve months and who approved each" is a query result, not a multi-week evidence collection project.

Compliance regimes (SOC2 Type 2, ISO 27001, PCI DSS, HIPAA, FedRAMP, GDPR, SOX ITGC) are modeled as entity rows with scoping, control mapping, and audit cycle configuration. Compliance scope bridges link regimes to services and data classifications. Compliance findings track gaps with severity, status, resolution change sets, and resolution text. Evidence records link to compliance regimes, services, machines, credentials, certificates, and manual operations through typed bridge tables.

---

## Implementation Path

The implementation follows six phases, gated by criteria not calendar.

**Phase 1: Decide cardinality.** One OpsDB or N. Deliverable: documented decision with structural rationale. Days, not weeks.

**Phase 2: Determine baseline schema.** Adapt the 138-entity schema, hand-load representative data from three or more domains, validate the fit. Deliverable: adapted schema repo with representative data. One to two weeks.

**Phase 3: Build dev API and start ingesting data.** Deploy minimal API, write puller scripts against real data sources, iterate on schema. Deliverable: dev OpsDB answering real operational questions by querying. Two to four weeks.

**Phase 4: Determine shared library core.** Build the API client library and logging library, refactor phase 3 scripts to use them, inventory existing operational code. Deliverable: two working libraries, scripts getting smaller. One to two weeks.

**Phase 5: Design and implement change management.** Build production API with full gate, write approval rules, register runners, execute dev-to-operational transition. Deliverable: working governance pipeline with full audit trail. Four to eight weeks.

**Phase 6: Add operational logic.** Pick the most painful domain, build the first operational runner, produce queryable trail. Phase 6 doesn't end — new runners are added as new domains are coordinated. Deliverable: first runner delivering operational benefit. One to two weeks for the first runner.

Total path: two to four months with three to four engineers, not all full-time. With the FOSS project providing the schema, API, libraries, core runners, and importers, the timeline compresses to two to six weeks of configuration and organizational decisions.

---

## What OpsDB Is Not

**Not a wiki.** Long-form prose lives in wiki systems. OpsDB holds structured pointers (runbook references with last-reviewed timestamps, dashboard references with purpose classifications).

**Not a monitoring system.** Full-resolution time series lives in Prometheus, Datadog, or equivalent. OpsDB holds cached summaries and authority pointers for live queries.

**Not a code repository.** Source code and container images live in git repos and registries. OpsDB holds metadata references (runner image references, package data).

**Not a secrets manager.** Secret values live in Vault or equivalent. OpsDB holds pointers (paths, version metadata, rotation timestamps) but never values.

**Not an orchestrator.** OpsDB never invokes work. Runners poll, react, and coordinate through shared data.

**Not a runtime dependency for live services.** Services keep running when OpsDB is unreachable. OpsDB is the coordination substrate, not the data plane. Runners cache configuration locally for partition tolerance.

**Not a replacement for operational skill.** Services still need to run. Monitoring still alerts. Incidents still happen. Capacity still needs planning. OpsDB makes these activities coordinated, governed, and auditable. It does not eliminate them.

---

## The Economics

The cost curve of OpsDB is logarithmic. The up-front investment is real — building (or deploying) the substrate, establishing the schema, standing up the API, writing approval rules, registering runners. Each subsequent domain brought into OpsDB costs less than the one before because the libraries are more complete, the patterns are established, and the schema covers more ground.

The cost curve of the alternative — aggregated operational tooling adopted tool by tool — is linear at best and trends exponential. Each new tool brings its own data model, its own auth story, its own audit trail (or lack of one), its own failure modes. Integrations between tools grow combinatorially. Periodic "consolidation" projects provide temporary relief before fragmentation resumes.

On SOC2 alone, OpsDB pays for itself in the first year. Evidence collection that costs $125-225k annually in engineering time and tooling subscriptions becomes a query against data that was produced as a byproduct of normal operations.

The organizations that benefit most are the ones that have been through two or three consolidation cycles and can see the pattern clearly enough to invest in breaking it. The FOSS project lowers the barrier from "build it" to "deploy it," making the investment accessible to organizations that would never build from scratch but can configure and adopt.
