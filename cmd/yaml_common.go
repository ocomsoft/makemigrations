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
	"github.com/ocomsoft/makemigrations/internal/scanner"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

// YAMLComponents holds the initialized YAML processing components
type YAMLComponents struct {
	StateManager       *yamlpkg.StateManager
	Scanner            *scanner.Scanner
	Parser             *yamlpkg.Parser
	Merger             *yamlpkg.Merger
	DiffEngine         *yamlpkg.DiffEngine
	MigrationGenerator *yamlpkg.MigrationGenerator
}

// InitializeYAMLComponents creates and initializes all YAML processing components
func InitializeYAMLComponents(dbType yamlpkg.DatabaseType, verbose bool, silentMode bool) *YAMLComponents {
	// Load config for additional settings
	cfg := config.LoadOrDefault("")

	// Override config silent setting with command line flag if provided
	cfgSilent := cfg.Migration.Silent
	if silentMode {
		cfgSilent = true
	}

	return &YAMLComponents{
		StateManager:       yamlpkg.NewStateManager(verbose),
		Scanner:            scanner.New(verbose),
		Parser:             yamlpkg.NewParser(verbose),
		Merger:             yamlpkg.NewMerger(verbose),
		DiffEngine:         yamlpkg.NewDiffEngine(verbose),
		MigrationGenerator: yamlpkg.NewMigrationGeneratorWithFullConfig(dbType, verbose, cfg.Migration.IncludeDownSQL, cfg.Migration.ReviewCommentPrefix, cfg.Migration.DestructiveOperations, cfgSilent, cfg.Migration.RejectionCommentPrefix, cfg.Migration.FilePrefix),
	}
}

// ScanAndParseSchemas scans for YAML schema files and parses them with include support
func ScanAndParseSchemas(components *YAMLComponents, verbose bool) ([]*yamlpkg.Schema, error) {
	// Scan for YAML schema files
	schemaFiles, err := components.Scanner.ScanYAMLModules()
	if err != nil {
		return nil, fmt.Errorf("failed to scan modules: %w", err)
	}

	if verbose {
		color.Green("Found %d YAML schema files\n", len(schemaFiles))
		for _, file := range schemaFiles {
			marker := ""
			if file.HasMarker {
				marker = " (with marker)"
			}
			color.Cyan("  - %s%s\n", file.ModulePath, marker)
		}
	}

	if len(schemaFiles) == 0 {
		return nil, fmt.Errorf("no YAML schema files found")
	}

	// Parse all YAML schemas with include support
	var allSchemas []*yamlpkg.Schema
	for _, file := range schemaFiles {
		if verbose {
			color.Blue("Processing schema file: %s\n", file.ModulePath)
		}

		// Use include-aware parsing if the file has a path, otherwise fall back to content parsing
		var schema *yamlpkg.Schema
		if file.FilePath != "" {
			// Parse with include support using file path
			schema, err = components.Parser.ParseSchemaFile(file.FilePath)
		} else {
			// Fall back to content-based parsing (no includes supported)
			schema, err = components.Parser.ParseSchema(file.Content)
		}

		if err != nil {
			return nil, fmt.Errorf("parsing failed for %s: %w", file.ModulePath, err)
		}

		// Run basic structure validation but continue if it fails
		if err := components.Parser.ValidateSchemaStructure(schema); err != nil {
			color.Yellow("Structure validation warning for %s: %v\n", file.ModulePath, err)
		}

		allSchemas = append(allSchemas, schema)
	}

	return allSchemas, nil
}

// MergeAndValidateSchemas merges schemas and validates the result
func MergeAndValidateSchemas(components *YAMLComponents, allSchemas []*yamlpkg.Schema, dbType yamlpkg.DatabaseType, verbose bool) (*yamlpkg.Schema, error) {
	// Merge schemas
	mergedSchema, err := components.Merger.MergeSchemas(allSchemas)
	if err != nil {
		return nil, fmt.Errorf("failed to merge schemas: %w", err)
	}

	if verbose {
		color.Green("Merged schema: %d tables\n", len(mergedSchema.Tables))
		color.Blue("Available tables:")
		for _, table := range mergedSchema.Tables {
			color.Cyan("  - %s\n", table.Name)
		}
	}

	// Final validation on merged schema - show issues but continue
	finalValidationErrors := components.Parser.ValidateComprehensive(mergedSchema, dbType)
	if len(finalValidationErrors) > 0 {
		color.Yellow("\nMerged schema validation issues:\n")
		fmt.Print(components.Parser.FormatValidationErrors(finalValidationErrors))

		// Check if there are fatal errors that prevent migration generation
		hasFatalErrors := false
		for _, validationErr := range finalValidationErrors {
			if validationErr.Severity != "warning" {
				hasFatalErrors = true
				break
			}
		}

		if hasFatalErrors {
			return nil, fmt.Errorf("merged schema validation failed - please fix the foreign key references and other errors")
		}
	}

	return mergedSchema, nil
}

// ExecuteYAMLMakeMigrations handles the complete YAML-based migration generation process
func ExecuteYAMLMakeMigrations(cmd *cobra.Command, configFile, databaseType string, verbose, silent, dryRun, check bool, customName string) error {
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

// ExecuteYAMLInit handles the complete YAML project initialization process
func ExecuteYAMLInit(databaseType string, verbose bool) error {
	if verbose {
		color.Cyan("Initializing makemigrations for YAML project")
		color.Cyan("===========================================")
	}

	// Check if we're in a Go module
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found. Please run this command from the root of a Go module")
	}

	// Parse database type
	dbType, err := yamlpkg.ParseDatabaseType(databaseType)
	if err != nil {
		return fmt.Errorf("invalid database type: %w", err)
	}

	// Load or create config
	cfg := config.DefaultConfig()
	cfg.Database.Type = databaseType
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
	color.White("  3. Apply migration with: goose -dir %s %s <connection-string> up\n", migrationsDir, databaseType)
	color.White("  4. Use 'makemigrations makemigrations' to generate future migrations\n")

	return nil
}

// ExecuteDumpSQL handles the complete dump SQL process
func ExecuteDumpSQL(cmd *cobra.Command, databaseType string, verbose bool) error {
	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Dumping merged schema as SQL\n")
		fmt.Fprintf(cmd.ErrOrStderr(), "============================\n")
	}

	// Parse database type
	dbType, err := yamlpkg.ParseDatabaseType(databaseType)
	if err != nil {
		return fmt.Errorf("invalid database type: %w", err)
	}

	// Initialize YAML components
	components := InitializeYAMLComponents(dbType, verbose, false)
	sqlConverter := yamlpkg.NewSQLConverter(dbType, verbose)

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Database type: %s\n", dbType)
		fmt.Fprintf(cmd.ErrOrStderr(), "\n1. Scanning Go modules for YAML schema files...\n")
	}

	// Scan and parse schemas using shared function but adapt verbose output for dump_sql
	allSchemas, err := ScanAndParseSchemas(components, false) // Don't use verbose mode here since we customize output
	if err != nil {
		if err.Error() == "no YAML schema files found" {
			fmt.Fprintf(cmd.ErrOrStderr(), "No YAML schema files found. Nothing to dump.\n")
			return nil
		}
		return err
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n2. Parsing and merging YAML schemas...\n")
	}

	// Merge and validate schemas using shared function
	mergedSchema, err := MergeAndValidateSchemas(components, allSchemas, dbType, false) // Don't use verbose here since we customize output
	if err != nil {
		return fmt.Errorf("merged schema validation failed: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Merged schema: %d tables\n", len(mergedSchema.Tables))
		fmt.Fprintf(cmd.ErrOrStderr(), "\n3. Validating merged schema...\n")
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n4. Generating SQL...\n")
	}

	// Convert to SQL
	sql, err := sqlConverter.ConvertSchema(mergedSchema)
	if err != nil {
		return fmt.Errorf("failed to generate SQL: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Generated %d lines of SQL\n", len(sql))
		fmt.Fprintf(cmd.ErrOrStderr(), "\n5. SQL Output:\n")
		fmt.Fprintf(cmd.ErrOrStderr(), "================\n")
	}

	// Output SQL to stdout
	fmt.Print(sql)

	return nil
}

// ExecuteFindIncludes handles the complete find includes process
func ExecuteFindIncludes(cmd *cobra.Command, configFile, schemaPath string, interactive, includeWorkspace, verbose, schemaProvided bool) error {
	// Load configuration
	cfg := config.LoadOrDefault(configFile)

	// Apply config settings
	verbose = cfg.Output.Verbose
	if !cfg.Output.ColorEnabled {
		color.NoColor = true
	}

	if verbose {
		color.Cyan("Schema Include Discovery Tool")
		color.Cyan("=============================")
	}

	// Check if schema flag was provided
	schemaProvided = cmd.Flags().Changed("schema")

	// If schema not provided, search for schema.yaml files
	if !schemaProvided {
		if verbose {
			color.Blue("No --schema flag provided, searching for schema.yaml files...")
		}

		localSchemas, err := findLocalSchemaFiles()
		if err != nil {
			return fmt.Errorf("failed to search for local schema files: %w", err)
		}

		if len(localSchemas) == 0 {
			return fmt.Errorf("no schema.yaml files found in current directory and subdirectories")
		}

		if len(localSchemas) == 1 {
			// Use the single schema file found
			schemaPath = localSchemas[0].Path
			if verbose {
				color.Green("Found schema file: %s (database: %s)", schemaPath, localSchemas[0].DatabaseName)
			}
		} else {
			// Multiple schema files found, prompt user
			selectedPath, err := selectLocalSchemaFile(localSchemas)
			if err != nil {
				return fmt.Errorf("failed to select schema file: %w", err)
			}
			schemaPath = selectedPath
		}
	}

	// Validate schema file exists
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return fmt.Errorf("schema file not found: %s", schemaPath)
	}

	if verbose {
		color.Blue("\n1. Discovering schemas in Go modules...")
	}

	// Discover schemas
	discovered, err := discoverSchemas()
	if err != nil {
		return fmt.Errorf("failed to discover schemas: %w", err)
	}

	if len(discovered) == 0 {
		color.Yellow("No YAML schemas found in Go modules.")
		return nil
	}

	if verbose {
		color.Green("Found %d schema(s)\n", len(discovered))
	}

	// Load existing schema
	existingSchema, err := loadExistingSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to load existing schema: %w", err)
	}

	// Filter out already included schemas
	newSchemas := filterNewSchemas(discovered, existingSchema)
	if len(newSchemas) == 0 {
		color.Yellow("All discovered schemas are already included.")
		return nil
	}

	if verbose {
		color.Blue("\n2. Processing discovered schemas...")
	}

	// Handle interactive vs automatic mode
	var schemasToAdd []DiscoveredSchema
	if interactive {
		schemasToAdd, err = selectSchemasInteractively(newSchemas)
		if err != nil {
			return fmt.Errorf("interactive selection failed: %w", err)
		}
	} else {
		schemasToAdd = newSchemas
		if verbose {
			color.Green("Adding %d new schema(s) automatically\n", len(schemasToAdd))
		}
	}

	if len(schemasToAdd) == 0 {
		color.Yellow("No schemas selected for inclusion.")
		return nil
	}

	if verbose {
		color.Blue("\n3. Updating schema file...")
	}

	// Update schema file
	err = updateSchemaWithIncludes(schemaPath, existingSchema, schemasToAdd)
	if err != nil {
		return fmt.Errorf("failed to update schema: %w", err)
	}

	color.Green("\nSuccessfully added %d include(s) to %s", len(schemasToAdd), schemaPath)

	// Show what was added
	color.Cyan("\nAdded includes:")
	for _, schema := range schemasToAdd {
		marker := ""
		if schema.IsWorkspace {
			marker = " (workspace)"
		}
		color.Cyan("  - %s -> %s%s", schema.ModulePath, schema.RelativePath, marker)
	}

	return nil
}
