package struct2schema

import (
	"testing"
)

// TestNewRelationshipDetector verifies creation.
func TestNewRelationshipDetector(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)
	if rd == nil {
		t.Fatal("NewRelationshipDetector returned nil")
	}
	if rd.verbose {
		t.Error("verbose should be false")
	}
}

// TestToSnakeCase verifies CamelCase to snake_case conversion.
func TestToSnakeCase(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)

	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"User", "user"},
		{"UserProfile", "user_profile"},
		{"ID", "i_d"},
		{"HTMLParser", "h_t_m_l_parser"},
		{"simple", "simple"},
		{"camelCase", "camel_case"},
		{"ABCDef", "a_b_c_def"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := rd.toSnakeCase(tt.input)
			if got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestGenerateJunctionTableName verifies alphabetically sorted junction table names.
func TestGenerateJunctionTableName(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)

	tests := []struct {
		table1 string
		table2 string
		want   string
	}{
		{"users", "roles", "roles_users"},
		{"roles", "users", "roles_users"},
		{"alpha", "beta", "alpha_beta"},
		{"same", "same", "same_same"},
	}

	for _, tt := range tests {
		t.Run(tt.table1+"_"+tt.table2, func(t *testing.T) {
			t.Parallel()
			got := rd.generateJunctionTableName(tt.table1, tt.table2)
			if got != tt.want {
				t.Errorf("generateJunctionTableName(%q, %q) = %q, want %q", tt.table1, tt.table2, got, tt.want)
			}
		})
	}
}

// TestDetectRelationshipsExplicitFK verifies detection of explicit foreign key tags.
func TestDetectRelationshipsExplicitFK(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)

	structs := []GoStruct{
		{
			Name:    "Order",
			Package: "models",
			Fields: []GoField{
				{
					Name:       "UserID",
					Type:       "int",
					Tag:        `gorm:"foreignKey:User"`,
					IsExported: true,
				},
			},
		},
		{
			Name:    "User",
			Package: "models",
			Fields: []GoField{
				{
					Name:       "ID",
					Type:       "int",
					IsExported: true,
				},
			},
		},
	}

	rels := rd.DetectRelationships(structs)

	fkRels := rd.GetRelationshipsByType(rels, RelationshipForeignKey)
	if len(fkRels) != 1 {
		t.Fatalf("expected 1 FK relationship, got %d", len(fkRels))
	}

	fk := fkRels[0]
	if fk.SourceStruct != "Order" {
		t.Errorf("SourceStruct = %q, want %q", fk.SourceStruct, "Order")
	}
	if fk.TargetStruct != "User" {
		t.Errorf("TargetStruct = %q, want %q", fk.TargetStruct, "User")
	}
}

// TestDetectRelationshipsExplicitM2M verifies detection of explicit many-to-many tags.
func TestDetectRelationshipsExplicitM2M(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)

	structs := []GoStruct{
		{
			Name:    "User",
			Package: "models",
			Fields: []GoField{
				{
					Name:       "Roles",
					Type:       "[]Role",
					Tag:        `gorm:"many2many:user_roles"`,
					IsSlice:    true,
					IsExported: true,
				},
			},
		},
		{
			Name:    "Role",
			Package: "models",
			Fields: []GoField{
				{
					Name:       "ID",
					Type:       "int",
					IsExported: true,
				},
			},
		},
	}

	rels := rd.DetectRelationships(structs)

	m2mRels := rd.GetRelationshipsByType(rels, RelationshipManyToMany)
	if len(m2mRels) != 1 {
		t.Fatalf("expected 1 M2M relationship, got %d", len(m2mRels))
	}

	m2m := m2mRels[0]
	if m2m.JunctionTable != "user_roles" {
		t.Errorf("JunctionTable = %q, want %q", m2m.JunctionTable, "user_roles")
	}
}

// TestDetectRelationshipsInferredFK verifies inferred FK from struct type fields.
func TestDetectRelationshipsInferredFK(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)

	structs := []GoStruct{
		{
			Name:    "Post",
			Package: "models",
			Fields: []GoField{
				{
					Name:           "Author",
					Type:           "User",
					UnderlyingType: "User",
					IsExported:     true,
				},
			},
		},
		{
			Name:    "User",
			Package: "models",
			Fields: []GoField{
				{
					Name:       "ID",
					Type:       "int",
					IsExported: true,
				},
			},
		},
	}

	rels := rd.DetectRelationships(structs)

	fkRels := rd.GetRelationshipsByType(rels, RelationshipForeignKey)
	if len(fkRels) != 1 {
		t.Fatalf("expected 1 inferred FK, got %d", len(fkRels))
	}

	fk := fkRels[0]
	if fk.SourceStruct != "Post" {
		t.Errorf("SourceStruct = %q, want %q", fk.SourceStruct, "Post")
	}
	if fk.TargetStruct != "User" {
		t.Errorf("TargetStruct = %q, want %q", fk.TargetStruct, "User")
	}
	if fk.OnDelete != "RESTRICT" {
		t.Errorf("OnDelete = %q, want %q", fk.OnDelete, "RESTRICT")
	}
}

// TestDetectRelationshipsInferredM2M verifies inferred M2M from slice-of-struct fields.
func TestDetectRelationshipsInferredM2M(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)

	structs := []GoStruct{
		{
			Name:    "User",
			Package: "models",
			Fields: []GoField{
				{
					Name:           "Groups",
					Type:           "[]Group",
					UnderlyingType: "Group",
					IsSlice:        true,
					IsExported:     true,
				},
			},
		},
		{
			Name:    "Group",
			Package: "models",
			Fields: []GoField{
				{
					Name:       "ID",
					Type:       "int",
					IsExported: true,
				},
			},
		},
	}

	rels := rd.DetectRelationships(structs)

	m2mRels := rd.GetRelationshipsByType(rels, RelationshipManyToMany)
	if len(m2mRels) != 1 {
		t.Fatalf("expected 1 inferred M2M, got %d", len(m2mRels))
	}

	m2m := m2mRels[0]
	// Junction table should be alphabetically sorted
	if m2m.JunctionTable != "group_user" {
		t.Errorf("JunctionTable = %q, want %q", m2m.JunctionTable, "group_user")
	}
}

// TestDetectRelationshipsSkipsUnexported verifies that unexported fields are skipped.
func TestDetectRelationshipsSkipsUnexported(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)

	structs := []GoStruct{
		{
			Name:    "Post",
			Package: "models",
			Fields: []GoField{
				{
					Name:           "author",
					Type:           "User",
					UnderlyingType: "User",
					IsExported:     false,
					IsEmbedded:     false,
				},
			},
		},
		{
			Name:    "User",
			Package: "models",
			Fields:  []GoField{},
		},
	}

	rels := rd.DetectRelationships(structs)
	if len(rels) != 0 {
		t.Errorf("expected 0 relationships for unexported field, got %d", len(rels))
	}
}

// TestDetectRelationshipsNoKnownStruct verifies that fields referencing
// unknown struct types are not treated as relationships.
func TestDetectRelationshipsNoKnownStruct(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)

	structs := []GoStruct{
		{
			Name:    "Post",
			Package: "models",
			Fields: []GoField{
				{
					Name:           "Metadata",
					Type:           "SomeExternal",
					UnderlyingType: "SomeExternal",
					IsExported:     true,
				},
			},
		},
	}

	rels := rd.DetectRelationships(structs)
	if len(rels) != 0 {
		t.Errorf("expected 0 relationships for unknown type, got %d", len(rels))
	}
}

// TestGetRelationshipsBySource verifies filtering by source struct.
func TestGetRelationshipsBySource(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)
	rels := []Relationship{
		{Type: RelationshipForeignKey, SourceStruct: "Order", TargetStruct: "User"},
		{Type: RelationshipForeignKey, SourceStruct: "Post", TargetStruct: "User"},
		{Type: RelationshipManyToMany, SourceStruct: "User", TargetStruct: "Role"},
	}

	orderRels := rd.GetRelationshipsBySource(rels, "Order")
	if len(orderRels) != 1 {
		t.Errorf("expected 1, got %d", len(orderRels))
	}

	userRels := rd.GetRelationshipsBySource(rels, "User")
	if len(userRels) != 1 {
		t.Errorf("expected 1, got %d", len(userRels))
	}
}

// TestGetRelationshipsByTarget verifies filtering by target struct.
func TestGetRelationshipsByTarget(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)
	rels := []Relationship{
		{Type: RelationshipForeignKey, SourceStruct: "Order", TargetStruct: "User"},
		{Type: RelationshipForeignKey, SourceStruct: "Post", TargetStruct: "User"},
		{Type: RelationshipManyToMany, SourceStruct: "User", TargetStruct: "Role"},
	}

	userTargetRels := rd.GetRelationshipsByTarget(rels, "User")
	if len(userTargetRels) != 2 {
		t.Errorf("expected 2, got %d", len(userTargetRels))
	}

	roleTargetRels := rd.GetRelationshipsByTarget(rels, "Role")
	if len(roleTargetRels) != 1 {
		t.Errorf("expected 1, got %d", len(roleTargetRels))
	}
}

// TestBuildStructMap verifies the struct name lookup map includes package-prefixed keys.
func TestBuildStructMap(t *testing.T) {
	t.Parallel()

	rd := NewRelationshipDetector(false)
	structs := []GoStruct{
		{Name: "User", Package: "models"},
		{Name: "Role", Package: "auth"},
	}

	structMap := rd.buildStructMap(structs)

	if _, ok := structMap["User"]; !ok {
		t.Error("expected User in map")
	}
	if _, ok := structMap["models.User"]; !ok {
		t.Error("expected models.User in map")
	}
	if _, ok := structMap["Role"]; !ok {
		t.Error("expected Role in map")
	}
	if _, ok := structMap["auth.Role"]; !ok {
		t.Error("expected auth.Role in map")
	}
}
