package gooseparser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/gooseparser"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
	return path
}

func TestParseFile_BasicUpDown(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "0001_initial.sql", `-- +goose Up
CREATE TABLE users (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE users;
`)
	got, err := gooseparser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if want := "CREATE TABLE users (id INTEGER PRIMARY KEY);"; got.ForwardSQL != want {
		t.Errorf("ForwardSQL:\ngot  %q\nwant %q", got.ForwardSQL, want)
	}
	if want := "DROP TABLE users;"; got.BackwardSQL != want {
		t.Errorf("BackwardSQL:\ngot  %q\nwant %q", got.BackwardSQL, want)
	}
}

func TestParseFile_StatementBeginEnd(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "0002_complex.sql", `-- +goose Up
-- +goose StatementBegin
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    body TEXT
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE posts;
-- +goose StatementEnd
`)
	got, err := gooseparser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got.ForwardSQL == "" {
		t.Error("ForwardSQL should not be empty")
	}
	if strings.Contains(got.ForwardSQL, "StatementBegin") {
		t.Error("ForwardSQL should not contain StatementBegin marker")
	}
	if strings.Contains(got.BackwardSQL, "StatementEnd") {
		t.Error("BackwardSQL should not contain StatementEnd marker")
	}
	wantForward := "CREATE TABLE posts (\n    id INTEGER PRIMARY KEY,\n    body TEXT\n);"
	if got.ForwardSQL != wantForward {
		t.Errorf("ForwardSQL:\ngot  %q\nwant %q", got.ForwardSQL, wantForward)
	}
	wantBackward := "DROP TABLE posts;"
	if got.BackwardSQL != wantBackward {
		t.Errorf("BackwardSQL:\ngot  %q\nwant %q", got.BackwardSQL, wantBackward)
	}
}

func TestParseFile_NoDownSection(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "0003_irreversible.sql", `-- +goose Up
INSERT INTO config (key, value) VALUES ('version', '2');
`)
	got, err := gooseparser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if got.ForwardSQL == "" {
		t.Error("ForwardSQL should not be empty")
	}
	if got.BackwardSQL != "" {
		t.Errorf("BackwardSQL should be empty, got %q", got.BackwardSQL)
	}
}

func TestParseFile_MultipleStatements(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "0004_multi.sql", `-- +goose Up
CREATE TABLE a (id INTEGER PRIMARY KEY);
CREATE TABLE b (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE b;
DROP TABLE a;
`)
	got, err := gooseparser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if !strings.Contains(got.ForwardSQL, "CREATE TABLE a") {
		t.Error("ForwardSQL missing CREATE TABLE a")
	}
	if !strings.Contains(got.ForwardSQL, "CREATE TABLE b") {
		t.Error("ForwardSQL missing CREATE TABLE b")
	}
}

func TestExtractDescription(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"20240101120000_initial.sql", "initial"},
		{"00001_initial.sql", "initial"},
		{"20240102_add_phone_field.sql", "add_phone_field"},
		{"0001_my_migration.sql", "my_migration"},
		{"justname.sql", "justname"},
	}
	for _, tt := range tests {
		got := gooseparser.ExtractDescription(tt.filename)
		if got != tt.want {
			t.Errorf("ExtractDescription(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}

func TestExtractVersionID(t *testing.T) {
	tests := []struct {
		filename string
		want     int64
		wantErr  bool
	}{
		{"20240101120000_initial.sql", 20240101120000, false},
		{"00001_initial.sql", 1, false},
		{"0003_add_phone.sql", 3, false},
		{"notanumber_bad.sql", 0, true},
	}
	for _, tt := range tests {
		got, err := gooseparser.ExtractVersionID(tt.filename)
		if (err != nil) != tt.wantErr {
			t.Errorf("ExtractVersionID(%q) error = %v, wantErr %v", tt.filename, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ExtractVersionID(%q) = %d, want %d", tt.filename, got, tt.want)
		}
	}
}

