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

