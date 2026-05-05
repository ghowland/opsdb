
// === internal/pg/conn.go ===
package pg

import "database/sql"

// DB wraps a sql.DB connection pool with OpsDB-specific helpers.
type DB struct {
	pool *sql.DB
	dsn  string
}

// Connect opens a connection pool to Postgres and validates connectivity.
func Connect(dsn string) (*DB, error) {
	// TODO: sql.Open("postgres", dsn)
	// TODO: pool.Ping() to validate
	// TODO: set pool configuration (max open, max idle, lifetime)
	// TODO: return wrapped DB
	return nil, nil
}

// Close closes the connection pool.
func (db *DB) Close() error {
	// TODO: db.pool.Close()
	return nil
}

// Ping tests connection liveness.
func (db *DB) Ping() error {
	// TODO: db.pool.Ping()
	return nil
}

// Pool returns the underlying sql.DB for use by transaction helpers.
func (db *DB) Pool() *sql.DB {
	return db.pool
}

// DSNFromEnv reads a DSN from the named environment variable and validates format.
func DSNFromEnv(envVar string) (string, error) {
	// TODO: os.Getenv(envVar)
	// TODO: check non-empty
	// TODO: basic format validation (starts with postgres://)
	// TODO: return DSN string
	return "", nil
}

