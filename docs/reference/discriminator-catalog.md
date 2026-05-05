# Discriminator Catalog

Entities using the typed payload pattern: a `*_type` enum field selects
which JSON schema validates the `*_data_json` payload.

## cloud_resource

**Discriminator field:** `cloud_resource_type`
**Payload field:** `cloud_data_json`

| Type value | JSON schema | Provider |
|------------|-------------|----------|
| ec2_instance | cloud_resource/ec2_instance.yaml | AWS |
| gce_instance | cloud_resource/gce_instance.yaml | GCP |
| azure_vm | cloud_resource/azure_vm.yaml | Azure |
| s3_bucket | cloud_resource/s3_bucket.yaml | AWS |
| gcs_bucket | cloud_resource/gcs_bucket.yaml | GCP |
| azure_blob_container | cloud_resource/azure_blob_container.yaml | Azure |
| rds_database | cloud_resource/rds_database.yaml | AWS |
| cloud_sql_instance | cloud_resource/cloud_sql_instance.yaml | GCP |
| azure_sql | cloud_resource/azure_sql.yaml | Azure |
| lambda_function | cloud_resource/lambda_function.yaml | AWS |
| cloud_run_service | cloud_resource/cloud_run_service.yaml | GCP |
| azure_function | cloud_resource/azure_function.yaml | Azure |
| vpc | cloud_resource/vpc.yaml | AWS |
| vnet | cloud_resource/vnet.yaml | Azure |
| cloud_network | cloud_resource/cloud_network.yaml | GCP |
| load_balancer | cloud_resource/load_balancer.yaml | AWS |
| application_gateway | cloud_resource/application_gateway.yaml | Azure |
| cloud_lb | cloud_resource/cloud_lb.yaml | GCP |
| cloudfront_distribution | cloud_resource/cloudfront_distribution.yaml | AWS |
| cloud_cdn | cloud_resource/cloud_cdn.yaml | GCP |
| azure_cdn | cloud_resource/azure_cdn.yaml | Azure |
| iam_role | cloud_resource/iam_role.yaml | AWS |
| service_account | cloud_resource/service_account.yaml | GCP |
| azure_service_principal | cloud_resource/azure_service_principal.yaml | Azure |
| route53_zone | cloud_resource/route53_zone.yaml | AWS |
| cloud_dns_zone | cloud_resource/cloud_dns_zone.yaml | GCP |
| azure_dns_zone | cloud_resource/azure_dns_zone.yaml | Azure |
| cloudwatch_log_group | cloud_resource/cloudwatch_log_group.yaml | AWS |
| cloud_logging_bucket | cloud_resource/cloud_logging_bucket.yaml | GCP |
| log_analytics_workspace | cloud_resource/log_analytics_workspace.yaml | Azure |

## storage_resource

**Discriminator field:** `storage_resource_type`
**Payload field:** `storage_data_json`

| Type value | JSON schema |
|------------|-------------|
| ebs | storage_resource/ebs.yaml |
| s3 | storage_resource/s3.yaml |
| gcs | storage_resource/gcs.yaml |
| azure_blob | storage_resource/azure_blob.yaml |
| nfs_export | storage_resource/nfs_export.yaml |
| ceph_rbd | storage_resource/ceph_rbd.yaml |
| local_disk | storage_resource/local_disk.yaml |
| iscsi | storage_resource/iscsi.yaml |

## k8s_workload (version)

**Discriminator field:** `workload_type` (on parent k8s_workload)
**Payload field:** `workload_data_json`

| Type value | JSON schema |
|------------|-------------|
| deployment | (workload-specific fields in payload) |
| statefulset | |
| daemonset | |
| job | |
| cronjob | |
| replicaset | |

## authority

**Discriminator field:** `authority_type`
**Payload field:** `authority_data_json`

| Type value | JSON schema |
|------------|-------------|
| prometheus_server | authority/prometheus_server.yaml |
| log_aggregator | authority/log_aggregator.yaml |
| secret_vault | authority/secret_vault.yaml |
| wiki | authority/wiki.yaml |
| dashboard_platform | authority/dashboard_platform.yaml |
| code_repository | authority/code_repository.yaml |
| identity_provider | authority/identity_provider.yaml |
| runbook_store | authority/runbook_store.yaml |
| ticketing_system | authority/ticketing_system.yaml |
| chat_platform | authority/chat_platform.yaml |
| status_page | authority/status_page.yaml |
| artifact_registry | authority/artifact_registry.yaml |
| container_registry | authority/container_registry.yaml |

## authority_pointer

**Discriminator field:** `pointer_type`
**Payload field:** `pointer_data_json`

| Type value | JSON schema |
|------------|-------------|
| metric | (pointer to specific metric in monitoring authority) |
| log_query | (pointer to log query in log aggregator) |
| secret | (pointer to secret path in vault — never the value) |
| dashboard | (pointer to dashboard in dashboard platform) |
| runbook | (pointer to runbook in runbook store) |
| wiki_page | (pointer to wiki page) |
| ticket | (pointer to ticket in ticketing system) |
| code_path | (pointer to code path in repository) |
| chat_thread | (pointer to chat thread) |
| artifact | (pointer to artifact in registry) |
| container_image | (pointer to container image) |

## schedule

**Discriminator field:** `schedule_type`
**Payload field:** `schedule_data_json`

| Type value | JSON schema |
|------------|-------------|
| cron_expression | schedule/cron_expression.yaml |
| rate_based | schedule/rate_based.yaml |
| event_triggered | schedule/event_triggered.yaml |
| calendar_anchored | schedule/calendar_anchored.yaml |
| deadline_driven | schedule/deadline_driven.yaml |
| manual | schedule/manual.yaml |

## policy

**Discriminator field:** `policy_type`
**Payload field:** `policy_data_json`

| Type value | JSON schema |
|------------|-------------|
| security_zone | policy/security_zone.yaml |
| data_classification | policy/data_classification.yaml |
| retention | policy/retention.yaml |
| approval_rule | policy/approval_rule.yaml |
| escalation | policy/escalation.yaml |
| change_management | policy/change_management.yaml |
| schedule_governance | policy/schedule_governance.yaml |
| access_control | policy/access_control.yaml |
| compliance_scope | policy/compliance_scope.yaml |

## runner_spec (version)

**Discriminator field:** `runner_spec_type` (on parent runner_spec)
**Payload field:** `runner_data_json`

| Type value | JSON schema |
|------------|-------------|
| config_apply | runner_spec/config_apply.yaml |
| template_generate | runner_spec/template_generate.yaml |
| k8s_apply | runner_spec/k8s_apply.yaml |
| cloud_provision | runner_spec/cloud_provision.yaml |
| monitor_collect | runner_spec/monitor_collect.yaml |
| alert_dispatch | runner_spec/alert_dispatch.yaml |
| drift_detect | runner_spec/drift_detect.yaml |
| verify_evidence | runner_spec/verify_evidence.yaml |
| reconcile | runner_spec/reconcile.yaml |
| scheduler_enforce | runner_spec/scheduler_enforce.yaml |
| puller | runner_spec/puller.yaml |
| compliance_scan | runner_spec/compliance_scan.yaml |
| credential_rotator | runner_spec/credential_rotator.yaml |
| certificate_renewer | runner_spec/certificate_renewer.yaml |
| manual_operation_tracker | runner_spec/manual_operation_tracker.yaml |

## monitor

**Discriminator field:** `monitor_type`
**Payload field:** `monitor_data_json`

| Type value | JSON schema |
|------------|-------------|
| script_local | monitor/script_local.yaml |
| script_remote | monitor/script_remote.yaml |
| prometheus_query | monitor/prometheus_query.yaml |
| http_probe | monitor/http_probe.yaml |
| tcp_probe | monitor/tcp_probe.yaml |
| cloud_metric | monitor/cloud_metric.yaml |
| k8s_event_watch | monitor/k8s_event_watch.yaml |

## evidence_record

**Discriminator field:** `evidence_record_type`
**Payload field:** `evidence_record_data_json`

| Type value | JSON schema |
|------------|-------------|
| backup_verification | evidence_record/backup_verification.yaml |
| certificate_validity | evidence_record/certificate_validity.yaml |
| compliance_scan | evidence_record/compliance_scan.yaml |
| credential_rotation_verification | evidence_record/credential_rotation_verification.yaml |
| access_review | evidence_record/access_review.yaml |
| physical_inspection | evidence_record/physical_inspection.yaml |
| tape_rotation_completed | evidence_record/tape_rotation_completed.yaml |
| keycard_revocation_completed | evidence_record/keycard_revocation_completed.yaml |
| license_renewal_completed | evidence_record/license_renewal_completed.yaml |
| vendor_contract_review_completed | evidence_record/vendor_contract_review_completed.yaml |

## manual_operation

**Discriminator field:** `manual_operation_type`
**Payload field:** `manual_operation_data_json`

| Type value | JSON schema |
|------------|-------------|
| tape_rotation | manual_operation/tape_rotation.yaml |
| vendor_review | manual_operation/vendor_review.yaml |
| keycard_audit | manual_operation/keycard_audit.yaml |
| license_renewal | manual_operation/license_renewal.yaml |
| contract_renewal | manual_operation/contract_renewal.yaml |
| evidence_collection | manual_operation/evidence_collection.yaml |
| physical_inspection | manual_operation/physical_inspection.yaml |

## configuration_variable

**Discriminator field:** `variable_type`
**Payload field:** `variable_data_json`

| Type value | JSON schema |
|------------|-------------|
| string | configuration_variable/string.yaml |
| int | configuration_variable/int.yaml |
| float | configuration_variable/float.yaml |
| bool | configuration_variable/bool.yaml |
| json | configuration_variable/json.yaml |
| secret_reference | configuration_variable/secret_reference.yaml |

## JSON Schema Location

All JSON payload schemas live under `schema/json_schemas/{discriminator_entity}/`.
Each file is YAML (for consistency with entity schemas) defining the allowed
fields, types, and constraints for that discriminator value.

JSON payload schemas are one level deep. Lists-of-lists, maps-of-lists, and
lists-of-maps are forbidden. If a payload needs nested structure, factor it
into a separate entity.
