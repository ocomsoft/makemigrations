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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ocomsoft/makemigrations/internal/analyzer"
	"github.com/ocomsoft/makemigrations/internal/diff"
	"github.com/ocomsoft/makemigrations/internal/generator"
	"github.com/ocomsoft/makemigrations/internal/merger"
	"github.com/ocomsoft/makemigrations/internal/parser"
	"github.com/ocomsoft/makemigrations/internal/scanner"
	"github.com/ocomsoft/makemigrations/internal/state"
	"github.com/ocomsoft/makemigrations/internal/version"
	"github.com/ocomsoft/makemigrations/internal/writer"
)

var (
	cfgFile    string
	configFile string // Config file path
	dryRun     bool
	check      bool
	customName string
	verbose    bool
	silent     bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "makemigrations",
	Short: "Django-style migration generator for Go",
	Long: `Generate database migrations from schema.sql files in Go modules.

This tool scans Go module dependencies for schema.sql files, merges them into 
a unified schema, and generates Goose-compatible migration files by comparing 
against the last known schema state.

When run without a subcommand, defaults to 'makemigrations_sql'.

Available commands:
- init: Initialize migrations directory and create initial migration from YAML schemas
- init_sql: Initialize migrations directory and create initial migration from SQL schemas
- makemigrations: Generate migrations from YAML schemas
- makemigrations_sql: Generate migrations from schema.sql files

Features:
- Scans direct Go module dependencies for sql/schema.sql files
- Merges duplicate tables with intelligent conflict resolution
- Handles foreign key dependencies and circular references
- Generates both UP and DOWN migrations
- Adds REVIEW comments for destructive operations
- Compatible with Goose migration runner`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default to makemigrations_sql when no subcommand is provided
		// Import the logic directly since this is the default behavior
		return runDefaultMakeMigrations(cmd, args)
	},
}

// GetRootCmd returns the root command for embedding in other applications
func GetRootCmd() *cobra.Command {
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Display version at startup for all commands
	fmt.Printf("%s\n", version.GetDisplayVersion())
	cobra.CheckErr(rootCmd.Execute())
}

// runDefaultMakeMigrations runs the makemigrations_sql functionality as the default command
func runDefaultMakeMigrations(cmd *cobra.Command, args []string) error {
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

func init() {
	cobra.OnInitialize(initConfig)

	// Global flag for config file
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file path (default: migrations/makemigrations.config.yaml)")

	// Add the main command flags
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be generated without creating files")
	rootCmd.Flags().BoolVar(&check, "check", false, "Exit with error code if migrations are needed (for CI/CD)")
	rootCmd.Flags().StringVar(&customName, "name", "", "Override auto-generated migration name")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed processing information")
	rootCmd.Flags().BoolVar(&silent, "silent", false, "Skip prompts for destructive operations (use review comments instead)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".makemigrations" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".makemigrations")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
