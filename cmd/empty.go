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

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
)

var (
	emptyMigName    string
	emptyMigDryRun  bool
	emptyMigVerbose bool
)

// emptyCmd is the "makemigrations empty" subcommand. It creates a blank
// migration .go file with no operations, similar to Django's
// `makemigrations --empty`. The developer fills in the operations manually.
var emptyCmd = &cobra.Command{
	Use:   "empty",
	Short: "Create a blank migration with no operations",
	Long: `Creates a blank migration .go file with an empty Operations slice and
a TODO comment as a placeholder. Use this to write custom migrations that
contain operations not generated automatically from schema changes.

The generated migration automatically depends on the current DAG leaves
(the most recent migrations), so it will be applied in the correct order.

Example:
  makemigrations empty --name add_custom_triggers`,
	RunE: runEmpty,
}

func init() {
	rootCmd.AddCommand(emptyCmd)
	emptyCmd.Flags().StringVar(&emptyMigName, "name", "blank",
		"Custom migration name suffix (default: blank)")
	emptyCmd.Flags().BoolVar(&emptyMigDryRun, "dry-run", false,
		"Print generated migration without writing")
	emptyCmd.Flags().BoolVar(&emptyMigVerbose, "verbose", false,
		"Show detailed output")
}

// runEmpty generates a blank migration file and writes it to the migrations
// directory. It queries the existing DAG to determine the correct dependency
// list so the blank migration slots into the chain correctly.
func runEmpty(_ *cobra.Command, _ []string) error {
	cfg := config.LoadOrDefault(configFile)
	migrationsDir := cfg.Migration.Directory

	// Scan for existing migration files to determine next number and deps.
	goFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	if err != nil {
		return fmt.Errorf("scanning migrations directory: %w", err)
	}

	var migFiles []string
	for _, f := range goFiles {
		if filepath.Base(f) != "main.go" {
			migFiles = append(migFiles, f)
		}
	}

	// Query DAG for current leaves (dependencies for the new migration).
	var deps []string
	if len(migFiles) > 0 {
		dagOut, err := queryDAG(migrationsDir, emptyMigVerbose)
		if err != nil {
			return fmt.Errorf("querying migration DAG: %w", err)
		}
		deps = dagOut.Leaves
	}

	count := len(migFiles)
	name := BuildMigrationName(count, emptyMigName, "")

	if emptyMigVerbose {
		fmt.Printf("Generating blank migration: %s\n", name)
		if len(deps) > 0 {
			fmt.Printf("Dependencies: %v\n", deps)
		}
	}

	gen := codegen.NewBlankGenerator()
	src, err := gen.GenerateBlank(name, deps)
	if err != nil {
		return fmt.Errorf("generating blank migration: %w", err)
	}

	if emptyMigDryRun {
		fmt.Println(src)
		return nil
	}

	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return fmt.Errorf("creating migrations directory: %w", err)
	}

	outPath := filepath.Join(migrationsDir, codegen.MigrationFileName(name))
	if err := os.WriteFile(outPath, []byte(src), 0o644); err != nil {
		return fmt.Errorf("writing blank migration: %w", err)
	}

	fmt.Printf("Created %s\n", outPath)
	fmt.Println("Edit the file and add your migration operations.")
	return nil
}
