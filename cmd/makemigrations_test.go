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
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// newTestShimCmd creates a fresh cobra.Command wired to runMakemigrationsShim
// with its own output buffer, isolated from the global command tree.
func newTestShimCmd() (*cobra.Command, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{
		Use:  "makemigrations",
		RunE: runMakemigrationsShim,
	}
	cmd.Flags().BoolVar(&makemigrationsShimDryRun, "dry-run", false, "dry run")
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	return cmd, buf
}

// setupConfigDir creates a temp directory with a migrations/ subdirectory and
// writes the old config file. It changes the working directory to the temp dir
// and returns a cleanup function that restores the original working directory.
func setupConfigDir(t *testing.T) (string, func()) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}

	// Write legacy config file
	oldConfig := filepath.Join(migrationsDir, "makemigrations.config.yaml")
	if err := os.WriteFile(oldConfig, []byte("database:\n  type: postgresql\n"), 0644); err != nil {
		t.Fatalf("failed to write old config: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	return tmpDir, func() {
		os.Chdir(origDir) //nolint:errcheck // best-effort cleanup
	}
}

func TestShimRenameConfig(t *testing.T) {
	_, cleanup := setupConfigDir(t)
	defer cleanup()

	// Unset DATABASE_URL so only config rename runs
	t.Setenv("DATABASE_URL", "")

	cmd, buf := newTestShimCmd()
	makemigrationsShimDryRun = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify old file is gone and new file exists
	if fileExists(filepath.Join("migrations", "makemigrations.config.yaml")) {
		t.Error("old config file should have been renamed")
	}
	if !fileExists(filepath.Join("migrations", "morphic.config.yaml")) {
		t.Error("new config file should exist after rename")
	}

	output := buf.String()
	if !containsSubstring(output, "renamed") {
		t.Errorf("expected output to mention rename, got: %s", output)
	}
	if !containsSubstring(output, "morphic generate") {
		t.Errorf("expected deprecation notice, got: %s", output)
	}
}

func TestShimDryRun(t *testing.T) {
	_, cleanup := setupConfigDir(t)
	defer cleanup()

	t.Setenv("DATABASE_URL", "")

	cmd, buf := newTestShimCmd()
	makemigrationsShimDryRun = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old file should still exist
	if !fileExists(filepath.Join("migrations", "makemigrations.config.yaml")) {
		t.Error("old config file should still exist in dry-run mode")
	}
	// New file should NOT exist
	if fileExists(filepath.Join("migrations", "morphic.config.yaml")) {
		t.Error("new config file should not exist in dry-run mode")
	}

	output := buf.String()
	if !containsSubstring(output, "would rename") {
		t.Errorf("expected dry-run output to say 'would rename', got: %s", output)
	}
	if !containsSubstring(output, "Dry run complete") {
		t.Errorf("expected dry-run summary, got: %s", output)
	}
}

func TestShimIdempotent(t *testing.T) {
	_, cleanup := setupConfigDir(t)
	defer cleanup()

	t.Setenv("DATABASE_URL", "")

	// First run: rename happens
	cmd1, _ := newTestShimCmd()
	makemigrationsShimDryRun = false
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	if !fileExists(filepath.Join("migrations", "morphic.config.yaml")) {
		t.Fatal("new config should exist after first run")
	}

	// Second run: should be a no-op
	cmd2, buf2 := newTestShimCmd()
	makemigrationsShimDryRun = false
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	output := buf2.String()
	if !containsSubstring(output, "Nothing to do") {
		t.Errorf("expected no-op message on second run, got: %s", output)
	}
}

func TestShimNoLegacyConfig(t *testing.T) {
	// Create a temp dir with only the new config — no old config at all
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("failed to create migrations dir: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origDir) //nolint:errcheck

	t.Setenv("DATABASE_URL", "")

	cmd, buf := newTestShimCmd()
	makemigrationsShimDryRun = false
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !containsSubstring(output, "no legacy config") {
		t.Errorf("expected no-legacy-config message, got: %s", output)
	}
	if !containsSubstring(output, "Nothing to do") {
		t.Errorf("expected nothing-to-do message, got: %s", output)
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Non-existent file
	if fileExists(filepath.Join(tmpDir, "nope.txt")) {
		t.Error("expected false for non-existent file")
	}

	// Existing file
	f := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(f, []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(f) {
		t.Error("expected true for existing file")
	}

	// Directory should return false
	if fileExists(tmpDir) {
		t.Error("expected false for directory")
	}
}

// containsSubstring returns true if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && bytes.Contains([]byte(s), []byte(substr))
}
