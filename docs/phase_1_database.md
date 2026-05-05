## OpsDB FOSS Phase 1: Schema Engine

### Technical Document

---

### Goal

A single Go binary, `opsdb-schema`, that reads a YAML schema repository and idempotently applies it to Postgres. The binary enforces the spec's closed vocabulary, naming conventions, reserved field injection, versioning sibling generation, and evolution rules mechanically. No human discipline required — if it violates the spec, the tool rejects it.

---

### 1. Repository Layout

The schema repo follows the spec's bootstrap sequence. The loader processes files in a defined order so that foreign key references resolve against already-loaded entities.

```
opsdb-schema-repo/
├── meta/
│   └── _schema_meta.yaml          # meta-schema: what a valid entity file looks like
├── conventions/
│   └── reserved.yaml              # universal reserved fields + governance fields
├── directory.yaml                 # ordered import list
└── domains/
    ├── 01_identity/
    │   ├── site.yaml
    │   ├── location.yaml
    │   ├── ops_user.yaml
    │   ├── ops_group.yaml
    │   ├── ops_group_member.yaml
    │   ├── ops_user_role.yaml
    │   └── ops_user_role_member.yaml
    ├── 02_substrate/
    │   ├── hardware_component.yaml
    │   ├── hardware_port.yaml
    │   ├── hardware_set.yaml
    │   ├── hardware_set_component.yaml
    │   ├── hardware_set_instance.yaml
    │   ├── hardware_set_instance_port_connection.yaml
    │   ├── megavisor.yaml
    │   ├── megavisor_instance.yaml
    │   ├── cloud_provider.yaml
    │   ├── cloud_account.yaml
    │   ├── cloud_resource.yaml
    │   ├── storage_resource.yaml
    │   ├── platform.yaml
    │   └── machine.yaml
    ├── 03_service/
    │   ├── package.yaml
    │   ├── package_interface.yaml
    │   ├── package_connection.yaml
    │   ├── service.yaml
    │   ├── service_package.yaml
    │   ├── service_interface_mount.yaml
    │   ├── service_connection.yaml
    │   ├── host_group.yaml
    │   ├── host_group_machine.yaml
    │   ├── host_group_package.yaml
    │   ├── site_location.yaml
    │   ├── service_level.yaml
    │   └── service_level_metric.yaml
    ├── 04_kubernetes/
    │   ├── k8s_cluster.yaml
    │   ├── k8s_cluster_node.yaml
    │   ├── k8s_namespace.yaml
    │   ├── k8s_workload.yaml
    │   ├── k8s_pod.yaml
    │   ├── k8s_helm_release.yaml
    │   ├── k8s_config_map.yaml
    │   ├── k8s_secret_reference.yaml
    │   └── k8s_service.yaml
    ├── 05_authority/
    │   ├── authority.yaml
    │   ├── authority_pointer.yaml
    │   ├── service_authority_pointer.yaml
    │   ├── machine_authority_pointer.yaml
    │   ├── k8s_cluster_authority_pointer.yaml
    │   └── cloud_resource_authority_pointer.yaml
    ├── 06_schedule/
    │   ├── schedule.yaml
    │   ├── runner_schedule.yaml
    │   ├── credential_rotation_schedule.yaml
    │   ├── certificate_expiration_schedule.yaml
    │   ├── compliance_audit_schedule.yaml
    │   ├── manual_operation_schedule.yaml
    │   └── manual_operation.yaml
    ├── 07_policy/
    │   ├── policy.yaml
    │   ├── service_policy.yaml
    │   ├── machine_policy.yaml
    │   ├── k8s_namespace_policy.yaml
    │   ├── cloud_account_policy.yaml
    │   ├── security_zone.yaml
    │   ├── security_zone_membership_service.yaml
    │   ├── security_zone_membership_machine.yaml
    │   ├── security_zone_membership_k8s_namespace.yaml
    │   ├── data_classification.yaml
    │   ├── retention_policy.yaml
    │   ├── approval_rule.yaml
    │   ├── escalation_path.yaml
    │   ├── escalation_step.yaml
    │   ├── service_escalation_path.yaml
    │   ├── change_management_rule.yaml
    │   ├── compliance_regime.yaml
    │   ├── compliance_scope_service.yaml
    │   └── compliance_scope_data_classification.yaml
    ├── 08_docs/
    │   ├── service_ownership.yaml
    │   ├── machine_ownership.yaml
    │   ├── k8s_cluster_ownership.yaml
    │   ├── cloud_resource_ownership.yaml
    │   ├── service_stakeholder.yaml
    │   ├── runbook_reference.yaml
    │   ├── service_runbook_reference.yaml
    │   ├── dashboard_reference.yaml
    │   └── service_dashboard_reference.yaml
    ├── 09_runner/
    │   ├── runner_spec.yaml
    │   ├── runner_capability.yaml
    │   ├── runner_machine.yaml
    │   ├── runner_instance.yaml
    │   ├── runner_service_target.yaml
    │   ├── runner_host_group_target.yaml
    │   ├── runner_k8s_namespace_target.yaml
    │   ├── runner_cloud_account_target.yaml
    │   ├── runner_job.yaml
    │   ├── runner_job_target_machine.yaml
    │   ├── runner_job_target_service.yaml
    │   ├── runner_job_target_k8s_workload.yaml
    │   ├── runner_job_target_cloud_resource.yaml
    │   └── runner_job_output_var.yaml
    ├── 10_monitoring/
    │   ├── monitor.yaml
    │   ├── monitor_machine_target.yaml
    │   ├── monitor_service_target.yaml
    │   ├── monitor_k8s_workload_target.yaml
    │   ├── monitor_cloud_resource_target.yaml
    │   ├── prometheus_config.yaml
    │   ├── prometheus_scrape_target.yaml
    │   ├── monitor_level.yaml
    │   ├── alert.yaml
    │   ├── alert_dependency.yaml
    │   ├── alert_fire.yaml
    │   ├── on_call_schedule.yaml
    │   └── on_call_assignment.yaml
    ├── 11_observation/
    │   ├── observation_cache_metric.yaml
    │   ├── observation_cache_state.yaml
    │   └── observation_cache_config.yaml
    ├── 12_config/
    │   └── configuration_variable.yaml
    ├── 13_change_mgmt/
    │   ├── change_set.yaml
    │   ├── change_set_field_change.yaml
    │   ├── change_set_approval_required.yaml
    │   ├── change_set_approval.yaml
    │   ├── change_set_rejection.yaml
    │   ├── change_set_validation.yaml
    │   ├── change_set_emergency_review.yaml
    │   └── change_set_bulk_membership.yaml
    ├── 14_audit/
    │   ├── audit_log_entry.yaml
    │   ├── evidence_record.yaml
    │   ├── evidence_record_service_target.yaml
    │   ├── evidence_record_machine_target.yaml
    │   ├── evidence_record_credential_target.yaml
    │   ├── evidence_record_certificate_target.yaml
    │   ├── evidence_record_compliance_regime_target.yaml
    │   ├── evidence_record_manual_operation_target.yaml
    │   ├── compliance_finding.yaml
    │   └── compliance_finding_target_service.yaml
    └── 15_schema_meta/
        ├── _schema_version.yaml
        ├── _schema_change_set.yaml
        ├── _schema_entity_type.yaml
        ├── _schema_field.yaml
        └── _schema_relationship.yaml
```

The `directory.yaml` lists domains in dependency order:

```yaml
imports:
  - domains/01_identity
  - domains/02_substrate
  - domains/03_service
  - domains/04_kubernetes
  - domains/05_authority
  - domains/06_schedule
  - domains/07_policy
  - domains/08_docs
  - domains/09_runner
  - domains/10_monitoring
  - domains/11_observation
  - domains/12_config
  - domains/13_change_mgmt
  - domains/14_audit
  - domains/15_schema_meta
```

Within each domain directory, files are loaded in alphabetical order. The numeric prefixes on domain directories establish cross-domain dependency order. Within a domain, entity file names are chosen so that alphabetical order respects intra-domain FK dependencies — entities referenced by others come first alphabetically (e.g., `authority.yaml` before `authority_pointer.yaml`).

---

### 2. Entity File Format

Each entity is one YAML file. The closed vocabulary is enforced — every key in the file must be from the recognized set. Any unrecognized key causes the loader to reject the file.

```yaml
# cloud_resource.yaml
name: cloud_resource
description: "Generic cloud resource with provider-specific typed payload"
category: substrate
versioned: true
soft_delete: true
hierarchical: false

fields:
  - name: cloud_account_id
    type: foreign_key
    references: cloud_account
    nullable: false
    description: "Owning cloud account"

  - name: location_id
    type: foreign_key
    references: location
    nullable: false
    description: "Cloud region or availability zone"

  - name: cloud_resource_type
    type: enum
    enum_values:
      - ec2_instance
      - gce_instance
      - azure_vm
      - s3_bucket
      - gcs_bucket
      - azure_blob_container
      - rds_database
      - cloud_sql_instance
      - azure_sql
      - lambda_function
      - cloud_run_service
      - azure_function
      - vpc
      - vnet
      - cloud_network
      - load_balancer
      - application_gateway
      - cloud_lb
      - cloudfront_distribution
      - cloud_cdn
      - azure_cdn
      - iam_role
      - service_account
      - azure_service_principal
      - route53_zone
      - cloud_dns_zone
      - azure_dns_zone
      - cloudwatch_log_group
      - cloud_logging_bucket
      - log_analytics_workspace
    nullable: false
    description: "Discriminator for cloud_data_json"

  - name: external_id
    type: varchar
    max_length: 512
    nullable: false
    description: "ARN, resource ID, or self-link"

  - name: name
    type: varchar
    max_length: 255
    nullable: false

  - name: cloud_data_json
    type: json
    json_type_discriminator: cloud_resource_type
    nullable: false
    description: "Provider-specific payload validated against registered schema"

  - name: provisioned_time
    type: datetime
    nullable: true

  - name: decommissioned_time
    type: datetime
    nullable: true

indexes:
  - fields: [cloud_account_id, cloud_resource_type]
  - fields: [cloud_resource_type]
  - fields: [external_id]
    unique: true

governance:
  _access_classification: true
  _retention_policy_id: true
```

---

### 3. Meta-Schema: The Closed Vocabulary

The meta-schema file (`meta/_schema_meta.yaml`) defines what constitutes a valid entity file. This is the one hardcoded structural definition — the loader validates every entity file against it.

**Allowed top-level keys:**

```
name, description, category, versioned, soft_delete, hierarchical, 
fields, indexes, governance
```

**Allowed field-level keys:**

```
name, type, nullable, description, default,
references,                          # foreign_key only
max_length, min_length,              # varchar only (max_length required)
max_length,                          # text only (optional)
min_value, max_value,                # int and float only
precision_decimal_places,            # float only
enum_values,                         # enum only
json_type_discriminator,             # json only (required)
unique,                              # any field
must_be_unique_within                # composite uniqueness scope
```

**The nine types:**

| Type | Postgres mapping | Required constraints |
|------|-----------------|---------------------|
| int | INTEGER | min_value, max_value optional |
| float | DOUBLE PRECISION | min_value, max_value, precision_decimal_places optional |
| varchar | VARCHAR(max_length) | max_length required, min_length optional |
| text | TEXT | max_length optional |
| boolean | BOOLEAN | none |
| datetime | TIMESTAMP WITHOUT TIME ZONE | none |
| json | JSONB | json_type_discriminator required |
| enum | VARCHAR with CHECK | enum_values required |
| foreign_key | INTEGER with FK constraint | references required |

**The three modifiers:** nullable (default false), default (literal only, not for foreign_key/datetime/json), unique.

**Forbidden in entity files:** regex patterns, expressions, conditionals, inheritance/extends, template variables, imports of other entity files. The loader rejects any file containing these.

---

### 4. Reserved Field Injection

The loader injects reserved fields mechanically based on entity file declarations. The entity file never declares these — they are added by the loader.

**Every table gets:**

```sql
id            SERIAL PRIMARY KEY,
created_time  TIMESTAMP NOT NULL DEFAULT NOW(),
updated_time  TIMESTAMP NOT NULL DEFAULT NOW()
```

**When `soft_delete: true`:**

```sql
is_active     BOOLEAN NOT NULL DEFAULT TRUE
```

**When `hierarchical: true`:**

```sql
parent_{entity_name}_id  INTEGER REFERENCES {entity_name}(id)
```

**When `versioned: true`, the loader generates a sibling table `{entity_name}_version`:**

```sql
CREATE TABLE {entity_name}_version (
    id                              SERIAL PRIMARY KEY,
    {entity_name}_id                INTEGER NOT NULL REFERENCES {entity_name}(id),
    version_serial                  INTEGER NOT NULL,
    parent_{entity_name}_version_id INTEGER REFERENCES {entity_name}_version(id),
    change_set_id                   INTEGER NOT NULL REFERENCES change_set(id),
    is_active_version               BOOLEAN NOT NULL DEFAULT FALSE,
    approved_for_production_time    TIMESTAMP,
    created_time                    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_time                    TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE({entity_name}_id, version_serial)
);
```

The sibling table is generated entirely from the `versioned: true` flag. No additional specification needed.

**When governance fields are declared:**

```yaml
governance:
  _requires_group: true
  _access_classification: true
  _retention_policy_id: true
  _audit_chain_hash: true
```

Each enabled field is injected with its spec-defined type:

```sql
_requires_group             VARCHAR(255),
_access_classification      VARCHAR(50),
_retention_policy_id        INTEGER REFERENCES retention_policy(id),
_audit_chain_hash           VARCHAR(128)
```

For observation cache tables, the loader injects `_observed_time`, `_authority_id`, and `_puller_runner_job_id` based on governance declarations.

---

### 5. Special Handling: audit_log_entry

The audit_log_entry entity gets unique DDL treatment. After creating the table, the loader executes:

```sql
REVOKE UPDATE, DELETE ON audit_log_entry FROM PUBLIC;
REVOKE UPDATE, DELETE ON audit_log_entry FROM opsdb_app_role;
REVOKE UPDATE, DELETE ON audit_log_entry FROM opsdb_admin_role;
```

This enforces append-only at the DDL level. No role, including the application role and admin role, can UPDATE or DELETE audit log rows. This is not a convention — it's a mechanical constraint.

The entity file signals this with a top-level declaration:

```yaml
name: audit_log_entry
append_only: true
```

The loader recognizes `append_only: true` and generates the REVOKE statements.

---

### 6. DDL Generation

The loader builds an internal representation of all entities, then generates Postgres DDL.

**Type mapping:**

```
int          → INTEGER
float        → DOUBLE PRECISION  
varchar      → VARCHAR({max_length})
text         → TEXT
boolean      → BOOLEAN
datetime     → TIMESTAMP WITHOUT TIME ZONE
json         → JSONB
enum         → VARCHAR(255)
foreign_key  → INTEGER
```

**Constraint generation:**

For int and float fields with min_value/max_value:
```sql
CONSTRAINT chk_{table}_{field}_range 
    CHECK ({field} >= {min_value} AND {field} <= {max_value})
```

For varchar with min_length:
```sql
CONSTRAINT chk_{table}_{field}_minlen 
    CHECK (LENGTH({field}) >= {min_length})
```

For enum fields:
```sql
CONSTRAINT chk_{table}_{field}_enum 
    CHECK ({field} IN ('value1', 'value2', ...))
```

For foreign_key fields:
```sql
CONSTRAINT fk_{table}_{field} 
    FOREIGN KEY ({field}) REFERENCES {referenced_table}(id)
```

For unique fields:
```sql
CONSTRAINT uq_{table}_{field} UNIQUE ({field})
```

For composite uniqueness (`must_be_unique_within`):
```sql
CREATE UNIQUE INDEX uix_{table}_{field1}_{field2} 
    ON {table} ({field1}, {field2});
```

For indexes declared in the entity file:
```sql
CREATE INDEX ix_{table}_{field1}_{field2} ON {table} ({field1}, {field2});
-- or if unique: true
CREATE UNIQUE INDEX uix_{table}_{field1}_{field2} ON {table} ({field1}, {field2});
```

**Nullable handling:** Fields are `NOT NULL` by default. When `nullable: true`, the NOT NULL constraint is omitted.

**Default handling:** When `default` is specified, `DEFAULT {literal_value}` is added. Boolean defaults render as `DEFAULT TRUE` or `DEFAULT FALSE`. String defaults render as `DEFAULT '{value}'`. Numeric defaults render as `DEFAULT {value}`.

---

### 7. Dependency Resolution and Apply Order

The loader builds a directed acyclic graph from foreign key references. Each entity is a node. Each foreign_key field creates an edge from the referencing entity to the referenced entity. Self-referential FKs (hierarchical entities) are noted but don't create edges in the ordering graph — they're handled by making the self-FK nullable and deferring the constraint.

Topological sort of this graph produces the apply order. Entities with no FK dependencies are created first. Entities referencing only already-created entities are created next. And so on.

If the graph has a cycle (which shouldn't happen in a well-formed schema but must be detected), the loader rejects the schema with an error identifying the cycle.

Versioning sibling tables depend on both their parent entity and the change_set entity. The loader places sibling table creation after both dependencies are satisfied.

---

### 8. Idempotent Application

The core feature. The loader runs against an existing database with data and makes it match the YAML without destroying anything.

**Scope argument:**

```
opsdb-schema apply --repo /path/to/repo --dsn postgres://... 
opsdb-schema apply --repo /path/to/repo --dsn postgres://... --scope "cloud_resource"
opsdb-schema apply --repo /path/to/repo --dsn postgres://... --scope "cloud_resource/cloud_data_json"
```

No scope means full database. Entity name means that entity and its sibling. Entity/field means that specific field only.

**State comparison:**

The loader reads current database state from two sources: the `_schema_entity_type`, `_schema_field`, and `_schema_relationship` tables if they exist (preferred), or by querying `information_schema` if the _schema_* tables don't exist yet (bootstrap case).

It compares current state against desired state from YAML and produces a diff.

**Allowed changes (applied automatically):**

| Change | DDL generated |
|--------|-------------|
| New entity | CREATE TABLE + indexes + constraints |
| New field, nullable: true | ALTER TABLE ADD COLUMN |
| New field, nullable: false, with default | ALTER TABLE ADD COLUMN with DEFAULT, then backfill existing rows |
| New enum value | DROP and re-CREATE CHECK constraint with expanded values |
| Widened min_value (decreased) | DROP and re-CREATE CHECK constraint |
| Widened max_value (increased) | DROP and re-CREATE CHECK constraint |
| Widened max_length | ALTER TABLE ALTER COLUMN TYPE VARCHAR(new_length) |
| Widened min_length (decreased) | DROP and re-CREATE CHECK constraint |
| New index | CREATE INDEX |
| New governance field enabled | ALTER TABLE ADD COLUMN |
| New versioning sibling (entity changed to versioned: true) | CREATE TABLE for sibling |

**Forbidden changes (rejected with error):**

| Change | Error message |
|--------|-------------|
| Field removed | "Field deletion forbidden. Mark deprecated via _schema_version_deprecated_id. Field: {table}.{field}" |
| Entity removed | "Entity deletion forbidden. Mark deprecated. Entity: {table}" |
| Field renamed | "Field rename forbidden. Add new field, deprecate old. Detected: {table}.{old} missing, {table}.{new} added" |
| Type changed | "Type change forbidden. Use duplication pattern: add new field with new type, double-write, migrate readers, deprecate old. Field: {table}.{field} was {old_type} now {new_type}" |
| min_value increased | "Narrowing numeric range forbidden. Existing rows may hold values below new minimum. Field: {table}.{field}" |
| max_value decreased | "Narrowing numeric range forbidden. Existing rows may hold values above new maximum. Field: {table}.{field}" |
| max_length decreased | "Narrowing length forbidden. Existing values may exceed new maximum. Field: {table}.{field}" |
| min_length increased | "Narrowing length forbidden. Existing values may fall below new minimum. Field: {table}.{field}" |
| Enum value removed | "Enum value removal forbidden. Existing rows may hold removed value. Use duplication pattern. Field: {table}.{field}, removed: {value}" |
| Unique constraint added to existing field | "Adding uniqueness to existing field forbidden. Existing rows may violate. Use duplication pattern. Field: {table}.{field}" |
| versioned changed from true to false | "Cannot remove versioning. Sibling table contains historical data." |
| soft_delete changed from true to false | "Cannot remove soft delete. Existing rows may use is_active=false." |

Each forbidden change produces a clear error citing the specific evolution rule violated and the alternative approach from the spec. The loader exits without modifying the database when any forbidden change is detected.

**Rename detection:** If a field disappears and a new field appears in the same entity with the same type and constraints, the loader flags this as a potential rename and rejects it, suggesting the duplication pattern instead. This is a heuristic — it may false-positive on coincidental add+deprecate — but false positives are safe (they just require the user to confirm intent by adding a `_deprecated: true` annotation to the old field's YAML).

---

### 9. _schema_* Table Population

After applying DDL changes, the loader updates the schema self-description tables.

**_schema_version:** A new row is inserted with an incremented version_serial, is_current set to true, and the previous current row's is_current set to false. The version_label is generated from the current date and a sequence number (e.g., "2026.05.05.01").

**_schema_entity_type:** One row per entity. On first load, all entities are inserted. On subsequent loads, new entities are inserted with _schema_version_introduced_id pointing to the new version. Removed entities (which the loader forbids from DDL changes) would get _schema_version_deprecated_id set if the YAML marks them deprecated.

**_schema_field:** One row per field per entity, including injected reserved fields. Same insert/deprecation logic as entity types.

**_schema_relationship:** One row per foreign key relationship. Source entity, source field, target entity, cardinality (one_to_many for standard FK, many_to_many for bridge tables detected by having exactly two non-self FKs), on_delete_action.

These tables make the schema queryable through the API like any other data. "Show me all fields added in the last schema version" is a standard query.

---

### 10. CLI Interface

```
opsdb-schema [command] [flags]

Commands:
  init        Create a new schema repository with meta-schema and conventions
  validate    Validate YAML files against meta-schema without touching database
  plan        Show what DDL would be executed without executing
  apply       Apply schema to database
  diff        Show differences between YAML and current database state
  export      Export current database schema as YAML files (reverse engineering)

Flags:
  --repo      Path to schema repository (default: current directory)
  --dsn       Postgres connection string
  --scope     Limit to specific entity or entity/field
  --verbose   Show generated DDL
  --dry-run   Alias for plan
  --version   Show version
```

**`opsdb-schema init`** creates the directory structure, meta-schema, conventions file, and an empty directory.yaml. Starting point for a new OpsDB.

**`opsdb-schema validate`** checks all YAML files against the meta-schema, resolves FK references, detects cycles, verifies naming conventions, checks enum values for validity, verifies json fields have json_type_discriminator. No database connection needed. Runs in CI as a schema PR check.

**`opsdb-schema plan`** does everything `apply` does except execute the DDL. Prints the DDL that would be executed, or prints the evolution violations that would block it. This is what you run before `apply` to see what will happen.

**`opsdb-schema diff`** compares YAML against the current database state and shows the differences in a human-readable format — new entities, new fields, changed constraints, etc. No DDL generated, just the diff.

**`opsdb-schema apply`** executes the full pipeline: validate YAML, connect to database, read current state, compute diff, check evolution rules, generate DDL, execute DDL in a transaction, update _schema_* tables, commit. If any step fails, the transaction rolls back and the database is unchanged.

**`opsdb-schema export`** reads an existing Postgres database and generates YAML entity files from its schema. This enables adopting the tool against an existing database. The exported files may need manual cleanup to match conventions, but they provide a starting point.

---

### 11. Testing Strategy

**Unit tests on the closed vocabulary enforcer.** Feed it valid entity files, confirm acceptance. Feed it files with regex, embedded logic, inheritance, conditional constraints, imports, templates — confirm each rejected with the correct error.

**Unit tests on type mapping.** Each of the nine types generates the correct Postgres DDL.

**Unit tests on reserved field injection.** An entity with `versioned: true, soft_delete: true, hierarchical: true` and governance fields enabled gets all the correct injected fields and a correct sibling table.

**Unit tests on dependency resolution.** A set of entities with various FK relationships produces the correct topological order. A cycle is detected and rejected.

**Integration tests against a real Postgres instance (via testcontainers or a test database):**

**Fresh apply.** Load the full 138-entity schema into an empty database. Verify all tables exist with correct columns, types, constraints, indexes, and FK relationships. Verify _schema_* tables populated correctly. Verify audit_log_entry has REVOKE applied.

**Idempotent re-apply.** Run apply again with no YAML changes. Verify zero DDL executed. Verify _schema_* tables unchanged.

**Additive evolution.** Add a new nullable field to an existing entity. Run apply. Verify ALTER TABLE ADD COLUMN executed. Verify _schema_field row added with correct _schema_version_introduced_id.

**Additive entity.** Add a new entity file. Run apply. Verify CREATE TABLE executed in correct dependency order.

**Enum widening.** Add a new value to an enum field. Run apply. Verify CHECK constraint updated.

**Range widening.** Decrease min_value or increase max_value on an int field. Run apply. Verify CHECK constraint updated.

**Length widening.** Increase max_length on a varchar field. Run apply. Verify column type altered.

**Forbidden: field deletion.** Remove a field from YAML. Run apply. Verify rejected with correct error message. Verify database unchanged.

**Forbidden: type change.** Change a field's type. Run apply. Verify rejected with duplication pattern guidance. Verify database unchanged.

**Forbidden: range narrowing.** Decrease max_value. Run apply. Verify rejected. Database unchanged.

**Forbidden: enum removal.** Remove an enum value. Run apply. Verify rejected. Database unchanged.

**Forbidden: rename detection.** Remove a field and add a new field of the same type. Run apply. Verify flagged as potential rename. Database unchanged.

**Scoped apply.** Add fields to two different entities. Run apply with scope limited to one entity. Verify only that entity's DDL executed.

**Data preservation.** Insert rows into tables. Run apply with additive changes. Verify existing data intact.

**Concurrent safety.** The apply command takes an advisory lock at the start and releases at the end. Two concurrent applies — second waits for first to complete, then runs as a no-op (idempotent).

---

### 12. Build and Distribution

Single binary, statically compiled Go. No CGO dependencies. The only runtime dependency is a reachable Postgres instance.

```
go build -o opsdb-schema ./cmd/opsdb-schema
```

Releases as GitHub releases with binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64. Container image for running in CI or Kubernetes.

The schema repository ships as a separate git repo (`opsdb-schema-repo`) containing the full 138-entity schema in YAML. The binary and the schema repo together are the complete Phase 1 deliverable.

---

### 13. Validation Criteria for Phase 1 Complete

Phase 1 is done when:

1. The full 138-entity schema from the spec exists as YAML files in the repo, valid against the meta-schema.

2. `opsdb-schema validate` passes on the complete repo with zero errors.

3. `opsdb-schema apply` against a fresh Postgres instance creates all 138 tables, all versioning sibling tables, all indexes, all constraints, all _schema_* metadata rows, and the audit_log_entry REVOKE grants, in a single transaction.

4. A second `opsdb-schema apply` with no changes produces zero DDL.

5. Every allowed evolution type (new field, new entity, enum widening, range widening, length widening, new index) succeeds and updates _schema_* tables correctly.

6. Every forbidden evolution type (field deletion, rename, type change, range narrowing, length narrowing, enum removal, uniqueness tightening) is rejected with a clear error message citing the spec's rule and alternative approach, with zero database modifications.

7. All tests pass in CI against a real Postgres instance.

8. The binary runs on Linux and macOS with no dependencies beyond Postgres connectivity.
