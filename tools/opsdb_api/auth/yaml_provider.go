//# tools/opsdb-api/auth/yaml_provider.go

package auth

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

// YAMLProvider implements auth.Provider using a YAML file backend.
// Zero external dependencies beyond the file system. Used for bootstrap,
// development, and testing per ADR-005 (yaml-auth-bootstrap).
//
// The users.yaml file contains usernames, bcrypt password hashes,
// ops_user_id mappings, and role/group memberships. Each request carries
// basic auth credentials; the provider validates the password against
// the stored bcrypt hash and returns an Identity.
type YAMLProvider struct {
	filePath string
	users    map[string]*YAMLUser
	mu       sync.RWMutex
}

// YAMLUser represents a single user entry in users.yaml.
type YAMLUser struct {
	Username       string   `yaml:"username"`
	PasswordBcrypt string   `yaml:"password_bcrypt"`
	OpsUserID      int      `yaml:"ops_user_id"`
	Roles          []string `yaml:"roles"`
	Groups         []string `yaml:"groups"`
}

// yamlUsersFile is the top-level structure of users.yaml.
type yamlUsersFile struct {
	Users []YAMLUser `yaml:"users"`
}

// NewYAMLProvider loads users.yaml and returns a ready provider.
// Returns error if the file cannot be read, parsed, or contains
// invalid entries (missing fields, bad bcrypt hashes, duplicates).
func NewYAMLProvider(filePath string) (*YAMLProvider, error) {
	provider := &YAMLProvider{
		filePath: filePath,
		users:    make(map[string]*YAMLUser),
	}

	err := provider.load()
	if err != nil {
		return nil, fmt.Errorf("failed to load YAML auth provider from %s: %w", filePath, err)
	}

	return provider, nil
}

// Authenticate validates username and password against bcrypt hashes
// in the loaded users file. Returns an Identity on success with
// OpsUserID, Username, Roles, Groups, and AuthMethod="yaml".
func (p *YAMLProvider) Authenticate(creds Credentials) (*Identity, error) {
	if !creds.HasBasicAuth() {
		return nil, fmt.Errorf("YAML auth requires basic auth credentials (username and password)")
	}

	p.mu.RLock()
	user, exists := p.users[creds.BasicUser]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown user: %s", creds.BasicUser)
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordBcrypt), []byte(creds.BasicPassword))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials for user %s", creds.BasicUser)
	}

	opsUserID := user.OpsUserID

	identity := &Identity{
		OpsUserID:  &opsUserID,
		Username:   user.Username,
		Roles:      copyStrings(user.Roles),
		Groups:     copyStrings(user.Groups),
		AuthMethod: "yaml",
	}

	return identity, nil
}

// RefreshToken is not supported by the YAML provider. Basic auth does
// not use tokens; each request carries credentials directly.
func (p *YAMLProvider) RefreshToken(token string) (*Identity, error) {
	return nil, fmt.Errorf("token refresh is not supported by the YAML auth provider; " +
		"each request must include basic auth credentials")
}

// Type returns "yaml".
func (p *YAMLProvider) Type() string {
	return "yaml"
}

// Reload re-reads the users.yaml file. Can be called to pick up changes
// without restarting the API server. The swap is atomic under the write
// lock — concurrent Authenticate calls see either the old or new user
// map, never a partial state.
func (p *YAMLProvider) Reload() error {
	return p.load()
}

// UserCount returns the number of loaded users.
func (p *YAMLProvider) UserCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.users)
}

// load reads and parses the users.yaml file into the provider's user map.
// Validates every entry: username non-empty, password_bcrypt non-empty and
// well-formed, ops_user_id positive, no duplicate usernames. Builds the
// new map fully before swapping it under the write lock so that a parse
// error does not corrupt the existing state.
func (p *YAMLProvider) load() error {
	data, err := os.ReadFile(p.filePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", p.filePath, err)
	}

	var file yamlUsersFile
	err = yaml.Unmarshal(data, &file)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", p.filePath, err)
	}

	if len(file.Users) == 0 {
		return fmt.Errorf("users.yaml contains no user entries")
	}

	// Build the new user map. Validate every entry before swapping.
	users := make(map[string]*YAMLUser, len(file.Users))
	for i := range file.Users {
		user := &file.Users[i]

		if user.Username == "" {
			return fmt.Errorf("user entry %d missing required field: username", i)
		}

		if user.PasswordBcrypt == "" {
			return fmt.Errorf("user %s missing required field: password_bcrypt", user.Username)
		}

		if user.OpsUserID <= 0 {
			return fmt.Errorf("user %s missing or invalid ops_user_id (must be positive integer)", user.Username)
		}

		// Validate that the password hash is well-formed bcrypt. This
		// catches typos and truncation in the users.yaml file at load
		// time rather than at first authentication attempt.
		_, err := bcrypt.Cost([]byte(user.PasswordBcrypt))
		if err != nil {
			return fmt.Errorf("user %s has invalid bcrypt hash: %w", user.Username, err)
		}

		if _, exists := users[user.Username]; exists {
			return fmt.Errorf("duplicate username in users.yaml: %s", user.Username)
		}

		users[user.Username] = user
	}

	// Swap the user map under the write lock. Concurrent reads on the
	// old map complete before or after this swap, never during.
	p.mu.Lock()
	p.users = users
	p.mu.Unlock()

	return nil
}

// copyStrings returns a shallow copy of a string slice. Prevents the
// Identity from sharing backing array with the YAMLUser, so that a
// subsequent Reload does not mutate previously-returned identities.
func copyStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
