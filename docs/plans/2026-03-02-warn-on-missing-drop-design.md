# Design: --warn-on-missing-drop flag

**Date:** 2026-03-02
**Status:** Approved

## Problem

When running `migrate up` or `migrate down`, if a DROP TABLE / DROP COLUMN / DROP INDEX
fails because the object does not exist (e.g. manually removed from the database),
the runner stops the migration entirely with an error. There is no way to continue past
a missing-object drop.

## Solution

Add a `--warn-on-missing-drop` flag to both `up` and `down`. When set:

- Drop operations that fail because the object does not exist print a `[WARNING]` line
  and continue.
- All other errors still fail as normal.

## Detection Approach

Each Provider gains a new method:

```go
IsNotFoundError(err error) bool
```

This method classifies a raw database error as "object does not exist". Each provider
implements it using its own error codes / message patterns:

| Provider       | Detection method                                        |
|----------------|---------------------------------------------------------|
| PostgreSQL     | `"does not exist"` in error string                     |
| MySQL / TiDB   | Error 1051 (unknown table), 1091 (can't drop key)      |
| SQLite / Turso | `"no such table"`, `"no such column"`, `"no such index"` |
| SQL Server     | Error 3701 (cannot drop object), 4902               |
| ClickHouse     | `"doesn't exist"` in error string                      |
| Redshift       | Same as PostgreSQL (both use pq driver)                 |
| Vertica        | `"does not exist"` in error string                     |
| StarRocks      | MySQL-compatible error codes                           |
| YDB            | `"not found"` / `"does not exist"`                     |
| Aurora DSQL    | Same as PostgreSQL                                      |

## Architecture

### 1. Provider interface (`internal/providers/provider.go`)

```go
// IsNotFoundError returns true when err indicates the object targeted by a
// DROP operation does not exist in the database.
IsNotFoundError(err error) bool
```

### 2. Runner options (`migrate/runner.go`)

```go
// RunOptions controls optional runner behaviour.
type RunOptions struct {
    WarnOnMissingDrop bool
}
```

`Runner.Up(to string, opts RunOptions)` and `Runner.Down(steps int, to string, opts RunOptions)`
accept options. Internal helpers `applyMigration` and `rollbackMigration` receive `opts`.

When executing SQL for a drop operation (`op.TypeName()` in `{"drop_table","drop_field","drop_index"}`):

```
exec SQL
if err != nil:
    if opts.WarnOnMissingDrop && provider.IsNotFoundError(err):
        fmt.Printf("[WARNING] %s — object not found, skipping\n", op.Describe())
        continue
    return err
```

### 3. App commands (`migrate/app.go`)

`buildUpCommand` and `buildDownCommand` each gain:

```
--warn-on-missing-drop   Warn and continue when a drop fails because the object does not exist
```

### 4. Docs (`docs/commands/migrate.md`)

Add `--warn-on-missing-drop` to the `up` and `down` flag tables with a usage note
and a troubleshooting entry.

## Testing

- **Unit:** `TestProvider_IsNotFoundError` for each provider — real error values returned
  by the driver, plus non-matching errors that must return false.
- **Integration (SQLite):** Apply a migration that drops a table; manually drop the table first;
  re-run `migrate up --warn-on-missing-drop` — expect warning printed, no error returned.
- **Existing tests:** All must continue passing.

## Out of Scope

- Detecting missing-object errors for non-drop operations (ALTER, RENAME).
- Automatic retry or schema repair.
