package runnerlib

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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
	ReportKeys    []ReportKeyDecl
	httpClient    *http.Client
}

// NewAPIClient creates an API client configured for runner use.
func NewAPIClient(endpoint string, authToken string) *APIClient {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		MaxIdleConns:         10,
		MaxIdleConnsPerHost:  10,
		IdleConnTimeout:      90 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	}

	return &APIClient{
		Endpoint:    strings.TrimRight(endpoint, "/"),
		AuthToken:   authToken,
		RetryConfig: DefaultRetryConfig(),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
	}
}

// WithCorrelation returns a copy of the client with correlation context set.
// Used at cycle start to propagate runner_job_id through all API calls.
func (c *APIClient) WithCorrelation(jobID int, correlationID string) *APIClient {
	if correlationID == "" {
		correlationID = uuid.New().String()
	}
	return &APIClient{
		Endpoint:      c.Endpoint,
		AuthToken:     c.AuthToken,
		CorrelationID: correlationID,
		RunnerJobID:   jobID,
		RetryConfig:   c.RetryConfig,
		ReportKeys:    c.ReportKeys,
		httpClient:    c.httpClient,
	}
}

// --- Read Operations ---

// GetEntity fetches one entity row by primary key.
func (c *APIClient) GetEntity(entityType string, entityID int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/%s/%d", entityType, entityID)
	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNotFound {
		return nil, &NotFoundError{EntityType: entityType, EntityID: entityID}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response for %s/%d: %w", entityType, entityID, err)
	}
	return result, nil
}

// GetEntityByName fetches one entity row by name field (convenience for specs).
func (c *APIClient) GetEntityByName(entityType string, name string) (map[string]interface{}, error) {
	result, err := c.Search(entityType, []SearchFilter{
		{Field: "name", Operator: "eq", Value: name},
	}, nil, 1, "")
	if err != nil {
		return nil, err
	}
	if len(result.Rows) == 0 {
		return nil, &NotFoundError{EntityType: entityType, EntityID: 0}
	}
	return result.Rows[0], nil
}

// GetEntityHistory fetches the version chain for one entity.
func (c *APIClient) GetEntityHistory(entityType string, entityID int) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/%s/%d/history", entityType, entityID)
	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNotFound {
		return nil, &NotFoundError{EntityType: entityType, EntityID: entityID}
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing history for %s/%d: %w", entityType, entityID, err)
	}
	return result, nil
}

// GetEntityAtTime reconstructs entity state at a specific timestamp.
func (c *APIClient) GetEntityAtTime(entityType string, entityID int, timestamp time.Time) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/%s/%d/at?time=%s", entityType, entityID, timestamp.Format(time.RFC3339Nano))
	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNotFound {
		return nil, &NotFoundError{EntityType: entityType, EntityID: entityID}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing at-time for %s/%d: %w", entityType, entityID, err)
	}
	return result, nil
}

// Search queries entities with filters, ordering, and pagination.
func (c *APIClient) Search(entityType string, filters []SearchFilter, ordering []OrderSpec, limit int, cursor string) (*SearchResult, error) {
	reqBody := map[string]interface{}{
		"entity_type": entityType,
		"filters":     filters,
		"limit":       limit,
	}
	if ordering != nil {
		reqBody["ordering"] = ordering
	}
	if cursor != "" {
		reqBody["cursor"] = cursor
	}

	body, _, err := c.doRequest("POST", "/api/v1/search", reqBody)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Rows       []map[string]interface{} `json:"rows"`
		Cursor     string                   `json:"cursor"`
		TotalCount int                      `json:"total_count"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}

	return &SearchResult{
		Rows:       raw.Rows,
		Cursor:     raw.Cursor,
		TotalCount: raw.TotalCount,
	}, nil
}

// GetDependencies walks substrate hierarchy or service connection graph.
func (c *APIClient) GetDependencies(entityType string, entityID int, pattern string, maxDepth int) ([]DependencyNode, error) {
	path := fmt.Sprintf("/api/v1/%s/%d/dependencies?pattern=%s&max_depth=%d", entityType, entityID, pattern, maxDepth)
	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNotFound {
		return nil, &NotFoundError{EntityType: entityType, EntityID: entityID}
	}

	var result []DependencyNode
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing dependencies: %w", err)
	}
	return result, nil
}

// ResolveAuthorityPointer performs a where-is-X lookup.
func (c *APIClient) ResolveAuthorityPointer(pointerID int) (*ResolveResult, error) {
	path := fmt.Sprintf("/api/v1/authority_pointer/%d/resolve", pointerID)
	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNotFound {
		return nil, &NotFoundError{EntityType: "authority_pointer", EntityID: pointerID}
	}

	var result ResolveResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing authority pointer resolution: %w", err)
	}
	return &result, nil
}

// ChangeSetView retrieves change set details filtered to viewer permissions.
func (c *APIClient) ChangeSetView(changeSetID int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/api/v1/change_set/%d/view", changeSetID)
	body, statusCode, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	if statusCode == http.StatusNotFound {
		return nil, &NotFoundError{EntityType: "change_set", EntityID: changeSetID}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing change set view: %w", err)
	}
	return result, nil
}

// --- Write Operations ---

// WriteObservation writes to observation cache, runner_job_output_var, or evidence_record.
// Performs local report key fail-fast before making the API call.
func (c *APIClient) WriteObservation(params *WriteObservationParams) (*WriteResult, error) {
	// Local report key fail-fast: check before round-trip.
	if c.RunnerJobID > 0 && len(c.ReportKeys) > 0 {
		if !c.isReportKeyDeclared(params.TargetTable, params.Key) {
			return nil, &UndeclaredReportKeyError{
				RunnerSpecID: 0,
				TargetTable:  params.TargetTable,
				Key:          params.Key,
			}
		}
	}

	reqBody := map[string]interface{}{
		"target_table":  params.TargetTable,
		"key":           params.Key,
		"value":         params.Value,
		"data_json":     params.DataJSON,
		"runner_job_id": params.RunnerJobID,
		"authority_id":  params.AuthorityID,
		"observed_time": params.ObservedTime.Format(time.RFC3339Nano),
	}

	// Idempotency key based on runner_job_id + key for safe retry.
	idempotencyKey := fmt.Sprintf("%d:%s:%s", params.RunnerJobID, params.TargetTable, params.Key)

	body, _, err := c.doRequestWithIdempotency("POST", "/api/v1/observation", reqBody, idempotencyKey)
	if err != nil {
		return nil, err
	}

	var raw struct {
		RowID int `json:"row_id"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing write observation result: %w", err)
	}
	return &WriteResult{RowID: raw.RowID}, nil
}

// SubmitChangeSet proposes a set of field changes through the change management pipeline.
func (c *APIClient) SubmitChangeSet(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	reqBody := map[string]interface{}{
		"name":          params.Name,
		"description":   params.Description,
		"reason":        params.Reason,
		"field_changes": params.FieldChanges,
		"is_emergency":  params.IsEmergency,
		"is_bulk":       params.IsBulk,
		"dry_run":       params.DryRun,
	}
	if params.TicketRef != nil {
		reqBody["ticket_ref"] = *params.TicketRef
	}

	// No retry for change sets — not idempotent without explicit key.
	body, _, err := c.doRequestNoRetry("POST", "/api/v1/change_set", reqBody)
	if err != nil {
		return nil, err
	}

	var result ChangeSetResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing change set result: %w", err)
	}
	return &result, nil
}

// EmergencyApply submits a change set with emergency=true and reduced approvals.
func (c *APIClient) EmergencyApply(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	params.IsEmergency = true
	return c.SubmitChangeSet(params)
}

// --- Change Management Actions ---

// ApproveChangeSet records a stakeholder approval on a change set.
func (c *APIClient) ApproveChangeSet(changeSetID int, comments string) error {
	path := fmt.Sprintf("/api/v1/change_set/%d/approve", changeSetID)
	reqBody := map[string]interface{}{
		"comments": comments,
	}
	_, _, err := c.doRequest("POST", path, reqBody)
	return err
}

// RejectChangeSet records a stakeholder rejection on a change set.
func (c *APIClient) RejectChangeSet(changeSetID int, reason string) error {
	path := fmt.Sprintf("/api/v1/change_set/%d/reject", changeSetID)
	reqBody := map[string]interface{}{
		"reason": reason,
	}
	_, _, err := c.doRequest("POST", path, reqBody)
	return err
}

// CancelChangeSet withdraws a change set.
func (c *APIClient) CancelChangeSet(changeSetID int) error {
	path := fmt.Sprintf("/api/v1/change_set/%d/cancel", changeSetID)
	_, _, err := c.doRequest("POST", path, nil)
	return err
}

// ApplyFieldChange applies one approved field change (used by change-set executor).
func (c *APIClient) ApplyFieldChange(changeSetID int, fieldChangeID int) error {
	path := fmt.Sprintf("/api/v1/change_set/%d/apply_field/%d", changeSetID, fieldChangeID)
	idempotencyKey := fmt.Sprintf("apply:%d:%d", changeSetID, fieldChangeID)
	_, _, err := c.doRequestWithIdempotency("POST", path, nil, idempotencyKey)
	return err
}

// MarkChangeSetApplied finalizes a change set after all field changes applied.
func (c *APIClient) MarkChangeSetApplied(changeSetID int) error {
	path := fmt.Sprintf("/api/v1/change_set/%d/mark_applied", changeSetID)
	_, _, err := c.doRequest("POST", path, nil)
	return err
}

// --- Watch ---

// Watch subscribes to entity changes with level-triggered backstop.
func (c *APIClient) Watch(entityType string, filters []SearchFilter, resumeToken string, callback func(WatchEvent)) error {
	path := fmt.Sprintf("/api/v1/watch/%s", entityType)
	if resumeToken != "" {
		path += "?resume_token=" + resumeToken
	}

	req, err := http.NewRequest("GET", c.Endpoint+path, nil)
	if err != nil {
		return fmt.Errorf("creating watch request: %w", err)
	}
	c.setHeaders(req, "")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &NetworkError{Message: "watch connection failed", Cause: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return c.parseErrorResponse(resp.StatusCode, respBody)
	}

	// Read SSE stream line by line.
	buf := make([]byte, 0, 4096)
	reader := resp.Body
	tmp := make([]byte, 1024)

	for {
		n, err := reader.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)

			// Process complete events (double newline delimited in SSE).
			for {
				idx := bytes.Index(buf, []byte("\n\n"))
				if idx == -1 {
					break
				}

				eventData := buf[:idx]
				buf = buf[idx+2:]

				var event WatchEvent
				// SSE format: "data: {json}\n\n"
				line := string(eventData)
				if strings.HasPrefix(line, "data: ") {
					jsonData := line[6:]
					if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
						continue // skip malformed events
					}
					callback(event)
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("watch stream ended by server")
			}
			return &NetworkError{Message: "watch stream read error", Cause: err}
		}
	}
}

// --- Internal ---

// doRequest executes an HTTP request with retry for GET requests and
// non-mutating POST requests.
func (c *APIClient) doRequest(method string, path string, body interface{}) ([]byte, int, error) {
	if method == "GET" {
		return c.doRequestWithRetry(method, path, body, "")
	}
	// POST requests that are not explicitly idempotent: retry only on network errors.
	return c.doRequestWithRetry(method, path, body, "")
}

// doRequestNoRetry executes an HTTP request without any retry.
// Used for non-idempotent mutations like change set submission.
func (c *APIClient) doRequestNoRetry(method string, path string, body interface{}) ([]byte, int, error) {
	return c.executeHTTP(method, path, body, "")
}

// doRequestWithIdempotency executes an HTTP request with retry enabled
// and an idempotency key header for safe retries of write operations.
func (c *APIClient) doRequestWithIdempotency(method string, path string, body interface{}, idempotencyKey string) ([]byte, int, error) {
	return c.doRequestWithRetry(method, path, body, idempotencyKey)
}

// doRequestWithRetry wraps executeHTTP with the retry logic.
func (c *APIClient) doRequestWithRetry(method string, path string, body interface{}, idempotencyKey string) ([]byte, int, error) {
	var lastBody []byte
	var lastStatus int
	var lastErr error

	retryErr := WithRetry(c.RetryConfig, func() error {
		respBody, status, err := c.executeHTTP(method, path, body, idempotencyKey)
		lastBody = respBody
		lastStatus = status
		lastErr = err

		if err != nil {
			if IsRetryable(err) {
				return err
			}
			return &nonRetryableError{err}
		}
		return nil
	})

	if retryErr != nil {
		// Unwrap non-retryable wrapper to return original error.
		if nre, ok := retryErr.(*nonRetryableError); ok {
			return lastBody, lastStatus, nre.err
		}
		return lastBody, lastStatus, lastErr
	}

	return lastBody, lastStatus, nil
}

// executeHTTP performs a single HTTP request and parses the response.
func (c *APIClient) executeHTTP(method string, path string, body interface{}, idempotencyKey string) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequest(method, c.Endpoint+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	c.setHeaders(req, idempotencyKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Classify network errors as retryable.
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, 0, &NetworkError{Message: "request timeout", Cause: err}
		}
		return nil, 0, &NetworkError{Message: "request failed", Cause: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, &NetworkError{Message: "reading response body", Cause: err}
	}

	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, c.parseErrorResponse(resp.StatusCode, respBody)
	}

	return respBody, resp.StatusCode, nil
}

// setHeaders applies standard headers to an outgoing request.
func (c *APIClient) setHeaders(req *http.Request, idempotencyKey string) {
	req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.CorrelationID != "" {
		req.Header.Set("X-Correlation-ID", c.CorrelationID)
	}
	if c.RunnerJobID > 0 {
		req.Header.Set("X-Runner-Job-ID", strconv.Itoa(c.RunnerJobID))
	}
	if idempotencyKey != "" {
		req.Header.Set("X-Idempotency-Key", idempotencyKey)
	}
}

// parseErrorResponse converts an HTTP error response into a typed error.
func (c *APIClient) parseErrorResponse(statusCode int, body []byte) error {
	var errResp struct {
		Step     int                    `json:"step"`
		StepName string                `json:"step_name"`
		Code     string                 `json:"code"`
		Message  string                 `json:"message"`
		Detail   map[string]interface{} `json:"detail"`
	}

	// Try to parse structured error. Fall back to raw body on parse failure.
	if err := json.Unmarshal(body, &errResp); err != nil {
		errResp.Code = "unknown"
		errResp.Message = string(body)
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return &AuthorizationDeniedError{
			Layer:   1,
			Message: errResp.Message,
		}
	case http.StatusForbidden:
		layer := errResp.Step
		if layer == 0 {
			layer = 2
		}
		return &AuthorizationDeniedError{
			Layer:   layer,
			Policy:  errResp.Code,
			Message: errResp.Message,
		}
	case http.StatusNotFound:
		return &NotFoundError{
			EntityType: fmt.Sprintf("(HTTP 404: %s)", errResp.Message),
		}
	case http.StatusConflict:
		if errResp.Code == "stale_version" {
			return parseStaleVersionError(errResp.Detail)
		}
		return &HTTPError{StatusCode: statusCode, Code: errResp.Code, Message: errResp.Message, Detail: errResp.Detail}
	case http.StatusBadRequest:
		if errResp.Code == "validation_failed" {
			return parseValidationError(errResp.Detail)
		}
		if errResp.Code == "undeclared_report_key" {
			key, _ := errResp.Detail["key"].(string)
			table, _ := errResp.Detail["target_table"].(string)
			return &UndeclaredReportKeyError{TargetTable: table, Key: key}
		}
		return &HTTPError{StatusCode: statusCode, Code: errResp.Code, Message: errResp.Message, Detail: errResp.Detail}
	case http.StatusTooManyRequests:
		return &HTTPError{StatusCode: statusCode, Code: "rate_limited", Message: errResp.Message, Detail: errResp.Detail}
	case http.StatusServiceUnavailable, http.StatusBadGateway:
		return &HTTPError{StatusCode: statusCode, Code: errResp.Code, Message: errResp.Message, Detail: errResp.Detail}
	default:
		return &HTTPError{StatusCode: statusCode, Code: errResp.Code, Message: errResp.Message, Detail: errResp.Detail}
	}
}

// parseStaleVersionError extracts stale entity details from the error detail map.
func parseStaleVersionError(detail map[string]interface{}) *StaleVersionError {
	sErr := &StaleVersionError{}
	if entities, ok := detail["stale_entities"].([]interface{}); ok {
		for _, e := range entities {
			if m, ok := e.(map[string]interface{}); ok {
				info := StaleEntityInfo{}
				if v, ok := m["entity_type"].(string); ok {
					info.EntityType = v
				}
				if v, ok := m["entity_id"].(float64); ok {
					info.EntityID = int(v)
				}
				if v, ok := m["drafted_version"].(float64); ok {
					info.DraftedVersion = int(v)
				}
				if v, ok := m["current_version"].(float64); ok {
					info.CurrentVersion = int(v)
				}
				sErr.StaleEntities = append(sErr.StaleEntities, info)
			}
		}
	}
	return sErr
}

// parseValidationError extracts field error details from the error detail map.
func parseValidationError(detail map[string]interface{}) *ValidationFailedError {
	vErr := &ValidationFailedError{}
	if fields, ok := detail["fields"].([]interface{}); ok {
		for _, f := range fields {
			if m, ok := f.(map[string]interface{}); ok {
				fe := FieldError{}
				if v, ok := m["field"].(string); ok {
					fe.Field = v
				}
				if v, ok := m["code"].(string); ok {
					fe.Code = v
				}
				if v, ok := m["message"].(string); ok {
					fe.Message = v
				}
				vErr.Fields = append(vErr.Fields, fe)
			}
		}
	}
	return vErr
}

// isReportKeyDeclared checks whether a key is in the locally cached report
// key declarations for this runner.
func (c *APIClient) isReportKeyDeclared(targetTable string, key string) bool {
	for _, rk := range c.ReportKeys {
		if rk.TargetTable == targetTable && rk.Key == key {
			return true
		}
	}
	return false
}

// nonRetryableError wraps an error to signal WithRetry to stop immediately.
type nonRetryableError struct {
	err error
}

func (e *nonRetryableError) Error() string {
	return e.err.Error()
}

// --- Request/Response Types ---

// SearchFilter represents one filter predicate for search operations.
type SearchFilter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// OrderSpec represents one ordering directive for search operations.
type OrderSpec struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// SearchResult holds paginated search results.
type SearchResult struct {
	Rows       []map[string]interface{}
	Cursor     string
	TotalCount int
}

// DependencyNode represents one node in a dependency walk result.
type DependencyNode struct {
	EntityType string                 `json:"entity_type"`
	EntityID   int                    `json:"entity_id"`
	Depth      int                    `json:"depth"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ResolveResult holds authority pointer resolution details.
type ResolveResult struct {
	AuthorityID     int                    `json:"authority_id"`
	AuthorityName   string                 `json:"authority_name"`
	AuthorityType   string                 `json:"authority_type"`
	BaseURL         string                 `json:"base_url"`
	PointerType     string                 `json:"pointer_type"`
	Locator         string                 `json:"locator"`
	PointerDataJSON map[string]interface{} `json:"pointer_data_json"`
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
	EntityType   string      `json:"entity_type"`
	EntityID     int         `json:"entity_id"`
	FieldName    string      `json:"field_name"`
	BeforeValue  interface{} `json:"before_value"`
	AfterValue   interface{} `json:"after_value"`
	ChangeType   string      `json:"change_type"`
	VersionStamp int         `json:"version_stamp"`
}

// ChangeSetResult holds the result of a change set operation.
type ChangeSetResult struct {
	ChangeSetID      int           `json:"change_set_id"`
	Status           string        `json:"status"`
	ApprovalRequired []interface{} `json:"approval_required"`
	ValidationErrors []interface{} `json:"validation_errors"`
	DryRunResult     interface{}   `json:"dry_run_result"`
}

// WatchEvent represents one change event in a watch stream.
type WatchEvent struct {
	Type       string                 `json:"type"`
	EntityType string                 `json:"entity_type"`
	EntityID   int                    `json:"entity_id"`
	Data       map[string]interface{} `json:"data"`
	Version    int                    `json:"version"`
	Timestamp  time.Time              `json:"timestamp"`
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

// FieldError describes one field that failed validation.
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

// StaleEntityInfo records one entity whose version advanced since drafting.
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

// HTTPError represents an HTTP error response from the API.
type HTTPError struct {
	StatusCode int
	Code       string
	Message    string
	Detail     map[string]interface{}
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s: %s", e.StatusCode, e.Code, e.Message)
}
