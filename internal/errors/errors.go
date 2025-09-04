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
package errors

import (
	"fmt"
	"strings"
)

// Common error types for the makemigrations tool

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

type SchemaParseError struct {
	FilePath string
	Line     int
	Message  string
}

func (e SchemaParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("schema parse error in %s at line %d: %s", e.FilePath, e.Line, e.Message)
	}
	return fmt.Sprintf("schema parse error in %s: %s", e.FilePath, e.Message)
}

type DependencyError struct {
	TableName string
	Message   string
}

func (e DependencyError) Error() string {
	return fmt.Sprintf("dependency error for table %s: %s", e.TableName, e.Message)
}

type CircularDependencyError struct {
	Cycle []string
}

func (e CircularDependencyError) Error() string {
	return fmt.Sprintf("circular dependency detected: %s", strings.Join(e.Cycle, " -> "))
}

type MigrationError struct {
	Operation string
	Message   string
}

func (e MigrationError) Error() string {
	return fmt.Sprintf("migration error during %s: %s", e.Operation, e.Message)
}

// Error wrapping helpers
func NewValidationError(field, message string) error {
	return ValidationError{Field: field, Message: message}
}

func NewSchemaParseError(filePath string, line int, message string) error {
	return SchemaParseError{FilePath: filePath, Line: line, Message: message}
}

func NewDependencyError(tableName, message string) error {
	return DependencyError{TableName: tableName, Message: message}
}

func NewCircularDependencyError(cycle []string) error {
	return CircularDependencyError{Cycle: cycle}
}

func NewMigrationError(operation, message string) error {
	return MigrationError{Operation: operation, Message: message}
}

// Utility functions for error checking
func IsValidationError(err error) bool {
	_, ok := err.(ValidationError)
	return ok
}

func IsSchemaParseError(err error) bool {
	_, ok := err.(SchemaParseError)
	return ok
}

func IsDependencyError(err error) bool {
	_, ok := err.(DependencyError)
	return ok
}

func IsCircularDependencyError(err error) bool {
	_, ok := err.(CircularDependencyError)
	return ok
}

func IsMigrationError(err error) bool {
	_, ok := err.(MigrationError)
	return ok
}
