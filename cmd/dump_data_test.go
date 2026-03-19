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
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// resetDumpDataFlags resets all package-level flag variables used by the
// dump-data command to their zero values. This prevents test pollution
// when rootCmd is reused across tests.
func resetDumpDataFlags() {
	dumpDataName = ""
	dumpDataDryRun = false
	dumpDataVerbose = false
	dumpDataConflictKey = nil
	dumpDataDSN = ""
	configFile = ""
}

// createTestSQLiteDB creates a SQLite database at the given path, executes
// the provided setup SQL, and returns the closed database path. It fails the
// test if any SQL operation encounters an error.
func createTestSQLiteDB(t *testing.T, dbPath, setupSQL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("opening SQLite DB: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(setupSQL); err != nil {
		t.Fatalf("executing setup SQL: %v", err)
	}
}

// writeTestConfig writes a minimal makemigrations config YAML to the given
// path with the specified migrations directory.
func writeTestConfig(t *testing.T, cfgPath, migrationsDir string) {
	t.Helper()

	content := "database:\n  type: sqlite\nmigration:\n  directory: " + migrationsDir + "\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
}

// captureStdout redirects os.Stdout to a pipe, executes the given function,
// then restores stdout and returns the captured output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	return buf.String()
}

// TestDumpData_DryRun verifies that dump-data with --dry-run prints the
// generated migration source to stdout without writing any file.
func TestDumpData_DryRun(t *testing.T) {
	t.Cleanup(func() {
		resetDumpDataFlags()
		rootCmd.SetArgs([]string{})
	})

	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	dbPath := filepath.Join(tmpDir, "test.db")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create SQLite DB with a colours table.
	createTestSQLiteDB(t, dbPath, `
		CREATE TABLE colours (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
		INSERT INTO colours (id, name) VALUES (1, 'Red');
		INSERT INTO colours (id, name) VALUES (2, 'Blue');
	`)

	// Write config file.
	writeTestConfig(t, cfgPath, migrationsDir)

	// Execute dump-data in dry-run mode with stdout capture.
	// Use --conflict-key since there are no existing migrations to derive PKs from.
	rootCmd.SetArgs([]string{
		"--config", cfgPath,
		"dump-data", "colours",
		"--dsn", dbPath,
		"--conflict-key", "id",
		"--dry-run",
	})

	output := captureStdout(t, func() {
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("rootCmd.Execute: %v", err)
		}
	})

	// Verify expected content in the generated migration source.
	if !strings.Contains(output, "colours") {
		t.Errorf("expected output to contain table name 'colours', got:\n%s", output)
	}

	if !strings.Contains(output, "Red") {
		t.Errorf("expected output to contain row value 'Red', got:\n%s", output)
	}

	if !strings.Contains(output, "UpsertData") {
		t.Errorf("expected output to contain 'UpsertData', got:\n%s", output)
	}

	if !strings.Contains(output, "ConflictKeys") {
		t.Errorf("expected output to contain 'ConflictKeys', got:\n%s", output)
	}
}

// TestDumpData_MissingArg verifies that dump-data fails when no table name
// argument is provided (cobra.MinimumNArgs(1) rejects it).
func TestDumpData_MissingArg(t *testing.T) {
	t.Cleanup(func() {
		resetDumpDataFlags()
		rootCmd.SetArgs([]string{})
	})

	rootCmd.SetArgs([]string{"dump-data"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no table name argument provided, got nil")
	}
}

// TestDumpData_NoMigrationsDir_PKFromFlag verifies the fallback path where
// there are no existing migrations and primary keys come from --conflict-key.
func TestDumpData_NoMigrationsDir_PKFromFlag(t *testing.T) {
	t.Cleanup(func() {
		resetDumpDataFlags()
		rootCmd.SetArgs([]string{})
	})

	tmpDir := t.TempDir()
	// Point migrations dir to a path that does not exist.
	migrationsDir := filepath.Join(tmpDir, "nonexistent_migrations")
	dbPath := filepath.Join(tmpDir, "test.db")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Create SQLite DB with a widgets table.
	createTestSQLiteDB(t, dbPath, `
		CREATE TABLE widgets (id INTEGER PRIMARY KEY, colour TEXT);
		INSERT INTO widgets (id, colour) VALUES (1, 'green');
	`)

	// Write config file.
	writeTestConfig(t, cfgPath, migrationsDir)

	// Execute dump-data with --conflict-key override and --dry-run.
	rootCmd.SetArgs([]string{
		"--config", cfgPath,
		"dump-data", "widgets",
		"--dsn", dbPath,
		"--conflict-key", "id",
		"--dry-run",
	})

	output := captureStdout(t, func() {
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("rootCmd.Execute: %v", err)
		}
	})

	// Verify expected content in the generated migration source.
	if !strings.Contains(output, "widgets") {
		t.Errorf("expected output to contain table name 'widgets', got:\n%s", output)
	}

	if !strings.Contains(output, "green") {
		t.Errorf("expected output to contain row value 'green', got:\n%s", output)
	}
}
