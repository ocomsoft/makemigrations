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
	"time"
)

// MigrationGenerator generates Goose-compatible migrations from YAML diffs
type MigrationGenerator struct {
	verbose                bool
	databaseType           DatabaseType
	includeDownSQL         bool
	reviewCommentPrefix    string
	destructiveOperations  []string
	silent                 bool
	rejectionCommentPrefix string
	filePrefix             string
}

// NewMigrationGenerator creates a new YAML migration generator
func NewMigrationGenerator(databaseType DatabaseType, verbose bool) *MigrationGenerator {
	return &MigrationGenerator{
		verbose:        verbose,
		databaseType:   databaseType,
		includeDownSQL: true,             // Default to true for backward compatibility
		filePrefix:     "20060102150405", // Default format
	}
}

// NewMigrationGeneratorWithConfig creates a new YAML migration generator with config
func NewMigrationGeneratorWithConfig(databaseType DatabaseType, verbose bool, includeDownSQL bool) *MigrationGenerator {
	return &MigrationGenerator{
		verbose:               verbose,
		databaseType:          databaseType,
		includeDownSQL:        includeDownSQL,
		reviewCommentPrefix:   "-- REVIEW: ",
		destructiveOperations: []string{"table_removed", "field_removed", "index_removed", "table_renamed", "field_renamed", "field_modified"},
		filePrefix:            "20060102150405", // Default format
	}
}

// NewMigrationGeneratorWithFullConfig creates a new YAML migration generator with full config
func NewMigrationGeneratorWithFullConfig(databaseType DatabaseType, verbose bool, includeDownSQL bool, reviewPrefix string, destructiveOps []string, silent bool, rejectionPrefix string, filePrefix string) *MigrationGenerator {
	return &MigrationGenerator{
		verbose:                verbose,
		databaseType:           databaseType,
		includeDownSQL:         includeDownSQL,
		reviewCommentPrefix:    reviewPrefix,
		destructiveOperations:  destructiveOps,
		silent:                 silent,
		rejectionCommentPrefix: rejectionPrefix,
		filePrefix:             filePrefix,
	}
}

// Migration represents a complete migration file
type Migration struct {
	Filename    string
	Description string
	UpSQL       string
	DownSQL     string
	Destructive bool
}

// GenerateMigration generates a complete migration from a YAML schema diff
func (mg *MigrationGenerator) GenerateMigration(diff *SchemaDiff, oldSchema, newSchema *Schema, customName string) (*Migration, error) {
	if diff == nil || !diff.HasChanges {
		return nil, fmt.Errorf("no changes to generate migration for")
	}

	// Create SQL converter
	converter := NewSQLConverterWithConfig(mg.databaseType, mg.verbose, false, mg.reviewCommentPrefix, mg.destructiveOperations, mg.silent, mg.rejectionCommentPrefix)

	// Convert diff to SQL
	upSQL, downSQL, err := converter.ConvertDiffToSQL(diff, oldSchema, newSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert diff to SQL: %w", err)
	}

	// Generate migration name
	var description string
	if customName != "" {
		description = customName
	} else {
		diffEngine := NewDiffEngine(mg.verbose)
		description = diffEngine.GenerateMigrationName(diff)
	}

	// Generate timestamp-based filename
	timestamp := time.Now().UTC().Format(mg.filePrefix)
	filename := fmt.Sprintf("%s_%s.sql", timestamp, description)

	// Format as Goose migration
	upMigration := mg.formatGooseMigration(upSQL, true)

	var downMigration string
	if mg.includeDownSQL {
		downMigration = mg.formatGooseMigration(downSQL, false)
	}

	migration := &Migration{
		Filename:    filename,
		Description: description,
		UpSQL:       upMigration,
		DownSQL:     downMigration,
		Destructive: diff.IsDestructive,
	}

	if mg.verbose {
		fmt.Printf("Generated migration: %s (destructive: %v)\n", filename, diff.IsDestructive)
	}

	return migration, nil
}

// formatGooseMigration formats SQL as a Goose migration section
func (mg *MigrationGenerator) formatGooseMigration(sql string, isUp bool) string {
	if strings.TrimSpace(sql) == "" {
		return ""
	}

	var builder strings.Builder

	direction := "Up"
	if !isUp {
		direction = "Down"
	}

	builder.WriteString(fmt.Sprintf("-- +goose %s\n", direction))
	builder.WriteString("-- +goose StatementBegin\n")
	builder.WriteString(sql)

	// Ensure SQL ends with newline
	if !strings.HasSuffix(sql, "\n") {
		builder.WriteString("\n")
	}

	builder.WriteString("-- +goose StatementEnd\n")

	return builder.String()
}

// GenerateCompleteMigration generates a complete migration file content
func (mg *MigrationGenerator) GenerateCompleteMigration(diff *SchemaDiff, oldSchema, newSchema *Schema, customName string) (string, error) {
	migration, err := mg.GenerateMigration(diff, oldSchema, newSchema, customName)
	if err != nil {
		return "", err
	}

	var content strings.Builder

	// Add migration header comment
	content.WriteString(fmt.Sprintf("-- Migration: %s\n", migration.Description))
	content.WriteString(fmt.Sprintf("-- Generated: %s\n", time.Now().UTC().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("-- Database: %s\n", mg.databaseType))

	if migration.Destructive {
		content.WriteString("-- WARNING: This migration contains destructive operations\n")
	}

	content.WriteString("\n")

	// Add UP migration
	if migration.UpSQL != "" {
		content.WriteString(migration.UpSQL)
		content.WriteString("\n")
	}

	// Add DOWN migration
	if migration.DownSQL != "" {
		content.WriteString(migration.DownSQL)
	}

	return content.String(), nil
}

// GenerateInitialMigration generates an initial migration from a complete schema
func (mg *MigrationGenerator) GenerateInitialMigration(schema *Schema, customName string) (string, error) {
	if schema == nil {
		return "", fmt.Errorf("schema cannot be nil")
	}

	// Create SQL converter
	converter := NewSQLConverterWithConfig(mg.databaseType, mg.verbose, false, mg.reviewCommentPrefix, mg.destructiveOperations, mg.silent, mg.rejectionCommentPrefix)

	// Convert entire schema to SQL
	upSQL, err := converter.ConvertSchema(schema)
	if err != nil {
		return "", fmt.Errorf("failed to convert schema to SQL: %w", err)
	}

	// Generate description
	description := "initial_schema"
	if customName != "" {
		description = customName
	}

	// Generate DOWN migration (drop all tables in reverse order)
	downSQL := mg.generateInitialDownMigration(schema)

	// Format as complete migration
	var content strings.Builder

	// Add migration header
	content.WriteString(fmt.Sprintf("-- Migration: %s\n", description))
	content.WriteString(fmt.Sprintf("-- Generated: %s\n", time.Now().UTC().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("-- Database: %s\n", mg.databaseType))
	content.WriteString("-- Initial schema migration\n\n")

	// Add UP migration
	upMigration := mg.formatGooseMigration(upSQL, true)
	content.WriteString(upMigration)
	content.WriteString("\n")

	// Add DOWN migration if enabled
	if mg.includeDownSQL {
		downMigration := mg.formatGooseMigration(downSQL, false)
		content.WriteString(downMigration)
	}

	return content.String(), nil
}

// generateInitialDownMigration generates the DOWN migration for initial schema
func (mg *MigrationGenerator) generateInitialDownMigration(schema *Schema) string {
	// Analyze dependencies to get reverse order
	analyzer := NewDependencyAnalyzer(mg.verbose)
	tableOrder, err := analyzer.TopologicalSort(schema)
	if err != nil {
		// If we can't sort, just use tables in reverse order
		tableOrder = make([]string, len(schema.Tables))
		for i, table := range schema.Tables {
			tableOrder[i] = table.Name
		}
	}

	// Create SQL converter for quoting
	converter := NewSQLConverterWithConfig(mg.databaseType, mg.verbose, false, mg.reviewCommentPrefix, mg.destructiveOperations, mg.silent, mg.rejectionCommentPrefix)

	var dropStatements []string

	// Drop junction tables first (they depend on main tables)
	junctionTables, err := analyzer.GenerateJunctionTables(schema)
	if err == nil {
		for _, table := range junctionTables {
			stmt := fmt.Sprintf("-- REVIEW\nDROP TABLE IF EXISTS %s;", converter.quoteName(table.Name))
			dropStatements = append(dropStatements, stmt)
		}
	}

	// Drop tables in reverse dependency order
	for i := len(tableOrder) - 1; i >= 0; i-- {
		tableName := tableOrder[i]
		stmt := fmt.Sprintf("-- REVIEW\nDROP TABLE IF EXISTS %s;", converter.quoteName(tableName))
		dropStatements = append(dropStatements, stmt)
	}

	return strings.Join(dropStatements, "\n\n")
}

// ValidateMigration validates that a migration is well-formed
func (mg *MigrationGenerator) ValidateMigration(migration *Migration) error {
	if migration == nil {
		return fmt.Errorf("migration cannot be nil")
	}

	if migration.Filename == "" {
		return fmt.Errorf("migration filename cannot be empty")
	}

	if migration.Description == "" {
		return fmt.Errorf("migration description cannot be empty")
	}

	if migration.UpSQL == "" && migration.DownSQL == "" {
		return fmt.Errorf("migration must have either UP or DOWN SQL")
	}

	// Validate that it contains proper Goose annotations
	if migration.UpSQL != "" {
		if !strings.Contains(migration.UpSQL, "-- +goose Up") {
			return fmt.Errorf("UP migration missing Goose annotation")
		}
		if !strings.Contains(migration.UpSQL, "-- +goose StatementBegin") {
			return fmt.Errorf("UP migration missing StatementBegin")
		}
		if !strings.Contains(migration.UpSQL, "-- +goose StatementEnd") {
			return fmt.Errorf("UP migration missing StatementEnd")
		}
	}

	if migration.DownSQL != "" {
		if !strings.Contains(migration.DownSQL, "-- +goose Down") {
			return fmt.Errorf("DOWN migration missing Goose annotation")
		}
		if !strings.Contains(migration.DownSQL, "-- +goose StatementBegin") {
			return fmt.Errorf("DOWN migration missing StatementBegin")
		}
		if !strings.Contains(migration.DownSQL, "-- +goose StatementEnd") {
			return fmt.Errorf("DOWN migration missing StatementEnd")
		}
	}

	return nil
}

// GetChangesSummary returns a human-readable summary of the changes
func (mg *MigrationGenerator) GetChangesSummary(diff *SchemaDiff) string {
	if diff == nil || !diff.HasChanges {
		return "No changes"
	}

	var summary []string

	// Count changes by type
	changeCounts := make(map[ChangeType]int)
	for _, change := range diff.Changes {
		changeCounts[change.Type]++
	}

	// Format summary
	for changeType, count := range changeCounts {
		var description string
		switch changeType {
		case ChangeTypeTableAdded:
			description = "table(s) added"
		case ChangeTypeTableRemoved:
			description = "table(s) removed"
		case ChangeTypeFieldAdded:
			description = "field(s) added"
		case ChangeTypeFieldRemoved:
			description = "field(s) removed"
		case ChangeTypeFieldModified:
			description = "field(s) modified"
		default:
			description = "change(s)"
		}

		summary = append(summary, fmt.Sprintf("%d %s", count, description))
	}

	result := strings.Join(summary, ", ")
	if diff.IsDestructive {
		result += " (includes destructive changes)"
	}

	return result
}

// GenerateFilename generates a migration filename with timestamp
func (mg *MigrationGenerator) GenerateFilename(description string) string {
	timestamp := time.Now().UTC().Format(mg.filePrefix)

	// Clean description for filename
	cleanDesc := strings.ReplaceAll(description, " ", "_")
	cleanDesc = strings.ReplaceAll(cleanDesc, "-", "_")
	cleanDesc = strings.ToLower(cleanDesc)

	return fmt.Sprintf("%s_%s.sql", timestamp, cleanDesc)
}

// GetMigrationStats returns statistics about the migration
func (mg *MigrationGenerator) GetMigrationStats(migration *Migration) map[string]interface{} {
	stats := make(map[string]interface{})

	stats["filename"] = migration.Filename
	stats["description"] = migration.Description
	stats["destructive"] = migration.Destructive
	stats["has_up_sql"] = migration.UpSQL != ""
	stats["has_down_sql"] = migration.DownSQL != ""

	// Count lines of SQL
	if migration.UpSQL != "" {
		stats["up_sql_lines"] = len(strings.Split(strings.TrimSpace(migration.UpSQL), "\n"))
	}
	if migration.DownSQL != "" {
		stats["down_sql_lines"] = len(strings.Split(strings.TrimSpace(migration.DownSQL), "\n"))
	}

	return stats
}
