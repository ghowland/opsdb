//# tools/opsdb_api/config/config.go

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the API server configuration loaded from a DOS config.yaml.
// All paths are resolved to absolute form. The DSN is resolved from the
// environment variable named in the config file — the config file never
// contains the DSN itself because the DSN is a secret.
type Config struct {
	SubstrateName  string
	SubstrateDesc  string
	SiteName       string
	DSN            string
	ListenAddress  string
	TLSCertPath    string
	TLSKeyPath     string
	AuthBackend    string // yaml, oidc, service_account
	AuthConfigPath string
	OIDCIssuer     string
	OIDCClientID   string
	OIDCAudience   string
	SchemaRepoPath string
}

// configFile mirrors the YAML structure of a DOS config.yaml. The YAML
// has four top-level sections: substrate identity, database connection,
// API server settings, and schema repo location.
type configFile struct {
	Substrate struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		SiteName    string `yaml:"site_name"`
	} `yaml:"substrate"`

	Database struct {
		DSNEnvVar string `yaml:"dsn_env_var"`
	} `yaml:"database"`

	API struct {
		ListenAddress  string `yaml:"listen_address"`
		TLSCertPath    string `yaml:"tls_cert_path"`
		TLSKeyPath     string `yaml:"tls_key_path"`
		AuthBackend    string `yaml:"auth_backend"`
		AuthConfigPath string `yaml:"auth_config_path"`
		OIDCIssuer     string `yaml:"oidc_issuer"`
		OIDCClientID   string `yaml:"oidc_client_id"`
		OIDCAudience   string `yaml:"oidc_audience"`
	} `yaml:"api"`

	Schema struct {
		RepoPath string `yaml:"repo_path"`
	} `yaml:"schema"`
}

// LoadConfig reads a DOS config.yaml, resolves environment variables and
// relative paths, validates required fields, and returns a Config ready
// for the API server to consume.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var file configFile
	err = yaml.Unmarshal(data, &file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// All relative paths in the config are resolved against the directory
	// that contains the config file itself.
	configDir := filepath.Dir(path)

	// --- Validate required fields ---

	if file.Substrate.Name == "" {
		return nil, fmt.Errorf("config missing required field: substrate.name")
	}

	if file.Database.DSNEnvVar == "" {
		return nil, fmt.Errorf("config missing required field: database.dsn_env_var")
	}

	if file.API.ListenAddress == "" {
		return nil, fmt.Errorf("config missing required field: api.listen_address")
	}

	if file.API.AuthBackend == "" {
		return nil, fmt.Errorf("config missing required field: api.auth_backend")
	}

	// --- Validate auth backend value ---

	switch file.API.AuthBackend {
	case "yaml", "oidc", "service_account":
		// valid
	default:
		return nil, fmt.Errorf("unsupported auth_backend: %s (supported: yaml, oidc, service_account)", file.API.AuthBackend)
	}

	// --- Resolve DSN from environment variable ---

	dsn := os.Getenv(file.Database.DSNEnvVar)
	if dsn == "" {
		return nil, fmt.Errorf("environment variable %s (from database.dsn_env_var) is not set or empty",
			file.Database.DSNEnvVar)
	}

	// --- Resolve relative paths against config file directory ---

	authConfigPath := resolvePath(configDir, file.API.AuthConfigPath)
	tlsCertPath := resolvePath(configDir, file.API.TLSCertPath)
	tlsKeyPath := resolvePath(configDir, file.API.TLSKeyPath)
	schemaRepoPath := resolvePath(configDir, file.Schema.RepoPath)

	// --- Validate backend-specific fields ---

	if file.API.AuthBackend == "yaml" && authConfigPath == "" {
		return nil, fmt.Errorf("auth_backend=yaml requires auth_config_path pointing to users.yaml")
	}

	if file.API.AuthBackend == "service_account" && authConfigPath == "" {
		return nil, fmt.Errorf("auth_backend=service_account requires auth_config_path")
	}

	if file.API.AuthBackend == "oidc" {
		if file.API.OIDCIssuer == "" {
			return nil, fmt.Errorf("auth_backend=oidc requires oidc_issuer")
		}
		if file.API.OIDCClientID == "" {
			return nil, fmt.Errorf("auth_backend=oidc requires oidc_client_id")
		}
	}

	// --- Validate TLS: both cert and key must be present, or neither ---

	if (tlsCertPath == "") != (tlsKeyPath == "") {
		return nil, fmt.Errorf("tls_cert_path and tls_key_path must both be set or both be empty")
	}

	// --- Build and return Config ---

	cfg := &Config{
		SubstrateName:  file.Substrate.Name,
		SubstrateDesc:  file.Substrate.Description,
		SiteName:       file.Substrate.SiteName,
		DSN:            dsn,
		ListenAddress:  file.API.ListenAddress,
		TLSCertPath:    tlsCertPath,
		TLSKeyPath:     tlsKeyPath,
		AuthBackend:    file.API.AuthBackend,
		AuthConfigPath: authConfigPath,
		OIDCIssuer:     file.API.OIDCIssuer,
		OIDCClientID:   file.API.OIDCClientID,
		OIDCAudience:   file.API.OIDCAudience,
		SchemaRepoPath: schemaRepoPath,
	}

	return cfg, nil
}

// resolvePath resolves a path relative to a base directory. Returns empty
// string for empty input. Absolute paths pass through unchanged. Relative
// paths are joined against baseDir to produce an absolute path.
func resolvePath(baseDir string, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}
