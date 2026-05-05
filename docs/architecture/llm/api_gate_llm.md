# OPSDB API LAYER — LLM-COMPACT FORM
# Format: pipe-delimited tables, ID refs.
# Read order: principles → operations → gate-steps → auth-layers → auth-delegation → auth-flows → search → predicates → view-modes → versioning → concurrency → lifecycle → stakeholder-routing → validation → auto-approval → emergency → bulk → report-keys-schema → report-keys-flow → report-keys-value → audit-log → audit-fields → tamper-evidence → boundaries → worked-example → relationships → sections

# principles(id|principle|rationale)
A1|API is the single gate|every interaction with OpsDB data passes through it; no out-of-band path
A2|API is self-contained operational software|not built on K8s/cloud/orchestrator/anything OpsDB models; outlives all authorities
A3|API delegates only authentication and credential resolution|IdP for human auth+secret backend for credentials; everything else local
A4|API does not invoke runners|every operation request-driven; no internal scheduler; no triggers fire on state transition
A5|API does not communicate with stakeholders|notification dispatch is a runner concern; API records state transitions
A6|API does not apply approved change_sets|change-set executor runner drains the queue; API gates the apply-writes
A7|API gates+validates+routes+records+responds|runners do world-side work
A8|every operation traverses the same gate|10 enforcement steps applied uniformly
A9|configuration as data not code|all governance evaluated against OpsDB rows; changing what API enforces = changing data
A10|0/1/N rule applied|one gate not two; one auth layer not parallel; one audit table not per-domain; one set of write ops not privileged-vs-regular
A11|schema validation forbids regex|DoS vector + complexity sink; declarative bounds only
A12|append-only at strictest level for audit log|DDL enforces no UPDATE/DELETE for any role
A13|API stays simple by refusing what doesn't belong|each boundary kept makes API better at what it does

# operations(id|name|class|purpose|reads|writes|gating)
AO1|get_entity|read|fetch one row by primary key|entity_type+id|-|five-layer auth
AO2|get_entity_history|read|fetch version chain for one entity|entity_type+id+optional time/version range|-|five-layer auth
AO3|get_entity_at_time|read|reconstruct field values active at timestamp|entity_type+id+timestamp|-|five-layer auth; one row lookup not chain replay
AO4|search|read|discovery surface across entity types|filter+joins+projection+ordering+pagination+freshness+view-mode|-|five-layer auth+bounds
AO5|get_dependencies|read|walk substrate hierarchy or service connections|starting entity+relationship pattern|-|five-layer auth+depth+cycle bounds
AO6|resolve_authority_pointer|read|where-is-X lookup|authority_pointer_id|-|five-layer auth
AO7|change_set_view|read|scoped or full view of change set for approver|change_set_id+mode|-|filtered to viewer's approval scope
AO8|write_observation|write-direct|runner writes observation/evidence|table+key+value|observation_cache_*|runner_job_output_var|evidence_record|runner report key + five-layer auth
AO9|submit_change_set|write-change-set|propose N field changes with reason|field changes+reason+ticket+urgency+metadata|change_set+change_set_field_change+change_set_approval_required|five-layer + validation pipeline + dry_run support
AO10|approve_change_set|change-mgmt-action|stakeholder approves|change_set_id+optional comments|change_set_approval+may transition status|verify caller in required approver group
AO11|reject_change_set|change-mgmt-action|stakeholder rejects|change_set_id+reason|change_set_rejection+may halt|verify caller in required approver group
AO12|cancel_change_set|change-mgmt-action|withdrawal|change_set_id|change_set status=cancelled|submitter or sufficient authority
AO13|emergency_apply|write-change-set|break-glass path|same as submit + emergency=true|change_set with is_emergency+change_set_emergency_review pending|emergency authority per policy
AO14|bulk_submit_change_set|write-change-set|transaction touching many entities|same as submit + bulk membership|change_set+change_set_bulk_membership|chunked validation+different approval rules possible
AO15|apply_change_set_field_change|write-direct|executor applies one field change|change_set_field_change_id|entity row update+version sibling row+applied_status|verify executor authority+change_set in approved+not yet applied
AO16|mark_change_set_applied|change-mgmt-action|finalize change set status|change_set_id|change_set status=applied|verify all field changes applied successfully

# gate_steps(step|name|what_happens|data_consulted|failure)
AG1|1|Authentication|verify caller identity against IdP (humans) or secret backend (runners); resolve to ops_user or runner_machine|IdP assertion+secret backend tokens+ops_user mapping|invalid creds → reject + audit
AG2|2|Authorization|evaluate five-layer model against operation+target+caller; first denial fails|5 layers (§auth_layers)|first denial → reject with layer info + audit
AG3|3|Schema Validation|verify operation shape matches registered schema for affected entity types and fields|_schema_entity_type+_schema_field|malformed → reject with structured error
AG4|4|Bound Validation|verify field values within declared bounds (numeric ranges+enum membership+FK existence+anchored patterns)|OPSDB-7 declarative constraints in schema|out-of-bounds → reject; no regex evaluated
AG5|5|Policy Evaluation|consult policy rows for additional governance (data classification+retention+SoD+time-of-day)|policy rows of relevant types|violation → reject
AG6|6|Versioning Preparation|prepare version row to be written for change-managed entities|*_version sibling schema|none (preparation step)
AG7|7|Change Management Routing|evaluate approval_rule policies+compute required approver groups+write change_set_approval_required|approval_rule policies+ownership/stakeholder bridges|determines auto-approve vs human approval vs blocking
AG8|8|Audit Logging|record operation in audit_log_entry with full attribution; append-only|audit_log_entry table|atomic with operation outcome
AG9|9|Execution|apply atomic write OR record change_set+field_changes OR record approval/rejection OR perform entity update+version write|target tables|substrate failure → recorded in audit
AG10|10|Response|return result with metadata (success/structured-error+affected-row-IDs+computed-approvals+audit-entry-id for correlation)|response shape per operation|none

# auth_layers(id|layer|source_data|check|denial_consequence)
AL1|Layer 1: Standard Role and Group|ops_user_role_member+ops_group_member|baseline access by role mapping to operation classes|operation rejected at layer 1
AL2|Layer 2: Per-Entity Governance|_requires_group field on rows|caller must be member of named group beyond layer 1|operation rejected at layer 2
AL3|Layer 3: Per-Field Classification|_access_classification on fields/tables|caller's clearance level must meet or exceed classification|specific fields omitted or operation rejected
AL4|Layer 4: Per-Runner Authority|runner_capability+runner_*_target bridges|operation must match runner's declared scope|operation rejected at layer 4
AL5|Layer 5: Policy Rules|policy rows of type access_control|time-of-day+SoD+tenure-based+other constraints|reject OR inject additional approval requirements
# composition: AND across all 5; first denial halts; audit records which layer denied + triggering policy

# auth_delegation(concern|delegated_to|api_owns|why)
AD1|human identity verification|IdP (LDAP|AD|OIDC|SAML)|ops_user mapping rows|identity changes are HR-driven; fast-moving
AD2|credential values|secret backend (Vault+equivalents)|pointers to where credentials live (k8s_secret_reference+config var of type secret_reference)|secrets need-to-know+audit-on-read+ephemeral semantics
AD3|operational authorization|nothing - locally enforced|access policies+policy rules|slower governance-driven; mixing with identity creates drift
AD4|runner identity|secret backend issues credentials|runner_machine mapping|runners authenticate with own service account credentials

# auth_flows(flow|mechanism|attribution)
AF1|Human Authentication|SSO via IdP→signed assertion→API verifies signature OR callback→resolves to ops_user|acting_ops_user_id
AF2|Runner Authentication|shared OpsDB API client acquires creds from secret backend at startup→refresh before expiration→creds in headers→API validates by callback or signed token→resolves to runner_machine|acting_service_account_id
AF3|Web-Application-Mediated|human SSO to web app→app calls runner with verified human identity→runner authenticates as itself→passes originating human identity in request|both: acting_service_account_id (runner) + acting_ops_user_id (human)

# search(id|capability|shape|notes)
AS1|Filter Predicates|equality+inequality+comparison+IN+LIKE-anchored+IS NULL+BETWEEN+JSON path containment|compose via AND/OR/NOT with explicit grouping; depth-bounded
AS2|Named Join Paths|service.host_group|service.connections|machine.megavisor_instance.parent_chain|entity.audit_log|change_set.field_changes|change_set.approvals_required|paths registered as schema metadata; recursive with cycle detection+depth bounds
AS3|Projection|standard|*|summary|full_with_history|explicit field lists|filtered by access policy; metadata indicates omitted fields
AS4|Ordering|list of (field+direction) pairs|deterministic; ties broken by id for stable cursor pagination
AS5|Pagination|cursor-based default|offset-based for results under threshold (default 10000)|cursors opaque; encode snapshot+stable ordering keys
AS6|Bounds|max result size+max join depth+max query time+max predicate composition depth+rate limits per caller|policy data; configurable per role; bounded queries refused with structured feedback
AS7|Freshness|max_staleness_seconds for cached observation rows|filters out stale rows; response indicates filtered count
AS8|View Modes|standard|with_history|at_time|standard=current; with_history=current+chain; at_time=reconstructed at timestamp via single version lookup
AS9|Result Reporting|matched rows + metadata (count|cursor|freshness summary|filtering disclosures|query trace for privileged callers)|trace useful for missing-index detection

# predicates(id|predicate|shape|restrictions)
AP1|Equality|field = value|none
AP2|Inequality|field != value|none
AP3|Comparison|field > | >= | < | <= value|on ordered types
AP4|Set Membership|field IN [values]|bounded list size
AP5|Pattern|field LIKE 'prefix%' or 'suffix' anchored|no regex; use simple anchored forms
AP6|Null Check|field IS NULL | IS NOT NULL|none
AP7|Range|field BETWEEN low AND high|on ordered types
AP8|JSON Containment|field_data_json @> path = value|for typed payload fields

# view_modes(mode|content|use_case)
AV1|standard|current state of each matched entity|normal operational queries
AV2|with_history|current state + full version chain|change history investigations
AV3|at_time|state of each matched entity reconstructed at timestamp|"show me what production looked like at incident time"

# versioning(id|aspect|mechanism|tradeoff)
AVE1|Full-State Version Rows|each version row contains all fields not just changed ones|storage cost ↑; reconstruction is O(1) row lookup not O(N) chain replay
AVE2|What Versioned|change-managed entities have *_version siblings|configuration+policies+schedules+metadata+ownership+stakeholder bridges+runner specs+capabilities+schema metadata
AVE3|What Not Versioned|observation tables+runner_job+runner_job_output_var+evidence_record+alert_fire+on_call_assignment|append-only or overwrite per pattern; world is SoT for observation
AVE4|Append Only Strict|audit_log_entry|DDL enforces no UPDATE/DELETE for any role
AVE5|Computed by Tooling|_schema_* tables populated on _schema_change_set apply|not directly written
AVE6|Sharding|schema-declarative for high-frequency entities; by time range or entity_id range or hybrid|API reads sharding scheme + routes; consumers see merged results transparently
AVE7|Retention|per entity-type default + _retention_policy_id override per row|reaper trims past horizon; whole shards droppable when entirely past horizon
AVE8|Rollback as Change Set|never side channel; submit change_set with field_changes restoring target version values|standard validation+approval pipeline; new version row records the rollback

# concurrency(step|mechanism|recovery)
AC1|Draft|each change_set_field_change carries version stamp of entity drafted against|none
AC2|Submit|API checks each touched entity's current version against drafted-against version|fails loud with stale_version error if any entity advanced
AC3|Recovery|submitter retrieves current values+reconciles proposed change against new state+resubmits|loud-at-submit prevents silent overwrites and post-approval failures
AC4|Approval|approvers never see stale state because submit failed already|none

# lifecycle(state|meaning|transitions_to|observed_by)
LC1|draft|under construction; not yet submitted|submitted|submitter
LC2|submitted|API received|validating (automatic)|API
LC3|validating|lint+schema+semantic+policy+lint checks running|pending_approval (success) | draft with errors (recoverable) | rejected (unrecoverable)|API
LC4|pending_approval|validation passed; awaiting stakeholder approvals|approved | rejected | expired | cancelled|notification runner reads
LC5|approved|all required approvals received; ready for to-perform queue|applied | (no other transitions)|change-set executor runner reads
LC6|applied|executor successfully applied all field changes|terminal (only rolled back via new change_set)|none
LC7|rejected|required approver rejected per rule's rejection semantics|terminal|none
LC8|expired|submission deadline passed without sufficient approvals|terminal|none
LC9|cancelled|submitter or sufficient authority withdrew|terminal|none
LC10|failed|bulk apply chunk failed; rolled back|terminal; finding filed|none
LC11|applying|substate during bulk apply|applied | failed|specialized executor

# stakeholder_routing(step|action|source|result)
SR1|enumerate field changes|read change_set_field_change rows|change_set+field_changes|list of (entity_type+entity_id+field) tuples
SR2|walk ownership bridges|service_ownership+machine_ownership+k8s_cluster_ownership+cloud_resource_ownership for each touched entity|bridge tables|ops_user_role rows responsible
SR3|walk stakeholder bridges|service_stakeholder+other stakeholder bridges|bridge tables|additional non-ownership roles
SR4|evaluate approval rules|policy rows of type approval_rule against entity+namespace+fields+metadata (data classification+security zone+compliance scope)|approval_rule policies|requirements per matching rule
SR5|compute requirements|one or more change_set_approval_required rows specifying group+approver_count_required|computed at submit|written to OpsDB
SR6|track fulfillment|each requirement independent; all must be fulfilled|change_set_approval_required.fulfilled_count+is_fulfilled|change_set transitions to approved when all is_fulfilled

# stakeholder_examples(scenario|requirements_emitted)
SE1|change touching service X|service owner approval
SE2|change touches _security_zone field or sensitive namespace|security team approval
SE3|entity in compliance_scope_service for active regime|compliance team approval
SE4|change touches _schema_* tables|schema steward approval
SE5|entity in production namespace + change to runtime field|production operations approval

# validation(id|type|checks|failure_behavior|data_source)
AV1|Schema Validation|every field change matches entity's schema; field exists+type matches+required fields present on creates|blocks|registered schemas via _schema_entity_type+_schema_field
AV2|Bound Validation|numeric ranges within declared bounds+enum membership+FK existence+simple anchored patterns|blocks|OPSDB-7 declarative constraints in schema
AV3|Semantic Validation|cross-field invariants (min<=max+status implies dependent field set)|blocks|entity-type metadata declarations
AV4|Policy Validation|change does not violate active policies (regulated_pci entity needs compliance approver+production from authorized actor)|blocks fail-closed OR warns with explicit ack|policy rows
AV5|Lint Validation|org style+naming+required metadata populated+descriptions present|warnings allow proceed; errors block|org style guide as data
AV6|Dependency Check|service_connection walks verify change does not break downstream contracts|blocks unless dependents tolerant or modified in same change_set|service_connection rows

# auto_approval(category|gating|examples)
AA1|Direct Write Path|no change_set; just authenticated write|cached observation+evidence_record+runner_job_output_var via write_observation
AA2|Auto-Approved|change_set transitions through validating→pending_approval→approved without human|drift corrections in non-prod+routine credential rotations+low-risk reconciler outputs in scope+minor patch upgrades
AA3|Approval Required|routes to human approvers per matching rules|production database changes+security policy+compliance scope+schema changes+high-severity alert config
AA4|Per Target Per Runner|same runner has different gating for different targets via policy|runner auto-approves staging drift+approval-required production drift+refuses compliance-restricted

# emergency(aspect|rule)
EM1|Authority|caller has emergency authority per policy (on-call elevated rights+designated emergency response role)
EM2|Approval|reduced approvals (often single approver+sometimes self-approved)
EM3|Flag|change_set.is_emergency=true+change_set_emergency_review row in pending status
EM4|Review Window|72 hours default (organizationally configurable in policy data)
EM5|Overdue|verifier runner files emergency_review_overdue finding+escalates via notification every 24 hours
EM6|No Auto Rollback|emergency change might be load-bearing; reverting without review creates worse problem
EM7|Audit|always recorded as emergency; queryable; non-negotiable that all emergency changes reviewed eventually

# bulk(aspect|behavior)
BU1|Validation|chunked at default 1000 field changes; interim feedback per chunk|early bad-submission feedback without waiting for full validation
BU2|Coherence|change_set remains one unit; either all chunks validate and transition together or fail together|atomicity at change_set boundary not chunk
BU3|Apply Phase|chunked apply; change_set in applying substate until all chunks complete|partial application never visible externally
BU4|Failure Recovery|if any chunk fails during apply: roll completed chunks via version sibling rows+transition to failed+file finding|all-or-nothing
BU5|Approval Variation|bulk may require one approval for bundle rather than per-entity per change_management_rule|policy-driven
BU6|Common Use Cases|fleet-wide credential rotation+policy rollouts+schema-coordinated migrations|bulk amortizes approval overhead

# report_keys_schema(entity|fields|versioned|purpose)
RKS1|runner_report_key|runner_spec_id+report_target_table+report_key+report_key_data_json+is_active|yes (versioning sibling)|gates runner's writable surface against 5 specific tables
RKS2|runner_report_key_version|standard versioning sibling fields|self|change_set-managed evolution of declarations
# target_tables: observation_cache_metric|observation_cache_state|observation_cache_config|runner_job_output_var|evidence_record
# scope: declarations at runner_spec level not runner_machine level (all instances share same surface)

# report_keys_flow(step|check|outcome)
RKF1|1|API receives runner write to one of five target tables|standard auth runs first
RKF2|2|API looks up runner_report_key rows for the runner's spec + target table|finds declared keys
RKF3|3|API checks submitted key against declared set|undeclared → reject with undeclared_report_key error+full audit
RKF4|4|API validates submitted value against report_key_data_json constraints (numeric range+enum+structural)|invalid → reject with invalid_report_key_value error+detail
RKF5|5|API performs write+records audit+responds|standard write completion

# report_keys_value(category|effect)
RKV1|Prevents|misconfigured/compromised puller writing arbitrary keys outside declared surface
RKV2|Prevents|evidence-runner emitting evidence types outside its declared scope
RKV3|Prevents|reconciler emitting unrelated coordination output keys
RKV4|Strengthens|audit trail: every observation/evidence/coord-output traceable to declared authorization
RKV5|Strengthens|investigation: "where did this data come from" returns runner+declaration+change_set
RKV6|Strengthens|compliance provenance: every evidence_record traceable to runner+declaration via standard joins
RKV7|Fails Closed|declared scope IS the writable surface; legitimacy is declarative not implicit

# audit_log(aspect|content)
AU1|Writer|API only; no direct DB writes for any role
AU2|Append Only|DDL grants no UPDATE/DELETE on audit_log_entry for any role including substrate operators
AU3|Crypto Chain|optional _audit_chain_hash; each entry hashes own contents + prior entry hash; tamper detected by chain break
AU4|Retention|policy-driven; compliance regimes typically 7+ years; reaper does not touch without explicit retention granting
AU5|Audit of Audit|deletions of audit rows recorded in separate audit-of-audit table with at-least-as-long retention floor
AU6|Query Access|auditor role: read access to audit_log_entry+version siblings+change-mgmt records+evidence+policy data
AU7|Query Patterns|every change touching service X in Q3+every authz denial for runner+every emergency change+every approval by user+every write to entity with caller and value summary
AU8|Anomaly Handling|partial-write/invalid-timestamp/malformed entry → mark in separate audit_log_anomaly table; never modify original row
AU9|Sole Strict Append-Only|other tables look append-only but actually permit updates/deletes (cache overwrites+retention reapers); audit log uniquely strict

# audit_fields(field|content)
AUF1|Operation Identity|API endpoint called+method (read|write|get|search|submit)+action_type structured operation class
AUF2|Caller Identity|acting_ops_user_id (humans)+acting_service_account_id (runners)+both populated for web-mediated
AUF3|Target Identity|target_entity_type+target_entity_id; multi-target ops use bridge tables for per-target detail
AUF4|Operation Detail|request_data_summary+response_data_summary; structured; full values stored elsewhere (version siblings+cache tables)
AUF5|Result|success|validation_failed|authorization_denied|rate_limited|internal_error
AUF6|Context|client IP+user agent+request ID for correlation+change_set_id where applicable
AUF7|Timestamp|API-supplied high-precision monotonic; client-supplied recorded in summary not used for acted_time
AUF8|Optional|_audit_chain_hash for tamper-evidence regimes

# tamper_evidence(rule|content)
TE1|Mechanism|each audit_log_entry includes hash covering entry contents + prior entry's hash
TE2|Chain Property|modification of any historical entry breaks chain at that point and all subsequent entries
TE3|Verification|tooling reads chain forward+recomputes each hash+detects breaks
TE4|Break Treatment|detected break is itself a finding (tampered with OR substrate fault corrupted chain)
TE5|Opt-In|per regime requirements; without chaining audit log still has attribution+append-only history
TE6|Cost|hash computation per write; query cost unchanged

# boundaries(id|not_a|why_not|belongs_in)
B1|Orchestrator|API does not invoke runners; would compromise passive-substrate commitment|runners poll+react+coordinate through OpsDB rows
B2|Notification System|adding would couple to delivery infrastructure that evolves at different pace|notification runner reads state transitions+dispatches via configured channels
B3|Change-Set Applier|adding would couple API to executor timing|change-set executor runner polls approved-not-yet-applied+applies via API write ops
B4|Workflow Engine|change_set lifecycle is the only workflow API enforces|other workflows (incident response+deployment+capacity+escalation) coordinate through OpsDB rows
B5|Code Distribution Path|runner code+packages+container images+helm charts not in OpsDB|CI/CD+container registries+package repositories with own tooling
B6|Secrets Store|API resolves auth credentials from secret backends; never stores values|vault+equivalents; OpsDB holds pointers only
B7|Status Page or Public-Facing API|serves humans+automation+auditors operationally inside org|external systems consume via curated runner-mediated paths
B8|Search Engine|structured filter+joins+predicates over schema; no free-text+full-document indexing+semantic+vector|wiki+documentation systems with own query interfaces
B9|Dashboard System|serves reads dashboards consume; does not build/render/maintain UI state|dashboard systems sit on top with own tooling

# worked_example(step|action|result)
WE1|Initial declaration|change_set proposed by alice@org adding 3 runner_report_key rows for prometheus_host_metrics_puller_v1: host_cpu_seconds_total (float 0-1e12)+host_memory_bytes_total (int 0-1e15)+host_disk_bytes_total (int 0-1e18)|approved by security team per approval rule for runner declarations; applied
WE2|Normal operation|puller submits writes for declared keys|pass verification; recorded in observation_cache_metric
WE3|Misconfiguration|puller code modified to also emit database_credentials_active|API rejects each write with undeclared_report_key; full audit recorded with runner identity+submitted key
WE4|Detection|operator queries recent rejections|finds misconfiguration via audit log
WE5|Resolution|operator decides if new key legitimate; if yes change_set proposes adding declaration+security review approves/rejects per policy; if no fix runner code+file finding for over-reach|system handled correctly: unauthorized write rejected+trail visible+response is normal change_set discussion+no data leaked into observation_cache_metric

# relationships(from|rel|to)
A1|enables|A8
A2|enables|stable-API-across-storage-engines
A3|delegates|AD1
A3|delegates|AD2
A3|owns|AD3
A4|enables|passive-substrate-commitment
A4|implies|notification-runner-pattern
A5|implies|notification-runner-pattern
A6|implies|change-set-executor-runner
A8|implements|all-operations
A9|enables|change-without-redeployment
A11|prevents|DoS-via-regex
A12|enables|integrity-of-history
AO1|read_class|five-layer-auth
AO4|implements|named-join-paths-traversal
AO5|implements|substrate-walking-for-runners
AO5|enables|RW1+RW2+RW3+RW4+RW5 from OPSDB-5
AO8|gated_by|RKS1
AO9|writes|change_set+change_set_field_change+change_set_approval_required
AO9|triggers|stakeholder-routing
AO10|may_transition|change_set status to approved
AO11|may_halt|change_set
AO13|requires|emergency authority
AO13|creates|change_set_emergency_review pending
AO14|chunked_validation|true
AO14|chunked_apply|true
AO15|verifies|change_set status=approved+not yet applied
AO15|writes|entity row update+version sibling
AO16|verifies|all field changes applied successfully
AG1|prereq_of|AG2
AG2|prereq_of|AG3
AG3|prereq_of|AG4
AG4|prereq_of|AG5
AG5|prereq_of|AG6
AG6|prereq_of|AG7
AG7|prereq_of|AG8
AG8|prereq_of|AG9
AG9|prereq_of|AG10
AL1|composes_with|AL2
AL2|composes_with|AL3
AL3|composes_with|AL4
AL4|composes_with|AL5
AL5|may_inject|additional approval requirements
AL_ALL|first_denial_halts|true
AF1|attributes|acting_ops_user_id
AF2|attributes|acting_service_account_id
AF3|attributes|both (acting_service_account_id + acting_ops_user_id)
AS1|composes|AS2+AS3+AS4+AS5+AS6+AS7+AS8
AS2|enables|stack-walking-via-named-paths
AS6|prevents|API-self-DoS+substrate-resource-exhaustion
AVE1|enables|O(1)-point-in-time-reconstruction
AVE6|interacts_with|AVE7 (sharded retention)
AVE8|never|side-channel
AC2|fails_loud|true
AC2|prevents|silent-overwrites+post-approval-state-drift
LC4|observed_by|notification-runner
LC5|observed_by|change-set-executor-runner
SR4|computes|change_set_approval_required rows
SR6|transitions|change_set to approved when all is_fulfilled
AV1|blocks_on_failure|true
AV2|blocks_on_failure|true
AV2|consumes|OPSDB-7 declarative constraints
AV3|blocks_on_failure|true
AV4|may_warn_or_block|true
AV5|warnings_allow_proceed|errors_block
AV6|may_block|true
AA1|no_change_set|true
AA2|no_human_intervention|true
AA3|requires_human|true
EM3|always_recorded|true
EM4|configurable|in policy data
EM6|enforces|never auto-rollback emergency changes
BU2|atomicity_at|change_set boundary
BU4|all_or_nothing|via version sibling rollback
RKS1|change_managed|true
RKS1|gates|AO8
RKF3|fails_closed|true
RKF4|fails_closed|true
RKV7|implements|fail-closed on writable surface
AU2|strictest_in_schema|true
AU3|opt_in|per regime
AU5|recursive_audit|bottoms out at compliance horizon
AU9|uniquely_strict|true
TE2|enforces|tamper detection
TE5|opt_in|true
B1|prevents|coupling to delivery+timing infrastructure
B2|enables|notification runner pattern
B3|enables|change-set executor pattern
B4|preserves|API simplicity
B6|preserves|secret backend boundary
B9|preserves|API simplicity
WE5|demonstrates|RKV7 fail-closed property
WE5|demonstrates|RKV4 audit trail strengthening

# section_index(section|title|ids)
1|Introduction|A1,A2,A3,A4,A5,A6,A7
2|Conventions|inherited from prior series; AS1 forbids regex per A11
3|API Surface|AO1-AO16,AG1-AG10
4|Search API|AS1-AS9,AP1-AP8,AV1-AV3
5|Versioning Machinery|AVE1-AVE8,AC1-AC4
6|Authentication and Authorization|AF1-AF3,AD1-AD4,AL1-AL5
7|Change Management as Gate Function|LC1-LC11,SR1-SR6,SE1-SE5,AV1-AV6,AA1-AA4,EM1-EM7,BU1-BU6
8|Runner Report Keys|RKS1,RKS2,RKF1-RKF5,RKV1-RKV7,WE1-WE5
9|Audit Logging|AU1-AU9,AUF1-AUF8,TE1-TE6
10|What This API Does Not Do|B1-B9
11|Closing|A1-A13 restated structurally

# decode_legend
operation_classes: read|write-direct|write-change-set|change-mgmt-action|admin
gate_step_order: AG1→AG2→AG3→AG4→AG5→AG6→AG7→AG8→AG9→AG10
auth_layer_composition: AL1 AND AL2 AND AL3 AND AL4 AND AL5; first denial halts
sot_values: opsdb|authority|external|self
attribution_columns: acting_ops_user_id|acting_service_account_id|both
versioned_classifications: change-managed (versioned)|observation-only|append-only|computed-by-tooling
predicate_restrictions: no regex; declarative bounds only; bounded composition depth
report_key_target_tables: observation_cache_metric|observation_cache_state|observation_cache_config|runner_job_output_var|evidence_record
rel_types: enables|prevents|implements|delegates|owns|implies|reads_from|writes_to|gated_by|composes|composes_with|may_inject|attributes|prereq_of|consumes|interacts_with|never|fails_loud|fails_closed|verifies|writes|triggers|may_transition|may_halt|requires|creates|chunked_validation|chunked_apply|change_managed|gates|enforces|opt_in|recursive_audit|uniquely_strict|first_denial_halts|all_or_nothing|atomicity_at|always_recorded|configurable|preserves|demonstrates|strictest_in_schema|read_class|may_warn_or_block|warnings_allow_proceed|errors_block|blocks_on_failure|may_block|attributes
boundary_pattern: each "not a X" boundary preserves API simplicity by refusing scope expansion
