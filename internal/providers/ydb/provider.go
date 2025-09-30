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
package ydb

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
	"github.com/ocomsoft/makemigrations/internal/utils"
)

// Provider implements the Provider interface for YDB (Yandex Database)
// YDB is a distributed SQL database with ACID transactions
type Provider struct{}

// New creates a new YDB provider
func New() *Provider {
	return &Provider{}
}

// QuoteName quotes database identifiers for YDB (backticks)
func (p *Provider) QuoteName(name string) string {
	return fmt.Sprintf("`%s`", name)
}

// SupportsOperation checks if YDB supports a specific operation
func (p *Provider) SupportsOperation(operation string) bool {
	switch operation {
	case "DROP_COLUMN", "ALTER_COLUMN":
		return true
	case "RENAME_TABLE", "RENAME_COLUMN":
		return false // YDB has limited schema modification support
	default:
		return false
	}
}

// ConvertFieldType converts YAML field type to YDB-specific SQL type
func (p *Provider) ConvertFieldType(field *types.Field) string {
	switch field.Type {
	case "varchar":
		return "String" // YDB uses String for text
	case "text":
		return "String"
	case "integer":
		return "Int32"
	case "bigint":
		return "Int64"
	case "serial":
		return "Int64" // YDB doesn't have auto-increment
	case "float":
		return "Double"
	case "decimal":
		return "Decimal(22,9)" // YDB decimal with fixed precision
	case "boolean":
		return "Bool"
	case "date":
		return "Date"
	case "time":
		return "Interval" // YDB doesn't have TIME type
	case "timestamp":
		return "Datetime"
	case "uuid":
		return "String" // Store UUID as string
	case "jsonb":
		return "Json" // YDB has native Json type
	default:
		return "String"
	}
}

// GetDefaultValue converts default value references to YDB-specific values
func (p *Provider) GetDefaultValue(defaultRef string, defaults map[string]string) (string, error) {
	if value, exists := defaults[defaultRef]; exists {
		return value, nil
	}
	return fmt.Sprintf("'%s'", defaultRef), nil
}

// GenerateCreateIndex generates CREATE INDEX statement for YDB
func (p *Provider) GenerateCreateIndex(index *types.Index, tableName string) string {
	// YDB has limited index support
	var quotedFields []string
	for _, fieldName := range index.Fields {
		quotedFields = append(quotedFields, p.QuoteName(fieldName))
	}

	return fmt.Sprintf("-- YDB has limited secondary index support. Consider including %s in PRIMARY KEY for %s (%s);",
		index.Name, tableName, strings.Join(quotedFields, ", "))
}

// GenerateDropIndex generates DROP INDEX statement for YDB
func (p *Provider) GenerateDropIndex(indexName, tableName string) string {
	return fmt.Sprintf("-- YDB doesn't support DROP INDEX for %s on %s;", indexName, tableName)
}

// GenerateDropTable generates DROP TABLE statement
func (p *Provider) GenerateDropTable(tableName string) string {
	return fmt.Sprintf("DROP TABLE %s;", p.QuoteName(tableName))
}

// GenerateAddColumn generates ALTER TABLE ADD COLUMN statement
func (p *Provider) GenerateAddColumn(tableName string, field *types.Field) string {
	fieldDef := fmt.Sprintf("%s %s", p.QuoteName(field.Name), p.ConvertFieldType(field))
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", p.QuoteName(tableName), fieldDef)
}

// GenerateDropColumn generates ALTER TABLE DROP COLUMN statement
func (p *Provider) GenerateDropColumn(tableName, columnName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", p.QuoteName(tableName), p.QuoteName(columnName))
}

// GenerateRenameTable generates RENAME TABLE statement
func (p *Provider) GenerateRenameTable(oldName, newName string) string {
	return fmt.Sprintf("-- YDB doesn't support RENAME TABLE from %s to %s;", oldName, newName)
}

// GenerateRenameColumn generates RENAME COLUMN statement
func (p *Provider) GenerateRenameColumn(tableName, oldName, newName string) string {
	return fmt.Sprintf("-- YDB doesn't support RENAME COLUMN for %s.%s -> %s;", tableName, oldName, newName)
}

// GenerateCreateTable generates CREATE TABLE statement for YDB
func (p *Provider) GenerateCreateTable(schema *types.Schema, table *types.Table) (string, error) {
	var fieldDefs []string
	var primaryKeys []string

	for _, field := range table.Fields {
		fieldDef, err := p.convertField(schema, &field)
		if err != nil {
			return "", fmt.Errorf("failed to convert field %s: %w", field.Name, err)
		}

		if fieldDef != "" {
			fieldDefs = append(fieldDefs, fieldDef)
		}

		if field.PrimaryKey {
			primaryKeys = append(primaryKeys, p.QuoteName(field.Name))
		}
	}

	var sql strings.Builder
	sql.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", p.QuoteName(table.Name)))

	for i, def := range fieldDefs {
		sql.WriteString("    " + def)
		if i < len(fieldDefs)-1 {
			sql.WriteString(",")
		}
		sql.WriteString("\n")
	}

	// YDB requires primary key
	if len(primaryKeys) > 0 {
		sql.WriteString(fmt.Sprintf(",\n    PRIMARY KEY (%s)\n", strings.Join(primaryKeys, ", ")))
	} else {
		sql.WriteString("\n")
	}

	sql.WriteString(");")
	return sql.String(), nil
}

func (p *Provider) convertField(schema *types.Schema, field *types.Field) (string, error) {
	if field.Type == "many_to_many" {
		return "", nil
	}

	var def strings.Builder
	def.WriteString(p.QuoteName(field.Name))
	def.WriteString(" ")

	sqlType := p.ConvertFieldType(field)

	// YDB uses Optional<Type> for nullable fields
	if field.IsNullable() && !field.PrimaryKey {
		def.WriteString("Optional<")
		def.WriteString(sqlType)
		def.WriteString(">")
	} else {
		def.WriteString(sqlType)
	}

	// Handle default values
	if field.Default != "" {
		// Convert default value using the schema's defaults mapping
		// Note: YDB has limited default value support, mainly for auto-generated fields
		defaultValue := utils.ConvertDefaultValue(schema, "ydb", field.Default)
		def.WriteString(" DEFAULT " + defaultValue)
	}

	return def.String(), nil
}

// Remaining interface methods with YDB-specific implementations or placeholders
func (p *Provider) GenerateAlterColumn(tableName string, oldField, newField *types.Field) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) GenerateForeignKeyConstraint(tableName, fieldName, referencedTable, onDelete string) string {
	return fmt.Sprintf("-- YDB doesn't support foreign key constraints for %s.%s -> %s;", tableName, fieldName, referencedTable)
}

func (p *Provider) GenerateDropForeignKeyConstraint(tableName, constraintName string) string {
	return fmt.Sprintf("-- YDB doesn't support foreign key constraints for %s.%s;", tableName, constraintName)
}

func (p *Provider) GenerateJunctionTable(table1, table2 string, schema *types.Schema) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) InferForeignKeyType(referencedTable string, schema *types.Schema) string {
	return "Int64"
}

func (p *Provider) GenerateIndexes(schema *types.Schema) string {
	var comments []string
	for _, table := range schema.Tables {
		for _, field := range table.Fields {
			if field.Type == "foreign_key" {
				comment := fmt.Sprintf("-- YDB: Consider including %s.%s in PRIMARY KEY for better performance;", table.Name, field.Name)
				comments = append(comments, comment)
			}
		}
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
	return "-- YDB doesn't support foreign key constraints;"
}

// GetDatabaseSchema extracts schema information from a YDB database
func (p *Provider) GetDatabaseSchema(connectionString string) (*types.Schema, error) {
	return nil, fmt.Errorf("YDB schema extraction not implemented yet")
}
