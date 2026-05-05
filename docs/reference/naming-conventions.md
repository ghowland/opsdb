# Naming Conventions Reference

All names in the schema follow these rules. Enforced by the loader's validator
via `internal/conventions/naming.go`.

## Entity Names

| Rule | Example | Violation |
|------|---------|-----------|
| Lowercase with underscores | `service_connection` | `ServiceConnection`, `service-connection` |
| Singular form | `service` | `services` |
| No leading underscore (except schema meta) | `policy` | `_policy` |
| Leading underscore for schema metadata only | `_schema_version` | `schema_version` (for meta tables) |
| Maximum 63 characters | — | Postgres identifier limit |
| No double underscores | `service_connection` | `service__connection` |
| No trailing underscore | `service_connection` | `service_connection_` |

## Field Names

| Rule | Example | Violation |
|------|---------|-----------|
| Lowercase with underscores | `max_replicas` | `maxReplicas` |
| `_time` suffix for DATETIME fields | `created_time`, `approved_for_production_time` | `created_at`, `created_date` |
| `_date` suffix for DATE fields | `expiration_date` | `expiration_time` (if DATE type) |
| `is_` prefix for present-state booleans | `is_active`, `is_emergency` | `active`, `emergency` |
| `was_` prefix for past-event booleans | `was_reviewed` | `reviewed` (if past-event) |
| `_` prefix for governance/admin fields | `_requires_group`, `_access_classification` | `requires_group` |

## Foreign Key Names

| Rule | Example | Violation |
|------|---------|-----------|
| Named `{referenced_table}_id` | `service_id`, `change_set_id` | `svc_id`, `cs_id` |
| Role prefix when multiple FKs to same target | `vendor_company_id`, `client_company_id` | `company_id_1`, `company_id_2` |
| Self-referential FK for hierarchical | `parent_location_id` | `parent_id` |
| Self-referential FK for version chain | `parent_service_version_id` | `prior_version_id` |

## Composite Names

| Rule | Example |
|------|---------|
| Hierarchical prefix: specific-to-general | `cloud_resource_security_group_membership` |
| Each component carries meaning | `service_authority_pointer` = pointer linking service to authority |
| Recomposable without provenance loss | Reading the name tells you the relationship |

## Index Names

| Pattern | Example |
|---------|---------|
| `idx_{entity}_{fields}` | `idx_service_connection_source_service_id` |
| `uq_{entity}_{fields}` for unique | `uq_ops_user_username` |
| `ck_{entity}_{field}_{type}` for check | `ck_service_min_replicas_range` |
| `fk_{entity}_{field}` for foreign key | `fk_service_connection_source_service_id` |

## Discriminator Pattern Names

| Component | Pattern | Example |
|-----------|---------|---------|
| Type field | `{concept}_type` | `cloud_resource_type`, `policy_type` |
| Payload field | `{concept}_data_json` | `cloud_data_json`, `policy_data_json` |
| Type values | `lowercase_underscore` | `ec2_instance`, `approval_rule` |

## Version Sibling Names

| Component | Pattern | Example |
|-----------|---------|---------|
| Sibling table | `{entity}_version` | `service_version` |
| Parent FK | `{entity}_id` | `service_id` |
| Version chain FK | `parent_{entity}_version_id` | `parent_service_version_id` |

## Why Verbose

Names get long: `cloud_resource_security_group_membership`,
`change_set_emergency_review`, `evidence_record_compliance_regime_target`.
This is the price of unambiguous structural meaning. Each component carries
meaning. The pattern is `parent_concept_subconcept`.

The alternative — short names — requires institutional knowledge. New team
members guess wrong. Old members forget. The cost is keystrokes. The benefit
is structural transparency: new tables fit the pattern, readers learn the
pattern once and apply it across the schema.
