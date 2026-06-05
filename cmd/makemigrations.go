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

// cmd/makemigrations.go — legacy upgrade shim that migrates config and DB table
// names from the old "makemigrations" naming to "morphic".
package cmd

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ocomsoft/morphic/migrate"
)

var makemigrationsShimDryRun bool

// makemigrationsShimCmd is the legacy upgrade shim. It renames the old config
// file and DB history table to their morphic equivalents.
var makemigrationsShimCmd = &cobra.Command{
	Use:        "makemigrations",
	Short:      "Legacy upgrade shim — migrates config and DB table names",
	Long: `Upgrade a project from the legacy "makemigrations" naming convention to "morphic".

This command performs three actions:
  1. Renames migrations/makemigrations.config.yaml to migrations/morphic.config.yaml
  2. Renames the makemigrations_history DB table to morphic_history
  3. Rewrites import paths in migration .go files from makemigrations to morphic

Both operations are idempotent — running this command multiple times is safe.
Use --dry-run to preview what would happen without making changes.`,
	Deprecated: "use 'morphic generate' to generate migrations",
	Hidden:     true,
	RunE:       runMakemigrationsShim,
}

func init() {
	rootCmd.AddCommand(makemigrationsShimCmd)
	makemigrationsShimCmd.Flags().BoolVar(&makemigrationsShimDryRun, "dry-run", false,
		"Show what would be done without making changes")
}

// runMakemigrationsShim performs the legacy-to-morphic upgrade steps.
func runMakemigrationsShim(cmd *cobra.Command, _ []string) error {
	var actions []string

	// Step 1: Rename config file if needed
	configRenamed, err := shimRenameConfig(cmd, makemigrationsShimDryRun)
	if err != nil {
		return err
	}
	if configRenamed {
		actions = append(actions, "config file renamed")
	}

	// Step 2: Rename DB history table if DATABASE_URL is available
	dbURL := migrate.EnvOr("DATABASE_URL", "")
	if dbURL != "" {
		tableRenamed, dbErr := shimRenameHistoryTable(cmd, dbURL, makemigrationsShimDryRun)
		if dbErr != nil {
			return dbErr
		}
		if tableRenamed {
			actions = append(actions, "history table renamed")
		}
	} else {
		cmd.Println("Skipping DB history table rename: DATABASE_URL not set")
	}

	// Step 3: Rewrite legacy import paths in migration .go files
	rewritten, err := shimRewriteImports(cmd, makemigrationsShimDryRun)
	if err != nil {
		return err
	}
	if rewritten > 0 {
		actions = append(actions, fmt.Sprintf("%d migration file(s) updated", rewritten))
	}

	// Step 4: Print summary
	if len(actions) == 0 {
		cmd.Println("Nothing to do — project is already upgraded.")
	} else if makemigrationsShimDryRun {
		cmd.Println("Dry run complete. No changes were made.")
	} else {
		cmd.Println("Project upgraded to morphic successfully.")
	}
	cmd.Println("")
	cmd.Println("NOTICE: The 'makemigrations' command is deprecated.")
	cmd.Println("Use 'morphic generate' to generate migrations from now on.")

	return nil
}

// shimRenameConfig checks whether the legacy config file exists and the new one
// does not, then renames it. Returns true if a rename was performed (or would be
// performed in dry-run mode).
func shimRenameConfig(cmd *cobra.Command, dryRun bool) (bool, error) {
	migrationsDir := "migrations"
	oldPath := filepath.Join(migrationsDir, "makemigrations.config.yaml")
	newPath := filepath.Join(migrationsDir, "morphic.config.yaml")

	oldExists := fileExists(oldPath)
	newExists := fileExists(newPath)

	if !oldExists {
		cmd.Println("Config: no legacy config file found — nothing to rename.")
		return false, nil
	}
	if newExists {
		cmd.Println("Config: morphic.config.yaml already exists — skipping rename.")
		return false, nil
	}

	if dryRun {
		cmd.Printf("Config: would rename %s -> %s\n", oldPath, newPath)
		return true, nil
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return false, fmt.Errorf("failed to rename config file: %w", err)
	}
	cmd.Printf("Config: renamed %s -> %s\n", oldPath, newPath)
	return true, nil
}

// shimRenameHistoryTable connects to the database and renames the legacy history
// table if it exists and the new table does not. Returns true if a rename was
// performed (or would be performed in dry-run mode).
func shimRenameHistoryTable(cmd *cobra.Command, dbURL string, dryRun bool) (bool, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return false, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	oldExists := tableExists(db, "makemigrations_history")
	newExists := tableExists(db, "morphic_history")

	if !oldExists {
		cmd.Println("DB: no legacy makemigrations_history table found — nothing to rename.")
		return false, nil
	}
	if newExists {
		cmd.Println("DB: morphic_history table already exists — skipping rename.")
		return false, nil
	}

	if dryRun {
		cmd.Println("DB: would rename table makemigrations_history -> morphic_history")
		return true, nil
	}

	if err := RenameHistoryTable(db, "makemigrations_history", "morphic_history"); err != nil {
		return false, err
	}
	cmd.Println("DB: renamed table makemigrations_history -> morphic_history")
	return true, nil
}

// RenameHistoryTable executes ALTER TABLE to rename oldName to newName.
// This is a standalone function (not a provider method) because ALTER TABLE ...
// RENAME TO is standard SQL that works across PostgreSQL, MySQL, SQLite, etc.
func RenameHistoryTable(db *sql.DB, oldName, newName string) error {
	query := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", oldName, newName)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to rename table %s to %s: %w", oldName, newName, err)
	}
	return nil
}

// tableExists checks if a table exists by attempting a SELECT against it.
func tableExists(db *sql.DB, tableName string) bool {
	query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", tableName) //nolint:gosec // table name is a constant, not user input
	_, err := db.Query(query)                                    //nolint:rowserrcheck // we only care about the error
	return err == nil
}

// fileExists returns true if the path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

const (
	legacyImport  = "github.com/ocomsoft/makemigrations/"
	currentImport = "github.com/ocomsoft/morphic/"
)

// shimRewriteImports scans all .go files in the migrations directory and
// rewrites any legacy makemigrations import paths to the morphic equivalents.
// Returns the number of files that were (or would be) modified.
func shimRewriteImports(cmd *cobra.Command, dryRun bool) (int, error) {
	migrationsDir := "migrations"
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.go"))
	if err != nil {
		return 0, fmt.Errorf("scanning migrations directory: %w", err)
	}

	count := 0
	for _, path := range files {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return 0, fmt.Errorf("reading %s: %w", path, readErr)
		}
		if !bytes.Contains(data, []byte(legacyImport)) {
			continue
		}
		updated := strings.ReplaceAll(string(data), legacyImport, currentImport)
		if dryRun {
			cmd.Printf("Imports: would rewrite %s\n", path)
		} else {
			if writeErr := os.WriteFile(path, []byte(updated), 0o644); writeErr != nil {
				return 0, fmt.Errorf("writing %s: %w", path, writeErr)
			}
			cmd.Printf("Imports: rewrote %s\n", path)
		}
		count++
	}

	if count == 0 {
		cmd.Println("Imports: no legacy import paths found — nothing to rewrite.")
	}
	return count, nil
}
