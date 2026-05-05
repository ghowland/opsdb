
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

