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

package codegen_test

import (
	"strings"
	"testing"

	"github.com/ocomsoft/makemigrations/internal/codegen"
)

func TestBlankGenerator_GenerateBlank(t *testing.T) {
	g := codegen.NewBlankGenerator()
	src, err := g.GenerateBlank("0003_add_custom_sql", []string{"0002_prev"})
	if err != nil {
		t.Fatalf("GenerateBlank: %v", err)
	}

	// Must compile as valid Go (go/format will fail otherwise)
	if !strings.Contains(src, "package main") {
		t.Error("expected package main")
	}
	if !strings.Contains(src, `Name:`) {
		t.Error("expected Name field")
	}
	if !strings.Contains(src, `"0003_add_custom_sql"`) {
		t.Error("expected migration name in output")
	}
	if !strings.Contains(src, `"0002_prev"`) {
		t.Error("expected dependency in output")
	}
	if !strings.Contains(src, "TODO") {
		t.Error("expected TODO comment in blank migration")
	}
	if !strings.Contains(src, "Operations") {
		t.Error("expected Operations field")
	}
}

func TestBlankGenerator_GenerateBlank_EmptyDeps(t *testing.T) {
	g := codegen.NewBlankGenerator()
	src, err := g.GenerateBlank("0001_blank", []string{})
	if err != nil {
		t.Fatalf("GenerateBlank with empty deps: %v", err)
	}
	if !strings.Contains(src, `Dependencies: []string{}`) {
		t.Errorf("expected empty dependencies slice, got:\n%s", src)
	}
}

func TestBlankGenerator_GenerateBlank_MultipleDeps(t *testing.T) {
	g := codegen.NewBlankGenerator()
	src, err := g.GenerateBlank("0005_blank", []string{"0003_a", "0004_b"})
	if err != nil {
		t.Fatalf("GenerateBlank: %v", err)
	}
	if !strings.Contains(src, `"0003_a"`) {
		t.Error("expected first dependency")
	}
	if !strings.Contains(src, `"0004_b"`) {
		t.Error("expected second dependency")
	}
}
