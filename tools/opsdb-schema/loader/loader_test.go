package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ghowland/opsdb/internal/model"
	"github.com/ghowland/opsdb/internal/pg"
	"github.com/ghowland/opsdb/internal/testutil"
	"github.com/ghowland/opsdb/internal/vocabulary"
)

// --- Parser Tests ---

func TestParseMinimalEntity(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.MinimalValidEntity())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")

	entity, rawYAML, err := ParseEntityFile(entityPath)
	if err != nil {
		t.Fatalf("unexpected error parsing minimal entity: %v", err)
	}
	if entity.Name == "" {
		t.Fatal("entity name is empty")
	}
	if entity.Name != "test_entity" {
		t.Fatalf("expected name test_entity, got %s", entity.Name)
	}
	if len(entity.Fields) == 0 {
		t.Fatal("entity has no fields")
	}
	if rawYAML == nil {
		t.Fatal("raw YAML map is nil")
	}
}

func TestParseEntityWithAllTypes(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.EntityWithAllTypes())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")

	entity, _, err := ParseEntityFile(entityPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entity.Fields) < 9 {
		t.Fatalf("expected at least 9 fields, got %d", len(entity.Fields))
	}

	typesSeen := make(map[string]bool)
	for _, f := range entity.Fields {
		typesSeen[f.Type] = true
	}

	expectedTypes := []string{"int", "float", "varchar", "text", "boolean", "datetime", "date", "enum", "json"}
	for _, et := range expectedTypes {
		if !typesSeen[et] {
			t.Errorf("expected field type %s not found", et)
		}
	}
}

func TestParseDirectoryYAML(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.MinimalValidEntity())
	directoryPath := filepath.Join(repoDir, "schema", "directory.yaml")

	paths, err := ParseDirectoryYAML(directoryPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("directory.yaml returned no paths")
	}
	if !strings.Contains(paths[0], "entity_000.yaml") {
		t.Fatalf("expected path containing entity_000.yaml, got %s", paths[0])
	}
}

// --- Validation Tests ---

// NOTE: testutil.EntityWithReservedFieldCollision may need to be added to
// internal/testutil/fixtures.go if it does not exist. This test validates
// that the validator rejects entity files declaring reserved field names.
func TestValidateRejectsReservedFieldNames(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.EntityWithReservedFieldCollision())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")

	entity, rawYAML, err := ParseEntityFile(entityPath)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	metaPath := filepath.Join(repoDir, "schema", "meta", "_schema_meta.yaml")
	metaSchema, err := ParseMetaSchema(metaPath)
	if err != nil {
		t.Fatalf("meta-schema error: %v", err)
	}

	errors := Validate(entity, rawYAML, metaSchema, make(map[string]bool))
	if len(errors) == 0 {
		t.Fatal("expected validation errors for reserved field name, got none")
	}

	foundReserved := false
	for _, e := range errors {
		if strings.Contains(strings.ToLower(e.Message), "reserved") {
			foundReserved = true
			break
		}
	}
	if !foundReserved {
		t.Errorf("expected error mentioning 'reserved', got: %v", errors)
	}
}

func TestForbiddenRegex(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.EntityWithForbiddenRegex())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")

	entity, rawYAML, err := ParseEntityFile(entityPath)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	metaPath := filepath.Join(repoDir, "schema", "meta", "_schema_meta.yaml")
	metaSchema, _ := ParseMetaSchema(metaPath)

	errors := Validate(entity, rawYAML, metaSchema, make(map[string]bool))

	foundRegex := false
	for _, e := range errors {
		if strings.Contains(strings.ToLower(e.Message), "regex") ||
			strings.Contains(strings.ToLower(e.Message), "pattern") {
			foundRegex = true
			break
		}
	}
	if !foundRegex {
		t.Errorf("expected forbidden violation for regex, got: %v", errors)
	}
}

func TestForbiddenInheritance(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.EntityWithForbiddenInheritance())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")

	_, rawYAML, err := ParseEntityFile(entityPath)
	if err != nil {
		return // parse rejection of unknown key is also acceptable
	}

	violations := vocabulary.ScanForForbiddenPatterns(rawYAML)
	foundInheritance := false
	for _, v := range violations {
		if v.Pattern == "inheritance" {
			foundInheritance = true
			break
		}
	}
	if !foundInheritance {
		t.Error("expected forbidden violation for inheritance")
	}
}

func TestForbiddenLogic(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.EntityWithForbiddenLogic())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")

	_, rawYAML, err := ParseEntityFile(entityPath)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	violations := vocabulary.ScanForForbiddenPatterns(rawYAML)
	foundLogic := false
	for _, v := range violations {
		if v.Pattern == "embedded_logic" {
			foundLogic = true
			break
		}
	}
	if !foundLogic {
		t.Error("expected forbidden violation for embedded logic (NOW())")
	}
}

// NOTE: testutil.EntityWithForbiddenTemplating may need to be added to
// internal/testutil/fixtures.go if it does not exist.
func TestForbiddenTemplating(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.EntityWithForbiddenTemplating())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")

	_, rawYAML, err := ParseEntityFile(entityPath)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	violations := vocabulary.ScanForForbiddenPatterns(rawYAML)
	foundTemplate := false
	for _, v := range violations {
		if v.Pattern == "templating" {
			foundTemplate = true
			break
		}
	}
	if !foundTemplate {
		t.Error("expected forbidden violation for templating syntax")
	}
}

// --- Resolver Tests ---

// NOTE: testutil.SchemaRepoDirOrdered and testutil.ThreeEntityChain may need
// to be added to internal/testutil/fixtures.go if they do not exist.

func TestResolverTopologicalSort(t *testing.T) {
	parent, child := testutil.TwoEntitiesWithFK()
	repoDir := testutil.SchemaRepoDirOrdered(t,
		map[string]string{"parent_entity": parent, "child_entity": child},
		[]string{"parent_entity", "child_entity"},
	)

	schema, err := Load(repoDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	parentIdx, childIdx := -1, -1
	for i, name := range schema.LoadOrder {
		if name == "parent_entity" {
			parentIdx = i
		}
		if name == "child_entity" {
			childIdx = i
		}
	}
	if parentIdx == -1 || childIdx == -1 {
		t.Fatal("parent or child not found in LoadOrder")
	}
	if parentIdx >= childIdx {
		t.Fatalf("parent should precede child: parent=%d child=%d", parentIdx, childIdx)
	}
}

func TestResolverDetectsCycles(t *testing.T) {
	entityA, entityB := testutil.CyclicEntities()

	schema := &model.Schema{Entities: make(map[string]*model.Entity)}

	tmpDir := t.TempDir()
	pathA := filepath.Join(tmpDir, "cycle_a.yaml")
	pathB := filepath.Join(tmpDir, "cycle_b.yaml")
	os.WriteFile(pathA, []byte(entityA), 0644)
	os.WriteFile(pathB, []byte(entityB), 0644)

	entA, _, _ := ParseEntityFile(pathA)
	entB, _, _ := ParseEntityFile(pathB)
	if entA != nil {
		schema.Entities[entA.Name] = entA
	}
	if entB != nil {
		schema.Entities[entB.Name] = entB
	}

	err := Resolve(schema)
	if err == nil {
		t.Fatal("expected error for cyclic FK references")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "cycle") {
		t.Fatalf("expected cycle error, got: %v", err)
	}
}

func TestResolverThreeEntityChain(t *testing.T) {
	gp, p, c := testutil.ThreeEntityChain()
	repoDir := testutil.SchemaRepoDirOrdered(t,
		map[string]string{"grandparent": gp, "parent_node": p, "leaf_node": c},
		[]string{"grandparent", "parent_node", "leaf_node"},
	)

	schema, err := Load(repoDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	gpIdx, pIdx, cIdx := -1, -1, -1
	for i, name := range schema.LoadOrder {
		switch name {
		case "grandparent":
			gpIdx = i
		case "parent_node":
			pIdx = i
		case "leaf_node":
			cIdx = i
		}
	}
	if gpIdx >= pIdx || pIdx >= cIdx {
		t.Fatalf("wrong order: gp=%d p=%d c=%d", gpIdx, pIdx, cIdx)
	}
}

// --- Injector Tests ---

func TestInjectorUniversalFields(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.MinimalValidEntity())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")
	reservedPath := filepath.Join(repoDir, "schema", "conventions", "reserved.yaml")

	entity, _, _ := ParseEntityFile(entityPath)
	reserved, _ := ParseReserved(reservedPath)

	schema := &model.Schema{
		Entities:  map[string]*model.Entity{entity.Name: entity},
		LoadOrder: []string{entity.Name},
	}

	if err := Inject(schema, reserved); err != nil {
		t.Fatalf("inject error: %v", err)
	}

	fieldNames := make(map[string]bool)
	for _, f := range entity.Fields {
		fieldNames[f.Name] = true
	}
	for _, name := range []string{"id", "created_time", "updated_time"} {
		if !fieldNames[name] {
			t.Errorf("missing universal field %s", name)
		}
	}
	for _, f := range entity.Fields {
		if (f.Name == "id" || f.Name == "created_time" || f.Name == "updated_time") && !f.IsReserved {
			t.Errorf("%s should be IsReserved", f.Name)
		}
	}
}

func TestInjectorVersioningSibling(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.VersionedEntity())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")
	reservedPath := filepath.Join(repoDir, "schema", "conventions", "reserved.yaml")

	entity, _, _ := ParseEntityFile(entityPath)
	reserved, _ := ParseReserved(reservedPath)

	schema := &model.Schema{
		Entities:  map[string]*model.Entity{entity.Name: entity},
		LoadOrder: []string{entity.Name},
	}
	_ = Resolve(schema)
	if err := Inject(schema, reserved); err != nil {
		t.Fatalf("inject error: %v", err)
	}

	siblingName := entity.Name + "_version"
	sibling, exists := schema.Entities[siblingName]
	if !exists {
		t.Fatalf("sibling %s not found", siblingName)
	}

	if !sibling.IsSibling {
		t.Error("sibling should have IsSibling = true")
	}
	if sibling.ParentEntity != entity.Name {
		t.Errorf("sibling ParentEntity should be %s, got %s", entity.Name, sibling.ParentEntity)
	}

	sibFields := make(map[string]bool)
	for _, f := range sibling.Fields {
		sibFields[f.Name] = true
	}
	for _, name := range []string{"version_serial", "is_active_version", "approved_for_production_time", entity.Name + "_id", "change_set_id"} {
		if !sibFields[name] {
			t.Errorf("missing sibling field %s", name)
		}
	}
}

func TestInjectorHierarchical(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.HierarchicalEntity())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")
	reservedPath := filepath.Join(repoDir, "schema", "conventions", "reserved.yaml")

	entity, _, _ := ParseEntityFile(entityPath)
	reserved, _ := ParseReserved(reservedPath)

	schema := &model.Schema{
		Entities:  map[string]*model.Entity{entity.Name: entity},
		LoadOrder: []string{entity.Name},
	}
	if err := Inject(schema, reserved); err != nil {
		t.Fatalf("inject error: %v", err)
	}

	expectedFK := "parent_" + entity.Name + "_id"
	found := false
	for _, f := range entity.Fields {
		if f.Name == expectedFK {
			found = true
			if f.Type != "foreign_key" {
				t.Errorf("expected FK type, got %s", f.Type)
			}
			if f.References != entity.Name {
				t.Errorf("expected self-ref, got %s", f.References)
			}
			if !f.Nullable {
				t.Error("hierarchical FK should be nullable")
			}
		}
	}
	if !found {
		t.Fatalf("missing %s", expectedFK)
	}
}

func TestInjectorGovernanceFields(t *testing.T) {
	repoDir := testutil.SchemaRepoDir(t, testutil.EntityWithGovernance())
	entityPath := filepath.Join(repoDir, "schema", "domains", "test", "entity_000.yaml")
	reservedPath := filepath.Join(repoDir, "schema", "conventions", "reserved.yaml")

	entity, _, _ := ParseEntityFile(entityPath)
	reserved, _ := ParseReserved(reservedPath)

	schema := &model.Schema{
		Entities:  map[string]*model.Entity{entity.Name: entity},
		LoadOrder: []string{entity.Name},
	}
	if err := Inject(schema, reserved); err != nil {
		t.Fatalf("inject error: %v", err)
	}

	fieldNames := make(map[string]bool)
	for _, f := range entity.Fields {
		fieldNames[f.Name] = true
		if strings.HasPrefix(f.Name, "_") && !f.IsGovernance {
			t.Errorf("%s should be IsGovernance", f.Name)
		}
	}
	for _, name := range []string{"_requires_group", "_access_classification", "_retention_policy_id"} {
		if !fieldNames[name] {
			t.Errorf("missing governance field %s", name)
		}
	}
}

// --- Evolution Tests ---

func TestEvolutionAllowsNewEntity(t *testing.T) {
	diff := &SchemaDiff{NewEntities: []string{"new_table"}}
	result, _ := CheckEvolution(diff)
	if len(result.Allowed) != 1 || result.Allowed[0].ChangeType != "new_entity" {
		t.Fatal("new entity should be allowed")
	}
	if len(result.Forbidden) != 0 {
		t.Fatal("no forbidden expected")
	}
}

func TestEvolutionAllowsNewNullableField(t *testing.T) {
	diff := &SchemaDiff{
		NewFields: []DiffItem{{Entity: "e", Field: "f", DesiredValue: "varchar"}},
	}
	result, _ := CheckEvolution(diff)
	if len(result.Allowed) != 1 {
		t.Fatalf("expected 1 allowed, got %d", len(result.Allowed))
	}
}

func TestEvolutionForbidsFieldDeletion(t *testing.T) {
	diff := &SchemaDiff{
		RemovedFields: []DiffItem{{Entity: "e", Field: "f", CurrentValue: "varchar"}},
	}
	result, _ := CheckEvolution(diff)
	if len(result.Forbidden) != 1 || result.Forbidden[0].Rule != "delete_field" {
		t.Fatal("field deletion should be forbidden")
	}
	if !strings.Contains(result.Forbidden[0].Alternative, "deprecate") {
		t.Error("alternative should mention deprecate")
	}
}

func TestEvolutionForbidsEntityDeletion(t *testing.T) {
	diff := &SchemaDiff{RemovedEntities: []string{"old_table"}}
	result, _ := CheckEvolution(diff)
	if len(result.Forbidden) != 1 || result.Forbidden[0].Rule != "delete_entity" {
		t.Fatal("entity deletion should be forbidden")
	}
}

func TestEvolutionForbidsTypeChange(t *testing.T) {
	diff := &SchemaDiff{
		TypeChanges: []DiffItem{{Entity: "e", Field: "f", CurrentValue: "integer", DesiredValue: "varchar(255)"}},
	}
	result, _ := CheckEvolution(diff)
	if len(result.Forbidden) != 1 || result.Forbidden[0].Rule != "type_change" {
		t.Fatal("type change should be forbidden")
	}
	if !strings.Contains(result.Forbidden[0].Alternative, "duplication") {
		t.Error("alternative should mention duplication")
	}
}

func TestEvolutionDetectsRename(t *testing.T) {
	diff := &SchemaDiff{
		TypeChanges: []DiffItem{{Entity: "e", Field: "old_name", CurrentValue: "varchar(255)"}},
		NewFields:   []DiffItem{{Entity: "e", Field: "new_name", DesiredValue: "varchar(255)"}},
	}
	result, _ := CheckEvolution(diff)
	foundRename := false
	for _, fc := range result.Forbidden {
		if fc.Rule == "rename_field" {
			foundRename = true
		}
	}
	if !foundRename {
		t.Error("should detect rename")
	}
}

func TestEvolutionAllowsWideningRange(t *testing.T) {
	diff := &SchemaDiff{
		ChangedConstraints: []DiffItem{
			{Entity: "e", Field: "f", CurrentValue: 100, DesiredValue: 200, Description: "max_value: 100 -> 200"},
		},
	}
	result, _ := CheckEvolution(diff)
	if len(result.Allowed) != 1 {
		t.Fatalf("expected 1 allowed, got %d", len(result.Allowed))
	}
}

func TestEvolutionForbidsNarrowingRange(t *testing.T) {
	diff := &SchemaDiff{
		ChangedConstraints: []DiffItem{
			{Entity: "e", Field: "f", CurrentValue: 200, DesiredValue: 100, Description: "max_value: 200 -> 100"},
		},
	}
	result, _ := CheckEvolution(diff)
	if len(result.Forbidden) != 1 || result.Forbidden[0].Rule != "narrow_range" {
		t.Fatal("range narrowing should be forbidden")
	}
}

// --- Generator Tests ---

func TestGeneratorCreateTable(t *testing.T) {
	entity := &model.Entity{
		Name: "test_table", Category: "identity",
		Fields: []model.Field{
			{Name: "id", Type: "int", Nullable: false, IsReserved: true},
			{Name: "label", Type: "varchar", Nullable: false, MaxLength: 255},
			{Name: "status", Type: "enum", Nullable: false, EnumValues: []string{"active", "inactive"}},
		},
	}
	stmt := generateCreateTable(entity)
	if !strings.Contains(stmt.SQL, "CREATE TABLE IF NOT EXISTS test_table") {
		t.Error("missing CREATE TABLE")
	}
	if !strings.Contains(stmt.SQL, "label VARCHAR(255)") {
		t.Error("missing label column")
	}
	if !strings.Contains(stmt.SQL, "CHECK (status IN (") {
		t.Error("missing enum CHECK")
	}
}

func TestGeneratorFKConstraint(t *testing.T) {
	field := &model.Field{Name: "service_id", Type: "foreign_key", References: "service"}
	stmt := generateFKConstraint("service_connection", field)
	if !strings.Contains(stmt.SQL, "FOREIGN KEY (service_id)") {
		t.Error("missing FK clause")
	}
	if !strings.Contains(stmt.SQL, "REFERENCES service(id)") {
		t.Error("missing REFERENCES")
	}
}

func TestGeneratorAppendOnlyRevoke(t *testing.T) {
	roles := []string{"opsdb_app_role", "opsdb_runner_role", "opsdb_readonly_role"}
	stmts := generateRevokeAppendOnly("audit_log_entry", roles)
	if len(stmts) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(stmts))
	}
	for _, s := range stmts {
		if !strings.Contains(s.SQL, "REVOKE UPDATE, DELETE") {
			t.Errorf("missing REVOKE: %s", s.SQL)
		}
	}
}

func TestGeneratorOrderByDependency(t *testing.T) {
	stmts := []DDLStatement{
		{Entity: "child", Phase: 1}, {Entity: "parent", Phase: 1},
		{Entity: "child", Phase: 2}, {Entity: "parent", Phase: 2},
	}
	ordered := OrderByDependency(stmts, []string{"parent", "child"})
	if ordered[0].Entity != "parent" || ordered[0].Phase != 1 {
		t.Error("parent phase 1 should be first")
	}
	if ordered[1].Entity != "child" || ordered[1].Phase != 1 {
		t.Error("child phase 1 should be second")
	}
}

// --- Integration Tests ---

func TestFullLoadPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	parent, child := testutil.TwoEntitiesWithFK()
	repoDir := testutil.SchemaRepoDirOrdered(t,
		map[string]string{"parent_entity": parent, "child_entity": child},
		[]string{"parent_entity", "child_entity"},
	)
	schema, err := Load(repoDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(schema.Entities) < 2 {
		t.Fatal("expected at least 2 entities")
	}
	if len(schema.LoadOrder) < 2 {
		t.Fatal("expected at least 2 in load order")
	}
	if len(schema.Relationships) == 0 {
		t.Fatal("expected relationships")
	}
}

func TestApplyToPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tdb := testutil.StartTestPostgres(t)
	if tdb == nil {
		return
	}
	testutil.ResetTestDB(t, tdb)

	parent, child := testutil.TwoEntitiesWithFK()
	repoDir := testutil.SchemaRepoDirOrdered(t,
		map[string]string{"parent_entity": parent, "child_entity": child},
		[]string{"parent_entity", "child_entity"},
	)
	schema, err := Load(repoDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	var changes []AllowedChange
	for _, name := range schema.LoadOrder {
		changes = append(changes, AllowedChange{Entity: name, ChangeType: "new_entity"})
	}

	db, err := pg.Connect(tdb.DSN)
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer db.Close()

	stmts, err := GenerateDDL(schema, changes)
	if err != nil {
		t.Fatalf("generate error: %v", err)
	}
	result, err := Apply(db, stmts, false)
	if err != nil {
		t.Fatalf("apply error: %v", err)
	}
	if result.EntitiesCreated == 0 {
		t.Error("no entities created")
	}
	if !testutil.TableExists(t, tdb, "parent_entity") {
		t.Error("parent_entity not found")
	}
	if !testutil.TableExists(t, tdb, "child_entity") {
		t.Error("child_entity not found")
	}
}

func TestDryRunDoesNotPersist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tdb := testutil.StartTestPostgres(t)
	if tdb == nil {
		return
	}
	testutil.ResetTestDB(t, tdb)

	repoDir := testutil.SchemaRepoDir(t, testutil.MinimalValidEntity())
	schema, err := Load(repoDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	var changes []AllowedChange
	for _, name := range schema.LoadOrder {
		changes = append(changes, AllowedChange{Entity: name, ChangeType: "new_entity"})
	}

	db, err := pg.Connect(tdb.DSN)
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer db.Close()

	stmts, _ := GenerateDDL(schema, changes)
	_, err = DryRun(db, stmts)
	if err != nil {
		t.Fatalf("dry run error: %v", err)
	}
	if testutil.TableExists(t, tdb, "test_entity") {
		t.Error("table exists after dry run — rollback failed")
	}
}

func TestMetaPopulation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tdb := testutil.StartTestPostgres(t)
	if tdb == nil {
		return
	}
	testutil.ResetTestDB(t, tdb)

	parent, child := testutil.TwoEntitiesWithFK()
	repoDir := testutil.SchemaRepoDirOrdered(t,
		map[string]string{"parent_entity": parent, "child_entity": child},
		[]string{"parent_entity", "child_entity"},
	)
	schema, err := Load(repoDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	var changes []AllowedChange
	for _, name := range schema.LoadOrder {
		changes = append(changes, AllowedChange{Entity: name, ChangeType: "new_entity"})
	}

	db, err := pg.Connect(tdb.DSN)
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer db.Close()

	stmts, _ := GenerateDDL(schema, changes)
	_, err = Apply(db, stmts, false)
	if err != nil {
		t.Fatalf("apply error: %v", err)
	}

	err = pg.WithTransaction(db, func(tx *pg.Tx) error {
		return PopulateMeta(tx, schema, changes, "test-apply")
	})
	if err != nil {
		t.Fatalf("meta population error: %v", err)
	}

	// Verify _schema_version has a current row.
	versionCount := testutil.QueryScalarInt(t, tdb,
		"SELECT count(*) FROM _schema_version WHERE is_current = true")
	if versionCount != 1 {
		t.Errorf("expected 1 current schema version, got %d", versionCount)
	}

	// Verify entity types populated.
	entityCount := testutil.QueryScalarInt(t, tdb,
		"SELECT count(*) FROM _schema_entity_type")
	if entityCount < 2 {
		t.Errorf("expected at least 2 entity types, got %d", entityCount)
	}

	// Verify fields populated.
	fieldCount := testutil.QueryScalarInt(t, tdb,
		"SELECT count(*) FROM _schema_field")
	if fieldCount == 0 {
		t.Error("no fields in _schema_field")
	}
}

func TestFullApplyAndDiffCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	tdb := testutil.StartTestPostgres(t)
	if tdb == nil {
		return
	}
	testutil.ResetTestDB(t, tdb)

	repoDir := testutil.SchemaRepoDir(t, testutil.MinimalValidEntity())
	schema, err := Load(repoDir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	var changes []AllowedChange
	for _, name := range schema.LoadOrder {
		changes = append(changes, AllowedChange{Entity: name, ChangeType: "new_entity"})
	}

	db, err := pg.Connect(tdb.DSN)
	if err != nil {
		t.Fatalf("connect error: %v", err)
	}
	defer db.Close()

	stmts, _ := GenerateDDL(schema, changes)
	_, err = Apply(db, stmts, false)
	if err != nil {
		t.Fatalf("apply error: %v", err)
	}
	err = pg.WithTransaction(db, func(tx *pg.Tx) error {
		return PopulateMeta(tx, schema, changes, "test")
	})
	if err != nil {
		t.Fatalf("meta error: %v", err)
	}

	// Re-read current state and diff — should show no changes.
	current, err := ReadCurrentState(db)
	if err != nil {
		t.Fatalf("read state error: %v", err)
	}

	diff, err := Diff(schema, current)
	if err != nil {
		t.Fatalf("diff error: %v", err)
	}

	if len(diff.NewEntities) > 0 {
		t.Errorf("expected no new entities, got %d: %v", len(diff.NewEntities), diff.NewEntities)
	}
	if len(diff.RemovedEntities) > 0 {
		t.Errorf("expected no removed entities, got %d", len(diff.RemovedEntities))
	}
	if len(diff.TypeChanges) > 0 {
		t.Errorf("expected no type changes, got %d", len(diff.TypeChanges))
	}
}
