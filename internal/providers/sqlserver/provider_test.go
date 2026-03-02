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
package sqlserver

import (
	"errors"
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/types"
)

func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("mssql: Cannot drop the table 'users', because it does not exist or you do not have permission."), true},
		{errors.New(`mssql: Cannot find the object "idx_email" because it does not exist or you do not have permission.`), true},
		{errors.New("mssql: Invalid object name 'users'."), false},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		got := p.IsNotFoundError(tc.err)
		if got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func boolPtr(b bool) *bool { return &b }

func TestProvider_GenerateAlterColumn_TypeChange(t *testing.T) {
	p := New()
	old := &types.Field{Name: "score", Type: "integer"}
	nw := &types.Field{Name: "score", Type: "bigint"}
	got, err := p.GenerateAlterColumn("results", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check that it contains ALTER COLUMN with the right type
	if !strings.Contains(got, "ALTER TABLE [results] ALTER COLUMN [score]") {
		t.Errorf("expected ALTER COLUMN statement, got:\n%s", got)
	}
}

func TestProvider_GenerateAlterColumn_NullableToNotNull(t *testing.T) {
	p := New()
	old := &types.Field{Name: "email", Type: "varchar", Length: 255, Nullable: boolPtr(true)}
	nw := &types.Field{Name: "email", Type: "varchar", Length: 255, Nullable: boolPtr(false)}
	got, err := p.GenerateAlterColumn("users", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "NOT NULL") {
		t.Errorf("expected NOT NULL in:\n%s", got)
	}
}

func TestProvider_GenerateAlterColumn_AddDefault(t *testing.T) {
	p := New()
	old := &types.Field{Name: "status", Type: "varchar", Length: 50}
	nw := &types.Field{Name: "status", Type: "varchar", Length: 50, Default: "active"}
	got, err := p.GenerateAlterColumn("orders", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "DEFAULT 'active'") {
		t.Errorf("expected DEFAULT clause in:\n%s", got)
	}
}

func TestProvider_GenerateAlterColumn_NoChange(t *testing.T) {
	p := New()
	old := &types.Field{Name: "name", Type: "varchar", Length: 100}
	nw := &types.Field{Name: "name", Type: "varchar", Length: 100}
	got, err := p.GenerateAlterColumn("things", old, nw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for no-change alter, got: %q", got)
	}
}
