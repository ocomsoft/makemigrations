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
	StateManager *yamlpkg.StateManager
	Scanner      *scanner.Scanner
	Parser       *yamlpkg.Parser
	Merger       *yamlpkg.Merger
	DiffEngine   *yamlpkg.DiffEngine
}

// InitializeYAMLComponents creates and initializes all YAML processing components
func InitializeYAMLComponents(dbType yamlpkg.DatabaseType, verbose bool) *YAMLComponents {
	return &YAMLComponents{
		StateManager: yamlpkg.NewStateManager(verbose),
		Scanner:      scanner.New(verbose),
		Parser:       yamlpkg.NewParser(verbose),
		Merger:       yamlpkg.NewMerger(verbose),
		DiffEngine:   yamlpkg.NewDiffEngine(verbose),
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

// ExecuteDumpSQL handles the complete dump SQL process.
// When pending is true, it shows only the SQL for pending schema changes
// (what the next migration would do). When false, it dumps the full schema.
func ExecuteDumpSQL(cmd *cobra.Command, databaseType string, pending bool, verbose bool) error {
	// Parse database type
	dbType, err := yamlpkg.ParseDatabaseType(databaseType)
	if err != nil {
		return fmt.Errorf("invalid database type: %w", err)
	}

	// Initialize YAML components
	components := InitializeYAMLComponents(dbType, verbose)

	if verbose {
		if pending {
			fmt.Fprintf(cmd.ErrOrStderr(), "Dumping pending schema changes as SQL\n")
			fmt.Fprintf(cmd.ErrOrStderr(), "=====================================\n")
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Dumping merged schema as SQL\n")
			fmt.Fprintf(cmd.ErrOrStderr(), "============================\n")
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Database type: %s\n", dbType)
		fmt.Fprintf(cmd.ErrOrStderr(), "\n1. Scanning Go modules for YAML schema files...\n")
	}

	// Scan and parse schemas
	allSchemas, err := ScanAndParseSchemas(components, false)
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

	// Merge and validate schemas
	mergedSchema, err := MergeAndValidateSchemas(components, allSchemas, dbType, false)
	if err != nil {
		return fmt.Errorf("merged schema validation failed: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Merged schema: %d tables\n", len(mergedSchema.Tables))
	}

	if pending {
		return executePendingDumpSQL(cmd, dbType, mergedSchema, verbose)
	}

	return executeFullDumpSQL(cmd, dbType, mergedSchema, verbose)
}

// executeFullDumpSQL dumps the complete schema as CREATE TABLE statements.
func executeFullDumpSQL(cmd *cobra.Command, dbType yamlpkg.DatabaseType, mergedSchema *yamlpkg.Schema, verbose bool) error {
	sqlConverter := yamlpkg.NewSQLConverter(dbType, verbose)

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n3. Generating SQL...\n")
	}

	sql, err := sqlConverter.ConvertSchema(mergedSchema)
	if err != nil {
		return fmt.Errorf("failed to generate SQL: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Generated %d lines of SQL\n", len(sql))
		fmt.Fprintf(cmd.ErrOrStderr(), "\nSQL Output:\n")
		fmt.Fprintf(cmd.ErrOrStderr(), "================\n")
	}

	fmt.Print(sql)
	return nil
}

// executePendingDumpSQL shows only SQL for pending schema changes by comparing
// the current YAML schema against the state from existing migrations.
func executePendingDumpSQL(cmd *cobra.Command, dbType yamlpkg.DatabaseType, currentSchema *yamlpkg.Schema, verbose bool) error {
	cfg := config.LoadOrDefault(configFile)
	migrationsDir := cfg.Migration.Directory

	// Check for existing migration files
	var prevSchema *yamlpkg.Schema

	goFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	if err != nil {
		return fmt.Errorf("scanning migrations directory: %w", err)
	}

	// Filter to migration files only (exclude main.go)
	var migFiles []string
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			migFiles = append(migFiles, f)
		}
	}

	if len(migFiles) > 0 {
		if verbose {
			fmt.Fprintf(cmd.ErrOrStderr(), "\n3. Querying migration DAG for previous state...\n")
		}
		dagOut, err := queryDAG(migrationsDir, verbose)
		if err != nil {
			return fmt.Errorf("querying migration DAG: %w", err)
		}
		prevSchema = schemaStateToYAMLSchema(dagOut.SchemaState, string(dbType))
	} else {
		if verbose {
			fmt.Fprintf(cmd.ErrOrStderr(), "\n3. No existing migrations found, treating previous state as empty...\n")
		}
	}

	// Diff previous state against current schema
	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n4. Computing schema diff...\n")
	}
	diffEngine := yamlpkg.NewDiffEngine(verbose)
	diff, err := diffEngine.CompareSchemas(prevSchema, currentSchema)
	if err != nil {
		return fmt.Errorf("computing schema diff: %w", err)
	}

	if !diff.HasChanges {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "No pending changes.")
		return nil
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "Found %d pending changes\n", len(diff.Changes))
		fmt.Fprintf(cmd.ErrOrStderr(), "\n5. Generating SQL for pending changes...\n")
	}

	// Convert diff to SQL using silent mode (no interactive prompts)
	sqlConverter := yamlpkg.NewSQLConverterWithConfig(dbType, verbose, false, "", nil, true, "")
	upSQL, _, err := sqlConverter.ConvertDiffToSQL(diff, prevSchema, currentSchema)
	if err != nil {
		return fmt.Errorf("failed to generate pending SQL: %w", err)
	}

	if verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "\nPending SQL Output:\n")
		fmt.Fprintf(cmd.ErrOrStderr(), "================\n")
	}

	fmt.Print(upSQL)
	return nil
}

// ExecuteFindIncludes handles the complete find includes process
func ExecuteFindIncludes(cmd *cobra.Command, configFile, schemaPath string, interactive, includeWorkspace bool) error {
	// Load configuration
	cfg := config.LoadOrDefault(configFile)

	// Apply config settings
	verbose := cfg.Output.Verbose
	if !cfg.Output.ColorEnabled {
		color.NoColor = true
	}

	if verbose {
		color.Cyan("Schema Include Discovery Tool")
		color.Cyan("=============================")
	}

	// Check if schema flag was provided
	schemaProvided := cmd.Flags().Changed("schema")

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
