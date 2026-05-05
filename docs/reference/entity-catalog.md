# Entity Catalog

Auto-generated from schema YAML files. Do not edit manually.
Re-generate with: `scripts/generate-entity-catalog.sh`

This file is a placeholder. Run the generator to populate it from the
current schema:

```bash
scripts/generate-entity-catalog.sh
```

See `schema/directory.yaml` for the complete ordered list of entities
and `schema/domains/` for the YAML definitions.

## Summary

138 entities across 17 categories:

| Category | Entity count | Scope |
|----------|-------------|-------|
| Site and Location | 2 | DOS scope, physical/logical locations |
| Identity | 5 | Users, groups, roles |
| Substrate | 15 | Hardware, megavisor, machines, cloud, storage |
| Service Abstraction | 14 | Services, packages, interfaces, connections, host groups |
| Kubernetes | 9 | Clusters, nodes, namespaces, workloads, pods, helm, configmaps |
| Cloud Resources | 1 (+30 discriminator types) | Generic resource with typed payloads |
| Authority Directory | 6 | Typed pointers to monitoring, logs, secrets, docs |
| Schedules | 7 | When things happen |
| Policy | 17 | Security zones, classifications, retention, approval, escalation |
| Documentation Metadata | 9 | Ownership, runbooks, dashboards |
| Runners | 15 | Specs, capabilities, jobs, output, targets |
| Monitoring and Alerting | 14 | Monitors, alerts, on-call, suppression |
| Cached Observation | 3 | Pulled state from authorities |
| Configuration Variables | 1 (+6 discriminator types) | Typed key-value |
| Change Management | 8 | Change sets, approvals, validation |
| Audit and Evidence | 10 | Audit log, evidence, compliance findings |
| Schema Metadata | 5 | OpsDB record of own schema |
