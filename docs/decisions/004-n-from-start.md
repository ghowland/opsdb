# Decision 004: N-Substrate From Start

## Status

Accepted.

## Context

The spec's cardinality rule says 0, 1, or N — there is no 2. Most organizations need one OpsDB. Some need more than one for structural reasons (security perimeter separation, legal residency requirements, independent organizational units, air-gapped systems). The question is whether the project should build for N from the start or build for 1 and add N-support later.

## Decision

The project structures for N substrates from day one. The master repository ships with two DOS configurations (production and staging) that demonstrate the N pattern. The schema is shared. The tooling is shared. The configuration diverges per substrate.

## Rationale

**The cost difference is negligible now, substantial later.** Building for N at the start means one extra directory in the repo that single-substrate organizations ignore. Retrofitting N onto a system that grew organically as a single substrate means reconciling schema management processes, runner deployment patterns, configuration management, and governance models that were never designed to be shared. The first cost is one directory. The second cost is a project.

**The pattern teaches itself.** Having two DOS configurations in the repo demonstrates what is shared (schema, tools, libraries) and what diverges (data, users, policies, runners) without requiring the reader to imagine it. A new user exploring the repo sees two substrate configurations side by side and immediately understands the N-pipeline.

**Bootstrap at two catches problems early.** The CI pipeline applies the schema to two test databases, seeds both, and runs integration tests against both. Issues with schema synchronization, runner configuration for multiple targets, and cross-substrate reference handling surface during development, not after deployment.

**Adding N+1 is mechanical.** Copy a DOS directory, edit config, create database, apply schema, seed, start. Zero code changes. This only works if the architecture was designed for N. If it was designed for 1, adding the second substrate requires architectural changes — and those changes under pressure (we need a second substrate now because the regulator said so) are the most expensive kind.

## Tradeoffs

The two-DOS structure adds mild complexity for organizations that will only ever run one substrate. They see a `dos/opsdb-ops-staging/` directory they don't use. This is a directory, not a burden. The documentation explains that single-substrate organizations use only the production DOS and ignore staging.

The N-pipeline adds concepts (cross-substrate references, federated reads, diverged policies) that single-substrate organizations don't need to understand. The documentation covers these in the N-substrate doc, separate from the main architecture docs, so they don't add cognitive load for the common case.

