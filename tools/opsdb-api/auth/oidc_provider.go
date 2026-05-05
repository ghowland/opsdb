// === opsdb-api/auth/provider.go ===
package auth

// Identity represents a resolved caller identity after authentication.
type Identity struct {
	OpsUserID        *int   // set for human callers
	RunnerMachineID  *int   // set for runner callers
	RunnerSpecID     *int   // set for runner callers
	Username         string // human username or service account name
	Roles            []string
	Groups           []string
	AuthMethod       string // yaml, oidc, service_account
	IsWebMediated    bool   // true when runner carries human identity
}

// Credentials represents raw credentials extracted from an API request.
type Credentials struct {
	BearerToken    string
	BasicUser      string
	BasicPassword  string
	SAMLAssertion  string
	OIDCToken      string
	ClientIP       string
	UserAgent      string
}

// Provider is the interface all auth backends implement.
type Provider interface {
	// Authenticate validates credentials and returns a resolved identity.
	// Returns error on invalid/expired/unresolvable credentials.
	Authenticate(creds Credentials) (*Identity, error)

	// RefreshToken refreshes an expiring token and returns updated identity.
	// Not all providers support refresh; returns error if unsupported.
	RefreshToken(token string) (*Identity, error)

	// Type returns the provider type name: "yaml", "oidc", "service_account".
	Type() string
}

// NewProvider creates an auth provider based on configuration.
// Routes to yaml, oidc, or service_account provider.
func NewProvider(providerType string, configPath string) (Provider, error) {
	// TODO: switch on providerType
	// "yaml" → NewYAMLProvider(configPath)
	// "oidc" → NewOIDCProvider(configPath)
	// "service_account" → NewServiceAccountProvider(configPath)
	// unknown → error
	return nil, nil
}


// === opsdb-api/auth/yaml_provider.go ===
package auth

// YAMLProvider implements auth.Provider using a YAML file backend.
// Zero external dependencies. Used for bootstrap, development, and testing.
type YAMLProvider struct {
	// TODO: users map[string]YAMLUser loaded from users.yaml
	// TODO: filePath for reload capability
}

// YAMLUser represents a user entry in users.yaml.
type YAMLUser struct {
	Username       string
	PasswordBcrypt string   // bcrypt hash, never plaintext
	OpsUserID      int
	Roles          []string
	Groups         []string
}

// NewYAMLProvider loads users.yaml and returns a provider.
func NewYAMLProvider(filePath string) (*YAMLProvider, error) {
	// TODO: read and parse YAML file
	// TODO: validate each entry has username, password hash, ops_user_id
	// TODO: build lookup map by username
	// TODO: return provider
	return nil, nil
}

// Authenticate validates username/password against bcrypt hashes in the YAML file.
func (p *YAMLProvider) Authenticate(creds Credentials) (*Identity, error) {
	// TODO: look up user by creds.BasicUser
	// TODO: bcrypt.CompareHashAndPassword(stored hash, creds.BasicPassword)
	// TODO: on match: return Identity with OpsUserID, Roles, Groups
	// TODO: on mismatch: return error (invalid credentials)
	// TODO: on not found: return error (unknown user)
	return nil, nil
}

// RefreshToken is not supported by the YAML provider.
func (p *YAMLProvider) RefreshToken(token string) (*Identity, error) {
	// TODO: return error: refresh not supported for YAML auth
	return nil, nil
}

// Type returns "yaml".
func (p *YAMLProvider) Type() string {
	return "yaml"
}


// === opsdb-api/auth/oidc_provider.go ===
package auth

// OIDCProvider implements auth.Provider using OIDC token validation.
// For production human authentication via Okta, Azure AD, Google, etc.
type OIDCProvider struct {
	// TODO: issuer URL
	// TODO: client ID
	// TODO: audience
	// TODO: JWKS cache (keys fetched from issuer, cached with TTL)
	// TODO: ops_user mapping lookup (OIDC subject → ops_user_id)
}

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	Audience     string
	JWKSCacheTTL int // seconds
}

// NewOIDCProvider creates an OIDC provider from configuration.
func NewOIDCProvider(configPath string) (*OIDCProvider, error) {
	// TODO: read OIDC config from file
	// TODO: fetch JWKS from issuer discovery endpoint
	// TODO: cache JWKS with configured TTL
	// TODO: return provider
	return nil, nil
}

// Authenticate validates an OIDC token.
func (p *OIDCProvider) Authenticate(creds Credentials) (*Identity, error) {
	// TODO: extract token from creds.OIDCToken or creds.BearerToken
	// TODO: validate JWT signature against cached JWKS
	// TODO: validate issuer, audience, expiration, not-before
	// TODO: extract subject claim
	// TODO: look up ops_user_id from subject (query OpsDB or local mapping cache)
	// TODO: extract roles/groups from token claims if present
	// TODO: return Identity
	return nil, nil
}

// RefreshToken refreshes an OIDC token using the refresh token grant.
func (p *OIDCProvider) RefreshToken(token string) (*Identity, error) {
	// TODO: call issuer token endpoint with refresh_token grant
	// TODO: validate new token
	// TODO: return updated Identity
	return nil, nil
}

// Type returns "oidc".
func (p *OIDCProvider) Type() string {
	return "oidc"
}


// === opsdb-api/auth/serviceaccount_provider.go ===
package auth

// ServiceAccountProvider implements auth.Provider for runner service accounts.
// Validates tokens issued by the secret backend, resolves to runner_machine.
type ServiceAccountProvider struct {
	// TODO: token validation method (symmetric HMAC, asymmetric JWT, vault token lookup)
	// TODO: runner_machine mapping cache
}

// NewServiceAccountProvider creates a service account provider.
func NewServiceAccountProvider(configPath string) (*ServiceAccountProvider, error) {
	// TODO: read config for token validation method
	// TODO: load signing key or configure vault lookup endpoint
	// TODO: prime runner_machine mapping cache from OpsDB
	// TODO: return provider
	return nil, nil
}

// Authenticate validates a service account token.
func (p *ServiceAccountProvider) Authenticate(creds Credentials) (*Identity, error) {
	// TODO: extract token from creds.BearerToken
	// TODO: validate token (signature check, expiry, issuer)
	// TODO: extract service account identifier from token claims
	// TODO: look up runner_machine_id and runner_spec_id from mapping cache
	// TODO: return Identity with RunnerMachineID, RunnerSpecID
	// TODO: if token also carries human identity (web-mediated), set both IDs and IsWebMediated
	return nil, nil
}

// RefreshToken refreshes a service account token.
func (p *ServiceAccountProvider) RefreshToken(token string) (*Identity, error) {
	// TODO: call secret backend for new token
	// TODO: validate new token
	// TODO: return updated Identity
	return nil, nil
}

// Type returns "service_account".
func (p *ServiceAccountProvider) Type() string {
	return "service_account"
}


// === opsdb-api/cmd/main.go ===
package main

import "os"

// main is the CLI entrypoint for the opsdb-api binary.
// Loads configuration, initializes auth provider, connects to database,
// loads runtime schema, starts HTTP server.
func main() {
	// TODO: parse --config flag for DOS config.yaml path
	// TODO: config.LoadConfig(path) to read DOS configuration
	// TODO: pg.Connect(dsn) to open database connection
	// TODO: schema.LoadRuntimeSchema(db) to read _schema_* tables
	// TODO: auth.NewProvider(config.AuthBackend, config.AuthConfigPath)
	// TODO: initialize gate pipeline with db, schema, auth provider
	// TODO: register HTTP handlers for all 16 API operations
	// TODO: configure TLS if cert/key paths provided
	// TODO: start HTTP server on config.ListenAddress
	// TODO: block on shutdown signal (SIGTERM, SIGINT)
	// TODO: graceful shutdown: stop accepting, drain in-flight, close db
	os.Exit(0)
}


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


// === opsdb-api/gate/step_auth.go ===
package gate

// StepAuthenticate is gate step 1: Authentication.
// Validates caller credentials via the configured auth provider.
// Resolves to ops_user (human), runner_machine (runner), or both (web-mediated).
func StepAuthenticate(ctx *GateContext) error {
	// TODO: extract credentials from ctx.Request.RawCredentials
	// TODO: call auth.Provider.Authenticate(credentials)
	// TODO: on success: set ctx.Identity with resolved user/runner IDs
	// TODO: on failure: set ctx.Rejected = true, ctx.RejectionError with step=1,
	//       code=authentication_failed, message from provider
	// TODO: log authentication attempt (success or failure) for step 8
	return nil
}


// === opsdb-api/gate/step_authz.go ===
package gate

// StepAuthorize is gate step 2: Authorization.
// Evaluates five layers with AND composition. First denial halts.
// Records which layer denied and which policy triggered it.
func StepAuthorize(ctx *GateContext) error {
	// TODO: Layer 1: Standard Role and Group
	//   read ops_user_role_member + ops_group_member for caller
	//   check role permits operation class (read, write-direct, write-cs, cm-action)
	//   if denied: reject with layer=1
	//
	// TODO: Layer 2: Per-Entity Governance
	//   read _requires_group on target entity row (if it exists)
	//   check caller is member of required group
	//   if denied: reject with layer=2
	//
	// TODO: Layer 3: Per-Field Classification
	//   read _access_classification on target fields/table
	//   check caller clearance >= classification
	//   if insufficient for specific fields: add to OmittedFields (reads) or reject (writes)
	//   if denied: reject with layer=3
	//
	// TODO: Layer 4: Per-Runner Authority
	//   if caller is runner:
	//     read runner_capability rows
	//     read runner_*_target bridge rows (service, namespace, cloud_account, host_group)
	//     check operation target within declared scope
	//     if denied: reject with layer=4
	//
	// TODO: Layer 5: Policy Rules
	//   read policy rows of type access_control
	//   evaluate time-of-day, SoD, tenure, IP restrictions
	//   if denied: reject with layer=5
	//   if additional approval needed: add to ctx for step 7
	//
	// TODO: set ctx.AuthzResult
	return nil
}


// === opsdb-api/gate/step_schema_validate.go ===
package gate

// StepSchemaValidate is gate step 3: Schema Validation.
// Checks operation shape against registered schema metadata in _schema_* tables.
func StepSchemaValidate(ctx *GateContext) error {
	// TODO: read entity type from runtime schema cache
	//   if entity type not found: reject with "unknown entity type"
	//
	// TODO: for writes (create, update):
	//   for each field in the operation:
	//     check field exists in _schema_field for this entity type
	//     check value type matches field type
	//   for creates:
	//     check all required (non-nullable, no-default) fields are present
	//   for unknown fields:
	//     reject with "unknown field {name} on entity {type}"
	//
	// TODO: set ctx.SchemaValid = true on success
	// TODO: on failure: reject with step=3, structured error listing each failing field
	return nil
}


// === opsdb-api/gate/step_bound_validate.go ===
package gate

// StepBoundValidate is gate step 4: Bound Validation.
// Checks field values against declared constraints from the schema.
// No regex. Declarative bounds only.
func StepBoundValidate(ctx *GateContext) error {
	// TODO: for each field value in the operation:
	//   int/float: check against min_value, max_value if declared
	//   varchar: check against min_length, max_length
	//   enum: check value is in enum_values list
	//   foreign_key: check referenced row exists (SELECT 1 FROM target WHERE id = value)
	//   json: validate structure against registered JSON schema for discriminator value
	//         (look up discriminator field value, find matching json_schema, validate)
	//
	// TODO: no regex evaluation at any point
	// TODO: anchored pattern matching (prefix%, %suffix) for fields that declare it
	//       implemented as string HasPrefix/HasSuffix, not regex
	//
	// TODO: on failure: reject with step=4, list of (field, constraint, submitted value, bound)
	// TODO: set ctx.BoundsValid = true on success
	return nil
}


// === opsdb-api/gate/step_policy.go ===
package gate

// StepPolicyEvaluate is gate step 5: Policy Evaluation.
// Consults policy rows for semantic invariants, data classification,
// retention, separation of duty, and other governance rules.
func StepPolicyEvaluate(ctx *GateContext) error {
	// TODO: read policy rows relevant to the target entity:
	//   policies linked via service_policy, machine_policy, k8s_namespace_policy, cloud_account_policy
	//   policies of type semantic_invariant matching entity type
	//
	// TODO: evaluate semantic invariants (cross-field constraints):
	//   "min_replicas <= max_replicas"
	//   "if status = decommissioned then decommissioned_time must be set"
	//   these are data rows, not hardcoded checks
	//
	// TODO: check data classification consistency:
	//   field classification not higher than entity classification
	//
	// TODO: check retention policy compatibility
	//
	// TODO: check SoD if relevant (submitter != approver checked at approval time,
	//       but SoD policy may restrict other combinations)
	//
	// TODO: policy violations either block (fail-closed) or produce warnings
	//       based on policy configuration
	// TODO: set ctx.PolicyResult
	return nil
}


// === opsdb-api/gate/step_versioning.go ===
package gate

// StepVersioningPrepare is gate step 6: Versioning Preparation.
// Prepares version sibling row for change-managed entities.
// Only runs for write operations against versioned entities.
func StepVersioningPrepare(ctx *GateContext) error {
	// TODO: check if target entity is versioned (from runtime schema)
	//   if not versioned: skip (ctx.VersionInfo = nil)
	//
	// TODO: read current active version for the entity:
	//   SELECT version_serial, id FROM {entity}_version
	//   WHERE {entity}_id = target_id AND is_active_version = true
	//
	// TODO: compute next version_serial = current + 1
	// TODO: set parent version ID = current version ID
	// TODO: change_set_id will be set when change_set is created (step 9)
	//
	// TODO: store in ctx.VersionInfo for step 9 to write
	// TODO: this step never rejects; it only prepares data
	return nil
}


// === opsdb-api/gate/step_changemgmt.go ===
package gate

// StepChangeManagementRoute is gate step 7: Change Management Routing.
// Evaluates approval rules, walks ownership and stakeholder bridges,
// computes required approvals, determines auto-approve vs human approval.
// Only runs for write-change-set operations.
func StepChangeManagementRoute(ctx *GateContext) error {
	// TODO: if operation class is not write-change-set: skip
	//
	// TODO: step SR1: enumerate field changes from the proposed change set
	//   list of (entity_type, entity_id, field_name) tuples
	//
	// TODO: step SR2: walk ownership bridges
	//   for each touched entity:
	//     read service_ownership / machine_ownership / k8s_cluster_ownership / cloud_resource_ownership
	//     collect responsible ops_user_role rows
	//
	// TODO: step SR3: walk stakeholder bridges
	//   read service_stakeholder + other stakeholder bridges
	//   collect additional interested roles
	//
	// TODO: step SR4: evaluate approval rules
	//   read approval_rule policy rows
	//   match against entity types, namespaces, fields, metadata
	//     (data classification, security zone, compliance scope)
	//   each matching rule produces an approval requirement
	//
	// TODO: step SR5: compute requirements
	//   create change_set_approval_required entries:
	//     one per matching rule with group_id, approver_count_required
	//
	// TODO: check auto-approval policies:
	//   if all requirements satisfiable by auto-approval → set AutoApproved = true
	//   otherwise → ApprovalRequired with the requirement list
	//
	// TODO: set ctx.CMRouting
	return nil
}


// === opsdb-api/gate/step_audit.go ===
package gate

// StepAuditLog is gate step 8: Audit Logging.
// Constructs and writes audit_log_entry. Runs on BOTH success and rejection paths.
// Atomic with the operation outcome (within the same transaction).
func StepAuditLog(ctx *GateContext) error {
	// TODO: construct audit_log_entry row:
	//   site_id: from context
	//   acting_ops_user_id: from ctx.Identity (nil for runner-only)
	//   acting_service_account_id: from ctx.Identity (nil for human-only)
	//   api_endpoint: from ctx.Request.Operation
	//   http_method: derived from operation class
	//   action_type: from operation class (read, create, update, delete, approve, reject, etc.)
	//   target_entity_type: from ctx.Request.TargetEntity
	//   target_entity_id: from ctx.Request.TargetEntityID
	//   request_data_summary: summarize request params (not full payload)
	//   response_status: HTTP status code (200, 400, 401, 403, etc.)
	//   response_data_summary: summarize response (affected IDs or error)
	//   client_ip_address: from ctx.Request.ClientIP
	//   client_user_agent: from ctx.Request.UserAgent
	//   acted_time: NOW() from database (API-supplied, not client-supplied)
	//
	// TODO: if tamper evidence enabled:
	//   compute _audit_chain_hash = hash(entry contents + previous entry hash)
	//
	// TODO: INSERT into audit_log_entry (append-only table)
	// TODO: set ctx.AuditEntryID for correlation in response
	//
	// TODO: this step NEVER rejects — it records what happened
	return nil
}


// === opsdb-api/gate/step_execute.go ===
package gate

// StepExecute is gate step 9: Execution.
// Performs the actual database write. Only runs if not rejected in prior steps.
// All writes within a single operation are atomic (single transaction).
func StepExecute(ctx *GateContext) error {
	// TODO: if ctx.Rejected: skip execution entirely
	//
	// TODO: switch on operation type:
	//
	// CASE write_observation:
	//   INSERT or UPSERT into target observation cache table
	//   (keyed by authority+entity_type+entity_id+state_key for state cache,
	//    authority+hostname+metric_key for metric cache)
	//
	// CASE submit_change_set:
	//   INSERT change_set row
	//   INSERT change_set_field_change rows (one per field change)
	//   INSERT change_set_approval_required rows (from step 7)
	//   if auto-approved: transition change_set status to approved
	//   if emergency: INSERT change_set_emergency_review row with status=pending
	//
	// CASE apply_change_set_field_change:
	//   UPDATE target entity row with new field value
	//   INSERT {entity}_version row with full entity state (step 6 prepared this)
	//   UPDATE change_set_field_change.applied_status = applied
	//
	// CASE approve_change_set:
	//   INSERT change_set_approval row
	//   UPDATE change_set_approval_required.fulfilled_count
	//   if all requirements fulfilled: transition change_set status to approved
	//
	// CASE reject_change_set:
	//   INSERT change_set_rejection row
	//   transition change_set status to rejected
	//
	// CASE cancel_change_set:
	//   transition change_set status to cancelled
	//
	// CASE mark_change_set_applied:
	//   verify all field changes have applied_status=applied
	//   transition change_set status to applied, set applied_time
	//
	// CASE get_entity / search / etc. (reads):
	//   delegate to operations/read.go (already executed before gate for reads)
	//
	// TODO: set ctx.ExecutionResult with affected row IDs
	return nil
}


// === opsdb-api/gate/step_response.go ===
package gate

// StepResponse is gate step 10: Response Construction.
// Assembles the API response from GateContext. Always runs.
func StepResponse(ctx *GateContext) *GateResponse {
	// TODO: if ctx.Rejected:
	//   return GateResponse{
	//     Success: false,
	//     Error: ctx.RejectionError,
	//     AuditEntryID: ctx.AuditEntryID,
	//   }
	//
	// TODO: if success:
	//   return GateResponse{
	//     Success: true,
	//     Data: ctx.ExecutionResult (or read result),
	//     AuditEntryID: ctx.AuditEntryID,
	//     Warnings: ctx.Warnings,
	//     Metadata: {
	//       affected_row_ids: from execution,
	//       computed_approvals: from CM routing (for change set submissions),
	//       version_info: from versioning step,
	//     },
	//   }
	return nil
}


// === opsdb-api/operations/read.go ===
package operations

// GetEntity fetches one entity row by primary key.
// Returns current state with all fields the caller is authorized to see.
func GetEntity(entityType string, entityID int, authzResult interface{}) (interface{}, error) {
	// TODO: SELECT * FROM {entityType} WHERE id = {entityID}
	// TODO: apply field omissions from authzResult.OmittedFields
	// TODO: return structured row data with version stamp and governance metadata
	return nil, nil
}

// GetEntityHistory fetches the version chain for one entity.
// Returns current state plus all prior versions, ordered by version_serial.
func GetEntityHistory(entityType string, entityID int, timeRange interface{}) (interface{}, error) {
	// TODO: SELECT * FROM {entityType}_version WHERE {entityType}_id = {entityID}
	//       ORDER BY version_serial DESC
	// TODO: optionally filter by time range on approved_for_production_time
	// TODO: return ordered version chain
	return nil, nil
}

// GetEntityAtTime reconstructs field values active at a specific timestamp.
// Single lookup against version sibling — O(1) because versions contain full state.
func GetEntityAtTime(entityType string, entityID int, timestamp interface{}) (interface{}, error) {
	// TODO: SELECT * FROM {entityType}_version
	//       WHERE {entityType}_id = {entityID}
	//       AND approved_for_production_time <= {timestamp}
	//       ORDER BY approved_for_production_time DESC LIMIT 1
	// TODO: return reconstructed row at that point in time
	return nil, nil
}

// Search is the discovery surface across entity types.
// Accepts filters, joins, projection, ordering, pagination, freshness, view mode.
func Search(params *SearchParams) (*SearchResult, error) {
	// TODO: build SQL query from params:
	//   WHERE clause from filter predicates (equality, inequality, comparison, IN, LIKE anchored, IS NULL, BETWEEN, JSON containment)
	//   JOIN clause from named join paths (registered in _schema_relationship)
	//   SELECT clause from projection mode (standard, summary, full_with_history, explicit fields)
	//   ORDER BY from ordering params (field + direction pairs, tie-break by id)
	//   cursor or offset pagination
	// TODO: apply bounds: max result size, max join depth, max query time, max predicate depth
	//   reject if bounds exceeded
	// TODO: apply freshness filter for observation cache rows (max_staleness_seconds)
	// TODO: apply field omissions from authorization
	// TODO: return SearchResult with rows, cursor, count, freshness summary, filtering disclosures
	return nil, nil
}

// SearchParams holds search operation parameters.
type SearchParams struct {
	EntityType    string
	Filters       []FilterPredicate
	Joins         []string
	Projection    string // standard, summary, full_with_history, or field list
	Ordering      []OrderSpec
	Cursor        string
	Offset        int
	Limit         int
	MaxStaleness  int // seconds, for observation cache
	ViewMode      string // standard, with_history, at_time
}

// FilterPredicate represents one filter condition.
type FilterPredicate struct {
	Field    string
	Operator string // eq, ne, gt, gte, lt, lte, in, like, is_null, is_not_null, between, json_contains
	Value    interface{}
}

// OrderSpec represents one ordering directive.
type OrderSpec struct {
	Field     string
	Direction string // asc, desc
}

// SearchResult holds search results.
type SearchResult struct {
	Rows              []map[string]interface{}
	Cursor            string
	TotalCount        int
	FreshnessSummary  map[string]interface{}
	FilterDisclosures []string
}

// GetDependencies walks the substrate hierarchy or service connection graph.
// Used for stack-walking queries (decommission awareness, failure domain analysis, etc.).
func GetDependencies(startEntity string, startID int, pattern string, maxDepth int) (interface{}, error) {
	// TODO: switch on pattern:
	//   "substrate_parent_chain": recursive CTE walking megavisor_instance.parent_megavisor_instance_id
	//   "service_connections": walk service_connection rows from source service
	//   "location_ancestry": walk location.parent_location_id up to root
	//   "host_group_machines": walk host_group → host_group_machine → machine
	// TODO: enforce maxDepth and cycle detection
	// TODO: return dependency chain as ordered list of (entity_type, entity_id, depth, metadata)
	return nil, nil
}


// === opsdb-api/operations/write_observation.go ===
package operations

// WriteObservation handles the write_observation operation.
// Runner writes to observation cache tables, runner_job_output_var, or evidence_record.
// Validates report key before writing.
func WriteObservation(params *WriteObservationParams) (*WriteResult, error) {
	// TODO: report key enforcement is done by reportkeys.Enforce() called from gate step
	//       (fail-fast before this function is called)
	//
	// TODO: switch on params.TargetTable:
	//   observation_cache_metric:
	//     UPSERT keyed by (authority_id, hostname, metric_key)
	//     set metric_value, metric_data_json, _observed_time, _puller_runner_job_id
	//   observation_cache_state:
	//     UPSERT keyed by (entity_type, entity_id, state_key)
	//     set state_value, state_data_json, _observed_time, _puller_runner_job_id
	//   observation_cache_config:
	//     UPSERT keyed by (authority_id, hostname, config_key)
	//     set config_value, config_data_json, _observed_time, _puller_runner_job_id
	//   runner_job_output_var:
	//     INSERT (runner_job_id, var_name, var_value, var_type)
	//   evidence_record:
	//     INSERT with all evidence fields
	//
	// TODO: return WriteResult with written row ID
	return nil, nil
}

// WriteObservationParams holds write_observation parameters.
type WriteObservationParams struct {
	TargetTable    string
	Key            string
	Value          interface{}
	DataJSON       map[string]interface{}
	RunnerJobID    int
	AuthorityID    int
	ObservedTime   interface{} // time.Time
}

// WriteResult holds the result of a write operation.
type WriteResult struct {
	RowID int
}


// === opsdb-api/operations/write_changeset.go ===
package operations

// SubmitChangeSet handles the submit_change_set operation.
// Creates change_set, change_set_field_change, and change_set_approval_required rows.
// Supports dry_run mode.
func SubmitChangeSet(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	// TODO: validate optimistic concurrency (each field change's version stamp vs current)
	//       via concurrency.ValidateVersionStamps()
	//       if stale: return stale_version error
	//
	// TODO: if params.DryRun:
	//   run full validation pipeline
	//   compute approval requirements
	//   return result without writing any rows
	//
	// TODO: INSERT change_set row (status=submitted)
	// TODO: INSERT change_set_field_change rows (one per field change, apply_order set)
	// TODO: run validation pipeline (schema, bound, semantic, policy, lint, dependency)
	//   on recoverable error: update change_set status to draft, return errors
	//   on unrecoverable error: update change_set status to rejected, return errors
	//   on success: update change_set status to pending_approval (or approved if auto)
	// TODO: INSERT change_set_approval_required rows from CM routing
	// TODO: return ChangeSetResult
	return nil, nil
}

// EmergencyApply handles the emergency_apply operation.
// Same as submit but with is_emergency=true and reduced approvals.
func EmergencyApply(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	// TODO: verify caller has emergency authority per policy
	// TODO: submit with is_emergency=true
	// TODO: INSERT change_set_emergency_review row with status=pending
	// TODO: approval requirements reduced per emergency path policy
	// TODO: return ChangeSetResult
	return nil, nil
}

// BulkSubmit handles the bulk_submit_change_set operation.
// Chunked validation and atomic submission.
func BulkSubmit(params *SubmitChangeSetParams) (*ChangeSetResult, error) {
	// TODO: set is_bulk=true on change_set
	// TODO: chunk field changes (default 1000 per chunk)
	// TODO: validate each chunk, providing interim feedback
	//   on any chunk failure: entire change_set fails
	// TODO: if all chunks valid: write all rows atomically
	// TODO: approval routing may produce bundle-level approval (not per-entity)
	// TODO: return ChangeSetResult
	return nil, nil
}

// SubmitChangeSetParams holds change set submission parameters.
type SubmitChangeSetParams struct {
	SiteID        int
	Name          string
	Description   string
	Reason        string
	FieldChanges  []FieldChange
	TicketRef     *int // authority_pointer_id
	IsEmergency   bool
	IsBulk        bool
	DryRun        bool
	ProposerUser  *int // ops_user_id
	ProposerJob   *int // runner_job_id
}

// FieldChange represents one field change in a change set submission.
type FieldChange struct {
	EntityType      string
	EntityID        int
	FieldName       string
	BeforeValue     interface{}
	AfterValue      interface{}
	ChangeType      string // create, update, delete
	VersionStamp    int    // version of entity this change was drafted against
}

// ChangeSetResult holds the result of a change set operation.
type ChangeSetResult struct {
	ChangeSetID       int
	Status            string
	ApprovalRequired  []interface{} // computed approval requirements
	ValidationErrors  []interface{} // if validation failed
	DryRunResult      interface{}   // populated only for dry_run
}


// === opsdb-api/operations/changeset_actions.go ===
package operations

// ApproveChangeSet records a stakeholder approval.
// Verifies caller is in a required approver group.
// Increments fulfilled count. May transition status to approved.
func ApproveChangeSet(changeSetID int, approverUserID int, comments string) error {
	// TODO: read change_set_approval_required rows for this change set
	// TODO: determine which requirement(s) the approver can fulfill
	//       (approver must be in ops_group_required_id for at least one requirement)
	// TODO: check SoD: approver != submitter (if policy requires)
	// TODO: INSERT change_set_approval row
	// TODO: UPDATE change_set_approval_required: increment fulfilled_count, set is_fulfilled if met
	// TODO: if ALL requirements now is_fulfilled=true:
	//       UPDATE change_set status to approved
	return nil
}

// RejectChangeSet records a stakeholder rejection.
// Verifies caller is in a required approver group.
// Transitions change set to rejected per rejection semantics.
func RejectChangeSet(changeSetID int, rejectorUserID int, reason string) error {
	// TODO: verify rejector is in a required approver group
	// TODO: INSERT change_set_rejection row
	// TODO: evaluate rejection_behavior from matching approval_rule:
	//       any_rejects_halts → immediately reject
	//       majority_rejects_halts → count rejections vs required, reject if majority
	//       all_must_reject → reject only if all approvers reject
	// TODO: if rejected: UPDATE change_set status to rejected
	return nil
}

// CancelChangeSet withdraws a change set.
// Available to original submitter or user with sufficient authority.
func CancelChangeSet(changeSetID int, cancellerUserID int) error {
	// TODO: read change_set to verify status is cancellable (draft, submitted, pending_approval)
	// TODO: verify canceller is submitter OR has cancel authority
	// TODO: UPDATE change_set status to cancelled
	return nil
}

// ApplyFieldChange applies one approved field change.
// Used by the change-set executor runner.
func ApplyFieldChange(changeSetID int, fieldChangeID int, executorID int) error {
	// TODO: verify change_set status is approved
	// TODO: verify field change applied_status is pending (not already applied)
	// TODO: verify caller has executor authority
	// TODO: read the field change (entity_type, entity_id, field_name, after_value)
	// TODO: UPDATE target entity row: SET field_name = after_value
	// TODO: INSERT {entity}_version row with full entity state after update
	//       (version_serial, parent version, change_set_id, is_active_version=true)
	// TODO: UPDATE previous version: is_active_version = false
	// TODO: UPDATE change_set_field_change: applied_status = applied
	// TODO: on failure: UPDATE applied_status = failed, set applied_error_text
	return nil
}

// MarkChangeSetApplied finalizes change set status after all field changes applied.
func MarkChangeSetApplied(changeSetID int, executorID int) error {
	// TODO: read all change_set_field_change rows for this change set
	// TODO: verify ALL have applied_status = applied
	//       if any are pending or failed: return error
	// TODO: UPDATE change_set status = applied, applied_time = NOW()
	return nil
}


// === opsdb-api/operations/resolve.go ===
package operations

// ResolveAuthorityPointer performs a where-is-X lookup.
// Returns authority connection details and locator. Does NOT fetch from authority.
func ResolveAuthorityPointer(pointerID int) (*ResolveResult, error) {
	// TODO: read authority_pointer row by ID
	// TODO: read parent authority row for connection details
	// TODO: return ResolveResult with:
	//   authority base_url
	//   authority_type
	//   pointer_type
	//   locator (path/identifier within authority)
	//   pointer_data_json
	//   last_verified_time
	return nil, nil
}

// ResolveResult holds the result of an authority pointer resolution.
type ResolveResult struct {
	AuthorityID       int
	AuthorityName     string
	AuthorityType     string
	BaseURL           string
	PointerType       string
	Locator           string
	PointerDataJSON   map[string]interface{}
	LastVerifiedTime  interface{} // time.Time or nil
}


// === opsdb-api/operations/watch.go ===
package operations

// Watch implements the streaming watch operation.
// Long-poll or WebSocket subscription to entity changes.
// On reconnect: fetches current state first, then streams from resume token
// (level-triggered backstop).
func Watch(params *WatchParams, callback func(event WatchEvent)) error {
	// TODO: if params.ResumeToken provided:
	//   validate token, extract last-seen state
	//   full list of matching entities to establish current state
	//   send SYNC event with current state
	//   then stream changes from the resume point
	//
	// TODO: if no resume token:
	//   full list of matching entities
	//   send initial SNAPSHOT events for each
	//   begin streaming changes
	//
	// TODO: change detection:
	//   poll updated_time on target entity type at configured interval
	//   OR listen on Postgres NOTIFY channel (if configured)
	//   for each change: send WatchEvent to callback
	//
	// TODO: generate opaque resume token encoding current position
	// TODO: handle client disconnect gracefully
	return nil
}

// WatchParams holds watch subscription parameters.
type WatchParams struct {
	EntityType  string
	Filters     []FilterPredicate
	ResumeToken string
}

// WatchEvent represents one change event in a watch stream.
type WatchEvent struct {
	Type       string                 // ADDED, MODIFIED, DELETED, SYNC
	EntityType string
	EntityID   int
	Data       map[string]interface{}
	Version    int
	Timestamp  interface{} // time.Time
}


// === opsdb-api/reportkeys/enforcer.go ===
package reportkeys

// Enforcer validates runner write_observation calls against declared report keys.
// Caches declarations per runner spec for fast lookups.
type Enforcer struct {
	// TODO: cache map[int]map[string][]ReportKey — runner_spec_id → target_table → declared keys
}

// ReportKey represents one declared report key for a runner.
type ReportKey struct {
	Key            string
	TargetTable    string
	ConstraintJSON map[string]interface{} // report_key_data_json constraints
}

// NewEnforcer creates a report key enforcer.
func NewEnforcer() *Enforcer {
	// TODO: initialize empty cache
	return nil
}

// Enforce validates a runner's observation write against declared report keys.
// Returns nil on pass, structured error on rejection.
// Fail-closed: undeclared keys are always rejected.
func (e *Enforcer) Enforce(runnerSpecID int, targetTable string, key string, value interface{}) error {
	// TODO: look up cached declarations for runner_spec_id + target_table
	// TODO: if not cached: call CacheDeclarations to load
	//
	// TODO: check submitted key is in declared set
	//   if not: return undeclared_report_key error with runner identity + submitted key
	//
	// TODO: find matching declaration
	// TODO: validate submitted value against report_key_data_json constraints:
	//   numeric range, enum membership, structural shape
	//   if invalid: return invalid_report_key_value error with detail
	//
	// TODO: return nil on pass
	return nil
}

// CacheDeclarations loads report key declarations for a runner spec from OpsDB.
func (e *Enforcer) CacheDeclarations(runnerSpecID int) error {
	// TODO: SELECT * FROM runner_report_key
	//       WHERE runner_spec_id = runnerSpecID AND is_active = true
	// TODO: group by report_target_table
	// TODO: store in cache keyed by runner_spec_id → target_table → []ReportKey
	return nil
}

// InvalidateCache clears cached declarations for a runner spec.
// Called when runner_report_key rows are modified.
func (e *Enforcer) InvalidateCache(runnerSpecID int) {
	// TODO: delete cache entry for runnerSpecID
}


// === opsdb-api/schema/runtime_schema.go ===
package schema

// RuntimeSchema holds the in-memory representation of the OpsDB schema
// loaded from _schema_* tables. Provides fast lookups for entity types,
// fields, constraints, and relationships. Refreshed when schema version changes.
type RuntimeSchema struct {
	// TODO: entityTypes map[string]*EntityTypeMeta keyed by table name
	// TODO: fields map[string]map[string]*FieldMeta keyed by entity → field name
	// TODO: relationships map[string][]RelationshipMeta keyed by entity name
	// TODO: currentVersionSerial int
	// TODO: lastRefreshed time.Time
}

// EntityTypeMeta holds metadata about one entity type.
type EntityTypeMeta struct {
	ID          int
	TableName   string
	Description string
	Introduced  int // schema version serial
	Deprecated  *int // nil if not deprecated
}

// FieldMeta holds metadata about one field.
type FieldMeta struct {
	ID              int
	EntityTypeID    int
	FieldName       string
	FieldType       string
	IsNullable      bool
	IsPrimaryKey    bool
	IsForeignKey    bool
	FKTargetEntity  string // empty if not FK
	DefaultValue    *string
	Constraints     map[string]interface{} // parsed from constraint_data_json
	Description     string
	Introduced      int
	Deprecated      *int
}

// RelationshipMeta holds metadata about one relationship.
type RelationshipMeta struct {
	SourceEntity   string
	SourceField    string
	TargetEntity   string
	Cardinality    string
	OnDeleteAction string
}

// LoadRuntimeSchema reads _schema_entity_type, _schema_field, _schema_relationship
// from the database and builds lookup maps.
func LoadRuntimeSchema(db interface{}) (*RuntimeSchema, error) {
	// TODO: SELECT * FROM _schema_version WHERE is_current = true
	//       get current version serial
	// TODO: SELECT * FROM _schema_entity_type
	//       build entityTypes map
	// TODO: SELECT * FROM _schema_field
	//       build fields map keyed by entity → field name
	//       parse constraint_data_json into structured constraints
	// TODO: SELECT * FROM _schema_relationship
	//       build relationships map
	// TODO: set currentVersionSerial and lastRefreshed
	return nil, nil
}

// Refresh checks if schema version has changed and reloads if so.
func (rs *RuntimeSchema) Refresh(db interface{}) error {
	// TODO: SELECT version_serial FROM _schema_version WHERE is_current = true
	// TODO: if different from rs.currentVersionSerial: call LoadRuntimeSchema
	// TODO: if same: no-op
	return nil
}

// GetEntityType looks up entity type metadata by table name.
func (rs *RuntimeSchema) GetEntityType(name string) (*EntityTypeMeta, bool) {
	// TODO: look up in entityTypes map
	return nil, false
}

// GetField looks up field metadata by entity type and field name.
func (rs *RuntimeSchema) GetField(entityType string, fieldName string) (*FieldMeta, bool) {
	// TODO: look up in fields[entityType][fieldName]
	return nil, false
}

// GetRelationships returns all relationships for an entity type.
func (rs *RuntimeSchema) GetRelationships(entityType string) []RelationshipMeta {
	// TODO: look up in relationships map
	return nil
}

// GetAllEntityTypes returns all registered entity type names.
func (rs *RuntimeSchema) GetAllEntityTypes() []string {
	// TODO: return sorted keys of entityTypes map
	return nil
}

// GetFieldsForEntity returns all fields for an entity type.
func (rs *RuntimeSchema) GetFieldsForEntity(entityType string) []*FieldMeta {
	// TODO: return all fields from fields[entityType]
	return nil
}

// IsVersioned checks if an entity type has a versioning sibling.
func (rs *RuntimeSchema) IsVersioned(entityType string) bool {
	// TODO: check if {entityType}_version exists in entityTypes map
	return false
}


// === opsdb-api/concurrency/optimistic.go ===
package concurrency

// ValidateVersionStamps checks each field change's version stamp against
// the current version of the target entity. Returns stale_version error
// with details of which entities are stale.
func ValidateVersionStamps(fieldChanges []FieldChangeStamp, db interface{}) error {
	// TODO: for each unique (entity_type, entity_id) in fieldChanges:
	//   read current version_serial from {entity_type}_version
	//     WHERE {entity_type}_id = entity_id AND is_active_version = true
	//   compare against field change's VersionStamp
	//   if current > drafted-against: add to stale list
	//
	// TODO: if stale list non-empty:
	//   return StaleVersionError with list of (entity_type, entity_id,
	//     drafted_version, current_version) for each stale entity
	//
	// TODO: return nil if all stamps current
	return nil
}

// FieldChangeStamp holds the minimum info needed for version stamp validation.
type FieldChangeStamp struct {
	EntityType   string
	EntityID     int
	VersionStamp int // version_serial this change was drafted against
}

// StaleVersionError indicates one or more entities have advanced since drafting.
type StaleVersionError struct {
	StaleEntities []StaleEntity
}

func (e *StaleVersionError) Error() string {
	// TODO: format message listing stale entities
	return "stale_version"
}

// StaleEntity records one entity that is stale.
type StaleEntity struct {
	EntityType     string
	EntityID       int
	DraftedVersion int
	CurrentVersion int
}


// === opsdb-api/config/config.go ===
package config

// Config holds the API server configuration loaded from a DOS config.yaml.
type Config struct {
	SubstrateName    string
	SubstrateDesc    string
	SiteName         string
	DSN              string // resolved from environment variable
	ListenAddress    string
	TLSCertPath      string
	TLSKeyPath       string
	AuthBackend      string // yaml, oidc, service_account
	AuthConfigPath   string
	SchemaRepoPath   string
}

// LoadConfig reads a DOS config.yaml and resolves all values.
func LoadConfig(path string) (*Config, error) {
	// TODO: read YAML file at path
	// TODO: parse substrate section: name, description, site_name
	// TODO: parse database section: dsn_env_var → os.Getenv to get actual DSN
	//   error if env var not set or empty
	// TODO: parse api section: listen_address, tls_cert_path, tls_key_path,
	//   auth_backend, auth_config_path
	// TODO: parse schema section: repo_path (resolve relative to config file location)
	// TODO: validate required fields present
	// TODO: return Config
	return nil, nil
}
