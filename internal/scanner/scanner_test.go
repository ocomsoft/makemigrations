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
package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/errors"
)

func TestScanner_ScanModules_ValidModule(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "scanner_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create go.mod
	goModContent := `module test/module

go 1.21

require (
	github.com/example/dep v1.0.0
)
`
	err = os.WriteFile("go.mod", []byte(goModContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create schema.sql
	sqlDir := filepath.Join("sql")
	os.MkdirAll(sqlDir, 0755)
	schemaContent := `-- MIGRATION_SCHEMA
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL
);
`
	err = os.WriteFile(filepath.Join(sqlDir, "schema.sql"), []byte(schemaContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	scanner := New(false)
	schemas, err := scanner.ScanModules()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(schemas) != 1 {
		t.Fatalf("Expected 1 schema, got %d", len(schemas))
	}

	schema := schemas[0]
	if schema.ModulePath != "current module" {
		t.Errorf("Expected module path 'current module', got '%s'", schema.ModulePath)
	}

	if !schema.HasMarker {
		t.Error("Expected schema to have MIGRATION_SCHEMA marker")
	}

	if !strings.Contains(schema.Content, "CREATE TABLE users") {
		t.Error("Expected schema content to contain CREATE TABLE users")
	}
}

func TestScanner_ScanModules_NoGoMod(t *testing.T) {
	// Create temporary directory without go.mod
	tmpDir, err := os.MkdirTemp("", "scanner_test_no_mod")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	scanner := New(false)
	_, err = scanner.ScanModules()

	if err == nil {
		t.Fatal("Expected error for missing go.mod")
	}

	if !errors.IsValidationError(err) {
		t.Errorf("Expected ValidationError, got %T", err)
	}
}

func TestScanner_ScanModules_EmptyGoMod(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "scanner_test_empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create empty go.mod
	err = os.WriteFile("go.mod", []byte(""), 0644)
	if err != nil {
		t.Fatal(err)
	}

	scanner := New(false)
	_, err = scanner.ScanModules()

	if err == nil {
		t.Fatal("Expected error for empty go.mod")
	}

	if !errors.IsValidationError(err) {
		t.Errorf("Expected ValidationError, got %T", err)
	}
}

func TestScanner_readSchemaFile(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedMarker bool
	}{
		{
			name: "with marker",
			content: `-- MIGRATION_SCHEMA
CREATE TABLE test (id INT);`,
			expectedMarker: true,
		},
		{
			name:           "without marker",
			content:        `CREATE TABLE test (id INT);`,
			expectedMarker: false,
		},
		{
			name: "marker with whitespace",
			content: `   -- MIGRATION_SCHEMA   
CREATE TABLE test (id INT);`,
			expectedMarker: true,
		},
	}

	scanner := New(false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			content, hasMarker, err := scanner.readSchemaFile(reader)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if hasMarker != tt.expectedMarker {
				t.Errorf("Expected marker %v, got %v", tt.expectedMarker, hasMarker)
			}

			if content != tt.content {
				t.Errorf("Expected content %q, got %q", tt.content, content)
			}
		})
	}
}
