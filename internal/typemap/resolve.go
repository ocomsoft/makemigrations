package typemap

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/ocomsoft/makemigrations/internal/types"
)

// ResolveType evaluates a type mapping string against a field.
// If the mapping contains Go template syntax ({{ }}), it is executed
// with the field as data. Otherwise the string is returned as-is.
func ResolveType(mapping string, field *types.Field) (string, error) {
	if !strings.Contains(mapping, "{{") {
		return mapping, nil
	}

	tmpl, err := template.New("type").Option("missingkey=error").Parse(mapping)
	if err != nil {
		return "", fmt.Errorf("invalid type mapping template %q: %w", mapping, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, field); err != nil {
		return "", fmt.Errorf("failed to resolve type mapping %q: %w", mapping, err)
	}

	return buf.String(), nil
}
