/*
MIT License

# Copyright (c) 2025 OcomSoft

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package migrate

import (
	"fmt"
	"sort"
)

// Graph is a directed acyclic graph (DAG) of migrations.
// Each node represents one migration; edges represent dependencies.
type Graph struct {
	nodes map[string]*graphNode
}

type graphNode struct {
	migration *Migration
	parents   []*graphNode // migrations this depends on
	children  []*graphNode // migrations that depend on this
}

// BuildGraph constructs a Graph from a Registry.
// Returns an error if any dependency is missing or if a cycle is detected.
func BuildGraph(reg *Registry) (*Graph, error) {
	g := &Graph{nodes: make(map[string]*graphNode)}

	// Create all nodes first
	for _, m := range reg.All() {
		g.nodes[m.Name] = &graphNode{migration: m}
	}

	// Wire edges and detect missing dependencies
	for _, node := range g.nodes {
		for _, dep := range node.migration.Dependencies {
			parent, exists := g.nodes[dep]
			if !exists {
				return nil, fmt.Errorf("migration %q depends on %q which is not registered", node.migration.Name, dep)
			}
			node.parents = append(node.parents, parent)
			parent.children = append(parent.children, node)
		}
	}

	// Detect cycles via DFS
	if err := g.detectCycles(); err != nil {
		return nil, err
	}

	return g, nil
}

// detectCycles uses DFS coloring (white=0, grey=1, black=2) to find cycles.
func (g *Graph) detectCycles() error {
	color := make(map[string]int)
	var visit func(name string) error
	visit = func(name string) error {
		color[name] = 1 // grey: in progress
		for _, child := range g.nodes[name].children {
			if color[child.migration.Name] == 1 {
				return fmt.Errorf("cycle detected involving migration %q", child.migration.Name)
			}
			if color[child.migration.Name] == 0 {
				if err := visit(child.migration.Name); err != nil {
					return err
				}
			}
		}
		color[name] = 2 // black: done
		return nil
	}
	for name := range g.nodes {
		if color[name] == 0 {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

// Linearize returns all migrations in topological order using Kahn's algorithm.
// Deterministic: nodes at the same level are sorted by name.
func (g *Graph) Linearize() ([]*Migration, error) {
	inDegree := make(map[string]int)
	for name, node := range g.nodes {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
		for _, child := range node.children {
			inDegree[child.migration.Name]++
		}
	}

	// Collect nodes with no incoming edges (roots)
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue) // deterministic ordering

	var result []*Migration
	for len(queue) > 0 {
		sort.Strings(queue) // sort at each step for determinism
		name := queue[0]
		queue = queue[1:]
		result = append(result, g.nodes[name].migration)
		var nextBatch []string
		for _, child := range g.nodes[name].children {
			inDegree[child.migration.Name]--
			if inDegree[child.migration.Name] == 0 {
				nextBatch = append(nextBatch, child.migration.Name)
			}
		}
		queue = append(queue, nextBatch...)
	}

	if len(result) != len(g.nodes) {
		return nil, fmt.Errorf("cycle detected during linearization (processed %d of %d nodes)", len(result), len(g.nodes))
	}
	return result, nil
}

// Roots returns names of migrations with no dependencies (no parents).
func (g *Graph) Roots() []string {
	var roots []string
	for name, node := range g.nodes {
		if len(node.parents) == 0 {
			roots = append(roots, name)
		}
	}
	sort.Strings(roots)
	return roots
}

// Leaves returns names of migrations that no other migration depends on (no children).
func (g *Graph) Leaves() []string {
	var leaves []string
	for name, node := range g.nodes {
		if len(node.children) == 0 {
			leaves = append(leaves, name)
		}
	}
	sort.Strings(leaves)
	return leaves
}

// DetectBranches returns groups of leaf migration names when there are multiple leaves
// (indicating concurrent development branches). Returns empty slice if the graph is linear.
func (g *Graph) DetectBranches() [][]string {
	leaves := g.Leaves()
	if len(leaves) <= 1 {
		return [][]string{}
	}
	return [][]string{leaves}
}

// HasBranches returns true if the graph contains any branching structure.
// This includes divergent branches (multiple leaves) and diamond topologies
// where a node has more than one child, even if the branches later converge.
func (g *Graph) HasBranches() bool {
	if len(g.Leaves()) > 1 {
		return true
	}
	for _, node := range g.nodes {
		if len(node.children) > 1 {
			return true
		}
	}
	return false
}

// ReconstructState replays all operations in topological order to produce the
// full schema state as it would exist after all registered migrations have run.
func (g *Graph) ReconstructState() (*SchemaState, error) {
	order, err := g.Linearize()
	if err != nil {
		return nil, fmt.Errorf("linearizing graph for state reconstruction: %w", err)
	}
	state := NewSchemaState()
	for _, mig := range order {
		for _, op := range mig.Operations {
			if err := op.Mutate(state); err != nil {
				return nil, fmt.Errorf("mutating state for migration %q operation %q: %w", mig.Name, op.Describe(), err)
			}
		}
	}
	return state, nil
}

// DAGOutput is the JSON-serialisable representation of the full migration graph.
// This is what the compiled migration binary emits via the `dag --format json` command.
type DAGOutput struct {
	Migrations  []MigrationSummary `json:"migrations"`
	Roots       []string           `json:"roots"`
	Leaves      []string           `json:"leaves"`
	HasBranches bool               `json:"has_branches"`
	SchemaState *SchemaState       `json:"schema_state"`
}

// MigrationSummary is a JSON-serialisable summary of a single migration.
type MigrationSummary struct {
	Name         string             `json:"name"`
	Dependencies []string           `json:"dependencies"`
	Operations   []OperationSummary `json:"operations"`
}

// OperationSummary is a JSON-serialisable summary of a single operation.
type OperationSummary struct {
	Type        string `json:"type"`
	Table       string `json:"table,omitempty"`
	Description string `json:"description"`
}

// ToDAGOutput builds a DAGOutput from this graph, including the reconstructed schema state.
func (g *Graph) ToDAGOutput() (*DAGOutput, error) {
	order, err := g.Linearize()
	if err != nil {
		return nil, err
	}
	state, err := g.ReconstructState()
	if err != nil {
		return nil, err
	}

	out := &DAGOutput{
		Roots:       g.Roots(),
		Leaves:      g.Leaves(),
		HasBranches: g.HasBranches(),
		SchemaState: state,
	}

	for _, mig := range order {
		ms := MigrationSummary{
			Name:         mig.Name,
			Dependencies: mig.Dependencies,
		}
		if ms.Dependencies == nil {
			ms.Dependencies = []string{}
		}
		if ms.Operations == nil {
			ms.Operations = []OperationSummary{}
		}
		for _, op := range mig.Operations {
			ms.Operations = append(ms.Operations, OperationSummary{
				Type:        op.TypeName(),
				Table:       op.TableName(),
				Description: op.Describe(),
			})
		}
		out.Migrations = append(out.Migrations, ms)
	}
	return out, nil
}
