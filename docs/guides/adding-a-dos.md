# Adding a DOS

How to add a new OpsDB substrate instance.

## When to add one

Only for structural reasons. Re-read `dos/README.md` for the valid and invalid
reasons. If you cannot name a structural reason (security perimeter, legal/regulatory,
organizational boundary), you do not need another substrate.

## Steps

### 1. Copy an existing DOS directory

```bash
cp -r dos/prod-0 dos/prod-2
```

### 2. Edit config.yaml

Change three things:

```yaml
substrate:
  name: prod-2
  description: "Production OpsDB — APAC security perimeter"
  site_name: prod-2

database:
  dsn_env_var: OPSDB_PROD2_DSN
```

Add the new substrate to the `cross_references.known_substrates` list in every
existing DOS config so they know about it.

### 3. Edit seed/site.yaml

```yaml
records:
  - name: prod-2
    description: "Production OpsDB — APAC security perimeter"
```

### 4. Edit auth/users.yaml

Set the bootstrap users authorized for this substrate. In production these
may be entirely different people than the other substrates.

### 5. Edit importer credentials

Update `importers/credentials/` with the infrastructure this substrate
observes. Different regions, different cloud accounts, different clusters.

Update `importers/enabled.yaml` to enable the importers relevant to this
substrate's scope.

### 6. Create the database

```bash
createdb opsdb_prod2
export OPSDB_PROD2_DSN="postgres://..."
```

### 7. Apply the same schema

```bash
bin/opsdb-schema apply --repo . --dsn "$OPSDB_PROD2_DSN" --verbose
```

Same schema YAML, same loader, same DDL. The database is structurally
identical to every other substrate.

### 8. Start the API and seed

```bash
bin/opsdb-api --dos dos/prod-2
scripts/seed.sh --dos dos/prod-2 --api http://localhost:8080
```

### 9. Start runners

Deploy the same runner binaries with `--dos dos/prod-2`. They read their
config from OpsDB (seeded in step 8) and their overrides from the DOS directory.

## What stays shared

Schema repo, API code, runner binaries, library suite, change management
discipline. You do not fork any of these.

## What diverges

Data, authorized users, audit log, runner population, importer credentials,
seed data, runner overrides. Each substrate is independent in these dimensions.

## Updating cross-references

If entities in one substrate need to reference entities in another, use
the cross-OpsDB reference pattern: typed pointers with substrate-id,
entity-locator, and policy-hints. These are first-class data in the schema
(`cross_opsdb_reference` pattern). Each API decides independently whether
to resolve incoming cross-references based on its own policy.
