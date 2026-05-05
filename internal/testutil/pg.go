
// === internal/testutil/pg.go ===
package testutil

import "testing"

// TestDB holds a test Postgres instance connection info.
type TestDB struct {
	DSN         string
	ContainerID string
	// TODO: container handle from testcontainers
}

// StartTestPostgres starts a Postgres container via testcontainers,
// waits for readiness, returns a handle with DSN.
func StartTestPostgres(t *testing.T) (*TestDB, error) {
	// TODO: start postgres:16 container via testcontainers-go
	// TODO: wait for port to be ready
	// TODO: construct DSN from container host/port
	// TODO: register t.Cleanup to call StopTestPostgres
	// TODO: return TestDB with DSN
	return nil, nil
}

// StopTestPostgres stops and removes the test container.
func StopTestPostgres(tdb *TestDB) error {
	// TODO: container.Terminate(ctx)
	return nil
}

// ResetTestDB drops all tables and sequences, returning database to empty state.
// Used between test cases for isolation.
func ResetTestDB(tdb *TestDB) error {
	// TODO: connect to test database
	// TODO: query pg_tables for all tables in public schema
	// TODO: DROP TABLE ... CASCADE for each
	// TODO: drop all sequences
	return nil
}


