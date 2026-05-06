## Where OpsDB Application Architecture Fits

### Naturally Strong Fit — OpsDB As Primary Backend

**Business SaaS Products** — CRM, ERP, HR systems, project management, invoicing, subscription management, customer portals. These are the sweet spot. Every entity matters, history matters, access control matters, audit matters. The change set model maps directly to business workflows. Approval routing is a feature, not overhead. The read patterns are human-paced and cacheable. Multi-tenancy falls out of the authorization layers. OpsDB is the backend. Your web layer is a thin consumer.

**Internal Business Tools** — admin dashboards, inventory management, asset tracking, procurement, vendor management, employee onboarding systems. Same characteristics as SaaS but internal. The governance model prevents the "someone changed the spreadsheet and nobody knows who" problem that plagues internal tools. Lower stakes than customer-facing but the same architecture works with lighter approval policies.

**Compliance and Regulatory Platforms** — anything under SOC2, HIPAA, PCI-DSS, GDPR, SOX. This is OpsDB's home territory. Continuous evidence production, queryable audit trail, access review, retention policies, segregation of duties. The platform doesn't just support compliance — compliance is a native property of using it.

**Operational Platforms** — the original use case. Infrastructure management, cloud resource governance, on-call management, runbook systems, change management, configuration management databases. Every commitment in the design was made for this domain.

**Case Management Systems** — legal case tracking, support ticket backends, insurance claims, loan processing. These are entity-with-lifecycle systems where state transitions need approval, attribution, and audit. The change set lifecycle maps directly to case state machines. Evidence records map to case documentation.

**Healthcare Record Systems** — patient records, treatment plans, medication tracking, appointment scheduling. Every record is versioned, every access is audited, retention policies are regulatory, access control is per-field (diagnosis visible to doctors, billing codes visible to billing). The `_access_classification` field does the heavy lifting.

**Financial Services Backends** — account management, transaction governance, portfolio tracking, regulatory reporting. Not the transaction processing engine itself — the governance layer around it. Every account change is a change set. Every approval is attributed. Every state is reconstructable at any point in time for regulatory queries.

**Document and Knowledge Management Backends** — wiki backends, policy management, procedure tracking, contract management. Not storing the prose itself (that's the wiki anti-pattern) but managing the structured metadata: ownership, review dates, approval status, version history, access classification. With the draft mode flags, also suitable for the document content itself.

**Personal Data Platforms** — the home use case we discussed. Recipe collections, book trackers, household inventory, personal finance, journals, any structured personal data. Draft mode flags make the editing experience fluid. You own your data, your schema, your API.

**Education Platforms** — course management, student records, assignment tracking, grading. Versioned assignments, attributed grades, auditable grade changes, access-controlled student records. The approval workflow handles grade dispute resolution.

**Research Data Management** — experiment tracking, dataset cataloging, protocol versioning, lab inventory, sample tracking. Versioned protocols, attributed changes, evidence records for experiment results, retention policies per dataset.

### Suitable With Caveats — OpsDB As Part Of The Architecture

**E-commerce Product Catalogs** — product entities, categories, pricing, inventory levels. OpsDB handles the catalog management layer well: product definitions are change-managed, pricing changes go through approval, inventory adjustments are audited. But the cart and checkout flow needs a separate fast-path system. OpsDB governs the catalog, a lightweight service handles the transaction. OpsDB position: catalog backend, not transaction processor.

**Content Management Systems** — blog platforms, publishing systems, marketing site backends. The editorial workflow maps to change sets: draft, review, approve, publish. Versioning gives you full content history. But high-traffic content serving needs a CDN and cache layer in front. OpsDB is the editorial backend. Content delivery is a separate concern that reads from it.

**IoT Device Management** — device registration, configuration, firmware tracking, fleet management. The device registry and configuration management are strong fits. Firmware rollout is a change set with approval. But telemetry ingestion at device scale — thousands of devices reporting every second — overwhelms the gate pipeline. OpsDB manages the fleet. A time-series database handles telemetry. Pullers summarize telemetry into observation cache tables.

**API Gateway Configuration** — route definitions, rate limit policies, API key management, consumer registration. The configuration management is a natural fit. Every route change is governed and auditable. But the runtime request routing must not depend on OpsDB — that's the runtime dependency anti-pattern. OpsDB manages gateway configuration. The gateway caches it locally and operates independently.

**Workflow and Approval Systems** — beyond OpsDB's built-in change management. If your entire product is a workflow engine with arbitrary user-defined workflows, OpsDB's change set lifecycle is one specific workflow, not a general-purpose workflow engine. But OpsDB can be the data backend for a workflow engine: workflow definitions as versioned entities, workflow instances as state-tracked entities, transitions as change sets. OpsDB position: state backend, not workflow orchestrator.

**Multi-Player Turn-Based Games** — game state, player records, match history, leaderboards. Game state changes are essentially change sets. Turn history is version history. Match records are auditable. But real-time game state synchronization needs something faster. OpsDB is the persistent game state backend and match history. Real-time sync is a separate layer.

**Scheduling and Booking Systems** — appointment scheduling, resource booking, event management. The entities fit naturally. Booking conflicts are cross-field invariants in policy data. Calendar views are search queries. But double-booking prevention under high concurrency needs the optimistic concurrency control to be fast enough. Works well at moderate scale. At high contention — thousands of concurrent bookings for the same resource — you need a specialized reservation system. OpsDB position: booking management backend for moderate-scale systems.

**Supply Chain and Logistics** — shipment tracking, warehouse management, order fulfillment, supplier management. Entity modeling is strong. State transitions through the fulfillment pipeline map to change sets. Audit trail matters for disputes. But real-time tracking of moving vehicles needs a streaming system. OpsDB manages the supply chain data model. Real-time tracking feeds into it as observations.

### Not Suitable As Primary Backend — Different Tool Needed

**Real-Time Communication** — chat applications, video conferencing, live collaboration (Google Docs-style concurrent editing). The fundamental requirement is sub-millisecond message delivery between connected clients. The gate pipeline adds latency that makes real-time feel sluggish. Concurrent editing requires conflict-free replicated data types or operational transformation, not optimistic concurrency with change sets. Chat message history could live in OpsDB as observation data, but the real-time message bus is a separate system entirely.

**High-Frequency Transaction Processing** — payment processing, stock trading engines, ad auction systems, real-time bidding. These need microsecond latency, thousands of transactions per second through a single serialization point, and domain-specific consistency models. The ten gate steps are too heavy. The change set model is too slow. These systems need purpose-built transaction engines. OpsDB could govern the configuration and policies around them — trading rules, account limits, compliance policies — but not process the transactions.

**Real-Time Gaming** — FPS, MMO, competitive multiplayer. Game state updates at 60Hz. Physics simulation, collision detection, state interpolation. Nothing about the gate pipeline or the change set model makes sense here. These are specialized real-time systems with their own consistency models. OpsDB could manage player accounts, match history, and leaderboards, but the game server is a completely different architecture.

**Stream Processing and Event Pipelines** — Kafka-style event streaming, real-time analytics, CEP (complex event processing). These systems process millions of events per second with sub-second latency. Events are immutable, append-only, and processed in order. The observation cache tables could hold aggregated summaries, but OpsDB is not an event processing engine. Position: downstream consumer of aggregated stream results, not the stream processor.

**Machine Learning Training and Inference** — model training pipelines, feature stores, inference serving. Training is compute-bound with specialized data access patterns (batch reads of enormous datasets). Inference needs millisecond latency. Neither maps to the change set model. OpsDB could manage the ML operations metadata — experiment tracking, model registry, deployment approvals, A/B test configuration — but not the training or inference itself.

**Time-Series Databases** — metrics storage, monitoring data at full resolution, sensor data at high frequency. OpsDB explicitly refuses this (the monitoring replacement anti-pattern, O50). The observation cache tables hold summaries. Full-resolution time-series belongs in Prometheus, InfluxDB, TimescaleDB, or equivalent. OpsDB holds pointers to where the time-series live and cached summaries for operational queries.

**Search Engines and Full-Text Indexing** — document search, semantic search, vector search. OpsDB's search API supports structured filter predicates, not full-text or semantic search. If your application's core value is search — a search engine, a document discovery platform, a recommendation engine — you need Elasticsearch, Meilisearch, or a vector database. OpsDB could be the backend for the document metadata, with search indexes built from it.

**Large-Scale Media Storage and Delivery** — video streaming, image hosting, file sharing at scale. OpsDB stores structured data, not binary blobs. Media storage needs object stores (S3, GCS), CDNs, transcoding pipelines. OpsDB manages the media metadata: ownership, access control, version history, retention. The media itself lives elsewhere, referenced by authority pointers.

**Operating System and Embedded Systems** — kernel code, device drivers, firmware, RTOS. These are bare-metal, real-time, resource-constrained environments. Nothing about a Postgres-backed API server applies.

**Compilers, Interpreters, and Developer Tools** — IDEs, language servers, build systems, package managers. These are standalone tools with their own data models and performance requirements. Not a web service backend problem.

**High-Performance Scientific Computing** — simulation, numerical analysis, HPC workloads. These are compute-bound with specialized data access patterns. Not a governance problem.

### The Pattern

The dividing line is consistent. OpsDB is suitable when the core requirement is governed state management — entities that need validation, versioning, access control, audit, and structured relationships. It's unsuitable when the core requirement is raw throughput, real-time latency, or specialized computation.

When it's suitable, it's the backend. When it's partially suitable, it governs the configuration and metadata while a specialized system handles the hot path. When it's unsuitable, it might still manage the operational layer around the specialized system — experiment tracking around ML, fleet management around IoT, match history around gaming — but it's not in the critical path.

The question for any system is: is the primary challenge "managing governed state correctly" or "processing events/data/computation fast"? If the first, OpsDB is the backend. If the second, OpsDB manages the governance around whatever processes fast.
