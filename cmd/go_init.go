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
	"strings"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

// ExecuteGoMigrationInit initializes the migrations/ directory for the Go migration framework.
// If Goose-style *.sql files are detected in the migrations directory, it automatically
// delegates to ExecuteMigrateToGo to convert them. Otherwise, if a .schema_snapshot.yaml
// exists it generates an initial Go migration from it. Always generates main.go and go.mod
// if they don't already exist.
func ExecuteGoMigrationInit(databaseType string, verbose bool) error {
	cfg := config.DefaultConfig()
	migrationsDir := cfg.Migration.Directory
	gen := codegen.NewGoGenerator()

	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("creating migrations directory: %w", err)
	}

	// Auto-upgrade: if Goose SQL migrations exist, convert them to Go migrations.
	sqlFiles, _ := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if len(sqlFiles) > 0 {
		fmt.Printf("Detected %d Goose SQL migration(s) in %s — running migrate-to-go...\n",
			len(sqlFiles), migrationsDir)
		return ExecuteMigrateToGo(migrationsDir, false, true, false, os.Stdout)
	}

	var initialMigName string

	// Detect existing snapshot and generate initial migration from it
	sm := yamlpkg.NewStateManager(verbose)
	existingSchema, err := sm.LoadSchemaSnapshot(migrationsDir)
	if err == nil && existingSchema != nil {
		initialMigName = "0001_initial"
		diff := schemaToInitialDiff(existingSchema)
		src, err := gen.GenerateMigration(initialMigName, []string{}, diff, existingSchema, nil, nil)
		if err != nil {
			return fmt.Errorf("generating initial migration: %w", err)
		}
		migPath := filepath.Join(migrationsDir, codegen.MigrationFileName(initialMigName))
		if err := os.WriteFile(migPath, []byte(src), 0644); err != nil {
			return fmt.Errorf("writing initial migration: %w", err)
		}
		fmt.Printf("Created %s (from existing schema snapshot)\n", migPath)
	}

	// Generate main.go only if it doesn't exist
	mainPath := filepath.Join(migrationsDir, "main.go")
	if _, statErr := os.Stat(mainPath); os.IsNotExist(statErr) {
		if err := os.WriteFile(mainPath, []byte(gen.GenerateMainGo()), 0644); err != nil {
			return fmt.Errorf("writing main.go: %w", err)
		}
		fmt.Printf("Created %s\n", mainPath)
	}

	// Determine module name and Go version from go.mod in current directory.
	moduleName := readModuleName() + "/migrations"
	parentGoVersion := findParentGoVersion(".")

	// Generate go.mod only if it doesn't exist
	goModPath := filepath.Join(migrationsDir, "go.mod")
	if _, statErr := os.Stat(goModPath); os.IsNotExist(statErr) {
		if err := os.WriteFile(goModPath, []byte(gen.GenerateGoMod(moduleName, "main", parentGoVersion)), 0644); err != nil {
			return fmt.Errorf("writing go.mod: %w", err)
		}
		fmt.Printf("Created %s\n", goModPath)
	}

	// Print next steps
	if initialMigName != "" {
		fmt.Printf(`
Your database already has these tables applied. Mark this migration as applied without re-running SQL:

  cd %s && go mod tidy && go build -o migrate .
  ./migrate fake %s

`, migrationsDir, initialMigName)
	} else {
		fmt.Printf(`
Initialization complete. No existing schema found.

To generate your first migration:
  makemigrations makemigrations --name "initial"

Then build and run:
  cd %s && go mod tidy && go build -o migrate .
  ./migrate up
`, migrationsDir)
	}

	return nil
}

// schemaToInitialDiff converts a yaml.Schema to a SchemaDiff that treats every
// table as newly added. Used to generate the initial Go migration from an
// existing .schema_snapshot.yaml.
func schemaToInitialDiff(schema *yamlpkg.Schema) *yamlpkg.SchemaDiff {
	diff := &yamlpkg.SchemaDiff{HasChanges: true}
	for _, t := range schema.Tables {
		diff.Changes = append(diff.Changes, yamlpkg.Change{
			Type:      yamlpkg.ChangeTypeTableAdded,
			TableName: t.Name,
			NewValue:  t,
		})
	}
	return diff
}

// readModuleName reads the Go module name from go.mod in the current working directory.
// Returns "myproject" if go.mod is not found or cannot be parsed.
func readModuleName() string {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "myproject"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return "myproject"
}
