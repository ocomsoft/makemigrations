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
package diff

import (
	"fmt"
	"strings"
)

// Simplified diff engine that compares SQL schemas
// In production, use pg-schema-diff with actual database connections

type Plan struct {
	Statements []Statement
}

type Statement struct {
	DDL        string
	ObjectType string
	ObjectName string
}

type Engine struct {
	verbose bool
}

func New(verbose bool) *Engine {
	return &Engine{
		verbose: verbose,
	}
}

func (e *Engine) ComputeDiff(oldSQL, newSQL string) (*Plan, error) {
	// For MVP, we'll do a simple text-based comparison
	// In production, integrate with pg-schema-diff properly

	plan := &Plan{
		Statements: []Statement{},
	}

	// If old is empty, this is the initial migration
	if strings.TrimSpace(oldSQL) == "" && strings.TrimSpace(newSQL) != "" {
		// Split new SQL into statements
		statements := e.splitStatements(newSQL)
		for _, stmt := range statements {
			if stmt != "" {
				plan.Statements = append(plan.Statements, Statement{
					DDL:        stmt,
					ObjectType: e.detectObjectType(stmt),
					ObjectName: e.extractObjectName(stmt),
				})
			}
		}
	} else if oldSQL != newSQL {
		// For now, just detect that there's a difference
		// In production, use pg-schema-diff for accurate comparison
		plan.Statements = append(plan.Statements, Statement{
			DDL:        "-- Schema changes detected. Please use pg-schema-diff for accurate diff generation.",
			ObjectType: "COMMENT",
			ObjectName: "",
		})
	}

	if e.verbose {
		fmt.Printf("Diff generated: %d statements\n", len(plan.Statements))
	}

	return plan, nil
}

func (e *Engine) HasChanges(plan *Plan) bool {
	return plan != nil && len(plan.Statements) > 0
}

func (e *Engine) GetStatements(plan *Plan) []string {
	if plan == nil {
		return nil
	}

	var statements []string
	for _, stmt := range plan.Statements {
		statements = append(statements, stmt.DDL)
	}

	return statements
}

func (e *Engine) splitStatements(sql string) []string {
	// Simple statement splitter
	var statements []string
	var current strings.Builder
	inString := false
	var stringChar rune

	for _, r := range sql {
		switch {
		case !inString && (r == '\'' || r == '"'):
			inString = true
			stringChar = r
			current.WriteRune(r)
		case inString && r == stringChar:
			inString = false
			current.WriteRune(r)
		case !inString && r == ';':
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	// Add last statement if any
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

func (e *Engine) detectObjectType(stmt string) string {
	upperStmt := strings.ToUpper(strings.TrimSpace(stmt))
	switch {
	case strings.HasPrefix(upperStmt, "CREATE TABLE"):
		return "TABLE"
	case strings.HasPrefix(upperStmt, "CREATE INDEX"):
		return "INDEX"
	case strings.HasPrefix(upperStmt, "CREATE SEQUENCE"):
		return "SEQUENCE"
	case strings.HasPrefix(upperStmt, "CREATE FUNCTION"):
		return "FUNCTION"
	case strings.HasPrefix(upperStmt, "CREATE TRIGGER"):
		return "TRIGGER"
	case strings.HasPrefix(upperStmt, "CREATE TYPE"):
		return "TYPE"
	case strings.HasPrefix(upperStmt, "ALTER TABLE"):
		return "ALTER"
	case strings.HasPrefix(upperStmt, "DROP"):
		return "DROP"
	default:
		return "UNKNOWN"
	}
}

func (e *Engine) extractObjectName(stmt string) string {
	// Simple object name extraction
	words := strings.Fields(stmt)
	if len(words) >= 3 {
		name := words[2]
		// Remove quotes if present
		name = strings.Trim(name, `"`)
		// Remove schema prefix if present
		parts := strings.Split(name, ".")
		return parts[len(parts)-1]
	}
	return ""
}
