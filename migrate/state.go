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

package migrate

import "fmt"

// SchemaState holds the in-memory representation of the database schema at a
// specific point in the migration graph. Operations call Mutate() to update
// it as they are applied during graph traversal.
type SchemaState struct {
	Tables       map[string]*TableState `json:"tables"`
	Defaults     map[string]string      `json:"defaults,omitempty"`      // active DB-type defaults from SetDefaults operations
	TypeMappings map[string]string      `json:"type_mappings,omitempty"` // active provider's type mappings from SetTypeMappings operations
}

// TableState holds the state of a single table.
type TableState struct {
	Name    string  `json:"name"`
	Fields  []Field `json:"fields"`
	Indexes []Index `json:"indexes"`
}

// NewSchemaState returns an empty SchemaState.
func NewSchemaState() *SchemaState {
	return &SchemaState{Tables: make(map[string]*TableState)}
}

// SetDefaults updates the active schema defaults map on the state.
// Called by SetDefaults operations during migration traversal.
func (s *SchemaState) SetDefaults(defaults map[string]string) {
	s.Defaults = defaults
}

// SetTypeMappings updates the active type mappings on the state.
// Called by SetTypeMappings operations during migration traversal.
func (s *SchemaState) SetTypeMappings(m map[string]string) {
	s.TypeMappings = m
}

// AddTable adds a new table. Returns error if the table already exists.
func (s *SchemaState) AddTable(name string, fields []Field, indexes []Index) error {
	if _, exists := s.Tables[name]; exists {
		return fmt.Errorf("table %q already exists in schema state", name)
	}
	if fields == nil {
		fields = []Field{}
	}
	if indexes == nil {
		indexes = []Index{}
	}
	s.Tables[name] = &TableState{Name: name, Fields: fields, Indexes: indexes}
	return nil
}

// DropTable removes a table. Returns error if the table does not exist.
func (s *SchemaState) DropTable(name string) error {
	if _, exists := s.Tables[name]; !exists {
		return fmt.Errorf("table %q does not exist in schema state", name)
	}
	delete(s.Tables, name)
	return nil
}

// RenameTable renames a table. Returns error if old name does not exist or new name already exists.
func (s *SchemaState) RenameTable(oldName, newName string) error {
	t, exists := s.Tables[oldName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", oldName)
	}
	if _, exists := s.Tables[newName]; exists {
		return fmt.Errorf("table %q already exists in schema state", newName)
	}
	t.Name = newName
	s.Tables[newName] = t
	delete(s.Tables, oldName)
	return nil
}

// AddField appends a field to an existing table. Returns error if the field name already exists.
func (s *SchemaState) AddField(tableName string, field Field) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for _, f := range t.Fields {
		if f.Name == field.Name {
			return fmt.Errorf("field %q already exists in table %q", field.Name, tableName)
		}
	}
	t.Fields = append(t.Fields, field)
	return nil
}

// DropField removes a named field from an existing table.
func (s *SchemaState) DropField(tableName, fieldName string) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for i, f := range t.Fields {
		if f.Name == fieldName {
			t.Fields = append(t.Fields[:i], t.Fields[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("field %q does not exist in table %q", fieldName, tableName)
}

// AlterField replaces a field (matched by name) in an existing table.
func (s *SchemaState) AlterField(tableName string, newField Field) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for i, f := range t.Fields {
		if f.Name == newField.Name {
			t.Fields[i] = newField
			return nil
		}
	}
	return fmt.Errorf("field %q does not exist in table %q", newField.Name, tableName)
}

// RenameField renames a field within an existing table.
func (s *SchemaState) RenameField(tableName, oldName, newName string) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for i, f := range t.Fields {
		if f.Name == oldName {
			t.Fields[i].Name = newName
			return nil
		}
	}
	return fmt.Errorf("field %q does not exist in table %q", oldName, tableName)
}

// AddIndex appends an index to an existing table. Returns error if the index name already exists.
func (s *SchemaState) AddIndex(tableName string, index Index) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for _, idx := range t.Indexes {
		if idx.Name == index.Name {
			return fmt.Errorf("index %q already exists in table %q", index.Name, tableName)
		}
	}
	t.Indexes = append(t.Indexes, index)
	return nil
}

// DropIndex removes a named index from an existing table.
func (s *SchemaState) DropIndex(tableName, indexName string) error {
	t, exists := s.Tables[tableName]
	if !exists {
		return fmt.Errorf("table %q does not exist in schema state", tableName)
	}
	for i, idx := range t.Indexes {
		if idx.Name == indexName {
			t.Indexes = append(t.Indexes[:i], t.Indexes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("index %q does not exist in table %q", indexName, tableName)
}
