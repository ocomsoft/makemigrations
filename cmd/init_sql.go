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
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ocomsoft/makemigrations/internal/analyzer"
	"github.com/ocomsoft/makemigrations/internal/generator"
	"github.com/ocomsoft/makemigrations/internal/merger"
	"github.com/ocomsoft/makemigrations/internal/parser"
	"github.com/ocomsoft/makemigrations/internal/scanner"
	"github.com/ocomsoft/makemigrations/internal/state"
	"github.com/ocomsoft/makemigrations/internal/writer"
)

// initSQLCmd represents the init_sql command
var initSQLCmd = &cobra.Command{
	Use:   "init_sql",
	Short: "Initialize migrations directory and create initial migration from SQL schemas",
	Long: `Initialize the migrations directory structure and create an initial migration
from existing schema.sql files.

This command:
- Creates the migrations/ directory if it doesn't exist
- Scans for schema.sql files in Go module dependencies
- Merges all schemas into a unified schema
- Creates an initial migration file (00001_initial.sql)
- Sets up the schema snapshot for future migrations

Use this command when setting up makemigrations for the first time in a project
that uses SQL schema files.`,
	RunE: runInitSQL,
}

func init() {
	rootCmd.AddCommand(initSQLCmd)

	initSQLCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed processing information")
}

func runInitSQL(_ *cobra.Command, _ []string) error {
	if verbose {
		fmt.Println("Initializing makemigrations for project")
		fmt.Println("=====================================")
	}

	// Check if we're in a Go module
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found. Please run this command from the root of a Go module")
	}

	// Initialize components
	stateManager := state.New("", verbose)
	scannerInstance := scanner.New(verbose)
	parserInstance := parser.New(verbose)
	mergerInstance := merger.New(verbose)
	analyzerInstance := analyzer.New(verbose)
	migrationGenerator := generator.New(stateManager, verbose)
	migrationWriter := writer.New(verbose)

	// Create migrations directory
	migrationsDir := stateManager.GetMigrationsDir()
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	if verbose {
		fmt.Printf("Created migrations directory: %s\n", migrationsDir)
	}

	// Check if already initialized
	snapshotPath := stateManager.GetSnapshotPath()
	if _, err := os.Stat(snapshotPath); err == nil {
		fmt.Printf("Project already initialized. Schema snapshot exists at: %s\n", snapshotPath)
		fmt.Println("Use 'makemigrations' to generate new migrations.")
		return nil
	}

	if verbose {
		fmt.Println("\n1. Scanning Go modules for schema files...")
	}

	// Scan for schema files
	schemaFiles, err := scannerInstance.ScanModules()
	if err != nil {
		return fmt.Errorf("failed to scan modules: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d schema files\n", len(schemaFiles))
		for _, file := range schemaFiles {
			marker := ""
			if file.HasMarker {
				marker = " (with marker)"
			}
			fmt.Printf("  - %s%s\n", file.ModulePath, marker)
		}
	}

	if len(schemaFiles) == 0 {
		fmt.Println("No schema files found.")
		fmt.Println("To use makemigrations, add schema.sql files to your Go modules at sql/schema.sql")

		// Still create empty snapshot for future use
		if err := stateManager.SaveSnapshot(""); err != nil {
			return fmt.Errorf("failed to create empty schema snapshot: %w", err)
		}

		fmt.Printf("Created empty schema snapshot at: %s\n", snapshotPath)
		return nil
	}

	if verbose {
		fmt.Println("\n2. Parsing and merging schemas...")
	}

	// Parse and merge all schemas
	var allStatements []parser.Statement
	for _, file := range schemaFiles {
		statements, err := parserInstance.ParseSchema(file.Content)
		if err != nil {
			if verbose {
				fmt.Printf("Warning: Failed to parse %s: %v\n", file.ModulePath, err)
			}
			continue
		}
		allStatements = append(allStatements, statements...)
	}

	// Merge schemas
	mergedSchema, err := mergerInstance.MergeSchemas(allStatements, "initial")
	if err != nil {
		return fmt.Errorf("failed to merge schemas: %w", err)
	}

	if verbose {
		fmt.Printf("Merged schema: %d tables, %d indexes\n",
			len(mergedSchema.Tables), len(mergedSchema.Indexes))
	}

	if verbose {
		fmt.Println("\n3. Analyzing dependencies and generating ordered SQL...")
	}

	// Order statements by dependencies
	tableOrder, err := analyzerInstance.OrderStatements(mergedSchema)
	if err != nil {
		return fmt.Errorf("failed to order statements: %w", err)
	}

	// Generate the complete ordered SQL
	newSQL := analyzerInstance.GenerateOrderedSQL(mergedSchema, tableOrder)

	if verbose {
		fmt.Println("\n4. Creating initial migration...")
	}

	// Since this is the initial migration, we'll create it with all the current schema
	// Split the SQL into individual statements for the migration
	var statements []string
	if newSQL != "" {
		// Parse the generated SQL back into statements for the migration
		parsedStatements, err := parserInstance.ParseSchema(newSQL)
		if err == nil {
			for _, stmt := range parsedStatements {
				statements = append(statements, stmt.SQL)
			}
		} else {
			// Fallback: use the raw SQL
			statements = append(statements, newSQL)
		}
	}

	// Generate initial migration
	migration, err := migrationGenerator.GenerateMigration(statements, []string{}, "initial")
	if err != nil {
		return fmt.Errorf("failed to generate initial migration: %w", err)
	}

	// Write migration file
	migrationPath := migrationGenerator.GetMigrationPath(migration.Filename)
	if err := migrationWriter.WriteMigration(migration, migrationPath); err != nil {
		return fmt.Errorf("failed to write migration: %w", err)
	}

	if verbose {
		fmt.Println("\n5. Saving schema snapshot...")
	}

	// Save schema snapshot
	if err := stateManager.SaveSnapshot(newSQL); err != nil {
		return fmt.Errorf("failed to save schema snapshot: %w", err)
	}

	fmt.Printf("✅ Project initialized successfully!\n\n")
	fmt.Printf("Created:\n")
	fmt.Printf("  - Initial migration: %s\n", migration.Filename)
	fmt.Printf("  - Schema snapshot: %s\n", filepath.Base(snapshotPath))
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Review the initial migration file\n")
	fmt.Printf("  2. Apply it with: goose -dir %s postgres <connection-string> up\n", migrationsDir)
	fmt.Printf("  3. Use 'makemigrations' to generate future migrations\n")

	return nil
}
