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

	"github.com/ocomsoft/makemigrations/internal/analyzer"
	"github.com/ocomsoft/makemigrations/internal/diff"
	"github.com/ocomsoft/makemigrations/internal/generator"
	"github.com/ocomsoft/makemigrations/internal/merger"
	"github.com/ocomsoft/makemigrations/internal/parser"
	"github.com/ocomsoft/makemigrations/internal/scanner"
	"github.com/ocomsoft/makemigrations/internal/state"
	"github.com/ocomsoft/makemigrations/internal/struct2schema"
	"github.com/ocomsoft/makemigrations/internal/writer"
)

// ExecuteStruct2Schema handles the complete struct-to-schema conversion process
func ExecuteStruct2Schema(inputDir, outputFile, configFile, targetDB string, dryRun, verbose bool) error {
	if verbose {
		fmt.Println("struct2schema - Go struct to YAML schema converter")
		fmt.Println("=============================================")
	}

	// Initialize the struct2schema processor
	processor, err := struct2schema.NewProcessor(struct2schema.ProcessorConfig{
		InputDir:   inputDir,
		OutputFile: outputFile,
		ConfigFile: configFile,
		TargetDB:   targetDB,
		DryRun:     dryRun,
		Verbose:    verbose,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize processor: %w", err)
	}

	// Process the structs
	if err := processor.Process(); err != nil {
		return fmt.Errorf("failed to process structs: %w", err)
	}

	if dryRun {
		fmt.Println("\nDry run completed successfully - no files were modified")
	} else {
		if verbose {
			fmt.Printf("\nSchema file written to: %s\n", outputFile)
		}
		fmt.Println("struct2schema completed successfully")
	}

	return nil
}

// ExecuteSQLMakeMigrations handles the complete SQL-based migration generation process
func ExecuteSQLMakeMigrations(verbose, dryRun, check bool, customName string) error {
	if verbose {
		fmt.Println("Django-style migration generator for Go")
		fmt.Println("=====================================")
	}

	// Initialize components
	stateManager := state.New("", verbose)
	scannerInstance := scanner.New(verbose)
	parserInstance := parser.New(verbose)
	mergerInstance := merger.New(verbose)
	analyzerInstance := analyzer.New(verbose)
	diffEngine := diff.New(verbose)
	migrationGenerator := generator.New(stateManager, verbose)
	migrationWriter := writer.New(verbose)

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
		fmt.Println("No schema files found. Nothing to do.")
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
	mergedSchema, err := mergerInstance.MergeSchemas(allStatements, "current")
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
		fmt.Println("\n4. Loading previous schema snapshot...")
	}

	// Load previous schema
	oldSQL, err := stateManager.LoadSnapshot()
	if err != nil {
		return fmt.Errorf("failed to load schema snapshot: %w", err)
	}

	if verbose {
		fmt.Println("\n5. Computing schema differences...")
	}

	// Compute diff
	plan, err := diffEngine.ComputeDiff(oldSQL, newSQL)
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	// Check if there are changes
	if !diffEngine.HasChanges(plan) {
		fmt.Println("No changes detected. Schema is up to date.")
		if check {
			return nil
		}
		return nil
	}

	statements := diffEngine.GetStatements(plan)

	if verbose {
		fmt.Printf("Found %d changes\n", len(statements))
	}

	// Check mode - exit with error if changes found
	if check {
		fmt.Printf("Schema changes detected (%d statements). Run without --check to generate migration.\n", len(statements))
		return fmt.Errorf("schema changes detected")
	}

	if verbose {
		fmt.Println("\n6. Generating migration...")
	}

	// Generate migration
	migration, err := migrationGenerator.GenerateMigration(statements, []string{}, customName)
	if err != nil {
		return fmt.Errorf("failed to generate migration: %w", err)
	}

	// Dry run mode
	if dryRun {
		fmt.Println("DRY RUN - Migration that would be generated:")
		fmt.Println("=" + fmt.Sprintf("%42s", ""))
		fmt.Print(migrationWriter.PreviewMigration(migration))
		return nil
	}

	if verbose {
		fmt.Println("\n7. Writing migration file...")
	}

	// Write migration file
	migrationPath := migrationGenerator.GetMigrationPath(migration.Filename)
	if err := migrationWriter.WriteMigration(migration, migrationPath); err != nil {
		return fmt.Errorf("failed to write migration: %w", err)
	}

	if verbose {
		fmt.Println("\n8. Updating schema snapshot...")
	}

	// Save new schema snapshot
	if err := stateManager.SaveSnapshot(newSQL); err != nil {
		return fmt.Errorf("failed to save schema snapshot: %w", err)
	}

	fmt.Printf("Migration generated: %s\n", migration.Filename)

	if migration.IsDestructive {
		fmt.Println("\n⚠️  WARNING: This migration contains destructive operations.")
		fmt.Println("   Please review the migration file carefully before applying.")
	}

	fmt.Printf("\nTo apply the migration, run: goose -dir %s postgres <connection-string> up\n",
		stateManager.GetMigrationsDir())

	return nil
}
