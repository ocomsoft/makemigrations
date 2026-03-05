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

package migrate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultRef is a string type that marks a row value as a reference to a named
// schema default rather than a literal value. When UpsertData.Up encounters a
// DefaultRef it resolves the key through the active defaults map (set by the
// SetDefaults operation) and emits the resolved SQL expression verbatim —
// without quoting it as a string literal.
//
// If the key is not present in the defaults map, the key itself is emitted as
// a raw SQL expression (useful for calling built-in functions directly, e.g.
// DefaultRef("NOW()") or DefaultRef("gen_random_uuid()")).
//
// Example:
//
//	&m.UpsertData{
//	    Table:        "items",
//	    ConflictKeys: []string{"code"},
//	    Rows: []map[string]any{
//	        {"id": m.DefaultRef("uuid"), "code": "AU", "name": "Australia"},
//	    },
//	}
//
// With SetDefaults{"uuid": "uuid_generate_v4()"} active this produces:
//
//	INSERT INTO "items" ("code", "id", "name")
//	VALUES ('AU', uuid_generate_v4(), 'Australia')
//	ON CONFLICT ("code") DO UPDATE SET ...
type DefaultRef string

// FormatLiteral converts a Go value into a SQL literal string suitable for
// embedding directly in a SQL statement. It is used by UpsertData.Up to
// pre-format row values before passing them to the provider's GenerateUpsert.
// DefaultRef values are NOT handled here — they are resolved in UpsertData.Up
// before FormatLiteral is called.
//
// Supported types:
//   - nil              → NULL
//   - string           → 'value' (single quotes escaped as ”)
//   - bool             → TRUE / FALSE
//   - int variants     → decimal literal
//   - uint variants    → decimal literal
//   - float32/float64  → decimal literal (strconv.FormatFloat, 'f' format)
//   - time.Time        → 'YYYY-MM-DD HH:MM:SS'
//   - fmt.Stringer     → quoted using the Stringer output
//   - everything else  → quoted using fmt.Sprintf("%v")
func FormatLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	case int:
		return strconv.Itoa(val)
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case time.Time:
		return "'" + val.UTC().Format("2006-01-02 15:04:05") + "'"
	case fmt.Stringer:
		return "'" + strings.ReplaceAll(val.String(), "'", "''") + "'"
	default:
		return "'" + strings.ReplaceAll(fmt.Sprintf("%v", val), "'", "''") + "'"
	}
}

// SortedKeys returns the keys of the map in alphabetical order.
// Used by UpsertData.Up to establish a consistent column ordering across rows.
func SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
