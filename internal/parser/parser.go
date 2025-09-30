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
package parser

import (
	"fmt"
	"regexp"
	"strings"
)

type StatementType string

const (
	CreateTable    StatementType = "CREATE_TABLE"
	AlterTable     StatementType = "ALTER_TABLE"
	CreateIndex    StatementType = "CREATE_INDEX"
	CreateSequence StatementType = "CREATE_SEQUENCE"
	CreateFunction StatementType = "CREATE_FUNCTION"
	CreateTrigger  StatementType = "CREATE_TRIGGER"
	CreateType     StatementType = "CREATE_TYPE"
	CreateView     StatementType = "CREATE_VIEW"
	Unknown        StatementType = "UNKNOWN"
)

type Statement struct {
	Type       StatementType
	ObjectName string
	SQL        string
	// For tables, this includes column definitions
	Columns []Column
	// For indexes
	IndexedTable   string
	IndexedColumns []string
	IsUnique       bool
	// Foreign key relationships
	ForeignKeys []ForeignKey
}

type Column struct {
	Name         string
	DataType     string
	Size         int // For VARCHAR(n)
	IsNullable   bool
	IsPrimaryKey bool
	DefaultValue string
}

type ForeignKey struct {
	Name              string
	Columns           []string
	ReferencedTable   string
	ReferencedColumns []string
}

type Parser struct {
	verbose bool
}

func New(verbose bool) *Parser {
	return &Parser{
		verbose: verbose,
	}
}

func (p *Parser) ParseSchema(sql string) ([]Statement, error) {
	// Normalize the SQL
	sql = p.normalizeSQL(sql)

	// Split into statements
	rawStatements := p.splitStatements(sql)

	var statements []Statement
	for _, raw := range rawStatements {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		stmt, err := p.parseStatement(raw)
		if err != nil {
			if p.verbose {
				fmt.Printf("Warning: could not parse statement: %v\n", err)
			}
			// Add as unknown statement
			stmt = Statement{
				Type: Unknown,
				SQL:  raw,
			}
		}
		statements = append(statements, stmt)
	}

	return statements, nil
}

func (p *Parser) normalizeSQL(sql string) string {
	// Remove comments (except special markers)
	lines := strings.Split(sql, "\n")
	var normalized []string
	inBlockComment := false

	for _, line := range lines {
		// Check for block comments
		if strings.Contains(line, "/*") {
			inBlockComment = true
		}
		if inBlockComment {
			if strings.Contains(line, "*/") {
				inBlockComment = false
			}
			continue
		}

		// Skip line comments except MIGRATION_SCHEMA marker
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			if !strings.Contains(line, "MIGRATION_SCHEMA") {
				continue
			}
		}

		normalized = append(normalized, line)
	}

	return strings.Join(normalized, "\n")
}

func (p *Parser) splitStatements(sql string) []string {
	// Simple split by semicolon (this can be improved)
	// TODO: Handle semicolons within strings, functions, etc.
	statements := strings.Split(sql, ";")
	var result []string

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			result = append(result, stmt)
		}
	}

	return result
}

func (p *Parser) parseStatement(sql string) (Statement, error) {
	sql = strings.TrimSpace(sql)
	upperSQL := strings.ToUpper(sql)

	stmt := Statement{
		SQL: sql,
	}

	switch {
	case strings.HasPrefix(upperSQL, "CREATE TABLE"):
		return p.parseCreateTable(sql)
	case strings.HasPrefix(upperSQL, "CREATE INDEX") || strings.HasPrefix(upperSQL, "CREATE UNIQUE INDEX"):
		return p.parseCreateIndex(sql)
	case strings.HasPrefix(upperSQL, "CREATE SEQUENCE"):
		stmt.Type = CreateSequence
		stmt.ObjectName = p.extractObjectName(sql, "SEQUENCE")
	case strings.HasPrefix(upperSQL, "CREATE FUNCTION"):
		stmt.Type = CreateFunction
		stmt.ObjectName = p.extractObjectName(sql, "FUNCTION")
	case strings.HasPrefix(upperSQL, "CREATE TRIGGER"):
		stmt.Type = CreateTrigger
		stmt.ObjectName = p.extractObjectName(sql, "TRIGGER")
	case strings.HasPrefix(upperSQL, "CREATE TYPE"):
		stmt.Type = CreateType
		stmt.ObjectName = p.extractObjectName(sql, "TYPE")
	case strings.HasPrefix(upperSQL, "CREATE VIEW"):
		stmt.Type = CreateView
		stmt.ObjectName = p.extractObjectName(sql, "VIEW")
	case strings.HasPrefix(upperSQL, "ALTER TABLE"):
		stmt.Type = AlterTable
		stmt.ObjectName = p.extractObjectName(sql, "TABLE")
	default:
		stmt.Type = Unknown
	}

	return stmt, nil
}

func (p *Parser) parseCreateTable(sql string) (Statement, error) {
	stmt := Statement{
		Type: CreateTable,
		SQL:  sql,
	}

	// Extract table name
	re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(["\w.]+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		stmt.ObjectName = p.cleanIdentifier(matches[1])
	}

	// Extract column definitions (simplified)
	start := strings.Index(strings.ToUpper(sql), "(")
	end := strings.LastIndex(sql, ")")
	if start > 0 && end > start {
		columnDefs := sql[start+1 : end]
		stmt.Columns = p.parseColumns(columnDefs)
		stmt.ForeignKeys = p.parseForeignKeys(columnDefs)
	}

	return stmt, nil
}

func (p *Parser) parseColumns(columnDefs string) []Column {
	var columns []Column

	// Split by comma (simplified - doesn't handle all cases)
	parts := p.smartSplit(columnDefs, ',')

	for _, part := range parts {
		part = strings.TrimSpace(part)
		upperPart := strings.ToUpper(part)

		// Skip constraints
		if strings.Contains(upperPart, "CONSTRAINT") ||
			strings.Contains(upperPart, "PRIMARY KEY") ||
			strings.Contains(upperPart, "FOREIGN KEY") ||
			strings.Contains(upperPart, "CHECK") ||
			strings.Contains(upperPart, "UNIQUE") {
			continue
		}

		col := p.parseColumn(part)
		if col.Name != "" {
			columns = append(columns, col)
		}
	}

	return columns
}

func (p *Parser) parseColumn(def string) Column {
	col := Column{
		IsNullable: true, // Default to nullable
	}

	parts := strings.Fields(def)
	if len(parts) < 2 {
		return col
	}

	col.Name = p.cleanIdentifier(parts[0])
	col.DataType = strings.ToUpper(parts[1])

	// Extract size for VARCHAR, CHAR, etc.
	if strings.Contains(col.DataType, "(") {
		re := regexp.MustCompile(`\((\d+)\)`)
		matches := re.FindStringSubmatch(col.DataType)
		if len(matches) > 1 {
			_, _ = fmt.Sscanf(matches[1], "%d", &col.Size)
			col.DataType = strings.Split(col.DataType, "(")[0]
		}
	}

	// Check for constraints
	upperDef := strings.ToUpper(def)
	if strings.Contains(upperDef, "NOT NULL") {
		col.IsNullable = false
	}
	if strings.Contains(upperDef, "PRIMARY KEY") {
		col.IsPrimaryKey = true
		col.IsNullable = false
	}
	if strings.Contains(upperDef, "DEFAULT") {
		// Extract default value (simplified)
		idx := strings.Index(upperDef, "DEFAULT")
		if idx >= 0 {
			remaining := def[idx+7:]
			parts := strings.Fields(remaining)
			if len(parts) > 0 {
				col.DefaultValue = parts[0]
			}
		}
	}

	return col
}

func (p *Parser) parseForeignKeys(columnDefs string) []ForeignKey {
	var foreignKeys []ForeignKey

	// Look for FOREIGN KEY constraints
	re := regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\(([^)]+)\)\s*REFERENCES\s+(["\w.]+)\s*\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(columnDefs, -1)

	for _, match := range matches {
		if len(match) > 3 {
			fk := ForeignKey{
				Columns:           p.parseColumnList(match[1]),
				ReferencedTable:   p.cleanIdentifier(match[2]),
				ReferencedColumns: p.parseColumnList(match[3]),
			}
			foreignKeys = append(foreignKeys, fk)
		}
	}

	// Also look for inline foreign key definitions
	parts := p.smartSplit(columnDefs, ',')
	for _, part := range parts {
		if strings.Contains(strings.ToUpper(part), "REFERENCES") {
			re := regexp.MustCompile(`(?i)(\w+)\s+.*REFERENCES\s+(["\w.]+)\s*\(([^)]+)\)`)
			matches := re.FindStringSubmatch(part)
			if len(matches) > 3 {
				fk := ForeignKey{
					Columns:           []string{p.cleanIdentifier(matches[1])},
					ReferencedTable:   p.cleanIdentifier(matches[2]),
					ReferencedColumns: p.parseColumnList(matches[3]),
				}
				foreignKeys = append(foreignKeys, fk)
			}
		}
	}

	return foreignKeys
}

func (p *Parser) parseCreateIndex(sql string) (Statement, error) {
	stmt := Statement{
		Type: CreateIndex,
		SQL:  sql,
	}

	upperSQL := strings.ToUpper(sql)
	stmt.IsUnique = strings.Contains(upperSQL, "UNIQUE")

	// Extract index name and table
	re := regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(["\w.]+)\s+ON\s+(["\w.]+)\s*\(([^)]+)\)`)
	matches := re.FindStringSubmatch(sql)

	if len(matches) > 3 {
		stmt.ObjectName = p.cleanIdentifier(matches[1])
		stmt.IndexedTable = p.cleanIdentifier(matches[2])
		stmt.IndexedColumns = p.parseColumnList(matches[3])
	}

	return stmt, nil
}

func (p *Parser) extractObjectName(sql, objectType string) string {
	pattern := fmt.Sprintf(`(?i)%s\s+(?:IF\s+NOT\s+EXISTS\s+)?(["\w.]+)`, objectType)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return p.cleanIdentifier(matches[1])
	}
	return ""
}

func (p *Parser) cleanIdentifier(id string) string {
	// Remove quotes and normalize
	id = strings.Trim(id, `"`)
	id = strings.TrimSpace(id)
	return id
}

func (p *Parser) parseColumnList(list string) []string {
	var columns []string
	parts := strings.Split(list, ",")
	for _, part := range parts {
		col := p.cleanIdentifier(part)
		if col != "" {
			columns = append(columns, col)
		}
	}
	return columns
}

func (p *Parser) smartSplit(s string, sep rune) []string {
	var result []string
	var current strings.Builder
	parenDepth := 0
	inQuote := false

	for _, r := range s {
		switch r {
		case '"', '\'':
			inQuote = !inQuote
			current.WriteRune(r)
		case '(':
			if !inQuote {
				parenDepth++
			}
			current.WriteRune(r)
		case ')':
			if !inQuote {
				parenDepth--
			}
			current.WriteRune(r)
		case sep:
			if parenDepth == 0 && !inQuote {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}
