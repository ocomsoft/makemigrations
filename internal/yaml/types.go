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
	"github.com/ocomsoft/makemigrations/internal/types"
)

// Re-export types for backwards compatibility
type Include = types.Include
type Schema = types.Schema
type Database = types.Database
type Defaults = types.Defaults
type Table = types.Table
type Field = types.Field
type ForeignKey = types.ForeignKey
type ManyToMany = types.ManyToMany
type Index = types.Index
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
