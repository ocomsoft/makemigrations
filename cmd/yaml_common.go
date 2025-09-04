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

	"github.com/fatih/color"
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
