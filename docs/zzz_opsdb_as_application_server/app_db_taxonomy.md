# AppDB Application Taxonomy

## A Technical Reference for Governed Application Development

---

## 1. What This Document Is

This taxonomy names every building block, guarantee, and design rule relevant to building applications on a governed data substrate. The substrate — OpsDB Application Architecture — validates every write against a declared schema, enforces access control, manages change approval workflows, versions every governed entity with full state history, and logs every action to an append-only audit trail.

The taxonomy uses three axes. **Mechanisms** are the building blocks that do work. **Properties** are the guarantees claimed about how that work behaves. **Principles** are the rules that govern which building blocks are chosen and how they fit together. These are different kinds of things. The same word often names all three — "durability" can refer to a mechanism that persists data, a guarantee that data survives failure, or a rule about preferring durable writes. Precision requires saying which one you mean.

Every item has an origin tag. Items tagged `infra` come from operational infrastructure — managing servers, networks, and production environments. Items tagged `app` come from application development — building software products for users. Items tagged `silo` come from data-only execution architecture — systems where the compiled binary contains no application knowledge and all behavior is expressed as data. All three sources contribute because applications built on this substrate inherit operational rigor, serve application domains, and benefit from data-driven execution patterns.

Some items carry an abstraction qualifier: "application layer." This means the guarantee holds within the OpsDB boundary — the schema engine, the API pipeline, the change management system, the versioning system, the audit log, and the runner framework. It does not necessarily hold below that boundary, in the Go runtime, the Postgres database engine, or the operating system. This scoping is honest. OpsDB is a userspace application, not an operating system.

---

## 2. Mechanisms

Mechanisms are the building blocks. Each one takes inputs, produces outputs, and may have side effects. They are identified by what they do, not how they are implemented. Multiple implementations of the same mechanism can coexist.

### 2.1 Information Movement

These mechanisms cause data to travel between points.

**Channel** — a path along which data travels between two endpoints. Every API call traverses a channel. Channels can be persistent or ephemeral, reliable or best-effort, ordered or unordered.

**Fanout** — one source sends to many destinations. The notification system uses fanout when dispatching alerts to multiple recipients through multiple channels simultaneously.

**Funnel** — many sources send to one collector. When multiple runner processes write observations to the same cache tables, those writes funnel into a shared destination.

**Replicator** — copies data from one location to another continuously or on demand. Database replication copies the primary database to read replicas. Runners copying external system state into observation cache tables are a form of replication.

**Relay** — receives data on one channel and emits it on another, possibly transforming it. The API acts as a relay between frontend applications and the database, mediating every interaction.

### 2.2 Selection

These mechanisms choose among candidates.

**Index** — a data structure that accelerates lookup by mapping keys to locations. Every database table has indexes declared in the schema. The search API's performance depends on these indexes.

**Selector** — given a population, returns the subset matching a predicate. Every search query is a selector invocation — filter by entity type, by field values, by time range, by access scope.

**Comparator** — given two things, determines whether they are equal, ordered, or different. Version stamp comparison at change set submission uses a comparator to detect whether the entity has been modified since the submitter last read it.

**Hasher** — a deterministic function that produces a bounded output from any input. The audit log's tamper-evidence chain uses a cryptographic hasher to link each entry to the previous one.

**Ranker** — given candidates, orders them by score. Search results ordered by field and direction use a ranker. Approval rule matching uses ranking when multiple rules apply.

**Router** — given an input, picks one output path. Change management approval routing is a router: given a change set touching specific entities, it routes to the appropriate approver groups.

**Multiplicative Scoring with Zero-Gating** `silo` — a specific scoring method where each factor multiplies into a running score starting at 1.0. Any factor scoring zero immediately zeros the entire score, providing fail-fast elimination. This is more expressive than simple ranking because a single disqualifying factor outweighs all other scores. An extension point for approval systems where multiple considerations compose multiplicatively.

### 2.3 Representation

These mechanisms determine how things are expressed.

**Wrap/Unwrap** — encapsulates data in a layer that adds context, structure, or encoding. JSON serialization wraps application data for transport over HTTP. The discriminator pattern wraps heterogeneous payloads with a type indicator that selects the appropriate validation schema.

**Schema** — a description of what data is permitted. The schema engine is the primary representation mechanism: YAML files declare entity types, field types, constraints, and relationships. The API validates every write against the runtime schema metadata.

**Namespace** — a scope within which names are unique. Entity type names and field names exist within the schema namespace. Naming conventions enforce uniqueness and disambiguation.

**Naming Convention** — a function that constructs identifiers from component data deterministically. Foreign keys are named `referenced_entity_id`. Datetime fields are suffixed `_time`. Boolean fields are prefixed `is_` or `was_`. These conventions are enforced by the schema loader.

**Semantic Repurposing** `silo` — the same structural schema serves multiple unrelated domains by changing only the data content. The API pipeline, versioning system, change management, and audit log have no knowledge of what domain the application serves. A billing application and a healthcare application and a recipe tracker all use the same infrastructure with different schema YAML files. The infrastructure is domain-agnostic. Meaning is in the data, not in the code.

### 2.4 Storage

These mechanisms hold data over time.

**Buffer** — an in-memory holder of bounded size. API request processing uses buffers during pipeline evaluation.

**Cache** — a fast, capacity-bounded copy of data whose source of truth is elsewhere. The tenant-aware read cache between the API and the database accelerates common queries. Observation cache tables hold copies of external system state written by runner processes. If a cache is lost, it can be rebuilt from its source.

**Store** — a durable holder that is the source of truth. The Postgres database is the primary store for all governed entities. If data is lost from the store, it is lost — unlike a cache, a store is not rebuildable from elsewhere.

**Journal** — an append-only sequential record of changes suitable for replay. The Postgres write-ahead log is a journal that enables crash recovery by replaying committed but not yet flushed changes.

**Log** — an append-only record of events without replay as its purpose. The audit log is a log: it records every API action for inspection and audit, but it is not designed to be replayed to reconstruct database state.

**Snapshot** — a point-in-time copy of state. Database backups produced by the backup runner are snapshots. A snapshot is frozen — it represents state at one moment, not live state.

**Tombstone** — a marker indicating that something was deleted, distinct from absence. Soft-deleted entities (with `is_active` set to false) are tombstones. The entity persists in the database for history and audit purposes even though it is logically deleted.

### 2.5 Versioning

These mechanisms preserve and manage state lineage.

**Version Stamp** — a monotonic identifier attached to a state. Each version of a governed entity carries a `version_serial` number that increases with every change. Version stamps enable optimistic concurrency: the system compares the stamp you drafted against with the current stamp to detect conflicts.

**History** — an ordered sequence of versions with relationships preserved. Versioning sibling tables store the full history of every governed entity. Each version row contains all field values (not just the ones that changed), linked to the previous version and to the change set that produced it.

**Merge Algorithm** — given two divergent histories, produces a unified result. OpsDB does not use merge algorithms because optimistic concurrency prevents divergence: conflicts are detected and rejected at submission rather than merged after the fact.

**Diff** — a structural description of the change between two versions. Each `change_set_field_change` record is a diff: it identifies the entity, the field, the value before, and the value after.

**Reference** — a named pointer to a version. The `is_current` flag on schema version rows is a reference indicating which version is active. Authority pointers are references to where external facts live.

### 2.6 Lifecycle

These mechanisms bound a thing's extent in time.

**TTL (Time To Live)** — a time after which something is considered expired. The `max_staleness_seconds` parameter on observation cache queries is a TTL: cached data older than the threshold is filtered out.

**Lease** — a time-bounded grant of authority. Authentication tokens issued by identity providers and secret backends have bounded lifetimes. When the lease expires, re-authentication is required.

**Reaper** — a process that removes expired things. The reaper runner reads retention policy data and removes or soft-deletes entities past their configured retention horizon.

**Drainer** — a process that gracefully removes something from active service before destruction. When a runner shuts down, it completes its current cycle before stopping, draining in-progress work.

### 2.7 Sensing

These mechanisms produce data about state.

**Probe** — a synthetic test of liveness, health, or correctness. Health checks on the API deployment are probes: they ask "are you alive?" and "are you ready to serve requests?"

**Counter** — a monotonic numeric tally. Runner metrics track operation counts. The observation library emits counters for API calls, cache hits, and validation rejections.

**Gauge** — a current numeric value. Queue depth, cache hit rate, and active connection count are gauges: they report the current value, not an accumulation.

**Histogram** — a bucketed distribution of values over time. API response time distributions and runner cycle time distributions are histograms: they show the shape of a quantity's variation.

**Watch** — a subscription to changes, delivered as a push stream. The streaming API operation lets consumers subscribe to entity changes with a resume token. On reconnect, the system fetches current state then streams from the token, ensuring missed changes are caught.

**Heartbeat** — a periodic still-alive emission. Runner job records serve as implicit heartbeats: a runner that is alive produces job records on its cycle schedule. A runner that stops producing them is considered unhealthy.

**Fact Regeneration** `silo` — converting the current state of entities into a declarative fact set on every evaluation cycle, so that decisions always use current data rather than stale cached state. Runners implement this in their get phase: each cycle re-reads current state from the API rather than relying on data from a prior cycle. The discipline that runners hold no state across cycles is a form of fact regeneration.

**Structured Trace** `silo` — capturing the complete decision chain for an operation in domain-aware terms, with the ability to replay with modified rules and compare outcomes. The audit log and version history provide the data substrate. An extension point exists for per-change-set decision tracing that captures which validation rules fired, which approval policies matched, and what the gate pipeline evaluation produced, with replay capability for "what if the policy had been different."

### 2.8 Control Loop

These mechanisms observe state and act on it.

**Reconciler** — observes desired state versus actual state, takes action to close the gap, and repeats. Reconciler runners compare governed entities against cached observations and either correct discrepancies or propose change sets. The reconciler pattern is level-triggered: it reacts to current state, not to events. If an event is missed, the next cycle catches the discrepancy.

**Reactor** — receives events and runs handlers, firing once per event. Reactor runners process webhooks and watch streams. Because reactors are edge-triggered (they react to events, not state), they are paired with reconciler backstops that catch anything the reactor misses.

**Scheduler** — assigns work to time slots by policy. The scheduler runner kind enforces runner schedules on target infrastructure, ensuring cron entries and Kubernetes CronJobs match what the schedule entities declare.

**Workqueue** — a buffered, deduplicated, retry-aware task list. Approved change sets waiting for the change-set executor form a workqueue: the executor reads the queue, applies each change set, and marks it completed.

**Hot-Swap via Data Replacement** `silo` — replacing behavior definitions at runtime through data changes rather than code deployment. In OpsDB, approval rules, validation policies, access control configurations, notification routing, and runner specifications are all data rows. Changing them through the change set pipeline takes effect when the change set is applied. No restart or redeployment is needed. The governance layer wraps the replacement with attribution, approval, versioning, and audit.

### 2.9 Gating

These mechanisms decide what is permitted.

**Authenticator** — verifies that a caller is who they claim to be. Gate pipeline step 1 delegates authentication to an identity provider for humans (via SSO) or validates service account credentials for runners (via a secret backend). The API does not store passwords.

**Authorizer** — checks whether a verified caller has permission to perform a specific action on a specific target. Gate pipeline step 2 evaluates five authorization layers: role and group membership, per-entity governance fields, per-field access classification, per-runner scope declarations, and policy rules.

**Validator** — checks whether a request meets declared schema and constraint rules. Gate pipeline steps 3 and 4 validate that every field in a write request exists in the schema, has the correct type, and falls within declared bounds (numeric ranges, string lengths, enum values, foreign key existence).

**Mutator** — modifies a request before acceptance. The schema loader injects reserved fields (id, created\_time, updated\_time) into every entity. The API sets timestamps on writes. These are mutations: the system modifies the data before storing it.

**Filter** — accepts, drops, or modifies data based on rules. Policy evaluation at gate step 5 filters operations against additional constraints: data classification rules, separation of duty, time-of-day restrictions.

**Limiter** — bounds rate or concurrency. Rate limiting per identity and per endpoint prevents any single caller from overwhelming the API.

**State Machine Evaluator** `app` — takes a declared set of states, a declared set of valid transitions, the current state, and a proposed transition, and accepts or rejects the transition based on the lifecycle graph. Applications have entities with lifecycles where the path through states matters: an invoice cannot go from "paid" to "draft" without reopening, a support case cannot go from "closed" to "in progress" without explicit reopen. This is different from field validation (which checks values) and from authorization (which checks permissions). It validates that the proposed state transition is legal in the declared lifecycle.

**Rule Engine** `app` — evaluates a population of declared rules against input data and produces a decision, a matching set, or a computed value. Business rules — pricing logic, eligibility criteria, approval requirements, fraud scoring — are expressed as data rows. The rule engine evaluates them mechanically. This is different from validation (which checks against schema bounds) and from selection (which finds matching data given a rule). The rule engine finds matching rules given data and computes their output.

**Ingress Validation (Partial Geometric Security)** `silo` — structural validation that rejects non-conforming requests before any semantic interpretation occurs. The closed schema vocabulary prevents injection of new operation types. Fixed field types prevent type confusion. No regex is evaluated. No embedded logic is executed. At the application layer, input never reaches the semantic evaluation step unless it has passed structural validation first. This is the deployable subset of geometric security for systems running on standard technology stacks.

### 2.10 Allocation

These mechanisms decide how much of what goes where.

**Pool** — a collection of equivalent resources available for checkout and return. Database connection pools manage connections between the API and Postgres.

**Quota** — a maximum amount of a resource that a party may consume. Per-tenant query limits, configurable per role as policy data, are quotas.

**Scheduler (Allocation)** — assigns work to resources by placement policy. Runner machine capacity declarations determine how many concurrent jobs each machine can handle.

**Sharder** — partitions workload across resources. Schema-declarative sharding for high-frequency entities is a potential future extension, where the API reads the sharding scheme and routes queries while consumers see merged results.

### 2.11 Coordination

These mechanisms synchronize multiple parties.

**Lock** — exclusive access to a resource. Advisory locks prevent concurrent schema applications: only one schema change can execute at a time.

**Election** — a protocol that selects one among many. Not directly used by OpsDB; Postgres handles leader selection for replication internally.

**Barrier** — a synchronization point where participants wait until all have arrived. Bulk change set chunk boundaries act as barriers: all chunks must validate before any transition.

**Quorum** — a rule requiring N-of-M parties to agree. Not directly used by OpsDB at the application layer; Postgres replication quorum is configured at the storage engine level.

**Sequencer** — assigns monotonic positions. Version serial numbers are assigned by a sequencer: each version of an entity gets the next number in a monotonic sequence.

**Scene as Isolation Boundary** `silo` — a complete execution context with its own data, behavior definitions, and access control. Each AppDB is a scene: it has its own schema, its own API deployment, its own database, its own runners, and its own policies. Access between AppDBs is controlled by typed pointers with policy-mediated federation. Default is no cross-access. Within a single AppDB, the five authorization layers provide scene-like isolation at the entity level.

### 2.12 Transformation

These mechanisms produce new data from input data.

**Renderer** — produces output by combining a template with data. The templating library renders configuration files and reports through variable substitution and file inclusion. Templates are deliberately limited: no expressions, no conditionals, no function calls. Logic that would go into templates goes into runners that produce concrete values.

**Transformer** — a function from input to output. Runner transformation logic converts external system data into OpsDB entity shapes. Diff computation transforms two states into a change description.

**Compactor** — merges multiple inputs into a smaller equivalent output. Postgres vacuum reclaims storage from deleted and updated rows.

**Accumulator** `app` — maintains a running aggregate that is updated incrementally as data arrives, without re-reading the entire dataset. Account balances, inventory levels, budget remaining, usage meters, and revenue recognized to date are accumulators. Unlike a counter (which only observes), an accumulator participates in the write path and persists a derived quantity that must remain consistent with its source data.

**Temporal Projection** `app` — expands a compact schedule definition into concrete datetime instances over a time range. Given a cron expression or a recurrence rule, it produces the list of specific dates and times when something should happen. This enables display ("show me the next ten occurrences"), conflict detection ("does this booking overlap with an existing one"), and availability computation ("which time slots are open").

### 2.13 Resilience

These mechanisms handle failure.

**Retrier** — re-runs a failed operation with backoff and jitter. The API client library retries transient failures. The coordination library provides configurable retry policies with exponential backoff.

**Circuit Breaker** — stops calling a failing dependency and periodically tests for recovery. The library suite provides per-target circuit breakers that prevent cascading failure when an external system is down.

**Bulkhead** — isolates failure domains so one failing component does not drag others down. Each AppDB is an isolated failure domain: a problem in one AppDB does not affect others.

**Hedger** — issues redundant requests to reduce tail latency. Available in the library suite for latency-sensitive runners, though not commonly used.

**Failover** — switches from a primary to a standby on failure detection. The failover handler runner kind detects primary failure, performs failover, and updates OpsDB with the new topology.

### 2.14 Mechanism Execution Funnel (silo)

**Execution Funnel** — a pipeline that composes multiple different types of mechanisms into a monotonic narrowing sequence. Each layer uses a fundamentally different computational approach, but they chain together so that each layer's output becomes the next layer's input and the candidate set only shrinks. The OpsDB gate pipeline is a variant: ten steps, each of which can reject, narrowing from "any request" to "executed write." A richer variant would compose structural validation, predicate logic evaluation, utility scoring, and atomic execution as distinct computational layers rather than variations of the same check pattern.

---

## 3. Properties

Properties are guarantees that hold over operations. Each property has conditions under which it holds and conditions under which it degrades. Properties are organized into four bands.

### 3.1 Data Integrity

These properties claim that data is correct, survives failure, and is protected.

**Idempotency** `infra` — applying the same operation with the same inputs more than once produces the same end state as applying it once. In OpsDB, change sets commit atomically and version stamps prevent re-application. Runner actions are designed to be safely retryable.

**Atomicity** `infra` — a defined unit of work either fully completes or has no externally visible effect. All field changes within a change set commit together or none commit. There is no partial application visible to other consumers.

**Durability** `infra` — data acknowledged as committed survives the failure modes specified. Postgres provides durability through its write-ahead log and optional replication.

**Consistency-data** `infra` — declared constraints and invariants hold at specified boundaries. The schema engine declares field types and bounds. The gate pipeline validates every write against these declarations. Foreign key existence is checked. Enum values are verified against the declared set.

**Integrity** `infra` — data has not been altered without detection between defined points. The audit log's optional cryptographic chain hashes each entry over its own contents plus the previous entry's hash. TLS protects data in transit.

**Authenticity** `infra` — the source of a request is verifiable. SSO delegation verifies human identity through an identity provider. Service account credentials verify runner identity through a secret backend.

**Confidentiality** `infra` — data is unreadable to unauthorized parties. The `_access_classification` governance field and the five-layer authorization model enforce field-level confidentiality. Fields the caller cannot access are omitted from responses.

**Lifecycle Integrity** `app` — an entity's state transitions conform to a declared lifecycle graph at all times. No entity can be in a state that was not reached through a valid transition sequence. This is different from consistency-data (which checks field values) because an entity can have valid field values while having arrived at its current state through an invalid transition. An invoice marked "paid" might have valid fields but might not have gone through the required "approved" state first.

**Computational Correctness** `app` — derived quantities (account balances, inventory levels, billing totals, scores) are consistent with their source data at specified boundaries. The persisted balance equals what you would get if you summed all transactions from scratch. Verifier runners periodically recompute derived quantities and compare them against stored values, producing evidence records on match or compliance findings on mismatch.

**Non-repudiation** `app` — a party who performed an action cannot deny having performed it. This requires that authentication is non-transferable (no shared accounts), the audit trail is tamper-evident, and cryptographic evidence is preserved. It is stronger than authenticity (which verifies identity at request time) and auditability (which can reconstruct events) because it claims the evidence is sufficient to prevent denial in a legal or regulatory context.

**Structural Security (Application Layer)** `silo` — at the application validation layer, security properties derive from the absence of mechanism rather than from policy enforcement. The closed schema vocabulary means that injection of new operation types cannot be expressed — the vocabulary does not contain the words for it. The absence of regex means catastrophic backtracking cannot occur. The absence of embedded logic means arbitrary code execution through schema files cannot occur. These are structural impossibilities, not policies that could be misconfigured. This property holds within the OpsDB abstraction boundary; it does not hold at the transport parsing layer beneath HTTP.

### 3.2 Behavioral

These properties claim how operations behave at runtime.

**Determinism** `infra` — the same inputs produce the same outputs and the same side effects. The closed constraint vocabulary (no regex, no embedded logic) makes validation deterministic and bounded.

**Convergence** `infra` — repeated application of an operation drives state toward a fixed point. Level-triggered runners re-evaluate current state each cycle. If the state matches the desired state, no action is taken. If it does not, corrective action is taken. Over repeated cycles, the system converges.

**Liveness** `infra` — the system continues to make progress under specified conditions. Runner cycles continue executing. The change-set executor drains its queue. The reaper enforces retention. Progress is bounded and observable.

**Availability** `infra` — the system responds within a specified timeframe. The read cache, multiple API instances, and runner independence from OpsDB availability contribute to availability. Hot-path systems connected via runners continue operating on cached configuration even when OpsDB is unavailable.

**Boundedness** `infra` — resource consumption stays within specified limits. Every query has a maximum result size, maximum join depth, and maximum execution time. Every runner has a retry budget, execution time limit, and scope-per-cycle bound. Rate limits cap per-identity and per-endpoint request rates.

**Isolation** `infra` — concurrent operations do not visibly interfere with each other. Optimistic concurrency control detects conflicts at change set submission. Change sets commit atomically. One user's write cannot silently overwrite another's.

**Reversibility** `infra` — a completed operation can be undone. Rollback is a change set that proposes field changes restoring the values from a prior version. It goes through the same validation, approval, and audit pipeline as any other change. The rollback itself becomes a versioned event in the history.

**Vocabulary Closure (Application Layer)** `silo` — the set of possible operations at the application layer is a closed finite set that cannot be extended at runtime. The schema has nine types, three modifiers, and six constraints. The API has sixteen operations. The gate pipeline has ten steps. Runner gating has three modes. No schema file, policy row, or runner configuration can introduce new operation types. Adding a new primitive requires revising the specification, not changing a runtime configuration. This property holds within the OpsDB abstraction boundary; the Go runtime and Postgres beneath it can execute arbitrary operations.

**Hot-Swap Safety (Governed)** `silo` — behavior definitions can be replaced at runtime without interrupting processing, without corrupting state, and with full governance around the replacement. Approval rules, validation policies, access control, notification routing, and runner configurations are all data rows changeable through the change set pipeline. When a change set is applied, the new behavior takes effect immediately. The old behavior is preserved in version history. The replacement is attributed, approved, versioned, and auditable.

**Domain Opacity** `silo` — the infrastructure contains no knowledge of the application domain it serves. The gate pipeline does not know whether it is validating an invoice or a patient record. The schema engine does not know whether it is generating tables for a billing system or a recipe tracker. Domain semantics exist only in schema YAML files and data rows loaded at runtime. The same infrastructure binary serves any application.

### 3.3 Distribution

These properties arise specifically from systems that span multiple nodes or serve multiple parties.

**Consistency-replica** `infra` — a read after a write returns the written value or a value satisfying a specified relationship (linearizable, sequential, causal, eventual). The level of consistency depends on the storage engine's replication configuration.

**Ordering** `infra` — operations apply or appear in a specified order. Change set field changes have an explicit apply order. Version serial numbers are monotonically assigned.

**Locality** `infra` — related data sits close together in storage or access terms. Observation cache tables provide locality for frequently accessed external state. The read cache provides locality for common queries.

**Stability under Change** `infra` — adding or removing components does not cause disproportionate disruption. The strict schema evolution rules — additive only, no deletions, no renames — ensure that schema changes never break existing consumers.

**Failure Transparency** `infra` — failures of specified components are hidden from clients up to specified bounds. Runner independence from OpsDB availability means that hot-path systems connected via runners continue operating on cached configuration even when OpsDB is temporarily unavailable.

### 3.4 Operational

These properties concern visibility and reconstruction.

**Observability** `infra` — relevant state is queryable or subscribable through specified interfaces. The audit log, version history, runner job records, and metrics emission make the system's state visible to operators and tools.

**Auditability** `infra` — past operations are reconstructible from preserved records. The append-only audit log, the version chain on every governed entity, and the change set trail with approval records make it possible to reconstruct what happened, who did it, who approved it, and what the state was at any point in time.

### 3.5 Property Orthogonality

Several properties that seem related are actually independent. Understanding where they are independent prevents design errors.

Durability and availability are independent. Data can be durable (safely stored) but unavailable (the database is down and no one can read it). Data can be available (readable from cache) but not durable (the cache survives a restart but the primary store does not).

Consistency-data and consistency-replica are independent. Every replica can enforce all declared constraints (consistency-data) while replicas disagree about which value is current (consistency-replica violated).

Lifecycle integrity and consistency-data are independent. An entity can have valid field values (all constraints pass) while having arrived at its current state through an invalid transition sequence (lifecycle integrity violated).

Computational correctness and consistency-data are independent. A derived value can satisfy all schema constraints (non-negative, correct type, within range) while being incorrect relative to its source data (the balance does not match the sum of transactions).

Vocabulary closure and boundedness are independent. A system can bound every resource (memory, CPU, connections) while permitting arbitrary operations within those bounds. A system can restrict its operation vocabulary to a closed set while permitting unbounded resource use within those operations.

Non-repudiation and auditability are independent. An audit trail can reconstruct events without preventing denial if authentication is transferable (shared credentials allow one person to act as another).

---

## 4. Principles

Principles are rules that govern which mechanisms are chosen and how they are assembled. They do not perform work or make guarantees about specific operations.

### 4.1 Data and Logic

**Data Primacy** `infra` — data outlives logic. Make data the source of truth. Treat logic as a replaceable transformation. Configuration belongs in data rows, not code paths. The schema is the long-lived artifact. Code is the shell around it.

**Single Source of Truth** `infra` — every fact has exactly one authoritative location. All others are derived caches. OpsDB is the source of truth for governed state. External systems (cloud providers, identity providers, monitoring systems) are the source of truth for their domains. Observation cache tables are derived copies, not authorities.

**Convention over Lookup** `infra` — when a deterministic function over data can produce an answer, prefer it to a registry that must be queried. Foreign keys are named `referenced_entity_id` by convention. Datetime fields are suffixed `_time` by convention. These conventions produce predictable names without requiring a lookup.

**Express Business Rules as Data** `app` — business rules (pricing logic, eligibility criteria, approval requirements, lifecycle transitions, scoring weights) should be expressed as data rows evaluated by a mechanical engine, not as code. This extends data primacy from configuration to business rules. The rule engine mechanism evaluates policy rows. Changing business rules means changing data, not deploying code.

### 4.2 Scale and Cardinality

**Zero, One, or N** `infra` — there are only three real cardinalities. Design for whichever is correct. Never design for two, because organizational pressure will push past it. One AppDB per application. N instances when distributing for structural reasons (legal, regulatory, organizational). Never two instances of the same application.

**Comprehensive over Aggregate** `infra` — slice the whole domain before populating it, rather than accreting entities from the bottom up. An aggregate system grows piece by piece with no plan for the whole and reaches internal inconsistency. A comprehensive system starts from the whole, subdivides, and maintains coherence.

**One Way to Do Each Thing** `infra` — within an environment, converge on a single canonical method per task category. One API path. One library suite. One runner pattern. No shadow tools, no second API with different rules, no alternative write path that bypasses the gate pipeline.

**Vocabulary Restriction at the Application Layer** `silo` — the set of possible operations is finite and fixed. Nine schema types. Three modifiers. Six constraints. Sixteen API operations. Ten gate steps. Adding a new primitive requires revising the specification. This extends boundedness from resource limits to vocabulary limits: not just "how much" is bounded but "what" is bounded.

### 4.3 Failure and Resilience

**Idempotent Retry** `infra` — every operation crossing a failure boundary should be safely retryable. Failures are inevitable. Retries are the standard recovery mechanism. Non-idempotent operations force a choice between data loss (no retry) and duplication (blind retry). Idempotent operations avoid both.

**Level-Triggered over Edge-Triggered** `infra` — react to current state, not to event streams. Events can be missed (dropped messages, failed webhooks, crashed consumers). State is always available for comparison. A reconciler that checks current state each cycle catches discrepancies regardless of how they arose. An event-triggered reactor that misses an event misses the work forever unless a reconciler backstop catches it.

**Fail Closed** `infra` — when uncertain, deny rather than allow. The gate pipeline rejects requests that fail validation. Authorization denials halt the pipeline. Runners that cannot verify their scope refuse to act. In security and integrity contexts, the cost of a false rejection (a failed operation) is less than the cost of a false acceptance (a breach or corruption).

**Fail Open** `infra` — when uncertain, continue degraded rather than halt. In availability-critical contexts (hot-path systems serving users), the cost of a false rejection (an outage) may exceed the cost of allowing unknown input. This principle applies to hot-path systems, not to governed state in OpsDB, where fail-closed is the default.

**Bound Everything** `infra` — every queue has a maximum depth. Every cache has a maximum size. Every connection has a timeout. Every retry has a budget. Every query has a maximum result size and execution time. Unbounded resources fail unboundedly.

**Reversible Changes** `infra` — prefer mechanisms that allow rollback. Changes have unintended consequences. The ability to revert is the fastest recovery mechanism. Rollback in OpsDB is a change set through the standard pipeline, not a special mechanism.

**Security by Anatomy at the Application Layer** `silo` — at the application validation layer, security derives from structural limitation of what the system can express, not from rules about who can do what. A misconfigured policy cannot introduce regex because the vocabulary does not contain regex. A compromised runner cannot write outside its declared scope because both the API and the library suite validate declarations and reject violations. This is different from fail-closed (which is a behavioral policy that could be overridden) because the vocabulary to override does not exist within the OpsDB boundary.

**Shape Before Meaning at the API Layer** `silo` — structural validation precedes semantic interpretation in the gate pipeline. Steps 3 and 4 (schema and bound validation) run before step 5 (policy and semantic evaluation). Input that does not conform structurally is rejected before the system considers what it means. This prevents injection, type confusion, and constraint violation by ensuring that the semantic layer only sees structurally valid data.

### 4.4 Dependency and Structure

**Minimize Dependencies** `infra` — each new dependency multiplies failure modes and migration costs. The runner library suite is minimized. No framework owns the runner's main loop. Hot-path systems do not depend on OpsDB availability at runtime.

**Separate Planes** `infra` — the data plane (serving user traffic), the control plane (configuring the data plane), and the management plane (observing both) have different requirements and blast radii. In split-backend applications, the hot-path system is the data plane. OpsDB is the control and management plane. They are connected by runners, not by runtime dependencies.

**Layer for Separation of Concerns** `infra` — each layer does one job with a clean interface to the next. Frontend handles presentation. The API gate handles governance. The substrate handles storage. Runners handle backend logic. Libraries handle world-side operations.

**Bucket for Locality and Accounting** `infra` — group data by access pattern, tenant, lifecycle, or failure domain. Per-tenant bucketing through authorization layers. Per-type retention through retention policies.

**Separate Domain Logic from Infrastructure Logic** `app` — domain logic (billing computation, eligibility determination, move validation) must be isolated from infrastructure logic (retry, authentication, serialization, logging). Domain logic changes at domain pace. Infrastructure logic changes at platform pace. In OpsDB Application Architecture, domain logic lives in runners and policy data. Infrastructure logic lives in the gate pipeline and library suite. Runners call libraries. Libraries never call runner logic.

**Separate Read Models from Write Models** `app` — when read patterns diverge from write patterns (aggregated dashboards, denormalized list views, search-optimized projections), maintain separate read models derived from the governed write model. The read model is a cache. The write model is the source of truth. Inconsistency between them is a freshness problem, not a correctness problem. Observation cache tables written by runners serve as read models.

**Infrastructure Fix Protects All Consumers** `silo` — because the infrastructure (gate pipeline, schema engine, versioning system, library suite) is shared across all applications built on it, fixing a bug in any infrastructure component fixes it for every application simultaneously. A validation bug fixed in the gate pipeline is fixed for the billing application, the healthcare application, and the recipe tracker in the same release. This justifies investing in comprehensive shared infrastructure over per-application custom code.

### 4.5 Distribution

**Local Cache plus Global Truth** `infra` — keep an updatable local copy of what is needed. Fall through to the global source when the local copy is stale or missing. The read cache, runner-cycle cache, local replicas, and hot-path cached configuration all implement this. Local is fast and survives partitions. Global is authoritative but slower and partition-vulnerable.

**Centralize Policy, Decentralize Enforcement** `infra` — one place defines the rules. Many places enforce them locally. Policies live in OpsDB as data rows. Enforcement happens at the API gate (for OpsDB writes) and at the library suite (for world-side actions). Two enforcement surfaces, one policy source, both checking the same declarations.

**Push the Decision Down** `infra` — make decisions at the lowest layer that has enough information. Hot-path decisions happen locally in the hot-path system. Governed decisions go through OpsDB. Pushing decisions down reduces round-trips and avoids serialization through central choke points.

**Push the Work Down or Out** `infra` — edge caches absorb traffic. CDNs absorb edge traffic. Each layer reduces what the next layer must handle. Content delivery separated from content governance.

### 4.6 Operator Relationship

**Make State Observable** `infra` — if you cannot see it, you cannot operate it. Every operational decision requires evidence. The audit log, version history, runner job records, and metrics emission make the system's state visible.

**Removing Classes of Work** `infra` — the goal of automation is not doing work faster. It is making work no longer need to be done. OpsDB removes authentication, validation, versioning, audit logging, and change management as classes of work that each application must build. They are provided by the platform.

**Marginal Cost of New Behavior Approaches Zero** `silo` — adding the 1000th entity type costs the same engineering effort as adding the 10th. No new API endpoints. No new validation code. No new authorization logic. No new audit handling. Write a YAML file, run the loader. Similarly, adding the 1000th runner costs the same as the 10th: 150 to 300 lines of domain logic plus library calls.

---

## 5. Application Classes

Applications are classified by their structural profile: which mechanisms dominate, which properties are critical, and where OpsDB sits in the architecture. Two applications in different domains (healthcare and project management) may have nearly identical profiles because their structural requirements are similar.

### 5.1 Architecture Positions

**Primary Backend** — OpsDB is the only backend. No separate hot-path system. The entire application is governed state management with a frontend on top. Governed state is 90-100% of the data model.

**Split Backend** — OpsDB governs most state. A specialized system handles a specific processing requirement (checkout flow, content delivery, telemetry ingestion, reservation management). Governed state is 70-90% of the data model. Runners bridge the two systems.

**Operational Wrapper** — OpsDB governs configuration, policies, and audit around a hot-path-dominant system (real-time communication, stream processing, game server, ML inference). Governed state is 10-30% of the data model. The hot-path system is the primary architecture.

**Metadata Manager** — OpsDB holds structured metadata about a specialized system (time-series database, search engine, graph database, object store). Governed state is 5-10% of the data model. OpsDB is not in the data path.

### 5.2 Primary Backend Applications

**Business SaaS** (CRM, ERP, HR systems) — every entity is governed. CRUD with validation, approval routing, version history, audit trail. The change set model maps directly to business workflows. Multi-tenancy through authorization layers.

**Internal Business Tools** — admin dashboards, inventory management, procurement. Same structure as SaaS with lighter approval policies.

**Case Management** — legal cases, support tickets, insurance claims. State machine lifecycle with approval workflows, deadline tracking, SLA computation.

**Healthcare Records** — patient records, treatments, appointments. Per-field access classification for data sensitivity. Non-repudiation for medical record authorship. HIPAA compliance as native property.

**Compliance Platform** — continuous evidence production, control verification, finding lifecycle, framework-to-control mapping.

**Education Platform** — course management, student records, assignment tracking. Versioned assignments, attributed grades, schedule conflict detection.

**Research Data Management** — experiment tracking, dataset cataloging, protocol versioning, sample chain-of-custody.

**Personal Data Platform** — recipes, books, home inventory, personal finance, workout logs. Single user, auto-approve everything, governance features invisible but structural properties (versioning, search, validation) remain valuable.

**Document and Knowledge Management** — structured metadata over unstructured content. Ownership tracking, review scheduling, version history.

**Inventory and Asset Tracking** — location hierarchies, stock level computation, reorder thresholds, depreciation.

**Procurement and Vendor Management** — multi-level approval routing, budget tracking, contract lifecycle, vendor scoring.

### 5.3 Split Backend Applications

**E-commerce** — catalog management in OpsDB (governed, versioned, auditable). Cart and checkout in a separate fast-path service. Inventory reservation needs optimistic locking. Payment integration through runners.

**Content Management** — editorial workflow in OpsDB (draft, review, approve, publish). Content delivery through CDN. Template rendering for published content.

**Booking and Scheduling** — booking configuration in OpsDB. Interval conflict detection. Timezone handling. At high contention (thousands of concurrent bookings for the same resource), a specialized reservation engine handles the lock contention.

**IoT Fleet Management** — device registry and fleet configuration in OpsDB. Telemetry ingestion in a time-series database. Runners bridge fleet management to telemetry summaries.

**Supply Chain and Logistics** — shipment state machines, warehouse management, order fulfillment in OpsDB. Real-time tracking in a streaming system.

**Workflow and Approval Engine** — workflow state in OpsDB. Execution orchestration (parallel branches, conditional routing) in a separate engine.

**Subscription and Billing** — plan definitions, proration, usage metering, invoice generation, payment retry, dunning sequences, revenue recognition. Payment processing through external gateway connected via runners.

### 5.4 Operational Wrapper Applications

**Real-time Communication** — user accounts and configuration in OpsDB. Message delivery through WebSocket or MQTT message bus.

**Stream Processing** — pipeline configuration in OpsDB. Event processing through Kafka, Flink, or equivalent.

**Real-time Gaming** — player accounts, matchmaking parameters, reward tables in OpsDB. Game simulation in dedicated game server.

**ML Training and Inference** — model registry, experiment metadata, deployment approvals in OpsDB. Training on GPU clusters. Inference at millisecond latency.

**Video Streaming** — content catalog and licensing in OpsDB. Transcoding, CDN routing, and adaptive bitrate in delivery infrastructure.

**Ad Auction** — campaign management and policies in OpsDB. Bid resolution in auction engine.

**High-Frequency Trading** — accounts, rules, compliance in OpsDB. Order execution in matching engine.

### 5.5 Metadata Manager Applications

**Time-Series Database** — metadata and authority pointers in OpsDB. Full-resolution metrics in specialized engine.

**Search Engine** — source metadata in OpsDB. Indexes and query execution in search engine.

**Graph Database** — entity metadata in OpsDB. Graph queries in specialized engine.

**Object/Blob Storage** — media metadata in OpsDB. Binary data in object store.

**Message Broker** — topic and consumer configuration in OpsDB. Message handling in broker engine.

---

## 6. Algorithms

Algorithms are the computational patterns that runners, the API, and external systems implement. Each algorithm composes specific mechanisms and contributes to specific properties.

### 6.1 State Management

**CRUD Lifecycle** — create, read, update, delete with validation. Implemented in the gate pipeline. Used by every application class.

**State Machine** — entity progresses through declared states with validated transitions. Transition rules as policy data. State machine evaluator validates. Version history records transition sequence.

**Approval Routing** — fan-out to approver groups computed by walking ownership and stakeholder bridges across touched entities. Computed at change set submission. Each required approval tracked independently.

**Optimistic Concurrency** — version stamps compared at submission. Stale versions rejected loudly. Submitter reconciles and resubmits.

**Double-Entry Bookkeeping** — every value change produces a balancing entry. Used in financial applications where the accumulator must always balance.

**Soft Delete with Retention** — is\_active set to false. Row persists. Reaper removes after retention horizon.

### 6.2 Query and Discovery

**Cursor Pagination** — stable ordering plus opaque cursor token. Deterministic results across pages.

**Faceted Filter** — predicate composition with AND/OR/NOT. Bounded depth. Schema-validated field names and types.

**Hierarchy Traversal** — recursive walk of self-referential foreign key chains. Depth-bounded with cycle detection.

**Graph Walk** — relationship traversal across entity types through named join paths. Bounded depth.

**Time-Range Query** — filter by temporal bounds with freshness annotation on cached data.

### 6.3 Scheduling and Temporal

**Cron Evaluation** — next-run computation from cron expression or schedule entity.

**Interval Conflict Detection** — overlapping interval identification for booking and scheduling applications.

**Deadline Tracking** — time-until-due computation with escalation triggering.

**Recurrence Expansion** — generate concrete instances from a recurrence rule over a time window.

**Timezone Normalization** — convert and compare timestamps across timezones correctly.

### 6.4 Financial and Metering

**Proration** — partial-period charge calculation for billing applications.

**Usage Metering** — accumulate usage events into billable quantities.

**Tax Calculation** — jurisdiction-based tax rate selection and application.

**Revenue Recognition** — allocate revenue across performance obligations over time.

**Budget Tracking** — committed versus spent versus remaining against a budget entity.

**Dunning Sequence** — escalating collection attempts on overdue accounts.

### 6.5 Reconciliation and Verification

**Desired-vs-Observed Diff** — compare governed entities against cached observations. Reconciler runners implement this on every cycle.

**Drift Detection** — identify discrepancies without correcting them. File proposed change sets for human review.

**Evidence Accumulation** — scheduled verification producing structured pass/fail evidence records.

**Reconciliation with External** — match OpsDB records against external authority data. Puller provides external state, reconciler compares.

**Integrity Verification** — hash chain or checksum validation over historical audit records.

### 6.6 Scoring and Ranking

**ELO/Rating Computation** — update player or entity ratings based on match outcomes.

**Vendor/Candidate Scoring** — multi-criteria weighted score with configurable weights as policy data.

**Priority Queue** — bounded priority ordering for work items.

### 6.7 Integration

**Authority Polling** — scheduled read from external API with transformation to OpsDB entity shape.

**Webhook Processing** — event receipt with idempotency check and state update.

**Configuration Push** — format governed state for an external system's native configuration format and push it.

**Observation Pull** — read results from an external system and write them as OpsDB observations.

**Schema Mapping** — transform between an external system's data format and OpsDB entity shapes.

### 6.8 Hot-Path Algorithms (External to OpsDB)

Order matching, auction resolution, physics simulation, adaptive bitrate selection, windowed aggregation, CRDT merge, gradient descent, spatial partitioning, and content-addressed storage. These execute in specialized systems outside OpsDB. OpsDB governs the configuration and policies around them and records their results as observations.

---

## 7. Governance Flags and Property Impact

Governance flags adjust which gate pipeline steps apply to a specific table. They relax the recording properties (versioning, audit, change management) while preserving the enforcement properties (authentication, authorization, validation).

**`_autoversion_disabled`** — skips version row creation on interim writes. Versioning weakened: interim states between explicit version commits are not individually recoverable. Reversibility weakened at per-write granularity. All other properties preserved.

**`_edit_latest_version`** — writes directly to the current row, skipping change set routing. Change management skipped for interim writes. Lifecycle integrity weakened because state transitions are not individually tracked between commits. All other properties preserved.

**`_audit_logs_disabled`** — skips audit log entries for interim writes. Per-write auditability lost. Non-repudiation weakened because interim actions are not individually attributable. All other properties preserved.

**Invariant:** validation (schema and bounds), authorization (all five layers), and structural security (vocabulary closure) are never affected by any governance flag. Flags only affect recording. Enforcement always runs.

When a version commit is explicitly triggered, all ten gate steps run. The commit produces a version row, a change set record, and an audit entry. Committed versions have the full property set regardless of flag configuration.

---

## 8. Failure Modes

**Process crash** — liveness lost briefly. In-flight atomicity lost for uncommitted work. Durability preserved through write-ahead log. Runner picks up on next cycle.

**Database primary failure** — liveness and availability lost until failover. Durability preserved through WAL. Runners with cached configuration continue operating.

**Stale derived quantity** — computational correctness violated. Verifier runner detects on next cycle and files a finding.

**Invalid lifecycle transition** — lifecycle integrity violated. Should have been caught by state machine evaluator. Investigation through audit trail reveals the gap.

**Policy misconfiguration** — property impact depends on which policy is misconfigured. Access policy error affects confidentiality. Lifecycle rule error affects lifecycle integrity. All policy changes are versioned, so the prior configuration is recoverable.

**Runner scope over-provisioning** — structural security weakened because runner has more access than needed. Audit trail records all actions. Scope can be narrowed through change set.

**Draft mode data loss** — reversibility at per-save granularity lost for draft-mode tables. Committed versions fully protected. Working state between commits not individually recoverable.

**Change set executor lag** — liveness for approved changes reduced. Changes wait in queue. No data corruption. Latency problem, not correctness problem.

**Cross-AppDB reference failure** — availability of referenced data lost. Local data remains intact. Application handles gracefully or logs finding.

**Concurrent change set conflict** — isolation for second submitter failed. Stale version error returned. Submitter reconciles and resubmits.

---

## 9. Abstraction Layer Scoping

Properties and mechanisms hold at specific layers. Three layers exist.

**Full stack** — items provided by the storage engine and operating system. Durability (Postgres WAL), availability (OS process management), consistency-replica (Postgres replication).

**OpsDB application layer** — items provided by the schema engine, gate pipeline, change management, versioning, audit log, runners, and library suite. This is where most application-specific and data-only-architecture-derived items operate. Vocabulary closure, structural security, domain opacity, lifecycle integrity, computational correctness, non-repudiation, and all governance-flag-adjustable properties hold at this layer.

**External hot-path systems** — items provided by specialized systems connected via runners. OpsDB does not claim properties for these systems. The hot-path system claims its own properties. OpsDB governs the configuration around them and records their results.

The honest scoping is: OpsDB controls the application layer. It trusts the storage engine for durability and consistency-replica. It does not control the Go runtime, the Postgres query executor, or the operating system. Properties qualified "application layer" hold within the OpsDB boundary. They may or may not hold below it.

---

## 10. Summary

This taxonomy enumerates 74 mechanisms, 28 properties, and 30 principles for building applications on a governed data substrate. The mechanisms are the building blocks. The properties are the guarantees. The principles are the rules for choosing and assembling building blocks to achieve guarantees.

Of the 74 mechanisms, 62 are inherited from operational infrastructure, 4 are specific to application development (state machine evaluation, rule engine, accumulator, temporal projection), and 8 come from data-only execution architecture (execution funnel, fact regeneration, multiplicative scoring, semantic repurposing, hot-swap, scene isolation, structured trace, ingress validation).

Of the 28 properties, 21 are inherited, 3 are application-specific (lifecycle integrity, computational correctness, non-repudiation), and 4 come from data-only architecture (vocabulary closure, structural security, hot-swap safety, domain opacity).

Of the 30 principles, 22 are inherited, 3 are application-specific (separate domain from infrastructure, express business rules as data, separate read models from write models), and 5 come from data-only architecture (security by anatomy, vocabulary restriction, shape before meaning, infrastructure fix protects all, marginal cost approaches zero).

The taxonomy classifies 33 application types across 4 architecture positions: primary backend (12 types where OpsDB is the only backend), split backend (9 types with governed state plus a specialized hot-path system), operational wrapper (7 types where OpsDB governs configuration around a hot-path-dominant system), and metadata manager (5 types where OpsDB holds structured metadata about a specialized system).

46 algorithms enumerate the computational patterns: state management, query and discovery, scheduling, financial computation, reconciliation, scoring, integration, and hot-path processing.

The three-axis structure is the load-bearing claim. The contents are revisable. As new application domains are explored and new patterns discovered, the taxonomy grows by adding items to existing families, bands, and groups. The structural separation of mechanism, property, and principle holds regardless of what specific items populate each axis.

---

