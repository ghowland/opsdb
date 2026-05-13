# OpsDB N-Substrate Architecture

## What This Document Covers

Most organizations need one OpsDB. Some need more than one. This document specifies when multiple substrates are justified, when they are not, how the N-substrate pipeline works, what is shared across substrates, what diverges, and how to bootstrap from one to N without retrofitting.

The spec's cardinality rule is simple: 0, 1, or N. There is no 2. If you need more than one, you need an architecture that handles any number. Building for exactly two creates a system that breaks at three.

---

## When One Is Enough

One OpsDB substrate serves one operational domain. For most organizations, one operational domain covers everything — production infrastructure, corporate infrastructure, employee fleet, cloud accounts, Kubernetes clusters, monitoring, on-call, compliance. The data is partitioned by site rows within the single substrate. Access is controlled by the five-layer authorization model. Different teams see different data based on their roles and group memberships.

One OpsDB is simpler. One schema to manage. One API to operate. One audit log to query. One set of runners. One change management pipeline. One approval workflow. The single-substrate architecture should be the default assumption, and the burden of proof is on the argument for multiple substrates.

A single OpsDB can scale to serve large query loads through standard database techniques — read replicas for query distribution, connection pooling, caching, and if necessary sharding by entity type or time range. Performance is not a valid reason to split into multiple substrates because the performance problem can be solved within a single substrate.

---

## When N Is Required

Multiple substrates are justified only by structural reasons — properties of the organization or its operating environment that cannot be addressed by access control, data partitioning, or scaling within a single substrate.

### Security Perimeter Separation

When the API's five-layer authorization model is structurally insufficient to enforce the required separation. This occurs when a breach of one substrate must not expose data in another — not because the access policies are wrong, but because the threat model demands that no single vulnerability in the API, the database, or the network path can bridge the two domains.

A defense contractor managing classified and unclassified infrastructure needs separate substrates. A financial institution with a trading floor isolated from corporate IT by regulatory mandate needs separate substrates. The separation is physical and architectural, not just logical.

### Legal and Regulatory Residency

When GDPR, sectoral regulations, or data sovereignty requirements demand that operational data for certain domains resides in specific jurisdictions. An organization with European customers subject to GDPR and US government customers subject to FedRAMP may need substrates in different regions with different data residency guarantees.

The data must physically reside in the required jurisdiction. A single substrate with row-level filtering does not satisfy residency requirements because the data traverses the same network, lives on the same storage, and is accessible through the same API endpoint.

### Independent Organizational Units

When business units operate as genuinely separate organizations — different processes, different conventions, different personnel, different compliance scopes. Recent acquisitions that haven't been integrated. Divisions that operate under different regulatory regimes. Subsidiaries in different countries with independent IT operations.

The key indicator is that the coordination cost of a shared substrate exceeds the benefit. If two business units don't share personnel, don't share processes, don't share compliance requirements, and don't need to query each other's operational data, forcing them into a single substrate creates coordination overhead with no payoff.

### Air Gap

When physical isolation is required — classified systems, industrial control systems, systems with no network connectivity to the rest of the organization. These are substrates by definition because they cannot share a database or API endpoint with anything outside their physical boundary.

---

## When N Is Not Justified

The spec explicitly lists invalid reasons for splitting into multiple substrates. If the rationale falls into any of these categories, the decision is wrong and phase one is not complete.

### Technical Fragility

"We need a separate OpsDB for production to protect it from corporate tooling experiments." This is a symptom of poor operational discipline, not a structural requirement. The solution is proper access control and change management within a single substrate — test changes in a staging environment, gate production changes through approval, use the five-layer auth model to prevent unauthorized writes. Splitting substrates because you don't trust your own governance means you need better governance, not more substrates.

### Convenience

"Two would be easier to manage." Two substrates means two schemas to keep synchronized, two API deployments to operate, two sets of runners to maintain, two audit logs to query, two change management pipelines to govern. The overhead of the N-pipeline always exceeds the overhead of managing a single substrate. Convenience is never structural.

### Premature Optimization

"We might need to split eventually." Stay at one until structure forces N. Retrofitting N onto a system designed for one is more expensive than bootstrapping N from the start, but bootstrapping N when you don't need it is wasted effort. The architecture supports N from day one (the DOS configuration pattern, the shared schema, the diverged data). You don't build the pipeline until you need it. You build the architecture so the pipeline is possible when you need it.

### Performance

"One substrate can't serve our query load." Scale within the single substrate. Read replicas handle query distribution. Connection pooling handles concurrent connections. Caching handles repeated queries. Sharding by entity type or time range handles data volume. If the performance problem persists after exhausting single-substrate scaling options, the problem is likely query design or missing indexes, not substrate count.

---

## What Is Shared, What Diverges

The N-substrate architecture has a clear separation between shared components (the same across all substrates) and diverged components (independent per substrate).

### Shared

**Schema repository.** One git repo containing the YAML entity files. Every substrate runs the same schema. When the schema evolves, the change is applied to all substrates. Different substrates cannot have different schemas — that would make cross-substrate references impossible to validate and would fragment the runner population.

**Library suite contracts and implementations.** One set of libraries consumed by all runner populations. The API client library, the world-side libraries, the coordination libraries, the observation libraries — all identical across substrates. A runner built against the library suite works against any substrate.

**API code.** One codebase deployed N times. Each deployment is configured with its own database connection, its own auth backend configuration, and its own listen address, but the code is identical. A bug fix to the API is deployed to all substrates. A new API feature is available at all substrates simultaneously.

**Change management discipline.** The same rules apply at every substrate. The validation pipeline, the approval routing logic, the emergency path, the bulk operations — all identical. What varies is the policy data that parameterizes these mechanisms (different approval rules, different auto-approval policies), not the mechanisms themselves.

**Tool binaries.** The same runner binaries, the same importer binaries, the same schema engine binary. Built once, deployed to all substrates.

### Diverged

**Data.** Each substrate is its own write authority. The cloud resources in the production substrate are different rows than the cloud resources in the staging substrate (if staging is a separate substrate). The data diverges because it represents different operational reality.

**Users authorized.** Each substrate has its own `ops_user`, `ops_group`, and `ops_user_role` rows. The same person might have admin access at the staging substrate and read-only access at the production substrate. Or the substrates might serve completely different personnel (the security perimeter case).

**Audit log.** Each substrate has its own append-only audit log. Audit queries are per-substrate. An auditor examining the production substrate's history queries the production audit log. Cross-substrate audit correlation requires querying both substrates independently.

**Runners deployed.** Each substrate has its own runner population. The same runner binary might be deployed at both substrates, but each deployment is a separate runner instance with its own runner_machine row, its own runner_spec configuration, its own schedule, and its own target scope. A puller at the production substrate reads from production cloud accounts. A puller at the staging substrate reads from staging cloud accounts.

**Policies and approval rules.** Each substrate has its own policy data. The production substrate might require two approvers for any change to compliance-scoped entities. The staging substrate might auto-approve everything. The policy mechanisms are shared (same API code evaluates them), but the policy data is diverged.

**Seed data.** Each substrate is seeded independently — different site names, different admin users, different base policies, different authority configurations.

---

## Cross-Substrate References

When N substrates exist, entities in one substrate sometimes need to reference entities in another. A service in the production substrate might depend on a database managed in a separate regulated-data substrate. An incident investigation might need to correlate changes across substrates.

Cross-substrate references are typed pointers: substrate identity plus entity locator plus policy hints. They are first-class data in the schema — `cross_opsdb_reference` rows that name the target substrate, the target entity type, and the target entity ID.

### Federated Reads

Each API enforces what is resolvable from the calling context. When a query at substrate A encounters a cross-reference to substrate B, the API at substrate A decides whether to include the reference in the result (based on policy). It does not fetch the data from substrate B — the caller receives the reference and can query substrate B directly if they have access.

This is deliberate. Cross-substrate data fetching would create coupling between substrates (A depends on B being available), would complicate the authorization model (does the caller's auth at A imply auth at B?), and would make the audit trail ambiguous (which substrate's audit log records the read?).

### Cross-Substrate Writes

Not supported through the OpsDB API. Each substrate is its own write authority. Changes that need to happen at multiple substrates are coordinated through external means — a human filing change sets at each substrate, or a runner with credentials at multiple substrates submitting change sets independently.

This constraint exists because cross-substrate writes would require distributed transactions or eventual consistency between substrates, both of which add complexity that undermines the simplicity of the single-substrate model. Each substrate's change management pipeline, approval workflow, and audit trail operate independently. Cross-substrate coordination is an organizational process, not a database transaction.

---

## The N Pipeline

### Bootstrap at Two

The spec recommends bootstrapping the N-pipeline at two substrates even when the organization knows they will eventually need three or more. Two is the minimum number that forces N-mode thinking — shared schema deployment, diverged configuration, cross-substrate reference handling, runner deployment to multiple targets.

Starting at two catches failure modes early while the investment is small. Schema synchronization issues (propagation bugs, version pinning), library deployment across substrates, runner configuration management for multiple targets — all of these surface at two substrates with small data volumes, where they are cheap to fix. Discovering them at five substrates with production data is expensive.

The cost argument is concrete: building the N-pipeline at N=2 costs slightly more than building for N=1. Retrofitting the N-pipeline onto a system that grew from 1 to 2 independently (two separate OpsDB installations that were never designed to share schema or tooling) costs much more, because every component that should have been shared must be reconciled, and every component that should have diverged must be separated.

By N=3, the pipeline is mature. Adding the fourth, fifth, or tenth substrate is routine — deploy the shared schema, deploy the API, seed the substrate, configure runners, start importers.

### Pipeline Operations

**Schema deployment.** The schema steward merges changes to the schema repository. The schema engine is run against each substrate's database. The order doesn't matter because schema changes are additive — adding a field at substrate A before substrate B doesn't create inconsistency, it just means substrate A has the field slightly earlier.

In practice, a CI pipeline runs `opsdb_schema apply` against each substrate's database in sequence. If any substrate's apply fails (which shouldn't happen for additive changes against valid schemas, but could happen due to connectivity or permission issues), the pipeline stops and alerts. The failed substrate is behind by one schema version until the issue is resolved. This is acceptable because schema changes are additive — the old schema is a subset of the new schema, so runners built against the old schema continue working.

**API deployment.** The same API binary is deployed to each substrate's API endpoint. Standard deployment practices apply — rolling update, health checks, rollback on failure. The deployments are independent — updating the API at substrate A does not require updating at substrate B simultaneously, though keeping versions aligned is good practice.

**Runner deployment.** Each substrate has its own runner population defined by `runner_machine` and `runner_spec` rows in that substrate. Deploying a new runner version means updating the runner binary at each substrate where it runs. The runner spec configuration (which targets to process, which schedules to follow, which bounds to enforce) is per-substrate data.

**Library distribution.** Libraries are distributed through standard package mechanisms (Go modules, container images). All substrates consume the same library versions. Updating a library means publishing a new version and updating runners that consume it — the same process regardless of how many substrates exist.

---

## DOS Configuration for N Substrates

The master repository's `dos/` directory contains one directory per substrate. Each directory has the same structure — config.yaml, auth directory, seed directory, runners directory, importers directory. The structure is uniform so that tooling works identically against any substrate.

### config.yaml

Each substrate's config.yaml names the substrate, points to its database (via environment variable for the DSN, never a literal connection string in the file), and points to the shared schema directory. The schema path is relative — all substrates point to the same `../../schema` directory.

```yaml
substrate:
  name: ops-prod
  description: "Production operational substrate"
database:
  dsn_env_var: OPSDB_PROD_DSN
schema:
  repo_path: ../../schema
```

```yaml
substrate:
  name: ops-staging
  description: "Staging operational substrate"
database:
  dsn_env_var: OPSDB_STAGING_DSN
schema:
  repo_path: ../../schema
```

Same schema path. Different database. Different identity. This is the N-pipeline in configuration.

### Policy Divergence

The staging substrate might have permissive auto-approval policies — all drift corrections auto-approve, no human approval required for most changes. The production substrate has strict policies — human approval required for compliance-scoped entities, security team approval for security-relevant fields, schema steward approval for schema changes.

These differences are expressed entirely in seed data and policy rows, not in code or configuration files. The same API code evaluates the same approval routing logic against different policy data and produces different outcomes. Staging auto-approves because the policy data says to. Production requires humans because the policy data says to.

### Runner Divergence

The production substrate might run importers against production AWS accounts and production Kubernetes clusters. The staging substrate runs importers against staging accounts and staging clusters. The runner binaries are identical — the configuration (which accounts, which clusters, which credentials) diverges.

Some runners might run only at one substrate. A compliance scanner might run at production but not staging (staging doesn't need compliance evidence). A notification runner might run at both but with different channel configurations (production pages go to the production on-call, staging alerts go to a Slack channel).

---

## Adding a New Substrate

When the organization determines that a new substrate is needed (validated against the structural criteria, not the convenience criteria), the process is mechanical.

Copy an existing DOS directory as a starting point. Edit config.yaml with the new substrate name and database connection. Edit seed files for the new environment — site name, admin user, base policies, authority configurations. Edit runner and importer enabled lists for what this substrate needs.

Create the database. Run `opsdb_schema apply` against it — the same schema, deployed to a new database. Run the seed script. Start the API. Start runners and importers.

The new substrate uses the same schema, same tools, same libraries as every other substrate. Only data and configuration diverge. This is N=3 (or N=4, or N=10) with zero code changes, zero schema changes, and zero library changes.

### What Requires Organizational Decisions

The technical process of adding a substrate is mechanical. The organizational decisions are not.

Who has access to the new substrate? What approval rules govern it? Which domains are promoted to governed entities versus observation-only? Which runners address which operational domains? How do cross-substrate references work between this substrate and existing ones?

These are policy decisions expressed as data in the new substrate's seed files and policy rows. They require the same deliberate organizational choices that the original substrate required. The N-pipeline makes the technical side routine so the team can focus on the governance side.

---

## Operating Multiple Substrates

### Day-to-Day

Each substrate operates independently. Its API serves its consumers. Its runners run their cycles. Its importers pull from their authorities. Its audit log accumulates. Its change sets flow through its approval pipeline.

An operations team working across multiple substrates uses the same tools and the same queries at each one. The API operations are identical. The schema is identical. The runner patterns are identical. The team's mental model transfers directly — learning one substrate teaches you all of them.

### Schema Evolution

Schema changes are proposed, reviewed, and merged in the shared schema repository. The schema engine is run against each substrate. All substrates converge on the new schema. The schema steward reviews changes once (in the schema repo), not N times (once per substrate).

If a schema change requires coordination with library changes or runner changes, the ordering discipline applies: schema first, then library, then runners. This ordering is per-substrate — substrate A might receive the schema change, library update, and runner update on Monday, while substrate B receives them on Tuesday. The lag is acceptable because all changes are additive.

### Monitoring and Health

Each substrate's health is observable through its own runner job history, its own observation data, and its own audit log. A meta-monitoring runner (running outside any individual substrate or running at a designated "management" substrate) can query each substrate's API to check health indicators — are runners running on schedule? Are observations fresh? Are change sets being processed?

Cross-substrate health correlation (did the same issue affect both substrates?) requires querying both APIs and joining the results outside OpsDB. This is a limitation of the diverged-data model — each substrate's audit log is independent. The tradeoff is simplicity and isolation: each substrate's integrity does not depend on any other substrate being available or correct.

### Disaster Recovery

Each substrate is independently recoverable. Database backups, point-in-time recovery, and replica promotion are per-substrate operations using standard database tooling. Recovering substrate A does not require substrate B to be available or consistent.

Cross-substrate references may become stale after recovery if one substrate is restored to a point in time when the other had different data. The references are pointers, not embedded data, so staleness is detectable (the reference points to an entity that doesn't exist or has different attributes at the target substrate). Resolution is manual — update the references or accept the divergence.

---

## The Architecture From Day One

The master repository is structured for N substrates from the start. The `dos/` directory contains two substrate configurations (production and staging) that demonstrate the pattern. The schema is shared. The tools are shared. The configuration diverges.

An organization that needs only one substrate ignores the staging DOS directory and uses only production. The architecture is the same. The schema engine, the API, the runners, the importers — all work identically whether there's one substrate or ten. The N-pipeline is latent in the directory structure, ready to activate when structural requirements demand it.

This is the bootstrap-at-largest-cardinality-from-smallest principle. The cost of N-ready architecture at N=1 is one extra directory that nobody uses. The cost of retrofitting N-readiness onto a system that grew organically to N=2 is reconciling two independent schemas, two independent codebases, two independent runner populations, and two independent governance models into a shared pipeline. The first cost is negligible. The second cost is substantial. Pay the negligible cost from day one.
