# Decision 002: Monorepo

## Status

Accepted.

## Context

The OpsDB project includes the schema engine, the API server, the runner framework library, five core runners, seven importers, the schema YAML files, the JSON payload schemas, two DOS configurations, documentation, and CI workflows. These could be organized as separate repositories (one per tool, one for schema, one for docs) or as a single repository.

## Decision

Everything lives in one repository. Tools, schema, DOS configurations, documentation, CI workflows, and shared internal packages coexist in a single git repo under a single Go module.

## Rationale

**Atomic changes.** A schema change that requires a corresponding API change and a runner update lands as one commit, one PR, one review. In a multi-repo setup, the same change spans three PRs in three repos with three review cycles, and the intermediate state (schema updated but API not yet, or API updated but runner not yet) creates a window where things are inconsistent.

**Shared code without versioning overhead.** The `internal/` packages (Postgres helpers, model types, naming conventions, vocabulary definitions) are imported directly by every tool. In a multi-repo setup, these would be a separate library repo with its own version, and every tool repo would pin to a version. Updating a shared type means publishing a new library version and updating pins in every consuming repo. In the monorepo, updating a shared type is a code change in the same commit as the consumers.

**Schema and code coevolve.** The schema YAML files and the schema engine that processes them live together. The entity definitions and the API that validates against them live together. The runner specs and the runners that implement them live together. Changes that span schema and code are the norm, not the exception.

**Single CI pipeline.** One repo means one CI configuration. Schema validation, Go tests, integration tests, and binary builds all run in the same pipeline. A multi-repo setup requires CI coordination to ensure that changes in the schema repo trigger tests in the tool repos, which adds complexity and latency.

**Adoption simplicity.** An organization evaluating OpsDB clones one repo and has everything — tools, schema, example DOS configurations, documentation. No dependency graph of repos to discover and clone. `git clone`, `make all`, done.

## Tradeoffs

The repository will grow large over time as documentation, schema files, and JSON payload schemas accumulate. Git handles this well for text files. Binary artifacts (compiled binaries, container images) are not stored in the repo — they are built by CI and distributed through releases and registries.

Organizations forking the project to customize their DOS configurations must fork the entire repo. In practice this is fine — they track upstream for tool and schema changes and diverge only in the `dos/` directory. Git's merge mechanics handle this naturally.
