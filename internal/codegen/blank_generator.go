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

package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
)

// BlankGenerator generates blank (empty) migration .go files. Blank migrations
// have no operations and contain a TODO comment as a placeholder for the
// developer to add custom operations.
type BlankGenerator struct{}

// NewBlankGenerator creates a new BlankGenerator.
func NewBlankGenerator() *BlankGenerator {
	return &BlankGenerator{}
}

// GenerateBlank generates the source code for a blank migration .go file.
// name is the migration name (e.g. "0003_add_custom_sql").
// deps is the list of migration names this migration depends on (typically the
// current DAG leaves).
//
// The generated file has an empty Operations slice with a TODO comment so the
// developer can fill in custom operations such as &m.RunSQL{...}.
func (g *BlankGenerator) GenerateBlank(name string, deps []string) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("package main\n\n")
	buf.WriteString("import m \"github.com/ocomsoft/makemigrations/migrate\"\n\n")
	buf.WriteString("func init() {\n")
	fmt.Fprintf(&buf, "\tm.Register(&m.Migration{\n")
	fmt.Fprintf(&buf, "\t\tName:         %q,\n", name)

	depStrs := make([]string, len(deps))
	for i, d := range deps {
		depStrs[i] = fmt.Sprintf("%q", d)
	}
	fmt.Fprintf(&buf, "\t\tDependencies: []string{%s},\n", strings.Join(depStrs, ", "))

	// Empty operations block with a helpful TODO comment.
	buf.WriteString("\t\tOperations: []m.Operation{\n")
	buf.WriteString("\t\t\t// TODO: Add migration operations here.\n")
	buf.WriteString("\t\t\t// Example: &m.RunSQL{ForwardSQL: \"SELECT 1\", BackwardSQL: \"\"},\n")
	buf.WriteString("\t\t},\n")

	buf.WriteString("\t})\n")
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("formatting blank migration: %w\nRaw:\n%s", err, buf.String())
	}
	return string(formatted), nil
}
