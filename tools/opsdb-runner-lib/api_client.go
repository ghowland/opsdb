//# tools/opsdb-runner-lib/api_client.go

go
package runnerlib

import (
	"fmt"
	"time"
)

// APIClient wraps HTTP calls to the OpsDB API with authentication,
// correlation ID propagation, retry, and structured error handling.
// Not a framework — runners call these functions directly.
type APIClient struct {
	Endpoint      string
	AuthToken     string
	CorrelationID string
	RunnerJobID   int
	RetryConfig   RetryConfig
	// TODO: http.Client with configured timeouts and TLS
}

// NewAPIClient creates an API client configured for runner use.
func NewAPIClient(endpoint string, authToken string) *APIClient {
	// TODO: create http.Client with:
	//   30s connect timeout, 60s request timeout, TLS config
	// TODO: set default retry config via DefaultRetryConfig()
	// TODO: return client
	return nil
}

// WithCorrelation returns a copy of the client with correlation context set.
// Used at cycle start to propagate runner_job_id through all API calls.
func (c *APIClient) WithCorrelation(jobID int, correlationID string) *APIClient {
	// TODO: shallow copy client
	// TODO: set RunnerJobID and CorrelationID on copy
	// TODO: return copy
	return nil
}

// --- Read Operations ---

// GetEntity fetches one entity row by primary key.
func (c *APIClient) GetEntity(entityType string, entityID int) (map[string]interface{}, error) {
	// TODO: build GET request to /api/v1/{entityType}/{entityID}
	// TODO: set auth header, correlation headers
	// TODO: call doRequest with retry
	// TODO: parse response JSON into map
	// TODO: on 404: return nil, NotFoundError
	// TODO: on auth error: return nil, AuthorizationDeniedError
	// TODO: return parsed data
	return nil, nil
}

// GetEntityByName fetches one entity row by name field (convenience for specs).
func (c *APIClient) GetEntityByName(entityType string, name string) (map[string]interface{}, error) {
	// TODO: call Search with filter name=eq=name, limit 1
	// TODO: if no results: return nil, NotFoundError
	// TODO: return first result
	return nil, nil
}

// GetEntityHistory fetches the version chain for one entity.
func (c *APIClient) GetEntityHistory(entityType string, entityID int) ([]map[string]interface{}, error) {
	// TODO: build GET request to /api/v1/{entityType}/{entityID}/history
	// TODO: set auth and correlation headers
	// TODO: call doRequest with retry
	// TODO: parse response JSON into slice of maps
	// TODO: return ordered version chain
	return nil, nil
}

// GetEntityAtTime reconstructs entity state at a specific timestamp.
func (c *APIClient) GetEntityAtTime(entityType string, entityID int, timestamp time.Time) (map[string]interface{}, error) {
	// TODO: build GET request to /api/v1/{entityType}/{entityID}/at?time={timestamp}
	// TODO: set auth and correlation headers
	// TODO: call doRequest with retry
	// TODO: parse response
	return nil, nil
}

// Search queries entities with filters, ordering, and pagination.
func (c *APIClient) Search(entityType string, filters []SearchFilter, ordering []OrderSpec, limit int, cursor string) (*SearchResult, error) {
	// TODO: build POST request to /api/v1/search
	// TODO: body: {entity_type, filters, ordering, limit, cursor}
	// TODO: set auth and correlation headers
	// TODO: call doRequest with retry
	// TODO: parse response into SearchResult
	return nil, nil
}

// GetDependencies walks substrate hierarchy or service connection graph.
func (c *APIClient) GetDependencies(entityType string, entityID int, pattern string, maxDepth int) ([]DependencyNode, error) {
	// TODO: build GET request to /api/v1/{entityType}/{entityID}/dependencies?pattern={pattern}&max_depth={maxDepth}
	// TODO: set auth and correlation headers
	// TODO: call doRequest with retry
	// TODO: parse response into dependency chain
	return nil, nil
}

// ResolveAuthorityPointer performs a where-is-X lookup.
func (c *APIClient) ResolveAuthorityPointer(pointerID int) (*ResolveResult, error) {
	// TODO: build GET request to /api/v1/authority_pointer/{pointerID}/resolve
	// TODO: set auth and correlation headers
	// TODO: call doRequest with retry
	// TODO: parse response
	return nil, nil
}

// ChangeSetView retrieves change set details filtered to viewer permissions.
func (c *APIClient) ChangeSetView(changeSetID int) (map[string]interface{}, error) {
	// TODO: build GET request to /api/v1/change_set/{changeSetID}/view
	// TODO: set auth and correlation headers
	// TODO: call doRequest with retry
	// TODO: parse response
	return nil, nil
}

// --- Write Operations ---

// WriteObservation writes to observation cache, runner_job_output_var, or evidence_record.
// Performs local report key fail-fast before making the API call.
func (c *APIClient) WriteObservation(params *WriteObservationParams) (*WriteResult, error) {
	// TODO: if RunnerJobID set, validate report key locally against cached declarations:
	//   check params.Key is in declared set for params.TargetTable
	//   if not declared: return UndeclaredReportKeyError without making API call
	// TODO: build POST request to /api/v1/observation
	// TODO: body: {target_table, key, value, data_json, runner_job_id, authority_id, observed_time}
	// TODO: set auth and correlation headers
	// TODO: call doRequest with retry (idempotency key on runner_job_id + key)
	// TODO: parse response
	return nil, nil
}

// SubmitChangeSet proposes a set of field changes through the change management pipeline.
func (c *APIClient) SubmitChangeSet(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	// TODO: build POST request to /api/v1/change_set
	// TODO: body: {name, description, reason, field_changes, is_emergency, is_bulk, dry_run, ticket_ref}
	// TODO: each field_change includes: entity_type, entity_id, field_name, before_value, after_value, change_type, version_stamp
	// TODO: set auth and correlation headers
	// TODO: call doRequest (no retry for change sets — not idempotent without explicit key)
	// TODO: on stale_version error: return StaleVersionError with detail
	// TODO: on validation error: return ValidationFailedError with field details
	// TODO: parse response into ChangeSetResult
	return nil, nil
}

// EmergencyApply submits a change set with emergency=true and reduced approvals.
func (c *APIClient) EmergencyApply(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	// TODO: set params.IsEmergency = true
	// TODO: call SubmitChangeSet
	return nil, nil
}

// --- Change Management Actions ---

// ApproveChangeSet records a stakeholder approval on a change set.
func (c *APIClient) ApproveChangeSet(changeSetID int, comments string) error {
	// TODO: build POST request to /api/v1/change_set/{changeSetID}/approve
	// TODO: body: {comments}
	// TODO: set auth and correlation headers
	// TODO: call doRequest
	return nil
}

// RejectChangeSet records a stakeholder rejection on a change set.
func (c *APIClient) RejectChangeSet(changeSetID int, reason string) error {
	// TODO: build POST request to /api/v1/change_set/{changeSetID}/reject
	// TODO: body: {reason}
	// TODO: set auth and correlation headers
	// TODO: call doRequest
	return nil
}

// CancelChangeSet withdraws a change set.
func (c *APIClient) CancelChangeSet(changeSetID int) error {
	// TODO: build POST request to /api/v1/change_set/{changeSetID}/cancel
	// TODO: set auth and correlation headers
	// TODO: call doRequest
	return nil
}

// ApplyFieldChange applies one approved field change (used by change-set executor).
func (c *APIClient) ApplyFieldChange(changeSetID int, fieldChangeID int) error {
	// TODO: build POST request to /api/v1/change_set/{changeSetID}/apply_field/{fieldChangeID}
	// TODO: set auth and correlation headers
	// TODO: call doRequest with idempotency key on changeSetID+fieldChangeID
	return nil
}

// MarkChangeSetApplied finalizes a change set after all field changes applied.
func (c *APIClient) MarkChangeSetApplied(changeSetID int) error {
	// TODO: build POST request to /api/v1/change_set/{changeSetID}/mark_applied
	// TODO: set auth and correlation headers
	// TODO: call doRequest
	return nil
}

// --- Watch ---

// Watch subscribes to entity changes with level-triggered backstop.
func (c *APIClient) Watch(entityType string, filters []SearchFilter, resumeToken string, callback func(WatchEvent)) error {
	// TODO: build GET request to /api/v1/watch/{entityType}?resume_token={resumeToken}
	// TODO: set auth and correlation headers
	// TODO: if resumeToken provided:
	//   server sends SYNC event with current state then streams changes
	// TODO: if no resumeToken:
	//   server sends SNAPSHOT events for all matching entities then streams
	// TODO: read SSE or WebSocket stream, parse each event, call callback
	// TODO: on disconnect: return error (caller responsible for reconnect with new resume token)
	return nil
}

// --- Internal ---

// doRequest executes an HTTP request with retry, auth headers, and correlation headers.
func (c *APIClient) doRequest(method string, path string, body interface{}) ([]byte, int, error) {
	// TODO: serialize body to JSON if non-nil
	// TODO: create http.Request with method, c.Endpoint+path, body
	// TODO: set headers:
	//   Authorization: Bearer {c.AuthToken}
	//   X-Correlation-ID: {c.CorrelationID}
	//   X-Runner-Job-ID: {c.RunnerJobID} (if set)
	//   Content-Type: application/json
	// TODO: execute with retry via WithRetry if method is GET or has idempotency key
	// TODO: read response body
	// TODO: if status >= 400: parse error response, return typed error
	// TODO: return body bytes and status code
	return nil, 0, nil
}

// --- Request/Response Types ---

// SearchFilter represents one filter predicate for search operations.
type SearchFilter struct {
	Field    string
	Operator string // eq, ne, gt, gte, lt, lte, in, like, is_null, is_not_null, between, json_contains
	Value    interface{}
}

// OrderSpec represents one ordering directive for search operations.
type OrderSpec struct {
	Field     string
	Direction string // asc, desc
}

// SearchResult holds paginated search results.
type SearchResult struct {
	Rows       []map[string]interface{}
	Cursor     string
	TotalCount int
}

// DependencyNode represents one node in a dependency walk result.
type DependencyNode struct {
	EntityType string
	EntityID   int
	Depth      int
	Metadata   map[string]interface{}
}

// ResolveResult holds authority pointer resolution details.
type ResolveResult struct {
	AuthorityID     int
	AuthorityName   string
	AuthorityType   string
	BaseURL         string
	PointerType     string
	Locator         string
	PointerDataJSON map[string]interface{}
}

// WriteObservationParams holds parameters for observation writes.
type WriteObservationParams struct {
	TargetTable  string
	Key          string
	Value        interface{}
	DataJSON     map[string]interface{}
	RunnerJobID  int
	AuthorityID  int
	ObservedTime time.Time
}

// WriteResult holds the result of an observation write.
type WriteResult struct {
	RowID int
}

// SubmitChangeSetParams holds parameters for change set submission.
type SubmitChangeSetParams struct {
	Name         string
	Description  string
	Reason       string
	FieldChanges []FieldChangeParam
	TicketRef    *int
	IsEmergency  bool
	IsBulk       bool
	DryRun       bool
}

// FieldChangeParam represents one field change in a submission.
type FieldChangeParam struct {
	EntityType   string
	EntityID     int
	FieldName    string
	BeforeValue  interface{}
	AfterValue   interface{}
	ChangeType   string // create, update, delete
	VersionStamp int
}

// ChangeSetResult holds the result of a change set operation.
type ChangeSetResult struct {
	ChangeSetID      int
	Status           string
	ApprovalRequired []interface{}
	ValidationErrors []interface{}
	DryRunResult     interface{}
}

// WatchEvent represents one change event in a watch stream.
type WatchEvent struct {
	Type       string // ADDED, MODIFIED, DELETED, SYNC, SNAPSHOT
	EntityType string
	EntityID   int
	Data       map[string]interface{}
	Version    int
	Timestamp  time.Time
}

// --- Error Types ---

// NotFoundError indicates the requested entity does not exist.
type NotFoundError struct {
	EntityType string
	EntityID   int
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s/%d not found", e.EntityType, e.EntityID)
}

// AuthorizationDeniedError indicates the caller lacks permission.
type AuthorizationDeniedError struct {
	Layer   int
	Policy  string
	Message string
}

func (e *AuthorizationDeniedError) Error() string {
	return fmt.Sprintf("authorization denied at layer %d: %s", e.Layer, e.Message)
}

// ValidationFailedError indicates field validation failed.
type ValidationFailedError struct {
	Fields []FieldError
}

type FieldError struct {
	Field   string
	Code    string
	Message string
}

func (e *ValidationFailedError) Error() string {
	return fmt.Sprintf("validation failed: %d field errors", len(e.Fields))
}

// StaleVersionError indicates optimistic concurrency conflict.
type StaleVersionError struct {
	StaleEntities []StaleEntityInfo
}

type StaleEntityInfo struct {
	EntityType     string
	EntityID       int
	DraftedVersion int
	CurrentVersion int
}

func (e *StaleVersionError) Error() string {
	return fmt.Sprintf("stale version: %d entities changed since draft", len(e.StaleEntities))
}

// UndeclaredReportKeyError indicates a runner tried to write an undeclared key.
type UndeclaredReportKeyError struct {
	RunnerSpecID int
	TargetTable  string
	Key          string
}

func (e *UndeclaredReportKeyError) Error() string {
	return fmt.Sprintf("undeclared report key %q for table %s", e.Key, e.TargetTable)
}

// NetworkError indicates a transient network failure (retryable).
type NetworkError struct {
	Message string
	Cause   error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error: %s", e.Message)
}


