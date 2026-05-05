# OpsDB API Gate

## What This Document Covers

The API gate is the single path through which every interaction with OpsDB data passes. No direct database access, no out-of-band writes, no SSH-and-fix-it. Every read, every write, every approval, every observation — all of it traverses the same ten-step enforcement pipeline.

This document specifies what the gate does, how each step works, and what the API deliberately refuses to do.

---

## Design Principles

The API is self-contained operational software. It is not built on Kubernetes, not deployed through the infrastructure it models, not dependent on any cloud provider or orchestration platform. It outlives all of them. When the organization migrates from AWS to GCP, or replaces Kubernetes with something else, the API keeps running unchanged because it has no dependency on any of those systems.

The API delegates exactly two concerns externally: human identity verification (to an IdP like Okta, Azure AD, or LDAP) and credential values (to a secret backend like Vault). Everything else — authorization, validation, change management routing, audit logging — is evaluated locally against data stored in OpsDB itself. Changing what the API enforces means changing data rows, not changing code or redeploying.

The API does not invoke runners. It does not send notifications. It does not apply approved change sets. It does not orchestrate workflows. It records state transitions, and runners observe those transitions and act. This is the passive substrate commitment — the API answers queries and accepts writes, and never initiates work.

---

## The Ten-Step Gate

Every operation — read or write, human or runner, routine or emergency — passes through the same pipeline. Steps execute in order. Any step can reject the operation. Rejection at any step produces an audit log entry recording the rejection, the step that rejected, and the reason.

### Step 1: Authentication

The API verifies the caller's identity. For humans, this means validating a signed assertion from the organization's IdP (OIDC token, SAML assertion, or equivalent). For runners, this means validating a service account token issued by the secret backend. The API resolves the verified identity to either an `ops_user` row (humans) or a `runner_machine` row (runners).

Web-application-mediated requests carry both identities — the runner's service account that made the API call and the human identity that initiated the action through the web interface. Both are recorded in the audit log.

If credentials are invalid, expired, or unresolvable to a known identity, the operation is rejected and an audit entry is written recording the failed authentication attempt.

The API ships with three authentication backends: a YAML file backend for development and testing (no external dependencies), OIDC for production human authentication, and service account token validation for runners. The YAML backend enables a zero-dependency bootstrap — you can stand up an OpsDB and start using it without configuring an IdP first.

### Step 2: Authorization

Authorization evaluates five layers, composed with AND logic. Every layer must permit the operation. The first denial halts evaluation — subsequent layers are not checked. The audit entry records which layer denied and the specific policy that triggered the denial.

**Layer 1: Standard Role and Group.** The caller's role memberships (via `ops_user_role_member`) and group memberships (via `ops_group_member`) are checked against the operation class. Roles map to operation classes — a read-only role can perform get and search operations but not writes. This is the baseline access control.

**Layer 2: Per-Entity Governance.** Individual entity rows can carry a `_requires_group` field specifying a group the caller must belong to beyond their standard role. This gates access to sensitive rows — a particular service's configuration might require membership in the team that owns it, even if the caller has a general write role.

**Layer 3: Per-Field Classification.** Fields and tables can carry an `_access_classification` value (public, internal, confidential, restricted, regulated). The caller's clearance level must meet or exceed the classification. When a caller lacks clearance for specific fields, those fields are omitted from read results (with metadata indicating the omission) or the write operation is rejected.

**Layer 4: Per-Runner Authority.** For runner callers, the operation must match the runner's declared scope. The API checks `runner_capability` rows and `runner_*_target` bridge rows (runner_service_target, runner_k8s_namespace_target, runner_cloud_account_target, runner_host_group_target) to verify the runner is authorized to touch the target entity. A runner declared to operate on the production Kubernetes namespace cannot write observations about the staging namespace.

**Layer 5: Policy Rules.** Policy rows of type `access_control` encode additional constraints — time-of-day restrictions (no production changes between 2 AM and 6 AM except emergencies), separation of duty rules (the person who proposed a change cannot approve it), tenure-based restrictions (new employees cannot approve production changes in their first 30 days). Policy rules can either reject the operation outright or inject additional approval requirements into the change management routing.

### Step 3: Schema Validation

The operation's shape is validated against the registered schema. The API reads `_schema_entity_type` and `_schema_field` rows to determine what fields exist for the target entity type, what their types are, and which are required.

For creates, all required (non-nullable, no-default) fields must be present. For updates, every field named in the change must exist in the schema and the value type must match. No unknown fields are accepted — if a field isn't registered in `_schema_field`, the write is rejected.

This validation is data-driven. The API does not hardcode entity structures. When the schema is updated by the schema executor runner and `_schema_*` rows change, the API picks up the new schema on its next refresh cycle. Adding a new entity type to OpsDB means adding YAML files to the schema repo and running the loader — no API code changes.

### Step 4: Bound Validation

Field values are checked against their declared constraints. Numeric fields (int, float) are checked against `min_value` and `max_value`. String fields (varchar) are checked against `min_length` and `max_length`. Enum fields are checked against `enum_values`. Foreign key fields are checked for existence of the referenced row.

No regex is evaluated at this step or any other. Regex is a DoS vector (catastrophic backtracking), introduces dialect variation across implementations, produces unpredictable edge cases, and adds an embedded mini-language into the validation layer. The API uses declarative bounds only — numeric ranges, length ranges, enum membership, foreign key existence, and anchored pattern matching (prefix or suffix only, implemented as string operations not regex).

### Step 5: Policy Evaluation

Policy rows beyond access control are consulted. Data classification policies determine whether the change requires additional handling. Retention policies are checked for consistency. Semantic invariants stored as policy data are evaluated — cross-field constraints like "min_replicas must be less than or equal to max_replicas" or "if status is decommissioned then decommissioned_time must be set."

These cross-field constraints are deliberately kept out of the schema and in policy data. The schema defines per-field bounds (mechanical, rarely changing). Policy data defines cross-field invariants (organizational, changing more frequently). Keeping them separate means schema changes are rare and cheap while invariant changes flow through the normal change management pipeline.

Policy violations either block the operation (fail closed) or produce warnings that require explicit acknowledgment, depending on the policy's configuration.

### Step 6: Versioning Preparation

For change-managed entities, the API prepares the version sibling row. It computes the next `version_serial` for the entity, sets `parent_*_version_id` to point at the current active version, and stages the version row for writing in the execution step.

Version rows contain the full state of the entity at that version — all fields, not just the changed ones. This means point-in-time reconstruction is a single row lookup (find the version active at timestamp T) rather than replaying a chain of deltas. The storage cost is higher. The query cost is O(1) instead of O(N). For an operational database where incident investigation queries need to be fast, this is the correct tradeoff.

### Step 7: Change Management Routing

For write operations against change-managed entities, the API evaluates approval rules and computes who needs to approve the change.

The routing process walks through five steps. First, enumerate the field changes in the change set. Second, walk ownership bridges to find which roles own the affected entities. Third, walk stakeholder bridges to find additional interested parties. Fourth, evaluate approval rule policies against the entity types, namespaces, fields, and metadata involved. Fifth, compute the resulting requirements and write `change_set_approval_required` rows specifying which groups must approve and how many approvers are needed from each group.

The API then determines whether this change set auto-approves or requires human approval. Auto-approval policies (themselves change-managed data) define conditions under which changes can proceed without human intervention — drift corrections in non-production environments, routine credential rotations, reconciler outputs within declared scope bounds. If any matching approval rule requires human approval, the change set enters `pending_approval` status.

This step does not send notifications. It writes state transitions. The notification runner observes those transitions and dispatches through configured channels. The API records, runners act.

### Step 8: Audit Logging

An `audit_log_entry` row is constructed capturing the full context of the operation: who did it (acting_ops_user_id and/or acting_service_account_id), what they did (API endpoint, method, action type), what they targeted (entity type and ID), what they sent and what came back (request and response summaries), whether it succeeded or failed (result status), and when (API-supplied high-precision timestamp).

The audit write is atomic with the operation outcome. If the operation succeeds, the audit entry records success. If the operation fails, the audit entry records the failure. There is no window where an operation completes without an audit record.

The `audit_log_entry` table has no UPDATE or DELETE permissions for any database role, enforced at DDL level. This is not a convention — it is a mechanical constraint. The substrate operator, the API service account, the admin role — none of them can modify or delete audit rows. This is the strictest append-only guarantee in the entire schema.

Optionally, each entry includes an `_audit_chain_hash` covering the entry's contents and the prior entry's hash. This creates a tamper-evidence chain — modifying any historical entry breaks the chain at that point and all subsequent entries. Verification tooling reads the chain forward, recomputes hashes, and detects breaks. A detected break is itself a compliance finding.

### Step 9: Execution

The actual write occurs. For direct writes (observations, evidence), the row is inserted. For change-set submissions, the `change_set`, `change_set_field_change`, and `change_set_approval_required` rows are written. For approvals and rejections, the `change_set_approval` or `change_set_rejection` row is written and the change-set status may transition. For change-set field change applications (by the executor runner), the target entity row is updated and the version sibling row is written.

All writes within a single operation are atomic at the database transaction level. A change-set submission either writes all its rows or none of them. A field change application either updates the entity, writes the version row, and updates the applied status, or does nothing.

### Step 10: Response

The API returns the result with metadata. For successful operations: affected row IDs, computed approvals (for change-set submissions), the audit entry ID for correlation. For failures: structured error information identifying the gate step that rejected, the specific rule or constraint that was violated, and enough detail for the caller to understand what to fix.

---

## API Operations

The API exposes sixteen operations grouped by class.

### Read Operations

**get_entity** fetches one row by primary key. Returns the current state of the entity with all fields the caller is authorized to see.

**get_entity_history** fetches the version chain for one entity. Returns the current state plus all prior versions, ordered by version serial. Optionally filtered by time range or version range.

**get_entity_at_time** reconstructs the field values that were active at a specific timestamp. This is a single lookup against the version sibling table — find the version whose `approved_for_production_time` is the most recent one at or before the requested timestamp. O(1) because version rows contain full state.

**search** is the discovery surface. It accepts filter predicates (equality, inequality, comparison, set membership, anchored patterns, null checks, range, JSON containment), named join paths (service to host group, machine to megavisor parent chain, entity to audit log), projection modes (standard, summary, full with history, explicit field lists), ordering, and pagination (cursor-based by default, offset-based for small result sets).

Search is bounded. Maximum result size, maximum join depth, maximum query time, maximum predicate composition depth, and rate limits per caller are all configurable as policy data. Queries exceeding bounds are refused with structured feedback explaining which bound was hit. This prevents the API from becoming a DoS vector against its own substrate.

Search supports a freshness parameter (`max_staleness_seconds`) for cached observation rows, filtering out observations older than the threshold. The response metadata indicates how many rows were filtered for staleness.

**get_dependencies** walks the substrate hierarchy or service connection graph starting from a given entity. This enables the stack-walking queries runners use — decommission awareness (service to host group to machine to megavisor instance walking the parent chain), failure domain analysis (primary and replica walked to ancestry to compare rack and datacenter), capacity awareness (Kubernetes nodes to underlying machines to hardware sets). Depth and cycle bounds prevent runaway traversal.

**resolve_authority_pointer** performs a where-is-X lookup. Given an authority pointer ID, returns the authority coordinates (base URL, pointer type, locator within the authority). The caller then queries the authority directly. The API resolves the pointer but does not fetch from the authority.

**change_set_view** returns a scoped or full view of a change set for an approver. The view is filtered to the caller's approval scope — an approver sees only the field changes relevant to the approval rules they satisfy.

### Write Operations

**write_observation** is the direct write path for runners writing cached observations, evidence records, runner job output variables, and runner job results. No change set is created. The write passes through authentication, authorization, and runner report key enforcement, then writes directly.

Runner report key enforcement is critical here. Each runner spec declares which keys it is authorized to write to which target tables, with constraints on values. The API looks up `runner_report_key` rows for the runner's spec and target table, verifies the submitted key is in the declared set, validates the value against the key's constraints, and rejects undeclared or invalid keys. This prevents a misconfigured or compromised puller from writing arbitrary data outside its declared surface. The writable surface is declarative, not implicit. Fail closed.

**submit_change_set** proposes N field changes with a reason. The caller provides the field changes (entity type, entity ID, field name, before value, after value for each), a reason text, an optional ticket reference, urgency metadata, and optionally a dry-run flag. The API validates, routes for approval, and records the change set. With dry-run enabled, the full validation pipeline runs but no rows are written — the response shows what approval requirements would be computed.

Each field change carries the version stamp of the entity it was drafted against. At submit time, the API checks each touched entity's current version against the drafted-against version. If any entity has advanced since the change was drafted, the submit fails with a `stale_version` error. The caller retrieves current values, reconciles their proposed changes against the new state, and resubmits. This optimistic concurrency model prevents silent overwrites — no change can be approved against stale state because stale submissions never reach the approval stage.

**emergency_apply** is the break-glass path. Same as submit but with `is_emergency=true`. The caller must have emergency authority per policy (typically on-call engineers with elevated rights). Approval requirements are reduced (often single approver, sometimes self-approved). A `change_set_emergency_review` row is created in pending status with a review window (default 72 hours, configurable). The emergency review monitor runner escalates overdue reviews. Emergency changes are always recorded as such and are queryable — there is no mechanism to file an emergency change that doesn't leave a permanent, conspicuous trail.

**bulk_submit_change_set** handles transactions touching many entities. Validation is chunked (default 1000 field changes per chunk) with interim feedback per chunk. The change set remains one atomic unit — either all chunks validate and the change set transitions together, or it fails together. During apply, if any chunk fails, completed chunks are rolled back via version sibling rows and the change set transitions to failed with a finding filed. Bulk change sets may require one approval for the bundle rather than per-entity approval, driven by policy.

**apply_change_set_field_change** is used by the change-set executor runner to apply one approved field change. The API verifies the caller has executor authority, the change set is in approved status, and the specific field change hasn't been applied yet. It then applies the entity row update and writes the version sibling row.

**mark_change_set_applied** finalizes the change set status after all field changes have been applied. The API verifies all field changes in the set have `applied_status=applied` before transitioning the change set to applied status.

### Change Management Actions

**approve_change_set** records a stakeholder approval. The API verifies the caller is a member of one of the required approver groups. The approval is recorded. If all approval requirements are now fulfilled, the change set transitions to approved status.

**reject_change_set** records a stakeholder rejection with a reason. The change set transitions to rejected status per the rejection semantics of the matching approval rule.

**cancel_change_set** withdraws a change set. Available to the original submitter or someone with sufficient authority.

---

## Change Set Lifecycle

A change set moves through a defined state machine. Terminal states have no outgoing transitions.

**draft** → **submitted**: The submitter has finished composing the change and sends it to the API.

**submitted** → **validating**: Automatic. The API begins running the validation pipeline.

**validating** → **pending_approval**: All validations passed.

**validating** → **draft**: Recoverable validation errors. The submitter can fix and resubmit.

**validating** → **rejected**: Unrecoverable validation errors.

**pending_approval** → **approved**: All required approvals received.

**pending_approval** → **rejected**: A required approver rejected.

**pending_approval** → **expired**: The submission deadline passed without sufficient approvals.

**pending_approval** → **cancelled**: The submitter or sufficient authority withdrew.

**approved** → **applied**: The executor runner successfully applied all field changes. This is a terminal state. Rollback is accomplished by submitting a new change set that restores the prior version's values — standard validation, standard approval, standard audit trail.

**approved** → **applying**: Substate during bulk apply.

**applying** → **applied**: All chunks completed.

**applying** → **failed**: A chunk failed. Completed chunks rolled back via version siblings. Finding filed. Terminal.

---

## What the API Does Not Do

Each of these boundaries is a deliberate refusal that preserves the API's simplicity and the passive substrate commitment.

**Not an orchestrator.** The API does not invoke runners. Runners poll for state transitions (approved change sets, pending notifications, overdue reviews) and act independently. If the API invoked runners, it would become coupled to runner deployment, timing, and failure modes. Runners coordinate through shared data in OpsDB rows, not through API-mediated messaging.

**Not a notification system.** The API does not send emails, Slack messages, pages, or any other notifications. It records state transitions. The notification runner reads those transitions and dispatches through configured channels. Notification infrastructure evolves at a different pace than the API — adding a new notification channel means updating the notification runner, not the API.

**Not a change-set applier.** The API does not execute approved change sets. The change-set executor runner drains the approved queue and applies each field change through the API's `apply_change_set_field_change` operation. If the API applied change sets internally, it would be coupled to executor timing and would need its own retry logic, failure handling, and scheduling — all of which are runner concerns.

**Not a workflow engine.** The change-set lifecycle (draft through applied) is the only workflow the API enforces. Other workflows — incident response, deployment pipelines, capacity planning, escalation sequences — coordinate through OpsDB rows. Runners read relevant state and act. The API does not model or enforce workflow transitions beyond change management.

**Not a search engine.** The API supports structured queries with filters, joins, and predicates over the schema. It does not support full-text search, semantic search, vector search, or document indexing. Those belong in wiki and documentation systems with their own query interfaces.

**Not a dashboard system.** The API serves reads that dashboards consume. It does not build, render, or maintain dashboard state. Dashboard systems sit on top of the API with their own tooling.

**Not a secrets store.** The API resolves authentication credentials from secret backends. It never stores secret values. OpsDB holds pointers to secrets (paths in Vault, references in configuration variables of type `secret_reference`), never the secrets themselves.

**Not a code distribution path.** Runner code, packages, container images, and Helm charts are not stored in OpsDB. Those belong in CI/CD pipelines, container registries, and package repositories. OpsDB holds metadata references (runner image references, package data) but not the artifacts.

---

## Configuration as Data

Every governance decision the API enforces is evaluated against data rows in OpsDB. Authorization policies, approval rules, auto-approval conditions, emergency authority grants, validation constraints, search bounds, rate limits — all of them are rows in policy tables, change-managed through the same change-set discipline as any other configuration.

Changing what the API enforces means submitting a change set that modifies policy data. The change set goes through validation, approval routing, and audit logging. The policy change is itself governed by the governance it modifies — approval rules for modifying approval rules are approved by the schema steward or security team.

This creates a closed loop. The API enforces policy. Policy is data. Changing data goes through the API. The API enforces the change. No escape hatch, no backdoor, no "just update the config file and restart." Every governance change is proposed, validated, approved, applied, and audited through the same pipeline as every other change.
