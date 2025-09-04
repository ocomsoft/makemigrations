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
package struct2schema

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
	"github.com/ocomsoft/makemigrations/internal/version"
)

// Generator handles generation of YAML schema from Go structs
type Generator struct {
	typeMapper *TypeMapper
	tagParser  *TagParser
	verbose    bool
}

// NewGenerator creates a new schema generator
func NewGenerator(typeMapper *TypeMapper, tagParser *TagParser, verbose bool) *Generator {
	return &Generator{
		typeMapper: typeMapper,
		tagParser:  tagParser,
		verbose:    verbose,
	}
}

// GenerateSchema generates a complete schema from Go structs and relationships
func (g *Generator) GenerateSchema(structs []GoStruct, relationships []Relationship) (*types.Schema, error) {
	if g.verbose {
		fmt.Printf("Generating schema from %d struct(s)\n", len(structs))
	}

	schema := &types.Schema{
		Database: types.Database{
			Name:             "generated_schema",
			Version:          "1.0.0",
			MigrationVersion: version.GetVersion(),
		},
		Defaults: g.typeMapper.CreateDefaultsForAllDBs(),
		Tables:   []types.Table{},
	}

	// Convert structs to tables
	for _, goStruct := range structs {
		table, err := g.convertStructToTable(goStruct)
		if err != nil {
			return nil, fmt.Errorf("failed to convert struct %s: %w", goStruct.Name, err)
		}
		if table != nil {
			schema.Tables = append(schema.Tables, *table)
		}
	}

	// Add many-to-many junction tables
	junctionTables := g.generateJunctionTables(relationships)
	schema.Tables = append(schema.Tables, junctionTables...)

	if g.verbose {
		fmt.Printf("Generated %d table(s) from structs\n", len(schema.Tables))
	}

	return schema, nil
}

// convertStructToTable converts a Go struct to a database table
func (g *Generator) convertStructToTable(goStruct GoStruct) (*types.Table, error) {
	tableName := g.getTableName(goStruct)
	if tableName == "" {
		return nil, fmt.Errorf("could not determine table name for struct %s", goStruct.Name)
	}

	table := &types.Table{
		Name:    tableName,
		Fields:  []types.Field{},
		Indexes: []types.Index{},
	}

	// Convert fields
	for _, field := range goStruct.Fields {
		// Skip unexported fields
		if !field.IsExported && !field.IsEmbedded {
			continue
		}

		// Handle embedded structs by flattening their fields
		if field.IsEmbedded {
			// For now, we'll skip embedded structs as they require more complex handling
			// This could be enhanced in the future
			if g.verbose {
				fmt.Printf("Skipping embedded field: %s in struct %s\n", field.Name, goStruct.Name)
			}
			continue
		}

		// Skip slice fields - they are handled as many-to-many relationships at the schema level
		if field.IsSlice {
			if g.verbose {
				fmt.Printf("Skipping slice field: %s in struct %s (handled as many-to-many relationship)\n", field.Name, goStruct.Name)
			}
			continue
		}

		dbField, err := g.convertField(field, goStruct)
		if err != nil {
			return nil, fmt.Errorf("failed to convert field %s: %w", field.Name, err)
		}

		if dbField != nil {
			table.Fields = append(table.Fields, *dbField)
		}
	}

	// Ensure we have at least one field
	if len(table.Fields) == 0 {
		if g.verbose {
			fmt.Printf("Skipping struct %s - no valid fields found\n", goStruct.Name)
		}
		return nil, nil
	}

	// Add a primary key if none exists
	if !g.hasPrimaryKey(table.Fields) {
		idField := types.Field{
			Name:       "id",
			Type:       "serial",
			PrimaryKey: true,
		}
		// Insert at the beginning
		table.Fields = append([]types.Field{idField}, table.Fields...)
	}

	return table, nil
}

// convertField converts a Go struct field to a database field
func (g *Generator) convertField(field GoField, goStruct GoStruct) (*types.Field, error) {
	tagInfo := g.tagParser.ParseTags(field.Tag)

	// Skip ignored fields
	if tagInfo.Ignore {
		return nil, nil
	}

	// Get column name
	columnName := tagInfo.ColumnName
	if columnName == "" {
		columnName = g.toSnakeCase(field.Name)
	}

	// Map the type
	sqlType, length, precision, scale, nullable := g.typeMapper.MapType(
		field.Type, field.IsPointer, field.IsSlice, tagInfo)

	dbField := &types.Field{
		Name:       columnName,
		Type:       sqlType,
		Length:     length,
		Precision:  precision,
		Scale:      scale,
		PrimaryKey: tagInfo.PrimaryKey,
		AutoCreate: tagInfo.AutoCreate,
		AutoUpdate: tagInfo.AutoUpdate,
	}

	// Set nullability
	if nullable != nil {
		dbField.SetNullable(*nullable)
	} else if field.IsPointer {
		dbField.SetNullable(true)
	}

	// Set default value
	if tagInfo.Default != "" {
		dbField.Default = tagInfo.Default
	}

	// Handle foreign key relationships
	if sqlType == "foreign_key" || tagInfo.ForeignKey != nil {
		if tagInfo.ForeignKey != nil {
			dbField.ForeignKey = &types.ForeignKey{
				Table:    g.toSnakeCase(tagInfo.ForeignKey.Table),
				OnDelete: tagInfo.ForeignKey.OnDelete,
			}
		} else {
			// Infer foreign key from type
			refTable := g.inferTableName(field.UnderlyingType)
			dbField.ForeignKey = &types.ForeignKey{
				Table:    refTable,
				OnDelete: "RESTRICT", // Default to RESTRICT
			}
		}
	}

	// Many-to-many relationships are handled separately at the schema level through junction tables
	// This section is removed as slice fields are now skipped entirely

	return dbField, nil
}

// generateJunctionTables generates junction tables for many-to-many relationships
func (g *Generator) generateJunctionTables(relationships []Relationship) []types.Table {
	var tables []types.Table

	for _, rel := range relationships {
		if rel.Type == RelationshipManyToMany {
			junctionTable := types.Table{
				Name: rel.JunctionTable,
				Fields: []types.Field{
					{
						Name: rel.SourceTable + "_id",
						Type: "foreign_key",
						ForeignKey: &types.ForeignKey{
							Table:    rel.SourceTable,
							OnDelete: "CASCADE",
						},
					},
					{
						Name: rel.TargetTable + "_id",
						Type: "foreign_key",
						ForeignKey: &types.ForeignKey{
							Table:    rel.TargetTable,
							OnDelete: "CASCADE",
						},
					},
				},
				Indexes: []types.Index{
					{
						Name:   rel.JunctionTable + "_unique",
						Fields: []string{rel.SourceTable + "_id", rel.TargetTable + "_id"},
						Unique: true,
					},
				},
			}
			tables = append(tables, junctionTable)
		}
	}

	return tables
}

// getTableName determines the table name for a struct
func (g *Generator) getTableName(goStruct GoStruct) string {
	// Check for table name in struct tags
	tableName := g.tagParser.GetTableName(goStruct.Tags)
	if tableName != "" {
		return tableName
	}

	// Convert struct name to snake_case
	return g.toSnakeCase(goStruct.Name)
}

// inferTableName infers a table name from a Go type name
func (g *Generator) inferTableName(typeName string) string {
	// Clean the type name
	cleanName := typeName
	if strings.HasPrefix(cleanName, "*") {
		cleanName = cleanName[1:]
	}
	if strings.HasPrefix(cleanName, "[]") {
		cleanName = cleanName[2:]
	}

	// Remove package prefix
	if idx := strings.LastIndex(cleanName, "."); idx != -1 {
		cleanName = cleanName[idx+1:]
	}

	return g.toSnakeCase(cleanName)
}

// hasPrimaryKey checks if any field in the table is a primary key
func (g *Generator) hasPrimaryKey(fields []types.Field) bool {
	for _, field := range fields {
		if field.PrimaryKey {
			return true
		}
	}
	return false
}

// toSnakeCase converts a string from CamelCase to snake_case
func (g *Generator) toSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	result.Grow(len(s) + 5) // Pre-allocate with some extra space

	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Add underscore before uppercase letters (except the first character)
			result.WriteRune('_')
		}
		// Convert to lowercase
		if r >= 'A' && r <= 'Z' {
			result.WriteRune(r + 32) // Convert to lowercase
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}
