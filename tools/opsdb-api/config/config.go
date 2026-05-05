
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
