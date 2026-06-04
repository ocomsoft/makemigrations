package struct2schema

import (
	"testing"

	"github.com/ocomsoft/morphic/internal/types"
)

// TestNewGenerator verifies generator creation.
func TestNewGenerator(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)
	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}
}

// TestGenerateSchemaBasic verifies schema generation from simple structs.
func TestGenerateSchemaBasic(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "User",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "ID", Type: "int", IsExported: true, Tag: `gorm:"primaryKey"`},
				{Name: "Name", Type: "string", IsExported: true},
				{Name: "Email", Type: "string", IsExported: true},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	if schema == nil {
		t.Fatal("schema is nil")
	}
	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}

	table := schema.Tables[0]
	if table.Name != "user" {
		t.Errorf("table name = %q, want %q", table.Name, "user")
	}

	// Should have ID, Name, Email = 3 fields
	if len(table.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(table.Fields))
	}

	// Check ID is primary key
	idField := table.Fields[0]
	if !idField.PrimaryKey {
		t.Error("ID should be primary key")
	}
}

// TestGenerateSchemaAutoPK verifies that a primary key is auto-added
// when none is specified.
func TestGenerateSchemaAutoPK(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "Item",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "Title", Type: "string", IsExported: true},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	table := schema.Tables[0]
	// First field should be auto-generated id
	if table.Fields[0].Name != "id" {
		t.Errorf("first field = %q, want %q", table.Fields[0].Name, "id")
	}
	if table.Fields[0].Type != "serial" {
		t.Errorf("auto-PK type = %q, want %q", table.Fields[0].Type, "serial")
	}
	if !table.Fields[0].PrimaryKey {
		t.Error("auto-PK should have PrimaryKey = true")
	}
}

// TestGenerateSchemaSkipsUnexported verifies unexported fields are skipped.
func TestGenerateSchemaSkipsUnexported(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "Secret",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "Public", Type: "string", IsExported: true},
				{Name: "private", Type: "string", IsExported: false},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	table := schema.Tables[0]
	for _, f := range table.Fields {
		if f.Name == "private" {
			t.Error("unexported field 'private' should not be in schema")
		}
	}
}

// TestGenerateSchemaSkipsIgnored verifies fields with ignore tag are skipped.
func TestGenerateSchemaSkipsIgnored(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "Widget",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "Name", Type: "string", IsExported: true},
				{Name: "Temp", Type: "string", IsExported: true, Tag: `db:"-"`},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	table := schema.Tables[0]
	for _, f := range table.Fields {
		if f.Name == "temp" {
			t.Error("ignored field should not be in schema")
		}
	}
}

// TestGenerateSchemaSkipsEmbedded verifies embedded structs are skipped (current behaviour).
func TestGenerateSchemaSkipsEmbedded(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "Post",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "BaseModel", Type: "BaseModel", IsEmbedded: true, IsExported: true},
				{Name: "Title", Type: "string", IsExported: true},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	table := schema.Tables[0]
	for _, f := range table.Fields {
		if f.Name == "base_model" {
			t.Error("embedded field should be skipped")
		}
	}
}

// TestGenerateSchemaSkipsSliceFields verifies slice fields are skipped (M2M handled separately).
func TestGenerateSchemaSkipsSliceFields(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "User",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "Name", Type: "string", IsExported: true},
				{Name: "Tags", Type: "[]string", IsSlice: true, IsExported: true},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	table := schema.Tables[0]
	for _, f := range table.Fields {
		if f.Name == "tags" {
			t.Error("slice field should be skipped")
		}
	}
}

// TestGenerateSchemaJunctionTables verifies M2M junction tables are generated.
func TestGenerateSchemaJunctionTables(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "User",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "Name", Type: "string", IsExported: true},
			},
		},
	}

	relationships := []Relationship{
		{
			Type:          RelationshipManyToMany,
			SourceStruct:  "User",
			SourceTable:   "user",
			TargetStruct:  "Role",
			TargetTable:   "role",
			JunctionTable: "user_role",
		},
	}

	schema, err := gen.GenerateSchema(structs, relationships)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	// Should have user table + junction table
	if len(schema.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(schema.Tables))
	}

	// Find junction table
	var junction *interface{}
	for _, table := range schema.Tables {
		if table.Name == "user_role" {
			// Verify junction table structure
			if len(table.Fields) != 2 {
				t.Errorf("junction table should have 2 fields, got %d", len(table.Fields))
			}
			if len(table.Indexes) != 1 {
				t.Errorf("junction table should have 1 index, got %d", len(table.Indexes))
			}
			if !table.Indexes[0].Unique {
				t.Error("junction index should be unique")
			}
			junction = new(interface{})
			break
		}
	}
	if junction == nil {
		t.Error("junction table 'user_role' not found")
	}
}

// TestGenerateSchemaForeignKeyField verifies foreign key field generation.
func TestGenerateSchemaForeignKeyField(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "Order",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "ID", Type: "int", IsExported: true, Tag: `gorm:"primaryKey"`},
				{
					Name:           "Customer",
					Type:           "Customer",
					UnderlyingType: "Customer",
					IsExported:     true,
				},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	table := schema.Tables[0]

	// Find the customer field
	var custField *interface{}
	for _, f := range table.Fields {
		if f.Name == "customer" {
			if f.ForeignKey == nil {
				t.Error("customer field should have a foreign key")
			} else {
				if f.ForeignKey.Table != "customer" {
					t.Errorf("FK table = %q, want %q", f.ForeignKey.Table, "customer")
				}
				if f.ForeignKey.OnDelete != "RESTRICT" {
					t.Errorf("FK OnDelete = %q, want %q", f.ForeignKey.OnDelete, "RESTRICT")
				}
			}
			custField = new(interface{})
			break
		}
	}
	if custField == nil {
		t.Error("customer field not found in generated table")
	}
}

// TestGenerateSchemaColumnNameFromTag verifies that column names come from tags.
func TestGenerateSchemaColumnNameFromTag(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "Item",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "FullName", Type: "string", IsExported: true, Tag: `db:"full_name"`},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	table := schema.Tables[0]
	// Should have auto-PK + full_name
	found := false
	for _, f := range table.Fields {
		if f.Name == "full_name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected field with name 'full_name' from db tag")
	}
}

// TestGeneratorToSnakeCase verifies the generator's toSnakeCase method.
func TestGeneratorToSnakeCase(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"User", "user"},
		{"UserProfile", "user_profile"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := gen.toSnakeCase(tt.input)
			if got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestGenerateSchemaEmptyStruct verifies that a struct with no valid
// fields is skipped (returns nil table, no error).
func TestGenerateSchemaEmptyStruct(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "Empty",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "hidden", Type: "string", IsExported: false},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	if len(schema.Tables) != 0 {
		t.Errorf("expected 0 tables for struct with no exportable fields, got %d", len(schema.Tables))
	}
}

// TestGenerateSchemaDefaults verifies that defaults for all DBs are populated.
func TestGenerateSchemaDefaults(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	structs := []GoStruct{
		{
			Name:    "Anything",
			Package: "models",
			Tags:    map[string]string{},
			Fields: []GoField{
				{Name: "Val", Type: "string", IsExported: true},
			},
		},
	}

	schema, err := gen.GenerateSchema(structs, nil)
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	if schema.Defaults.ForProvider(types.DatabasePostgreSQL) == nil {
		t.Error("PostgreSQL defaults should be populated")
	}
	if schema.Defaults.ForProvider(types.DatabaseMySQL) == nil {
		t.Error("MySQL defaults should be populated")
	}
}

// TestInferTableName verifies table name inference from Go type names.
func TestInferTableName(t *testing.T) {
	t.Parallel()

	tm, err := NewTypeMapper("", "postgresql")
	if err != nil {
		t.Fatalf("NewTypeMapper: %v", err)
	}
	tp := NewTagParser()
	gen := NewGenerator(tm, tp, false)

	tests := []struct {
		typeName string
		want     string
	}{
		{"User", "user"},
		{"*User", "user"},
		{"[]User", "user"},
		{"models.UserProfile", "user_profile"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			t.Parallel()
			got := gen.inferTableName(tt.typeName)
			if got != tt.want {
				t.Errorf("inferTableName(%q) = %q, want %q", tt.typeName, got, tt.want)
			}
		})
	}
}
