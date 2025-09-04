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
package starrocks

import (
	"fmt"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/types"
)

// Provider implements the Provider interface for StarRocks
// StarRocks is an MPP analytical database for real-time analytics
type Provider struct{}

// New creates a new StarRocks provider
func New() *Provider {
	return &Provider{}
}

// QuoteName quotes database identifiers for StarRocks (backticks like MySQL)
func (p *Provider) QuoteName(name string) string {
	return fmt.Sprintf("`%s`", name)
}

// SupportsOperation checks if StarRocks supports a specific operation
func (p *Provider) SupportsOperation(operation string) bool {
	switch operation {
	case "DROP_COLUMN", "ALTER_COLUMN":
		return true
	case "RENAME_TABLE", "RENAME_COLUMN":
		return false // StarRocks has limited schema modification
	default:
		return false
	}
}

// ConvertFieldType converts YAML field type to StarRocks-specific SQL type
func (p *Provider) ConvertFieldType(field *types.Field) string {
	switch field.Type {
	case "varchar":
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "STRING" // StarRocks STRING type
	case "text":
		return "STRING"
	case "integer":
		return "INT"
	case "bigint":
		return "BIGINT"
	case "serial":
		return "BIGINT" // StarRocks doesn't have auto-increment
	case "float":
		return "FLOAT"
	case "decimal":
		if field.Precision > 0 && field.Scale >= 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DECIMAL(27,9)" // StarRocks high precision default
	case "boolean":
		return "BOOLEAN"
	case "date":
		return "DATE"
	case "time":
		return "TIME" // StarRocks doesn't support TIME, use STRING
	case "timestamp":
		return "DATETIME"
	case "uuid":
		return "VARCHAR(36)"
	case "jsonb":
		return "JSON" // StarRocks has native JSON support
	default:
		return "STRING"
	}
}

// GetDefaultValue converts default value references to StarRocks-specific values
func (p *Provider) GetDefaultValue(defaultRef string, defaults map[string]string) (string, error) {
	if value, exists := defaults[defaultRef]; exists {
		return value, nil
	}
	return fmt.Sprintf("'%s'", defaultRef), nil
}

// GenerateCreateIndex generates CREATE INDEX statement for StarRocks
func (p *Provider) GenerateCreateIndex(index *types.Index, tableName string) string {
	// StarRocks uses different indexing strategy
	var quotedFields []string
	for _, fieldName := range index.Fields {
		quotedFields = append(quotedFields, p.QuoteName(fieldName))
	}

	return fmt.Sprintf("-- StarRocks uses bitmap/bloom filter indexes. Consider creating bitmap index for %s on %s (%s);",
		index.Name, tableName, strings.Join(quotedFields, ", "))
}

// GenerateDropIndex generates DROP INDEX statement for StarRocks
func (p *Provider) GenerateDropIndex(indexName, tableName string) string {
	return fmt.Sprintf("-- StarRocks index management differs from traditional databases for %s on %s;", indexName, tableName)
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
	return fmt.Sprintf("-- StarRocks doesn't support RENAME TABLE from %s to %s;", oldName, newName)
}

// GenerateRenameColumn generates RENAME COLUMN statement
func (p *Provider) GenerateRenameColumn(tableName, oldName, newName string) string {
	return fmt.Sprintf("-- StarRocks doesn't support RENAME COLUMN for %s.%s -> %s;", tableName, oldName, newName)
}

// GenerateCreateTable generates CREATE TABLE statement for StarRocks
func (p *Provider) GenerateCreateTable(schema *types.Schema, table *types.Table) (string, error) {
	var fieldDefs []string
	var primaryKeys []string

	for _, field := range table.Fields {
		fieldDef, err := p.convertField(&field)
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

	sql.WriteString(")")

	// StarRocks requires ENGINE and key specification
	if len(primaryKeys) > 0 {
		sql.WriteString(fmt.Sprintf("\nPRIMARY KEY (%s)", strings.Join(primaryKeys, ", ")))
	}

	// Default to OLAP engine with DUPLICATE KEY model
	sql.WriteString("\nENGINE=OLAP\nDUPLICATE KEY")

	if len(primaryKeys) > 0 {
		sql.WriteString(fmt.Sprintf("(%s)", strings.Join(primaryKeys, ", ")))
	} else {
		// Use first column as duplicate key if no primary key
		if len(fieldDefs) > 0 {
			// Extract first field name
			firstField := strings.Fields(fieldDefs[0])[0]
			sql.WriteString(fmt.Sprintf("(%s)", firstField))
		}
	}

	sql.WriteString("\nDISTRIBUTED BY HASH")
	if len(primaryKeys) > 0 {
		sql.WriteString(fmt.Sprintf("(%s)", strings.Join(primaryKeys, ", ")))
	} else if len(fieldDefs) > 0 {
		firstField := strings.Fields(fieldDefs[0])[0]
		sql.WriteString(fmt.Sprintf("(%s)", firstField))
	}

	sql.WriteString("\nPROPERTIES (\n    \"replication_num\" = \"1\"\n);")
	return sql.String(), nil
}

func (p *Provider) convertField(field *types.Field) (string, error) {
	if field.Type == "many_to_many" {
		return "", nil
	}

	var def strings.Builder
	def.WriteString(p.QuoteName(field.Name))
	def.WriteString(" ")

	sqlType := p.ConvertFieldType(field)
	def.WriteString(sqlType)

	if !field.IsNullable() {
		def.WriteString(" NOT NULL")
	}

	if field.Default != "" {
		def.WriteString(" DEFAULT " + field.Default)
	}

	return def.String(), nil
}

// Remaining interface methods
func (p *Provider) GenerateAlterColumn(tableName string, oldField, newField *types.Field) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) GenerateForeignKeyConstraint(tableName, fieldName, referencedTable, onDelete string) string {
	return fmt.Sprintf("-- StarRocks doesn't support foreign key constraints for %s.%s -> %s;", tableName, fieldName, referencedTable)
}

func (p *Provider) GenerateDropForeignKeyConstraint(tableName, constraintName string) string {
	return fmt.Sprintf("-- StarRocks doesn't support foreign key constraints for %s.%s;", tableName, constraintName)
}

func (p *Provider) GenerateJunctionTable(table1, table2 string, schema *types.Schema) (string, error) {
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) InferForeignKeyType(referencedTable string, schema *types.Schema) string {
	return "BIGINT"
}

func (p *Provider) GenerateIndexes(schema *types.Schema) string {
	var comments []string

	for _, table := range schema.Tables {
		for _, field := range table.Fields {
			if field.Type == "foreign_key" {
				comment := fmt.Sprintf("-- StarRocks: Consider bitmap index for %s.%s for better query performance;", table.Name, field.Name)
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
	return "-- StarRocks doesn't support foreign key constraints;"
}

// GetDatabaseSchema extracts schema information from a StarRocks database
func (p *Provider) GetDatabaseSchema(connectionString string) (*types.Schema, error) {
	return nil, fmt.Errorf("StarRocks schema extraction not implemented yet")
}
