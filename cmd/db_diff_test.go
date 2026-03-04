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
	"encoding/json"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	yamlpkg "github.com/ocomsoft/makemigrations/internal/yaml"
)

func init() {
	// Disable color output so ANSI escape codes do not interfere with
	// string matching in test assertions.
	color.NoColor = true
}

// TestDBDiffCommandRegistered verifies that the db-diff command is registered
// as a subcommand of rootCmd.
func TestDBDiffCommandRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "db-diff" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'db-diff' command to be registered on rootCmd, but it was not found")
	}
}

// TestDBDiffCommandHasRequiredFlags checks that the db-diff command exposes all
// required flags for database connection and output configuration.
func TestDBDiffCommandHasRequiredFlags(t *testing.T) {
	var dbDiff *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "db-diff" {
			dbDiff = c
			break
		}
	}
	if dbDiff == nil {
		t.Fatal("db-diff command not found on rootCmd")
	}

	requiredFlags := []string{
		"host",
		"port",
		"database",
		"username",
		"password",
		"sslmode",
		"db-type",
		"format",
		"verbose",
	}

	for _, name := range requiredFlags {
		if dbDiff.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q on db-diff command, but it was not found", name)
		}
	}
}

// TestNormalizeDBSchema verifies that SQL-native types returned by database
// introspection are mapped to the canonical YAML schema types.
func TestNormalizeDBSchema(t *testing.T) {
	schema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "name", Type: "character varying"},
					{Name: "count", Type: "int4"},
					{Name: "score", Type: "double precision"},
					{Name: "active", Type: "bool"},
					{Name: "created_at", Type: "timestamp without time zone"},
					{Name: "bio", Type: "text"},
					{Name: "price", Type: "numeric"},
					{Name: "data", Type: "jsonb"},
					{Name: "uid", Type: "uuid"},
					{Name: "big_count", Type: "int8"},
					{Name: "small_count", Type: "int2"},
				},
			},
		},
	}

	normalizeDBSchema(&schema)

	expected := map[string]string{
		"name":        "varchar",
		"count":       "integer",
		"score":       "float",
		"active":      "boolean",
		"created_at":  "timestamp",
		"bio":         "text",
		"price":       "decimal",
		"data":        "jsonb",
		"uid":         "uuid",
		"big_count":   "bigint",
		"small_count": "integer",
	}

	for _, field := range schema.Tables[0].Fields {
		want, ok := expected[field.Name]
		if !ok {
			t.Errorf("unexpected field %q in test schema", field.Name)
			continue
		}
		if field.Type != want {
			t.Errorf("field %q: expected type %q, got %q", field.Name, want, field.Type)
		}
	}
}

// TestFormatDBDiff_NoChanges verifies that the formatter outputs a "no
// differences" message when the diff contains no changes.
func TestFormatDBDiff_NoChanges(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: false,
		Changes:    []yamlpkg.Change{},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "No differences") {
		t.Errorf("expected output to contain 'No differences', got:\n%s", output)
	}
}

// TestFormatDBDiff_MissingTable verifies that a table_removed change is
// reported as a missing table in the formatted output.
func TestFormatDBDiff_MissingTable(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeTableRemoved,
				TableName:   "audit_log",
				Description: "Table audit_log exists in schema but not in database",
				Destructive: true,
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "audit_log") {
		t.Errorf("expected output to contain 'audit_log', got:\n%s", output)
	}
	if !strings.Contains(output, "Missing") {
		t.Errorf("expected output to contain 'Missing', got:\n%s", output)
	}
}

// TestFormatDBDiff_ExtraTable verifies that a table_added change is reported
// as an extra table in the formatted output.
func TestFormatDBDiff_ExtraTable(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeTableAdded,
				TableName:   "temp_cache",
				Description: "Table temp_cache exists in database but not in schema",
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "temp_cache") {
		t.Errorf("expected output to contain 'temp_cache', got:\n%s", output)
	}
	if !strings.Contains(output, "Extra") {
		t.Errorf("expected output to contain 'Extra', got:\n%s", output)
	}
}

// TestFormatDBDiff_FieldDiff verifies that a field-level change includes both
// the table name and the field name in the formatted output.
func TestFormatDBDiff_FieldDiff(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeFieldRemoved,
				TableName:   "users",
				FieldName:   "deleted_at",
				Description: "Field deleted_at exists in schema but not in database",
				Destructive: true,
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "users") {
		t.Errorf("expected output to contain 'users', got:\n%s", output)
	}
	if !strings.Contains(output, "deleted_at") {
		t.Errorf("expected output to contain 'deleted_at', got:\n%s", output)
	}
}

// TestFormatDBDiff_Summary verifies that the summary section of the output
// includes the total count of changes.
func TestFormatDBDiff_Summary(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges:    true,
		IsDestructive: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeTableRemoved,
				TableName:   "audit_log",
				Description: "Table audit_log exists in schema but not in database",
				Destructive: true,
			},
			{
				Type:        yamlpkg.ChangeTypeTableAdded,
				TableName:   "temp_cache",
				Description: "Table temp_cache exists in database but not in schema",
			},
			{
				Type:        yamlpkg.ChangeTypeFieldRemoved,
				TableName:   "users",
				FieldName:   "deleted_at",
				Description: "Field deleted_at exists in schema but not in database",
				Destructive: true,
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "3") {
		t.Errorf("expected output to contain '3' (total change count), got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// Integration tests for runDBDiffWithSchemas
// ---------------------------------------------------------------------------

// TestRunDBDiffWithSchemas_NoDiff verifies that two identical schemas produce
// no drift error and the output reports "No differences".
func TestRunDBDiffWithSchemas_NoDiff(t *testing.T) {
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	}
	dbSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)

	if err != nil {
		t.Fatalf("expected no error for identical schemas, got: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "No differences") {
		t.Errorf("expected output to contain 'No differences', got:\n%s", output)
	}
}

// TestRunDBDiffWithSchemas_MissingTable verifies that a table present in the
// DAG schema but absent from the DB schema is reported as missing drift.
func TestRunDBDiffWithSchemas_MissingTable(t *testing.T) {
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
				},
			},
			{
				Name: "audit_log",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
				},
			},
		},
	}
	dbSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)

	if err == nil {
		t.Fatal("expected an error indicating drift, got nil")
	}
	output := buf.String()
	if !strings.Contains(output, "audit_log") {
		t.Errorf("expected output to contain 'audit_log', got:\n%s", output)
	}
	if !strings.Contains(output, "Missing") {
		t.Errorf("expected output to contain 'Missing', got:\n%s", output)
	}
}

// TestRunDBDiffWithSchemas_ExtraTable verifies that a table present in the DB
// but absent from the DAG schema is reported as extra drift.
func TestRunDBDiffWithSchemas_ExtraTable(t *testing.T) {
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
				},
			},
		},
	}
	dbSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
				},
			},
			{
				Name: "legacy_cache",
				Fields: []yamlpkg.Field{
					{Name: "key", Type: "varchar", Length: 255},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)

	if err == nil {
		t.Fatal("expected an error indicating drift, got nil")
	}
	output := buf.String()
	if !strings.Contains(output, "legacy_cache") {
		t.Errorf("expected output to contain 'legacy_cache', got:\n%s", output)
	}
	if !strings.Contains(output, "Extra") {
		t.Errorf("expected output to contain 'Extra', got:\n%s", output)
	}
}

// TestRunDBDiffWithSchemas_TypeNormalization verifies that SQL-native types
// in the DB schema are normalized to match the canonical DAG types, resulting
// in no drift when the underlying types are semantically equivalent.
func TestRunDBDiffWithSchemas_TypeNormalization(t *testing.T) {
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "products",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
					{Name: "name", Type: "varchar", Length: 255},
					{Name: "price", Type: "decimal"},
					{Name: "active", Type: "boolean"},
				},
			},
		},
	}
	dbSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "products",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid"},
					{Name: "name", Type: "character varying", Length: 255},
					{Name: "price", Type: "numeric"},
					{Name: "active", Type: "bool"},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)

	if err != nil {
		t.Fatalf("expected no error after type normalization, got: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "No differences") {
		t.Errorf("expected output to contain 'No differences' after normalization, got:\n%s", output)
	}
}

// TestRunDBDiffWithSchemas_JSONFormat verifies that JSON output mode produces
// valid JSON that can be unmarshalled into a SchemaDiff.
func TestRunDBDiffWithSchemas_JSONFormat(t *testing.T) {
	dagSchema := &yamlpkg.Schema{}
	dbSchema := &yamlpkg.Schema{}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, dagSchema, dbSchema, "json", false)

	if err != nil {
		t.Fatalf("expected no error for empty schemas with JSON format, got: %v", err)
	}

	output := buf.Bytes()
	var result yamlpkg.SchemaDiff
	if jsonErr := json.Unmarshal(output, &result); jsonErr != nil {
		t.Errorf("expected valid JSON output, got unmarshal error: %v\nraw output:\n%s", jsonErr, string(output))
	}
}

// TestFormatDBDiff_IndexAdded verifies that an index_added change is reported
// in an "Index Differences" section rather than "Field Differences".
func TestFormatDBDiff_IndexAdded(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeIndexAdded,
				TableName:   "users",
				FieldName:   "idx_users_email",
				Description: "Add index 'idx_users_email' on table 'users'",
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "Index Differences") {
		t.Errorf("expected output to contain 'Index Differences', got:\n%s", output)
	}
	if !strings.Contains(output, "idx_users_email") {
		t.Errorf("expected output to contain 'idx_users_email', got:\n%s", output)
	}
	if strings.Contains(output, "Field Differences") {
		t.Errorf("index changes should NOT appear under 'Field Differences', got:\n%s", output)
	}
}

// TestFormatDBDiff_IndexRemoved verifies that an index_removed change is
// reported in the "Index Differences" section.
func TestFormatDBDiff_IndexRemoved(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges:    true,
		IsDestructive: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeIndexRemoved,
				TableName:   "orders",
				FieldName:   "idx_orders_status",
				Description: "Remove index 'idx_orders_status' from table 'orders'",
				Destructive: true,
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "Index Differences") {
		t.Errorf("expected output to contain 'Index Differences', got:\n%s", output)
	}
	if !strings.Contains(output, "idx_orders_status") {
		t.Errorf("expected output to contain 'idx_orders_status', got:\n%s", output)
	}
}

// TestFormatDBDiff_ForeignKeyDiff verifies that foreign key changes are
// reported in a "Foreign Key Differences" section.
func TestFormatDBDiff_ForeignKeyDiff(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:        yamlpkg.ChangeTypeFieldModified,
				TableName:   "orders",
				FieldName:   "user_id",
				Description: "Change field 'orders.user_id' foreign key from users to none",
				OldValue:    &yamlpkg.ForeignKey{Table: "users", OnDelete: "CASCADE"},
				NewValue:    nil,
				Destructive: true,
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "Foreign Key Differences") {
		t.Errorf("expected output to contain 'Foreign Key Differences', got:\n%s", output)
	}
	if !strings.Contains(output, "user_id") {
		t.Errorf("expected output to contain 'user_id', got:\n%s", output)
	}
}

// TestFormatDBDiff_SummaryWithIndexAndFK verifies that the summary section
// includes separate counts for index and foreign key changes.
func TestFormatDBDiff_SummaryWithIndexAndFK(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges:    true,
		IsDestructive: true,
		Changes: []yamlpkg.Change{
			{
				Type:      yamlpkg.ChangeTypeFieldRemoved,
				TableName: "users", FieldName: "deleted_at",
				Destructive: true,
			},
			{
				Type:      yamlpkg.ChangeTypeIndexAdded,
				TableName: "users", FieldName: "idx_users_email",
			},
			{
				Type:      yamlpkg.ChangeTypeIndexRemoved,
				TableName: "orders", FieldName: "idx_orders_old",
				Destructive: true,
			},
			{
				Type:      yamlpkg.ChangeTypeFieldModified,
				TableName: "orders", FieldName: "user_id",
				Description: "Change field 'orders.user_id' foreign key from users to none",
				OldValue:    &yamlpkg.ForeignKey{Table: "users", OnDelete: "CASCADE"},
				NewValue:    nil,
				Destructive: true,
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "Index changes") {
		t.Errorf("expected summary to contain 'Index changes', got:\n%s", output)
	}
	if !strings.Contains(output, "FK changes") {
		t.Errorf("expected summary to contain 'FK changes', got:\n%s", output)
	}
}

// TestRunDBDiff_UnsupportedProvider verifies that attempting to run db-diff
// with a non-PostgreSQL database type returns a clear, actionable error message
// rather than the raw "not implemented yet" stub error from the provider.
func TestRunDBDiff_UnsupportedProvider(t *testing.T) {
	// Save and restore the global databaseType flag value
	orig := databaseType
	defer func() { databaseType = orig }()

	databaseType = "mysql"

	// Execute the command via rootCmd so the full Cobra flag/RunE path is exercised.
	rootCmd.SetArgs([]string{"db-diff", "--db-type=mysql"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !strings.Contains(err.Error(), "not yet support") {
		t.Errorf("expected 'not yet support' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mysql") {
		t.Errorf("expected 'mysql' in error, got: %v", err)
	}
}
