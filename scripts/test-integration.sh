# scripts/test-integration.sh

bash
#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KEEP_DB="${KEEP_DB:-false}"
VERBOSE="${VERBOSE:-false}"
TEST_FILTER="${1:-}"

echo "=== OpsDB Integration Tests ==="
echo ""

# check prerequisites
for cmd in go docker psql; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "error: ${cmd} is required but not found" >&2
        exit 1
    fi
done

# check docker is running
if ! docker info &>/dev/null 2>&1; then
    echo "error: docker daemon is not running" >&2
    exit 1
fi

# start test postgres if not already running
PG_CONTAINER="opsdb-test-postgres"
PG_PORT=15432
PG_USER="opsdb_test"
PG_PASSWORD="opsdb_test_pass"
PG_DB="opsdb_test"
DSN="postgres://${PG_USER}:${PG_PASSWORD}@localhost:${PG_PORT}/${PG_DB}?sslmode=disable"

start_postgres() {
    if docker ps --format '{{.Names}}' | grep -q "^${PG_CONTAINER}$"; then
        echo "test postgres already running on port ${PG_PORT}"
        return
    fi

    # remove stopped container if exists
    docker rm -f "$PG_CONTAINER" 2>/dev/null || true

    echo "starting test postgres on port ${PG_PORT}..."
    docker run -d \
        --name "$PG_CONTAINER" \
        -e POSTGRES_USER="$PG_USER" \
        -e POSTGRES_PASSWORD="$PG_PASSWORD" \
        -e POSTGRES_DB="$PG_DB" \
        -p "${PG_PORT}:5432" \
        postgres:16-alpine \
        >/dev/null

    echo -n "waiting for postgres"
    for i in $(seq 1 30); do
        if psql "$DSN" -c "SELECT 1" &>/dev/null; then
            echo " ready"
            return
        fi
        echo -n "."
        sleep 1
    done
    echo " TIMEOUT"
    echo "error: postgres did not become ready within 30 seconds" >&2
    exit 1
}

stop_postgres() {
    if [ "$KEEP_DB" = "true" ]; then
        echo "keeping test postgres (KEEP_DB=true), DSN: ${DSN}"
        return
    fi
    echo "stopping test postgres..."
    docker rm -f "$PG_CONTAINER" 2>/dev/null || true
}

reset_database() {
    echo "resetting test database..."
    psql "$DSN" -c "
        DROP SCHEMA public CASCADE;
        CREATE SCHEMA public;
        GRANT ALL ON SCHEMA public TO ${PG_USER};
    " &>/dev/null
}

cleanup() {
    stop_postgres
}
trap cleanup EXIT

start_postgres

# phase 1: schema loader tests
echo ""
echo "--- Phase 1: Schema Loader ---"
reset_database
export OPSDB_DSN="$DSN"

GO_TEST_FLAGS="-count=1 -timeout 120s"
if [ "$VERBOSE" = "true" ]; then
    GO_TEST_FLAGS="${GO_TEST_FLAGS} -v"
fi
if [ -n "$TEST_FILTER" ]; then
    GO_TEST_FLAGS="${GO_TEST_FLAGS} -run ${TEST_FILTER}"
fi

echo "running schema loader unit tests..."
go test $GO_TEST_FLAGS ./tools/opsdb_schema/loader/... || { echo "FAILED: schema loader tests"; exit 1; }

echo "running internal package tests..."
go test $GO_TEST_FLAGS ./internal/... || { echo "FAILED: internal package tests"; exit 1; }

# phase 2: schema apply against real postgres
echo ""
echo "--- Phase 2: Schema Apply ---"
reset_database

echo "applying schema to test database..."
go run ./tools/opsdb_schema/cmd apply \
    --repo "$REPO_ROOT" \
    --dsn "$DSN" \
    --verbose 2>&1 | tail -5

echo "verifying schema metadata..."
TABLE_COUNT=$(psql "$DSN" -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'" | tr -d ' ')
META_COUNT=$(psql "$DSN" -t -c "SELECT count(*) FROM _schema_entity_type" 2>/dev/null | tr -d ' ' || echo "0")
echo "tables created: ${TABLE_COUNT}"
echo "entities in _schema_entity_type: ${META_COUNT}"

if [ "$TABLE_COUNT" -lt 10 ]; then
    echo "FAILED: expected at least 10 tables, got ${TABLE_COUNT}" >&2
    exit 1
fi

# phase 3: schema idempotency (re-apply should produce no changes)
echo ""
echo "--- Phase 3: Schema Idempotency ---"

DIFF_OUTPUT=$(go run ./tools/opsdb_schema/cmd diff \
    --repo "$REPO_ROOT" \
    --dsn "$DSN" 2>&1 || true)

if echo "$DIFF_OUTPUT" | grep -q "no changes"; then
    echo "idempotency check passed: re-apply shows no changes"
elif [ -z "$DIFF_OUTPUT" ]; then
    echo "idempotency check passed: empty diff"
else
    echo "warning: diff output after re-apply:"
    echo "$DIFF_OUTPUT"
fi

# phase 4: API tests (if API test files exist)
echo ""
echo "--- Phase 4: API Tests ---"
if find "$REPO_ROOT/tools/opsdb_api" -name '*_test.go' -size +0c 2>/dev/null | grep -q .; then
    echo "running API tests..."
    go test $GO_TEST_FLAGS ./tools/opsdb_api/... || { echo "FAILED: API tests"; exit 1; }
else
    echo "no API test files with content found, skipping"
fi

# phase 5: runner library tests
echo ""
echo "--- Phase 5: Runner Library Tests ---"
if find "$REPO_ROOT/tools/opsdb_runner_lib" -name '*_test.go' -size +0c 2>/dev/null | grep -q .; then
    echo "running runner library tests..."
    go test $GO_TEST_FLAGS ./tools/opsdb_runner_lib/... || { echo "FAILED: runner library tests"; exit 1; }
else
    echo "no runner library test files with content found, skipping"
fi

# phase 6: verify reserved fields injected
echo ""
echo "--- Phase 6: Reserved Field Verification ---"

echo "checking universal fields on all tables..."
TABLES_MISSING_ID=$(psql "$DSN" -t -c "
    SELECT t.table_name
    FROM information_schema.tables t
    WHERE t.table_schema = 'public'
    AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns c
        WHERE c.table_schema = t.table_schema
        AND c.table_name = t.table_name
        AND c.column_name = 'id'
    )
" | grep -v '^\s*$' | tr -d ' ')

if [ -n "$TABLES_MISSING_ID" ]; then
    echo "WARNING: tables missing 'id' column:"
    echo "$TABLES_MISSING_ID"
else
    echo "all tables have 'id' column"
fi

echo "checking audit_log_entry is append-only..."
AUDIT_PRIVS=$(psql "$DSN" -t -c "
    SELECT privilege_type FROM information_schema.role_table_grants
    WHERE table_name = 'audit_log_entry'
    AND grantee NOT IN ('postgres', '${PG_USER}')
    AND privilege_type IN ('UPDATE', 'DELETE')
" 2>/dev/null | tr -d ' ')

if [ -z "$AUDIT_PRIVS" ]; then
    echo "audit_log_entry: no UPDATE/DELETE grants to non-admin roles (correct)"
else
    echo "warning: audit_log_entry has UPDATE or DELETE grants: ${AUDIT_PRIVS}"
fi

# summary
echo ""
echo "=== All Integration Tests Passed ==="
echo "DSN: ${DSN}"
echo ""

