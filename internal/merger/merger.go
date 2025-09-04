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
package merger

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/parser"
)

type MergedSchema struct {
	Tables    map[string]*MergedTable
	Indexes   map[string]*MergedIndex
	Sequences []parser.Statement
	Functions []parser.Statement
	Triggers  []parser.Statement
	Types     []parser.Statement
	Views     []parser.Statement
}

type MergedTable struct {
	Name        string
	Columns     map[string]*MergedColumn
	ForeignKeys []parser.ForeignKey
	Sources     []string // Track which modules contributed to this table
}

type MergedColumn struct {
	parser.Column
	Source string // Track which module this column came from
}

type MergedIndex struct {
	parser.Statement
	Source string
}

type Merger struct {
	verbose bool
}

func New(verbose bool) *Merger {
	return &Merger{
		verbose: verbose,
	}
}

func (m *Merger) MergeSchemas(schemas []parser.Statement, source string) (*MergedSchema, error) {
	merged := &MergedSchema{
		Tables:    make(map[string]*MergedTable),
		Indexes:   make(map[string]*MergedIndex),
		Sequences: []parser.Statement{},
		Functions: []parser.Statement{},
		Triggers:  []parser.Statement{},
		Types:     []parser.Statement{},
		Views:     []parser.Statement{},
	}

	for _, stmt := range schemas {
		switch stmt.Type {
		case parser.CreateTable:
			if err := m.mergeTable(merged, stmt, source); err != nil {
				return nil, err
			}
		case parser.CreateIndex:
			if err := m.mergeIndex(merged, stmt, source); err != nil {
				return nil, err
			}
		case parser.CreateSequence:
			merged.Sequences = append(merged.Sequences, stmt)
		case parser.CreateFunction:
			merged.Functions = append(merged.Functions, stmt)
		case parser.CreateTrigger:
			merged.Triggers = append(merged.Triggers, stmt)
		case parser.CreateType:
			merged.Types = append(merged.Types, stmt)
		case parser.CreateView:
			merged.Views = append(merged.Views, stmt)
		}
	}

	return merged, nil
}

func (m *Merger) mergeTable(merged *MergedSchema, stmt parser.Statement, source string) error {
	tableName := strings.ToLower(stmt.ObjectName)

	existingTable, exists := merged.Tables[tableName]
	if !exists {
		// New table
		merged.Tables[tableName] = &MergedTable{
			Name:        stmt.ObjectName,
			Columns:     make(map[string]*MergedColumn),
			ForeignKeys: stmt.ForeignKeys,
			Sources:     []string{source},
		}

		// Add columns
		for _, col := range stmt.Columns {
			colName := strings.ToLower(col.Name)
			merged.Tables[tableName].Columns[colName] = &MergedColumn{
				Column: col,
				Source: source,
			}
		}

		if m.verbose {
			fmt.Printf("Added new table: %s from %s\n", stmt.ObjectName, source)
		}
		return nil
	}

	// Merge with existing table
	if m.verbose {
		fmt.Printf("Merging table: %s from %s\n", stmt.ObjectName, source)
	}

	existingTable.Sources = append(existingTable.Sources, source)

	// Merge columns
	for _, col := range stmt.Columns {
		colName := strings.ToLower(col.Name)
		existingCol, colExists := existingTable.Columns[colName]

		if !colExists {
			// New column
			existingTable.Columns[colName] = &MergedColumn{
				Column: col,
				Source: source,
			}
			if m.verbose {
				fmt.Printf("  Added column: %s\n", col.Name)
			}
		} else {
			// Merge column - apply conflict resolution
			mergedCol := m.mergeColumn(existingCol.Column, col, source)
			existingTable.Columns[colName] = &MergedColumn{
				Column: mergedCol,
				Source: fmt.Sprintf("%s,%s", existingCol.Source, source),
			}
			if m.verbose {
				fmt.Printf("  Merged column: %s\n", col.Name)
			}
		}
	}

	// Merge foreign keys
	existingTable.ForeignKeys = m.mergeForeignKeys(existingTable.ForeignKeys, stmt.ForeignKeys)

	return nil
}

func (m *Merger) mergeColumn(existing, new parser.Column, source string) parser.Column {
	merged := existing

	// Apply conflict resolution rules

	// 1. VARCHAR size - larger wins
	if existing.DataType == "VARCHAR" && new.DataType == "VARCHAR" {
		if new.Size > existing.Size {
			merged.Size = new.Size
			if m.verbose {
				fmt.Printf("    VARCHAR size conflict: %d vs %d, using %d\n",
					existing.Size, new.Size, merged.Size)
			}
		}
	}

	// 2. NULL constraints - NOT NULL wins
	if !new.IsNullable && existing.IsNullable {
		merged.IsNullable = false
		if m.verbose {
			fmt.Printf("    Nullable conflict: NOT NULL wins\n")
		}
	}

	// 3. Primary key - if either is primary key, merged is primary key
	if new.IsPrimaryKey && !existing.IsPrimaryKey {
		merged.IsPrimaryKey = true
		merged.IsNullable = false
		if m.verbose {
			fmt.Printf("    Primary key added\n")
		}
	}

	// 4. Default value - keep existing if present, otherwise use new
	if merged.DefaultValue == "" && new.DefaultValue != "" {
		merged.DefaultValue = new.DefaultValue
	}

	return merged
}

func (m *Merger) mergeForeignKeys(existing, new []parser.ForeignKey) []parser.ForeignKey {
	// Simple merge - combine both lists and deduplicate
	fkMap := make(map[string]parser.ForeignKey)

	for _, fk := range existing {
		key := m.foreignKeySignature(fk)
		fkMap[key] = fk
	}

	for _, fk := range new {
		key := m.foreignKeySignature(fk)
		if _, exists := fkMap[key]; !exists {
			fkMap[key] = fk
		}
	}

	var merged []parser.ForeignKey
	for _, fk := range fkMap {
		merged = append(merged, fk)
	}

	return merged
}

func (m *Merger) foreignKeySignature(fk parser.ForeignKey) string {
	cols := strings.Join(fk.Columns, ",")
	refCols := strings.Join(fk.ReferencedColumns, ",")
	return fmt.Sprintf("%s->%s(%s)", cols, fk.ReferencedTable, refCols)
}

func (m *Merger) mergeIndex(merged *MergedSchema, stmt parser.Statement, source string) error {
	indexName := strings.ToLower(stmt.ObjectName)

	if existing, exists := merged.Indexes[indexName]; exists {
		// Index name conflict
		if m.verbose {
			fmt.Printf("Index name conflict: %s from %s (existing from %s)\n",
				stmt.ObjectName, source, existing.Source)
		}

		// Check if they're identical
		if m.areIndexesIdentical(existing.Statement, stmt) {
			// Identical indexes, no problem
			return nil
		}

		// Different indexes with same name - rename the new one
		counter := 2
		newName := fmt.Sprintf("%s_%d", indexName, counter)
		for merged.Indexes[newName] != nil {
			counter++
			newName = fmt.Sprintf("%s_%d", indexName, counter)
		}

		stmt.ObjectName = newName
		merged.Indexes[newName] = &MergedIndex{
			Statement: stmt,
			Source:    source,
		}

		if m.verbose {
			fmt.Printf("  Renamed conflicting index to: %s\n", newName)
		}
	} else {
		merged.Indexes[indexName] = &MergedIndex{
			Statement: stmt,
			Source:    source,
		}

		if m.verbose {
			fmt.Printf("Added index: %s from %s\n", stmt.ObjectName, source)
		}
	}

	return nil
}

func (m *Merger) areIndexesIdentical(a, b parser.Statement) bool {
	if a.IndexedTable != b.IndexedTable {
		return false
	}

	if a.IsUnique != b.IsUnique {
		return false
	}

	if len(a.IndexedColumns) != len(b.IndexedColumns) {
		return false
	}

	for i, col := range a.IndexedColumns {
		if col != b.IndexedColumns[i] {
			return false
		}
	}

	return true
}

func (m *Merger) GenerateSQL(merged *MergedSchema) string {
	var sql strings.Builder

	// Generate CREATE TYPE statements first
	for _, stmt := range merged.Types {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE SEQUENCE statements
	for _, stmt := range merged.Sequences {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE TABLE statements
	for _, table := range merged.Tables {
		m.generateCreateTable(&sql, table)
	}

	// Generate CREATE INDEX statements
	for _, idx := range merged.Indexes {
		sql.WriteString(idx.Statement.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE FUNCTION statements
	for _, stmt := range merged.Functions {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE TRIGGER statements
	for _, stmt := range merged.Triggers {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	// Generate CREATE VIEW statements
	for _, stmt := range merged.Views {
		sql.WriteString(stmt.SQL)
		sql.WriteString(";\n\n")
	}

	return sql.String()
}

func (m *Merger) generateCreateTable(sql *strings.Builder, table *MergedTable) {
	sql.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", table.Name))

	// Generate columns
	first := true
	for _, col := range table.Columns {
		if !first {
			sql.WriteString(",\n")
		}
		first = false

		sql.WriteString("    ")
		sql.WriteString(col.Column.Name)
		sql.WriteString(" ")
		sql.WriteString(col.Column.DataType)

		if col.Column.Size > 0 {
			sql.WriteString(fmt.Sprintf("(%d)", col.Column.Size))
		}

		if col.Column.IsPrimaryKey {
			sql.WriteString(" PRIMARY KEY")
		}

		if !col.Column.IsNullable {
			sql.WriteString(" NOT NULL")
		}

		if col.Column.DefaultValue != "" {
			sql.WriteString(" DEFAULT ")
			sql.WriteString(col.Column.DefaultValue)
		}
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
