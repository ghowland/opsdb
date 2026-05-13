# OpsDB Runner Library — Technical Specification

## Package: `tools/opsdb-runner-lib/` (package `runnerlib`)

### Overview

Six files forming the runner support library. Not a framework — runners call these functions, they don't call the runner. No dependencies on `internal/` packages; communicates with OpsDB exclusively over HTTP via the API server.

**External dependencies:** `github.com/google/uuid` (used in api_client.go and logging.go for correlation IDs).

---

## File: lifecycle.go

### Purpose
Defines the central `RunnerConfig` type and all lifecycle management functions. Hub file — every other file in the package references `RunnerConfig`.

### Types Defined

**`RunnerConfig`** — All runtime state for a runner instance.
- `SpecName string` — runner spec name from OpsDB
- `SpecVersion int` — active version serial
- `RunnerSpecID int` — runner_spec entity ID
- `RunnerMachineID int` — runner_machine entity ID (from env)
- `APIEndpoint string` — OpsDB API base URL
- `AuthToken string` — bearer token for API auth
- `SiteID int` — site entity ID (from env)
- `DryRun bool` — skip act/set phases
- `MaxCycles int` — 0 = unlimited
- `CycleInterval time.Duration` — sleep between cycles
- `CycleCount int` — cycles completed so far
- `CurrentJobID int` — runner_job ID for current cycle
- `BoundHits []BoundHit` — bounds hit this cycle
- `ReportKeys []ReportKeyDecl` — cached report key declarations
- `SpecDataJSON map[string]interface{}` — parsed runner_data_json
- `Logger *Logger` — structured logger
- `Client *APIClient` — API client instance
- `shutdownCh chan struct{}` — closed on shutdown signal
- `shutdownOnce sync.Once` — ensures single close
- `mu sync.Mutex` — protects mutable fields

**`BoundHit`** — Record of a bound reached during a cycle.
- `BoundName string`
- `BoundValue interface{}`
- `HitTime time.Time`

**`ReportKeyDecl`** — Cached report key for fail-fast validation.
- `Key string`
- `TargetTable string`
- `ConstraintJSON map[string]interface{}`

### Functions

- `Init(runnerSpecName string) (*RunnerConfig, error)` — Reads `OPSDB_API_ENDPOINT` from env, calls `LoadRunnerConfig`, sets up signal handler, logs initialization. Entry point for all runners.
- `ShouldRun(config *RunnerConfig) bool` — Non-blocking check on shutdown channel and max cycles. Called in the runner's main loop condition.
- `WaitForNextCycle(config *RunnerConfig) error` — Sleeps for `CycleInterval` or until shutdown. Returns error on shutdown.
- `StartCycle(config *RunnerConfig) (int, error)` — Increments cycle count, resets bound hits, creates runner_job row via `WriteObservation`, updates logger and client with new job ID. Returns job ID.
- `FinishCycle(config *RunnerConfig, status string, summary map[string]interface{}) error` — Writes cycle finish data to runner_job row. Upgrades status to `completed_with_bounds` if bounds were hit.
- `RecordBoundHit(config *RunnerConfig, boundName string, boundValue interface{})` — Appends to `BoundHits` slice. Logs at warn level.
- `Shutdown(config *RunnerConfig)` — Closes shutdown channel. Safe to call multiple times via `sync.Once`.
- `setupSignalHandler(config *RunnerConfig)` — Goroutine listening for SIGTERM/SIGINT, calls `Shutdown`.

### Cross-References
- Calls `envOrError` (config.go)
- Calls `LoadRunnerConfig` (config.go)
- Calls `NewLogger`, `Field` (logging.go)
- Calls `WriteObservation` via `config.Client` (api_client.go)
- Uses `WriteObservationParams`, `WriteResult` (api_client.go)

### IOSE Deviations
- **`StartCycle` and `FinishCycle` not in IOSE.** The IOSE describes `Init`, `ShouldRun`, `WaitForNextCycle`, `RecordBoundHit`, `Shutdown`. The naive files add `StartCycle` (creates runner_job row, assigns job ID) and `FinishCycle` (writes finish status). These are necessary for the runner_job lifecycle but were not explicitly listed in the IOSE. They should be retained — the IOSE's `Init` description implies job tracking exists but doesn't enumerate these functions.

---

## File: config.go

### Purpose
Runner configuration loading from OpsDB API and environment. Typed accessors for `runner_data_json` values.

### Functions

- `LoadRunnerConfig(specName string, apiEndpoint string) (*RunnerConfig, error)` — Authenticates via `OPSDB_AUTH_TOKEN` env var, creates `APIClient`, looks up runner_spec by name, finds active version, parses `runner_data_json`, loads report keys, reads runner identity from env vars (`OPSDB_RUNNER_MACHINE_ID`, `OPSDB_SITE_ID`). Builds and returns `RunnerConfig`.
- `RefreshConfig(config *RunnerConfig) error` — Re-reads active version. If version changed, updates `SpecVersion`, `SpecDataJSON`, `CycleInterval`, `MaxCycles`, `DryRun`, and report keys. Thread-safe via mutex. Logs the version change.
- `GetSpecData(config *RunnerConfig, key string) (interface{}, bool)` — Raw typed accessor.
- `GetSpecDataString(config, key) (string, bool)`
- `GetSpecDataInt(config, key) (int, bool)` — Handles JSON float64 → int conversion.
- `GetSpecDataBool(config, key) (bool, bool)`
- `GetSpecDataFloat(config, key) (float64, bool)`
- `GetSpecDataStringSlice(config, key) ([]string, bool)` — Handles `[]interface{}` → `[]string`.
- `GetSpecDataDuration(config, key) (time.Duration, bool)` — Reads seconds as int, returns `time.Duration`.
- `GetSpecDataMap(config, key) (map[string]interface{}, bool)`

### Internal Helpers
- `loadReportKeys(client *APIClient, runnerSpecID int) ([]ReportKeyDecl, error)` — Searches `runner_report_key` entity.
- `extractIntField(row map[string]interface{}, field string) (int, error)` — JSON number handling.
- `extractJSONField(row map[string]interface{}, field string) (map[string]interface{}, error)` — Handles both string (parse) and map (passthrough).
- `jsonIntOrDefault(m, key, default) int`
- `jsonBoolOrDefault(m, key, default) bool`
- `envOrError(key string) (string, error)`
- `envIntOrDefault(key string, default int) int`

### Cross-References
- Calls `NewAPIClient`, `GetEntityByName`, `Search` (api_client.go)
- Uses `SearchFilter`, `OrderSpec`, `ReportKeyDecl` (api_client.go / lifecycle.go)
- Calls `NewLogger`, `Field` (logging.go)
- Reads/writes `RunnerConfig` fields (lifecycle.go)

### IOSE Deviations
- **`GetSpecData*` family not in IOSE.** The IOSE describes `LoadRunnerConfig` and `RefreshConfig` but doesn't enumerate the typed accessors. These are convenience functions that provide safe typed access to `runner_data_json`. They should be retained — runners need them to read their spec configuration.
- **`RefreshConfig` takes `*RunnerConfig` not `config *RunnerConfig`.** The IOSE signature matches. No deviation.

---

## File: logging.go

### Purpose
Structured JSON logging with runner context fields on every line.

### Types Defined

**`Logger`** — Structured logger with runner context.
- `SpecName string`
- `SpecVersion int`
- `RunnerMachineID int`
- `JobID int`
- `CorrelationID string`
- `minLevel int` — from `OPSDB_LOG_LEVEL` env var
- `output io.Writer` — defaults to `os.Stdout`

**`LogField`** — Key-value pair for structured fields.
- `Key string`
- `Value interface{}`

### Functions

- `Field(key string, value interface{}) LogField` — Constructor for log fields. Used everywhere.
- `NewLogger(config *RunnerConfig) *Logger` — Creates logger from config. Reads `OPSDB_LOG_LEVEL` env var (debug/info/warn/error). Generates correlation ID via uuid.
- `NewTestLogger(w io.Writer) *Logger` — Test helper writing to provided writer.
- `(l *Logger) WithJobID(jobID int) *Logger` — Returns copy with job ID set. Called from `StartCycle`.
- `(l *Logger) Info(msg string, fields ...LogField)`
- `(l *Logger) Warn(msg string, fields ...LogField)`
- `(l *Logger) Error(msg string, fields ...LogField)`
- `(l *Logger) Debug(msg string, fields ...LogField)`
- `(l *Logger) emit(severity string, msg string, fields []LogField)` — Builds JSON map, marshals, writes line. Fallback to unstructured on marshal failure.

### Log Entry Fields
Every JSON log line includes: `timestamp`, `severity`, `message`, `runner_spec`, and conditionally `runner_spec_version`, `runner_machine_id`, `runner_job_id`, `correlation_id`, plus any caller-provided fields.

### Cross-References
- Takes `*RunnerConfig` (lifecycle.go) in `NewLogger`
- `LogField` and `Field` used by every other file in the package

### IOSE Deviations
- **`NewTestLogger` not in IOSE.** Added for testability. Should be retained.
- **IOSE mentions `WithJobID` returns `*Logger`.** Matches implementation.

---

## File: retry.go

### Purpose
Retry with exponential backoff, jitter, and error classification.

### Types Defined

**`RetryConfig`** — Controls retry behavior.
- `MaxAttempts int` — including first attempt
- `BaseDelay time.Duration`
- `Multiplier float64`
- `JitterFraction float64` — 0.0 to 1.0
- `MaxTotalDuration time.Duration` — hard ceiling

### Functions

- `DefaultRetryConfig() RetryConfig` — 3 attempts, 1s base, 2x multiplier, 25% jitter, 30s max.
- `WithRetry(config RetryConfig, fn func() error) error` — Retry loop with backoff. Stops on success, non-retryable error, max attempts, or max duration.
- `WithIdempotencyKey(key string, fn func(idempotencyKey string) error) error` — Simple pass-through threading the key to the function.
- `IsRetryable(err error) bool` — Classifies errors using `errors.As`:
  - Retryable: `*NetworkError`, `*net.OpError`, `*net.DNSError`, `net.Error` (timeout), `*HTTPError` with 502/503/429
  - Not retryable: `*AuthorizationDeniedError`, `*ValidationFailedError`, `*StaleVersionError`, `*NotFoundError`, `*UndeclaredReportKeyError`, `*nonRetryableError`, unknown errors
- `computeDelay(config RetryConfig, attempt int) time.Duration` — Exponential backoff with symmetric jitter, clamped to non-negative, capped at max total duration.

### Cross-References
- Uses error types from api_client.go: `NetworkError`, `HTTPError`, `AuthorizationDeniedError`, `ValidationFailedError`, `StaleVersionError`, `NotFoundError`, `UndeclaredReportKeyError`, `nonRetryableError`
- `RetryConfig` and `DefaultRetryConfig` consumed by `NewAPIClient` (api_client.go)
- `WithRetry` and `IsRetryable` called by `doRequestWithRetry` (api_client.go)

### IOSE Deviations
- None. Matches IOSE exactly.

---

## File: api_client.go

### Purpose
HTTP client wrapping all 16 OpsDB API operations plus Watch streaming.

### Types Defined

**`APIClient`** — HTTP client with auth, correlation, retry, report key fail-fast.
- `Endpoint string` — base URL, trailing slash stripped
- `AuthToken string` — bearer token
- `CorrelationID string` — propagated via header
- `RunnerJobID int` — propagated via header
- `RetryConfig RetryConfig` — from retry.go
- `ReportKeys []ReportKeyDecl` — from lifecycle.go, for fail-fast
- `httpClient *http.Client` — configured with TLS 1.2+, timeouts

**Request/Response types:**
- `SearchFilter` — `Field`, `Operator`, `Value`
- `OrderSpec` — `Field`, `Direction`
- `SearchResult` — `Rows`, `Cursor`, `TotalCount`
- `DependencyNode` — `EntityType`, `EntityID`, `Depth`, `Metadata`
- `ResolveResult` — authority pointer resolution details
- `WriteObservationParams` — `TargetTable`, `Key`, `Value`, `DataJSON`, `RunnerJobID`, `AuthorityID`, `ObservedTime`
- `WriteResult` — `RowID`
- `SubmitChangeSetParams` — `Name`, `Description`, `Reason`, `FieldChanges`, `TicketRef`, `IsEmergency`, `IsBulk`, `DryRun`
- `FieldChangeParam` — one field change in a submission
- `ChangeSetResult` — `ChangeSetID`, `Status`, `ApprovalRequired`, `ValidationErrors`, `DryRunResult`
- `WatchEvent` — `Type`, `EntityType`, `EntityID`, `Data`, `Version`, `Timestamp`

**Error types:**
- `NotFoundError` — entity not found (404)
- `AuthorizationDeniedError` — permission denied (401/403), includes `Layer` and `Policy`
- `ValidationFailedError` — field validation failed (400), includes `[]FieldError`
- `FieldError` — `Field`, `Code`, `Message`
- `StaleVersionError` — optimistic concurrency conflict (409), includes `[]StaleEntityInfo`
- `StaleEntityInfo` — `EntityType`, `EntityID`, `DraftedVersion`, `CurrentVersion`
- `UndeclaredReportKeyError` — runner wrote undeclared key (400)
- `NetworkError` — transient network failure (retryable), includes `Cause`
- `HTTPError` — generic HTTP error, includes `StatusCode`, `Code`, `Message`, `Detail`
- `nonRetryableError` — internal wrapper to stop retry loop

### Functions — Read Operations
- `NewAPIClient(endpoint, authToken string) *APIClient`
- `(c *APIClient) WithCorrelation(jobID int, correlationID string) *APIClient`
- `(c *APIClient) GetEntity(entityType string, entityID int) (map[string]interface{}, error)`
- `(c *APIClient) GetEntityByName(entityType string, name string) (map[string]interface{}, error)` — convenience wrapper over Search
- `(c *APIClient) GetEntityHistory(entityType string, entityID int) ([]map[string]interface{}, error)`
- `(c *APIClient) GetEntityAtTime(entityType string, entityID int, timestamp time.Time) (map[string]interface{}, error)`
- `(c *APIClient) Search(entityType string, filters []SearchFilter, ordering []OrderSpec, limit int, cursor string) (*SearchResult, error)`
- `(c *APIClient) GetDependencies(entityType string, entityID int, pattern string, maxDepth int) ([]DependencyNode, error)`
- `(c *APIClient) ResolveAuthorityPointer(pointerID int) (*ResolveResult, error)`
- `(c *APIClient) ChangeSetView(changeSetID int) (map[string]interface{}, error)`

### Functions — Write Operations
- `(c *APIClient) WriteObservation(params *WriteObservationParams) (*WriteResult, error)` — local report key fail-fast before API call, idempotency key based on runner_job_id + table + key
- `(c *APIClient) SubmitChangeSet(params *SubmitChangeSetParams) (*ChangeSetResult, error)` — no retry (not idempotent)
- `(c *APIClient) EmergencyApply(params *SubmitChangeSetParams) (*ChangeSetResult, error)` — sets IsEmergency=true, delegates to SubmitChangeSet

### Functions — Change Management Actions
- `(c *APIClient) ApproveChangeSet(changeSetID int, comments string) error`
- `(c *APIClient) RejectChangeSet(changeSetID int, reason string) error`
- `(c *APIClient) CancelChangeSet(changeSetID int) error`
- `(c *APIClient) ApplyFieldChange(changeSetID int, fieldChangeID int) error` — with idempotency key
- `(c *APIClient) MarkChangeSetApplied(changeSetID int) error`

### Functions — Watch
- `(c *APIClient) Watch(entityType string, filters []SearchFilter, resumeToken string, callback func(WatchEvent)) error` — SSE stream reader

### Internal Functions
- `doRequest` — delegates to `doRequestWithRetry`
- `doRequestNoRetry` — delegates to `executeHTTP` directly
- `doRequestWithIdempotency` — delegates to `doRequestWithRetry` with key
- `doRequestWithRetry` — wraps `executeHTTP` with `WithRetry`, wraps non-retryable errors
- `executeHTTP` — single HTTP request: marshal body, create request, set headers, execute, read response, parse errors on 4xx/5xx
- `setHeaders` — Authorization, Content-Type, Accept, X-Correlation-ID, X-Runner-Job-ID, X-Idempotency-Key
- `parseErrorResponse` — maps HTTP status codes to typed errors
- `parseStaleVersionError` — extracts `StaleEntityInfo` from detail map
- `parseValidationError` — extracts `FieldError` from detail map
- `isReportKeyDeclared` — linear scan of cached report keys

### Cross-References
- Uses `RetryConfig`, `DefaultRetryConfig`, `WithRetry`, `IsRetryable` (retry.go)
- Uses `ReportKeyDecl` (lifecycle.go)
- Uses `uuid.New()` for correlation IDs

### IOSE Deviations
- **`GetEntityByName` not in IOSE.** Added as convenience wrapper over Search for looking up specs by name. Used by `LoadRunnerConfig` in config.go. Should be retained.
- **`ChangeSetView` not in IOSE.** The IOSE lists it as an operations read function but doesn't explicitly list it in the API client. It exists in the naive file and is used by runners. Should be retained.
- **`BulkSubmit` from IOSE not implemented.** The IOSE mentions `BulkSubmit` as a separate function. The naive code handles bulk via `SubmitChangeSetParams.IsBulk` flag on `SubmitChangeSet`. Functionally equivalent. Note for future: could add `BulkSubmitChangeSet` as a convenience if callers want a separate entry point.
- **`Watch` callback signature.** IOSE shows `callback func(event WatchEvent)`. Implementation matches.
- **Error type naming.** IOSE describes typed errors as `ValidationFailed`, `AuthorizationDenied`, `StaleVersion`, `NotFound`, `BoundExceeded`, `NetworkError`, `InternalError`. Implementation uses `ValidationFailedError`, `AuthorizationDeniedError`, `StaleVersionError`, `NotFoundError`, `NetworkError`, `HTTPError`. Missing: `BoundExceeded` (bound enforcement is done via `RecordBoundHit` in lifecycle, not as an error type) and `InternalError` (covered by `HTTPError` with 5xx status codes). These are naming differences, not functional gaps.

---

## File: dryrun.go

### Purpose
Dry-run mode support. Check flag, log planned actions, skip act/set phases.

### Functions

- `IsDryRun(config *RunnerConfig) bool` — Thread-safe read of `config.DryRun`.
- `LogPlan(logger *Logger, planDescription string, plan interface{})` — JSON-serializes the plan and logs at Info level with structured fields.
- `SkipActPhase(logger *Logger)` — Logs that act phase is being skipped.
- `SkipSetPhase(logger *Logger)` — Logs that set phase is being skipped.

### Cross-References
- Uses `RunnerConfig` (lifecycle.go)
- Uses `Logger`, `Field` (logging.go)

### IOSE Deviations
- **`SkipActPhase` and `SkipSetPhase` not in IOSE.** The IOSE describes `IsDryRun` and `LogPlan`. The skip functions are convenience wrappers for consistent log output. Should be retained.

---

## Integration Notes for Future Sessions

1. **`go.mod` must include `github.com/google/uuid`** — used by api_client.go and logging.go.
2. **All runners follow the same cycle pattern** using these functions:
   ```
   config := Init(specName)
   for ShouldRun(config) {
       RefreshConfig(config)
       jobID := StartCycle(config)
       // get phase: read from authority/OpsDB
       // act phase: compute changes (skip if IsDryRun)
       // set phase: write to OpsDB (skip if IsDryRun)
       FinishCycle(config, status, summary)
       WaitForNextCycle(config)
   }
   Shutdown(config)
   ```
3. **Report key fail-fast** is two-layer: `APIClient.isReportKeyDeclared` checks locally before making the HTTP call, and the API server's `reportkeys.Enforcer` checks server-side.
4. **Thread safety** — `RunnerConfig.mu` protects `SpecVersion`, `SpecDataJSON`, `CycleInterval`, `MaxCycles`, `DryRun`, `CycleCount`, `CurrentJobID`, `BoundHits`, `ReportKeys`. `Logger` and `Client` are replaced atomically (pointer swap) in `StartCycle`, not mutated.
5. **Importers and runners depend on this package.** All 7 importer packages and all 5 runner packages import `runnerlib` and use the types and functions documented here.
