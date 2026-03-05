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

package migrate_test

import (
	"testing"
	"time"

	"github.com/ocomsoft/makemigrations/migrate"
)

func TestFormatLiteral_nil(t *testing.T) {
	if got := migrate.FormatLiteral(nil); got != "NULL" {
		t.Errorf("nil: got %q, want %q", got, "NULL")
	}
}

func TestFormatLiteral_string(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "'hello'"},
		{"it's", "'it''s'"},
		{"", "''"},
	}
	for _, c := range cases {
		if got := migrate.FormatLiteral(c.in); got != c.want {
			t.Errorf("string %q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatLiteral_bool(t *testing.T) {
	if got := migrate.FormatLiteral(true); got != "TRUE" {
		t.Errorf("true: got %q", got)
	}
	if got := migrate.FormatLiteral(false); got != "FALSE" {
		t.Errorf("false: got %q", got)
	}
}

func TestFormatLiteral_integers(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{int(42), "42"},
		{int8(-1), "-1"},
		{int16(1000), "1000"},
		{int32(99), "99"},
		{int64(123456789), "123456789"},
		{uint(7), "7"},
		{uint64(18446744073709551615), "18446744073709551615"},
	}
	for _, c := range cases {
		if got := migrate.FormatLiteral(c.in); got != c.want {
			t.Errorf("%T(%v): got %q, want %q", c.in, c.in, got, c.want)
		}
	}
}

func TestFormatLiteral_floats(t *testing.T) {
	if got := migrate.FormatLiteral(float64(3.14)); got != "3.14" {
		t.Errorf("float64: got %q", got)
	}
	if got := migrate.FormatLiteral(float32(1.5)); got != "1.5" {
		t.Errorf("float32: got %q", got)
	}
}

func TestFormatLiteral_time(t *testing.T) {
	ts := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	if got := migrate.FormatLiteral(ts); got != "'2024-03-15 10:30:00'" {
		t.Errorf("time: got %q", got)
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]any{"zebra": 1, "alpha": 2, "mango": 3}
	keys := migrate.SortedKeys(m)
	want := []string{"alpha", "mango", "zebra"}
	if len(keys) != len(want) {
		t.Fatalf("len: got %d, want %d", len(keys), len(want))
	}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("keys[%d]: got %q, want %q", i, k, want[i])
		}
	}
}
