# Quickstart

Get a working OpsDB from zero to queryable in five steps.

## Prerequisites

- Go 1.22+
- Postgres 16+
- Docker (for integration tests)
- Git

## Steps

### 1. Clone and build

```bash
git clone https://github.com/ghowland/opsdb.git
cd opsdb
scripts/build-all.sh
```

Binaries land in `bin/`.

### 2. Create a Postgres database

```bash
createdb opsdb_dev
export OPSDB_DSN="postgres://localhost:5432/opsdb_dev?sslmode=disable"
```

### 3. Apply the schema

```bash
bin/opsdb-schema apply --repo . --dsn "$OPSDB_DSN" --verbose
```

This reads every YAML file under `schema/`, validates against the meta-schema,
resolves FK dependencies, generates DDL, applies it inside an advisory-locked
transaction, and populates the `_schema_*` metadata tables. On success it prints
a count of tables created, fields added, and constraints applied.

Verify:

```bash
psql "$OPSDB_DSN" -c "SELECT count(*) FROM _schema_entity_type;"
```

Should return 138+ entity types.

### 4. Start the API with a DOS config

For local development, use one of the example DOS directories:

```bash
export OPSDB_PROD0_DSN="$OPSDB_DSN"
bin/opsdb-api --dos dos/prod-0
```

The API starts on port 8080 using the YAML auth provider with the bootstrap
users defined in `dos/prod-0/auth/users.yaml`.

Test connectivity:

```bash
curl -u admin:changeme http://localhost:8080/api/v1/site/1
```

### 5. Seed initial data

```bash
scripts/seed.sh --dos dos/prod-0 --api http://localhost:8080 --token "$(echo -n admin:changeme | base64)"
```

This loads the site identity, admin user, base policies, core runner specs,
and runner service accounts. After seeding, the substrate is ready for
importers and runners.

## What to do next

- Write your first importer: [Writing an Importer](writing-an-importer.md)
- Write your first runner: [Writing a Runner](writing-a-runner.md)
- Understand change management: [Approval Rules](approval-rules.md)
- Add a second substrate: [Adding a DOS](adding-a-dos.md)
- Transition from dev to production: [Dev to Operational](dev-to-operational.md)
