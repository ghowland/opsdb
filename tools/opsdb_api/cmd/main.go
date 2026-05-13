//# tools/opsdb_api/cmd/main.go

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/tools/opsdb_api/auth"
	"github.com/ghowland/opsdb/tools/opsdb_api/config"
	"github.com/ghowland/opsdb/tools/opsdb_api/gate"
	"github.com/ghowland/opsdb/tools/opsdb_api/operations"
	"github.com/ghowland/opsdb/tools/opsdb_api/reportkeys"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb_api/schema"
)

func main() {
	configPath := flag.String("config", "", "path to DOS config.yaml")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "error: --config flag is required\n")
		os.Exit(2)
	}

	// Load configuration from the DOS config.yaml for this substrate.
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load config: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "config loaded: substrate=%s site=%s\n", cfg.SubstrateName, cfg.SiteName)

	// Connect to Postgres. The DSN was resolved from an environment variable
	// by config.LoadConfig — the config file names the env var, not the DSN
	// itself, because the DSN is a secret.
	db, err := pg.Connect(cfg.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to connect to database: %v\n", err)
		os.Exit(2)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: database ping failed: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "database connection established\n")

	// Load the runtime schema from _schema_entity_type, _schema_field, and
	// _schema_relationship tables. This is the API's cached view of what
	// entities and fields exist — used by the gate for schema validation,
	// bound validation, and column filtering on writes.
	rtSchema, err := runtimeschema.LoadRuntimeSchema(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load runtime schema: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "runtime schema loaded: %d entity types\n", rtSchema.EntityCount())

	// Initialize the auth provider. For now only the YAML backend is
	// implemented. OIDC and service account providers will be added later;
	// their cases return clear errors so the failure is obvious.
	authProvider, err := newAuthProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize auth provider: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "auth provider initialized: %s\n", authProvider.Type())

	// Initialize the runner report key enforcer. This is consulted by the
	// gate during write_observation calls to verify the runner is authorized
	// to write the submitted key to the target observation table.
	reportKeyEnforcer := reportkeys.NewEnforcer(db)

	// Create the gate pipeline with all its dependencies. The gate is the
	// single entry point for every API operation — it runs the 10-step
	// sequence (auth, authz, schema validate, bound validate, policy,
	// versioning, change mgmt, audit, execute, response) uniformly on
	// every request.
	gatePipeline := gate.NewGate(db, rtSchema, authProvider, reportKeyEnforcer)

	// Create the operation handlers. These own HTTP request parsing and
	// response writing. Each handler constructs a GateRequest and delegates
	// to gatePipeline.ProcessRequest for the actual processing.
	ops := operations.NewHandlers(db, rtSchema, gatePipeline)

	// Register all HTTP routes.
	mux := http.NewServeMux()
	registerRoutes(mux, ops, gatePipeline)

	// Build the HTTP server with conservative timeouts.
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	// Configure TLS if both cert and key paths are provided. Config
	// validation already ensures both-or-neither.
	if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		server.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}
	}

	// Start a background goroutine that polls _schema_version every 30
	// seconds and reloads the runtime schema when a new version is detected.
	// This lets the API pick up schema_executor applies without a full
	// restart.
	schemaRefreshCtx, schemaRefreshCancel := context.WithCancel(context.Background())
	defer schemaRefreshCancel()
	go refreshSchemaLoop(schemaRefreshCtx, db, rtSchema)

	// Start the HTTP server in a goroutine so we can block on shutdown
	// signals in the main goroutine.
	serverErr := make(chan error, 1)
	go func() {
		if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
			fmt.Fprintf(os.Stdout, "opsdb_api listening on %s (TLS)\n", cfg.ListenAddress)
			serverErr <- server.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath)
		} else {
			fmt.Fprintf(os.Stdout, "opsdb_api listening on %s\n", cfg.ListenAddress)
			serverErr <- server.ListenAndServe()
		}
	}()

	// Block until we receive a shutdown signal or the server errors out.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stdout, "received signal %s, starting graceful shutdown\n", sig)
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}

	// Graceful shutdown: stop the schema refresh loop, then drain
	// in-flight HTTP requests with a 30-second timeout.
	schemaRefreshCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "graceful shutdown failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "opsdb_api stopped\n")
}

// newAuthProvider creates the appropriate auth provider based on the
// configured auth backend. Only YAML is implemented now; OIDC and
// service account return clear errors.
func newAuthProvider(cfg *config.Config) (auth.Provider, error) {
	switch cfg.AuthBackend {
	case "yaml":
		return auth.NewYAMLProvider(cfg.AuthConfigPath)
	case "oidc":
		return nil, fmt.Errorf("oidc auth backend is not yet implemented")
	case "serviceaccount", "service_account":
		return nil, fmt.Errorf("service_account auth backend is not yet implemented")
	default:
		return nil, fmt.Errorf("unknown auth backend: %s", cfg.AuthBackend)
	}
}

// registerRoutes maps HTTP paths to the 16 API operations plus health
// and readiness probes. All API routes go through the gate pipeline
// which enforces the 10-step sequence uniformly on every request.
func registerRoutes(mux *http.ServeMux, ops *operations.Handlers, gatePipeline *gate.Gate) {
	// Read operations
	mux.HandleFunc("/api/v1/entity/get", ops.GetEntity)
	mux.HandleFunc("/api/v1/entity/history", ops.GetEntityHistory)
	mux.HandleFunc("/api/v1/entity/at-time", ops.GetEntityAtTime)
	mux.HandleFunc("/api/v1/search", ops.Search)
	mux.HandleFunc("/api/v1/dependencies", ops.GetDependencies)
	mux.HandleFunc("/api/v1/authority/resolve", ops.ResolveAuthorityPointer)
	mux.HandleFunc("/api/v1/changeset/view", ops.ChangeSetView)

	// Write operations — observation (direct write path, no change management)
	mux.HandleFunc("/api/v1/observation/write", ops.WriteObservation)

	// Write operations — change set path
	mux.HandleFunc("/api/v1/changeset/submit", ops.SubmitChangeSet)
	mux.HandleFunc("/api/v1/changeset/bulk-submit", ops.BulkSubmitChangeSet)
	mux.HandleFunc("/api/v1/changeset/emergency-apply", ops.EmergencyApply)

	// Change management actions
	mux.HandleFunc("/api/v1/changeset/approve", ops.ApproveChangeSet)
	mux.HandleFunc("/api/v1/changeset/reject", ops.RejectChangeSet)
	mux.HandleFunc("/api/v1/changeset/cancel", ops.CancelChangeSet)

	// Executor operations (called by the change_set_executor runner)
	mux.HandleFunc("/api/v1/changeset/apply-field-change", ops.ApplyFieldChange)
	mux.HandleFunc("/api/v1/changeset/mark-applied", ops.MarkApplied)

	// Streaming
	mux.HandleFunc("/api/v1/watch", ops.Watch)

	// Health and readiness probes
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		handleReadyz(w, r, gatePipeline)
	})
}

// handleHealthz is the liveness probe. Returns 200 if the process is
// running. No dependency checks — if the process can serve HTTP, it's live.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleReadyz is the readiness probe. Returns 200 only if the API can
// actually serve requests: database is reachable, runtime schema is
// loaded with at least one entity type, and auth provider is configured.
func handleReadyz(w http.ResponseWriter, r *http.Request, g *gate.Gate) {
	w.Header().Set("Content-Type", "application/json")
	if !g.IsReady() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready"}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}

// refreshSchemaLoop polls for schema version changes every 30 seconds
// and reloads the runtime schema when a new version is detected. This
// lets the API pick up schema applies from the schema_executor runner
// without requiring a full restart.
func refreshSchemaLoop(ctx context.Context, db *pg.DB, rtSchema *runtimeschema.RuntimeSchema) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := rtSchema.Refresh(db)
			if err != nil {
				// Log the error but don't crash — the API continues serving
				// with the previously loaded schema. The next tick will retry.
				fmt.Fprintf(os.Stderr, "schema refresh failed: %v\n", err)
			}
		}
	}
}
