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

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/ocomsoft/makemigrations/internal/config"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

var (
	databaseType string
)

// makemigrationsCmd represents the makemigrations command (YAML-based)
var makemigrationsCmd = &cobra.Command{
	Use:   "makemigrations",
	Short: "Django-style migration generator from YAML schemas",
	Long: `Generate database migrations from schema.yaml files in Go modules.

This tool scans Go module dependencies for schema/schema.yaml files, merges them 
into a unified schema, and generates Goose-compatible migration files by comparing 
against the last known schema state.

The YAML schema format supports:
- Multiple database types (PostgreSQL, MySQL, SQL Server, SQLite)
- Foreign key relationships with cascade options
- Many-to-many relationships with auto-generated junction tables
- Database-specific default value mappings
- Field constraints and indexes

Features:
- Scans direct Go module dependencies for schema/schema.yaml files
- Merges duplicate tables with intelligent conflict resolution
- Handles foreign key dependencies and circular references
- Generates both UP and DOWN migrations with database-specific SQL
- Adds REVIEW comments for destructive operations
- Compatible with Goose migration runner

Database Support:
- PostgreSQL (default)
- MySQL
- SQL Server
- SQLite`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runYAMLMakeMigrations(cmd, args)
	},
}

// runYAMLMakeMigrations runs the YAML-based migration generation
func runYAMLMakeMigrations(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg := config.LoadOrDefault(configFile)

	// Override config with command-line flags
	if cmd.Flags().Changed("database") {
		cfg.Database.Type = databaseType
	}
	if cmd.Flags().Changed("verbose") {
		cfg.Output.Verbose = verbose
	}

	// Apply config settings
	verbose = cfg.Output.Verbose
	if !cfg.Output.ColorEnabled {
		color.NoColor = true
	}

	if verbose {
		color.Cyan("Django-style YAML migration generator for Go")
		color.Cyan("===========================================")
	}

	// Parse database type
	dbType, err := yamlpkg.ParseDatabaseType(cfg.Database.Type)
	if err != nil {
		return fmt.Errorf("invalid database type: %w", err)
	}

	// Initialize YAML components
	components := InitializeYAMLComponents(dbType, verbose, silent)

	if verbose {
		color.Yellow("Database type: %s\n", dbType)
		color.Blue("\n1. Scanning Go modules for YAML schema files...")
	}

	// Scan and parse schemas
	allSchemas, err := ScanAndParseSchemas(components, verbose)
	if err != nil {
		if err.Error() == "no YAML schema files found" {
			color.Yellow("No YAML schema files found. Nothing to do.")
			return nil
		}
		return err
	}

	if verbose {
		color.Blue("\n2. Parsing and merging YAML schemas...")
	}

	// Merge and validate schemas
	mergedSchema, err := MergeAndValidateSchemas(components, allSchemas, dbType, verbose)
	if err != nil {
		color.Red("\nCannot generate migration due to validation errors above.")
		color.Red("Please fix the foreign key references and other errors, then try again.")
		return err
	}

	if verbose {
		color.Blue("\n3. Loading previous schema snapshot...")
	}

	// Load previous schema
	oldSchema, err := components.StateManager.LoadSchemaSnapshot("")
	if err != nil {
		if verbose {
			color.Yellow("No previous snapshot found (this is normal for first run): %v\n", err)
		}
		oldSchema = nil // First run
	}

	if verbose {
		color.Blue("\n4. Computing schema differences...")
	}

	// Compute diff
	diff, err := components.DiffEngine.CompareSchemas(oldSchema, mergedSchema)
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	// Check if there are changes
	if !diff.HasChanges {
		color.Green("No changes detected. Schema is up to date.")
		if check {
			return nil
		}
		return nil
	}

	if verbose {
		color.Yellow("Found %d changes\n", len(diff.Changes))
		if diff.IsDestructive {
			color.Red("⚠️  Some changes are destructive")
		}
	}

	// Check mode - exit with error if changes found
	if check {
		color.Yellow("Schema changes detected (%d changes). Run without --check to generate migration.\n", len(diff.Changes))
		return fmt.Errorf("schema changes detected")
	}

	if verbose {
		color.Blue("\n5. Generating migration...")
	}

	// Generate migration - use initial migration if no previous schema exists
	var migrationContent string
	if oldSchema == nil {
		// First time - generate initial migration with full schema including constraints
		migrationContent, err = components.MigrationGenerator.GenerateInitialMigration(mergedSchema, customName)
		if err != nil {
			return fmt.Errorf("failed to generate initial migration: %w", err)
		}
	} else {
		// Incremental migration - generate diff-based migration
		migrationContent, err = components.MigrationGenerator.GenerateCompleteMigration(diff, oldSchema, mergedSchema, customName)
		if err != nil {
			return fmt.Errorf("failed to generate complete migration: %w", err)
		}
	}

	// Dry run mode
	if dryRun {
		color.Cyan("DRY RUN - Migration that would be generated:")
		color.Cyan("=" + fmt.Sprintf("%42s", ""))
		fmt.Print(migrationContent)
		return nil
	}

	if verbose {
		color.Blue("\n6. Writing migration file...")
	}

	// Ensure migrations directory exists
	migrationsDir := cfg.Migration.Directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Generate filename for the migration
	var filename string
	if oldSchema == nil {
		// For initial migration, generate filename based on custom name or "initial"
		description := "initial"
		if customName != "" {
			description = customName
		}
		filename = components.MigrationGenerator.GenerateFilename(description)
	} else {
		// For incremental migration, get filename from migration object
		migration, err := components.MigrationGenerator.GenerateMigration(diff, oldSchema, mergedSchema, customName)
		if err != nil {
			return fmt.Errorf("failed to generate migration for filename: %w", err)
		}
		filename = migration.Filename
	}

	// Write migration file
	migrationPath := filepath.Join(migrationsDir, filename)
	if err := os.WriteFile(migrationPath, []byte(migrationContent), 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	if verbose {
		color.Blue("\n7. Updating schema snapshot...")
	}

	// Save new schema snapshot
	if err := components.StateManager.SaveSchemaSnapshot(mergedSchema, ""); err != nil {
		return fmt.Errorf("failed to save schema snapshot: %w", err)
	}

	color.Green("Migration generated: %s\n", filename)

	// Show warning for destructive operations
	if oldSchema != nil && diff.IsDestructive {
		color.Red("\n⚠️  WARNING: This migration contains destructive operations.")
		color.Red("   Please review the migration file carefully before applying.")
	}

	color.Cyan("\nTo apply the migration, run: goose -dir %s %s <connection-string> up\n",
		migrationsDir, cfg.Database.Type)

	return nil
}

func init() {
	rootCmd.AddCommand(makemigrationsCmd)

	// Add YAML-specific flags
	makemigrationsCmd.Flags().StringVar(&databaseType, "database", "postgresql",
		"Target database type (postgresql, mysql, sqlserver, sqlite)")
	makemigrationsCmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Show what would be generated without creating files")
	makemigrationsCmd.Flags().BoolVar(&check, "check", false,
		"Exit with error code if migrations are needed (for CI/CD)")
	makemigrationsCmd.Flags().StringVar(&customName, "name", "",
		"Override auto-generated migration name")
	makemigrationsCmd.Flags().BoolVar(&verbose, "verbose", false,
		"Show detailed processing information")
	makemigrationsCmd.Flags().BoolVar(&silent, "silent", false,
		"Skip prompts for destructive operations (use review comments instead)")
}
