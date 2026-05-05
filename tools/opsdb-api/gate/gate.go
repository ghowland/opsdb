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

// GateRequest represents an incoming API request to be processed through the gate.
type GateRequest struct {
	Operation      string                 // get_entity, submit_change_set, etc.
	OperationClass string                 // read, write-direct, write-cs, cm-action, stream
	TargetEntity   string                 // entity type name
	TargetEntityID int                    // entity ID (0 for creates/searches)
	Params         map[string]interface{} // operation-specific parameters
	RawCredentials auth.Credentials       // passed to auth step
	ClientIP       string
	UserAgent      string
	RequestID      string // unique per request for correlation
	ReceivedAt     time.Time
}

// GateResponse represents the result of gate processing.
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
	Step     int                    // which gate step rejected (1-10)
	StepName string                 // human-readable step name
	Code     string                 // validation_failed, authorization_denied, stale_version, etc.
	Message  string
	Detail   map[string]interface{} // step-specific error detail
}

func (e *GateError) Error() string {
	return fmt.Sprintf("gate step %d (%s): %s: %s", e.Step, e.StepName, e.Code, e.Message)
}

// GateContext carries state through the 10-step pipeline.
// Each step reads from and writes to this context.
type GateContext struct {
	Request         *GateRequest
	DB              *pg.DB
	Schema          *runtimeschema.RuntimeSchema
	AuthProvider    auth.Provider
	ReportKeys      *reportkeys.Enforcer

	// step results — populated as pipeline progresses
	Identity        *auth.Identity     // set by step 1
	AuthzResult     *AuthzResult       // set by step 2
	SchemaValid     bool               // set by step 3
	BoundsValid     bool               // set by step 4
	PolicyResult    *PolicyResult      // set by step 5
	VersionInfo     *VersionPrepResult // set by step 6
	CMRouting       *CMRoutingResult   // set by step 7
	AuditEntryID    int                // set by step 8
	ExecutionResult *ExecutionResult   // set by step 9

	// rejection state
	Rejected       bool
	RejectionError *GateError

	// accumulated warnings from any step
	Warnings []string
}

// AuthzResult holds the outcome of the five-layer authorization check.
type AuthzResult struct {
	Allowed       bool
	DeniedLayer   int    // which layer denied (1-5), 0 if allowed
	DeniedPolicy  string // policy that caused denial
	OmittedFields []string // fields omitted due to access classification
}

// PolicyResult holds the outcome of policy evaluation (step 5).
type PolicyResult struct {
	Passed   bool
	Warnings []string // non-blocking policy warnings
	Blocks   []string // blocking policy violations
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

// Gate orchestrates the 10-step pipeline. Holds references to all
// dependencies that steps need.
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
// schema loaded, auth provider configured.
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

// stepName maps step numbers to human-readable names.
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
// Each step can reject; first rejection stops the pipeline.
// Audit logging (step 8) runs on both success and rejection paths.
func (g *Gate) ProcessRequest(req *GateRequest) *GateResponse {
	ctx := &GateContext{
		Request:      req,
		DB:           g.db,
		Schema:       g.schema,
		AuthProvider: g.auth,
		ReportKeys:   g.reportKeys,
	}

	// step 1: authentication
	stepAuthenticate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// step 2: authorization (five layers, first denial halts)
	stepAuthorize(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// step 3: schema validation
	stepSchemaValidate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// step 4: bound validation
	stepBoundValidate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// step 5: policy evaluation
	stepPolicyEvaluate(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// step 6: versioning preparation (non-rejecting for reads)
	stepVersioningPrepare(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// step 7: change management routing (non-rejecting for reads and direct writes)
	stepChangeMgmtRoute(ctx)
	if ctx.Rejected {
		stepAuditLog(ctx)
		return stepResponse(ctx)
	}

	// step 8: audit logging — always runs, atomic with operation outcome
	stepAuditLog(ctx)

	// step 9: execution — only if not rejected
	if !ctx.Rejected {
		stepExecute(ctx)
		// if execution fails, the audit entry already recorded the attempt;
		// the error is captured in the response
	}

	// step 10: response — always runs
	return stepResponse(ctx)
}

// reject marks the context as rejected with a structured error.
// Called by individual step implementations when they need to halt the pipeline.
func reject(ctx *GateContext, step int, code string, message string, detail map[string]interface{}) {
	ctx.Rejected = true
	ctx.RejectionError = &GateError{
		Step:     step,
		StepName: stepNames[step],
		Code:     code,
		Message:  message,
		Detail:   detail,
	}
}

// warn adds a non-blocking warning to the context.
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

// isReadOnly returns true if the operation is a read.
func isReadOnly(opClass string) bool {
	return opClass == "read" || opClass == "stream"
}
