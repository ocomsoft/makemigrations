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

package migrate_test

import (
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/migrate"
)

func TestRegistry_Register(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial", Dependencies: []string{}})
	all := reg.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(all))
	}
	if all[0].Name != "0001_initial" {
		t.Fatalf("expected '0001_initial', got %q", all[0].Name)
	}
}

func TestRegistry_Register_Duplicate_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate migration name")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "duplicate migration name") {
			t.Fatalf("unexpected panic value: %v", r)
		}
	}()
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial"})
	reg.Register(&migrate.Migration{Name: "0001_initial"})
}

func TestRegistry_Get(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial"})
	m, ok := reg.Get("0001_initial")
	if !ok {
		t.Fatal("expected to find '0001_initial'")
	}
	if m.Name != "0001_initial" {
		t.Fatalf("expected '0001_initial', got %q", m.Name)
	}
	_, ok = reg.Get("missing")
	if ok {
		t.Fatal("expected false for missing migration")
	}
}

func TestRegistry_Register_Nil_Panics(t *testing.T) {
	r := migrate.NewRegistry()
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected panic for nil migration")
		}
		msg, ok := rec.(string)
		if !ok || !strings.Contains(msg, "nil") {
			t.Fatalf("unexpected panic value: %v", rec)
		}
	}()
	r.Register(nil)
}

func TestRegistry_Register_EmptyName_Panics(t *testing.T) {
	r := migrate.NewRegistry()
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("expected panic for empty migration name")
		}
		msg, ok := rec.(string)
		if !ok || !strings.Contains(msg, "empty") {
			t.Fatalf("unexpected panic value: %v", rec)
		}
	}()
	r.Register(&migrate.Migration{Name: ""})
}

func TestRegistry_InsertionOrder(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0002_second"})
	reg.Register(&migrate.Migration{Name: "0001_first"})
	all := reg.All()
	if all[0].Name != "0002_second" || all[1].Name != "0001_first" {
		t.Fatal("expected insertion order preserved")
	}
}

func TestRegistry_Resolve_ExactMatch(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial"})
	got, err := reg.Resolve("0001_initial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0001_initial" {
		t.Fatalf("expected %q, got %q", "0001_initial", got)
	}
}

func TestRegistry_Resolve_PartialName_Errors(t *testing.T) {
	// fake "0001" must not silently match "0001_initial" — require the full name.
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial"})
	_, err := reg.Resolve("0001")
	if err == nil {
		t.Fatal("expected error for partial (non-exact) name")
	}
	if !strings.Contains(err.Error(), "no migration") {
		t.Fatalf("expected 'no migration' in error, got: %v", err)
	}
}

func TestRegistry_Resolve_NotFound(t *testing.T) {
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{Name: "0001_initial"})
	_, err := reg.Resolve("9999_missing")
	if err == nil {
		t.Fatal("expected error for non-existent migration")
	}
	if !strings.Contains(err.Error(), "no migration") {
		t.Fatalf("expected 'no migration' in error, got: %v", err)
	}
}
