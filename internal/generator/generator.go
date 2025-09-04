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
package generator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ocomsoft/makemigrations/internal/state"
)

type Generator struct {
	stateManager *state.Manager
	verbose      bool
}

func New(stateManager *state.Manager, verbose bool) *Generator {
	return &Generator{
		stateManager: stateManager,
		verbose:      verbose,
	}
}

type Migration struct {
	Number        int
	Name          string
	UpSQL         string
	DownSQL       string
	Filename      string
	IsDestructive bool
}

func (g *Generator) GenerateMigration(upStatements, downStatements []string, customName string) (*Migration, error) {
	number, err := g.stateManager.GetNextMigrationNumber()
	if err != nil {
		return nil, err
	}

	// Generate name
	name := customName
	if name == "" {
		name = g.generateName(upStatements)
	}
	name = g.sanitizeName(name)

	// Check for destructive operations
	isDestructive := g.hasDestructiveOperations(upStatements)

	// Generate filename
	filename := fmt.Sprintf("%05d_%s.sql", number, name)

	// Build up migration
	upSQL := g.buildUpMigration(upStatements, isDestructive)

	// Build down migration
	downSQL := g.buildDownMigration(downStatements, true) // Down is always potentially destructive

	migration := &Migration{
		Number:        number,
		Name:          name,
		UpSQL:         upSQL,
		DownSQL:       downSQL,
		Filename:      filename,
		IsDestructive: isDestructive,
	}

	if g.verbose {
		fmt.Printf("Generated migration: %s\n", filename)
		if isDestructive {
			fmt.Println("  Warning: Contains destructive operations")
		}
	}

	return migration, nil
}

func (g *Generator) generateName(statements []string) string {
	// Analyze statements to generate a meaningful name
	var operations []string
	tables := make(map[string]bool)

	for _, stmt := range statements {
		upperStmt := strings.ToUpper(stmt)

		// Extract operation and table
		if strings.Contains(upperStmt, "CREATE TABLE") {
			if table := g.extractTableName(stmt, "CREATE TABLE"); table != "" {
				operations = append(operations, fmt.Sprintf("create_%s", table))
				tables[table] = true
			}
		} else if strings.Contains(upperStmt, "ALTER TABLE") {
			if table := g.extractTableName(stmt, "ALTER TABLE"); table != "" {
				if strings.Contains(upperStmt, "ADD COLUMN") {
					operations = append(operations, fmt.Sprintf("add_column_to_%s", table))
				} else if strings.Contains(upperStmt, "DROP COLUMN") {
					operations = append(operations, fmt.Sprintf("drop_column_from_%s", table))
				} else {
					operations = append(operations, fmt.Sprintf("alter_%s", table))
				}
				tables[table] = true
			}
		} else if strings.Contains(upperStmt, "DROP TABLE") {
			if table := g.extractTableName(stmt, "DROP TABLE"); table != "" {
				operations = append(operations, fmt.Sprintf("drop_%s", table))
				tables[table] = true
			}
		} else if strings.Contains(upperStmt, "CREATE INDEX") {
			if idx := g.extractIndexName(stmt); idx != "" {
				operations = append(operations, fmt.Sprintf("create_index_%s", idx))
			}
		} else if strings.Contains(upperStmt, "DROP INDEX") {
			if idx := g.extractIndexName(stmt); idx != "" {
				operations = append(operations, fmt.Sprintf("drop_index_%s", idx))
			}
		}
	}

	// Generate name from operations
	if len(operations) > 0 {
		if len(operations) <= 2 {
			return strings.Join(operations, "_and_")
		}
		// If many operations, use generic name with table count
		return fmt.Sprintf("migrate_%d_tables", len(tables))
	}

	return "migration"
}

func (g *Generator) extractTableName(stmt, prefix string) string {
	pattern := fmt.Sprintf(`(?i)%s\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?(["\w.]+)`, prefix)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) > 1 {
		name := strings.Trim(matches[1], `"`)
		name = strings.ToLower(name)
		// Remove schema prefix if present
		parts := strings.Split(name, ".")
		return parts[len(parts)-1]
	}
	return ""
}

func (g *Generator) extractIndexName(stmt string) string {
	re := regexp.MustCompile(`(?i)(?:CREATE|DROP)\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?(["\w.]+)`)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) > 1 {
		name := strings.Trim(matches[1], `"`)
		return strings.ToLower(name)
	}
	return ""
}

func (g *Generator) sanitizeName(name string) string {
	// Replace non-alphanumeric characters with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	name = re.ReplaceAllString(name, "_")

	// Remove leading/trailing underscores
	name = strings.Trim(name, "_")

	// Limit length
	if len(name) > 100 {
		name = name[:100]
	}

	// Ensure it's not empty
	if name == "" {
		name = "migration"
	}

	return name
}

func (g *Generator) hasDestructiveOperations(statements []string) bool {
	for _, stmt := range statements {
		upperStmt := strings.ToUpper(stmt)
		if strings.Contains(upperStmt, "DROP") || strings.Contains(upperStmt, "DELETE") {
			return true
		}
	}
	return false
}

func (g *Generator) buildUpMigration(statements []string, hasDestructive bool) string {
	var sql strings.Builder

	sql.WriteString("-- +goose Up\n")
	sql.WriteString("-- +goose StatementBegin\n")

	if hasDestructive {
		sql.WriteString("-- REVIEW: This migration contains destructive operations\n")
		sql.WriteString("\n")
	}

	for _, stmt := range statements {
		// Add review comment for individual destructive operations
		upperStmt := strings.ToUpper(stmt)
		if strings.Contains(upperStmt, "DROP") || strings.Contains(upperStmt, "DELETE") {
			sql.WriteString("-- REVIEW\n")
		}

		sql.WriteString(stmt)
		if !strings.HasSuffix(strings.TrimSpace(stmt), ";") {
			sql.WriteString(";")
		}
		sql.WriteString("\n")
	}

	sql.WriteString("-- +goose StatementEnd\n")

	return sql.String()
}

func (g *Generator) buildDownMigration(statements []string, hasDestructive bool) string {
	var sql strings.Builder

	sql.WriteString("-- +goose Down\n")
	sql.WriteString("-- +goose StatementBegin\n")

	if hasDestructive || len(statements) > 0 {
		sql.WriteString("-- REVIEW: Rollback migration - verify data safety\n")
		sql.WriteString("\n")
	}

	if len(statements) == 0 {
		sql.WriteString("-- No automatic rollback generated\n")
		sql.WriteString("-- TODO: Add manual rollback statements if needed\n")
	} else {
		for _, stmt := range statements {
			// Add review comment for all down operations
			sql.WriteString("-- REVIEW\n")

			sql.WriteString(stmt)
			if !strings.HasSuffix(strings.TrimSpace(stmt), ";") {
				sql.WriteString(";")
			}
			sql.WriteString("\n")
		}
	}

	sql.WriteString("-- +goose StatementEnd\n")

	return sql.String()
}

func (g *Generator) GetMigrationPath(filename string) string {
	return filepath.Join(g.stateManager.GetMigrationsDir(), filename)
}

func (g *Generator) GenerateTimestampName(name string) string {
	timestamp := time.Now().UTC().Format("20060102150405")
	return fmt.Sprintf("%s_%s.sql", timestamp, name)
}
