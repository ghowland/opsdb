//# internal/testutil/pg.go

package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDB holds connection information for a test Postgres instance.
// The instance is managed externally (by test-integration.sh or a manually
// started container). This package connects to it, not manages it.
type TestDB struct {
	DSN  string
	Pool *pgxpool.Pool
}

// DefaultTestDSN is the connection string for the test Postgres instance
// started by scripts/test-integration.sh.
const DefaultTestDSN = "postgres://opsdb_test:opsdb_test_pass@localhost:15432/opsdb_test?sslmode=disable"

// StartTestPostgres connects to a running test Postgres instance and
// validates connectivity. If the OPSDB_TEST_DSN environment variable is
// set, uses that; otherwise uses DefaultTestDSN.
//
// The caller is responsible for having a Postgres instance running.
// For integration tests, scripts/test-integration.sh handles this.
// For local development, run the container manually:
//
//	docker run -d --name opsdb-test-postgres \
//	  -e POSTGRES_USER=opsdb_test -e POSTGRES_PASSWORD=opsdb_test_pass \
//	  -e POSTGRES_DB=opsdb_test -p 15432:5432 postgres:16-alpine
func StartTestPostgres(t *testing.T) *TestDB {
	t.Helper()

	dsn := DefaultTestDSN
	if envDSN := os.Getenv("OPSDB_TEST_DSN"); envDSN != "" {
		dsn = envDSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("test postgres not available (start with scripts/test-integration.sh): %v", err)
		return nil
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test postgres not responding: %v", err)
		return nil
	}

	tdb := &TestDB{
		DSN:  dsn,
		Pool: pool,
	}

	t.Cleanup(func() {
		tdb.Pool.Close()
	})

	return tdb
}

// ResetTestDB drops all tables, sequences, and types in the public schema,
// returning the database to an empty state. Used between test cases for
// isolation. Preserves the public schema itself.
func ResetTestDB(t *testing.T, tdb *TestDB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Drop all tables in public schema.
	rows, err := tdb.Pool.Query(ctx,
		`SELECT tablename FROM pg_tables WHERE schemaname = 'public'`)
	if err != nil {
		t.Fatalf("querying tables for reset: %v", err)
	}

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning table name: %v", err)
		}
		tables = append(tables, name)
	}
	rows.Close()

	for _, table := range tables {
		_, err := tdb.Pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %q CASCADE", table))
		if err != nil {
			t.Fatalf("dropping table %s: %v", table, err)
		}
	}

	// Drop all sequences in public schema.
	seqRows, err := tdb.Pool.Query(ctx,
		`SELECT sequencename FROM pg_sequences WHERE schemaname = 'public'`)
	if err != nil {
		t.Fatalf("querying sequences for reset: %v", err)
	}

	var sequences []string
	for seqRows.Next() {
		var name string
		if err := seqRows.Scan(&name); err != nil {
			t.Fatalf("scanning sequence name: %v", err)
		}
		sequences = append(sequences, name)
	}
	seqRows.Close()

	for _, seq := range sequences {
		_, err := tdb.Pool.Exec(ctx, fmt.Sprintf("DROP SEQUENCE IF EXISTS %q CASCADE", seq))
		if err != nil {
			t.Fatalf("dropping sequence %s: %v", seq, err)
		}
	}

	// Drop all custom enum types in public schema.
	typeRows, err := tdb.Pool.Query(ctx,
		`SELECT typname FROM pg_type t
		 JOIN pg_namespace n ON t.typnamespace = n.oid
		 WHERE n.nspname = 'public' AND t.typtype = 'e'`)
	if err != nil {
		t.Fatalf("querying types for reset: %v", err)
	}

	var types []string
	for typeRows.Next() {
		var name string
		if err := typeRows.Scan(&name); err != nil {
			t.Fatalf("scanning type name: %v", err)
		}
		types = append(types, name)
	}
	typeRows.Close()

	for _, typ := range types {
		_, err := tdb.Pool.Exec(ctx, fmt.Sprintf("DROP TYPE IF EXISTS %q CASCADE", typ))
		if err != nil {
			t.Fatalf("dropping type %s: %v", typ, err)
		}
	}
}

// TableExists checks whether a table exists in the public schema.
func TableExists(t *testing.T, tdb *TestDB, tableName string) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool
	err := tdb.Pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`, tableName).Scan(&exists)
	if err != nil {
		t.Fatalf("checking table existence for %s: %v", tableName, err)
	}
	return exists
}

// ColumnExists checks whether a column exists on a table in the public schema.
func ColumnExists(t *testing.T, tdb *TestDB, tableName string, columnName string) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool
	err := tdb.Pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
		)`, tableName, columnName).Scan(&exists)
	if err != nil {
		t.Fatalf("checking column existence for %s.%s: %v", tableName, columnName, err)
	}
	return exists
}

// ColumnType returns the data type of a column as reported by information_schema.
func ColumnType(t *testing.T, tdb *TestDB, tableName string, columnName string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var dataType string
	err := tdb.Pool.QueryRow(ctx,
		`SELECT data_type FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`,
		tableName, columnName).Scan(&dataType)
	if err != nil {
		t.Fatalf("reading column type for %s.%s: %v", tableName, columnName, err)
	}
	return dataType
}

// ColumnIsNullable returns whether a column allows NULL values.
func ColumnIsNullable(t *testing.T, tdb *TestDB, tableName string, columnName string) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var isNullable string
	err := tdb.Pool.QueryRow(ctx,
		`SELECT is_nullable FROM information_schema.columns
		 WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`,
		tableName, columnName).Scan(&isNullable)
	if err != nil {
		t.Fatalf("reading nullable for %s.%s: %v", tableName, columnName, err)
	}
	return isNullable == "YES"
}

// TableCount returns the number of tables in the public schema.
func TableCount(t *testing.T, tdb *TestDB) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	err := tdb.Pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&count)
	if err != nil {
		t.Fatalf("counting tables: %v", err)
	}
	return count
}

// RowCount returns the number of rows in a table.
func RowCount(t *testing.T, tdb *TestDB, tableName string) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	query := fmt.Sprintf("SELECT count(*) FROM %q", tableName)
	err := tdb.Pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		t.Fatalf("counting rows in %s: %v", tableName, err)
	}
	return count
}

// HasConstraint checks whether a named constraint exists on a table.
func HasConstraint(t *testing.T, tdb *TestDB, tableName string, constraintName string) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool
	err := tdb.Pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.table_constraints
			WHERE table_schema = 'public' AND table_name = $1 AND constraint_name = $2
		)`, tableName, constraintName).Scan(&exists)
	if err != nil {
		t.Fatalf("checking constraint %s on %s: %v", constraintName, tableName, err)
	}
	return exists
}

// HasIndex checks whether a named index exists on a table.
func HasIndex(t *testing.T, tdb *TestDB, tableName string, indexName string) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool
	err := tdb.Pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = $1 AND indexname = $2
		)`, tableName, indexName).Scan(&exists)
	if err != nil {
		t.Fatalf("checking index %s on %s: %v", indexName, tableName, err)
	}
	return exists
}

// ExecSQL executes arbitrary SQL against the test database.
// Used for test setup and verification queries.
func ExecSQL(t *testing.T, tdb *TestDB, sql string, args ...interface{}) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := tdb.Pool.Exec(ctx, sql, args...)
	if err != nil {
		t.Fatalf("executing SQL: %v\nSQL: %s", err, sql)
	}
}

// QueryScalarInt executes a query and scans a single integer result.
func QueryScalarInt(t *testing.T, tdb *TestDB, query string, args ...interface{}) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result int
	err := tdb.Pool.QueryRow(ctx, query, args...).Scan(&result)
	if err != nil {
		t.Fatalf("scalar int query failed: %v\nQuery: %s", err, query)
	}
	return result
}

// QueryScalarString executes a query and scans a single string result.
func QueryScalarString(t *testing.T, tdb *TestDB, query string, args ...interface{}) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result string
	err := tdb.Pool.QueryRow(ctx, query, args...).Scan(&result)
	if err != nil {
		t.Fatalf("scalar string query failed: %v\nQuery: %s", err, query)
	}
	return result
}

// QueryScalarBool executes a query and scans a single boolean result.
func QueryScalarBool(t *testing.T, tdb *TestDB, query string, args ...interface{}) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result bool
	err := tdb.Pool.QueryRow(ctx, query, args...).Scan(&result)
	if err != nil {
		t.Fatalf("scalar bool query failed: %v\nQuery: %s", err, query)
	}
	return result
}
