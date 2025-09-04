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
package yaml

import (
	"fmt"
	"strings"
)

// DependencyAnalyzer handles foreign key dependency analysis and topological sorting for YAML schemas
type DependencyAnalyzer struct {
	verbose bool
}

// NewDependencyAnalyzer creates a new YAML dependency analyzer
func NewDependencyAnalyzer(verbose bool) *DependencyAnalyzer {
	return &DependencyAnalyzer{
		verbose: verbose,
	}
}

// Dependency represents a foreign key dependency between tables
type Dependency struct {
	FromTable string
	ToTable   string
	FieldName string
	Type      string // "foreign_key" or "many_to_many"
}

// AnalyzeDependencies analyzes foreign key dependencies in a YAML schema
func (a *DependencyAnalyzer) AnalyzeDependencies(schema *Schema) ([]Dependency, error) {
	var dependencies []Dependency

	for _, table := range schema.Tables {
		for _, field := range table.Fields {
			if field.Type == "foreign_key" && field.ForeignKey != nil {
				refTable := field.ForeignKey.Table

				// Skip namespaced tables for now
				if strings.Contains(refTable, ".") {
					if a.verbose {
						fmt.Printf("Skipping namespaced foreign key dependency: %s -> %s\n", table.Name, refTable)
					}
					continue
				}

				dependencies = append(dependencies, Dependency{
					FromTable: table.Name,
					ToTable:   refTable,
					FieldName: field.Name,
					Type:      "foreign_key",
				})

				if a.verbose {
					fmt.Printf("Found foreign key dependency: %s.%s -> %s\n", table.Name, field.Name, refTable)
				}
			}

			if field.Type == "many_to_many" && field.ManyToMany != nil {
				refTable := field.ManyToMany.Table

				// Skip namespaced tables for now
				if strings.Contains(refTable, ".") {
					if a.verbose {
						fmt.Printf("Skipping namespaced many-to-many dependency: %s -> %s\n", table.Name, refTable)
					}
					continue
				}

				dependencies = append(dependencies, Dependency{
					FromTable: table.Name,
					ToTable:   refTable,
					FieldName: field.Name,
					Type:      "many_to_many",
				})

				if a.verbose {
					fmt.Printf("Found many-to-many dependency: %s.%s -> %s\n", table.Name, field.Name, refTable)
				}
			}
		}
	}

	return dependencies, nil
}

// TopologicalSort sorts tables based on their foreign key dependencies
func (a *DependencyAnalyzer) TopologicalSort(schema *Schema) ([]string, error) {
	dependencies, err := a.AnalyzeDependencies(schema)
	if err != nil {
		return nil, err
	}

	// Build adjacency list and in-degree count
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize all tables
	for _, table := range schema.Tables {
		graph[table.Name] = make([]string, 0)
		inDegree[table.Name] = 0
	}

	// Build the dependency graph
	for _, dep := range dependencies {
		// Check if both tables exist in the schema
		if _, exists := inDegree[dep.ToTable]; !exists {
			if a.verbose {
				fmt.Printf("Warning: Dependency references unknown table: %s\n", dep.ToTable)
			}
			continue
		}

		graph[dep.ToTable] = append(graph[dep.ToTable], dep.FromTable)
		inDegree[dep.FromTable]++
	}

	// Perform topological sort using Kahn's algorithm
	var result []string
	queue := make([]string, 0)

	// Find all tables with no incoming edges
	for table, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, table)
		}
	}

	for len(queue) > 0 {
		// Remove a table from the queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// For each dependent table
		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for circular dependencies
	if len(result) != len(schema.Tables) {
		remaining := make([]string, 0)
		for table := range inDegree {
			if inDegree[table] > 0 {
				remaining = append(remaining, table)
			}
		}
		return nil, fmt.Errorf("circular dependency detected among tables: %v", remaining)
	}

	if a.verbose {
		fmt.Printf("Topological sort order: %v\n", result)
	}

	return result, nil
}

// GenerateJunctionTables generates junction table definitions for many-to-many relationships
func (a *DependencyAnalyzer) GenerateJunctionTables(schema *Schema) ([]Table, error) {
	var junctionTables []Table

	for _, table := range schema.Tables {
		for _, field := range table.Fields {
			if field.Type == "many_to_many" && field.ManyToMany != nil {
				refTable := field.ManyToMany.Table

				// Skip namespaced tables for now
				if strings.Contains(refTable, ".") {
					if a.verbose {
						fmt.Printf("Skipping junction table for namespaced reference: %s -> %s\n", table.Name, refTable)
					}
					continue
				}

				// Generate junction table name
				junctionTableName := fmt.Sprintf("%s_%s", table.Name, field.Name)

				// Find the primary key type of the source table
				sourcePKType := "integer"
				sourcePKField := table.GetPrimaryKeyField()
				if sourcePKField != nil {
					sourcePKType = a.mapFieldTypeForForeignKey(sourcePKField.Type)
				}

				// Find the primary key type of the target table
				targetPKType := "integer"
				targetTable := schema.GetTableByName(refTable)
				if targetTable != nil {
					targetPKField := targetTable.GetPrimaryKeyField()
					if targetPKField != nil {
						targetPKType = a.mapFieldTypeForForeignKey(targetPKField.Type)
					}
				}

				// Create junction table
				junctionTable := Table{
					Name: junctionTableName,
					Fields: []Field{
						{
							Name:       "id",
							Type:       "serial",
							PrimaryKey: true,
							Nullable:   func() *bool { b := false; return &b }(),
						},
						{
							Name:     fmt.Sprintf("%s_id", table.Name),
							Type:     sourcePKType,
							Nullable: func() *bool { b := false; return &b }(),
						},
						{
							Name:     fmt.Sprintf("%s_id", refTable),
							Type:     targetPKType,
							Nullable: func() *bool { b := false; return &b }(),
						},
					},
				}

				junctionTables = append(junctionTables, junctionTable)

				if a.verbose {
					fmt.Printf("Generated junction table: %s for %s.%s -> %s\n",
						junctionTableName, table.Name, field.Name, refTable)
				}
			}
		}
	}

	return junctionTables, nil
}

// mapFieldTypeForForeignKey maps field types to appropriate foreign key types
func (a *DependencyAnalyzer) mapFieldTypeForForeignKey(fieldType string) string {
	switch fieldType {
	case "serial":
		return "integer"
	case "uuid":
		return "uuid"
	case "bigint":
		return "bigint"
	default:
		return "integer"
	}
}

// ValidateCircularDependencies checks for circular dependencies in the schema
func (a *DependencyAnalyzer) ValidateCircularDependencies(schema *Schema) error {
	_, err := a.TopologicalSort(schema)
	return err
}

// GetDependentTables returns all tables that depend on the given table
func (a *DependencyAnalyzer) GetDependentTables(schema *Schema, tableName string) ([]string, error) {
	dependencies, err := a.AnalyzeDependencies(schema)
	if err != nil {
		return nil, err
	}

	var dependents []string
	for _, dep := range dependencies {
		if dep.ToTable == tableName {
			dependents = append(dependents, dep.FromTable)
		}
	}

	return dependents, nil
}

// GetReferencedTables returns all tables that the given table references
func (a *DependencyAnalyzer) GetReferencedTables(schema *Schema, tableName string) ([]string, error) {
	dependencies, err := a.AnalyzeDependencies(schema)
	if err != nil {
		return nil, err
	}

	var referenced []string
	for _, dep := range dependencies {
		if dep.FromTable == tableName {
			referenced = append(referenced, dep.ToTable)
		}
	}

	return referenced, nil
}

// GetManyToManyRelationships returns all many-to-many relationships in the schema
func (a *DependencyAnalyzer) GetManyToManyRelationships(schema *Schema) []Dependency {
	var relationships []Dependency

	for _, table := range schema.Tables {
		for _, field := range table.Fields {
			if field.Type == "many_to_many" && field.ManyToMany != nil {
				relationships = append(relationships, Dependency{
					FromTable: table.Name,
					ToTable:   field.ManyToMany.Table,
					FieldName: field.Name,
					Type:      "many_to_many",
				})
			}
		}
	}

	return relationships
}
