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
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/ocomsoft/makemigrations/internal/providers"
)

// PromptResponse represents the user's response to a destructive operation prompt
type PromptResponse int

const (
	PromptGenerate PromptResponse = 1 // Generate SQL without any prefix
	PromptReview   PromptResponse = 2 // Mark for review (use review prefix)
	PromptOmit     PromptResponse = 3 // Don't generate statement (omit from output)
	PromptExit     PromptResponse = 4 // Exit migration generation
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

// SQLConverter converts YAML schema definitions to database-specific SQL DDL
type SQLConverter struct {
	provider               providers.Provider
	verbose                bool
	databaseType           DatabaseType
	safeTypeChanges        bool
	reviewCommentPrefix    string
	destructiveOperations  map[string]bool
	silent                 bool
	rejectionCommentPrefix string
}

// NewSQLConverter creates a new SQL converter
func NewSQLConverter(databaseType DatabaseType, verbose bool) *SQLConverter {
	provider, err := providers.NewProvider(databaseType)
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
	provider, err := providers.NewProvider(databaseType)
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

// NewSQLConverterWithConfig creates a new SQL converter with full configuration
func NewSQLConverterWithConfig(databaseType DatabaseType, verbose bool, safeTypeChanges bool, reviewPrefix string, destructiveOps []string, silent bool, rejectionPrefix string) *SQLConverter {
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

	provider, err := providers.NewProvider(databaseType)
	if err != nil {
		return nil
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
	}
}

// ConvertSchema converts a complete YAML schema to SQL DDL statements
func (sc *SQLConverter) ConvertSchema(schema *Schema) (string, error) {
	if schema == nil {
		return "", fmt.Errorf("schema cannot be nil")
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

// convertField converts a YAML field definition to SQL field definition
func (sc *SQLConverter) convertField(schema *Schema, _ string, field *Field) (string, string, error) {

	var def strings.Builder
	def.WriteString(sc.quoteName(field.Name))
	def.WriteString(" ")

	// Convert field type
	sqlType, err := sc.convertFieldTypeWithSchema(schema, field)
	if err != nil {
		return "", "", err
	}
	def.WriteString(sqlType)

	// Add NOT NULL constraint
	if !field.IsNullable() {
		def.WriteString(" NOT NULL")
	}

	// Handle auto_create and auto_update for timestamp fields (takes precedence over field.Default)
	if field.AutoCreate && field.Type == "timestamp" {
		switch sc.databaseType {
		case DatabasePostgreSQL:
			def.WriteString(" DEFAULT CURRENT_TIMESTAMP")
		case DatabaseMySQL:
			def.WriteString(" DEFAULT CURRENT_TIMESTAMP")
		case DatabaseSQLServer:
			def.WriteString(" DEFAULT GETDATE()")
		case DatabaseSQLite:
			def.WriteString(" DEFAULT CURRENT_TIMESTAMP")
		}
	} else if field.Default != "" {
		// Add default value only if auto_create is not set
		defaultValue, err := sc.convertDefaultValue(schema, field.Default)
		if err != nil {
			return "", "", fmt.Errorf("failed to convert default value for field %s: %w", field.Name, err)
		}
		def.WriteString(" DEFAULT " + defaultValue)
	}

	// Generate primary key constraint if needed
	var constraint string
	if field.PrimaryKey {
		constraint = fmt.Sprintf("PRIMARY KEY (%s)", sc.quoteName(field.Name))
	}

	return def.String(), constraint, nil
}

// convertFieldType converts YAML field type to database-specific SQL type
func (sc *SQLConverter) convertFieldType(field *Field) (string, error) {
	return sc.convertFieldTypeWithSchema(nil, field)
}

// convertFieldTypeWithSchema converts YAML field type to database-specific SQL type with schema context
func (sc *SQLConverter) convertFieldTypeWithSchema(schema *Schema, field *Field) (string, error) {
	switch field.Type {
	case "varchar":
		if field.Length <= 0 {
			return "", fmt.Errorf("varchar field must have a positive length")
		}
		return fmt.Sprintf("VARCHAR(%d)", field.Length), nil

	case "text":
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return SQLTypeText, nil
		case DatabaseMySQL:
			if field.Length > 0 {
				return fmt.Sprintf("VARCHAR(%d)", field.Length), nil
			}
			return SQLTypeText, nil
		case DatabaseSQLServer:
			return "NVARCHAR(MAX)", nil
		case DatabaseSQLite:
			return SQLTypeText, nil
		default:
			return SQLTypeText, nil
		}

	case "integer":
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return "INTEGER", nil
		case DatabaseMySQL:
			return "INT", nil
		case DatabaseSQLServer:
			return "INT", nil
		case DatabaseSQLite:
			return "INTEGER", nil
		default:
			return "INTEGER", nil
		}

	case "bigint":
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return "BIGINT", nil
		case DatabaseMySQL:
			return "BIGINT", nil
		case DatabaseSQLServer:
			return "BIGINT", nil
		case DatabaseSQLite:
			return "INTEGER", nil // SQLite uses INTEGER for all integer types
		default:
			return "BIGINT", nil
		}

	case "serial":
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return "SERIAL", nil
		case DatabaseMySQL:
			return "INT AUTO_INCREMENT", nil
		case DatabaseSQLServer:
			return "INT IDENTITY(1,1)", nil
		case DatabaseSQLite:
			return "INTEGER", nil
		default:
			return "SERIAL", nil
		}

	case FieldTypeFloat:
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return "REAL", nil
		case DatabaseMySQL:
			return SQLTypeFloat, nil
		case DatabaseSQLServer:
			return SQLTypeFloat, nil
		case DatabaseSQLite:
			return "REAL", nil
		default:
			return SQLTypeFloat, nil
		}

	case "decimal":
		if field.Precision <= 0 {
			return "", fmt.Errorf("decimal field must have a positive precision")
		}
		return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale), nil

	case FieldTypeBoolean:
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return SQLTypeBoolean, nil
		case DatabaseMySQL:
			return SQLTypeBoolean, nil
		case DatabaseSQLServer:
			return "BIT", nil
		case DatabaseSQLite:
			return "INTEGER", nil // SQLite uses INTEGER for boolean
		default:
			return SQLTypeBoolean, nil
		}

	case "date":
		return "DATE", nil

	case "timestamp":
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return "TIMESTAMPTZ", nil
		case DatabaseMySQL:
			return "TIMESTAMP", nil
		case DatabaseSQLServer:
			return "DATETIME2", nil
		case DatabaseSQLite:
			return "TIMESTAMP", nil
		default:
			return "TIMESTAMP", nil
		}

	case "time":
		return "TIME", nil

	case "foreign_key":
		// Look up the referenced table's primary key type
		if field.ForeignKey != nil {
			return sc.getForeignKeyType(schema, field.ForeignKey.Table)
		}
		// Fallback to integer if no foreign key info
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return "INTEGER", nil
		case DatabaseMySQL:
			return "INT", nil
		case DatabaseSQLServer:
			return "INT", nil
		case DatabaseSQLite:
			return "INTEGER", nil
		default:
			return "INTEGER", nil
		}

	case FieldTypeUUID:
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return SQLTypeUUID, nil
		case DatabaseMySQL:
			return SQLTypeChar36, nil
		case DatabaseSQLServer:
			return SQLTypeUniqueIdentifier, nil
		case DatabaseSQLite:
			return SQLTypeText, nil
		default:
			return SQLTypeUUID, nil
		}

	case "json", "jsonb":
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return "JSONB", nil
		case DatabaseMySQL:
			return "JSON", nil
		case DatabaseSQLServer:
			return "NVARCHAR(MAX)", nil
		case DatabaseSQLite:
			return SQLTypeText, nil
		default:
			return "JSONB", nil
		}

	default:
		return "", fmt.Errorf("unsupported field type: %s", field.Type)
	}
}

// convertDefaultValue converts YAML default value to database-specific SQL
func (sc *SQLConverter) convertDefaultValue(schema *Schema, defaultValue string) (string, error) {
	if schema == nil {
		return "", fmt.Errorf("schema is required for default value conversion")
	}

	// Get the appropriate defaults mapping for the database type
	var defaults map[string]string
	switch sc.databaseType {
	case DatabasePostgreSQL:
		defaults = schema.Defaults.PostgreSQL
	case DatabaseMySQL:
		defaults = schema.Defaults.MySQL
	case DatabaseSQLServer:
		defaults = schema.Defaults.SQLServer
	case DatabaseSQLite:
		defaults = schema.Defaults.SQLite
	default:
		return "", fmt.Errorf("unsupported database type: %s", sc.databaseType)
	}

	// Look up the default value in the mapping
	if sqlDefault, exists := defaults[defaultValue]; exists {
		return sqlDefault, nil
	}

	// If not found in mapping, return as literal value
	return fmt.Sprintf("'%s'", defaultValue), nil
}

// generateIndexes generates CREATE INDEX statements
func (sc *SQLConverter) generateIndexes(schema *Schema) string {
	return sc.provider.GenerateIndexes(schema)
}

// generateForeignKeyConstraints generates ALTER TABLE statements for foreign key constraints
func (sc *SQLConverter) generateForeignKeyConstraints(schema *Schema, junctionTables []Table) string {
	var constraints []string

	// Process main tables
	for _, table := range schema.Tables {
		tableConstraints := sc.generateTableForeignKeys(table.Name, table.Fields)
		constraints = append(constraints, tableConstraints...)
	}

	// Process junction tables
	for _, table := range junctionTables {
		tableConstraints := sc.generateTableForeignKeys(table.Name, table.Fields)
		constraints = append(constraints, tableConstraints...)
	}

	if len(constraints) == 0 {
		return ""
	}

	return strings.Join(constraints, "\n")
}

// translateOnDeleteAction translates Django-style on_delete values to SQL-standard ones
func (sc *SQLConverter) translateOnDeleteAction(action string) string {
	if action == "" {
		return "RESTRICT"
	}

	// PostgreSQL and MySQL don't support PROTECT, use RESTRICT instead
	// PROTECT in Django means "prevent deletion if referenced"
	// which is exactly what RESTRICT does in SQL
	if action == "PROTECT" {
		return "RESTRICT"
	}

	// Handle other Django-style values
	switch action {
	case "SET_NULL":
		return "SET NULL"
	case "SET_DEFAULT":
		return "SET DEFAULT"
	default:
		// CASCADE and RESTRICT are already SQL-standard
		return action
	}
}

// generateTableForeignKeys generates foreign key constraints for a table
func (sc *SQLConverter) generateTableForeignKeys(tableName string, fields []Field) []string {
	var constraints []string

	for _, field := range fields {
		if field.Type == "foreign_key" && field.ForeignKey != nil {
			// Skip namespaced tables for now
			if strings.Contains(field.ForeignKey.Table, ".") {
				continue
			}

			constraintName := fmt.Sprintf("fk_%s_%s", tableName, field.Name)
			onDelete := sc.translateOnDeleteAction(field.ForeignKey.OnDelete)

			// Convert table name to proper format
			referencedTableName := sc.normalizeTableName(field.ForeignKey.Table)

			constraint := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (id) ON DELETE %s;",
				sc.quoteName(tableName),
				sc.quoteName(constraintName),
				sc.quoteName(field.Name),
				sc.quoteName(referencedTableName),
				onDelete)

			constraints = append(constraints, constraint)
		}
	}

	return constraints
}

// getForeignKeyType determines the appropriate SQL type for a foreign key field
// by looking up the referenced table's primary key type
func (sc *SQLConverter) getForeignKeyType(schema *Schema, referencedTableName string) (string, error) {
	if sc.verbose {
		fmt.Printf("DEBUG: getForeignKeyType called with referencedTableName=%s\n", referencedTableName)
		if schema == nil {
			fmt.Printf("DEBUG: schema is nil!\n")
		} else {
			fmt.Printf("DEBUG: schema has %d tables\n", len(schema.Tables))
		}
	}

	// Skip namespaced table references (like "auth.User" or "filesystem.FileMetaData")
	if strings.Contains(referencedTableName, ".") {
		// For namespaced tables, assume UUID primary key as default
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return SQLTypeUUID, nil
		case DatabaseMySQL:
			return SQLTypeChar36, nil
		case DatabaseSQLServer:
			return SQLTypeUniqueIdentifier, nil
		case DatabaseSQLite:
			return SQLTypeText, nil
		default:
			return SQLTypeUUID, nil
		}
	}

	// Find the referenced table
	var referencedTable *Table
	for _, table := range schema.Tables {
		if strings.EqualFold(table.Name, referencedTableName) {
			referencedTable = &table
			break
		}
		// Also try converting CamelCase to snake_case
		if strings.EqualFold(table.Name, sc.camelToSnake(referencedTableName)) {
			referencedTable = &table
			break
		}
		// And try converting snake_case to CamelCase
		if strings.EqualFold(sc.snakeToCamel(table.Name), referencedTableName) {
			referencedTable = &table
			break
		}
	}

	if referencedTable == nil {
		// Table not found, assume UUID primary key as default
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return SQLTypeUUID, nil
		case DatabaseMySQL:
			return SQLTypeChar36, nil
		case DatabaseSQLServer:
			return SQLTypeUniqueIdentifier, nil
		case DatabaseSQLite:
			return SQLTypeText, nil
		default:
			return SQLTypeUUID, nil
		}
	}

	// Look for explicit primary key field
	for _, field := range referencedTable.Fields {
		if field.PrimaryKey {
			// Found explicit primary key, return appropriate foreign key type
			return sc.getForeignKeyTypeFromPrimaryKey(&field)
		}
	}

	// Look for "id" field
	for _, field := range referencedTable.Fields {
		if strings.EqualFold(field.Name, "id") {
			// Found id field, return appropriate foreign key type
			return sc.getForeignKeyTypeFromPrimaryKey(&field)
		}
	}

	// No explicit primary key or id field found - assume UUID with default generator
	switch sc.databaseType {
	case DatabasePostgreSQL:
		return SQLTypeUUID, nil
	case DatabaseMySQL:
		return SQLTypeChar36, nil
	case DatabaseSQLServer:
		return SQLTypeUniqueIdentifier, nil
	case DatabaseSQLite:
		return SQLTypeText, nil
	default:
		return SQLTypeUUID, nil
	}
}

// getForeignKeyTypeFromPrimaryKey returns the appropriate foreign key type for a primary key field
func (sc *SQLConverter) getForeignKeyTypeFromPrimaryKey(pkField *Field) (string, error) {
	switch pkField.Type {
	case "serial":
		// Foreign keys to SERIAL fields should be INTEGER
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return "INTEGER", nil
		case DatabaseMySQL:
			return "INT", nil
		case DatabaseSQLServer:
			return "INT", nil
		case DatabaseSQLite:
			return "INTEGER", nil
		default:
			return "INTEGER", nil
		}
	case FieldTypeUUID:
		// Foreign keys to UUID fields should be UUID
		switch sc.databaseType {
		case DatabasePostgreSQL:
			return SQLTypeUUID, nil
		case DatabaseMySQL:
			return SQLTypeChar36, nil
		case DatabaseSQLServer:
			return SQLTypeUniqueIdentifier, nil
		case DatabaseSQLite:
			return SQLTypeText, nil
		default:
			return SQLTypeUUID, nil
		}
	case "integer", "bigint":
		// Foreign keys to integer fields should match the same type
		return sc.convertFieldTypeWithSchema(nil, pkField)
	default:
		// For other types, use the same type as the primary key
		return sc.convertFieldTypeWithSchema(nil, pkField)
	}
}

// camelToSnake converts CamelCase to snake_case
func (sc *SQLConverter) camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// snakeToCamel converts snake_case to CamelCase
func (sc *SQLConverter) snakeToCamel(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == '_' {
			capitalizeNext = true
		} else {
			if capitalizeNext && 'a' <= r && r <= 'z' {
				result.WriteRune(r - 32) // Convert to uppercase
				capitalizeNext = false
			} else {
				result.WriteRune(r)
				capitalizeNext = false
			}
		}
	}
	return result.String()
}

// normalizeTableName converts table references to the proper format for SQL
// It converts CamelCase table names to snake_case
func (sc *SQLConverter) normalizeTableName(tableName string) string {
	// Skip namespaced tables (e.g., "filesystem.FileMetaData")
	if strings.Contains(tableName, ".") {
		return tableName
	}

	// Convert CamelCase to snake_case
	return sc.camelToSnake(tableName)
}

// quoteName quotes database identifiers if needed
func (sc *SQLConverter) quoteName(name string) string {
	return sc.provider.QuoteName(name)
}

// ConvertDiffToSQL converts a YAML schema diff to SQL migration statements
func (sc *SQLConverter) ConvertDiffToSQL(diff *SchemaDiff, oldSchema, newSchema *Schema) (upSQL, downSQL string, err error) {
	if diff == nil || !diff.HasChanges {
		return "", "", nil
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

		// Handle UP statement
		if upStmt != "" {
			if sc.shouldAddReviewComment(change) {
				// Prompt user for destructive operations (unless in silent mode)
				response, err := sc.promptForDestructiveOperation(upStmt, change)
				if err != nil {
					return "", "", fmt.Errorf("failed to prompt for destructive operation: %w", err)
				}

				switch response {
				case PromptGenerate:
					// User chose to generate - add statement without any prefix
					upStatements = append(upStatements, upStmt)
				case PromptReview:
					// User chose to mark for review - apply review comment
					upStmt = sc.addReviewComment(upStmt)
					upStatements = append(upStatements, upStmt)
				case PromptOmit:
					// User chose to omit - don't add to statements
					// (statement is discarded)
				case PromptExit:
					// User chose to exit - return error to stop migration generation
					return "", "", fmt.Errorf("migration generation cancelled by user")
				}
			} else {
				// Not a destructive operation or no review needed
				upStatements = append(upStatements, upStmt)
			}
		}

		// Handle DOWN statement
		if downStmt != "" {
			if sc.shouldAddReviewComment(change) {
				// For down statements, prompt with different context
				if !sc.silent {
					fmt.Printf("\n--- DOWN statement for the same operation ---\n")
				}
				response, err := sc.promptForDestructiveOperation(downStmt, change)
				if err != nil {
					return "", "", fmt.Errorf("failed to prompt for destructive operation (down): %w", err)
				}

				switch response {
				case PromptGenerate:
					// User chose to generate - add statement without any prefix
					downStatements = append(downStatements, downStmt)
				case PromptReview:
					// User chose to mark for review - apply review comment
					downStmt = sc.addReviewComment(downStmt)
					downStatements = append(downStatements, downStmt)
				case PromptOmit:
					// User chose to omit - don't add to statements
					// (statement is discarded)
				case PromptExit:
					// User chose to exit - return error to stop migration generation
					return "", "", fmt.Errorf("migration generation cancelled by user")
				}
			} else {
				// Not a destructive operation or no review needed
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

// convertChangeToSQL converts a single change to SQL statements
func (sc *SQLConverter) convertChangeToSQL(change Change, oldSchema, newSchema *Schema) (upSQL, downSQL string, err error) {
	switch change.Type {
	case ChangeTypeTableAdded:
		if newTable, ok := change.NewValue.(Table); ok {
			upSQL, err = sc.ConvertTable(newSchema, &newTable)
			if err != nil {
				return "", "", err
			}
			downSQL = fmt.Sprintf("DROP TABLE %s;", sc.quoteName(change.TableName))
		}

	case ChangeTypeTableRemoved:
		upSQL = fmt.Sprintf("DROP TABLE %s;", sc.quoteName(change.TableName))
		if oldTable, ok := change.OldValue.(Table); ok {
			downSQL, err = sc.ConvertTable(oldSchema, &oldTable)
			if err != nil {
				return "", "", err
			}
		}

	case ChangeTypeFieldAdded:
		if newField, ok := change.NewValue.(Field); ok {
			var fieldDef string
			fieldDef, _, err = sc.convertField(newSchema, change.TableName, &newField)
			if err != nil {
				return "", "", err
			}
			if fieldDef != "" { // Skip foreign_key fields that don't create columns
				upSQL = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", sc.quoteName(change.TableName), fieldDef)
				downSQL = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", sc.quoteName(change.TableName), sc.quoteName(change.FieldName))
			}
		}

	case ChangeTypeFieldRemoved:
		upSQL = fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", sc.quoteName(change.TableName), sc.quoteName(change.FieldName))
		if oldField, ok := change.OldValue.(Field); ok {
			var fieldDef string
			fieldDef, _, err = sc.convertField(oldSchema, change.TableName, &oldField)
			if err != nil {
				return "", "", err
			}
			if fieldDef != "" { // Skip foreign_key fields that don't create columns
				downSQL = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", sc.quoteName(change.TableName), fieldDef)
			}
		}

	case ChangeTypeFieldModified:
		// For field modifications, we need to look up the full field information
		// since OldValue/NewValue only contain the changed property values

		// Get the field from both schemas
		oldTable := oldSchema.GetTableByName(change.TableName)
		newTable := newSchema.GetTableByName(change.TableName)

		if oldTable == nil || newTable == nil {
			return "", "", fmt.Errorf("table not found: %s", change.TableName)
		}

		oldField := oldTable.GetFieldByName(change.FieldName)
		newField := newTable.GetFieldByName(change.FieldName)

		if oldField == nil || newField == nil {
			return "", "", fmt.Errorf("field not found: %s.%s", change.TableName, change.FieldName)
		}

		upSQL, downSQL, err = sc.generateFieldModificationSQL(change.TableName, oldField, newField, oldSchema, newSchema)
		if err != nil {
			return "", "", err
		}

	case ChangeTypeTableRenamed:
		// Table renaming - need old and new table names
		if oldTableName, ok := change.OldValue.(string); ok {
			if newTableName, ok := change.NewValue.(string); ok {
				upSQL = fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", sc.quoteName(oldTableName), sc.quoteName(newTableName))
				downSQL = fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", sc.quoteName(newTableName), sc.quoteName(oldTableName))
			}
		}

	case ChangeTypeFieldRenamed:
		// Field renaming - need old and new field names
		if oldFieldName, ok := change.OldValue.(string); ok {
			if newFieldName, ok := change.NewValue.(string); ok {
				quotedTableName := sc.quoteName(change.TableName)
				switch sc.databaseType {
				case DatabasePostgreSQL:
					upSQL = fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", quotedTableName, sc.quoteName(oldFieldName), sc.quoteName(newFieldName))
					downSQL = fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", quotedTableName, sc.quoteName(newFieldName), sc.quoteName(oldFieldName))
				case DatabaseMySQL:
					// MySQL requires the full column definition for CHANGE
					// We'll need to look up the field definition
					table := newSchema.GetTableByName(change.TableName)
					if table != nil {
						field := table.GetFieldByName(newFieldName)
						if field != nil {
							fieldDef, _, err := sc.convertField(newSchema, change.TableName, field)
							if err == nil && fieldDef != "" {
								// Extract type part for MySQL CHANGE syntax
								typePart := strings.TrimSpace(strings.TrimPrefix(fieldDef, sc.quoteName(field.Name)))
								upSQL = fmt.Sprintf("ALTER TABLE %s CHANGE %s %s %s;", quotedTableName, sc.quoteName(oldFieldName), sc.quoteName(newFieldName), typePart)
								downSQL = fmt.Sprintf("ALTER TABLE %s CHANGE %s %s %s;", quotedTableName, sc.quoteName(newFieldName), sc.quoteName(oldFieldName), typePart)
							}
						}
					}
				case DatabaseSQLServer:
					// SQL Server uses sp_rename
					upSQL = fmt.Sprintf("EXEC sp_rename '%s.%s', '%s', 'COLUMN';", change.TableName, oldFieldName, newFieldName)
					downSQL = fmt.Sprintf("EXEC sp_rename '%s.%s', '%s', 'COLUMN';", change.TableName, newFieldName, oldFieldName)
				case DatabaseSQLite:
					// SQLite doesn't support column renaming directly
					upSQL = fmt.Sprintf("-- SQLite doesn't support ALTER TABLE RENAME COLUMN. Manual table recreation required for %s.%s -> %s", change.TableName, oldFieldName, newFieldName)
					downSQL = fmt.Sprintf("-- SQLite doesn't support ALTER TABLE RENAME COLUMN. Manual table recreation required for %s.%s -> %s", change.TableName, newFieldName, oldFieldName)
				}
			}
		}

	case ChangeTypeIndexAdded:
		// Index addition - handle both new Index struct and legacy string format
		if index, ok := change.NewValue.(Index); ok {
			upSQL = sc.provider.GenerateCreateIndex(&index, change.TableName)
			downSQL = sc.provider.GenerateDropIndex(index.Name, change.TableName)
		} else if indexName, ok := change.NewValue.(string); ok {
			// Legacy format: create a simple index from the field name
			index := Index{
				Name:   indexName,
				Fields: []string{change.FieldName},
				Unique: false,
			}
			upSQL = sc.provider.GenerateCreateIndex(&index, change.TableName)
			downSQL = sc.provider.GenerateDropIndex(index.Name, change.TableName)
		}

	case ChangeTypeIndexRemoved:
		// Index removal - handle both new Index struct and legacy string format
		if index, ok := change.OldValue.(Index); ok {
			upSQL = sc.provider.GenerateDropIndex(index.Name, change.TableName)
			downSQL = sc.provider.GenerateCreateIndex(&index, change.TableName)
		} else if indexName, ok := change.OldValue.(string); ok {
			// Legacy format: create a simple index from the field name
			index := Index{
				Name:   indexName,
				Fields: []string{change.FieldName},
				Unique: false,
			}
			upSQL = sc.provider.GenerateDropIndex(index.Name, change.TableName)
			downSQL = sc.provider.GenerateCreateIndex(&index, change.TableName)
		}
	}

	return upSQL, downSQL, nil
}

// generateFieldModificationSQL generates SQL for field modifications
func (sc *SQLConverter) generateFieldModificationSQL(tableName string, oldField, newField *Field, oldSchema, newSchema *Schema) (upSQL, downSQL string, err error) {
	var upStatements []string
	var downStatements []string

	quotedTableName := sc.quoteName(tableName)
	quotedFieldName := sc.quoteName(oldField.Name)

	// Handle type changes or other modifications that require type/definition changes
	typeChanged := oldField.Type != newField.Type
	lengthChanged := oldField.Length != newField.Length
	precisionChanged := oldField.Precision != newField.Precision
	scaleChanged := oldField.Scale != newField.Scale

	if typeChanged || lengthChanged || precisionChanged || scaleChanged {

		// Get the new field definition
		newFieldDef, _, err := sc.convertField(newSchema, tableName, newField)
		if err != nil {
			return "", "", err
		}

		oldFieldDef, _, err := sc.convertField(oldSchema, tableName, oldField)
		if err != nil {
			return "", "", err
		}

		// Skip if either field is foreign_key that doesn't create a direct column
		if newFieldDef == "" || oldFieldDef == "" {
			return "", "", nil
		}

		// Extract just the type part (remove the field name)
		newTypePart := strings.TrimSpace(strings.TrimPrefix(newFieldDef, sc.quoteName(newField.Name)))
		oldTypePart := strings.TrimSpace(strings.TrimPrefix(oldFieldDef, sc.quoteName(oldField.Name)))

		// Use safe type changes if enabled and type actually changed
		if sc.safeTypeChanges && typeChanged {
			upSQL, downSQL, err := sc.generateSafeTypeChangeSQL(tableName, oldField, newField, oldSchema, newSchema)
			if err != nil {
				return "", "", err
			}
			upStatements = append(upStatements, upSQL)
			downStatements = append(downStatements, downSQL)
		} else {
			// Standard type change approach
			switch sc.databaseType {
			case DatabasePostgreSQL:
				upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", quotedTableName, quotedFieldName, sc.extractTypeFromDefinition(newTypePart)))
				downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", quotedTableName, quotedFieldName, sc.extractTypeFromDefinition(oldTypePart)))
			case DatabaseMySQL:
				upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;", quotedTableName, newFieldDef))
				downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;", quotedTableName, oldFieldDef))
			case DatabaseSQLServer:
				upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s;", quotedTableName, newFieldDef))
				downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s;", quotedTableName, oldFieldDef))
			case DatabaseSQLite:
				// SQLite doesn't support ALTER COLUMN, requires recreate table approach
				upStatements = append(upStatements, fmt.Sprintf("-- SQLite doesn't support ALTER COLUMN TYPE. Manual table recreation required for %s.%s", tableName, oldField.Name))
				downStatements = append(downStatements, fmt.Sprintf("-- SQLite doesn't support ALTER COLUMN TYPE. Manual table recreation required for %s.%s", tableName, oldField.Name))
			}
		}
	}

	// Handle nullable changes (separate from type changes for PostgreSQL)
	if sc.databaseType == DatabasePostgreSQL {
		// Helper function to get nullable bool value
		oldNullable := oldField.Nullable == nil || *oldField.Nullable // default to true if nil
		newNullable := newField.Nullable == nil || *newField.Nullable // default to true if nil

		if oldNullable != newNullable {
			if newNullable {
				upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", quotedTableName, quotedFieldName))
				downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", quotedTableName, quotedFieldName))
			} else {
				upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", quotedTableName, quotedFieldName))
				downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", quotedTableName, quotedFieldName))
			}
		}
	}

	// Handle default value changes (separate for PostgreSQL)
	if sc.databaseType == DatabasePostgreSQL {
		oldDefault := sc.getFieldDefaultValue(oldField, oldSchema)
		newDefault := sc.getFieldDefaultValue(newField, newSchema)

		if oldDefault != newDefault {
			if newDefault == "" {
				upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;", quotedTableName, quotedFieldName))
			} else {
				upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;", quotedTableName, quotedFieldName, newDefault))
			}

			if oldDefault == "" {
				downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;", quotedTableName, quotedFieldName))
			} else {
				downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;", quotedTableName, quotedFieldName, oldDefault))
			}
		}
	}

	return strings.Join(upStatements, "\n"), strings.Join(downStatements, "\n"), nil
}

// extractTypeFromDefinition extracts the type part from a field definition
func (sc *SQLConverter) extractTypeFromDefinition(def string) string {
	// Remove leading/trailing spaces and extract the type part before any constraints
	def = strings.TrimSpace(def)

	// Find the type (everything before DEFAULT, NOT NULL, etc.)
	parts := strings.Fields(def)
	if len(parts) == 0 {
		return def
	}

	// The type is typically the first part, but handle parentheses for types like VARCHAR(255)
	typeStr := parts[0]
	if len(parts) > 1 && strings.Contains(parts[1], "(") {
		// Handle cases like "VARCHAR (255)" or compound types
		for i := 1; i < len(parts); i++ {
			part := parts[i]
			typeStr += " " + part
			if strings.Contains(part, ")") {
				break
			}
		}
	}

	return typeStr
}

// getFieldDefaultValue gets the default value for a field from the schema
func (sc *SQLConverter) getFieldDefaultValue(field *Field, schema *Schema) string {
	if field.Default == "" {
		return ""
	}

	// Use the existing convertDefaultValue method
	sqlValue, err := sc.convertDefaultValue(schema, field.Default)
	if err != nil {
		return ""
	}

	return sqlValue
}

// generateSafeTypeChangeSQL generates SQL for safe column type changes using temporary columns
func (sc *SQLConverter) generateSafeTypeChangeSQL(tableName string, oldField, newField *Field, oldSchema, newSchema *Schema) (upSQL, downSQL string, err error) {
	quotedTableName := sc.quoteName(tableName)
	quotedFieldName := sc.quoteName(oldField.Name)
	tempFieldName := fmt.Sprintf("%s_temp_migration", oldField.Name)
	quotedTempFieldName := sc.quoteName(tempFieldName)

	// Get field definitions
	newFieldDef, _, err := sc.convertField(newSchema, tableName, newField)
	if err != nil {
		return "", "", err
	}

	oldFieldDef, _, err := sc.convertField(oldSchema, tableName, oldField)
	if err != nil {
		return "", "", err
	}

	// Skip if either field is foreign_key that doesn't create a direct column
	if newFieldDef == "" || oldFieldDef == "" {
		return "", "", nil
	}

	// Create temporary field definition by replacing field name
	tempFieldDef := strings.Replace(newFieldDef, sc.quoteName(newField.Name), quotedTempFieldName, 1)

	var upStatements []string
	var downStatements []string

	switch sc.databaseType {
	case DatabasePostgreSQL:
		// Step 1: Add temporary column with new type
		upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", quotedTableName, tempFieldDef))

		// Step 2: Copy data from old column to new column with type conversion
		upStatements = append(upStatements, fmt.Sprintf("UPDATE %s SET %s = %s::%s;",
			quotedTableName, quotedTempFieldName, quotedFieldName, sc.getPostgreSQLTypeName(newField)))

		// Step 3: Drop old column
		upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", quotedTableName, quotedFieldName))

		// Step 4: Rename temporary column to original name
		upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;",
			quotedTableName, quotedTempFieldName, quotedFieldName))

		// Down migration (reverse the process)
		downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", quotedTableName, tempFieldDef))
		downStatements = append(downStatements, fmt.Sprintf("UPDATE %s SET %s = %s::%s;",
			quotedTableName, quotedTempFieldName, quotedFieldName, sc.getPostgreSQLTypeName(oldField)))
		downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", quotedTableName, quotedFieldName))
		downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;",
			quotedTableName, quotedTempFieldName, quotedFieldName))

	case DatabaseMySQL:
		// MySQL approach: use temporary column with explicit type conversion
		upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", quotedTableName, tempFieldDef))
		upStatements = append(upStatements, fmt.Sprintf("UPDATE %s SET %s = CAST(%s AS %s);",
			quotedTableName, quotedTempFieldName, quotedFieldName, sc.getMySQLTypeName(newField)))
		upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", quotedTableName, quotedFieldName))
		upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s CHANGE %s %s %s;",
			quotedTableName, quotedTempFieldName, quotedFieldName, strings.TrimSpace(strings.TrimPrefix(newFieldDef, sc.quoteName(newField.Name)))))

		// Down migration
		tempOldFieldDef := strings.Replace(oldFieldDef, sc.quoteName(oldField.Name), quotedTempFieldName, 1)
		downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", quotedTableName, tempOldFieldDef))
		downStatements = append(downStatements, fmt.Sprintf("UPDATE %s SET %s = CAST(%s AS %s);",
			quotedTableName, quotedTempFieldName, quotedFieldName, sc.getMySQLTypeName(oldField)))
		downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", quotedTableName, quotedFieldName))
		downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s CHANGE %s %s %s;",
			quotedTableName, quotedTempFieldName, quotedFieldName, strings.TrimSpace(strings.TrimPrefix(oldFieldDef, sc.quoteName(oldField.Name)))))

	case DatabaseSQLServer:
		// SQL Server approach
		upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s ADD %s;", quotedTableName, tempFieldDef))
		upStatements = append(upStatements, fmt.Sprintf("UPDATE %s SET %s = CAST(%s AS %s);",
			quotedTableName, quotedTempFieldName, quotedFieldName, sc.getSQLServerTypeName(newField)))
		upStatements = append(upStatements, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", quotedTableName, quotedFieldName))
		upStatements = append(upStatements, fmt.Sprintf("EXEC sp_rename '%s.%s', '%s', 'COLUMN';", tableName, tempFieldName, oldField.Name))

		// Down migration
		tempOldFieldDef := strings.Replace(oldFieldDef, sc.quoteName(oldField.Name), quotedTempFieldName, 1)
		downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s ADD %s;", quotedTableName, tempOldFieldDef))
		downStatements = append(downStatements, fmt.Sprintf("UPDATE %s SET %s = CAST(%s AS %s);",
			quotedTableName, quotedTempFieldName, quotedFieldName, sc.getSQLServerTypeName(oldField)))
		downStatements = append(downStatements, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", quotedTableName, quotedFieldName))
		downStatements = append(downStatements, fmt.Sprintf("EXEC sp_rename '%s.%s', '%s', 'COLUMN';", tableName, tempFieldName, oldField.Name))

	case DatabaseSQLite:
		// SQLite doesn't support safe column type changes either, fallback to comment
		upStatements = append(upStatements, fmt.Sprintf("-- SQLite doesn't support safe column type changes. Manual table recreation required for %s.%s", tableName, oldField.Name))
		downStatements = append(downStatements, fmt.Sprintf("-- SQLite doesn't support safe column type changes. Manual table recreation required for %s.%s", tableName, oldField.Name))
	}

	return strings.Join(upStatements, "\n"), strings.Join(downStatements, "\n"), nil
}

// getPostgreSQLTypeName returns the PostgreSQL type name for a field
func (sc *SQLConverter) getPostgreSQLTypeName(field *Field) string {
	switch field.Type {
	case "varchar":
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "VARCHAR"
	case "text":
		return "TEXT"
	case "integer":
		return "INTEGER"
	case "bigint":
		return "BIGINT"
	case "decimal":
		if field.Precision > 0 && field.Scale > 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DECIMAL"
	case FieldTypeFloat:
		return "REAL"
	case FieldTypeBoolean:
		return "BOOLEAN"
	case "timestamp":
		return "TIMESTAMPTZ"
	case "date":
		return "DATE"
	case "time":
		return "TIME"
	case FieldTypeUUID:
		return "UUID"
	case "json", "jsonb":
		return "JSONB"
	default:
		return strings.ToUpper(field.Type)
	}
}

// getMySQLTypeName returns the MySQL type name for a field
func (sc *SQLConverter) getMySQLTypeName(field *Field) string {
	switch field.Type {
	case "varchar":
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "VARCHAR(255)"
	case "text":
		return "TEXT"
	case "integer":
		return "INT"
	case "bigint":
		return "BIGINT"
	case "decimal":
		if field.Precision > 0 && field.Scale > 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DECIMAL"
	case FieldTypeFloat:
		return "FLOAT"
	case FieldTypeBoolean:
		return "TINYINT(1)"
	case "timestamp":
		return "TIMESTAMP"
	case "date":
		return "DATE"
	case "time":
		return "TIME"
	case FieldTypeUUID:
		return "CHAR(36)"
	case "json", "jsonb":
		return "JSON"
	default:
		return strings.ToUpper(field.Type)
	}
}

// getSQLServerTypeName returns the SQL Server type name for a field
func (sc *SQLConverter) getSQLServerTypeName(field *Field) string {
	switch field.Type {
	case "varchar":
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "VARCHAR(255)"
	case "text":
		return "NVARCHAR(MAX)"
	case "integer":
		return "INT"
	case "bigint":
		return "BIGINT"
	case "decimal":
		if field.Precision > 0 && field.Scale > 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DECIMAL"
	case FieldTypeFloat:
		return "FLOAT"
	case FieldTypeBoolean:
		return "BIT"
	case "timestamp":
		return "DATETIME2"
	case "date":
		return "DATE"
	case "time":
		return "TIME"
	case FieldTypeUUID:
		return "UNIQUEIDENTIFIER"
	case "json", "jsonb":
		return "NVARCHAR(MAX)"
	default:
		return strings.ToUpper(field.Type)
	}
}

// shouldAddReviewComment determines if a review comment should be added for a change
func (sc *SQLConverter) shouldAddReviewComment(change Change) bool {
	// First check if this is actually a destructive operation
	if !isDestructiveOperation(change.Type) {
		return false
	}

	// If no prefix is set (empty string), never add review comments
	if sc.reviewCommentPrefix == "" {
		return false
	}

	// Check if this change type should get a review comment
	return sc.destructiveOperations[string(change.Type)]
}

// promptForDestructiveOperation prompts the user with options for handling a destructive operation
func (sc *SQLConverter) promptForDestructiveOperation(sqlStmt string, change Change) (PromptResponse, error) {
	// If in silent mode, always use review comment (option 2)
	if sc.silent {
		return PromptReview, nil
	}

	// Create colored output functions
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	white := color.New(color.FgWhite).SprintFunc()

	// Show the destructive operation with RED highlighting
	fmt.Printf("\n%s\n", red("⚠️  DESTRUCTIVE OPERATION DETECTED"))
	fmt.Printf("%s %s\n", red("Operation:"), change.Type)
	fmt.Printf("%s\n%s\n", red("SQL Statement:"), white(sqlStmt))

	// Show the 4 options
	fmt.Printf("\n%s\n", yellow("Choose an action:"))
	fmt.Printf("  %s - Generate SQL (execute the destructive operation)\n", cyan("1"))
	fmt.Printf("  %s - Mark for review (comment out with review prefix)\n", cyan("2"))
	fmt.Printf("  %s - Don't generate statement (omit completely)\n", cyan("3"))
	fmt.Printf("  %s - Exit migration generation\n", cyan("4"))

	// Prompt for input
	fmt.Printf("\n%s ", red("Enter choice (1-4):"))

	// Read user input
	reader := bufio.NewReader(os.Stdin)
	for {
		response, err := reader.ReadString('\n')
		if err != nil {
			return PromptExit, fmt.Errorf("failed to read user input: %w", err)
		}

		// Parse response
		choice := strings.TrimSpace(response)
		switch choice {
		case "1":
			return PromptGenerate, nil
		case "2":
			return PromptReview, nil
		case "3":
			return PromptOmit, nil
		case "4":
			return PromptExit, nil
		default:
			fmt.Printf("%s Please enter 1, 2, 3, or 4: ", red("Invalid choice."))
		}
	}
}

// addReviewComment adds a review comment to comment out the SQL statement
func (sc *SQLConverter) addReviewComment(sqlStmt string) string {
	// If no prefix is set, return unchanged
	if sc.reviewCommentPrefix == "" {
		return sqlStmt
	}

	// Split into lines and add comment prefix to each line to comment them out
	lines := strings.Split(sqlStmt, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			// Comment out this line by prefixing it with the review comment
			lines[i] = sc.reviewCommentPrefix + line
		}
	}

	return strings.Join(lines, "\n")
}
