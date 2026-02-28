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

// Package gooseparser parses Goose-format SQL migration files into forward
// and backward SQL strings for use in RunSQL migration operations.
package gooseparser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Section constants for tracking which part of the migration file we are parsing.
const (
	sectionNone     = 0
	sectionForward  = 1
	sectionBackward = 2
)

// Migration holds the parsed content of a Goose SQL migration file.
type Migration struct {
	// ForwardSQL is the SQL from the -- +goose Up section.
	ForwardSQL string
	// BackwardSQL is the SQL from the -- +goose Down section.
	// Empty if no Down section is present.
	BackwardSQL string
}

// ParseFile reads a Goose-format .sql file and returns the parsed Migration.
// Goose markers (-- +goose Up, -- +goose Down, -- +goose StatementBegin,
// -- +goose StatementEnd) are stripped; all other content is preserved.
func ParseFile(path string) (Migration, error) {
	f, err := os.Open(path)
	if err != nil {
		return Migration{}, fmt.Errorf("opening migration file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var (
		forward  strings.Builder
		backward strings.Builder
		section  = sectionNone
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check for section-changing markers.
		if trimmed == "-- +goose Up" {
			section = sectionForward
			continue
		}
		if trimmed == "-- +goose Down" {
			section = sectionBackward
			continue
		}

		// Strip StatementBegin/StatementEnd markers.
		if trimmed == "-- +goose StatementBegin" || trimmed == "-- +goose StatementEnd" {
			continue
		}

		// Accumulate SQL into the active section.
		switch section {
		case sectionForward:
			if forward.Len() > 0 {
				forward.WriteString("\n")
			}
			forward.WriteString(line)
		case sectionBackward:
			if backward.Len() > 0 {
				backward.WriteString("\n")
			}
			backward.WriteString(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return Migration{}, fmt.Errorf("reading migration file: %w", err)
	}

	return Migration{
		ForwardSQL:  strings.TrimSpace(forward.String()),
		BackwardSQL: strings.TrimSpace(backward.String()),
	}, nil
}

// ExtractDescription strips the leading numeric prefix and file extension from
// a Goose migration filename, returning just the description part.
// For example, "20240101120000_initial.sql" returns "initial".
func ExtractDescription(filename string) string {
	// Strip directory and extension.
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	// Find the first underscore; everything after it is the description.
	idx := strings.Index(base, "_")
	if idx < 0 {
		return base
	}
	return base[idx+1:]
}

// ExtractVersionID parses the numeric prefix from a Goose migration filename
// as an int64. This matches the version_id stored in goose_db_version.
// For example, "00001_initial.sql" returns 1.
func ExtractVersionID(filename string) (int64, error) {
	// Strip directory and extension.
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	// Take prefix before first underscore.
	prefix := base
	if idx := strings.Index(base, "_"); idx >= 0 {
		prefix = base[:idx]
	}

	id, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing version id from %q: %w", filename, err)
	}
	return id, nil
}
