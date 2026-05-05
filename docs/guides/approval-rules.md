# Approval Rules

How change management routing works and how to configure it.

## The mechanism

Every write to a change-managed entity goes through a change set. The API's
step 7 (change management routing) evaluates approval rules against the
proposed changes and computes who must approve before the change applies.

The pipeline:

1. **Enumerate field changes** — list of (entity_type, entity_id, field_name) tuples.
2. **Walk ownership bridges** — service_ownership, machine_ownership, etc. to find owners.
3. **Walk stakeholder bridges** — service_stakeholder and others for interested parties.
4. **Evaluate approval rules** — policy rows of type `approval_rule` matched against the change.
5. **Compute requirements** — one `change_set_approval_required` row per matching rule.
6. **Check auto-approval** — if all requirements satisfiable automatically, skip humans.

## Writing an approval rule

Approval rules are policy rows with `policy_type: approval_rule`. The
`policy_data_json` controls matching and requirements.

### Match criteria

```yaml
policy_data_json:
  match:
    entity_types: ["service", "machine"]    # which entity types trigger this rule
    fields: ["config_data_json"]            # optional: only these fields (empty = all)
    exclude_fields: ["description"]         # optional: skip these fields
    context:
      security_zone: ["production"]         # optional: only in these zones
      data_classification: ["confidential"] # optional: only at this classification+
```

All match criteria are AND-composed. A change must match every specified
criterion to trigger the rule. Omitted criteria match everything.

### Requirements

```yaml
  require:
    approver_count: 1                       # how many approvals needed
    approver_source: ownership              # ownership, stakeholder, or explicit group
    required_groups: ["security_reviewers"] # at least one approver from this group
  rejection_behavior: any_rejects_halts     # or majority_rejects_halts, all_must_reject
```

### Auto-approval

```yaml
  auto_approve: true
```

When `auto_approve: true`, the change set transitions through pending_approval
to approved without human intervention. The approval is recorded in the audit
trail as auto-approved with the rule that authorized it.

Use auto-approval for low-risk, high-frequency changes: drift corrections in
staging, routine credential rotations, observation cache updates.

## Gating modes

Changes flow through one of three paths:

| Path | When | Examples |
|------|------|---------|
| Direct write | Observation-only data, never change-managed | Cached observations, evidence records, runner jobs |
| Auto-approved change set | Policy auto-approves without human | Drift corrections in non-prod, minor config |
| Approval-required change set | Routes to human approvers | Production config, security policy, schema |

The same runner can have different gating for different targets. A drift
detector auto-approves staging corrections but requires approval for production
changes. This is controlled by the approval rules matching on context
(security zone, entity type, field names), not by the runner's code.

## Emergency path

Emergency changes use `is_emergency: true`. They commit with reduced approvals
(often single approver, sometimes self-approved) but create a
`change_set_emergency_review` row that must be reviewed within 72 hours
(configurable via the `cm_emergency_review_window` policy). The emergency
review monitor runner escalates overdue reviews.

## Separation of duty

The submitter of a change set cannot approve their own change set (when SoD
policy is active). This is enforced at approval time — the API checks
`change_set.submitted_by_ops_user_id != approver_ops_user_id`.

## Adding a new rule

Submit a change set modifying the policy entity:

```bash
# via API or through UI that sits on top of API
POST /api/v1/change_set
{
  "name": "Add compliance approval rule",
  "reason": "SOX requires two approvers for financial service changes",
  "field_changes": [{
    "entity_type": "policy",
    "entity_id": 0,
    "field_name": "*",
    "change_type": "create",
    "after_value": {
      "name": "approval_financial_services",
      "policy_type": "approval_rule",
      "policy_data_json": { ... }
    }
  }]
}
```

The new approval rule itself goes through the existing approval pipeline.
Approval rules for policies require the `security_reviewers` group
(per the base policies seed).
