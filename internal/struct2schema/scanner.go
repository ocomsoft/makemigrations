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
package struct2schema

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// GoStruct represents a parsed Go struct with metadata
type GoStruct struct {
	Name       string
	Package    string
	Fields     []GoField
	Tags       map[string]string // struct-level tags
	SourceFile string
	Position   token.Pos
}

// GoField represents a field within a Go struct
type GoField struct {
	Name           string
	Type           string
	Tag            string
	IsPointer      bool
	IsSlice        bool
	IsEmbedded     bool
	IsExported     bool
	UnderlyingType string // For custom types
	Position       token.Pos
}

// Scanner handles scanning Go source files and extracting struct definitions
type Scanner struct {
	verbose bool
	fileSet *token.FileSet
}

// NewScanner creates a new Go source code scanner
func NewScanner(verbose bool) *Scanner {
	return &Scanner{
		verbose: verbose,
		fileSet: token.NewFileSet(),
	}
}

// ScanDirectory recursively scans a directory for Go files and extracts struct definitions
func (s *Scanner) ScanDirectory(rootDir string) ([]GoStruct, error) {
	var structs []GoStruct

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		// Skip test files
		if strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		// Skip directories we should ignore
		if info.IsDir() {
			dirName := filepath.Base(path)
			if shouldSkipDirectory(dirName) {
				if s.verbose {
					fmt.Printf("Skipping directory: %s\n", path)
				}
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		if s.verbose {
			fmt.Printf("Parsing file: %s\n", path)
		}

		// Parse the Go file
		fileStructs, err := s.parseGoFile(path)
		if err != nil {
			if s.verbose {
				fmt.Printf("Warning: Failed to parse %s: %v\n", path, err)
			}
			return nil // Continue processing other files
		}

		structs = append(structs, fileStructs...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", rootDir, err)
	}

	return structs, nil
}

// parseGoFile parses a single Go file and extracts struct definitions
func (s *Scanner) parseGoFile(filename string) ([]GoStruct, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the Go source code
	file, err := parser.ParseFile(s.fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	var structs []GoStruct

	// Walk the AST to find struct declarations
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			if node.Tok == token.TYPE {
				for _, spec := range node.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							goStruct := s.parseStruct(typeSpec.Name.Name, file.Name.Name, structType, filename, typeSpec.Pos())
							structs = append(structs, goStruct)
						}
					}
				}
			}
		}
		return true
	})

	return structs, nil
}

// parseStruct extracts field information from an AST struct type
func (s *Scanner) parseStruct(name, packageName string, structType *ast.StructType, filename string, pos token.Pos) GoStruct {
	goStruct := GoStruct{
		Name:       name,
		Package:    packageName,
		Fields:     []GoField{},
		Tags:       make(map[string]string),
		SourceFile: filename,
		Position:   pos,
	}

	for _, field := range structType.Fields.List {
		goFields := s.parseField(field)
		goStruct.Fields = append(goStruct.Fields, goFields...)
	}

	return goStruct
}

// parseField extracts information from an AST field
func (s *Scanner) parseField(field *ast.Field) []GoField {
	var fields []GoField

	// Get the field type as a string
	fieldType := s.typeToString(field.Type)
	isPointer := false
	isSlice := false
	underlyingType := fieldType

	// Analyze type structure
	switch t := field.Type.(type) {
	case *ast.StarExpr:
		isPointer = true
		underlyingType = s.typeToString(t.X)
	case *ast.ArrayType:
		isSlice = true
		underlyingType = s.typeToString(t.Elt)
	}

	// Get struct tag if present
	var tag string
	if field.Tag != nil {
		tag = field.Tag.Value
	}

	// Handle multiple field names (e.g., "a, b int")
	if len(field.Names) == 0 {
		// Embedded field
		fields = append(fields, GoField{
			Name:           fieldType,
			Type:           fieldType,
			Tag:            tag,
			IsPointer:      isPointer,
			IsSlice:        isSlice,
			IsEmbedded:     true,
			IsExported:     true, // Embedded fields are considered exported
			UnderlyingType: underlyingType,
			Position:       field.Pos(),
		})
	} else {
		for _, fieldName := range field.Names {
			fields = append(fields, GoField{
				Name:           fieldName.Name,
				Type:           fieldType,
				Tag:            tag,
				IsPointer:      isPointer,
				IsSlice:        isSlice,
				IsEmbedded:     false,
				IsExported:     isExported(fieldName.Name),
				UnderlyingType: underlyingType,
				Position:       field.Pos(),
			})
		}
	}

	return fields
}

// typeToString converts an AST type expression to a string
func (s *Scanner) typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return s.typeToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + s.typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + s.typeToString(t.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return fmt.Sprintf("%T", t)
	}
}

// shouldSkipDirectory determines if a directory should be skipped during scanning
func shouldSkipDirectory(dirName string) bool {
	skipDirs := []string{
		".git",
		".svn",
		".hg",
		"vendor",
		"node_modules",
		".vscode",
		".idea",
		"tmp",
		"temp",
		"bin",
		"build",
		"dist",
		".DS_Store",
	}

	for _, skip := range skipDirs {
		if dirName == skip {
			return true
		}
	}

	return false
}

// isExported checks if a field name is exported (starts with uppercase)
func isExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}
