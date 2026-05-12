//# internal/pg/conn.go

package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool connection pool with OpsDB-specific helpers.
type DB struct {
	Pool *pgxpool.Pool
	dsn  string
}

// Row wraps a pgx.Row so callers outside this package don't import pgx directly.
type Row struct {
	row pgx.Row
}

// Scan reads columns from the row into dest values.
func (r *Row) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

// Rows wraps pgx.Rows with a Columns() method that returns []string,
// which scanRowToMap and other generic row-reading code needs.
type Rows struct {
	rows pgx.Rows
}

// Next advances to the next row. Returns false when no more rows.
func (r *Rows) Next() bool {
	return r.rows.Next()
}

// Scan reads columns from the current row into dest values.
func (r *Rows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

// Close releases the rows back to the pool.
func (r *Rows) Close() {
	r.rows.Close()
}

// Err returns any error encountered during iteration.
func (r *Rows) Err() error {
	return r.rows.Err()
}

// Columns returns the column names for the result set.
func (r *Rows) Columns() ([]string, error) {
	descs := r.rows.FieldDescriptions()
	names := make([]string, len(descs))
	for i, d := range descs {
		names[i] = d.Name
	}
	return names, nil
}

// DefaultMaxConns is the default maximum number of connections in the pool.
const DefaultMaxConns = 25

// DefaultMinConns is the default minimum number of idle connections maintained.
const DefaultMinConns = 2

// DefaultMaxConnLifetime is the default maximum lifetime of a connection
// before it is closed and replaced.
const DefaultMaxConnLifetime = 5 * time.Minute

// DefaultMaxConnIdleTime is the default maximum time a connection can sit
// idle before it is closed.
const DefaultMaxConnIdleTime = 1 * time.Minute

// Connect opens a connection pool to Postgres and validates connectivity
// with a ping. Configures pool size and connection lifetimes.
func Connect(dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("empty DSN provided")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing DSN: %w", err)
	}

	config.MaxConns = int32(DefaultMaxConns)
	config.MinConns = int32(DefaultMinConns)
	config.MaxConnLifetime = DefaultMaxConnLifetime
	config.MaxConnIdleTime = DefaultMaxConnIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{Pool: pool, dsn: dsn}, nil
}

// ConnectWithPoolSize opens a connection pool with a specific max connection count.
func ConnectWithPoolSize(dsn string, maxConns int, minConns int, maxLifetime time.Duration) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("empty DSN provided")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing DSN: %w", err)
	}

	if maxConns > 0 {
		config.MaxConns = int32(maxConns)
	} else {
		config.MaxConns = int32(DefaultMaxConns)
	}
	if minConns > 0 {
		config.MinConns = int32(minConns)
	} else {
		config.MinConns = int32(DefaultMinConns)
	}
	if maxLifetime > 0 {
		config.MaxConnLifetime = maxLifetime
	} else {
		config.MaxConnLifetime = DefaultMaxConnLifetime
	}
	config.MaxConnIdleTime = DefaultMaxConnIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{Pool: pool, dsn: dsn}, nil
}

// Close closes the connection pool.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// Ping tests connection liveness against the database.
func (db *DB) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.Pool.Ping(ctx)
}

// Query executes a query that returns rows.
func (db *DB) Query(query string, args ...interface{}) (*Rows, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{rows: rows}, nil
}

// QueryRow executes a query expected to return at most one row.
func (db *DB) QueryRow(query string, args ...interface{}) *Row {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return &Row{row: db.Pool.QueryRow(ctx, query, args...)}
}

// QueryRows is an alias for Query. Some call sites use this name.
func (db *DB) QueryRows(query string, args ...interface{}) (*Rows, error) {
	return db.Query(query, args...)
}

// DSN returns the connection string this pool was created with, redacted.
func (db *DB) DSN() string {
	return redactDSN(db.dsn)
}

// DSNFromEnv reads a DSN from the named environment variable, validates
// that it is non-empty and has a recognizable Postgres prefix.
func DSNFromEnv(envVar string) (string, error) {
	if envVar == "" {
		return "", fmt.Errorf("empty environment variable name")
	}

	dsn := os.Getenv(envVar)
	if dsn == "" {
		return "", fmt.Errorf("environment variable %s is not set or empty", envVar)
	}

	if err := validateDSNFormat(dsn); err != nil {
		return "", fmt.Errorf("environment variable %s: %w", envVar, err)
	}

	return dsn, nil
}

// ---------------------------------------------------------------------------
// Error classification helpers
// ---------------------------------------------------------------------------

// IsNoRows returns true if the error indicates no rows were returned.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IsUndefinedTable returns true if the error is Postgres code 42P01
// (undefined_table).
func IsUndefinedTable(err error) bool {
	return isPgErrorCode(err, "42P01")
}

// IsUndefinedColumn returns true if the error is Postgres code 42703
// (undefined_column).
func IsUndefinedColumn(err error) bool {
	return isPgErrorCode(err, "42703")
}

// isPgErrorCode checks whether the error chain contains a pgconn.PgError
// with the given SQLSTATE code.
func isPgErrorCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	return false
}

// ---------------------------------------------------------------------------
// JSON helpers — thin wrappers so callers don't import encoding/json
// ---------------------------------------------------------------------------

// MarshalJSON serializes a value to JSON bytes.
func MarshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalJSON deserializes JSON bytes into a value.
func UnmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// ---------------------------------------------------------------------------
// DSN format helpers
// ---------------------------------------------------------------------------

func validateDSNFormat(dsn string) error {
	dsn = strings.TrimSpace(dsn)

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if len(dsn) < 15 {
			return fmt.Errorf("DSN too short to be valid")
		}
		return nil
	}

	lower := strings.ToLower(dsn)
	if strings.Contains(lower, "host=") || strings.Contains(lower, "dbname=") {
		return nil
	}

	return fmt.Errorf("DSN does not start with postgres:// and does not contain host= or dbname= keywords")
}

func redactDSN(dsn string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		atIdx := strings.Index(dsn, "@")
		if atIdx == -1 {
			return dsn
		}
		schemeEnd := strings.Index(dsn, "://") + 3
		userInfo := dsn[schemeEnd:atIdx]
		colonIdx := strings.Index(userInfo, ":")
		if colonIdx == -1 {
			return dsn
		}
		return dsn[:schemeEnd] + userInfo[:colonIdx] + ":***@" + dsn[atIdx+1:]
	}

	lower := strings.ToLower(dsn)
	pwIdx := strings.Index(lower, "password=")
	if pwIdx == -1 {
		return dsn
	}
	afterPw := pwIdx + len("password=")
	endIdx := strings.IndexByte(dsn[afterPw:], ' ')
	if endIdx == -1 {
		return dsn[:afterPw] + "***"
	}
	return dsn[:afterPw] + "***" + dsn[afterPw+endIdx:]
}
