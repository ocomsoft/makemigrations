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

// Package migrate provides the runtime library for the makemigrations Go migration framework.
// Generated migration files import this package and call Register() in their init() functions.
package migrate

// Migration represents a single database migration with its name, dependencies, and operations.
type Migration struct {
	Name         string      // Unique identifier e.g. "0001_initial"
	Dependencies []string    // Names of migrations this depends on
	Operations   []Operation // Ordered list of schema operations to apply
	Replaces     []string    // For squashed migrations: names of migrations this replaces
}

// Field represents a database column definition used in migration operations.
type Field struct {
	Name       string
	Type       string // varchar, text, integer, uuid, boolean, timestamp, foreign_key, etc.
	PrimaryKey bool
	Nullable   bool
	Default    string // default reference name e.g. "new_uuid", "now", "true"
	Length     int    // for varchar
	Precision  int    // for decimal/numeric
	Scale      int    // for decimal/numeric
	AutoCreate bool   // auto-set on row creation (created_at)
	AutoUpdate bool   // auto-set on row update (updated_at)
	ForeignKey *ForeignKey
	ManyToMany *ManyToMany
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Table    string
	OnDelete string
	OnUpdate string
}

// ManyToMany represents a many-to-many relationship via junction table.
type ManyToMany struct {
	Table string
}

// Index represents a database index definition.
type Index struct {
	Name   string
	Fields []string
	Unique bool
}
