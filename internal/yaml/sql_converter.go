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
	"sort"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/providers"
)

// PromptResponse represents the user's response to a destructive operation prompt
type PromptResponse int

const (
	PromptGenerate     PromptResponse = 1 // Generate SQL without any prefix
	PromptReview       PromptResponse = 2 // Mark for review (use review prefix)
	PromptOmit         PromptResponse = 3 // Don't generate statement (omit from output)
	PromptExit         PromptResponse = 4 // Exit migration generation
	PromptGenerateAll  PromptResponse = 5 // Generate all remaining destructive operations without prompting
	PromptIgnoreErrors PromptResponse = 6 // Generate with IgnoreErrors: true (runner continues on failure)
)

// Field types
const (
	FieldTypeVarchar    = "varchar"
	FieldTypeText       = "text"
	FieldTypeInteger    = "integer"
	FieldTypeBigint     = "bigint"
	FieldTypeSerial     = "serial"
	FieldTypeFloat      = "float"
	FieldTypeDecimal    = "decimal"
	FieldTypeBoolean    = "boolean"
	FieldTypeDate       = "date"
	FieldTypeTime       = "time"
	FieldTypeTimestamp  = "timestamp"
	FieldTypeUUID       = "uuid"
	FieldTypeJSON       = "json"
	FieldTypeJSONB      = "jsonb"
	FieldTypeForeignKey = "foreign_key"
)

// SQL types
const (
	SQLTypeText             = "TEXT"
	SQLTypeInteger          = "INTEGER"
	SQLTypeInt              = "INT"
	SQLTypeBigint           = "BIGINT"
	SQLTypeFloat            = "FLOAT"
	SQLTypeReal             = "REAL"
	SQLTypeDecimal          = "DECIMAL"
	SQLTypeBoolean          = "BOOLEAN"
	SQLTypeDate             = "DATE"
	SQLTypeTime             = "TIME"
	SQLTypeTimestamp        = "TIMESTAMP"
	SQLTypeUUID             = "UUID"
	SQLTypeChar36           = "CHAR(36)"
	SQLTypeUniqueIdentifier = "UNIQUEIDENTIFIER"
	SQLTypeJSONB            = "JSONB"
	SQLTypeNVarcharMax      = "NVARCHAR(MAX)"
)

// Common values
const (
	DefaultMigrationsDir = "migrations"
)

// PromptFunc is called when a destructive operation is detected during SQL conversion.
// It receives the SQL statement and change details, and returns the user's choice.
// Implementations can use any UI (bubbletea TUI, plain stdin, etc.).
type PromptFunc func(sqlStmt string, change Change) (PromptResponse, error)

// IsDestructiveOperation returns true if the operation type may cause data loss.
// Exported so callers outside this package (e.g. cmd) can apply the same logic
// when building per-change decisions before code generation.
func IsDestructiveOperation(changeType ChangeType) bool {
	return isDestructiveOperation(changeType)
}

// isDestructiveOperation returns true if the operation type is actually destructive
func isDestructiveOperation(changeType ChangeType) bool {
	switch changeType {
	case ChangeTypeTableRemoved, ChangeTypeFieldRemoved, ChangeTypeIndexRemoved,
		ChangeTypeTableRenamed, ChangeTypeFieldRenamed, ChangeTypeFieldModified:
		return true
	case ChangeTypeTableAdded, ChangeTypeFieldAdded, ChangeTypeIndexAdded:
		return false // These are safe operations
	default:
		return false // Be conservative - don't prompt for unknown operations
	}
}

// SQLConverter converts YAML schema definitions to database-specific SQL DDL.
//
// Per-change-type operation handlers live in sql_operations.go.
// Review comment logic lives in sql_review.go.
type SQLConverter struct {
	provider               providers.Provider
	verbose                bool
	databaseType           DatabaseType
	safeTypeChanges        bool
	reviewCommentPrefix    string
	destructiveOperations  map[string]bool
	silent                 bool
	rejectionCommentPrefix string
	generateAllTypes       map[ChangeType]bool // Tracks which change types should auto-generate without prompting
	promptFunc             PromptFunc          // Called for destructive operations; nil means silent (PromptReview)
}

// NewSQLConverter creates a new SQL converter
func NewSQLConverter(databaseType DatabaseType, verbose bool) *SQLConverter {
	provider, err := providers.NewProvider(databaseType, nil)
	if err != nil {
		// For backwards compatibility, fallback to a default provider if creation fails
		// This shouldn't happen with valid database types, but return nil instead of panic for tests
		return nil
	}

	return &SQLConverter{
		provider:            provider,
		verbose:             verbose,
		databaseType:        databaseType,
		reviewCommentPrefix: "-- REVIEW: ",
		destructiveOperations: map[string]bool{
			"table_removed":  true,
			"field_removed":  true,
			"index_removed":  true,
			"table_renamed":  true,
			"field_renamed":  true,
			"field_modified": true,
		},
	}
}

// NewSQLConverterWithOptions creates a new SQL converter with configuration options
func NewSQLConverterWithOptions(databaseType DatabaseType, verbose bool, safeTypeChanges bool) *SQLConverter {
	provider, err := providers.NewProvider(databaseType, nil)
	if err != nil {
		return nil
	}

	return &SQLConverter{
		provider:            provider,
		verbose:             verbose,
		databaseType:        databaseType,
		safeTypeChanges:     safeTypeChanges,
		reviewCommentPrefix: "-- REVIEW: ",
		destructiveOperations: map[string]bool{
			"table_removed":  true,
			"field_removed":  true,
			"index_removed":  true,
			"table_renamed":  true,
			"field_renamed":  true,
			"field_modified": true,
		},
	}
}

// NewSQLConverterWithConfig creates a new SQL converter with full configuration.
// The promptFn parameter is called for destructive operations. When nil or when
// silent is true, destructive operations default to PromptReview (mark for review).
func NewSQLConverterWithConfig(databaseType DatabaseType, verbose bool, safeTypeChanges bool, reviewPrefix string, destructiveOps []string, silent bool, rejectionPrefix string, promptFn ...PromptFunc) *SQLConverter {
	// Build destructive operations map
	destructiveMap := make(map[string]bool)
	for _, op := range destructiveOps {
		destructiveMap[op] = true
	}

	// Note: Empty reviewPrefix is valid and means no review comments
	if len(destructiveMap) == 0 {
		// Default destructive operations
		destructiveMap = map[string]bool{
			"table_removed":  true,
			"field_removed":  true,
			"index_removed":  true,
			"table_renamed":  true,
			"field_renamed":  true,
			"field_modified": true,
		}
	}

	provider, err := providers.NewProvider(databaseType, nil)
	if err != nil {
		return nil
	}

	var fn PromptFunc
	if len(promptFn) > 0 && promptFn[0] != nil {
		fn = promptFn[0]
	}

	return &SQLConverter{
		provider:               provider,
		verbose:                verbose,
		databaseType:           databaseType,
		safeTypeChanges:        safeTypeChanges,
		reviewCommentPrefix:    reviewPrefix,
		destructiveOperations:  destructiveMap,
		silent:                 silent,
		rejectionCommentPrefix: rejectionPrefix,
		promptFunc:             fn,
	}
}

// ConvertSchema converts a complete YAML schema to SQL DDL statements
func (sc *SQLConverter) ConvertSchema(schema *Schema) (string, error) {
	if schema == nil {
		return "", fmt.Errorf("schema cannot be nil")
	}

	// Apply user-defined type mappings from schema if present
	if mappings := schema.TypeMappings.ForProvider(sc.databaseType); mappings != nil {
		sc.provider.SetTypeMappings(mappings)
	}

	// Analyze dependencies and get topological order
	analyzer := NewDependencyAnalyzer(sc.verbose)
	tableOrder, err := analyzer.TopologicalSort(schema)
	if err != nil {
		return "", fmt.Errorf("failed to analyze dependencies: %w", err)
	}

	// Generate junction tables for many-to-many relationships
	junctionTables, err := analyzer.GenerateJunctionTables(schema)
	if err != nil {
		return "", fmt.Errorf("failed to generate junction tables: %w", err)
	}

	var sqlStatements []string

	// Convert tables in dependency order
	for _, tableName := range tableOrder {
		table := schema.GetTableByName(tableName)
		if table == nil {
			continue
		}

		sql, err := sc.ConvertTable(schema, table)
		if err != nil {
			return "", fmt.Errorf("failed to convert table %s: %w", tableName, err)
		}
		sqlStatements = append(sqlStatements, sql)
	}

	// Add junction tables at the end
	for _, junctionTable := range junctionTables {
		sql, err := sc.ConvertTable(schema, &junctionTable)
		if err != nil {
			return "", fmt.Errorf("failed to convert junction table %s: %w", junctionTable.Name, err)
		}
		sqlStatements = append(sqlStatements, sql)
	}

	// Generate indexes and foreign key constraints
	indexSQL := sc.generateIndexes(schema)
	if indexSQL != "" {
		sqlStatements = append(sqlStatements, indexSQL)
	}

	foreignKeySQL := sc.generateForeignKeyConstraints(schema, junctionTables)
	if foreignKeySQL != "" {
		sqlStatements = append(sqlStatements, foreignKeySQL)
	}

	return strings.Join(sqlStatements, "\n\n"), nil
}

// ConvertTable converts a single YAML table definition to SQL CREATE TABLE statement
func (sc *SQLConverter) ConvertTable(schema *Schema, table *Table) (string, error) {
	// Use provider to generate CREATE TABLE statement
	sql, err := sc.provider.GenerateCreateTable(schema, table)
	if err != nil {
		return "", err
	}

	if sc.verbose {
		fmt.Printf("Generated CREATE TABLE for %s\n", table.Name)
	}

	return sql, nil
}

// ConvertDiffToSQL converts a YAML schema diff to SQL migration statements
func (sc *SQLConverter) ConvertDiffToSQL(diff *SchemaDiff, oldSchema, newSchema *Schema) (upSQL, downSQL string, err error) {
	if diff == nil || !diff.HasChanges {
		return "", "", nil
	}

	// Apply user-defined type mappings from the target schema if present
	if newSchema != nil {
		if mappings := newSchema.TypeMappings.ForProvider(sc.databaseType); mappings != nil {
			sc.provider.SetTypeMappings(mappings)
		}
	}

	var upStatements []string
	var downStatements []string

	// Sort changes by type to ensure proper order
	sort.Slice(diff.Changes, func(i, j int) bool {
		return sc.getChangeOrder(diff.Changes[i].Type) < sc.getChangeOrder(diff.Changes[j].Type)
	})

	for _, change := range diff.Changes {
		upStmt, downStmt, err := sc.convertChangeToSQL(change, oldSchema, newSchema)
		if err != nil {
			return "", "", fmt.Errorf("failed to convert change %s: %w", change.Description, err)
		}

		// Handle UP statement — track the user's decision to mirror it for DOWN.
		includeDown := true
		reviewDown := false

		if upStmt != "" {
			if sc.shouldAddReviewComment(change) {
				// Prompt user for destructive operations (unless in silent mode or no prompt func)
				response, err := sc.handleDestructivePrompt(upStmt, change)
				if err != nil {
					return "", "", fmt.Errorf("failed to prompt for destructive operation: %w", err)
				}

				switch response {
				case PromptGenerate:
					// User chose to generate - add statement without any prefix
					upStatements = append(upStatements, upStmt)
				case PromptReview:
					// User chose to mark for review - mirror review tag on DOWN
					upStmt = sc.addReviewComment(upStmt)
					upStatements = append(upStatements, upStmt)
					reviewDown = true
				case PromptOmit:
					// User chose to omit - mirror by also omitting DOWN
					includeDown = false
				case PromptExit:
					// User chose to exit - return error to stop migration generation
					return "", "", fmt.Errorf("migration generation cancelled by user")
				case PromptGenerateAll:
					// User chose to generate all - enable auto-generate for this change type and add current statement
					if sc.generateAllTypes == nil {
						sc.generateAllTypes = make(map[ChangeType]bool)
					}
					sc.generateAllTypes[change.Type] = true
					upStatements = append(upStatements, upStmt)
				}
			} else {
				// Not a destructive operation or no review needed
				upStatements = append(upStatements, upStmt)
			}
		}

		// Handle DOWN statement — mirrors the user's decision for UP.
		if downStmt != "" && includeDown {
			if reviewDown {
				downStatements = append(downStatements, sc.addReviewComment(downStmt))
			} else {
				downStatements = append(downStatements, downStmt)
			}
		}
	}

	return strings.Join(upStatements, "\n\n"), strings.Join(downStatements, "\n\n"), nil
}

// getChangeOrder returns the order priority for different change types
func (sc *SQLConverter) getChangeOrder(changeType ChangeType) int {
	switch changeType {
	case ChangeTypeTableAdded:
		return 1
	case ChangeTypeFieldAdded:
		return 2
	case ChangeTypeFieldModified:
		return 3
	case ChangeTypeFieldRemoved:
		return 4
	case ChangeTypeTableRemoved:
		return 5
	default:
		return 999
	}
}

// handleDestructivePrompt determines the response for a destructive operation.
// It checks silent mode and generateAllTypes before delegating to the injected
// promptFunc. When no promptFunc is set, it defaults to PromptReview.
func (sc *SQLConverter) handleDestructivePrompt(sqlStmt string, change Change) (PromptResponse, error) {
	// If in silent mode, always use review comment (option 2)
	if sc.silent {
		return PromptReview, nil
	}

	// If "Generate All" mode is enabled for this change type, auto-generate without prompting
	if sc.generateAllTypes != nil && sc.generateAllTypes[change.Type] {
		return PromptGenerate, nil
	}

	// If no prompt function is set, default to review
	if sc.promptFunc == nil {
		return PromptReview, nil
	}

	return sc.promptFunc(sqlStmt, change)
}
