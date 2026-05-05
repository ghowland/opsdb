## OpsDB FOSS Technical Plan

### Component 1: Schema Engine (opsdb-schema)

**What it does:** Reads the YAML schema repository, validates it against the closed vocabulary, and generates Postgres DDL. Applies schema changes idempotently to a running database.

**Input:** A git repository containing the schema files organized per the spec — a directory.yaml at the root listing imports in dependency order, per-domain directories containing entity YAML files, a meta/_schema_meta.yaml defining the meta-schema, and conventions/reserved.yaml defining reserved fields.

**Entity file format per the spec vocabulary:**

```yaml
name: cloud_resource
versioned: true
fields:
  - name: cloud_account_id
    type: foreign_key
    references: cloud_account
    nullable: false
  - name: cloud_resource_type
    type: enum
    enum_values:
      - ec2_instance
      - gce_instance
      - rds_database
      # ... full list from spec
    nullable: false
  - name: cloud_data_json
    type: json
    json_type_discriminator: cloud_resource_type
    nullable: false
  - name: is_active
    type: boolean
    default: true
indexes:
  - fields: [cloud_account_id, cloud_resource_type]
  - fields: [cloud_resource_type]
```

**The loader process, following the spec's bootstrap sequence:**

First, validate the meta-schema file against the loader's hardcoded baseline. This is the one piece of hardcoded schema — the description of what a valid schema file looks like. It defines the nine types, three modifiers, and six-plus constraints from the closed vocabulary.

Second, load conventions/reserved.yaml and validate against the meta-schema. This defines the universal reserved fields — id as auto-increment integer, created_time and updated_time as datetime, parent_id as self-FK when hierarchical, is_active as boolean when soft-deletable.

Third, process the imports list from directory.yaml in order. Each entity file is validated against the meta-schema. Foreign key references are resolved against entities loaded so far — this is why import order matters and why directory.yaml specifies it explicitly. The loader rejects any file containing vocabulary outside the closed set. No regex. No embedded logic. No conditional constraints. No inheritance. No templating. No imports within entity files. Each of these refusals is enforced mechanically by the loader, not by convention.

Fourth, generate Postgres DDL from the validated schema. CREATE TABLE statements with columns typed appropriately — int maps to INTEGER, varchar maps to VARCHAR(max_length), foreign_key maps to INTEGER with a FOREIGN KEY constraint, json maps to JSONB, enum maps to VARCHAR with a CHECK constraint against the enum values list, datetime maps to TIMESTAMP. Every table gets the reserved fields injected. Versioned entities get their sibling table generated — cloud_resource_version with version_serial, parent_cloud_resource_version_id, change_set_id, is_active_version, and approved_for_production_time. Indexes generated from the indexes list in each file. Foreign key constraints generated for every foreign_key field. Composite unique constraints generated from must_be_unique_within declarations.

Fifth, apply DDL to Postgres in dependency order. Tables with no foreign key dependencies first, then tables referencing those, and so on. The loader builds a dependency graph from the foreign key declarations and topologically sorts it.

Sixth, populate the _schema_* tables. Every entity type gets a row in _schema_entity_type. Every field gets a row in _schema_field. Every relationship gets a row in _schema_relationship. A row in _schema_version with is_current set to true marks this schema version as active.

**Idempotent updates — the critical feature:**

The loader must handle being run against an existing database with data. It accepts a scope argument specifying what to update — the full database, a specific table, or a specific field, using the triple format db/table/field with wildcards.

For a new entity that doesn't exist as a table yet, generate and execute CREATE TABLE. For an existing entity with new fields, generate ALTER TABLE ADD COLUMN. For widened numeric ranges or length bounds, generate ALTER TABLE with updated CHECK constraints. For new enum values, update the CHECK constraint. For new indexes, CREATE INDEX.

The loader refuses to execute anything that violates the schema evolution rules. No column drops. No column renames. No type changes. No narrowing of ranges or lengths. No removal of enum values. If the YAML changes request any of these, the loader prints a clear error citing the specific rule violated and exits without modifying the database.

The schema diff between current database state and desired YAML state is computed by reading _schema_field rows and comparing against the loaded YAML. This makes the _schema_* tables the record of what was applied, and the YAML files the declaration of what should be.

**Implementation language:** Go or Python. Go for a single binary distribution. Python for faster community contribution. I'd recommend Go — the loader is infrastructure tooling that should be dependency-minimal per the spec's own principles.

**Deliverable:** A single binary, opsdb-schema, that takes a schema repo path and a Postgres connection string and makes the database match the schema, idempotently, within the evolution rules.

---

### Component 2: API Gate (opsdb-api)

**What it does:** Implements the sixteen API operations with the 10-step gate sequence. Serves as the only path to read or write OpsDB data. No direct database access for any consumer.

**The API is data-driven, not code-driven.** The gate doesn't hardcode which entities exist or what fields they have. It reads _schema_entity_type, _schema_field, and _schema_relationship at startup and builds its validation rules from that data. When the schema is updated by the loader and _schema_* rows change, the API picks up the new schema on its next refresh cycle. Adding a new entity type to the OpsDB means adding YAML files and running the loader — no API code changes, no redeployment.

**The 10-step gate, implemented as a pipeline where each step can reject:**

Step 1, Authentication. Pluggable auth backend. Ships with three backends: YAML file lookup for development and testing (a users.yaml with usernames, hashed passwords, and role assignments — no external dependencies needed to get started), OIDC for production human auth, and service account token validation for runner auth. The auth step resolves the caller to an ops_user_id or a runner_machine_id. Every subsequent step uses this resolved identity.

Step 2, Authorization. Five layers evaluated in order, first denial halts. Layer one checks the caller's role against the operation class — reads, writes, change-management actions. Layer two checks _requires_group on the target entity row if it exists. Layer three checks _access_classification on the target fields against the caller's clearance. Layer four, for runner callers, checks runner_capability and runner target bridge rows against the operation. Layer five evaluates policy rows of type access_control for time-of-day restrictions, separation-of-duty rules, tenure requirements. The authorization engine reads all policy data from the database — changing authorization rules means changing data rows through change-sets, not changing code.

Step 3, Schema Validation. The operation shape is checked against _schema_field rows for the affected entity type. Field exists? Type matches? Required fields present on creates? This is a mechanical lookup against the schema metadata tables, not hardcoded validation logic.

Step 4, Bound Validation. Numeric fields checked against min_value and max_value from the schema. String fields checked against min_length and max_length. Enum fields checked against enum_values. Foreign key fields checked for existence of referenced row. No regex evaluated, ever.

Step 5, Policy Evaluation. Policy rows of type semantic_invariant evaluated. These are the cross-field constraints the schema deliberately excludes — "if status is X then field Y must be set," "min_replicas must be less than or equal to max_replicas." These rules are data rows, change-managed, queryable.

Step 6, Versioning Preparation. For change-managed entities, prepare the version sibling row to be written. Compute the next version_serial for this entity. Set parent version pointer to current active version.

Step 7, Change Management Routing. Evaluate approval_rule policy rows against the entity type, namespace, fields being changed, and metadata. Compute required approver groups. Write change_set_approval_required rows. Determine whether this routes to auto-approval or human approval based on matching auto-approval policies.

Step 8, Audit Logging. Construct audit_log_entry with all fields from the spec — acting user, target entity, operation type, request summary, response summary, result, client IP, request ID, correlation ID, change_set_id where applicable, timestamp. This write is atomic with the operation outcome. The audit table has no UPDATE or DELETE grants for any role, enforced at DDL level.

Step 9, Execution. For direct writes (observations, evidence), apply the write. For change-set submissions, write change_set, change_set_field_change, and change_set_approval_required rows. For approvals and rejections, update change_set state. For change-set field change applications (by the executor runner), apply the actual field update and write the version sibling row.

Step 10, Response. Return the result with metadata — entity data, version information, any validation warnings.

**The operations, grouped by class:**

Read operations: get_entity, get_entity_history, get_entity_at_time, search, get_dependencies, resolve_authority_pointer, change_set_view. All pass through auth layers. Search supports filter, join, projection, pagination, and freshness parameters — but not full-text search, not semantic search, not vector search. Structured queries only. The API is not a search engine.

Write-direct operations: write_observation (for pullers writing cached state), apply_change_set_field_change (for the executor runner applying approved changes).

Write-change-set operations: submit_change_set, emergency_apply, bulk_submit_change_set. These create change-set rows and route through the approval pipeline.

Change-management actions: approve_change_set, reject_change_set, cancel_change_set, mark_change_set_applied.

Stream operation: watch. Long-poll or WebSocket subscription to entity changes. On reconnect, the client fetches current state then streams from a resume token — always level-triggered backstop per the spec's principle.

**Concurrency control:** Every change_set_field_change carries the version stamp of the entity it was drafted against. At submit time, the API checks each touched entity's current version against the drafted-against version. Stale version means loud rejection — the submitter retrieves current values, reconciles, and resubmits. No silent overwrites.

**Runner report key enforcement:** When a runner writes to observation_cache_metric, observation_cache_state, observation_cache_config, runner_job_output_var, or evidence_record, the API looks up runner_report_key rows for that runner's spec and target table. If the submitted key isn't in the declared set, reject. If the value doesn't match the report_key_data_json constraints, reject. Fail closed.

**Starting configuration for development:** The YAML auth backend ships with a default admin user and a default runner service account. A seed script creates the minimum policy rows needed for the API to function — a default access control policy granting the admin role full access, and a default approval rule routing schema changes to the admin. This means you can deploy opsdb-schema, then opsdb-api, and immediately start reading and writing through the API with no external dependencies beyond Postgres.

**Implementation language:** Go. The API is infrastructure software. Single binary. Minimal dependencies. Embeds the YAML auth backend so zero-dependency startup works. OIDC and Vault backends as optional plugins or compile-time flags.

**Deliverable:** A single binary, opsdb-api, that takes a Postgres connection string and a config file (specifying auth backend, listen address, TLS settings) and serves the full API surface.

---

### Component 3: Core Runners (opsdb-runners)

**What it does:** Ships the minimum set of runners needed to operate the OpsDB itself, plus a runner framework library that makes writing new runners trivial.

**The runner framework library (opsdb-runner-lib):**

Not a framework in the controlling sense — the spec explicitly forbids the library from owning the runner's main loop. It's a toolkit that provides the get/act/set lifecycle as composable functions.

The core loop a runner author writes:

```
func main():
    config = opsdb_runner_lib.init(runner_spec_name)
    
    while config.should_run():
        # GET phase - no side effects
        spec = config.api.get_entity("runner_spec", config.spec_id)
        targets = config.api.search("runner_service_target", 
                                     runner_spec_id=config.spec_id)
        # ... read whatever this runner needs
        
        # COMPUTE phase - no side effects  
        plan = compute_actions(spec, targets, observed_state)
        
        if config.dry_run:
            config.log_plan(plan)
            continue
        
        # ACT phase - side effects through libraries
        results = execute_plan(plan, config.libs)
        
        # SET phase - write results through API
        job_id = config.api.write_runner_job(results)
        for result in results:
            config.api.write_observation(result, job_id)
        
        config.wait_for_next_cycle()
```

The library provides: API client initialization with auth and correlation ID propagation. Structured logging with runner_job_id context. Retry with backoff and jitter. Dry-run mode support. Cycle timing and scheduling. Graceful shutdown handling. Bound enforcement — if the runner exceeds its declared time or memory bounds, the library logs what bound was hit and terminates cleanly.

The runner author writes compute_actions and execute_plan. Everything else is library.

**Runners that ship with the project:**

**Change-Set Executor.** The most critical runner. Reads change_sets with status "approved." For each approved change-set, reads the change_set_field_change rows, applies each field change through the API's apply_change_set_field_change operation, writes the version sibling rows, and marks the change-set as applied. This is the runner that closes the change-management loop — without it, approved changes sit in the queue forever. Gating mode: direct write after verifying executor authority and change-set in approved state. Trigger: polls for approved change-sets on a short interval, maybe every 30 seconds.

**Schema Executor.** Reads _schema_change_set rows with status "approved." Runs the opsdb-schema loader against the schema repo at the specified commit. Updates _schema_* tables. Marks the schema change-set as applied. This runner is how schema evolution happens in production — a schema change-set goes through approval, and this runner applies it. Gating mode: requires schema steward approval. Trigger: polls for approved schema change-sets.

**Reaper.** Reads retention_policy rows. For each policy, queries the target table for rows older than the retention horizon. For cached observation tables, deletes directly (these are not change-managed). For change-managed entities, soft-deletes by setting is_active to false through a change-set if policy requires governance, or directly if policy permits. Writes runner_job recording what was reaped. Trigger: scheduled, maybe daily.

**Emergency Review Monitor.** Reads change_sets where is_emergency is true and change_set_emergency_review rows where status is pending. If the review window (default 72 hours) has elapsed without review, writes a compliance_finding and escalates through the escalation path. Trigger: scheduled, maybe hourly.

**Notification Runner.** Reads state transitions that require notification — change-sets moving to pending_approval, emergency changes filed, compliance findings created, escalation timeouts. Sends notifications through configured channels. Ships with email and webhook backends. Slack and PagerDuty as optional plugins. Gating mode: direct write (writes observation data about notifications sent). Trigger: polls for notification-worthy state transitions.

Each of these runners ships with its runner_spec YAML definition, including declared capabilities, target scopes, report keys, schedules, and bounds. A seed script registers them in the OpsDB through the API.

**Deliverable:** The opsdb-runner-lib package (Go module), plus five runner binaries, plus their runner_spec YAML definitions, plus a seed script that registers them.

---

### Component 4: Authority Importers (opsdb-import)

**What it does:** Reads your live infrastructure and writes it to the OpsDB through the API. Not a one-time migration tool — these are pullers that run continuously as runners, following the get/act/set pattern.

**The importer architecture:**

Each importer is a runner built on opsdb-runner-lib. It reads from one authority (AWS, GCP, Kubernetes, Okta, PagerDuty, etc.), transforms the data to the OpsDB schema, and writes through the API using write_observation for cached state or submit_change_set for configuration data that should be governed.

The critical design decision: **importers write to observation cache tables by default, not to the main entity tables.** The first import of your AWS infrastructure writes to observation_cache_state with entity_type "cloud_resource" and state_key per resource. This lets the team see their infrastructure in the OpsDB immediately without committing to the OpsDB as source of truth for that data yet.

When the team is ready to promote cached observations to governed entities — meaning AWS resource definitions live in the OpsDB and changes go through change-sets — a promotion runner reads observation_cache_state rows and submits change-sets to create the corresponding cloud_resource rows. This is a deliberate transition, not automatic. The team decides when each domain moves from "observed" to "governed."

**AWS Importer (opsdb-import-aws):**

Reads from AWS APIs using the standard SDK. Requires IAM credentials with read-only access — the importer never writes to AWS, only reads.

What it imports, mapped to schema entities:

EC2 instances become cloud_resource rows with cloud_resource_type "ec2_instance" and cloud_data_json containing instance_type, ami_id, vpc_id, subnet_id, security_group_ids, and other instance metadata as flat fields per the DSNC flattening rules — these are per-row metadata of the parent.

Security group memberships break out to bridge tables because they have independent lifecycle and there are N of them per instance. This is the list-of-N test applied correctly.

EBS volumes are themselves cloud_resource rows with type "ebs_volume" linked through a cloud_resource_to_cloud_resource relationship, not flattened into the EC2 JSON. This is the correct shape per the spec.

RDS instances become cloud_resource rows with type "rds_database." S3 buckets with type "s3_bucket." IAM roles with type "iam_role." VPCs, subnets, load balancers, Route53 zones — each gets its discriminator type and typed JSON payload.

Cloud accounts become cloud_account rows. Regions and availability zones map to location rows in the location hierarchy.

The importer handles pagination, rate limiting, and multi-region scanning. It writes _observed_time and _authority_id on every observation row. It declares runner_report_key entries for every state_key it writes.

**GCP Importer (opsdb-import-gcp):**

Same pattern, different APIs. GCE instances, Cloud SQL, GCS buckets, GKE clusters (which also populate k8s_cluster rows), IAM service accounts, VPCs, Cloud DNS zones. Maps to the same schema entities using different discriminator values.

**Kubernetes Importer (opsdb-import-k8s):**

Reads from the Kubernetes API using in-cluster service account or kubeconfig.

Clusters become k8s_cluster rows linked to a service row (a Kubernetes cluster is a service in OpsDB's model). Nodes become k8s_cluster_node rows linked to machine rows. Namespaces become k8s_namespace rows. Deployments, StatefulSets, DaemonSets, Jobs, CronJobs become k8s_workload rows with the workload_type discriminator and workload_data_json containing the spec details. Pods become k8s_pod rows linked to megavisor_instance (since a pod is a substrate unit). Helm releases become k8s_helm_release rows. ConfigMaps become k8s_config_map rows with values written to configuration_variable. Secrets become k8s_secret_reference rows — pointer to the secret, never the value.

The K8s importer uses the watch API for near-real-time state tracking. On reconnect it does a full list (level-triggered backstop) then resumes watching. This is the spec's reconciler pattern applied to the importer itself.

**Identity Importer (opsdb-import-identity):**

Reads from Okta, Azure AD, or LDAP. Users become ops_user rows. Groups become ops_group rows. Group memberships become ops_group_member rows. Role assignments become ops_user_role_member rows.

Ships with Okta and Azure AD backends. LDAP as a third option for orgs still running on-prem directory services.

**Monitoring Importer (opsdb-import-monitoring):**

Reads from Prometheus or Datadog. Does not import raw metrics — the OpsDB is not a time-series database. Imports metric metadata, alert definitions, and current alert state.

Prometheus scrape configs become prometheus_config and prometheus_scrape_target rows. Alert rules become monitor and alert rows. Currently firing alerts become alert_fire rows. Metric metadata (what metrics exist, their labels, their associated services) writes to observation_cache_metric.

**On-Call Importer (opsdb-import-oncall):**

Reads from PagerDuty or Opsgenie. Schedules become on_call_schedule rows. Current assignments become on_call_assignment rows. Escalation policies become escalation_path and escalation_step rows.

**Secret Metadata Importer (opsdb-import-secrets):**

Reads from Vault or AWS Secrets Manager. Imports secret paths and metadata only — never values. Secret paths become authority_pointer rows with pointer_type "secret." Version metadata and rotation timestamps imported for tracking rotation compliance.

**The initial import experience:**

An org deploying OpsDB for the first time runs the importers against their live environment. Within hours they have a queryable operational data store containing their full infrastructure — every cloud resource, every Kubernetes workload, every service, every on-call schedule, every alert definition, every secret path. Not because someone hand-entered it, but because the importers read it from the authoritative sources.

This is the moment the value proposition becomes tangible. The org can immediately query across domains that were previously siloed. "Show me every service running on nodes in us-east-1, who is on call for each, and what alerts are configured" is one query across data that previously lived in AWS, Kubernetes, PagerDuty, and Datadog.

**Deliverable:** Importer binaries — opsdb-import-aws, opsdb-import-gcp, opsdb-import-k8s, opsdb-import-identity, opsdb-import-monitoring, opsdb-import-oncall, opsdb-import-secrets. Each with runner_spec YAML definitions, report key declarations, and documentation mapping source fields to schema entities. Plus a quickstart script that deploys all relevant importers for a given environment profile (aws-k8s, gcp-k8s, aws-bare-metal, etc).

---

## Deployment Sequence

The getting-started experience for an org:

**Step 1:** Clone the schema repo. Run opsdb-schema against a fresh Postgres database. Database is ready. Takes under a minute.

**Step 2:** Run the API seed script to create default admin user and base policies. Start opsdb-api. API is serving. Takes under a minute.

**Step 3:** Configure credentials for your authorities — AWS access keys, kubeconfig, Okta API token, PagerDuty API key, Prometheus endpoint. Run the quickstart script that deploys the relevant importers. Data starts flowing. Within an hour your full infrastructure is queryable in the OpsDB.

**Step 4:** Deploy the core runners — change-set executor, reaper, emergency review monitor, notification runner. The OpsDB is now self-managing.

**Step 5:** Start writing approval rules as change-set submissions through the API. Register your custom runners. Transition from "observed" to "governed" for domains you're ready to manage through OpsDB.

From zero to queryable operational data in an afternoon. From queryable to governed in the following weeks, at whatever pace the org is comfortable with. The importers give you immediate value — unified visibility — while the change-management pipeline gives you long-term value — governed operations — and you adopt each at your own pace.

The up-front cost argument is over. If the FOSS project exists, the cost of trying it is one afternoon and a Postgres instance.
