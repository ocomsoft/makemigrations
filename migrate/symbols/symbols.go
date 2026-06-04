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

// Package symbols holds the yaegi symbol map for morphic migrations.
//
// The map exposes the public API of github.com/ocomsoft/morphic/migrate
// to interpreted migration files so they can be loaded without invoking the Go
// toolchain. Users who write migrations whose RunSQL bodies (or other
// hand-written code) import third-party packages can call Register to add
// extra symbol maps before the CLI loads migrations.
package symbols

import "reflect"

// Symbols is the global yaegi symbol map. The package's own init() functions
// (in auto-generated files like migrate.go) populate it with entries for the
// migrate package. Third-party callers can add more entries via Register.
var Symbols = map[string]map[string]reflect.Value{}

// Register merges extra symbol maps into Symbols. It is the public extension
// point for projects whose migration source files import packages outside of
// the migrate package's symbol map. Generate the input map with
// `yaegi extract <pkg>` and call Register from an init() in your project.
func Register(extra map[string]map[string]reflect.Value) {
	for pkg, syms := range extra {
		if Symbols[pkg] == nil {
			Symbols[pkg] = map[string]reflect.Value{}
		}
		for name, val := range syms {
			Symbols[pkg][name] = val
		}
	}
}
