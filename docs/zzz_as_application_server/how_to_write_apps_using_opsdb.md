# Building Applications on OpsDB

## A Mechanical Reference for Application Developers

---

### 1. What This Is

OpsDB is a governed data substrate. You define your data model in YAML files. A loader turns those files into database tables and runtime metadata. A single API serves every read and write. Every write is validated against your declared schema, checked against access control policies, optionally routed through an approval workflow, versioned with full state, and logged to an append-only audit trail. Backend logic is expressed as small programs called runners that read from the API, act in the world, and write results back through the same API.

You do not build an API layer. You do not build authentication, authorization, validation, versioning, audit logging, change management, or background job infrastructure. You define your entities, write your runners, and build your frontend. The substrate handles everything between the frontend and the database.

This paper explains every mechanical component of that substrate so you can use it properly. It assumes you want to build applications. It does not assume you care about operational infrastructure management, which is the domain OpsDB was originally designed for. The mechanics are the same regardless of what you build on them.

---

### 2. The Schema Engine

#### 2.1 What a schema is

Your data model is a set of YAML files in a git repository. Each file defines one entity type — one table in the database. The file declares the entity's name, its fields with their types and constraints, its relationships to other entities, and metadata about how the system should treat it.

A master manifest file called `directory.yaml` lists every entity file in dependency order. Earlier files cannot reference entities defined in later files. The loader processes the list top to bottom, validating each file against a meta-schema and resolving foreign key references against entities already loaded.

When you run the loader, it reads every file, validates them, generates database DDL appropriate for your storage engine, creates the tables, and populates metadata tables that the API reads at runtime. After the loader finishes, your API can serve requests against every entity you defined.

#### 2.2 The closed constraint vocabulary

The schema language has a fixed set of primitives. You cannot extend it.

**Nine types.** `int` for integers. `float` for floating-point numbers. `varchar` for bounded-length strings. `text` for long strings. `boolean` for true/false. `datetime` for timestamps. `date` for dates without time. `json` for typed structured payloads. `enum` for closed sets of allowed values. `foreign_key` for references to other entities.

**Three modifiers.** `nullable` controls whether a field can be null, defaulting to false. `default` sets a literal default value — no expressions, no function calls, no computed defaults. `unique` declares that a field's values must be unique across all rows, or in combination with other fields via composite unique indexes.

**Constraints per type.** Integer and float fields accept `min_value` and `max_value` for inclusive numeric bounds. Float fields accept `precision_decimal_places`. Varchar and text fields accept `min_length` and `max_length`, with max\_length required for varchar. Enum fields require `enum_values` listing every permitted value. Foreign key fields require `references` naming the target entity. Fields can declare `must_be_unique_within` for composite uniqueness scoping.

That is the entire vocabulary: nine types, three modifiers, and a small set of constraints. Every field in every entity in every application built on this substrate is expressed in these primitives.

#### 2.3 What the vocabulary forbids

The vocabulary is closed specifically to exclude patterns that create complexity in the validation pipeline.

**No regex.** Regular expressions are a DoS vector through catastrophic backtracking, have dialect variation across engines, and introduce an embedded mini-language into schema files. If you need pattern matching, use enum sets for closed alternatives, length bounds for size constraints, or handle richer validation at the API's semantic validation step through policy data.

**No embedded logic.** Every value in a schema file is a literal. The loader does not evaluate expressions. Default values cannot be `NOW()`, `previous_value + 1`, or any computed expression. If you need computed defaults, a runner sets them when creating entities.

**No conditional constraints.** Cross-field invariants like "if status is active then start\_date must be non-null" do not belong in schema files. They belong in policy data evaluated at the API's semantic validation step. This separation exists because schema changes are expensive (governed, versioned, additive-only) while policy changes are routine (also governed and versioned, but added as data rows without schema migration).

**No inheritance.** There is no `extends` directive, no parent entity, no shared base class. Two entities with similar fields each declare their fields independently. The only shared field mechanism is reserved fields — a controlled set of common fields like `id`, `created_time`, and `updated_time` that entities opt into by name.

**No templating.** Schema files are not parameterized. There are no template variables, no macros, no per-environment substitution. One schema per OpsDB instance. Variation across environments is handled through runtime configuration data, not different schemas.

**No imports within entity files.** Entity files do not import other files. Only the master manifest imports. Entity files are leaf-level data.

Each of these refusals closes off a category of complexity that would propagate into every consumer of the API. The loader remains mechanical. Validation remains bounded in time. Schema files remain inspectable as data rather than programs.

#### 2.4 Foreign keys and relationships

Foreign key fields reference another entity by name. The reference always targets the `id` field of the target entity. References to non-primary-key fields are not supported.

Foreign keys follow a naming convention: `referenced_entity_id`. If an entity has two foreign keys to the same target, a role prefix disambiguates: `vendor_company_id` and `client_company_id` both reference the `company` entity.

Self-referential foreign keys create hierarchies. A `location` entity with a `parent_location_id` field referencing itself creates a tree: region → datacenter → rack → shelf. A `comment` entity with a `parent_comment_id` creates threaded discussions.

Polymorphic relationships — where one entity relates to multiple different entity types — use bridge tables. Instead of a polymorphic foreign key column that could point to any table (which breaks referential integrity), you create one bridge table per source-target pair. A `service_ownership` bridge links `service` to `ops_user_role`. A `machine_ownership` bridge links `machine` to `ops_user_role`. Each bridge has clean foreign key constraints to both sides.

#### 2.5 The discriminator pattern for typed payloads

When an entity needs to hold heterogeneous structured data — different JSON payloads depending on a type field — the schema uses a discriminator pattern. A `cloud_resource` entity has a `cloud_resource_type` field (an enum with values like `ec2_instance`, `gcs_bucket`, `azure_vm`) and a `cloud_data_json` field (type json). The `cloud_data_json` field declares `json_type_discriminator: cloud_resource_type`, which tells the API to validate the JSON payload against the schema registered for that type value.

Each discriminator value has its own JSON schema file declaring the fields, types, and constraints for that payload shape. The API reads the discriminator field's value, looks up the registered schema, and validates the payload recursively. If the payload does not match, the write is rejected with a structured error identifying which fields failed and why.

The JSON vocabulary extends the base nine types with two collection types: `list` (ordered, with element type, min/max count, and per-element constraints) and `map` (key-value, with key and value types, max entries, and per-key/value constraints). Lists of lists, maps of lists, and deep nesting are forbidden. If your data needs nested structure beyond one level, the design signals that you should factor it into separate entity types with foreign keys.

This pattern is how you handle heterogeneous data without losing type safety. A `monitor` entity with different configuration shapes per monitor type, a `policy` entity with different rule shapes per policy type, a `schedule` entity with different timing shapes per schedule type — all use the same discriminator pattern.

#### 2.6 Reserved fields

Every entity automatically receives a set of reserved fields that you do not declare in your schema file:

`id` — integer primary key, auto-incrementing. Every entity has one.

`created_time` — datetime, set on insert, never updated. When the row was created.

`updated_time` — datetime, set on insert and on every update. When the row was last modified.

Entities that declare `soft_delete: true` additionally receive `is_active` — a boolean that defaults to true. Deletion sets it to false rather than removing the row. The row persists for history and audit purposes. A reaper runner handles actual removal after the retention horizon.

Entities that declare `hierarchical: true` receive a self-referential foreign key for tree structure.

Entities that declare `versioned: true` receive a versioning sibling table (described in Section 5).

You do not declare these fields. The loader injects them based on your entity's declarations. They appear in the database and in the API's schema metadata.

#### 2.7 Governance fields

Beyond the universal reserved fields, entities can opt into governance fields — underscore-prefixed fields that the API consults for access control and operational behavior:

`_requires_group` — a reference to an access group. The API's authorization layer checks that the caller is a member of this group before allowing access to the row. This provides per-entity access scoping.

`_access_classification` — a string indicating data sensitivity (public, internal, confidential, restricted, regulated). The API's authorization layer checks the caller's clearance level against this classification. This provides per-field access control when applied to specific fields.

`_retention_policy_id` — a reference to a retention policy entity that overrides the default retention for this entity type. The reaper runner consults this when deciding how long to keep version history.

`_audit_chain_hash` — used for cryptographic tamper evidence on the audit log. Each entry hashes its own contents plus the prior entry's hash, creating a chain where modification of any historical entry is detectable.

Governance fields are declared in the schema file separately from operational fields for visual clarity, but the loader treats them identically. They produce columns, they appear in schema metadata, and the API validates writes against their constraints like any other field.

---

### 3. The API Gate

#### 3.1 One gate, every operation

Every interaction with your data passes through a single API. Your frontend calls it. Your runners call it. Your admin tools call it. There is no direct database access for application purposes. There is no second API with different rules. There is no out-of-band path that bypasses validation or audit.

The API exposes sixteen operations organized into four classes:

**Read operations.** `get_entity` fetches one row by primary key. `get_entity_history` fetches the version chain for one entity. `get_entity_at_time` reconstructs the field values that were active at a specific timestamp. `search` provides filtered, joined, paginated queries across entity types. `get_dependencies` walks relationship graphs. `resolve_authority_pointer` looks up where external facts live. `change_set_view` shows the current state of a change set to an approver.

**Direct write operations.** `write_observation` allows a runner to write cached data, evidence records, or output variables. `apply_change_set_field_change` allows the change-set executor to apply one approved field change.

**Change set operations.** `submit_change_set` proposes a bundle of field changes with a reason. `emergency_apply` submits with reduced approval requirements for break-glass situations. `bulk_submit_change_set` handles large multi-entity changes with chunked validation.

**Change management actions.** `approve_change_set`, `reject_change_set`, `cancel_change_set`, and `mark_change_set_applied` manage the lifecycle of proposed changes.

#### 3.2 The ten-step pipeline

Every operation traverses a ten-step enforcement pipeline. Steps are sequential. The first failure halts the pipeline. The response identifies which step failed and why.

**Step 1: Authentication.** The API verifies the caller's identity. For humans, this delegates to an identity provider via SSO — the API receives a signed assertion and resolves it to an internal user identity. For runners, this validates a service account credential issued by a secret backend. The API does not store passwords or secrets. It delegates identity verification and consumes the result.

**Step 2: Authorization.** Five layers, composed via AND — all must pass, first denial halts. Described in Section 4.

**Step 3: Schema validation.** For write operations, the API checks that every field in the request matches the registered schema for the target entity type. The field must exist. The type must match. Required fields must be present on creates. The check is mechanical — it reads the schema metadata tables and compares.

**Step 4: Bound validation.** For write operations, the API checks that every field value falls within its declared constraints. Numeric values within min/max range. String lengths within min/max bounds. Enum values within the declared set. Foreign key targets exist. JSON payloads match the schema registered for their discriminator value. No regex is evaluated. Every check is a comparison or a lookup.

**Step 5: Policy evaluation.** The API consults policy data rows for additional governance. Data classification policies may restrict who can write certain values. Retention policies may affect how the write is handled. Separation-of-duty policies may prevent the same person from both proposing and approving a change. Cross-field invariants declared as policy rows are evaluated here — "if status is active then start\_date must be set" is a policy row, not a schema constraint.

**Step 6: Versioning preparation.** For writes to versioned entities, the API prepares a version row to capture the full state after the write. This step only runs for change-managed entities with versioning enabled.

**Step 7: Change management routing.** For change set submissions, the API evaluates approval rule policies to determine who needs to approve the change. It walks ownership and stakeholder bridges to find the relevant approver groups. It writes `change_set_approval_required` rows specifying which groups must approve and how many approvers are needed from each. If policy rules determine the change qualifies for auto-approval, the change set transitions to approved without human intervention.

**Step 8: Audit logging.** The API writes an entry to the append-only audit log recording the caller's identity, the operation, the target entity, the outcome, and contextual metadata. This step runs on both success and rejection paths. Every interaction is recorded regardless of whether it succeeded.

**Step 9: Execution.** The actual database operation — insert, update, delete, or change set creation — executes atomically with the audit log entry.

**Step 10: Response.** The API returns the result with metadata: affected row IDs, computed approval requirements, audit entry ID for correlation, and any warnings.

#### 3.3 What this means for your application

Your frontend makes API calls. Each call passes through ten steps. The response is either a successful result with metadata or a structured error identifying which step failed, which field or policy caused the failure, and what the expected constraint was.

You do not implement validation in your frontend and hope it matches the backend. The backend is the validation. Your frontend can do client-side validation for user experience, but the API is the authoritative check. A field that the schema says is an integer between 1 and 100 will be rejected at step 4 if you send 101, regardless of what your frontend allows.

You do not implement authorization in your application code. The API enforces it. A user who lacks the required group membership for an entity will not receive that entity in search results and will be rejected on write attempts. Your frontend renders what the API returns. If the API omits a field due to access classification, your frontend does not need to know why — it simply does not receive the field.

You do not implement audit logging. Every API call produces an audit entry. You can query the audit log through the same search API you use for everything else. "Show me every change to this entity in the last month" is a search query, not a custom reporting feature you build.

---

### 4. Authorization

#### 4.1 The five layers

Authorization is evaluated at step 2 of the gate pipeline. Five layers compose via AND — all must pass, first denial halts. The response identifies which layer denied and which specific policy or membership caused the denial.

**Layer 1: Standard role and group.** Every user has role memberships (through `ops_user_role_member`) and group memberships (through `ops_group_member`). Roles map to operation classes — which types of operations the user can perform. Groups map to entity scopes — which collections of entities the user can access. A user with the "editor" role in the "billing" group can read and write billing entities but not HR entities.

**Layer 2: Per-entity governance.** Individual entities can carry a `_requires_group` governance field specifying a group that the caller must belong to, beyond whatever layer 1 permits. This allows row-level access control. A project entity with `_requires_group = "project_alpha_team"` is only accessible to members of that group, even if the caller has a role that normally permits access to project entities in general.

**Layer 3: Per-field classification.** Individual fields can carry an `_access_classification` (public, internal, confidential, restricted, regulated). The caller's clearance level must meet or exceed the field's classification. If a caller's clearance allows "internal" but a field is classified "confidential," the API omits that field from read responses and rejects write attempts that include it. The response includes metadata indicating that fields were omitted due to access classification.

**Layer 4: Per-runner authority.** When the caller is a runner (identified by service account credentials), the API checks that the operation falls within the runner's declared scope. A runner's scope is defined by capability declarations and target bridge rows in the schema — which entity types it can read, which tables it can write to, which external systems it can access. A metrics puller runner that is declared to write only to `observation_cache_metric` cannot write to `invoice` or `patient_record`.

**Layer 5: Policy rules.** Additional constraints expressed as policy data rows. Time-of-day restrictions (certain operations only during business hours). Separation of duty (the person who proposed a change cannot approve it). Tenure-based access (new employees cannot access restricted data for 90 days). Custom constraints specific to your application domain. Policy rules can deny an operation or inject additional approval requirements into a change set.

#### 4.2 How this gives you multi-tenancy

If your application serves multiple customers or teams, the authorization layers provide tenant isolation without any custom code.

Layer 1 gives each tenant a group. Users belong to their tenant's group. Entities belong to their tenant's scope via naming convention or explicit group assignment.

Layer 2 gives each entity row a `_requires_group` value pointing to the owning tenant's group. Search results automatically exclude entities the caller cannot access. Write attempts to another tenant's entities are denied.

Layer 3 gives sensitive fields per-field classification. A customer's payment information is classified "restricted." Only users with sufficient clearance see it.

The read scaling cache keys entries by access scope, preventing cross-tenant data leakage through the cache. Two users with different group memberships querying the same entity type receive separate cache entries with different results.

Changing tenant access configuration is a change set. Adding a user to a tenant group, modifying access classifications, adjusting policy rules — all go through the standard pipeline with validation, approval, versioning, and audit.

---

### 5. Versioning

#### 5.1 How versioning works

When you declare `versioned: true` on an entity, the loader generates a versioning sibling table. If your entity is `service`, the sibling is `service_version`. The sibling table contains all of the parent entity's fields plus versioning metadata: a monotonic version serial number, a link to the prior version, a link to the change set that produced the version, whether this is the currently active version, and when it was approved for production.

Every write to a versioned entity creates a new row in the sibling table containing the full state of the entity after the write. This is a full-state snapshot, not a delta. The version row contains every field value, not just the ones that changed.

This design choice trades storage for reconstruction speed. Reconstructing the state of an entity at any point in time is a single row lookup in the version table — find the version that was active at that timestamp and read its fields. The alternative — storing only deltas and replaying the chain from the beginning — requires reading every version from the start to the target time. At 200 versions, chain replay reads 200 rows. Full-state lookup reads 1 row.

#### 5.2 What gets versioned

Entities fall into four categories:

**Change-managed.** Versioned entities whose writes go through the change set pipeline. Every change produces a version row linked to the change set that caused it. This is the default for application domain entities — your projects, tasks, invoices, patients, recipes, whatever your application manages.

**Observation-only.** Entities written by runners as cached state from external sources. These are overwritten or appended each cycle. The external source is the source of truth, not OpsDB. Examples: cached metrics from a monitoring system, pod status from Kubernetes, payment status from Stripe. These are not versioned because the volume is too high and the source of truth is external.

**Append-only.** The audit log. No UPDATE or DELETE permission for any role, enforced at the database level. Entries are written by the API and never modified.

**Computed by tooling.** The schema metadata tables. Populated by the schema loader when a schema change is applied. Not directly written by users or runners.

#### 5.3 Point-in-time reconstruction

The `get_entity_at_time` operation takes an entity type, an entity ID, and a timestamp. It returns the field values that were active at that moment. Because version rows contain full state, this is a single query: find the version row for this entity whose active window contains the requested timestamp.

This enables your application to answer "what did this look like at time T" for any versioned entity. What was the project's status when the incident happened. What were the invoice line items before the correction. What were the access control rules when the disputed action occurred. Each is a single API call.

#### 5.4 Rollback

Rollback is not a special operation. It is a change set that proposes field changes restoring the values from a prior version. You query the version history to find the version you want to restore, construct field changes setting each field to its value in that version, and submit the change set through the standard pipeline.

The rollback change set goes through validation, approval routing, and audit like any other change set. The rollback itself becomes a versioned event in the history. There is no side channel, no special undo button that bypasses governance. The history records: version 5 was the change, version 6 was the rollback, and the change set for version 6 references the version it restored.

---

### 6. Change Sets

#### 6.1 What a change set is

A change set is a bundle of one or more proposed field changes submitted with a stated reason. It is the unit of mutation for governed data. Instead of directly updating a row, you propose a set of changes, the system validates and routes them, approvers approve or reject, and an executor applies the approved changes atomically.

Each change set contains:

A list of field changes, each specifying: target entity type, target entity ID, target field name, the value before the change, the value after the change, whether this is a create, update, or delete, and an apply order.

A reason text explaining why the changes are being made.

Metadata: who proposed it (human user or runner), whether it is an emergency, whether it is a bulk change, an optional link to an external ticket, and an expected apply time.

#### 6.2 The lifecycle

A change set moves through these states:

**Draft.** Under construction. The submitter is building the field change list. Not yet submitted to the API.

**Submitted.** The API has received the change set. Validation begins automatically.

**Validating.** Schema validation, bound validation, semantic validation, policy validation, and lint checks run against every field change. If all pass, the change set moves to pending approval. If validation fails, the change set returns to draft with structured errors identifying every failure.

**Pending approval.** Validation passed. The API has computed which approver groups are required and how many approvers from each group must approve. A notification runner detects change sets entering this state and dispatches notifications to the required approvers through configured channels.

**Approved.** All required approvals have been received. The change set is ready for execution. A change-set executor runner detects approved change sets and applies each field change through the API.

**Applied.** The executor has successfully applied all field changes. Each field change updated the target entity row and created a version row in the versioning sibling table. The change set's applied time is recorded.

**Terminal states.** Rejected (an approver rejected), expired (the approval deadline passed without sufficient approvals), cancelled (the submitter withdrew), failed (execution failed during a bulk apply, with rollback of partially applied changes).

#### 6.3 Auto-approval

Not every change set requires human approval. The approval routing step evaluates approval rule policies against the change set's contents. If no matching rule requires human approval — because the entity types are low-stakes, the fields are non-sensitive, the change is within defined bounds, or the proposing runner has auto-approval authority for this scope — the change set transitions directly from pending approval to approved.

The change set still exists. It is still validated, versioned, and audited. The approval trail shows that it was auto-approved per the matching policy rule. The only difference is that no human was asked to approve.

This means your application can have immediate writes for routine changes while requiring human approval for consequential ones. Updating a task's description auto-approves. Modifying billing configuration requires the finance team's approval. Both go through the same pipeline. The difference is policy data, not code.

#### 6.4 Emergency changes

When a production incident requires immediate changes that cannot wait for normal approval, the emergency path provides reduced approvals with mandatory post-incident review. The caller must have emergency authority per policy. The change set is flagged as emergency. An emergency review record is created in pending status with a configurable review window (default 72 hours). A verifier runner monitors overdue reviews and escalates.

Emergency changes are always recorded as emergency in the audit trail. They are always reviewed eventually. The emergency path exists because incidents are real and blocking all changes pending approval during an outage causes harm. The review requirement exists because bypassing normal approval without accountability causes different harm.

#### 6.5 Optimistic concurrency

When you draft a change set, each field change carries a version stamp — the version of the entity you drafted against. When you submit the change set, the API checks each touched entity's current version against the version you drafted against. If any entity has advanced since you drafted — because someone else submitted a change to the same entity — the submission fails with a `stale_version` error identifying which entities are stale.

This is optimistic concurrency. No locks are held during drafting. Conflicts are detected at submit time, not at draft time. The submitter fetches the current state, reconciles their proposed changes against the new state, and resubmits.

This prevents silent overwrites. If two people independently change different fields on the same entity, the second submission will fail and require the submitter to reconcile. This is intentional. Silent overwrites are a data integrity problem that is worse than the inconvenience of reconciliation.

#### 6.6 Bulk changes

When a change touches many entities — a fleet-wide configuration update, a policy rollout, a coordinated migration — the bulk change set bundles all changes into a single atomic unit. Validation is chunked (default 1000 field changes per chunk) to provide early feedback without waiting for the full set to validate. Apply is chunked to manage resource consumption. If any chunk fails during apply, completed chunks are rolled back through version sibling rows and the change set transitions to failed with a finding filed.

Bulk change sets can have different approval rules than individual change sets. A fleet-wide credential rotation that would require 500 individual approvals can be approved once as a bundle per policy.

---

### 7. The Search API

#### 7.1 Query structure

The search API provides structured queries across entity types. A query specifies:

**Filter predicates.** Equality (`field = value`), inequality (`field != value`), comparison (`field > value`, `field >= value`, `field < value`, `field <= value`), set membership (`field IN [values]`), anchored pattern matching (`field LIKE 'prefix%'` — no regex), null checks (`field IS NULL`, `field IS NOT NULL`), range (`field BETWEEN low AND high`), and JSON containment (`field_data_json @> path = value`). Predicates compose via AND, OR, and NOT with explicit grouping and bounded depth.

**Named join paths.** Relationships between entity types are registered as schema metadata. You can traverse them in queries: `service.host_group` walks from a service to its host group, `change_set.field_changes` walks from a change set to its field changes, `machine.megavisor_instance.parent_chain` walks up the substrate hierarchy. Join paths have cycle detection and configurable depth bounds.

**Projection.** Which fields to return. Standard returns all accessible fields. Explicit field lists return only specified fields. The API filters the projection by access classification — fields the caller cannot see are omitted regardless of what was requested.

**Ordering.** A list of field and direction pairs. Ties are broken by `id` for stable cursor-based pagination.

**Pagination.** Cursor-based by default. Cursors are opaque tokens encoding the snapshot and ordering keys. Offset-based pagination is available for result sets under a configurable threshold (default 10,000).

**Freshness.** For queries against cached observation data, a `max_staleness_seconds` parameter filters out rows whose `_observed_time` is older than the threshold. The response indicates how many rows were filtered.

**View modes.** `standard` returns current state. `with_history` returns current state plus the full version chain. `at_time` returns state reconstructed at a specified timestamp.

#### 7.2 Bounds

Every query is bounded. Maximum result size, maximum join depth, maximum query time, maximum predicate composition depth, and rate limits per caller. Bounds are configurable per role as policy data. Queries exceeding bounds are rejected with structured feedback identifying which bound was hit and what the limit is.

Bounds exist because unbounded queries are a denial-of-service vector against your own database. A query that joins five entity types, applies fifteen predicates, and returns a million rows will consume resources that affect every other user of the system. Bounds prevent this by making resource consumption explicit and configurable.

#### 7.3 Using search as your query layer

For most application read patterns, the search API is your query layer. List views are search queries with filters and pagination. Detail views are `get_entity` calls. History views are `get_entity_history` calls. Dashboard aggregations are search queries with appropriate filters.

For read patterns that require denormalized data — computed aggregations, pre-joined views, materialized summaries — you write a runner that reads from governed entities, computes the denormalized form, and writes the results to observation cache tables. Your frontend reads from the cache tables through the same search API. The cache is rebuildable from the governed source. If you lose it, the runner rebuilds it on its next cycle.

---

### 8. Runners

#### 8.1 What a runner is

A runner is a small, single-purpose program that follows a three-phase pattern:

**Get.** Read data from OpsDB through the API. Read data from external sources through shared libraries. No side effects in this phase. The runner gathers everything it needs to decide what to do.

**Act.** Perform work in the world through shared libraries. Create cloud resources, send notifications, apply Kubernetes manifests, query external APIs, render templates. Every action is bounded by the runner's declared configuration: retry budget, execution time, scope.

**Set.** Write results back to OpsDB through the API. Record what happened, what succeeded, what failed. Every write produces an audit trail.

A runner is typically 150 to 300 lines of domain-specific logic plus library calls. The shared library suite handles authentication, retry, circuit breaking, logging, metrics, and interaction with external systems. The runner handles the decision logic: what to read, what to do with it, and what to write back.

#### 8.2 Runner kinds

Ten standard patterns cover the common cases:

**Puller.** Reads from an external system and writes observations to OpsDB cache tables. A Stripe puller reads payment statuses. A weather puller reads forecasts. A GitHub puller reads repository metadata. The external system is the source of truth. OpsDB holds a cached snapshot.

**Reconciler.** Compares desired state from OpsDB entities with observed state from cache tables or external queries. Computes the difference. Either corrects it directly (for low-stakes changes) or proposes a change set for approval (for high-stakes changes). A subscription reconciler compares what the customer should have provisioned versus what is actually provisioned and corrects drift.

**Verifier.** Checks that scheduled work happened or that state meets expectations. Writes a structured evidence record with pass/fail status and detail. A backup verifier confirms today's backup completed. An SLA verifier checks that service level metrics are within bounds. A compliance scanner evaluates policies against entity configurations.

**Change-set executor.** Reads approved change sets from OpsDB. Applies each field change through the API. Marks the change set as applied when all field changes succeed. This is the runner that closes the loop between approval and execution.

**Reaper.** Reads retention policy data. Removes or soft-deletes data past the retention horizon. Keeps the database from growing indefinitely by enforcing the lifecycle rules declared in policy data.

**Drift detector.** A reconciler variant that proposes change sets instead of acting directly. Finds discrepancies between desired and observed state and files them as proposed changes for human review.

**Reactor.** Responds to events rather than scheduled cycles. Processes webhook callbacks, watches for state transitions, handles external notifications. Paired with a reconciler backstop that catches anything the reactor misses.

**Scheduler.** Enforces runner schedules on target infrastructure. Ensures cron entries, Kubernetes CronJobs, or systemd timers match what OpsDB declares.

**Bootstrapper.** Sets up new resources from minimal state. Provisions a new tenant, initializes a new service instance, bootstraps a new machine into the managed fleet.

**Failover handler.** Detects primary failure and performs failover. Updates OpsDB with the new topology. Writes evidence records documenting what happened.

#### 8.3 Runner disciplines

Three disciplines constrain every runner:

**Idempotency.** Running the same inputs against the same state produces the same end state. If a runner crashes mid-cycle and restarts, re-running the cycle does not create duplicates, send duplicate notifications, or apply changes twice. Where natural idempotency is not possible (sending an email is inherently non-idempotent), uniqueness keys prevent re-execution.

**Level-triggered.** Runners react to current state, not event history. A reconciler reads the current desired and observed state on every cycle and computes the diff. If an event was missed — a webhook that did not arrive, a queue message that was dropped — the next cycle's state comparison catches the discrepancy. The system converges to correct state through repeated observation, not through processing a complete event log.

**Bounded.** Every runner has explicit limits declared in its configuration data: maximum retry count, maximum execution time, maximum scope per cycle, maximum memory consumption. If a bound is hit, the runner records which bound was hit in its job output, stops cleanly, and the next cycle continues from current state. Unbounded runners consume unbounded resources and fail unpredictably.

#### 8.4 Gating modes

The gating mode determines whether a runner's writes go through change management:

**Direct write.** For observation data, evidence records, and runner job output. The write is authenticated, validated, and audited, but it does not create a change set or route through approval. Observation data is a record of what was observed, not a proposed change to governed state.

**Auto-approved change set.** For routine operational changes where a change set is desirable for tracking and audit but human approval is not needed. The change set is created, validated, and audited. Approval policies evaluate and determine that auto-approval is appropriate. The change set transitions to approved without human intervention. Examples: routine drift correction, scheduled credential rotation, minor configuration updates within declared bounds.

**Approval-required change set.** For changes where human judgment is needed. The change set routes to the appropriate approver groups. A notification runner alerts the approvers. The change waits until sufficient approvals are received. Examples: production database changes, security policy modifications, compliance-scope changes.

The same runner can have different gating modes for different targets. A drift correction runner might auto-approve corrections in a staging environment, require approval for corrections in production, and refuse to act on compliance-restricted entities (filing a finding instead). The gating mode is determined by policy data evaluated against the runner's target declarations, not by logic in the runner itself.

#### 8.5 Runner authority as data

A runner's authority — what it can read, what it can write, what external systems it can access — is declared in OpsDB as data. Capability rows declare what the runner can do. Target bridge rows declare which entities it can target. Report key rows declare which specific keys it can write to observation tables.

The API checks these declarations at step 2 layer 4 of the gate pipeline. The shared library suite checks them before making world-side calls. If a runner attempts an operation outside its declared scope, both the API and the library reject the attempt with a structured error and audit entry.

Changing a runner's authority means submitting a change set that modifies its capability or target rows. The change goes through approval. The audit trail records who authorized the expansion. If a runner's scope needs to be narrowed — because it was over-provisioned or because its role changed — the same process applies. Runner authority is governed data, not configuration in a deployment manifest.

---

### 9. The Shared Library Suite

#### 9.1 What the libraries do

Runners interact with the outside world through a standardized library suite. Each library wraps a category of operations with consistent authentication, retry handling, error classification, correlation ID propagation, and scope validation.

**OpsDB API client** (mandatory). The only path runners use to read from and write to OpsDB. Handles authentication, correlation IDs linking every API call back to the runner job, automatic retry on stale version errors (configurable), structured error handling, and report key fail-fast validation before round-trips.

**Kubernetes operations** (conditional). Apply manifests, query resources, watch streams, Helm operations. Validates operations against the runner's declared Kubernetes namespace targets before making calls.

**Cloud operations** (conditional). Provider-agnostic interface with per-provider backends for AWS, GCP, Azure. Validates against the runner's declared cloud account targets.

**Secret access** (conditional). Reads secrets from Vault, AWS Secrets Manager, or equivalent at runtime. Never persists secret values. Never writes them to OpsDB. Never logs them. Secrets exist in memory only during the call that needs them.

**Logging and metrics** (mandatory). Structured log format with runner job ID, correlation ID, runner spec name, version, and source location on every line. Metrics emission validated against runner capability declarations. Consistent format across all runners enables aggregation, correlation, and dashboards.

**Tracing** (mandatory). Distributed trace context propagation. Trace IDs correlate with audit log entries and runner job records. You can pivot from a trace span to the OpsDB audit entry for the API call that span represents.

**Retry and resilience** (optional). Exponential backoff with jitter, circuit breaking with per-target state, hedged requests for tail latency reduction, bulkhead isolation for failure domain separation. These compose inside every outbound library call.

**Notification** (conditional). Channel-agnostic notification dispatch. Email, Slack, webhook, paging — configured as OpsDB data rows pointing to the channel endpoints. The library resolves recipients through on-call assignments and escalation paths, dispatches through the configured channel, and records the result.

**Templating** (conditional). Variable substitution and file inclusion. No expressions, no conditionals, no function calls. Templates are data that produce output by combining with values. Logic that would go into templates goes into upstream runners that produce concrete values as configuration variables.

**Git operations** (conditional). Clone, commit, push, tag, create pull request. Authenticated via secret backend credentials. Structured commit messages link back to change set IDs for audit trail composition.

#### 9.2 Why libraries enforce scope

Every world-side library validates operations against the runner's declared scope before making external calls. The Kubernetes library checks the runner's namespace targets. The cloud library checks cloud account targets. The notification library checks notification scope and paging authority.

This creates two-sided enforcement. The API gate enforces scope on OpsDB writes (steps 1-10). The library suite enforces scope on world-side actions (before reaching external systems). Both check the same declarations — runner capability and target rows in OpsDB. Both fail closed — if the declaration is missing, the operation is denied.

The result is that a runner cannot act outside its declared scope through any path. Unauthorized OpsDB writes are caught at the API gate. Unauthorized world-side actions are caught at the library layer. Both produce structured errors and audit entries. Investigation follows the same path: denial → runner spec → declarations → change set that created the declarations → approver who authorized.

#### 9.3 Why the library keeps runners small

Without the library suite, a runner that reads Kubernetes pod status and writes it to OpsDB would need to: authenticate to the Kubernetes API, handle pagination of large pod lists, retry on transient errors with backoff and jitter, detect and handle API version differences, authenticate to OpsDB, construct observation writes with proper timestamps and runner job references, handle stale version errors, log every operation in a consistent format with correlation IDs, emit metrics in the expected format, and propagate trace context.

With the library suite, the same runner reads pod status through the Kubernetes library (which handles auth, pagination, retry, and version compatibility) and writes observations through the OpsDB API client (which handles auth, correlation, retry, and error classification). The runner's code is the decision logic: which pods to read, how to transform the data, which observation keys to write. The infrastructure is library calls.

This is why runners are typically 150 to 300 lines. The library suite handles the 1000+ lines of infrastructure that every runner would otherwise reimplement, inconsistently, with different error handling, different retry logic, and different logging formats.

---

### 10. Data-Driven Behavior

#### 10.1 The principle

In a conventional application, behavior is encoded in code. Validation rules are if-statements in endpoint handlers. Approval workflows are procedural code that checks conditions and sends notifications. Access control is middleware that reads configuration files. Notification routing is hardcoded channel selection. Changing any of these requires a code change, a test cycle, and a deployment.

In OpsDB Application Architecture, these behaviors are expressed as data rows that the gate pipeline and runners consult at runtime.

#### 10.2 What is data-driven

**Validation.** Per-field constraints are schema declarations. Cross-field invariants are policy rows evaluated at the API's semantic validation step. Adding a new invariant — "if priority is critical then assignee must be set" — means creating a policy row, not modifying code.

**Approval routing.** Which changes require approval, from which groups, with how many approvers. Approval rules are policy rows that match against entity types, fields, namespaces, data classifications, and security zones. Changing the approval workflow means changing policy data.

**Access control.** Which roles can access which entities, which fields, under which conditions. Role memberships, group memberships, per-entity governance fields, per-field classifications, and policy rules are all OpsDB data.

**Notification routing.** Which state transitions trigger notifications, through which channels, to which recipients. Authority rows point to channel endpoints. On-call assignments determine who receives pages. Escalation paths define the sequence of notifications. All data.

**Retention.** How long version history and cached data are retained per entity type. Retention policies are data rows. A reaper runner reads them and enforces them.

**Scheduling.** When runners execute, when backups happen, when certificates expire. Schedule entities with typed payloads — cron expressions, rate-based intervals, event triggers, calendar-anchored dates.

**Runner configuration.** What each runner does, what it targets, what bounds it operates within. Runner spec entities with typed JSON payloads.

**Runner authority.** Which entities a runner can read, which tables it can write to, which external systems it can access. Capability rows and target bridge rows.

#### 10.3 What this means for your application

Every one of these is changeable through the standard change set pipeline. Every change is validated, attributed, approved (or auto-approved per policy), versioned, and audited. Your application changes its behavior without deployment. And every behavioral change has the same governance properties as every data change.

You can query "who changed the approval rules for this entity type, when, and what was the prior configuration" the same way you query any other entity's history. You can reconstruct the access control configuration that was in effect at any point in time. You can audit every behavioral change with the same tools you use to audit data changes.

---

### 11. The Audit Trail

#### 11.1 What is recorded

Every API operation produces an audit log entry. The entry records:

The operation identity: which API endpoint was called, which HTTP method, and a structured action type (read, create, update, delete, approve, reject, change\_set\_submit, schema\_change).

The caller identity: the authenticated user ID for human actions, the service account ID for runner actions, or both for operations where a runner acts on behalf of a human.

The target identity: entity type and entity ID. Multi-target operations use bridge tables for per-target detail.

The operation detail: a summary of the request data and response data. Full values are stored in version sibling rows and cache tables. The audit entry carries a summary for quick scanning.

The result: success, validation\_failed, authorization\_denied, rate\_limited, or internal\_error.

Context: client IP address, user agent, a request ID for correlation, and the change set ID where applicable.

A timestamp: API-supplied, high-precision. Client-supplied timestamps are recorded in the summary but not used for the `acted_time` field.

Optional tamper evidence: a cryptographic hash covering the entry's contents plus the prior entry's hash. Modifying any historical entry breaks the chain at that point and all subsequent entries.

#### 11.2 Append-only enforcement

The audit log table has no UPDATE or DELETE permission for any database role, including the substrate operator (DBA). This is enforced at the database DDL level. The API writes entries. Nothing modifies or removes them.

If an audit entry needs correction — a malformed entry, an invalid timestamp, a partial write — the correction is recorded in a separate anomaly table. The original entry is never modified. The anomaly table references the original entry and describes the issue.

Retention of audit entries is governed by policy data, typically aligned with compliance requirements (7+ years for SOC2, SOX, and similar frameworks). The reaper runner does not touch audit entries without explicit retention configuration that permits it. Deletion of audit rows, if ever permitted by retention policy, is recorded in a separate audit-of-audit table with at-least-as-long retention.

#### 11.3 Querying the audit trail

The audit log is queryable through the same search API as any other entity. Filter by entity type, entity ID, time range, caller identity, action type, or result. Join to change set records, version history, and evidence records through named join paths.

Common queries your application can support without custom code:

"Every change to entity X in the last month" — filter by target entity ID and time range.

"Every action by user Y today" — filter by caller identity and time range.

"Every authorization denial for runner Z" — filter by caller identity and result type.

"Every emergency change in Q3" — filter by action type and change set emergency flag.

"Every approval by user Y" — filter by action type = approve and caller identity.

"The complete provenance chain for this entity's current state" — get entity history joined to change sets joined to approvals.

---

### 12. Schema Evolution

#### 12.1 What you can change

The schema evolves through additive changes:

Add new fields (must be nullable, since existing rows will not have a value).

Add new enum values (existing rows holding previous values remain valid).

Widen numeric ranges (decrease min\_value or increase max\_value).

Widen string length bounds (increase max\_length).

Add new entity types.

Add new indexes.

Add new approval rule references.

#### 12.2 What you cannot change

The following changes are forbidden by absolute rule:

**Delete fields or entities.** Version rows reference field values. Audit log entries reference field changes. Deleting a field would orphan historical data or require rewriting history, which is forbidden. Deprecated fields remain in the schema. The column persists. The data remains queryable. The `_schema_version_deprecated_id` field marks when deprecation occurred.

**Rename fields or entities.** Every consumer — runners, audit entries, version history, search queries — references fields and entities by name. Renaming breaks all of them. Instead: add a new field with the new name, deprecate the old field. Both coexist. Names are absolute forever.

**Change field types.** An int field that becomes a float breaks every consumer expecting an integer. An enum field that loses a value breaks rows holding that value. Instead: the six-step duplication pattern described below.

**Narrow numeric ranges.** Existing rows might hold values that are now out of range. Widening is the only allowed direction.

**Remove enum values.** Existing rows might hold the removed value. Add a new field with the narrower set and deprecate the old field.

**Tighten uniqueness constraints.** Existing rows might violate the new constraint.

#### 12.3 The duplication pattern

When you need to make a change that would be forbidden — changing a field's type, narrowing an enum, restructuring a field — the six-step duplication pattern converts it into a sequence of allowed changes:

Step 1: Add the new field alongside the old field. Both exist in the schema and database.

Step 2: Begin writing to both fields. All code that writes to the old field is updated to also write to the new field.

Step 3: Migrate readers. Code that reads from the old field is updated to read from the new field.

Step 4: Mark the old field deprecated in the schema metadata.

Step 5: Continue writing to both fields for a safety period. The old field becomes a tombstone — values are written but nothing reads them.

Step 6: The old field is never removed. It remains as a deprecated column. Storage cost is the price of stable history.

#### 12.4 Schema changes as governed change sets

Schema changes flow through a specialized change management path. A developer edits schema YAML files and opens a pull request. The schema steward reviews structural integrity, naming conventions, and comprehensive coherence. CI can run the loader independently on the proposed changes.

On merge, a diff is computed between the current schema (in the `_schema_*` metadata tables) and the new schema (in the merged repository). The diff is expressed as schema evolution operations: add entity, add field, widen enum, add index, mark deprecated. These operations are submitted as a schema change set with stricter approval rules than normal data changes.

When approved, a specialized schema executor runner reads the approved change set, generates appropriate DDL for the storage engine, applies the DDL atomically where possible, updates the schema metadata tables, and marks the change set as applied. The schema metadata in OpsDB now matches the repository.

---

### 13. Connecting to Specialized Systems

#### 13.1 When the gate pipeline is too heavy

Some applications have processing requirements that the ten-step gate pipeline cannot serve: sub-millisecond latency for real-time operations, thousands of writes per second for event ingestion, specialized computation like machine learning inference or physics simulation.

These processing requirements do not mean OpsDB is unsuitable for the application. They mean the application has a hot path that needs a specialized system and a governed state layer that OpsDB manages.

#### 13.2 The connection pattern

OpsDB manages the governed state: accounts, configuration, policies, rules, schedules, access control, audit trail. The specialized system handles the hot path: order execution, message delivery, telemetry ingestion, real-time computation.

Runners bridge the two:

A **configuration runner** reads governed state from OpsDB — trading rules, rate limits, feature flags, routing policies — formats it for the specialized system's native configuration, and pushes it. The specialized system reads from this configuration at startup or on a refresh cycle.

An **observation runner** pulls results from the specialized system — executed trades, delivered messages, processed events — and writes them to OpsDB as observation data. The governed state now includes the outcomes of the hot-path processing.

The specialized system never depends on OpsDB at runtime. If OpsDB is unavailable, the specialized system continues operating with its current cached configuration. When OpsDB returns, the runners catch up. This follows two architectural properties: the local replica pattern (cached copies survive partition) and the passive substrate commitment (OpsDB does not invoke work, so its unavailability does not block the systems it governs).

#### 13.3 What this looks like

A payment processing application: OpsDB manages merchant accounts, fee schedules, compliance policies, fraud thresholds, and settlement rules. The payment gateway processes transactions at high speed using cached configuration. An observation runner writes transaction outcomes back to OpsDB for reconciliation, dispute handling, and compliance reporting.

A real-time collaboration tool: OpsDB manages user accounts, document metadata, access permissions, and audit trails. A real-time sync engine (WebSocket-based) handles concurrent editing with CRDTs or operational transformation. An observation runner writes edit summaries back to OpsDB. Document version commits go through the standard change set pipeline.

A machine learning platform: OpsDB manages the model registry, experiment metadata, deployment approvals, A/B test configurations, and monitoring thresholds. Training runs on GPU clusters. Inference serves at millisecond latency. Observation runners write training metrics and inference performance data back to OpsDB.

In every case, the hot path is a small fraction of the total data model. The governed state — accounts, configuration, policies, history, audit — is the majority of the application's entities and the part that benefits from validation, versioning, approval, and audit.

---

### 14. Draft Mode

#### 14.1 When full governance is too heavy

The default governance model creates a version row on every write, routes consequential changes through approval, and logs every action to the audit trail. For configuration data, financial records, and compliance-relevant state, this is appropriate.

For interactive editing — writing a document, drafting a note, iterating on a configuration value — the default creates friction without proportional value. Every keystroke would produce a version row. The audit log would fill with autosave entries. The user experience would feel heavy for fluid creative work.

#### 14.2 The three governance flags

Three per-table governance flags address this:

`_autoversion_disabled` — the API skips creating a version row on each write. The current row is mutable. The user works in a draft state.

`_edit_latest_version` — the API applies writes directly to the current row, skipping change set routing. Authentication, schema validation, bound validation, and access control still run. The change management and versioning steps are skipped.

`_audit_logs_disabled` — the API skips audit log entries for interim saves.

When the user explicitly commits a version — a deliberate action, not an autosave — all ten gate steps run. A version row is created with full state. A change set records the delta from the prior committed version. The audit log records the commit. Committed versions are immutable and fully governed.

#### 14.3 What is still enforced in draft mode

Authentication (step 1) — you must be who you claim to be.

Authorization (step 2) — you must have access to the entity.

Schema validation (step 3) — the data must conform to declared types and constraints.

Bound validation (step 4) — values must be within declared ranges.

Policy evaluation (step 5) — access classification and policy rules still apply.

Draft mode relaxes the recording properties (versioning, change management, audit for interim saves) while preserving the enforcement properties (identity, access, validation). The data in draft-mode tables is still structurally valid. It is still access-controlled. It is simply not individually versioned or audited until explicitly committed.

#### 14.4 Configuring draft mode per table

The flags are declared in the schema. Different tables can have different configurations. A `document` table has draft mode for fluid editing. A `budget` table has full governance for financial accountability. A `recipe` table might use draft mode for the instructions text but full governance for ingredient quantities.

The flags are governance fields — changeable through change sets, versioned, auditable. Enabling draft mode on a table is itself a governed decision.

---

### 15. Compliance as Native Property

#### 15.1 What compliance frameworks ask for

Compliance frameworks — SOC2, ISO 27001, PCI-DSS, HIPAA, GDPR, SOX — ask for evidence that specific controls operate continuously. Access control records. Change management documentation. Audit trails with attribution. Data classification and handling evidence. Retention policy enforcement. Segregation of duties.

In conventional applications, preparing for a compliance audit is a project. Teams assemble evidence from scattered sources, reconstruct access histories from incomplete logs, and demonstrate that controls operated continuously when they were often only checked periodically.

#### 15.2 How OpsDB provides it

Every property that compliance frameworks require is a native consequence of using the gate pipeline:

Access control evidence comes from the authorization layers and the audit log. Every read and write passes through five authorization layers. Every access is logged.

Change management evidence comes from the change set pipeline. Every governed mutation has a proposed-by, approved-by, and applied-at chain.

Audit trail evidence comes from the append-only audit log. Every action is attributed to an authenticated identity with timestamp, target, and outcome.

Data classification evidence comes from the `_access_classification` governance field and the authorization layers that enforce it.

Retention evidence comes from retention policy entities and the reaper runner that enforces them.

Segregation of duties evidence comes from change management rules that prevent self-approval and from the audit log that records who proposed and who approved separately.

Evidence of control effectiveness comes from verifier runners that produce structured evidence records with pass/fail status.

An auditor can query the same system that operators and automation use. "Show me every access to patient records in Q3 with the identity of each accessor" is a search query against the audit log. "Show me every change to security policies this year with the full approval chain" is a search query against change sets joined to approvals. "Show me evidence that backups were verified weekly" is a search query against evidence records.

#### 15.3 What this means for your application

If you build on OpsDB Application Architecture, your compliance posture is a continuous queryable property rather than a periodic scramble. You do not add compliance features later. The gate pipeline produces compliance evidence on every operation. Your auditor queries the system. Your compliance report is a set of search queries.

This does not mean compliance is automatic. You still need to configure the right policies, define the right approval rules, deploy the right verifier runners, and maintain the right retention settings. But the infrastructure for producing and recording compliance evidence is built into the platform. You configure it. You do not build it.

---

### 16. Personal and Small-Scale Use

#### 16.1 Scaling down

The same architecture operates at personal scale. A single OpsDB instance on a Raspberry Pi, a small VPS, or a home server serves as a personal data platform.

You define schemas for whatever you want to track. A recipe collection, a book tracker, a home inventory, a personal finance log, a workout journal, a wine cellar catalog. Each is a handful of YAML files. The loader runs and the API serves them.

At personal scale, the governance features simplify:

Authentication is a single user or a household with two or three users.

Authorization uses a single role. Per-entity governance and per-field classification are unused unless the household has privacy requirements between members.

Change management auto-approves everything. No approval routing is needed when you are the only stakeholder.

The audit log runs in the background. It is queryable if you need it — "what did I change last Tuesday" — but you are not running compliance audits against it.

Compliance features are invisible. No SOC2, no HIPAA, no regulatory requirements.

What remains valuable:

Schema validation ensures your data stays clean. A recipe with a negative prep time is rejected. An inventory item without a location is rejected. The schema enforces the structure you defined.

Versioning means you never permanently lose data. You accidentally overwrote your grandmother's recipe? The version history has every prior state. Submit a change set restoring the previous version.

The search API gives you structured queries across all your data. Find all recipes tagged "italian" with prep time under 30 minutes. Find all books read in 2024 sorted by rating. Find all inventory items in the garage.

You own everything. Your data is in a Postgres database on hardware you control. There is no vendor, no subscription, no terms of service, no API deprecation notice. If your hardware dies, you restore from your backup runner's snapshots and continue.

#### 16.2 Personal runners

Runners provide automation over data you own:

A weather runner pulls forecasts and writes observations. Your garden journal frontend shows weather alongside planting notes.

A fitness API runner pulls workout data from your watch and writes to your exercise tracking entities.

A bank API runner pulls transactions and categorizes them based on rules defined as policy data in OpsDB.

A reminder runner reads schedule entities — vehicle maintenance due dates, subscription renewals, library book due dates — and sends notifications to your phone through a configured push notification channel.

A backup runner snapshots your OpsDB instance to external storage on a schedule.

Each is a small program using the OpsDB API client library and one or two world-side libraries. The same runner pattern, the same library suite, the same three-phase cycle. The only difference is scale.

---

### 17. Construction Guidance

#### 17.1 Designing your schema

Start with the top-level taxonomy. What are the major entity categories in your application? How do they relate? For a project management tool: projects, tasks, users, teams, comments, labels, attachments. For an e-commerce catalog: products, categories, inventory, pricing, orders, customers.

Pick the most important domain and slice it into entities, relationships, and constraints. Do not try to model everything at once. Define the core entities, give them fields with appropriate types and bounds, and run the loader. You now have a working API for your core domain.

Build a runner or two to force the schema to be useful. A puller that imports data from an existing system. A verifier that checks data quality. A notification runner that watches for state transitions. The runner will expose gaps in your schema — missing fields, missing relationships, awkward naming — while the schema is still young and changes are cheap.

Do another domain. The second domain refines the top-level taxonomy. You discover that projects and products both need ownership, so you extract an ownership bridge pattern. You discover that tasks and orders both need status workflows, so you align your status enum values and change management rules.

Keep going. Each domain refines the schema. Each runner confirms the schema is useful. The schema is never "finished." It covers what matters and absorbs new things cleanly.

#### 17.2 Naming conventions

Singular names for all tables and fields. `task` not `tasks`. `user` not `users`.

Lowercase with underscores. `task_assignment` not `TaskAssignment`.

Hierarchical prefixes from specific to general. `web_site` then `web_site_widget`.

Foreign keys named `referenced_entity_id`. `project_id` references `project`.

Datetime fields suffixed `_time`. `created_time`, `approved_time`, `due_time`.

Date fields suffixed `_date`. `birth_date`, `expiration_date`.

Present-state booleans prefixed `is_`. `is_active`, `is_running`, `is_published`.

Past-event booleans prefixed `was_`. `was_approved`, `was_escalated`.

Governance fields prefixed with underscore. `_requires_group`, `_access_classification`.

These conventions are enforced by the loader. Non-conforming names are rejected at schema load time.

#### 17.3 Designing your runners

Identify the inputs: what OpsDB entities does the runner read? What external sources does it consult?

Identify the outputs: what OpsDB entities does it write? What side effects does it produce in the world?

Choose the gating mode: direct write for observations and evidence, auto-approved change set for routine governed changes, approval-required change set for consequential changes.

Choose the trigger: scheduled (cron-like), event-driven (webhook or watch), or continuous (long-running with per-cycle state refresh).

Specify bounds: retry budget, execution time, scope per cycle, memory limit. Declare these in the runner spec's configuration data.

Define idempotency: what does "same end state" mean for this runner? What uniqueness keys prevent re-execution of non-idempotent operations?

Build it small. If the runner exceeds 300 lines, it is either doing more than one thing or reimplementing functionality that belongs in the library suite.

#### 17.4 Building your frontend

Your frontend is a thin consumer of the OpsDB API. It authenticates users via SSO delegation. It translates user actions into API calls. It renders what the API returns.

It does not implement its own validation. The API validates.

It does not implement its own authorization. The API authorizes. The frontend renders what the API permits.

It does not have its own database. OpsDB is the database.

It can do client-side validation for user experience — showing an error message before the user submits a form — but the API is the authoritative check.

It can cache API responses for performance — but the API's read scaling cache, keyed by access scope, handles the common case.

It can use any technology: React, Vue, server-rendered templates, mobile native, CLI. The API is the interface. The frontend is the shell.

---

### 18. What This Is Not

OpsDB Application Architecture is not a web framework. You still need a frontend, a web server, and a presentation layer. OpsDB is the data substrate beneath them.

It is not a replacement for specialized databases. Time-series databases, graph databases, vector databases, and stream processing systems serve specific access patterns that a relational governed substrate does not. OpsDB manages the governed state around these systems and integrates with them through runners.

It is not a workflow engine. The change set lifecycle is one specific workflow (propose → validate → approve → apply). General-purpose workflow orchestration with arbitrary branching is a different system. OpsDB can be the state backend for a workflow engine.

It is not an orchestrator. Runners coordinate through shared state, not through invocation. OpsDB does not schedule runners, trigger them, or manage their processes.

It is not optimal for early-stage prototyping where the data model is still being discovered. The strict evolution rules impose costs when you are iterating rapidly on what entities should exist. Starting in a less rigid system and migrating to OpsDB once the domain stabilizes is a reasonable approach.

It is not a search engine, a document store, a media server, or a real-time communication system. Each of these has specialized requirements that the governed data substrate does not address. OpsDB manages the metadata around them.

---

### 19. Summary

OpsDB Application Architecture gives you a governed data substrate where every entity you define immediately receives: schema validation, bound checking, five-layer authorization, optional change management with configurable approval routing, full version history with point-in-time reconstruction, append-only audit logging with attribution, optimistic concurrency control, a bounded search API, configurable retention policies, and self-describing schema metadata.

Your application has three layers. The frontend handles presentation and user interaction. The OpsDB substrate handles data governance. Runners handle backend logic.

Behavior is configured through data: validation rules, approval policies, access control, notification routing, scheduling, retention, runner configuration. Changing application behavior means changing data rows, not deploying code.

The architecture scales from a personal data platform on a Raspberry Pi to an enterprise backend under regulatory oversight. The schema is the long-lived artifact. The API is the single gate. The runners are the replaceable logic. The governance model adapts through data.

The strict parts — no field deletion, no renames, no type changes, no regex, no embedded logic, append-only audit — exist because partial enforcement is worse than full enforcement. When consumers can trust that the schema never removes what they depend on, they can be simple. When every write is validated and audited, compliance is a property of the system rather than a project bolted on later.

The flexible parts — draft mode governance flags, auto-approval policies, configurable retention, per-table access classification, data-driven behavior — exist because different applications have different requirements. The governance model adapts to your domain through data configuration, not through code modification.

You define your entities. You write your runners. You build your frontend. The substrate handles everything between them.
