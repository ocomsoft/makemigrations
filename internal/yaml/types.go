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
package yaml

import (
	"github.com/ocomsoft/morphic/internal/types"
)

// Include is an alias for types.Include for backwards compatibility.
type Include = types.Include

// Schema is an alias for types.Schema for backwards compatibility.
type Schema = types.Schema

// Database is an alias for types.Database for backwards compatibility.
type Database = types.Database

// Defaults is an alias for types.Defaults for backwards compatibility.
type Defaults = types.Defaults

// TypeMappings is an alias for types.TypeMappings for backwards compatibility.
type TypeMappings = types.TypeMappings

// Table is an alias for types.Table for backwards compatibility.
type Table = types.Table

// Field is an alias for types.Field for backwards compatibility.
type Field = types.Field

// ForeignKey is an alias for types.ForeignKey for backwards compatibility.
type ForeignKey = types.ForeignKey

// ManyToMany is an alias for types.ManyToMany for backwards compatibility.
type ManyToMany = types.ManyToMany

// Index is an alias for types.Index for backwards compatibility.
type Index = types.Index

// DatabaseType is an alias for types.DatabaseType for backwards compatibility.
type DatabaseType = types.DatabaseType

// Re-export constants
const (
	DatabasePostgreSQL = types.DatabasePostgreSQL
	DatabaseMySQL      = types.DatabaseMySQL
	DatabaseSQLServer  = types.DatabaseSQLServer
	DatabaseSQLite     = types.DatabaseSQLite
)

// Re-export variables
var ValidFieldTypes = types.ValidFieldTypes

// Re-export functions
var ParseDatabaseType = types.ParseDatabaseType
var IsValidFieldType = types.IsValidFieldType
var IsValidDatabase = types.IsValidDatabase
