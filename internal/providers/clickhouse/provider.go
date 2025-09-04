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
package clickhouse

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
)

// Provider implements the Provider interface for ClickHouse
// ClickHouse has very different SQL syntax and concepts from other databases
type Provider struct{}

// New creates a new ClickHouse provider
func New() *Provider {
	return &Provider{}
}

// QuoteName quotes database identifiers for ClickHouse (backticks like MySQL)
func (p *Provider) QuoteName(name string) string {
	return fmt.Sprintf("`%s`", name)
}

// SupportsOperation checks if ClickHouse supports a specific operation
func (p *Provider) SupportsOperation(operation string) bool {
	switch operation {
	case "DROP_COLUMN", "ALTER_COLUMN":
		return true
	case "RENAME_TABLE", "RENAME_COLUMN":
		// ClickHouse supports RENAME TABLE but it's different syntax
		return false
	default:
		return false
	}
}

// ConvertFieldType converts YAML field type to ClickHouse-specific SQL type
func (p *Provider) ConvertFieldType(field *types.Field) string {
	switch field.Type {
	case "varchar":
		if field.Length > 0 {
			return fmt.Sprintf("FixedString(%d)", field.Length)
		}
		return "String"
	case "text":
		return "String"
	case "integer":
		return "Int32"
	case "bigint":
		return "Int64"
	case "serial":
		return "UInt64" // ClickHouse doesn't have auto-increment, typically use UInt64
	case "float":
		return "Float32"
	case "decimal":
		if field.Precision > 0 && field.Scale >= 0 {
			return fmt.Sprintf("Decimal(%d,%d)", field.Precision, field.Scale)
		}
		return "Decimal(18,2)"
	case "boolean":
		return "UInt8" // ClickHouse uses UInt8 for boolean (0/1)
	case "date":
		return "Date"
	case "time":
		return "DateTime" // ClickHouse doesn't have separate TIME type
	case "timestamp":
		return "DateTime"
	case "uuid":
		return "UUID"
	case "jsonb":
		return "String" // ClickHouse doesn't have native JSON, store as String
	default:
		return "String"
	}
}

// GetDefaultValue converts default value references to ClickHouse-specific values
func (p *Provider) GetDefaultValue(defaultRef string, defaults map[string]string) (string, error) {
	if value, exists := defaults[defaultRef]; exists {
		return value, nil
	}
	// Return as literal value if not found in mapping
	return fmt.Sprintf("'%s'", defaultRef), nil
}

// GenerateCreateIndex generates CREATE INDEX statement for ClickHouse
// ClickHouse doesn't have traditional indexes, but has skip indexes and primary keys
func (p *Provider) GenerateCreateIndex(index *types.Index, tableName string) string {
	// ClickHouse doesn't support traditional CREATE INDEX
	// This would need to be implemented as a skip index or handled during table creation
	var quotedFields []string
	for _, fieldName := range index.Fields {
		quotedFields = append(quotedFields, p.QuoteName(fieldName))
	}

	// Return a comment explaining this limitation
	return fmt.Sprintf("-- ClickHouse doesn't support CREATE INDEX. Consider using skip indexes or include in PRIMARY KEY during table creation for %s on %s (%s);",
		index.Name, tableName, strings.Join(quotedFields, ", "))
}

// GenerateDropIndex generates DROP INDEX statement for ClickHouse
func (p *Provider) GenerateDropIndex(indexName, tableName string) string {
	return fmt.Sprintf("-- ClickHouse doesn't support DROP INDEX for %s on %s;", indexName, tableName)
}

// GenerateDropTable generates DROP TABLE statement
func (p *Provider) GenerateDropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE %s;", p.QuoteName(tableName))
}

// GenerateAddColumn generates ALTER TABLE ADD COLUMN statement
func (p *Provider) GenerateAddColumn(tableName string, field *types.Field) string {
	fieldDef := fmt.Sprintf("%s %s", p.QuoteName(field.Name), p.ConvertFieldType(field))

	// ClickHouse ADD COLUMN syntax
	if field.Default != "" {
		fieldDef += fmt.Sprintf(" DEFAULT %s", field.Default)
	}

	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", p.QuoteName(tableName), fieldDef)
}

// GenerateDropColumn generates ALTER TABLE DROP COLUMN statement
func (p *Provider) GenerateDropColumn(tableName, columnName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", p.QuoteName(tableName), p.QuoteName(columnName))
}

// GenerateRenameTable generates RENAME TABLE statement
func (p *Provider) GenerateRenameTable(oldName, newName string) string {
	return fmt.Sprintf("RENAME TABLE %s TO %s;", p.QuoteName(oldName), p.QuoteName(newName))
}

// GenerateRenameColumn generates ALTER TABLE RENAME COLUMN statement
func (p *Provider) GenerateRenameColumn(tableName, oldName, newName string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;",
		p.QuoteName(tableName), p.QuoteName(oldName), p.QuoteName(newName))
}

// GenerateCreateTable generates CREATE TABLE statement for ClickHouse
func (p *Provider) GenerateCreateTable(schema *types.Schema, table *types.Table) (string, error) {
	var fieldDefs []string
	var primaryKeys []string

	for _, field := range table.Fields {
		fieldDef, err := p.convertField(&field)
		if err != nil {
			return "", fmt.Errorf("failed to convert field %s: %w", field.Name, err)
		}

		// Only add non-empty field definitions (skip many_to_many fields)
		if fieldDef != "" {
			fieldDefs = append(fieldDefs, fieldDef)
		}

		// Collect primary key fields
		if field.PrimaryKey {
			primaryKeys = append(primaryKeys, p.QuoteName(field.Name))
		}
	}

	// Build CREATE TABLE statement
	var sql strings.Builder
	sql.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", p.QuoteName(table.Name)))

	for i, def := range fieldDefs {
		sql.WriteString("    " + def)
		if i < len(fieldDefs)-1 {
			sql.WriteString(",")
		}
		sql.WriteString("\n")
	}

	sql.WriteString(")")

	// ClickHouse requires an ENGINE clause
	// Default to MergeTree with primary key if available, otherwise use Log
	if len(primaryKeys) > 0 {
		sql.WriteString(fmt.Sprintf("\nENGINE = MergeTree()\nPRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	} else {
		sql.WriteString("\nENGINE = Log()")
	}

	sql.WriteString(";")
	return sql.String(), nil
}

// convertField converts a YAML field definition to ClickHouse field definition
func (p *Provider) convertField(field *types.Field) (string, error) {
	// Skip many_to_many fields - they don't create actual columns
	if field.Type == "many_to_many" {
		return "", nil
	}

	var def strings.Builder
	def.WriteString(p.QuoteName(field.Name))
	def.WriteString(" ")

	// Convert field type
	sqlType := p.ConvertFieldType(field)
	def.WriteString(sqlType)

	// ClickHouse doesn't have NULL/NOT NULL in the same way as other databases
	// All columns are NOT NULL by default unless you use Nullable(Type)
	if field.IsNullable() && !field.PrimaryKey {
		// Reset and rebuild with Nullable wrapper
		def.Reset()
		def.WriteString(p.QuoteName(field.Name))
		def.WriteString(" Nullable(")
		def.WriteString(sqlType)
		def.WriteString(")")
	}

	// Handle defaults
	if field.AutoCreate && field.Type == "timestamp" {
		def.WriteString(" DEFAULT now()")
	} else if field.Default != "" {
		def.WriteString(" DEFAULT " + field.Default)
	}

	return def.String(), nil
}

// Placeholder implementations for remaining interface methods

func (p *Provider) GenerateAlterColumn(tableName string, oldField, newField *types.Field) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) GenerateForeignKeyConstraint(tableName, fieldName, referencedTable, onDelete string) string {
	// ClickHouse doesn't support foreign keys
	return fmt.Sprintf("-- ClickHouse doesn't support foreign key constraints for %s.%s -> %s;", tableName, fieldName, referencedTable)
}

func (p *Provider) GenerateDropForeignKeyConstraint(tableName, constraintName string) string {
	return fmt.Sprintf("-- ClickHouse doesn't support foreign key constraints for %s.%s;", tableName, constraintName)
}

func (p *Provider) GenerateJunctionTable(table1, table2 string, schema *types.Schema) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) InferForeignKeyType(referencedTable string, schema *types.Schema) string {
	return "UInt64" // Default to UInt64 for foreign keys in ClickHouse
}

func (p *Provider) GenerateIndexes(schema *types.Schema) string {
	var comments []string

	for _, table := range schema.Tables {
		// Generate comments for foreign key fields
		for _, field := range table.Fields {
			if field.Type == "foreign_key" {
				comment := fmt.Sprintf("-- ClickHouse doesn't support indexes. Consider using skip indexes for %s.%s;", table.Name, field.Name)
				comments = append(comments, comment)
			}
		}

		// Generate comments for table-level indexes
		for _, index := range table.Indexes {
			comment := p.GenerateCreateIndex(&index, table.Name)
			comments = append(comments, comment)
		}
	}

	if len(comments) == 0 {
		return ""
	}

	return strings.Join(comments, "\n")
}

func (p *Provider) GenerateForeignKeyConstraints(schema *types.Schema, junctionTables []types.Table) string {
	return "-- ClickHouse doesn't support foreign key constraints;"
}

// GetDatabaseSchema extracts schema information from a ClickHouse database
func (p *Provider) GetDatabaseSchema(connectionString string) (*types.Schema, error) {
	return nil, fmt.Errorf("ClickHouse schema extraction not implemented yet")
}
