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
package scanner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/ocomsoft/makemigrations/internal/errors"
)

type SchemaFile struct {
	ModulePath string
	FilePath   string
	Content    string
	HasMarker  bool
	Type       SchemaType
}

type SchemaType string

const (
	SchemaTypeSQL  SchemaType = "sql"
	SchemaTypeYAML SchemaType = "yaml"
)

type Scanner struct {
	verbose bool
}

func New(verbose bool) *Scanner {
	return &Scanner{
		verbose: verbose,
	}
}

func (s *Scanner) ScanModules() ([]SchemaFile, error) {
	return s.ScanModulesWithType(SchemaTypeSQL)
}

func (s *Scanner) ScanYAMLModules() ([]SchemaFile, error) {
	return s.ScanModulesWithType(SchemaTypeYAML)
}

func (s *Scanner) ScanModulesWithType(schemaType SchemaType) ([]SchemaFile, error) {
	goModPath := "go.mod"

	// Validate go.mod exists
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil, errors.NewValidationError("go.mod", "file not found - ensure you're in a Go module directory")
	}

	goModBytes, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	if len(goModBytes) == 0 {
		return nil, errors.NewValidationError("go.mod", "file is empty")
	}

	modFile, err := modfile.Parse(goModPath, goModBytes, nil)
	if err != nil {
		return nil, errors.NewSchemaParseError(goModPath, 0, fmt.Sprintf("invalid go.mod syntax: %v", err))
	}

	if modFile.Module == nil {
		return nil, errors.NewValidationError("go.mod", "missing module declaration")
	}

	var schemas []SchemaFile

	// Scan direct dependencies
	for _, req := range modFile.Require {
		if req.Indirect {
			continue // Skip indirect dependencies
		}

		if s.verbose {
			fmt.Printf("Scanning module: %s@%s\n", req.Mod.Path, req.Mod.Version)
		}

		modPath := s.getModulePath(req.Mod.Path, req.Mod.Version)
		if modPath == "" {
			if s.verbose {
				fmt.Printf("  Module path not found in cache\n")
			}
			continue
		}

		schema, err := s.findSchemaInPathWithType(modPath, req.Mod.Path, schemaType)
		if err != nil {
			if s.verbose {
				fmt.Printf("  Error scanning: %v\n", err)
			}
			continue
		}

		if schema != nil {
			schemas = append(schemas, *schema)
			if s.verbose {
				fmt.Printf("  Found %s schema file (marker: %v)\n", schemaType, schema.HasMarker)
			}
		}
	}

	// Check current module last - find all schemas
	if s.verbose {
		fmt.Printf("Scanning current directory for %s schema\n", schemaType)
	}

	currentSchemas, err := s.findAllSchemasInPathWithType(".", "current module", schemaType)
	if err != nil {
		if s.verbose {
			fmt.Printf("  Error scanning current directory: %v\n", err)
		}
	} else if len(currentSchemas) > 0 {
		schemas = append(schemas, currentSchemas...)
		if s.verbose {
			fmt.Printf("  Found %d %s schema file(s) in current directory\n", len(currentSchemas), schemaType)
		}
	}

	return schemas, nil
}

func (s *Scanner) findSchemaInPath(basePath, modulePath string) (*SchemaFile, error) {
	return s.findSchemaInPathWithType(basePath, modulePath, SchemaTypeSQL)
}

func (s *Scanner) findSchemaInPathWithType(basePath, modulePath string, schemaType SchemaType) (*SchemaFile, error) {
	schemas, err := s.findAllSchemasInPathWithType(basePath, modulePath, schemaType)
	if err != nil {
		return nil, err
	}
	if len(schemas) == 0 {
		return nil, nil
	}

	// Return the first schema found
	return &schemas[0], nil
}

func (s *Scanner) findAllSchemasInPathWithType(basePath, modulePath string, schemaType SchemaType) ([]SchemaFile, error) {
	var targetFilename string
	var targetDirname string

	switch schemaType {
	case SchemaTypeSQL:
		targetDirname = "sql"
		targetFilename = "schema.sql"
	case SchemaTypeYAML:
		targetDirname = "schema"
		targetFilename = "schema.yaml"
	default:
		return nil, fmt.Errorf("unsupported schema type: %s", schemaType)
	}

	var schemas []SchemaFile

	// Search recursively for all schema files
	err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking even if there's an error with this path
		}

		// Check if this is the target file in a target directory
		if d.Name() == targetFilename && filepath.Base(filepath.Dir(path)) == targetDirname {
			file, err := os.Open(path)
			if err != nil {
				return nil // Continue walking
			}
			defer file.Close()

			content, hasMarker, err := s.readSchemaFileWithType(file, schemaType)
			if err != nil {
				return nil // Continue walking
			}

			schema := SchemaFile{
				ModulePath: modulePath,
				FilePath:   path,
				Content:    content,
				HasMarker:  hasMarker,
				Type:       schemaType,
			}

			schemas = append(schemas, schema)

			if s.verbose {
				fmt.Printf("  Found %s schema at: %s\n", schemaType, path)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return schemas, nil
}

func (s *Scanner) readSchemaFile(r io.Reader) (string, bool, error) {
	return s.readSchemaFileWithType(r, SchemaTypeSQL)
}

func (s *Scanner) readSchemaFileWithType(r io.Reader, schemaType SchemaType) (string, bool, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	hasMarker := false

	firstLine := true
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)

		if firstLine {
			firstLine = false
			switch schemaType {
			case SchemaTypeSQL:
				if strings.HasPrefix(strings.TrimSpace(line), "-- MIGRATION_SCHEMA") {
					hasMarker = true
				}
			case SchemaTypeYAML:
				// File name and path are enough
				hasMarker = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", false, err
	}

	return strings.Join(lines, "\n"), hasMarker, nil
}

func (s *Scanner) getModulePath(modPath, version string) string {
	// Try to find the module in the Go module cache
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		goPath = filepath.Join(home, "go")
	}

	// Clean the version string (remove +incompatible suffix if present)
	version = strings.TrimSuffix(version, "+incompatible")

	// Try standard module cache location
	cachePath := filepath.Join(goPath, "pkg", "mod", fmt.Sprintf("%s@%s", modPath, version))
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath
	}

	// Try with escaped module path (Go escapes certain characters in paths)
	escapedPath := strings.ReplaceAll(modPath, "/", "!")
	cachePath = filepath.Join(goPath, "pkg", "mod", fmt.Sprintf("%s@%s", escapedPath, version))
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath
	}

	return ""
}
