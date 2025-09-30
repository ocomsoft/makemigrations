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
package postgresql

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	"github.com/ocomsoft/makemigrations/internal/types"
	"github.com/ocomsoft/makemigrations/internal/version"
)

// Provider implements the Provider interface for PostgreSQL
type Provider struct{}

// New creates a new PostgreSQL provider
func New() *Provider {
	return &Provider{}
}

// QuoteName quotes database identifiers for PostgreSQL
func (p *Provider) QuoteName(name string) string {
	return fmt.Sprintf(`"%s"`, name)
}

// SupportsOperation checks if PostgreSQL supports a specific operation
func (p *Provider) SupportsOperation(operation string) bool {
	switch operation {
	case "RENAME_COLUMN", "RENAME_TABLE", "DROP_COLUMN", "ALTER_COLUMN":
		return true
	default:
		return false
	}
}

// ConvertFieldType converts YAML field type to PostgreSQL-specific SQL type
func (p *Provider) ConvertFieldType(field *types.Field) string {
	switch field.Type {
	case "varchar":
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "TEXT"
	case "text":
		return "TEXT"
	case "integer":
		return "INTEGER"
	case "bigint":
		return "BIGINT"
	case "serial":
		return "SERIAL"
	case "float":
		return "REAL"
	case "decimal":
		if field.Precision > 0 && field.Scale >= 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DECIMAL"
	case "boolean":
		return "BOOLEAN"
	case "date":
		return "DATE"
	case "time":
		return "TIME"
	case "timestamp":
		return "TIMESTAMP"
	case "uuid":
		return "UUID"
	case "jsonb":
		return "JSONB"
	default:
		return "TEXT"
	}
}

// GetDefaultValue converts default value references to PostgreSQL-specific values
func (p *Provider) GetDefaultValue(defaultRef string, defaults map[string]string) (string, error) {
	if value, exists := defaults[defaultRef]; exists {
		return value, nil
	}
	// Return as literal value if not found in mapping
	return fmt.Sprintf("'%s'", defaultRef), nil
}

// GenerateCreateIndex generates CREATE INDEX statement for PostgreSQL
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

// GenerateDropIndex generates DROP INDEX statement for PostgreSQL
func (p *Provider) GenerateDropIndex(indexName, tableName string) string {
	return fmt.Sprintf("DROP INDEX %s;", p.QuoteName(indexName))
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

// GenerateRenameTable generates ALTER TABLE RENAME statement
func (p *Provider) GenerateRenameTable(oldName, newName string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", p.QuoteName(oldName), p.QuoteName(newName))
}

// GenerateRenameColumn generates ALTER TABLE RENAME COLUMN statement
func (p *Provider) GenerateRenameColumn(tableName, oldName, newName string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;",
		p.QuoteName(tableName), p.QuoteName(oldName), p.QuoteName(newName))
}

// Placeholder implementations for remaining interface methods
// These will be implemented in the next step

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

	sql.WriteString(");")
	return sql.String(), nil
}

// convertField converts a YAML field definition to PostgreSQL field definition
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
		defaultValue := p.convertDefaultValue(schema, field.Default)
		def.WriteString(" DEFAULT " + defaultValue)
	}

	// Generate primary key constraint if needed
	var constraint string
	if field.PrimaryKey {
		constraint = fmt.Sprintf("PRIMARY KEY (%s)", p.QuoteName(field.Name))
	}

	return def.String(), constraint, nil
}

// convertDefaultValue converts YAML default value to PostgreSQL-specific SQL
func (p *Provider) convertDefaultValue(schema *types.Schema, defaultValue string) string {
	if schema == nil || schema.Defaults.PostgreSQL == nil {
		// If no schema or defaults mapping, try to handle common cases
		// Check if it's a numeric value
		if _, err := strconv.ParseFloat(defaultValue, 64); err == nil {
			return defaultValue // Return numeric values as-is
		}
		// Check for boolean values
		if defaultValue == "true" || defaultValue == "false" {
			return defaultValue // PostgreSQL accepts true/false
		}
		// Otherwise treat as string literal
		return fmt.Sprintf("'%s'", defaultValue)
	}

	// Look up the default value in the PostgreSQL defaults mapping
	if sqlDefault, exists := schema.Defaults.PostgreSQL[defaultValue]; exists {
		return sqlDefault
	}

	// If not found in mapping, determine if it needs quotes
	// Check if it's a numeric value
	if _, err := strconv.ParseFloat(defaultValue, 64); err == nil {
		return defaultValue // Return numeric values as-is
	}

	// Check for boolean values
	if defaultValue == "true" || defaultValue == "false" {
		return defaultValue // PostgreSQL accepts true/false
	}

	// Otherwise treat as string literal
	return fmt.Sprintf("'%s'", defaultValue)
}

func (p *Provider) GenerateAlterColumn(tableName string, oldField, newField *types.Field) (string, error) {
	// TODO: Implement
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) GenerateForeignKeyConstraint(tableName, fieldName, referencedTable, onDelete string) string {
	// TODO: Implement
	return ""
}

func (p *Provider) GenerateDropForeignKeyConstraint(tableName, constraintName string) string {
	// TODO: Implement
	return ""
}

func (p *Provider) GenerateJunctionTable(table1, table2 string, schema *types.Schema) (string, error) {
	// TODO: Implement
	return "", fmt.Errorf("not implemented yet")
}

func (p *Provider) InferForeignKeyType(referencedTable string, schema *types.Schema) string {
	// TODO: Implement
	return ""
}

func (p *Provider) GenerateIndexes(schema *types.Schema) string {
	var indexes []string

	for _, table := range schema.Tables {
		// Generate indexes for foreign key fields
		for _, field := range table.Fields {
			if field.Type == "foreign_key" {
				indexName := fmt.Sprintf("idx_%s_%s", table.Name, field.Name)
				indexSQL := fmt.Sprintf("CREATE INDEX %s ON %s (%s);",
					p.QuoteName(indexName),
					p.QuoteName(table.Name),
					p.QuoteName(field.Name))
				indexes = append(indexes, indexSQL)
			}
		}

		// Generate table-level indexes (including unique indexes)
		for _, index := range table.Indexes {
			indexSQL := p.GenerateCreateIndex(&index, table.Name)
			indexes = append(indexes, indexSQL)
		}
	}

	if len(indexes) == 0 {
		return ""
	}

	return strings.Join(indexes, "\n")
}

func (p *Provider) GenerateForeignKeyConstraints(schema *types.Schema, junctionTables []types.Table) string {
	// TODO: Implement
	return ""
}

// GetDatabaseSchema extracts schema information from a PostgreSQL database
func (p *Provider) GetDatabaseSchema(connectionString string) (*types.Schema, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	schema := &types.Schema{
		Database: types.Database{
			Name:             "extracted_schema",
			Version:          "1.0.0",
			MigrationVersion: version.GetVersion(),
		},
		Defaults: types.Defaults{
			PostgreSQL: map[string]string{
				"blank":    "''",
				"now":      "CURRENT_TIMESTAMP",
				"new_uuid": "gen_random_uuid()",
				"today":    "CURRENT_DATE",
				"zero":     "'0'",
				"true":     "'true'",
				"false":    "'false'",
				"null":     "null",
			},
		},
		Tables: []types.Table{},
	}

	tables, err := p.extractTables(db)
	if err != nil {
		return nil, fmt.Errorf("failed to extract tables: %w", err)
	}

	for _, table := range tables {
		fields, err := p.extractFields(db, table.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to extract fields for table %s: %w", table.Name, err)
		}
		table.Fields = fields

		indexes, err := p.extractIndexes(db, table.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to extract indexes for table %s: %w", table.Name, err)
		}
		table.Indexes = indexes

		schema.Tables = append(schema.Tables, table)
	}

	return schema, nil
}

// extractTables gets all user tables from the public schema
func (p *Provider) extractTables(db *sql.DB) ([]types.Table, error) {
	query := `
		SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []types.Table
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}

		tables = append(tables, types.Table{
			Name:    tableName,
			Fields:  []types.Field{},
			Indexes: []types.Index{},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over table rows: %w", err)
	}

	return tables, nil
}

// extractFields gets all fields for a specific table
func (p *Provider) extractFields(db *sql.DB, tableName string) ([]types.Field, error) {
	query := `
		SELECT 
			c.column_name,
			c.data_type,
			c.character_maximum_length,
			c.numeric_precision,
			c.numeric_scale,
			c.is_nullable,
			c.column_default,
			CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary_key
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT ku.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage ku
				ON tc.constraint_name = ku.constraint_name
			WHERE tc.table_schema = 'public'
				AND tc.table_name = $1
				AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.column_name = pk.column_name
		WHERE c.table_schema = 'public'
			AND c.table_name = $1
		ORDER BY c.ordinal_position
	`

	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query fields for table %s: %w", tableName, err)
	}
	defer rows.Close()

	var fields []types.Field
	for rows.Next() {
		var (
			columnName    string
			dataType      string
			maxLength     sql.NullInt64
			numPrecision  sql.NullInt64
			numScale      sql.NullInt64
			isNullable    string
			columnDefault sql.NullString
			isPrimaryKey  bool
		)

		if err := rows.Scan(&columnName, &dataType, &maxLength, &numPrecision, &numScale, &isNullable, &columnDefault, &isPrimaryKey); err != nil {
			return nil, fmt.Errorf("failed to scan field data: %w", err)
		}

		nullable := isNullable == "YES"
		field := types.Field{
			Name:       columnName,
			Type:       p.convertSQLTypeToYAML(dataType),
			Nullable:   &nullable,
			PrimaryKey: isPrimaryKey,
		}

		// Set length, precision, scale
		if maxLength.Valid && maxLength.Int64 > 0 {
			field.Length = int(maxLength.Int64)
		}
		if numPrecision.Valid && numPrecision.Int64 > 0 {
			field.Precision = int(numPrecision.Int64)
		}
		if numScale.Valid && numScale.Int64 >= 0 {
			field.Scale = int(numScale.Int64)
		}

		// Handle default values
		if columnDefault.Valid {
			field.Default = p.convertSQLDefaultToYAML(columnDefault.String)
		}

		// Check for auto-increment/serial types
		if strings.Contains(columnDefault.String, "nextval(") {
			field.Type = "serial"
			field.Default = "" // Remove default for serial fields
		}

		fields = append(fields, field)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over field rows: %w", err)
	}

	// Get foreign key information
	fkFields, err := p.extractForeignKeys(db, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to extract foreign keys: %w", err)
	}

	// Update fields with foreign key information
	for i, field := range fields {
		if fkInfo, exists := fkFields[field.Name]; exists {
			fields[i].Type = "foreign_key"
			fields[i].ForeignKey = &types.ForeignKey{
				Table:    fkInfo.ReferencedTable,
				OnDelete: fkInfo.OnDelete,
			}
		}
	}

	return fields, nil
}

// extractForeignKeys gets foreign key constraints for a table
func (p *Provider) extractForeignKeys(db *sql.DB, tableName string) (map[string]struct {
	ReferencedTable string
	OnDelete        string
}, error) {
	query := `
		SELECT 
			kcu.column_name,
			ccu.table_name AS referenced_table,
			rc.delete_rule
		FROM information_schema.referential_constraints rc
		JOIN information_schema.key_column_usage kcu
			ON rc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage ccu
			ON rc.unique_constraint_name = ccu.constraint_name
		WHERE kcu.table_schema = 'public'
			AND kcu.table_name = $1
	`

	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys: %w", err)
	}
	defer rows.Close()

	fkMap := make(map[string]struct {
		ReferencedTable string
		OnDelete        string
	})

	for rows.Next() {
		var columnName, referencedTable, deleteRule string
		if err := rows.Scan(&columnName, &referencedTable, &deleteRule); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key data: %w", err)
		}

		onDelete := p.convertSQLOnDeleteToYAML(deleteRule)
		fkMap[columnName] = struct {
			ReferencedTable string
			OnDelete        string
		}{
			ReferencedTable: referencedTable,
			OnDelete:        onDelete,
		}
	}

	return fkMap, nil
}

// extractIndexes gets all indexes for a table
func (p *Provider) extractIndexes(db *sql.DB, tableName string) ([]types.Index, error) {
	query := `
		SELECT DISTINCT
			i.indexname,
			i.indexdef,
			CASE WHEN i.indexdef LIKE '%UNIQUE%' THEN true ELSE false END as is_unique
		FROM pg_indexes i
		WHERE i.schemaname = 'public'
			AND i.tablename = $1
			AND i.indexname NOT LIKE '%_pkey'  -- Exclude primary key indexes
		ORDER BY i.indexname
	`

	rows, err := db.Query(query, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	var indexes []types.Index
	for rows.Next() {
		var indexName, indexDef string
		var isUnique bool

		if err := rows.Scan(&indexName, &indexDef, &isUnique); err != nil {
			return nil, fmt.Errorf("failed to scan index data: %w", err)
		}

		// Extract field names from index definition
		fields := p.parseIndexFields(indexDef)
		if len(fields) == 0 {
			continue // Skip if we can't parse fields
		}

		indexes = append(indexes, types.Index{
			Name:   indexName,
			Fields: fields,
			Unique: isUnique,
		})
	}

	return indexes, nil
}

// Helper functions for type conversion

func (p *Provider) convertSQLTypeToYAML(sqlType string) string {
	switch {
	case strings.HasPrefix(sqlType, "character varying"):
		return "varchar"
	case sqlType == "text":
		return "text"
	case sqlType == "integer":
		return "integer"
	case sqlType == "bigint":
		return "bigint"
	case sqlType == "real":
		return "float"
	case strings.HasPrefix(sqlType, "numeric"):
		return "decimal"
	case sqlType == "boolean":
		return "boolean"
	case sqlType == "date":
		return "date"
	case sqlType == "time without time zone":
		return "time"
	case strings.HasPrefix(sqlType, "timestamp"):
		return "timestamp"
	case sqlType == "uuid":
		return "uuid"
	case sqlType == "jsonb":
		return "jsonb"
	case sqlType == "json":
		return "jsonb"
	default:
		return "text" // Default fallback
	}
}

func (p *Provider) convertSQLDefaultToYAML(sqlDefault string) string {
	switch {
	case strings.Contains(sqlDefault, "CURRENT_TIMESTAMP"):
		return "now"
	case strings.Contains(sqlDefault, "CURRENT_DATE"):
		return "today"
	case strings.Contains(sqlDefault, "gen_random_uuid()"):
		return "new_uuid"
	case sqlDefault == "true":
		return "true"
	case sqlDefault == "false":
		return "false"
	case sqlDefault == "''::text" || sqlDefault == "''":
		return "blank"
	case sqlDefault == "0":
		return "zero"
	default:
		// Return the literal value, removing any type casts
		cleaned := strings.Split(sqlDefault, "::")[0]
		return strings.Trim(cleaned, "'")
	}
}

func (p *Provider) convertSQLOnDeleteToYAML(sqlOnDelete string) string {
	switch strings.ToUpper(sqlOnDelete) {
	case "CASCADE":
		return "CASCADE"
	case "RESTRICT":
		return "RESTRICT"
	case "SET NULL":
		return "SET_NULL"
	case "NO ACTION":
		return "PROTECT"
	default:
		return "PROTECT" // Default fallback
	}
}

func (p *Provider) parseIndexFields(indexDef string) []string {
	// Simple parser for index definition
	// Example: CREATE INDEX idx_name ON table_name (field1, field2)
	start := strings.Index(indexDef, "(")
	end := strings.LastIndex(indexDef, ")")

	if start == -1 || end == -1 || end <= start {
		return []string{}
	}

	fieldsStr := indexDef[start+1 : end]
	fieldsList := strings.Split(fieldsStr, ",")

	var fields []string
	for _, field := range fieldsList {
		field = strings.TrimSpace(field)
		field = strings.Trim(field, "\"") // Remove quotes
		if field != "" {
			fields = append(fields, field)
		}
	}

	return fields
}
