
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

