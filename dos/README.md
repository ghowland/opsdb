# DOS — Distributed Operating System Instances

![Fig. 1: OpsDB as a Closed Operational Loop — operate, govern, and evidence through the same motion.](../docs/figures/opsdb_01_operational_loop.png)

## What This Directory Is

Each subdirectory under `dos/` represents one OpsDB substrate instance — one
independent database with its own data, users, audit log, and runner population.
All instances share the same schema (from `schema/`), the same API code, the same
runner binaries, and the same shared library suite. What differs is the data each
holds, who is authorized to access it, and which runners operate against it.

This is the N-substrate pattern from the OpsDB specification.

## Why N Instead of 1

The specification defines exactly three valid structural reasons to run more
than one OpsDB instance:

1. **Security perimeter** — breach of one substrate must not expose another.
   API-layer authorization alone is insufficient when the threat model requires
   physical data separation.

2. **Legal/regulatory** — GDPR, sectoral regulation, or data sovereignty
   requires that certain operational data reside in specific jurisdictions
   or on specific infrastructure.

3. **Organizational boundary** — business units operating as effectively
   separate companies, or recent acquisitions not yet sharing processes and
   conventions, where the coordination cost of a single substrate exceeds
   the benefit.

The specification explicitly rejects these as reasons for N > 1:

- "Technical fragility" (protect prod from experiments) — sign of bad ops, not structure.
- "Convenience" (two would be easier) — convenience is not structural.
- "Premature" (we might need to split eventually) — stay at 1 until structure forces N.
- "Performance" (one can't serve our query load) — scale within a single substrate.

Two is an invalid cardinality. You have 1, or you have N. If you have 2, you
designed for N=2, which will break at N=3. This repo demonstrates N from the start,
even though N=2, because the N-pipeline at N=2 costs only slightly more than N=1
at N=1, while N-pipeline retrofitted onto N=2-grown-independently costs far more.

## This Example

This repository ships with two production substrates:

- **prod-0** — First production OpsDB instance. In a real deployment this might
  be the Americas region, the core business unit, or the primary security perimeter.

- **prod-1** — Second production OpsDB instance. In a real deployment this might
  be EMEA (for GDPR), an acquired subsidiary, or a separate security boundary.

For simplicity of the example, both are configured identically except for names
and connection strings. In a real deployment they would diverge on:

- Different Postgres instances (possibly different regions or providers)
- Different authorized users (per-substrate access control)
- Different importer credentials (each reads from its own infrastructure)
- Different runner overrides (retention policies, notification backends, schedules)
- Different seed data (site name, initial policies matching local requirements)

A real organization would typically also have a **corp** DOS covering everything
that is not production — corporate infrastructure, development environments,
internal tooling, office networks. Staging lives in whichever world the
organization decides: some put staging under the production DOS because staging
hardware is production-adjacent, others put it under corp because staging is
a development concern. The principle is "slice the pie" — every piece of
operational reality is covered by exactly one substrate, no gaps, no overlaps.

## What Is Shared vs What Diverges

### Shared (one copy, deployed to all substrates)

| Component | Location | Why shared |
|-----------|----------|------------|
| Schema repository | `schema/` | One schema, N databases. Same entity types everywhere. |
| Library suite | `internal/`, runner lib | Same contracts, same behavior, same runner code. |
| API code | `tools/opsdb-api/` | One codebase deployed N times. |
| Change management discipline | Same approval rules pattern | Same governance model at each substrate. |
| Runner binaries | `tools/runners/`, `tools/importers/` | Same executables, different config. |

### Diverged (independent per substrate)

| Component | Location | Why diverged |
|-----------|----------|--------------|
| Data | Each Postgres instance | Each substrate is its own write authority. |
| Authorized users | `dos/{name}/auth/` | Per-substrate access control. |
| Audit log | `audit_log_entry` table per DB | Independent, non-shared audit trails. |
| Runners deployed | `dos/{name}/runners/` | Each substrate has its own runner population. |
| Importer credentials | `dos/{name}/importers/` | Each reads from its own infrastructure. |
| Seed data | `dos/{name}/seed/` | Site identity, initial policies, bootstrap users. |

### Cross-substrate references

Cross-OpsDB reads are supported via typed pointers (substrate-id + entity-locator +
policy-hints). Cross-OpsDB writes are not supported — each substrate is its own
write authority. Coordination between substrates happens through external means
(shared git repos, shared documentation, organizational process).

## Directory Structure


dos/
├── prod-0/                    # First production substrate
│   ├── config.yaml            # Substrate identity, database, API, auth config
│   ├── auth/
│   │   └── users.yaml         # Bootstrap users (YAML auth provider)
│   ├── seed/
│   │   ├── site.yaml          # Site identity (first record in the database)
│   │   ├── admin_user.yaml    # Initial admin ops_user
│   │   ├── base_policies.yaml # Security zones, classifications, retention, approval rules
│   │   ├── core_runner_specs.yaml  # Runner specs for self-management runners
│   │   └── runner_service_accounts.yaml  # Service accounts for runner auth
│   ├── importers/
│   │   ├── enabled.yaml       # Which importers to run against this substrate
│   │   └── credentials/       # Per-authority credential references
│   │       ├── aws.yaml
│   │       ├── k8s.yaml
│   │       └── pagerduty.yaml
│   └── runners/
│       ├── enabled.yaml       # Which runners to deploy for this substrate
│       └── overrides/         # Per-runner config overrides for this substrate
│           ├── notification.yaml
│           └── reaper.yaml
└── prod-1/                    # Second production substrate (same structure)
    └── ...


## How to Use

### Bootstrap a new substrate

bash
# 1. Apply schema to the substrate's database
OPSDB_DSN="postgres://..." opsdb-schema apply --repo . --dsn "$OPSDB_DSN"

# 2. Start the API pointing at this substrate's DOS config
opsdb-api --dos dos/prod-0

# 3. Seed initial data
scripts/seed.sh --dos dos/prod-0 --api http://localhost:8080

# 4. Start runners
# (each runner reads its config from the DOS directory)


### Add a new substrate

1. Copy an existing DOS directory: `cp -r dos/prod-0 dos/prod-2`
2. Edit `config.yaml` with the new substrate name, database DSN, and site identity.
3. Edit `auth/users.yaml` with the authorized bootstrap users.
4. Edit `seed/site.yaml` with the new site name.
5. Update importer credentials for the new substrate's infrastructure.
6. Apply schema, start API, seed, start runners — same process.

The schema and all code are shared. Only the per-substrate configuration diverges.


# Diagrams

## Definitions Become Runtime Control

![Fig. 2: Definitions Become Runtime Control — schema definitions become live runtime behavior through the loader, database, and metadata path.](../docs/figures/opsdb_02_schema_to_runtime.png)

## Change Management

![Fig. 3: Governed Change as Staged Motion — intent, approval, execution, and version history are separate visible stages.](../docs/figures/opsdb_03_governed_change_flow.png)

## Provenance Through Time

![Fig. 4: Provenance Through Time — one action becomes a queryable timeline instead of a reconstructed story.](../docs/figures/opsdb_04_provenance_timeline.png)

## Automation Shift

![Fig. 5: Automation Topology Shift — runners coordinate through OpsDB state instead of calling each other directly.](../docs/figures/opsdb_05_automation_topology.png)

## One Operational Grammar Across Domains

![Fig. 6: One Operational Grammar Across Domains — cloud, Kubernetes, internal systems, manual procedures, and audit all fit one operational model.](../docs/figures/opsdb_06_operational_landscape.png)

## The API Gate as a Control Membrane

![Fig. 7: The API Gate as a Control Membrane — many callers cross one ordered boundary before durable state can change.](../docs/figures/opsdb_07_api_membrane.png)

## Operations and Audit as One System

![Fig. 8: Operations and Audit as One System — operational events produce evidence as a native property of execution.](../docs/figures/opsdb_08_operations_audit_unified.png)

