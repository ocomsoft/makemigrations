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
	initDatabaseType string
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize migrations directory and create initial migration from YAML schemas",
	Long: `Initialize the migrations directory structure and create an initial migration
from existing schema.yaml files.

This command:
- Creates the migrations/ directory if it doesn't exist
- Scans for schema.yaml files in Go module dependencies
- Merges all schemas into a unified schema
- Creates an initial migration file (00001_initial.sql)
- Sets up the YAML schema snapshot for future migrations

Use this command when setting up makemigrations for the first time in a project
that uses YAML schema files.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVar(&initDatabaseType, "database", "postgresql",
		"Target database type (postgresql, mysql, sqlserver, sqlite)")
	initCmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed processing information")
}

func runInit(_ *cobra.Command, _ []string) error {
	if verbose {
		color.Cyan("Initializing makemigrations for YAML project")
		color.Cyan("===========================================")
	}

	// Check if we're in a Go module
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found. Please run this command from the root of a Go module")
	}

	// Parse database type
	dbType, err := yamlpkg.ParseDatabaseType(initDatabaseType)
	if err != nil {
		return fmt.Errorf("invalid database type: %w", err)
	}

	// Load or create config
	cfg := config.DefaultConfig()
	cfg.Database.Type = initDatabaseType
	cfg.Output.Verbose = verbose

	// Initialize YAML components
	components := InitializeYAMLComponents(dbType, verbose, false)

	// Create migrations directory
	migrationsDir := cfg.Migration.Directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Create config file if it doesn't exist
	configPath := config.GetConfigPath()
	if !config.ConfigExists() {
		if err := cfg.Save(configPath); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
		if verbose {
			color.Green("Created config file: %s\n", configPath)
		}
	}

	if verbose {
		color.Green("Created migrations directory: %s\n", migrationsDir)
		color.Yellow("Database type: %s\n", dbType)
	}

	// Check if already initialized
	snapshotPath := components.StateManager.GetSnapshotPath("")
	if _, err := os.Stat(snapshotPath); err == nil {
		color.Yellow("Project already initialized. YAML schema snapshot exists at: %s\n", snapshotPath)
		color.Cyan("Use 'makemigrations makemigrations' to generate new migrations.")
		return nil
	}

	if verbose {
		color.Blue("\n1. Scanning Go modules for YAML schema files...")
	}

	// Scan and parse schemas
	allSchemas, err := ScanAndParseSchemas(components, verbose)
	if err != nil {
		if err.Error() == "no YAML schema files found" {
			color.Yellow("No YAML schema files found.")
			color.Cyan("To use makemigrations, add schema.yaml files to your Go modules at schema/schema.yaml")

			// Create initial empty schema for future use
			if err := components.StateManager.CreateInitialSnapshot("project", ""); err != nil {
				return fmt.Errorf("failed to create initial schema snapshot: %w", err)
			}

			color.Green("Created empty YAML schema snapshot at: %s\n", snapshotPath)
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
		color.Red("\nCannot generate initial migration due to validation errors above.")
		color.Red("Please fix the foreign key references and other errors, then try again.")
		return err
	}

	if verbose {
		color.Blue("\n3. Creating initial migration...")
	}

	// Since this is the initial migration, create it from the complete schema
	migrationContent, err := components.MigrationGenerator.GenerateInitialMigration(mergedSchema, "initial")
	if err != nil {
		return fmt.Errorf("failed to generate initial migration: %w", err)
	}

	// Generate filename for initial migration
	migrationFilename := components.MigrationGenerator.GenerateFilename("initial")

	// Write migration file
	migrationPath := filepath.Join(migrationsDir, migrationFilename)
	if err := os.WriteFile(migrationPath, []byte(migrationContent), 0644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}

	if verbose {
		color.Blue("\n4. Saving YAML schema snapshot...")
	}

	// Save YAML schema snapshot
	if err := components.StateManager.SaveSchemaSnapshot(mergedSchema, ""); err != nil {
		return fmt.Errorf("failed to save schema snapshot: %w", err)
	}

	color.Green("✅ YAML project initialized successfully!\n\n")
	color.Green("Created:\n")
	color.Cyan("  - Initial migration: %s\n", migrationFilename)
	color.Cyan("  - YAML schema snapshot: %s\n", filepath.Base(snapshotPath))
	color.Cyan("  - Config file: %s\n", configPath)
	color.Blue("\nNext steps:\n")
	color.White("  1. Review the initial migration file\n")
	color.White("  2. Customize configuration in %s\n", configPath)
	color.White("  3. Apply migration with: goose -dir %s %s <connection-string> up\n", migrationsDir, initDatabaseType)
	color.White("  4. Use 'makemigrations makemigrations' to generate future migrations\n")

	return nil
}
