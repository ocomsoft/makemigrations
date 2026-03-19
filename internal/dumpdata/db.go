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

// Package dumpdata provides database introspection and row fetching utilities
// for the dump-data feature. It supports PostgreSQL, SQLite, MySQL, and TiDB
// for primary key detection, and generic row fetching for any SQL database.
package dumpdata

import (
	"database/sql"
	"fmt"
	"time"
)

// OpenDB opens a database connection using the given driver name and DSN,
// then verifies the connection is alive with a ping.
func OpenDB(driverName, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// DetectPrimaryKeys introspects the database to find primary key columns for
// the given table. The dbType parameter determines which SQL dialect to use
// for the introspection query. Supported values are "postgresql", "postgres",
// "sqlite", "mysql", and "tidb". For unsupported database types, it returns
// nil, nil so the caller can fall back to a user-specified conflict key.
func DetectPrimaryKeys(db *sql.DB, dbType, table string) ([]string, error) {
	switch dbType {
	case "postgresql", "postgres":
		return detectPrimaryKeysPostgres(db, table)
	case "sqlite":
		return detectPrimaryKeysSQLite(db, table)
	case "mysql", "tidb":
		return detectPrimaryKeysMySQL(db, table)
	default:
		return nil, nil
	}
}

// detectPrimaryKeysPostgres queries information_schema to find primary key
// columns for a PostgreSQL table.
func detectPrimaryKeysPostgres(db *sql.DB, table string) ([]string, error) {
	query := `SELECT kcu.column_name
FROM information_schema.key_column_usage kcu
JOIN information_schema.table_constraints tc
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_name = kcu.table_name
WHERE kcu.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'
ORDER BY kcu.ordinal_position`

	return queryStringColumn(db, query, table)
}

// detectPrimaryKeysSQLite uses PRAGMA table_info to find primary key columns
// for a SQLite table. Columns with pk > 0 are primary keys, sorted by the pk
// value in ascending order.
func detectPrimaryKeysSQLite(db *sql.DB, table string) ([]string, error) {
	// PRAGMA doesn't support parameterized queries, but table names come from
	// our own schema introspection, not user input.
	query := fmt.Sprintf("PRAGMA table_info(%s)", table) //nolint:gosec // table name from internal schema

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("PRAGMA table_info failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type colInfo struct {
		name string
		pk   int
	}

	var pkCols []colInfo

	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notnull int
		var dfltValue *string
		var pk int

		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan PRAGMA result: %w", err)
		}

		if pk > 0 {
			pkCols = append(pkCols, colInfo{name: name, pk: pk})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating PRAGMA results: %w", err)
	}

	if len(pkCols) == 0 {
		return nil, nil
	}

	// Sort by pk value (already in order from PRAGMA, but be explicit)
	// Simple insertion sort since PK columns are typically few
	for i := 1; i < len(pkCols); i++ {
		for j := i; j > 0 && pkCols[j].pk < pkCols[j-1].pk; j-- {
			pkCols[j], pkCols[j-1] = pkCols[j-1], pkCols[j]
		}
	}

	result := make([]string, len(pkCols))
	for i, col := range pkCols {
		result[i] = col.name
	}

	return result, nil
}

// detectPrimaryKeysMySQL queries information_schema to find primary key
// columns for a MySQL or TiDB table.
func detectPrimaryKeysMySQL(db *sql.DB, table string) ([]string, error) {
	query := `SELECT column_name FROM information_schema.key_column_usage
WHERE table_name = ? AND constraint_name = 'PRIMARY' AND table_schema = DATABASE()
ORDER BY ordinal_position`

	return queryStringColumn(db, query, table)
}

// queryStringColumn executes a query that returns a single string column and
// collects all results into a slice.
func queryStringColumn(db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []string

	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}

		result = append(result, val)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

// FetchRows queries all rows from the given table and returns them as a slice
// of maps (column name to value) along with the ordered column names. Values
// are normalized using NormalizeValue. It first tries ANSI SQL double-quoted
// table names and falls back to MySQL backtick quoting if the first attempt fails.
func FetchRows(db *sql.DB, table string) ([]map[string]any, []string, error) {
	// Try ANSI SQL double-quoted identifier first (works for PostgreSQL, SQLite)
	query := fmt.Sprintf(`SELECT * FROM "%s"`, table) //nolint:gosec // table name from internal schema
	rows, err := db.Query(query)

	if err != nil {
		// Fall back to backtick quoting for MySQL/TiDB
		query = fmt.Sprintf("SELECT * FROM `%s`", table) //nolint:gosec // table name from internal schema
		rows, err = db.Query(query)

		if err != nil {
			return nil, nil, fmt.Errorf("failed to query table %q: %w", table, err)
		}
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var result []map[string]any

	for rows.Next() {
		// Create a slice of pointers to any for scanning
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		rowMap := make(map[string]any, len(columns))
		for i, col := range columns {
			rowMap[col] = NormalizeValue(values[i])
		}

		result = append(result, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, columns, nil
}

// NormalizeValue converts driver-specific types to clean Go types suitable for
// serialization. It handles nil, []byte, time.Time, int64, float64, bool, and
// falls back to fmt.Sprintf for any other type.
func NormalizeValue(v any) any {
	switch val := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(val)
	case time.Time:
		return val.UTC().Format("2006-01-02 15:04:05.999999")
	case int64:
		return val
	case float64:
		return val
	case bool:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}
