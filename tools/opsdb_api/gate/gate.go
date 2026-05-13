//# tools/opsdb_api/gate/gate.go

package gate

import (
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/tools/opsdb_api/auth"
	"github.com/ghowland/opsdb/tools/opsdb_api/reportkeys"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb_api/schema"
)

// Identity is a type alias so gate step files can reference auth.Identity
// without each file importing the auth package directly. This keeps the
// gate package's dependency on auth consolidated in this file.
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
	if e == nil {
		return "<nil gate error>"
	}
	return e.StepName + ": " + e.Code + ": " + e.Message
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

// AuthzResult holds the outcome of the five-layer authorization check (step 2).
type AuthzResult struct {
	Allowed       bool
	DeniedLayer   int
	DeniedPolicy  string
	OmittedFields []string
}

// PolicyResult holds the outcome of policy evaluation (step 5).
type PolicyResult struct {
	Passed   bool
	Warnings []string
	Blocks   []string
}

// VersionPrepResult holds the prepared versioning data (step 6).
type VersionPrepResult struct {
	NextSerial  int
	ParentVID   int
	ChangeSetID int
}

// CMRoutingResult holds the change management routing outcome (step 7).
type CMRoutingResult struct {
	AutoApproved     bool
	ApprovalRequired []ApprovalRequirement
}

// ApprovalRequirement represents one computed approval requirement.
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

// Gate orchestrates the 10-step pipeline. Holds references to all
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
// Steps are implemented in their own files:
//
//	step_auth.go           — step 1: stepAuthenticate
//	step_authz.go          — step 2: stepAuthorize
//	step_schema_validate.go — step 3: stepSchemaValidate
//	step_bound_validate.go — step 4: stepBoundValidate
//	step_policy.go         — step 5: stepPolicyEvaluate
//	step_versioning.go     — step 6: stepVersioningPrepare
//	step_changemgmt.go     — step 7: stepChangeMgmtRoute
//	step_audit.go          — step 8: stepAuditLog
//	step_execute.go        — step 9: stepExecute
//	step_response.go       — step 10: stepResponse
func (g *Gate) ProcessRequest(req *GateRequest) *GateResponse {
	ctx := &GateContext{
		Request:      req,
		DB:           g.db,
		Schema:       g.schema,
		AuthProvider: g.auth,
		ReportKeys:   g.reportKeys,
	}

	// Step 1: Authentication
	stepAuthenticate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// Step 2: Authorization (five layers)
	stepAuthorize(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// Step 3: Schema validation
	stepSchemaValidate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// Step 4: Bound validation
	stepBoundValidate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// Step 5: Policy evaluation
	stepPolicyEvaluate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// Step 6: Versioning preparation
	stepVersioningPrepare(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// Step 7: Change management routing
	stepChangeMgmtRoute(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// Step 8: Audit logging — always runs
	stepAuditLog(ctx)

	// Step 9: Execution — only if not rejected
	if !ctx.Rejected {
		stepExecute(ctx)
	}

	// Step 10: Response — always runs
	return stepResponse(ctx)
}

// ---------------------------------------------------------------------------
// Pipeline helpers
// ---------------------------------------------------------------------------

// reject marks the context as rejected with a structured error.
// Called by individual step implementations when they need to halt
// the pipeline. Only the first rejection takes effect.
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

// isWriteOperation returns true if the operation class involves writes.
func isWriteOperation(opClass string) bool {
	switch opClass {
	case "write-direct", "write-cs", "cm-action":
		return true
	default:
		return false
	}
}

// isChangeManaged returns true if the operation class goes through
// the change set pipeline.
func isChangeManaged(opClass string) bool {
	return opClass == "write-cs"
}

// isReadOnly returns true if the operation is a read or stream.
func isReadOnly(opClass string) bool {
	return opClass == "read" || opClass == "stream"
}
