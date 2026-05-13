# OpsDB Package: Build and Distribution TODO

## From Skeleton to `apt install opsdb`

---

## Current State

The skeleton exists. The file listing shows 230+ files across schema YAML (120+ entities), Go implementation (gate pipeline, schema loader, API operations, runner library, core runners, importers), DOS configuration (two production instances), and supporting infrastructure (conventions, vocabulary, test utilities, scripts).

The skeleton has the correct structure, correct file organization, correct API surface, and correct separation of concerns. The work remaining is making each component fully operational against Postgres, then packaging the result as a distributable system.

This document enumerates every task needed to reach a production-ready `apt install opsdb` package, organized by component in dependency order. Each task states what exists, what's needed, and what "done" means.

---

## Phase 1: Core Data Layer

Everything else depends on this. The database connection, the schema metadata tables, and the ability to read and write rows through Go code.

### 1.1 Postgres Connection Management

**Exists:** `internal/pg/conn.go` (5KB), `internal/pg/tx.go` (4KB), `internal/pg/advisory_lock.go` (2KB).

**Needed:** Verify connection pooling works under concurrent API requests. Verify transaction isolation level is set correctly for change set atomicity. Verify advisory locking prevents concurrent schema applications. Add connection health checking with automatic reconnection. Add configurable pool size from DOS config YAML.

**Done when:** The API server can sustain 100 concurrent requests against Postgres with correct transaction isolation and no connection leaks. Advisory lock prevents two schema applications from running simultaneously.

### 1.2 Schema Metadata Tables

**Exists:** `schema/meta/_schema_meta.yaml` (22KB) defines the meta-schema. `schema/domains/15_schema_meta/` has YAML for `_schema_entity_type`, `_schema_field`, `_schema_relationship`, `_schema_version`, `_schema_change_set`.

**Needed:** Verify the meta-schema tables are created correctly by the loader before any domain tables. These tables must exist first because the API reads them at runtime to validate requests. Verify the loader populates them accurately — every entity type, every field, every constraint, every relationship reflected in metadata.

**Done when:** After `opsdb schema load`, querying `_schema_entity_type` returns every entity in the schema, `_schema_field` returns every field with correct types and constraints, and `_schema_relationship` returns every foreign key relationship. The API reads these tables to validate requests without hardcoded knowledge of the schema.

---

## Phase 2: Schema Engine

The loader must parse YAML, validate it, generate DDL, create tables, and populate metadata. The evolution rules must be enforced.

### 2.1 YAML Parser

**Exists:** `tools/opsdb_schema/loader/parser.go` (17KB).

**Needed:** Verify parsing handles every field type (int, float, varchar, text, boolean, datetime, date, json, enum, foreign_key), every modifier (nullable, default, unique), and every constraint (min_value, max_value, min_length, max_length, enum_values, references, precision_decimal_places, must_be_unique_within). Verify JSON schema parsing for discriminated payloads — the `json_schemas/` directory has 80+ JSON schema files that must parse correctly. Verify `directory.yaml` ordering is respected — entities are loaded in dependency order.

**Done when:** Every YAML file in `schema/domains/` and `schema/json_schemas/` parses without error. Parser produces correct internal representation for every field type, modifier, and constraint.

### 2.2 Validator

**Exists:** `tools/opsdb_schema/loader/validator.go` (10KB).

**Needed:** Verify validation catches every forbidden pattern from the vocabulary: no regex, no embedded logic, no conditional constraints, no inheritance, no templating, no imports within entity files. Verify naming convention enforcement from `internal/conventions/naming.go` — singular names, lowercase underscore, FK naming, datetime suffix, boolean prefix, governance field prefix. Verify constraint consistency — min_value <= max_value, enum_values non-empty, FK references point to entities that exist in the manifest.

**Done when:** Valid YAML passes. Invalid YAML (wrong type, forbidden pattern, naming violation, inconsistent constraint) fails with a structured error identifying the file, field, and violation.

### 2.3 Resolver

**Exists:** `tools/opsdb_schema/loader/resolver.go` (7KB).

**Needed:** Verify foreign key resolution — every `references` field points to an entity type that exists and has been loaded (dependency order from `directory.yaml`). Verify self-referential foreign keys (hierarchical entities). Verify bridge table relationships resolve correctly in both directions.

**Done when:** All 120+ entities resolve their foreign keys without error. A circular dependency or missing reference produces a clear error.

### 2.4 Injector

**Exists:** `tools/opsdb_schema/loader/injector.go` (9KB).

**Needed:** Verify reserved field injection based on `schema/conventions/reserved.yaml`. Every entity gets `id`, `created_time`, `updated_time`. Entities with `soft_delete: true` get `is_active`. Entities with `versioned: true` get a versioning sibling table. Entities with `hierarchical: true` get a self-referential FK. Governance fields (`_requires_group`, `_access_classification`, etc.) are injected when declared.

**Done when:** The internal representation after injection includes all reserved fields for every entity. The fields appear in the generated DDL and in the schema metadata tables.

### 2.5 DDL Generator

**Exists:** `tools/opsdb_schema/loader/generator.go` (12KB).

**Needed:** Verify DDL generation for Postgres. Correct column types (int → INTEGER, float → DOUBLE PRECISION, varchar → VARCHAR(N), text → TEXT, boolean → BOOLEAN, datetime → TIMESTAMPTZ, date → DATE, json → JSONB, enum → VARCHAR with CHECK, foreign_key → INTEGER REFERENCES). Correct constraints (NOT NULL, DEFAULT, UNIQUE, CHECK for min/max, CHECK for enum values, FOREIGN KEY with REFERENCES). Correct index generation. Correct versioning sibling table generation — `*_version` table with all parent fields plus version metadata. Correct audit log table with no UPDATE/DELETE grants.

**Done when:** Running the generated DDL against a clean Postgres database creates all tables with correct columns, types, constraints, indexes, and grants. The audit log table has no UPDATE or DELETE permission for any role.

### 2.6 Schema Differ

**Exists:** `tools/opsdb_schema/loader/differ.go` (20KB).

**Needed:** Verify diff computation between current schema (from `_schema_*` metadata tables) and proposed schema (from YAML files). Detect: new entities, new fields, widened numeric ranges, widened string lengths, new enum values, new indexes. Detect and reject: deleted fields, renamed fields, type changes, narrowed ranges, removed enum values, tightened uniqueness.

**Done when:** Adding a field to a YAML file produces a diff showing the addition. Attempting to delete or rename a field produces an evolution rule violation error with the specific rule cited.

### 2.7 Evolution Enforcer

**Exists:** `tools/opsdb_schema/loader/evolution.go` (11KB).

**Needed:** Verify enforcement of every forbidden change from the schema evolution rules. Deletions rejected. Renames rejected. Type changes rejected. Range narrowing rejected. Enum value removal rejected. Uniqueness tightening rejected. Each rejection produces a structured error identifying the entity, field, and specific rule violated. Allowed changes pass: new fields (nullable), new enum values, widened ranges, widened lengths, new entities, new indexes.

**Done when:** Every forbidden change is rejected with the correct error. Every allowed change passes. The test suite covers every case from Appendix J of the architecture paper (forbidden patterns SF07-SF12).

### 2.8 Schema Applier

**Exists:** `tools/opsdb_schema/loader/applier.go` (3KB).

**Needed:** Verify atomic application of DDL changes within a transaction. Advisory lock acquired before application. DDL executed. Schema metadata tables updated. Advisory lock released. If any step fails, the transaction rolls back and the schema metadata remains unchanged.

**Done when:** Schema changes apply atomically. A failure mid-application leaves the database in the pre-application state. Concurrent schema application attempts are serialized by the advisory lock.

### 2.9 Schema Metadata Population

**Exists:** `tools/opsdb_schema/loader/meta.go` (10KB).

**Needed:** Verify that after schema application, the `_schema_entity_type`, `_schema_field`, `_schema_relationship`, and `_schema_version` tables accurately reflect the current schema. Every entity type, every field with every constraint, every relationship, and a version record linking to the schema change set.

**Done when:** The API can discover the complete schema at runtime by reading metadata tables. A query against `_schema_field` for entity type "resource" returns every field with its type, constraints, and modifiers.

---

## Phase 3: Gate Pipeline

Each step must be fully operational. The pipeline orchestrator must execute steps in sequence, halt on first failure, and produce structured errors.

### 3.1 Pipeline Orchestrator

**Exists:** `tools/opsdb_api/gate/gate.go` (8KB).

**Needed:** Verify the orchestrator calls each step in sequence (1-10), passes context between steps, halts on first failure, and returns a structured error identifying the failing step, the field or policy that caused the failure, and the constraint or rule that was violated. Verify that audit logging (step 8) runs on both success and failure paths.

**Done when:** A valid request traverses all 10 steps and returns a success response. An invalid request halts at the correct step and returns a structured error. A rejected request still produces an audit log entry.

### 3.2 Step 1: Authentication

**Exists:** `tools/opsdb_api/gate/step_auth.go` (1.8KB), `tools/opsdb_api/auth/` (four provider files totaling 35KB).

**Needed:** Verify OIDC provider authenticates human users via SSO token. Verify service account provider authenticates runners via credential. Verify YAML provider authenticates dev users from the DOS auth config (for local development without an IdP). Each provider resolves the caller to an internal user identity. Failed authentication halts the pipeline at step 1.

**Done when:** A request with a valid OIDC token resolves to a user identity. A request with a valid service account credential resolves to a runner identity. A request with no credentials or invalid credentials is rejected at step 1 with a structured error. The dev YAML provider works for local development without external dependencies.

### 3.3 Step 2: Authorization (Five Layers)

**Exists:** `tools/opsdb_api/gate/step_authz.go` (17KB).

**Needed:** Verify all five layers compose via AND, first denial halts.

Layer 1: Query `ops_user_role_member` and `ops_group_member` for the authenticated identity. Check that the caller's roles permit the operation type on the target entity type.

Layer 2: If the target entity has a `_requires_group` field with a non-null value, check that the caller is a member of that group.

Layer 3: For each field in the request (write) or response (read), check the field's `_access_classification` against the caller's clearance level. Omit classified fields from read responses with metadata indicating omission. Reject writes that include classified fields the caller can't access.

Layer 4: If the caller is a service account (runner), check that the operation falls within the runner's declared scope — capability rows and target bridge rows.

Layer 5: Evaluate policy rules — separation of duty, time-of-day restrictions, tenure-based access, custom constraints.

**Done when:** Each layer independently accepts or denies. The composition halts on first denial. The denial response identifies which layer denied and why. Read responses omit classified fields. Runner scope is enforced. Policy rules evaluate correctly.

### 3.4 Step 3: Schema Validation

**Exists:** `tools/opsdb_api/gate/step_schema_validate.go` (7KB).

**Needed:** For write operations, verify every field in the request exists in the schema metadata for the target entity type. Verify the field type matches (integer value for int field, string for varchar, etc.). Verify required fields are present on creates. Reject unknown fields. Reject type mismatches. Produce structured errors identifying the field and the expected type.

**Done when:** A write with all valid fields passes. A write with an unknown field is rejected naming the field. A write with a type mismatch is rejected naming the field and the expected type.

### 3.5 Step 4: Bound Validation

**Exists:** `tools/opsdb_api/gate/step_bound_validate.go` (15KB).

**Needed:** Verify every field value falls within declared constraints. Integer and float within min_value/max_value. String length within min_length/max_length. Enum value within enum_values set. Foreign key target exists in the referenced table. JSON payload matches the discriminated schema for its type value. Produce structured errors identifying the field, the constraint, the limit, and the submitted value.

**Done when:** A value within bounds passes. A value outside bounds is rejected with the constraint name, the limit, and the submitted value. FK to nonexistent entity is rejected. JSON payload with wrong structure for its discriminator is rejected with the specific field in the JSON that failed.

### 3.6 Step 5: Policy Evaluation

**Exists:** `tools/opsdb_api/gate/step_policy.go` (14KB).

**Needed:** Query policy rows matching the operation's entity type, field, namespace, data classification, and security zone. Evaluate cross-field invariants (semantic_invariant policies). Evaluate separation of duty rules. Evaluate time-of-day restrictions. Evaluate custom policy constraints. Produce structured errors identifying the policy rule that triggered and the condition that wasn't met.

**Done when:** A write that satisfies all policies passes. A write that violates a cross-field invariant (e.g., status is "active" but start_date is null) is rejected naming the policy rule. Separation of duty prevents self-approval.

### 3.7 Step 6: Versioning Preparation

**Exists:** `tools/opsdb_api/gate/step_versioning.go` (2.3KB).

**Needed:** For writes to versioned entities, prepare a version row containing the full entity state after the write. All fields, not just the changed ones. Link to the change set that will produce this version. Set version serial to next monotonic value. Set is_current flag. Clear is_current on the previous version row.

**Done when:** After a write to a versioned entity, a version row exists in the `*_version` sibling table with all field values, the correct version serial, and a link to the producing change set. The previous version has is_current set to false.

### 3.8 Step 7: Change Management Routing

**Exists:** `tools/opsdb_api/gate/step_changemgmt.go` (15KB).

**Needed:** For change set submissions, evaluate approval rule policies to determine routing. Walk ownership and stakeholder bridges for touched entities to find approver groups. Create `change_set_approval_required` rows specifying groups and counts needed. If auto-approval policies match, transition the change set to approved without human intervention. If emergency flag is set, apply emergency path logic — reduced approvals, mandatory review record.

**Done when:** A change set touching a low-stakes entity auto-approves per policy. A change set touching a high-stakes entity routes to the correct approver groups with the correct counts. Emergency change sets create emergency review records.

### 3.9 Step 8: Audit Logging

**Exists:** `tools/opsdb_api/gate/step_audit.go` (9KB).

**Needed:** Write an append-only entry to `audit_log_entry` recording: caller identity, operation, target entity type and ID, outcome (success or specific failure), contextual metadata (client IP, user agent, request ID, change set ID), and timestamp. Run on both success and rejection paths. If cryptographic chaining is enabled, hash the entry over its contents plus the prior entry's hash.

**Done when:** Every API operation — successful or rejected — produces an audit log entry. The entry contains all required fields. The audit_log_entry table has no UPDATE or DELETE grants. The cryptographic chain (when enabled) links each entry to the previous.

### 3.10 Step 9: Execution

**Exists:** `tools/opsdb_api/gate/step_execute.go` (21KB).

**Needed:** Execute the actual database operation — INSERT, UPDATE, soft DELETE, or change set creation — atomically with the audit log entry and the version row (if applicable). For change set applies, execute each field change in the declared apply order. For bulk change sets, chunk execution with rollback on failure.

**Done when:** A successful write produces the correct row in the target table, the correct version row (if versioned), and the correct audit entry, all within one transaction. A failed execution rolls back all three.

### 3.11 Step 10: Response Construction

**Exists:** `tools/opsdb_api/gate/step_response.go` (2.7KB).

**Needed:** Construct the response with: affected row IDs, computed approval requirements (if change set), audit entry ID for correlation, version serial (if versioned), and any warnings. Filter response fields by the caller's access classification (fields they can't see are omitted with metadata indicating omission).

**Done when:** Successful responses include all metadata. Rejected responses include the step, field, and constraint that caused rejection. Classified fields are omitted with indication.

---

## Phase 4: API Operations

The sixteen operations exposed through the HTTP API.

### 4.1 Read Operations

**Exists:** `tools/opsdb_api/operations/read.go` (24KB).

**Needed:** Verify `get_entity` — fetch one row by primary key, filtered by authorization. Verify `get_entity_history` — fetch the version chain for one entity. Verify `get_entity_at_time` — reconstruct field values at a timestamp from version rows. Verify `search` — filter predicates, join paths, projection, ordering, cursor pagination, freshness annotations, view modes (standard, with_history, at_time). Verify `get_dependencies` — walk relationship graphs. Verify `resolve_authority_pointer` — look up external fact locations.

**Done when:** Each read operation returns correct results filtered by the caller's authorization. Search handles all predicate types, joins, pagination, and view modes. Unauthorized entities are excluded from results. Unauthorized fields are omitted.

### 4.2 Write Operations

**Exists:** `tools/opsdb_api/operations/write_changeset.go` (20KB), `tools/opsdb_api/operations/write_observation.go` (9KB).

**Needed:** Verify `submit_change_set` — create a change set with field changes, route through approval. Verify `write_observation` — runner writes cached data with direct write gating. Verify `emergency_apply` — reduced approvals with emergency flag and review record. Verify `bulk_submit_change_set` — chunked validation and atomic apply. Verify `apply_change_set_field_change` — executor applies one approved field change.

**Done when:** Change sets create correctly, route through approval correctly, and apply atomically. Observations write with audit but without change sets. Emergency applies create review records. Bulk change sets chunk correctly and roll back on failure.

### 4.3 Change Management Actions

**Exists:** `tools/opsdb_api/operations/changeset_actions.go` (17KB).

**Needed:** Verify `approve_change_set` — an authorized approver approves, approval count tracks toward threshold. Verify `reject_change_set` — an authorized approver rejects with reason. Verify `cancel_change_set` — the proposer withdraws. Verify `mark_change_set_applied` — the executor marks completion after all field changes apply.

**Done when:** The change set lifecycle (draft → submitted → validating → pending_approval → approved → applied, plus terminal states rejected/expired/cancelled/failed) works correctly with all transitions producing audit entries.

### 4.4 Watch Operation

**Exists:** `tools/opsdb_api/operations/watch.go` (12KB).

**Needed:** Verify streaming subscription to entity changes with resume token. On reconnect, fetch current state then stream from the token.

**Done when:** A client can subscribe to changes on an entity type and receive updates as they occur. Reconnection with a resume token catches missed changes.

### 4.5 Optimistic Concurrency

**Exists:** `tools/opsdb_api/concurrency/optimistic.go` (4KB).

**Needed:** Verify version stamp comparison at change set submission. Each field change carries the version of the entity the submitter drafted against. If the entity has advanced since drafting, submission fails with a stale_version error identifying which entities are stale.

**Done when:** Two concurrent edits to the same entity: the first succeeds, the second fails with stale_version. The error identifies the entity and the current version.

### 4.6 Report Key Enforcement

**Exists:** `tools/opsdb_api/reportkeys/enforcer.go` (10KB).

**Needed:** Verify that runners can only write to observation keys declared in their runner_report_key rows. A runner attempting to write an undeclared key is rejected with a structured error.

**Done when:** A runner writing to a declared key succeeds. A runner writing to an undeclared key is rejected at the API with an error and an audit entry recording the violation.

### 4.7 Runtime Schema

**Exists:** `tools/opsdb_api/schema/runtime_schema.go` (12KB).

**Needed:** Verify the API loads schema metadata from the `_schema_*` tables at startup and uses it for all validation. Verify schema changes are picked up without API restart (either polling or notification from the schema executor).

**Done when:** The API validates requests against the runtime schema. A schema change applied by the schema executor is reflected in subsequent API requests without restarting the API.

---

## Phase 5: Core Runners

The runners that the substrate needs to function. Application-specific runners are written by developers; these are infrastructure.

### 5.1 Change Set Executor

**Exists:** `tools/runners/change_set_executor/` (11KB).

**Needed:** Verify the executor reads approved change sets, applies each field change through the API in the declared apply order, and marks the change set as applied. Handle failure: if a field change fails, the change set transitions to failed with the error recorded.

**Done when:** An approved change set is applied within one runner cycle. The entity row is updated. The version row is created. The change set status is "applied." A failed apply produces a "failed" status with the error detail.

### 5.2 Reaper

**Exists:** `tools/runners/reaper/` (26KB).

**Needed:** Verify the reaper reads retention_policy rows, identifies entities and version rows past their retention horizon, and removes or soft-deletes them. Verify the reaper does not touch audit log entries without explicit retention configuration. Verify deletion of audit rows (if permitted) is recorded in a separate audit-of-audit table.

**Done when:** Entities past retention are removed. Version history past retention is trimmed. Audit entries are untouched unless explicitly configured. Each reaper action is logged.

### 5.3 Notification Runner

**Exists:** `tools/runners/notification_runner/` (36KB total with backends).

**Needed:** Verify the runner detects state transitions requiring notification — change sets entering pending_approval, change sets applied, emergency changes, evidence record failures. Verify recipient resolution through on_call_assignment entities. Verify dispatch through configured channels — email backend and webhook backend are implemented. Verify escalation path logic — if primary contact doesn't respond within the configured window, escalate to the next step.

**Done when:** A change set entering pending_approval dispatches a notification to the required approvers through the configured channel. Escalation fires if the approval doesn't arrive within the window.

### 5.4 Schema Executor

**Exists:** `tools/runners/schema_executor/` (34KB).

**Needed:** Verify the executor reads approved schema change sets, generates DDL from the approved changes, applies the DDL atomically with advisory locking, and updates the schema metadata tables. Verify the API picks up the schema change after application.

**Done when:** A schema change set (new entity, new field, widened constraint) is applied by the executor. The database tables reflect the change. The metadata tables reflect the change. The API validates against the new schema.

### 5.5 Emergency Review Monitor

**Exists:** `tools/runners/emergency_review_monitor/` (14KB).

**Needed:** Verify the monitor reads emergency_review records with pending status and checks against the review deadline. Verify overdue reviews trigger escalation through the notification runner. Verify completed reviews update the record status.

**Done when:** Emergency changes past the review window (default 72 hours) trigger escalation. Completed reviews are recorded.

---

## Phase 6: Runner Library

The shared library that application developers use to write custom runners.

### 6.1 API Client

**Exists:** `tools/opsdb_runner_lib/api_client.go` (26KB).

**Needed:** Verify all API operations are callable through the client — search, get_entity, submit_change_set, write_observation, approve, reject, cancel. Verify authentication with service account credentials. Verify correlation ID propagation — every API call carries the runner job ID for audit trail composition. Verify automatic retry on stale version errors (configurable). Verify report key fail-fast — check declared keys before making the API call.

**Done when:** A runner can read entities, submit change sets, write observations, and manage change set lifecycle through the client. Every call carries correlation IDs. Unauthorized writes are rejected and the error is structured.

### 6.2 Configuration

**Exists:** `tools/opsdb_runner_lib/config.go` (11KB).

**Needed:** Verify runner configuration loading from the DOS config and from the runner spec entity in OpsDB. Verify bound loading — retry budget, execution time, scope per cycle, memory limit. Verify target loading — which entity types the runner can read and write.

**Done when:** A runner loads its configuration at startup and enforces its declared bounds during execution.

### 6.3 Lifecycle

**Exists:** `tools/opsdb_runner_lib/lifecycle.go` (7KB).

**Needed:** Verify the three-phase lifecycle — get, act, set — with clean boundaries. Verify the runner writes a job record at cycle start and updates it at cycle end with duration, records processed, and errors. Verify graceful shutdown — the runner completes its current cycle before stopping.

**Done when:** Each runner cycle produces a job record. Shutdown completes the current cycle. The job record contains timing and result information.

### 6.4 Retry and Resilience

**Exists:** `tools/opsdb_runner_lib/retry.go` (5KB).

**Needed:** Verify exponential backoff with jitter. Verify circuit breaking with per-target state. Verify retry budget enforcement — a runner that exhausts its retry budget stops and records which bound was hit.

**Done when:** Transient failures are retried with backoff. Persistent failures trip the circuit breaker. Retry budget exhaustion halts the runner cleanly.

### 6.5 Logging

**Exists:** `tools/opsdb_runner_lib/logging.go` (4KB).

**Needed:** Verify structured log format with runner job ID, correlation ID, runner spec name, version, and source location on every line. Verify consistent format across all runners.

**Done when:** Every runner log line includes the required fields. The format is parseable by standard log aggregation tools.

### 6.6 Dry Run

**Exists:** `tools/opsdb_runner_lib/dryrun.go` (1.6KB).

**Needed:** Verify dry run mode — the runner executes the get and act phases but skips the set phase. Logs what it would write without writing.

**Done when:** A runner in dry-run mode reads data, computes results, logs intended writes, and writes nothing.

---

## Phase 7: CLI Tool

The `opsdb` command that developers interact with.

### 7.1 Project Initialization

**Needed:** `opsdb init <name>` creates the project directory structure from the template. Copies the meta-schema, conventions, vocabulary definitions, and default seed data from the installed package. Creates the DOS dev configuration with local Postgres defaults, single dev user, auto-approve-all policies. Creates the Makefile. Creates an empty schema directory with a starter `directory.yaml`.

**Done when:** `opsdb init myapp && cd myapp && make setup && make serve` produces a working API with no entities. The developer adds a YAML file, updates the manifest, runs `make schema`, and the API serves the new entity.

### 7.2 Schema Commands

**Needed:** `opsdb schema load` — run the full loader pipeline. `opsdb schema diff` — show pending changes without applying. `opsdb schema validate` — validate YAML without touching the database. Each command reads the DOS config for database connection.

**Done when:** Each command works against the project's schema directory and DOS configuration.

### 7.3 Seed Commands

**Needed:** `opsdb seed apply` — load seed data YAML files through the API as change sets. Seed data includes admin user, base policies, core runner specs, runner service accounts, and site identity.

**Done when:** `opsdb seed apply` populates the database with the seed data from the DOS configuration. The admin user can authenticate. The base policies are active. The core runner specs are registered.

### 7.4 API and Runner Commands

**Needed:** `opsdb api serve` — start the API server with the DOS configuration. `opsdb runner start` — start the runner host with core runners. Both run as foreground processes for development, with flags for background/daemon mode for production.

**Done when:** Both commands start their respective services and log to stdout. Ctrl-C shuts down gracefully.

### 7.5 Query and Inspection Commands

**Needed:** `opsdb query <proql>` — execute a ProQL query and display results. `opsdb audit search` — query the audit log with filters. `opsdb version show <entity> <id>` — display version history. `opsdb version at <entity> <id> <timestamp>` — point-in-time reconstruction. `opsdb changeset submit` — submit a change set from CLI (for scripting and testing).

**Done when:** Each command produces useful output against a running OpsDB instance.

---

## Phase 8: Packaging

### 8.1 Build System

**Needed:** Build script that compiles four Go binaries (`opsdb`, `opsdb_api`, `opsdb_schema`, `opsdb-runner`) for amd64 and arm64. Static linking for portability. Version stamping from git tag.

**Done when:** `make build` produces four statically linked binaries in `./dist/` with version information embedded.

### 8.2 Debian Package

**Needed:** Debian package definition that installs:

- Binaries to `/usr/bin/`
- Shared resources (meta-schema, conventions, vocabulary, templates, seed data) to `/usr/lib/opsdb/`
- Documentation to `/usr/share/doc/opsdb/`
- Example schemas to `/usr/share/doc/opsdb/examples/`

Package depends on `postgresql (>= 14)`. No other runtime dependencies.

Post-install script prints: "Run 'opsdb init myapp' to create a new project."

**Done when:** `dpkg -i opsdb_0.1.0_amd64.deb` installs cleanly on Ubuntu 22.04 and 24.04. `opsdb init myapp` works after installation.

### 8.3 APT Repository

**Needed:** A hosted APT repository (could be a simple S3 bucket with the right structure, or a GitHub Pages site, or a dedicated package host) so users can `sudo apt install opsdb` after adding the repository.

**Done when:** Users add the repository, run `apt update`, and `apt install opsdb` installs the package with all dependencies.

### 8.4 Alternative Distribution

**Needed:** For users not on Debian/Ubuntu: a tarball containing the four binaries plus the `lib/` directory. An install script that copies binaries to `/usr/local/bin/` and shared resources to `/usr/local/lib/opsdb/`. A Homebrew formula for macOS. A Docker image with everything pre-installed for container-based deployment.

**Done when:** The system is installable on Ubuntu (apt), macOS (brew), any Linux (tarball), and containers (Docker).

---

## Phase 9: Testing

### 9.1 Schema Loader Tests

**Exists:** `tools/opsdb_schema/loader/loader_test.go` (23KB).

**Needed:** Verify test coverage for every field type, every constraint, every modifier, every forbidden pattern, every evolution rule. Tests run against a real Postgres instance (not mocks).

**Done when:** The test suite covers every case from the vocabulary (types.go, constraints.go, modifiers.go, forbidden.go) and every evolution rule. All tests pass.

### 9.2 Gate Pipeline Tests

**Needed:** Integration tests that send requests through the full pipeline and verify each step's behavior. Test cases for: valid writes accepted, type mismatches rejected at step 3, bound violations rejected at step 4, authorization denials at step 2 (each layer), policy violations at step 5, optimistic concurrency at submission, version row creation at step 6, change management routing at step 7, audit entry on success and failure at step 8.

**Done when:** Each gate step has test cases for its accept and reject paths. The full pipeline has end-to-end tests covering the complete request lifecycle.

### 9.3 Integration Test Script

**Exists:** `scripts/test-integration.sh` (6KB).

**Needed:** Verify the script creates a test database, loads the schema, seeds data, starts the API, runs test scenarios against the API, and tears down. The test scenarios cover: CRUD operations, change set lifecycle, authorization filtering, version history, audit log queries, optimistic concurrency conflicts.

**Done when:** `make test` runs the full integration test suite and reports pass/fail.

### 9.4 Fixture Data

**Exists:** `internal/testutil/fixtures.go` (19KB), `internal/testutil/pg.go` (10KB).

**Needed:** Verify fixtures create realistic test data — entities with relationships, change sets in various lifecycle states, version histories with multiple versions, audit entries, policy configurations. Verify the Postgres test helper creates and tears down test databases cleanly.

**Done when:** Test fixtures provide enough data to exercise every pipeline step and every operation. Test databases are created and destroyed without leaking.

---

## Phase 10: Documentation

### 10.1 README

**Exists:** `README.md` (6KB), `dos/README.md` (7KB).

**Needed:** Update README with: installation instructions (apt install), quickstart (init, schema, serve), project structure explanation, link to full documentation.

**Done when:** A developer reading only the README can install OpsDB, create a project, define an entity, load the schema, start the API, and create their first entity through a curl command.

### 10.2 Example Schemas

**Needed:** A set of example application schemas in `/usr/share/doc/opsdb/examples/`: a project management app (project, task, assignment, comment), a booking system (resource, booking, customer, blackout_date), a personal data platform (recipe, ingredient, book, reading_session). Each example is a complete schema directory with YAML files and a directory.yaml that can be loaded.

**Done when:** A developer can copy an example schema into their project, run `make schema`, and have a working API for that domain.

### 10.3 Man Pages

**Needed:** Man pages for `opsdb`, `opsdb_api`, `opsdb_schema`, `opsdb-runner`. Each documents the command's flags, configuration, and behavior. Installed to `/usr/share/man/` by the package.

**Done when:** `man opsdb` displays usage documentation after package installation.

---

## Dependency Order

The phases have dependencies. Work within each phase can often be parallelized, but phases must complete in order.

Phase 1 (data layer) blocks everything — nothing works without database connectivity and schema metadata tables.

Phase 2 (schema engine) blocks Phase 3 — the gate pipeline reads schema metadata that the loader creates.

Phase 3 (gate pipeline) blocks Phase 4 — API operations use the pipeline.

Phase 4 (API operations) blocks Phase 5 — runners call the API.

Phase 5 (core runners) and Phase 6 (runner library) can proceed in parallel once Phase 4 is functional.

Phase 7 (CLI) can begin once Phase 2 is functional (schema commands) and continues as other phases complete.

Phase 8 (packaging) begins once all binaries compile and the template directory is finalized.

Phase 9 (testing) runs continuously throughout, with coverage expanding as each phase completes.

Phase 10 (documentation) runs continuously, with content finalized after Phase 8.

---

## Effort Estimate

The skeleton has the correct structure and significant implementation. The work is making each component fully operational and verified.

Phase 1: 2-3 days. Connection management and metadata table verification.

Phase 2: 3-5 days. The parser and validator are the largest — 17KB and 10KB respectively. DDL generation and evolution enforcement need thorough testing against Postgres.

Phase 3: 5-7 days. Authorization (17KB) and change management (15KB) are the most complex steps. Each needs integration testing against the database with realistic policy configurations.

Phase 4: 3-5 days. Read operations (24KB) and write operations (20KB + 9KB) need verification against the full pipeline.

Phase 5: 3-4 days. Five runners, each following the same pattern. The notification runner (36KB total) is the most complex.

Phase 6: 2-3 days. The API client (26KB) needs verification of every operation. Lifecycle and retry are smaller.

Phase 7: 2-3 days. The CLI is a thin wrapper around the other components. Init is template copying. Commands delegate to the schema loader, API, and runner binaries.

Phase 8: 2-3 days. Debian packaging, build scripts, repository setup.

Phase 9: Continuous, 3-5 days focused effort for comprehensive coverage.

Phase 10: Continuous, 2-3 days for finalization.

**Total: 25-40 working days. 5-8 weeks at one developer, 2-4 weeks at two developers working in parallel on independent phases.**

This aligns with the original estimate. The skeleton reduces the work from "design and build a system" to "verify and complete an implemented system." The design decisions are made. The file structure is decided. The API surface is decided. The schema is complete. The work is implementation verification — making each function do what its structure says it does, against a real database, with real data, through real tests.
