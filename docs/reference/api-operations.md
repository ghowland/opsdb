# API Operations Reference

16 operations. Every one traverses the 10-step gate.

## Read Operations

### get_entity

Fetch one entity row by primary key.

- **Method:** `GET /api/v1/{entity_type}/{id}`
- **Class:** read
- **Auth:** five-layer authorization
- **Returns:** entity row with all fields caller is authorized to see. Fields above caller's classification are omitted silently.

### get_entity_history

Fetch the version chain for one entity.

- **Method:** `GET /api/v1/{entity_type}/{id}/history`
- **Class:** read
- **Auth:** five-layer authorization
- **Query params:** `from_time`, `to_time` (optional time range filter)
- **Returns:** ordered list of version rows, newest first. Each version contains full entity state at that point.

### get_entity_at_time

Reconstruct entity state at a specific timestamp.

- **Method:** `GET /api/v1/{entity_type}/{id}/at?time={RFC3339}`
- **Class:** read
- **Auth:** five-layer authorization
- **Returns:** single version row representing entity state at the requested time. O(1) lookup against version sibling table.

### search

Discovery surface across entity types.

- **Method:** `POST /api/v1/search`
- **Class:** read
- **Auth:** five-layer authorization
- **Body:**

```json
{
  "entity_type": "service",
  "filters": [
    {"field": "name", "operator": "like", "value": "api-%"},
    {"field": "is_active", "operator": "eq", "value": true}
  ],
  "joins": ["service_ownership"],
  "projection": "standard",
  "ordering": [{"field": "name", "direction": "asc"}],
  "limit": 50,
  "cursor": "",
  "max_staleness_seconds": 300
}
```

- **Filter operators:** `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `in`, `like` (anchored only: `prefix%` or `%suffix`), `is_null`, `is_not_null`, `between`, `json_contains`
- **Projection modes:** `standard` (all non-governance fields), `summary` (name + id + is_active), `full_with_history` (includes version chain), explicit field list
- **Bounds enforced:** max 1000 results per page, max 3 joins, max 30s query time, max 10 filter predicates
- **Returns:** `{rows, cursor, total_count, freshness_summary, filter_disclosures}`

### get_dependencies

Walk substrate hierarchy or service connection graph.

- **Method:** `GET /api/v1/{entity_type}/{id}/dependencies?pattern={pattern}&max_depth={n}`
- **Class:** read
- **Auth:** five-layer authorization
- **Patterns:** `substrate_parent_chain`, `service_connections`, `location_ancestry`, `host_group_machines`
- **Returns:** ordered list of `{entity_type, entity_id, depth, metadata}` with cycle detection

### resolve_authority_pointer

Where-is-X lookup.

- **Method:** `GET /api/v1/authority_pointer/{id}/resolve`
- **Class:** read
- **Auth:** five-layer authorization
- **Returns:** authority base_url, authority_type, pointer_type, locator, pointer_data_json, last_verified_time. Does NOT fetch from the authority.

### change_set_view

Scoped view of a change set for approvers.

- **Method:** `GET /api/v1/change_set/{id}/view`
- **Class:** read
- **Auth:** filtered to viewer's approval scope
- **Returns:** change set metadata, field changes, approval requirements, approval/rejection records, validation results

## Write Operations

### write_observation

Runner writes observation data. Direct write path, no change management.

- **Method:** `POST /api/v1/observation`
- **Class:** write-direct
- **Auth:** five-layer + report key enforcement
- **Body:**

```json
{
  "target_table": "observation_cache_state",
  "key": "pod_status",
  "value": "running",
  "data_json": {"restart_count": 0, "ready": true},
  "runner_job_id": 42,
  "authority_id": 7,
  "observed_time": "2025-01-15T10:30:00Z"
}
```

- **Target tables:** `observation_cache_metric`, `observation_cache_state`, `observation_cache_config`, `runner_job_output_var`, `evidence_record`
- **Report key enforcement:** submitted key must be declared in `runner_report_key` for the caller's runner spec. Undeclared keys rejected with `undeclared_report_key`.

### submit_change_set

Propose field changes through change management.

- **Method:** `POST /api/v1/change_set`
- **Class:** write-change-set
- **Auth:** five-layer + validation pipeline + change management routing
- **Body:**

```json
{
  "name": "Update service config",
  "description": "Increase replica count for API service",
  "reason": "Traffic growth",
  "field_changes": [
    {
      "entity_type": "service",
      "entity_id": 15,
      "field_name": "max_replicas",
      "before_value": 5,
      "after_value": 10,
      "change_type": "update",
      "version_stamp": 3
    }
  ],
  "ticket_ref": 42,
  "dry_run": false
}
```

- **Validation pipeline:** schema validation → bound validation → semantic validation → policy validation → lint validation → dependency check
- **Concurrency:** each field change's `version_stamp` checked against current entity version. Stale stamps return `stale_version` error.
- **Dry run:** `dry_run: true` runs full validation and computes approvals without writing rows.
- **Returns:** `{change_set_id, status, approval_required, validation_errors, dry_run_result}`

### emergency_apply

Break-glass path with reduced approvals.

- **Method:** `POST /api/v1/change_set/emergency`
- **Class:** write-change-set
- **Auth:** caller must have emergency authority per policy
- **Behavior:** same as submit but `is_emergency=true`, reduced approvals, creates `change_set_emergency_review` row with 72-hour review window.

### bulk_submit_change_set

Multi-entity atomic change set.

- **Method:** `POST /api/v1/change_set/bulk`
- **Class:** write-change-set
- **Behavior:** chunked validation (1000 field changes per chunk). All chunks must pass. Atomic commit or none. May produce bundle-level approval rather than per-entity.

## Change Management Actions

### approve_change_set

- **Method:** `POST /api/v1/change_set/{id}/approve`
- **Class:** cm-action
- **Auth:** caller must be in a required approver group
- **Body:** `{"comments": "Looks good"}`
- **Behavior:** inserts approval, increments fulfilled count. Transitions to approved when all requirements met.

### reject_change_set

- **Method:** `POST /api/v1/change_set/{id}/reject`
- **Class:** cm-action
- **Auth:** caller must be in a required approver group
- **Body:** `{"reason": "Replica count too high for current capacity"}`
- **Behavior:** inserts rejection. Rejection behavior per rule: `any_rejects_halts`, `majority_rejects_halts`, or `all_must_reject`.

### cancel_change_set

- **Method:** `POST /api/v1/change_set/{id}/cancel`
- **Class:** cm-action
- **Auth:** original submitter or user with cancel authority
- **Behavior:** transitions to cancelled. Only valid from draft, submitted, or pending_approval.

### apply_change_set_field_change

Used by the change-set executor runner.

- **Method:** `POST /api/v1/change_set/{cs_id}/apply_field/{fc_id}`
- **Class:** write-direct (post-approval)
- **Auth:** caller must have executor authority, change set must be approved, field change must be pending
- **Behavior:** updates target entity row, writes version sibling, marks field change as applied.

### mark_change_set_applied

- **Method:** `POST /api/v1/change_set/{id}/mark_applied`
- **Class:** cm-action
- **Auth:** executor authority
- **Behavior:** verifies all field changes applied, transitions to applied, sets applied_time.

## Streaming

### watch

- **Method:** `GET /api/v1/watch/{entity_type}?resume_token={token}`
- **Class:** stream
- **Auth:** five-layer authorization
- **Behavior:** SSE or WebSocket stream. Without resume token: sends SNAPSHOT events for all matching entities then streams changes. With resume token: sends SYNC event with current state then streams from resume point. Level-triggered backstop on reconnect.
- **Events:** `ADDED`, `MODIFIED`, `DELETED`, `SYNC`, `SNAPSHOT`
