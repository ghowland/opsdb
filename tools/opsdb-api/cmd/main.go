//# tools/opsdb-api/cmd/main.go

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
	"github.com/ghowland/opsdb/tools/opsdb-api/auth"
	"github.com/ghowland/opsdb/tools/opsdb-api/config"
	"github.com/ghowland/opsdb/tools/opsdb-api/gate"
	"github.com/ghowland/opsdb/tools/opsdb-api/operations"
	"github.com/ghowland/opsdb/tools/opsdb-api/reportkeys"
	runtimeschema "github.com/ghowland/opsdb/tools/opsdb-api/schema"
)

func main() {
	configPath := flag.String("config", "", "path to DOS config.yaml")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "error: --config flag is required\n")
		os.Exit(2)
	}

	// load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load config: %v\n", err)
		os.Exit(2)
	}

	// connect to database
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

	// load runtime schema from _schema_* tables
	rtSchema, err := runtimeschema.LoadRuntimeSchema(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load runtime schema: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "runtime schema loaded: %d entity types\n", rtSchema.EntityCount())

	// initialize auth provider
	authProvider, err := newAuthProvider(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize auth provider: %v\n", err)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stdout, "auth provider initialized: %s\n", authProvider.Type())

	// initialize report key enforcer
	reportKeyEnforcer := reportkeys.NewEnforcer(db)

	// initialize gate pipeline
	gatePipeline := gate.NewGate(db, rtSchema, authProvider, reportKeyEnforcer)

	// initialize operation handlers
	ops := operations.NewHandlers(db, rtSchema, gatePipeline)

	// register HTTP routes
	mux := http.NewServeMux()
	registerRoutes(mux, ops, gatePipeline)

	// build HTTP server
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}

	// configure TLS if cert and key paths are provided
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

	// start schema refresh goroutine — reloads runtime schema when
	// _schema_version changes, so the API picks up schema applies
	// without restart
	schemaRefreshCtx, schemaRefreshCancel := context.WithCancel(context.Background())
	defer schemaRefreshCancel()
	go refreshSchemaLoop(schemaRefreshCtx, db, rtSchema)

	// start server in goroutine so we can handle shutdown signals
	serverErr := make(chan error, 1)
	go func() {
		if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
			fmt.Fprintf(os.Stdout, "opsdb-api listening on %s (TLS)\n", cfg.ListenAddress)
			serverErr <- server.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath)
		} else {
			fmt.Fprintf(os.Stdout, "opsdb-api listening on %s\n", cfg.ListenAddress)
			serverErr <- server.ListenAndServe()
		}
	}()

	// block on shutdown signal or server error
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

	// graceful shutdown: stop accepting new connections, drain in-flight
	schemaRefreshCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "graceful shutdown failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "opsdb-api stopped\n")
	os.Exit(0)
}

// newAuthProvider creates the appropriate auth provider based on config.
func newAuthProvider(cfg *config.Config) (auth.Provider, error) {
	switch cfg.AuthBackend {
	case "yaml":
		return auth.NewYAMLProvider(cfg.AuthConfigPath)
	case "oidc":
		return auth.NewOIDCProvider(cfg.OIDCIssuer, cfg.OIDCClientID, cfg.OIDCAudience)
	case "serviceaccount":
		return auth.NewServiceAccountProvider(cfg.AuthConfigPath)
	default:
		return nil, fmt.Errorf("unknown auth backend: %s", cfg.AuthBackend)
	}
}

// registerRoutes maps HTTP paths to the 16 API operations.
// All routes go through the gate pipeline which enforces the 10-step
// sequence uniformly on every request.
func registerRoutes(mux *http.ServeMux, ops *operations.Handlers, gatePipeline *gate.Gate) {
	// read operations
	mux.HandleFunc("/api/v1/entity/get", ops.GetEntity)
	mux.HandleFunc("/api/v1/entity/history", ops.GetEntityHistory)
	mux.HandleFunc("/api/v1/entity/at-time", ops.GetEntityAtTime)
	mux.HandleFunc("/api/v1/search", ops.Search)
	mux.HandleFunc("/api/v1/dependencies", ops.GetDependencies)
	mux.HandleFunc("/api/v1/authority/resolve", ops.ResolveAuthorityPointer)
	mux.HandleFunc("/api/v1/changeset/view", ops.ChangeSetView)

	// write operations — observation (direct write path)
	mux.HandleFunc("/api/v1/observation/write", ops.WriteObservation)

	// write operations — change set path
	mux.HandleFunc("/api/v1/changeset/submit", ops.SubmitChangeSet)
	mux.HandleFunc("/api/v1/changeset/bulk-submit", ops.BulkSubmitChangeSet)
	mux.HandleFunc("/api/v1/changeset/emergency-apply", ops.EmergencyApply)

	// change management actions
	mux.HandleFunc("/api/v1/changeset/approve", ops.ApproveChangeSet)
	mux.HandleFunc("/api/v1/changeset/reject", ops.RejectChangeSet)
	mux.HandleFunc("/api/v1/changeset/cancel", ops.CancelChangeSet)

	// executor operations (called by change-set-executor runner)
	mux.HandleFunc("/api/v1/changeset/apply-field-change", ops.ApplyFieldChange)
	mux.HandleFunc("/api/v1/changeset/mark-applied", ops.MarkApplied)

	// streaming
	mux.HandleFunc("/api/v1/watch", ops.Watch)

	// health and readiness
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		handleReadyz(w, r, gatePipeline)
	})
}

// handleHealthz is the liveness probe. Returns 200 if the process is running.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleReadyz is the readiness probe. Returns 200 if the API can serve
// requests: database reachable, runtime schema loaded, auth provider ready.
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

// refreshSchemaLoop periodically checks for schema version changes and
// reloads the runtime schema when a new version is detected. This lets
// the API pick up schema-executor applies without a full restart.
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
				fmt.Fprintf(os.Stderr, "schema refresh failed: %v\n", err)
			}
		}
	}
}
