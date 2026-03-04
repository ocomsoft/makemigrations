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
// TestRunDBDiffWithSchemas_IndexDiff verifies that index differences between
// DAG and DB schemas are detected and reported.
func TestRunDBDiffWithSchemas_IndexDiff(t *testing.T) {
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
				Indexes: []yamlpkg.Index{
					{Name: "idx_users_email", Fields: []string{"email"}, Unique: true},
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
				// No indexes in live DB
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)

	if err == nil {
		t.Fatal("expected drift error for missing index")
	}
	output := buf.String()
	if !strings.Contains(output, "Index Differences") {
		t.Errorf("expected 'Index Differences' section, got:\n%s", output)
	}
	if !strings.Contains(output, "idx_users_email") {
		t.Errorf("expected 'idx_users_email' in output, got:\n%s", output)
	}
}

// TestRunDBDiffWithSchemas_IndexMatch verifies that matching indexes produce
// no drift.
func TestRunDBDiffWithSchemas_IndexMatch(t *testing.T) {
	idx := []yamlpkg.Index{
		{Name: "idx_users_email", Fields: []string{"email"}, Unique: true},
	}
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
				Indexes: idx,
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
				Indexes: idx,
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)

	if err != nil {
		t.Fatalf("expected no error for matching indexes, got: %v", err)
	}
	if !strings.Contains(buf.String(), "No differences") {
		t.Errorf("expected 'No differences', got:\n%s", buf.String())
	}
}

// TestRunDBDiffWithSchemas_ForeignKeyDiff verifies that foreign key differences
// between DAG and DB schemas are detected and reported.
func TestRunDBDiffWithSchemas_ForeignKeyDiff(t *testing.T) {
	nullable := true
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
				},
			},
			{
				Name: "orders",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{
						Name: "user_id", Type: "foreign_key",
						Nullable:   &nullable,
						ForeignKey: &yamlpkg.ForeignKey{Table: "users", OnDelete: "CASCADE"},
					},
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
				},
			},
			{
				Name: "orders",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{
						Name: "user_id", Type: "foreign_key",
						Nullable:   &nullable,
						ForeignKey: &yamlpkg.ForeignKey{Table: "users", OnDelete: "SET NULL"},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)

	if err == nil {
		t.Fatal("expected drift error for FK difference")
	}
	output := buf.String()
	if !strings.Contains(output, "Foreign Key Differences") {
		t.Errorf("expected 'Foreign Key Differences' section, got:\n%s", output)
	}
	if !strings.Contains(output, "user_id") {
		t.Errorf("expected 'user_id' in output, got:\n%s", output)
	}
}

// TestRunDBDiffWithSchemas_ForeignKeyMatch verifies that matching foreign keys
// produce no drift.
func TestRunDBDiffWithSchemas_ForeignKeyMatch(t *testing.T) {
	nullable := false
	fk := &yamlpkg.ForeignKey{Table: "users", OnDelete: "CASCADE"}
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
				},
			},
			{
				Name: "orders",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "user_id", Type: "foreign_key", Nullable: &nullable, ForeignKey: fk},
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
				},
			},
			{
				Name: "orders",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "user_id", Type: "foreign_key", Nullable: &nullable, ForeignKey: fk},
				},
			},
		},
	}

	var buf bytes.Buffer
	err := runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "text", false)

	if err != nil {
		t.Fatalf("expected no error for matching FKs, got: %v", err)
	}
	if !strings.Contains(buf.String(), "No differences") {
		t.Errorf("expected 'No differences', got:\n%s", buf.String())
	}
}

// TestRunDBDiffWithSchemas_JSONIncludesIndexAndFK verifies that JSON output
// includes index and foreign key changes.
func TestRunDBDiffWithSchemas_JSONIncludesIndexAndFK(t *testing.T) {
	dagSchema := yamlpkg.Schema{
		Tables: []yamlpkg.Table{
			{
				Name: "users",
				Fields: []yamlpkg.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
				},
				Indexes: []yamlpkg.Index{
					{Name: "idx_test", Fields: []string{"id"}, Unique: false},
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
				},
				// No indexes
			},
		},
	}

	var buf bytes.Buffer
	_ = runDBDiffWithSchemas(&buf, &dagSchema, &dbSchema, "json", false)

	var result yamlpkg.SchemaDiff
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if !result.HasChanges {
		t.Error("expected has_changes=true")
	}
	found := false
	for _, ch := range result.Changes {
		if ch.Type == yamlpkg.ChangeTypeIndexRemoved && ch.FieldName == "idx_test" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected index_removed change for idx_test in JSON output")
	}
}

// TestGenerateTableSnippet verifies that a Table struct is marshalled to a
// pasteable YAML snippet with proper indentation under a tables: key.
func TestGenerateTableSnippet(t *testing.T) {
	table := yamlpkg.Table{
		Name: "orders",
		Fields: []yamlpkg.Field{
			{Name: "id", Type: "uuid", PrimaryKey: true},
			{Name: "total", Type: "decimal", Precision: 10, Scale: 2},
		},
		Indexes: []yamlpkg.Index{
			{Name: "idx_orders_total", Fields: []string{"total"}},
		},
	}

	snippet, err := generateTableSnippet(table)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(snippet, "- name: orders") {
		t.Errorf("expected '- name: orders' in snippet, got:\n%s", snippet)
	}
	if !strings.Contains(snippet, "type: uuid") {
		t.Errorf("expected 'type: uuid' in snippet, got:\n%s", snippet)
	}
	if !strings.Contains(snippet, "idx_orders_total") {
		t.Errorf("expected 'idx_orders_total' in snippet, got:\n%s", snippet)
	}
}

// TestGenerateFieldSnippet verifies that a Field struct is marshalled to YAML
// wrapped in a minimal table context showing which table it belongs to.
func TestGenerateFieldSnippet(t *testing.T) {
	field := yamlpkg.Field{
		Name:   "email",
		Type:   "varchar",
		Length: 255,
	}

	snippet, err := generateFieldSnippet("users", field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(snippet, "# Table: users") {
		t.Errorf("expected '# Table: users' in snippet, got:\n%s", snippet)
	}
	if !strings.Contains(snippet, "name: email") {
		t.Errorf("expected 'name: email' in snippet, got:\n%s", snippet)
	}
	if !strings.Contains(snippet, "length: 255") {
		t.Errorf("expected 'length: 255' in snippet, got:\n%s", snippet)
	}
}

// TestGenerateIndexSnippet verifies that an Index struct is marshalled to YAML
// wrapped in a minimal table context.
func TestGenerateIndexSnippet(t *testing.T) {
	index := yamlpkg.Index{
		Name:   "idx_users_email",
		Fields: []string{"email"},
		Unique: true,
	}

	snippet, err := generateIndexSnippet("users", index)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(snippet, "# Table: users") {
		t.Errorf("expected '# Table: users' in snippet, got:\n%s", snippet)
	}
	if !strings.Contains(snippet, "name: idx_users_email") {
		t.Errorf("expected 'name: idx_users_email' in snippet, got:\n%s", snippet)
	}
	if !strings.Contains(snippet, "unique: true") {
		t.Errorf("expected 'unique: true' in snippet, got:\n%s", snippet)
	}
}

// TestFormatDBDiff_SnippetsExtraTable verifies that the text output includes
// a pasteable YAML snippet for an extra table (table in DB but not in DAG).
func TestFormatDBDiff_SnippetsExtraTable(t *testing.T) {
	nullable := false
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:      yamlpkg.ChangeTypeTableAdded,
				TableName: "sessions",
				NewValue: yamlpkg.Table{
					Name: "sessions",
					Fields: []yamlpkg.Field{
						{Name: "id", Type: "uuid", PrimaryKey: true},
						{Name: "token", Type: "varchar", Length: 255, Nullable: &nullable},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "YAML Snippets") {
		t.Errorf("expected 'YAML Snippets' section, got:\n%s", output)
	}
	if !strings.Contains(output, "- name: sessions") {
		t.Errorf("expected '- name: sessions' in snippet, got:\n%s", output)
	}
	if !strings.Contains(output, "type: uuid") {
		t.Errorf("expected 'type: uuid' in snippet, got:\n%s", output)
	}
}

// TestFormatDBDiff_SnippetsMissingIndex verifies that the text output includes
// a pasteable YAML snippet for a missing index (index in DAG but not in DB).
func TestFormatDBDiff_SnippetsMissingIndex(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: true,
		Changes: []yamlpkg.Change{
			{
				Type:      yamlpkg.ChangeTypeIndexRemoved,
				TableName: "users",
				FieldName: "idx_users_email",
				OldValue: yamlpkg.Index{
					Name:   "idx_users_email",
					Fields: []string{"email"},
					Unique: true,
				},
				Destructive: true,
			},
		},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if !strings.Contains(output, "YAML Snippets") {
		t.Errorf("expected 'YAML Snippets' section, got:\n%s", output)
	}
	if !strings.Contains(output, "idx_users_email") {
		t.Errorf("expected 'idx_users_email' in snippet, got:\n%s", output)
	}
}

// TestFormatDBDiff_NoSnippetsWhenNoChanges verifies that the YAML Snippets
// section is NOT shown when there are no differences.
func TestFormatDBDiff_NoSnippetsWhenNoChanges(t *testing.T) {
	diff := &yamlpkg.SchemaDiff{
		HasChanges: false,
		Changes:    []yamlpkg.Change{},
	}

	var buf bytes.Buffer
	formatDBDiff(&buf, diff, false)

	output := buf.String()
	if strings.Contains(output, "YAML Snippets") {
		t.Errorf("expected NO 'YAML Snippets' section when no changes, got:\n%s", output)
	}
}

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
