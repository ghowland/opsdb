//# tools/opsdb_api/auth/provider.go

package auth

// Provider is the interface all auth backends implement. The API gate
// calls Authenticate on every request to resolve caller identity. The
// gate does not care which backend is in use — YAML, OIDC, or service
// account all return the same Identity struct through the same interface.
type Provider interface {
	// Authenticate validates the credentials and returns a resolved
	// identity. Returns error if credentials are invalid, missing, or
	// the identity cannot be resolved to an ops_user or runner_machine.
	Authenticate(creds Credentials) (*Identity, error)

	// RefreshToken refreshes an expiring token and returns the updated
	// identity. Not all backends support this — YAML returns an error,
	// OIDC directs clients to refresh with the IdP directly.
	RefreshToken(token string) (*Identity, error)

	// Type returns the backend type name: "yaml", "oidc", "serviceaccount".
	Type() string
}

// Credentials carries the raw authentication material from an API request.
// Different auth backends consume different fields — YAML uses BasicUser
// and BasicPassword, OIDC uses BearerToken or OIDCToken, service accounts
// use BearerToken. The gate passes the full struct; the backend reads
// what it needs.
type Credentials struct {
	BasicUser     string
	BasicPassword string
	BearerToken   string
	OIDCToken     string
}

// HasBasicAuth returns true if basic auth credentials are present.
func (c Credentials) HasBasicAuth() bool {
	return c.BasicUser != ""
}

// HasBearerToken returns true if a bearer token or OIDC token is present.
func (c Credentials) HasBearerToken() bool {
	return c.BearerToken != "" || c.OIDCToken != ""
}

// Identity represents a resolved caller identity. Returned by all auth
// backends. Consumed by the gate for authorization, audit attribution,
// and change management routing.
//
// Human callers have OpsUserID set. Runner callers have RunnerMachineID
// and RunnerSpecID set. Web-application-mediated writes have both
// OpsUserID (the originating human) and RunnerMachineID (the runner
// that performed the call) set — the audit trail preserves both.
//
// Pointer fields are nil when not applicable: a human authenticated via
// YAML has no RunnerMachineID; a runner authenticated via service account
// has no OpsUserID (unless acting on behalf of a human).
type Identity struct {
	OpsUserID       *int
	RunnerMachineID *int
	RunnerSpecID    *int
	Username        string
	Roles           []string
	Groups          []string
	AuthMethod      string // "yaml", "oidc", "serviceaccount"
}

// IsHuman returns true if the identity has an ops_user_id, indicating
// a human caller (or a runner acting on behalf of a human).
func (id *Identity) IsHuman() bool {
	return id.OpsUserID != nil
}

// IsRunner returns true if the identity has a runner_machine_id,
// indicating a runner caller.
func (id *Identity) IsRunner() bool {
	return id.RunnerMachineID != nil
}

// HasRole returns true if the identity has the named role.
func (id *Identity) HasRole(role string) bool {
	for _, r := range id.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasGroup returns true if the identity is a member of the named group.
func (id *Identity) HasGroup(group string) bool {
	for _, g := range id.Groups {
		if g == group {
			return true
		}
	}
	return false
}
