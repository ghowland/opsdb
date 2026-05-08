# ProQL: Building a Prolog Query Language in Go
## A Mechanical Reference for Schema-Driven Logic Queries

---

## 1. What ProQL Is and What It Replaces

ProQL is a minimal Prolog evaluator that queries data through DR paths resolved against a schema. It replaces structured predicate APIs — filter/join/project/paginate — with logic programming where variable unification is the join, DR path resolution is the schema traversal, and backtracking is the search.

The same query expressed three ways:

**SQL:**
```sql
SELECT b.id, b.status, r.name
FROM booking b
JOIN resource r ON b.resource_id = r.id
WHERE b.status = 'confirmed'
AND r.name = 'Conference Room A';
```

**OpsDB Search API:**
```json
{
  "entity_type": "booking",
  "filters": {"status": "confirmed"},
  "joins": ["booking.resource"],
  "projection": ["id", "status", "resource.name"]
}
```

The search API requires that `booking.resource` is pre-registered as a named join path in schema metadata. If the join path wasn't declared, this query can't run without registering it first.

**ProQL:**
```prolog
?- booking.B.status("confirmed"),
   booking.B.resource_id(RID),
   resource.RID.name("Conference Room A").
```

The variable `RID` appears in both the booking lookup and the resource lookup. That shared variable is the join. No pre-registration needed. Any relationship expressible through the schema's foreign keys is queryable through variable binding.

Where ProQL separates from the search API is multi-entity queries that would require multiple API calls:

```prolog
?- booking.B.resource_id(RID),
   booking.B.status(S),
   member(S, [pending, confirmed]),
   booking.B.start_time(Start),
   booking.B.end_time(End),
   <(Start, RequestedEnd),
   >(End, RequestedStart),
   resource.RID.name(RName),
   booking.B.customer_id(CID),
   customer.CID.name(CName).
```

One query. Three entity types joined through shared variables. Time range comparison. Set membership filter. The search API would need three separate calls — search bookings, look up each resource, look up each customer — plus application code to join the results and filter by time overlap.

The search API remains better for simple CRUD reads: list all tasks, get one entity by ID, paginate through results. ProQL is for queries where the logic is the query — multi-entity joins, recursive traversals, negation, complex filtering across relationships.

ProQL results are bidirectional. Edit a value in the results, the system constructs a change set and submits it through the OpsDB API gate. All ten pipeline steps run — validation, authorization, versioning, audit. The query is both the read interface and the write interface.

---

## 2. The Term

The Term is the universal value container. It is a fat struct — all fields present, one set active based on the type tag. Unused fields are zero-valued. No pointer chasing, no union tricks, no indirection. The type tag tells you which fields are live.

### 2.1 Term Types

The type enum covers every value shape the OpsDB substrate and application layer produce or consume:

```go
type TermType int32

const (
    // Core logic types
    Atom     TermType = iota  // Literal string: "active", "project"
    Variable                   // Unbound variable: X, WID, Status
    Number                     // Numeric value: float64
    Boolean                    // true/false
    Null                       // Explicit null
    List                       // Ordered collection of terms
    Map                        // Key-value pairs for JSON payloads

    // Temporal
    DateTime                   // Timestamp with timezone
    Date                       // Date without time
    Duration                   // Time span for TTLs, retention, timeouts
    TimeRange                  // Start/end pair for bookings, versions
    CronExpr                   // Cron expression for schedules

    // Identity
    UserID                     // Human identity
    ServiceAccountID           // Runner identity
    GroupID                    // Access group
    RoleID                     // Role reference
    SessionID                  // Active session

    // Entity references
    EntityRef                  // Entity type + ID pair
    FieldRef                   // Entity type + field name
    VersionRef                 // Entity type + ID + version serial
    ChangeSetRef               // Change set ID

    // Schema
    SchemaType                 // int, float, varchar, text, boolean,
                               // datetime, date, json, enum, foreign_key
    EnumValue                  // Value from a declared enum set
    Constraint                 // min, max, length constraint
    JSONPayload                // Discriminated typed JSON

    // Authorization
    AccessClassification       // public, internal, confidential,
                               // restricted, regulated
    Permission                 // read, write, create, delete, approve
    PolicyResult               // allow, deny, require_approval
    AuthScope                  // Composed result from all five layers

    // Change management
    ChangeSetStatus            // draft, submitted, validating,
                               // pending_approval, approved, applied
    FieldChange                // Entity, field, old value, new value
    ApprovalRequirement        // Group + count needed
    ApprovalDecision           // approve/reject with identity

    // Versioning
    VersionStamp               // Monotonic serial
    VersionState               // Full state snapshot
    DiffEntry                  // Single field difference

    // Audit
    AuditAction                // read, create, update, delete, approve
    AuditOutcome               // success, validation_failed,
                               // authorization_denied
    AuditEntry                 // Complete audit record
    CorrelationID              // Links related operations

    // Runner
    RunnerKind                 // puller, reconciler, verifier, etc.
    RunnerPhase                // get, act, set
    GatingMode                 // direct_write, auto_approve,
                               // approval_required
    RunnerBound                // Specific bound with limit and consumed
    EvidenceRecord             // Pass/fail from verifier
    ObservationEntry           // Cached external state with freshness

    // Governance
    GovernanceFlag             // _autoversion_disabled, etc.
    PropertyImpact             // Which properties a flag weakens

    // Query results
    Binding                    // Complete variable binding set
    ResultSet                  // Ordered collection of bindings

    // Spatial
    Point2D                    // x, y
    Rect2D                     // x, y, width, height
    GeoCoord                   // latitude, longitude
    GeoRadius                  // center + radius

    // Financial
    Money                      // amount + currency code
    Percentage                 // Explicit percentage semantics

    // State machine
    StateTransition            // from, to, is_valid
    LifecycleGraph             // States + valid transitions

    // Notification
    Channel                    // email, slack, webhook, push
    Recipient                  // Resolved target
    EscalationStep             // Step with timing

    // External system
    AuthorityPointer           // Where external facts live
    ExternalCredential         // Opaque handle, never the secret
    CircuitState               // closed, open, half_open
)
```

### 2.2 The Term Struct

```go
type Term struct {
    Type TermType

    // --- Core ---
    AtomVal     string
    VariableVal string
    NumberVal   float64
    BoolVal     bool
    ListVal     []Term
    MapVal      map[string]Term

    // --- Temporal ---
    TimeVal     time.Time
    DateVal     time.Time
    DurationVal time.Duration
    TimeRangeVal [2]time.Time
    CronVal     string

    // --- Identity ---
    IdentityID  int64
    IdentityStr string

    // --- Entity reference ---
    EntityType  string
    EntityID    int64
    FieldName   string
    VersionNum  int64
    ChangeSetID int64

    // --- Schema ---
    SchemaTypeVal string
    EnumVal       string
    EnumSet       []string
    ConstraintKey string
    ConstraintNum float64
    JSONVal       map[string]Term
    Discriminator string

    // --- Authorization ---
    Classification string
    PermissionVal  string
    PolicyResultVal string
    AuthLayers     [5]bool
    DeniedLayer    int32
    DeniedReason   string

    // --- Change management ---
    CSStatus      string
    OldValue      *Term
    NewValue      *Term
    ApprovalGroup string
    ApprovalCount int32
    ApprovalBy    int64
    ApprovalTime  time.Time
    IsEmergency   bool

    // --- Version ---
    VersionSerial int64
    VersionFields map[string]Term
    DiffField     string
    DiffOld       *Term
    DiffNew       *Term

    // --- Audit ---
    AuditActionVal  string
    AuditOutcomeVal string
    AuditCallerID   int64
    AuditCallerType string
    AuditTargetType string
    AuditTargetID   int64
    AuditTimestamp  time.Time
    AuditRequestID  string
    AuditChainHash  string

    // --- Runner ---
    RunnerKindVal   string
    RunnerPhaseVal  string
    GatingModeVal   string
    BoundKey        string
    BoundLimit      float64
    BoundConsumed   float64
    EvidencePass    bool
    EvidenceDetail  string
    ObservedTime    time.Time
    Freshness       float64

    // --- Governance ---
    FlagName      string
    FlagEnabled   bool
    AffectedProps []string

    // --- Binding/Result ---
    BindingVars    map[string]Term
    ResultBindings []map[string]Term

    // --- Spatial ---
    X         float64
    Y         float64
    Width     float64
    Height    float64
    Latitude  float64
    Longitude float64
    Radius    float64

    // --- Financial ---
    Amount   float64
    Currency string
    Pct      float64

    // --- State machine ---
    FromState   string
    ToState     string
    IsValid     bool
    States      []string
    Transitions [][2]string

    // --- Notification ---
    ChannelType    string
    RecipientID    int64
    RecipientAddr  string
    EscalationIdx  int32
    EscalationWait time.Duration

    // --- External ---
    AuthPointerTarget string
    AuthPointerURI    string
    CircuitStateVal   string
    CircuitFailCount  int32
    CircuitLastFail   time.Time
}
```

### 2.3 Term Construction

```go
func AtomTerm(val string) Term {
    return Term{Type: Atom, AtomVal: val}
}

func VarTerm(name string) Term {
    return Term{Type: Variable, VariableVal: name}
}

func NumTerm(val float64) Term {
    return Term{Type: Number, NumberVal: val}
}

func TimeTerm(val time.Time) Term {
    return Term{Type: DateTime, TimeVal: val}
}

func EntityRefTerm(entityType string, id int64) Term {
    return Term{Type: EntityRef, EntityType: entityType, EntityID: id}
}

func MoneyTerm(amount float64, currency string) Term {
    return Term{Type: Money, Amount: amount, Currency: currency}
}

func EnumTerm(val string, validSet []string) Term {
    return Term{Type: EnumValue, EnumVal: val, EnumSet: validSet}
}

func BoolTerm(val bool) Term {
    return Term{Type: Boolean, BoolVal: val}
}

func NullTerm() Term {
    return Term{Type: Null}
}

func ListTerm(items []Term) Term {
    return Term{Type: List, ListVal: items}
}
```

### 2.4 Term Equality

```go
func (t *Term) Equals(other *Term) bool {
    if t.Type != other.Type {
        return false
    }
    switch t.Type {
    case Atom:
        return t.AtomVal == other.AtomVal
    case Variable:
        return t.VariableVal == other.VariableVal
    case Number:
        return t.NumberVal == other.NumberVal
    case Boolean:
        return t.BoolVal == other.BoolVal
    case Null:
        return true
    case DateTime:
        return t.TimeVal.Equal(other.TimeVal)
    case Date:
        return t.DateVal.Equal(other.DateVal)
    case EntityRef:
        return t.EntityType == other.EntityType &&
               t.EntityID == other.EntityID
    case EnumValue:
        return t.EnumVal == other.EnumVal
    case Money:
        return t.Amount == other.Amount &&
               t.Currency == other.Currency
    case GeoCoord:
        return t.Latitude == other.Latitude &&
               t.Longitude == other.Longitude
    case Point2D:
        return t.X == other.X && t.Y == other.Y
    default:
        return false
    }
}
```

### 2.5 Term Display

```go
func (t *Term) String() string {
    switch t.Type {
    case Atom:
        return t.AtomVal
    case Variable:
        return "?" + t.VariableVal
    case Number:
        if t.NumberVal == float64(int64(t.NumberVal)) {
            return fmt.Sprintf("%d", int64(t.NumberVal))
        }
        return fmt.Sprintf("%.4f", t.NumberVal)
    case Boolean:
        if t.BoolVal { return "true" }
        return "false"
    case Null:
        return "null"
    case DateTime:
        return t.TimeVal.Format(time.RFC3339)
    case EntityRef:
        return fmt.Sprintf("%s#%d", t.EntityType, t.EntityID)
    case EnumValue:
        return t.EnumVal
    case Money:
        return fmt.Sprintf("%s %.2f", t.Currency, t.Amount)
    default:
        return fmt.Sprintf("<%s>", t.Type)
    }
}
```

The Term is a universal container. Every value the system produces or consumes fits in one struct. The type tag determines which fields are live. The engine never inspects fields that don't match the tag.

---

## 3. Facts and the Knowledge Base

### 3.1 Facts

A fact is a predicate name plus an argument list of terms.

```go
type Fact struct {
    Predicate string
    Args      []Term
}

func NewFact(predicate string, args ...Term) Fact {
    return Fact{Predicate: predicate, Args: args}
}
```

Every OpsDB entity row becomes a set of facts — one per field — with the DR path as the predicate and the field value as the argument.

A booking entity with ID 42:

```json
{
  "id": 42,
  "status": "confirmed",
  "start_time": "2026-06-15T10:00:00Z",
  "resource_id": 7,
  "customer_id": 15,
  "total_price": 150.00
}
```

Becomes these facts:

```
booking.42.status("confirmed")
booking.42.start_time("2026-06-15T10:00:00Z")
booking.42.resource_id(7)
booking.42.customer_id(15)
booking.42.total_price(150.00)
```

Each fact has a predicate that is the full DR path with the entity ID resolved, and one argument that is the field value as a Term.

### 3.2 Entity to Facts Conversion

```go
func EntityToFacts(entityType string, id int64,
                   fields map[string]Term) []Fact {
    facts := make([]Fact, 0, len(fields))
    for fieldName, value := range fields {
        predicate := fmt.Sprintf("%s.%d.%s", entityType, id, fieldName)
        facts = append(facts, NewFact(predicate, value))
    }
    return facts
}
```

### 3.3 The Knowledge Base

The knowledge base stores facts indexed by predicate prefix for fast lookup. The index key is the entity type — so `booking.*` lookups scan only booking facts, not every fact in the system.

```go
type KnowledgeBase struct {
    // Facts indexed by entity type for fast lookup
    FactsByType map[string][]Fact

    // All facts in a flat slice for full scans
    AllFacts []Fact

    // Rules indexed by head predicate
    Rules map[string][]Rule

    // Schema metadata — valid entity types and their fields
    Schema *SchemaMetadata

    // Authorization context for the current caller
    AuthContext *AuthContext
}

func NewKnowledgeBase(schema *SchemaMetadata) *KnowledgeBase {
    return &KnowledgeBase{
        FactsByType: make(map[string][]Fact),
        AllFacts:    make([]Fact, 0),
        Rules:       make(map[string][]Rule),
        Schema:      schema,
    }
}

func (kb *KnowledgeBase) AddFact(f Fact) {
    kb.AllFacts = append(kb.AllFacts, f)

    // Extract entity type from predicate for indexing
    entityType := extractEntityType(f.Predicate)
    if entityType != "" {
        kb.FactsByType[entityType] = append(
            kb.FactsByType[entityType], f)
    }
}

func (kb *KnowledgeBase) AddRule(r Rule) {
    kb.Rules[r.Head] = append(kb.Rules[r.Head], r)
}

func extractEntityType(predicate string) string {
    dot := strings.IndexByte(predicate, '.')
    if dot < 0 {
        return predicate
    }
    return predicate[:dot]
}
```

### 3.4 Loading from OpsDB

```go
func LoadFromAPI(kb *KnowledgeBase, api *OpsDBClient,
                 entityType string) error {
    results, err := api.Search(entityType, SearchParams{})
    if err != nil {
        return err
    }

    schema := kb.Schema.GetEntity(entityType)
    if schema == nil {
        return fmt.Errorf("unknown entity type: %s", entityType)
    }

    for _, row := range results.Rows {
        id := row["id"].(int64)
        fields := make(map[string]Term)

        for _, field := range schema.Fields {
            val, ok := row[field.Name]
            if !ok || val == nil {
                fields[field.Name] = NullTerm()
                continue
            }
            fields[field.Name] = ValueToTerm(val, field.Type)
        }

        for _, fact := range EntityToFacts(entityType, id, fields) {
            kb.AddFact(fact)
        }
    }
    return nil
}

func ValueToTerm(val interface{}, fieldType string) Term {
    switch fieldType {
    case "int":
        return NumTerm(float64(val.(int64)))
    case "float":
        return NumTerm(val.(float64))
    case "varchar", "text":
        return AtomTerm(val.(string))
    case "boolean":
        return BoolTerm(val.(bool))
    case "datetime":
        return TimeTerm(val.(time.Time))
    case "date":
        return Term{Type: Date, DateVal: val.(time.Time)}
    case "enum":
        return AtomTerm(val.(string))
    case "foreign_key":
        return NumTerm(float64(val.(int64)))
    case "json":
        return Term{Type: JSONPayload,
                    JSONVal: jsonToTermMap(val)}
    default:
        return AtomTerm(fmt.Sprintf("%v", val))
    }
}
```

The schema metadata validates the loading. Only fields declared in the schema produce facts. A field not in the schema is not loaded — the closed vocabulary prevents unknown predicates from entering the knowledge base.

---

## 4. DR Path Resolution

A DR path is a dot-separated sequence of segments that walks the schema's entity graph. Each segment is either a literal (matches exactly) or a variable (generates choice points).

### 4.1 Path Segments

```go
type SegmentType int32

const (
    LiteralSegment  SegmentType = iota  // "booking", "status"
    VariableSegment                      // B, RID, WID
    NumberSegment                        // 42 — literal entity ID
)

type PathSegment struct {
    Type     SegmentType
    Literal  string
    Variable string
    Number   int64
}
```

### 4.2 Parsing a DR Path

The path `booking.B.status(S)` parses into:

| Position | Segment | Type | Meaning |
|----------|---------|------|---------|
| 0 | booking | Literal | Entity type |
| 1 | B | Variable | Entity ID — generates choice points |
| 2 | status | Literal | Field name |
| arg | S | Variable | Field value — the binding target |

```go
type DRPath struct {
    Segments []PathSegment
    Arg      Term    // The term in parentheses at the end
}

func ParseDRPath(raw string) (DRPath, error) {
    // Split "booking.B.status(S)" into path and arg
    parenIdx := strings.IndexByte(raw, '(')
    if parenIdx < 0 {
        return DRPath{}, fmt.Errorf("missing argument: %s", raw)
    }

    pathPart := raw[:parenIdx]
    argPart := raw[parenIdx+1 : len(raw)-1] // strip parens

    segments := strings.Split(pathPart, ".")
    parsed := make([]PathSegment, len(segments))

    for i, seg := range segments {
        if isVariable(seg) {
            parsed[i] = PathSegment{
                Type: VariableSegment, Variable: seg}
        } else if isNumber(seg) {
            n, _ := strconv.ParseInt(seg, 10, 64)
            parsed[i] = PathSegment{
                Type: NumberSegment, Number: n}
        } else {
            parsed[i] = PathSegment{
                Type: LiteralSegment, Literal: seg}
        }
    }

    arg := parseTerm(argPart)

    return DRPath{Segments: parsed, Arg: arg}, nil
}

func isVariable(s string) bool {
    if len(s) == 0 { return false }
    return s[0] >= 'A' && s[0] <= 'Z'
}

func isNumber(s string) bool {
    _, err := strconv.ParseInt(s, 10, 64)
    return err == nil
}
```

### 4.3 Schema Validation

Before execution, the path is validated against schema metadata. Segment 0 must be a valid entity type. Segment 2 (the field) must be a valid field on that entity type. If either fails, the query is rejected at parse time.

```go
func (kb *KnowledgeBase) ValidatePath(path DRPath) error {
    if len(path.Segments) < 3 {
        return fmt.Errorf("path too short: need entity.id.field")
    }

    // Segment 0 must be entity type
    entitySeg := path.Segments[0]
    if entitySeg.Type != LiteralSegment {
        return fmt.Errorf("first segment must be entity type")
    }

    entity := kb.Schema.GetEntity(entitySeg.Literal)
    if entity == nil {
        return fmt.Errorf("unknown entity type: %s",
                          entitySeg.Literal)
    }

    // Segment 2 must be a field on that entity type
    fieldSeg := path.Segments[2]
    if fieldSeg.Type == LiteralSegment {
        if !entity.HasField(fieldSeg.Literal) {
            return fmt.Errorf("unknown field %s on entity %s",
                              fieldSeg.Literal, entitySeg.Literal)
        }
    }

    return nil
}
```

A typo — `booking.B.statis(S)` — fails here, not at runtime. The closed vocabulary prevents invalid queries structurally.

### 4.4 Resolving a Path to Fact Lookups

Resolution converts a DR path into concrete fact lookups against the knowledge base. Variable segments generate choice points — the engine iterates over all matching values.

```go
type FactLookup struct {
    EntityType string
    EntityID   int64      // -1 if variable (iterate all)
    IDVar      string     // variable name for entity ID
    FieldName  string
    Arg        Term       // what to unify the field value against
}

func PathToLookup(path DRPath) FactLookup {
    lookup := FactLookup{
        EntityType: path.Segments[0].Literal,
        EntityID:   -1,
        Arg:        path.Arg,
    }

    // Segment 1 — entity ID
    idSeg := path.Segments[1]
    switch idSeg.Type {
    case NumberSegment:
        lookup.EntityID = idSeg.Number
    case VariableSegment:
        lookup.IDVar = idSeg.Variable
    }

    // Segment 2 — field name
    fieldSeg := path.Segments[2]
    if fieldSeg.Type == LiteralSegment {
        lookup.FieldName = fieldSeg.Literal
    }

    return lookup
}
```

When `EntityID` is -1 and `IDVar` is set, the engine iterates over all entity IDs of that type. Each ID is a choice point. The variable gets bound to the ID, and backtracking tries the next ID if a subsequent goal fails.

### 4.5 Deep Paths

Some schemas have nested structure — JSON payloads with discriminated types, or multi-level field access. A path like `booking.B.data_booking.payment_info.method(M)` walks into a JSON payload. Segments beyond position 2 navigate the JSON structure:

```go
func ResolveDeepPath(path DRPath, entity map[string]Term) (Term, bool) {
    // Start at the field (segment 2)
    current := entity[path.Segments[2].Literal]

    // Walk remaining segments into nested structure
    for i := 3; i < len(path.Segments); i++ {
        seg := path.Segments[i]
        if current.Type != Map && current.Type != JSONPayload {
            return Term{}, false
        }

        nested, ok := current.MapVal[seg.Literal]
        if !ok {
            if seg.Type == VariableSegment {
                // Variable segment in JSON — iterate keys
                // handled by backtracking in the solver
                return Term{}, false
            }
            return Term{}, false
        }
        current = nested
    }

    return current, true
}
```

---

## 5. Unification

Unification is the core operation. Given two terms and a current set of variable bindings, unification either fails or produces updated bindings that make the terms identical.

### 5.1 The Bindings Map

![Fig. 2: Variable Binding Accumulation — the binding map grows from empty to three variables as each goal adds its binding through unification.](./figures/proql_02_binding_flow.png)

```go
type Bindings map[string]Term

func NewBindings() Bindings {
    return make(Bindings)
}

func (b Bindings) Clone() Bindings {
    clone := make(Bindings, len(b))
    for k, v := range b {
        clone[k] = v
    }
    return clone
}

// Resolve follows variable chains to find the concrete value
func (b Bindings) Resolve(t Term) Term {
    for t.Type == Variable {
        bound, ok := b[t.VariableVal]
        if !ok {
            return t // still unbound
        }
        t = bound
    }
    return t
}
```

### 5.2 The Unify Function

```go
func Unify(a, b Term, bindings Bindings) (Bindings, bool) {
    a = bindings.Resolve(a)
    b = bindings.Resolve(b)

    // Two unbound variables — bind one to the other
    if a.Type == Variable && b.Type == Variable {
        result := bindings.Clone()
        result[a.VariableVal] = b
        return result, true
    }

    // Variable unifies with anything — bind it
    if a.Type == Variable {
        result := bindings.Clone()
        result[a.VariableVal] = b
        return result, true
    }
    if b.Type == Variable {
        result := bindings.Clone()
        result[b.VariableVal] = a
        return result, true
    }

    // Null unifies with Null
    if a.Type == Null && b.Type == Null {
        return bindings, true
    }

    // Same type — compare values
    if a.Type == b.Type {
        if a.Equals(&b) {
            return bindings, true
        }
        return nil, false
    }

    // Number-to-Number even if subtypes differ
    if isNumeric(a.Type) && isNumeric(b.Type) {
        if toFloat(a) == toFloat(b) {
            return bindings, true
        }
        return nil, false
    }

    // Atom matches EnumValue by string
    if (a.Type == Atom && b.Type == EnumValue) ||
       (a.Type == EnumValue && b.Type == Atom) {
        aStr := atomString(a)
        bStr := atomString(b)
        if aStr == bStr {
            return bindings, true
        }
        return nil, false
    }

    return nil, false
}

func isNumeric(t TermType) bool {
    return t == Number || t == Money || t == Percentage
}

func toFloat(t Term) float64 {
    switch t.Type {
    case Number:
        return t.NumberVal
    case Money:
        return t.Amount
    case Percentage:
        return t.Pct
    default:
        return 0
    }
}

func atomString(t Term) string {
    if t.Type == Atom { return t.AtomVal }
    if t.Type == EnumValue { return t.EnumVal }
    return ""
}
```

No occur check is needed. The fat struct is flat — terms don't nest arbitrarily. The only pointer fields are `OldValue` and `NewValue` in change management terms, and those are never unification targets.

### 5.3 Unification Walkthrough

Query: `booking.B.status("confirmed")`

The engine finds fact `booking.42.status("confirmed")`.

Step 1: Unify entity ID. B is a variable. 42 is a number. Bind B → 42.
Step 2: Unify field value. "confirmed" (from query, Atom) with "confirmed" (from fact, Atom or EnumValue). Equal. Succeed.

Result bindings: `{B: 42}`.

The engine also finds fact `booking.43.status("pending")`.

Step 1: Unify entity ID. B is a variable. Bind B → 43.
Step 2: Unify "confirmed" with "pending". Not equal. Fail.

This binding is discarded. Backtracking tries the next fact.

---

## 6. Backtracking

![Fig. 1: Backtracking Tree — three choice points branching through three goals, with pruning and two result leaves.](./figures/proql_01_backtracking_tree.png)

Backtracking explores all possible variable bindings. A goal may match multiple facts. The engine tries each one. If a subsequent goal fails with a given binding, the engine returns to the last choice point and tries the next alternative.

### 6.1 The Goal

```go
type GoalType int32

const (
    DRPathGoal  GoalType = iota  // DR path resolution
    BuiltinGoal                   // <, >, member, \+, etc.
    RuleGoal                      // User-defined rule call
)

type Goal struct {
    Type     GoalType
    Path     DRPath       // for DRPathGoal
    Builtin  string       // for BuiltinGoal: "<", "member", etc.
    Args     []Term       // for BuiltinGoal and RuleGoal
    RuleName string       // for RuleGoal
    Negated  bool         // \+ prefix
}
```

### 6.2 The Solver

```go
type QueryBounds struct {
    MaxBacktracks int64
    MaxResults    int
    MaxTimeMs     int64
}

type SolveState struct {
    Bounds     QueryBounds
    Backtracks int64
    StartTime  time.Time
    KB         *KnowledgeBase
}

func Solve(state *SolveState, goals []Goal,
           bindings Bindings) []Bindings {

    // Bounds check
    if state.Backtracks > state.Bounds.MaxBacktracks {
        return nil
    }
    elapsed := time.Since(state.StartTime).Milliseconds()
    if elapsed > state.Bounds.MaxTimeMs {
        return nil
    }

    // All goals satisfied — this binding set is a result
    if len(goals) == 0 {
        return []Bindings{bindings.Clone()}
    }

    goal := goals[0]
    remaining := goals[1:]
    var results []Bindings

    switch goal.Type {

    case DRPathGoal:
        candidates := state.KB.FindMatchingFacts(goal.Path, bindings)
        for _, candidate := range candidates {
            state.Backtracks++
            newBindings, ok := UnifyFact(goal.Path, candidate,
                                         bindings)
            if ok {
                sub := Solve(state, remaining, newBindings)
                results = append(results, sub...)
                if len(results) >= state.Bounds.MaxResults {
                    return results
                }
            }
        }

    case BuiltinGoal:
        newBindings, ok := EvalBuiltin(goal.Builtin, goal.Args,
                                        bindings)
        if goal.Negated {
            ok = !ok
        }
        if ok {
            results = Solve(state, remaining, newBindings)
        }

    case RuleGoal:
        rules := state.KB.Rules[goal.RuleName]
        for _, rule := range rules {
            state.Backtracks++
            expanded, ok := ExpandRule(rule, goal.Args, bindings)
            if ok {
                // Prepend rule body goals to remaining goals
                combined := append(expanded, remaining...)
                sub := Solve(state, combined, bindings)
                results = append(results, sub...)
            }
        }
    }

    return results
}
```

### 6.3 Finding Matching Facts

```go
func (kb *KnowledgeBase) FindMatchingFacts(path DRPath,
    bindings Bindings) []Fact {

    entityType := path.Segments[0].Literal
    allFacts := kb.FactsByType[entityType]
    if allFacts == nil {
        return nil
    }

    idSeg := path.Segments[1]
    fieldSeg := path.Segments[2]

    var candidates []Fact

    for _, fact := range allFacts {
        // Parse the fact's predicate to extract entity ID and field
        parts := strings.SplitN(fact.Predicate, ".", 3)
        if len(parts) < 3 {
            continue
        }

        factID, err := strconv.ParseInt(parts[1], 10, 64)
        if err != nil {
            continue
        }
        factField := parts[2]

        // Check field name matches
        if fieldSeg.Type == LiteralSegment &&
           factField != fieldSeg.Literal {
            continue
        }

        // Check entity ID matches (if bound)
        if idSeg.Type == NumberSegment && factID != idSeg.Number {
            continue
        }
        if idSeg.Type == VariableSegment {
            bound := bindings.Resolve(
                VarTerm(idSeg.Variable))
            if bound.Type == Number &&
               int64(bound.NumberVal) != factID {
                continue
            }
        }

        // Authorization check
        if !kb.AuthContext.CanAccess(entityType, factID,
                                     factField) {
            continue
        }

        candidates = append(candidates, fact)
    }

    return candidates
}
```

### 6.4 Unifying a Fact Against a Path

```go
func UnifyFact(path DRPath, fact Fact,
               bindings Bindings) (Bindings, bool) {

    current := bindings.Clone()

    // Extract entity ID from fact predicate
    parts := strings.SplitN(fact.Predicate, ".", 3)
    factID, _ := strconv.ParseInt(parts[1], 10, 64)

    // Unify entity ID with path segment
    idSeg := path.Segments[1]
    if idSeg.Type == VariableSegment {
        var ok bool
        current, ok = Unify(
            VarTerm(idSeg.Variable),
            NumTerm(float64(factID)),
            current)
        if !ok {
            return nil, false
        }
    }

    // Unify field value with path argument
    if len(fact.Args) > 0 {
        var ok bool
        current, ok = Unify(path.Arg, fact.Args[0], current)
        if !ok {
            return nil, false
        }
    }

    return current, true
}
```

### 6.5 Backtracking Walkthrough

Query:
```prolog
?- booking.B.status("confirmed"),
   booking.B.resource_id(RID),
   resource.RID.name(RName).
```

Knowledge base has three bookings:
- booking 42: status="confirmed", resource_id=7
- booking 43: status="pending", resource_id=7
- booking 44: status="confirmed", resource_id=12

And two resources:
- resource 7: name="Conference Room A"
- resource 12: name="Parking Spot 3"

**Goal 1:** `booking.B.status("confirmed")`

Three choice points (one per booking). Try booking 42:
- Bind B → 42. Unify "confirmed" with "confirmed". Succeed.
- Continue to goal 2 with `{B: 42}`.

**Goal 2:** `booking.B.resource_id(RID)` with B=42

B is bound to 42. Look up `booking.42.resource_id`. Find 7.
- Bind RID → 7. Succeed.
- Continue to goal 3 with `{B: 42, RID: 7}`.

**Goal 3:** `resource.RID.name(RName)` with RID=7

RID is bound to 7. Look up `resource.7.name`. Find "Conference Room A".
- Bind RName → "Conference Room A". Succeed.
- All goals satisfied. Result: `{B: 42, RID: 7, RName: "Conference Room A"}`.

Back to goal 1, try booking 43:
- Bind B → 43. Unify "confirmed" with "pending". Fail.
- Backtrack. Try booking 44.

Try booking 44:
- Bind B → 44. Unify "confirmed" with "confirmed". Succeed.
- Goal 2 with B=44: resource_id=12, bind RID → 12.
- Goal 3 with RID=12: name="Parking Spot 3", bind RName → "Parking Spot 3".
- Result: `{B: 44, RID: 12, RName: "Parking Spot 3"}`.

Final results: two binding sets. Booking 43 was filtered out by status. The join happened through variable sharing. No join declaration was needed.

---

## 7. Comparison and Built-in Predicates

Built-in predicates don't resolve against the knowledge base. They evaluate their arguments directly. Each is a Go function with the same signature.

### 7.1 Built-in Evaluation

```go
type BuiltinFunc func(args []Term, bindings Bindings) (Bindings, bool)

var Builtins = map[string]BuiltinFunc{
    "<":       builtinLT,
    ">":       builtinGT,
    "<=":      builtinLTE,
    ">=":      builtinGTE,
    "==":      builtinEQ,
    "\\=":     builtinNEQ,
    "is":      builtinIs,
    "member":  builtinMember,
    "between": builtinBetween,
    "var":     builtinVar,
    "nonvar":  builtinNonvar,
    "prefix":  builtinPrefix,
    "count":   builtinCount,
    "sum":     builtinSum,
    "min":     builtinMin,
    "max":     builtinMax,
    "avg":     builtinAvg,
}
```

### 7.2 Comparison

```go
func builtinLT(args []Term, b Bindings) (Bindings, bool) {
    if len(args) != 2 { return b, false }
    left := b.Resolve(args[0])
    right := b.Resolve(args[1])
    if !isNumeric(left.Type) || !isNumeric(right.Type) {
        // DateTime comparison
        if left.Type == DateTime && right.Type == DateTime {
            return b, left.TimeVal.Before(right.TimeVal)
        }
        return b, false
    }
    return b, toFloat(left) < toFloat(right)
}

func builtinGT(args []Term, b Bindings) (Bindings, bool) {
    if len(args) != 2 { return b, false }
    left := b.Resolve(args[0])
    right := b.Resolve(args[1])
    if !isNumeric(left.Type) || !isNumeric(right.Type) {
        if left.Type == DateTime && right.Type == DateTime {
            return b, left.TimeVal.After(right.TimeVal)
        }
        return b, false
    }
    return b, toFloat(left) > toFloat(right)
}

// LTE, GTE, EQ, NEQ follow the same pattern
```

### 7.3 Set Membership

```go
func builtinMember(args []Term, b Bindings) (Bindings, bool) {
    if len(args) != 2 { return b, false }
    element := b.Resolve(args[0])
    list := b.Resolve(args[1])

    if list.Type != List { return b, false }

    // If element is a variable, this should generate
    // choice points — but for built-in simplicity,
    // we require element to be bound
    if element.Type == Variable { return b, false }

    for _, item := range list.ListVal {
        resolved := b.Resolve(item)
        if element.Equals(&resolved) {
            return b, true
        }
    }
    return b, false
}
```

### 7.4 Arithmetic

```go
func builtinIs(args []Term, b Bindings) (Bindings, bool) {
    if len(args) != 2 { return b, false }
    target := args[0]  // variable to bind
    expr := args[1]    // expression to evaluate

    result, ok := evalArith(expr, b)
    if !ok { return b, false }

    return Unify(target, NumTerm(result), b)
}

func evalArith(t Term, b Bindings) (float64, bool) {
    t = b.Resolve(t)

    if isNumeric(t.Type) {
        return toFloat(t), true
    }

    // For compound arithmetic, parse from the term's atom
    // representation: "A / B", "A * B + C"
    // Minimal implementation handles binary operations
    if t.Type == Atom {
        return evalArithExpr(t.AtomVal, b)
    }

    return 0, false
}
```

### 7.5 Aggregation

```go
func builtinCount(args []Term, b Bindings) (Bindings, bool) {
    // count(Var, GoalExpr, Result)
    // Count how many bindings of Var satisfy GoalExpr
    if len(args) != 3 { return b, false }

    // This requires access to the solver — aggregation
    // built-ins receive a reference to the solve state
    // Implementation deferred to the solver integration
    return b, false
}
```

Aggregation built-ins (`count`, `sum`, `min`, `max`, `avg`) need access to the solver because they run a sub-query internally, collect the results, and compute the aggregate. The implementation wraps the Solve function:

```go
func solveCount(state *SolveState, countVar string,
                subGoals []Goal, bindings Bindings) (int, bool) {
    results := Solve(state, subGoals, bindings)
    seen := make(map[string]bool)
    for _, result := range results {
        val := result.Resolve(VarTerm(countVar))
        key := val.String()
        seen[key] = true
    }
    return len(seen), true
}
```

### 7.6 Negation as Failure

```go
// \+ Goal succeeds if Goal fails (has no solutions)
// Handled in the solver: when goal.Negated is true,
// the solver inverts the result of evaluation
```

The solver already handles this — when `goal.Negated` is true, success becomes failure and failure becomes success. The bindings from the negated goal are discarded; only the current bindings continue.

---

## 8. The @Directives

@sort, @limit, @offset, @distinct are not logic operations. They are result-set transformations applied after all backtracking completes.

### 8.1 Directive Types

```go
type DirectiveType int32

const (
    SortDirective     DirectiveType = iota
    LimitDirective
    OffsetDirective
    DistinctDirective
)

type SortOrder int32

const (
    Ascending  SortOrder = iota
    Descending
)

type Directive struct {
    Type     DirectiveType
    Variable string     // for sort and distinct
    Order    SortOrder   // for sort
    Count    int         // for limit and offset
}
```

### 8.2 Parsing Directives

```go
func ParseDirectives(raw string) []Directive {
    var directives []Directive
    // Find @tokens in the query string
    // @sort(Var, asc)  @sort(Var, desc)
    // @limit(N)  @offset(N)  @distinct  @distinct(Var)

    tokens := extractDirectiveTokens(raw)
    for _, tok := range tokens {
        switch {
        case strings.HasPrefix(tok, "@sort"):
            v, order := parseSortArgs(tok)
            directives = append(directives, Directive{
                Type: SortDirective, Variable: v, Order: order})
        case strings.HasPrefix(tok, "@limit"):
            n := parseIntArg(tok)
            directives = append(directives, Directive{
                Type: LimitDirective, Count: n})
        case strings.HasPrefix(tok, "@offset"):
            n := parseIntArg(tok)
            directives = append(directives, Directive{
                Type: OffsetDirective, Count: n})
        case strings.HasPrefix(tok, "@distinct"):
            v := parseOptionalVarArg(tok)
            directives = append(directives, Directive{
                Type: DistinctDirective, Variable: v})
        }
    }
    return directives
}
```

### 8.3 Applying Directives

The pipeline: solve → collect bindings → @distinct → @sort → @offset → @limit → return.

```go
func ApplyDirectives(results []Bindings,
                     directives []Directive) []Bindings {
    current := results

    for _, d := range directives {
        switch d.Type {

        case DistinctDirective:
            current = applyDistinct(current, d.Variable)

        case SortDirective:
            current = applySort(current, d.Variable, d.Order)

        case OffsetDirective:
            if d.Count < len(current) {
                current = current[d.Count:]
            } else {
                current = nil
            }

        case LimitDirective:
            if d.Count < len(current) {
                current = current[:d.Count]
            }
        }
    }

    return current
}

func applyDistinct(results []Bindings, varName string) []Bindings {
    seen := make(map[string]bool)
    var unique []Bindings
    for _, b := range results {
        var key string
        if varName != "" {
            key = b.Resolve(VarTerm(varName)).String()
        } else {
            // Distinct on all variables
            key = bindingsKey(b)
        }
        if !seen[key] {
            seen[key] = true
            unique = append(unique, b)
        }
    }
    return unique
}

func applySort(results []Bindings, varName string,
               order SortOrder) []Bindings {
    sort.SliceStable(results, func(i, j int) bool {
        a := results[i].Resolve(VarTerm(varName))
        b := results[j].Resolve(VarTerm(varName))

        cmp := compareTerm(a, b)
        if order == Descending {
            return cmp > 0
        }
        return cmp < 0
    })
    return results
}

func compareTerm(a, b Term) int {
    if isNumeric(a.Type) && isNumeric(b.Type) {
        fa, fb := toFloat(a), toFloat(b)
        if fa < fb { return -1 }
        if fa > fb { return 1 }
        return 0
    }
    if a.Type == DateTime && b.Type == DateTime {
        if a.TimeVal.Before(b.TimeVal) { return -1 }
        if a.TimeVal.After(b.TimeVal) { return 1 }
        return 0
    }
    // String comparison for atoms/enums
    sa := a.String()
    sb := b.String()
    if sa < sb { return -1 }
    if sa > sb { return 1 }
    return 0
}
```

---

## 9. Rules

![Fig. 8: Recursive Ancestor Traversal — two rules traverse a location hierarchy, each recursion level shown with its clause and accumulated results.](./figures/proql_08_recursive_ancestor.png)

A rule is a named query that can be called from other queries. Rules enable composition and recursion.

### 9.1 Rule Structure

```go
type Rule struct {
    Head     string   // predicate name
    HeadArgs []Term   // argument pattern for matching
    Body     []Goal   // goals to satisfy
}
```

### 9.2 Defining Rules

A booking conflict rule:

```prolog
conflict(B1, B2) :-
    booking.B1.resource_id(RID),
    booking.B2.resource_id(RID),
    B1 \= B2,
    booking.B1.start_time(S1),
    booking.B1.end_time(E1),
    booking.B2.start_time(S2),
    booking.B2.end_time(E2),
    <(S1, E2),
    >(E1, S2).
```

This defines: two bookings conflict if they share a resource, are different bookings, and their time ranges overlap. The shared variable `RID` is the resource join. `B1 \= B2` prevents self-matching. The four comparisons check overlap.

Use it in a query:

```prolog
?- conflict(B1, B2),
   booking.B1.customer_id(CID),
   customer.CID.name(CName).
```

Find all conflicting booking pairs and get the customer name for the first booking.

### 9.3 Rule Expansion

When the solver encounters a rule goal, it expands the rule body with the head variables unified against the call arguments:

```go
func ExpandRule(rule Rule, callArgs []Term,
                bindings Bindings) ([]Goal, bool) {

    // Create fresh variable names to avoid capture
    suffix := fmt.Sprintf("_%d", freshCounter)
    freshCounter++

    // Unify call arguments with head arguments
    current := bindings.Clone()
    for i, headArg := range rule.HeadArgs {
        if i >= len(callArgs) { break }

        renamed := renameVars(headArg, suffix)
        var ok bool
        current, ok = Unify(callArgs[i], renamed, current)
        if !ok {
            return nil, false
        }
    }

    // Rename variables in body goals to avoid capture
    expanded := make([]Goal, len(rule.Body))
    for i, goal := range rule.Body {
        expanded[i] = renameGoalVars(goal, suffix)
    }

    return expanded, true
}

var freshCounter int64
```

Variable renaming prevents capture — each rule invocation gets unique variable names so recursive calls don't accidentally share bindings.

### 9.4 Recursion

The location hierarchy traversal:

```prolog
ancestor(X, Y) :- location.X.parent_location_id(Y).
ancestor(X, Y) :- location.X.parent_location_id(Z), ancestor(Z, Y).
```

Two rules with the same head. The solver tries both. The first handles the direct parent case. The second recurses through intermediate ancestors. The variable Z is fresh on each expansion, so the recursion terminates when there are no more parents.

The bounds system prevents infinite recursion. If `MaxBacktracks` is exceeded, the solver halts and returns what it has so far.

Query: `ancestor(42, A)` — find all ancestors of location 42.

The solver expands `ancestor(42, A)` against the first rule: look up `location.42.parent_location_id`. Say it finds 10. Bind A → 10. Result: {A: 10}.

Then expands against the second rule: look up `location.42.parent_location_id`, get Z=10, then recursively call `ancestor(10, A)`. That finds 10's parent (say 3), which finds 3's parent (say 1), which has no parent — recursion terminates. Results: {A: 10}, {A: 3}, {A: 1}.

---

## 10. Bidirectional Write-Back

![Fig. 3: Bidirectional Edit Cycle — query to results to edit to change set through the gate pipeline to requery, completing the governed loop.](./figures/proql_03_edit_cycle.png)

Query results are editable. Each binding set carries enough information to construct a change set when a value is modified.

### 10.1 Tracking Origin

Each binding needs to record where its value came from — which entity type, which entity ID, which field. The solver annotates bindings during fact unification:

```go
type BindingOrigin struct {
    EntityType string
    EntityID   int64
    FieldName  string
}

type AnnotatedBindings struct {
    Values  Bindings
    Origins map[string]BindingOrigin  // variable name → origin
}
```

When the solver unifies a path argument with a fact value, it records the origin:

```go
func UnifyFactWithOrigin(path DRPath, fact Fact,
    bindings Bindings,
    origins map[string]BindingOrigin) (Bindings, map[string]BindingOrigin, bool) {

    newBindings, ok := UnifyFact(path, fact, bindings)
    if !ok {
        return nil, nil, false
    }

    newOrigins := cloneOrigins(origins)

    // Record origin for the argument variable
    if path.Arg.Type == Variable {
        parts := strings.SplitN(fact.Predicate, ".", 3)
        entityID, _ := strconv.ParseInt(parts[1], 10, 64)

        newOrigins[path.Arg.VariableVal] = BindingOrigin{
            EntityType: parts[0],
            EntityID:   entityID,
            FieldName:  parts[2],
        }
    }

    return newBindings, newOrigins, true
}
```

### 10.2 Constructing a Change Set from Edits

```go
type FieldChangeRequest struct {
    EntityType string
    EntityID   int64
    FieldName  string
    OldValue   Term
    NewValue   Term
}

func DiffBindings(original, modified AnnotatedBindings) []FieldChangeRequest {
    var changes []FieldChangeRequest

    for varName, newVal := range modified.Values {
        oldVal, exists := original.Values[varName]
        if !exists { continue }

        // Skip if unchanged
        if oldVal.Equals(&newVal) { continue }

        // Look up origin
        origin, hasOrigin := original.Origins[varName]
        if !hasOrigin { continue }

        changes = append(changes, FieldChangeRequest{
            EntityType: origin.EntityType,
            EntityID:   origin.EntityID,
            FieldName:  origin.FieldName,
            OldValue:   oldVal,
            NewValue:   newVal,
        })
    }

    return changes
}
```

### 10.3 Submitting Through the Gate

```go
func WriteBack(api *OpsDBClient, changes []FieldChangeRequest,
               reason string) (*ChangeSetResult, error) {

    fieldChanges := make([]APIFieldChange, len(changes))
    for i, ch := range changes {
        fieldChanges[i] = APIFieldChange{
            EntityType: ch.EntityType,
            EntityID:   ch.EntityID,
            FieldName:  ch.FieldName,
            OldValue:   ch.OldValue.String(),
            NewValue:   ch.NewValue.String(),
        }
    }

    return api.SubmitChangeSet(ChangeSetRequest{
        FieldChanges: fieldChanges,
        Reason:       reason,
    })
}
```

The change set goes through the full gate pipeline. Step 3 validates the new value against the schema — if the field is an enum, the new value must be in the declared set. Step 2 checks authorization — the caller must have write permission. Step 6 creates a version row. Step 8 writes the audit entry. The write-back is fully governed.

### 10.4 The Edit-Save-Requery Cycle

```go
func EditCycle(state *SolveState, query string,
               api *OpsDBClient) (*EditSession, error) {

    // Parse and solve
    goals, directives := ParseQuery(query)
    rawResults := Solve(state, goals, NewBindings())
    results := ApplyDirectives(rawResults, directives)

    // Annotate with origins
    annotated := annotateResults(results)

    return &EditSession{
        Query:     query,
        Results:   annotated,
        API:       api,
        State:     state,
    }, nil
}

func (es *EditSession) ApplyEdit(rowIdx int, varName string,
    newValue Term, reason string) (*EditSession, error) {

    // Get original binding
    original := es.Results[rowIdx]

    // Create modified copy
    modified := original.Clone()
    modified.Values[varName] = newValue

    // Compute diff
    changes := DiffBindings(original, modified)
    if len(changes) == 0 {
        return es, nil
    }

    // Write through API gate
    result, err := WriteBack(es.API, changes, reason)
    if err != nil {
        return nil, err
    }

    // Re-load affected entities
    for _, ch := range changes {
        ReloadEntity(es.State.KB, es.API,
                     ch.EntityType, ch.EntityID)
    }

    // Re-query
    return EditCycle(es.State, es.Query, es.API)
}
```

### 10.5 HTMX Integration

The edit cycle maps directly to HTMX. Query results render as a table. Each cell is an edit target. Saving triggers the write-back and swaps fresh results:

```html
<table id="query-results">
  <tr>
    <th>Booking</th><th>Status</th><th>Resource</th>
  </tr>
  <tr>
    <td>${B}</td>
    <td>
      <select name="Status"
              hx-put="/proql/edit?row=0&var=Status&query=..."
              hx-target="#query-results"
              hx-swap="innerHTML">
        <option value="pending">Pending</option>
        <option value="confirmed" selected>Confirmed</option>
        <option value="cancelled">Cancelled</option>
      </select>
    </td>
    <td>${RName}</td>
  </tr>
</table>
```

The select options come from the enum set on the Term — the EnumValue term carries its valid set, so the HTMX template can render the dropdown without a separate schema lookup.

---

## 11. Authorization in the Query Engine

![Fig. 4: Authorization Silent Filtering — same query, two users, different visible facts, different result sets with no error.](./figures/proql_04_auth_filtering.png)

Every fact access during query resolution passes through authorization. The five layers evaluate for each entity and field the query touches. Unauthorized access doesn't error — the binding fails silently.

### 11.1 Authorization Check

```go
type AuthContext struct {
    CallerID     int64
    CallerType   string    // "user" or "service_account"
    Roles        []string
    Groups       []string
    Clearance    string    // access classification level
    RunnerScope  *RunnerScope  // nil for human callers
}

func (ac *AuthContext) CanAccess(entityType string,
    entityID int64, fieldName string) bool {

    // Layer 1: Role and group membership
    if !ac.hasRoleForEntityType(entityType) {
        return false
    }

    // Layer 2: Per-entity governance (_requires_group)
    requiredGroup := ac.getRequiredGroup(entityType, entityID)
    if requiredGroup != "" && !ac.isMemberOf(requiredGroup) {
        return false
    }

    // Layer 3: Per-field classification
    fieldClass := ac.getFieldClassification(entityType, fieldName)
    if !ac.clearanceCovers(fieldClass) {
        return false
    }

    // Layer 4: Runner scope (only for service accounts)
    if ac.RunnerScope != nil {
        if !ac.RunnerScope.canRead(entityType, fieldName) {
            return false
        }
    }

    // Layer 5: Policy rules (time-of-day, tenure, custom)
    if !ac.passesPolicyRules(entityType, entityID, fieldName) {
        return false
    }

    return true
}
```

### 11.2 Silent Filtering

The authorization check runs inside `FindMatchingFacts`. Facts the caller can't access are not returned. The query engine never sees them. The variable doesn't bind through unauthorized paths.

Two users run the same query:

```prolog
?- booking.B.customer_id(CID),
   customer.CID.name(CName),
   customer.CID.email(Email).
```

User A has clearance "internal" — sees all three fields. Gets results with B, CID, CName, and Email bound.

User B has clearance "public" — the `email` field is classified "internal." The authorization check at layer 3 fails for `email`. The `customer.CID.email(Email)` goal produces no matching facts. The entire binding fails because all goals must succeed. User B gets no results for customers whose email they can't see.

If the query is restructured to make email optional:

```prolog
?- booking.B.customer_id(CID),
   customer.CID.name(CName).
```

User B now gets results with B, CID, and CName — just not Email. The query adapts to what the caller can see without explicit handling in the query itself.

---

## 12. Operational Query Examples

### 12.1 Runners Near Execution Bounds

Find runners that consumed more than 90% of their execution time budget:

```prolog
?- runner_job.J.runner_spec_id(RID),
   runner_spec.RID.name(RName),
   runner_job.J.duration_seconds(Duration),
   runner_spec.RID.max_execution_seconds(MaxExec),
   is(Pct, Duration / MaxExec),
   >(Pct, 0.9),
   runner_job.J.started_time(StartTime)
   @sort(Pct, desc) @limit(20).
```

One query replaces: search runner_jobs, search runner_specs, join by runner_spec_id, compute percentage in application code, filter, sort. The division and comparison happen in the query. The join through `RID` is implicit.

### 12.2 Stale Change Sets

Find change sets pending approval for more than 24 hours with their affected entities:

```prolog
?- change_set.CS.status("pending_approval"),
   change_set.CS.submitted_time(SubTime),
   <(SubTime, CutoffTime),
   change_set.CS.proposed_by_user_id(ProposerID),
   ops_user.ProposerID.name(ProposerName),
   change_set_field_change.FC.change_set_id(CS),
   change_set_field_change.FC.entity_type(EType),
   change_set_field_change.FC.entity_id(EID)
   @sort(SubTime, asc).
```

The `CutoffTime` would be bound before the query executes — injected as a pre-bound variable set to 24 hours ago.

### 12.3 Drift Detection

Find entities where desired state differs from observed state:

```prolog
?- desired_config.E.field_name(Field),
   desired_config.E.value(Desired),
   observation_cache.E.field_name(Field),
   observation_cache.E.value(Observed),
   Desired \= Observed,
   desired_config.E.entity_type(EType),
   desired_config.E.entity_id(EID).
```

The shared variables `E` and `Field` join desired config to observation cache by entity and field. The `\=` filters to only the drifted values. A reconciler runner's entire get phase — which normally requires two search API calls plus application-code comparison — is one query.

### 12.4 Approval Chain Reconstruction

Find the complete provenance for a specific entity's current state:

```prolog
?- entity_version.V.entity_type("service"),
   entity_version.V.entity_id(42),
   entity_version.V.is_current(true),
   entity_version.V.change_set_id(CS),
   change_set.CS.proposed_by_user_id(ProposerID),
   ops_user.ProposerID.name(ProposerName),
   change_set.CS.reason(Reason),
   change_set_approval.A.change_set_id(CS),
   change_set_approval.A.approved_by_user_id(ApproverID),
   ops_user.ApproverID.name(ApproverName),
   change_set_approval.A.approved_time(ApprovedTime)
   @sort(ApprovedTime, asc).
```

Six entity types joined through shared variables: entity_version, change_set, ops_user (twice — proposer and approver), change_set_approval. The provenance chain from entity to version to change set to proposer and approvers is one query.

### 12.5 Emergency Change Audit

Find all emergency changes in a time window with their review status:

```prolog
?- change_set.CS.is_emergency(true),
   change_set.CS.submitted_time(SubTime),
   >(SubTime, WindowStart),
   <(SubTime, WindowEnd),
   change_set.CS.proposed_by_user_id(UID),
   ops_user.UID.name(UserName),
   emergency_review.ER.change_set_id(CS),
   emergency_review.ER.status(ReviewStatus),
   emergency_review.ER.review_deadline(Deadline)
   @sort(SubTime, desc).
```

### 12.6 Retention Policy Compliance

Find entities past their retention horizon that haven't been reaped:

```prolog
?- retention_policy.RP.entity_type(EType),
   retention_policy.RP.retention_days(Days),
   entity_version.V.entity_type(EType),
   entity_version.V.created_time(Created),
   age_days(Created, Age),
   >(Age, Days),
   entity_version.V.entity_id(EID),
   entity_version.V.is_current(false)
   @sort(Age, desc) @limit(100).
```

---

## 13. Application Query Examples

![Fig. 7: Multi-Entity Join Graph — five entity types connected by shared variables, each variable an implicit join with no declaration needed.](./figures/proql_07_variable_join_graph.png)

### 13.1 Booking Availability

Find whether a resource is available for a requested time slot — the negation pattern:

```prolog
available(RID, ReqStart, ReqEnd) :-
    resource.RID.is_bookable(true),
    \+ (booking.B.resource_id(RID),
        booking.B.status(S),
        member(S, [pending, confirmed]),
        booking.B.start_time(Start),
        booking.B.end_time(End),
        <(Start, ReqEnd),
        >(End, ReqStart)),
    \+ (blackout_date.BD.resource_id(RID),
        blackout_date.BD.blackout_date(D),
        >=(D, ReqStart),
        <=(D, ReqEnd)).

?- available(RID, "2026-06-15T10:00", "2026-06-15T12:00"),
   resource.RID.name(RName),
   resource.RID.hourly_rate(Rate).
```

The rule defines availability: the resource is bookable, no existing bookings overlap the requested window, and no blackout dates fall within it. The query finds all available resources with their names and rates.

### 13.2 Invoice with Full Detail

```prolog
?- invoice.I.customer_id(CID),
   customer.CID.name(CName),
   invoice.I.status("finalized"),
   invoice.I.invoice_number(InvNum),
   invoice.I.total_amount(Total),
   invoice_line_item.LI.invoice_id(I),
   invoice_line_item.LI.description(LineDesc),
   invoice_line_item.LI.amount(LineAmt),
   observation_cache_payment.P.invoice_id(I),
   observation_cache_payment.P.payment_status(PayStatus)
   @sort(I, desc) @limit(20).
```

Three entity types plus observation cache in one query. The invoice, its line items, the customer name, and the payment status from the external payment processor — all joined through shared variables.

### 13.3 Task Dependency Chain (Recursive)

```prolog
depends_on(T1, T2) :-
    task_dependency.D.task_id(T1),
    task_dependency.D.depends_on_task_id(T2).
depends_on(T1, T3) :-
    depends_on(T1, T2),
    depends_on(T2, T3).

?- depends_on(TaskID, Dep),
   task.Dep.title(Title),
   task.Dep.status(Status),
   task.Dep.assignee_id(AID),
   ops_user.AID.name(AssigneeName)
   @sort(Dep, asc).
```

`TaskID` is pre-bound. The recursive rule finds all transitive dependencies. The query gets the title, status, and assignee for each dependency. The equivalent SQL requires a recursive CTE. The equivalent search API requires multiple calls with application-code graph traversal.

### 13.4 Project Dashboard Aggregation

```prolog
?- project.P.name(PName),
   project.P.status("active"),
   count(T, (task.T.project_id(P), task.T.status("done")), DoneCount),
   count(T2, task.T2.project_id(P), TotalCount),
   is(Pct, DoneCount / TotalCount * 100)
   @sort(Pct, desc).
```

For each active project: count done tasks, count total tasks, compute completion percentage. One query produces the dashboard data. The search API would require one call per project plus application-code aggregation.

### 13.5 User Activity Across Entities

```prolog
?- audit_log.AL.caller_id(UID),
   audit_log.AL.action("update"),
   audit_log.AL.target_entity_type(EType),
   audit_log.AL.target_entity_id(EID),
   audit_log.AL.acted_time(ActionTime),
   >(ActionTime, SinceTime),
   ops_user.UID.name(UserName)
   @sort(ActionTime, desc) @limit(50).
```

Recent update activity across all entity types for all users. The audit log is queried through the same DR paths as any other entity. The join to ops_user through `UID` gets the username.

### 13.6 State Machine Validation Query

Check whether a proposed transition is valid:

```prolog
valid_transition(EType, FromState, ToState) :-
    state_transition_rule.R.entity_type(EType),
    state_transition_rule.R.from_state(FromState),
    state_transition_rule.R.to_state(ToState),
    state_transition_rule.R.is_allowed(true).

?- valid_transition("task", "in_progress", "done").
```

Returns true (the binding succeeds) or false (no matching facts). The state machine validation is a query, not application code.

### 13.7 Customer 360 View

Everything about a customer in one query:

```prolog
?- customer.C.id(CustID),
   customer.C.name(CName),
   customer.C.email(Email),
   booking.B.customer_id(CustID),
   booking.B.status(BookStatus),
   booking.B.start_time(BookTime),
   booking.B.resource_id(RID),
   resource.RID.name(ResourceName),
   observation_cache_payment.P.booking_id(B),
   observation_cache_payment.P.payment_status(PayStatus)
   @sort(BookTime, desc) @limit(50).
```

Customer details, their bookings, the resource names, and payment statuses — five entity types joined through shared variables. The authorization layers filter what the caller can see. An agent with "internal" clearance sees email. A public API consumer doesn't.

---

## 14. The Evaluator — Complete Implementation

### 14.1 The Query Parser

The parser converts a query string into a list of goals and a list of directives.

```go
func ParseQuery(raw string) ([]Goal, []Directive) {
    // Separate directives from goals
    goalPart, directivePart := splitDirectives(raw)

    // Parse directives
    directives := ParseDirectives(directivePart)

    // Split goal part on commas (respecting parentheses)
    goalStrings := splitGoals(goalPart)

    goals := make([]Goal, 0, len(goalStrings))
    for _, gs := range goalStrings {
        gs = strings.TrimSpace(gs)
        if gs == "" { continue }

        goal := parseGoal(gs)
        goals = append(goals, goal)
    }

    return goals, directives
}

func parseGoal(raw string) Goal {
    // Check for negation prefix
    negated := false
    if strings.HasPrefix(raw, "\\+ ") ||
       strings.HasPrefix(raw, "\\+(") {
        negated = true
        raw = strings.TrimPrefix(raw, "\\+ ")
        raw = strings.TrimPrefix(raw, "\\+")
    }

    // Check for built-in predicates
    for name := range Builtins {
        if strings.HasPrefix(raw, name+"(") {
            args := parseArgList(raw[len(name):])
            return Goal{
                Type:    BuiltinGoal,
                Builtin: name,
                Args:    args,
                Negated: negated,
            }
        }
    }

    // Check for comparison operators in prefix form
    if len(raw) > 2 && (raw[0] == '<' || raw[0] == '>') {
        op := string(raw[0])
        if raw[1] == '=' { op = raw[:2] }
        args := parseArgList(raw[len(op):])
        return Goal{
            Type:    BuiltinGoal,
            Builtin: op,
            Args:    args,
            Negated: negated,
        }
    }

    // Check for DR path (contains dots and parentheses)
    if strings.ContainsRune(raw, '.') &&
       strings.ContainsRune(raw, '(') {
        path, err := ParseDRPath(raw)
        if err == nil {
            return Goal{
                Type:    DRPathGoal,
                Path:    path,
                Negated: negated,
            }
        }
    }

    // Check for rule call
    if strings.ContainsRune(raw, '(') {
        parenIdx := strings.IndexByte(raw, '(')
        ruleName := raw[:parenIdx]
        args := parseArgList(raw[parenIdx:])
        return Goal{
            Type:     RuleGoal,
            RuleName: ruleName,
            Args:     args,
            Negated:  negated,
        }
    }

    // Bare atom — treat as a zero-arg rule call
    return Goal{
        Type:     RuleGoal,
        RuleName: raw,
        Negated:  negated,
    }
}

func parseTerm(raw string) Term {
    raw = strings.TrimSpace(raw)

    // Quoted string → Atom
    if len(raw) >= 2 && raw[0] == '"' &&
       raw[len(raw)-1] == '"' {
        return AtomTerm(raw[1 : len(raw)-1])
    }

    // Number
    if n, err := strconv.ParseFloat(raw, 64); err == nil {
        return NumTerm(n)
    }

    // Boolean
    if raw == "true" { return BoolTerm(true) }
    if raw == "false" { return BoolTerm(false) }
    if raw == "null" { return NullTerm() }

    // Variable (starts with uppercase)
    if isVariable(raw) {
        return VarTerm(raw)
    }

    // List [a, b, c]
    if len(raw) >= 2 && raw[0] == '[' &&
       raw[len(raw)-1] == ']' {
        items := splitArgs(raw[1 : len(raw)-1])
        terms := make([]Term, len(items))
        for i, item := range items {
            terms[i] = parseTerm(item)
        }
        return ListTerm(terms)
    }

    // Default: atom
    return AtomTerm(raw)
}

func splitGoals(raw string) []string {
    var parts []string
    depth := 0
    start := 0

    for i := 0; i < len(raw); i++ {
        switch raw[i] {
        case '(':
            depth++
        case ')':
            depth--
        case ',':
            if depth == 0 {
                parts = append(parts, raw[start:i])
                start = i + 1
            }
        }
    }
    if start < len(raw) {
        parts = append(parts, raw[start:])
    }
    return parts
}

func splitDirectives(raw string) (string, string) {
    idx := strings.Index(raw, "@")
    if idx < 0 {
        return raw, ""
    }
    return strings.TrimSpace(raw[:idx]),
           strings.TrimSpace(raw[idx:])
}
```

### 14.2 The Rule Parser

```go
func ParseRule(raw string) (Rule, error) {
    // Split on ":-"
    parts := strings.SplitN(raw, ":-", 2)
    if len(parts) != 2 {
        return Rule{}, fmt.Errorf("missing :- in rule")
    }

    headPart := strings.TrimSpace(parts[0])
    bodyPart := strings.TrimSpace(parts[1])

    // Remove trailing period
    bodyPart = strings.TrimRight(bodyPart, ". ")

    // Parse head
    parenIdx := strings.IndexByte(headPart, '(')
    headName := headPart
    var headArgs []Term
    if parenIdx >= 0 {
        headName = headPart[:parenIdx]
        argStr := headPart[parenIdx+1 : len(headPart)-1]
        for _, a := range splitArgs(argStr) {
            headArgs = append(headArgs, parseTerm(a))
        }
    }

    // Parse body goals
    bodyGoals, _ := ParseQuery(bodyPart)

    return Rule{
        Head:     headName,
        HeadArgs: headArgs,
        Body:     bodyGoals,
    }, nil
}
```

### 14.3 The Complete Solve Loop

```go
func Execute(kb *KnowledgeBase, query string,
             bounds QueryBounds) (*QueryResult, error) {

    goals, directives := ParseQuery(query)

    // Validate all DR paths against schema
    for _, g := range goals {
        if g.Type == DRPathGoal {
            if err := kb.ValidatePath(g.Path); err != nil {
                return nil, err
            }
        }
    }

    state := &SolveState{
        Bounds:    bounds,
        StartTime: time.Now(),
        KB:        kb,
    }

    rawResults := Solve(state, goals, NewBindings())
    results := ApplyDirectives(rawResults, directives)

    return &QueryResult{
        Bindings:   results,
        Backtracks: state.Backtracks,
        ElapsedMs:  time.Since(state.StartTime).Milliseconds(),
        Query:      query,
    }, nil
}

type QueryResult struct {
    Bindings   []Bindings
    Backtracks int64
    ElapsedMs  int64
    Query      string
}

func (qr *QueryResult) String() string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf(
        "--- RESULTS (%d matches in %dms) ---\n",
        len(qr.Bindings), qr.ElapsedMs))
    sb.WriteString(fmt.Sprintf("Query: %s\n\n", qr.Query))

    for i, b := range qr.Bindings {
        sb.WriteString(fmt.Sprintf("Result %d: ", i+1))
        first := true
        for varName, val := range b {
            if !first { sb.WriteString(", ") }
            sb.WriteString(fmt.Sprintf("%s=%s",
                                       varName, val.String()))
            first = false
        }
        sb.WriteString("\n")
    }
    return sb.String()
}
```

### 14.4 Schema Metadata

```go
type SchemaMetadata struct {
    Entities map[string]*EntitySchema
}

type EntitySchema struct {
    Name   string
    Fields map[string]*FieldSchema
}

type FieldSchema struct {
    Name      string
    Type      string  // int, float, varchar, etc.
    MinValue  *float64
    MaxValue  *float64
    MaxLength *int
    EnumValues []string
    References string  // FK target entity type
    Nullable  bool
    Classification string  // access classification
}

func (s *SchemaMetadata) GetEntity(name string) *EntitySchema {
    return s.Entities[name]
}

func (e *EntitySchema) HasField(name string) bool {
    _, ok := e.Fields[name]
    return ok
}

func (e *EntitySchema) GetField(name string) *FieldSchema {
    return e.Fields[name]
}
```

---

## 15. Bounds and Safety

Every query must be bounded. Unbounded queries consume unbounded resources.

### 15.1 The Bounds

```go
type QueryBounds struct {
    MaxBacktracks int64   // maximum choice points explored
    MaxResults    int     // maximum binding sets returned
    MaxTimeMs     int64   // wall clock timeout
    MaxDepth      int     // maximum recursion depth for rules
}

func DefaultBounds() QueryBounds {
    return QueryBounds{
        MaxBacktracks: 100000,
        MaxResults:    1000,
        MaxTimeMs:     5000,
        MaxDepth:      50,
    }
}
```

### 15.2 Enforcement in the Solver

The solver checks bounds on every recursive call. Three checks, each is a comparison and an early return:

```go
// At the top of Solve():
if state.Backtracks > state.Bounds.MaxBacktracks {
    return nil
}
if time.Since(state.StartTime).Milliseconds() >
   state.Bounds.MaxTimeMs {
    return nil
}
if len(results) >= state.Bounds.MaxResults {
    return results
}
```

### 15.3 Bounds Per Role

Bounds are configurable per role as policy data in OpsDB. An admin gets higher limits. A public API consumer gets lower limits. The query engine reads the caller's role from the auth context and looks up the bounds:

```go
func BoundsForRole(role string, policies map[string]QueryBounds) QueryBounds {
    if bounds, ok := policies[role]; ok {
        return bounds
    }
    return DefaultBounds()
}
```

### 15.4 Structured Bound Errors

When a bound is hit, the result includes which bound and how much was consumed:

```go
type BoundError struct {
    Bound     string  // "backtracks", "results", "time", "depth"
    Limit     int64
    Consumed  int64
    Partial   []Bindings  // results found before the bound
}
```

The caller sees: "Query halted: backtrack limit 100000 reached after 87 results in 3200ms." They know to narrow their query or request higher bounds.

---

## 16. Performance

### 16.1 Indexed Knowledge Base

![Fig. 5: Indexing Performance — O(N) unindexed scan diverges from O(1) indexed lookup as entity count grows.](./figures/proql_05_indexing_performance.png)

The naive approach scans all facts for every goal. The indexed approach looks up by entity type and optionally by entity ID:

```go
type IndexedKB struct {
    // Primary index: entity_type → entity_id → field → fact
    Index map[string]map[int64]map[string]Fact

    // Type-level index: entity_type → all facts
    ByType map[string][]Fact

    // Rules
    Rules map[string][]Rule

    Schema *SchemaMetadata
    Auth   *AuthContext
}

func (kb *IndexedKB) Lookup(entityType string, entityID int64,
    fieldName string) (Fact, bool) {

    byID, ok := kb.Index[entityType]
    if !ok { return Fact{}, false }

    byField, ok := byID[entityID]
    if !ok { return Fact{}, false }

    fact, ok := byField[fieldName]
    return fact, ok
}

func (kb *IndexedKB) LookupAllIDs(
    entityType string) []int64 {

    byID, ok := kb.Index[entityType]
    if !ok { return nil }

    ids := make([]int64, 0, len(byID))
    for id := range byID {
        ids = append(ids, id)
    }
    return ids
}
```

With indexing, `booking.42.status` resolves in one map lookup — O(1) regardless of how many entities exist. Without indexing, it scans every booking fact — O(N) where N is the number of booking facts.

For 10,000 booking entities with 6 fields each (60,000 facts), the difference is: indexed lookup for a specific entity and field is one map access. Unindexed scan checks 60,000 predicates. For a query with three goals, each touching a bound entity ID, that's 3 lookups vs 180,000 comparisons.

### 16.2 Path Pre-Resolution

DR paths can be validated and partially resolved at parse time. The entity type and field name segments are looked up against schema metadata once. At execution time, only the variable segments need resolution:

```go
type ResolvedPath struct {
    EntityType   string
    EntitySchema *EntitySchema
    FieldName    string
    FieldSchema  *FieldSchema
    IDSegment    PathSegment  // variable or literal
    Arg          Term
}

func PreResolvePath(path DRPath,
    schema *SchemaMetadata) (*ResolvedPath, error) {

    entityType := path.Segments[0].Literal
    es := schema.GetEntity(entityType)
    if es == nil {
        return nil, fmt.Errorf("unknown entity: %s", entityType)
    }

    fieldName := path.Segments[2].Literal
    fs := es.GetField(fieldName)
    if fs == nil {
        return nil, fmt.Errorf("unknown field: %s.%s",
                               entityType, fieldName)
    }

    return &ResolvedPath{
        EntityType:   entityType,
        EntitySchema: es,
        FieldName:    fieldName,
        FieldSchema:  fs,
        IDSegment:    path.Segments[1],
        Arg:          path.Arg,
    }, nil
}
```

### 16.3 Goal Ordering

![Fig. 6: Goal Ordering — restrictive-first narrows the search space early (funnel) while permissive-first scans wide before filtering late.](./figures/proql_06_goal_ordering.png)

The solver evaluates goals left to right. Goals that bind variables early reduce the search space for later goals. A goal with a bound entity ID does one lookup. A goal with an unbound entity ID iterates all entities of that type.

The developer controls this through goal ordering in the query. Place the most restrictive goals first:

```prolog
% Good — status filter first, reduces candidates for the join
?- booking.B.status("confirmed"),
   booking.B.resource_id(RID),
   resource.RID.name(RName).

% Worse — resource name first, iterates all resources then all bookings
?- resource.RID.name(RName),
   booking.B.resource_id(RID),
   booking.B.status("confirmed").
```

The first query iterates only confirmed bookings, then looks up each resource. The second iterates all resources, then all bookings referencing each resource, then filters by status. Same results, different performance.

---

## 17. Integration with the HTMX Method

### 17.1 Three Paths Plus ProQL

The HTMX+OpsDB method has three paths. ProQL adds a fourth interaction pattern and enhances paths 2 and 3.

**Path 1 (HTMX direct to API)** — unchanged for simple CRUD. The search API handles list views, detail views, basic filtering and pagination. ProQL is not needed here.

**Path 2 (mini service handler)** — handlers can use ProQL instead of multiple search API calls. The booking validator that checks availability across resource pools, blackout dates, and existing bookings:

```go
func bookingValidator(ctx *HandlerContext) Result {
    query := fmt.Sprintf(`
        booking.B.resource_id(%d),
        booking.B.status(S),
        member(S, [pending, confirmed]),
        booking.B.start_time(Start),
        booking.B.end_time(End),
        <(Start, "%s"),
        >(End, "%s")`,
        ctx.Data["resource_id"],
        ctx.Data["end_time"],
        ctx.Data["start_time"])

    result, err := ctx.ProQL.Execute(query, DefaultBounds())
    if err != nil {
        return Reject("query error")
    }

    if len(result.Bindings) > 0 {
        return Reject("resource already booked")
    }
    return Accept(ctx.Data)
}
```

One query replaces three search API calls plus application-code join and comparison logic.

**Path 3 (runner data)** — runners can use ProQL for their get phase. A reconciler that compares desired vs observed state:

```go
func (r *ConfigReconciler) Get() {
    query := `
        desired_config.E.field_name(Field),
        desired_config.E.value(Desired),
        observation_cache.E.field_name(Field),
        observation_cache.E.value(Observed),
        Desired \= Observed`

    r.drifted, _ = r.ProQL.Execute(query, DefaultBounds())
}
```

The drift detection is the query. No application-code comparison loop.

**Path 4 (bidirectional query editor)** — ProQL query results rendered as editable HTMX tables. This is the VSCode plugin pattern translated to the web:

```html
<div id="query-editor">
  <textarea name="query"
            hx-post="/proql/execute"
            hx-target="#query-results"
            hx-trigger="keyup changed delay:500ms">
?- booking.B.status(Status),
   booking.B.resource_id(RID),
   resource.RID.name(RName)
   @sort(B, asc) @limit(20).
  </textarea>

  <div id="query-results">
    <!-- Results rendered as editable table -->
  </div>
</div>
```

The textarea sends the query on change (debounced 500ms). The results render as a table where each cell is an hx-put target. Editing a cell and tabbing out submits the change through the API gate, then re-executes the query and swaps fresh results.

### 17.2 ProQL in the Route Manifest

Logic paths can reference ProQL queries as a step type:

```yaml
logic_paths:
  booking_detail_flow:
    steps:
      - step: query_proql
        query: |
          booking.${id}.status(Status),
          booking.${id}.resource_id(RID),
          resource.RID.name(RName),
          booking.${id}.customer_id(CID),
          customer.CID.name(CName),
          observation_cache_payment.P.booking_id(${id}),
          observation_cache_payment.P.payment_status(PayStatus)
        bind_from: path_params

      - step: return
        template: booking/detail
```

The `${id}` is substituted from path parameters. The query result bindings are available to the template. One query replaces what would be four search API calls: get booking, get resource, get customer, get payment status.

### 17.3 Schema-Driven Query Validation

The app compiler validates ProQL queries in logic paths the same way it validates search API calls — every entity type and field referenced in the query must exist in the schema. Invalid queries are rejected at compile time, not at runtime.

---

## Summary

ProQL is a minimal Prolog evaluator with seven components: terms (fat struct with type tag), facts (predicate plus arguments), unification (type-aware value matching with variable binding), backtracking (depth-first search over choice points), DR path resolution (schema-validated entity traversal), built-in predicates (comparison, membership, arithmetic, aggregation), and @directives (post-processing sort/limit/offset/distinct).

The evaluator is roughly 1500 lines of Go. The core is three functions: `Unify` (compare two terms and produce bindings), `Solve` (recursive backtracking over goals), and `FindMatchingFacts` (indexed fact lookup with authorization filtering). Everything else is parsing, built-ins, directives, and write-back construction.

Queries are bidirectional. Results carry origin annotations that enable change set construction when values are edited. Writes go through the OpsDB API gate — all ten steps run. The query is both the read and the write interface.

Authorization is silent filtering. Facts the caller can't access don't match. Variables don't bind through unauthorized paths. Two users running the same query see different results based on their authorization context. No special handling in the query.

Every query is bounded. Maximum backtracks, maximum results, maximum time, maximum recursion depth. Bounds are configurable per role as policy data. Queries exceeding bounds halt and return partial results with a structured error identifying which bound was hit.

ProQL replaces the search API for complex queries — multi-entity joins, recursive traversals, negation, aggregation across relationships. The search API remains better for simple CRUD reads. Both share the same knowledge base, the same schema validation, the same authorization layers, and the same gate pipeline for writes.
