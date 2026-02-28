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
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ocomsoft/makemigrations/internal/codegen"
	"github.com/ocomsoft/makemigrations/internal/config"
	"github.com/ocomsoft/makemigrations/internal/gooseparser"
	"github.com/spf13/cobra"
)

// migrateEntry holds the mapping between a legacy SQL file and the new Go migration file.
type migrateEntry struct {
	sqlFile   string
	goName    string
	versionID int64 // used by history migration to match against goose_db_version.version_id
}

// Package-level flag variables for the migrate-to-go command.
var (
	migrateToGoDryRun    bool
	migrateToGoNoHistory bool
	migrateToGoNoDelete  bool
	migrateToGoForce     bool
)

// migrateToGoCmd converts legacy Goose SQL migration files to Go RunSQL migration files.
var migrateToGoCmd = &cobra.Command{
	Use:   "migrate-to-go",
	Short: "Convert Goose SQL migrations to Go RunSQL migration files",
	Long: `Converts all .sql Goose migration files in the migrations directory
into Go source files that use the RunSQL operation. Optionally migrates
the goose_db_version history to the new migration_record table.

By default, the original .sql files are deleted after successful conversion.
Use --no-delete to keep them. Use --dry-run to preview changes without writing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadOrDefault(configFile)
		return ExecuteMigrateToGo(
			cfg.Migration.Directory,
			migrateToGoDryRun,
			migrateToGoNoHistory,
			migrateToGoNoDelete,
			cmd.OutOrStdout(),
		)
	},
}

func init() {
	rootCmd.AddCommand(migrateToGoCmd)

	migrateToGoCmd.Flags().BoolVar(&migrateToGoDryRun, "dry-run", false, "Preview changes without writing files")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoNoHistory, "no-history", false, "Skip migrating goose_db_version history")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoNoDelete, "no-delete", false, "Keep original .sql files after conversion")
	migrateToGoCmd.Flags().BoolVar(&migrateToGoForce, "force", false, "Overwrite existing .go migration files")
}

// ExecuteMigrateToGo is the main entry point for the migrate-to-go conversion.
// It discovers SQL files in migrationsDir, parses them with gooseparser, generates
// Go RunSQL migration files, and optionally cleans up the SQL originals.
func ExecuteMigrateToGo(migrationsDir string, dryRun, noHistory, noDelete bool, out io.Writer) error {
	// Step 1: Discover SQL files.
	sqlFiles, err := discoverSQLFiles(migrationsDir)
	if err != nil {
		return err
	}
	if len(sqlFiles) == 0 {
		return fmt.Errorf("no .sql migration files found in %s", migrationsDir)
	}

	// Step 2: Check for existing Go migration files (unless --force).
	if !migrateToGoForce {
		if err := checkNoExistingGoMigrations(migrationsDir); err != nil {
			return err
		}
	}

	// Step 3: Build migrateEntry slice from the discovered SQL files.
	entries := make([]migrateEntry, 0, len(sqlFiles))
	for i, sqlFile := range sqlFiles {
		desc := gooseparser.ExtractDescription(sqlFile)
		goName := fmt.Sprintf("%04d_%s", i+1, desc)

		versionID, verErr := gooseparser.ExtractVersionID(sqlFile)
		if verErr != nil {
			return fmt.Errorf("extracting version from %s: %w", sqlFile, verErr)
		}

		entries = append(entries, migrateEntry{
			sqlFile:   sqlFile,
			goName:    goName,
			versionID: versionID,
		})
	}

	// Step 4: Parse each SQL file and generate Go migration source.
	for i, entry := range entries {
		mig, parseErr := gooseparser.ParseFile(filepath.Join(migrationsDir, entry.sqlFile))
		if parseErr != nil {
			return fmt.Errorf("parsing %s: %w", entry.sqlFile, parseErr)
		}

		// Build dependency list: first migration has none, subsequent depend on the previous.
		var deps []string
		if i > 0 {
			deps = []string{entries[i-1].goName}
		}

		src, genErr := generateRunSQLMigration(entry.goName, deps, mig.ForwardSQL, mig.BackwardSQL)
		if genErr != nil {
			return fmt.Errorf("generating Go source for %s: %w", entry.sqlFile, genErr)
		}

		if dryRun {
			fmt.Fprintf(out, "Would create %s.go:\n%s\n", entry.goName, src)
			continue
		}

		goPath := filepath.Join(migrationsDir, entry.goName+".go")
		if writeErr := os.WriteFile(goPath, []byte(src), 0o644); writeErr != nil {
			return fmt.Errorf("writing %s: %w", goPath, writeErr)
		}
		fmt.Fprintf(out, "%s Created %s\n", green("✓"), goPath)
	}

	// Step 5: Generate main.go and go.mod if not present (skip in dry-run).
	if !dryRun {
		gen := codegen.NewGoGenerator()

		mainPath := filepath.Join(migrationsDir, "main.go")
		if _, statErr := os.Stat(mainPath); os.IsNotExist(statErr) {
			if writeErr := os.WriteFile(mainPath, []byte(gen.GenerateMainGo()), 0o644); writeErr != nil {
				return fmt.Errorf("writing main.go: %w", writeErr)
			}
			fmt.Fprintf(out, "%s Created %s\n", green("✓"), mainPath)
		}

		goModPath := filepath.Join(migrationsDir, "go.mod")
		if _, statErr := os.Stat(goModPath); os.IsNotExist(statErr) {
			moduleName := readModuleName() + "/migrations"
			version := "v0.3.0"
			if writeErr := os.WriteFile(goModPath, []byte(gen.GenerateGoMod(moduleName, version)), 0o644); writeErr != nil {
				return fmt.Errorf("writing go.mod: %w", writeErr)
			}
			fmt.Fprintf(out, "%s Created %s\n", green("✓"), goModPath)
		}
	}

	// Step 6: Migrate goose history (Task 3 — stubbed out for now).
	if !noHistory && !dryRun {
		// History migration will be implemented in Task 3.
		// For now, print a reminder.
		fmt.Fprintf(out, "%s Skipping history migration (not yet implemented)\n", yellow("⚠"))
	}

	// Step 7: Delete original SQL files.
	if !noDelete && !dryRun {
		for _, entry := range entries {
			sqlPath := filepath.Join(migrationsDir, entry.sqlFile)
			if rmErr := os.Remove(sqlPath); rmErr != nil {
				return fmt.Errorf("deleting %s: %w", sqlPath, rmErr)
			}
			fmt.Fprintf(out, "%s Deleted %s\n", cyan("✗"), sqlPath)
		}
	}

	return nil
}

// discoverSQLFiles returns the names of all .sql files in dir, sorted alphabetically.
func discoverSQLFiles(dir string) ([]string, error) {
	pattern := filepath.Join(dir, "*.sql")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing %s: %w", pattern, err)
	}

	sort.Strings(matches)

	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, filepath.Base(m))
	}
	return names, nil
}

// checkNoExistingGoMigrations returns an error if any .go files (other than main.go)
// exist in dir. This prevents accidental overwrites of previously converted migrations.
func checkNoExistingGoMigrations(dir string) error {
	pattern := filepath.Join(dir, "*.go")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing %s: %w", pattern, err)
	}

	for _, m := range matches {
		base := filepath.Base(m)
		if base == "main.go" {
			continue
		}
		return fmt.Errorf("existing Go migration file found: %s (use --force to overwrite)", base)
	}
	return nil
}

// generateRunSQLMigration produces a gofmt'd Go source file that registers a RunSQL
// migration with the given name, dependencies, and SQL content.
func generateRunSQLMigration(name string, deps []string, forwardSQL, backwardSQL string) (string, error) {
	var b strings.Builder

	b.WriteString("package main\n\n")
	b.WriteString("import m \"github.com/ocomsoft/makemigrations/migrate\"\n\n")
	b.WriteString("func init() {\n")
	b.WriteString("\tm.Register(&m.Migration{\n")
	b.WriteString(fmt.Sprintf("\t\tName: %q,\n", name))

	// Dependencies
	b.WriteString("\t\tDependencies: []string{")
	if len(deps) > 0 {
		depStrs := make([]string, len(deps))
		for i, d := range deps {
			depStrs[i] = fmt.Sprintf("%q", d)
		}
		b.WriteString(strings.Join(depStrs, ", "))
	}
	b.WriteString("},\n")

	// Operations
	b.WriteString("\t\tOperations: []m.Operation{\n")
	b.WriteString("\t\t\t&m.RunSQL{\n")
	b.WriteString(fmt.Sprintf("\t\t\t\tForwardSQL:  %s,\n", goRawString(forwardSQL)))
	b.WriteString(fmt.Sprintf("\t\t\t\tBackwardSQL: %s,\n", goRawString(backwardSQL)))
	b.WriteString("\t\t\t},\n")
	b.WriteString("\t\t},\n")

	b.WriteString("\t})\n")
	b.WriteString("}\n")

	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return b.String(), fmt.Errorf("formatting generated code: %w", err)
	}
	return string(formatted), nil
}

// goRawString wraps s in a Go backtick raw string literal. If s is empty, it
// returns `""`. If s contains backticks, it splits on them and joins with
// concatenated backtick-containing string literals.
func goRawString(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.Contains(s, "`") {
		return "`" + s + "`"
	}
	// Split on backtick and join with ` + "`" + `
	parts := strings.Split(s, "`")
	var b strings.Builder
	for i, part := range parts {
		if i > 0 {
			b.WriteString(" + \"`\" + ")
		}
		b.WriteString("`")
		b.WriteString(part)
		b.WriteString("`")
	}
	return b.String()
}
