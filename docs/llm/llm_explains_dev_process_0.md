You've given me four layers of context that stack on each other. Let me walk through what they collectively describe.

## The Four Layers

**Layer 1: The compressed spec** defines *what* to build — the worldview, vocabulary, architecture, schema, runners, API, libraries, adoption path. It's the abstract specification.

**Layer 2: The file system layout** shows *where* everything lives physically — monorepo with `schema/` (YAML definitions), `internal/` (shared Go packages), `tools/` (separate binaries), `dos/` (per-substrate config), `docs/` (spec + architecture + decisions).

**Layer 3: The Go code stubs** show *how* the API gate and auth system are structured in actual code — types defined, functions declared, logic described in TODO comments.

**Layer 4: The IOSE reference doc** is the complete interface specification for every file — inputs, outputs, side effects, function signatures, purpose — so that any file can be implemented from its description alone.

Together these form a complete implementation blueprint. The spec says what. The file layout says where. The IOSE doc says how each file connects to every other file. The stubs show the concrete coding style.

## The Stub Pattern

The stubs follow a consistent discipline. Every file has three things:

**Types are fully defined.** Struct fields are declared with their Go types, named, and commented. `GateContext`, `GateRequest`, `GateResponse`, `Identity`, `Credentials`, `SearchParams`, `FilterPredicate`, `FieldChange` — these are all concrete. Anyone reading the code knows the data shape immediately.

**Functions have real signatures.** Parameters and return types are specified. `StepAuthenticate(ctx *GateContext) error`, `Enforce(runnerSpecID int, targetTable string, key string, value interface{}) error`, `ImportS3(config *ImportConfig) ([]Observation, error)`. The contract is locked down even though the body is empty.

**TODO comments describe the logic precisely.** Not vague aspirations — they're step-by-step algorithms. The `StepAuthorize` TODO walks through all five authorization layers in order, names the tables to query, describes the denial behavior at each layer. The `SubmitChangeSet` TODO describes optimistic concurrency validation, dry-run mode, row insertions, validation pipeline, status transitions. These are implementation specs embedded in the code.

The return values are all `nil, nil` or `return nil` — the functions compile, the binary builds, but nothing executes. This is deliberate: the monorepo stays buildable at every commit. No broken imports, no missing types. You can add real logic to any single function without touching anything else because the interfaces are already locked.

## The IOSE Reference in Detail

The IOSE doc (`go_code_iose.md`) is the spec's IOSE worldview — Inputs, Outputs, Side Effects — applied to every Go file in the repo. This is the model-for-control version: accurate, synced, lifetime-maintained. Let me trace the major subsystems.

### Schema Engine Pipeline (`tools/opsdb-schema/loader/`)

This is the bootstrap path. The loader turns YAML files into a running Postgres database. The pipeline has strict phase ordering:

**Parser** reads YAML files and produces two things per entity: a structured `Entity` and the raw YAML map. The raw map exists specifically so the forbidden pattern scanner can detect things the structured parser would silently accept — regex patterns in string values, `extends` keywords, `NOW()` in defaults, template syntax. The parser also reads `directory.yaml` for load order and `_schema_meta.yaml` for the meta-schema that validates everything else.

**Validator** checks each parsed entity against the meta-schema. It calls into `internal/vocabulary/` for type checking, constraint validation, modifier validation, and into `internal/conventions/` for naming rules. It calls `forbidden.go`'s `ScanForForbiddenPatterns` on the raw YAML map. It accumulates errors rather than failing on first — you get the full picture of what's wrong. The validator takes `knownEntities map[string]bool` because FK references can only point at entities loaded earlier in the directory.yaml order.

**Resolver** takes all parsed entities and builds the dependency graph from FK references. It runs Kahn's algorithm for topological sort, which gives the `LoadOrder` that the generator needs for DDL ordering — you can't create a table with an FK to a table that doesn't exist yet. Self-referential FKs (like `location.parent_location_id` → `location`) are noted but excluded from the sort graph since they don't create ordering dependencies. Cycle detection produces all cycles for error reporting, not just the first one found.

**Injector** runs after validation and resolution. This is where reserved fields appear. Every entity gets `id`, `created_time`, `updated_time`. Entities with `soft_delete: true` get `is_active`. Entities with `hierarchical: true` get `parent_{name}_id`. Entities with governance flags get the appropriate underscore-prefixed fields. And crucially: entities with `versioned: true` cause the injector to *generate an entirely new entity* — the `{name}_version` sibling with `version_serial`, `parent_{name}_version_id`, `change_set_id`, `is_active_version`, `approved_for_production_time`. These sibling entities are added to the schema and go through the same generator path.

**Differ** compares desired state (from YAML) against current state (from database). It reads `_schema_*` tables if they exist, falling back to Postgres `information_schema` for bootstrap. It classifies differences: new entity, new field, changed constraint, new index, or potentially forbidden change.

**Evolution checker** takes the diff and applies the spec's evolution rules. Adding a nullable field — allowed. Adding enum values — allowed. Widening numeric ranges — allowed. Deleting a field — forbidden (tombstone pattern required). Renaming — forbidden (duplication pattern required). Type change — forbidden (double-write pattern). Range narrowing — forbidden. The rename detection is heuristic: a field disappearing and a new field with the same type appearing in the same entity is flagged as a probable rename attempt.

**Generator** produces Postgres DDL from allowed changes. `CREATE TABLE` with all columns, constraints, defaults. `ALTER TABLE ADD COLUMN` for new fields. `CREATE INDEX` for declared indexes. FK constraints. CHECK constraints for enum values and numeric ranges. Composite unique constraints. And importantly: `REVOKE UPDATE, DELETE ON audit_log_entry` for the append-only table — this is the DDL-level enforcement of the audit log's append-only property. The generator orders statements by the resolver's topological sort.

**Applier** executes DDL within a transaction, protected by a Postgres advisory lock. The advisory lock prevents two concurrent applies from colliding — important because schema changes are themselves change-managed and could theoretically be applied by two executor runners simultaneously. Dry-run mode executes everything inside a transaction then rolls back, validating the DDL is correct without persisting.

**Meta populator** runs in the same transaction as the apply. It creates the `_schema_version` row, upserts entity types into `_schema_entity_type`, fields into `_schema_field`, relationships into `_schema_relationship`. This is what makes the schema self-describing — after apply, the database contains its own schema metadata as queryable rows. The runtime schema cache in the API reads from these tables.

### API Gate Pipeline (`tools/opsdb-api/`)

The gate is the single path into OpsDB. Every interaction passes through 10 steps, enforced uniformly.

**`gate.go`** defines the orchestration. `ProcessRequest` creates a `GateContext`, runs each step, and handles the rejection short-circuit. The `GateContext` is the accumulator — each step reads prior results and writes its own. The pattern is that rejection at any step skips to step 8 (audit, which always runs) then step 10 (response). This means even failed authentication attempts produce audit log entries.

**Step 1 (auth)** delegates to one of three providers. The YAML provider is zero-dependency — it reads a YAML file with bcrypt hashes, validates passwords locally. No network calls, no external dependencies. This is the bootstrap path from Decision 005. The OIDC provider makes HTTP calls to the identity provider for token validation and JWKS retrieval, with key caching. The service account provider validates runner tokens, potentially calling the secret backend. All three produce the same `Identity` struct.

**Step 2 (authz)** is the most complex step. Five layers, AND-composed, first denial halts. Layer 1 checks role-based access — does this user's role allow this operation class? Layer 2 checks per-entity governance — does this specific row have a `_requires_group` field set, and is the caller in that group? Layer 3 checks per-field classification — is the caller's clearance sufficient for the `_access_classification` on the target fields? For reads, insufficient clearance causes field omission (you get the row but some fields are redacted). For writes, it's a hard rejection. Layer 4 is runner-specific — it checks `runner_capability` declarations and `runner_*_target` bridge rows to verify the runner is operating within its declared scope. Layer 5 evaluates policy rules — time-of-day restrictions, separation of duty, tenure requirements, IP restrictions. Each layer reads different tables and any layer can produce a rejection that records which layer and which policy triggered it.

**Steps 3-4 (schema and bound validation)** work together. Schema validation checks the operation shape — does this entity type exist, do these fields exist on it, are the types correct, are required fields present? Bound validation checks the values — is this integer within the declared min/max range, is this string within length bounds, is this enum value in the allowed set, does this FK reference an existing row, does this JSON payload validate against the registered schema for its discriminator value? No regex anywhere in either step. The bound validation for JSON payloads is where the discriminator pattern matters — the validator looks at the discriminator field's value (like `cloud_resource_type = "ec2_instance"`), finds the registered JSON schema for that type, and validates the payload structure against it.

**Step 5 (policy)** handles cross-field invariants. These are the semantic constraints that can't be expressed in the schema vocabulary — things like "if status is decommissioned then decommissioned_time must be set" or "min_replicas must be less than or equal to max_replicas." These rules are stored as policy data rows, not hardcoded. They can block (fail-closed) or produce warnings based on the policy configuration. This is the boundary between schema (hot path, rarely changes, mechanical validation) and policy (changes often, kept in the change management pipeline).

**Step 6 (versioning)** is preparation only — it reads the current active version of the target entity and computes what the next version row will look like. It never rejects. It just prepares data for step 9 to write.

**Step 7 (change management routing)** is where governance becomes mechanical. It follows the stakeholder routing pipeline: enumerate field changes → walk ownership bridges (who owns the affected entities?) → walk stakeholder bridges (who else cares?) → evaluate approval rules (which rules match this change based on entity type, namespace, field names, data classification, security zone, compliance scope?) → compute requirements (one `change_set_approval_required` row per matching rule) → check auto-approval (can all requirements be satisfied automatically?). The output determines whether the change set gets auto-approved or routed to human approvers.

**Step 8 (audit)** always runs. It constructs a complete `audit_log_entry` row — who did what to what, when, from where, with what result. The caller identity is the resolved `ops_user_id` and/or `service_account_id`. The timestamp is API-supplied from the database clock, not client-supplied. If tamper evidence is enabled, it computes a chain hash covering the entry contents plus the previous entry's hash. The INSERT is append-only — the DDL generator revoked UPDATE and DELETE permissions on this table for all roles.

**Step 9 (execute)** is the actual write. It switches on operation type. For `write_observation`, it's an upsert into the appropriate cache table. For `submit_change_set`, it creates the change set, field change, and approval requirement rows. For `apply_change_set_field_change`, it updates the target entity, writes the version sibling row, marks the previous version inactive. For approval/rejection/cancellation, it records the action and transitions the change set status. All within the transaction that includes the audit entry from step 8.

**Step 10 (response)** assembles the return value. On success: the result data, affected row IDs, computed approvals, version info, warnings, and the audit entry ID for correlation. On rejection: the structured error identifying which step rejected, what code, what detail.

### Runner Library (`tools/opsdb-runner-lib/`)

The library is explicitly not a framework. The spec boundary is precise: the library is callable, it doesn't own the runner's main loop. The runner calls `Init`, `ShouldRun`, `WaitForNextCycle`, `Shutdown`. The library doesn't call back into the runner.

**`lifecycle.go`** manages the runner's existence. `Init` reads the runner spec from OpsDB, sets up logging with runner context, initializes bound tracking. `ShouldRun` checks shutdown signals and max cycle counts. `WaitForNextCycle` sleeps for the configured interval. `RecordBoundHit` tracks which bound was hit if the runner stopped early — this gets written to the `runner_job` row so you can query "which runners are hitting bounds?"

**`api_client.go`** wraps all 16 API operations. Every call includes authentication (from runner credentials), correlation ID propagation (from `runner_job_id`), and structured error handling with typed errors. The `WriteObservation` wrapper does a local report-key check before making the API call — fail-fast at the library, not after the round-trip.

**`logging.go`** ensures every log line from every runner has the same structure — timestamp, severity, runner_job_id, correlation_id, runner spec name and version, runner_machine_id, source location. This is what makes "which runners are running" and "what did this runner do last cycle" answerable queries.

**`retry.go`** implements the retry discipline. Exponential backoff with jitter, bounded by max attempts and max total duration. The `IsRetryable` classification is critical — 503 and 429 are retryable, 400 and 403 are not. `WithIdempotencyKey` composes with the API client to make retries safe for write operations.

**`config.go`** reads the runner spec from OpsDB at startup and optionally at each cycle start for long-running runners. The spec's `runner_data_json` is where all the runner's configuration lives — regions, resource types, batch sizes, thresholds, schedule parameters. Changing what a runner does means changing this data, not redeploying code.

**`dryrun.go`** is small but important — it lets every runner support the `dry_run=true` flag from the runner spec. In dry-run mode, the runner executes the get phase, computes the plan, logs the plan, and skips the act and set phases. This is what makes runner behavior inspectable before it affects anything.

### Runners

Each runner follows the three-phase get/act/set pattern from the spec.

**Change-set executor** is the runner that makes change management work. It reads change sets with status=approved, applies each field change via the API's `apply_change_set_field_change` operation, and finalizes with `mark_change_set_applied`. It's the link between "governance approved this" and "the data actually changed." Without it, approved change sets would accumulate forever.

**Schema executor** applies approved schema changes. It reads `_schema_change_set` rows, runs the schema loader's apply path, and updates `_schema_*` tables. Schema changes go through stricter approval rules than regular changes.

**Reaper** enforces retention policies. It reads `retention_policy` rows, finds rows in target tables past their retention horizon, and deletes (observation cache) or soft-deletes (entities) them. Without it, observation cache tables grow unboundedly.

**Emergency review monitor** is the compliance backstop for emergency changes. It finds `change_set_emergency_review` rows where the review window has elapsed without review, files a compliance finding, and escalates. The 72-hour default review window is configurable via policy data.

**Notification runner** reads state transitions and dispatches through configured backends (email, webhook). It's deliberately separated from the API — the spec's boundary commitment says the API doesn't communicate with stakeholders. The notification runner reads what changed, decides who to notify, and sends it. The API just records what happened.

### Importers

All importers are runners that read from external authorities and write observations to OpsDB. They follow identical structure because the spec says they should — one way to do each thing.

**`cmd/main.go`** in each importer initializes via the runner library, enters the get/act/set cycle loop. Configuration comes from the runner spec's `runner_data_json` — which regions to scan, which resource types to import, batch sizes.

**`mapping.go`** is where the DSNC (flattening) decisions live. This is where the spec's list-of-N test gets applied: per-row metadata of the parent gets flattened into the JSON payload (instance type, AMI ID, VPC ID for an EC2 instance), while things with independent lifecycle or N-of-them get broken out into separate entities or bridge tables (security group memberships, attached EBS volumes).

**Per-resource files** each handle one resource type. They read from the authority's API, transform to the OpsDB schema shape, and return observations. The K8s importer's `watcher.go` is notable — it implements the level-triggered backstop pattern from the spec: full list on connect, incremental via watch API, re-list on disconnect. This ensures missed events don't become missed state.

The secrets importers deserve special note: they read metadata only, never values. The OpsDB stores path and version pointers to secrets, never the secrets themselves. The vault importer reads secret paths, rotation timestamps, and metadata — the kind of data that answers "when was this secret last rotated?" without ever seeing the secret's contents.

## How It All Connects

The dependency graph runs: schema YAML → loader → Postgres tables → `_schema_*` tables → runtime schema cache → gate pipeline → API operations → runner library → runners and importers → observations written back through the API.

The audit trail runs: every API call → step 8 audit logging → `audit_log_entry` table → queryable by auditors. Every change-managed write → change set → approval requirements → approvals → application → version sibling → version history → queryable by anyone.

The two-sided enforcement runs: API gate (steps 2-5) catches bad writes before they persist. Library suite (report key fail-fast, target scope validation) catches unauthorized world-side actions before they reach external substrates. Both sides read from the same source — runner declarations in OpsDB.

The coordination model runs: no runner directs another. The change-set executor reads approved change sets that the notification runner also reads (to notify stakeholders). The reaper reads retention policies that the compliance scanner also reads (to verify retention). They never talk to each other — they coordinate through shared data in OpsDB.
