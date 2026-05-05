//# tools/opsdb-api/auth/provider.go

package auth

import "fmt"

// Identity represents a resolved caller identity after authentication.
// For human callers, OpsUserID is set. For runner callers, RunnerMachineID
// and RunnerSpecID are set. When a runner carries a human identity
// (web-mediated operations), both OpsUserID and runner fields are set.
type Identity struct {
	OpsUserID       *int
	RunnerMachineID *int
	RunnerSpecID    *int
	Username        string
	Roles           []string
	Groups          []string
	AuthMethod      string // yaml, oidc, service_account
	IsWebMediated   bool
}

// IsHuman returns true if the identity represents a human caller.
func (id *Identity) IsHuman() bool {
	return id.OpsUserID != nil
}

// IsRunner returns true if the identity represents a runner caller.
func (id *Identity) IsRunner() bool {
	return id.RunnerMachineID != nil
}

// HasRole checks whether the identity holds the named role.
func (id *Identity) HasRole(role string) bool {
	for _, r := range id.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasGroup checks whether the identity is a member of the named group.
func (id *Identity) HasGroup(group string) bool {
	for _, g := range id.Groups {
		if g == group {
			return true
		}
	}
	return false
}

// Credentials represents raw credentials extracted from an API request.
// Exactly one credential field should be populated per request.
type Credentials struct {
	BearerToken   string
	BasicUser     string
	BasicPassword string
	SAMLAssertion string
	OIDCToken     string
	ClientIP      string
	UserAgent     string
}

// HasBasicAuth returns true if basic auth credentials are present.
func (c *Credentials) HasBasicAuth() bool {
	return c.BasicUser != "" && c.BasicPassword != ""
}

// HasBearerToken returns true if a bearer token is present.
func (c *Credentials) HasBearerToken() bool {
	return c.BearerToken != ""
}

// HasOIDCToken returns true if an OIDC token is present.
func (c *Credentials) HasOIDCToken() bool {
	return c.OIDCToken != ""
}

// HasSAMLAssertion returns true if a SAML assertion is present.
func (c *Credentials) HasSAMLAssertion() bool {
	return c.SAMLAssertion != ""
}

// Provider is the interface all auth backends implement.
// The API gate calls Authenticate on every request as step 1.
// The resolved Identity flows through all subsequent gate steps.
type Provider interface {
	// Authenticate validates credentials and returns a resolved identity.
	// Returns error on invalid, expired, or unresolvable credentials.
	Authenticate(creds Credentials) (*Identity, error)

	// RefreshToken refreshes an expiring token and returns updated identity.
	// Not all providers support refresh; returns error if unsupported.
	RefreshToken(token string) (*Identity, error)

	// Type returns the provider type name: "yaml", "oidc", "service_account".
	Type() string
}

// NewProvider creates an auth provider based on configuration.
// Routes to the appropriate provider constructor.
func NewProvider(providerType string, configPath string) (Provider, error) {
	switch providerType {
	case "yaml":
		return NewYAMLProvider(configPath)
	case "oidc":
		return NewOIDCProvider(configPath)
	case "service_account":
		return NewServiceAccountProvider(configPath)
	default:
		return nil, fmt.Errorf("unknown auth provider type: %q (supported: yaml, oidc, service_account)", providerType)
	}
}

// NewOIDCProvider creates an OIDC auth provider from a config file path.
// The config file contains issuer, client_id, and audience.
func NewOIDCProvider(configPath string) (Provider, error) {
	cfg, err := loadOIDCConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OIDC config from %s: %w", configPath, err)
	}
	return newOIDCProviderFromConfig(cfg)
}

// NewServiceAccountProvider creates a service account auth provider
// from a config file path. Used for runner authentication.
func NewServiceAccountProvider(configPath string) (Provider, error) {
	cfg, err := loadServiceAccountConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load service account config from %s: %w", configPath, err)
	}
	return newServiceAccountProviderFromConfig(cfg)
}

// loadOIDCConfig reads OIDC provider configuration from a YAML file.
func loadOIDCConfig(path string) (*OIDCConfig, error) {
	// delegates to oidc_provider.go for actual parsing
	return parseOIDCConfigFile(path)
}

// loadServiceAccountConfig reads service account provider configuration
// from a YAML file.
func loadServiceAccountConfig(path string) (*ServiceAccountConfig, error) {
	// delegates to serviceaccount_provider.go for actual parsing
	return parseServiceAccountConfigFile(path)
}