# OpsDB Application Development Methods Manual

## How Your Existing Workflow Translates to OpsDB Development

---

### Introduction

This manual maps established development methodologies to OpsDB application development. Each chapter takes a methodology you already practice and translates every concept, practice, and artifact to its OpsDB equivalent. The translation is direct: you want this, you do that.

This manual assumes you have read the quickstart guide and understand the core concepts: schema YAML, the gate pipeline, change sets, runners, and the library suite. If a translation requires detailed construction steps, it references the relevant how-to guide or the COMP-9 construction reference rather than duplicating the instructions.

---

### 1. Agile and Scrum

#### 1.1 Sprint Planning

In Agile, you plan sprints around user stories estimated in points. Work decomposes into tasks across models, controllers, validation, auth, tests, and deployment.

In OpsDB development, the work categories change. Infrastructure stories disappear — there is no "add authentication," "build validation layer," "implement audit logging," or "write API endpoints" because the gate pipeline provides these from the first entity definition. The remaining story categories are:

**Schema stories.** Declaring new entities, adding fields, configuring governance fields, designing relationships. These are small — a new entity with 10 fields is an hour of YAML writing, a loader run, and verification through the API. Estimate in hours, not days.

**Policy stories.** Configuring approval rules, access control, retention policies, state transition rules, cross-field invariants. Each policy is a data row. Estimate in hours.

**Runner stories.** Implementing domain-specific logic — external integrations, computations, notifications, reconciliation. These are the closest to conventional development work. A runner is 150–300 lines of code using the library suite. Estimate in days, not weeks.

**Frontend stories.** Building UI that consumes the OpsDB API. This is conventional frontend work — OpsDB does not reduce frontend effort. Estimate at your normal frontend rates.

The total story count per sprint drops because infrastructure stories are eliminated. The estimation baseline shifts — what was an 8-point story in conventional development (entity with CRUD endpoints, validation, auth, tests) is a 1–2 point story in OpsDB development (YAML declaration, loader run, verification).

#### 1.2 User Stories

The user story format does not change. "As a project manager, I want to track task assignments, so that I know who is working on what." The implementation tasks change.

Conventional implementation: write Task model, write Assignment model, write migration, write TaskController with CRUD endpoints, write AssignmentController, add authorization checks, add validation rules, write serializers, write tests, deploy.

OpsDB implementation: declare `task` entity in schema YAML with fields for title, description, status, priority, due date. Declare `task_assignment` entity with foreign keys to `task` and `ops_user`. Set `_requires_group` on tasks to the project's member group. Configure an approval rule for assignment changes if needed. Run the loader. Verify through the API: create a task, assign it, search for assignments by user, confirm access control filters correctly.

The acceptance criteria translate to declarations. "Only project members can see tasks" is `_requires_group` set to the project group — a field value on the entity, not a code path. "Task status must follow the workflow" is a state transition policy row — data evaluated at gate step 5, not a validator method. "All assignment changes must be audited" is automatic — every change set produces an audit entry regardless of configuration.

#### 1.3 Definition of Done

In Agile, done typically means: code written, tests passing, code reviewed, deployed, acceptance criteria verified.

In OpsDB development, done means:

Schema loads without error — the loader accepts all declarations. This verifies structural correctness, naming conventions, foreign key targets, constraint validity, and evolution rule compliance.

Seed data validates constraints — submitting test data through the API confirms that valid data is accepted and invalid data is rejected. The gate pipeline is the test runner.

Policies produce correct behavior — submitting change sets confirms that approval routing, access control, and state transition rules work as intended.

Runners produce correct output — running each runner against test data confirms that the domain logic reads the right inputs, computes the right results, and writes the right outputs.

Frontend renders correctly — the UI displays what the API returns, handles approval-pending states, and respects field-level omissions from access classification.

The "tests passing" criterion changes. You do not write tests for validation, authorization, versioning, or audit — those are platform tests maintained once. You verify that your specific declarations produce the correct behavior by exercising them through the API.

#### 1.4 Backlog Grooming

In Agile, grooming prioritizes and refines stories in the backlog.

In OpsDB development, separate schema stories from runner stories from policy stories during grooming, and prioritize them differently.

Schema stories should be groomed earliest and reviewed most carefully. Schema decisions are permanent — fields cannot be deleted or renamed. A poorly named entity lives forever as a deprecated tombstone. Invest grooming time in getting schema design right.

Runner stories can be groomed at normal pace. Runners are conventional code — replaceable, refactorable, independently deployable. A runner with a bug is a code fix, not a permanent scar.

Policy stories are often small but high-impact. A single approval rule change can alter the governance model for an entire entity type. Groom them with attention to their blast radius — what behavior changes when this policy row changes?

#### 1.5 Iteration and Schema Evolution

In Agile, iteration refines the product over sprints. Refactoring improves code without changing behavior.

In OpsDB development, schema evolution follows additive rules: you can add entities, add fields, add enum values, widen numeric ranges, widen string lengths. You cannot delete, rename, narrow, or change types.

This means schema mistakes are not refactorable in the traditional sense. The mitigation is the six-step duplication pattern — add a better field alongside the old one, migrate consumers, deprecate the old field. The old field becomes a tombstone: present in the schema forever, but no longer operationally active.

For domains where the model is not yet understood, use the graduated formalization pattern. Start with a generic collection entity that holds discriminated JSON payloads. Track things loosely. When the domain stabilizes — when you know what entities, fields, and relationships you need — write a proper schema and migrate the data with a runner. The generic collection is the exploration sandbox. The proper schema is the permanent home.

---

### 2. Test-Driven Development

#### 2.1 The Test-First Cycle

In TDD, the cycle is: write a failing test, write code to pass it, refactor. Red-green-refactor.

In OpsDB development, the cycle translates to: specify expected behavior as schema constraints and policy rules, declare them in YAML, verify through the API. The declaration is both the specification and the implementation. The gate pipeline is the test runner that executes every constraint on every write, forever.

Practicing test-first in OpsDB:

Before writing a schema declaration, write down the data you expect to be accepted and the data you expect to be rejected. "A task priority must be between 1 and 5. Submitting priority 3 should succeed. Submitting priority 0 should fail. Submitting priority 6 should fail."

Write the declaration: `type: int, min: 1, max: 5`.

Run the loader. Submit priority 3 through the API — it succeeds. Submit priority 0 — the gate pipeline rejects it at bound validation. Submit priority 6 — rejected.

This is TDD. The expected behavior was specified before the implementation. The implementation is a YAML line. The test execution is an API call. The assertions are the gate pipeline's accept/reject responses.

For policies, the same pattern applies. Before writing a state transition rule, write down which transitions should be accepted and which should be rejected. "A task in status 'done' should not be allowed to transition directly to 'in_progress'." Write the transition policy row. Submit a change set attempting done → in_progress. The gate pipeline rejects it. The policy row is the test specification and the implementation simultaneously.

#### 2.2 Unit Testing

In TDD, unit tests verify individual functions in isolation.

In OpsDB development, the unit test equivalents are:

**Schema constraint tests.** For each field, verify the boundary conditions: submit a value below min (rejected), at min (accepted), at max (accepted), above max (rejected), wrong type (rejected), null when not nullable (rejected), FK to nonexistent entity (rejected). Each verification is an API call. The gate pipeline is the assertion engine.

**Policy rule tests.** For each policy, verify the matching conditions: submit a change set that should match the policy (it triggers), submit one that should not match (it does not trigger). For approval rules: submit a change to a field governed by the rule, verify it routes to the correct approvers. For state transition rules: submit a valid transition (accepted), submit an invalid transition (rejected).

**Runner domain logic tests.** For the act phase of each runner, set up input state in the database, run the runner, check what changed. The runner's inputs and outputs are data rows. The test fixture is the database. The assertions are queries against the database after the runner executes.

The runner test is the only one that resembles conventional unit testing. Schema and policy tests are verification of declarations, not verification of code. But the discipline is the same: specify expected behavior, exercise the system, check the result.

#### 2.3 Integration Testing

In TDD, integration tests verify that components work together.

In OpsDB development, the gate pipeline is the integration layer. It connects validation, authorization, policy evaluation, versioning, change management, and audit into a single pipeline. Integration testing verifies that these concerns compose correctly for your specific schema.

Integration test scenarios:

Submit a change set affecting a sensitive field. Verify it routes to the correct approver, the approver can approve, the change is applied, the version row is created, and the audit entry records the full chain: proposer, approver, timestamp, field values before and after.

Submit a search query as a user with limited access. Verify the results exclude entities the user cannot see (filtered by `_requires_group`) and omit fields the user cannot see (filtered by `_access_classification`).

Run a puller runner against a mock external API. Verify the observation cache is populated with correctly transformed data and freshness timestamps.

Submit a change set that violates a cross-field invariant policy. Verify the rejection includes the correct error message from the policy row.

These integration tests exercise the full path through the gate pipeline for your specific schema and policies. They verify that your declarations compose correctly — that the approval rule for field X works with the access classification on entity Y and the state transition rule on field Z.

#### 2.4 Test Coverage

In TDD, coverage measures what percentage of code is exercised by tests.

In OpsDB development, structural validation coverage is 100% by construction. Every field has type checking. Every constrained field has bound checking. Every foreign key has referential integrity checking. Every entity has authentication and authorization. Every mutation has audit logging. You did not write this code, so you do not test it. The platform tests it once.

What needs explicit test coverage:

Cross-field invariant policy rows — do the conditions match the domain rules? Each invariant rule should have at least one test case where the condition is met and the requirement is satisfied (accepted), one where the condition is met and the requirement is not satisfied (rejected), and one where the condition is not met (rule does not apply).

Approval rule routing — does the right change go to the right approver? Each approval rule should have a test case that triggers it and a test case that does not trigger it.

Runner domain logic — does the computation produce correct results? Each runner's act phase should have test cases covering the domain logic branches.

The testing surface is an order of magnitude smaller than conventional development because the infrastructure concerns (auth, validation, versioning, audit, serialization, pagination) are tested once at the platform level.

#### 2.5 Refactoring

In TDD, refactoring changes implementation without changing behavior, protected by the test suite.

In OpsDB development, refactoring applies differently to each artifact type.

**Schema:** cannot be refactored in the traditional sense. Fields cannot be renamed or deleted. The six-step duplication pattern is the equivalent: add a better field alongside the old one, begin writing to both, migrate readers, deprecate the old field. The old field remains as a tombstone. This is more expensive than code refactoring, which is why getting schema design right during grooming (Section 1.4) matters.

**Runners:** refactor freely. Runners are conventional code. If the refactored runner reads the same inputs and writes the same outputs, the behavior is preserved. The get-act-set pattern provides a natural boundary: the get phase and set phase are library calls that do not change, only the act phase logic changes. Run the runner against test data before and after refactoring to verify identical output.

**Policies:** change policy rows through governed change sets. The version history provides the safety net — you can see the prior policy configuration and roll back by submitting a change set that restores it. Policy changes are auditable and reversible.

**Frontend:** refactor normally. OpsDB does not change frontend refactoring practices.

---

### 3. Domain-Driven Design

#### 3.1 Bounded Contexts

In DDD, bounded contexts define boundaries within which a domain model is internally consistent. Different contexts may model the same real-world concept differently.

In OpsDB development, bounded contexts map to AppDBs. Each AppDB has its own schema, database, API, runners, and policies — a complete self-contained DOS. The billing AppDB and the project management AppDB are separate bounded contexts. They may both reference "customer" but with different schemas tailored to their domain.

How to identify AppDB boundaries: if two domains have different schema evolution lifecycles (one changes weekly, the other quarterly), different approval policies (one auto-approves everything, the other requires two approvers for financial changes), different retention requirements (one retains 30 days, the other 7 years), or different deployment cadences — they are separate AppDBs.

Cross-context communication uses typed pointers with federated reads. AppDB A holds a pointer (a foreign key to an entity in AppDB B's schema). A runner in AppDB A can follow the pointer to read from AppDB B's API. The communication is narrow, typed, and explicit. There is no shared database, no shared schema, no implicit coupling.

#### 3.2 Entities and Value Objects

In DDD, entities have identity and lifecycle. Value objects are defined by their attributes, have no identity, and are interchangeable.

In OpsDB development, DDD entities map directly to entity declarations in schema YAML. Each entity gets an auto-injected `id` field, lifecycle fields (`created_time`, `updated_time`, `is_active`, `created_by_user_id`, `updated_by_user_id`), and optional versioning with full state history.

Value objects map to one of three things depending on their usage:

Fields within an entity — an address (street, city, postal code) that always belongs to a specific customer is a set of fields on the customer entity.

JSON payload contents — when value objects have varied structure depending on type (different kinds of payment methods, different kinds of schedule configurations), they go in a discriminated JSON payload validated against a type-specific JSON schema.

Standalone entities — when a value object needs to be shared across multiple entities (a currency, a unit of measure, a classification level), promote it to an entity with its own identity. In OpsDB this is often a lookup entity with a small number of rows referenced by foreign keys from other entities.

The decision: if the value object needs its own version history, its own access control, or is queried independently, it is an entity. Otherwise it is fields or JSON content.

#### 3.3 Aggregates

In DDD, aggregates are clusters of entities treated as a unit for data changes. Modifications to an aggregate are atomic. The aggregate root is the only entry point.

In OpsDB development, the change set is the consistency boundary. A bulk change set modifies an invoice and its line items atomically — all changes apply or none do. The aggregate root is the entity that owns the others through foreign keys. An invoice is the root. Line items, tax entries, and payment references are children owned by the invoice via `invoice_id` foreign keys.

You do not need to code aggregate boundaries or enforce them in application logic. The atomic change set provides the transactional boundary. Foreign key integrity ensures children reference a real root. The gate pipeline validates every change within the set independently (each field against its schema, each FK against its target) and applies them together.

To enforce that changes to child entities must go through the root (a common DDD aggregate rule), configure approval rules on the child entity types that require the root entity's owner to approve. A line item change routes to the invoice owner for approval. This is a policy row, not code.

#### 3.4 Domain Events

In DDD, domain events signal that something meaningful happened in the domain. Event handlers react to events, often across aggregate boundaries.

In OpsDB development, domain events are implicit in state changes. When a task's status changes from "pending" to "approved," that transition is a change set with a version row recording the old and new state, the proposer, the approver, and the timestamp. There is no explicit event object published to a bus.

Runners react to state changes on their next cycle. A notification runner reads "all change sets applied since my last cycle" and finds the task approval. It dispatches a notification. A reconciler reads "all entities where desired state differs from observed state" and proposes corrections. The reaction is level-triggered, not edge-triggered — if a runner misses a cycle, the next cycle catches the current state and reacts to whatever is true now.

This is a deliberate architectural choice from the ops book (level-triggered over edge-triggered). The benefit: if an event is lost (a webhook fails, a message queue drops a message, a subscriber crashes), the system self-heals because the next runner cycle reads current state and converges. In event-driven systems, a lost event can leave the system in an inconsistent state until someone notices and manually triggers recovery.

To find "what happened since last time": query change sets with a time filter and entity type filter. The change set records contain the field changes, the proposer, the approver, and the reason. This is a search query, not a subscription.

#### 3.5 Repositories

In DDD, repositories provide collection-like access to aggregates, abstracting the persistence mechanism. You implement a repository per aggregate root.

In OpsDB development, the search API is every repository simultaneously. It provides filtering (equality, comparison, set membership, null checks, range, pattern matching), composable with AND/OR/NOT, across all entity types. Named join paths provide aggregate traversal — a search for tasks can include the project, the assignee, and recent comments through declared join paths. Projection controls which fields are returned. Cursor pagination handles large result sets.

You do not build repositories. You do not write data access code. You call the search API with predicates and get back the entities that match, filtered by the caller's authorization, with the fields the caller can see. The search API handles the query optimization, the access control filtering, the field-level omissions, and the pagination mechanics.

If you need a query pattern the search API does not directly support — aggregations, complex joins, denormalized views — a runner reads governed entities and writes the results to observation cache tables optimized for that read pattern. The observation cache is a derived read model. The search API serves both the governed entities and the cache tables through the same interface.

#### 3.6 Domain Services

In DDD, domain services contain logic that does not belong to a single entity — cross-entity computations, external integrations, complex business rules.

In OpsDB development, domain services are runners. Each runner is a domain service with a clear contract: declared inputs (which entities it reads, which external systems it consults), declared outputs (which tables it writes to, which side effects it produces), declared bounds (retry budget, execution time, scope per cycle), and a declared gating mode (direct write for observations, auto-approve for low-risk changes, approval-required for high-risk changes).

An invoice generation runner reads subscriptions, plans, usage records, and tax policies, computes line items and totals, and creates invoice entities through change sets. This is a domain service. A reconciliation runner reads desired state and observed state, computes the diff, and proposes corrections. This is a domain service. A notification runner reads state transitions and dispatches messages through configured channels. This is a domain service.

Each runner is 150–300 lines because the library suite handles infrastructure: authentication, retry, circuit breaking, logging, scope enforcement. The runner contains only the domain logic — the computation or decision that is specific to this service.

#### 3.7 Ubiquitous Language

In DDD, the ubiquitous language is the shared vocabulary between developers and domain experts, reflected consistently in the code.

In OpsDB development, the ubiquitous language is the schema. Entity names are the domain nouns. Field names are the domain attributes. Enum values are the domain categories. Relationship names (expressed through FK naming) are the domain associations.

The schema is readable by non-developers — it is YAML with English names, not code with technical constructs. A domain expert can review a schema file and verify that the entity names, field names, and enum values match the domain vocabulary. The naming conventions (singular, lowercase underscore, specific-to-general prefixes) enforce consistency mechanically.

The schema metadata tables make the vocabulary queryable at runtime. The API can return the list of entity types, the fields on each type, the constraints on each field, and the relationships between types. Tools that adapt to the schema — admin dashboards, report builders, data browsers — read this metadata and present the domain vocabulary without hardcoded knowledge.

---

### 4. DevOps and Platform Engineering

#### 4.1 CI/CD Pipeline

In DevOps, the CI/CD pipeline builds, tests, and deploys code through automated stages: lint, build, unit test, integration test, staging deploy, production deploy.

In OpsDB development, the CI pipeline verifies schema and runner artifacts:

**Lint stage:** validate YAML syntax, check naming conventions against the convention rules, verify forbidden patterns are absent (no regex, no embedded logic, no conditional constraints, no inheritance, no templating).

**Schema validation stage:** run the loader against the proposed schema in a test environment. The loader checks structural correctness, FK targets, constraint validity, and evolution rules (no deletions, no renames, no narrowing). If the loader rejects, the CI pipeline fails.

**Seed data stage:** submit test data through the API in the test environment. Valid data should be accepted. Invalid data should be rejected at the expected gate step with the expected error.

**Runner test stage:** run each runner against the test environment. Verify outputs match expected results.

The CD pipeline applies changes:

**Schema deployment:** the schema executor runner applies approved schema changes to the database. Additive changes (new entities, new fields) apply without downtime. The schema executor generates DDL, executes it within a transaction, and updates schema metadata tables.

**Runner deployment:** updated runner binaries or runner spec changes deploy through the standard deployment mechanism. If only runner spec data changed (configuration, bounds, trigger timing), the change is a governed change set applied through the API — no binary deployment needed.

**Policy deployment:** policy changes are change sets applied through the API. They take effect when the change set is approved and applied. No deployment pipeline needed — the data-driven behavior model means behavioral changes are data changes.

#### 4.2 Infrastructure as Code

In DevOps, IaC declares infrastructure state in version-controlled files: Terraform for cloud resources, Ansible for configuration, Helm for Kubernetes.

In OpsDB development, the schema YAML files are IaC for your application's data infrastructure. They declare what entities exist, what fields they have, what constraints apply, what relationships connect them, and what governance configurations govern them. They live in version control. They are applied by the loader, which generates the database DDL. The schema is the single source of truth for the data model.

Policy rows, runner specs, approval rules, and retention policies can be seeded from YAML files and applied through change sets during deployment. The seed data YAML files serve as the IaC for application behavior configuration. After initial seeding, ongoing changes to these configurations happen through the change set pipeline — governed, versioned, and auditable.

The relationship: git holds the schema definition (the structural source of truth). OpsDB holds the operational state (the runtime source of truth, including policy configurations that have been modified since seeding). Schema changes start as git commits and flow through the loader. Behavioral changes start as change sets and flow through the gate pipeline.

#### 4.3 Monitoring and Observability

In DevOps, observability means three pillars: logs (what happened), metrics (how much), and traces (the path through the system).

In OpsDB development, all three are built into the infrastructure:

**Logs.** The audit log records every API action: caller identity, operation, target entity, outcome, timestamp, contextual metadata. The audit log is queryable through the search API — the same interface that serves application data serves operational data. "Who changed this entity and when" is a search query, not a log grep.

**Metrics.** Runner health observations are written to observation cache on every cycle: cycle duration, records processed, errors encountered. External metrics (from Prometheus, Datadog, cloud provider) are imported by puller runners into observation cache tables with freshness timestamps. The search API queries both application metrics and infrastructure metrics.

**Traces.** Correlation IDs propagate through the library suite from the initial API request through runner execution to external system calls. The audit log entries for a single operation share a correlation ID. Following the chain from "user submitted change set" through "approval routing" through "runner execution" through "external API call" is a query filtered by correlation ID.

For integration with external monitoring: a puller runner imports metrics from Prometheus or Datadog into observation cache. A push runner exports OpsDB audit and health data to external observability platforms. The library suite emits structured logs and metrics in standard formats.

#### 4.4 Incident Response

In DevOps, incident response follows a cycle: detect, alert, respond, resolve, review.

In OpsDB development, each stage maps to system capabilities:

**Detect.** Verifier runners check conditions on schedules and write evidence records. A verifier that detects a failing condition writes a failing evidence record. A monitoring puller imports alert data from external systems.

**Alert.** The notification runner reads evidence records with failing results and alert fire entities, resolves the on-call assignment, and dispatches through configured channels (email, webhook, chat). Escalation paths define what happens if the primary contact does not respond.

**Respond.** The on-call responder queries the search API: what changed recently (change sets filtered by time), what is the current state of the affected entities (get_entity), what was the state before the incident (get_entity_at_time), what are the dependencies (get_dependencies). AI observation runners produce incident summaries at configured granularity — 80-character headline for the mobile alert, 2000-character summary for the war room.

**Resolve.** If the fix is a configuration change, submit a change set through the standard pipeline. If it is an emergency, use the emergency_apply operation — reduced approvals, mandatory audit flag, post-incident review window. If the fix is an external system change, the config push runner applies it and the observation pull runner confirms the result.

**Review.** The audit log contains the complete chain: what was detected, who was alerted, what actions were taken, what changes were made, who approved them. Version history shows the state before and after each change. Evidence records document verification results. The post-incident review references specific entity IDs, change set IDs, and timestamps — not reconstructed narratives from scattered log files.

#### 4.5 GitOps

In DevOps, GitOps uses git as the source of truth. Desired state is declared in git. A reconciliation loop converges actual state to match git.

In OpsDB development, git holds the schema definition and the initial seed data. The schema loader is the reconciliation mechanism — it compares the declared schema in git with the current database state and applies additive changes. This is GitOps for the data model.

Operational configuration (policy rows, approval rules, runner specs, access control) starts from seed data in git but evolves through change sets in OpsDB. The OpsDB version history is the operational source of truth for these configurations — not git. The reason: operational configurations change at operational pace (an approval rule adjusted during an incident, a retention policy updated for a new regulation), and these changes must be governed (attributed, approved, audited). Forcing them through git commits would bypass the governance model.

The reconciliation loop exists at the runner level: reconciler runners compare OpsDB's governed state with external systems' observed state and converge the difference. This is the same pattern as GitOps reconciliation (ArgoCD comparing git manifests with cluster state) but the source of truth is OpsDB's governed entities, not git files.

---

### 5. Clean Architecture and Hexagonal Architecture

#### 5.1 Layers and Dependencies

In Clean Architecture, the application has concentric layers: entities (innermost, domain core), use cases (application logic), interface adapters (controllers, presenters), and frameworks (outermost). Dependencies point inward — outer layers depend on inner layers, never the reverse.

In OpsDB development, the layers map to:

**Domain core (innermost): schema declarations.** Entity types, field definitions, constraints, relationships. The schema depends on nothing. It is pure declaration — YAML with no imports, no function calls, no references to infrastructure.

**Application logic: policy rows.** Approval rules, state transition rules, cross-field invariants, retention policies, access control configurations. Policies reference schema entities (they say "this rule applies to entity type X, field Y") but they do not reference runners, the frontend, or external systems.

**Interface adapters: runners.** Runners connect the domain to external systems. A puller runner adapts an external API to OpsDB observation cache format. A notification runner adapts OpsDB state transitions to notification channel dispatch. A config push runner adapts OpsDB governed state to an external system's native configuration format. Runners depend on the schema (they read and write entities) and on policies (their gating mode is determined by policy).

**Infrastructure (outermost): gate pipeline and library suite.** The gate pipeline processes every request. The library suite mediates every external interaction. These are the framework layer — they depend on nothing within the application. The application depends on them, but only through the API interface and library call interface.

Dependencies point inward. Runners depend on policies and schema. Policies depend on schema. Schema depends on nothing. The gate pipeline and library suite are the outer shell that everything passes through. This is Clean Architecture's dependency rule, enforced not by code discipline but by the nature of the artifacts — YAML cannot import Go code, and policy rows cannot invoke runner functions.

#### 5.2 Ports and Adapters

In Hexagonal Architecture, ports define the application's interface to the outside world. Primary ports are driven by external actors. Secondary ports drive external systems.

In OpsDB development:

**The primary port is the OpsDB API.** Every interaction from frontends, humans, automation, and AI enters through this single interface. There is one primary port, not multiple. The gate pipeline is the implementation of the primary port — it processes every request through the same ten steps regardless of the caller.

**Secondary ports are the library suite's external connectors.** The Kubernetes library, cloud provider libraries, secrets library, notification library, git library, LLM library — each is a secondary port with a defined interface. Runners use these ports to reach external systems.

**Adapters are runners.** A Stripe puller runner is an adapter that implements the "import payment data" interaction using the Stripe API through the HTTP library. A Slack notification runner is an adapter that implements the "dispatch notification" interaction using the Slack API through the notification library. Each runner adapts between the application's domain model (entities in the schema) and an external system's interface.

Swapping an adapter means replacing a runner. To switch from Stripe to a different payment processor, write a new puller runner that reads from the new processor's API and writes to the same observation cache tables in the same format. The schema does not change. The policies do not change. The other runners do not change. Only the adapter changes.

#### 5.3 The Domain Core

In Clean Architecture, the domain core contains business entities and business rules with no dependencies on external systems, frameworks, or infrastructure.

In OpsDB development, the domain core is the schema YAML plus the policy rows. The schema declares what entities exist and what constraints apply. The policies declare what rules govern behavior. Neither contains infrastructure logic, external system references, framework dependencies, or executable code.

This is a stronger separation than Clean Architecture typically achieves in code. In conventional Clean Architecture, the domain core is a set of classes that must be carefully kept free of infrastructure imports — a discipline that erodes over time as developers add convenience methods, utility imports, or framework annotations. In OpsDB, the domain core is data files that literally cannot have import statements, function calls, or class hierarchies. The separation is structural, not disciplinary.

---

### 6. Microservices

#### 6.1 Service Decomposition

In Microservices, the application is decomposed into small, independently deployable services. Each service owns its data and communicates through APIs or events.

In OpsDB development, the decomposition question splits into two levels:

**Between applications: AppDBs.** Each AppDB is a self-contained DOS — its own schema, database, API, runners, policies, and audit trail. The billing application and the project management application are separate AppDBs. They are independently deployable, independently scalable, and share no data.

**Within an application: runners.** A billing runner, a notification runner, a reconciliation runner, and an integration runner are separate capabilities within one AppDB. They share the same database and API. They coordinate through shared data — runner A writes a result, runner B reads it on its next cycle. They deploy as threads in the same server runner process.

The decision boundary: if two capabilities need different schemas, different approval policies, or different deployment lifecycles, they are separate AppDBs (separate microservices in conventional terms). If they share a schema and differ only in domain logic, they are separate runners within one AppDB (separate threads in one process).

The practical consequence: most applications that would be 5–15 microservices in conventional architecture are one AppDB with 5–15 runner threads. The distributed systems overhead — service discovery, inter-service authentication, distributed tracing, saga coordination, API versioning between services — disappears because the runners share a database and communicate through data, not network calls.

#### 6.2 Inter-Service Communication

In Microservices, services communicate through synchronous APIs (REST, gRPC) or asynchronous events (message queues, event buses).

In OpsDB development:

**Within an AppDB:** runners communicate through shared data. Runner A writes observation cache rows. Runner B reads them on its next cycle. No API calls between runners. No message queue. No event bus. The database is the communication channel. Every read and write goes through the gate pipeline, so every communication is authenticated, authorized, validated, and audited.

This is simpler than inter-service communication because there are no network failures, no serialization mismatches, no API version compatibility issues, and no message ordering problems. The data is in the database. Both runners see the same data through the same API.

**Between AppDBs:** typed pointers with federated reads. AppDB A holds a pointer (a foreign key referencing an entity in AppDB B). A runner or frontend in AppDB A follows the pointer to read from AppDB B's API. This is a cross-service API call — but it is explicit (typed pointer, not arbitrary URL), narrow (specific entity reference, not broad query), and infrequent (runners sync periodically, not on every request).

#### 6.3 The Saga Pattern

In Microservices, sagas coordinate multi-step distributed transactions using compensating actions. If step 3 fails, steps 1 and 2 are compensated (rolled back through domain-specific reversal operations).

In OpsDB development:

**Within an AppDB:** the bulk change set provides atomic multi-entity changes. No saga needed. Either all changes in the set apply or none do. An invoice with line items, tax entries, and a payment record is one bulk change set — atomic by construction.

**Between AppDBs:** the runner bridge pattern provides eventual consistency without sagas. A runner in AppDB A reads state, computes a decision, and writes to AppDB A through a change set. A separate runner in AppDB B reads from AppDB A (via federated read) and writes corresponding changes to AppDB B through its own change set. If the second step fails, it fails cleanly — the change set in AppDB B is not created. On the next cycle, the runner in AppDB B reads the current state from AppDB A again and tries again. This is level-triggered convergence, not saga compensation.

The advantage over sagas: sagas require every step to have a compensating action, and the compensation must be correct (a notoriously difficult requirement). Level-triggered convergence requires only that the runner can read current state and compute the correct action from it. If any step fails, the next cycle recomputes from current state rather than unwinding a chain of compensations.

#### 6.4 Service Mesh

In Microservices, the service mesh handles cross-cutting concerns across services: mutual TLS, rate limiting, retries, circuit breaking, distributed tracing.

In OpsDB development, there is no service mesh because there are no inter-service calls within an AppDB. The gate pipeline handles all cross-cutting concerns for data operations: authentication, authorization (five layers), validation, policy evaluation, audit logging. Rate limiting is policy data evaluated per caller. Circuit breaking is in the library suite for external calls. Tracing is built into the audit log through correlation IDs.

For external system calls (runners calling cloud APIs, payment processors, notification channels), the library suite provides retry, circuit breaking, timeout management, and correlation ID propagation. Each library connector is purpose-built for its external system, not a generic mesh proxy.

---

### 7. Methodology-Independent Practices

#### 7.1 Code Review

Whatever methodology you practice, code review applies to OpsDB development in a focused way. The review targets are different because the artifacts are different.

**Schema review.** The schema steward reviews entity declarations for: naming convention compliance, appropriate type selection, constraint completeness (are bounds reasonable?), relationship correctness (do FKs reference the right targets?), governance field configuration (is access classification set appropriately?), evolution rule compliance (no deletions, renames, or narrowing in schema changes), and notes documentation (especially for draft mode flags where the property tradeoff must be documented). Schema review is the highest-stakes review because schema decisions are permanent.

**Runner review.** Review focuses on the act phase — the domain logic. Does the runner read the correct inputs? Does the computation produce correct results? Is the output written to the correct tables with the correct gating mode? Is the runner idempotent — running it twice on the same input produces the same end state? Are the scope declarations correct — does the runner declare only the tables it actually needs? The get phase and set phase are library calls that follow standard patterns and need less scrutiny.

**Policy review.** Review focuses on governance impact: does this approval rule route to the right approvers? Does this access control configuration match the intended data sensitivity model? Does this retention policy meet regulatory requirements? Policy changes can have large blast radius — a single approval rule change affects every change set that matches its criteria.

**Frontend review.** Standard frontend review practices apply. OpsDB does not change how you review UI code.

#### 7.2 Documentation

In OpsDB development, the primary documentation artifacts are the schema files themselves.

Schema YAML is self-documenting through entity names, field names, types, constraints, and the `notes` field. A developer reading `task.yaml` sees every field the entity has, every constraint that applies, every relationship it participates in, and the governance configuration — all in one file. The `notes` field provides domain context, design rationale, and governance tradeoff documentation (especially important for entities with draft mode flags).

Policy rows document governance rules as data. You can query "what approval rules apply to this entity type" through the search API. The answer is a set of policy rows, each with a description, match criteria, and effect. This is live documentation that cannot drift from the implementation because it is the implementation.

Runner specs document automation behavior as data. You can query "what runners operate on this entity type" through the search API. Each runner spec declares what it reads, what it writes, what triggers it, and what bounds it operates within.

The schema metadata tables make all of this queryable at runtime. Tools, dashboards, and admin interfaces can discover the data model, the governance rules, and the automation configuration without hardcoded knowledge.

Supplementary documentation — architecture decisions, domain context, operational procedures — lives in markdown files alongside the schema in the repository. But the schema is always the authoritative reference for what the data model is. If the markdown disagrees with the schema, the schema is right.

#### 7.3 Onboarding

A new developer joining an OpsDB application team reads:

The schema YAML files — to understand the data model. Entity names, field types, relationships, and governance configurations tell them what the application manages and how it is structured.

The policy rows — to understand the governance rules. Who approves what, who can access what, what transitions are valid, how long data is retained.

The runner specs — to understand the automation. What external systems are integrated, what computations run in the background, what reconciliation and verification happens.

The frontend code — to understand the user experience.

They do not need to understand the gate pipeline internals, the authorization implementation details, the versioning system mechanics, or the audit infrastructure architecture. Those are platform concerns, documented in the OpsDB specification, not application concerns. The new developer works with schema declarations, policy data, runner code, and frontend code — never with infrastructure code.

#### 7.4 Debugging

Debugging in OpsDB development follows a consistent path through data, not through code.

**What happened?** Query the audit log. Filter by entity type, entity ID, time range, or caller identity. The audit log records every API action with the operation, outcome, caller, and timestamp.

**What changed?** Query the version history. Each version row contains the full entity state, the change set that produced it, the proposer, and the approvers. Diff any two versions to see exactly what changed.

**What was the state at a specific time?** Use `get_entity_at_time`. The response is the version row that was current at the specified timestamp — a single row lookup.

**What did the runner do?** Query runner job records. Each runner cycle writes start time, duration, records processed, and errors to observation cache. The runner's set-phase writes are in the audit log attributed to its service account.

**What did the external system report?** Query observation cache tables. Puller runners write external state with freshness timestamps. You can see what the external system reported and when.

**Why was this change rejected?** The audit log records rejections with the gate step that failed and the reason. "Rejected at step 4: field priority value 6 exceeds maximum 5" tells you exactly what happened. "Rejected at step 7: required approval from project_owner role not received" tells you the change is pending approval, not failed.

The debugging path is always: audit log (what happened) → version history (what changed) → observation cache (what was the external state) → runner job records (what did automation do). Every step is a search query through the same API.

#### 7.5 Performance Optimization

Performance in OpsDB development means optimizing your schema design, query patterns, and runner efficiency. The gate pipeline's performance is a platform concern — the application developer optimizes their usage of the platform, not the platform itself.

**Schema optimization.** Add indexes for fields that appear frequently in search predicates. Avoid overly wide entities (30+ fields) — split into related entities with foreign keys. Use appropriate field types — varchar with length bounds instead of text when the length is bounded, int instead of float when the value is always whole.

**Query optimization.** Use projection to return only the fields the consumer needs, not all fields. Use cursor pagination, not offset pagination — cursor pagination is O(1) per page, offset pagination is O(N) for page N. Limit join depth for queries that traverse relationships. Use freshness annotations on observation cache queries to avoid processing stale data.

**Runner optimization.** Bound the scope per cycle — process at most N entities per run. Use batched API calls (bulk_submit) instead of individual change sets when applying multiple corrections. Cache external API responses within a cycle when the external data does not change during the cycle. Respect the retry budget — do not retry indefinitely.

**Cache configuration.** Size the read cache for your read/write ratio. Configure tenant-aware cache keying to prevent cross-tenant cache pollution. Set cache TTLs that match the freshness requirements of your read patterns.

---

### 8. Quick Translation Reference

| You Want | In Conventional Development | In OpsDB Development |
|---|---|---|
| Define a data model | Write model classes, migrations, ORM config | Write schema YAML, run the loader |
| Add a new entity | Model class, migration, controller, serializer, routes, tests | Add a YAML file, run the loader |
| Add a field | Migration, model change, serializer change, validation change, tests | Add a field to the YAML file, run the loader |
| Validate input | Write validation code in models or middleware | Declare constraints in schema YAML (type, bounds, enum, FK) |
| Cross-field validation | Write custom validator class or method | Create a semantic_invariant policy row |
| Authenticate users | Integrate auth library, write middleware, handle sessions | Configure IdP in OpsDB, frontend acquires token, gate step 1 validates |
| Authorize access | Write authorization checks per endpoint, per action | Configure roles, groups, _requires_group, _access_classification — gate step 2 evaluates |
| Version data | Build version history tables, write versioning logic | Set versioned: true on entity — version rows created automatically |
| Audit changes | Build audit log table, write audit middleware, decide what to log | Automatic — every API action produces an audit entry |
| Approval workflows | Build approval models, approval logic, notification integration | Create approval_rule policy rows — gate step 7 routes automatically |
| Change management | Build change request system from scratch | Built in — every governed write is a change set with attribution and approval |
| Search and filter | Write query endpoints, handle pagination, build filter parsing | Use the search API — predicates, joins, projection, pagination built in |
| Reconstruct past state | Replay event log or diff chain (if you built one) | Call get_entity_at_time — O(1) lookup |
| Background processing | Build job queue, write workers, handle retry and failure | Write runners using the library suite — get/act/set pattern |
| External integration | Build API client, handle auth, retry, error handling, data mapping | Write a puller runner — library suite handles auth, retry, circuit breaking |
| Notifications | Build notification system, channel routing, recipient resolution | Write a notification runner — reads config data for channels and recipients |
| Retention policies | Build cleanup jobs, configure per-table, handle compliance | Create retention policy rows — reaper runner enforces them |
| Multi-tenancy | Build tenant isolation in data layer, auth, queries | Configure _requires_group per entity — gate pipeline filters automatically |
| Draft editing mode | Build autosave, version commits, two-state editing | Set draft mode flags on entity — gate pipeline adjusts automatically |
| AI-generated summaries | Build LLM integration, prompt management, output storage | Write an AI observation runner — output is observation cache with freshness |
| AI-proposed changes | Build AI pipeline with human review workflow | AI runner proposes change sets — same gate pipeline, human approval via policy |
| Test validation rules | Write unit tests for each validator | Submit invalid data through the API — gate pipeline rejects it |
| Test authorization | Write auth tests per endpoint per role | Query the API as different users — gate pipeline filters results |
| Deploy a change | CI/CD pipeline: build, test, deploy binary | Schema changes: run loader. Policy changes: submit change set. Runner changes: deploy binary |
| Debug a problem | Read log files, step through code, query database directly | Query audit log, version history, observation cache through the search API |
| Onboard a developer | Read the codebase across models, controllers, services, middleware | Read schema YAML, policy rows, runner specs |
| Comply with SOC2/ISO/PCI/HIPAA | Build compliance evidence collection as a separate project | Query the audit log, version history, and evidence records that already exist |
