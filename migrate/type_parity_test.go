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

// type_parity_test.go ensures that migrate.Field, migrate.Index, and
// migrate.ForeignKey stay in sync with their types.Field, types.Index, and
// types.ForeignKey counterparts. If a field is added to one struct but not the
// other, these tests will fail — acting as a safety net against silent drift.
package migrate

import (
	"reflect"
	"testing"

	"github.com/ocomsoft/morphic/internal/types"
)

// TestFieldStructParity verifies that migrate.Field and types.Field have the
// same exported fields, with documented exceptions for intentional differences.
func TestFieldStructParity(t *testing.T) {
	// Known exceptions: fields that intentionally differ between the two structs.
	//   Nullable:   bool in migrate.Field vs *bool in types.Field (public API constraint)
	//   ForeignKey: *migrate.ForeignKey vs *types.ForeignKey (separate FK types)
	//   ManyToMany: *migrate.ManyToMany vs *types.ManyToMany (separate M2M types)
	exceptions := map[string]bool{
		"Nullable":   true,
		"ForeignKey": true,
		"ManyToMany": true,
	}

	migrateType := reflect.TypeOf(Field{})
	typesType := reflect.TypeOf(types.Field{})

	// Check every field in types.Field exists in migrate.Field.
	for i := 0; i < typesType.NumField(); i++ {
		field := typesType.Field(i)
		if exceptions[field.Name] {
			continue
		}
		if _, ok := migrateType.FieldByName(field.Name); !ok {
			t.Errorf("types.Field has field %q but migrate.Field does not — add it to migrate.Field or to the exceptions map", field.Name)
		}
	}

	// Check every field in migrate.Field exists in types.Field.
	for i := 0; i < migrateType.NumField(); i++ {
		field := migrateType.Field(i)
		if exceptions[field.Name] {
			continue
		}
		if _, ok := typesType.FieldByName(field.Name); !ok {
			t.Errorf("migrate.Field has field %q but types.Field does not — add it to types.Field or to the exceptions map", field.Name)
		}
	}
}

// TestIndexStructParity verifies that migrate.Index and types.Index have the
// same exported fields, with documented exceptions for intentional differences.
func TestIndexStructParity(t *testing.T) {
	// Known exceptions:
	//   ForeignKey: informational annotation on types.Index only (YAML concern,
	//               indicates which FK relationship the index supports — does not
	//               affect SQL generation and is not needed at runtime).
	exceptions := map[string]bool{
		"ForeignKey": true,
	}

	migrateType := reflect.TypeOf(Index{})
	typesType := reflect.TypeOf(types.Index{})

	for i := 0; i < typesType.NumField(); i++ {
		field := typesType.Field(i)
		if exceptions[field.Name] {
			continue
		}
		if _, ok := migrateType.FieldByName(field.Name); !ok {
			t.Errorf("types.Index has field %q but migrate.Index does not — add it to migrate.Index or to the exceptions map", field.Name)
		}
	}

	for i := 0; i < migrateType.NumField(); i++ {
		field := migrateType.Field(i)
		if exceptions[field.Name] {
			continue
		}
		if _, ok := typesType.FieldByName(field.Name); !ok {
			t.Errorf("migrate.Index has field %q but types.Index does not — add it to types.Index or to the exceptions map", field.Name)
		}
	}
}

// TestForeignKeyStructParity verifies that migrate.ForeignKey and
// types.ForeignKey have the same exported fields.
func TestForeignKeyStructParity(t *testing.T) {
	exceptions := map[string]bool{}

	migrateType := reflect.TypeOf(ForeignKey{})
	typesType := reflect.TypeOf(types.ForeignKey{})

	for i := 0; i < typesType.NumField(); i++ {
		field := typesType.Field(i)
		if exceptions[field.Name] {
			continue
		}
		if _, ok := migrateType.FieldByName(field.Name); !ok {
			t.Errorf("types.ForeignKey has field %q but migrate.ForeignKey does not — add it to migrate.ForeignKey or to the exceptions map", field.Name)
		}
	}

	for i := 0; i < migrateType.NumField(); i++ {
		field := migrateType.Field(i)
		if exceptions[field.Name] {
			continue
		}
		if _, ok := typesType.FieldByName(field.Name); !ok {
			t.Errorf("migrate.ForeignKey has field %q but types.ForeignKey does not — add it to types.ForeignKey or to the exceptions map", field.Name)
		}
	}
}

// TestManyToManyStructParity verifies that migrate.ManyToMany and
// types.ManyToMany have the same exported fields.
func TestManyToManyStructParity(t *testing.T) {
	exceptions := map[string]bool{}

	migrateType := reflect.TypeOf(ManyToMany{})
	typesType := reflect.TypeOf(types.ManyToMany{})

	for i := 0; i < typesType.NumField(); i++ {
		field := typesType.Field(i)
		if exceptions[field.Name] {
			continue
		}
		if _, ok := migrateType.FieldByName(field.Name); !ok {
			t.Errorf("types.ManyToMany has field %q but migrate.ManyToMany does not — add it to migrate.ManyToMany or to the exceptions map", field.Name)
		}
	}

	for i := 0; i < migrateType.NumField(); i++ {
		field := migrateType.Field(i)
		if exceptions[field.Name] {
			continue
		}
		if _, ok := typesType.FieldByName(field.Name); !ok {
			t.Errorf("migrate.ManyToMany has field %q but types.ManyToMany does not — add it to types.ManyToMany or to the exceptions map", field.Name)
		}
	}
}
