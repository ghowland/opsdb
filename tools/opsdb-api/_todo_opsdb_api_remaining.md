# OpsDB API Server — Technical Specification

## Document Purpose

This document describes the current state of the `tools/opsdb-api/` package: what has been written, what each file does, how the files compose, what integration issues are known and deferred, and what remains to be built. A developer (or LLM) reading this document should be able to continue the work without re-reading the nine OPSDB spec papers or the prior conversation.

---

## 1. Architecture Overview

The OpsDB API server is the single gate through which all interactions with OpsDB data pass. It is a Go HTTP server backed by Postgres. Every API request flows through a 10-step pipeline called "the gate" which enforces authentication, authorization, schema validation, bound validation, policy evaluation, versioning, change management routing, audit logging, execution, and response construction — uniformly on every request.

The server is self-contained operational software. It depends on Postgres for storage and on an identity provider for authentication (currently bootstrapped via a YAML file backend). It does not depend on Kubernetes, any specific cloud, or any system the OpsDB models.

Three populations consume the API through scoped access: humans operating the system, automation runners performing operational work, and auditors verifying compliance. All three use the same gate, the same schema, the same audit trail.

### Key architectural commitments

- **The OpsDB is passive.** It answers queries and accepts writes. It does not invoke runners, fire triggers, push notifications, or orchestrate anything. Runners drive all active work.
- **The API is the only path.** No direct database access except for substrate operators under separation-of-duties controls. Every interaction produces an audit trail.
- **The gate is uniform.** Every operation — read, write, approve, reject — flows through the same 10-step sequence. Steps that don't apply to a given operation class pass through without rejecting.
- **Configuration is data.** Approval rules, policies, runner declarations, schema metadata — all are rows in the OpsDB queried at gate time, not hardcoded logic.

---

## 2. Package Structure

```
tools/opsdb-api/
├── cmd/
│   └── main.go                  — CLI entrypoint, server startup, graceful shutdown
├── config/
│   └── config.go                — DOS config.yaml loading and validation
├── auth/
│   ├── provider.go              — Provider interface, Credentials, Identity types
│   └── yaml_provider.go         — YAML file auth backend (bootstrap/dev)
├── gate/
│   ├── gate.go                  — Pipeline orchestrator, shared types, ProcessRequest
│   ├── step_auth.go             — Step 1: Authentication
│   ├── step_authz.go            — Step 2: Authorization (five layers)
│   ├── step_schema_validate.go  — Step 3: Schema validation
│   ├── step_bound_validate.go   — Step 4: Bound validation
│   ├── step_policy.go           — Step 5: Policy evaluation
│   ├── step_versioning.go       — Step 6: Versioning preparation
│   ├── step_changemgmt.go       — Step 7: Change management routing
│   ├── step_audit.go            — Step 8: Audit logging
│   ├── step_execute.go          — Step 9: Execution (database writes)
│   └── step_response.go         — Step 10: Response construction
├── operations/
│   ├── changeset_actions.go     — NOT YET WRITTEN
│   ├── read.go                  — NOT YET WRITTEN
│   ├── resolve.go               — NOT YET WRITTEN
│   ├── watch.go                 — NOT YET WRITTEN
│   ├── write_changeset.go       — NOT YET WRITTEN
│   └── write_observation.go     — NOT YET WRITTEN
├── reportkeys/
│   └── enforcer.go              — Runner report key enforcement
└── schema/
    └── runtime_schema.go        — Runtime schema cache from _schema_* tables
```

### Dependencies on `internal/` packages

```
internal/pg/
├── conn.go           — Connect, Close, Ping, DSNFromEnv
├── tx.go             — WithTransaction, ExecInTx, QueryRowInTx
└── advisory_lock.go  — AcquireAdvisoryLock, WaitForAdvisoryLock, ReleaseAdvisoryLock
```

---

## 3. What Has Been Written (Complete Files)

### 3.1 `cmd/main.go`

**Purpose:** Server entrypoint. Parses `--config` flag, loads config, connects to Postgres, loads runtime schema, initializes YAML auth provider, creates gate pipeline, registers HTTP routes for all 16 API operations plus health/readiness probes, starts HTTP server with TLS support, runs schema refresh loop (30-second polling), handles graceful shutdown on SIGINT/SIGTERM.

**Key decisions:**
- Only YAML auth is implemented. OIDC and service account cases return `"not yet implemented"` errors.
- Routes use `http.ServeMux` from stdlib — no framework.
- Schema refresh runs in a background goroutine that calls `rtSchema.Refresh(db)` every 30 seconds, checking `_schema_version.is_current` for changes.
- Server timeouts: read 30s, header 10s, write 60s, idle 120s.
- The `operations.Handlers` type is referenced but not yet implemented — main.go compiles against its interface but the HTTP handlers don't exist yet.

### 3.2 `config/config.go`

**Purpose:** Reads a DOS `config.yaml` (one per OpsDB substrate — e.g., `dos/prod-0/config.yaml`). Resolves the database DSN from an environment variable (the config file names the env var, not the DSN itself, because the DSN is a secret). Resolves relative file paths against the config file's directory. Validates required fields, auth backend type, TLS both-or-neither, backend-specific fields.

**No internal dependencies.** This is a leaf package.

**Config struct fields:** `SubstrateName`, `SubstrateDesc`, `SiteName`, `DSN`, `ListenAddress`, `TLSCertPath`, `TLSKeyPath`, `AuthBackend` (yaml/oidc/service_account), `AuthConfigPath`, `OIDCIssuer`, `OIDCClientID`, `OIDCAudience`, `SchemaRepoPath`.

### 3.3 `auth/provider.go`

**Purpose:** Defines the shared types for the auth package.

**Types:**
- `Provider` interface: `Authenticate(Credentials) (*Identity, error)`, `RefreshToken(string) (*Identity, error)`, `Type() string`
- `Credentials` struct: `BasicUser`, `BasicPassword`, `BearerToken`, `OIDCToken`. Methods: `HasBasicAuth() bool`, `HasBearerToken() bool`
- `Identity` struct: `OpsUserID *int`, `RunnerMachineID *int`, `RunnerSpecID *int`, `Username string`, `Roles []string`, `Groups []string`, `AuthMethod string`. Methods: `IsHuman() bool`, `IsRunner() bool`, `HasRole(string) bool`, `HasGroup(string) bool`

Pointer fields on Identity are nil when not applicable: a human has no RunnerMachineID, a runner has no OpsUserID (unless acting on behalf of a human in web-mediated writes).

### 3.4 `auth/yaml_provider.go`

**Purpose:** Bootstrap auth backend. Reads `users.yaml` (username, bcrypt hash, ops_user_id, roles, groups). Validates all entries on load — non-empty username, valid bcrypt hash, positive ops_user_id, no duplicates. Thread-safe under RWMutex. Supports `Reload()` without restart. Returns `Identity` with `AuthMethod="yaml"`.

**External deps:** `golang.org/x/crypto/bcrypt`, `gopkg.in/yaml.v3`

### 3.5 `gate/gate.go`

**Purpose:** The pipeline orchestrator. Defines all types that flow through the 10-step pipeline and the `ProcessRequest` method that calls each step in sequence.

**Key types:**
- `GateRequest` — Operation, OperationClass (`read`/`stream`/`write-direct`/`write-cs`/`cm-action`), TargetEntity, TargetEntityID, Params, RawCredentials, ClientIP, UserAgent, RequestID, ReceivedAt
- `GateContext` — Created per request, threaded through every step. Holds DB, Schema, AuthProvider, ReportKeys references plus per-step results (Identity, AuthzResult, SchemaValid, BoundsValid, PolicyResult, VersionInfo, CMRouting, AuditEntryID, ExecutionResult, Rejected, RejectionError, Warnings)
- `GateResponse` — Success, Data, Error, AuditEntryID, Warnings, Metadata
- `GateError` — Step, StepName, Code, Message, Detail (implements `error` interface)
- Result sub-structs: `AuthzResult`, `PolicyResult`, `VersionPrepResult`, `CMRoutingResult`, `ApprovalRequirement`, `ExecutionResult`
- `Identity` type alias: `type Identity = auth.Identity` — allows gate step files to reference auth.Identity without each importing the auth package

**Pipeline flow in `ProcessRequest`:**
1. Each step function is called in order (step 1 through step 10)
2. After each step, `ctx.Rejected` is checked
3. If rejected: jump to step 8 (audit — always runs) then step 10 (response)
4. Step 8 runs unconditionally after steps 1-7 pass
5. Step 9 runs only if not rejected
6. Step 10 always runs

**Helper functions:** `reject(ctx, step, code, message, detail)`, `warn(ctx, message)`, `isWriteOperation(opClass)`, `isChangeManaged(opClass)`, `isReadOnly(opClass)`

### 3.6 `gate/step_auth.go`

**Purpose:** Step 1 — Authentication. Validates credentials via the configured auth provider, resolves to Identity. Rejects if: no provider configured, no credentials provided, authentication fails, identity is nil, identity has no resolved principal (neither OpsUserID nor RunnerMachineID).

**Helper:** `isCredentialsEmpty(creds) bool` — checks `!HasBasicAuth() && !HasBearerToken()`. Will need extension when OIDC/service account providers are added.

### 3.7 `gate/step_authz.go`

**Purpose:** Step 2 — Five-layer authorization. All five layers must pass; first denial halts.

**Layer 1 (Role/Group):** Queries `ops_user_role_member` for the user's roles. Maps role names to permitted operation classes: admin→all, operator→all, reader→read/stream, runner→all (deferred to layer 4), auditor→read, approver→read+cm-action. Runners pass layer 1 unconditionally.

**Layer 2 (Entity Governance):** Reads `_requires_group` from the target entity row. If set, caller must be in that group. Skips for zero TargetEntityID. Handles gracefully when the column doesn't exist on the entity type.

**Layer 3 (Field Classification):** Reads `_access_classification` from `_schema_field` for involved fields. Derives caller clearance from roles (admin→restricted, operator/auditor→confidential, others→internal). For reads: omits classified fields from results. For writes: rejects if clearance insufficient.

**Layer 4 (Runner Authority):** Only for runner callers. Checks `runner_capability` rows (does the runner spec declare a capability for this operation on this entity type?) and `runner_*_target` bridge tables (is the specific entity in the runner's declared scope?). Bridge mappings: service→runner_service_target, host_group→runner_host_group_target, k8s_namespace→runner_k8s_namespace_target, cloud_account→runner_cloud_account_target.

**Layer 5 (Policy Rules):** Loads `access_control` policy rows matching the target entity type. Evaluates separation of duty (submitter cannot approve their own change set — queries `change_set.proposed_by_ops_user_id`), IP restrictions (stubbed — returns true), time-of-day restrictions (stubbed — returns true).

**Shared helpers defined here (used by other step files):** `extractRequestedFields(req) []string`, `deriveCallerClearance(ctx) string`, `clearanceMeetsClassification(clearance, classification) bool`

### 3.8 `gate/step_schema_validate.go`

**Purpose:** Step 3 — Schema validation. Verifies target entity type exists in runtime schema, validates submitted field names exist and are writable.

**Currently implemented:** Entity type existence check, field name validation (unknown fields rejected), reserved field writability check (id/created_time/updated_time never writable; is_active writable for observation and field change apply; governance underscore fields writable through change management), deprecated field rejection.

**Deferred (stub comments in code):**
- `checkTypeCompatibility` — verifies value types match declared field types (int for int fields, string for varchar, etc.). Currently passes all values through; type mismatches caught at database level.
- `checkRequiredFields` — verifies non-nullable, no-default, non-reserved, non-governance fields are present on creates. Currently passes; missing fields caught by database NOT NULL constraints.

**Key functions:** `extractFieldValues(req) map[string]interface{}` — routes by OperationClass: write-direct extracts from params minus routing keys, write-cs returns nil (field changes validated at apply time), cm-action returns nil. `isRoutingParam(key)` — identifies target_table, key, value, runner_job_id, authority_id, data_json as non-field params. `isCreateOperation(req)`, `isReservedFieldWritable(fieldName, operation)`.

### 3.9 `gate/step_bound_validate.go`

**Purpose:** Step 4 — Bound validation against the closed constraint vocabulary from OPSDB-7. No regex, ever.

**Per-type validators:**
- `validateIntBounds` — min_value, max_value
- `validateFloatBounds` — min_value, max_value, precision_decimal_places
- `validateVarcharBounds` — min_length, max_length (character count via `len([]rune(...))`)
- `validateTextBounds` — max_length
- `validateEnumBounds` — value must be in enum_values set
- `validateFKExists` — queries referenced table to verify row exists
- `validateBoolean` — type check only
- `validateJSONPayload` — reads discriminator value, looks up registered JSON schema via `ctx.Schema.GetJSONSchema()`, validates required fields, per-field type/bounds, list count bounds, map entry count bounds. One level deep per OPSDB-7 §9.4.

**Numeric helpers defined here (used by other step files):** `toInt(v interface{}) (int, bool)`, `toFloat(v interface{}) (float64, bool)`, `countDecimalPlaces(f float64) int`. These are the `(value, bool)` variants. step_execute.go uses `toIntErr(v interface{}) (int, error)` for cases where the error message matters.

**Type aliases defined here (temporary, see integration issues §6):** `RuntimeFieldMeta`, `RuntimeJSONSchemaMeta`, `RuntimeJSONFieldSchema` — will be replaced with imports from `schema` package.

### 3.10 `gate/step_policy.go`

**Purpose:** Step 5 — Policy evaluation. Evaluates semantic invariants (cross-field constraints from policy data), entity-linked policies, and classification consistency.

**Semantic invariants:** Loaded from `policy` rows where `policy_type='semantic_invariant'`. Six operators: `lte` (e.g., min_replicas <= max_replicas), `gte`, `eq`, `neq`, `requires_if` (if field A equals X, field B must be set), `requires_unless`. Each has a fail mode: `block` (reject) or `warn` (pass with warning). Default is block (fail closed). Invariants evaluate against merged state: current entity values + proposed changes.

**Entity-linked policies:** Loaded via bridge tables (service_policy, machine_policy, k8s_namespace_policy, cloud_account_policy). Dispatches by policy_type: security_zone (checks restricted_operations list), data_classification (checks caller clearance against minimum_clearance), change_management (skipped — handled by step 7), retention (skipped — enforced by reaper runner).

**Classification consistency:** Reads entity-level `_access_classification`, compares against field-level classifications for proposed writes. Warns when a field's classification is lower than the entity's.

### 3.11 `gate/step_versioning.go`

**Purpose:** Step 6 — Versioning preparation. For write operations against versioned entities, computes NextSerial and ParentVID for the version sibling row that step 9 will insert.

**Behavior:** Skips for reads and non-versioned entities. For creates (TargetEntityID=0): NextSerial=1, ParentVID=0. For updates: queries the version sibling table for the active version row, sets NextSerial=current+1, ParentVID=current version row ID. Handles missing version table (bootstrap), missing version rows (first change-managed write), and transient errors gracefully — warns but never rejects.

**Helper:** `readActiveVersion(db, entityType, entityID) (serial, versionID, error)` — builds table/column names from entity type name following DSNC convention, handles `sql.ErrNoRows` and undefined table errors.

### 3.12 `gate/step_changemgmt.go`

**Purpose:** Step 7 — Change management routing. For change set submissions, determines who must approve and whether auto-approval is possible.

**Five-step sequence within this step:**
1. **Enumerate field changes** — extracts `(entity_type, entity_id, field_name)` tuples from params
2. **Walk ownership bridges** — queries service_ownership, machine_ownership, k8s_cluster_ownership, cloud_resource_ownership to find responsible ops_user_roles
3. **Walk stakeholder bridges** — queries service_stakeholder for notification routing (not approval requirements)
4. **Load and match approval rules** — reads `policy` rows where `policy_type='approval_rule'`, matches by entity type, field name, security zone membership, field classification
5. **Compute requirements** — one `ApprovalRequirement` per matching rule, determine auto-approval (no rules matched, or all rules have `auto_approvable=true`)

**Security zone matching:** Queries zone membership bridge tables (security_zone_membership_service, _machine, _k8s_namespace) joined to security_zone for zone name.

**Classification matching:** Queries `_schema_field` for field classifications matching rule targets.

**Skips** for non-change-managed operations (reads, direct writes, cm-actions).

### 3.13 `gate/step_audit.go`

**Purpose:** Step 8 — Audit logging. Always runs, on both success and rejection paths. Constructs and writes an `audit_log_entry` row.

**What it records:** API endpoint (operation name), action type (derived per-operation), HTTP method (derived from operation class), target entity type and ID, caller identity (ops_user_id and/or runner_machine_id), request summary (param keys, not values — avoids storing sensitive data), response status (success/pending/error-specific status), response summary.

**Tamper-evidence chain hashing:** Optional. Checks if a `compliance_scope` policy has `audit_chain_hash_enabled=true`. If enabled, computes SHA-256 hash over the entry contents plus the previous entry's chain hash, forming an append-only chain where modifying any historical entry breaks all subsequent hashes.

**Timestamps:** Uses `NOW()` in the SQL INSERT so timestamps come from the database clock, not the application clock.

**Failure handling:** Audit insert failure is recorded as a warning on the context — the operation may still succeed, but the audit gap must be investigated.

### 3.14 `gate/step_execute.go`

**Purpose:** Step 9 — Execution. The only step that writes to the database. All writes are atomic within a single Postgres transaction via `pg.WithTransaction`.

**Operation dispatch:**
- `write_observation` → routes by target table to upsert (observation_cache_metric/state/config) or insert (runner_job_output_var, evidence_record)
- `submit_change_set` / `bulk_submit_change_set` → creates change_set row (status from CMRouting or default pending_approval), field_change rows, approval_required rows
- `emergency_apply` → same as submit but is_emergency=true, status=approved, creates emergency_review row
- `apply_change_set_field_change` → reads pending field change, updates entity row, marks applied, optionally inserts version sibling row
- `approve_change_set` → inserts approval, increments fulfilled_count on matching requirements, transitions to approved if all fulfilled
- `reject_change_set` → inserts rejection, transitions to rejected
- `cancel_change_set` → transitions to cancelled from draft/pending_approval
- `mark_change_set_applied` → verifies all field changes applied, transitions to applied

**Key helpers:** `toIntErr(v) (int, error)` — the error-returning variant for execute step. `safeUserID(identity) interface{}` — returns *int or nil for SQL. `filterToColumns(ctx, table, params) map[string]interface{}` — uses runtime schema to keep only valid column names. `isInSlice(needle, haystack)`.

### 3.15 `gate/step_response.go`

**Purpose:** Step 10 — Response construction. Dispatches to `buildRejectionResponse` or `buildSuccessResponse`. Success response includes execution result metadata (affected_row_ids, version_row_ids, change_set_id), CM routing info (auto_approved, approval_requirements), version info (version_serial, parent_version_id), and omitted fields from authorization.

### 3.16 `reportkeys/enforcer.go`

**Purpose:** Runner report key enforcement per OPSDB-6 §8. Validates write_observation calls against `runner_report_key` row declarations. Fail-closed: undeclared keys always rejected.

**Cache:** Per runner-spec, keyed as `runner_spec_id → target_table → []ReportKey`. Loaded on first access, invalidatable per-spec or globally.

**Constraint validation:** Type, enum, numeric range (min/max), string length (max), required fields within JSON structure.

**Error types:** `UndeclaredKeyError`, `InvalidKeyValueError` — both typed with `Is*` check functions.

**Note on integration:** The enforcer is created in main.go and passed to the gate, but no gate step currently calls it. The call should be added to `executeWriteObservation` in step_execute.go — see §6.

### 3.17 `schema/runtime_schema.go`

**Purpose:** In-memory cache of schema metadata loaded from `_schema_entity_type`, `_schema_field`, `_schema_relationship`. Provides fast lookups consumed by every gate step.

**Types exported:**
- `RuntimeSchema` — thread-safe container with RWMutex
- `EntityTypeMeta` — ID, TableName, Description, Category, Versioned, SoftDelete, AppendOnly, Introduced, Deprecated
- `FieldMeta` — the comprehensive field metadata struct with all constraint fields, classification, reserved/governance/deprecated flags
- `RelationshipMeta` — SourceEntity, SourceField, TargetEntity, Cardinality, OnDeleteAction
- `JSONSchemaMeta` — RequiredFields, Fields (map to JSONFieldMeta)
- `JSONFieldMeta` — Type, EnumValues, MinValue/MaxValue (*float64), MinLength/MaxLength/MinCount/MaxCount/MaxEntries (*int)

**Methods:** `LoadRuntimeSchema(db)`, `Refresh(db)`, `EntityCount()`, `GetEntityType(name)`, `GetField(entityType, fieldName)`, `GetAllFields(entityType)`, `GetRelationships(entityType)`, `GetAllEntityTypes()`, `GetJSONSchema(entityType, fieldName, discriminatorValue)`, `IsVersioned(entityType)`, `VersionSerial()`, `LastRefreshed()`

**Refresh behavior:** Creates entirely new RuntimeSchema via LoadRuntimeSchema, then swaps all maps under write lock. Readers holding old pointers see consistent old data for their request's duration.

---

## 4. What Has NOT Been Written

### 4.1 `operations/` package (6 files)

This is the HTTP handler layer that sits between the HTTP server and the gate pipeline. Each handler parses the HTTP request (JSON body, query parameters, auth headers), constructs a `GateRequest`, calls `gate.ProcessRequest()`, and serializes the `GateResponse` as JSON back to the HTTP client.

**Files needed:**

**`operations/handlers.go`** (new — not in skeleton) — The `Handlers` struct and `NewHandlers` constructor that main.go calls. Holds references to db, runtime schema, and gate. Each method on `Handlers` is an `http.HandlerFunc`.

**`operations/read.go`** — Handlers for: `GetEntity`, `GetEntityHistory`, `GetEntityAtTime`, `Search`, `GetDependencies`. Each constructs a `GateRequest` with OperationClass="read", runs it through the gate, then performs the actual database query and returns results. The gate handles auth/authz/audit; the handler handles the query.

Per the IOSE: Search builds queries from filter predicates, named join paths, projection, ordering, and cursor-based pagination. `GetDependencies` walks the substrate hierarchy via recursive `megavisor_instance.parent_megavisor_instance_id` or `service_connection` edges.

**`operations/resolve.go`** — Handler for `ResolveAuthorityPointer`. Looks up `authority_pointer` row, returns authority connection details and locator.

**`operations/write_observation.go`** — Handler for `WriteObservation`. Parses target table, key, value from request body. Constructs GateRequest with OperationClass="write-direct". The actual write is performed by step_execute.go's `executeWriteObservation`.

**`operations/write_changeset.go`** — Handlers for `SubmitChangeSet`, `BulkSubmitChangeSet`, `EmergencyApply`. Parses field_changes array, reason, metadata. Constructs GateRequest with OperationClass="write-cs". Actual write performed by step_execute.go.

**`operations/changeset_actions.go`** — Handlers for `ApproveChangeSet`, `RejectChangeSet`, `CancelChangeSet`, `ApplyFieldChange`, `MarkApplied`. Constructs GateRequest with OperationClass="cm-action". Also handler for `ChangeSetView` (read operation returning scoped or full view of a change set's field changes and approval status).

**`operations/watch.go`** — Handler for `Watch`. Long-poll or WebSocket subscription to entity changes. Level-triggered on reconnect (fetches current state, then streams changes from resume token).

### 4.2 Auth providers (deferred)

- `auth/oidc_provider.go` — OIDC token validation with JWKS caching, IdP discovery, ops_user resolution. A complete skeleton exists in the repo but was excluded from this implementation pass.
- `auth/serviceaccount_provider.go` — Service account token auth for runners. Secret-backend token validation, runner_machine resolution.

### 4.3 `internal/pg` additions needed

Several functions are called by gate step files but may not exist in `internal/pg` yet:

- `func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row` — called by step_audit.go, step_versioning.go, step_authz.go, step_policy.go, step_changemgmt.go, runtime_schema.go
- `func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error)` — called by step_authz.go, step_policy.go, step_changemgmt.go, reportkeys/enforcer.go, runtime_schema.go
- `func QueryRowInTx(tx *Tx, query string, args ...interface{}) *sql.Row` — called by step_execute.go
- `func QuoteIdentifier(name string) string` — wraps name in double quotes with escaping. Called by step_execute.go, step_audit.go, step_versioning.go, step_authz.go, step_policy.go, step_changemgmt.go
- `func IsNoRows(err error) bool` — wraps `errors.Is(err, sql.ErrNoRows)`. Called by step_versioning.go, step_authz.go
- `func IsUndefinedTable(err error) bool` — checks Postgres error code 42P01. Called by step_versioning.go, step_changemgmt.go, step_policy.go, reportkeys/enforcer.go, runtime_schema.go
- `func IsUndefinedColumn(err error) bool` — checks Postgres error code 42703. Called by step_authz.go

These are all small — typically 1-5 lines each. The `internal/pg` package exists with `Connect`, `Close`, `Ping`, `WithTransaction`, `ExecInTx` already present; these additions extend it.

---

## 5. How the Gate Steps Compose

### Data flow through GateContext

```
Request arrives → main.go HTTP handler
  → operations/*.go parses HTTP, constructs GateRequest
    → gate.ProcessRequest(req) creates GateContext
      → step 1: stepAuthenticate     → ctx.Identity
      → step 2: stepAuthorize        → ctx.AuthzResult
      → step 3: stepSchemaValidate   → ctx.SchemaValid
      → step 4: stepBoundValidate    → ctx.BoundsValid
      → step 5: stepPolicyEvaluate   → ctx.PolicyResult
      → step 6: stepVersioningPrepare → ctx.VersionInfo
      → step 7: stepChangeMgmtRoute  → ctx.CMRouting
      → step 8: stepAuditLog         → ctx.AuditEntryID (ALWAYS RUNS)
      → step 9: stepExecute          → ctx.ExecutionResult (only if not rejected)
      → step 10: stepResponse        → GateResponse
    ← operations/*.go serializes GateResponse as JSON
  ← HTTP response to client
```

### Operation classes and which steps are active

| Operation Class | Examples | Steps with logic | Steps that pass through |
|---|---|---|---|
| read | get_entity, search | 1, 2, 3, 8, 10 | 4, 5, 6, 7, 9 (empty result) |
| stream | watch | 1, 2, 8, 10 | 3, 4, 5, 6, 7, 9 |
| write-direct | write_observation | 1, 2, 3, 4, 5, 8, 9, 10 | 6 (if not versioned), 7 |
| write-cs | submit_change_set | 1, 2, 3, 5, 6, 7, 8, 9, 10 | 4 (deferred to apply time) |
| cm-action | approve, reject, apply | 1, 2, 5, 8, 9, 10 | 3, 4, 6, 7 |

### Cross-file function dependencies within gate package

All step files are in package `gate` and can call each other's exported and unexported functions freely. Key shared functions:

| Function | Defined in | Used by |
|---|---|---|
| `reject`, `warn` | gate.go | all step files |
| `isWriteOperation`, `isChangeManaged`, `isReadOnly` | gate.go | most step files |
| `extractRequestedFields` | step_authz.go | step_authz, step_policy, step_changemgmt |
| `deriveCallerClearance` | step_authz.go | step_authz, step_policy |
| `clearanceMeetsClassification` | step_authz.go | step_authz, step_policy |
| `extractFieldValues` | step_schema_validate.go | step_schema_validate, step_bound_validate, step_policy |
| `isCreateOperation`, `isReservedFieldWritable` | step_schema_validate.go | step_schema_validate |
| `toInt`, `toFloat`, `countDecimalPlaces` | step_bound_validate.go | step_bound_validate, step_schema_validate, step_changemgmt |
| `toIntErr` | step_execute.go | step_execute only |
| `safeUserID`, `filterToColumns`, `isInSlice` | step_execute.go | step_execute only |
| `classificationAtLeast` | step_policy.go | step_policy |

---

## 6. Known Integration Issues (Deferred)

### 6.1 Report key enforcement not wired into execute step

The `reportkeys.Enforcer` is created in main.go and stored in `GateContext.ReportKeys`, but `executeWriteObservation` in step_execute.go does not call it. The integration point is:

```go
// In executeWriteObservation, after determining targetTable and key:
if ctx.Identity.IsRunner() && ctx.Identity.RunnerSpecID != nil {
    err := reportkeys.Enforce(ctx.ReportKeys, *ctx.Identity.RunnerSpecID, targetTable, key, value)
    if err != nil {
        return fmt.Errorf("report key rejected: %w", err)
    }
}
```

Note: The enforcer's `Enforce` method is currently defined as a standalone function `Enforce(e *Enforcer, ...)` rather than a method `(e *Enforcer) Enforce(...)`. This should be reconciled — either change to method syntax or update the call site.

### 6.2 Runtime schema type aliases in step_bound_validate.go

`step_bound_validate.go` defines `RuntimeFieldMeta`, `RuntimeJSONSchemaMeta`, and `RuntimeJSONFieldSchema` as local types. These need to be removed and replaced with imports from `schema` package (`schema.FieldMeta`, `schema.JSONSchemaMeta`, `schema.JSONFieldMeta`).

The mismatch: `JSONFieldMeta.MinValue/MaxValue` in the schema package are `*float64`, while the local `RuntimeJSONFieldSchema` used `*interface{}`. The JSON validation functions in step_bound_validate.go need to compare against `*float64` directly instead of calling `toFloat` on dereferenced `*interface{}`.

### 6.3 step_schema_validate.go deferred functions

`checkTypeCompatibility` and `checkRequiredFields` are documented as stubs with comments describing what they'll do. They need to be implemented using `schema.FieldMeta` types from the runtime schema package.

### 6.4 step_schema_validate.go uses field metadata without concrete types

The `validateFieldName` function calls `ctx.Schema.GetField()` and accesses `.IsReserved`, `.IsDeprecated`, `.DeprecatedAlternative` on the result. The runtime schema package exports `*FieldMeta` with these as public fields, so this works. But the file was written before runtime_schema.go was finalized and should be reviewed for any access pattern mismatches.

### 6.5 `pg.UnmarshalJSON` vs `json.Unmarshal`

Several skeleton files reference `pg.UnmarshalJSON(data, &target)`. Some written files use `json.Unmarshal` directly. The files should be consistent. Either add `func UnmarshalJSON(data []byte, v interface{}) error { return json.Unmarshal(data, v) }` to `internal/pg`, or use `json.Unmarshal` everywhere. The written files (step_policy.go, step_changemgmt.go, reportkeys/enforcer.go) already use `json.Unmarshal` directly.

### 6.6 `db.Query` method on `*pg.DB`

Multiple step files call `db.Query()` and `db.QueryRow()` as methods on `*pg.DB`. The `internal/pg` package needs these methods. They delegate to the underlying connection pool:

```go
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
    return db.pool.Query(query, args...)
}

func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
    return db.pool.QueryRow(query, args...)
}
```

### 6.7 `extractFieldValues` defined in two places

`step_schema_validate.go` defines `extractFieldValues(req *GateRequest) map[string]interface{}` which routes by OperationClass. `step_bound_validate.go`'s skeleton also defined a version that reads from `req.Params["fields"]`. The written code uses the step_schema_validate.go version consistently. Verify no other file redefines it.

### 6.8 `extractRequestedFields` vs `extractFieldValues`

These are different functions with different purposes:
- `extractRequestedFields(req) []string` — returns field NAMES only. Defined in step_authz.go. Used for authorization checks and policy evaluation.
- `extractFieldValues(req) map[string]interface{}` — returns field name→value MAP. Defined in step_schema_validate.go. Used for validation and execution.

Both are needed. The naming is intentionally different.

---

## 7. The Operations Package — What Needs to Be Built

### 7.1 Architecture

The operations package is the HTTP translation layer. It does NOT contain business logic — the gate pipeline handles all validation, authorization, and execution. Operations handlers:

1. Parse the HTTP request (JSON body, query params, auth headers)
2. Extract authentication credentials from the `Authorization` header
3. Construct a `gate.GateRequest` with the right Operation, OperationClass, TargetEntity, TargetEntityID, and Params
4. Call `gate.ProcessRequest(req)`
5. For read operations: perform the actual database query (the gate validates but doesn't read data in step 9 — reads return empty ExecutionResult)
6. Serialize the `gate.GateResponse` plus any query results as JSON

### 7.2 `Handlers` struct

```go
type Handlers struct {
    db       *pg.DB
    schema   *schema.RuntimeSchema
    gate     *gate.Gate
}

func NewHandlers(db *pg.DB, schema *schema.RuntimeSchema, gate *gate.Gate) *Handlers
```

Each API operation is a method on `Handlers` with signature `func(w http.ResponseWriter, r *http.Request)`.

### 7.3 Operations by file

**read.go:**
- `GetEntity` — operation="get_entity", class="read". Params: entity_type, entity_id. Query: SELECT * FROM {entity} WHERE id = $1.
- `GetEntityHistory` — operation="get_entity_history", class="read". Params: entity_type, entity_id, optional time_range. Query: version sibling table ordered by version_serial.
- `GetEntityAtTime` — operation="get_entity_at_time", class="read". Params: entity_type, entity_id, timestamp. Query: version sibling for version active at timestamp.
- `Search` — operation="search", class="read". Complex: filter predicates, named join paths, projection, ordering, cursor pagination.
- `GetDependencies` — operation="get_dependencies", class="read". Recursive walk via megavisor_instance.parent_megavisor_instance_id or service_connection edges.

**resolve.go:**
- `ResolveAuthorityPointer` — operation="resolve_authority_pointer", class="read". Params: authority_pointer_id. Returns authority base_url, pointer locator, metadata.

**write_observation.go:**
- `WriteObservation` — operation="write_observation", class="write-direct". Params: target_table, key, value, plus table-specific fields.

**write_changeset.go:**
- `SubmitChangeSet` — operation="submit_change_set", class="write-cs". Params: name, reason, field_changes array.
- `BulkSubmitChangeSet` — operation="bulk_submit_change_set", class="write-cs". Same shape, is_bulk=true.
- `EmergencyApply` — operation="emergency_apply", class="write-cs". Same shape, is_emergency=true.

**changeset_actions.go:**
- `ApproveChangeSet` — operation="approve_change_set", class="cm-action". Params: change_set_id, comment.
- `RejectChangeSet` — operation="reject_change_set", class="cm-action". Params
