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
package dumpdata_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ocomsoft/makemigrations/internal/dumpdata"
)

// TestDetectPrimaryKeys_SQLite verifies that DetectPrimaryKeys correctly
// identifies the primary key column for a SQLite table.
func TestDetectPrimaryKeys_SQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite3: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE test_pk (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	pks, err := dumpdata.DetectPrimaryKeys(db, "sqlite", "test_pk")
	if err != nil {
		t.Fatalf("DetectPrimaryKeys returned error: %v", err)
	}

	if len(pks) != 1 {
		t.Fatalf("expected 1 primary key, got %d", len(pks))
	}

	if pks[0] != "id" {
		t.Errorf("expected primary key 'id', got %q", pks[0])
	}
}

// TestFetchRows_SQLite verifies that FetchRows returns the correct number of
// rows and columns from a SQLite table.
func TestFetchRows_SQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite3: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, price REAL)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec(`INSERT INTO items (id, name, price) VALUES (1, 'Widget', 9.99), (2, 'Gadget', 19.99)`)
	if err != nil {
		t.Fatalf("failed to insert rows: %v", err)
	}

	rows, cols, err := dumpdata.FetchRows(db, "items")
	if err != nil {
		t.Fatalf("FetchRows returned error: %v", err)
	}

	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}

	if len(cols) != 3 {
		t.Errorf("expected 3 columns, got %d", len(cols))
	}

	// Verify column names are present
	expectedCols := map[string]bool{"id": true, "name": true, "price": true}
	for _, col := range cols {
		if !expectedCols[col] {
			t.Errorf("unexpected column name: %q", col)
		}
	}

	// Verify row data is accessible
	for i, row := range rows {
		if row["id"] == nil {
			t.Errorf("row %d: id is nil", i)
		}
		if row["name"] == nil {
			t.Errorf("row %d: name is nil", i)
		}
		if row["price"] == nil {
			t.Errorf("row %d: price is nil", i)
		}
	}
}

// TestNormalizeValue verifies the type conversion rules for NormalizeValue.
func TestNormalizeValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "nil stays nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "[]byte to string",
			input:    []byte("hello"),
			expected: "hello",
		},
		{
			name:     "int64 passthrough",
			input:    int64(42),
			expected: int64(42),
		},
		{
			name:     "float64 passthrough",
			input:    float64(3.14),
			expected: float64(3.14),
		},
		{
			name:     "bool passthrough",
			input:    true,
			expected: true,
		},
		{
			name:     "time.Time to formatted string",
			input:    time.Date(2024, 1, 15, 10, 30, 45, 123456000, time.UTC),
			expected: "2024-01-15 10:30:45.123456",
		},
		{
			name:     "other type to string via Sprintf",
			input:    struct{ X int }{X: 5},
			expected: "{5}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dumpdata.NormalizeValue(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeValue(%v) = %v (%T), want %v (%T)",
					tt.input, result, result, tt.expected, tt.expected)
			}
		})
	}
}

// TestDetectPrimaryKeys_UnsupportedDB verifies that an unsupported database
// type returns nil, nil rather than an error.
func TestDetectPrimaryKeys_UnsupportedDB(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite3: %v", err)
	}
	defer func() { _ = db.Close() }()

	pks, err := dumpdata.DetectPrimaryKeys(db, "clickhouse", "any_table")
	if err != nil {
		t.Errorf("expected nil error for unsupported DB, got: %v", err)
	}
	if pks != nil {
		t.Errorf("expected nil primary keys for unsupported DB, got: %v", pks)
	}
}
