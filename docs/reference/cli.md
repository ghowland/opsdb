# CLI Reference

## opsdb_schema

Schema management tool. Validates, diffs, plans, and applies schema changes.

### Commands

#### validate

Parse and validate schema YAML files. No database required.

```bash
opsdb_schema validate --repo .
```

Checks: YAML syntax, meta-schema conformance, naming conventions, type/constraint/modifier validity, forbidden patterns (regex, embedded logic, inheritance, templating, imports), reserved field collisions, mutually exclusive flags.

Exit codes: 0 = clean, 1 = validation errors.

#### plan

Show what DDL would be generated against a database.

```bash
opsdb_schema plan --repo . --dsn "$OPSDB_DSN"
```

Runs the full pipeline: load → diff → evolution check → DDL generation. Prints each DDL statement with entity name and description. Forbidden changes printed as errors with alternatives.

#### apply

Execute DDL against the database.

```bash
opsdb_schema apply --repo . --dsn "$OPSDB_DSN" [--verbose] [--dry-run]
```

Acquires Postgres advisory lock. Begins transaction. Executes DDL in dependency order. Populates `_schema_*` metadata tables. Commits.

`--dry-run`: executes all DDL then rolls back. Validates correctness without persisting.

`--verbose`: prints each SQL statement as it executes.

#### diff

Show differences between YAML and current database.

```bash
opsdb_schema diff --repo . --dsn "$OPSDB_DSN"
```

Output format:
```
+ new entity: alert_fire
+ new field: service.max_replicas (int)
~ changed constraint: service.min_replicas.max_value: 10 -> 100
! forbidden: service.old_name (field deletion)
```

#### export

Dump current database schema as YAML.

```bash
opsdb_schema export --dsn "$OPSDB_DSN"
```

Reads from `_schema_*` tables (preferred) or `information_schema` (fallback). Writes to stdout.

#### init

Create a new empty schema repository.

```bash
opsdb_schema init --repo /path/to/new/repo
```

Creates `schema/meta/_schema_meta.yaml`, `schema/conventions/reserved.yaml`, `schema/directory.yaml`, and `schema/domains/`.

### Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | `.` | Path to schema repository root |
| `--dsn` | string | `$OPSDB_DSN` | Postgres connection string |
| `--scope` | string | | Limit to entity or entity/field |
| `--verbose` | bool | false | Verbose output |
| `--dry-run` | bool | false | Apply: rollback instead of commit |

## opsdb_api

API server.

```bash
opsdb_api --dos dos/prod-0
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dos` | string | required | Path to DOS directory |

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `OPSDB_{NAME}_DSN` | yes | Postgres DSN (variable name from config.yaml `dsn_env_var`) |
| `OPSDB_SA_SIGNING_KEY` | for service_account auth | Key for service account token validation |

### Health Endpoint

`GET /healthz` — returns 200 if API is running and database is reachable.

## Importers

All importers follow the same CLI pattern:

```bash
opsdb_import_{authority} --dos dos/prod-0
```

### Environment Variables (per importer)

Each importer reads credentials from environment variables referenced in its DOS credential config. Common pattern:

| Variable | Description |
|----------|-------------|
| `OPSDB_AUTH_TOKEN` | API auth token for this runner's service account |
| `OPSDB_API_ENDPOINT` | API endpoint URL |
| `OPSDB_RUNNER_MACHINE_ID` | This runner's machine ID in OpsDB |
| `OPSDB_SITE_ID` | Site ID for this substrate |

Plus authority-specific variables (AWS keys, kubeconfig paths, API tokens).

## Runners

All runners follow the same CLI pattern:

```bash
opsdb-{runner-name} --dos dos/prod-0
```

Same environment variables as importers. Configuration read from `runner_spec_version.runner_data_json` in OpsDB at startup.

## scripts/

### build-all.sh

Build all binaries.

```bash
scripts/build-all.sh
```

Output in `bin/`. Injects version and build time via ldflags.

### seed.sh

Seed initial data into a substrate.

```bash
scripts/seed.sh --dos dos/prod-0 --api http://localhost:8080 [--token TOKEN] [--dry-run]
```

### test-integration.sh

Run integration tests against a Postgres container.

```bash
scripts/test-integration.sh [test-filter]
```

Environment: `KEEP_DB=true` to preserve test database, `VERBOSE=true` for verbose output.

### generate-entity-catalog.sh

Generate entity catalog markdown from schema YAML.

```bash
scripts/generate-entity-catalog.sh
```

Output: `docs/reference/entity-catalog.md`
