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
	yaml "gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

// IncludeProcessor handles processing schema includes with circular dependency tracking
type IncludeProcessor struct {
	resolver   *ModuleResolver
	verbose    bool
	processed  map[string]bool // Track processed files to prevent circular dependencies
	processing map[string]bool // Track currently processing files to detect immediate cycles
}

// NewIncludeProcessor creates a new include processor
func NewIncludeProcessor(verbose bool) *IncludeProcessor {
	return &IncludeProcessor{
		resolver:   NewModuleResolver(verbose),
		verbose:    verbose,
		processed:  make(map[string]bool),
		processing: make(map[string]bool),
	}
}

// ProcessIncludes processes all includes in a schema recursively
func (ip *IncludeProcessor) ProcessIncludes(schema *Schema, currentFile string) (*Schema, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}

	// Normalize the current file path
	normalizedFile, err := filepath.Abs(currentFile)
	if err != nil {
		normalizedFile = currentFile
	}

	if ip.verbose {
		fmt.Printf("Processing includes for schema: %s\n", normalizedFile)
	}

	// Process includes and collect all schemas
	allSchemas, err := ip.collectAllSchemas(schema, normalizedFile)
	if err != nil {
		return nil, err
	}

	if len(allSchemas) == 1 {
		// No includes to process
		return allSchemas[0], nil
	}

	// Merge all schemas with main schema winning conflicts
	mergedSchema, err := ip.mergeSchemas(allSchemas)
	if err != nil {
		return nil, fmt.Errorf("failed to merge schemas: %w", err)
	}

	if ip.verbose {
		fmt.Printf("Successfully processed %d schemas (including %d includes)\n",
			len(allSchemas), len(allSchemas)-1)
	}

	return mergedSchema, nil
}

// collectAllSchemas collects the main schema and all included schemas recursively
func (ip *IncludeProcessor) collectAllSchemas(schema *Schema, currentFile string) ([]*Schema, error) {
	var allSchemas []*Schema

	// Add the main schema first (so it wins in conflicts)
	allSchemas = append(allSchemas, schema)

	// Process each include
	for i, include := range schema.Include {
		if ip.verbose {
			fmt.Printf("Processing include %d: module=%s, path=%s\n", i, include.Module, include.Path)
		}

		includedSchemas, err := ip.processInclude(include, currentFile)
		if err != nil {
			return nil, fmt.Errorf("failed to process include %d (module=%s, path=%s): %w",
				i, include.Module, include.Path, err)
		}

		allSchemas = append(allSchemas, includedSchemas...)
	}

	return allSchemas, nil
}

// processInclude processes a single include and returns all schemas (including nested)
func (ip *IncludeProcessor) processInclude(include Include, currentFile string) ([]*Schema, error) {
	// Resolve the include path
	includePath, err := ip.resolver.ResolveIncludePath(include)
	if err != nil {
		return nil, err
	}

	// Normalize the include path
	normalizedPath, err := filepath.Abs(includePath)
	if err != nil {
		normalizedPath = includePath
	}

	// Check if we're already processing this file (immediate circular dependency)
	if ip.processing[normalizedPath] {
		if ip.verbose {
			fmt.Printf("Detected circular dependency, skipping: %s\n", normalizedPath)
		}
		return nil, nil
	}

	// Check if we've already processed this file (skip duplicate)
	if ip.processed[normalizedPath] {
		if ip.verbose {
			fmt.Printf("Already processed, skipping: %s\n", normalizedPath)
		}
		return nil, nil
	}

	// Mark as currently processing
	ip.processing[normalizedPath] = true
	defer delete(ip.processing, normalizedPath)

	// Mark as processed
	ip.processed[normalizedPath] = true

	if ip.verbose {
		fmt.Printf("Loading included schema: %s\n", normalizedPath)
	}

	// Load the included schema
	includedSchema, err := ip.loadSchemaFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load included schema %s: %w", normalizedPath, err)
	}

	// Recursively process includes in the included schema
	allIncludedSchemas, err := ip.collectAllSchemas(includedSchema, normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to process includes in %s: %w", normalizedPath, err)
	}

	return allIncludedSchemas, nil
}

// loadSchemaFile loads a YAML schema file from disk
func (ip *IncludeProcessor) loadSchemaFile(filePath string) (*Schema, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var schema Schema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", filePath, err)
	}

	if err := schema.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed for %s: %w", filePath, err)
	}

	return &schema, nil
}

// mergeSchemas merges multiple schemas with main schema (first) winning conflicts
func (ip *IncludeProcessor) mergeSchemas(schemas []*Schema) (*Schema, error) {
	if len(schemas) == 0 {
		return nil, fmt.Errorf("no schemas to merge")
	}

	if len(schemas) == 1 {
		return schemas[0], nil
	}

	// Start with the main schema (first one)
	result := &Schema{
		Database: schemas[0].Database, // Main schema database info wins
		Defaults: schemas[0].Defaults, // Main schema defaults win
		Tables:   make([]Table, 0),
	}

	// Track tables by name to handle conflicts
	tableMap := make(map[string]*Table)

	// Process all schemas (main first, so it wins conflicts)
	for schemaIndex, schema := range schemas {
		if ip.verbose && schemaIndex > 0 {
			fmt.Printf("Merging schema %d with %d tables\n", schemaIndex, len(schema.Tables))
		}

		for _, table := range schema.Tables {
			tableName := table.Name

			if existingTable, exists := tableMap[tableName]; exists {
				// Table conflict - merge with main wins
				if ip.verbose {
					fmt.Printf("Table conflict for '%s', merging with main schema winning\n", tableName)
				}

				mergedTable, err := ip.mergeTables(existingTable, &table, schemaIndex == 0)
				if err != nil {
					return nil, fmt.Errorf("failed to merge table %s: %w", tableName, err)
				}

				tableMap[tableName] = mergedTable
			} else {
				// New table
				tableCopy := table
				tableMap[tableName] = &tableCopy
			}
		}
	}

	// Convert map back to slice
	for _, table := range tableMap {
		result.Tables = append(result.Tables, *table)
	}

	if ip.verbose {
		fmt.Printf("Merge complete: %d tables in final schema\n", len(result.Tables))
	}

	return result, nil
}

// mergeTables merges two tables with mainWins determining conflict resolution
func (ip *IncludeProcessor) mergeTables(existing *Table, new *Table, mainWins bool) (*Table, error) {
	if existing.Name != new.Name {
		return nil, fmt.Errorf("cannot merge tables with different names: %s vs %s", existing.Name, new.Name)
	}

	// Start with the winning table
	var result *Table
	var other *Table

	if mainWins {
		result = existing // Main schema table wins
		other = new
	} else {
		result = new
		other = existing
	}

	// Create a copy to avoid modifying the original
	merged := &Table{
		Name:   result.Name,
		Fields: make([]Field, 0),
	}

	// Track fields by name
	fieldMap := make(map[string]*Field)

	// First add all fields from the winning table
	for _, field := range result.Fields {
		fieldCopy := field
		fieldMap[field.Name] = &fieldCopy
	}

	// Then add fields from the other table if they don't conflict
	for _, field := range other.Fields {
		if _, exists := fieldMap[field.Name]; !exists {
			fieldCopy := field
			fieldMap[field.Name] = &fieldCopy

			if ip.verbose {
				fmt.Printf("Adding field '%s' from %s schema to table '%s'\n",
					field.Name, map[bool]string{true: "main", false: "included"}[!mainWins], merged.Name)
			}
		} else if ip.verbose {
			fmt.Printf("Field conflict for '%s' in table '%s', main schema wins\n", field.Name, merged.Name)
		}
	}

	// Convert map back to slice
	for _, field := range fieldMap {
		merged.Fields = append(merged.Fields, *field)
	}

	return merged, nil
}

// Reset clears the processed file tracking (useful for testing)
func (ip *IncludeProcessor) Reset() {
	ip.processed = make(map[string]bool)
	ip.processing = make(map[string]bool)
}
