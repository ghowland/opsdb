# OPSDB SHARED LIBRARY SUITE — LLM-COMPACT FORM
# Format: pipe-delimited tables, ID refs.
# Read order: principles → families → libraries → contract → boundary → runner-content → api-client → world-libs → world-ops → coordination → observation → notification → templating → git → versioning → steward → two-sided → policy-validation → fail-closed → audit-composition → failure-modes → refusals → adoption → examples → relationships → sections

# principles(id|principle|rationale)
L1|Contract not implementation|library is contract specification not implementation; multiple implementations of same contract coexist (different languages+transports); runner pins to contract not impl
L2|Suite is one suite|within org one library suite; N language impls of same contract valid; parallel suites in same language forbidden
L3|Library/runner boundary by mechanical test|"would two runners reimplement this?" yes→library; no→runner
L4|Runners stay small because libraries do heavy lifting|~200 lines runner with suite vs ~1500 without; 50 lines glue+150 lines runner-specific decisions
L5|Mandatory libraries enforce single path|API client mandatory+observation libraries mandatory; bypass produces inconsistency suite exists to prevent
L6|Library evolution by accretion|pattern in 1 runner stays in runner; pattern in 3 candidate; pattern in 10 confirmed extraction
L7|Pulling logic out of library back into runners is rare|once library others depend; removal forces reimplementation everywhere
L8|Library is operational realization of one-way-to-do-each-thing|applied to runner-world interaction
L9|Two-sided policy enforcement|API gate (OPSDB-6) at OpsDB write time + library suite at world-side action time; runner declarations input to both
L10|Library refuses fragmentation|team wanting "their own version" solving real problem; absorb into standard library; library_steward holds discipline
L11|Templates deliberately dumb|substitution+inclusion only; logic-needing-templates upstream into runners producing concrete values
L12|Secrets never persisted by library|in memory only during call; logging records path+caller+timestamp+result never value
L13|Library validates inputs at call boundary|rejects malformed before reaching world or OpsDB
L14|Library propagates correlation IDs|runner_job_id root → API call chain → audit_log_entry; query joins via correlation field

# families(id|family|naming_prefix|role)
LF1|API access|opsdb.api|the only path runners use to touch OpsDB; mandatory
LF2|World-side substrate|opsdb.world.*|wrappers per external substrate (kubernetes|cloud|host|registry|secret|identity|monitoring|pointer)
LF3|Coordination and resilience|opsdb.coordination.*|retry|circuit_breaker|hedger|bulkhead|failover; mechanism patterns from OPSDB-9
LF4|Observation|opsdb.observation.*|logging|metrics|tracing; mandatory and uniform
LF5|Notification|opsdb.notification|channel-agnostic notification operations
LF6|Templating and rendering|opsdb.templating.*|deliberately dumb; substitution+inclusion only
LF7|Git operations|opsdb.git|clone|commit|push|tag|PR; for GitOps integration and schema-evolution tooling

# libraries(id|name|family|mandatory|purpose|enforces_policy)
LL1|opsdb.api|LF1|yes|only path runner→OpsDB; auth+correlation+stale-version+audit|yes; report-key check fail-fast
LL2|opsdb.world.kubernetes|LF2|conditional|K8s API access for runners|yes; runner_k8s_namespace_target
LL3|opsdb.world.cloud|LF2|conditional|provider-agnostic cloud ops with per-provider backends|yes; runner_cloud_account_target
LL4|opsdb.world.host|LF2|conditional|SSH+remote command for legacy substrate|yes; runner machine/host_group target
LL5|opsdb.world.registry|LF2|conditional|container/artifact registry access|yes; runner registry access decl
LL6|opsdb.world.secret|LF2|conditional|secret backend access; never persists values|yes; runner secret access target
LL7|opsdb.world.identity|LF2|conditional|IdP operational queries (lookup+membership+watch)|reads only; no mutations
LL8|opsdb.world.monitoring|LF2|conditional|monitoring authority queries (Prom+Datadog+log aggregator)|reads; runner monitoring scope
LL9|opsdb.world.pointer|LF2|convenience|resolve_and_fetch given authority_pointer_id|composes API client + world lib
LL10|opsdb.coordination.retry|LF3|optional|retry+backoff+jitter+idempotency keys|composes with all outbound libs
LL11|opsdb.coordination.circuit_breaker|LF3|optional|prevents cascading failure; per-target state|may sync via observation_cache_state
LL12|opsdb.coordination.hedger|LF3|optional|reduce tail latency via redundant requests|requires idempotent operation
LL13|opsdb.coordination.bulkhead|LF3|optional|isolate failure domains via bounded resource pools|domain key per call
LL14|opsdb.coordination.failover|LF3|optional|primary→replica routing|hides topology from runner
LL15|opsdb.observation.logging|LF4|yes|structured log format with correlation+runner_job_id|bypass=not first-class log
LL16|opsdb.observation.metrics|LF4|yes|metrics emission in standard format|validates against runner_capability decl
LL17|opsdb.observation.tracing|LF4|yes|distributed trace context propagation|joins to audit_log_entry+runner_job
LL18|opsdb.notification|LF5|conditional|email|chat|page|ticket; channel-agnostic|yes; runner notification scope+paging authority
LL19|opsdb.templating.helm_values|LF6|conditional|render helm values from OpsDB config; resolves secret refs|no expressions; substitution only
LL20|opsdb.templating.config|LF6|conditional|render config templates with substitution+inclusion+bounded iteration|no expressions+no conditionals+no functions+no embedded code
LL21|opsdb.templating.report|LF6|conditional|generate reports for human consumption|substitution+inclusion only
LL22|opsdb.git|LF7|conditional|git ops via secret-backend creds|yes; runner repo access decl

# contract_components(id|component|content)
LC_C1|operations|what can be called; name+description+inputs+outputs+guarantees+failure modes
LC_C2|inputs|typed parameters with bounds; library validates+rejects malformed at call boundary
LC_C3|outputs|typed return values with structured metadata; success/failure status+structured error+observation data (latency+retry count)
LC_C4|guarantees|idempotency where applicable+ordering where applicable+freshness annotations from caches+bounded execution time
LC_C5|failure modes|auth failures+authz failures (incl library-layer policy denials)+network errors with retry classification+world-side errors+timeout+bound exceeded

# boundary(aspect|content)
LB1|test|"if same code would appear in two runners → library; if specific to one runner's job → runner"
LB2|in library|authentication mechanisms; world-side substrate access; resilience patterns; observation; notification; templating; git
LB3|in runner|runner-specific decision logic; when to act; transform inputs to actions; interpret authority responses for runner's purpose
LB4|boundary changes|patterns migrate runner→library by accretion; reverse rare
LB5|extraction trigger|3 runners with same pattern = candidate; 10 = confirmed; library_steward calibrates
LB6|test for runner size|>500 lines = doing more than one thing OR reinventing what libraries provide

# runner_content(id|category|examples)
LR1|Decision logic|when to act+threshold computation+target selection+repair-vs-skip choice
LR2|Transformation|inputs to actions+queries to typed targets+results to evidence payloads
LR3|Interpretation|authority responses for runner's specific purpose+domain-specific normalization
LR4|Construction|change_set field changes+report payloads+evidence record details
LR5|Per-target detail|per_target_status interpretation+per_target_data_json shape

# api_client_ops(id|operation|class|inputs|outputs|notes)
LA1|get_entity|read|entity_type+id|row+version stamp+last_updated_time+freshness+governance flags consulted|primary key fetch
LA2|get_entity_history|read|entity_type+id+time_range|version sibling rows ordered+linked to change_set_id|version chain
LA3|get_entity_at_time|read|entity_type+id+timestamp|reconstructed row state|single lookup against version sibling
LA4|search|read|filter+joins+projection+ordering+pagination+freshness+view-mode|results+cursor+freshness summary+filtering disclosures|discovery surface
LA5|get_dependencies|read|starting_entity+relationship_pattern|resolved chain|library translates pattern to API search calls
LA6|resolve_authority_pointer|read|authority_pointer_id|connection details+locator+last_verified_time+pointer metadata|library does NOT fetch from authority
LA7|change_set_view|read|change_set_id+view_mode|scoped or full view+summary|filtered to viewer permissions
LA8|write_observation|write-direct|target_table+key+value+payload|outcome|library validates report-key authorization locally fail-fast before round-trip
LA9|submit_change_set|write-cs|field_changes+reason+metadata|change_set_id|library handles structured construction+optimistic concurrency stamps+dry-run support
LA10|approve_change_set|cm-action|change_set_id+comment|approval recorded|caller identity verified through API
LA11|reject_change_set|cm-action|change_set_id+reason|rejection recorded|
LA12|cancel_change_set|cm-action|change_set_id|change_set status=cancelled|withdrawal
LA13|emergency_apply|write-cs|field_changes+reason+justification|change_set with is_emergency+pending review|break-glass
LA14|apply_change_set_field_change|write-direct|change_set_id+field_change_id|applied|executor's apply-write
LA15|mark_change_set_applied|cm-action|change_set_id|change_set status=applied|finalize after all field changes applied
LA16|watch|stream|entity_type+filter+resume_token|event stream to callback|library handles stream+reconnect+resume; on reconnect fetches current state then streams from token (always level-triggered backstop)

# api_client_features(id|feature|content)
LA_F1|stale-version retry|library auto-refetches+retries N times (default 3) when runner opted in to auto-reconciliation; only if retry merge does not introduce conflicts; complex bundles handled by runner
LA_F2|audit correlation|runner_job_id root → headers → audit_log_entry; chains for runner-triggered runners; query joins via correlation
LA_F3|failure surfacing|validation_failed+authorization_denied+stale_version+not_found+bound_exceeded+network_error+internal_error
LA_F4|why mandatory|alternate paths would make OPSDB-6 disciplines advisory; not building alternative paths IS the discipline

# world_libs(id|library|substrate|target_validation_against|key_ops)
LW1|LL2|kubernetes|runner_k8s_namespace_target (cluster+namespace)|apply_manifest|query_resource|query_resources|watch_resources|helm_render|helm_install|exec_in_pod|get_pod_logs
LW2|LL3|cloud (AWS+GCP+Azure)|runner_cloud_account_target|provision_resource|query_resource|query_resources|modify_resource|decommission_resource|watch_resource_events
LW3|LL4|host (SSH)|runner machine target+host_group target|exec_command|copy_file|apply_config
LW4|LL5|registry (ECR+GCR+Docker Hub+Harbor+Artifactory)|runner registry access decl|pull_image|query_image_metadata|list_tags|verify_signature
LW5|LL6|secret backend (Vault+AWS-SM+GCP-SM+Azure-KV)|runner secret access target|fetch_secret|store_secret|rotate_credential|list_secrets
LW6|LL7|identity provider|reads only|get_user|get_group_members|check_membership|watch_identity_changes
LW7|LL8|monitoring (Prom+Datadog+log aggregators)|runner monitoring scope|prometheus_query|prometheus_query_range|datadog_query|log_query|fetch_recent_alerts
LW8|LL9|composition over LL1+world libs|none directly|resolve_and_fetch
# rule: each substrate gets own library because substrates evolve at different cadences; isolation contains change

# coordination(id|library|pattern|ops|composition)
LC1|LL10|retry+backoff+jitter|with_retry|with_idempotency_key|composes inside every outbound lib; runners can wrap own logic
LC2|LL11|circuit_breaker|call_with_breaker|state per-runner-instance default OR synced via observation_cache_state per target; pre-trip from cached health
LC3|LL12|hedger|hedge_call|requires operation marked idempotent; library validates before allowing hedge
LC4|LL13|bulkhead|with_bulkhead|domain_key per call; pool sizing+queueing+timeout per policy
LC5|LL14|failover|call_with_failover|primary→replicas in order; hides topology
# why libraries not runner code: each pattern non-trivial+subtle correctness; library reviewed once tested once used many times

# observation(id|library|format|emitted_per_call|special)
LO1|LL15 logging|JSON or logfmt typically|timestamp+severity+runner_job_id+correlation_id+runner_spec name+version+runner_machine_id+source location|destination from runtime env (stdout in containers+syslog in hosts+direct to aggregators)
LO2|LL16 metrics|prometheus or statsd or datadog|counter_increment|gauge_set|histogram_observe|timer|labels validated against runner_capability declarations; metrics not declared rejected (analog of report_keys for outbound metrics)
LO3|LL17 tracing|OTel or vendor-native|start_span|with_span|inject_trace_context|extract_trace_context|trace IDs correlated with audit_log_entry+runner_job; pivot from trace to OpsDB structure
# why mandatory: consistent observation precondition for operational visibility; different formats break aggregation+correlation+dashboards; "make state observable" applied to runner population itself

# notification(aspect|content)
LN1|operations|send_notification (channel_id+content+recipients+urgency)|send_to_role (resolves on_call_assignment)|page (severity+escalation_path lookup)|create_ticket|post_to_thread
LN2|channel config|in OpsDB as authority rows of types chat_platform|ticketing_system|paging_provider; library reads at startup+runtime
LN3|recipient resolution|role→on_call_assignment→ops_user→contact info→dispatch via channel; runner says "page database SRE on-call about this"; library handles chain
LN4|authorization|library validates runner notification target scope+paging requires explicit paging authority
LN5|why library not API|OPSDB-6 §10: API does not communicate with stakeholders because channels evolve at different cadence; library contains channel evolution; channel switch = library backend change + OpsDB authority row update

# templating(aspect|content)
LT1|allowed|variable substitution {{ var }}+inclusion {{ include "other" }}+bounded iteration {% for item in list_var %}
LT2|forbidden|expressions ({{ a + b }}+filters)+conditionals over expressions+function calls+embedded code
LT3|libraries|LL19 helm_values|LL20 config|LL21 report
LT4|why dumb|template language with logic accumulates over time; authors find it easier to add conditional than runner; templates become opaque code embedded in supposed-data
LT5|cost|occasional duplication (value computed two slightly different ways for two templates)
LT6|benefit|templates inspectable as data
LT7|upstream pattern|logic in runner produces concrete value as configuration_variable+template substitutes variable

# git(aspect|content)
LG1|operations|clone|commit|push|tag|create_pull_request|fetch_file
LG2|auth|via secret backend (SSH keys+deploy tokens+OAuth tokens per provider)
LG3|authorization|dual: runner declared scope (which repos) + git provider permissions; both must permit
LG4|structured commit message|helper structured_commit_message(change_set_id+summary+references); helm git exporter calls helper not ad hoc; consistent across runner population enables pivot back to OpsDB-side trail

# versioning(id|aspect|content)
LV1|semver|MAJOR.MINOR.PATCH
LV2|PATCH|bug fixes no contract change
LV3|MINOR|new operations OR backward-compat extensions; new optional params+new return fields+new capability declarations
LV4|MAJOR|breaks contract; rare; deprecation cycles required
LV5|deprecation pattern|new alongside old → both supported N cycles (typically 3-5) → deprecation warnings → runners migrate at own pace → removal only after steward confirms no consumer depends → tombstone documentation remains
LV6|removal rare|most "deprecated" remain available indefinitely as legacy paths; cost is supporting them; benefit is not breaking consumers
LV7|test suite|every contract has test suite; new implementation gated by passing; "implements the contract" operationally defined as "passes the suite"
LV8|test coverage|all operations+all declared failure modes+all policy enforcement+idempotency where claimed+bounded behavior where claimed
LV9|release pipeline|normal software pipeline; runners pin to versions+update deliberately through own release cycle
LV10|coordination with schema evolution|library changes touching schema land schema change_set first → library version using new fields released after → runners migrate after; order matters

# steward(aspect|content)
LS1|role|library_steward parallel to schema_steward; senior engineer/architect
LS2|reviews|new library proposals (library or runner?)|contract additions (right shape?)|contract removals (consumers migrated?)|cross-library coherence (compose well?)|implementation quality (all impls pass tests?)
LS3|resists fragmentation|every team wanting "own version of library X" solving real problem; absorb into standard
LS4|investment|10-25% role typical orgs; full-time + small team for largest orgs with hundreds of runners + multiple language impls
LS5|investment compounds|well-stewarded suite makes next runner cheap; team finds library calls already exist+documented+tested+integrated; their runner small because suite good

# two_sided(id|surface|enforces_against|input|failure_mode)
LP1|API gate (OPSDB-6 §3.5)|OpsDB writes|10-step gate sequence|caught before persisting; structured error to runner
LP2|library suite|world-side actions|runner declarations as OpsDB rows|caught before reaching substrate; structured error to runner
LP3|composition|both surfaces use same input (runner declarations) → produce same fail-closed result
LP4|comprehensive coverage|runner cannot through any path act outside declared scope: writes caught at gate+world-side caught at library
LP5|"runner authority is data" (OPSDB-5 §8.5)|holds across every action; every authority is data+every check mechanical+every failure fail-closed and audit-logged

# policy_validation(library|extracted_target|declaration|failure)
PV1|LL2 K8s|cluster+namespace|runner_k8s_namespace_target|library_authorization_denied with target+missing decl
PV2|LL3 cloud|cloud_account_id|runner_cloud_account_target|same shape
PV3|LL4 host|host_id|machine target OR host_group target|same shape
PV4|LL5 registry|registry+repo|runner registry access decl|same shape
PV5|LL6 secret|secret path|runner secret access target|same shape
PV6|LL18 notification|channel_id|runner notification scope+paging authority for pages|same shape
PV7|LL22 git|repo|runner repo access decl|same shape
# pattern: extract target from operation params → look up runner declarations from cached OpsDB data → check coverage → proceed if covered OR reject with structured error → log to observation library + emit metric

# fail_closed(aspect|content)
FC1|principle|if library cannot determine authorization → refuse rather than allow (OPSDB-9 §5.3 fail closed at library layer)
FC2|partition tolerance|library uses last-known-good cache of declarations
FC3|bounded staleness|after threshold library refuses calls because declarations no longer trusted
FC4|threshold per-library policy-driven|short for security-sensitive (secrets+paging+cloud provisioning); longer for benign

# audit_composition(aspect|content)
AC1|library rejections|structured logs queryable alongside API rejections
AC2|joined query|"every authorization denial for runner X in last hour" returns gate denials (audit_log_entry) + library denials (observation logs+metrics)
AC3|denial structure|surface that denied + missing declaration + attempted target
AC4|investigation pivot|denial → runner_spec → change_set defining declarations → approver who authorized
AC5|trail composition|world-side action attempted → library's check → OpsDB declaration → change_set creating declaration → approver = queryable join
AC6|completion claim|audit trail OPSDB-6 promised at OpsDB write surface extends through library layer to cover world-side actions

# failure_modes(id|error|meaning|runner_response)
FM1|validation_failed|schema or bound validation rejected call|runner records failure; next cycle picks up
FM2|authorization_denied|one of 5 layers denied (OPSDB-6 §6.2)|runner records; investigation may surface declaration gap
FM3|stale_version|optimistic concurrency conflict|runner fetches current+reconciles+resubmits OR library auto-retries if opted in
FM4|not_found|targeted entity does not exist|runner records; possibly skip target
FM5|bound_exceeded|query+search+rate limit exceeded|runner records; possibly back off
FM6|network_error|transport-level failure with retry classification|library retries if classified retryable
FM7|internal_error|API itself failed|rare; itself a finding
FM8|library_authorization_denied|library-layer scope check failed|runner records; investigation surfaces declaration gap
FM9|undeclared_report_key|library fail-fast before round-trip|runner records; investigation surfaces missing declaration

# refusals(id|not_a|why|belongs_in)
LD1|Runner framework|library is callable not controlling shell; framework owning runner's main loop couples every runner to framework's evolution|runner's main loop+event dispatch+lifecycle stay runner's responsibility
LD2|Workflow engine|library does not mediate runner-to-runner messaging; would be orchestrator by another path (OPSDB-2 §4.1)|runners coordinate through OpsDB rows
LD3|Code distribution system|library impls distributed via PyPI+container registries+language-native+internal artifact stores; OpsDB holds operational data not code|standard package mechanisms
LD4|Secrets store|secret library accesses backends; never persists values; recursive boundary from OPSDB-4 §13.8|secret backend is SoT
LD5|Service mesh|library makes outbound calls; does not intercept other components' traffic|service-mesh products at network layer
LD6|UI|no UI in suite; runners observed via observation libs + dashboards on OpsDB|downstream concern consuming OpsDB
LD7|Database|OpsDB is the database; library state ephemeral or written to OpsDB|library uses OpsDB OR operates ephemerally

# adoption(step|action|purpose)
AD1|minimum starting set|LL1 (api client) + LL15 (logging)|smallest viable suite; many simple runners need nothing more
AD2|add world-side as domains arrive|LL2 when K8s coordination added; LL3 when cloud governance added; LL8 when monitoring integrated|each library built once+paid back many times by subsequent consumers
AD3|add coordination as patterns emerge|extract once seen 3 times; calibrate not premature not deferred|library_steward judges
AD4|cross-implementation portability|first language canonical reference; subsequent ports gated by passing contract test suite|order: API client first → standard observation → world-side as needed → coordination as patterns appear
AD5|library_steward investment grows|small suite small role; mature suite (20 libraries+multiple language impls) substantial|proportional to suite maturity+rate of change
AD6|discipline patterns|resist fragmentation+resist feature creep+invest continuously+maintain steward role+treat as long-lived artifact|same patterns from prior papers applied at library layer

# examples(runner|in_runner|in_library|approx_lines)
EX1|PVC-repair|attention-worthy state selector+repair decision tree (rebind/resize/replace)+manifest construction+outcome recording per_target_data_json|API client (state read+change_set submit)+K8s ops (apply+watch+events)+retry+observation+notification (rare)|~200 lines (150 specific + 50 glue)
EX2|drift detector|desired state query+observed state query+entity-class-specific diff (fields ignored+normalized)+act-vs-review decision per policy+change_set field changes construction|API client+future opsdb.utility.diff+observation|~250 lines
EX3|backup verifier|read backup schedule+decide which should have happened+evidence record payload+storage company response interpretation+confirmation email content interpretation|API client (schedule read+evidence write)+storage ops (cloud or storage_provider lib)+IMAP (small email lib)+observation|~300 lines
# pattern: runner contains decision logic; libraries contain world-side I/O+OpsDB I/O+resilience+observation; runner small because libraries good

# relationships(from|rel|to)
L1|implements|L2
L1|enables|L8
L3|enforces|boundary-discipline
L4|requires|L5
L5|prevents|inconsistency-across-runner-population
L9|composes|API-gate+library-suite
L9|enforces|comprehensive-coverage
L10|prevents|library-fragmentation
L11|enforces|configuration-as-data
L12|preserves|secret-backend-boundary
L14|enables|audit-trail-composition
LL1|mandatory_for|every-runner
LL1|enforces|API-only-path-discipline
LL2|enforces|PV1
LL3|enforces|PV2
LL4|enforces|PV3
LL5|enforces|PV4
LL6|enforces|PV5
LL6|preserves|secret-non-persistence
LL18|enforces|PV6
LL22|enforces|PV7
LL15|mandatory|true
LL16|mandatory|true
LL17|mandatory|true
LL15|enforces|consistent-log-format
LL16|enforces|metric-declaration-discipline
LL10|composes_inside|every-outbound-library
LL11|may_sync_via|observation_cache_state
LL12|requires|operation-idempotency
LF1|prereq_of|all-other-families
LF4|prereq_of|operational-visibility
LA8|fail_fast|true
LA8|gated_by|runner_report_key
LA9|handles|optimistic-concurrency
LA9|supports|dry-run
LA13|creates|change_set_emergency_review
LA16|level_triggered_on_reconnect|true
LF6|forbids|expressions+conditionals+functions+embedded-code
LF6|allows|substitution+inclusion+bounded-iteration
LV5|parallels|OPSDB-7-§12.4-duplication-pattern
LV7|gates|new-implementation-acceptance
LV10|orders|schema-first-then-library-then-runner
LS1|parallels|schema_steward (OPSDB-2 §14.12)
LS3|prevents|library-fragmentation
LP1|enforces|OpsDB-write-surface
LP2|enforces|world-side-action-surface
LP1|composes_with|LP2
LP3|input|runner-declarations
LP4|prevents|action-outside-declared-scope
LP5|completes|runner-authority-as-data
PV_ALL|fail_closed|true
PV_ALL|data_driven|true
FC1|implements|OPSDB-9-§5.3
FC3|bounded_staleness|true
AC6|completes|OPSDB-6-§11-promise
FM3|may_auto_retry|via LA_F1
FM8|new_in|OPSDB-8
FM9|fail_fast_at|library-layer
LD1|prevents|framework-coupling
LD2|prevents|orchestrator-by-another-path
LD3|preserves|code-vs-data-boundary
LD4|preserves|secret-non-persistence-recursively
LD5|preserves|application-vs-network-layer-boundary
LD6|preserves|operational-vs-presentation-boundary
LD7|preserves|single-database-of-record
AD1|sufficient_for|simple-pullers+simple-verifiers
AD2|each_library_built_once|paid-back-many-times
AD3|extraction_threshold|3-runners-with-pattern
AD4|gating|contract-test-suite-passage
AD5|investment_proportional_to|suite-maturity+change-rate
AD6|continues|same-discipline-as-prior-papers
EX1|demonstrates|L4-line-count-claim
EX2|demonstrates|L4-line-count-claim
EX3|demonstrates|L4-line-count-claim
EX_ALL|demonstrate|L3-boundary-test
LP_ALL|implement|two-sided-enforcement
AC_ALL|implement|trail-completion-through-library-layer

# section_index(section|title|ids)
1|Introduction|L1,L2,L3,L4,L5,L8,L9
2|Conventions|inherited from prior series; L11 forbids expressions in templates
3|What Shared Library Is|LC_C1,LC_C2,LC_C3,LC_C4,LC_C5,LB1,LB2,LB3,LB4,LB5,LB6,LR1,LR2,LR3,LR4,LR5,L6,L7,L10
4|OpsDB API Client|LL1,LA1-LA16,LA_F1,LA_F2,LA_F3,LA_F4
5|World-Side Substrate Libraries|LL2-LL9,LW1-LW8
6|Coordination and Resilience|LL10-LL14,LC1-LC5
7|Observation Libraries|LL15-LL17,LO1,LO2,LO3
8|Notification Libraries|LL18,LN1,LN2,LN3,LN4,LN5
9|Templating and Rendering|LL19,LL20,LL21,LT1-LT7
10|Git and Version Control|LL22,LG1,LG2,LG3,LG4
11|Library Implementation Discipline|LV1-LV10,LS1-LS5
12|Library/Runner Boundary in Practice|EX1,EX2,EX3
13|Two-Sided Policy Enforcement|LP1-LP5,PV1-PV7,FC1-FC4,AC1-AC6
14|What Library Suite Is Not|LD1-LD7
15|Adoption and Growth|AD1-AD6
16|Closing|L1-L14 restated structurally

# decode_legend
families: api|world|coordination|observation|notification|templating|git
mandatory_libraries: opsdb.api|opsdb.observation.logging|opsdb.observation.metrics|opsdb.observation.tracing
operation_classes: read|write-direct|write-cs|cm-action|stream
contract_components: operations|inputs|outputs|guarantees|failure-modes
boundary_test: would two runners reimplement this? yes→library no→runner
extraction_threshold: 3-runners-pattern=candidate; 10=confirmed
versioning: MAJOR.MINOR.PATCH semver with deprecation cycles parallel to OPSDB-7 §12.4
two_sided_surfaces: API-gate (writes)+library-suite (world-side actions)
policy_validation_pattern: extract-target → look-up-declarations-cache → check-coverage → proceed-or-reject-fail-closed → log+metric
fail_closed_principle: if cannot determine authorization → refuse not allow
template_allowed: substitution|inclusion|bounded-iteration
template_forbidden: expressions|conditionals-over-expressions|function-calls|embedded-code
secret_discipline: in-memory-only-during-call; never-logged|written-to-OpsDB|persisted-in-runner-state
runner_size_target: ~200 lines (150 specific + 50 library glue)
suite_starting_set: opsdb.api + opsdb.observation.logging
sot_for_secrets: secret-backend (vault+equivalents); library never persists
sot_for_code: repositories (git+container-registries); OpsDB never holds
sot_for_runner_authority: OpsDB declaration rows; libraries+API both validate against
rel_types: implements|enables|requires|enforces|composes|composes_with|composes_inside|prevents|preserves|completes|gates|orders|parallels|forbids|allows|input|may_sync_via|may_auto_retry|fail_fast|fail_closed|data_driven|level_triggered_on_reconnect|mandatory|mandatory_for|prereq_of|gated_by|sufficient_for|extraction_threshold|investment_proportional_to|continues|demonstrate|new_in|fail_fast_at|each_library_built_once|paid-back-many-times|bounded_staleness
