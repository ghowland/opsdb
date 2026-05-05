# Dev to Operational Transition

How to move from a development OpsDB to a production-governed one.

## What changes

| Concern | Dev | Operational |
|---------|-----|-------------|
| API | Minimal: authenticated reads and writes only | Full 10-step gate with all enforcement |
| Change management | None — direct writes | Change sets for all change-managed entities |
| Authorization | Rough role-based read/write | Five-layer model |
| Runner report keys | Not enforced | Enforced — undeclared keys rejected |
| Audit log | Simple request logging | Append-only with full attribution |
| Data ingestion | Ad-hoc scripts writing directly | Registered runners with declared scopes |

## The cutover

This is a moment, not a gradient. Before the cutover, ad-hoc scripts can write
to the dev substrate. After, only registered runners with declared scopes can
write to the operational substrate.

Plan, schedule, and execute this deliberately.

## Steps

### 1. Document what was loaded under dev API

List every data source that fed the dev OpsDB, what scripts loaded the data,
and what entity types were populated. This is your baseline inventory.

### 2. Register runner specs for every data source

For each dev script that wrote data, create a `runner_spec` row declaring:
- What entity types it reads and writes
- What report keys it uses
- What targets it operates on (services, namespaces, cloud accounts)
- What schedule it runs on
- What capabilities it declares

### 3. Create service accounts for every runner

Each runner gets its own ops_user with `auth_source: service_account`.
Token issuance goes through the secret backend.

### 4. Write approval rules

Define which entity types require approval, who approves, and what
auto-approves. Start with the base policies from the seed files and
adjust to your organization's requirements.

### 5. Turn on the operational API

Replace the dev API process with one running the full 10-step gate.
From this point forward:

- Every write is authenticated and authorized through five layers
- Every change-managed write goes through a change set
- Every observation write is validated against report key declarations
- Every operation produces an append-only audit log entry

### 6. Retire dev scripts

Replace each ad-hoc script with its registered runner. The runner uses
the shared library suite, authenticates with its service account, and
writes through the API.

## What happens to historical data

Data loaded under the dev API is preserved. Change set history for that data
is incomplete — that is acceptable. Audit trails for initial loading are
incomplete. OpsDB's governance claims apply to changes after cutover, not
retroactively.

The schema metadata records the schema version at cutover as the canonical
baseline. Subsequent schema changes flow through `_schema_change_set`.

## Common mistakes

- **Gradual transition** — half dev, half operational creates a period where
  governance is advisory. Make it a clean cutover.
- **Skipping runner registration** — scripts that bypass the API after cutover
  break the audit trail and make OpsDB's claims about governance unreliable.
- **Not writing approval rules first** — without rules, the change management
  step has nothing to enforce. Write rules before cutover.
- **Leaving dev credentials active** — revoke dev-era tokens and passwords
  at cutover. Only registered service accounts and OIDC-authenticated humans
  should have access post-cutover.
