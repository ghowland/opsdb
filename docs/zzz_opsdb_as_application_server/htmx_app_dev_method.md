# Building Web Applications with HTMX and OpsDB

## A Development Method for Governed Web Applications

---

## 1. What This Is and Where It Stands

This document describes a method for building web applications. The frontend is HTMX — HTML pages that make HTTP requests and swap fragments. The backend is OpsDB — a governed data substrate that validates, authorizes, versions, and audits every interaction through a single API. Between them, where your application needs domain logic, thread runner mini services handle the computation and hand results to OpsDB for storage.

The method produces web applications where you write three things: schema YAML files defining your data model, handler functions containing your domain logic, and HTMX templates rendering your pages. Everything else — validation, authentication, authorization, versioning, change management, audit logging, search, pagination, concurrency control — is provided by the substrate.

**Current status.** The OpsDB substrate is fully specified. The specification covers the gate pipeline, the schema engine, the change management system, the versioning system, the audit log, the runner framework, and the shared library suite. A skeleton implementation exists with example code demonstrating each component. The system is not yet running. First implementation — building the working gate pipeline, the loader, and the API — makes the substrate viable for every application type and pattern described here. The HTMX framework layer described in this document is a design that builds on the substrate.

The design is complete. The engineering remaining is implementation, not research. This document teaches the method so developers can evaluate it now and build on it when the substrate reaches viability.

---

## 2. Three Paths, One Backend

Every endpoint in your application takes one of three paths. There is no fourth.

**Path 1: HTMX direct to OpsDB API.** The page makes an HTTP request to the OpsDB API. The API validates, authorizes, versions, and audits the operation. The response comes back. HTMX swaps the result into the page. No application code runs between the page and the API. Use this path when the request is pure CRUD — creating a task, editing a comment, listing projects, viewing a detail page. The schema defines the validation. The API enforces it. The page renders the result.

**Path 2: HTMX to thread runner mini service to OpsDB API.** The page makes an HTTP request to a mini service — a handler function running as a thread in your application server. The handler applies domain logic: checks availability across resource pools, computes invoice totals, validates business rules against external data, scores candidates. The handler produces a result. The framework writes that result to OpsDB through the API. The API validates, authorizes, versions, and audits. The response returns through the mini service to the page. HTMX swaps the result. Use this path when the request needs domain logic before the write — logic that the schema constraints cannot express.

**Path 3: HTMX reads runner-produced data from OpsDB API.** A polling runner — a separate background process on a cycle — reads from an external system (a payment processor, a weather API, a monitoring service) and writes observations to OpsDB through the API. The HTMX page reads those observations through the same search API it uses for everything else. Use this path when the data originates outside your application and arrives via background synchronization.

The decision framework:

| Question | Answer | Path |
|---|---|---|
| Does the request need domain logic before the write? | No | Path 1 — HTMX direct to API |
| Does the request need domain logic before the write? | Yes | Path 2 — HTMX to mini service to API |
| Does the data come from a background process or external source? | Yes | Path 3 — HTMX reads from API |

Every endpoint is one of these three. When designing an endpoint, ask the question. The answer determines what you write and what you skip.

---

## 3. The Schema Is Your Data Model

Your data model is a set of YAML files. One file per entity type. Each file declares the entity's fields with types, constraints, and relationships. A loader reads the files, generates database tables, and populates metadata that the API reads at runtime. After the loader runs, the API serves your entities.

Here is a project management application with four entities.

**project.yaml:**

```yaml
name: project
fields:
  name:
    type: varchar
    max_length: 255
  description:
    type: text
    nullable: true
  status:
    type: enum
    enum_values: [planning, active, paused, completed, archived]
    default: planning
  start_date:
    type: date
    nullable: true
  target_end_date:
    type: date
    nullable: true
governance:
  _requires_group:
    type: foreign_key
    references: ops_group
    nullable: true
versioned: true
soft_delete: true
```

**task.yaml:**

```yaml
name: task
fields:
  title:
    type: varchar
    max_length: 500
  description:
    type: text
    nullable: true
  status:
    type: enum
    enum_values: [backlog, todo, in_progress, review, done, cancelled]
    default: backlog
  priority:
    type: int
    min_value: 1
    max_value: 5
  due_date:
    type: date
    nullable: true
  project_id:
    type: foreign_key
    references: project
versioned: true
soft_delete: true
```

**task_assignment.yaml:**

```yaml
name: task_assignment
fields:
  task_id:
    type: foreign_key
    references: task
  user_id:
    type: foreign_key
    references: ops_user
  role:
    type: enum
    enum_values: [assignee, reviewer, observer]
    default: assignee
  assigned_date:
    type: date
versioned: true
```

**comment.yaml:**

```yaml
name: comment
fields:
  body:
    type: text
  task_id:
    type: foreign_key
    references: task
  parent_comment_id:
    type: foreign_key
    references: comment
    nullable: true
versioned: true
soft_delete: true
```

Run the loader. It validates every file — field types must be from the nine allowed types (int, float, varchar, text, boolean, datetime, date, json, enum, foreign_key), foreign key references must point to entities that exist, constraints must be consistent. If validation passes, the loader generates database tables and populates schema metadata tables.

The API now serves these entities. Create a task:

```
POST /api/task
{
  "title": "Design landing page",
  "status": "todo",
  "priority": 3,
  "project_id": 1
}
```

The API runs the request through a ten-step pipeline. Step 1 authenticates the caller. Step 2 checks authorization — does the caller have permission to create tasks in this project? Step 3 checks schema validation — does the `title` field exist? Is it a string? Step 4 checks bound validation — is `priority` between 1 and 5? Is `project_id` a real project? Step 5 evaluates any policy rules. Steps 6-7 handle versioning and change management. Step 8 writes an audit log entry. Step 9 executes the database insert. Step 10 returns the result.

If the request is valid, the response includes the created entity with its auto-generated `id`, `created_time`, and `updated_time`. If the request is invalid, the response identifies the exact step that failed, the field that caused it, and the constraint that was violated:

```json
{
  "error": "bound_validation_failed",
  "step": 4,
  "field": "priority",
  "constraint": "max_value",
  "limit": 5,
  "submitted": 8,
  "message": "priority must be at most 5"
}
```

You defined four YAML files. You now have a validated, authorized, versioned, audited API for projects, tasks, assignments, and comments. You wrote no controllers, no middleware, no migration files, no model classes, no serializers.

The schema is the single definition. The API validates against it. HTMX forms are generated from it. There is no second definition that could disagree.

---

## 4. HTMX Direct to API — Pure CRUD

For entities where no domain logic is needed before the write, the HTMX page talks directly to the OpsDB API. The HTML is derived from the schema. Field types map to input types. Constraints map to HTML attributes. The API's structured errors map back to form fields through HTMX swap targets.

**A task list page:**

```html
<div id="task-list">
  <form hx-get="/api/task/search" hx-target="#task-results" hx-trigger="submit">
    <select name="filter_status">
      <option value="">All</option>
      <option value="backlog">Backlog</option>
      <option value="todo">To Do</option>
      <option value="in_progress">In Progress</option>
      <option value="review">Review</option>
      <option value="done">Done</option>
    </select>

    <select name="filter_priority_gte">
      <option value="">Any Priority</option>
      <option value="1">1+</option>
      <option value="3">3+</option>
      <option value="5">5 only</option>
    </select>

    <button type="submit">Filter</button>
  </form>

  <div id="task-results">
    <!-- search results swapped here -->
  </div>

  <button hx-get="/api/task/search?cursor=${next_cursor}"
          hx-target="#task-results"
          hx-swap="innerHTML">
    Next Page
  </button>
</div>
```

The filter controls are derived from the schema. The `status` field is an enum — it becomes a select with options from `enum_values`. The `priority` field is an int with min 1 and max 5 — it becomes a range filter. The search API handles the predicate composition, the access control filtering, and the cursor pagination.

**A task edit form:**

```html
<form hx-put="/api/task/42"
      hx-target="#task-detail"
      hx-swap="innerHTML">

  <label for="title">Title</label>
  <input type="text" name="title" value="Design landing page"
         maxlength="500" required>
  <span id="error-title" class="error"></span>

  <label for="status">Status</label>
  <select name="status" required>
    <option value="backlog">Backlog</option>
    <option value="todo" selected>To Do</option>
    <option value="in_progress">In Progress</option>
    <option value="review">Review</option>
    <option value="done">Done</option>
    <option value="cancelled">Cancelled</option>
  </select>
  <span id="error-status" class="error"></span>

  <label for="priority">Priority</label>
  <input type="number" name="priority" value="3"
         min="1" max="5" required>
  <span id="error-priority" class="error"></span>

  <label for="due_date">Due Date</label>
  <input type="date" name="due_date" value="2026-06-15">
  <span id="error-due_date" class="error"></span>

  <button type="submit">Save</button>
</form>
```

The `maxlength="500"` on the title input comes from the schema's `max_length: 500`. The `min="1" max="5"` on the priority input comes from `min_value: 1, max_value: 5`. The select options come from `enum_values`. These are client-side hints for user experience — the API validates authoritatively regardless of what the HTML attributes say.

When the form submits, the API either accepts the write and returns the updated entity, or rejects it with a structured error. The HTMX response handling swaps the error into the correct field's error span:

```html
<!-- returned by API on validation failure, swapped into the form -->
<span id="error-priority" class="error">
  priority must be at most 5
</span>
```

**What happens with access control.** If the caller lacks permission to see a field — because the field has an `_access_classification` higher than the caller's clearance — the API omits the field from responses. The HTMX page never receives the field. There is no hidden HTML element with sensitive data that CSS hides. The data never reaches the browser. The page renders what the API returns, and the API returns only what the caller can see.

**What happens with change management.** Some writes return immediately — the change was auto-approved per policy and applied. Some writes return a pending status — the change requires human approval. The HTMX page handles both cases:

```html
<!-- returned by API when change is auto-approved and applied -->
<div id="task-detail">
  <h2>Design landing page</h2>
  <p>Status: In Progress</p>
  <p>Updated just now</p>
</div>

<!-- returned by API when change requires approval -->
<div id="task-detail">
  <h2>Design landing page</h2>
  <p>Status: To Do (change to In Progress pending approval)</p>
  <p>Awaiting approval from: Project Lead</p>
  <div hx-get="/api/change_set/417/status"
       hx-trigger="every 5s"
       hx-target="#approval-status">
    <span id="approval-status">Pending</span>
  </div>
</div>
```

The page doesn't decide which path to take. The API decides based on policy data. The page renders whichever response the API returns. If the policy changes — an entity type moves from auto-approve to approval-required — the page adapts without modification because it renders the API response, not a hardcoded workflow.

For pure CRUD endpoints, the developer writes: the schema YAML (which they already wrote), the HTMX templates (which can be generated from schema metadata), and nothing else. No controller. No validation code. No authorization check. No audit logging. No versioning logic.

---

## 5. Thread Runner Mini Services — Your Application Logic

When an endpoint needs domain logic before the write, the request goes through a thread runner mini service. The mini service is a handler function — not a controller, not a service class, not an endpoint. A function that receives the request context and returns a result.

A booking system needs availability checking. The schema can enforce that a booking has valid dates, a valid resource reference, and a valid customer reference. The schema cannot enforce that the requested time slot is available across resource pools with blackout dates and timezone adjustments. That is domain logic. It belongs in a handler function.

**The handler function:**

```python
def booking_validator(context):
    """
    Receives: context.request_data (the booking request fields)
              context.user (the authenticated caller)
              context.prior_results (results from earlier steps)

    Returns: accept(data) or reject(reasons)
    """
    booking = context.request_data
    resource_id = booking["resource_id"]
    start_time = booking["start_time"]
    end_time = booking["end_time"]

    # Read current bookings for this resource from OpsDB
    existing = context.api.search("booking", filters={
        "resource_id": resource_id,
        "status__in": ["confirmed", "pending"],
        "start_time__lt": end_time,
        "end_time__gt": start_time
    })

    if existing.count > 0:
        return reject([{
            "field": "start_time",
            "reason": "Resource is already booked for this time period"
        }])

    # Check blackout dates
    blackouts = context.api.search("blackout_date", filters={
        "resource_id": resource_id,
        "blackout_date__gte": start_time.date(),
        "blackout_date__lte": end_time.date()
    })

    if blackouts.count > 0:
        return reject([{
            "field": "start_time",
            "reason": "Requested dates include blackout periods",
            "detail": [b["blackout_date"] for b in blackouts.rows]
        }])

    return accept(booking)
```

This function is 30 lines. It receives validated, authenticated request data — the framework already confirmed the caller is who they claim to be and the data matches the schema. The function applies domain reasoning: is the slot available? Are there blackout conflicts? It returns accept or reject with structured reasons.

The function does not handle HTTP parsing. It does not check authentication. It does not validate field types or constraints. It does not write to the database. It does not log anything. It does not manage transactions. It makes a domain decision. Everything else is handled by the framework and the OpsDB API.

**Mini services vs polling runners.** Thread runner mini services and polling runners are different things with different jobs. Mini services are synchronous — they handle a request and return a response. They are latency-sensitive. They contain your application's request-path logic. Polling runners are asynchronous — they run on a cycle, read current state, and act on it. They are throughput-oriented. They contain your application's background logic. Both write through the same API. Both are governed by the same pipeline. But they serve different purposes.

A mini service that processes a booking request runs when the user clicks "Book." A polling runner that syncs payment status from Stripe runs every 30 seconds regardless of user activity. Same domain, different operational profiles, different homes.

---

## 6. Logic Paths — Composing Steps in YAML

Each custom endpoint is declared as a logic path — an ordered sequence of steps in YAML. The framework executes the steps in order. Each step's output is available to subsequent steps. First failure halts the pipeline.

**The step vocabulary is closed.** Seven step types. No plugins, no middleware chains, no custom step types.

`validate_schema` — runs the request data against OpsDB schema constraints as a fast-fail before hitting the API. Client-side pre-validation for user experience. The API validates again authoritatively.

`validate_custom` — calls a handler function for domain validation. The function receives the request context and returns accept or reject. This is where your application logic lives.

`verify_external` — calls an external service through the library suite for verification. Payment preauthorization, identity verification, address validation. The library suite handles authentication, retry, and circuit breaking. The step declaration specifies the service and the failure behavior.

`compute` — calls a handler function that transforms or enriches the data. The pricing calculator. The invoice totals computer. The scoring algorithm. Takes validated input and produces the data to write.

`write` — writes to OpsDB through the API. The ten-step gate pipeline runs. This is where data enters the governed substrate.

`query` — reads from OpsDB after the write. Fetches freshly written data joined with related entities for the response.

`notify` — dispatches notifications asynchronously. Uses the notification library with channel configuration from OpsDB data.

`return` — selects the HTMX template and passes the data from prior steps. The template renders the HTML fragment that HTMX swaps into the page.

**A booking request flow:**

```yaml
logic_paths:

  booking_request_flow:
    steps:
      - step: validate_schema
        source: opsdb_schema

      - step: validate_custom
        handler: booking_validator

      - step: verify_external
        handler: payment_preauth
        service: stripe
        on_failure: reject

      - step: compute
        handler: booking_finalizer

      - step: write
        target: opsdb
        operation: change_set

      - step: query
        source: opsdb
        query: booking_with_resource_and_customer

      - step: notify
        handler: booking_confirmation_notifier
        async: true

      - step: return
        template: booking/confirmation
```

Read this YAML and you know exactly what happens when a user submits a booking request. The data is checked against the schema. The booking validator checks availability and blackout dates. Stripe preauthorizes the payment. The booking finalizer computes the final price and confirmation code. The result is written to OpsDB as a change set. The freshly written booking is queried back with its resource and customer details joined. A confirmation notification is dispatched asynchronously. The confirmation page is rendered.

A simpler flow for task assignment:

```yaml
  task_assignment_flow:
    steps:
      - step: validate_schema
        source: opsdb_schema

      - step: validate_custom
        handler: assignment_validator

      - step: write
        target: opsdb
        operation: change_set

      - step: notify
        handler: assignment_notifier
        async: true

      - step: return
        template: task/detail
```

Five steps instead of eight. No external verification, no computation step, no post-write query. Simple flows have fewer steps.

**The route manifest ties paths to URLs:**

```yaml
app:
  name: booking_system
  opsdb: booking_appdb
  schema_ref: ./schema/

routes:
  - path: /resources
    entity: resource
    crud: true
    views: [list, detail, edit, create]

  - path: /customers
    entity: customer
    crud: true
    views: [list, detail, edit, create]

  - path: /bookings
    entity: booking
    crud: true
    views: [list, detail]

  - path: /bookings/request
    method: POST
    logic_path: booking_request_flow
    template: booking/request_form

  - path: /tasks/{id}/assign
    method: POST
    logic_path: task_assignment_flow
```

CRUD routes generate pages from schema metadata. Custom routes point to logic paths. Every endpoint in the application is visible in this file.

---

## 7. Reading Runner-Produced Data

Polling runners run on cycles. A payment status runner queries Stripe every 30 seconds and writes the current status of each pending payment to OpsDB observation cache tables. A weather runner queries a forecast API every hour and writes conditions to observation cache. A metrics runner pulls system health data and writes summaries.

HTMX pages read this data through the same search API used for everything else.

**A payment status display on an invoice page:**

```html
<div hx-get="/api/observation_cache_payment/search?booking_id=42&max_staleness_seconds=60"
     hx-trigger="load, every 10s"
     hx-target="#payment-status">
  <div id="payment-status">
    Loading payment status...
  </div>
</div>
```

This div loads payment status on page load and refreshes every 10 seconds. The `max_staleness_seconds=60` parameter tells the search API to filter out observations older than 60 seconds — if the payment runner hasn't written in the last minute, the response indicates stale data rather than showing outdated status.

**Weather alongside garden notes:**

```html
<div class="garden-journal">
  <div class="planting-notes">
    <div hx-get="/api/planting_note/search?plot_id=7&order=-created_time"
         hx-trigger="load"
         hx-target="#notes-list">
      <div id="notes-list">Loading notes...</div>
    </div>
  </div>

  <div class="weather-panel">
    <div hx-get="/api/observation_cache_weather/search?location_id=1&max_staleness_seconds=3600"
         hx-trigger="load"
         hx-target="#weather-data">
      <div id="weather-data">Loading weather...</div>
    </div>
  </div>
</div>
```

The planting notes are governed entities written by users through the API. The weather data is observation cache written by a polling runner. Both are read through the same search API. Both are rendered by HTMX swapping HTML fragments. The page doesn't know or care which path produced the data.

**The communication model.** Runners and HTMX pages never talk to each other directly. Runners write to OpsDB. HTMX pages read from OpsDB. OpsDB is the communication channel. There are no webhooks from runners to pages, no server-sent events, no WebSocket connections for most application patterns. The polling runner writes on its cycle. The HTMX page reads on its refresh interval. The data meets in OpsDB.

For most application read patterns — showing payment status, displaying weather, rendering dashboards with aggregated metrics, showing sync status from external systems — this model is simpler than real-time push. The HTMX `hx-trigger="every Ns"` provides the refresh. The observation cache provides the data. The freshness annotation provides staleness detection.

---

## 8. What the Substrate Handles

When you define an entity in the schema and run the loader, that entity immediately has all of the following. You do not build any of it.

**Schema validation.** Every write is checked against the declared field types and constraints. An integer field rejects strings. A varchar field rejects values exceeding its max_length. An enum field rejects values not in its declared set. A foreign key field rejects references to entities that don't exist. Every field, every write, every time.

**Five-layer authorization.** Layer 1: role and group membership — which operations the caller can perform on which entity types. Layer 2: per-entity governance — the `_requires_group` field scopes access to specific groups per row. Layer 3: per-field classification — `_access_classification` controls which fields the caller can see based on their clearance level. Fields the caller cannot see are omitted from API responses entirely. Layer 4: per-runner authority — automated writers are restricted to their declared scope. Layer 5: policy rules — separation of duty, time-of-day restrictions, custom constraints. All five layers compose via AND. First denial halts.

**Full version history.** Every write to a versioned entity creates a version row containing the complete entity state — all fields, not just the ones that changed. Reconstructing the state at any point in time is a single row lookup, not a chain replay. "What did this look like last Tuesday at 3pm" is one API call.

**Change management.** Writes to governed entities are expressed as change sets — bundles of proposed field changes with a stated reason. Change sets route to approvers based on policy data. Low-stakes changes auto-approve per policy — the change set is created, validated, and applied within the same request. The user experiences it as immediate. High-stakes changes route to human approvers. Both paths produce the same audit trail. Changing who approves what means changing policy data, not deploying code.

**Append-only audit logging.** Every API operation — read, write, approve, reject, search — produces an audit log entry recording the caller identity, the operation, the target entity, the outcome, and contextual metadata. The audit log table has no UPDATE or DELETE permission for any database role. Entries are written and never modified. The audit log is queryable through the same search API.

**Optimistic concurrency control.** When two people edit the same entity, the second submission detects that the entity has changed since they loaded it and rejects with a stale version error. Silent overwrites are prevented. The submitter reconciles and resubmits.

**Search API.** Filter predicates (equality, comparison, set membership, pattern matching, null checks, range), composable with AND/OR/NOT. Named join paths for traversing relationships. Projection for controlling which fields are returned. Cursor-based pagination. Freshness annotations for cached data. All bounded — maximum result size, maximum join depth, maximum query time — configurable per role.

**Retention policies.** Configurable per entity type as policy data. A reaper runner enforces them. Version history, observation cache, and audit entries have independently configurable retention.

**Draft mode.** Three per-table governance flags relax the recording properties for interactive editing. Authentication, authorization, and validation always run. Versioning, change management, and audit logging for interim saves can be skipped. Explicit version commits restore full governance. A document table can have draft mode for fluid editing while a budget table has full governance for every keystroke.

These exist on every entity from the moment you define it in the schema. You don't add them later. You don't build them. The first entity you define has all of them.

---

## 9. The App Compiler

The app compiler reads your YAML files and produces a running application. It validates everything before runtime.

**Input files:**

- Schema YAML — your entity definitions (read by the OpsDB loader)
- Route manifest YAML — your endpoint declarations
- Logic path YAML — your step sequences for custom endpoints
- Handler registrations — your domain logic functions mapped to handler names

**What the compiler validates:**

Every entity referenced in a route exists in the schema. Every handler referenced in a logic path is registered. Every query step references valid entity types and field names from the schema. Every external service referenced in verify_external steps has a configured connector in the library suite. Every logic path ends with a return step. Every CRUD route references an entity with the views it declares.

If anything doesn't resolve, the compiler rejects with a structured error before runtime:

```
ERROR: Route /bookings/request references logic_path 'booking_request_flow'
       Step 2 (validate_custom) references handler 'booking_validator'
       No handler registered with name 'booking_validator'
       
       Registered handlers: assignment_validator, invoice_finalizer
```

You don't discover at runtime that a route is misconfigured.

**What the compiler produces:**

The route table mapping every URL path to its handler — either a generated CRUD handler or a step pipeline executor.

HTMX templates for CRUD entities — generated from schema metadata. List views with filter controls, detail views, edit forms, create forms. All field types mapped to input types. All constraints mapped to HTML attributes.

The step pipeline runtime — the engine that executes logic path steps in order, threads context through, calls handlers, calls the OpsDB API, calls external services through the library suite, and renders HTMX templates for responses.

**Deployment by artifact type:**

Schema changes deploy through the OpsDB schema executor — a specialized runner that applies approved DDL changes atomically. Additive only. New entities and new fields don't require downtime.

Route and logic path changes deploy through the compiler — rebuild and restart the application server. These are YAML file changes, not code changes.

Handler code deploys as binary updates — rebuild the application with the updated handler functions and restart.

Policy changes — approval rules, access control configuration, retention policies — deploy as change sets through the OpsDB API. They take effect when the change set is approved and applied. No rebuild, no restart. Changing who approves what, who can access what, or how long data is retained is a data change, not a deployment.

---

## 10. Walkthrough — Build a Booking System

This section builds a complete application from zero. Every file, every decision, every step.

The application is a resource booking system. Users browse resources, manage their customer profile, request bookings with availability checking and payment preauthorization, and view booking status including payment state synced from Stripe.

### 10.1 Schema

Four governed entities and one observation cache entity.

**schema/resource.yaml:**

```yaml
name: resource
fields:
  name:
    type: varchar
    max_length: 255
  description:
    type: text
    nullable: true
  resource_type:
    type: enum
    enum_values: [room, equipment, vehicle, space]
  hourly_rate:
    type: float
    min_value: 0
    precision_decimal_places: 2
  is_bookable:
    type: boolean
    default: true
  location:
    type: varchar
    max_length: 500
    nullable: true
versioned: true
soft_delete: true
```

**schema/customer.yaml:**

```yaml
name: customer
fields:
  name:
    type: varchar
    max_length: 255
  email:
    type: varchar
    max_length: 255
    must_be_unique: true
  phone:
    type: varchar
    max_length: 50
    nullable: true
  stripe_customer_id:
    type: varchar
    max_length: 255
    nullable: true
governance:
  _access_classification:
    fields:
      stripe_customer_id: restricted
      email: internal
      phone: internal
versioned: true
soft_delete: true
```

The `stripe_customer_id` field is classified `restricted` — only users with sufficient clearance see it. The `email` and `phone` fields are classified `internal` — not visible to public-role users. The API enforces this. The HTMX page never receives the fields the caller can't see.

**schema/booking.yaml:**

```yaml
name: booking
fields:
  resource_id:
    type: foreign_key
    references: resource
  customer_id:
    type: foreign_key
    references: customer
  start_time:
    type: datetime
  end_time:
    type: datetime
  status:
    type: enum
    enum_values: [pending, confirmed, cancelled, completed, no_show]
    default: pending
  total_price:
    type: float
    min_value: 0
    precision_decimal_places: 2
  confirmation_code:
    type: varchar
    max_length: 20
    nullable: true
    must_be_unique: true
  notes:
    type: text
    nullable: true
versioned: true
soft_delete: true
```

**schema/blackout_date.yaml:**

```yaml
name: blackout_date
fields:
  resource_id:
    type: foreign_key
    references: resource
  blackout_date:
    type: date
  reason:
    type: varchar
    max_length: 255
    nullable: true
versioned: true
```

**schema/observation_cache_payment.yaml:**

```yaml
name: observation_cache_payment
fields:
  booking_id:
    type: foreign_key
    references: booking
  stripe_payment_intent_id:
    type: varchar
    max_length: 255
  payment_status:
    type: enum
    enum_values: [pending, processing, succeeded, failed, refunded]
  amount_cents:
    type: int
    min_value: 0
  failure_reason:
    type: varchar
    max_length: 500
    nullable: true
  _observed_time:
    type: datetime
```

This is observation cache — written by the payment status polling runner, not by users. The `_observed_time` field enables freshness filtering.

Run the loader. Five entities are now served by the API.

### 10.2 Routes and Logic Paths

**app/manifest.yaml:**

```yaml
app:
  name: booking_system
  opsdb: booking_appdb
  schema_ref: ./schema/

routes:
  # Path 1: CRUD direct to API
  - path: /resources
    entity: resource
    crud: true
    views: [list, detail]

  - path: /resources/admin
    entity: resource
    crud: true
    views: [list, detail, edit, create]

  - path: /customers
    entity: customer
    crud: true
    views: [list, detail, edit, create]

  - path: /bookings
    entity: booking
    crud: true
    views: [list, detail]

  # Path 2: Custom logic through mini service
  - path: /bookings/request
    method: POST
    logic_path: booking_request_flow
    template: booking/request_form

  - path: /bookings/{id}/cancel
    method: POST
    logic_path: booking_cancellation_flow

  # Path 3: Runner-produced data read through API
  # (No route declaration needed — the HTMX templates
  # query observation_cache_payment directly through
  # the search API using hx-get)
```

Path 3 doesn't need route declarations because the HTMX templates read observation cache directly from the API. The route manifest only declares endpoints that the application server handles.

**app/logic_paths.yaml:**

```yaml
logic_paths:

  booking_request_flow:
    steps:
      - step: validate_schema
        source: opsdb_schema

      - step: validate_custom
        handler: booking_validator

      - step: verify_external
        handler: payment_preauth
        service: stripe
        on_failure: reject

      - step: compute
        handler: booking_finalizer

      - step: write
        target: opsdb
        operation: change_set

      - step: query
        source: opsdb
        query: booking_with_resource_and_customer

      - step: notify
        handler: booking_confirmation_notifier
        async: true

      - step: return
        template: booking/confirmation

  booking_cancellation_flow:
    steps:
      - step: validate_custom
        handler: cancellation_validator

      - step: verify_external
        handler: payment_refund
        service: stripe
        on_failure: reject

      - step: compute
        handler: cancellation_finalizer

      - step: write
        target: opsdb
        operation: change_set

      - step: query
        source: opsdb
        query: booking_with_resource_and_customer

      - step: notify
        handler: cancellation_notifier
        async: true

      - step: return
        template: booking/cancelled
```

### 10.3 Handler Functions

**handlers/booking_validator.py:**

```python
def booking_validator(context):
    booking = context.request_data
    resource_id = booking["resource_id"]
    start_time = booking["start_time"]
    end_time = booking["end_time"]

    # Validate time ordering
    if end_time <= start_time:
        return reject([{
            "field": "end_time",
            "reason": "End time must be after start time"
        }])

    # Check resource is bookable
    resource = context.api.get_entity("resource", resource_id)
    if not resource["is_bookable"]:
        return reject([{
            "field": "resource_id",
            "reason": "This resource is not currently available for booking"
        }])

    # Check for overlapping bookings
    overlapping = context.api.search("booking", filters={
        "resource_id": resource_id,
        "status__in": ["pending", "confirmed"],
        "start_time__lt": end_time,
        "end_time__gt": start_time
    })

    if overlapping.count > 0:
        return reject([{
            "field": "start_time",
            "reason": "Resource is already booked for this time period"
        }])

    # Check blackout dates
    blackouts = context.api.search("blackout_date", filters={
        "resource_id": resource_id,
        "blackout_date__gte": start_time.date(),
        "blackout_date__lte": end_time.date()
    })

    if blackouts.count > 0:
        return reject([{
            "field": "start_time",
            "reason": "Requested dates include blackout periods"
        }])

    # Pass resource forward for price computation
    return accept(booking, extra={"resource": resource})
```

**handlers/payment_preauth.py:**

```python
def payment_preauth(context):
    booking = context.request_data
    resource = context.prior_results["resource"]
    customer = context.api.get_entity("customer", booking["customer_id"])

    hours = (booking["end_time"] - booking["start_time"]).total_seconds() / 3600
    amount_cents = int(resource["hourly_rate"] * hours * 100)

    if not customer.get("stripe_customer_id"):
        return reject([{
            "field": "customer_id",
            "reason": "Customer has no payment method on file"
        }])

    result = context.stripe.payment_intents.create(
        amount=amount_cents,
        currency="usd",
        customer=customer["stripe_customer_id"],
        capture_method="manual"
    )

    if result.status != "requires_capture":
        return reject([{
            "field": "customer_id",
            "reason": "Payment preauthorization failed"
        }])

    return accept(booking, extra={
        "payment_intent_id": result.id,
        "amount_cents": amount_cents
    })
```

**handlers/booking_finalizer.py:**

```python
import secrets
import string

def booking_finalizer(context):
    booking = context.request_data
    resource = context.prior_results["resource"]
    amount_cents = context.prior_results["amount_cents"]

    code = ''.join(secrets.choice(string.ascii_uppercase + string.digits) for _ in range(8))

    hours = (booking["end_time"] - booking["start_time"]).total_seconds() / 3600

    return accept({
        **booking,
        "total_price": round(resource["hourly_rate"] * hours, 2),
        "confirmation_code": code,
        "status": "confirmed"
    })
```

Each handler is short. Each does one thing. The booking validator checks domain rules — 35 lines. The payment preauth calls Stripe through the library suite — 20 lines. The booking finalizer computes the final price and generates a confirmation code — 10 lines. The total application logic for the booking request flow is 65 lines across three functions.

### 10.4 Polling Runner

The payment status runner syncs from Stripe on a 30-second cycle.

**runners/payment_status_puller.py:**

```python
class PaymentStatusPuller(Runner):
    kind = "puller"
    cycle_seconds = 30
    targets = ["observation_cache_payment"]

    def get(self):
        # Read all bookings with pending/confirmed status
        self.bookings = self.api.search("booking", filters={
            "status__in": ["pending", "confirmed"]
        })

    def act(self):
        self.results = []
        for booking in self.bookings.rows:
            # Find the payment intent for this booking
            existing = self.api.search("observation_cache_payment", filters={
                "booking_id": booking["id"]
            })

            if existing.count == 0:
                continue

            payment = existing.rows[0]
            intent = self.stripe.payment_intents.retrieve(
                payment["stripe_payment_intent_id"]
            )

            self.results.append({
                "booking_id": booking["id"],
                "stripe_payment_intent_id": intent.id,
                "payment_status": self._map_status(intent.status),
                "amount_cents": intent.amount,
                "failure_reason": intent.last_payment_error.message
                    if intent.last_payment_error else None
            })

    def set(self):
        for result in self.results:
            self.api.write_observation(
                "observation_cache_payment",
                result,
                uniqueness_key=["booking_id"]
            )

    def _map_status(self, stripe_status):
        mapping = {
            "requires_payment_method": "pending",
            "requires_confirmation": "pending",
            "requires_action": "processing",
            "processing": "processing",
            "succeeded": "succeeded",
            "canceled": "failed",
        }
        return mapping.get(stripe_status, "pending")
```

The runner follows the three-phase pattern. Get: read bookings that need payment tracking. Act: query Stripe for current payment status through the library suite. Set: write observations to OpsDB through the API. The `uniqueness_key` ensures each booking has one observation row that gets updated each cycle rather than accumulating duplicates.

The library suite handles Stripe authentication, retry on transient failures, and circuit breaking if Stripe is down. The runner handles the domain logic: which bookings to check, how to map Stripe statuses to application statuses.

### 10.5 HTMX Pages

**templates/booking/request_form.html:**

```html
<div id="booking-form-container">
  <h2>Request a Booking</h2>

  <form hx-post="/bookings/request"
        hx-target="#booking-form-container"
        hx-swap="innerHTML">

    <label for="resource_id">Resource</label>
    <select name="resource_id" required
            hx-get="/api/resource/search?is_bookable=true"
            hx-trigger="load"
            hx-target="#resource-options">
      <option value="">Select a resource...</option>
      <span id="resource-options"></span>
    </select>
    <span id="error-resource_id" class="error"></span>

    <label for="customer_id">Customer</label>
    <select name="customer_id" required
            hx-get="/api/customer/search?order=name"
            hx-trigger="load"
            hx-target="#customer-options">
      <option value="">Select a customer...</option>
      <span id="customer-options"></span>
    </select>
    <span id="error-customer_id" class="error"></span>

    <label for="start_time">Start Time</label>
    <input type="datetime-local" name="start_time" required>
    <span id="error-start_time" class="error"></span>

    <label for="end_time">End Time</label>
    <input type="datetime-local" name="end_time" required>
    <span id="error-end_time" class="error"></span>

    <label for="notes">Notes</label>
    <textarea name="notes" maxlength="65535"></textarea>

    <button type="submit">Request Booking</button>
  </form>
</div>
```

The resource select populates itself on page load by querying the search API for bookable resources. The customer select does the same. Both are HTMX requests — no JavaScript needed to populate dropdowns from the API.

**templates/booking/confirmation.html:**

```html
<div id="booking-form-container">
  <h2>Booking Confirmed</h2>

  <div class="confirmation-details">
    <p><strong>Confirmation Code:</strong> ${booking.confirmation_code}</p>
    <p><strong>Resource:</strong> ${resource.name}</p>
    <p><strong>Time:</strong> ${booking.start_time} — ${booking.end_time}</p>
    <p><strong>Total:</strong> $${booking.total_price}</p>
    <p><strong>Status:</strong> ${booking.status}</p>
  </div>

  <div class="payment-status"
       hx-get="/api/observation_cache_payment/search?booking_id=${booking.id}&max_staleness_seconds=60"
       hx-trigger="load, every 10s"
       hx-target="#payment-info">
    <div id="payment-info">
      Checking payment status...
    </div>
  </div>

  <a hx-get="/bookings/${booking.id}"
     hx-target="#main-content">View Booking Details</a>
</div>
```

The confirmation page shows the booking details from the query step's response and starts polling payment status from the observation cache. The payment status div refreshes every 10 seconds. The polling runner writes the current Stripe status every 30 seconds. The page picks it up on its next refresh cycle.

**templates/booking/detail.html:**

```html
<div id="booking-detail">
  <h2>Booking ${booking.confirmation_code}</h2>

  <div class="booking-info">
    <p><strong>Resource:</strong> ${resource.name} (${resource.resource_type})</p>
    <p><strong>Customer:</strong> ${customer.name}</p>
    <p><strong>Time:</strong> ${booking.start_time} — ${booking.end_time}</p>
    <p><strong>Total:</strong> $${booking.total_price}</p>
    <p><strong>Status:</strong> ${booking.status}</p>
  </div>

  <div class="payment-status"
       hx-get="/api/observation_cache_payment/search?booking_id=${booking.id}&max_staleness_seconds=60"
       hx-trigger="load, every 10s"
       hx-target="#payment-info">
    <div id="payment-info">Loading payment status...</div>
  </div>

  <div class="booking-history">
    <h3>History</h3>
    <div hx-get="/api/booking/${booking.id}/history"
         hx-trigger="load"
         hx-target="#history-list">
      <div id="history-list">Loading history...</div>
    </div>
  </div>

  <div class="booking-actions">
    <button hx-post="/bookings/${booking.id}/cancel"
            hx-target="#booking-detail"
            hx-confirm="Cancel this booking?">
      Cancel Booking
    </button>
  </div>
</div>
```

The detail page shows booking info, live payment status from observation cache, version history from the OpsDB versioning system, and a cancel action that goes through the cancellation logic path. Three paths in one page: governed entity data (path 1 read), runner-produced observation cache (path 3), and a custom action (path 2).

### 10.6 What the Developer Wrote

Total files:

- 5 schema YAML files (entities)
- 1 route manifest YAML file
- 1 logic paths YAML file
- 4 handler functions (~100 lines total)
- 1 polling runner (~50 lines)
- 4 HTMX templates (plus generated CRUD templates)

Total application code: roughly 150 lines of domain logic across handlers and the runner, plus YAML declarations and HTML templates.

What the developer did not write: authentication, authorization, input validation beyond domain rules, API endpoints, database migrations, model classes, serializers, audit logging, version history management, change management workflows, concurrency control, search and pagination infrastructure, retry logic for external APIs, background job infrastructure, notification dispatch infrastructure.

### 10.7 Testing the Application

**Schema tests.** Submit valid and invalid data through the API. Create a booking with end_time before start_time — the booking_validator rejects it. Create a booking with priority outside the declared range — the gate pipeline rejects it at step 4. Create a booking referencing a nonexistent resource — the gate pipeline rejects it at step 4. Each rejection returns a structured error identifying the field and constraint.

**Handler tests.** Call each handler function with test context. Call booking_validator with an overlapping booking in the database — it returns reject. Call it with a clear time slot — it returns accept. Call payment_preauth with a customer missing a Stripe ID — it returns reject. Each handler is a function. Test it like a function.

**Integration tests.** Submit a full booking request through the logic path. Verify the booking entity was created in OpsDB. Verify the version row exists with the correct field values. Verify the audit log entry records the operation, the caller, and the outcome. Verify the payment observation cache row was written by the runner on its next cycle. Each verification is a search API call.

**What you don't test.** You don't test that the gate pipeline validates field types — the platform tests that. You don't test that authorization filters search results — the platform tests that. You don't test that version rows contain full state — the platform tests that. You test your domain logic and your declarations.

---

## 11. What This Method Changes About How You Work

**Sprint planning.** Infrastructure stories disappear. There is no "add authentication" story, no "build validation layer" story, no "implement audit logging" story. The remaining categories are schema stories (small, measured in hours, but permanent — invest grooming time in getting names and relationships right), handler stories (conventional code, your domain logic), runner stories (small programs using the library suite), and frontend stories (HTMX templates, unchanged from normal frontend work).

**Estimation.** What was an 8-point story in conventional development — an entity with CRUD endpoints, validation, authorization, tests — is a 1-2 point story. A YAML file, a loader run, and verification through the API. Custom logic paths are larger but bounded — the handler functions are the work, and they're 20-50 lines each.

**Testing.** The gate pipeline is the test runner for validation and authorization. You verify your declarations by submitting data through the API and checking accept/reject responses. You test your handler functions as functions. You don't write tests for infrastructure concerns.

**Debugging.** Every problem follows the same investigation path. What happened? Query the audit log. What changed? Query the version history. What did the runner see? Query the observation cache. What was the state at the time? Call get_entity_at_time. Every step is a search API call through the same interface your application uses.

**Schema decisions are permanent.** Fields cannot be deleted, renamed, or have their types changed. This is intentional — it means version history, audit logs, and every consumer that references a field by name never break. The cost is that mistakes in naming or typing require the six-step duplication pattern to correct: add a new field alongside the old one, migrate writers and readers, deprecate the old field. The old field remains forever. Invest the grooming time to get schema design right. Runner code is replaceable. Schema decisions are not.

**The YAML is the documentation.** Read the route manifest — you see every endpoint. Read a logic path — you see every step a request passes through. Read the schema — you see every entity, field, and constraint. A new developer reads three YAML files and understands the application's structure without reading implementation code. The implementation code is just the handler functions — the domain logic — which is the only part that requires domain expertise to understand.

---

## 12. Current Status and Path to Viability

The OpsDB substrate is fully specified. The specification covers:

- The ten-step gate pipeline with authentication, five-layer authorization, schema validation, bound validation, policy evaluation, versioning preparation, change management routing, audit logging, execution, and response construction.
- The schema engine with nine types, three modifiers, closed constraint vocabulary, and the loader that generates database structure from YAML files.
- The change management system with change set lifecycle, approval routing from policy data, auto-approval, emergency path, and bulk operations.
- The versioning system with full-state version rows and point-in-time reconstruction.
- The append-only audit log with optional cryptographic chaining.
- The runner framework with ten runner kinds, three-phase pattern, three disciplines, gating modes, and authority declarations.
- The shared library suite with OpsDB API client, external service connectors, retry and resilience, notification dispatch, and scope enforcement.

Skeleton implementation exists with example code for each component. The specification includes concrete schema examples, API interaction examples, runner examples, and logic path examples.

The system is not yet running. The work remaining is first implementation:

**Phase 1: Core substrate.** Build the working schema loader, the gate pipeline, and the API. After this phase, developers can define schema YAML, run the loader, and interact with governed entities through the API. Schema validation, authorization, versioning, and audit logging are operational.

**Phase 2: Runner framework.** Build the runner execution environment, the shared library suite, and the standard runner kinds. After this phase, polling runners, reconcilers, and verifiers operate against the substrate.

**Phase 3: HTMX framework layer.** Build the app compiler, the route manifest processor, the logic path step pipeline, the handler registration system, and the HTMX template generator. After this phase, the full development method described in this document is operational.

Phase 1 is the critical path. The gate pipeline is the substrate — everything else builds on it. Phase 2 enables background processing. Phase 3 enables the web application development method. Each phase produces a usable system. Phase 1 alone is sufficient for API-only applications. Phase 1 plus 2 is sufficient for API-plus-automation applications. All three phases produce the full HTMX web application method.

The design is complete. The implementation is engineering — bounded, estimable, and sequential. Each phase has a clear deliverable and a clear test: does the gate pipeline accept valid writes and reject invalid ones? Do runners execute on cycle and write through the API? Does the compiler produce a working application from YAML files and handler functions?

---

## Summary

The method has three layers. HTMX pages handle presentation — HTML with attributes that make HTTP requests and swap fragments. Thread runner mini services handle domain logic — handler functions that receive validated, authenticated request data and return accept/reject or transformed data. OpsDB handles governance — validation, authorization, versioning, change management, and audit on every operation through a single API.

Every endpoint takes one of three paths. Pure CRUD goes direct from HTMX to the OpsDB API with no application code. Custom logic goes through a mini service composed as a step pipeline declared in YAML. Runner-produced data is read from OpsDB observation cache through the same search API.

You write schema YAML, handler functions, logic path YAML, and HTMX templates. The substrate handles everything between the frontend and the database. The compiler validates everything resolves before runtime. The gate pipeline enforces governance on every operation without application code.

The schema is the single definition of your data model. The route manifest is the single definition of your application's endpoints. The logic paths are the single definition of your request processing flows. Each definition exists in one place. There is no second definition that could disagree.
