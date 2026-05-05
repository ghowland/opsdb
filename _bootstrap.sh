#!/bin/bash
# Run from repo root: ~/Geoff/work/opsdb/
# Creates all directories and empty files for the OpsDB master repo

# ============================================================
# Top-level files
# ============================================================
touch go.mod
touch go.sum
touch Makefile

# ============================================================
# tools/opsdb-schema
# ============================================================
mkdir -p tools/opsdb-schema/cmd
mkdir -p tools/opsdb-schema/loader
touch tools/opsdb-schema/cmd/main.go
touch tools/opsdb-schema/loader/loader.go
touch tools/opsdb-schema/loader/parser.go
touch tools/opsdb-schema/loader/validator.go
touch tools/opsdb-schema/loader/resolver.go
touch tools/opsdb-schema/loader/injector.go
touch tools/opsdb-schema/loader/differ.go
touch tools/opsdb-schema/loader/evolution.go
touch tools/opsdb-schema/loader/generator.go
touch tools/opsdb-schema/loader/applier.go
touch tools/opsdb-schema/loader/meta.go
touch tools/opsdb-schema/loader/loader_test.go

# ============================================================
# tools/opsdb-api
# ============================================================
mkdir -p tools/opsdb-api/cmd
mkdir -p tools/opsdb-api/gate
mkdir -p tools/opsdb-api/auth
mkdir -p tools/opsdb-api/operations
mkdir -p tools/opsdb-api/schema
mkdir -p tools/opsdb-api/reportkeys
mkdir -p tools/opsdb-api/concurrency
mkdir -p tools/opsdb-api/config
touch tools/opsdb-api/cmd/main.go
touch tools/opsdb-api/gate/gate.go
touch tools/opsdb-api/gate/step_auth.go
touch tools/opsdb-api/gate/step_authz.go
touch tools/opsdb-api/gate/step_schema_validate.go
touch tools/opsdb-api/gate/step_bound_validate.go
touch tools/opsdb-api/gate/step_policy.go
touch tools/opsdb-api/gate/step_versioning.go
touch tools/opsdb-api/gate/step_changemgmt.go
touch tools/opsdb-api/gate/step_audit.go
touch tools/opsdb-api/gate/step_execute.go
touch tools/opsdb-api/gate/step_response.go
touch tools/opsdb-api/auth/provider.go
touch tools/opsdb-api/auth/yaml_provider.go
touch tools/opsdb-api/auth/oidc_provider.go
touch tools/opsdb-api/auth/serviceaccount_provider.go
touch tools/opsdb-api/operations/read.go
touch tools/opsdb-api/operations/write_observation.go
touch tools/opsdb-api/operations/write_changeset.go
touch tools/opsdb-api/operations/changeset_actions.go
touch tools/opsdb-api/operations/resolve.go
touch tools/opsdb-api/operations/watch.go
touch tools/opsdb-api/schema/runtime_schema.go
touch tools/opsdb-api/reportkeys/enforcer.go
touch tools/opsdb-api/concurrency/optimistic.go
touch tools/opsdb-api/config/config.go

# ============================================================
# tools/opsdb-runner-lib
# ============================================================
mkdir -p tools/opsdb-runner-lib
touch tools/opsdb-runner-lib/lifecycle.go
touch tools/opsdb-runner-lib/api_client.go
touch tools/opsdb-runner-lib/logging.go
touch tools/opsdb-runner-lib/retry.go
touch tools/opsdb-runner-lib/dryrun.go
touch tools/opsdb-runner-lib/config.go

# ============================================================
# tools/runners
# ============================================================
mkdir -p tools/runners/change-set-executor/cmd
touch tools/runners/change-set-executor/cmd/main.go
touch tools/runners/change-set-executor/executor.go

mkdir -p tools/runners/schema-executor/cmd
touch tools/runners/schema-executor/cmd/main.go
touch tools/runners/schema-executor/executor.go

mkdir -p tools/runners/reaper/cmd
touch tools/runners/reaper/cmd/main.go
touch tools/runners/reaper/reaper.go

mkdir -p tools/runners/emergency-review-monitor/cmd
touch tools/runners/emergency-review-monitor/cmd/main.go
touch tools/runners/emergency-review-monitor/monitor.go

mkdir -p tools/runners/notification-runner/cmd
mkdir -p tools/runners/notification-runner/backends
touch tools/runners/notification-runner/cmd/main.go
touch tools/runners/notification-runner/runner.go
touch tools/runners/notification-runner/backends/email.go
touch tools/runners/notification-runner/backends/webhook.go

# ============================================================
# tools/importers
# ============================================================
mkdir -p tools/importers/opsdb-import-aws/cmd
touch tools/importers/opsdb-import-aws/cmd/main.go
touch tools/importers/opsdb-import-aws/ec2.go
touch tools/importers/opsdb-import-aws/rds.go
touch tools/importers/opsdb-import-aws/s3.go
touch tools/importers/opsdb-import-aws/iam.go
touch tools/importers/opsdb-import-aws/vpc.go
touch tools/importers/opsdb-import-aws/route53.go
touch tools/importers/opsdb-import-aws/mapping.go

mkdir -p tools/importers/opsdb-import-gcp/cmd
touch tools/importers/opsdb-import-gcp/cmd/main.go
touch tools/importers/opsdb-import-gcp/gce.go
touch tools/importers/opsdb-import-gcp/cloudsql.go
touch tools/importers/opsdb-import-gcp/gcs.go
touch tools/importers/opsdb-import-gcp/gke.go
touch tools/importers/opsdb-import-gcp/iam.go
touch tools/importers/opsdb-import-gcp/mapping.go

mkdir -p tools/importers/opsdb-import-k8s/cmd
touch tools/importers/opsdb-import-k8s/cmd/main.go
touch tools/importers/opsdb-import-k8s/cluster.go
touch tools/importers/opsdb-import-k8s/node.go
touch tools/importers/opsdb-import-k8s/namespace.go
touch tools/importers/opsdb-import-k8s/workload.go
touch tools/importers/opsdb-import-k8s/pod.go
touch tools/importers/opsdb-import-k8s/helm.go
touch tools/importers/opsdb-import-k8s/configmap.go
touch tools/importers/opsdb-import-k8s/secret.go
touch tools/importers/opsdb-import-k8s/service.go
touch tools/importers/opsdb-import-k8s/watcher.go

mkdir -p tools/importers/opsdb-import-identity/cmd
touch tools/importers/opsdb-import-identity/cmd/main.go
touch tools/importers/opsdb-import-identity/okta.go
touch tools/importers/opsdb-import-identity/azuread.go
touch tools/importers/opsdb-import-identity/ldap.go

mkdir -p tools/importers/opsdb-import-monitoring/cmd
touch tools/importers/opsdb-import-monitoring/cmd/main.go
touch tools/importers/opsdb-import-monitoring/prometheus.go
touch tools/importers/opsdb-import-monitoring/datadog.go

mkdir -p tools/importers/opsdb-import-oncall/cmd
touch tools/importers/opsdb-import-oncall/cmd/main.go
touch tools/importers/opsdb-import-oncall/pagerduty.go
touch tools/importers/opsdb-import-oncall/opsgenie.go

mkdir -p tools/importers/opsdb-import-secrets/cmd
touch tools/importers/opsdb-import-secrets/cmd/main.go
touch tools/importers/opsdb-import-secrets/vault.go
touch tools/importers/opsdb-import-secrets/aws_sm.go

# ============================================================
# internal/
# ============================================================
mkdir -p internal/pg
touch internal/pg/conn.go
touch internal/pg/tx.go
touch internal/pg/advisory_lock.go

mkdir -p internal/model
touch internal/model/entity.go
touch internal/model/field.go
touch internal/model/relationship.go
touch internal/model/schema.go

mkdir -p internal/conventions
touch internal/conventions/naming.go
touch internal/conventions/reserved.go

mkdir -p internal/vocabulary
touch internal/vocabulary/types.go
touch internal/vocabulary/modifiers.go
touch internal/vocabulary/constraints.go
touch internal/vocabulary/forbidden.go

mkdir -p internal/testutil
touch internal/testutil/pg.go
touch internal/testutil/fixtures.go

# ============================================================
# schema/
# ============================================================
mkdir -p schema/meta
touch schema/meta/_schema_meta.yaml

mkdir -p schema/conventions
touch schema/conventions/reserved.yaml

touch schema/directory.yaml

# schema/json_schemas - typed payload schemas
mkdir -p schema/json_schemas/cloud_resource
touch schema/json_schemas/cloud_resource/ec2_instance.yaml
touch schema/json_schemas/cloud_resource/gce_instance.yaml
touch schema/json_schemas/cloud_resource/azure_vm.yaml
touch schema/json_schemas/cloud_resource/s3_bucket.yaml
touch schema/json_schemas/cloud_resource/gcs_bucket.yaml
touch schema/json_schemas/cloud_resource/azure_blob_container.yaml
touch schema/json_schemas/cloud_resource/rds_database.yaml
touch schema/json_schemas/cloud_resource/cloud_sql_instance.yaml
touch schema/json_schemas/cloud_resource/azure_sql.yaml
touch schema/json_schemas/cloud_resource/lambda_function.yaml
touch schema/json_schemas/cloud_resource/cloud_run_service.yaml
touch schema/json_schemas/cloud_resource/azure_function.yaml
touch schema/json_schemas/cloud_resource/vpc.yaml
touch schema/json_schemas/cloud_resource/vnet.yaml
touch schema/json_schemas/cloud_resource/cloud_network.yaml
touch schema/json_schemas/cloud_resource/load_balancer.yaml
touch schema/json_schemas/cloud_resource/application_gateway.yaml
touch schema/json_schemas/cloud_resource/cloud_lb.yaml
touch schema/json_schemas/cloud_resource/cloudfront_distribution.yaml
touch schema/json_schemas/cloud_resource/cloud_cdn.yaml
touch schema/json_schemas/cloud_resource/azure_cdn.yaml
touch schema/json_schemas/cloud_resource/iam_role.yaml
touch schema/json_schemas/cloud_resource/service_account.yaml
touch schema/json_schemas/cloud_resource/azure_service_principal.yaml
touch schema/json_schemas/cloud_resource/route53_zone.yaml
touch schema/json_schemas/cloud_resource/cloud_dns_zone.yaml
touch schema/json_schemas/cloud_resource/azure_dns_zone.yaml
touch schema/json_schemas/cloud_resource/cloudwatch_log_group.yaml
touch schema/json_schemas/cloud_resource/cloud_logging_bucket.yaml
touch schema/json_schemas/cloud_resource/log_analytics_workspace.yaml

mkdir -p schema/json_schemas/authority
touch schema/json_schemas/authority/prometheus_server.yaml
touch schema/json_schemas/authority/log_aggregator.yaml
touch schema/json_schemas/authority/secret_vault.yaml
touch schema/json_schemas/authority/wiki.yaml
touch schema/json_schemas/authority/dashboard_platform.yaml
touch schema/json_schemas/authority/code_repository.yaml
touch schema/json_schemas/authority/identity_provider.yaml
touch schema/json_schemas/authority/runbook_store.yaml
touch schema/json_schemas/authority/ticketing_system.yaml
touch schema/json_schemas/authority/chat_platform.yaml
touch schema/json_schemas/authority/status_page.yaml
touch schema/json_schemas/authority/artifact_registry.yaml
touch schema/json_schemas/authority/container_registry.yaml

mkdir -p schema/json_schemas/policy
touch schema/json_schemas/policy/security_zone.yaml
touch schema/json_schemas/policy/data_classification.yaml
touch schema/json_schemas/policy/retention.yaml
touch schema/json_schemas/policy/approval_rule.yaml
touch schema/json_schemas/policy/escalation.yaml
touch schema/json_schemas/policy/change_management.yaml
touch schema/json_schemas/policy/schedule_governance.yaml
touch schema/json_schemas/policy/access_control.yaml
touch schema/json_schemas/policy/compliance_scope.yaml

mkdir -p schema/json_schemas/runner_spec
touch schema/json_schemas/runner_spec/config_apply.yaml
touch schema/json_schemas/runner_spec/template_generate.yaml
touch schema/json_schemas/runner_spec/k8s_apply.yaml
touch schema/json_schemas/runner_spec/cloud_provision.yaml
touch schema/json_schemas/runner_spec/monitor_collect.yaml
touch schema/json_schemas/runner_spec/alert_dispatch.yaml
touch schema/json_schemas/runner_spec/drift_detect.yaml
touch schema/json_schemas/runner_spec/verify_evidence.yaml
touch schema/json_schemas/runner_spec/reconcile.yaml
touch schema/json_schemas/runner_spec/scheduler_enforce.yaml
touch schema/json_schemas/runner_spec/puller.yaml
touch schema/json_schemas/runner_spec/compliance_scan.yaml
touch schema/json_schemas/runner_spec/credential_rotator.yaml
touch schema/json_schemas/runner_spec/certificate_renewer.yaml
touch schema/json_schemas/runner_spec/manual_operation_tracker.yaml

mkdir -p schema/json_schemas/schedule
touch schema/json_schemas/schedule/cron_expression.yaml
touch schema/json_schemas/schedule/rate_based.yaml
touch schema/json_schemas/schedule/event_triggered.yaml
touch schema/json_schemas/schedule/calendar_anchored.yaml
touch schema/json_schemas/schedule/deadline_driven.yaml
touch schema/json_schemas/schedule/manual.yaml

mkdir -p schema/json_schemas/monitor
touch schema/json_schemas/monitor/script_local.yaml
touch schema/json_schemas/monitor/script_remote.yaml
touch schema/json_schemas/monitor/prometheus_query.yaml
touch schema/json_schemas/monitor/http_probe.yaml
touch schema/json_schemas/monitor/tcp_probe.yaml
touch schema/json_schemas/monitor/cloud_metric.yaml
touch schema/json_schemas/monitor/k8s_event_watch.yaml

mkdir -p schema/json_schemas/evidence_record
touch schema/json_schemas/evidence_record/backup_verification.yaml
touch schema/json_schemas/evidence_record/certificate_validity.yaml
touch schema/json_schemas/evidence_record/compliance_scan.yaml
touch schema/json_schemas/evidence_record/credential_rotation_verification.yaml
touch schema/json_schemas/evidence_record/access_review.yaml
touch schema/json_schemas/evidence_record/physical_inspection.yaml
touch schema/json_schemas/evidence_record/tape_rotation_completed.yaml
touch schema/json_schemas/evidence_record/keycard_revocation_completed.yaml
touch schema/json_schemas/evidence_record/license_renewal_completed.yaml
touch schema/json_schemas/evidence_record/vendor_contract_review_completed.yaml

mkdir -p schema/json_schemas/manual_operation
touch schema/json_schemas/manual_operation/tape_rotation.yaml
touch schema/json_schemas/manual_operation/vendor_review.yaml
touch schema/json_schemas/manual_operation/keycard_audit.yaml
touch schema/json_schemas/manual_operation/license_renewal.yaml
touch schema/json_schemas/manual_operation/contract_renewal.yaml
touch schema/json_schemas/manual_operation/evidence_collection.yaml
touch schema/json_schemas/manual_operation/physical_inspection.yaml

mkdir -p schema/json_schemas/storage_resource
touch schema/json_schemas/storage_resource/ebs.yaml
touch schema/json_schemas/storage_resource/s3.yaml
touch schema/json_schemas/storage_resource/gcs.yaml
touch schema/json_schemas/storage_resource/azure_blob.yaml
touch schema/json_schemas/storage_resource/nfs_export.yaml
touch schema/json_schemas/storage_resource/ceph_rbd.yaml
touch schema/json_schemas/storage_resource/local_disk.yaml
touch schema/json_schemas/storage_resource/iscsi.yaml

mkdir -p schema/json_schemas/configuration_variable
touch schema/json_schemas/configuration_variable/string.yaml
touch schema/json_schemas/configuration_variable/int.yaml
touch schema/json_schemas/configuration_variable/float.yaml
touch schema/json_schemas/configuration_variable/bool.yaml
touch schema/json_schemas/configuration_variable/json.yaml
touch schema/json_schemas/configuration_variable/secret_reference.yaml

# schema/domains - entity YAML files
mkdir -p schema/domains/01_identity
touch schema/domains/01_identity/site.yaml
touch schema/domains/01_identity/location.yaml
touch schema/domains/01_identity/ops_user.yaml
touch schema/domains/01_identity/ops_group.yaml
touch schema/domains/01_identity/ops_group_member.yaml
touch schema/domains/01_identity/ops_user_role.yaml
touch schema/domains/01_identity/ops_user_role_member.yaml

mkdir -p schema/domains/02_substrate
touch schema/domains/02_substrate/hardware_component.yaml
touch schema/domains/02_substrate/hardware_port.yaml
touch schema/domains/02_substrate/hardware_set.yaml
touch schema/domains/02_substrate/hardware_set_component.yaml
touch schema/domains/02_substrate/hardware_set_instance.yaml
touch schema/domains/02_substrate/hardware_set_instance_port_connection.yaml
touch schema/domains/02_substrate/megavisor.yaml
touch schema/domains/02_substrate/megavisor_instance.yaml
touch schema/domains/02_substrate/cloud_provider.yaml
touch schema/domains/02_substrate/cloud_account.yaml
touch schema/domains/02_substrate/cloud_resource.yaml
touch schema/domains/02_substrate/storage_resource.yaml
touch schema/domains/02_substrate/platform.yaml
touch schema/domains/02_substrate/machine.yaml

mkdir -p schema/domains/03_service
touch schema/domains/03_service/package.yaml
touch schema/domains/03_service/package_interface.yaml
touch schema/domains/03_service/package_connection.yaml
touch schema/domains/03_service/service.yaml
touch schema/domains/03_service/service_package.yaml
touch schema/domains/03_service/service_interface_mount.yaml
touch schema/domains/03_service/service_connection.yaml
touch schema/domains/03_service/host_group.yaml
touch schema/domains/03_service/host_group_machine.yaml
touch schema/domains/03_service/host_group_package.yaml
touch schema/domains/03_service/site_location.yaml
touch schema/domains/03_service/service_level.yaml
touch schema/domains/03_service/service_level_metric.yaml

mkdir -p schema/domains/04_kubernetes
touch schema/domains/04_kubernetes/k8s_cluster.yaml
touch schema/domains/04_kubernetes/k8s_cluster_node.yaml
touch schema/domains/04_kubernetes/k8s_namespace.yaml
touch schema/domains/04_kubernetes/k8s_workload.yaml
touch schema/domains/04_kubernetes/k8s_pod.yaml
touch schema/domains/04_kubernetes/k8s_helm_release.yaml
touch schema/domains/04_kubernetes/k8s_config_map.yaml
touch schema/domains/04_kubernetes/k8s_secret_reference.yaml
touch schema/domains/04_kubernetes/k8s_service.yaml

mkdir -p schema/domains/05_authority
touch schema/domains/05_authority/authority.yaml
touch schema/domains/05_authority/authority_pointer.yaml
touch schema/domains/05_authority/service_authority_pointer.yaml
touch schema/domains/05_authority/machine_authority_pointer.yaml
touch schema/domains/05_authority/k8s_cluster_authority_pointer.yaml
touch schema/domains/05_authority/cloud_resource_authority_pointer.yaml

mkdir -p schema/domains/06_schedule
touch schema/domains/06_schedule/schedule.yaml
touch schema/domains/06_schedule/runner_schedule.yaml
touch schema/domains/06_schedule/credential_rotation_schedule.yaml
touch schema/domains/06_schedule/certificate_expiration_schedule.yaml
touch schema/domains/06_schedule/compliance_audit_schedule.yaml
touch schema/domains/06_schedule/manual_operation_schedule.yaml
touch schema/domains/06_schedule/manual_operation.yaml

mkdir -p schema/domains/07_policy
touch schema/domains/07_policy/policy.yaml
touch schema/domains/07_policy/service_policy.yaml
touch schema/domains/07_policy/machine_policy.yaml
touch schema/domains/07_policy/k8s_namespace_policy.yaml
touch schema/domains/07_policy/cloud_account_policy.yaml
touch schema/domains/07_policy/security_zone.yaml
touch schema/domains/07_policy/security_zone_membership_service.yaml
touch schema/domains/07_policy/security_zone_membership_machine.yaml
touch schema/domains/07_policy/security_zone_membership_k8s_namespace.yaml
touch schema/domains/07_policy/data_classification.yaml
touch schema/domains/07_policy/retention_policy.yaml
touch schema/domains/07_policy/approval_rule.yaml
touch schema/domains/07_policy/escalation_path.yaml
touch schema/domains/07_policy/escalation_step.yaml
touch schema/domains/07_policy/service_escalation_path.yaml
touch schema/domains/07_policy/change_management_rule.yaml
touch schema/domains/07_policy/compliance_regime.yaml
touch schema/domains/07_policy/compliance_scope_service.yaml
touch schema/domains/07_policy/compliance_scope_data_classification.yaml

mkdir -p schema/domains/08_docs
touch schema/domains/08_docs/service_ownership.yaml
touch schema/domains/08_docs/machine_ownership.yaml
touch schema/domains/08_docs/k8s_cluster_ownership.yaml
touch schema/domains/08_docs/cloud_resource_ownership.yaml
touch schema/domains/08_docs/service_stakeholder.yaml
touch schema/domains/08_docs/runbook_reference.yaml
touch schema/domains/08_docs/service_runbook_reference.yaml
touch schema/domains/08_docs/dashboard_reference.yaml
touch schema/domains/08_docs/service_dashboard_reference.yaml

mkdir -p schema/domains/09_runner
touch schema/domains/09_runner/runner_spec.yaml
touch schema/domains/09_runner/runner_capability.yaml
touch schema/domains/09_runner/runner_machine.yaml
touch schema/domains/09_runner/runner_instance.yaml
touch schema/domains/09_runner/runner_service_target.yaml
touch schema/domains/09_runner/runner_host_group_target.yaml
touch schema/domains/09_runner/runner_k8s_namespace_target.yaml
touch schema/domains/09_runner/runner_cloud_account_target.yaml
touch schema/domains/09_runner/runner_job.yaml
touch schema/domains/09_runner/runner_job_target_machine.yaml
touch schema/domains/09_runner/runner_job_target_service.yaml
touch schema/domains/09_runner/runner_job_target_k8s_workload.yaml
touch schema/domains/09_runner/runner_job_target_cloud_resource.yaml
touch schema/domains/09_runner/runner_job_output_var.yaml

mkdir -p schema/domains/10_monitoring
touch schema/domains/10_monitoring/monitor.yaml
touch schema/domains/10_monitoring/monitor_machine_target.yaml
touch schema/domains/10_monitoring/monitor_service_target.yaml
touch schema/domains/10_monitoring/monitor_k8s_workload_target.yaml
touch schema/domains/10_monitoring/monitor_cloud_resource_target.yaml
touch schema/domains/10_monitoring/prometheus_config.yaml
touch schema/domains/10_monitoring/prometheus_scrape_target.yaml
touch schema/domains/10_monitoring/monitor_level.yaml
touch schema/domains/10_monitoring/alert.yaml
touch schema/domains/10_monitoring/alert_dependency.yaml
touch schema/domains/10_monitoring/alert_fire.yaml
touch schema/domains/10_monitoring/on_call_schedule.yaml
touch schema/domains/10_monitoring/on_call_assignment.yaml

mkdir -p schema/domains/11_observation
touch schema/domains/11_observation/observation_cache_metric.yaml
touch schema/domains/11_observation/observation_cache_state.yaml
touch schema/domains/11_observation/observation_cache_config.yaml

mkdir -p schema/domains/12_config
touch schema/domains/12_config/configuration_variable.yaml

mkdir -p schema/domains/13_change_mgmt
touch schema/domains/13_change_mgmt/change_set.yaml
touch schema/domains/13_change_mgmt/change_set_field_change.yaml
touch schema/domains/13_change_mgmt/change_set_approval_required.yaml
touch schema/domains/13_change_mgmt/change_set_approval.yaml
touch schema/domains/13_change_mgmt/change_set_rejection.yaml
touch schema/domains/13_change_mgmt/change_set_validation.yaml
touch schema/domains/13_change_mgmt/change_set_emergency_review.yaml
touch schema/domains/13_change_mgmt/change_set_bulk_membership.yaml

mkdir -p schema/domains/14_audit
touch schema/domains/14_audit/audit_log_entry.yaml
touch schema/domains/14_audit/evidence_record.yaml
touch schema/domains/14_audit/evidence_record_service_target.yaml
touch schema/domains/14_audit/evidence_record_machine_target.yaml
touch schema/domains/14_audit/evidence_record_credential_target.yaml
touch schema/domains/14_audit/evidence_record_certificate_target.yaml
touch schema/domains/14_audit/evidence_record_compliance_regime_target.yaml
touch schema/domains/14_audit/evidence_record_manual_operation_target.yaml
touch schema/domains/14_audit/compliance_finding.yaml
touch schema/domains/14_audit/compliance_finding_target_service.yaml

mkdir -p schema/domains/15_schema_meta
touch schema/domains/15_schema_meta/_schema_version.yaml
touch schema/domains/15_schema_meta/_schema_change_set.yaml
touch schema/domains/15_schema_meta/_schema_entity_type.yaml
touch schema/domains/15_schema_meta/_schema_field.yaml
touch schema/domains/15_schema_meta/_schema_relationship.yaml

# ============================================================
# dos/opsdb-ops-prod
# ============================================================
mkdir -p dos/opsdb-ops-prod/auth
mkdir -p dos/opsdb-ops-prod/seed
mkdir -p dos/opsdb-ops-prod/runners/overrides
mkdir -p dos/opsdb-ops-prod/importers/credentials
touch dos/opsdb-ops-prod/config.yaml
touch dos/opsdb-ops-prod/auth/users.yaml
touch dos/opsdb-ops-prod/seed/site.yaml
touch dos/opsdb-ops-prod/seed/admin_user.yaml
touch dos/opsdb-ops-prod/seed/base_policies.yaml
touch dos/opsdb-ops-prod/seed/runner_service_accounts.yaml
touch dos/opsdb-ops-prod/seed/core_runner_specs.yaml
touch dos/opsdb-ops-prod/runners/enabled.yaml
touch dos/opsdb-ops-prod/runners/overrides/reaper.yaml
touch dos/opsdb-ops-prod/runners/overrides/notification.yaml
touch dos/opsdb-ops-prod/importers/enabled.yaml
touch dos/opsdb-ops-prod/importers/credentials/aws.yaml
touch dos/opsdb-ops-prod/importers/credentials/k8s.yaml
touch dos/opsdb-ops-prod/importers/credentials/pagerduty.yaml

# ============================================================
# dos/opsdb-ops-staging
# ============================================================
mkdir -p dos/opsdb-ops-staging/auth
mkdir -p dos/opsdb-ops-staging/seed
mkdir -p dos/opsdb-ops-staging/runners/overrides
mkdir -p dos/opsdb-ops-staging/importers/credentials
touch dos/opsdb-ops-staging/config.yaml
touch dos/opsdb-ops-staging/auth/users.yaml
touch dos/opsdb-ops-staging/seed/site.yaml
touch dos/opsdb-ops-staging/seed/admin_user.yaml
touch dos/opsdb-ops-staging/seed/base_policies.yaml
touch dos/opsdb-ops-staging/seed/runner_service_accounts.yaml
touch dos/opsdb-ops-staging/seed/core_runner_specs.yaml
touch dos/opsdb-ops-staging/runners/enabled.yaml
touch dos/opsdb-ops-staging/runners/overrides/reaper.yaml
touch dos/opsdb-ops-staging/runners/overrides/notification.yaml
touch dos/opsdb-ops-staging/importers/enabled.yaml
touch dos/opsdb-ops-staging/importers/credentials/aws.yaml
touch dos/opsdb-ops-staging/importers/credentials/k8s.yaml
touch dos/opsdb-ops-staging/importers/credentials/pagerduty.yaml

# ============================================================
# dos/ top-level
# ============================================================
touch dos/README.md

# ============================================================
# docs/
# ============================================================
mkdir -p docs/architecture
touch docs/architecture/overview.md
touch docs/architecture/schema-engine.md
touch docs/architecture/api-gate.md
touch docs/architecture/runner-pattern.md
touch docs/architecture/library-contracts.md
touch docs/architecture/importer-pattern.md
touch docs/architecture/n-substrate.md

mkdir -p docs/guides
touch docs/guides/quickstart.md
touch docs/guides/adding-a-dos.md
touch docs/guides/writing-a-runner.md
touch docs/guides/writing-an-importer.md
touch docs/guides/schema-evolution.md
touch docs/guides/approval-rules.md
touch docs/guides/dev-to-operational.md

mkdir -p docs/reference
touch docs/reference/cli.md
touch docs/reference/api-operations.md
touch docs/reference/entity-catalog.md
touch docs/reference/discriminator-catalog.md
touch docs/reference/evolution-rules.md
touch docs/reference/naming-conventions.md

mkdir -p docs/spec
touch docs/spec/OPSDB-1-overview.md
touch docs/spec/OPSDB-2-architecture.md
touch docs/spec/OPSDB-3-implementation.md
touch docs/spec/OPSDB-4-schema.md
touch docs/spec/OPSDB-5-runners.md
touch docs/spec/OPSDB-6-api.md
touch docs/spec/OPSDB-7-schema-construction.md
touch docs/spec/OPSDB-8-library-suite.md
touch docs/spec/OPSDB-9-vocabulary.md

mkdir -p docs/decisions
touch docs/decisions/001-go-only.md
touch docs/decisions/002-monorepo.md
touch docs/decisions/003-postgres-first.md
touch docs/decisions/004-n-from-start.md
touch docs/decisions/005-yaml-auth-bootstrap.md

# ============================================================
# scripts/
# ============================================================
mkdir -p scripts
touch scripts/seed.sh
touch scripts/build-all.sh
touch scripts/test-integration.sh
touch scripts/generate-entity-catalog.sh

# ============================================================
# .github/workflows/
# ============================================================
mkdir -p .github/workflows
touch .github/workflows/validate-schema.yaml
touch .github/workflows/test.yaml
touch .github/workflows/release.yaml

# ============================================================
echo "Done. $(find . -type f | grep -v .git | wc -l) files created."
