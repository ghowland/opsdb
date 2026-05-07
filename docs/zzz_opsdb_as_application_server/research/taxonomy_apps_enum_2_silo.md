## Patterns From Silo That Extend the OpsDB Application Taxonomy

---

### New Mechanism Patterns

**SM01 — Execution Funnel**
A layered pipeline where each layer narrows the candidate set using a different computational model before the next layer evaluates. In Silo: state machine filters to valid behaviors → Prolog filters to precondition-satisfied behaviors → Utility AI scores to winning behavior → Logic blocks execute the winner. Each layer uses a fundamentally different mechanism type (graph traversal, unification, multiplicative scoring, stack execution) but they compose as a monotonic reduction.

The infrastructure taxonomy has individual mechanisms — Selector, Filter, Ranker, Validator — and OpsDB's gate pipeline is a ten-step narrowing sequence. But the gate pipeline applies the same model (policy check) at each step. The Execution Funnel specifically composes heterogeneous mechanism types into a monotonic narrowing sequence where each layer's computational model is different from the others.

OpsDB applicability: the gate pipeline is a variant. A richer variant would compose schema validation (structural), policy evaluation (predicate logic), approval scoring (utility-based), and execution (atomic write) as distinct computational layers rather than variations of the same check pattern.

**SM02 — Fact Regeneration**
Every cycle, entity state is converted into declarative facts. Decisions use current-cycle facts, never stale cached facts. Facts are ephemeral — regenerated from scratch each heartbeat, ensuring the decision layer always operates on current truth.

The infrastructure taxonomy has Probe (synthetic test of state) and Watch (subscription to changes). Neither names the pattern of regenerating a complete declarative fact set from imperative state on every evaluation cycle as a precondition for decision evaluation.

OpsDB applicability: runner cycles implement a weaker form. Each runner cycle reads current state from OpsDB (the get phase) and makes decisions based on that read, not on cached state from a prior cycle. The discipline that runners hold no in-memory state across cycles and re-read current state each cycle is fact regeneration at the runner level. The pattern could be strengthened by formalizing fact generation as a declared transformation from entity state to decision-input facts.

**SM03 — Multiplicative Scoring with Zero-Gating**
A scoring mechanism where the running score starts at 1.0, each consideration multiplies into it, and any consideration scoring 0 immediately zeros the total. This provides fail-fast elimination: any single disqualifying factor eliminates the candidate regardless of scores on other factors. An average-and-fixup compensation addresses the mathematical bias from many considerations.

The infrastructure taxonomy's Ranker names ordering by score. It does not name the specific scoring algebra where multiplication with zero-gating provides fail-fast elimination.

OpsDB applicability: approval rule evaluation could use this pattern. Each approval consideration (entity type, field sensitivity, security zone, compliance scope, data classification) multiplies into a governance score. Any consideration scoring zero (forbidden change, unauthorized scope) immediately disqualifies. The pattern is more expressive than the current binary approve/deny model for cases where approval requirements should scale with the combined weight of multiple factors.

**SM04 — Semantic Repurposing**
The same fields, the same infrastructure, the same execution pipeline — used for a completely different domain by changing only the data tables. A stat field that tracked one quantity in one dataset tracks a different quantity in another. The infrastructure does not care about domain semantics. Meaning is in the data, not in the types.

The infrastructure taxonomy has Namespace and Schema. Neither names the pattern where the same structural schema serves multiple unrelated domains by reinterpreting field semantics through data configuration alone, with zero code changes.

OpsDB applicability: this is exactly what the AppDB model provides. The same OpsDB infrastructure — gate pipeline, versioning, change management, runners, library suite — serves a billing application and a healthcare application and a personal recipe tracker. The schema defines the domain. The infrastructure is domain-agnostic. Semantic repurposing is the architectural basis for OpsDB Application Architecture.

**SM05 — Hot-Swap via Data Table Replacement**
Behavior changes take effect by replacing data tables while the system runs. New state machines, new rules, new scoring configurations — all loaded as data, all active next cycle. No restart, no recompile, no deployment.

The infrastructure taxonomy's Reconciler compares desired versus actual and converges. Hot-swap is different: wholesale replacement of behavior definition, effective on the next evaluation cycle.

OpsDB applicability: the data-driven behavior model is a governed form of hot-swap. Changing approval rules, validation policies, notification routing, retention policies, or runner configuration takes effect when the change set is applied. The behavior changes without code deployment. The governance layer (change management, approval, audit) wraps the hot-swap with attribution and reversibility that raw table replacement lacks.

**SM06 — Scene as Isolation Boundary**
A scene is an isolated execution context with its own entity pool, behavior definitions, and access control. Scenes map to tenants, requests, processes, or workflows depending on the domain. Access between scenes is controlled by whitelist — both scene ID and field path must pass. Default is deny.

The infrastructure taxonomy has Bulkhead (isolates failure domains) and Namespace (scope for name uniqueness). The Scene combines complete execution context isolation with integrated access control into a single structural primitive.

OpsDB applicability: the AppDB model maps directly. Each AppDB is a scene — its own schema, API, database, runners, policies, and audit trail. Cross-AppDB access is controlled by typed pointers with policy-mediated federation. Default is no cross-access. The five-layer authorization model within a single AppDB provides scene-like isolation at the entity level through `_requires_group` and `_access_classification`.

**SM07 — Structured Trace**
Every entity, every cycle: state before and after, all behavior scores with per-consideration breakdown, logic execution with timestamps, stat diffs. Debug in domain terms — "why did entity X transition to state Y" — not in memory addresses. Blue/green replay: modify rules, replay the same cycle, compare outcomes.

The infrastructure taxonomy has Log (append-only record) and Watch (subscription to changes). Neither names the pattern of capturing the complete decision chain for every entity in domain-aware structured form with replay capability.

OpsDB applicability: the audit log and version history provide the data substrate for structured tracing. Every change set records what changed, who proposed it, who approved it, and what the prior state was. The version history allows reconstruction of state at any point. What OpsDB does not currently provide is the per-decision-cycle trace with replay — the ability to ask "given the state and policies at time T, what would the approval routing have computed?" and to replay with modified policies to compare outcomes. This is an extension point: a trace runner that captures gate pipeline evaluation details per change set, with a replay mechanism that re-evaluates with modified policy data.

**SM08 — Ingress Validation (Partial Geometric Security)**
At the API boundary, structural validation rejects non-conforming requests before semantic interpretation. The closed schema vocabulary prevents injection of new operation types. Fixed field types prevent type confusion. No regex evaluation. No embedded logic execution. The gate pipeline processes validated structure only — downstream logic never encounters raw unvalidated input.

This is the deployable subset of geometric security for systems running on standard stacks. It does not enforce byte-level shape validation before parsing (OpsDB parses JSON over HTTP before validating structure), so the parser itself remains an attack surface. But at the application validation layer, the properties hold: structurally non-conforming input is rejected before semantic interpretation, and the closed vocabulary prevents extension of the operation set through input.

OpsDB applicability: already implemented. The gate pipeline's schema validation (step 3) and bound validation (step 4) enforce structural conformance before policy evaluation (step 5) and execution (step 9). The closed constraint vocabulary (nine types, no regex, no embedded logic) prevents vocabulary extension through schema files. Runner report key enforcement prevents runners from writing outside their declared scope. The principle "shape before meaning" holds at the API layer: structural validation precedes semantic evaluation.

---

### New Property Patterns

**SP01 — Vocabulary Closure (Application Layer)**
The claim that the system's operation vocabulary at the application layer is a closed finite set and cannot be extended at runtime through any application-level mechanism. The schema vocabulary is fixed (nine types, three modifiers, six constraints). The API operations are fixed (sixteen operations). The gate pipeline steps are fixed (ten steps). No schema file, no policy row, no runner configuration can introduce new operation types.

This is distinct from Boundedness (P12), which claims resource consumption stays within limits. Vocabulary Closure claims the operation set itself is bounded and fixed. A system can be resource-bounded while permitting arbitrary operations within those bounds. Vocabulary Closure prevents arbitrary operations.

The qualification "application layer" is important. OpsDB achieves vocabulary closure at the schema, API, and runner layers. It does not achieve it at the infrastructure layer beneath — Go and Postgres can execute arbitrary operations. The closure holds within the OpsDB abstraction boundary.

**SP02 — Structural Security (Application Layer)**
The claim that at the application validation layer, security properties derive from the absence of mechanism rather than from policy enforcement. The closed schema vocabulary means injection of new operation types cannot be expressed. The absence of regex means catastrophic backtracking cannot occur. The absence of embedded logic means arbitrary code execution through schema files cannot occur. These are structural impossibilities at the application layer, not policies that could be misconfigured.

This is distinct from Confidentiality (P07) and Integrity (P05), which are properties achieved through mechanisms (encryption, checksums). Structural Security claims that certain attack categories are impossible by construction at the application layer because the vocabulary to express them does not exist.

The qualification "application layer" again applies. OpsDB does not control the Go runtime, the Postgres query executor, or the OS kernel. Structural security holds within the OpsDB abstraction boundary — the schema engine, the gate pipeline, and the runner scope enforcement.

**SP03 — Hot-Swap Safety (Governed)**
The claim that behavior definitions can be replaced at runtime — approval rules, validation policies, access control, notification routing, runner configuration — without interrupting processing, without corrupting state, and with full governance (attribution, approval, versioning, audit) around the replacement. The new behavior takes effect when the change set is applied. The old behavior is preserved in version history.

This extends the infrastructure taxonomy's Stability under Change (P18) with the specific claim that the behavior definition itself is replaceable with zero disruption. The "governed" qualifier distinguishes this from raw hot-swap: the replacement is not just safe, it is attributed, approved, versioned, and auditable.

**SP04 — Domain Opacity**
The claim that the infrastructure layer contains no knowledge of the application domain it serves. The gate pipeline does not know whether it is validating an invoice or a patient record. The schema engine does not know whether it is generating tables for a billing system or a recipe tracker. The runner library does not know whether it is connecting to Stripe or to a weather API. Domain semantics exist only in schema YAML files and data rows.

No existing property in the infrastructure taxonomy names this. Determinism (P08) claims reproducibility. Boundedness (P12) claims resource limits. Domain Opacity claims that the infrastructure is completely separable from the application — not through layering (where both layers exist in the code) but through the design that the infrastructure has no application-specific types, handlers, or logic paths.

---

### New Principle Patterns

**NP01 — Security by Anatomy at the Application Layer**
At the application validation layer, security derives from structural limitation of what the system can express, not from rules about who can do what. The closed schema vocabulary, the absence of regex, the absence of embedded logic, the runner scope declarations validated by both the API gate and the library suite — these are structural constraints, not policies. A misconfigured policy row cannot introduce regex evaluation because the vocabulary does not contain regex. A compromised runner cannot write outside its declared scope because both the API and the library check declarations and fail closed.

This is more specific than R09 (Fail Closed), which says deny when uncertain. NP01 says deny by not having the vocabulary to permit. R09 is a behavioral policy. NP01 is a structural property. The distinction matters because R09 can be overridden by changing policy. NP01 cannot be overridden because the vocabulary to override does not exist within the OpsDB abstraction boundary.

**NP02 — Vocabulary Restriction at the Application Layer**
The execution engine operates only through defined primitives: nine schema types, three modifiers, six constraints, sixteen API operations, ten gate steps, three runner gating modes. No schema file, policy row, or runner configuration can introduce new primitives. Adding a primitive is a revision of the specification, not a runtime configuration change.

This extends R11 (Bound Everything) from quantity bounds to vocabulary bounds. R11 says every queue has a max depth and every retry has a budget. NP02 says the set of possible operations is itself finite and fixed. A system can bound every resource (R11) while still permitting arbitrary operations within those bounds. NP02 prevents arbitrary operations by restricting the vocabulary.

**NP03 — Shape Before Meaning at the API Layer**
Untrusted input passes through structural validation before semantic interpretation. Schema validation (step 3) and bound validation (step 4) enforce structural conformance — correct types, within declared ranges, matching declared constraints — before policy evaluation (step 5) interprets the semantic meaning of the proposed change. If the structure is wrong, the request is rejected before the system evaluates what it means.

This is more specific than R09 (Fail Closed). R09 says deny when uncertain. NP03 says validate geometric structure before any semantic processing. The principle addresses injection, type confusion, and constraint violation by ensuring that input reaches the semantic layer only after structural validation has passed.

OpsDB already implements this through the gate pipeline step ordering. The principle names the pattern explicitly so that extensions to the pipeline (new validation steps, new policy types) maintain the invariant that structural checks precede semantic checks.

**NP04 — Infrastructure Fix Protects All Consumers**
Because the OpsDB infrastructure — gate pipeline, schema engine, versioning system, change management, library suite — is shared across all AppDBs and all applications built on the platform, fixing a bug in any infrastructure component fixes it for every application simultaneously. A validation bug fixed in the gate pipeline is fixed for the billing AppDB, the healthcare AppDB, and the personal recipe AppDB in the same release.

This is the amortization argument for investing in shared infrastructure. The infrastructure taxonomy's R13 (Minimize Dependencies) reduces failure modes. NP04 addresses the benefit side: shared infrastructure means shared fixes. The principle justifies the investment in a comprehensive substrate over per-application custom code.

**NP05 — Marginal Cost of New Behavior Approaches Zero**
Adding the 1000th entity type to an AppDB costs the same engineering effort as adding the 10th. No new API endpoints. No new validation code. No new authorization logic. No new audit handling. Write a YAML file, run the loader. The new entity immediately has the full property set: validation, authorization, versioning, change management, audit, search, retention.

Similarly, adding the 1000th runner costs the same as the 10th. The library suite handles all infrastructure concerns. The runner handles only domain-specific decision logic.

This extends R22 (Removing Classes of Work) from automation to architecture. R22 says automation should eliminate work categories. NP05 says architecture should make the marginal cost of new application behavior approach zero. R22 is about operational efficiency. NP05 is about development efficiency through shared infrastructure.

---

### Summary: What Silo Adds to OpsDB Application Taxonomy

**New Mechanisms: 8**

| ID | Name | OpsDB Implementation |
|---|---|---|
| SM01 | Execution Funnel | Gate pipeline variant; extension point for heterogeneous computational layers |
| SM02 | Fact Regeneration | Runner get-phase re-reads current state each cycle; extension point for declared fact transformation |
| SM03 | Multiplicative Scoring with Zero-Gating | Extension point for approval rule evaluation with weighted multi-factor scoring |
| SM04 | Semantic Repurposing | AppDB model: same infrastructure serves different domains through schema alone |
| SM05 | Hot-Swap via Data Replacement | Data-driven behavior model: policy/config changes take effect on apply without deployment |
| SM06 | Scene as Isolation Boundary | AppDB isolation: one DOS per application; five-layer authz within |
| SM07 | Structured Trace | Audit log + version history; extension point for per-change-set decision trace with replay |
| SM08 | Ingress Validation (Partial Geometric Security) | Gate pipeline steps 3-4: structural validation before semantic interpretation |

**New Properties: 4**

| ID | Name | OpsDB Scope |
|---|---|---|
| SP01 | Vocabulary Closure (Application Layer) | Holds within OpsDB abstraction boundary; not below |
| SP02 | Structural Security (Application Layer) | Holds within OpsDB abstraction boundary; not below |
| SP03 | Hot-Swap Safety (Governed) | Behavior replacement with attribution, approval, versioning, audit |
| SP04 | Domain Opacity | Infrastructure has no application-domain knowledge |

**New Principles: 5**

| ID | Name | OpsDB Implementation |
|---|---|---|
| NP01 | Security by Anatomy (Application Layer) | Closed vocabulary; absence of regex/logic/templating |
| NP02 | Vocabulary Restriction (Application Layer) | Fixed primitive set; extension is spec revision not runtime config |
| NP03 | Shape Before Meaning (API Layer) | Gate steps 3-4 before step 5; structural before semantic |
| NP04 | Infrastructure Fix Protects All | Shared gate pipeline, schema engine, library suite across all AppDBs |
| NP05 | Marginal Cost Approaches Zero | New entity = YAML file; new runner = 150-300 lines + library calls |

**Excluded (requires ring-0 or runtime control OpsDB does not have):**

| Original ID | Name | Why Excluded |
|---|---|---|
| NP04-orig | Errors Are Data Not Faults | Requires hardware exception trapping; Go/Postgres have own error handling |
| SP01-orig | Crash Impossibility | Requires control over entire execution stack |
| SM05-orig | Deterministic Heartbeat | Requires owning execution model; OpsDB serves concurrent requests |
| SP02-orig full | Vocabulary Closure (full stack) | Holds at OpsDB layer only; Go/Postgres execute arbitrary operations |
| SM10-orig full | Ingress Shim (full geometric) | Requires byte-level shape validation before parsing; OpsDB parses JSON first |
