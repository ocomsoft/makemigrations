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
package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/cmd"
)

func writeSQLFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func TestMigrateToGo_NoSQLFiles(t *testing.T) {
	dir := t.TempDir()
	err := cmd.ExecuteMigrateToGo(dir, true, true, false)
	if err == nil {
		t.Fatal("expected error when no SQL files present")
	}
}

func TestMigrateToGo_GeneratesGoFiles(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE users;
`)
	writeSQLFile(t, dir, "00002_add_phone.sql", `-- +goose Up
ALTER TABLE users ADD COLUMN phone TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN phone;
`)

	err := cmd.ExecuteMigrateToGo(dir, false, true, false)
	if err != nil {
		t.Fatalf("ExecuteMigrateToGo: %v", err)
	}

	for _, name := range []string{"0001_initial.go", "0002_add_phone.go", "main.go", "go.mod"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to be created", name)
		}
	}
}

func TestMigrateToGo_SQLDeletedAfterConversion(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE users;
`)

	err := cmd.ExecuteMigrateToGo(dir, false, true, false)
	if err != nil {
		t.Fatalf("ExecuteMigrateToGo: %v", err)
	}

	sqlPath := filepath.Join(dir, "00001_initial.sql")
	if _, err := os.Stat(sqlPath); !os.IsNotExist(err) {
		t.Error("expected SQL file to be deleted after successful conversion")
	}
}

func TestMigrateToGo_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE users;
`)

	err := cmd.ExecuteMigrateToGo(dir, true, true, false)
	if err != nil {
		t.Fatalf("ExecuteMigrateToGo dry-run: %v", err)
	}

	goPath := filepath.Join(dir, "0001_initial.go")
	if _, err := os.Stat(goPath); !os.IsNotExist(err) {
		t.Error("dry-run: expected no .go files to be written")
	}

	sqlPath := filepath.Join(dir, "00001_initial.sql")
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		t.Error("dry-run: expected SQL file to be preserved")
	}
}

func TestMigrateToGo_NoDelete(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE users;
`)

	err := cmd.ExecuteMigrateToGo(dir, false, true, true)
	if err != nil {
		t.Fatalf("ExecuteMigrateToGo: %v", err)
	}

	sqlPath := filepath.Join(dir, "00001_initial.sql")
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		t.Error("no-delete: expected SQL file to be preserved")
	}
}

func TestMigrateToGo_GeneratedContentIsValid(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE users;
`)

	if err := cmd.ExecuteMigrateToGo(dir, false, true, true); err != nil {
		t.Fatalf("ExecuteMigrateToGo: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "0001_initial.go"))
	if err != nil {
		t.Fatalf("reading 0001_initial.go: %v", err)
	}

	src := string(content)
	for _, want := range []string{
		"package main",
		`"0001_initial"`,
		"RunSQL",
		"CREATE TABLE users",
		"DROP TABLE users",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated file missing %q", want)
		}
	}
	// Verify Name field is present with the correct value (gofmt aligns fields)
	if !strings.Contains(src, "Name:") {
		t.Error("generated file missing Name field")
	}
}

func TestMigrateToGo_ExistingGoFilesBlocked(t *testing.T) {
	dir := t.TempDir()

	writeSQLFile(t, dir, "00001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);
`)
	if err := os.WriteFile(filepath.Join(dir, "0001_initial.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	err := cmd.ExecuteMigrateToGo(dir, false, true, false)
	if err == nil {
		t.Fatal("expected error when .go migration files already exist")
	}
}
