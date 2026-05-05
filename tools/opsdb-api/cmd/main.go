
// === opsdb-api/cmd/main.go ===
package main

import "os"

// main is the CLI entrypoint for the opsdb-api binary.
// Loads configuration, initializes auth provider, connects to database,
// loads runtime schema, starts HTTP server.
func main() {
	// TODO: parse --config flag for DOS config.yaml path
	// TODO: config.LoadConfig(path) to read DOS configuration
	// TODO: pg.Connect(dsn) to open database connection
	// TODO: schema.LoadRuntimeSchema(db) to read _schema_* tables
	// TODO: auth.NewProvider(config.AuthBackend, config.AuthConfigPath)
	// TODO: initialize gate pipeline with db, schema, auth provider
	// TODO: register HTTP handlers for all 16 API operations
	// TODO: configure TLS if cert/key paths provided
	// TODO: start HTTP server on config.ListenAddress
	// TODO: block on shutdown signal (SIGTERM, SIGINT)
	// TODO: graceful shutdown: stop accepting, drain in-flight, close db
	os.Exit(0)
}

