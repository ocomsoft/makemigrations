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

	"github.com/ocomsoft/makemigrations/internal/providers"
	"github.com/ocomsoft/makemigrations/internal/types"
)

// BuildProviderFromType creates a Provider from a database type string.
// It delegates to providers.NewProvider which supports all registered
// database types (postgresql, mysql, sqlite, sqlserver, etc.).
func BuildProviderFromType(dbType string) (providers.Provider, error) {
	dt, err := types.ParseDatabaseType(dbType)
	if err != nil {
		return nil, fmt.Errorf("parsing database type %q: %w", dbType, err)
	}
	p, err := providers.NewProvider(dt, nil)
	if err != nil {
		return nil, fmt.Errorf("creating provider for %q: %w", dbType, err)
	}
	return p, nil
}
