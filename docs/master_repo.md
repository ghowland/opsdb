## OpsDB Master Repository Specification

---

### Repository Name

`opsdb`

---

### Design Principles

One repo holds everything needed to evaluate, deploy, and operate OpsDB from zero to N substrates. Tools, schema, DOS configurations, and documentation colocate so that a single clone gives a complete working system. Organizations adopting OpsDB fork this repo and diverge only in the `dos/` directory — tools and schema remain upstream-trackable.

The N-substrate pattern is demonstrated from the start with two DOS configurations: production and staging. These are structurally identical (same schema, same tools, same API code) with diverged data, users, audit logs, and runners — exactly as the spec defines for the N pipeline. Organizations that need only one OpsDB ignore staging and use production. The architecture is the same either way.

All Go code lives under a single Go module rooted at the repository top level. Tools share internal packages. No vendored dependencies between tools — shared code lives in `internal/` and is imported directly.

---

### Repository Layout

```
opsdb/
│
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── LICENSE
│
├── tools/
│   │
│   ├── opsdb-schema/
│   │   ├── cmd/
│   │   │   └── main.go                     # CLI entrypoint
│   │   ├── loader/
│   │   │   ├── loader.go                   # orchestrates full pipeline
│   │   │   ├── parser.go                   # YAML file parsing
│   │   │   ├── validator.go                # meta-schema enforcement
│   │   │   ├── resolver.go                 # FK resolution + dependency graph
│   │   │   ├── injector.go                 # reserved field + sibling + governance injection
│   │   │   ├── differ.go                   # current-vs-desired state comparison
│   │   │   ├── evolution.go                # allowed/forbidden change classification
│   │   │   ├── generator.go                # DDL generation from internal model
│   │   │   ├── applier.go                  # transactional DDL execution
│   │   │   └── meta.go                     # _schema_* table population
│   │   └── loader_test.go                  # unit tests for loader package
│   │
│   ├── opsdb-api/
│   │   ├── cmd/
│   │   │   └── main.go
│   │   ├── gate/
│   │   │   ├── gate.go                     # 10-step pipeline orchestrator
│   │   │   ├── step_auth.go                # step 1: authentication
│   │   │   ├── step_authz.go               # step 2: five-layer authorization
│   │   │   ├── step_schema_validate.go     # step 3: schema validation
│   │   │   ├── step_bound_validate.go      # step 4: bound validation
│   │   │   ├── step_policy.go              # step 5: policy evaluation
│   │   │   ├── step_versioning.go          # step 6: versioning preparation
│   │   │   ├── step_changmgmt.go           # step 7: change management routing
│   │   │   ├── step_audit.go               # step 8: audit logging
│   │   │   ├── step_execute.go             # step 9: execution
│   │   │   └── step_response.go            # step 10: response
│   │   ├── auth/
│   │   │   ├── provider.go                 # auth provider interface
│   │   │   ├── yaml_provider.go            # YAML file auth for dev/test
│   │   │   ├── oidc_provider.go            # OIDC for production humans
│   │   │   └── serviceaccount_provider.go  # token auth for runners
│   │   ├── operations/
│   │   │   ├── read.go                     # get_entity, get_history, get_at_time, search, get_dependencies
│   │   │   ├── write_observation.go        # write_observation
│   │   │   ├── write_changeset.go          # submit, emergency, bulk
│   │   │   ├── changeset_actions.go        # approve, reject, cancel, apply, mark_applied
│   │   │   ├── resolve.go                  # resolve_authority_pointer
│   │   │   └── watch.go                    # streaming subscription
│   │   ├── schema/
│   │   │   └── runtime_schema.go           # loads _schema_* at startup, refreshes on change
│   │   ├── reportkeys/
│   │   │   └── enforcer.go                 # runner report key validation
│   │   ├── concurrency/
│   │   │   └── optimistic.go               # version stamp check on submit
│   │   └── config/
│   │       └── config.go                   # API configuration loading
│   │
│   ├── opsdb-runner-lib/
│   │   ├── lifecycle.go                    # init, cycle, shutdown, bound enforcement
│   │   ├── api_client.go                   # wraps opsdb-api HTTP calls
│   │   ├── logging.go                      # structured logging with runner context
│   │   ├── retry.go                        # retry with backoff, jitter, idempotency keys
│   │   ├── dryrun.go                       # dry-run mode support
│   │   └── config.go                       # runner configuration from spec + env
│   │
│   ├── runners/
│   │   ├── change-set-executor/
│   │   │   ├── cmd/
│   │   │   │   └── main.go
│   │   │   └── executor.go                 # drains approved change_sets
│   │   ├── schema-executor/
│   │   │   ├── cmd/
│   │   │   │   └── main.go
│   │   │   └── executor.go                 # applies approved _schema_change_sets
│   │   ├── reaper/
│   │   │   ├── cmd/
│   │   │   │   └── main.go
│   │   │   └── reaper.go                   # enforces retention policies
│   │   ├── emergency-review-monitor/
│   │   │   ├── cmd/
│   │   │   │   └── main.go
│   │   │   └── monitor.go                  # escalates overdue emergency reviews
│   │   └── notification-runner/
│   │       ├── cmd/
│   │       │   └── main.go
│   │       ├── runner.go                   # reads state transitions, dispatches
│   │       └── backends/
│   │           ├── email.go
│   │           └── webhook.go
│   │
│   └── importers/
│       ├── opsdb-import-aws/
│       │   ├── cmd/
│       │   │   └── main.go
│       │   ├── ec2.go
│       │   ├── rds.go
│       │   ├── s3.go
│       │   ├── iam.go
│       │   ├── vpc.go
│       │   ├── route53.go
│       │   └── mapping.go                  # AWS → OpsDB schema mapping
│       ├── opsdb-import-gcp/
│       │   ├── cmd/
│       │   │   └── main.go
│       │   ├── gce.go
│       │   ├── cloudsql.go
│       │   ├── gcs.go
│       │   ├── gke.go
│       │   ├── iam.go
│       │   └── mapping.go
│       ├── opsdb-import-k8s/
│       │   ├── cmd/
│       │   │   └── main.go
│       │   ├── cluster.go
│       │   ├── node.go
│       │   ├── namespace.go
│       │   ├── workload.go
│       │   ├── pod.go
│       │   ├── helm.go
│       │   ├── configmap.go
│       │   ├── secret.go
│       │   ├── service.go
│       │   └── watcher.go                  # K8s watch with level-triggered backstop
│       ├── opsdb-import-identity/
│       │   ├── cmd/
│       │   │   └── main.go
│       │   ├── okta.go
│       │   ├── azuread.go
│       │   └── ldap.go
│       ├── opsdb-import-monitoring/
│       │   ├── cmd/
│       │   │   └── main.go
│       │   ├── prometheus.go
│       │   └── datadog.go
│       ├── opsdb-import-oncall/
│       │   ├── cmd/
│       │   │   └── main.go
│       │   ├── pagerduty.go
│       │   └── opsgenie.go
│       └── opsdb-import-secrets/
│           ├── cmd/
│           │   └── main.go
│           ├── vault.go
│           └── aws_sm.go
│
├── internal/
│   ├── pg/
│   │   ├── conn.go                         # Postgres connection management
│   │   ├── tx.go                           # transaction helpers
│   │   └── advisory_lock.go               # advisory lock for concurrent safety
│   ├── model/
│   │   ├── entity.go                       # internal entity representation
│   │   ├── field.go                        # internal field representation
│   │   ├── relationship.go                 # internal relationship representation
│   │   └── schema.go                       # full schema as in-memory model
│   ├── conventions/
│   │   ├── naming.go                       # naming convention validation
│   │   └── reserved.go                     # reserved field definitions
│   ├── vocabulary/
│   │   ├── types.go                        # the nine types
│   │   ├── modifiers.go                    # the three modifiers
│   │   ├── constraints.go                  # the six+ constraints
│   │   └── forbidden.go                    # forbidden pattern detection
│   └── testutil/
│       ├── pg.go                           # test Postgres via testcontainers
│       └── fixtures.go                     # test schema fragments
│
├── schema/
│   ├── meta/
│   │   └── _schema_meta.yaml
│   ├── conventions/
│   │   └── reserved.yaml
│   ├── directory.yaml
│   ├── json_schemas/                       # registered JSON schemas for typed payloads
│   │   ├── cloud_resource/
│   │   │   ├── ec2_instance.yaml
│   │   │   ├── gce_instance.yaml
│   │   │   ├── rds_database.yaml
│   │   │   ├── s3_bucket.yaml
│   │   │   └── ...                         # one per discriminator value
│   │   ├── authority/
│   │   │   ├── prometheus_server.yaml
│   │   │   ├── secret_vault.yaml
│   │   │   └── ...
│   │   ├── policy/
│   │   │   ├── security_zone.yaml
│   │   │   ├── approval_rule.yaml
│   │   │   └── ...
│   │   ├── runner_spec/
│   │   │   ├── puller.yaml
│   │   │   ├── drift_detect.yaml
│   │   │   └── ...
│   │   ├── schedule/
│   │   │   ├── cron_expression.yaml
│   │   │   ├── rate_based.yaml
│   │   │   └── ...
│   │   ├── monitor/
│   │   │   ├── prometheus_query.yaml
│   │   │   ├── http_probe.yaml
│   │   │   └── ...
│   │   ├── evidence_record/
│   │   │   ├── backup_verification.yaml
│   │   │   ├── certificate_validity.yaml
│   │   │   └── ...
│   │   ├── manual_operation/
│   │   │   ├── tape_rotation.yaml
│   │   │   ├── vendor_review.yaml
│   │   │   └── ...
│   │   ├── storage_resource/
│   │   │   ├── ebs.yaml
│   │   │   ├── nfs_export.yaml
│   │   │   └── ...
│   │   └── configuration_variable/
│   │       ├── string.yaml
│   │       ├── int.yaml
│   │       ├── json.yaml
│   │       ├── secret_reference.yaml
│   │       └── ...
│   └── domains/
│       ├── 01_identity/
│       │   ├── site.yaml
│       │   ├── location.yaml
│       │   ├── ops_user.yaml
│       │   ├── ops_group.yaml
│       │   ├── ops_group_member.yaml
│       │   ├── ops_user_role.yaml
│       │   └── ops_user_role_member.yaml
│       ├── 02_substrate/
│       │   ├── hardware_component.yaml
│       │   ├── hardware_port.yaml
│       │   ├── hardware_set.yaml
│       │   ├── hardware_set_component.yaml
│       │   ├── hardware_set_instance.yaml
│       │   ├── hardware_set_instance_port_connection.yaml
│       │   ├── megavisor.yaml
│       │   ├── megavisor_instance.yaml
│       │   ├── cloud_provider.yaml
│       │   ├── cloud_account.yaml
│       │   ├── cloud_resource.yaml
│       │   ├── storage_resource.yaml
│       │   ├── platform.yaml
│       │   └── machine.yaml
│       ├── 03_service/
│       │   ├── package.yaml
│       │   ├── package_interface.yaml
│       │   ├── package_connection.yaml
│       │   ├── service.yaml
│       │   ├── service_package.yaml
│       │   ├── service_interface_mount.yaml
│       │   ├── service_connection.yaml
│       │   ├── host_group.yaml
│       │   ├── host_group_machine.yaml
│       │   ├── host_group_package.yaml
│       │   ├── site_location.yaml
│       │   ├── service_level.yaml
│       │   └── service_level_metric.yaml
│       ├── 04_kubernetes/
│       │   ├── k8s_cluster.yaml
│       │   ├── k8s_cluster_node.yaml
│       │   ├── k8s_namespace.yaml
│       │   ├── k8s_workload.yaml
│       │   ├── k8s_pod.yaml
│       │   ├── k8s_helm_release.yaml
│       │   ├── k8s_config_map.yaml
│       │   ├── k8s_secret_reference.yaml
│       │   └── k8s_service.yaml
│       ├── 05_authority/
│       │   ├── authority.yaml
│       │   ├── authority_pointer.yaml
│       │   ├── service_authority_pointer.yaml
│       │   ├── machine_authority_pointer.yaml
│       │   ├── k8s_cluster_authority_pointer.yaml
│       │   └── cloud_resource_authority_pointer.yaml
│       ├── 06_schedule/
│       │   ├── schedule.yaml
│       │   ├── runner_schedule.yaml
│       │   ├── credential_rotation_schedule.yaml
│       │   ├── certificate_expiration_schedule.yaml
│       │   ├── compliance_audit_schedule.yaml
│       │   ├── manual_operation_schedule.yaml
│       │   └── manual_operation.yaml
│       ├── 07_policy/
│       │   ├── policy.yaml
│       │   ├── service_policy.yaml
│       │   ├── machine_policy.yaml
│       │   ├── k8s_namespace_policy.yaml
│       │   ├── cloud_account_policy.yaml
│       │   ├── security_zone.yaml
│       │   ├── security_zone_membership_service.yaml
│       │   ├── security_zone_membership_machine.yaml
│       │   ├── security_zone_membership_k8s_namespace.yaml
│       │   ├── data_classification.yaml
│       │   ├── retention_policy.yaml
│       │   ├── approval_rule.yaml
│       │   ├── escalation_path.yaml
│       │   ├── escalation_step.yaml
│       │   ├── service_escalation_path.yaml
│       │   ├── change_management_rule.yaml
│       │   ├── compliance_regime.yaml
│       │   ├── compliance_scope_service.yaml
│       │   └── compliance_scope_data_classification.yaml
│       ├── 08_docs/
│       │   ├── service_ownership.yaml
│       │   ├── machine_ownership.yaml
│       │   ├── k8s_cluster_ownership.yaml
│       │   ├── cloud_resource_ownership.yaml
│       │   ├── service_stakeholder.yaml
│       │   ├── runbook_reference.yaml
│       │   ├── service_runbook_reference.yaml
│       │   ├── dashboard_reference.yaml
│       │   └── service_dashboard_reference.yaml
│       ├── 09_runner/
│       │   ├── runner_spec.yaml
│       │   ├── runner_capability.yaml
│       │   ├── runner_machine.yaml
│       │   ├── runner_instance.yaml
│       │   ├── runner_service_target.yaml
│       │   ├── runner_host_group_target.yaml
│       │   ├── runner_k8s_namespace_target.yaml
│       │   ├── runner_cloud_account_target.yaml
│       │   ├── runner_job.yaml
│       │   ├── runner_job_target_machine.yaml
│       │   ├── runner_job_target_service.yaml
│       │   ├── runner_job_target_k8s_workload.yaml
│       │   ├── runner_job_target_cloud_resource.yaml
│       │   └── runner_job_output_var.yaml
│       ├── 10_monitoring/
│       │   ├── monitor.yaml
│       │   ├── monitor_machine_target.yaml
│       │   ├── monitor_service_target.yaml
│       │   ├── monitor_k8s_workload_target.yaml
│       │   ├── monitor_cloud_resource_target.yaml
│       │   ├── prometheus_config.yaml
│       │   ├── prometheus_scrape_target.yaml
│       │   ├── monitor_level.yaml
│       │   ├── alert.yaml
│       │   ├── alert_dependency.yaml
│       │   ├── alert_fire.yaml
│       │   ├── on_call_schedule.yaml
│       │   └── on_call_assignment.yaml
│       ├── 11_observation/
│       │   ├── observation_cache_metric.yaml
│       │   ├── observation_cache_state.yaml
│       │   └── observation_cache_config.yaml
│       ├── 12_config/
│       │   └── configuration_variable.yaml
│       ├── 13_change_mgmt/
│       │   ├── change_set.yaml
│       │   ├── change_set_field_change.yaml
│       │   ├── change_set_approval_required.yaml
│       │   ├── change_set_approval.yaml
│       │   ├── change_set_rejection.yaml
│       │   ├── change_set_validation.yaml
│       │   ├── change_set_emergency_review.yaml
│       │   └── change_set_bulk_membership.yaml
│       ├── 14_audit/
│       │   ├── audit_log_entry.yaml
│       │   ├── evidence_record.yaml
│       │   ├── evidence_record_service_target.yaml
│       │   ├── evidence_record_machine_target.yaml
│       │   ├── evidence_record_credential_target.yaml
│       │   ├── evidence_record_certificate_target.yaml
│       │   ├── evidence_record_compliance_regime_target.yaml
│       │   ├── evidence_record_manual_operation_target.yaml
│       │   ├── compliance_finding.yaml
│       │   └── compliance_finding_target_service.yaml
│       └── 15_schema_meta/
│           ├── _schema_version.yaml
│           ├── _schema_change_set.yaml
│           ├── _schema_entity_type.yaml
│           ├── _schema_field.yaml
│           └── _schema_relationship.yaml
│
├── dos/
│   │
│   ├── README.md                            # explains N-substrate pattern + how to add/remove DOS
│   │
│   ├── opsdb-ops-prod/
│   │   ├── config.yaml                      # substrate identity + API config + DSN
│   │   ├── auth/
│   │   │   └── users.yaml                   # YAML auth backend for bootstrapping
│   │   ├── seed/
│   │   │   ├── site.yaml                    # initial site row(s)
│   │   │   ├── admin_user.yaml              # bootstrap admin ops_user + role
│   │   │   ├── base_policies.yaml           # default access control + approval rules
│   │   │   ├── runner_service_accounts.yaml # service accounts for core runners
│   │   │   └── core_runner_specs.yaml       # runner_spec rows for shipped runners
│   │   ├── runners/
│   │   │   ├── enabled.yaml                 # which runners active in this DOS
│   │   │   └── overrides/                   # per-runner config overrides for this DOS
│   │   │       ├── reaper.yaml
│   │   │       └── notification.yaml
│   │   └── importers/
│   │       ├── enabled.yaml                 # which importers active in this DOS
│   │       └── credentials/
│   │           ├── aws.yaml                 # credential source config (paths to env vars/vault, never values)
│   │           ├── k8s.yaml
│   │           └── pagerduty.yaml
│   │
│   └── opsdb-ops-staging/
│       ├── config.yaml
│       ├── auth/
│       │   └── users.yaml
│       ├── seed/
│       │   ├── site.yaml
│       │   ├── admin_user.yaml
│       │   ├── base_policies.yaml
│       │   ├── runner_service_accounts.yaml
│       │   └── core_runner_specs.yaml
│       ├── runners/
│       │   ├── enabled.yaml
│       │   └── overrides/
│       │       ├── reaper.yaml
│       │       └── notification.yaml
│       └── importers/
│           ├── enabled.yaml
│           └── credentials/
│               ├── aws.yaml
│               ├── k8s.yaml
│               └── pagerduty.yaml
│
├── docs/
│   ├── architecture/
│   │   ├── overview.md                      # what OpsDB is, link to spec papers
│   │   ├── schema-engine.md                 # opsdb-schema technical doc (phase 1 doc)
│   │   ├── api-gate.md                      # opsdb-api technical doc
│   │   ├── runner-pattern.md                # runner lifecycle + disciplines
│   │   ├── library-contracts.md             # opsdb-runner-lib contracts
│   │   ├── importer-pattern.md              # how importers work
│   │   └── n-substrate.md                   # N-DOS pattern explanation
│   ├── guides/
│   │   ├── quickstart.md                    # zero to queryable in an afternoon
│   │   ├── adding-a-dos.md                  # how to add a third DOS
│   │   ├── writing-a-runner.md              # step by step runner creation
│   │   ├── writing-an-importer.md           # step by step importer creation
│   │   ├── schema-evolution.md              # how to add fields, entities, enum values
│   │   ├── approval-rules.md               # how to write org-specific approval rules
│   │   └── dev-to-operational.md            # cutover guide
│   ├── reference/
│   │   ├── cli.md                           # all tool CLI reference
│   │   ├── api-operations.md                # all 16 API operations
│   │   ├── entity-catalog.md                # generated from schema YAML
│   │   ├── discriminator-catalog.md         # all typed payloads with JSON schemas
│   │   ├── evolution-rules.md               # allowed + forbidden changes
│   │   └── naming-conventions.md            # DSNC rules
│   ├── spec/
│   │   ├── OPSDB-1-overview.md              # upstream spec papers for reference
│   │   ├── OPSDB-2-architecture.md
│   │   ├── OPSDB-3-implementation.md
│   │   ├── OPSDB-4-schema.md
│   │   ├── OPSDB-5-runners.md
│   │   ├── OPSDB-6-api.md
│   │   ├── OPSDB-7-schema-construction.md
│   │   ├── OPSDB-8-library-suite.md
│   │   └── OPSDB-9-vocabulary.md
│   └── decisions/
│       ├── 001-go-only.md                   # why Go, no Python
│       ├── 002-monorepo.md                  # why single repo
│       ├── 003-postgres-first.md            # why Postgres as initial engine
│       ├── 004-n-from-start.md              # why N-substrate from day one
│       └── 005-yaml-auth-bootstrap.md       # why YAML auth for zero-dependency start
│
├── scripts/
│   ├── seed.sh                              # applies schema + seeds a DOS from its seed/ dir
│   ├── build-all.sh                         # builds all tool binaries
│   ├── test-integration.sh                  # runs integration tests against testcontainer PG
│   └── generate-entity-catalog.sh           # generates docs/reference/entity-catalog.md from schema YAML
│
└── .github/
    └── workflows/
        ├── validate-schema.yaml             # runs opsdb-schema validate on PR
        ├── test.yaml                        # unit + integration tests
        └── release.yaml                     # build + publish binaries
```

---

### Go Module Structure

The repository is a single Go module.

```
module github.com/ghowland/opsdb
```

Import paths for all packages follow from this root:

```go
import "github.com/ghowland/opsdb/tools/opsdb-schema/loader"
import "github.com/ghowland/opsdb/tools/opsdb-api/gate"
import "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
import "github.com/ghowland/opsdb/internal/pg"
import "github.com/ghowland/opsdb/internal/model"
import "github.com/ghowland/opsdb/internal/vocabulary"
import "github.com/ghowland/opsdb/internal/conventions"
```

Binaries are built from `cmd/main.go` files within each tool:

```
go build -o bin/opsdb-schema       ./tools/opsdb-schema/cmd
go build -o bin/opsdb-api          ./tools/opsdb-api/cmd
go build -o bin/opsdb-changeset-executor  ./tools/runners/change-set-executor/cmd
go build -o bin/opsdb-reaper       ./tools/runners/reaper/cmd
go build -o bin/opsdb-import-aws   ./tools/importers/opsdb-import-aws/cmd
go build -o bin/opsdb-import-k8s   ./tools/importers/opsdb_import_k8s/cmd
# ... and so on for each binary
```

The Makefile provides targets:

```makefile
.PHONY: all schema api runners importers test test-integration validate clean

all: schema api runners importers

schema:
	go build -o bin/opsdb-schema ./tools/opsdb-schema/cmd

api:
	go build -o bin/opsdb-api ./tools/opsdb-api/cmd

runners:
	go build -o bin/opsdb-changeset-executor ./tools/runners/change-set-executor/cmd
	go build -o bin/opsdb-schema-executor    ./tools/runners/schema-executor/cmd
	go build -o bin/opsdb-reaper             ./tools/runners/reaper/cmd
	go build -o bin/opsdb-emergency-monitor  ./tools/runners/emergency-review-monitor/cmd
	go build -o bin/opsdb-notification       ./tools/runners/notification-runner/cmd

importers:
	go build -o bin/opsdb-import-aws        ./tools/importers/opsdb-import-aws/cmd
	go build -o bin/opsdb-import-gcp        ./tools/importers/opsdb-import-gcp/cmd
	go build -o bin/opsdb-import-k8s        ./tools/importers/opsdb_import_k8s/cmd
	go build -o bin/opsdb-import-identity   ./tools/importers/opsdb_import_identity/cmd
	go build -o bin/opsdb-import-monitoring ./tools/importers/opsdb_import_monitoring/cmd
	go build -o bin/opsdb-import-oncall     ./tools/importers/opsdb_import_oncall/cmd
	go build -o bin/opsdb-import-secrets    ./tools/importers/opsdb_import_secrets/cmd

test:
	go test ./...

test-integration:
	OPSDB_TEST_PG=1 go test ./... -tags integration -count=1

validate:
	go run ./tools/opsdb-schema/cmd validate --repo ./schema

clean:
	rm -rf bin/
```

---

### DOS Configuration Structure

Each DOS directory represents one OpsDB substrate instance. The structure is uniform across all DOS configurations so that tooling works identically against any of them.

**config.yaml** — the substrate identity:

```yaml
# dos/opsdb-ops-prod/config.yaml
substrate:
  name: ops-prod
  description: "Production operational substrate"
  site_name: production

database:
  dsn_env_var: OPSDB_PROD_DSN        # DSN read from environment, never in file

api:
  listen_address: ":8443"
  tls_cert_path: /etc/opsdb/tls/cert.pem
  tls_key_path: /etc/opsdb/tls/key.pem
  auth_backend: yaml                  # yaml | oidc | service_account
  auth_config_path: ./auth/users.yaml

schema:
  repo_path: ../../schema             # relative path to shared schema
```

Every DOS points at the same `schema/` directory. Schema is shared. Configuration is diverged. This is the N pipeline in practice.

**seed/ directory** — bootstrap data loaded via opsdb-api after schema apply. Each seed file is a set of API write operations expressed as YAML, processed by the seed script in order:

```yaml
# dos/opsdb-ops-prod/seed/site.yaml
operations:
  - operation: create_entity
    entity_type: site
    fields:
      name: production
      description: "Production operational environment"
      domain: ops.example.com
```

```yaml
# dos/opsdb-ops-prod/seed/admin_user.yaml
operations:
  - operation: create_entity
    entity_type: ops_user
    fields:
      site_id: "@ref:site:production"     # resolved by seed script
      username: admin
      fullname: "OpsDB Administrator"
      email: admin@example.com
  - operation: create_entity
    entity_type: ops_user_role
    fields:
      site_id: "@ref:site:production"
      name: opsdb_admin
      description: "Full OpsDB administrative access"
  - operation: create_entity
    entity_type: ops_user_role_member
    fields:
      ops_user_role_id: "@ref:ops_user_role:opsdb_admin"
      ops_user_id: "@ref:ops_user:admin"
```

The `@ref:entity_type:name` syntax lets seed files reference rows created earlier in the seed sequence without hardcoding IDs. The seed script resolves these by querying the API after each creation.

**runners/enabled.yaml** — declares which runners this DOS activates:

```yaml
runners:
  - name: change-set-executor
    binary: opsdb-changeset-executor
    schedule: "continuous"
    override_file: overrides/changeset-executor.yaml
  - name: reaper
    binary: opsdb-reaper
    schedule: "daily"
    override_file: overrides/reaper.yaml
  - name: emergency-review-monitor
    binary: opsdb-emergency-monitor
    schedule: "hourly"
  - name: notification-runner
    binary: opsdb-notification
    schedule: "continuous"
    override_file: overrides/notification.yaml
```

**importers/enabled.yaml** — declares which importers this DOS activates:

```yaml
importers:
  - name: aws
    binary: opsdb-import-aws
    schedule: "every_5m"
    credential_config: credentials/aws.yaml
    targets:
      - cloud_account: "123456789012"
        regions: [us-east-1, us-west-2]
  - name: k8s
    binary: opsdb-import-k8s
    schedule: "watch"                     # continuous via K8s watch API
    credential_config: credentials/k8s.yaml
    targets:
      - cluster: prod-east
        kubeconfig_env_var: KUBECONFIG_PROD_EAST
  - name: oncall
    binary: opsdb-import-oncall
    schedule: "every_15m"
    credential_config: credentials/pagerduty.yaml
```

**Credential files never contain secrets.** They contain pointers to environment variables, Vault paths, or file paths where credentials are available at runtime:

```yaml
# dos/opsdb-ops-prod/importers/credentials/aws.yaml
credential_source: environment
access_key_env_var: OPSDB_AWS_ACCESS_KEY_ID
secret_key_env_var: OPSDB_AWS_SECRET_ACCESS_KEY
# alternative:
# credential_source: vault
# vault_path: secret/opsdb/aws-importer
# vault_addr_env_var: VAULT_ADDR
```

---

### Differences Between DOS Instances

The two shipped DOS configurations — prod and staging — demonstrate the N pipeline. What's shared and what diverges:

| Aspect | Shared | Diverged |
|--------|--------|----------|
| Schema YAML files | Same `schema/` directory | — |
| Tool binaries | Same binaries | — |
| Library code | Same `opsdb-runner-lib` | — |
| Database | — | Separate Postgres instances |
| API instance | — | Separate process per DOS |
| Users and roles | — | Different users.yaml per DOS |
| Seed data | — | Different site names, policies |
| Approval rules | — | Staging may auto-approve more broadly |
| Runners active | — | Staging may skip notification runner |
| Importers active | — | Staging imports from staging cloud accounts |
| Audit log | — | Independent per substrate |
| Credential sources | — | Different Vault paths or env vars |

The staging DOS might have more permissive auto-approval policies (all drift corrections auto-approve, no human approval required for most changes) while production has stricter policies. This is exactly the per-target gating the spec describes — same runner code, different policy data.

---

### Bootstrap Sequence

Getting from fresh clone to two running OpsDB substrates:

```bash
# 1. Build everything
make all

# 2. Set up Postgres instances (two databases, one per DOS)
createdb opsdb_prod
createdb opsdb_staging

# 3. Apply schema to both (same schema, two databases)
export OPSDB_PROD_DSN="postgres://localhost/opsdb_prod?sslmode=disable"
export OPSDB_STAGING_DSN="postgres://localhost/opsdb_staging?sslmode=disable"

bin/opsdb-schema apply --repo ./schema --dsn "$OPSDB_PROD_DSN"
bin/opsdb-schema apply --repo ./schema --dsn "$OPSDB_STAGING_DSN"

# 4. Start APIs (one per DOS)
bin/opsdb-api --config ./dos/opsdb-ops-prod/config.yaml &
bin/opsdb-api --config ./dos/opsdb-ops-staging/config.yaml &

# 5. Seed both substrates
./scripts/seed.sh ./dos/opsdb-ops-prod
./scripts/seed.sh ./dos/opsdb-ops-staging

# 6. Start core runners for each DOS
# (runner reads its DOS config to know which API to talk to)
bin/opsdb-changeset-executor --dos ./dos/opsdb-ops-prod &
bin/opsdb-changeset-executor --dos ./dos/opsdb-ops-staging &
bin/opsdb-reaper --dos ./dos/opsdb-ops-prod &
# ... etc

# 7. Start importers for each DOS
bin/opsdb-import-aws --dos ./dos/opsdb-ops-prod &
bin/opsdb-import-k8s --dos ./dos/opsdb-ops-prod &
# staging might import from staging AWS account
bin/opsdb-import-aws --dos ./dos/opsdb-ops-staging &
```

The `--dos` flag on runners and importers points to the DOS directory. The runner reads `config.yaml` to find the API address and credential source. All runners for a given DOS talk to that DOS's API instance, which talks to that DOS's database. Schema is shared. Data is diverged. The N pipeline is live.

---

### CI Pipeline

**On every PR:**

`opsdb-schema validate` runs against the schema directory. Any YAML file that violates the meta-schema, uses forbidden vocabulary, has unresolved FK references, or violates naming conventions fails the PR.

`go test ./...` runs unit tests for all packages.

`go vet` and `staticcheck` run for code quality.

**On merge to main:**

Integration tests run against a testcontainer Postgres instance. Full schema apply, idempotent re-apply, every allowed evolution type, every forbidden evolution type.

If integration tests pass, binaries are built for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and attached to a GitHub release.

**On schema changes (files in `schema/` modified):**

The PR check additionally runs `opsdb-schema plan` against a test database to show exactly what DDL would be generated. The plan output is posted as a PR comment so reviewers see the concrete database impact of the schema change.

---

### Development Workflow

**Adding a new entity to the schema:**

1. Create the YAML file in the appropriate domain directory.
2. If it depends on entities in a later domain, consider whether it belongs in the correct domain or the dependency order needs adjustment.
3. Run `make validate` to check meta-schema compliance.
4. Run `bin/opsdb-schema plan --repo ./schema --dsn "$OPSDB_PROD_DSN"` to see the DDL.
5. PR. CI validates. Schema steward reviews. Merge. Apply to substrates.

**Adding a new runner:**

1. Create directory under `tools/runners/{runner-name}/`.
2. Write `cmd/main.go` using `opsdb-runner-lib` lifecycle.
3. Write runner logic following get/act/set pattern.
4. Create runner_spec YAML in `dos/{dos-name}/seed/` for registration.
5. Add to `dos/{dos-name}/runners/enabled.yaml`.
6. Add build target to Makefile.
7. PR. CI builds and tests. Merge.

**Adding a new importer:**

1. Create directory under `tools/importers/opsdb-import-{authority}/`.
2. Write `cmd/main.go` using `opsdb-runner-lib` lifecycle.
3. Write mapping code translating authority data to schema entities.
4. Create credential config template in `dos/{dos-name}/importers/credentials/`.
5. Add to `dos/{dos-name}/importers/enabled.yaml`.
6. Add build target to Makefile.
7. PR. CI builds and tests. Merge.

**Adding a new DOS:**

1. Copy an existing DOS directory: `cp -r dos/opsdb-ops-prod dos/opsdb-ops-newenv`.
2. Edit `config.yaml` with new substrate name and DSN env var.
3. Edit seed files for environment-specific site, users, policies.
4. Edit `runners/enabled.yaml` and `importers/enabled.yaml` for what this environment needs.
5. Create the Postgres database.
6. Apply schema: `bin/opsdb-schema apply --repo ./schema --dsn "$NEW_DSN"`.
7. Seed: `./scripts/seed.sh ./dos/opsdb-ops-newenv`.
8. Start API and runners.

The new DOS uses the same schema, same tools, same libraries. Only data and configuration diverge. This is N=3 with zero code changes.
