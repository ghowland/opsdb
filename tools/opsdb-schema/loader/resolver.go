package loader

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ghowland/opsdb/internal/model"
)

// Resolve processes all FK fields across all entities, creates Relationship
// structs, builds the dependency graph, and runs topological sort.
// Stores results in schema.LoadOrder and schema.Relationships.
// Returns error if cycles are detected.
func Resolve(schema *model.Schema) error {
	// Build dependency graph from FK fields.
	graph, err := BuildDependencyGraph(schema.Entities)
	if err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	// Detect cycles before sorting — better error messages than Kahn's "not all visited."
	cycles := DetectCycles(graph)
	if len(cycles) > 0 {
		var cycleStrs []string
		for _, cycle := range cycles {
			cycleStrs = append(cycleStrs, strings.Join(cycle, " -> "))
		}
		return fmt.Errorf("circular FK dependencies detected:\n  %s", strings.Join(cycleStrs, "\n  "))
	}

	// Run topological sort.
	order, err := TopologicalSort(graph)
	if err != nil {
		return fmt.Errorf("topological sort failed: %w", err)
	}
	schema.LoadOrder = order

	// Build Relationship structs from FK fields.
	for _, entity := range schema.Entities {
		for _, field := range entity.Fields {
			if field.Type != "foreign_key" || field.References == "" {
				continue
			}

			cardinality := "many_to_one"
			if field.Unique {
				cardinality = "one_to_one"
			}

			isSelfRef := field.References == entity.Name

			schema.Relationships = append(schema.Relationships, model.Relationship{
				SourceEntity:      entity.Name,
				SourceField:       field.Name,
				TargetEntity:      field.References,
				Cardinality:       cardinality,
				OnDeleteAction:    "restrict",
				IsSelfReferential: isSelfRef,
			})
		}
	}

	return nil
}

// BuildDependencyGraph creates an adjacency list from FK references.
// Each key is an entity name, each value is the list of entities it depends on.
// Self-referential FK edges are excluded since they don't create ordering dependencies.
func BuildDependencyGraph(entities map[string]*model.Entity) (map[string][]string, error) {
	graph := make(map[string][]string, len(entities))

	// Initialize every entity with an empty dependency list.
	for name := range entities {
		graph[name] = nil
	}

	// Add edges from FK references.
	for name, entity := range entities {
		seen := make(map[string]bool) // deduplicate multiple FKs to same target
		for _, field := range entity.Fields {
			if field.Type != "foreign_key" || field.References == "" {
				continue
			}

			// Self-referential: skip (no ordering dependency).
			if field.References == name {
				continue
			}

			// Check referenced entity exists.
			if _, exists := entities[field.References]; !exists {
				return nil, fmt.Errorf("entity %q field %q references unknown entity %q (check directory.yaml order or entity name spelling)",
					name, field.Name, field.References)
			}

			// Deduplicate: entity may have multiple FKs to the same target.
			if seen[field.References] {
				continue
			}
			seen[field.References] = true

			graph[name] = append(graph[name], field.References)
		}
	}

	return graph, nil
}

// TopologicalSort performs Kahn's algorithm on the dependency graph.
// Returns entities in dependency order: entities with no dependencies first,
// then entities that depend only on already-listed entities.
// Returns error if the graph contains cycles (not all nodes visited).
func TopologicalSort(graph map[string][]string) ([]string, error) {
	// In-degree: how many dependencies each entity has.
	inDegree := make(map[string]int, len(graph))
	for name := range graph {
		inDegree[name] = 0
	}
	for _, deps := range graph {
		for _, dep := range deps {
			// This entity depends on dep, but in-degree counts how many
			// things depend on us. We need the reverse: in-degree = number
			// of dependencies (edges pointing INTO this node in the "must come after" sense).
			// Actually, for Kahn's: inDegree[entity] = len(graph[entity]) is the dependency count.
		}
		_ = dep
	}

	// Correct approach: inDegree[X] = number of entities X depends on.
	// Reverse graph: reverseAdj[Y] = list of entities that depend on Y.
	reverseAdj := make(map[string][]string, len(graph))
	for name := range graph {
		reverseAdj[name] = nil
		inDegree[name] = len(graph[name])
	}
	for name, deps := range graph {
		for _, dep := range deps {
			reverseAdj[dep] = append(reverseAdj[dep], name)
		}
	}

	// Initialize queue with entities that have no dependencies.
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	// Sort the initial queue for deterministic output.
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		// Dequeue first element.
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// For each entity that depends on this node:
		// decrement its in-degree, enqueue if zero.
		dependents := reverseAdj[node]
		sort.Strings(dependents) // deterministic ordering
		for _, dependent := range dependents {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(result) != len(graph) {
		// Cycle detected — some nodes never reached in-degree 0.
		var stuck []string
		for name, degree := range inDegree {
			if degree > 0 {
				stuck = append(stuck, name)
			}
		}
		sort.Strings(stuck)
		return nil, fmt.Errorf("cycle detected: entities with unresolved dependencies: %s",
			strings.Join(stuck, ", "))
	}

	return result, nil
}

// DetectCycles finds all cycles in the dependency graph using DFS.
// Returns a list of cycles, each cycle being a list of entity names
// forming the loop (last element connects back to first).
func DetectCycles(graph map[string][]string) [][]string {
	var cycles [][]string

	// States: 0 = unvisited, 1 = in recursion stack, 2 = fully visited.
	state := make(map[string]int, len(graph))
	parent := make(map[string]string, len(graph))

	// Get sorted keys for deterministic cycle detection.
	nodes := make([]string, 0, len(graph))
	for name := range graph {
		nodes = append(nodes, name)
	}
	sort.Strings(nodes)

	var dfs func(node string)
	dfs = func(node string) {
		state[node] = 1 // in recursion stack

		deps := graph[node]
		sort.Strings(deps)
		for _, dep := range deps {
			if state[dep] == 1 {
				// Found a cycle. Extract it from the parent chain.
				cycle := extractCycle(node, dep, parent)
				cycles = append(cycles, cycle)
			} else if state[dep] == 0 {
				parent[dep] = node
				dfs(dep)
			}
		}

		state[node] = 2 // fully visited
	}

	for _, node := range nodes {
		if state[node] == 0 {
			dfs(node)
		}
	}

	return cycles
}

// extractCycle traces the parent chain from 'from' back to 'to' to build
// the cycle path. Returns the cycle as [to, ..., from, to].
func extractCycle(from string, to string, parent map[string]string) []string {
	var path []string
	current := from
	for current != to {
		path = append([]string{current}, path...)
		prev, ok := parent[current]
		if !ok {
			// Safety: if we can't trace back, return what we have.
			break
		}
		current = prev

		// Safety bound: prevent infinite loop if parent chain is broken.
		if len(path) > len(parent)+1 {
			break
		}
	}
	path = append([]string{to}, path...)
	path = append(path, to) // close the cycle
	return path
}
