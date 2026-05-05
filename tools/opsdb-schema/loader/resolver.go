//# tools/opsdb-schema/loader/resolver.go

go
package loader

import (
	"fmt"

	"github.com/ghowland/opsdb/internal/model"
)

// Resolve processes all FK fields across all entities, creates Relationship
// structs, builds the dependency graph, and runs topological sort.
// Stores results in schema.LoadOrder and schema.Relationships.
// Returns error if cycles are detected.
func Resolve(schema *model.Schema) error {
	// TODO: build dependency graph from FK fields
	graph, err := BuildDependencyGraph(schema.Entities)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// TODO: detect cycles before sorting
	//   cycles := DetectCycles(graph)
	//   if len(cycles) > 0:
	//     format error listing each cycle: "cycle: A -> B -> C -> A"
	//     return error

	// TODO: run topological sort
	//   order, err := TopologicalSort(graph)
	//   if err: return err
	//   schema.LoadOrder = order

	// TODO: build Relationship structs from FK fields
	//   for each entity in schema.Entities:
	//     for each field where field.Type == "foreign_key":
	//       determine cardinality:
	//         if field.Unique: one_to_one
	//         else: many_to_one (from child perspective), one_to_many (from parent perspective)
	//       isSelfRef := field.References == entity.Name
	//       relationship := model.Relationship{
	//         SourceEntity: entity.Name,
	//         SourceField: field.Name,
	//         TargetEntity: field.References,
	//         Cardinality: cardinality,
	//         OnDeleteAction: "restrict" (default, can be overridden by field metadata),
	//         IsSelfReferential: isSelfRef,
	//       }
	//       schema.Relationships = append(schema.Relationships, relationship)

	_ = graph
	return fmt.Errorf("not implemented")
}

// BuildDependencyGraph creates an adjacency list from FK references.
// Self-referential FK edges are noted but excluded from the graph
// since they don't create ordering dependencies.
func BuildDependencyGraph(entities map[string]*model.Entity) (map[string][]string, error) {
	// TODO: initialize graph: map each entity name to empty slice
	// TODO: for each entity:
	//   for each field where field.Type == "foreign_key":
	//     if field.References == entity.Name: skip (self-referential)
	//     if field.References not in entities: return error "FK references unknown entity: {ref}"
	//     add field.References to graph[entity.Name] (entity depends on referenced entity)
	// TODO: return graph
	return nil, fmt.Errorf("not implemented")
}

// TopologicalSort performs Kahn's algorithm on the dependency graph.
// Returns entities in dependency order (entities with no dependencies first).
// Returns error if the graph contains cycles.
func TopologicalSort(graph map[string][]string) ([]string, error) {
	// TODO: compute in-degree for each node
	//   inDegree := map[string]int{}
	//   for each entity in graph: inDegree[entity] = 0
	//   for each entity, deps in graph:
	//     for each dep in deps:
	//       inDegree[entity]++ (entity depends on dep, so entity has higher in-degree)
	//   NOTE: actually the graph edges go from entity -> its dependencies,
	//   so we need to invert: if A depends on B, then B must come before A
	//   build reverse graph: for each entity with dep, add entity to reverse[dep]
	//   in-degree of entity = number of things it depends on

	// TODO: initialize queue with all nodes having in-degree 0
	// TODO: while queue not empty:
	//   dequeue node, add to result
	//   for each node that depends on this node (from reverse graph):
	//     decrement its in-degree
	//     if in-degree becomes 0: enqueue
	// TODO: if len(result) != len(graph): cycle detected, return error
	// TODO: return result
	return nil, fmt.Errorf("not implemented")
}

// DetectCycles finds all cycles in the dependency graph.
// Returns a list of cycles, each cycle being a list of entity names.
// Used for error reporting when TopologicalSort fails.
func DetectCycles(graph map[string][]string) [][]string {
	// TODO: DFS-based cycle detection
	// TODO: for each unvisited node:
	//   DFS with visited set and recursion stack
	//   if node encountered while on recursion stack: cycle found
	//   extract cycle from recursion stack
	// TODO: return all detected cycles
	return nil
}


