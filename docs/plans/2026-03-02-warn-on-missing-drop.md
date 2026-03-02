# warn-on-missing-drop Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--warn-on-missing-drop` to `migrate up` and `migrate down` so that DROP operations that fail because the object doesn't exist print a warning and continue instead of stopping.

**Architecture:** Each Provider gains `IsNotFoundError(err error) bool` to classify DB-specific "object not found" errors. The Runner receives a `RunOptions` struct and, when `WarnOnMissingDrop` is set, catches those errors on drop operations and warns instead of failing. Both `up` and `down` CLI flags forward the option.

**Tech Stack:** Go, `database/sql`, Cobra, existing `providers.Provider` interface.

---

### Task 1: Add `IsNotFoundError` to the Provider interface

**Files:**
- Modify: `internal/providers/provider.go`

**Step 1: Add the method to the interface**

Open `internal/providers/provider.go`. Add this method to the `Provider` interface, after `HistoryTableDDL()`:

```go
// IsNotFoundError returns true when err indicates that a DROP operation
// targeted an object that does not exist in the database.
// Used by the runner to warn-and-continue rather than fail.
IsNotFoundError(err error) bool
```

**Step 2: Build to confirm the interface change breaks all providers as expected**

```bash
go build ./...
```

Expected: compile errors for every provider — `does not implement Provider (missing method IsNotFoundError)`. That's correct — each provider needs implementing.

**Step 3: Commit the interface change alone**

```bash
git add internal/providers/provider.go
git commit -m "feat(providers): add IsNotFoundError to Provider interface"
```

---

### Task 2: Implement `IsNotFoundError` — PostgreSQL

**Files:**
- Modify: `internal/providers/postgresql/provider.go`

PostgreSQL returns errors via the `pq` driver. "Does not exist" errors always contain the phrase `"does not exist"` in the message.

**Step 1: Write the failing test**

There is no existing `provider_test.go` for PostgreSQL. Create `internal/providers/postgresql/provider_test.go`:

```go
package postgresql

import (
	"errors"
	"testing"
)

func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()

	cases := []struct {
		err     error
		want    bool
	}{
		{errors.New(`pq: table "users" does not exist`), true},
		{errors.New(`pq: column "email" of relation "users" does not exist`), true},
		{errors.New(`pq: index "idx_users_email" does not exist`), true},
		{errors.New(`pq: syntax error at or near "DROP"`), false},
		{errors.New(`connection refused`), false},
		{nil, false},
	}

	for _, tc := range cases {
		got := p.IsNotFoundError(tc.err)
		if got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**Step 2: Run to verify it fails**

```bash
go test ./internal/providers/postgresql/ -run TestProvider_IsNotFoundError -v
```

Expected: compile error — `p.IsNotFoundError undefined`.

**Step 3: Implement**

Add to `internal/providers/postgresql/provider.go` (add `"strings"` import if not present):

```go
// IsNotFoundError returns true when err is a PostgreSQL "does not exist" error.
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "does not exist")
}
```

**Step 4: Run tests**

```bash
go test ./internal/providers/postgresql/ -run TestProvider_IsNotFoundError -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/providers/postgresql/provider.go internal/providers/postgresql/provider_test.go
git commit -m "feat(providers/postgresql): implement IsNotFoundError"
```

---

### Task 3: Implement `IsNotFoundError` — MySQL

**Files:**
- Modify: `internal/providers/mysql/provider.go`

MySQL uses numeric error codes. Error 1051 = "unknown table", Error 1091 = "can't drop key/index (check that it exists)".
The `go-sql-driver/mysql` driver wraps these: `Error 1051: Unknown table 'foo'`.

**Step 1: Write the failing test**

Create `internal/providers/mysql/provider_test.go`:

```go
package mysql

import (
	"errors"
	"testing"
)

func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()

	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("Error 1051: Unknown table 'users'"), true},
		{errors.New("Error 1091: Can't DROP 'idx_email'; check that column/key exists"), true},
		{errors.New("Error 1049: Unknown database 'mydb'"), false},
		{errors.New("connection refused"), false},
		{nil, false},
	}

	for _, tc := range cases {
		got := p.IsNotFoundError(tc.err)
		if got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**Step 2: Run to verify it fails**

```bash
go test ./internal/providers/mysql/ -run TestProvider_IsNotFoundError -v
```

Expected: compile error — `p.IsNotFoundError undefined`.

**Step 3: Implement**

Add to `internal/providers/mysql/provider.go` (add `"strings"` import if not present):

```go
// IsNotFoundError returns true when err is a MySQL "unknown table" or
// "can't drop key" error (codes 1051 and 1091).
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1051") || strings.Contains(msg, "Error 1091")
}
```

**Step 4: Run tests**

```bash
go test ./internal/providers/mysql/ -run TestProvider_IsNotFoundError -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/providers/mysql/provider.go internal/providers/mysql/provider_test.go
git commit -m "feat(providers/mysql): implement IsNotFoundError"
```

---

### Task 4: Implement `IsNotFoundError` — SQLite

**Files:**
- Modify: `internal/providers/sqlite/provider.go`

SQLite returns messages like `"no such table: users"`, `"no such column: email"`, `"no such index: idx_email"`.

**Step 1: Write the failing test**

Create `internal/providers/sqlite/provider_test.go`:

```go
package sqlite

import (
	"errors"
	"testing"
)

func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()

	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("no such table: users"), true},
		{errors.New("no such column: email"), true},
		{errors.New("no such index: idx_email"), true},
		{errors.New("UNIQUE constraint failed: users.email"), false},
		{errors.New("connection refused"), false},
		{nil, false},
	}

	for _, tc := range cases {
		got := p.IsNotFoundError(tc.err)
		if got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**Step 2: Run to verify it fails**

```bash
go test ./internal/providers/sqlite/ -run TestProvider_IsNotFoundError -v
```

Expected: compile error.

**Step 3: Implement**

Add to `internal/providers/sqlite/provider.go` (add `"strings"` import if not present):

```go
// IsNotFoundError returns true when err is a SQLite "no such table/column/index" error.
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "no such table:") ||
		strings.HasPrefix(msg, "no such column:") ||
		strings.HasPrefix(msg, "no such index:")
}
```

**Step 4: Run tests**

```bash
go test ./internal/providers/sqlite/ -run TestProvider_IsNotFoundError -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/providers/sqlite/provider.go internal/providers/sqlite/provider_test.go
git commit -m "feat(providers/sqlite): implement IsNotFoundError"
```

---

### Task 5: Implement `IsNotFoundError` — SQL Server

**Files:**
- Modify: `internal/providers/sqlserver/provider.go`

SQL Server error 3701: "Cannot drop … because it does not exist or you do not have permission."
Error 4902: "Cannot find the object … because it does not exist or you do not have permission."
The `denisenkom/go-mssqldb` driver formats these as: `mssql: Cannot drop …`.

**Step 1: Write the failing test**

Create `internal/providers/sqlserver/provider_test.go`:

```go
package sqlserver

import (
	"errors"
	"testing"
)

func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()

	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("mssql: Cannot drop the table 'users', because it does not exist or you do not have permission."), true},
		{errors.New("mssql: Cannot find the object \"idx_email\" because it does not exist or you do not have permission."), true},
		{errors.New("mssql: Invalid object name 'users'."), false},
		{errors.New("connection refused"), false},
		{nil, false},
	}

	for _, tc := range cases {
		got := p.IsNotFoundError(tc.err)
		if got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**Step 2: Run to verify it fails**

```bash
go test ./internal/providers/sqlserver/ -run TestProvider_IsNotFoundError -v
```

**Step 3: Implement**

Add to `internal/providers/sqlserver/provider.go` (add `"strings"` import if not present):

```go
// IsNotFoundError returns true when err is a SQL Server "does not exist" error
// (error codes 3701 / 4902).
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "does not exist or you do not have permission")
}
```

**Step 4: Run tests**

```bash
go test ./internal/providers/sqlserver/ -run TestProvider_IsNotFoundError -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/providers/sqlserver/provider.go internal/providers/sqlserver/provider_test.go
git commit -m "feat(providers/sqlserver): implement IsNotFoundError"
```

---

### Task 6: Implement `IsNotFoundError` — remaining 8 providers

Each provider gets the same pattern. Do them all in one task since they are mechanical.

**Files:**
- Modify: `internal/providers/clickhouse/provider.go`
- Modify: `internal/providers/redshift/provider.go`
- Modify: `internal/providers/starrocks/provider.go`
- Modify: `internal/providers/tidb/provider.go`
- Modify: `internal/providers/turso/provider.go`
- Modify: `internal/providers/vertica/provider.go`
- Modify: `internal/providers/ydb/provider.go`
- Modify: `internal/providers/auroradsql/provider.go`
- Test: existing `provider_test.go` in each package

**Step 1: Add `TestProvider_IsNotFoundError` to each existing `provider_test.go`**

For each provider, append a test. Use the error strings appropriate to each:

**clickhouse** — errors contain `"doesn't exist"` or `"UNKNOWN_TABLE"`:
```go
func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct{ err error; want bool }{
		{errors.New("Code: 60. DB::Exception: Table default.users doesn't exist."), true},
		{errors.New("Code: 36. DB::Exception: Unknown DROP query"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**redshift** — same driver as PostgreSQL, same "does not exist" pattern:
```go
func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct{ err error; want bool }{
		{errors.New(`pq: table "users" does not exist`), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**starrocks** — MySQL-compatible errors (1051 / 1091):
```go
func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct{ err error; want bool }{
		{errors.New("Error 1051: Unknown table 'users'"), true},
		{errors.New("Error 1091: Can't DROP 'idx_email'; check that column/key exists"), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**tidb** — MySQL-compatible errors (1051 / 1091):
```go
func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct{ err error; want bool }{
		{errors.New("Error 1051: Unknown table 'users'"), true},
		{errors.New("Error 1091: Can't DROP 'idx_email'; check that column/key exists"), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**turso** — SQLite-compatible errors:
```go
func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct{ err error; want bool }{
		{errors.New("no such table: users"), true},
		{errors.New("no such column: email"), true},
		{errors.New("no such index: idx_email"), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**vertica** — errors contain `"does not exist"`:
```go
func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct{ err error; want bool }{
		{errors.New("ERROR 4566:  Table \"public\".\"users\" does not exist"), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**ydb** — errors contain `"not found"` or `"does not exist"`:
```go
func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct{ err error; want bool }{
		{errors.New("ydb: path not found"), true},
		{errors.New("ydb: table does not exist"), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**auroradsql** — PostgreSQL-compatible errors:
```go
func TestProvider_IsNotFoundError(t *testing.T) {
	p := New()
	cases := []struct{ err error; want bool }{
		{errors.New(`pq: table "users" does not exist`), true},
		{errors.New("connection refused"), false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := p.IsNotFoundError(tc.err); got != tc.want {
			t.Errorf("IsNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
```

**Step 2: Run tests — confirm they all fail**

```bash
go test ./internal/providers/... -run TestProvider_IsNotFoundError -v 2>&1 | grep -E "FAIL|PASS|compile"
```

**Step 3: Implement `IsNotFoundError` in each provider**

Add to each provider file (add `"strings"` import where missing):

```go
// clickhouse/provider.go
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "doesn't exist")
}

// redshift/provider.go
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "does not exist")
}

// starrocks/provider.go
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1051") || strings.Contains(msg, "Error 1091")
}

// tidb/provider.go
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1051") || strings.Contains(msg, "Error 1091")
}

// turso/provider.go
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "no such table:") ||
		strings.HasPrefix(msg, "no such column:") ||
		strings.HasPrefix(msg, "no such index:")
}

// vertica/provider.go
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "does not exist")
}

// ydb/provider.go
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist")
}

// auroradsql/provider.go
func (p *Provider) IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "does not exist")
}
```

**Step 4: Run all provider tests**

```bash
go test ./internal/providers/... -v 2>&1 | tail -20
```

Expected: all PASS, no compile errors.

**Step 5: Commit**

```bash
git add internal/providers/
git commit -m "feat(providers): implement IsNotFoundError for all providers"
```

---

### Task 7: Add `RunOptions` and update Runner

**Files:**
- Modify: `migrate/runner.go`
- Modify: `migrate/runner_test.go`

**Step 1: Write failing integration test**

Add to `migrate/runner_test.go`. This test applies a migration that drops a table, but the table doesn't exist in the DB. Without the flag it should fail; with it it should warn and succeed.

```go
func TestRunner_Up_WarnOnMissingDrop_SkipsIfNotFound(t *testing.T) {
	db := openTestDB(t)
	p := &sqlite.Provider{}  // import: "github.com/ocomsoft/makemigrations/internal/providers/sqlite"
	rec := migrate.NewMigrationRecorder(db, p)
	_ = rec.EnsureTable()

	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_drop_users",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.DropTable{Name: "users"}, // table does not exist in DB
		},
	})

	g, _ := migrate.BuildGraph(reg)
	runner := migrate.NewRunner(g, p, db, rec)

	// Without flag — should fail
	err := runner.Up("", migrate.RunOptions{})
	if err == nil {
		t.Fatal("expected error when table does not exist and WarnOnMissingDrop is false")
	}

	// Reset history so we can run again
	_, _ = db.Exec("DELETE FROM makemigrations_history")

	// With flag — should warn and succeed
	err = runner.Up("", migrate.RunOptions{WarnOnMissingDrop: true})
	if err != nil {
		t.Fatalf("expected no error with WarnOnMissingDrop=true, got: %v", err)
	}
}
```

**Step 2: Run to verify it fails**

```bash
go test ./migrate/ -run TestRunner_Up_WarnOnMissingDrop -v
```

Expected: compile error — `RunOptions` undefined, `Up` signature mismatch.

**Step 3: Add `RunOptions` struct and update Runner**

In `migrate/runner.go`:

1. Add `RunOptions` struct **before** the `Runner` struct:

```go
// RunOptions controls optional behaviour for Up and Down.
type RunOptions struct {
	// WarnOnMissingDrop causes drop operations that fail because the target
	// object does not exist to print a warning and continue rather than stop.
	WarnOnMissingDrop bool
}
```

2. Update `Up` and `Down` signatures:

```go
func (r *Runner) Up(to string, opts RunOptions) error {
```

```go
func (r *Runner) Down(steps int, to string, opts RunOptions) error {
```

3. Pass `opts` to `applyMigration` and `rollbackMigration`:

```go
func (r *Runner) applyMigration(mig *Migration, state *SchemaState, opts RunOptions) error {
```

```go
func (r *Runner) rollbackMigration(mig *Migration, state *SchemaState, opts RunOptions) error {
```

4. In `applyMigration`, after generating SQL, wrap `db.Exec` for drop operations:

```go
func (r *Runner) applyMigration(mig *Migration, state *SchemaState, opts RunOptions) error {
	for _, op := range mig.Operations {
		sqlStr, err := op.Up(r.provider, state, nil)
		if err != nil {
			return fmt.Errorf("generating SQL for operation %q: %w", op.Describe(), err)
		}
		if sqlStr != "" {
			if _, execErr := r.db.Exec(sqlStr); execErr != nil {
				if opts.WarnOnMissingDrop && isDropOp(op) && r.provider.IsNotFoundError(execErr) {
					fmt.Printf("[WARNING] %s — object not found in database, skipping\n", op.Describe())
				} else {
					return fmt.Errorf("executing SQL %q: %w", sqlStr, execErr)
				}
			}
		}
		if err := op.Mutate(state); err != nil {
			return fmt.Errorf("mutating state: %w", err)
		}
	}
	return r.recorder.RecordApplied(mig.Name)
}
```

5. Apply the same pattern to `rollbackMigration` (down direction):

```go
func (r *Runner) rollbackMigration(mig *Migration, state *SchemaState, opts RunOptions) error {
	for i := len(mig.Operations) - 1; i >= 0; i-- {
		op := mig.Operations[i]
		sqlStr, err := op.Down(r.provider, state, nil)
		if err != nil {
			return fmt.Errorf("generating down SQL for %q: %w", op.Describe(), err)
		}
		if sqlStr != "" {
			if _, execErr := r.db.Exec(sqlStr); execErr != nil {
				if opts.WarnOnMissingDrop && isDropOp(op) && r.provider.IsNotFoundError(execErr) {
					fmt.Printf("[WARNING] %s — object not found in database, skipping\n", op.Describe())
				} else {
					return fmt.Errorf("executing down SQL %q: %w", sqlStr, execErr)
				}
			}
		}
	}
	return r.recorder.RecordRolledBack(mig.Name)
}
```

6. Add `isDropOp` helper at the bottom of `runner.go`:

```go
// isDropOp returns true for operations that remove a database object.
// Only drop operations use WarnOnMissingDrop — other failures always stop.
func isDropOp(op Operation) bool {
	switch op.TypeName() {
	case "drop_table", "drop_field", "drop_index":
		return true
	default:
		return false
	}
}
```

7. Fix all `Up` / `Down` / `applyMigration` / `rollbackMigration` call sites within `runner.go` to pass `opts`.

**Step 4: Fix `ShowSQL` and `Status` — they call `Up`/`Down` indirectly**

`Status` and `ShowSQL` do not call `Up`/`Down` directly in runner.go — they only use `GetApplied`. No changes needed there.

**Step 5: Run tests**

```bash
go test ./migrate/ -run TestRunner_Up_WarnOnMissingDrop -v
```

Expected: PASS. Then run full suite:

```bash
go test ./migrate/... 2>&1 | tail -5
```

Expected: all PASS.

**Step 6: Commit**

```bash
git add migrate/runner.go migrate/runner_test.go
git commit -m "feat(runner): add RunOptions.WarnOnMissingDrop to Up and Down"
```

---

### Task 8: Update `app.go` to wire the flag through to Runner

**Files:**
- Modify: `migrate/app.go`
- Modify: `migrate/app_test.go`

**Step 1: Write failing test**

Add to `migrate/app_test.go`:

```go
func TestApp_Run_Up_WarnOnMissingDrop_Flag(t *testing.T) {
	// The table does not exist — without the flag this would fail;
	// with the flag it should warn and succeed.
	dbPath := filepath.Join(t.TempDir(), "test_warn_drop.db")
	reg := migrate.NewRegistry()
	reg.Register(&migrate.Migration{
		Name:         "0001_drop_missing",
		Dependencies: []string{},
		Operations: []migrate.Operation{
			&migrate.DropTable{Name: "nonexistent_table"},
		},
	})

	cfg := migrate.Config{DatabaseType: "sqlite", DBName: dbPath}
	app := migrate.NewAppWithRegistry(cfg, reg)

	origStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = origStdout
		_ = devNull.Close()
	}()

	err := app.Run([]string{"up", "--warn-on-missing-drop"})
	if err != nil {
		t.Fatalf("up --warn-on-missing-drop failed: %v", err)
	}
}
```

**Step 2: Run to verify it fails**

```bash
go test ./migrate/ -run TestApp_Run_Up_WarnOnMissingDrop -v
```

Expected: error — unknown flag `--warn-on-missing-drop`.

**Step 3: Update `app.go`**

In `buildUpCommand`:

```go
func (a *App) buildUpCommand() *cobra.Command {
	var toMigration string
	var warnOnMissingDrop bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Apply pending migrations",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.runUp(toMigration, migrate.RunOptions{WarnOnMissingDrop: warnOnMissingDrop})
		},
	}
	cmd.Flags().StringVar(&toMigration, "to", "", "Apply up to this migration name")
	cmd.Flags().BoolVar(&warnOnMissingDrop, "warn-on-missing-drop", false,
		"Warn and continue when a drop fails because the object does not exist")
	return cmd
}
```

In `buildDownCommand`:

```go
func (a *App) buildDownCommand() *cobra.Command {
	var steps int
	var toMigration string
	var warnOnMissingDrop bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Rollback migrations",
		RunE: func(_ *cobra.Command, _ []string) error {
			return a.runDown(steps, toMigration, migrate.RunOptions{WarnOnMissingDrop: warnOnMissingDrop})
		},
	}
	cmd.Flags().IntVar(&steps, "steps", 1, "Number of migrations to roll back")
	cmd.Flags().StringVar(&toMigration, "to", "", "Roll back to this migration name")
	cmd.Flags().BoolVar(&warnOnMissingDrop, "warn-on-missing-drop", false,
		"Warn and continue when a drop fails because the object does not exist")
	return cmd
}
```

Update `runUp` and `runDown` signatures:

```go
func (a *App) runUp(to string, opts RunOptions) error {
	r, err := a.buildRunner()
	if err != nil {
		return err
	}
	return r.Up(to, opts)
}

func (a *App) runDown(steps int, to string, opts RunOptions) error {
	r, err := a.buildRunner()
	if err != nil {
		return err
	}
	return r.Down(steps, to, opts)
}
```

**Step 4: Run tests**

```bash
go test ./migrate/ -run TestApp_Run_Up_WarnOnMissingDrop -v
```

Expected: PASS. Then full suite:

```bash
go test ./migrate/... 2>&1 | tail -5
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add migrate/app.go migrate/app_test.go
git commit -m "feat(app): wire --warn-on-missing-drop flag to up and down commands"
```

---

### Task 9: Run linter and fix any issues

**Step 1: Run goimports then golangci-lint**

```bash
goimports -w migrate/runner.go migrate/app.go
golangci-lint run --no-config ./...
```

Expected: 0 issues on new code (pre-existing issues in db2schema.go, goose.go, schema2diagram.go are acceptable).

**Step 2: Run full test suite one final time**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all packages PASS.

**Step 3: Commit if any lint fixes were needed**

```bash
git add -p
git commit -m "style: goimports and lint fixes for warn-on-missing-drop"
```

---

### Task 10: Update documentation

**Files:**
- Modify: `docs/commands/migrate.md`

**Step 1: Update the `up` flags table**

Find the `up` flags table and add the new flag:

| Flag | Default | Description |
|------|---------|-------------|
| `--to` | (none) | Stop after applying the named migration |
| `--warn-on-missing-drop` | `false` | Warn and continue when a DROP fails because the object does not exist |

**Step 2: Update the `down` flags table**

Find the `down` flags table and add the new flag:

| Flag | Default | Description |
|------|---------|-------------|
| `--steps` | `1` | Number of migrations to roll back |
| `--to` | (none) | Roll back until (but not including) this migration name |
| `--warn-on-missing-drop` | `false` | Warn and continue when a DROP fails because the object does not exist |

**Step 3: Add a usage example in the `up` section**

```bash
# Apply migrations, skipping drops for objects already removed manually
./migrations/migrate up --warn-on-missing-drop
```

**Step 4: Add a troubleshooting entry**

Under the Troubleshooting section, add:

```markdown
### Migration fails on DROP TABLE / DROP COLUMN / DROP INDEX

If the object was already removed from the database manually, use `--warn-on-missing-drop`:

```bash
makemigrations migrate up --warn-on-missing-drop
```

This prints a `[WARNING]` line for each skipped drop and continues. The migration is still recorded as applied.
```

**Step 5: Commit**

```bash
git add docs/commands/migrate.md
git commit -m "docs(migrate): document --warn-on-missing-drop flag"
```

---

### Task 11: Final verification

**Step 1: Run full test suite**

```bash
go test ./... 2>&1 | tail -20
```

Expected: all PASS, output pristine.

**Step 2: Verify build**

```bash
go build ./...
```

Expected: no errors.

**Step 3: Verify lint**

```bash
golangci-lint run --no-config ./... 2>&1 | grep -v "db2schema\|goose\.go\|schema2diagram"
```

Expected: 0 issues on new code.
