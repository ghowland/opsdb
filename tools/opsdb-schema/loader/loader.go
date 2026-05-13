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
	schemaDir := filepath.Join(repoPath, "schema")
	metaPath := filepath.Join(schemaDir, "meta", "_schema_meta.yaml")
	reservedPath := filepath.Join(schemaDir, "conventions", "reserved.yaml")
	directoryPath := filepath.Join(schemaDir, "directory.yaml")

	// Step 1: parse meta-schema.
	metaSchema, err := ParseMetaSchema(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse meta-schema at %s: %w", metaPath, err)
	}

	// Step 2: parse reserved field conventions.
	reserved, err := ParseReserved(reservedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reserved conventions at %s: %w", reservedPath, err)
	}

	// Step 3: read directory.yaml for entity file import order.
	entityPaths, err := ParseDirectoryYAML(directoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory.yaml at %s: %w", directoryPath, err)
	}

	// Step 4: initialize empty schema with parsed config references.
	schema := &model.Schema{
		Entities: make(map[string]*model.Entity),
		Reserved: reserved,
	}

	// Step 5: process each entity file in directory order.
	knownEntities := make(map[string]bool)

	for _, relPath := range entityPaths {
		fullPath := filepath.Join(schemaDir, relPath)

		entity, rawYAML, parseErr := ParseEntityFile(fullPath)
		if parseErr != nil {
			schema.Errors = append(schema.Errors, model.SchemaError{
				Entity:   relPath,
				Message:  fmt.Sprintf("parse error: %v", parseErr),
				Severity: "error",
			})
			continue
		}

		// Validate entity against meta-schema, naming conventions, forbidden patterns.
		validationErrors := Validate(entity, rawYAML, metaSchema, knownEntities)
		if len(validationErrors) > 0 {
			schema.Errors = append(schema.Errors, validationErrors...)
			// Still add the entity to the schema so subsequent entities can
			// reference it for FK validation. Errors are accumulated, not fatal.
		}

		// Check for duplicate entity names.
		if _, exists := schema.Entities[entity.Name]; exists {
			schema.Errors = append(schema.Errors, model.SchemaError{
				Entity:   entity.Name,
				Message:  fmt.Sprintf("duplicate entity name %q (first defined earlier in directory.yaml)", entity.Name),
				Severity: "error",
			})
			continue
		}

		schema.Entities[entity.Name] = entity
		knownEntities[entity.Name] = true
	}

	// If there are validation errors, return early with the schema and errors.
	// The caller can inspect schema.Errors for details.
	if len(schema.Errors) > 0 {
		return schema, fmt.Errorf("schema has %d validation error(s)", len(schema.Errors))
	}

	// Step 6: resolve FK dependencies and build topological sort.
	err = Resolve(schema)
	if err != nil {
		return schema, fmt.Errorf("dependency resolution failed: %w", err)
	}

	// Step 7: inject reserved fields, governance fields, versioning siblings.
	err = Inject(schema, reserved)
	if err != nil {
		return schema, fmt.Errorf("field injection failed: %w", err)
	}

	return schema, nil
}

// LoadAndValidateOnly runs the loading pipeline but stops before dependency
// resolution and injection. Used by the validate command when no database
// is available — validates YAML syntax, naming, types, constraints, and
// forbidden patterns without needing a running Postgres.
func LoadAndValidateOnly(repoPath string) (*model.Schema, error) {
	schemaDir := filepath.Join(repoPath, "schema")
	metaPath := filepath.Join(schemaDir, "meta", "_schema_meta.yaml")
	directoryPath := filepath.Join(schemaDir, "directory.yaml")

	metaSchema, err := ParseMetaSchema(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse meta-schema at %s: %w", metaPath, err)
	}

	entityPaths, err := ParseDirectoryYAML(directoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory.yaml at %s: %w", directoryPath, err)
	}

	schema := &model.Schema{
		Entities: make(map[string]*model.Entity),
	}

	knownEntities := make(map[string]bool)

	for _, relPath := range entityPaths {
		fullPath := filepath.Join(schemaDir, relPath)

		entity, rawYAML, parseErr := ParseEntityFile(fullPath)
		if parseErr != nil {
			schema.Errors = append(schema.Errors, model.SchemaError{
				Entity:   relPath,
				Message:  fmt.Sprintf("parse error: %v", parseErr),
				Severity: "error",
			})
			continue
		}

		validationErrors := Validate(entity, rawYAML, metaSchema, knownEntities)
		if len(validationErrors) > 0 {
			schema.Errors = append(schema.Errors, validationErrors...)
		}

		if _, exists := schema.Entities[entity.Name]; exists {
			schema.Errors = append(schema.Errors, model.SchemaError{
				Entity:   entity.Name,
				Message:  fmt.Sprintf("duplicate entity name %q", entity.Name),
				Severity: "error",
			})
			continue
		}

		schema.Entities[entity.Name] = entity
		knownEntities[entity.Name] = true
	}

	if len(schema.Errors) > 0 {
		return schema, fmt.Errorf("schema has %d validation error(s)", len(schema.Errors))
	}

	return schema, nil
}
