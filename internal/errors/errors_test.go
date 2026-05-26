package errors

import (
	"fmt"
	"testing"
)

func TestIsValidationError(t *testing.T) {
	direct := NewValidationError("name", "required")
	wrapped := fmt.Errorf("outer: %w", direct)
	other := NewMigrationError("create", "failed")

	if !IsValidationError(direct) {
		t.Error("expected direct ValidationError to match")
	}
	if !IsValidationError(wrapped) {
		t.Error("expected wrapped ValidationError to match")
	}
	if IsValidationError(other) {
		t.Error("expected MigrationError not to match ValidationError")
	}
	if IsValidationError(nil) {
		t.Error("expected nil not to match ValidationError")
	}
}

func TestIsSchemaParseError(t *testing.T) {
	direct := NewSchemaParseError("schema.yaml", 10, "bad syntax")
	wrapped := fmt.Errorf("outer: %w", direct)
	other := NewValidationError("field", "invalid")

	if !IsSchemaParseError(direct) {
		t.Error("expected direct SchemaParseError to match")
	}
	if !IsSchemaParseError(wrapped) {
		t.Error("expected wrapped SchemaParseError to match")
	}
	if IsSchemaParseError(other) {
		t.Error("expected ValidationError not to match SchemaParseError")
	}
	if IsSchemaParseError(nil) {
		t.Error("expected nil not to match SchemaParseError")
	}
}

func TestIsDependencyError(t *testing.T) {
	direct := NewDependencyError("users", "missing ref")
	wrapped := fmt.Errorf("outer: %w", direct)
	other := NewValidationError("field", "invalid")

	if !IsDependencyError(direct) {
		t.Error("expected direct DependencyError to match")
	}
	if !IsDependencyError(wrapped) {
		t.Error("expected wrapped DependencyError to match")
	}
	if IsDependencyError(other) {
		t.Error("expected ValidationError not to match DependencyError")
	}
	if IsDependencyError(nil) {
		t.Error("expected nil not to match DependencyError")
	}
}

func TestIsCircularDependencyError(t *testing.T) {
	direct := NewCircularDependencyError([]string{"a", "b", "a"})
	wrapped := fmt.Errorf("outer: %w", direct)
	other := NewDependencyError("t", "msg")

	if !IsCircularDependencyError(direct) {
		t.Error("expected direct CircularDependencyError to match")
	}
	if !IsCircularDependencyError(wrapped) {
		t.Error("expected wrapped CircularDependencyError to match")
	}
	if IsCircularDependencyError(other) {
		t.Error("expected DependencyError not to match CircularDependencyError")
	}
	if IsCircularDependencyError(nil) {
		t.Error("expected nil not to match CircularDependencyError")
	}
}

func TestIsMigrationError(t *testing.T) {
	direct := NewMigrationError("apply", "timeout")
	wrapped := fmt.Errorf("outer: %w", direct)
	other := NewValidationError("field", "bad")

	if !IsMigrationError(direct) {
		t.Error("expected direct MigrationError to match")
	}
	if !IsMigrationError(wrapped) {
		t.Error("expected wrapped MigrationError to match")
	}
	if IsMigrationError(other) {
		t.Error("expected ValidationError not to match MigrationError")
	}
	if IsMigrationError(nil) {
		t.Error("expected nil not to match MigrationError")
	}
}

// TestDoubleWrapped verifies errors.As unwraps multiple layers of wrapping.
func TestDoubleWrapped(t *testing.T) {
	base := NewValidationError("email", "invalid format")
	wrapped := fmt.Errorf("layer1: %w", base)
	doubleWrapped := fmt.Errorf("layer2: %w", wrapped)

	if !IsValidationError(doubleWrapped) {
		t.Error("expected double-wrapped ValidationError to match")
	}
}

// TestErrorMessages verifies the Error() output for each type.
func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ValidationError", NewValidationError("name", "required"), "validation error: name: required"},
		{"SchemaParseError with line", NewSchemaParseError("f.yaml", 5, "bad"), "schema parse error in f.yaml at line 5: bad"},
		{"SchemaParseError no line", NewSchemaParseError("f.yaml", 0, "bad"), "schema parse error in f.yaml: bad"},
		{"DependencyError", NewDependencyError("users", "missing"), "dependency error for table users: missing"},
		{"CircularDependencyError", NewCircularDependencyError([]string{"a", "b", "a"}), "circular dependency detected: a -> b -> a"},
		{"MigrationError", NewMigrationError("apply", "fail"), "migration error during apply: fail"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
