package typemap

import (
	"testing"

	"github.com/ocomsoft/makemigrations/internal/types"
)

func TestResolveType_PlainString(t *testing.T) {
	field := &types.Field{Type: "float"}
	got, err := ResolveType("NUMERIC", field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "NUMERIC" {
		t.Errorf("got %q, want %q", got, "NUMERIC")
	}
}

func TestResolveType_TemplateWithLength(t *testing.T) {
	field := &types.Field{Type: "varchar", Length: 100}
	got, err := ResolveType("NVARCHAR({{.Length}})", field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "NVARCHAR(100)" {
		t.Errorf("got %q, want %q", got, "NVARCHAR(100)")
	}
}

func TestResolveType_TemplateWithPrecisionScale(t *testing.T) {
	field := &types.Field{Type: "decimal", Precision: 9, Scale: 2}
	got, err := ResolveType("DECIMAL({{.Precision}},{{.Scale}})", field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "DECIMAL(9,2)" {
		t.Errorf("got %q, want %q", got, "DECIMAL(9,2)")
	}
}

func TestResolveType_InvalidTemplate(t *testing.T) {
	field := &types.Field{Type: "varchar"}
	_, err := ResolveType("BAD({{.Unknown}})", field)
	if err == nil {
		t.Fatal("expected error for invalid template field")
	}
}
