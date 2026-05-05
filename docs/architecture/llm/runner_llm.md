# OPSDB RUNNER DESIGN — LLM-COMPACT FORM
# Format: pipe-delimited tables, ID refs.
# Read order: pattern → lifecycle → kinds → kind-examples → libraries → disciplines → idempotency → gating → gating-per-target → stack-walking → gitops-cast → gitops-trail → gitops-variations → scenarios → anti-patterns → new-runner → pseudocode → walk-queries → tool-mapping → relationships → sections

# pattern(id|rule|rationale)
R1|get from OpsDB → act in world → set to OpsDB|three-phase shape every runner shares
R2|OpsDB is runner's only stable interface|other interfaces (cloud APIs|K8s API|SSH|vault) are transient and change; OpsDB schema versioned and absorbs change additively
R3|persistent inputs and outputs are exclusively OpsDB rows|world is read-from and acted-upon but only OpsDB persists
R4|get phase produces no side effects|clean phase separation; tools mixing phases hard to reason about
R5|act phase produces side effects|library-mediated; bounded; idempotent or with uniqueness keys
R6|set phase records what happened|every write through API; every write produces audit_log_entry
R7|no runner directs another runner|coordination implicit through shared data; no orchestrator
R8|coordination through shared substrate|runner B reads what runner A wrote on B's next cycle; OpsDB is rendezvous
R9|crashed runner blocks no other runner|next cycle picks up where it left off
R10|runner small enough to be fully knowable|200-500 lines of runner-specific logic; libs do heavy lifting
R11|runner is data-defined|configuration in runner_spec_version.runner_data_json; changing what runner does = changing data
R12|runner authority is data not code|capabilities scoped through policy data; no runner has hardcoded admin rights
R13|in-memory state for one cycle only|persistent state in OpsDB; long-running runners check OpsDB each cycle
R14|every API write produces audit|no out-of-band path; no bypass for any reason except documented break-glass

# lifecycle_phases(id|phase|what_happens|invariant)
LP1|Invocation|external trigger starts runner instance: scheduler|event watch|reconciler interval|change-set executor|runner does not invoke itself
LP2|Get|read runner_spec_version + needed data through API|each row carries freshness+version metadata; no side effects
LP3|Internal Computation|compute planned action set; diff|decisions|target selection|no side effects; output is concrete inspectable plan
LP4|Dry-Run Output|render planned action set if dry-run mode enabled|exits without executing when dry_run=true; deterministic-in-non-commit-mode
LP5|Act|execute planned actions through shared libs with retry/backoff/idempotency|each action bounded per runner_data_json
LP6|Set|write runner_job + output_vars + per-target rows + evidence + observation + change_set as appropriate|every write through API
LP7|Recorded Outcome|audit_log_entry produced for every write; runner exits or continues to next cycle|full trail queryable

# runner_kinds(id|name|purpose|reads|writes|idempotency|gating|trigger)
RK1|Puller|read authority transform to schema write to cache|runner_spec_version+authority+prometheus_config|observation_cache_*+runner_job|natural; re-run writes current value|direct write|scheduled
RK2|Reconciler|read desired+observed compute diff act|entity rows desired+observation_cache_* observed+policies|runner_job+target bridges+possibly change_set|converged state→no action; level-triggered|varies per target (auto or approval)|scheduled or long-running
RK3|Verifier|check scheduled work happened or state correct|schedule+target+prior evidence_record|evidence_record+target bridges+runner_job|each cycle produces new record; never modifies prior|direct write|scheduled
RK4|Scheduler|enforce runner_schedule on target substrate|runner_schedule+schedule+target runner_spec+substrate info|runner_job+side effects (cron+systemd+CronJob)|reconciles desired vs observed schedule state|auto-approved (routine) or approval (high-stakes)|long-running
RK5|Reactor|edge-triggered response to events|event+runner_spec_version|runner_job+downstream rows (alert_fire+change_set+observation)|library provides idempotency keys for replays|varies; mostly observation; some change_set|event
RK6|Drift Detector|reconciler-shape that proposes not acts|same as reconciler|change_set+runner_job|already-proposed→no re-propose until drift changes|through change_set discipline|scheduled
RK7|Change-Set Executor|read approved change_set apply field changes|change_set status=approved+change_set_field_change|entity rows+*_version rows+change_set status update+runner_job|applied status→no re-apply|direct write after approval|triggered by approved change_set
RK8|Reaper|apply retention policies trim old rows|retention_policy+target tables|deletes from cache/short-retention+is_active=false on entities+runner_job|naturally idempotent; nothing past horizon→nothing|direct on cache; soft-delete follows policy|scheduled
RK9|Bootstrapper|set up new machines from minimal state|templated config+host_group+package_version+(later)OpsDB|machine row+observation+runner_job|bootstrapped state recognized→exits or reconciles partial|auto-approved (or approval if sensitive)|new-host trigger
RK10|Failover Handler|detect primary failure perform failover verify update OpsDB|observation_cache_state+failover policy+topology|change_set+evidence_record+runner_job+side effects|already-failed-over→no re-action; reads current state|emergency-change or fast-track approval|event or scheduled

# runner_kind_examples(kind|example)
RK1|Prometheus metric scraper writing summary rows
RK1|Kubernetes API state puller writing pod status
RK1|Cloud control plane scraper writing resource state
RK1|Vault metadata puller writing secret-existence rows (never values)
RK1|Identity provider sync writing group membership
RK2|Kubernetes manifest reconciler comparing OpsDB workload spec vs cluster spec
RK2|Configuration drift corrector for hosts in host_group
RK2|Certificate renewer triggered by approaching expiration
RK2|Capacity adjuster scaling replicas based on metrics
RK2|DNS zone reconciler ensuring records match OpsDB-defined zone
RK3|Backup verifier confirming today's backup completed and is restorable
RK3|Certificate validity scanner checking expiration dates
RK3|Compliance scanner evaluating policies against entity configurations
RK3|Credential rotation verifier confirming credentials rotated on schedule
RK3|Access review confirmer recording quarterly access reviews
RK3|Manual operation verifier (tape rotation+vendor review+keycard revocation)
RK4|Cron entry deployer ensuring host-level cron matches runner schedules
RK4|Kubernetes CronJob reconciler ensuring cluster CronJobs match
RK4|Systemd timer manager
RK5|Webhook receiver writing incoming events to OpsDB
RK5|Kubernetes event watcher writing alert_fire rows
RK5|Ticketing system change handler updating entity metadata on transitions
RK6|Kubernetes manifest drift detector
RK6|Cloud resource drift detector (Terraform-state-style without Terraform)
RK6|Configuration drift detector for sensitive systems where auto-correct undesired
RK7|Single change-set executor reading any approved change set and applying
RK7|Specialized executors per entity class needing world-side action plus OpsDB updates
RK8|Cache reaper trimming observation_cache_metric past retention
RK8|Job log reaper trimming runner_job rows past retention
RK8|Tombstone reaper removing soft-deleted versioning siblings past retention
RK9|Cloud-init script pulling templated config applying registering VM
RK9|Kubernetes node bootstrap joining cluster registering in OpsDB
RK9|PXE boot finalizer for bare-metal hosts
RK10|Database primary-replica failover handler
RK10|Load-balancer backend failover
RK10|Cluster control-plane failover

# libraries(id|name|purpose|notes)
RL1|OpsDB API Client|authenticated retry-aware validation-handling client; ONLY path to OpsDB|every runner uses; no other path
RL2|Kubernetes Operations|apply manifests query state watch streams helm operations|wraps K8s client with standardized retry+error mapping
RL3|Cloud Operations|provider-agnostic interface with per-provider backends|AWS+GCP+Azure+others behind common ops; payloads as cloud_data_json
RL4|Secret Access|pull from vault/sops/KMS at runtime in-memory only|never logged|never written to OpsDB|never persisted
RL5|Logging and Metrics|uniform format with correlation IDs tying log lines to runner_job|mandatory; runners bypassing produce inconsistent population
RL6|Retry Backoff Idempotency Markers|token-bucket budgets+exponential backoff+jitter+uniqueness-key generation|exposed as decorators/wrappers
RL7|Notifications|email+chat+page through configured channels|channels+routing in OpsDB; library reads and dispatches
RL8|SSH Remote Command|for legacy substrate without richer API|used sparingly; preferred only when no API alternative
RL9|Git Operations|clone+commit+push+tag+pull-request|used by GitOps integration runners
RL10|OpsDB API Client (mandatory enforcement)|same as RL1|the framework's enforcement of one-way-to-do-each-thing

# disciplines(id|name|statement|enforcement|failure_if_violated)
RD1|Idempotency|every runner action safely retryable; same inputs→same end state|schema fields like change_set_field_change.applied_status; uniqueness markers; library handles mechanics|partial failure produces inconsistent state; re-running creates duplicates
RD2|Level-Triggered Over Edge-Triggered|read current state act on it not react to event streams as only source|reconcilers re-evaluate on every cycle; reactors paired with reconciler backstops|missed events become missed actions; system silently diverges
RD3|Bound Everything|every runner has explicit retry/time/queue/memory/scope bounds in runner_data_json|runner_job records what bound was hit if execution stopped early|unbounded runners crash unpredictably; impossible to reason about resource use

# idempotency_levels(level|meaning|mechanism)
IL1|Runner Level|re-running same inputs+state produces no additional side effects beyond first converging run|reconciler reads current state each cycle; converged state→no action
IL2|Action Level|each shared-lib call idempotent or carries uniqueness keys|library handles mechanics; runner provides keys where needed
IL3|Non-Idempotent Operations|some ops not naturally idempotent (email|payment|one-shot)|flagged in runner_spec.runner_data_json as requiring special handling; uniqueness keys applied where possible

# gating_modes(id|mode|when_used|examples|approval)
RG1|Direct Write|observation-only data; never goes through change management|pullers+verifiers+reapers+most reactors|none; audit only
RG2|Auto-Approved Change Set|change set recorded+audited+policy auto-approves without human|drift correctors+cert renewers+credential rotations+minor patch upgrades|policy evaluates rule and approves
RG3|Approval-Required Change Set|change set routes to human approvers|production DB changes+security policy+compliance scope+schema changes+high-severity alert config|human evaluates and approves

# gating_per_target(scenario|behavior)
GT1|same runner code different targets|gating mode is per-runner-spec AND per-target via policy
GT2|drift-correction in staging|auto-approve (low stakes fast iteration)
GT3|drift-correction in production low-risk fields|auto-approve (timeouts+replica counts within bounds)
GT4|drift-correction in production outside low-risk set|approval-required
GT5|drift-correction on compliance-restricted entities|refuse to act; file finding for compliance review

# stack_walking(id|decision|walk|enables)
RW1|Decommission Awareness|service→host_group→machine→megavisor_instance walking parent chain checking decommissioned_time or pending change_sets|reconciler skips action and logs why
RW2|Failure Domain Analysis|primary+replica each walked to ancestry; compare rack/DC|failover refuses if shared (offers no resilience)
RW3|Capacity Awareness|K8s nodes→underlying machines→hardware sets|sees actual hardware capacity not just cluster reports
RW4|Dependency-Aware Change Validation|service_connection rows from targeted service to dependents|validator files warning or rejects change breaking downstream contract
RW5|Locality-Aware Deployment|service→preferred locations→available capacity per location|schedules pods/VMs where capacity exists and locality matches policy

# gitops_cast(runner_id|name|reads|writes|role)
RC1|Helm Change-Set Executor|approved change_set targeting k8s_helm_release_version+configuration_variable|entity rows updated+runner_job_output_var (new version ready)|closes change-mgmt loop on data
RC2|Helm Git Exporter|output var from RC1+config vars (secrets resolved at render via vault)|git commit with structured message linking change_set ID+tag with change_set ID+output var with commit hash|exports OpsDB intent to git
RC3|Argo CD or Flux|git repo|cluster state|EXTERNAL not OpsDB runner; OpsDB not involved this step
RC4|Kubernetes Deploy Watcher|cluster watch API|observation_cache_state pod transitions+output vars (rollout_succeeded+pod_count+image_digest_deployed+errors)|records observed cluster outcome
RC5|Image Digest Verifier|change_set intended digests+RC4 output vars actual deployed digests|evidence_record(deployment_verification)+compliance_finding if mismatch|verifies intent matches reality
RC6|Drift Detector|OpsDB-known helm release version+cluster-observed helm release version|finding or auto-correct per policy|backstop for ongoing alignment

# gitops_trail(step|from|to|linkage)
GT_T1|change_set|change_set_field_change|change_set_id FK
GT_T2|change_set_field_change|runner_job (helm executor)|change_set applied_by_runner_job_id
GT_T3|runner_job (helm executor)|runner_job_output_var (new version ready)|runner_job_id FK
GT_T4|runner_job (executor)|runner_job (git exporter)|input_references_change_set_id link
GT_T5|runner_job (git exporter)|runner_job_output_var (commit_hash)|runner_job_id FK
GT_T6|runner_job (git exporter)|runner_job (deploy watcher)|references_helm_release_version_id from change_set_field_change.after_value
GT_T7|runner_job (deploy watcher)|runner_job_output_var (image_digest_deployed)|runner_job_id FK
GT_T8|runner_job (deploy watcher)|observation_cache_state (pod transitions)|_puller_runner_job_id
GT_T9|runner_job chain|evidence_record (digest verification)|references_change_set_id
GT_T10|every API write throughout|audit_log_entry|target_entity_type+target_entity_id

# gitops_disciplines(domain|sot|why)
GD1|intent|OpsDB (change_set+helm release version)|describes what was wanted
GD2|what's-checked-in-to-be-applied|git|Argo CD reconciles git-to-cluster
GD3|live state|cluster|pulled into OpsDB cache via RC4
GD4|the trail|OpsDB|all above tied together by IDs

# gitops_variations(id|variation|differs|stays)
GV1|Fixed-Version Deployment|change set pins exact image digests; RC5 verifies deployed digests match|same cast; pinning is data not code
GV2|Tag-Tracking Deployment|runner reads artifact registry's current tag and submits change_set with resolved digest|change set then flows through cast normally
GV3|Promotion (staging→production)|runner reads staging's deployed digest from cache and submits production change_set with that digest|approval per production policy then cast
GV4|Rollback|change_set restoring prior helm release version's values|cast processes like any other change_set; deployment goes back

# scenarios(id|name|standard|opsdb|key_delta)
RS1|Alert Response|PagerDuty pages→Slack→Grafana→kubectl→guess runbook→follow→update channel→post-incident wiki update (maybe)|alert_fire row→escalation runner reads service_escalation_path+on_call_assignment+sends page with resolved context (runbook+dashboard+recent change_sets+evidence+dependencies)|standard scatters across PagerDuty+Grafana+kubectl+wiki+Slack with no central record; OpsDB has one queryable trail
RS2|Deployment|developer PR→CI→Argo CD/kubectl apply→hope it worked→Datadog check (maybe)|change_set submitted+validated+approval routed+helm git exporter+Argo CD applies+deploy watcher+digest verifier+evidence record|standard hopes it worked; OpsDB has full §10 trail queryable
RS3|Certificate Renewal|cron somewhere runs cert-manager/ACME|hope it worked|alert at 7-days-from-expiry (maybe)|certificate_expiration_schedule+renewer runner+evidence_record+drift detector confirms+compliance_finding if fails|standard renewal is fire-and-forget; OpsDB tracks every renewal cycle with evidence
RS4|Compliance Evidence|auditor arrives→team scrambles→screenshots+tickets+Slack threads→binder produced→auditor accepts on faith|compliance_audit_schedule+verifier runners on continuous cadence+evidence_record accumulates+compliance_finding tracks gaps+auditor reads OpsDB directly|standard is reactive scramble; OpsDB is continuous queryable property
RS5|Drift Correction|nobody knows until incident or manual audit months later|drift detector on schedule finds within one cycle+auto-correct or file finding per policy|standard discovers drift after damage; OpsDB never accumulates past detection cycle

# pattern_consequence(scenario|standard_artifact|opsdb_artifact)
PC1|RS1|scattered evidence|one queryable join
PC2|RS2|hope-it-worked|structured trail with attribution
PC3|RS3|fire-and-forget|every cycle evidence record
PC4|RS4|periodic scramble|continuous evidence
PC5|RS5|delayed discovery|same-cycle detection

# anti_patterns(id|name|description|why_bad)
RA1|Orchestrating Other Runners|runner invokes other runners directly|creates orchestrator failure mode design refuses; coordination must be via OpsDB rows
RA2|State Outside OpsDB|runner persists state in local files OpsDB does not reflect|other runners and queries cannot see; "what I did last time" must be queryable
RA3|Reinventing Shared Libraries|runner implements own retry/K8s client/logging|two runners diverge in failure modes; produces inconsistency suite exists to prevent
RA4|Acting on Stale Cache Without Freshness Check|reads observation_cache_* without checking _observed_time|hour-old cache acted on as if minute-fresh; freshness check is part of get phase
RA5|Logic in Template Variables|template-time computation embedded in templates|complex resolution belongs in upstream runners producing concrete values; templates substitute values only
RA6|Skipping Audit Trail|bypasses API to avoid audit|no out-of-band path; violates entire design
RA7|Long-Running Runners with In-Memory State Across Cycles|accumulates state in memory between cycles|crashes lose state; persistent state must live in OpsDB
RA8|Multi-Domain Runners|deployment runner that also does monitoring+alerting+capacity|too big; split into one-thing runners
RA9|Privileged Authority Not Expressed as Policy|runner has admin rights hardcoded into runtime|authority must be data the API consults; policy rows
RA10|Treating OpsDB as Queue|polls OpsDB as message queue treating every row as job|OpsDB is database; pattern is read-state→decide→act; change-set executor is special case
RA11|Bypassing Change Management for Speed|routes around change-mgmt for non-emergency reasons|emergency has defined break-glass path; faster is not better than auditable

# new_runner_design(step|action|decision)
NR1|Identify the inputs|what OpsDB rows does runner read+what authorities does it consult|shapes get phase
NR2|Identify the outputs|what OpsDB rows does runner write+what side effects does it produce|shapes set phase and gating mode
NR3|Choose the gating|direct write OR auto-approved change set OR approval-required change set|depends on what writes and stakes
NR4|Choose the trigger|scheduled OR event-triggered OR invoked-by-other-runner-data OR long-running|determines invocation pattern
NR5|Specify the bounds|retry budget+execution time+scope per cycle+queue depth+memory|recorded in runner_data_json
NR6|Define idempotency|what does same-end-state mean for this runner+what uniqueness keys|document special case if cannot achieve
NR7|Write the runner spec|add runner_spec row with appropriate runner_spec_type+runner_data_json schema|register JSON schema with API for validation
NR8|Build the runner|small+single-purpose+using shared libs|test get-act-set; test idempotency by running twice
NR9|Deploy through change management|code in registry+spec in OpsDB+deployment is change_set creating runner_machine row|operational once change_set commits

# pseudocode_skeletons(kind|skeleton)
RP1_puller|spec=read(runner_spec_version);auth=read(authority,spec.authority_id);keys=spec.keys;job=create(runner_job,started);for k in keys:v=lib.authority_query(auth,k);write(observation_cache_metric,authority_id+metric_key+metric_value+_observed_time+_puller_runner_job_id=job);update(runner_job,finished,succeeded)
RP2_reconciler|spec=read(runner_spec_version);desired=read_many(spec.desired_query);observed=read_many(spec.observed_query);job=create(runner_job);diff=compute_diff(desired,observed);actions=decide(diff,spec.policies);if dry_run:log(actions);exit;for a in actions:if a.requires_change_set:create(change_set,proposed_by_runner_job_id=job);else:r=lib.execute(a);create(runner_job_target_*,job+target+per_target_status=r.status);update(runner_job,finished,succeeded)
RP3_verifier|spec=read(runner_spec_version);target=read(spec.target_table,spec.target_id);job=create(runner_job);result=check_condition(target,spec.condition);ev=create(evidence_record,type=spec.evidence_type+data=result.detail+produced_by_runner_job_id=job+status=passed-if-ok-else-failed+observed_time=now);create(evidence_record_*_target,ev+target+per_target_status);if not ok:create(compliance_finding,severity=spec.severity_on_fail+description=result.detail+filed_by_evidence_record_id=ev);update(runner_job,finished)
RP4_change_set_executor|spec=read(runner_spec_version);approved=read_many(change_set,status=approved,not_yet_applied);for cs in approved:job=create(runner_job);fcs=read_many(change_set_field_change,change_set_id=cs.id);ok=true;for fc in sorted(fcs,by=apply_order):try:update_field(fc.entity_type,fc.entity_id,fc.field,fc.after);update(change_set_field_change,id=fc.id,applied);except:update(change_set_field_change,failed,error_text);ok=false;break;if ok:update(change_set,id=cs.id,applied,applied_time);else:update(change_set,failed);update(runner_job,finished)
RP5_drift_detector|spec=read(runner_spec_version);desired=read_many(spec.desired_query);observed=read_many(spec.observed_query);job=create(runner_job);drifts=compute_diff(desired,observed);for d in drifts:existing=read_one(change_set,recent_for_this_spec+targets=d.entity+status_in=[draft,pending_approval,approved]);if existing:continue;create(change_set,proposed_by_runner_job_id=job,reason=Drift+d.summary);create(change_set_field_change,...);update(runner_job,finished,succeeded)

# stack_walk_queries(id|question|strategy|result)
RQ1|Is this service's underlying VM being decommissioned?|recursive CTE walking from service→host_group_machine→machine→megavisor_instance.parent_megavisor_instance_id chain; check decommissioned_time IS NOT NULL OR pending change_set_field_change targeting decommissioned_time on any walked instance|boolean is_decommissioning
RQ2|Do these two machines share a failure domain (rack)?|for each machine: walk hardware_set_instance.location_id up location.parent_id chain to find location_type=rack; compare rack IDs|boolean share_rack
RQ3|What is the full ancestry of this pod?|recursive CTE from k8s_pod.megavisor_instance_id up parent_megavisor_instance_id chain joining megavisor for type and hardware_set_instance/cloud_resource at each level|one row per substrate layer ordered by depth: depth+megavisor_type+external_id+hostname+hardware_location+cloud_type

# tool_mapping(tool|kind_slot|opsdb_extension)
RT1|Argo CD / Flux|external cluster reconciler (sits between git and cluster)|OpsDB-side write of intent (change_set)+OpsDB-side write of observed outcome (deploy watcher)+trail composition
RT2|Crossplane|reconciler for cloud resources via K8s API|OpsDB-side write of cloud_resource state+change_set governance
RT3|Pulumi / Terraform|apply-engine for infrastructure|OpsDB-side change_set submission+applied-state recording
RT4|Salt / Ansible / Puppet|reconciler for hosts|OpsDB-side host_group definition+applied-state recording (sysync model)
RT5|cert-manager|reconciler for certificates in K8s|OpsDB-side certificate inventory+evidence_record production
RT6|external-dns|reconciler for DNS|OpsDB-side DNS zone definition+observed-state recording
RT7|Velero|reconciler for K8s backups|OpsDB-side backup schedule+evidence_record per backup+verification runner
RT8|Prometheus|metric collection authority|OpsDB-side prometheus_config+observation_cache_metric writes by puller
RT9|PagerDuty / Opsgenie|page delivery authority|OpsDB-side alert_fire+escalation runner+on_call_assignment

# relationships(from|rel|to)
R1|implements|R2
R1|implements|R3
R1|implements|R4
R1|implements|R5
R1|implements|R6
R7|enforced_by|R8
R7|enables|R9
RK1|implements|R1
RK2|implements|R1
RK3|implements|R1
RK4|implements|R1
RK5|implements|R1
RK6|implements|R1
RK7|implements|R1
RK8|implements|R1
RK9|implements|R1
RK10|implements|R1
RK1|writes_to|RG1
RK3|writes_to|RG1
RK8|writes_to|RG1
RK4|writes_to|RG2
RK6|writes_to|RG2_or_RG3
RK7|gates|RG1_after_approval
RK10|gates|emergency_or_RG3
RK1|must_satisfy|RD1
RK1|must_satisfy|RD2
RK1|must_satisfy|RD3
RK2|must_satisfy|RD1
RK2|must_satisfy|RD2
RK2|must_satisfy|RD3
RK3|must_satisfy|RD1
RK3|must_satisfy|RD2
RK3|must_satisfy|RD3
RK4|must_satisfy|RD1
RK4|must_satisfy|RD3
RK5|must_satisfy|RD1
RK5|paired_with_backstop|RK2
RK6|must_satisfy|RD1
RK6|must_satisfy|RD2
RK7|must_satisfy|RD1
RK8|must_satisfy|RD1
RK9|must_satisfy|RD1
RK10|must_satisfy|RD1
RK1|uses|RL1
RK2|uses|RL1
RK3|uses|RL1
RK1|uses|RL5
RK1|uses|RL6
RK2|uses|RL2_or_RL3
RK2|uses|RL6
RK4|uses|RL1
RK7|uses|RL1
RC1|instance_of|RK7
RC2|instance_of|RK2_or_specialized
RC4|instance_of|RK1
RC5|instance_of|RK3
RC6|instance_of|RK6
RC2|uses|RL9
RC2|uses|RL4
RC4|uses|RL2
RC1|writes_to|GT_T2
RC2|writes_to|GT_T5
RC4|writes_to|GT_T7
RC4|writes_to|GT_T8
RC5|writes_to|GT_T9
RA1|violates|R7
RA2|violates|R3
RA3|violates|RL10
RA4|violates|LP2
RA5|violates|R11
RA6|violates|R14
RA7|violates|R13
RA8|violates|R10
RA9|violates|R12
RA10|misuses|R1
RA11|violates|R14
RD1|enforced_via|change_set_field_change.applied_status
RD2|enforced_via|reconciler_re_evaluation_each_cycle
RD3|enforced_via|runner_data_json_bounds+runner_job_records_bound_hit
RW1|uses|substrate_hierarchy_walk
RW2|uses|substrate_hierarchy_walk
RW3|uses|substrate_hierarchy_walk
RW4|uses|service_connection_walk
RW5|uses|service_to_locations_walk
RT1|opsdb_aware_extension|OpsDB-side intent+observed+trail
RT4|opsdb_aware_extension|host_group+applied-state recording
RT5|opsdb_aware_extension|certificate inventory+evidence
RS1|standard_uses|PagerDuty+Slack+Grafana+kubectl+wiki
RS1|opsdb_uses|alert_fire+escalation runner+resolved context+runner_job
RS2|opsdb_uses|gitops cast §10
RS3|opsdb_uses|certificate_expiration_schedule+RK2+RK3
RS4|opsdb_uses|compliance_audit_schedule+verifiers+evidence_record+findings
RS5|opsdb_uses|RK6
LP4|enables|deterministic_in_non_commit_mode
LP4|sysync_lineage|generalized_from_sysync
R11|sysync_lineage|generalized_from_sysync
GD1|sot_for|intent
GD2|sot_for|what_will_apply
GD3|sot_for|live_state
GD4|sot_for|trail

# section_index(section|title|ids)
1|Introduction|R1-R14
2|Runner Pattern|R1,R2,R3,R4,R5,R6,R7,R8,R9,R10,R11
3|Runner Lifecycle|LP1,LP2,LP3,LP4,LP5,LP6,LP7
4|Runner Kinds|RK1,RK2,RK3,RK4,RK5,RK6,RK7,RK8,RK9,RK10
5|Shared Library Suite|RL1,RL2,RL3,RL4,RL5,RL6,RL7,RL8,RL9,RL10
6|Coordination Through Shared Substrate|R7,R8,R9
7|Three Load-Bearing Disciplines|RD1,RD2,RD3,IL1,IL2,IL3
8|Change-Management Gating|RG1,RG2,RG3,GT1,GT2,GT3,GT4,GT5,R12
9|Stack-Walking and Dependency-Aware Runners|RW1,RW2,RW3,RW4,RW5
10|GitOps Integration Pattern|RC1,RC2,RC3,RC4,RC5,RC6,GT_T1-GT_T10,GD1,GD2,GD3,GD4,GV1,GV2,GV3,GV4
11|Standard vs OpsDB-Coordinated Practice|RS1,RS2,RS3,RS4,RS5,PC1,PC2,PC3,PC4,PC5
12|Designing New Runners|NR1,NR2,NR3,NR4,NR5,NR6,NR7,NR8,NR9
13|Anti-Patterns|RA1,RA2,RA3,RA4,RA5,RA6,RA7,RA8,RA9,RA10,RA11
A|Runner Kind Reference|RK1-RK10
B|Pseudocode by Kind|RP1_puller,RP2_reconciler,RP3_verifier,RP4_change_set_executor,RP5_drift_detector
C|GitOps Trail Join|GT_T1-GT_T10
D|Tool Mapping|RT1,RT2,RT3,RT4,RT5,RT6,RT7,RT8,RT9
E|Stack-Walking Queries|RQ1,RQ2,RQ3

# decode_legend
runner_kind_columns: id|name|purpose|reads|writes|idempotency|gating|trigger
gating_modes: direct-write|auto-approved-change-set|approval-required-change-set
disciplines: idempotency|level-triggered-over-edge-triggered|bound-everything
phases: get|act|set
trigger_types: scheduled|event|long-running|invoked-by-other-data|new-host|approved-change-set
sot_values: opsdb|authority|external|self
rel_types: implements|uses|writes_to|gates|must_satisfy|paired_with_backstop|instance_of|violates|misuses|enforced_via|enforced_by|enables|sot_for|opsdb_aware_extension|sysync_lineage|standard_uses|opsdb_uses
delta_meaning: scattered evidence vs one queryable join is the consistent shape across all scenarios
