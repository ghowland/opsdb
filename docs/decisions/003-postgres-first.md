# Decision 003: Postgres First

## Status

Accepted.

## Context

The spec declares storage engine independence as an architectural commitment — the schema is the design, the engine is the implementation choice. The nine field types map to standard SQL features. The project needs to pick one engine to implement first.

## Decision

The first and currently only supported storage engine is PostgreSQL.

## Rationale

**Feature completeness.** Postgres supports every feature the schema engine needs natively: JSONB for typed payloads with indexing and containment queries, CHECK constraints for enum validation and numeric bounds, foreign key constraints with CASCADE and RESTRICT, advisory locks for concurrent apply safety, SERIAL for auto-increment, TIMESTAMP for datetime, REVOKE for append-only enforcement on audit_log_entry, and transactional DDL (CREATE TABLE inside a transaction that rolls back atomically on failure). Most other engines lack at least one of these — MySQL's transactional DDL support is incomplete, SQLite lacks REVOKE.

**Operational maturity.** Postgres has decades of production track record, well-understood backup and recovery procedures, mature replication (streaming replication, logical replication), and broad availability as managed services (RDS, Cloud SQL, Azure Database for PostgreSQL). An operations team deploying OpsDB already knows how to operate Postgres or can learn from abundant documentation.

**JSON payload validation readiness.** JSONB with containment operators (`@>`) enables the API's JSON path containment predicate for typed payload queries. Postgres's jsonb_path_exists and jsonb_path_query functions enable future structured queries against typed payloads without application-layer parsing.

**Ecosystem.** Go's `database/sql` with `lib/pq` or `pgx` provides a mature, well-tested Postgres driver. The schema engine's DDL generation targets Postgres-specific syntax (SERIAL, JSONB, TIMESTAMP WITHOUT TIME ZONE) but the generation layer is isolated so that future engines require only a new generator, not changes to the validation, resolution, or evolution logic.

## Tradeoffs

Organizations running MySQL, SQL Server, or other engines cannot use OpsDB without Postgres. Since OpsDB is a new deployment (not a migration of an existing database), adding a Postgres instance is a low barrier. Managed Postgres is available on every major cloud provider.

The schema engine's DDL generator is Postgres-specific. Supporting additional engines requires implementing new generators. The spec's engine-independence commitment means this is architecturally possible — the schema YAML, the validation logic, the evolution rules, and the `_schema_*` population are all engine-independent. The generator is the only engine-specific component. Adding MySQL or SQLite support is a bounded effort, but it is not prioritized for the initial release.

