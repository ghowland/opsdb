package pg

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool connection pool with OpsDB-specific helpers.
type DB struct {
	Pool *pgxpool.Pool
	dsn  string
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
// Used when the DOS config specifies pool sizing.
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

// Close closes the connection pool. All acquired connections are released.
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

// DSN returns the connection string this pool was created with.
// Used for logging and diagnostics — never log the password portion.
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

// validateDSNFormat performs basic format validation on a DSN string.
// Accepts both URI format (postgres://...) and keyword/value format (host=... dbname=...).
func validateDSNFormat(dsn string) error {
	dsn = strings.TrimSpace(dsn)

	// URI format: postgres:// or postgresql://
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if len(dsn) < 15 {
			return fmt.Errorf("DSN too short to be valid")
		}
		return nil
	}

	// Keyword/value format: must contain at least host= or dbname=
	lower := strings.ToLower(dsn)
	if strings.Contains(lower, "host=") || strings.Contains(lower, "dbname=") {
		return nil
	}

	return fmt.Errorf("DSN does not start with postgres:// and does not contain host= or dbname= keywords")
}

// redactDSN removes the password from a DSN for safe logging.
func redactDSN(dsn string) string {
	// URI format: mask between second : and @
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

	// Keyword/value format: mask password= value
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
