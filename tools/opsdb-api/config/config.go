//# tools/opsdb-api/config/config.go

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the API server configuration loaded from a DOS config.yaml.
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

// configFile mirrors the YAML structure of a DOS config.yaml.
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

// LoadConfig reads a DOS config.yaml, resolves environment variables
// and relative paths, validates required fields, and returns a Config.
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

	configDir := filepath.Dir(path)

	// resolve DSN from environment variable
	if file.Database.DSNEnvVar == "" {
		return nil, fmt.Errorf("config missing required field: database.dsn_env_var")
	}
	dsn := os.Getenv(file.Database.DSNEnvVar)
	if dsn == "" {
		return nil, fmt.Errorf("environment variable %s (from database.dsn_env_var) is not set or empty", file.Database.DSNEnvVar)
	}

	// validate required fields
	if file.Substrate.Name == "" {
		return nil, fmt.Errorf("config missing required field: substrate.name")
	}
	if file.API.ListenAddress == "" {
		return nil, fmt.Errorf("config missing required field: api.listen_address")
	}
	if file.API.AuthBackend == "" {
		return nil, fmt.Errorf("config missing required field: api.auth_backend")
	}

	switch file.API.AuthBackend {
	case "yaml", "oidc", "service_account":
		// valid
	default:
		return nil, fmt.Errorf("unsupported auth_backend: %s (supported: yaml, oidc, service_account)", file.API.AuthBackend)
	}

	// resolve relative paths against config file directory
	authConfigPath := resolvePath(configDir, file.API.AuthConfigPath)
	tlsCertPath := resolvePath(configDir, file.API.TLSCertPath)
	tlsKeyPath := resolvePath(configDir, file.API.TLSKeyPath)
	schemaRepoPath := resolvePath(configDir, file.Schema.RepoPath)

	// validate auth config path is set for backends that need it
	if file.API.AuthBackend == "yaml" && authConfigPath == "" {
		return nil, fmt.Errorf("auth_backend=yaml requires auth_config_path pointing to users.yaml")
	}
	if file.API.AuthBackend == "service_account" && authConfigPath == "" {
		return nil, fmt.Errorf("auth_backend=service_account requires auth_config_path")
	}

	// validate OIDC fields when using OIDC backend
	if file.API.AuthBackend == "oidc" {
		if file.API.OIDCIssuer == "" {
			return nil, fmt.Errorf("auth_backend=oidc requires oidc_issuer")
		}
		if file.API.OIDCClientID == "" {
			return nil, fmt.Errorf("auth_backend=oidc requires oidc_client_id")
		}
	}

	// validate TLS: both cert and key must be present, or neither
	if (tlsCertPath == "") != (tlsKeyPath == "") {
		return nil, fmt.Errorf("tls_cert_path and tls_key_path must both be set or both be empty")
	}

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

// resolvePath resolves a path relative to a base directory.
// Returns empty string for empty input. Absolute paths pass through unchanged.
func resolvePath(baseDir string, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}
