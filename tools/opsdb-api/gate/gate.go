
// === opsdb-api/gate/gate.go ===
package gate

// GateRequest represents an incoming API request to be processed through the gate.
type GateRequest struct {
	Operation       string                 // get_entity, submit_change_set, etc.
	OperationClass  string                 // read, write-direct, write-change-set, change-mgmt-action
	TargetEntity    string                 // entity type name
	TargetEntityID  int                    // entity ID (0 for creates/searches)
	Params          map[string]interface{} // operation-specific parameters
	RawCredentials  interface{}            // passed to auth step
	ClientIP        string
	UserAgent       string
	RequestID       string                 // unique per request for correlation
}

// GateResponse represents the result of gate processing.
type GateResponse struct {
	Success       bool
	Data          interface{}            // result data on success
	Error         *GateError             // structured error on failure
	AuditEntryID  int                    // audit_log_entry.id for correlation
	Warnings      []string               // non-blocking warnings
	Metadata      map[string]interface{} // response metadata (affected IDs, computed approvals, etc.)
}

// GateError represents a structured rejection from any gate step.
type GateError struct {
	Step       int    // which gate step rejected (1-10)
	StepName   string // human-readable step name
	Code       string // error code: validation_failed, authorization_denied, stale_version, etc.
	Message    string
	Detail     map[string]interface{} // step-specific error detail
}

// GateContext carries state through the 10-step pipeline.
// Each step reads from and writes to this context.
type GateContext struct {
	Request         *GateRequest
	Identity        *Identity             // set by step 1
	AuthzResult     *AuthzResult          // set by step 2
	SchemaValid     bool                  // set by step 3
	BoundsValid     bool                  // set by step 4
	PolicyResult    *PolicyResult         // set by step 5
	VersionInfo     *VersionPrepResult    // set by step 6
	CMRouting       *CMRoutingResult      // set by step 7
	AuditEntryID    int                   // set by step 8
	ExecutionResult *ExecutionResult      // set by step 9
	Rejected        bool                  // true if any step rejected
	RejectionError  *GateError            // set by rejecting step
	Warnings        []string              // accumulated warnings
}

// ProcessRequest runs the 10-step gate pipeline on a request.
// Each step can reject; first rejection stops the pipeline.
// Audit logging (step 8) runs on both success and rejection paths.
func ProcessRequest(req *GateRequest) *GateResponse {
	// TODO: create GateContext from request
	// TODO: step 1: Authenticate (step_auth.go)
	//   if rejected: skip to step 8 (audit), then step 10 (response)
	// TODO: step 2: Authorize (step_authz.go)
	//   if rejected: skip to step 8, then step 10
	// TODO: step 3: Schema Validate (step_schema_validate.go)
	//   if rejected: skip to step 8, then step 10
	// TODO: step 4: Bound Validate (step_bound_validate.go)
	//   if rejected: skip to step 8, then step 10
	// TODO: step 5: Policy Evaluate (step_policy.go)
	//   if rejected: skip to step 8, then step 10
	// TODO: step 6: Versioning Prepare (step_versioning.go)
	// TODO: step 7: Change Management Route (step_changemgmt.go)
	// TODO: step 8: Audit Log (step_audit.go) — always runs
	// TODO: step 9: Execute (step_execute.go) — only if not rejected
	// TODO: step 10: Response (step_response.go) — always runs
	return nil
}

// Placeholder types referenced by GateContext
type Identity = struct{} // actually from auth package
type AuthzResult struct {
	Allowed      bool
	DeniedLayer  int
	DeniedPolicy string
	OmittedFields []string // fields omitted due to classification
}
type PolicyResult struct {
	Passed   bool
	Warnings []string
	Blocks   []string
}
type VersionPrepResult struct {
	NextSerial  int
	ParentVID   int
	ChangeSetID int
}
type CMRoutingResult struct {
	AutoApproved      bool
	ApprovalRequired  []ApprovalRequirement
}
type ApprovalRequirement struct {
	RuleID           int
	GroupID          int
	GroupName        string
	CountRequired    int
}
type ExecutionResult struct {
	AffectedRowIDs []int
	VersionRowIDs  []int
	ChangeSetID    int
}

