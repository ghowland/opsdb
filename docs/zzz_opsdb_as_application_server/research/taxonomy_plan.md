## Master AppDB Taxonomy — Planning Document

### Title

**AppDB Application Taxonomy: Mechanisms, Properties, and Principles for Governed Application Development**

### Purpose

A single unified taxonomy covering every mechanism, property, and principle relevant to building applications on the OpsDB Application Architecture. Draws from three sources: the infrastructure taxonomy (operational origins), application-specific patterns (discovered through application domain analysis), and Silo patterns (discovered through data-only execution architecture analysis, filtered to what operates at the OpsDB abstraction layer).

The taxonomy is descriptive. It names the parts. It does not prescribe construction.

### Structural Approach

Same three-axis framework as the infrastructure taxonomy: mechanisms (building blocks performing work), properties (contracts claimed about mechanisms), and principles (rules governing assembly). The AppDB taxonomy inherits the infrastructure taxonomy's primitives and extends them with application-specific and Silo-derived additions. Each item is tagged with its origin so the reader knows whether it came from operational infrastructure, application domain analysis, or data-only execution architecture.

---

### Section 1: Introduction

State the three-axis framework. State the load-bearing claim: mechanism, property, and principle are distinct kinds of things. Same word commonly names all three. Conflation prevents precise comparison. Qualification resolves ambiguity.

State the scope: this taxonomy covers mechanisms, properties, and principles for applications built on a governed data substrate. It assumes the substrate provides schema-driven validation, five-layer authorization, change management with approval routing, full-state versioning, append-only audit logging, and a bounded search API. The taxonomy names what applications built on this substrate use, require, and obey.

State the three sources and their relationship: infrastructure taxonomy provides the foundation, application analysis extends it for domain-specific patterns, Silo analysis extends it for data-only execution patterns available at the application layer.

---

### Section 2: Terminology and Qualification

Inherit from infrastructure taxonomy: mechanism is unit of work-doing, property is a contract, principle is a rule. Always qualify ambiguous words.

Add application-specific qualification: "application layer" qualifier for properties and principles that hold within the OpsDB abstraction boundary but not necessarily below it (Go runtime, Postgres, OS). This is the honest scoping that the Silo analysis surfaced.

Define the abstraction boundary: everything above the storage engine and the HTTP transport layer. Schema engine, gate pipeline, change management, versioning, audit logging, runner execution, library suite, and search API operate within this boundary. The Go runtime, Postgres query executor, operating system, and network transport operate below it.

---

### Section 3: Mechanism Axis

#### 3.1 Inherited Mechanisms

Inherit all 62 mechanisms from 13 families in the infrastructure taxonomy. These are the building blocks. Applications compose them. Reference the infrastructure taxonomy for definitions, subtypes, and composition relationships.

Organize by family with application-relevant annotations: which mechanisms are used directly by the OpsDB substrate (gate pipeline, versioning, audit), which are used by runners through the library suite (world-side operations), which are used by hot-path systems connected via runners, and which are unused in typical application architectures.

Table format per family:

```
inherited_mechanisms(id|name|family|opsdb_component|runner_use|hot_path_use|app_relevance)
```

Cover all 13 families: Information Movement, Selection, Representation, Storage, Versioning, Lifecycle, Sensing, Control Loop, Gating, Allocation, Coordination, Transformation, Resilience.

#### 3.2 Application-Specific Mechanisms

Four new mechanisms from application domain analysis:

```
app_mechanisms(id|name|family_home|definition|origin|used_by|opsdb_implementation)
```

**AM01 — State Machine Evaluator.** Gating family (F9). Takes declared states, declared transitions, current state, and proposed transition. Accepts or rejects the transition based on the declared lifecycle graph. Distinct from Reconciler (which converges) and Validator (which checks field values). Applications have entities with lifecycles where transition legality matters. Origin: application analysis. Implementation: policy rows declaring valid transitions evaluated at gate step 5, or a dedicated runner evaluating transition legality before change set submission.

**AM02 — Rule Engine.** Gating family (F9) or new family. Evaluates a population of declared rules against input data and produces a decision, a matching set, or a computed value. Distinct from Validator (schema-bound), Authorizer (identity-bound), Selector (finds matching data given rules — Rule Engine finds matching rules given data). Origin: application analysis. Implementation: policy rows with predicate structures evaluated at gate step 5 for validation rules, or runner-side evaluation for business logic rules.

**AM03 — Accumulator.** Transformation family (F12). Maintains a running aggregate updated incrementally on the write path — sum, count, min, max, weighted average. Distinct from Counter (observation-only) and Gauge (current value report). Accumulator participates in the write path and persists derived quantities. Origin: application analysis. Implementation: runner computes and writes derived values as part of change set or observation write, or gate pipeline extension that computes derived fields on commit.

**AM04 — Temporal Projection.** Transformation family (F12). Expands a schedule or recurrence rule into concrete instances over a time range. Input: compact rule (cron expression, recurrence definition, calendar anchor). Output: list of concrete datetime instances. Distinct from Scheduler (which assigns work to time slots) and TTL (which marks expiration). Origin: application analysis. Implementation: runner or API extension that projects schedule entities into concrete instance lists for display, conflict detection, and availability computation.

#### 3.3 Silo-Derived Mechanisms

Eight new mechanisms from Silo analysis filtered to OpsDB application layer:

```
silo_mechanisms(id|name|family_home|definition|origin|opsdb_implementation|abstraction_layer)
```

**SM01 — Execution Funnel.** New pattern (cross-family). Composes heterogeneous mechanism types into a monotonic narrowing sequence where each layer uses a different computational model. Distinct from Pipeline (general) and from the gate pipeline (which uses the same model — policy check — at each step). Origin: Silo execution pipeline. OpsDB implementation: gate pipeline is a variant; extension point for composing structural validation, predicate logic, utility scoring, and atomic execution as distinct computational layers. Abstraction layer: application.

**SM02 — Fact Regeneration.** Sensing family (F7) extension. Converts imperative state into declarative fact set on every evaluation cycle. Decisions always use current-cycle facts. Distinct from Probe (synthetic test) and Cache (holds copies). Origin: Silo prolog subsystem. OpsDB implementation: runner get-phase re-reads current state each cycle; formalized extension would declare fact transformation from entity state to decision-input facts. Abstraction layer: application.

**SM03 — Multiplicative Scoring with Zero-Gating.** Selection family (F2) extension. Running score starts 1.0, each consideration multiplies in, any zero immediately zeros total. Fail-fast elimination. Average-and-fixup compensation for many-consideration bias. Distinct from Ranker (orders by score without zero-gating algebra). Origin: Silo utility AI. OpsDB implementation: extension point for approval rule evaluation with weighted multi-factor scoring. Abstraction layer: application.

**SM04 — Semantic Repurposing.** Representation family (F3) extension. Same structural schema serves multiple unrelated domains through data reinterpretation alone. Infrastructure has no domain knowledge. Meaning is in data not types. Distinct from Namespace (naming scope) and Schema (structure description). Origin: Silo data-only architecture. OpsDB implementation: the AppDB model itself — same infrastructure serves billing, healthcare, personal data through schema alone. Abstraction layer: application.

**SM05 — Hot-Swap via Data Replacement.** Control Loop family (F8) extension. Behavior definition replaced at runtime through data, effective on next evaluation cycle. No restart, no recompile, no deployment. Distinct from Reconciler (which converges toward desired state — hot-swap replaces the definition of desired state). Origin: Silo data-only architecture. OpsDB implementation: data-driven behavior model — policy, approval rules, access control, runner config change on change set apply. Governance wraps the hot-swap with attribution and reversibility. Abstraction layer: application.

**SM06 — Scene as Isolation Boundary.** Coordination family (F11) extension. Complete execution context with own entity pool, behavior definitions, and integrated access control. Combines Bulkhead isolation with Authorizer-style access control. Default deny. Origin: Silo scene subsystem. OpsDB implementation: AppDB model — one DOS per application with own schema, API, database, runners, policies. Five-layer authorization within. Cross-AppDB access via typed pointers with policy-mediated federation. Abstraction layer: application.

**SM07 — Structured Trace.** Sensing family (F7) extension. Captures complete decision chain per entity per evaluation cycle in domain-aware structured form. Debug in domain terms. Replay with modified rules to compare outcomes. Distinct from Log (unstructured record) and Watch (change subscription). Origin: Silo trace subsystem. OpsDB implementation: audit log plus version history provides data substrate; extension point for per-change-set decision trace with gate pipeline evaluation details and policy replay. Abstraction layer: application.

**SM08 — Ingress Validation (Partial Geometric Security).** Gating family (F9) extension. Structural validation rejects non-conforming requests before semantic interpretation. Closed vocabulary prevents operation type injection. Fixed field types prevent type confusion. No regex. No embedded logic. Input never interpreted as logic at application layer, only as structure. Origin: Silo partial geometric security. OpsDB implementation: gate pipeline steps 3 and 4 before step 5. Closed schema vocabulary. Runner report key enforcement. Abstraction layer: application (does not control transport parsing beneath HTTP/JSON).

#### 3.4 Mechanism Summary Table

```
mechanism_counts(origin|count|families_extended)
Infrastructure taxonomy (inherited)|62|13 families
Application analysis (new)|4|F9 Gating (2) + F12 Transformation (2)
Silo analysis (new)|8|F2 Selection (1) + F3 Representation (1) + F7 Sensing (2) + F8 Control Loop (1) + F9 Gating (1) + F11 Coordination (1) + cross-family (1)
Total|74|13 families (no new families added)
```

---

### Section 4: Property Axis

#### 4.1 Inherited Properties

Inherit all 21 properties from 4 bands in the infrastructure taxonomy. Reference the infrastructure taxonomy for definitions, conditions, partial provision, and orthogonality.

Add application-relevant annotations for each: how OpsDB provides or supports it, which gate pipeline steps contribute, which runner disciplines contribute.

Table format:

```
inherited_properties(id|name|band|opsdb_provision|gate_steps|runner_contribution)
```

#### 4.2 Application-Specific Properties

Three new properties from application domain analysis:

```
app_properties(id|name|band|claim|conditions|origin|opsdb_implementation)
```

**AP01 — Lifecycle Integrity.** Data Integrity band (B1). Claims that an entity's state transitions conform to a declared lifecycle graph at all times. No entity occupies a state that was not reached through a valid transition sequence. Distinct from Consistency-data (field-level constraints) because Lifecycle Integrity is about transition path legality, not field value validity. Origin: application analysis. OpsDB implementation: state machine evaluator mechanism (AM01) combined with version history that records the transition sequence, plus policy rows declaring valid transitions.

**AP02 — Computational Correctness.** Data Integrity band (B1). Claims that derived quantities — balances, totals, inventory levels, scores — are consistent with their source data at specified boundaries. The derived value equals what recomputation from source data would produce. Distinct from Consistency-data (schema constraints) because Computational Correctness is about derived values matching their derivation function. Origin: application analysis. OpsDB implementation: verifier runners that periodically recompute derived quantities from source data and compare against persisted values, producing evidence records on match or compliance findings on mismatch.

**AP03 — Non-repudiation.** Data Integrity band (B1). Claims that a party who performed an action cannot deny having performed it. Requires non-transferable authentication (no shared credentials), tamper-evident audit trail, and preserved cryptographic evidence. Distinct from Authenticity (verifies identity at request time) and Auditability (reconstructs past operations) because Non-repudiation claims the evidence is sufficient to prevent denial. Origin: application analysis. OpsDB implementation: SSO delegation with per-user credentials (no shared accounts anti-pattern), append-only audit log with optional cryptographic chaining, version history linking every state to the change set and identity that produced it.

#### 4.3 Silo-Derived Properties

Four new properties from Silo analysis filtered to OpsDB application layer:

```
silo_properties(id|name|band|claim|origin|opsdb_scope|abstraction_qualifier)
```

**SP01 — Vocabulary Closure (Application Layer).** Behavioral band (B2). Claims that the operation vocabulary at the application layer is a closed finite set that cannot be extended at runtime. Nine schema types, three modifiers, six constraints, sixteen API operations, ten gate steps, three runner gating modes. No schema file, policy row, or runner configuration can introduce new operation types. Holds within OpsDB abstraction boundary. Does not hold below it (Go and Postgres execute arbitrary operations). Origin: Silo vocabulary limitation.

**SP02 — Structural Security (Application Layer).** Data Integrity band (B1). Claims that at the application validation layer, security properties derive from absence of mechanism rather than policy enforcement. The closed vocabulary means injection of new operation types cannot be expressed. Absence of regex means catastrophic backtracking cannot occur. Absence of embedded logic means arbitrary code execution through schema files cannot occur. Holds within OpsDB abstraction boundary. Origin: Silo geometric security.

**SP03 — Hot-Swap Safety (Governed).** Behavioral band (B2). Claims that behavior definitions can be replaced at runtime with zero disruption and full governance — attribution, approval, versioning, audit. New behavior takes effect on change set apply. Old behavior preserved in version history. Extends Stability under Change (P18) with the specific claim about behavior definition replacement. Origin: Silo hot-swap pattern.

**SP04 — Domain Opacity.** Behavioral band (B2). Claims that the infrastructure contains no application-domain knowledge. The gate pipeline, schema engine, versioning system, change management, library suite, and search API are domain-agnostic. Domain semantics exist only in schema YAML and data rows. The same binary infrastructure serves any application. Origin: Silo data-only architecture.

#### 4.4 Borderline Properties (Documented, Not Promoted)

Two properties identified but not promoted to full status. Documented for reference.

**BP01 — Temporal Consistency.** Correct time-dependent computation under timezone variation, DST transitions, and clock skew. Could be a condition on Determinism (P08). Frequent enough in booking, billing, and scheduling applications to warrant monitoring as a candidate for future promotion.

**BP02 — Fairness.** Equitable treatment of competing parties under declared rules. Could be a condition on specific mechanisms (auction, matchmaking, scheduling). Product requirement more than system property. Documented as mechanism-level concern rather than system-level property.

#### 4.5 Property Summary Table

```
property_counts(origin|count|bands_extended)
Infrastructure taxonomy (inherited)|21|4 bands
Application analysis (new)|3|B1 Data Integrity (3)
Silo analysis (new)|4|B1 Data Integrity (1) + B2 Behavioral (3)
Total|28|4 bands (no new bands added)
```

---

### Section 5: Principle Axis

#### 5.1 Inherited Principles

Inherit all 22 principles from 6 groups in the infrastructure taxonomy. Reference the infrastructure taxonomy for definitions, reasoning, domains, and counter-principles.

Add application-relevant annotations: which are universal to all applications on the substrate, which are class-specific, and how each manifests in OpsDB Application Architecture.

Table format:

```
inherited_principles(id|name|group|universal_or_class_specific|opsdb_manifestation)
```

#### 5.2 Application-Specific Principles

Three new principles from application domain analysis:

```
app_principles(id|name|group|rule|reasoning|origin|opsdb_implementation)
```

**NR01 — Separate Domain Logic from Infrastructure Logic.** Dependency group (G4). Domain logic (billing computation, eligibility determination, move validation) must be isolated from infrastructure logic (retry, auth, serialization, logging). Domain logic changes at domain pace. Infrastructure logic changes at platform pace. Origin: application analysis. OpsDB implementation: domain logic lives in runners and policy data. Infrastructure logic lives in the gate pipeline and library suite. Runners call library functions. Libraries do not call runner logic.

**NR02 — Express Business Rules as Data.** Data/Logic group (G1). Business rules — pricing logic, eligibility criteria, approval requirements, lifecycle transitions, scoring weights — should be expressed as data rows evaluated by a mechanical engine, not as code. Extends R01 (Data Primacy) from configuration to business rules. Origin: application analysis. OpsDB implementation: policy rows for validation rules, approval rule rows for routing, schedule entities for temporal rules, runner spec data for automation rules. The rule engine mechanism (AM02) evaluates them.

**NR03 — Separate Read Models from Write Models.** Dependency group (G4). When read patterns diverge from write patterns, maintain separate read models derived from the governed write model. The read model is a cache. The write model is the source of truth. Inconsistency is a freshness problem not a correctness problem. Distinct from R17 (Local Cache + Global Truth) which is about locality, not shape divergence. Origin: application analysis. OpsDB implementation: observation cache tables written by runners that denormalize governed entities into read-optimized shapes. Search API serves both governed entities and cache tables.

#### 5.3 Silo-Derived Principles

Five new principles from Silo analysis filtered to OpsDB application layer:

```
silo_principles(id|name|group|rule|reasoning|origin|opsdb_implementation|abstraction_qualifier)
```

**SN01 — Security by Anatomy at the Application Layer.** Failure/Resilience group (G3). At the application validation layer, security derives from structural limitation of what the system can express, not from rules about who can do what. A misconfigured policy cannot introduce regex because the vocabulary does not contain regex. A compromised runner cannot write outside declared scope because both API and library validate declarations. Distinct from R09 (Fail Closed) which is a behavioral policy. SN01 is a structural property. Origin: Silo geometric security. Abstraction qualifier: holds within OpsDB boundary.

**SN02 — Vocabulary Restriction at the Application Layer.** Scale/Cardinality group (G2). The set of possible operations is finite and fixed at the application layer. Nine types, three modifiers, six constraints, sixteen operations, ten gate steps. Adding a primitive is a specification revision. Extends R11 (Bound Everything) from quantity bounds to vocabulary bounds. Origin: Silo vocabulary limitation. Abstraction qualifier: holds within OpsDB boundary.

**SN03 — Shape Before Meaning at the API Layer.** Failure/Resilience group (G3). Structural validation precedes semantic interpretation. Gate steps 3-4 (schema and bound validation) run before step 5 (policy and semantic evaluation). Structurally non-conforming input is rejected before the system evaluates what it means. More specific than R09 (Fail Closed). Addresses injection, type confusion, and constraint violation. Origin: Silo partial geometric security. Abstraction qualifier: application layer only (transport parsing beneath HTTP is not controlled).

**SN04 — Infrastructure Fix Protects All Consumers.** Dependency/Structure group (G4). Shared infrastructure means shared fixes. A bug fixed in the gate pipeline, schema engine, or library suite is fixed for every AppDB simultaneously. Justifies investment in comprehensive substrate over per-application custom code. Origin: Silo tall-infra amortization. OpsDB implementation: single codebase for gate pipeline, schema engine, library suite serving all AppDBs.

**SN05 — Marginal Cost of New Behavior Approaches Zero.** Operator/Relationship group (G6). Adding the 1000th entity or the 1000th runner costs the same engineering effort as the 10th. No new API endpoints, validation code, authorization logic, or audit handling. Write YAML, run loader. Extends R22 (Removing Classes of Work) from operational automation to development architecture. Origin: Silo marginal cost observation. OpsDB implementation: schema engine provides full property set per entity. Library suite provides full infrastructure per runner.

#### 5.4 Principle Summary Table

```
principle_counts(origin|count|groups_extended)
Infrastructure taxonomy (inherited)|22|6 groups
Application analysis (new)|3|G1 Data/Logic (1) + G4 Dependency (2)
Silo analysis (new)|5|G2 Scale (1) + G3 Failure (2) + G4 Dependency (1) + G6 Operator (1)
Total|30|6 groups (no new groups added)
```

---

### Section 6: Application Class Profiles

Reproduce the 33 application classes from the application taxonomy. Each class annotated with its complete profile across all three axes using the unified numbering from this taxonomy.

Table format:

```
app_class_profile(id|name|position|dominant_mechanisms|critical_properties|governing_principles|key_algorithms)
```

Group by OpsDB architecture position: primary backend (AP01), split backend (AP02), operational wrapper (AP03), metadata manager (AP04).

---

### Section 7: Algorithm Enumeration

Reproduce the 46 algorithms from the application taxonomy. Each tagged with which mechanisms it composes, which properties it contributes to, and where it is implemented (gate pipeline, runner, library suite, or external hot-path system).

Table format:

```
algorithm_profile(id|name|composed_mechanisms|contributes_to_properties|implemented_in|origin)
```

Organized by category: state management, query and discovery, scheduling and temporal, financial and metering, reconciliation and verification, scoring and ranking, integration, hot-path (external).

---

### Section 8: OpsDB Component to Taxonomy Mapping

Map every OpsDB component to the taxonomy items it implements.

```
component_mapping(component|mechanisms_implemented|properties_provided|principles_enforced)
```

Components: schema engine, gate pipeline (per step), change management system, versioning system, audit log, search API, runner pattern, library suite (per library family), observation cache, retention system, report key enforcement.

This section is the bridge from abstract taxonomy to concrete implementation. A developer reading the taxonomy can trace from any mechanism, property, or principle to the OpsDB component that implements it.

---

### Section 9: Abstraction Layer Scoping

Enumerate which taxonomy items hold at which abstraction layer. Three layers:

**Full stack.** Items that hold from hardware through application. Only items inherited from the infrastructure taxonomy that the storage engine and OS provide (durability via Postgres WAL, availability via OS process management).

**OpsDB application layer.** Items that hold within the OpsDB abstraction boundary — schema engine, gate pipeline, change management, versioning, audit, runners, library suite. This is where most application-specific and Silo-derived items operate. Items qualified with "application layer" hold here.

**External hot-path systems.** Items that hold within specialized systems connected via runners. OpsDB does not claim these properties. The hot-path system claims them. OpsDB governs the configuration around them.

Table format:

```
abstraction_scoping(id|name|axis|full_stack|opsdb_layer|external_hot_path|notes)
```

This section makes the honest scoping explicit. Vocabulary Closure holds at the OpsDB layer but not below. Structural Security holds at the API validation layer but not at the transport parsing layer. Domain Opacity holds for the OpsDB binary but each AppDB's schema YAML is domain-specific by definition.

---

### Section 10: Cross-Reference Tables

#### 10.1 Mechanism to Property Coverage

Which mechanisms provide which properties. Same structure as infrastructure taxonomy's family-property coverage table, extended with application and Silo mechanisms.

```
mechanism_property_coverage(mechanism|primary_properties|secondary_properties)
```

#### 10.2 Principle to Mechanism Selection

Which principles govern the selection among mechanisms when multiple could provide a required property. Same structure as infrastructure taxonomy's triangle relationships.

```
principle_mechanism_selection(principle|selects_among|reasoning)
```

#### 10.3 Property to Application Class Requirements

Which properties are critical, important, or optional for each application class. Full matrix.

```
property_requirements(app_class|critical|important|optional)
```

#### 10.4 Impossible Combinations

Impossibility triplets inherited from infrastructure taxonomy plus any new ones discovered through application and Silo analysis.

```
impossibility_triplets(id|properties|observation|application_relevance)
```

New candidate: Lifecycle Integrity + Hot-Swap Safety + Immediate Consistency — replacing behavior definitions that include lifecycle transition rules while maintaining lifecycle integrity requires that the replacement is atomic with respect to any in-flight transitions.

---

### Section 11: Failure Modes

Failure modes relevant to applications built on the substrate. Inherit infrastructure taxonomy failure modes and add application-specific ones.

```
app_failure_modes(id|failure|properties_lost|properties_preserved|application_impact)
```

Application-specific additions: stale derived quantity (Computational Correctness lost), invalid lifecycle transition (Lifecycle Integrity lost), policy misconfiguration (correct properties depend on correct policy data), runner scope over-provisioning (Structural Security weakened at runner layer), draft mode data loss (version history granularity reduced for draft-mode tables).

---

### Section 12: Governance Flag Impact Analysis

Map each governance flag to its impact on every property in the taxonomy. Same structure as the property loss matrix from the second addendum, extended to cover all 28 properties.

```
flag_property_impact(flag|property|impact|notes)
```

Impact values: preserved, weakened, lost. Every flag-property combination documented.

---

### Section 13: Pattern Relationships

Full relationship graph across all three axes. Inherit infrastructure taxonomy relationships and extend with application and Silo relationships.

```
relationships(from|rel|to|origin)
```

Relationship types inherited: provides, requires, selects\_among, constrains\_realization\_of, grounds, enables, opposes, prevents, implements, composes, spans, and others from infrastructure taxonomy.

New relationship types if needed for application or Silo patterns.

---

### Section 14: Excluded Items

Items considered and excluded with reasoning. The ring-0 Silo patterns, the borderline application properties, and any patterns that were evaluated but determined to be compositions of existing primitives rather than new taxonomy items.

```
excluded(id|name|origin|reason|alternative)
```

---

### Section 15: Summary Statistics

```
totals:
  mechanisms: 74 (62 inherited + 4 application + 8 silo)
  properties: 28 (21 inherited + 3 application + 4 silo)
  principles: 30 (22 inherited + 3 application + 5 silo)
  mechanism_families: 13 (no new families)
  property_bands: 4 (no new bands)
  principle_groups: 6 (no new groups)
  application_classes: 33
  algorithms: 46
  opsdb_components_mapped: 12
  abstraction_layers: 3
  governance_flags_analyzed: 3 current + 2 hypothetical
  excluded_items: 7 (5 ring-0 silo + 2 borderline properties)
```

---

### Appendices

**A — Full Mechanism Reference.** All 74 mechanisms with definition, family, subtypes, composition relationships, OpsDB implementation, and origin tag.

**B — Full Property Reference.** All 28 properties with definition, band, conditions, partial provision, OpsDB provision, and origin tag.

**C — Full Principle Reference.** All 30 principles with definition, group, reasoning, counter-principles, OpsDB manifestation, and origin tag.

**D — Application Class Reference.** All 33 classes with complete profile (mechanisms, properties, principles, algorithms, OpsDB position).

**E — Algorithm Reference.** All 46 algorithms with description, mechanism composition, property contribution, implementation location, and application class usage.

**F — OpsDB Component Mapping.** Every component mapped to taxonomy items.

**G — Abstraction Layer Scoping.** Every taxonomy item mapped to the layers where it holds.

**H — Governance Flag Impact Matrix.** Every flag against every property.

**I — Excluded Items.** Every item considered and rejected with reasoning.

**J — Relationship Graph.** Full cross-axis relationship table.

---

### Format

Pipe-delimited tables in LLM-compact form as companion artifact. Cross-references to infrastructure taxonomy by original ID. Cross-references to OpsDB spec series by document and section. Origin tag on every item: `infra` for infrastructure taxonomy, `app` for application analysis, `silo` for Silo analysis. Abstraction layer qualifier on every Silo-derived item.

### Dependencies

Assumes reader has access to: the infrastructure taxonomy for inherited definitions, the OpsDB spec series for substrate details, the OpsDB Application Architecture paper for application patterns, and the Silo infrastructure specification for data-only execution patterns.
