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
package mysql

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
	"github.com/ocomsoft/makemigrations/internal/utils"
)

// Provider implements the Provider interface for MySQL
type Provider struct{}

// New creates a new MySQL provider
func New() *Provider {
	return &Provider{}
}

// QuoteName quotes database identifiers for MySQL
func (p *Provider) QuoteName(name string) string {
	return fmt.Sprintf("`%s`", name)
}

// SupportsOperation checks if MySQL supports a specific operation
func (p *Provider) SupportsOperation(operation string) bool {
	switch operation {
	case "RENAME_TABLE", "DROP_COLUMN", "ALTER_COLUMN":
		return true
	case "RENAME_COLUMN":
		return true // MySQL uses CHANGE syntax
	default:
		return false
	}
}

// ConvertFieldType converts YAML field type to MySQL-specific SQL type
func (p *Provider) ConvertFieldType(field *types.Field) string {
	switch field.Type {
	case "varchar":
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "TEXT"
	case "text":
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "TEXT"
	case "integer":
		return "INT"
	case "bigint":
		return "BIGINT"
	case "serial":
		return "INT AUTO_INCREMENT"
	case "float":
		return "FLOAT"
	case "decimal":
		if field.Precision > 0 && field.Scale >= 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DECIMAL"
	case "boolean":
		return "TINYINT(1)"
	case "date":
		return "DATE"
	case "time":
		return "TIME"
	case "timestamp":
		return "TIMESTAMP"
	case "uuid":
		return "CHAR(36)"
	case "json", "jsonb":
		return "JSON"
	default:
		return "TEXT"
	}
}

// GetDefaultValue converts default value references to MySQL-specific values
func (p *Provider) GetDefaultValue(defaultRef string, defaults map[string]string) (string, error) {
	if value, exists := defaults[defaultRef]; exists {
		return value, nil
	}
	// Return as literal value if not found in mapping
	return fmt.Sprintf("'%s'", defaultRef), nil
}

// GenerateCreateIndex generates CREATE INDEX statement for MySQL
func (p *Provider) GenerateCreateIndex(index *types.Index, tableName string) string {
	var quotedFields []string
	for _, fieldName := range index.Fields {
		quotedFields = append(quotedFields, p.QuoteName(fieldName))
	}

	indexType := "INDEX"
	if index.Unique {
		indexType = "UNIQUE INDEX"
	}

	return fmt.Sprintf("CREATE %s %s ON %s (%s);",
		indexType,
		p.QuoteName(index.Name),
		p.QuoteName(tableName),
		strings.Join(quotedFields, ", "))
}

// GenerateDropIndex generates DROP INDEX statement for MySQL
func (p *Provider) GenerateDropIndex(indexName, tableName string) string {
	return fmt.Sprintf("DROP INDEX %s ON %s;", p.QuoteName(indexName), p.QuoteName(tableName))
}

// GenerateDropTable generates DROP TABLE statement
func (p *Provider) GenerateDropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE %s;", p.QuoteName(tableName))
}

// GenerateAddColumn generates ALTER TABLE ADD COLUMN statement
func (p *Provider) GenerateAddColumn(tableName string, field *types.Field) string {
	fieldDef := fmt.Sprintf("%s %s", p.QuoteName(field.Name), p.ConvertFieldType(field))

	if !field.IsNullable() {
		fieldDef += " NOT NULL"
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

// GenerateRenameColumn generates ALTER TABLE CHANGE statement for MySQL
func (p *Provider) GenerateRenameColumn(tableName, oldName, newName string) string {
	// MySQL uses CHANGE syntax which requires the full column definition
	// This is a simplified version - would need field definition in real implementation
	return fmt.Sprintf("ALTER TABLE %s CHANGE %s %s VARCHAR(255);",
		p.QuoteName(tableName), p.QuoteName(oldName), p.QuoteName(newName))
}

// Placeholder implementations for remaining interface methods

func (p *Provider) GenerateCreateTable(schema *types.Schema, table *types.Table) (string, error) {
	var fieldDefs []string
	var constraints []string

	for _, field := range table.Fields {
		fieldDef, constraint, err := p.convertField(schema, &field)
		if err != nil {
			return "", fmt.Errorf("failed to convert field %s: %w", field.Name, err)
		}

		// Only add non-empty field definitions (skip many_to_many fields)
		if fieldDef != "" {
			fieldDefs = append(fieldDefs, fieldDef)
		}
		if constraint != "" {
			constraints = append(constraints, constraint)
		}
	}

	// Combine field definitions and constraints
	allDefs := append(fieldDefs, constraints...)

	// Build CREATE TABLE statement
	var sql strings.Builder
	sql.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", p.QuoteName(table.Name)))

	for i, def := range allDefs {
		sql.WriteString("    " + def)
		if i < len(allDefs)-1 {
			sql.WriteString(",")
		}
		sql.WriteString("\n")
	}

	sql.WriteString(") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;")
	return sql.String(), nil
}

// convertField converts a YAML field definition to MySQL field definition
func (p *Provider) convertField(schema *types.Schema, field *types.Field) (string, string, error) {
	// Skip many_to_many fields - they don't create actual columns
	if field.Type == "many_to_many" {
		return "", "", nil
	}

	var def strings.Builder
	def.WriteString(p.QuoteName(field.Name))
	def.WriteString(" ")

	// Convert field type
	sqlType := p.ConvertFieldType(field)
	def.WriteString(sqlType)

	// Add NOT NULL constraint
	if !field.IsNullable() {
		def.WriteString(" NOT NULL")
	}

	// Handle auto_create and auto_update for timestamp fields
	if field.AutoCreate && field.Type == "timestamp" {
		def.WriteString(" DEFAULT CURRENT_TIMESTAMP")
	} else if field.Default != "" {
		// Convert default value using the schema's defaults mapping
		defaultValue := utils.ConvertDefaultValue(schema, "mysql", field.Default)
		def.WriteString(" DEFAULT " + defaultValue)
	}

	// Generate primary key constraint if needed
	var constraint string
	if field.PrimaryKey {
		constraint = fmt.Sprintf("PRIMARY KEY (%s)", p.QuoteName(field.Name))
	}

	return def.String(), constraint, nil
}

func (p *Provider) GenerateAlterColumn(tableName string, oldField, newField *types.Field) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) GenerateForeignKeyConstraint(tableName, fieldName, referencedTable, onDelete string) string {
	return ""
}

func (p *Provider) GenerateDropForeignKeyConstraint(tableName, constraintName string) string {
	return ""
}

func (p *Provider) GenerateJunctionTable(table1, table2 string, schema *types.Schema) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) InferForeignKeyType(referencedTable string, schema *types.Schema) string {
	return ""
}

func (p *Provider) GenerateIndexes(schema *types.Schema) string {
	return ""
}

func (p *Provider) GenerateForeignKeyConstraints(schema *types.Schema, junctionTables []types.Table) string {
	return ""
}

// GetDatabaseSchema extracts schema information from a MySQL database
func (p *Provider) GetDatabaseSchema(connectionString string) (*types.Schema, error) {
	return nil, fmt.Errorf("MySQL schema extraction not implemented yet")
}
