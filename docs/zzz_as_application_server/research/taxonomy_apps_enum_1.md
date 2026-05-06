Good question. Let me walk through systematically.

## Method

The infrastructure taxonomy was built from the operational perspective. It covers mechanisms for moving, storing, selecting, transforming, coordinating, gating, sensing, and protecting data in production infrastructure. The question is whether application development introduces mechanical patterns, property requirements, or construction principles that the operational taxonomy does not already name.

---

## Mechanisms: What Applications Add

The 62 mechanisms across 13 families cover the infrastructure layer comprehensively. Most application patterns compose existing mechanisms rather than introducing new ones. But several application-specific mechanical patterns do not have clean homes in the existing families.

### New Mechanism Candidates

```
new_mechanisms(id|name|family_candidate|def|used_by|distinct_from|status)
```

**NM01 — State Machine Evaluator**
A mechanism that takes a declared set of states, a declared set of valid transitions, a current state, and a proposed transition, and accepts or rejects the transition. Distinct from Reconciler (which converges toward desired state) and Reactor (which fires on events). The state machine evaluator is a gating mechanism that enforces declared lifecycle rules. Configuration management systems don't need this because they converge to declared state. Applications need it because entities have lifecycles with rules about which transitions are permitted from which states: a case cannot go from "closed" to "in progress" without reopening, an invoice cannot go from "paid" to "draft."

- Family candidate: F9 (Gating) — it decides whether a transition is permitted
- Distinct from: Validator (checks field values), Authorizer (checks identity permissions), Reconciler (converges state)
- Used by: AC03 (case management), AC13 (e-commerce order), AC14 (editorial workflow), AC17 (shipment), AC18 (workflow engine), AC21 (subscription/billing), AC19 (game turn)
- Status: **new mechanism** — the operational taxonomy does not name lifecycle transition validation as a distinct mechanism. Operational systems converge to declared state. Application systems enforce declared transition graphs.

**NM02 — Rule Engine**
A mechanism that evaluates a set of declared rules against input data and produces a decision or a set of matching rules. Distinct from Validator (which checks against schema bounds) and Authorizer (which checks permissions). A rule engine evaluates domain-specific business logic expressed as data: pricing rules, eligibility criteria, game move validation, fraud scoring predicates, tax jurisdiction selection. The rules are data rows, not code. The engine evaluates them mechanically.

- Family candidate: F9 (Gating) or new family — it evaluates declared predicates but the output is not just accept/reject. It can be a score, a matching set, a selected rule, a computed value.
- Distinct from: Validator (schema-bound), Authorizer (identity-bound), Filter (per-message accept/drop), Selector (returns subset matching predicate — close but Selector operates on data populations, Rule Engine operates on rule populations against one input)
- Used by: AC05 (trading rules), AC13 (pricing/tax/eligibility), AC19 (move validation), AC21 (billing rules), AC27 (targeting predicates), AC28 (risk limits)
- Status: **new mechanism** — the operational taxonomy's Selector is the closest analog but operates in the wrong direction. Selector: given rules, find matching data. Rule Engine: given data, find matching rules and compute their output.

**NM03 — Accumulator**
A mechanism that maintains a running aggregate — sum, count, min, max, weighted average — updated incrementally as events arrive, without re-reading the full dataset. Distinct from Counter (which only counts) and Gauge (which reports current value). An Accumulator maintains a derived quantity that is updated incrementally: account balance, usage meter, inventory level, budget remaining, revenue recognized to date. The infrastructure taxonomy has Counter and Gauge in the Sensing family, but these are observation mechanisms. An Accumulator is a stateful computation mechanism that participates in the write path.

- Family candidate: F12 (Transformation) or F4 (Storage) — it transforms incrementally and stores the result
- Distinct from: Counter (observation, not write-path), Gauge (current value, not derived), Compactor (reduces redundancy, not computing aggregate)
- Used by: AC05 (position tracking), AC11 (inventory levels), AC12 (budget tracking), AC13 (cart totals), AC21 (usage metering, revenue recognition), AC27 (budget pacing), AC28 (position tracking)
- Status: **new mechanism** — operational systems observe metrics. Application systems compute and persist derived quantities on the write path. The Accumulator is a write-path computation mechanism the operational taxonomy does not name.

**NM04 — Temporal Projection**
A mechanism that expands a schedule or recurrence rule into concrete instances over a time range. Given a cron expression, a recurrence rule, or a calendar-anchored schedule, it produces the set of concrete datetime instances within a window. Distinct from Scheduler (which assigns work to time slots) and TTL (which marks expiration). Temporal Projection is a computation: recurrence rule in, instance list out.

- Family candidate: F12 (Transformation) — it transforms a compact rule into an expanded set
- Distinct from: Scheduler-control (decides when work runs, not when instances exist), TTL (expiration, not expansion)
- Used by: AC07 (class schedule), AC15 (booking availability), AC06 (compliance audit schedule), AC21 (billing cycle dates)
- Status: **new mechanism** — operational scheduling uses cron and rate-based triggers. Applications need to project schedules into calendars for display, conflict detection, and availability computation. The operational taxonomy's Scheduler executes work. Temporal Projection computes when things would occur without executing anything.

**NM05 — Conflict Detector**
A mechanism that identifies overlapping or contradictory claims on a shared resource within a time range or constraint space. Given a proposed claim (time interval, resource, quantity) and a set of existing claims, it determines whether the proposal conflicts. Distinct from Comparator (which compares two things for equality/ordering) and Lock (which serializes access). Conflict Detector operates over a population of claims in a constraint space.

- Family candidate: F2 (Selection) or F9 (Gating) — it selects conflicting claims, and the result gates acceptance
- Distinct from: Comparator (pairwise), Lock (serialization), Validator (field-level bounds)
- Used by: AC15 (booking conflicts), AC07 (schedule conflicts), AC24 (resource contention), AC18 (parallel branch resource conflicts)
- Status: **borderline** — could be modeled as a Selector query followed by a Validator check. But the combined pattern — find all claims in this resource-time space and determine if the new one fits — is common enough in applications and absent enough in operations to warrant naming.

**NM06 — Proration Calculator**
A mechanism that computes partial-period charges or allocations given a rate, a period, and a fractional usage window.

- Status: **not a mechanism** — this is a formula, not a mechanism. It is a specific instance of Transformer applied to billing data. Does not warrant separate naming.

**NM07 — Delegation Chain**
A mechanism that transfers authority from one party to another with constraints, forming a chain that can be walked to determine effective authority. Distinct from Authorizer (which checks permission) and Election (which selects a leader). Delegation creates transitive authority relationships.

- Family candidate: F9 (Gating)
- Distinct from: Authorizer (checks, does not delegate), Election (selects one, does not chain)
- Used by: AC18 (workflow delegation), AC03 (case reassignment), AC01 (approval delegation during absence)
- Status: **borderline** — operational systems handle delegation through group membership changes. Applications formalize delegation as a first-class chain with time bounds and constraints. Whether this is a new mechanism or a pattern built from Authorizer + Lease + History depends on how frequently the pattern recurs.

---

## Properties: What Applications Add

The 21 properties cover data integrity, behavioral, distribution, and operational bands. Most application requirements map to existing properties. But several application-specific guarantees do not have clean homes.

### New Property Candidates

```
new_properties(id|name|band_candidate|claim|conditions|distinct_from|status)
```

**NP01 — Lifecycle Integrity**
The claim that an entity's state transitions conform to a declared lifecycle graph at all times. No entity can be in a state that was not reached through a valid transition sequence from the initial state. Distinct from Consistency-data (which claims constraints hold) because Consistency-data is about field values, not about transition sequences. A row could have valid field values in state "paid" while having arrived there through an invalid transition (skipping "approved").

- Band candidate: B1 (Data Integrity)
- Conditions: lifecycle graph declared; transitions enumerated; current state and prior state tracked
- Distinct from: Consistency-data (field-level constraints), Ordering (sequence of operations, not validity of transitions)
- Used by: AC03, AC13, AC14, AC17, AC18, AC21
- Status: **new property** — operational systems converge to declared state and do not track transition legality. Application systems have entities whose lifecycle history matters: was this invoice properly approved before being paid? The state value might be valid while the path to it was not.

**NP02 — Temporal Consistency**
The claim that time-dependent computations — scheduling, billing, expiration, deadline enforcement — produce correct results under timezone variation, daylight saving transitions, leap seconds, and clock skew within a declared tolerance. Distinct from Determinism (which claims same inputs produce same outputs) because temporal consistency is specifically about time representation and computation, not general reproducibility.

- Band candidate: B2 (Behavioral)
- Conditions: timezone handling declared; DST transition behavior specified; clock source specified
- Distinct from: Determinism (general), Ordering (sequence, not temporal correctness)
- Used by: AC07, AC15, AC21, AC06
- Status: **borderline** — could be a specific condition on Determinism ("deterministic given identical clocks"). But the frequency of timezone bugs in applications and the specificity of the failure modes (billing computed in wrong timezone, booking shown at wrong time, deadline triggered early due to DST) suggest it warrants naming as a distinct property. Operations encounters clock skew as a failure mode. Applications encounter timezone computation as a correctness requirement.

**NP03 — Computational Correctness**
The claim that derived quantities — account balances, inventory levels, billing totals, ratings, scores — are consistent with their source data at specified boundaries. The derived value equals what you would get by recomputing from scratch against the source data. Distinct from Consistency-data (which claims declared constraints hold) because computational correctness is about derived values matching their derivation, not about schema constraints.

- Band candidate: B1 (Data Integrity)
- Conditions: derivation function declared; source data specified; boundaries specified (per-write, periodic, eventual)
- Distinct from: Consistency-data (schema constraints), Determinism (reproducibility, not correctness of derivation)
- Used by: AC05, AC11, AC12, AC13, AC21, AC28
- Status: **new property** — operational systems generally do not maintain derived quantities. They observe external state and cache it. Applications compute and persist derived quantities (balances, totals, scores) that must remain consistent with their sources. The property claim "this balance equals the sum of all transactions" is not a schema constraint and is not covered by existing properties.

**NP04 — Presentation Consistency**
The claim that different consumers viewing the same data at the same time see the same values, accounting for access control filtering. Distinct from Consistency-replica (which is about replica agreement on values) because Presentation Consistency is about what the application presents after access control, caching, and projection.

- Status: **not a new property** — this is Consistency-replica composed with Authorization. The existing properties cover it. Not warranted.

**NP05 — Non-repudiation**
The claim that a party who performed an action cannot deny having performed it. Distinct from Authenticity (which verifies identity) and Auditability (which reconstructs past operations). Non-repudiation specifically claims that the evidence is sufficient to prevent denial, which requires the audit trail to be tamper-evident and the authentication to be non-transferable.

- Band candidate: B1 (Data Integrity)
- Conditions: authentication non-transferable (no shared credentials); audit trail tamper-evident; cryptographic evidence preserved
- Distinct from: Authenticity (verifies identity at request time), Auditability (reconstructs, does not prevent denial)
- Used by: AC04 (medical record authorship), AC05 (trade execution), AC06 (compliance attestation), AC18 (approval actions)
- Status: **new property** — the operational taxonomy lists Authenticity and Auditability separately. Non-repudiation is the composed claim that the audit trail, combined with the authentication mechanism, produces evidence sufficient to prevent denial. Operational systems care about attribution. Application systems — especially financial, medical, and legal — care about non-repudiation as a specific legal and regulatory requirement.

**NP06 — Fairness**
The claim that competing parties receive equitable treatment under declared rules. No party is systematically advantaged or disadvantaged by the mechanism's implementation. Distinct from Isolation (which prevents interference) and Ordering (which establishes sequence). Fairness is about equitable treatment across a population of requests or users.

- Band candidate: B2 (Behavioral)
- Conditions: fairness criteria declared; measurement specified; tolerance specified
- Distinct from: Isolation (non-interference), Ordering (sequence), Boundedness (resource limits)
- Used by: AC15 (booking fairness), AC19 (matchmaking), AC27 (auction fairness), AC28 (order matching fairness)
- Status: **borderline** — operational systems handle fairness through fair-share scheduling and FIFO queues, which are mechanism properties, not system-level claims. Application systems make fairness a product requirement: the auction must be fair, the matchmaking must be balanced, the booking system must not advantage repeat customers over new ones. Whether this is a property of the application or a property of specific mechanisms within it determines whether it warrants top-level naming.

---

## Principles: What Applications Add

The 22 principles cover data/logic placement, scale, failure, dependency, distribution, and operator relationship. Most apply directly to applications. But application development introduces construction pressures the operational perspective does not face.

### New Principle Candidates

```
new_principles(id|name|group_candidate|rule|reasoning|distinct_from|status)
```

**NR01 — Separate Domain Logic from Infrastructure Logic**
Domain logic — the rules specific to your application (billing computation, move validation, eligibility determination) — must be isolated from infrastructure logic (retry, auth, serialization, logging). Domain logic changes at domain pace. Infrastructure logic changes at platform pace. Mixing them makes both harder to change.

- Group candidate: G4 (Dependency and structure)
- Distinct from: R13 Minimize dependencies (about quantity, not separation), R15 Layer for separation (general, not domain-specific)
- Used by: all application classes
- Status: **new principle** — the operational taxonomy's R15 (layer for separation of concerns) is general. Operations does not distinguish "domain logic" from "infrastructure logic" because operational logic is infrastructure logic. Applications have a third category — domain-specific business rules — that operations does not have. The principle that domain rules live in runners (or policy data) while infrastructure lives in the library suite is application-specific.

**NR02 — Express Business Rules as Data**
Business rules — pricing logic, eligibility criteria, approval requirements, lifecycle transition rules, scoring weights — should be expressed as data rows evaluated by a mechanical engine, not as code in application logic. This extends R01 (data primacy) from configuration to business rules.

- Group candidate: G1 (Data and logic)
- Distinct from: R01 Data primacy (about config and state; NR02 extends to business rules specifically)
- Used by: AC01-AC21 (all governed-state applications)
- Status: **new principle** — operational data primacy means configuration as data. Application data primacy means business rules as data. The extension is meaningful because business rules are more complex than configuration (they have predicates, conditions, computed outputs) and the temptation to encode them in code is stronger. The principle that the rule engine evaluates data rows rather than executing code paths is application-specific.

**NR03 — Separate Read Models from Write Models**
When read patterns diverge from write patterns — aggregated dashboards, denormalized list views, search-optimized projections — maintain separate read models derived from the governed write model. The read model is a cache. The write model is the source of truth. Inconsistency between them is a freshness problem, not a correctness problem.

- Group candidate: G4 (Dependency and structure)
- Distinct from: R17 Local cache + global truth (about locality/partition; NR03 is about shape divergence)
- Used by: AC01, AC13, AC14, AC21 (any application where read shapes differ from write shapes)
- Status: **new principle** — the operational taxonomy's R17 is about caching for locality and partition tolerance. NR03 is about maintaining differently-shaped projections of the same data for different access patterns. Operational systems cache the same shape locally. Applications denormalize into different shapes for different consumers. The observation cache tables in OpsDB are the mechanism; the principle that read models are derived caches is application-specific.

**NR04 — Preserve User Intent Through Processing**
When a user action passes through validation, approval, and execution, the final state must reflect the user's expressed intent, not an implementation artifact. If a user submits a change set to update a price from 10 to 15, and the execution pipeline transforms, validates, and applies it, the result must be price=15, not price=14.99 due to floating point or price=15.00000001 due to serialization. Intent preservation requires that the change set records the intended values and the execution path does not introduce transformation artifacts.

- Group candidate: G1 (Data and logic)
- Distinct from: P08 Determinism (reproducibility, not intent fidelity), P04 Consistency-data (constraints hold, not intent preserved)
- Used by: AC05, AC13, AC21 (any application where numeric precision matters)
- Status: **borderline** — this could be a specific condition on Determinism and Consistency-data rather than a separate principle. But the frequency of precision bugs in financial and billing applications, and the specific requirement that the system preserves what the user typed rather than what floating point produces, suggests it warrants naming.

**NR05 — Design for the Approval Audience**
When change sets route to approvers, the change set must be comprehensible to the approver. Field changes should carry human-readable context: what entity, what field, what the current value is, what the proposed value is, and why. This is not a technical requirement — it is a usability principle for the change management system.

- Status: **not a principle** — this is a UX guideline for change set presentation, not a construction principle. It belongs in implementation guidance, not in the taxonomy.

**NR06 — Monotonic Schema Growth**
The schema only grows. Entities and fields are added, never removed. Deprecated elements remain queryable. This is already encoded in the schema evolution rules (SF07, SF08) but at the application level it becomes a principle: plan for monotonic growth, budget for accumulated cruft, and design consumers to tolerate deprecated elements.

- Status: **already covered** — this is the schema evolution rules (OPSDB-7 §12) restated as a principle. The infrastructure taxonomy's P18 (stability under change) covers the property claim. Not a new principle.

---

## Summary: What Applications Add

### New Mechanisms (confirmed)

| ID | Name | Family | Application-Specific Because |
|---|---|---|---|
| NM01 | State Machine Evaluator | F9 (Gating) | Operations converges to state. Applications enforce transition graphs. |
| NM02 | Rule Engine | F9 (Gating) or new | Operations validates against bounds. Applications evaluate business rule populations against input. |
| NM03 | Accumulator | F12 (Transformation) | Operations observes metrics. Applications compute and persist derived quantities on the write path. |
| NM04 | Temporal Projection | F12 (Transformation) | Operations executes scheduled work. Applications compute when things would occur for display and conflict detection. |

### Borderline Mechanisms

| ID | Name | Status |
|---|---|---|
| NM05 | Conflict Detector | Composable from Selector + Validator but recurs frequently enough to warrant naming |
| NM07 | Delegation Chain | Composable from Authorizer + Lease + History but application-formalized |

### New Properties (confirmed)

| ID | Name | Band | Application-Specific Because |
|---|---|---|---|
| NP01 | Lifecycle Integrity | B1 (Data Integrity) | Operations doesn't track transition legality. Applications have entities whose path through states matters. |
| NP03 | Computational Correctness | B1 (Data Integrity) | Operations doesn't maintain derived quantities. Applications persist computed values that must match their sources. |
| NP05 | Non-repudiation | B1 (Data Integrity) | Operations cares about attribution. Applications have legal/regulatory requirements that evidence prevent denial. |

### Borderline Properties

| ID | Name | Status |
|---|---|---|
| NP02 | Temporal Consistency | Specific condition on Determinism; recurs frequently enough to warrant naming |
| NP06 | Fairness | Product requirement more than system property; specific mechanisms provide it |

### New Principles (confirmed)

| ID | Name | Group | Application-Specific Because |
|---|---|---|---|
| NR01 | Separate Domain from Infrastructure | G4 (Dependency) | Operations IS infrastructure. Applications have domain logic that must be isolated from it. |
| NR02 | Express Business Rules as Data | G1 (Data/Logic) | Operations has config-as-data. Applications extend this to business rules with predicates and computed outputs. |
| NR03 | Separate Read Models from Write Models | G4 (Dependency) | Operations caches same shape. Applications denormalize into different shapes for different consumers. |

### Borderline Principles

| ID | Name | Status |
|---|---|---|
| NR04 | Preserve User Intent | Specific condition on Determinism + Consistency; frequent enough in financial/billing to name |

---

## Totals

The application domain adds to the infrastructure taxonomy:

- **4 new mechanisms** (State Machine Evaluator, Rule Engine, Accumulator, Temporal Projection) plus 2 borderline
- **3 new properties** (Lifecycle Integrity, Computational Correctness, Non-repudiation) plus 2 borderline
- **3 new principles** (Separate Domain from Infrastructure, Express Business Rules as Data, Separate Read Models from Write Models) plus 1 borderline

The additions are modest because the infrastructure taxonomy was designed at the right level of abstraction. Most application patterns are compositions of existing primitives. The new items arise specifically from the differences between operational and application concerns: operations converges to declared state while applications enforce transition legality, operations observes metrics while applications compute and persist derived quantities, and operations is infrastructure while applications have a domain logic layer that must be separated from it.
