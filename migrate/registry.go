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
	"sync"
)

// Registry stores all registered migrations, preserving insertion order.
type Registry struct {
	mu         sync.RWMutex
	migrations map[string]*Migration
	order      []string
}

// NewRegistry creates an empty Registry. Used for testing; generated migration files use the global registry.
func NewRegistry() *Registry {
	return &Registry{migrations: make(map[string]*Migration)}
}

// Register adds a migration to this registry. Panics on duplicate names.
func (r *Registry) Register(m *Migration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.migrations[m.Name]; exists {
		panic(fmt.Sprintf("migration registration error: duplicate migration name %q", m.Name))
	}
	r.migrations[m.Name] = m
	r.order = append(r.order, m.Name)
}

// All returns all registered migrations in insertion order.
func (r *Registry) All() []*Migration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Migration, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.migrations[name])
	}
	return result
}

// Get returns a migration by name, and a boolean indicating whether it was found.
func (r *Registry) Get(name string) (*Migration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.migrations[name]
	return m, ok
}

// globalRegistry is populated by init() calls in generated migration files.
var globalRegistry = NewRegistry()

// Register adds a migration to the global registry.
// Called by each generated migration file's init() function. Panics on duplicates.
func Register(m *Migration) {
	globalRegistry.Register(m)
}

// GlobalRegistry returns the global registry. Used by app.go to build the migration graph.
func GlobalRegistry() *Registry {
	return globalRegistry
}
