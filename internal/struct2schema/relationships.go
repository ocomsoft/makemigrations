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
)

// RelationshipType represents the type of relationship between structs
type RelationshipType int

const (
	RelationshipForeignKey RelationshipType = iota
	RelationshipManyToMany
)

// Relationship represents a relationship between two structs/tables
type Relationship struct {
	Type          RelationshipType
	SourceStruct  string
	SourceTable   string
	SourceField   string
	TargetStruct  string
	TargetTable   string
	TargetField   string
	JunctionTable string
	OnDelete      string
	OnUpdate      string
}

// RelationshipDetector analyzes Go structs to detect relationships
type RelationshipDetector struct {
	verbose bool
}

// NewRelationshipDetector creates a new relationship detector
func NewRelationshipDetector(verbose bool) *RelationshipDetector {
	return &RelationshipDetector{
		verbose: verbose,
	}
}

// DetectRelationships analyzes structs to identify relationships between them
func (rd *RelationshipDetector) DetectRelationships(structs []GoStruct) []Relationship {
	if rd.verbose {
		fmt.Println("Detecting relationships between structs...")
	}

	var relationships []Relationship
	structMap := rd.buildStructMap(structs)

	for _, goStruct := range structs {
		structRelations := rd.analyzeStructRelationships(goStruct, structMap)
		relationships = append(relationships, structRelations...)
	}

	if rd.verbose {
		fmt.Printf("Found %d relationship(s)\n", len(relationships))
		for _, rel := range relationships {
			rd.logRelationship(rel)
		}
	}

	return relationships
}

// buildStructMap creates a map of struct names to GoStruct objects
func (rd *RelationshipDetector) buildStructMap(structs []GoStruct) map[string]GoStruct {
	structMap := make(map[string]GoStruct)
	for _, s := range structs {
		structMap[s.Name] = s
		// Also map with package prefix if available
		if s.Package != "" {
			structMap[s.Package+"."+s.Name] = s
		}
	}
	return structMap
}

// analyzeStructRelationships analyzes a single struct for relationships
func (rd *RelationshipDetector) analyzeStructRelationships(goStruct GoStruct, structMap map[string]GoStruct) []Relationship {
	var relationships []Relationship

	for _, field := range goStruct.Fields {
		// Skip unexported fields
		if !field.IsExported && !field.IsEmbedded {
			continue
		}

		// Parse tags to get explicit relationship info
		tagParser := NewTagParser()
		tagInfo := tagParser.ParseTags(field.Tag)

		// Check for explicit foreign key relationships
		if tagInfo.ForeignKey != nil {
			rel := Relationship{
				Type:         RelationshipForeignKey,
				SourceStruct: goStruct.Name,
				SourceTable:  rd.toSnakeCase(goStruct.Name),
				SourceField:  rd.toSnakeCase(field.Name),
				TargetStruct: tagInfo.ForeignKey.Table,
				TargetTable:  rd.toSnakeCase(tagInfo.ForeignKey.Table),
				OnDelete:     tagInfo.ForeignKey.OnDelete,
				OnUpdate:     tagInfo.ForeignKey.OnUpdate,
			}
			relationships = append(relationships, rel)
			continue
		}

		// Check for explicit many-to-many relationships
		if tagInfo.ManyToMany != nil {
			junctionTable := tagInfo.ManyToMany.JoinTable
			if junctionTable == "" {
				// Generate junction table name
				sourceTable := rd.toSnakeCase(goStruct.Name)
				targetTable := rd.toSnakeCase(tagInfo.ManyToMany.Table)
				junctionTable = rd.generateJunctionTableName(sourceTable, targetTable)
			}

			rel := Relationship{
				Type:          RelationshipManyToMany,
				SourceStruct:  goStruct.Name,
				SourceTable:   rd.toSnakeCase(goStruct.Name),
				SourceField:   rd.toSnakeCase(field.Name),
				TargetStruct:  tagInfo.ManyToMany.Table,
				TargetTable:   rd.toSnakeCase(tagInfo.ManyToMany.Table),
				JunctionTable: junctionTable,
			}
			relationships = append(relationships, rel)
			continue
		}

		// Infer relationships from field types
		inferredRel := rd.inferRelationshipFromType(goStruct, field, structMap)
		if inferredRel != nil {
			relationships = append(relationships, *inferredRel)
		}
	}

	return relationships
}

// inferRelationshipFromType infers relationship from Go field type
func (rd *RelationshipDetector) inferRelationshipFromType(goStruct GoStruct, field GoField, structMap map[string]GoStruct) *Relationship {
	// Clean the field type
	fieldType := field.UnderlyingType
	if fieldType == "" {
		fieldType = field.Type
	}

	// Remove pointer and slice indicators for analysis
	cleanType := strings.TrimPrefix(fieldType, "*")
	cleanType = strings.TrimPrefix(cleanType, "[]")

	// Remove package prefix for lookup
	if idx := strings.LastIndex(cleanType, "."); idx != -1 {
		cleanType = cleanType[idx+1:]
	}

	// Check if this type corresponds to a known struct
	if _, exists := structMap[cleanType]; !exists {
		return nil
	}

	// Determine relationship type based on field characteristics
	if field.IsSlice {
		// Slice field indicates many-to-many relationship
		sourceTable := rd.toSnakeCase(goStruct.Name)
		targetTable := rd.toSnakeCase(cleanType)

		return &Relationship{
			Type:          RelationshipManyToMany,
			SourceStruct:  goStruct.Name,
			SourceTable:   sourceTable,
			SourceField:   rd.toSnakeCase(field.Name),
			TargetStruct:  cleanType,
			TargetTable:   targetTable,
			JunctionTable: rd.generateJunctionTableName(sourceTable, targetTable),
		}
	} else {
		// Single struct field indicates foreign key relationship
		return &Relationship{
			Type:         RelationshipForeignKey,
			SourceStruct: goStruct.Name,
			SourceTable:  rd.toSnakeCase(goStruct.Name),
			SourceField:  rd.toSnakeCase(field.Name),
			TargetStruct: cleanType,
			TargetTable:  rd.toSnakeCase(cleanType),
			OnDelete:     "RESTRICT", // Default constraint
		}
	}
}

// generateJunctionTableName creates a junction table name from two table names
func (rd *RelationshipDetector) generateJunctionTableName(table1, table2 string) string {
	// Sort table names alphabetically for consistency
	if table1 > table2 {
		table1, table2 = table2, table1
	}
	return table1 + "_" + table2
}

// logRelationship logs information about a detected relationship
func (rd *RelationshipDetector) logRelationship(rel Relationship) {
	switch rel.Type {
	case RelationshipForeignKey:
		fmt.Printf("  FK: %s.%s -> %s\n", rel.SourceTable, rel.SourceField, rel.TargetTable)
	case RelationshipManyToMany:
		fmt.Printf("  M2M: %s.%s <-> %s (junction: %s)\n",
			rel.SourceTable, rel.SourceField, rel.TargetTable, rel.JunctionTable)
	}
}

// toSnakeCase converts a string from CamelCase to snake_case
func (rd *RelationshipDetector) toSnakeCase(s string) string {
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

// GetRelationshipsByType filters relationships by type
func (rd *RelationshipDetector) GetRelationshipsByType(relationships []Relationship, relType RelationshipType) []Relationship {
	var filtered []Relationship
	for _, rel := range relationships {
		if rel.Type == relType {
			filtered = append(filtered, rel)
		}
	}
	return filtered
}

// GetRelationshipsBySource filters relationships by source struct
func (rd *RelationshipDetector) GetRelationshipsBySource(relationships []Relationship, sourceStruct string) []Relationship {
	var filtered []Relationship
	for _, rel := range relationships {
		if rel.SourceStruct == sourceStruct {
			filtered = append(filtered, rel)
		}
	}
	return filtered
}

// GetRelationshipsByTarget filters relationships by target struct
func (rd *RelationshipDetector) GetRelationshipsByTarget(relationships []Relationship, targetStruct string) []Relationship {
	var filtered []Relationship
	for _, rel := range relationships {
		if rel.TargetStruct == targetStruct {
			filtered = append(filtered, rel)
		}
	}
	return filtered
}
