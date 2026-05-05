//# tools/opsdb-schema/loader/loader.go

go
package loader

import (
	"fmt"
	"path/filepath"

	"github.com/ghowland/opsdb/internal/model"
)

// Load orchestrates the full schema loading pipeline. Reads directory.yaml,
// loads meta-schema and conventions, processes each entity file in declared
// order, runs validation, resolves FK dependencies, injects reserved fields,
// and returns a complete Schema.
//
// This is the entry point for the loading half (before database interaction).
// The returned Schema is consumed by differ, evolution checker, generator, applier.
func Load(repoPath string) (*model.Schema, error) {
	// TODO: resolve paths
	schemaDir := filepath.Join(repoPath, "schema")
	metaPath := filepath.Join(schemaDir, "meta", "_schema_meta.yaml")
	reservedPath := filepath.Join(schemaDir, "conventions", "reserved.yaml")
	directoryPath := filepath.Join(schemaDir, "directory.yaml")

	// TODO: step 1: parse meta-schema
	//   metaSchema, err := ParseMetaSchema(metaPath)
	//   if err: return nil, fmt.Errorf("failed to parse meta-schema: %w", err)
	_ = metaPath

	// TODO: step 2: parse reserved field conventions
	//   reserved, err := ParseReserved(reservedPath)
	//   if err: return nil, fmt.Errorf("failed to parse reserved conventions: %w", err)
	_ = reservedPath

	// TODO: step 3: read directory.yaml for import order
	//   entityPaths, err := ParseDirectoryYAML(directoryPath)
	//   if err: return nil, fmt.Errorf("failed to parse directory.yaml: %w", err)
	_ = directoryPath

	// TODO: step 4: initialize empty schema
	//   schema := &model.Schema{
	//     Entities: make(map[string]*model.Entity),
	//     Errors:   []model.SchemaError{},
	//   }

	// TODO: step 5: process each entity file in directory order
	//   knownEntities := make(map[string]bool)
	//   for _, relPath := range entityPaths:
	//     fullPath := filepath.Join(schemaDir, relPath)
	//     entity, rawYAML, err := ParseEntityFile(fullPath)
	//     if err: accumulate parse error, continue
	//
	//     validate entity:
	//       errors := Validate(entity, rawYAML, metaSchema, knownEntities)
	//       if len(errors) > 0: accumulate, continue
	//
	//     add entity to schema.Entities[entity.Name]
	//     add entity.Name to knownEntities

	// TODO: step 6: resolve FK dependencies and build topological sort
	//   err = Resolve(schema)
	//   if err: return nil, fmt.Errorf("dependency resolution failed: %w", err)

	// TODO: step 7: inject reserved fields, governance fields, versioning siblings
	//   err = Inject(schema, reserved)
	//   if err: return nil, fmt.Errorf("injection failed: %w", err)

	// TODO: step 8: check for accumulated errors
	//   if len(schema.Errors) > 0:
	//     return schema, fmt.Errorf("schema has %d validation errors", len(schema.Errors))

	// TODO: return complete schema
	_ = schemaDir
	return nil, fmt.Errorf("not implemented")
}

// LoadAndValidateOnly runs the loading pipeline but stops before dependency
// resolution. Used by the validate command when no database is available.
func LoadAndValidateOnly(repoPath string) (*model.Schema, error) {
	// TODO: same as Load steps 1-5
	// TODO: skip steps 6-7 (resolve, inject)
	// TODO: return schema with any validation errors
	return nil, fmt.Errorf("not implemented")
}


