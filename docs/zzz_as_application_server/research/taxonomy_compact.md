# AppDB Application Taxonomy

## Mechanisms, Properties, and Principles for Governed Application Development

---

## 1. Introduction

This taxonomy enumerates every mechanism, property, and principle relevant to building applications on a governed data substrate. The substrate — OpsDB Application Architecture — provides schema-driven validation, five-layer authorization, change management with approval routing, full-state versioning, append-only audit logging, bounded search, and a runner-based automation layer. Applications built on this substrate compose the taxonomy's primitives to produce their specific behavior.

The taxonomy uses the three-axis framework established by the infrastructure taxonomy: mechanisms are building blocks that perform work, properties are contracts claimed about how work behaves, and principles are rules governing which mechanisms are chosen and how they are assembled. These are distinct kinds of things. The same word often names all three. Conflation prevents precise comparison. Resolution requires qualification: "durability mechanism" versus "durability property" versus "durability-related principle."

Three sources contribute to this taxonomy. The infrastructure taxonomy provides 62 mechanisms, 21 properties, and 22 principles discovered through operational infrastructure management. Application domain analysis adds mechanisms, properties, and principles specific to application development that operational infrastructure does not require. Silo architecture analysis adds mechanisms, properties, and principles from data-only execution systems, filtered to patterns that operate at the application layer within the OpsDB abstraction boundary.

Every item carries an origin tag: `infra` for infrastructure taxonomy, `app` for application analysis, `silo` for Silo analysis.

Every item derived from Silo analysis carries an abstraction qualifier stating whether it holds at the full stack, at the OpsDB application layer only, or at the external hot-path system layer. This scoping is honest: OpsDB runs as a userspace application on top of Postgres and a standard operating system. Properties that require control over the hardware exception model, the execution runtime, or the transport parsing layer beneath HTTP do not hold and are excluded.

The three-axis structure is the load-bearing claim. Contents of each axis are revisable. The structural claim — that mechanism, property, and principle are distinct — is not.

---

## 2. Terminology

**Mechanism.** A building block that takes inputs, produces outputs, and performs side effects. Identified by interface, not implementation. Multiple implementations of the same mechanism coexist.

**Property.** A contract that holds over operations performed by mechanisms. Claims span across operations, failures, and time. Multiple mechanisms can provide the same property. Properties have conditions under which they hold and conditions under which they degrade.

**Principle.** A rule governing choice and assembly. Does not perform work or make guarantees about specific operations. Constrains which mechanisms are chosen, how they are configured, and how they compose.

**Abstraction boundary.** The OpsDB abstraction boundary encompasses the schema engine, gate pipeline, change management system, versioning system, audit log, search API, runner execution, and library suite. Below this boundary: the Go runtime, Postgres query executor, operating system, and network transport. Properties qualified "application layer" hold within the boundary but not necessarily below it.

**Origin tags.** `infra` = inherited from infrastructure taxonomy. `app` = discovered through application domain analysis. `silo` = discovered through data-only execution architecture analysis, filtered to OpsDB-applicable patterns.

**Qualification discipline.** Always qualify ambiguous words. "Vocabulary closure mechanism" versus "vocabulary closure property." "Security by anatomy principle" versus "structural security property." The taxonomy enforces this throughout.

---

## 3. Mechanism Axis

### 3.1 Inherited Mechanisms (infra)

All 62 mechanisms from 13 families in the infrastructure taxonomy are inherited. Definitions, subtypes, and composition relationships are referenced from the infrastructure taxonomy. The following table annotates each family with its application relevance.

```
inherited_families(id|family|mechanism_count|opsdb_substrate|runner_library|hot_path|app_notes)
F1|Information Movement|5 (Channel,Fanout,Funnel,Replicator,Relay)|Replicator for DB replication|Cloud+K8s+SSH+notification libs|message bus,streaming|runners bridge OpsDB to external systems via library calls
F2|Selection|6 (Index,Selector,Comparator,Hasher,Ranker,Router)|search API indexes+selectors|query helpers in API client|search engines,auction scoring|search API provides Selector+Comparator+Ranker for application queries
F3|Representation|4 (Wrap/unwrap,Schema,Namespace,Naming convention)|schema engine,JSON validation,naming conventions|payload serialization|protocol encoding|schema engine is the primary Representation mechanism for applications
F4|Storage|7 (Buffer,Cache,Store,Journal,Log,Snapshot,Tombstone)|Postgres Store+Journal(WAL)+read cache+audit Log|observation cache writes|time-series,object store|Store and Journal delegated to Postgres; Cache managed by API layer
F5|Versioning|5 (Version stamp,History,Merge algorithm,Diff,Reference)|version stamps,full-state History,Diff in change sets|version comparison in reconcilers|git for GitOps|versioning sibling tables implement History with version stamps
F6|Lifecycle|4 (TTL,Lease,Reaper,Drainer)|TTL on cache,Reaper runner,retention policies|lease-based auth tokens|session management|Reaper runner enforces retention; TTL on observation cache
F7|Sensing|6 (Probe,Counter,Gauge,Histogram,Watch,Heartbeat)|audit log counters,Watch for streaming|logging+metrics+tracing libs|monitoring systems|mandatory observation libraries provide Counter+Gauge; Watch API for streaming
F8|Control Loop|4 (Reconciler,Reactor,Scheduler,Workqueue)|change-set executor drains queue|reconciler+reactor runner kinds|orchestrators|runner pattern implements Reconciler and Reactor; Scheduler via schedule entities
F9|Gating|6 (Authenticator,Authorizer,Validator,Mutator,Filter,Limiter)|all six in gate pipeline|library scope validation|API gateways,firewalls|gate pipeline is composed sequence of Authenticator→Authorizer→Validator→Filter→Limiter
F10|Allocation|4 (Pool,Quota,Scheduler-alloc,Sharder)|connection pooling,rate limit quotas|per-runner resource bounds|load balancers,container scheduling|rate limiting implements Quota; read cache implements Pool
F11|Coordination|5 (Lock,Election,Barrier,Quorum,Sequencer)|advisory locks for schema apply,Sequencer for version serial|idempotency keys|distributed consensus|advisory lock prevents concurrent schema applies; version serial is Sequencer
F12|Transformation|3 (Renderer,Transformer,Compactor)|template rendering (limited)|config+report rendering in libs|stream processors,ETL|templating library (substitution only); Compactor via Postgres vacuum
F13|Resilience|5 (Retrier,Circuit breaker,Bulkhead,Hedger,Failover)|retry in API client|retry+circuit breaker+bulkhead+failover in libs|every distributed system|library suite provides full resilience mechanism set
```

```
inherited_mechanisms(id|name|family|opsdb_component|app_relevance)
M01|Channel|F1|HTTP API transport|every API call traverses a channel
M02|Fanout|F1|notification dispatch to multiple channels|notification runner dispatches to N recipients
M03|Funnel|F1|observation writes from many runners|many pullers write to shared observation cache
M04|Replicator|F1|Postgres streaming replication|read replicas for scaling; cross-region for availability
M05|Relay|F1|API as relay between frontend and substrate|API mediates all frontend-to-database communication
M06|Index|F2|Postgres indexes on entity tables|search API performance depends on indexes declared in schema
M07|Selector|F2|search API filter predicates|every search query is a Selector invocation
M08|Comparator|F2|version stamp comparison at submit|optimistic concurrency uses Comparator on version stamps
M09|Hasher|F2|content hashing for audit chain|_audit_chain_hash uses cryptographic Hasher
M10|Ranker|F2|search API ordering|query results ordered by declared field+direction pairs
M11|Router|F2|change management approval routing|approval rules route change sets to appropriate approver groups
M12|Wrap/unwrap|F3|JSON serialization/deserialization|API boundary encoding; discriminator payload validation
M13|Schema|F3|schema engine|the primary mechanism: YAML→DDL→runtime metadata→API validation
M14|Namespace|F3|entity type naming+field naming|naming conventions enforce uniqueness and disambiguation
M15|Naming convention|F3|FK naming,datetime suffix,boolean prefix|loader enforces naming rules; deterministic identifier construction
M16|Buffer|F4|in-memory request buffers|API request processing pipeline buffers
M17|Cache|F4|read scaling cache,observation cache|tenant-aware cache between API and substrate
M18|Store|F4|Postgres as primary Store|authoritative data location for all governed entities
M19|Journal|F4|Postgres WAL|write-ahead log provides crash recovery
M20|Log|F4|audit_log_entry table|append-only record of every API action
M21|Snapshot|F4|database backups|backup runner produces snapshots on schedule
M22|Tombstone|F4|soft delete via is_active=false|deleted entities persist as tombstones for history
M23|Version stamp|F5|version_serial on sibling rows|monotonic identifier per entity version
M24|History|F5|versioning sibling tables|ordered sequence of full-state versions
M25|Merge algorithm|F5|not used (optimistic concurrency prevents divergence)|conflicts detected and rejected rather than merged
M26|Diff|F5|change_set_field_change records|structural description of change between versions
M27|Reference|F5|_schema_version.is_current,authority_pointer|named pointers to current version and external facts
M28|TTL|F6|observation cache freshness|max_staleness_seconds on cached observations
M29|Lease|F6|auth token expiration|SSO tokens and service account credentials have bounded lifetime
M30|Reaper|F6|reaper runner|enforces retention policies by removing data past horizon
M31|Drainer|F6|graceful runner shutdown|runner completes current cycle before stopping
M32|Probe|F7|health checks on API|liveness and readiness probes for API deployment
M33|Counter|F7|metrics counters in observation library|runner metrics emission; audit log entry counts
M34|Gauge|F7|current-value metrics|queue depths,cache hit rates,active connections
M35|Histogram|F7|latency distributions|API response time distributions; runner cycle time distributions
M36|Watch|F7|streaming API operation|subscribe to entity changes with resume token
M37|Heartbeat|F7|runner cycle as implicit heartbeat|runner_job records prove runner liveness
M38|Reconciler|F8|reconciler runner kind|compares desired vs observed,acts or proposes
M39|Reactor|F8|reactor runner kind|edge-triggered event response
M40|Scheduler (control)|F8|scheduler runner kind|enforces schedules on target substrate
M41|Workqueue|F8|change-set executor queue|approved change sets as work items
M42|Authenticator|F9|gate step 1|SSO delegation+service account validation
M43|Authorizer|F9|gate step 2 (five layers)|role+group+entity governance+field classification+runner scope+policy rules
M44|Validator|F9|gate steps 3-4|schema validation+bound validation
M45|Mutator|F9|reserved field injection by loader|loader injects id,created_time,updated_time; API sets timestamps
M46|Filter|F9|gate step 5 policy evaluation|policy rows filter operations by additional constraints
M47|Limiter|F9|rate limiting per identity per endpoint|gate pipeline rate limits
M48|Pool|F10|database connection pool|pgbouncer or equivalent managing connections
M49|Quota|F10|per-tenant query limits|configurable per role as policy data
M50|Scheduler (allocation)|F10|runner capacity allocation|runner_machine.capacity_concurrent_jobs
M51|Sharder|F10|potential future partitioning|schema-declarative for high-frequency entities
M52|Lock|F11|advisory locks for schema apply|prevents concurrent schema applications
M53|Election|F11|not directly used|single-primary Postgres handles leader selection
M54|Barrier|F11|bulk change set chunk boundary|all chunks must validate before any transitions
M55|Quorum|F11|not directly used|Postgres replication quorum if configured
M56|Sequencer|F11|version_serial generation|monotonic version numbers per entity
M57|Renderer|F12|templating library (substitution only)|helm values,config templates,report templates
M58|Transformer|F12|runner transformation logic|puller data transformation,diff computation
M59|Compactor|F12|Postgres vacuum|reclaims storage from deleted/updated rows
M60|Retrier|F13|API client retry+library retry|exponential backoff with jitter on transient failures
M61|Circuit breaker|F13|library circuit breaker|per-target failure detection and fast-fail
M62|Bulkhead|F13|AppDB isolation|each AppDB is an isolated failure domain
M63|Hedger|F13|not commonly used|available in library for latency-sensitive runners
M64|Failover|F13|failover handler runner kind|detects primary failure,performs failover,updates state
```

### 3.2 Application-Specific Mechanisms (app)

```
app_mechanisms(id|name|family|definition|distinct_from|used_by|opsdb_implementation|origin)
AM01|State Machine Evaluator|F9 Gating|Takes declared states,declared valid transitions,current state,proposed transition; accepts or rejects based on lifecycle graph|Validator(checks field values) Reconciler(converges state) Authorizer(checks permissions)|AC03 case mgmt+AC13 e-commerce order+AC14 editorial+AC17 shipment+AC18 workflow+AC21 billing+AC19 game turn|policy rows declaring valid transitions evaluated at gate step 5; version history records transition sequence|app
AM02|Rule Engine|F9 Gating|Evaluates population of declared rules against input data; produces decision,matching set,or computed value|Validator(schema-bound) Authorizer(identity-bound) Selector(finds matching data given rules — Rule Engine finds matching rules given data)|AC05 trading rules+AC13 pricing/tax/eligibility+AC19 move validation+AC21 billing rules+AC27 targeting+AC28 risk limits|policy rows with predicate structures; gate step 5 for validation rules; runner-side for business logic rules|app
AM03|Accumulator|F12 Transformation|Maintains running aggregate updated incrementally on write path — sum,count,min,max,weighted average; persists derived quantity|Counter(observation-only) Gauge(current value report) Compactor(reduces redundancy)|AC05 positions+AC11 inventory+AC12 budget+AC13 cart totals+AC21 usage metering+AC27 budget pacing+AC28 positions|runner computes and writes derived values via change set or observation; extension point for gate-computed derived fields|app
AM04|Temporal Projection|F12 Transformation|Expands schedule or recurrence rule into concrete datetime instances over time range|Scheduler-control(assigns work to slots) TTL(marks expiration)|AC07 class schedule+AC15 booking availability+AC06 compliance audit+AC21 billing cycles|runner or API extension projecting schedule entities into instance lists for display and conflict detection|app
```

### 3.3 Silo-Derived Mechanisms (silo)

```
silo_mechanisms(id|name|family|definition|distinct_from|opsdb_implementation|abstraction_layer|origin)
SM01|Execution Funnel|cross-family|Composes heterogeneous mechanism types into monotonic narrowing sequence; each layer uses different computational model (graph traversal,unification,scoring,bytecode)|Pipeline(general) gate pipeline(same model per step)|gate pipeline is variant using policy-check model per step; extension point for composing structural validation+predicate logic+utility scoring+atomic execution as distinct layers|application|silo
SM02|Fact Regeneration|F7 Sensing|Converts imperative state into declarative fact set on every evaluation cycle; decisions always use current-cycle facts|Probe(synthetic test) Cache(holds copies) Watch(reports changes)|runner get-phase re-reads current state each cycle; extension point for declared fact transformation from entity state to decision-input facts|application|silo
SM03|Multiplicative Scoring with Zero-Gating|F2 Selection|Running score starts 1.0; each consideration multiplies in; any zero immediately zeros total; average-and-fixup compensation|Ranker(orders by score without zero-gating algebra)|extension point for approval rule evaluation with weighted multi-factor scoring; any disqualifying factor immediately eliminates|application|silo
SM04|Semantic Repurposing|F3 Representation|Same structural schema serves multiple unrelated domains through data reinterpretation alone; infrastructure has no domain knowledge|Namespace(naming scope) Schema(structure description)|AppDB model: same infrastructure serves billing,healthcare,personal data through schema YAML alone|application|silo
SM05|Hot-Swap via Data Replacement|F8 Control Loop|Behavior definition replaced at runtime through data; effective on next evaluation cycle; no restart,no recompile|Reconciler(converges toward desired state — hot-swap replaces the definition)|data-driven behavior: policy,approval rules,access control,runner config change on change set apply; governance wraps replacement|application|silo
SM06|Scene as Isolation Boundary|F11 Coordination|Complete execution context with own entity pool,behavior definitions,integrated access control; default deny; maps to tenant,request,process,workflow|Bulkhead(failure domain isolation) Namespace(naming scope)|AppDB model: one DOS per app with own schema,API,database,runners,policies; five-layer authz within; cross-AppDB via typed pointers|application|silo
SM07|Structured Trace|F7 Sensing|Captures complete decision chain per entity per evaluation cycle in domain-aware structured form; replay with modified rules|Log(unstructured record) Watch(change subscription)|audit log+version history provides substrate; extension point for per-change-set decision trace with gate evaluation details and policy replay|application|silo
SM08|Ingress Validation|F9 Gating|Structural validation rejects non-conforming requests before semantic interpretation; closed vocabulary prevents operation type injection; fixed types prevent type confusion|Validator(semantic checking) Filter(content-based accept/drop)|gate steps 3-4 before step 5; closed schema vocabulary; runner report key enforcement; partial geometric security at application layer|application (transport parsing below HTTP not controlled)|silo
```

### 3.4 Mechanism Summary

```
mechanism_totals(origin|count|families)
infra|62|13 families (F1-F13)
app|4|F9 Gating (2)+F12 Transformation (2)
silo|8|F2(1)+F3(1)+F7(2)+F8(1)+F9(1)+F11(1)+cross-family(1)
total|74|13 families (no new families added)
```

---

## 4. Property Axis

### 4.1 Inherited Properties (infra)

```
inherited_properties(id|name|band|claim|opsdb_provision|gate_steps|origin)
P01|Idempotency|B1 Data Integrity|applying same operation more than once yields same end state as applying once|change_set atomic commit+version stamp check|AG9|infra
P02|Atomicity|B1|defined unit of work fully completes or has no visible effect|change_set atomic apply; all field changes commit or none|AG9|infra
P03|Durability|B1|committed data survives specified failure modes|Postgres WAL+replication|infrastructure below gate|infra
P04|Consistency-data|B1|declared constraints and invariants hold at specified boundaries|schema validation+bound validation+FK checks|AG3+AG4|infra
P05|Integrity|B1|data has not been altered without detection between defined points|_audit_chain_hash+TLS|AG8+infrastructure|infra
P06|Authenticity|B1|source of data or request is verifiable|SSO delegation+service account auth|AG1|infra
P07|Confidentiality|B1|data unreadable to unauthorized parties|_access_classification+five-layer authz|AG2|infra
P08|Determinism|B2 Behavioral|same inputs produce same outputs and same side effects|closed vocabulary (no regex,no logic); mechanical validation|schema engine|infra
P09|Convergence|B2|repeated application drives state toward fixed point|level-triggered runners; reconciler pattern|runner discipline|infra
P10|Liveness|B2|system continues to make progress under specified conditions|runner cycles+change-set executor draining queue|runner infrastructure|infra
P11|Availability|B2|system responds within specified timeframe|read cache+multi-instance+runner independence from OpsDB|infrastructure|infra
P12|Boundedness|B2|resource consumption stays within specified limit|query bounds+runner bounds+rate limits|AG4+runner config+AG7(rate limit)|infra
P13|Isolation|B2|concurrent operations do not visibly interfere|optimistic concurrency+change_set atomicity|AG9+version stamps|infra
P14|Reversibility|B2|completed operation can be undone|rollback as change_set restoring prior version values|AG7+AG9|infra
P15|Consistency-replica|B3 Distribution|read after write returns written value or specified relationship|read replicas+cache invalidation|infrastructure|infra
P16|Ordering|B3|operations apply in specified order|change_set apply_order+version_serial monotonic|AG9+versioning|infra
P17|Locality|B3|related data sits close together|observation cache locality+read cache|infrastructure|infra
P18|Stability under change|B3|adding/removing components does not cause disproportionate disruption|additive schema evolution+no deletion+no rename|schema engine|infra
P19|Failure transparency|B3|failures of specified components hidden from clients up to bounds|runner independence from OpsDB availability; local cache|runner architecture|infra
P20|Observability|B4 Operational|relevant state queryable through specified interfaces|audit log+version history+runner job records+metrics|AG8+versioning+runner jobs|infra
P21|Auditability|B4|past operations reconstructible from preserved records|append-only audit log+version chain+change_set trail|AG8+AG6+AG7|infra
```

### 4.2 Application-Specific Properties (app)

```
app_properties(id|name|band|claim|conditions|distinct_from|used_by|opsdb_implementation|origin)
AP01|Lifecycle Integrity|B1 Data Integrity|entity state transitions conform to declared lifecycle graph at all times; no entity in state not reached through valid transition sequence|lifecycle graph declared; transitions enumerated; current+prior state tracked|Consistency-data(field-level constraints not transition legality) Ordering(sequence not validity)|AC03+AC13+AC14+AC17+AC18+AC21+AC19|state machine evaluator(AM01)+version history recording transition sequence+policy rows declaring valid transitions|app
AP02|Computational Correctness|B1 Data Integrity|derived quantities (balances,totals,inventory levels,scores) consistent with source data at specified boundaries; derived value equals recomputation from source|derivation function declared; source data specified; boundaries specified (per-write,periodic,eventual)|Consistency-data(schema constraints) Determinism(reproducibility not correctness of derivation)|AC05+AC11+AC12+AC13+AC21+AC28|verifier runners periodically recompute from source; evidence records on match; compliance findings on mismatch|app
AP03|Non-repudiation|B1 Data Integrity|party who performed action cannot deny having performed it; evidence sufficient to prevent denial|authentication non-transferable (no shared credentials); audit trail tamper-evident; cryptographic evidence preserved|Authenticity(verifies identity at request time) Auditability(reconstructs but does not prevent denial)|AC04+AC05+AC06+AC18|SSO per-user credentials+append-only audit with crypto chaining+version history linking state to change set and identity|app
```

### 4.3 Silo-Derived Properties (silo)

```
silo_properties(id|name|band|claim|conditions|distinct_from|opsdb_scope|origin)
SP01|Vocabulary Closure|B2 Behavioral|operation vocabulary at application layer is closed finite set; cannot be extended at runtime through any application-level mechanism|schema vocabulary fixed (9 types,3 modifiers,6 constraints); API operations fixed (16); gate steps fixed (10); runner gating modes fixed (3)|Boundedness(resource limits not operation set limits)|holds within OpsDB abstraction boundary; Go+Postgres below boundary execute arbitrary operations|silo
SP02|Structural Security|B1 Data Integrity|at application validation layer security properties derive from absence of mechanism not policy enforcement; certain attack categories impossible by construction|closed vocabulary prevents injection; absence of regex prevents backtracking; absence of embedded logic prevents code execution|Confidentiality(achieved through encryption mechanism) Integrity(achieved through checksum mechanism)|holds within OpsDB abstraction boundary; transport parsing layer below not controlled|silo
SP03|Hot-Swap Safety (Governed)|B2 Behavioral|behavior definitions replaceable at runtime with zero disruption and full governance: attribution,approval,versioning,audit|replacement through change set pipeline; new behavior effective on apply; old behavior preserved in version history|Stability under change(P18 — component addition/removal not behavior definition replacement)|holds at OpsDB application layer|silo
SP04|Domain Opacity|B2 Behavioral|infrastructure contains no application-domain knowledge; gate pipeline,schema engine,versioning,change management,library suite are domain-agnostic; domain semantics exist only in schema YAML and data rows|same binary infrastructure serves any application|no existing property names this|holds at OpsDB application layer; each AppDB's schema YAML is domain-specific by definition|silo
```

### 4.4 Borderline Properties (documented, not promoted)

```
borderline_properties(id|name|status|reasoning)
BP01|Temporal Consistency|documented|correct time computation under timezone variation,DST,clock skew; could be condition on Determinism(P08); frequent in booking+billing+scheduling; candidate for future promotion
BP02|Fairness|documented|equitable treatment of competing parties under declared rules; product requirement more than system property; specific mechanisms provide it; documented as mechanism-level concern
```

### 4.5 Property Summary

```
property_totals(origin|count|bands)
infra|21|B1(7)+B2(7)+B3(5)+B4(2)
app|3|B1(3)
silo|4|B1(1)+B2(3)
total|28|4 bands (no new bands added)
```

### 4.6 Property Orthogonality

Inherited from infrastructure taxonomy:

```
orthogonality(id|distinction)
OR1|Durability vs Persistence vs Availability — durable-but-unavailable possible; available-but-not-durable possible
OR2|Consistency-data vs Consistency-replica — constraints can hold on each replica while replicas disagree on which value is current
OR3|Ordering vs Isolation — operations can be ordered without isolated and isolated without ordered
OR4|Idempotency vs Determinism — idempotent op can be nondeterministic; deterministic op can be non-idempotent
```

New from application and Silo analysis:

```
OR5|Lifecycle Integrity vs Consistency-data — entity can have valid field values while having arrived through invalid transition sequence
OR6|Computational Correctness vs Consistency-data — derived value can be internally consistent (no constraint violations) while being incorrect relative to its source data
OR7|Vocabulary Closure vs Boundedness — system can be resource-bounded while permitting arbitrary operations; vocabulary-closed while permitting unbounded resource use within closed operations
OR8|Structural Security vs Confidentiality — confidentiality protects data through encryption; structural security prevents attack categories through absence of vocabulary; system can have one without the other
OR9|Non-repudiation vs Auditability — audit trail can reconstruct events without preventing denial if authentication is transferable (shared credentials)
```

---

## 5. Principle Axis

### 5.1 Inherited Principles (infra)

```
inherited_principles(id|name|group|rule|universal_or_class_specific|opsdb_manifestation|origin)
R01|Data primacy|G1 Data/Logic|data outlives logic; make data SoT; treat logic as transformation; configuration in data not code paths|universal|schema is SoT; behavior as data; config as data; policies as data|infra
R02|Single source of truth|G1|every fact has exactly one authoritative location; all others are derived caches|universal|OpsDB for governed state; external authorities for their domains; observation cache is derived|infra
R03|Convention over lookup|G1|when function over data can produce answer prefer to registry that must be queried|universal|naming conventions; FK naming; schema conventions; deterministic identifier construction|infra
R04|0/1/infinity|G2 Scale/Cardinality|three real cardinalities; design for whichever is correct; never design for two|universal|one AppDB per app; N instances for distribution; never 2|infra
R05|Comprehensive over aggregate|G2|slice the whole; do not accrete from parts|universal|slice domain taxonomy whole before populating entities|infra
R06|One way to do each thing|G2|within environment converge on single method per task category|universal|one API path; one library suite; one runner pattern; no shadow tools|infra
R07|Idempotent retry|G3 Failure/Resilience|every operation crossing failure boundary safely retryable|universal|every runner action safely retryable; library handles mechanics|infra
R08|Level-triggered over edge-triggered|G3|react to current state not events; missed events caught next cycle|universal for runners|reconcilers re-evaluate current state each cycle; reactors paired with reconciler backstops|infra
R09|Fail closed|G3|when uncertain deny rather than allow|universal for security/integrity|authorization denials; scope violations; unknown state; gate rejects on validation failure|infra
R10|Fail open|G3|when uncertain continue degraded rather than halt|class-specific (AC22-AC28)|availability-critical hot paths; not for governed state|infra
R11|Bound everything|G3|every queue has max depth; every cache max size; every connection timeout; every retry budget|universal|query bounds; runner bounds; rate limits; retry budgets; schema constraint bounds|infra
R12|Reversible changes|G3|prefer mechanisms allowing rollback|universal|rollback as change_set; version history enables reversal; emergency path exists for break-glass|infra
R13|Minimize dependencies|G4 Dependency/Structure|each new dependency multiplies failure modes and migration costs|universal|runner lib suite minimized; no framework ownership of main loop; no OpsDB runtime dependency for hot path|infra
R14|Separate planes|G4|data-plane+control-plane+management-plane have different SLAs and blast radii|class-specific (AC13-AC28)|hot-path system is data plane; OpsDB is control/management plane; connected via runners|infra
R15|Layer for separation of concerns|G4|each layer does one job with clean interface to next|universal|frontend/API gate/substrate/runners; each layer has defined role|infra
R16|Bucket for locality and accounting|G4|group data by access pattern,tenant,lifecycle,failure domain|class-specific (AC12+AC13+AC16+AC21)|per-tenant bucketing via authorization layers; per-type retention|infra
R17|Local cache + global truth|G5 Distribution|keep updatable local copy; fall through to global when stale|universal|read cache; runner cycle cache; local replica; hot-path cached config|infra
R18|Centralize policy decentralize enforcement|G5|one place defines rules; many places enforce locally|universal|policies in OpsDB data; enforcement at API gate+library suite (two-sided)|infra
R19|Push decision down|G5|make decisions at lowest layer with enough information|class-specific (AC22-AC28)|hot-path decisions local; governed decisions through OpsDB|infra
R20|Push work down/out|G5|edge absorbs traffic; each layer reduces what next must handle|class-specific (AC14+AC26)|CDN; edge cache; content delivery separate from governance|infra
R21|Make state observable|G6 Operator|if you cannot see it you cannot operate it|universal|audit log; version history; runner job records; metrics; structured trace extension|infra
R22|Removing classes of work|G6|goal of automation is making work no longer need to be done|universal|OpsDB removes auth/validation/versioning/audit as classes of work to build per app|infra
```

### 5.2 Application-Specific Principles (app)

```
app_principles(id|name|group|rule|reasoning|distinct_from|opsdb_implementation|origin)
NR01|Separate domain from infrastructure|G4 Dependency|domain logic (billing,eligibility,move validation) isolated from infrastructure logic (retry,auth,serialization,logging); domain changes at domain pace,infrastructure at platform pace|operations IS infrastructure; applications have domain logic that must be separated|R15 Layer for separation(general not domain-specific)|domain logic in runners+policy data; infrastructure in gate pipeline+library suite; runners call libraries,libraries never call runner logic|app
NR02|Express business rules as data|G1 Data/Logic|business rules (pricing,eligibility,approval,lifecycle transitions,scoring weights) expressed as data rows evaluated by mechanical engine not code|business rules more complex than configuration (predicates,conditions,computed outputs); temptation to encode in code stronger|R01 Data primacy(config+state; NR02 extends to business rules)|policy rows for validation; approval rule rows for routing; schedule entities for temporal rules; runner spec data for automation rules; AM02 Rule Engine evaluates|app
NR03|Separate read models from write models|G4 Dependency|when read patterns diverge from write patterns maintain separate read models derived from governed write model; read model is cache; write model is SoT; inconsistency is freshness not correctness|operational systems cache same shape locally; applications denormalize into different shapes for different consumers|R17 Local cache+global truth(locality not shape divergence)|observation cache tables written by runners denormalizing governed entities; search API serves both governed entities and cache tables|app
```

### 5.3 Silo-Derived Principles (silo)

```
silo_principles(id|name|group|rule|reasoning|distinct_from|opsdb_implementation|abstraction_qualifier|origin)
SN01|Security by anatomy at application layer|G3 Failure/Resilience|at application validation layer security derives from structural limitation of what system can express not rules about who can do what|misconfigured policy cannot introduce regex (vocabulary absent); compromised runner cannot write outside scope (both API+library validate)|R09 Fail closed(behavioral policy that can be overridden; SN01 structural — vocabulary to override absent)|closed vocabulary; absence of regex/logic/templating; two-sided scope enforcement|holds within OpsDB abstraction boundary|silo
SN02|Vocabulary restriction at application layer|G2 Scale/Cardinality|set of possible operations finite and fixed at application layer; 9 types,3 modifiers,6 constraints,16 operations,10 gate steps; adding primitive is spec revision not runtime config|system can bound every resource(R11) while permitting arbitrary operations; NR02 prevents arbitrary operations by restricting vocabulary|R11 Bound everything(quantity bounds not vocabulary bounds)|fixed schema vocabulary; fixed API operations; fixed gate steps; fixed runner gating modes; extension is spec revision|holds within OpsDB abstraction boundary|silo
SN03|Shape before meaning at API layer|G3 Failure/Resilience|structural validation precedes semantic interpretation; gate steps 3-4 run before step 5; structurally non-conforming input rejected before system evaluates what it means|addresses injection,type confusion,constraint violation by ensuring input reaches semantic layer only after structural validation|R09 Fail closed(deny when uncertain; SN03 specifically orders structural before semantic)|gate step ordering: schema validation(3)+bound validation(4) before policy evaluation(5); maintained across pipeline extensions|application layer (transport parsing beneath HTTP not controlled)|silo
SN04|Infrastructure fix protects all consumers|G4 Dependency/Structure|shared infrastructure means shared fixes; bug fixed in gate pipeline or schema engine or library suite fixed for every AppDB simultaneously|justifies investment in comprehensive substrate over per-application custom code|R13 Minimize dependencies(reduces failure modes; SN04 addresses benefit of shared fixes)|single codebase for gate pipeline,schema engine,library suite serving all AppDBs|application layer|silo
SN05|Marginal cost of new behavior approaches zero|G6 Operator|1000th entity costs same engineering effort as 10th; 1000th runner same as 10th; no new endpoints,validation,authz,audit per entity|architecture produces zero-cost new behavior through shared infrastructure|R22 Removing classes of work(operational automation; SN05 development efficiency)|new entity=YAML file+loader; new runner=150-300 lines+library calls; full property set immediate|application layer|silo
```

### 5.4 Principle Summary

```
principle_totals(origin|count|groups)
infra|22|G1(3)+G2(3)+G3(6)+G4(4)+G5(4)+G6(2)
app|3|G1(1)+G4(2)
silo|5|G2(1)+G3(2)+G4(1)+G6(1)
total|30|6 groups (no new groups added)
```

### 5.5 Principle Conflicts

Inherited from infrastructure taxonomy:

```
principle_conflicts(id|pair|resolution)
PC1|Fail closed(R09) vs Fail open(R10)|per-domain: fail-closed for security+integrity; fail-open for availability
PC2|Centralize(R18) vs Decentralize(R19)|centralize policy decentralize enforcement is itself resolution
PC3|Comprehensive(R05) vs Aggregate|comprehensive preferred; aggregate as MVP practical reality
```

New from application and Silo analysis:

```
PC4|Express business rules as data(NR02) vs Separate domain from infrastructure(NR01)|rules as data does not mean rules in the gate pipeline; domain rules evaluated by runners or at gate step 5 as policy; infrastructure pipeline unchanged
PC5|Vocabulary restriction(SN02) vs Expressiveness|closed vocabulary limits what can be expressed; cross-field invariants and complex business rules deferred to policy data evaluated at step 5; expressiveness through data composition not vocabulary extension
PC6|Marginal cost approaches zero(SN05) vs Discovery-phase iteration|strict evolution rules impose cost during rapid data model iteration; migration from less rigid system to OpsDB post-discovery is valid path
```

---

## 6. Application Class Profiles

```
app_class_profiles(id|name|position|dominant_mechanisms|critical_properties|governing_principles|key_algorithms)
AC01|Business SaaS (CRM/ERP/HR)|AP01 primary|M13+M44+M43+M24+M20+M07+M11+AM01+AM02|P01+P02+P03+P04+P21+AP01|R01+R02+R05+R06+R07+R08+NR01+NR02|AL01 CRUD+AL02 state machine+AL03 approval routing+AL04 optimistic concurrency+AL07 cursor pagination+AL08 faceted filter
AC02|Internal Business Tools|AP01 primary|M13+M44+M43+M07+M24+M20|P01+P02+P03+P04+P21|R01+R05+R06+NR01|AL01+AL07+AL08+AL15 deadline tracking
AC03|Case Management|AP01 primary|M13+M44+M43+M24+M20+M07+M11+AM01+AM02|P01+P02+P03+P04+P14+P21+AP01|R01+R05+R06+R07+R08+NR01+NR02|AL01+AL02+AL03+AL07+AL09 hierarchy traversal+AL15
AC04|Healthcare Records|AP01 primary|M13+M44+M43+M24+M20+M07+AM01|P01+P02+P03+P04+P05+P06+P07+P21+AP03|R01+R05+R06+R09+NR01+SN01|AL01+AL02+AL07+AL08+AL12 time-range query+AL26 evidence accumulation
AC05|Financial Services Backend|AP02 split|M13+M44+M43+M24+M20+M07+M56+AM01+AM02+AM03|P01+P02+P03+P04+P05+P06+P13+P21+AP02+AP03|R01+R02+R05+R06+R07+R09+R14+NR01+NR02|AL01+AL02+AL03+AL04+AL05 double-entry+AL07+AL24 desired-vs-observed+AL27 external reconciliation
AC06|Compliance Platform|AP01 primary|M13+M44+M43+M24+M20+M07+M38+SM07|P01+P03+P04+P05+P06+P20+P21+AP02|R01+R05+R06+R07+R08+R21+NR01|AL01+AL07+AL12+AL15+AL24+AL26
AC07|Education Platform|AP01 primary|M13+M44+M43+M24+M07+AM01+AM04|P01+P02+P03+P04+P21+AP01|R01+R05+R06+NR01|AL01+AL02+AL07+AL13 cron evaluation+AL14 interval conflict+AL16 recurrence expansion
AC08|Research Data Management|AP02 split|M13+M44+M43+M24+M07+M20|P01+P02+P03+P04+P21|R01+R02+R05+NR01|AL01+AL07+AL08+AL09+AL12+AL26
AC09|Personal Data Platform|AP01 primary|M13+M44+M07+M24|P01+P03+P04+P14|R01+R05+R06+SN05|AL01+AL07+AL08+AL33 authority polling
AC10|Document/Knowledge Mgmt|AP01 primary|M13+M44+M43+M24+M20+M07+SM05|P01+P03+P04+P21+SP03|R01+R05+NR01+NR03|AL01+AL07+AL08+AL12+AL15
AC11|Inventory/Asset Tracking|AP01 primary|M13+M44+M43+M07+M24+AM03|P01+P02+P03+P04+AP02|R01+R05+R06+NR01|AL01+AL07+AL08+AL09+AL24
AC12|Procurement/Vendor Mgmt|AP01 primary|M13+M44+M43+M24+M20+M07+M11+AM01+AM02+AM03|P01+P02+P03+P04+P21+AP01|R01+R05+R06+NR01+NR02|AL01+AL02+AL03+AL07+AL15+AL22 budget tracking+AL30 vendor scoring
AC13|E-commerce|AP02 split|M13+M44+M43+M07+M24+M52+AM01+AM02+AM03+AM04|P01+P02+P03+P04+P11+P13+AP02|R01+R05+R06+R07+R14+NR01+NR02+NR03|AL01+AL02+AL04+AL07+AL14+AL18 proration+AL20 tax+AL27
AC14|Content Management|AP02 split|M13+M44+M43+M24+M07+M57+AM01+SM05|P01+P02+P03+P04+P21+SP03|R01+R05+R06+NR01+NR03|AL01+AL02+AL07+AL08+AL13
AC15|Booking/Scheduling|AP02 split|M13+M44+M43+M07+M52+AM01+AM04|P01+P02+P03+P04+P11+P13+AP01|R01+R05+R06+R07+NR01|AL01+AL04+AL07+AL13+AL14+AL16+AL17 timezone normalization
AC16|IoT Fleet Management|AP02 split|M13+M44+M43+M07+M38+M17|P01+P02+P03+P04+P20|R01+R02+R05+R08+R14+NR01|AL01+AL07+AL08+AL24+AL25 drift detection+AL33+AL35 config push
AC17|Supply Chain/Logistics|AP02 split|M13+M44+M43+M24+M07+M38+AM01|P01+P02+P03+P04+P21+AP01|R01+R02+R05+R06+R07+NR01|AL01+AL02+AL07+AL09+AL15+AL24+AL27
AC18|Workflow/Approval Engine|AP02 split|M13+M44+M43+M24+M07+M11+AM01+AM02|P01+P02+P03+P04+P21+AP01+AP03|R01+R05+R06+NR01+NR02|AL01+AL02+AL03+AL07+AL10 graph walk+AL15+AL31 priority queue
AC19|Turn-based Game|AP02 split|M13+M44+M43+M24+M07+M56+AM01+AM02|P01+P02+P03+P04+P13+AP01|R01+R05+R06+R07+NR01|AL01+AL02+AL04+AL07+AL29 ELO rating
AC20|API Gateway Config|AP02 split|M13+M44+M43+M07+M17+SM05|P01+P03+P04+P18+SP03|R01+R02+R05+R06+R17+SN04|AL01+AL07+AL35
AC21|Subscription/Billing|AP02 split|M13+M44+M43+M24+M07+M11+M56+AM01+AM02+AM03+AM04|P01+P02+P03+P04+P13+P21+AP01+AP02+AP03|R01+R02+R05+R06+R07+R09+R14+NR01+NR02|AL01+AL02+AL03+AL04+AL05+AL07+AL13+AL18+AL19 usage metering+AL21 revenue recognition+AL23 dunning
AC22|Real-time Communication|AP03 wrapper|M01+M02+M03+M52+M60+M61|P03+P10+P11+P16|R10+R11+R13+R14+R19|AL22(external: message routing+presence+delivery receipt)
AC23|Stream Processing|AP03 wrapper|M01+M03+M58+M60+M61|P03+P10+P11+P16|R10+R11+R13+R14|AL42(external: windowed aggregation+exactly-once+watermark)
AC24|Real-time Gaming (FPS/MMO)|AP03 wrapper|M01+M02+M52+M60+M61+M62|P10+P11+P12+P16+P19|R10+R11+R13+R14+R19|AL40(external: physics sim)+AL45(external: spatial partition)
AC25|ML Training/Inference|AP03 wrapper|M07+M17+M58+M50|P03+P10+P11+P12|R11+R13+R14+R21|AL44(external: gradient descent)+AL32(external: recommendation)
AC26|Video Streaming|AP03 wrapper|M01+M02+M17+M58+M50|P10+P11+P12+P17|R11+R13+R14+R20|AL41(external: adaptive bitrate)+AL46(external: content-addressed storage)
AC27|Ad Auction/RTB|AP03 wrapper|M07+M10+M11+M47+M58|P01+P10+P11+P12+P16|R09+R11+R13+R14+R19|AL39(external: auction resolution)+AL19
AC28|High-Frequency Trading|AP03 wrapper|M01+M03+M52+M56+M60+M61|P01+P02+P03+P04+P10+P11+P13+P16+P21|R01+R09+R11+R13+R14+NR01|AL38(external: order matching)+AL05
AC29|Time-Series Database|AP04 metadata|M06+M07+M17+M58+M59|P03+P11+P12+P17|R11+R13+R16|AL(external: columnar compression+downsampling+rollup)
AC30|Search Engine|AP04 metadata|M06+M07+M10+M58|P03+P10+P11+P17|R11+R13+R15|AL(external: inverted index+BM25+tokenization)
AC31|Graph Database|AP04 metadata|M06+M07+M08+M52|P03+P11+P13|R11+R13|AL(external: graph traversal+shortest path+PageRank)
AC32|Object/Blob Storage|AP04 metadata|M06+M09+M17+M28+M30|P03+P11+P12+P17|R11+R13+R16|AL46(external: content-addressed)+AL(external: erasure coding+lifecycle tiering)
AC33|Message Broker|AP04 metadata|M01+M03+M11+M28+M52+M56|P03+P10+P11+P16|R07+R11+R13|AL(external: topic routing+consumer group+offset tracking+compaction)
```

---

## 7. Algorithm Enumeration

### 7.1 State Management Algorithms

```
state_algorithms(id|name|description|mechanisms_composed|properties_contributed|implemented_in|used_by)
AL01|CRUD lifecycle|create-read-update-delete with validation|M13+M44+M43+M18+M24|P01+P02+P03+P04|gate pipeline (steps 3-9)|all classes
AL02|State machine|entity progresses through declared states with validated transitions|AM01+M24+M46|P04+AP01|policy data+gate step 5+version history|AC03+AC13+AC14+AC17+AC18+AC21+AC19
AL03|Approval routing|fan-out to approver groups computed from ownership/stakeholder bridges|M11+M07+M43|P21+AP03|gate step 7+bridge table traversal|AC01-AC12+AC18
AL04|Optimistic concurrency|version stamp comparison at submit; stale detection|M08+M23+M56|P01+P13|gate step 9 (version stamp check)|AC01-AC21
AL05|Double-entry bookkeeping|every value change produces balancing entry|AM03+M24+M02|P02+AP02|runner logic+change_set atomicity|AC05+AC21+AC28
AL06|Soft delete with retention|is_active=false; reaper enforces horizon|M22+M30+M28|P03+P14|reserved fields+reaper runner|all versioned classes
```

### 7.2 Query and Discovery Algorithms

```
query_algorithms(id|name|description|mechanisms_composed|properties_contributed|implemented_in|used_by)
AL07|Cursor pagination|stable ordering+opaque cursor token+seek method|M07+M10+M06|P12+P16|search API|all classes
AL08|Faceted filter|predicate composition across typed fields with AND/OR/NOT|M07+M06+M44|P04+P12|search API predicates|all classes
AL09|Hierarchy traversal|recursive walk of self-FK parent chain|M07+M06|P04|get_dependencies+named join paths|AC03+AC04+AC08+AC11+AC17
AL10|Graph walk (bounded)|relationship traversal with depth limit and cycle detection|M07+M06|P04+P12|get_dependencies|AC03+AC08+AC17+AC18
AL11|Full-text search|tokenization+inverted index+relevance scoring|M06+M10|P11|external search engine; OpsDB holds metadata|AC10+AC14+AC30
AL12|Time-range query|filter by temporal bounds with freshness annotation|M07+M06+M28|P04+P12|search API+observation cache|AC06+AC07+AC16+AC29
```

### 7.3 Scheduling and Temporal Algorithms

```
temporal_algorithms(id|name|description|mechanisms_composed|properties_contributed|implemented_in|used_by)
AL13|Cron evaluation|next-run computation from cron expression|AM04+M40|P10|schedule entity+scheduler runner|AC06+AC07+AC15+AC16+AC21
AL14|Interval conflict detection|overlapping interval identification for booking/scheduling|AM04+M07+M08|P04+AP01|runner logic+search query|AC07+AC15
AL15|Deadline tracking|time-until-due computation+escalation trigger|AM04+M36+M38|P10+P20|schedule entity+verifier runner|AC03+AC06+AC12+AC17
AL16|Recurrence expansion|generate instances from recurrence rule|AM04|P08|runner logic|AC07+AC15
AL17|Timezone normalization|convert and compare across timezones|AM04|P08|runner logic|AC07+AC15+AC22
```

### 7.4 Financial and Metering Algorithms

```
financial_algorithms(id|name|description|mechanisms_composed|properties_contributed|implemented_in|used_by)
AL18|Proration|partial-period charge calculation|AM03+M58|AP02|runner logic|AC13+AC21
AL19|Usage metering|accumulate usage events into billable quantities|AM03+M17+M33|AP02|observation cache+metering runner|AC21+AC27
AL20|Tax calculation|jurisdiction-based tax rate application|AM02+M58|AP02|runner logic+tax policy data|AC13+AC21
AL21|Revenue recognition|allocate revenue across obligations over time|AM03+M58|AP02|runner logic+policy data|AC21
AL22|Budget tracking|committed vs spent vs remaining|AM03+M07|AP02|runner logic+search aggregation|AC12
AL23|Dunning sequence|escalating collection attempts on overdue|AM01+M40+M38|P10+AP01|runner state machine+notification runner|AC21
```

### 7.5 Reconciliation and Verification Algorithms

```
reconciliation_algorithms(id|name|description|mechanisms_composed|properties_contributed|implemented_in|used_by)
AL24|Desired-vs-observed diff|compare governed entities against cached observations|M38+M08+M07+SM02|P09+P20|reconciler runner|AC05+AC06+AC16+AC17
AL25|Drift detection|identify discrepancies without correcting|M38+M08+M07+SM02|P09+P20|drift detector runner|AC06+AC16+AC20
AL26|Evidence accumulation|scheduled verification producing pass/fail records|M38+M32+M20|P20+P21|verifier runner|AC04+AC06
AL27|Reconciliation with external|match OpsDB records against external authority|M38+M08+M07+M04|P04+P09|reconciler runner+external puller|AC05+AC13+AC21
AL28|Integrity verification|hash chain or checksum validation over historical records|M09+M08+M20|P05+P21|audit chain verification tooling|AC04+AC05+AC06
```

### 7.6 Scoring and Ranking Algorithms

```
scoring_algorithms(id|name|description|mechanisms_composed|properties_contributed|implemented_in|used_by)
AL29|ELO/rating computation|update ratings based on match outcomes|AM03+M10+M58|AP02|runner logic|AC19
AL30|Vendor/candidate scoring|multi-criteria weighted score|AM02+SM03+M10|AP02|runner logic+policy data (weights)|AC12
AL31|Priority queue|bounded priority ordering for work items|M10+M41+M07|P10+P12+P16|runner logic+search ordering|AC03+AC18
AL32|Recommendation|collaborative filtering or content-based scoring|AM02+M10+M06|P11|external engine; OpsDB holds feature data|AC09+AC13+AC14
```

### 7.7 Integration Algorithms

```
integration_algorithms(id|name|description|mechanisms_composed|properties_contributed|implemented_in|used_by)
AL33|Authority polling|scheduled read from external API with transformation|M38+M01+M58+SM02|P09+P20|puller runner|all classes with external integrations
AL34|Webhook processing|event receipt+idempotency check+state update|M39+M60+M08|P01+P20|reactor runner|AC13+AC17+AC21+AC22
AL35|Configuration push|format governed state for external system native config|M58+M01+SM05|P09+SP03|config runner|AC16+AC20+AC22-AC28
AL36|Observation pull|read external results+write as OpsDB observations|M38+M01+M58+M17|P09+P20|observation runner|AC05+AC13+AC16+AC22-AC28
AL37|Schema mapping|transform between external data format and OpsDB entity shape|M58+M12+M13|P04+P08|puller runner transformation|all classes with importers
```

### 7.8 Hot-Path Algorithms (External)

```
hotpath_algorithms(id|name|description|used_by|implemented_in|opsdb_role)
AL38|Order matching|price-time priority matching|AC28|matching engine (external)|governs accounts+rules+compliance
AL39|Auction resolution|second-price or other auction|AC27|auction engine (external)|governs campaigns+policies
AL40|Physics simulation|tick-based position/velocity/collision|AC24|game server (external)|governs player data+config
AL41|Adaptive bitrate|quality selection based on bandwidth|AC26|player/CDN (external)|governs content catalog
AL42|Windowed aggregation|time-window grouping with watermark completion|AC23|stream processor (external)|governs pipeline config
AL43|CRDT merge|conflict-free replicated data type convergence|AC22|sync engine (external)|governs user accounts+metadata
AL44|Gradient descent|iterative parameter optimization|AC25|training cluster (external)|governs model registry+experiments
AL45|Spatial partitioning|divide world into regions for efficient query|AC24|game server (external)|governs world config
AL46|Content-addressed storage|hash-based deduplication and integrity|AC32|object store (external)|governs media metadata
```

---

## 8. OpsDB Component to Taxonomy Mapping

```
component_mapping(component|mechanisms|properties|principles)
Schema engine|M13(Schema)+M14(Namespace)+M15(Naming)+M45(Mutator-injection)+SM04(Semantic Repurposing)|P04(Consistency-data)+P08(Determinism)+P18(Stability)+SP01(Vocabulary Closure)+SP04(Domain Opacity)|R01+R03+R05+R06+SN02
Gate step 1 (Auth)|M42(Authenticator)|P06(Authenticity)|R09
Gate step 2 (AuthZ)|M43(Authorizer)|P07(Confidentiality)+AP03(Non-repudiation partial)|R09+R18+SN01
Gate step 3 (Schema validation)|M44(Validator)+SM08(Ingress Validation)|P04(Consistency-data)+SP02(Structural Security)|R09+SN03
Gate step 4 (Bound validation)|M44(Validator)+SM08(Ingress Validation)|P04+P12(Boundedness)+SP02|R09+R11+SN03
Gate step 5 (Policy evaluation)|M46(Filter)+AM01(State Machine Eval)+AM02(Rule Engine)|P04+AP01(Lifecycle Integrity)|R09+R18+NR02
Gate step 6 (Versioning)|M23(Version stamp)+M24(History)|P14(Reversibility)+P21(Auditability)|R12+R21
Gate step 7 (Change mgmt)|M11(Router)+M07(Selector)|P21(Auditability)+AP03(Non-repudiation)|R18+NR02
Gate step 8 (Audit)|M20(Log)+M09(Hasher for chain)|P05(Integrity)+P20(Observability)+P21(Auditability)|R21
Gate step 9 (Execution)|M18(Store)+M52(Lock-advisory)+M56(Sequencer)|P01(Idempotency)+P02(Atomicity)+P03(Durability)+P13(Isolation)|R07
Gate step 10 (Response)|M12(Wrap/unwrap)|P20(Observability)|R21
Change management system|M11(Router)+M07(Selector)+M41(Workqueue)+AM01|P01+P02+P14+P21+AP01+AP03|R01+R07+R12+NR02
Versioning system|M23+M24+M26(Diff)+M27(Reference)|P14+P21+SP03(Hot-Swap Governed)|R01+R12+R21
Audit log|M20(Log)+M09(Hasher)|P05+P20+P21+AP03+SP02|R09+R21+SN01
Search API|M06(Index)+M07(Selector)+M08(Comparator)+M10(Ranker)|P04+P11+P12+P16|R11+R21
Runner pattern|M38(Reconciler)+M39(Reactor)+M40(Scheduler)+SM02(Fact Regen)|P01+P09+P10+P12|R07+R08+R11+R13+NR01
Library suite - API client|M01(Channel)+M60(Retrier)+M08(Comparator)|P01+P06+P20|R06+R07+R13
Library suite - world-side|M01+M60+M61(Circuit breaker)+M62(Bulkhead)|P09+P10+P19|R07+R11+R13+SN01
Library suite - observation|M33(Counter)+M34(Gauge)+M35(Histogram)+M20(Log)|P20+P21|R06+R21
Library suite - notification|M02(Fanout)+M11(Router)|P10+P20|R06+R18
Library suite - templating|M57(Renderer)|P08(Determinism)|R01+NR01
Observation cache|M17(Cache)+M28(TTL)+M03(Funnel)|P11+P17|R02+R17+NR03
Retention system|M30(Reaper)+M28(TTL)+M22(Tombstone)|P12+P18|R11+R16
Report key enforcement|SM08(Ingress Validation)+M44(Validator)+M43(Authorizer)|SP01(Vocabulary Closure)+SP02(Structural Security)|R09+SN01+SN02
```

---

## 9. Abstraction Layer Scoping

```
abstraction_scoping(id|name|axis|full_stack|opsdb_layer|external_hot_path|notes)
P01|Idempotency|property|yes|yes|system-specific|Postgres provides; OpsDB provides; external system provides own
P02|Atomicity|property|yes|yes|system-specific|Postgres transaction; change_set atomicity; external provides own
P03|Durability|property|yes|yes|system-specific|Postgres WAL; replicated if configured
P04|Consistency-data|property|partial|yes|system-specific|Postgres CHECK+FK; OpsDB gate validates; external validates own
P05|Integrity|property|partial|yes|no|TLS in transit; audit chain at OpsDB layer; not claimed for external hot path
P06|Authenticity|property|yes|yes|no|SSO+service accounts at OpsDB layer; hot path has own auth
P07|Confidentiality|property|partial|yes|no|field-level at OpsDB layer; not claimed for external
P08|Determinism|property|partial|yes|no|closed vocabulary at OpsDB layer; Go/Postgres have nondeterministic paths
P09|Convergence|property|no|yes|no|runner discipline; not a stack property
P10|Liveness|property|partial|yes|system-specific|OS process management; runner cycles; external provides own
P11|Availability|property|partial|yes|system-specific|read cache; multi-instance; external provides own
P12|Boundedness|property|partial|yes|no|query+runner bounds at OpsDB layer; OS resources below
P13|Isolation|property|partial|yes|system-specific|optimistic concurrency at OpsDB; Postgres MVCC; external provides own
P14|Reversibility|property|no|yes|no|rollback as change_set; OpsDB-specific mechanism
P15|Consistency-replica|property|yes|delegated|system-specific|Postgres replication; OpsDB trusts engine
P16|Ordering|property|partial|yes|system-specific|version_serial at OpsDB; Postgres sequences; external provides own
P17|Locality|property|partial|yes|no|cache locality at OpsDB layer
P18|Stability under change|property|no|yes|no|schema evolution rules; OpsDB-specific
P19|Failure transparency|property|no|yes|no|runner independence from OpsDB; OpsDB-specific architecture
P20|Observability|property|partial|yes|no|audit log+version history at OpsDB layer; OS metrics below
P21|Auditability|property|no|yes|no|append-only audit log; OpsDB-specific
AP01|Lifecycle Integrity|property|no|yes|no|state machine evaluator at gate step 5
AP02|Computational Correctness|property|no|yes|no|verifier runners at OpsDB layer
AP03|Non-repudiation|property|no|yes|no|SSO+audit chain at OpsDB layer
SP01|Vocabulary Closure|property|no|yes (within boundary)|no|nine types+three modifiers+six constraints; Go/Postgres below not closed
SP02|Structural Security|property|no|yes (within boundary)|no|closed vocabulary at validation layer; transport parsing below not controlled
SP03|Hot-Swap Safety (Governed)|property|no|yes|no|data-driven behavior change via change sets
SP04|Domain Opacity|property|no|yes|no|infrastructure has no domain knowledge
SM01|Execution Funnel|mechanism|no|yes|no|gate pipeline variant
SM02|Fact Regeneration|mechanism|no|yes|no|runner get-phase
SM03|Multiplicative Scoring|mechanism|no|yes (extension)|no|approval scoring extension point
SM04|Semantic Repurposing|mechanism|no|yes|no|AppDB model
SM05|Hot-Swap via Data|mechanism|no|yes|no|data-driven behavior
SM06|Scene as Isolation|mechanism|no|yes|no|AppDB isolation
SM07|Structured Trace|mechanism|no|yes (extension)|no|audit+version history; trace extension point
SM08|Ingress Validation|mechanism|no|yes|no|gate steps 3-4
AM01|State Machine Evaluator|mechanism|no|yes|no|gate step 5+policy data
AM02|Rule Engine|mechanism|no|yes|no|gate step 5+runner-side
AM03|Accumulator|mechanism|no|yes|no|runner computation+observation writes
AM04|Temporal Projection|mechanism|no|yes|no|runner computation from schedule entities
SN01|Security by Anatomy|principle|no|yes (within boundary)|no|closed vocabulary; structural limitation
SN02|Vocabulary Restriction|principle|no|yes (within boundary)|no|fixed primitive set
SN03|Shape Before Meaning|principle|no|yes|no|structural before semantic in gate ordering
SN04|Infra Fix Protects All|principle|no|yes|no|shared codebase
SN05|Marginal Cost Zero|principle|no|yes|no|YAML+loader+library for new entities/runners
NR01|Separate Domain/Infra|principle|no|yes|no|runners for domain; libs+gate for infra
NR02|Business Rules as Data|principle|no|yes|no|policy rows+rule engine
NR03|Separate Read/Write Models|principle|no|yes|no|observation cache as read model
```

---

## 10. Cross-Reference Tables

### 10.1 Mechanism to Property Coverage

```
mechanism_property_coverage(mechanism|primary_properties|secondary_properties)
M13 Schema|P04+P08+P18+SP01+SP04|P12
M44 Validator|P04+SP02|P12
M43 Authorizer|P06+P07+AP03|SP02
M24 History|P14+P21|P20
M20 Log (audit)|P05+P20+P21|AP03
M23 Version stamp|P01+P13+P16|P14
M11 Router|P21|AP01+AP03
M38 Reconciler|P09+P10|P19+P20
M07 Selector|P04+P12|P16
M17 Cache|P11+P17|P12
M18 Store|P01+P02+P03|P04
M52 Lock|P02+P13|P16
M56 Sequencer|P16|P01
M60 Retrier|P01+P10|P11
M61 Circuit breaker|P10+P11+P19|P12
M62 Bulkhead|P19|P11+P12
M30 Reaper|P12+P18|P10
AM01 State Machine Eval|AP01|P04
AM02 Rule Engine|AP01+AP02|P04+P08
AM03 Accumulator|AP02|P04
AM04 Temporal Projection|P08|AP01(conflict detection)
SM01 Execution Funnel|P04+P12|SP02
SM02 Fact Regeneration|P09+P20|P08
SM03 Multiplicative Scoring|P04+P12|AP01
SM04 Semantic Repurposing|SP04|P18
SM05 Hot-Swap|SP03|P09+P18
SM06 Scene Isolation|P07+P13|P19+SP02
SM07 Structured Trace|P20+P21|P14
SM08 Ingress Validation|P04+SP02|SP01
```

### 10.2 Principle to Mechanism Selection

```
principle_mechanism_selection(principle|selects_among|reasoning)
R01 Data primacy|Schema(M13) over code-based validation|data outlives logic; schema is SoT
R06 One way|single Authenticator(M42)+single Authorizer(M43) over per-endpoint|consistency across API
R07 Idempotent retry|Retrier(M60) with Version stamp(M23)|safe retry via optimistic concurrency
R08 Level-triggered|Reconciler(M38) over Reactor(M39) as primary|missed events caught next cycle
R09 Fail closed|Validator(M44) reject over Mutator(M45) repair|deny malformed rather than fix
R11 Bound everything|Limiter(M47)+Quota(M49)+bounded Selector(M07)|explicit resource limits everywhere
R13 Minimize deps|Library suite over per-runner custom code|fewer failure modes
R17 Local cache|Cache(M17)+local replica over global-only|partition tolerance+performance
R18 Centralize policy|Schema(M13)+policy rows over distributed config|one place defines; many enforce
NR01 Separate domain/infra|Runner logic over gate pipeline for domain rules|domain changes at domain pace
NR02 Business rules as data|Rule Engine(AM02)+policy rows over code|rules changeable without deployment
NR03 Separate read/write|Cache(M17) observation tables over governed tables for reads|shape divergence addressed through derivation
SN01 Security by anatomy|SM08 Ingress Validation+closed vocabulary over runtime policy checks|structural impossibility over behavioral policy
SN02 Vocabulary restriction|Schema(M13) closed vocabulary over extensible type system|fixed primitive set prevents arbitrary operations
SN03 Shape before meaning|Validator(M44) at steps 3-4 before Filter(M46) at step 5|structural before semantic in pipeline ordering
```

### 10.3 Impossibility Combinations

Inherited:

```
impossibility_inherited(id|properties|observation|app_relevance)
IT01|Consistency-replica+Availability+Partition tolerance|CAP: under partition CP rejects writes AP accepts|AC22-AC28 must choose
IT02|Consistency-replica+Availability+Latency|PACELC: higher consistency=higher latency|read scaling tradeoff
IT03|Durability+Latency+Throughput|sync durability bounds throughput|observation write volume
IT04|Idempotency+Atomicity+Ordering|all three across distributed parties requires consensus|cross-AppDB operations
IT05|Confidentiality+Observability+Auditability|encrypted-at-rest harder to audit; audit data sensitive|healthcare+financial compliance
IT06|Locality+Stability under change+Availability|rebalancing for locality reduces availability briefly|schema migration windows
IT07|Reversibility+Atomicity(of side effects)+Latency|reversible distributed ops require sagas|external integration rollback
```

New from application and Silo analysis:

```
impossibility_new(id|properties|observation|app_relevance)
IT10|Lifecycle Integrity+Hot-Swap Safety+Immediate Consistency|replacing lifecycle rules while maintaining integrity of in-flight transitions requires atomic replacement|behavior definition changes must be atomic with respect to active state machines
IT11|Vocabulary Closure+Expressiveness+Developer Velocity|closed vocabulary limits what can be expressed; complex rules deferred to policy data evaluated at step 5|tension between structural safety and feature velocity
IT12|Computational Correctness+Availability+Freshness|recomputing derived quantities for verification requires source reads that may be unavailable or stale|eventual verification acceptable; per-write verification expensive
IT13|Non-repudiation+Usability+Availability|non-transferable auth (no shared accounts) and tamper-evident audit increase friction|justified friction for regulated domains; unnecessary for personal scale
IT14|Domain Opacity+Domain-specific Optimization|infrastructure that knows nothing about the domain cannot optimize for domain-specific access patterns|addressed through read model separation(NR03); hot-path systems optimize outside OpsDB
```

---

## 11. Failure Modes

```
failure_modes(id|failure|properties_lost|properties_preserved|app_impact|origin)
FM01|Single process crash|P10 Liveness (briefly)+in-flight P02 Atomicity|P03 Durability+P04 Consistency|API restart; change_set rolled back; runner picks up next cycle|infra
FM02|Postgres primary failure|P10+P11+in-flight writes|P03(with WAL)+P21(audit preserved)|failover required; runner independence preserves hot-path|infra
FM03|Network partition (OpsDB isolated)|P11 Availability for writes+P15 Replica consistency|P03+P19 for runners with cache|runners operate on cached config; hot-path systems unaffected|infra
FM04|Stale derived quantity|AP02 Computational Correctness|P04+P20+P21|verifier detects on next cycle; compliance finding filed|app
FM05|Invalid lifecycle transition|AP01 Lifecycle Integrity|P04(field values valid)+P21|state machine evaluator should have rejected; investigation via audit trail|app
FM06|Policy misconfiguration|depends on policy type — P07 if access policy wrong; AP01 if lifecycle rules wrong|P04+P20+P21|policy change is versioned; prior configuration recoverable; finding filed|app
FM07|Runner scope over-provisioning|SP02 Structural Security weakened at runner layer|SP01+P21|runner has more access than needed; principle of least privilege violated; audit trail shows all actions|silo-app
FM08|Draft mode data loss (inter-version)|P14 Reversibility at per-save granularity+P21 per-save Auditability|P04+P06+P07+committed P14+committed P21|working state between version commits not individually recoverable; committed versions fully protected|app
FM09|Schema evolution cruft|P17 Locality (deprecated fields consume space)|P18 Stability+P04+P21|storage cost; no functional impact; addressed through ignore-deprecated convention|infra
FM10|Change set executor lag|P10 Liveness for approved changes|P01+P02+P03+P21|approved changes wait in queue; no data corruption; latency not correctness|infra
FM11|Report key declaration gap|SP01 Vocabulary Closure for undeclared key|P04+P21|runner's write rejected fail-closed; audit records rejection; investigation finds missing declaration|silo
FM12|Cross-AppDB reference failure|P11 Availability of referenced data|P04(local data intact)+P21|typed pointer unresolvable; application handles gracefully or logs finding|app
FM13|Observation cache staleness|AP02 Computational Correctness for cache-dependent computations|P03+P04(governed data intact)|freshness annotations filter stale data; puller refreshes next cycle|infra
FM14|Concurrent change set conflict|P13 Isolation(second submitter fails)|P01+P04+P21|stale_version error; submitter reconciles and resubmits|infra
FM15|Emergency change without review|AP03 Non-repudiation weakened until review|P05+P20+P21|emergency flag + pending review record; escalation runner monitors overdue|app
```

---

## 12. Governance Flag Impact Analysis

```
flag_impact(flag|P01|P02|P03|P04|P05|P06|P07|P08|P09|P10|P11|P12|P13|P14|P15|P16|P17|P18|P19|P20|P21|AP01|AP02|AP03|SP01|SP02|SP03|SP04)
_autoversion_disabled|0|0|0|0|0|0|0|0|0|0|0|0|0|1|0|0|0|0|0|0|0|0|0|1|0|0|1|0
_edit_latest_version|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|1|0|1|0|0|1|0
_audit_logs_disabled|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|1|2|0|0|2|0|0|0|0
_change_set_bypass (hyp)|0|0|0|0|0|0|0|0|0|0|0|0|0|2|0|0|0|0|0|0|0|2|0|1|0|0|2|0
_audit_log_sampling (hyp)|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|0|1|1|0|0|1|0|0|0|0
```

```
impact_legend: 0=preserved|1=weakened|2=lost
columns: all 28 properties in taxonomy order
rows: three current flags + two hypothetical
```

**Invariant across all flags:** P04 (Consistency-data), P06 (Authenticity), P07 (Confidentiality), P08 (Determinism), SP01 (Vocabulary Closure), SP02 (Structural Security), SP04 (Domain Opacity) are never affected by any governance flag. Validation and authorization always run. Flags only affect recording properties (versioning, audit, change management).

---

## 13. Relationships

```
relationships(from|rel|to|origin)
```

### Mechanism Relationships

```
M13|provides|P04+P08+P18+SP01+SP04|infra+silo
M44|provides|P04+SP02|infra+silo
M43|provides|P06+P07|infra
M24|provides|P14+P21|infra
M20|provides|P05+P20+P21|infra
M23|provides|P01+P13+P16|infra
M38|provides|P09+P10|infra
M17|provides|P11+P17|infra
M18|provides|P01+P02+P03|infra
M52|provides|P02+P13|infra
M56|provides|P16|infra
M60|provides|P01+P10|infra
M61|provides|P10+P11+P19|infra
M62|provides|P19|infra
AM01|provides|AP01|app
AM02|provides|AP01+AP02|app
AM03|provides|AP02|app
AM04|provides|P08|app
SM01|provides|P04+P12+SP02|silo
SM02|provides|P09+P20|silo
SM03|provides|P04+P12|silo
SM04|provides|SP04|silo
SM05|provides|SP03|silo
SM06|provides|P07+P13+P19|silo
SM07|provides|P20+P21|silo
SM08|provides|P04+SP02+SP01|silo
```

### Principle to Mechanism Selection

```
R01|selects|M13 over code-based validation|infra
R06|selects|single M42+M43 over per-endpoint auth|infra
R07|selects|M60 with M23 for safe retry|infra
R08|selects|M38 over M39 as primary control loop|infra
R09|selects|M44 reject over M45 repair|infra
R11|selects|M47+M49+bounded M07|infra
R13|selects|library suite over per-runner custom|infra
R17|selects|M17+local replica over global-only|infra
R18|selects|M13+policy rows over distributed config|infra
NR01|selects|runner logic over gate for domain rules|app
NR02|selects|AM02+policy rows over code|app
NR03|selects|M17 observation tables over governed for reads|app
SN01|selects|SM08+closed vocabulary over runtime policy|silo
SN02|selects|M13 closed vocabulary over extensible types|silo
SN03|selects|M44 at steps 3-4 before M46 at step 5|silo
SN04|selects|shared gate pipeline over per-app custom|silo
SN05|selects|YAML+loader over per-entity custom code|silo
```

### Principle Constraints on Properties

```
R11|constrains_realization_of|P12 (Boundedness must be claimed everywhere)|infra
SN02|constrains_realization_of|SP01 (Vocabulary Closure at application layer)|silo
SN03|constrains_realization_of|SP02 (structural before semantic)|silo
R09|constrains_realization_of|SP02 (fail closed at validation)|infra+silo
NR02|constrains_realization_of|AP01 (lifecycle rules as data enables Lifecycle Integrity)|app
NR03|constrains_realization_of|P11 (read model separation improves Availability)|app
```

### Cross-Axis Triangle

```
triangle(id|aspect|content)
TR1|mechanisms→properties|mechanisms provide properties natively,configurably,or compositionally; AM01 natively provides AP01; SM08 compositionally provides SP02
TR2|properties→mechanisms|properties require mechanisms; AP01 requires AM01+M24; AP02 requires AM03+verifier runner; SP01 requires M13 closed vocabulary
TR3|principles→mechanisms|principles select among mechanisms; NR02 selects AM02 over code; SN01 selects SM08 over runtime policy; R08 selects M38 over M39
TR4|principles→properties|principles constrain property realization; SN02 constrains SP01; R11 constrains P12; NR02 constrains AP01
TR5|the triangle|mechanisms provide properties; principles govern mechanism choice; principles constrain property realization
```

---

## 14. Excluded Items

```
excluded(id|name|origin|reason|alternative)
EX01|Crash Impossibility|silo|requires hardware exception trapping; Go/Postgres have own error handling OpsDB cannot override|Liveness(P10) with runner resilience; graceful degradation
EX02|Errors Are Data Not Faults (principle)|silo|requires control over entire execution stack; Go panics and Postgres errors exist independently|R09 Fail closed+R11 Bound everything; defensive error handling
EX03|Deterministic Heartbeat|silo|requires owning execution model; OpsDB serves concurrent requests through Go goroutines and Postgres transactions|Sequencer(M56) for ordering; optimistic concurrency for isolation
EX04|Vocabulary Closure (full stack)|silo|Go runtime and Postgres execute arbitrary operations below OpsDB boundary|SP01 Vocabulary Closure qualified "application layer"
EX05|Ingress Shim (full geometric security)|silo|requires byte-level shape validation before HTTP/JSON parsing; OpsDB parses JSON first|SM08 Ingress Validation at application layer; partial geometric security
EX06|Temporal Consistency (property)|app|specific condition on Determinism(P08); could be promoted in future|documented as borderline BP01
EX07|Fairness (property)|app|product requirement more than system property; mechanism-level concern|documented as borderline BP02
```

---

## 15. Summary Statistics

```
totals:
  mechanisms: 74 (62 infra + 4 app + 8 silo)
  properties: 28 (21 infra + 3 app + 4 silo)
  principles: 30 (22 infra + 3 app + 5 silo)
  mechanism_families: 13 (no new families)
  property_bands: 4 (no new bands)
  principle_groups: 6 (no new groups)
  application_classes: 33
  algorithms: 46
  opsdb_components_mapped: 25
  abstraction_layers: 3 (full stack + OpsDB layer + external hot path)
  governance_flags_analyzed: 5 (3 current + 2 hypothetical)
  orthogonality_pairs: 9 (4 inherited + 5 new)
  impossibility_combinations: 12 (7 inherited + 5 new)
  failure_modes: 15
  excluded_items: 7 (5 ring-0 silo + 2 borderline properties)
  cross_references: mechanism→property coverage, principle→mechanism selection, principle→property constraint, component→taxonomy mapping, abstraction layer scoping
```

---

## Appendix A: Origin Tag Reference

```
origin_tags:
  infra: inherited from infrastructure taxonomy (OPSDB-9); operational infrastructure management origin; 62 mechanisms + 21 properties + 22 principles
  app: discovered through application domain analysis; patterns specific to application development that operational infrastructure does not require; 4 mechanisms + 3 properties + 3 principles
  silo: discovered through data-only execution architecture analysis (Silo COMP 1-5); filtered to patterns operating at OpsDB application layer; abstraction-qualified; 8 mechanisms + 4 properties + 5 principles
```

```
abstraction_qualifiers:
  full_stack: holds from hardware through application; items that storage engine and OS provide
  application_layer: holds within OpsDB abstraction boundary (schema engine, gate pipeline, change mgmt, versioning, audit, runners, library suite); does not hold below (Go runtime, Postgres, OS, network transport)
  external_hot_path: holds within specialized systems connected via runners; OpsDB does not claim; hot-path system claims own properties
```

```
decode_legend:
  mechanism_ids: M01-M64 (infra) + AM01-AM04 (app) + SM01-SM08 (silo)
  property_ids: P01-P21 (infra) + AP01-AP03 (app) + SP01-SP04 (silo)
  principle_ids: R01-R22 (infra) + NR01-NR03 (app) + SN01-SN05 (silo)
  app_class_ids: AC01-AC33
  algorithm_ids: AL01-AL46
  position_ids: AP01-AP04 (architecture positions)
  failure_mode_ids: FM01-FM15
  impossibility_ids: IT01-IT07 (inherited) + IT10-IT14 (new)
  orthogonality_ids: OR1-OR4 (inherited) + OR5-OR9 (new)
  excluded_ids: EX01-EX07
  borderline_ids: BP01-BP02
  flag_impact_values: 0=preserved | 1=weakened | 2=lost
  rel_types: provides | requires | selects | constrains_realization_of | grounds | enables | prevents | implements | composes
```
