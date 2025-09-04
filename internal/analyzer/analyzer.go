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
package analyzer

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/merger"
	"github.com/ocomsoft/makemigrations/internal/parser"
)

type DependencyGraph struct {
	nodes map[string]*Node
	edges map[string][]string // table -> list of tables it depends on
}

type Node struct {
	Name    string
	Table   *merger.MergedTable
	Visited bool
	InStack bool
}

type Analyzer struct {
	verbose bool
}

func New(verbose bool) *Analyzer {
	return &Analyzer{
		verbose: verbose,
	}
}

func (a *Analyzer) AnalyzeDependencies(schema *merger.MergedSchema) (*DependencyGraph, error) {
	graph := &DependencyGraph{
		nodes: make(map[string]*Node),
		edges: make(map[string][]string),
	}

	// Create nodes for all tables
	for name, table := range schema.Tables {
		graph.nodes[strings.ToLower(name)] = &Node{
			Name:  name,
			Table: table,
		}
		graph.edges[strings.ToLower(name)] = []string{}
	}

	// Build edges based on foreign keys
	for name, table := range schema.Tables {
		fromTable := strings.ToLower(name)
		for _, fk := range table.ForeignKeys {
			toTable := strings.ToLower(fk.ReferencedTable)

			// Skip self-references
			if fromTable == toTable {
				continue
			}

			// Add edge from table to referenced table
			if _, exists := graph.nodes[toTable]; exists {
				graph.edges[fromTable] = append(graph.edges[fromTable], toTable)
				if a.verbose {
					fmt.Printf("Dependency: %s -> %s\n", name, fk.ReferencedTable)
				}
			}
		}
	}

	return graph, nil
}

func (a *Analyzer) TopologicalSort(graph *DependencyGraph) ([]string, error) {
	var sorted []string
	var stack []string

	// Reset visited flags
	for _, node := range graph.nodes {
		node.Visited = false
		node.InStack = false
	}

	// Perform DFS for each unvisited node
	for name, node := range graph.nodes {
		if !node.Visited {
			if err := a.dfsVisit(graph, name, &sorted, &stack); err != nil {
				return nil, err
			}
		}
	}

	// Reverse the sorted list to get correct order
	for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}

	if a.verbose {
		fmt.Println("Table creation order:")
		for i, table := range sorted {
			fmt.Printf("  %d. %s\n", i+1, table)
		}
	}

	return sorted, nil
}

func (a *Analyzer) dfsVisit(graph *DependencyGraph, nodeName string, sorted *[]string, stack *[]string) error {
	node := graph.nodes[nodeName]

	if node.InStack {
		// Circular dependency detected
		cycle := a.findCycle(*stack, nodeName)
		return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
	}

	if node.Visited {
		return nil
	}

	node.Visited = true
	node.InStack = true
	*stack = append(*stack, nodeName)

	// Visit all dependencies
	for _, dep := range graph.edges[nodeName] {
		if err := a.dfsVisit(graph, dep, sorted, stack); err != nil {
			return err
		}
	}

	node.InStack = false
	*stack = (*stack)[:len(*stack)-1]
	*sorted = append(*sorted, node.Name)

	return nil
}

func (a *Analyzer) findCycle(stack []string, target string) []string {
	for i, name := range stack {
		if name == target {
			cycle := append(stack[i:], target)
			return cycle
		}
	}
	return []string{target}
}

func (a *Analyzer) OrderStatements(schema *merger.MergedSchema) ([]string, error) {
	// Analyze dependencies
	graph, err := a.AnalyzeDependencies(schema)
	if err != nil {
		return nil, err
	}

	// Perform topological sort
	order, err := a.TopologicalSort(graph)
	if err != nil {
		// If circular dependency, try to break it by deferring foreign key constraints
		if strings.Contains(err.Error(), "circular dependency") {
			if a.verbose {
				fmt.Printf("Warning: %v - will use deferred constraints\n", err)
			}
			// Return tables in any order, foreign keys will be added later
			for name := range schema.Tables {
				order = append(order, name)
			}
		} else {
			return nil, err
		}
	}

	return order, nil
}

func (a *Analyzer) GenerateOrderedSQL(schema *merger.MergedSchema, tableOrder []string) string {
	var sql strings.Builder

	// Track which tables have circular dependencies
	circularTables := make(map[string]bool)
	graph, _ := a.AnalyzeDependencies(schema)
	for name := range schema.Tables {
		if err := a.detectCircular(graph, name); err != nil {
			circularTables[strings.ToLower(name)] = true
		}
	}

	// Generate CREATE TYPE statements first
	for _, stmt := range schema.Types {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE SEQUENCE statements
	for _, stmt := range schema.Sequences {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE TABLE statements in order
	deferredFKs := []parser.ForeignKey{}
	for _, tableName := range tableOrder {
		table := schema.Tables[strings.ToLower(tableName)]
		if table == nil {
			continue
		}

		if circularTables[strings.ToLower(tableName)] {
			// Create table without foreign keys
			a.generateCreateTableWithoutFK(&sql, table)
			// Save foreign keys for later
			deferredFKs = append(deferredFKs, table.ForeignKeys...)
		} else {
			a.generateCreateTable(&sql, table)
		}
	}

	// Add deferred foreign key constraints
	for _, fk := range deferredFKs {
		sql.WriteString(fmt.Sprintf("ALTER TABLE %s ADD ", findTableForFK(schema, fk)))
		if fk.Name != "" {
			sql.WriteString(fmt.Sprintf("CONSTRAINT %s ", fk.Name))
		}
		sql.WriteString(fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s);\n",
			strings.Join(fk.Columns, ", "),
			fk.ReferencedTable,
			strings.Join(fk.ReferencedColumns, ", ")))
	}

	// Generate CREATE INDEX statements
	for _, idx := range schema.Indexes {
		sql.WriteString(idx.Statement.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE FUNCTION statements
	for _, stmt := range schema.Functions {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE TRIGGER statements
	for _, stmt := range schema.Triggers {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE VIEW statements
	for _, stmt := range schema.Views {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	return sql.String()
}

func (a *Analyzer) detectCircular(graph *DependencyGraph, start string) error {
	visited := make(map[string]bool)
	stack := make(map[string]bool)

	var dfs func(string) error
	dfs = func(node string) error {
		visited[node] = true
		stack[node] = true

		for _, dep := range graph.edges[node] {
			if stack[dep] {
				return fmt.Errorf("circular dependency")
			}
			if !visited[dep] {
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}

		stack[node] = false
		return nil
	}

	return dfs(start)
}

func (a *Analyzer) generateCreateTable(sql *strings.Builder, table *merger.MergedTable) {
	sql.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", table.Name))

	// Generate columns
	first := true
	for _, col := range table.Columns {
		if !first {
			sql.WriteString(",\n")
		}
		first = false

		a.writeColumnDef(sql, col)
	}

	// Generate foreign key constraints
	for _, fk := range table.ForeignKeys {
		sql.WriteString(",\n    ")
		if fk.Name != "" {
			sql.WriteString(fmt.Sprintf("CONSTRAINT %s ", fk.Name))
		}
		sql.WriteString(fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)",
			strings.Join(fk.Columns, ", "),
			fk.ReferencedTable,
			strings.Join(fk.ReferencedColumns, ", ")))
	}

	sql.WriteString("\n);\n\n")
}

func (a *Analyzer) generateCreateTableWithoutFK(sql *strings.Builder, table *merger.MergedTable) {
	sql.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", table.Name))

	// Generate columns only
	first := true
	for _, col := range table.Columns {
		if !first {
			sql.WriteString(",\n")
		}
		first = false

		a.writeColumnDef(sql, col)
	}

	sql.WriteString("\n);\n\n")
}

func (a *Analyzer) writeColumnDef(sql *strings.Builder, col *merger.MergedColumn) {
	sql.WriteString("    ")
	sql.WriteString(col.Name)
	sql.WriteString(" ")
	sql.WriteString(col.DataType)

	if col.Size > 0 {
		sql.WriteString(fmt.Sprintf("(%d)", col.Size))
	}

	if col.IsPrimaryKey {
		sql.WriteString(" PRIMARY KEY")
	}

	if !col.IsNullable {
		sql.WriteString(" NOT NULL")
	}

	if col.DefaultValue != "" {
		sql.WriteString(" DEFAULT ")
		sql.WriteString(col.DefaultValue)
	}
}

func findTableForFK(schema *merger.MergedSchema, fk parser.ForeignKey) string {
	// Find which table contains these columns
	for name, table := range schema.Tables {
		hasAllColumns := true
		for _, colName := range fk.Columns {
			if _, exists := table.Columns[strings.ToLower(colName)]; !exists {
				hasAllColumns = false
				break
			}
		}
		if hasAllColumns {
			return name
		}
	}
	return ""
}
