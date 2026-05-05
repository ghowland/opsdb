
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

