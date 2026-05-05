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

package interp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/interp"
)

const file0001 = `package main

import (
	m "github.com/ocomsoft/makemigrations/migrate"
)

func init() {
	m.Register(&m.Migration{
		Name:         "0001_initial",
		Dependencies: []string{},
		Operations: []m.Operation{
			&m.CreateTable{
				Name: "users",
				Fields: []m.Field{
					{Name: "id", Type: "uuid", PrimaryKey: true},
					{Name: "email", Type: "varchar", Length: 255},
				},
			},
		},
	})
}
`

const file0002 = `package main

import (
	m "github.com/ocomsoft/makemigrations/migrate"
)

func init() {
	m.Register(&m.Migration{
		Name:         "0002_add_index",
		Dependencies: []string{"0001_initial"},
		Operations: []m.Operation{
			&m.AddIndex{
				Table: "users",
				Index: m.Index{Name: "users_email_idx", Fields: []string{"email"}, Unique: true},
			},
		},
	})
}
`

const fileMain = `package main

import (
	"fmt"
	"os"

	m "github.com/ocomsoft/makemigrations/migrate"
)

func main() {
	app := m.NewApp(m.Config{})
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

func TestLoadRegistry(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "0001_initial.go"), file0001)
	mustWrite(t, filepath.Join(dir, "0002_add_index.go"), file0002)
	mustWrite(t, filepath.Join(dir, "main.go"), fileMain)

	reg, err := interp.LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(all))
	}
	if all[0].Name != "0001_initial" {
		t.Errorf("first migration name = %q, want %q", all[0].Name, "0001_initial")
	}
	if all[1].Name != "0002_add_index" {
		t.Errorf("second migration name = %q, want %q", all[1].Name, "0002_add_index")
	}
	if len(all[1].Dependencies) != 1 || all[1].Dependencies[0] != "0001_initial" {
		t.Errorf("0002 deps = %v, want [0001_initial]", all[1].Dependencies)
	}
	if len(all[0].Operations) != 1 {
		t.Fatalf("0001 ops = %d, want 1", len(all[0].Operations))
	}
	if all[0].Operations[0].TypeName() != "create_table" {
		t.Errorf("0001 op type = %q, want create_table", all[0].Operations[0].TypeName())
	}
}

func TestLoadRegistryEmpty(t *testing.T) {
	dir := t.TempDir()
	reg, err := interp.LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry empty: %v", err)
	}
	if len(reg.All()) != 0 {
		t.Errorf("expected empty registry, got %d", len(reg.All()))
	}
}

func TestLoadRegistryIsolated(t *testing.T) {
	// Two consecutive loads must not produce duplicate-registration panics.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "0001_initial.go"), file0001)
	if _, err := interp.LoadRegistry(dir); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if _, err := interp.LoadRegistry(dir); err != nil {
		t.Fatalf("second load: %v", err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
