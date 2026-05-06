## Application Taxonomy for OpsDB Application Architecture

### Planning Document

---

### Purpose

Enumerate all classes and types of software applications using the infrastructure taxonomy's three-axis framework — mechanisms, properties, and principles — as the structural basis. Each application class is described by which mechanisms it uses, which properties it requires, which principles govern its construction, and where OpsDB sits in its architecture. The taxonomy is descriptive: it names the parts and their relationships without prescribing construction.

---

### Structural Approach

The infrastructure taxonomy defines 62 mechanisms across 13 families, 21 properties across 4 bands, and 22 principles across 6 groups. Applications are compositions of these primitives. Two applications in different domains — a healthcare records system and a project management tool — may use nearly identical mechanism profiles because their structural requirements are similar even though their domain content differs.

The taxonomy classifies applications by their structural profile, not their domain. Domain is content. Structure is what mechanisms are needed, what properties must hold, and what principles govern assembly. Two applications with the same structural profile but different domains use the same OpsDB architecture position with different schemas.

---

### Axis 1: Application Mechanism Profile

Each application class is characterized by which mechanism families dominate its architecture and which are light or absent. This determines what the application fundamentally does with data.

#### Mechanism Family Utilization Per Application Class

```
families(id|family|role_in_applications)
AF1|Storage (F4)|every application stores state; dominant family for all
AF2|Gating (F9)|validation+auth+authz; dominant for governed applications
AF3|Versioning (F5)|history+lineage; dominant for auditable applications
AF4|Selection (F2)|query+filter+index; dominant for discovery-heavy applications
AF5|Information Movement (F1)|data flow between systems; dominant for integration-heavy applications
AF6|Control Loop (F8)|reconciliation+reaction; dominant for automation-heavy applications
AF7|Sensing (F7)|observation+metrics+health; dominant for monitoring-aware applications
AF8|Lifecycle (F6)|expiration+retention+draining; important for long-lived data
AF9|Coordination (F11)|locking+election+sequencing; dominant for concurrent-write applications
AF10|Transformation (F12)|rendering+computation+compaction; dominant for processing-heavy applications
AF11|Resilience (F13)|retry+circuit-break+failover; dominant for distributed applications
AF12|Allocation (F10)|pooling+quota+scheduling; dominant for multi-tenant resource applications
AF13|Representation (F3)|schema+namespace+encoding; foundational for all structured applications
```

#### Application Classes by Dominant Mechanism Profile

```
app_classes(id|name|dominant_families|secondary_families|light_or_absent|key_algorithms)
```

**Governed-State-Dominant Applications**

```
AC01|Business SaaS (CRM/ERP/HR)|AF1+AF2+AF3+AF4|AF5+AF8+AF13|AF6+AF9+AF10+AF11|CRUD+ACL evaluation+approval state machine+version chain+full-text filter+cursor pagination
AC02|Internal Business Tools|AF1+AF2+AF4|AF3+AF8+AF13|AF5+AF6+AF9+AF10+AF11|CRUD+role-based filter+basic versioning+search+scheduled reports
AC03|Case Management|AF1+AF2+AF3+AF4|AF5+AF6+AF8|AF9+AF10+AF11|state machine (case lifecycle)+deadline tracking+SLA computation+document attachment reference+stakeholder routing
AC04|Healthcare Records|AF1+AF2+AF3+AF4|AF5+AF8+AF13|AF6+AF9+AF10|field-level ACL+audit chain+consent management+retention by regulation+HL7/FHIR mapping
AC05|Financial Services Backend|AF1+AF2+AF3+AF9|AF4+AF5+AF8|AF6+AF10+AF11|double-entry ledger+optimistic concurrency+reconciliation+regulatory reporting aggregation+segregation of duties
AC06|Compliance Platform|AF1+AF2+AF3+AF7|AF4+AF5+AF6+AF8|AF9+AF10+AF11|evidence scheduling+control verification+finding lifecycle+framework-to-control mapping+continuous evidence accumulation
AC07|Education Platform|AF1+AF2+AF3+AF4|AF5+AF8|AF6+AF9+AF10+AF11|grade computation+assignment lifecycle+enrollment state machine+rubric evaluation+schedule conflict detection
AC08|Research Data Management|AF1+AF2+AF3+AF4|AF5+AF7+AF8|AF6+AF9+AF10|dataset cataloging+protocol versioning+experiment lineage DAG+sample chain-of-custody+retention by grant
AC09|Personal Data Platform|AF1+AF4|AF2+AF3+AF8|AF5+AF6+AF7+AF9+AF10+AF11|CRUD+search+basic versioning+single-user auth+scheduled automation
AC10|Document/Knowledge Mgmt|AF1+AF3+AF4|AF2+AF8+AF13|AF5+AF6+AF9+AF10|full-state versioning+ownership tracking+review scheduling+structured metadata over unstructured content+link integrity checking
AC11|Inventory/Asset Tracking|AF1+AF2+AF4|AF3+AF5+AF7+AF8|AF6+AF9+AF10+AF11|location hierarchy traversal+stock level computation+reorder threshold+depreciation calculation+barcode/SKU resolution
AC12|Procurement/Vendor Mgmt|AF1+AF2+AF3+AF4|AF5+AF8|AF6+AF9+AF10+AF11|approval routing (multi-level)+budget tracking+contract lifecycle+vendor scoring+purchase order state machine
```

**Split-Backend Applications (Governed State + Hot Path)**

```
AC13|E-commerce|AF1+AF2+AF4+AF9|AF3+AF5+AF8+AF12|AF6+AF10|catalog CRUD+inventory reservation (optimistic lock)+cart session+checkout state machine+payment gateway integration+price computation+tax calculation
AC14|Content Management|AF1+AF2+AF3+AF4|AF5+AF8+AF10+AF13|AF6+AF9+AF11|editorial workflow state machine+publish/unpublish+content versioning+template rendering+CDN invalidation signaling+scheduled publish
AC15|Booking/Scheduling|AF1+AF2+AF4+AF9|AF3+AF5+AF8|AF6+AF10+AF11|calendar interval arithmetic+conflict detection (overlapping intervals)+resource capacity tracking+waitlist queue+timezone normalization+recurrence expansion
AC16|IoT Fleet Management|AF1+AF2+AF4+AF5|AF3+AF7+AF8+AF11|AF6+AF9+AF10|device registry CRUD+firmware version tracking+fleet segmentation+command queuing+telemetry aggregation (external)+threshold-based alerting
AC17|Supply Chain/Logistics|AF1+AF2+AF3+AF4+AF5|AF7+AF8+AF11|AF6+AF9+AF10|shipment state machine+route optimization reference+ETA computation+warehouse bin assignment+customs document generation+carrier integration
AC18|Workflow/Approval Engine|AF1+AF2+AF3+AF4+AF6|AF5+AF8+AF9|AF10+AF11|directed graph traversal (workflow definition)+parallel branch/join+conditional routing+deadline escalation+delegation+SLA tracking
AC19|Multi-player Turn-based Game|AF1+AF2+AF3+AF4+AF9|AF5+AF8|AF6+AF10+AF11|game state versioning+move validation (rule engine)+matchmaking scoring+ELO/ranking computation+turn timeout+replay construction from history
AC20|API Gateway Config|AF1+AF2+AF4+AF13|AF3+AF5+AF8|AF6+AF9+AF10+AF11|route table CRUD+rate limit policy+API key lifecycle+consumer quota tracking+usage metering+configuration push to gateway cache
AC21|Subscription/Billing|AF1+AF2+AF3+AF4+AF9|AF5+AF8+AF12|AF6+AF10+AF11|plan definition+proration calculation+usage metering+invoice generation+payment retry state machine+dunning sequence+revenue recognition
```

**Hot-Path-Dominant Applications (OpsDB as Wrapper)**

```
AC22|Real-time Communication|AF1+AF5+AF9+AF11|AF2+AF4+AF7|AF3+AF6+AF8+AF10|message routing+presence tracking+delivery receipt+connection multiplexing+message ordering (per-channel)+typing indicator+read cursor
AC23|Stream Processing|AF1+AF5+AF10+AF11|AF4+AF7+AF9|AF2+AF3+AF6+AF8|windowed aggregation+exactly-once processing+watermark tracking+late event handling+checkpoint/restore+backpressure+partition assignment
AC24|Real-time Gaming (FPS/MMO)|AF1+AF5+AF9+AF11+AF12|AF2+AF7|AF3+AF4+AF6+AF8|physics simulation tick+spatial partitioning+hit detection+state interpolation+lag compensation+interest management+authoritative server reconciliation
AC25|ML Training/Inference|AF1+AF4+AF10+AF12|AF2+AF3+AF5+AF7|AF6+AF8+AF9+AF11|gradient descent+batch scheduling+model serialization+feature pipeline+A/B traffic splitting+canary evaluation+model versioning
AC26|Video Streaming|AF1+AF5+AF10+AF12|AF2+AF4+AF7+AF11|AF3+AF6+AF8+AF9|adaptive bitrate selection+segment encoding+manifest generation+CDN origin routing+DRM license serving+concurrent viewer counting+quality-of-experience metrics
AC27|Ad Auction/RTB|AF1+AF2+AF5+AF9+AF10+AF12|AF4+AF7+AF11|AF3+AF6+AF8|second-price auction+bid scoring+budget pacing+frequency capping+targeting predicate evaluation+latency-bounded response+spend reconciliation
AC28|High-Frequency Trading|AF1+AF5+AF9+AF10+AF11|AF2+AF4+AF7|AF3+AF6+AF8|order matching (price-time priority)+position tracking+risk limit enforcement+market data fan-out+order book maintenance+latency measurement+regulatory reporting accumulation
```

**Specialized Systems (OpsDB as Metadata Manager)**

```
AC29|Time-Series Database|AF1+AF4+AF10+AF12|AF5+AF7+AF8|AF2+AF3+AF6+AF9+AF11|columnar compression+downsampling+retention tiering+query over time range+tag indexing+rollup aggregation
AC30|Search Engine|AF1+AF4+AF10+AF13|AF5+AF7+AF12|AF2+AF3+AF6+AF8+AF9+AF11|inverted index construction+BM25/TF-IDF scoring+tokenization pipeline+facet aggregation+fuzzy matching+index segment merge
AC31|Graph Database|AF1+AF4+AF9+AF11|AF2+AF3+AF5|AF6+AF7+AF8+AF10+AF12|graph traversal (BFS/DFS)+shortest path+PageRank+community detection+property graph query+index-free adjacency
AC32|Object/Blob Storage|AF1+AF4+AF8+AF12|AF2+AF5+AF11|AF3+AF6+AF7+AF9+AF10|content-addressed storage+multipart upload+lifecycle tiering+versioning (object-level)+erasure coding+consistency model (eventual/strong)
AC33|Message Broker|AF1+AF5+AF8+AF9+AF11|AF4+AF7+AF12|AF2+AF3+AF6+AF10|topic/queue routing+consumer group coordination+offset tracking+partition assignment+dead letter routing+exactly-once delivery+compaction
```

---

### Axis 2: Application Property Requirements

Each application class requires specific properties to function correctly. Properties map to the infrastructure taxonomy's 21 properties across 4 bands.

```
property_bands(id|band|application_relevance)
PB1|Data Integrity (P01-P07)|every application needs some; governed apps need most
PB2|Behavioral (P08-P14)|determines runtime characteristics and user experience
PB3|Distribution (P15-P19)|relevant when application spans nodes or serves many tenants
PB4|Operational (P20-P21)|relevant for all production applications; critical for regulated ones
```

#### Property Requirements by Application Class

```
property_profiles(app_class|critical_properties|important_properties|optional_properties)
AC01|P01(idempotency)+P02(atomicity)+P03(durability)+P04(consistency-data)+P21(auditability)|P05(integrity)+P06(authenticity)+P11(availability)+P13(isolation)+P14(reversibility)+P20(observability)|P07(confidentiality)+P09(convergence)+P15(consistency-replica)+P16(ordering)
AC03|P01+P02+P03+P04+P14+P21|P05+P06+P10(liveness)+P11+P13+P16(ordering)+P20|P07+P09+P12(boundedness)+P15
AC04|P01+P02+P03+P04+P05+P06+P07+P21|P11+P13+P14+P20|P09+P10+P12+P15+P16
AC05|P01+P02+P03+P04+P05+P06+P13+P21|P07+P11+P14+P16+P20|P09+P10+P12+P15
AC06|P01+P03+P04+P05+P06+P20+P21|P02+P07+P11+P14|P09+P10+P12+P13+P15+P16
AC09|P01+P03+P04+P14|P02+P11+P20|P05+P06+P07+P09+P10+P12+P13+P15+P16+P21
AC13|P01+P02+P03+P04+P11+P13|P05+P06+P07+P14+P16+P20+P21|P09+P10+P12+P15
AC15|P01+P02+P03+P04+P11+P13|P10+P14+P16+P20|P05+P06+P07+P09+P12+P15+P21
AC22|P03+P10+P11+P16|P01+P05+P06+P12+P19(failure-transparency)|P02+P04+P07+P09+P13+P14+P15+P20+P21
AC24|P10+P11+P12+P16+P19|P01+P03+P09+P15+P18(stability-under-change)|P02+P04+P05+P06+P07+P13+P14+P20+P21
AC28|P01+P02+P03+P04+P10+P11+P13+P16+P21|P05+P06+P07+P12+P20|P09+P14+P15+P18+P19
```

#### Property-to-OpsDB-Mechanism Mapping

```
property_provision(property|opsdb_mechanism|gate_step|notes)
P01|change_set atomic commit+version stamp check|AG9|idempotency via optimistic concurrency
P02|change_set atomic apply (all or none)|AG9|per-change-set atomicity
P03|Postgres WAL+replication|infrastructure|storage engine responsibility
P04|schema validation+bound validation+FK checks|AG3+AG4|declared constraints enforced mechanically
P05|_audit_chain_hash+TLS|AG8+infrastructure|integrity of audit trail and transit
P06|SSO delegation+service account auth|AG1|identity verification delegated to IdP
P07|_access_classification+five-layer authz|AG2|field-level confidentiality
P08|closed vocabulary (no regex, no logic)|schema engine|deterministic validation
P09|level-triggered runners+reconciler pattern|runner discipline|convergence through repeated observation
P10|runner cycle+change-set executor draining|runner infrastructure|progress through bounded cycles
P11|read cache+multi-instance+runner independence|infrastructure|availability through caching and decoupling
P12|query bounds+runner bounds+rate limits|AG4+runner config|explicit resource limits
P13|optimistic concurrency+change_set atomicity|AG9+version stamps|isolation through version checking
P14|rollback as change_set restoring prior version|AG7+AG9|reversibility through governed path
P15|read replicas+cache|infrastructure|replica consistency per storage engine config
P16|change_set apply_order+version_serial|AG9+versioning|ordering through monotonic sequencing
P17|observation cache locality+read cache|infrastructure|data proximity through caching layer
P18|additive schema evolution+no deletion|schema engine|stability through absolute evolution rules
P19|runner independence from OpsDB availability|runner architecture|failure transparency through local cache
P20|audit log+version history+runner job records|AG8+versioning|observability through comprehensive recording
P21|append-only audit log+version chain+change_set trail|AG8+AG6+AG7|auditability through immutable records
```

---

### Axis 3: Application Construction Principles

The infrastructure taxonomy's 22 principles apply to application construction. Some are universal. Some apply only to specific application classes.

```
principle_applicability(principle|universal|class_specific|application_notes)
R01 Data primacy|yes|-|schema is SoT; behavior as data; config as data
R02 Single source of truth|yes|-|OpsDB for governed state; external authorities for their domains
R03 Convention over lookup|yes|-|naming conventions; schema conventions; FK naming
R04 0/1/infinity|yes|-|one AppDB per app; N instances for distribution; never 2
R05 Comprehensive over aggregate|yes|-|slice the domain whole before populating entities
R06 One way to do each thing|yes|-|one API path; one library suite; one runner pattern
R07 Idempotent retry|yes|-|every runner action safely retryable
R08 Level-triggered over edge-triggered|yes (for runners)|-|reconcilers re-evaluate current state each cycle
R09 Fail closed|yes (for security/integrity)|-|authorization denials; scope violations; unknown state
R10 Fail open|class-specific|AC22+AC24+AC26+AC27|availability-critical hot paths; not for governed state
R11 Bound everything|yes|-|query bounds; runner bounds; rate limits; retry budgets
R12 Reversible changes|yes|-|rollback as change_set; version history enables reversal
R13 Minimize dependencies|yes|-|runner lib suite minimized; no framework ownership of main loop
R14 Separate planes|class-specific|AC13+AC16+AC20+AC22-AC28|data plane (hot path) separate from control plane (OpsDB)
R15 Layer for separation|yes|-|frontend / API gate / substrate / runners
R16 Bucket for locality|class-specific|AC12+AC13+AC16+AC21|per-tenant bucketing; per-domain bucketing
R17 Local cache + global truth|yes|-|read cache; runner cache; local replica; hot-path cached config
R18 Centralize policy decentralize enforcement|yes|-|policies in OpsDB; enforcement at API gate + library suite
R19 Push decision down|class-specific|AC22-AC28|hot-path decisions local; governed decisions through OpsDB
R20 Push work down/out|class-specific|AC14+AC26|CDN; edge cache; content delivery separate from governance
R21 Make state observable|yes|-|audit log; version history; runner job records; metrics
R22 Removing classes of work|yes|-|OpsDB removes auth/validation/versioning/audit as classes of work
```

---

### Axis 4: OpsDB Architecture Position

```
positions(id|position|description|opsdb_role|hot_path_present)
AP01|Primary backend|OpsDB is the only backend; no separate hot path|full backend: schema+API+runners+versioning+audit|no
AP02|Split backend|OpsDB governs most state; specialized system handles specific hot path|governance layer + integration via runners|yes (bounded)
AP03|Operational wrapper|OpsDB governs config/policies/audit around a hot-path-dominant system|metadata and configuration management|yes (dominant)
AP04|Metadata manager|OpsDB holds metadata about a specialized system; not in data path|structured pointers and operational metadata|yes (entire system)
```

```
position_mapping(app_class|position|governed_pct|hot_path_system|connection_pattern)
AC01|AP01|95%+|none|web layer → OpsDB API
AC02|AP01|95%+|none|web layer → OpsDB API
AC03|AP01|95%+|none|web layer → OpsDB API
AC04|AP01|95%+|none|web layer → OpsDB API
AC05|AP02|90%|transaction processor|config runner pushes rules; observation runner pulls results
AC06|AP01|99%|none|web layer → OpsDB API
AC07|AP01|95%+|none|web layer → OpsDB API
AC08|AP02|90%|compute cluster|config runner pushes parameters; observation runner pulls metrics
AC09|AP01|99%|none|web layer → OpsDB API
AC10|AP01|95%+|none|web layer → OpsDB API
AC11|AP01|95%+|none|web layer → OpsDB API
AC12|AP01|95%+|none|web layer → OpsDB API
AC13|AP02|80%|checkout/cart service|catalog managed in OpsDB; cart/checkout separate
AC14|AP02|85%|CDN/delivery|editorial in OpsDB; content delivery via CDN
AC15|AP02|85%|reservation engine (high contention)|booking config in OpsDB; reservation lock separate at scale
AC16|AP02|70%|telemetry pipeline|fleet in OpsDB; time-series in specialized DB
AC17|AP02|80%|tracking/routing engine|supply chain data in OpsDB; real-time tracking separate
AC18|AP02|80%|workflow execution engine|state in OpsDB; execution orchestration separate
AC19|AP02|75%|real-time sync layer|game state in OpsDB; turn sync separate
AC20|AP02|85%|API gateway runtime|config in OpsDB; request routing at gateway
AC21|AP02|85%|payment processor|plans/invoices in OpsDB; payment processing separate
AC22|AP03|20%|message bus (WebSocket/MQTT)|accounts/config in OpsDB; message delivery separate
AC23|AP03|15%|stream processor (Kafka/Flink)|pipeline config in OpsDB; event processing separate
AC24|AP03|15%|game server|player data in OpsDB; game simulation separate
AC25|AP03|20%|training cluster+inference service|model registry/experiment tracking in OpsDB; compute separate
AC26|AP03|15%|transcoding+CDN+DRM|content catalog in OpsDB; delivery separate
AC27|AP03|10%|auction engine|campaign data in OpsDB; bid resolution separate
AC28|AP03|10%|matching engine|accounts/rules/compliance in OpsDB; order execution separate
AC29|AP04|5%|time-series engine|metadata+authority pointers in OpsDB
AC30|AP04|10%|search/index engine|source metadata in OpsDB; indexes built from it
AC31|AP04|10%|graph engine|entity metadata in OpsDB; graph queries in specialized DB
AC32|AP04|10%|object store|media metadata in OpsDB; blobs in S3/GCS
AC33|AP04|5%|broker engine|topic/consumer config in OpsDB; message handling in broker
```

---

### Key Algorithms Enumeration

Each application class uses specific algorithms. These are the computational patterns that runners and hot-path systems implement.

```
algorithms(id|name|description|used_by|implemented_in)
```

**State Management Algorithms**

```
AL01|CRUD lifecycle|create-read-update-delete with validation|all AC classes|OpsDB gate pipeline
AL02|State machine|entity progresses through declared states with validated transitions|AC03+AC13+AC14+AC17+AC18+AC21|policy data + runner validation
AL03|Approval routing|fan-out to approver groups computed from ownership/stakeholder bridges|AC01-AC12|OpsDB change management (gate step 7)
AL04|Optimistic concurrency|version stamp comparison at submit; stale detection|AC01-AC21|OpsDB gate pipeline (version stamps)
AL05|Double-entry bookkeeping|every value change produces balancing entry|AC05+AC21|runner logic + change_set atomicity
AL06|Soft delete with retention|is_active=false; reaper enforces horizon|all versioned classes|OpsDB reserved fields + reaper runner
```

**Query and Discovery Algorithms**

```
AL07|Cursor pagination|stable ordering + opaque cursor token + seek method|all AC classes|OpsDB search API
AL08|Faceted filter|predicate composition across typed fields with AND/OR/NOT|all AC classes|OpsDB search API predicates
AL09|Hierarchy traversal|recursive walk of self-FK parent chain|AC04+AC11+AC17|OpsDB get_dependencies + named join paths
AL10|Graph walk (bounded)|relationship traversal with depth limit and cycle detection|AC03+AC08+AC17+AC18|OpsDB get_dependencies
AL11|Full-text search|tokenization + inverted index + relevance scoring|AC10+AC14+AC30|external search engine; OpsDB holds metadata
AL12|Time-range query|filter by temporal bounds with freshness annotation|AC06+AC07+AC16+AC29|OpsDB search API + observation cache
```

**Scheduling and Temporal Algorithms**

```
AL13|Cron evaluation|next-run computation from cron expression|AC06+AC07+AC15+AC16+AC21|schedule entity + scheduler runner
AL14|Interval conflict detection|overlapping interval identification for booking/scheduling|AC15+AC07|runner logic + search query
AL15|Deadline tracking|time-until-due computation + escalation trigger|AC03+AC06+AC12+AC17|schedule entity + verifier runner
AL16|Recurrence expansion|generate instances from recurrence rule|AC07+AC15|runner logic
AL17|Timezone normalization|convert and compare across timezones|AC07+AC15+AC22|runner logic
```

**Financial and Metering Algorithms**

```
AL18|Proration|partial-period charge calculation|AC21|runner logic
AL19|Usage metering|accumulate usage events into billable quantities|AC21+AC27|observation cache + metering runner
AL20|Tax calculation|jurisdiction-based tax rate application|AC13+AC21|runner logic + tax policy data
AL21|Revenue recognition|allocate revenue across performance obligations over time|AC21|runner logic + policy data
AL22|Budget tracking|committed vs spent vs remaining against budget entity|AC12|runner logic + search aggregation
AL23|Dunning sequence|escalating collection attempts on overdue accounts|AC21|runner state machine + notification runner
```

**Reconciliation and Verification Algorithms**

```
AL24|Desired-vs-observed diff|compare governed entities against cached observations|AC05+AC06+AC16+AC17|reconciler runner pattern
AL25|Drift detection|identify discrepancies without correcting|AC06+AC16+AC20|drift detector runner pattern
AL26|Evidence accumulation|scheduled verification producing pass/fail records|AC04+AC06|verifier runner pattern
AL27|Reconciliation with external|match OpsDB records against external authority|AC05+AC13+AC21|reconciler runner + external puller
AL28|Integrity verification|hash chain or checksum validation over historical records|AC04+AC05+AC06|audit chain verification tooling
```

**Scoring and Ranking Algorithms**

```
AL29|ELO/rating computation|update ratings based on match outcomes|AC19|runner logic
AL30|Vendor/candidate scoring|multi-criteria weighted score|AC12|runner logic + policy data (weights)
AL31|Priority queue|bounded priority ordering for work items|AC03+AC18|runner logic + search ordering
AL32|Recommendation|collaborative filtering or content-based scoring|AC09+AC13+AC14|external engine; OpsDB holds feature data
```

**Integration Algorithms**

```
AL33|Authority polling|scheduled read from external API with transformation|all classes with pullers|puller runner pattern
AL34|Webhook processing|event receipt + idempotency check + state update|AC13+AC17+AC21+AC22|reactor runner pattern
AL35|Configuration push|format governed state for external system's native config|AC16+AC20+AC22-AC28|config runner pattern
AL36|Observation pull|read external results + write as OpsDB observations|AC05+AC13+AC16+AC22-AC28|observation runner pattern
AL37|Schema mapping|transform between external data format and OpsDB entity shape|all classes with importers|puller runner transformation logic
```

**Hot-Path Algorithms (Outside OpsDB)**

```
AL38|Order matching|price-time priority matching of buy/sell orders|AC28|matching engine (external)
AL39|Auction resolution|second-price or other auction mechanism|AC27|auction engine (external)
AL40|Physics simulation|tick-based position/velocity/collision computation|AC24|game server (external)
AL41|Adaptive bitrate|quality selection based on bandwidth estimation|AC26|player/CDN (external)
AL42|Windowed aggregation|time-window grouping with watermark-based completion|AC23|stream processor (external)
AL43|CRDT merge|conflict-free replicated data type convergence|AC22|sync engine (external)
AL44|Gradient descent|iterative parameter optimization|AC25|training cluster (external)
AL45|Spatial partitioning|divide world/map into regions for efficient query|AC24|game server (external)
AL46|Content-addressed storage|hash-based deduplication and integrity|AC32|object store (external)
```

---

### Cross-Reference: Algorithm to OpsDB Component

```
algorithm_implementation(algorithm|opsdb_component|runner_type|notes)
AL01|gate pipeline (steps 3-9)|n/a|built into API
AL02|policy data + gate step 5|change-set executor|transition rules as policy rows
AL03|gate step 7 + ownership/stakeholder bridges|notification runner|computed at submit time
AL04|gate step 9 (version stamp check)|n/a|built into API
AL05|change_set atomicity|billing runner|runner constructs balanced entries
AL06|reserved fields + retention_policy|reaper runner|built into schema engine + runner
AL07|search API|n/a|built into API
AL08|search API predicates|n/a|built into API
AL09|get_dependencies + named joins|n/a|built into API
AL10|get_dependencies with depth bound|n/a|built into API
AL13|schedule entity|scheduler runner|schedule_data_json typed payload
AL14|search query (overlapping ranges)|booking runner|runner queries for conflicts before proposing
AL15|schedule entity + evidence_record|verifier runner|deadline computed from schedule; evidence proves check
AL18|configuration_variable (rate data)|billing runner|runner reads rate data; computes proration
AL19|observation_cache_metric|metering runner|puller accumulates; metering runner aggregates
AL24|entity rows + observation_cache|reconciler runner|standard reconciler pattern
AL25|entity rows + observation_cache|drift detector runner|proposes change_set instead of acting
AL26|schedule + evidence_record|verifier runner|standard verifier pattern
AL27|entity rows + external puller|reconciler runner|puller provides external state
AL33|authority + runner_spec|puller runner|standard puller pattern
AL34|reactor runner + idempotency keys|reactor runner|paired with reconciler backstop
AL35|entity rows + runner_spec|config runner|runner reads governed state; formats for external
AL36|external API + observation_cache|observation runner|runner pulls results; writes observations
AL37|puller runner transformation|puller runner|mapping logic in runner; schema in OpsDB
```

---

### Pattern: Application Class to Complete Architecture

For each application class, the complete architecture specification is:

```
architecture_spec(app_class|schema_domains|runner_kinds|hot_path_system|connection_pattern|critical_properties|key_algorithms|opsdb_position)
```

Three representative examples:

```
AS01|AC01 (Business SaaS)|identity+domain entities+policies+schedules+audit+evidence|pullers (external sync)+reconcilers (drift)+verifiers (SLA)+change-set executor+reaper+notification|none|web layer → OpsDB API|P01+P02+P03+P04+P21|AL01+AL02+AL03+AL04+AL06+AL07+AL08|AP01 (primary backend)

AS02|AC13 (E-commerce)|products+categories+inventory+pricing+customers+orders+policies+audit|catalog puller+inventory reconciler+order verifier+notification+reaper|checkout/cart service+payment gateway|catalog in OpsDB; cart service reads cached catalog; payment runner bridges|P01+P02+P03+P04+P11+P13|AL01+AL02+AL04+AL07+AL14+AL18+AL20+AL27|AP02 (split backend)

AS03|AC28 (HFT)|accounts+instruments+rules+positions+compliance+audit|config runner (pushes rules)+observation runner (pulls executions)+compliance verifier+regulatory reporter|matching engine|config runner formats rules for engine; observation runner pulls trade results|P01+P02+P03+P04+P10+P11+P13+P16+P21|AL01+AL03+AL04+AL05+AL26+AL27+AL38|AP03 (operational wrapper)
```

---

### Taxonomy Relationships

```
relationships(from|rel|to)
AC_ALL|composed_of|mechanisms from AF1-AF13
AC_ALL|requires|properties from P01-P21
AC_ALL|governed_by|principles from R01-R22
AC_ALL|positioned_at|AP01-AP04
AC_ALL|implements|algorithms from AL01-AL46
AP01|governed_state_pct|90-100%
AP02|governed_state_pct|70-90%
AP03|governed_state_pct|10-30%
AP04|governed_state_pct|5-10%
AL01-AL12|implemented_in|OpsDB components (API+search+versioning)
AL13-AL37|implemented_in|runners using library suite
AL38-AL46|implemented_in|external hot-path systems
AF1-AF13|maps_to|infrastructure taxonomy F1-F13
P01-P21|maps_to|infrastructure taxonomy P01-P21
R01-R22|maps_to|infrastructure taxonomy R01-R22
```

---

### Summary Statistics

```
counts:
  application_classes: 33 (12 governed-dominant + 9 split-backend + 7 hot-path-dominant + 5 specialized)
  mechanism_families_used: 13 (same as infrastructure taxonomy)
  properties_used: 21 (same as infrastructure taxonomy)
  principles_used: 22 (same as infrastructure taxonomy)
  algorithms_enumerated: 46 (6 state mgmt + 6 query + 5 scheduling + 6 financial + 5 reconciliation + 4 scoring + 5 integration + 9 hot-path)
  opsdb_positions: 4 (primary + split + wrapper + metadata)
  algorithms_in_opsdb: 27 (AL01-AL27 + AL33-AL37 implemented via gate pipeline + runners + library suite)
  algorithms_external: 9 (AL38-AL46 implemented in hot-path systems)
  algorithms_in_runners: 20 (AL13-AL37 minus those built into API)
  algorithms_in_api: 10 (AL01-AL04 + AL06-AL10 built into gate pipeline and search)
```

This taxonomy classifies every application type by the same three axes the infrastructure taxonomy uses for mechanisms. The mechanism profile determines what the application does. The property profile determines what guarantees it needs. The principle profile determines how it is assembled. The OpsDB position determines where the governed data substrate sits in the architecture. The algorithm enumeration identifies the computational patterns that runners, the API, and external systems implement.
