# OpsDB Authority Importers

## What This Document Covers

Importers are the bridge between the world as it exists today and OpsDB as the system of record. They read your live infrastructure — cloud resources, Kubernetes clusters, identity providers, monitoring systems, on-call schedules, secret metadata — and write it to OpsDB through the API.

Importers are runners. They follow the same get/act/set pattern, use the same shared libraries, satisfy the same three disciplines (idempotency, level-triggered, bound everything), and produce the same audit trail as every other runner. There is no special importer mechanism — importing is just pulling, and pulling is what puller runners do.

This document specifies how importers work, how initial population differs from ongoing operation, how observed data gets promoted to governed configuration, and what ships with the FOSS project.

---

## The Two-Phase Pattern

Every authority importer operates in two modes, and the distinction matters for understanding how OpsDB takes over operational coordination from existing tools.

### Phase One: Observation

When an importer first runs against your infrastructure, it writes to observation cache tables. Your EC2 instances become rows in `observation_cache_state` with entity_type "cloud_resource" and a state_key per resource. Your Kubernetes pods become observation cache rows. Your PagerDuty schedules become observation cache rows. Everything is observed, nothing is governed.

This is deliberate. The importer is recording what exists without claiming authority over it. OpsDB can answer questions — "show me every EC2 instance in us-east-1" — but changes to those instances still flow through the existing tools (AWS console, Terraform, kubectl). OpsDB is watching, not controlling.

Observation writes use the direct write gating mode. No change sets. No approvals. The puller authenticates with its service account, the API validates the write against the runner's report key declarations, the observation is recorded, and an audit entry is produced. Fast, continuous, and audited.

### Phase Two: Promotion

When the organization is ready, observed data gets promoted to governed entities. This is the transition from "OpsDB watches" to "OpsDB coordinates."

Promotion is not automatic. It is a deliberate organizational decision, executed per domain, at whatever pace the team is comfortable with. A promotion runner reads observation cache rows and submits change sets to create the corresponding entity rows — `cloud_resource` rows from cloud observation cache, `k8s_workload` rows from Kubernetes observation cache, `service` rows from service observation cache.

Once promoted, those entities are change-managed. Modifications flow through change sets with validation, approval routing, and audit trails. The observation cache continues to be updated by the puller (recording what actually exists in the world), and drift detectors compare the governed configuration against the observed reality, filing findings or auto-correcting when they diverge.

The importer itself doesn't change between phase one and phase two. It keeps pulling from the authority and writing to observation cache. What changes is that the organization has created governed entity rows that represent desired state, and the observation cache now serves as the comparison surface for drift detection.

---

## How Importers Work

An importer follows the standard runner lifecycle.

### Get Phase

The importer reads its own `runner_spec_version` from OpsDB to get its configuration — which authority to read from, which resource types to import, which regions or namespaces to scan, what bounds apply. It reads authority connection details from OpsDB (the `authority` row identifying the target system). It reads its report key declarations to know which observation keys it's authorized to write.

No side effects. The importer is gathering its operating parameters.

### Act Phase

The importer reads from the external authority. For AWS, this means calling the AWS API with read-only credentials. For Kubernetes, this means querying the Kubernetes API or watching for changes. For PagerDuty, this means reading schedules and assignments through the PagerDuty API.

The importer handles pagination, rate limiting, multi-region scanning, and error recovery. It transforms the authority's response into the OpsDB schema shape — mapping AWS resource attributes to `cloud_data_json` fields, mapping Kubernetes pod specs to observation cache state keys, mapping PagerDuty schedules to on-call assignment structures.

The transformation follows the DSNC flattening rules. Per-row metadata of the parent (instance type, AMI ID, VPC ID for an EC2 instance) flattens into the typed JSON payload. Data with independent lifecycle, independent identity, or N-of-them (security group memberships, attached EBS volumes) breaks out to bridge tables or separate entity rows.

### Set Phase

The importer writes to OpsDB through the API. Each observation carries `_observed_time` (when the importer sampled the data), `_authority_id` (which authority it came from), and `_puller_runner_job_id` (which runner job produced it). A `runner_job` row records the overall import cycle — how many resources scanned, how many observations written, whether the cycle completed successfully.

Every write goes through the API gate. Every write is validated against the runner's report key declarations. Every write produces an audit log entry.

---

## Initial Population

The first time importers run against a live environment, the goal is to get the complete operational picture into OpsDB as quickly as possible. This is not a migration — it's a snapshot. The existing tools keep running. OpsDB is additive.

### What Happens

The organization configures credentials for their authorities — AWS access keys (read-only), kubeconfig, Okta API token, PagerDuty API key, Prometheus endpoint, Vault token (metadata-only access). They start the importers. Within minutes to hours (depending on the size of the infrastructure), OpsDB contains observation cache rows covering the full operational surface.

At this point OpsDB can answer cross-domain questions that previously required touring multiple tools. "Show me every service, its cloud resources, who is on call, and what alerts are configured" is a query across observation cache rows that correlate by entity type and ID. The data is all observed, not governed, but the unified view is immediately valuable.

### What Doesn't Happen

The importers don't modify anything in the existing infrastructure. They don't create change sets. They don't trigger approval workflows. They don't send notifications. They read and record. The existing operational tools continue to be the path through which changes are made.

No data is lost or overwritten in the import. The observation cache tables are keyed by authority, entity type, entity ID, and state key. Each write overwrites the previous observation for that key (this is the cache behavior — latest observation wins), but the audit log records every write, so the history of observations is always recoverable.

### Seeding Entity Rows

After the initial observation import, the organization needs basic entity infrastructure in OpsDB before promotion can happen — site rows, cloud provider rows, cloud account rows, authority rows, and the identity rows that support ownership and approval.

These seed rows are created through the seed scripts in the DOS configuration, not through the importers. The seed scripts submit API operations that create the foundational rows: the site, the admin user, the base policies, the cloud providers and accounts, the authority rows for each external system.

Some of these seed rows can be derived from observation data. The importer sees AWS resources in account 123456789012 — the seed script creates a `cloud_account` row for that account. The importer sees Kubernetes namespaces in a cluster named prod-east — the seed script creates a `k8s_cluster` row. The seed scripts are run once during initial setup and produce the scaffolding that promotion builds on.

---

## Promotion: From Observed to Governed

Promotion is the transition from "OpsDB sees this resource" to "OpsDB governs this resource." It is the moment the organization decides that a domain's configuration lives in OpsDB and changes to it flow through change management.

### The Promotion Runner

A promotion runner reads observation cache rows for a specific domain and submits change sets to create or update the corresponding governed entity rows. For cloud resources, it reads `observation_cache_state` rows with entity_type "cloud_resource" and submits change sets creating `cloud_resource` rows with the observed attributes.

The promotion runner is itself a runner following the standard pattern. It reads observation cache (get), computes which entities need to be created or updated (compute), and submits change sets (set). The change sets flow through the normal approval pipeline — the organization's approval rules determine who approves the promotion.

Promotion can be incremental. The organization might promote cloud resources in one region first, then expand. They might promote Kubernetes workloads in the production namespace but leave staging as observation-only. They might promote identity data (users, groups, roles) before promoting infrastructure. Each domain promotes independently at whatever pace makes sense.

### What Changes After Promotion

Before promotion, the puller writes to observation cache and that's the end of the story. After promotion, the observation cache continues to be updated (recording what actually exists), but now governed entity rows also exist (recording what should exist). This creates the desired-vs-observed comparison surface that drift detectors use.

If someone changes an EC2 instance through the AWS console after promotion, the puller records the new state in observation cache. The drift detector compares the governed `cloud_resource` row against the observation and detects the divergence. Depending on policy, it either auto-corrects (submits a change set reverting the observation to match the governed state), files a finding (someone changed production outside the governed process), or proposes a change set to update the governed state to match reality.

This is where the governance value emerges. Pre-promotion, changes happen anywhere and OpsDB just watches. Post-promotion, changes to governed entities flow through change sets with validation, approval, and audit. Unauthorized changes are detected within one drift detection cycle and surfaced.

### Promotion Is Not One-Time

Promotion is not a one-time event. The importers keep running forever. The observation cache keeps being updated. The comparison between governed and observed continues on every drift detection cycle.

New resources appear in the authority (someone launches a new EC2 instance). The puller records it in observation cache. The promotion runner sees an observation with no corresponding governed entity and proposes a change set to create one. The change set flows through approval. The new resource is now governed.

Resources disappear from the authority (an instance is terminated). The puller stops recording observations for it (the observation goes stale and eventually the reaper cleans it up). The drift detector notices the governed entity has no corresponding observation and files a finding — "governed cloud resource no longer exists in authority."

The importers provide the continuous bridge between world-as-it-is and OpsDB-as-it-should-be. Promotion makes the bridge bidirectional — not just watching, but governing.

---

## Shipped Importers

### AWS Importer (opsdb_import_aws)

Reads from AWS APIs using the standard SDK with read-only IAM credentials. Never writes to AWS.

**EC2 instances** become observation cache state rows with entity_type "cloud_resource", a state_key encoding the instance ID, and a state_data_json containing instance type, AMI ID, VPC ID, subnet ID, private and public IP addresses, security group IDs, instance state, launch time, and tags. On promotion, each instance becomes a `cloud_resource` row with cloud_resource_type "ec2_instance" and cloud_data_json containing the instance-specific attributes as flat fields per the DSNC flattening rules.

Security group memberships are N-per-instance with independent lifecycle. They are recorded as separate observation keys and on promotion become bridge table rows, not flattened fields in the EC2 JSON. This is the list-of-N test applied correctly.

**EBS volumes** are themselves cloud resources, not attributes of EC2 instances. They are recorded as separate observation cache rows with their own state_keys and on promotion become `cloud_resource` rows with cloud_resource_type "ebs_volume." The attachment relationship between an EC2 instance and its EBS volumes is recorded as a separate observation and on promotion becomes a typed relationship between cloud resources.

**RDS instances** become observation cache rows and on promotion become `cloud_resource` rows with type "rds_database." The importer records engine type, engine version, instance class, storage size, multi-AZ status, read replica configuration, parameter group settings, and security group associations.

**S3 buckets** become cloud_resource rows with type "s3_bucket." The importer records region, versioning status, encryption configuration, lifecycle rules, and access policy summary (not the full policy document — that stays in AWS as the authority).

**IAM roles** become cloud_resource rows with type "iam_role." The importer records the role name, ARN, attached policy names, trust policy summary, and creation date. Policy documents stay in AWS.

**VPCs, subnets, load balancers, Route53 zones** each become cloud_resource rows with their respective discriminator types. The importer handles pagination across all resource types, respects AWS API rate limits, and scans multiple regions as configured in the runner spec.

Cloud accounts become `cloud_account` rows (via seed scripts or promotion). Regions and availability zones map to `location` rows in the location hierarchy.

The importer declares report keys for every state_key it writes — host_cpu, host_memory, ec2_instance_state, rds_status, and so on. The API rejects any key the importer hasn't declared.

### GCP Importer (opsdb_import_gcp)

Same pattern as the AWS importer, different APIs.

**GCE instances** become cloud_resource rows with type "gce_instance." **Cloud SQL instances** become "cloud_sql_instance." **GCS buckets** become "gcs_bucket." **GKE clusters** trigger both a cloud_resource row and a k8s_cluster row (a GKE cluster is both a cloud resource and a Kubernetes cluster in OpsDB's model). **IAM service accounts** become "service_account." **VPCs and Cloud DNS zones** map to their respective types.

The importer uses GCP client libraries, handles pagination and quota, and scans multiple projects as configured.

### Kubernetes Importer (opsdb_import_k8s)

Reads from the Kubernetes API using in-cluster service account credentials or kubeconfig.

**Clusters** are recorded as observation cache rows and on promotion become `k8s_cluster` rows linked to `service` rows (a Kubernetes cluster is a service in OpsDB's model). The importer records distribution type (EKS, GKE, AKS, vanilla), version, API endpoint, and node count.

**Nodes** become observation cache rows and on promotion become `k8s_cluster_node` rows linking to `machine` rows. The importer records node role (control plane, worker, etcd, ingress), schedulability, capacity (CPU, memory, pods), allocatable resources, conditions, and the underlying instance ID (linking back to the cloud resource if applicable).

**Namespaces** become `k8s_namespace` rows. The importer records labels, annotations, and resource quotas.

**Workloads** (Deployments, StatefulSets, DaemonSets, Jobs, CronJobs, ReplicaSets) become observation cache rows and on promotion become `k8s_workload` rows with the workload_type discriminator. The importer records replica count, image references, resource requests and limits, environment variables (names only, not values for secrets), volume mounts, and pod template spec summary.

**Pods** become observation cache rows with frequent updates (pod state changes often). On promotion they become `k8s_pod` rows linked to `megavisor_instance` (a pod is a substrate unit in the unified hierarchy). The importer records pod phase, container statuses, node assignment, IP addresses, start time, and restart counts.

**Helm releases** become observation cache rows and on promotion become `k8s_helm_release` rows. The importer records chart name, chart version, release values (as configuration variables), and installation time.

**ConfigMaps** become observation cache rows and on promotion become `k8s_config_map` rows with their contents stored as `configuration_variable` rows.

**Secrets** become `k8s_secret_reference` rows — pointer to the secret in the cluster, never the value. The importer records the secret name, namespace, type (opaque, TLS, dockerconfigjson, service account token, basic auth), and which secret backend holds the actual value. Secret values never enter OpsDB.

**Services** (Kubernetes Service objects) become `k8s_service` rows with optional links to OpsDB `service` rows when the Kubernetes service corresponds to a tracked operational service.

The Kubernetes importer uses the watch API for near-real-time state tracking. On startup or reconnect, it does a full list operation (level-triggered backstop) to establish current state, then watches for incremental changes. If the watch stream disconnects, the importer re-lists and resumes. Missed events are impossible because the re-list captures current state regardless of what happened during the disconnection.

### Identity Importer (opsdb_import_identity)

Reads from identity providers. Ships with Okta, Azure AD, and LDAP backends.

**Users** become observation cache rows and on promotion become `ops_user` rows. The importer records username, full name, email, status (active, suspended, deprovisioned), group memberships, and last login time.

**Groups** become `ops_group` rows. The importer records group name, description, and membership list.

**Group memberships** become `ops_group_member` bridge rows linking groups to users.

**Role assignments** — if the identity provider has a role concept that maps to OpsDB's `ops_user_role` — become `ops_user_role_member` rows.

The identity importer is critical for the authorization model. Without accurate identity data, the five-layer auth model cannot evaluate group memberships or role assignments. This importer typically runs on a short cycle (every few minutes) to keep identity data current.

### Monitoring Importer (opsdb_import_monitoring)

Reads from monitoring systems. Ships with Prometheus and Datadog backends.

The monitoring importer does not import raw metrics — OpsDB is not a time-series database. It imports metric metadata, alert definitions, and current alert state.

**Prometheus scrape configurations** become `prometheus_config` and `prometheus_scrape_target` rows. The importer reads the Prometheus configuration API or config file to discover which targets are being scraped, at what intervals, with what labels.

**Alert rules** become `monitor` and `alert` rows. The importer records alert name, expression, severity, labels, annotations, and which service the alert is scoped to.

**Currently firing alerts** become `alert_fire` rows. The importer records fired time, alert state, labels, and annotations. Cleared alerts get their `cleared_time` updated.

**Metric metadata** — which metrics exist, their types (counter, gauge, histogram), their labels, and their associated services — writes to `observation_cache_metric`. This enables queries like "what metrics are available for this service" without hitting Prometheus directly.

### On-Call Importer (opsdb_import_oncall)

Reads from PagerDuty or Opsgenie.

**Schedules** become `on_call_schedule` rows linking roles to schedule definitions.

**Current and upcoming assignments** become `on_call_assignment` rows with start and stop times. The importer maintains a rolling window of assignments so that "who is on call right now" and "who is on call this weekend" are both answerable from OpsDB data.

**Escalation policies** become `escalation_path` and `escalation_step` rows. Each step records its type (notify role, notify user, page role, page user, wait, branch on acknowledge), its order, and its configuration.

**Service-to-escalation mappings** become `service_escalation_path` rows with severity thresholds.

### Secret Metadata Importer (opsdb_import_secrets)

Reads from Vault, AWS Secrets Manager, or GCP Secret Manager.

This importer records only metadata — paths, version numbers, creation times, rotation timestamps, expiration dates. It never reads or records secret values.

Secret paths become `authority_pointer` rows with pointer_type "secret." Version metadata enables tracking whether secrets are being rotated on schedule. Expiration dates enable the certificate and credential verification runners to check compliance without accessing the actual secrets.

The secret metadata importer enables a complete picture of credential hygiene without any credential exposure. "Show me every secret that hasn't been rotated in 90 days" is a query against metadata, not against secret values.

---

## Ongoing Operation

After initial population and any promotions the organization chooses to make, importers continue running on their configured schedules. This is not a separate mode — the same runner code runs the same way. The only difference is that observation cache rows now accumulate history in the audit log, governed entities exist for drift comparison, and the operational picture gets richer over time.

### Freshness

Every observation carries `_observed_time`. Downstream consumers (drift detectors, verifiers, dashboards, queries) use freshness to determine whether the observation is current enough to act on. The API's search operation supports a `max_staleness_seconds` parameter that filters out observations older than the threshold.

If an importer stops running (misconfiguration, credential expiration, authority outage), observations go stale. Consumers that check freshness see stale data and respond appropriately — drift detectors skip stale comparisons, dashboards show staleness warnings, verifiers record the freshness gap in evidence. Nothing silently acts on old data.

### Authority Failure

When an authority is unreachable (AWS API outage, Kubernetes API server down, PagerDuty maintenance window), the importer records the failure in its runner_job and continues to the next cycle. Existing observations remain in the cache with their last `_observed_time`. Consumers see the aging freshness and can query runner_job history to understand why observations stopped updating.

The importer does not clear the cache on authority failure — stale data is better than no data for most operational purposes. The freshness metadata makes the staleness visible so consumers can make informed decisions.

### Schema Evolution

When the OpsDB schema evolves (new fields added to cloud_resource, new discriminator values added to cloud_resource_type), importers pick up the changes naturally. New fields that the importer can populate from authority data get populated on the next cycle. New discriminator values that match new resource types the importer discovers get used immediately. The importer reads its own report key declarations from OpsDB, and those declarations can be updated through change sets to cover new observation keys.

No importer code change is needed for schema additions that the importer already has data for. Code changes are only needed when a new authority endpoint must be queried or a new data transformation is required.

---

## Writing a New Importer

Building an importer for a new authority follows the standard runner design process.

**Step 1: Identify the authority.** What system holds the data? What API does it expose? What credentials are needed (read-only access is sufficient for importers)?

**Step 2: Map authority data to schema entities.** Which OpsDB entity types and observation cache keys correspond to the authority's resources? Apply the DSNC flattening rules — per-row metadata flattens into typed JSON, independent-lifecycle data breaks out to bridge tables, lists of N become separate rows.

**Step 3: Declare report keys.** Every observation key the importer will write must be declared in `runner_report_key` rows. The API rejects undeclared keys. This forces the team to enumerate exactly what the importer will write before it writes anything.

**Step 4: Build the importer.** Use the runner library for lifecycle management. Use the API client library for OpsDB writes. Use standard HTTP or SDK clients for authority access (if no world-side library exists for this authority yet). Handle pagination, rate limiting, and error recovery. Transform authority responses to schema shape.

**Step 5: Register and deploy.** Create the runner spec through a change set. Configure credentials (pointers to environment variables or Vault paths, never values in configuration files). Deploy. The importer starts running on its configured schedule and observations start flowing.

**Step 6: Validate.** Query the observation cache. Do the observations match the authority's actual state? Are the entity types and state keys correct? Are the typed JSON payloads well-shaped? Run the DSNC tests — are lists of N properly broken out? Are per-row metadata properly flattened?

**Step 7: Promote when ready.** When the team is confident the observations are correct and complete, run the promotion process for this domain. Governed entities are created. Drift detection begins. The domain is now OpsDB-coordinated.

---

## The Quickstart Experience

An organization evaluating OpsDB runs the importers against their live environment and gets immediate value.

Clone the repo. Build the binaries. Stand up Postgres. Apply the schema. Start the API. Configure credentials for the authorities they care about. Run the quickstart script, which starts the relevant importers for their environment profile.

Within an hour, their full infrastructure is queryable in OpsDB. Every cloud resource, every Kubernetes workload, every on-call schedule, every alert definition, every secret path — unified in one place, queryable through one API, with a complete audit trail of every observation.

This is the moment the value proposition becomes tangible. The team runs a query that joins cloud resources, Kubernetes workloads, on-call schedules, and alert definitions — data that previously lived in four different tools with no connection between them. The query works. The data is there. The audit trail shows when each observation was made and which runner produced it.

The team hasn't committed to anything yet. No governance, no change management, no approval workflows. Just visibility. The importers are watching. The data is flowing. The organization decides what to promote, when, and at what pace.

From watching to governing is a series of deliberate steps, not a cliff. Each step adds value. Each step is reversible (stop promoting and the observations continue without governance). The importers make the first step — the watching step — as cheap and fast as possible, so the organization can see the value before committing to the investment.
