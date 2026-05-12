//# tools/opsdb-api/gate/gate.go

package gate

import (
	"fmt"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/tools/opsdb-api/auth"
	"github.com/ghowland/opsdb/tools/opsdb-api/reportkeys"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb-api/schema"
)

// Identity is a type alias so step_execute.go and other gate files can
// reference auth.Identity without importing the auth package directly.
// This keeps the gate package's dependency on auth to this single file.
type Identity = auth.Identity

// ---------------------------------------------------------------------------
// Request / Response / Error
// ---------------------------------------------------------------------------

// GateRequest represents an incoming API request to be processed through
// the 10-step gate pipeline.
type GateRequest struct {
	// Operation is the specific API operation name: get_entity,
	// submit_change_set, write_observation, approve_change_set, etc.
	Operation string

	// OperationClass groups operations by their write behavior:
	//   "read"         — get_entity, search, get_dependencies, etc.
	//   "stream"       — watch
	//   "write-direct" — write_observation (direct write, no change mgmt)
	//   "write-cs"     — submit_change_set, bulk, emergency (change managed)
	//   "cm-action"    — approve, reject, cancel, apply_field_change, mark_applied
	OperationClass string

	// TargetEntity is the entity type name this operation targets.
	TargetEntity string

	// TargetEntityID is the specific entity ID. Zero for creates, searches,
	// and operations that target multiple entities.
	TargetEntityID int

	// Params holds operation-specific parameters parsed from the HTTP
	// request body. Keys and value types vary per operation.
	Params map[string]interface{}

	// RawCredentials carries the authentication material from the HTTP
	// request — basic auth, bearer token, or OIDC token.
	RawCredentials auth.Credentials

	// Request metadata for audit logging
	ClientIP  string
	UserAgent string

	// RequestID is unique per request, used for correlation across the
	// audit log, structured logs, and traces.
	RequestID string

	// ReceivedAt is when the API server received the HTTP request.
	ReceivedAt time.Time
}

// GateResponse is returned by ProcessRequest to the HTTP handler, which
// serializes it as the API response.
type GateResponse struct {
	Success      bool
	Data         interface{}
	Error        *GateError
	AuditEntryID int
	Warnings     []string
	Metadata     map[string]interface{}
}

// GateError represents a structured rejection from any gate step.
type GateError struct {
	Step     int
	StepName string
	Code     string
	Message  string
	Detail   map[string]interface{}
}

// Error implements the error interface so GateError can be used as an error.
func (e *GateError) Error() string {
	return fmt.Sprintf("gate step %d (%s): %s: %s", e.Step, e.StepName, e.Code, e.Message)
}

// ---------------------------------------------------------------------------
// Gate context — carries state through the 10-step pipeline
// ---------------------------------------------------------------------------

// GateContext is created per request and threaded through every gate step.
// Each step reads what prior steps wrote and populates its own results.
type GateContext struct {
	// Inputs — set once at context creation, read by all steps
	Request      *GateRequest
	DB           *pg.DB
	Schema       *runtimeschema.RuntimeSchema
	AuthProvider auth.Provider
	ReportKeys   *reportkeys.Enforcer

	// Step 1 result: resolved caller identity
	Identity *Identity

	// Step 2 result: five-layer authorization outcome
	AuthzResult *AuthzResult

	// Step 3 result: schema validation passed
	SchemaValid bool

	// Step 4 result: bound validation passed
	BoundsValid bool

	// Step 5 result: policy evaluation outcome
	PolicyResult *PolicyResult

	// Step 6 result: prepared versioning data for change-managed entities
	VersionInfo *VersionPrepResult

	// Step 7 result: change management routing — who must approve
	CMRouting *CMRoutingResult

	// Step 8 result: audit log entry ID for correlation
	AuditEntryID int

	// Step 9 result: what was written to the database
	ExecutionResult *ExecutionResult

	// Rejection state — set by any step that halts the pipeline
	Rejected       bool
	RejectionError *GateError

	// Warnings accumulated from any step — non-blocking issues included
	// in the response
	Warnings []string
}

// ---------------------------------------------------------------------------
// Step result structs
// ---------------------------------------------------------------------------

// AuthzResult holds the outcome of the five-layer authorization check
// (step 2). When denied, DeniedLayer and DeniedPolicy identify which
// layer and which policy caused the denial.
type AuthzResult struct {
	Allowed       bool
	DeniedLayer   int      // 1-5 if denied, 0 if allowed
	DeniedPolicy  string   // policy name or rule ID that caused denial
	OmittedFields []string // fields omitted from response due to access classification
}

// PolicyResult holds the outcome of policy evaluation (step 5).
type PolicyResult struct {
	Passed   bool
	Warnings []string // non-blocking policy warnings
	Blocks   []string // blocking policy violations
}

// VersionPrepResult holds the prepared versioning data (step 6).
// Used by step 9 to insert the version sibling row when applying
// field changes to versioned entities.
type VersionPrepResult struct {
	NextSerial  int
	ParentVID   int
	ChangeSetID int
}

// CMRoutingResult holds the change management routing outcome (step 7).
// AutoApproved is true when all matching approval rules allow auto-approval
// for this change set, in which case the change set goes directly to
// approved status without waiting for human approvers.
type CMRoutingResult struct {
	AutoApproved     bool
	ApprovalRequired []ApprovalRequirement
}

// ApprovalRequirement represents one computed approval requirement —
// a specific group that must provide a specific number of approvals.
type ApprovalRequirement struct {
	RuleID        int
	GroupID       int
	GroupName     string
	CountRequired int
}

// ExecutionResult holds the outcome of the execution step (step 9).
type ExecutionResult struct {
	AffectedRowIDs []int
	VersionRowIDs  []int
	ChangeSetID    int
}

// ---------------------------------------------------------------------------
// Gate — the pipeline orchestrator
// ---------------------------------------------------------------------------

// Gate orchestrates the 10-step pipeline. It holds references to all
// shared dependencies that individual steps need. One Gate instance
// serves all requests for the lifetime of the API server process.
type Gate struct {
	db         *pg.DB
	schema     *runtimeschema.RuntimeSchema
	auth       auth.Provider
	reportKeys *reportkeys.Enforcer
}

// NewGate creates a gate pipeline with all required dependencies.
func NewGate(db *pg.DB, schema *runtimeschema.RuntimeSchema, authProvider auth.Provider, reportKeyEnforcer *reportkeys.Enforcer) *Gate {
	return &Gate{
		db:         db,
		schema:     schema,
		auth:       authProvider,
		reportKeys: reportKeyEnforcer,
	}
}

// IsReady returns true if the gate can serve requests: database reachable,
// schema loaded with at least one entity type, auth provider configured.
func (g *Gate) IsReady() bool {
	if g.db == nil || g.schema == nil || g.auth == nil {
		return false
	}
	err := g.db.Ping()
	if err != nil {
		return false
	}
	if g.schema.EntityCount() == 0 {
		return false
	}
	return true
}

// stepNames maps step numbers to human-readable names for error reporting
// and audit logging.
var stepNames = map[int]string{
	1:  "authentication",
	2:  "authorization",
	3:  "schema_validation",
	4:  "bound_validation",
	5:  "policy_evaluation",
	6:  "versioning_preparation",
	7:  "change_management_routing",
	8:  "audit_logging",
	9:  "execution",
	10: "response",
}

// ProcessRequest runs the 10-step gate pipeline on a request.
//
// Each step can reject the request by calling reject(). The first
// rejection stops the pipeline — no subsequent steps run except audit
// logging (step 8) which always runs on both success and rejection
// paths, and response construction (step 10) which always runs.
//
// The pipeline:
//  1. Authentication — validate credentials, resolve identity
//  2. Authorization — five-layer check (stubbed for now)
//  3. Schema validation — check operation shape (stubbed for now)
//  4. Bound validation — check field values (stubbed for now)
//  5. Policy evaluation — semantic invariants (stubbed for now)
//  6. Versioning preparation — prepare version row (stubbed for now)
//  7. Change management routing — compute approvals (stubbed for now)
//  8. Audit logging — record the operation (always runs)
//  9. Execution — perform the database write (only if not rejected)
//  10. Response — construct the API response (always runs)
func (g *Gate) ProcessRequest(req *GateRequest) *GateResponse {
	ctx := &GateContext{
		Request:      req,
		DB:           g.db,
		Schema:       g.schema,
		AuthProvider: g.auth,
		ReportKeys:   g.reportKeys,
	}

	// Step 1: Authentication — resolve caller identity from credentials.
	// This step is fully implemented: it calls the auth provider and
	// populates ctx.Identity.
	stepAuthenticate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return buildResponse(ctx)
	}

	// Step 2: Authorization — evaluate five layers of authorization.
	// STUB: allows all requests. When implemented, this will evaluate:
	//   Layer 1: standard role and group membership
	//   Layer 2: per-entity _requires_group governance field
	//   Layer 3: per-field _access_classification
	//   Layer 4: per-runner declared scope (runner_capability, runner_*_target)
	//   Layer 5: policy rules (time-of-day, segregation of duties, tenure)
	// First denial halts; all five must pass.
	stepAuthorize(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return buildResponse(ctx)
	}

	// Step 3: Schema validation — check that the operation's target entity
	// type exists, that field names are valid, that field types match.
	// STUB: passes all requests. When implemented, this will read from
	// the runtime schema cache (loaded from _schema_entity_type and
	// _schema_field tables) and reject malformed requests with structured
	// error feedback identifying which fields failed and why.
	stepSchemaValidate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return buildResponse(ctx)
	}

	// Step 4: Bound validation — check field values against declared
	// constraints: numeric ranges, string lengths, enum membership, FK
	// existence, precision limits.
	// STUB: passes all requests. When implemented, this will read
	// constraint metadata from _schema_field rows and validate each
	// submitted value. No regex — per OPSDB-7, the constraint vocabulary
	// is closed and regex is explicitly forbidden.
	stepBoundValidate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return buildResponse(ctx)
	}

	// Step 5: Policy evaluation — evaluate semantic invariants, data
	// classification rules, retention policies, segregation of duties.
	// STUB: passes all requests. When implemented, this will read policy
	// rows with policy_type='semantic_invariant', 'access_control',
	// 'retention', etc. and evaluate them against the operation context.
	// Blocking violations reject; non-blocking violations produce warnings.
	stepPolicyEvaluate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return buildResponse(ctx)
	}

	// Step 6: Versioning preparation — for change-managed entities,
	// compute the next version_serial and parent version ID for the
	// version sibling row that step 9 will insert.
	// STUB: leaves ctx.VersionInfo nil. Step 9 handles nil VersionInfo
	// gracefully — the entity update still applies, just without a
	// version history row until this step is implemented.
	stepVersioningPrepare(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return buildResponse(ctx)
	}

	// Step 7: Change management routing — for change-set submissions,
	// evaluate approval_rule policies to determine who must approve.
	// Walks ownership and stakeholder bridges to find required approver
	// groups. Determines auto-approve vs human approval.
	// STUB: leaves ctx.CMRouting nil. Step 9 handles nil CMRouting by
	// defaulting change sets to pending_approval status.
	stepChangeMgmtRoute(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return buildResponse(ctx)
	}

	// Step 8: Audit logging — record the operation in audit_log_entry.
	// Always runs on both success and rejection paths. This is the one
	// step that is NOT skipped on rejection — every API interaction
	// must be recorded.
	stepAuditLog(ctx)

	// Step 9: Execution — perform the actual database write. Only runs
	// if not rejected. Read operations pass through with an empty result.
	// Write operations run within a single Postgres transaction.
	if !ctx.Rejected {
		stepExecute(ctx)
	}

	// Step 10: Response — construct the GateResponse from the accumulated
	// context. Always runs.
	return buildResponse(ctx)
}

// ---------------------------------------------------------------------------
// Step 1: Authentication (fully implemented)
// ---------------------------------------------------------------------------

// stepAuthenticate validates the caller's credentials through the
// configured auth provider and populates ctx.Identity on success.
func stepAuthenticate(ctx *GateContext) {
	if ctx.AuthProvider == nil {
		reject(ctx, 1, "auth_not_configured",
			"no authentication provider is configured", nil)
		return
	}

	identity, err := ctx.AuthProvider.Authenticate(ctx.Request.RawCredentials)
	if err != nil {
		reject(ctx, 1, "authentication_failed", err.Error(), nil)
		return
	}

	if identity == nil {
		reject(ctx, 1, "authentication_failed",
			"auth provider returned nil identity", nil)
		return
	}

	ctx.Identity = identity
}

// ---------------------------------------------------------------------------
// Steps 2-7: Stubs (pass-through, to be implemented)
// ---------------------------------------------------------------------------

// stepAuthorize is step 2: Authorization.
// STUB: allows all requests. See ProcessRequest comments for what this
// will do when implemented.
func stepAuthorize(ctx *GateContext) {
	ctx.AuthzResult = &AuthzResult{
		Allowed: true,
	}
}

// stepSchemaValidate is step 3: Schema validation.
// STUB: passes all requests.
func stepSchemaValidate(ctx *GateContext) {
	ctx.SchemaValid = true
}

// stepBoundValidate is step 4: Bound validation.
// STUB: passes all requests.
func stepBoundValidate(ctx *GateContext) {
	ctx.BoundsValid = true
}

// stepPolicyEvaluate is step 5: Policy evaluation.
// STUB: passes all requests.
func stepPolicyEvaluate(ctx *GateContext) {
	ctx.PolicyResult = &PolicyResult{
		Passed: true,
	}
}

// stepVersioningPrepare is step 6: Versioning preparation.
// STUB: leaves ctx.VersionInfo nil. Step 9 (executeApplyFieldChange)
// checks for nil and skips version row insertion when this is not set.
func stepVersioningPrepare(ctx *GateContext) {
	// When implemented, this will:
	// 1. Check if the target entity type is versioned (via runtime schema)
	// 2. Query the current max version_serial for the target entity
	// 3. Set ctx.VersionInfo with NextSerial = max + 1, ParentVID = current
	//    active version row ID, ChangeSetID from the request
}

// stepChangeMgmtRoute is step 7: Change management routing.
// STUB: leaves ctx.CMRouting nil. Step 9 (executeSubmitChangeSet)
// defaults to pending_approval status when CMRouting is nil.
func stepChangeMgmtRoute(ctx *GateContext) {
	// When implemented, this will:
	// 1. Skip for read operations and direct writes (write_observation)
	// 2. For change set submissions, evaluate approval_rule policy rows
	//    against the affected entities
	// 3. Walk service_ownership, machine_ownership, k8s_cluster_ownership,
	//    cloud_resource_ownership bridges to find responsible ops_user_roles
	// 4. Compute change_set_approval_required rows
	// 5. Determine if all rules auto-approve (ctx.CMRouting.AutoApproved)
}

// ---------------------------------------------------------------------------
// Step 8: Audit logging
// ---------------------------------------------------------------------------

// stepAuditLog is step 8: Audit logging.
// Records the operation in audit_log_entry. Runs on BOTH success and
// rejection paths — every API interaction must be recorded regardless
// of outcome. This is the architectural commitment from OPSDB-6 §9:
// the audit log is the system's queryable memory of who did what.
//
// Currently a minimal implementation that inserts the core fields.
// Will be extended to include the full audit_log_entry schema with
// optional cryptographic chaining when stricter regimes require it.
func stepAuditLog(ctx *GateContext) {
	now := time.Now().UTC()

	var opsUserID interface{}
	var serviceAccountID interface{}
	if ctx.Identity != nil {
		if ctx.Identity.OpsUserID != nil {
			opsUserID = *ctx.Identity.OpsUserID
		}
		if ctx.Identity.RunnerMachineID != nil {
			serviceAccountID = *ctx.Identity.RunnerMachineID
		}
	}

	actionType := classifyActionType(ctx.Request.Operation, ctx.Request.OperationClass)

	responseStatus := "success"
	if ctx.Rejected {
		responseStatus = "rejected"
	}

	var auditEntryID int
	err := pg.QueryRowInDB(ctx.DB,
		"INSERT INTO audit_log_entry "+
			"(acting_ops_user_id, acting_service_account_id, "+
			"api_endpoint, http_method, action_type, "+
			"target_entity_type, target_entity_id, "+
			"response_status, client_ip_address, client_user_agent, "+
			"acted_time, created_time) "+
			"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11) "+
			"RETURNING id",
		opsUserID,
		serviceAccountID,
		ctx.Request.Operation,
		ctx.Request.OperationClass,
		actionType,
		ctx.Request.TargetEntity,
		nullIfZero(ctx.Request.TargetEntityID),
		responseStatus,
		ctx.Request.ClientIP,
		ctx.Request.UserAgent,
		now,
	).Scan(&auditEntryID)

	if err != nil {
		// Audit logging failure is serious but must not crash the request.
		// We record the failure as a warning — the operation itself may
		// still succeed, but the audit gap must be investigated.
		warn(ctx, fmt.Sprintf("audit log insert failed: %v", err))
		return
	}

	ctx.AuditEntryID = auditEntryID
}

// classifyActionType maps operation name and class to the action_type
// value stored in audit_log_entry.
func classifyActionType(operation string, operationClass string) string {
	switch operationClass {
	case "read", "stream":
		return "read"
	case "write-direct":
		return "create"
	case "write-cs":
		return "change_set_submit"
	case "cm-action":
		switch operation {
		case "approve_change_set":
			return "approve"
		case "reject_change_set":
			return "reject"
		case "cancel_change_set":
			return "change_set_submit" // cancel is a change set action
		case "apply_change_set_field_change":
			return "update"
		case "mark_change_set_applied":
			return "update"
		default:
			return operation
		}
	default:
		return operation
	}
}

// ---------------------------------------------------------------------------
// Step 10: Response construction
// ---------------------------------------------------------------------------

// buildResponse constructs the GateResponse from the accumulated context.
// Always runs as the final step.
func buildResponse(ctx *GateContext) *GateResponse {
	resp := &GateResponse{
		AuditEntryID: ctx.AuditEntryID,
		Warnings:     ctx.Warnings,
		Metadata:     make(map[string]interface{}),
	}

	if ctx.Rejected {
		resp.Success = false
		resp.Error = ctx.RejectionError
		return resp
	}

	resp.Success = true

	// Populate response data from execution result
	if ctx.ExecutionResult != nil {
		if ctx.ExecutionResult.ChangeSetID > 0 {
			resp.Metadata["change_set_id"] = ctx.ExecutionResult.ChangeSetID
		}
		if len(ctx.ExecutionResult.AffectedRowIDs) > 0 {
			resp.Metadata["affected_row_ids"] = ctx.ExecutionResult.AffectedRowIDs
		}
		if len(ctx.ExecutionResult.VersionRowIDs) > 0 {
			resp.Metadata["version_row_ids"] = ctx.ExecutionResult.VersionRowIDs
		}
	}

	return resp
}

// ---------------------------------------------------------------------------
// Pipeline helpers
// ---------------------------------------------------------------------------

// reject marks the context as rejected with a structured error.
// Called by individual step implementations when they need to halt
// the pipeline. Only the first rejection takes effect — subsequent
// calls are ignored (should not happen in normal flow since the
// pipeline short-circuits, but defensive).
func reject(ctx *GateContext, step int, code string, message string, detail map[string]interface{}) {
	if ctx.Rejected {
		return
	}
	ctx.Rejected = true
	ctx.RejectionError = &GateError{
		Step:     step,
		StepName: stepNames[step],
		Code:     code,
		Message:  message,
		Detail:   detail,
	}
}

// warn adds a non-blocking warning to the context. Warnings are included
// in the response but do not halt the pipeline.
func warn(ctx *GateContext, message string) {
	ctx.Warnings = append(ctx.Warnings, message)
}

// isWriteOperation returns true if the operation class involves writes
// to the database (direct observation writes, change set submissions,
// or change management actions).
func isWriteOperation(opClass string) bool {
	switch opClass {
	case "write-direct", "write-cs", "cm-action":
		return true
	default:
		return false
	}
}

// isChangeManaged returns true if the operation class goes through the
// change set pipeline (submit, bulk submit, emergency apply).
func isChangeManaged(opClass string) bool {
	return opClass == "write-cs"
}

// isReadOnly returns true if the operation is a read or stream.
func isReadOnly(opClass string) bool {
	return opClass == "read" || opClass == "stream"
}

// nullIfZero returns nil for zero values, producing SQL NULL in insert
// statements. Non-zero values pass through as-is.
func nullIfZero(v int) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
