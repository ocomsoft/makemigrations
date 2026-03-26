# dump-data

## Purpose

The `dump-data` command connects to a live database, reads all rows from the
specified tables, and generates a `.go` migration file containing `UpsertData`
operations for each table. This is useful for seeding reference or lookup data
(e.g. country codes, unit types, roles) so that the data can be version-controlled
and applied consistently across environments via the normal migration workflow.

## Usage

```
makemigrations dump-data [table1 table2 ...] [flags]
```

## Flags

| Flag             | Type     | Default     | Description                                                                                      |
|------------------|----------|-------------|--------------------------------------------------------------------------------------------------|
| `--name`         | string   | `""`        | Custom migration name suffix                                                                     |
| `--dry-run`      | bool     | `false`     | Print generated source without writing                                                           |
| `--verbose`      | bool     | `false`     | Show connection and row-count details                                                            |
| `--conflict-key` | []string | (auto)      | PK columns for ON CONFLICT; applied to all tables; required if table not in migration schema     |
| `--where`        | []string | (none)      | WHERE filter; use `table:condition` for per-table or just `condition` for all                     |
| `--dsn`          | string   | `""`        | Full database DSN (overrides host/port/etc.)                                                     |
| `--host`         | string   | `localhost` | Database host                                                                                    |
| `--port`         | int      | (varies)    | Database port                                                                                    |
| `--database`     | string   | `""`        | Database name                                                                                    |
| `--username`     | string   | `""`        | Database username                                                                                |
| `--password`     | string   | `""`        | Database password                                                                                |
| `--sslmode`      | string   | `disable`   | SSL mode (PostgreSQL)                                                                            |

## How PK detection works

- Primary-key columns are read from the migration **SchemaState** — the
  reconstructed schema built by walking your existing migration chain.
- The `--conflict-key` flag overrides automatic detection and applies the
  specified column(s) to **all** tables in the invocation.
- If the table does not yet exist in the migration schema (i.e. you have not
  generated migrations for it), you **must** supply `--conflict-key` so the
  generated `UpsertData` operation knows which columns form the conflict target.

## Generated output

Running the command produces a standard Go migration file. For example,
dumping a `unit_type` table with two rows generates code similar to:

```go
func init() {
    m.Register(&m.Migration{
        Name: "0003_dump_unit_type",
        Operations: []m.Operation{
            &m.UpsertData{
                Table:       "unit_type",
                ConflictKey: []string{"id"},
                Columns:     []string{"id", "name", "code"},
                Rows: [][]interface{}{
                    {1, "Metric", "MET"},
                    {2, "Imperial", "IMP"},
                },
            },
        },
    })
}
```

Each `UpsertData` operation translates to an `INSERT ... ON CONFLICT (pk) DO UPDATE`
statement at migration runtime.

## Examples

```bash
# Seed a single reference table
makemigrations dump-data unit_type

# Seed multiple tables at once
makemigrations dump-data unit_type currency --name seed_reference_data

# Preview without writing
makemigrations dump-data roles --dry-run

# Override conflict key (table not in schema yet)
makemigrations dump-data legacy_table --conflict-key id

# Specify database connection explicitly
makemigrations dump-data countries --dsn "host=prod-db port=5432 dbname=myapp user=ro sslmode=require"
```

## Filtering Rows with --where

By default, all rows are fetched. Use `--where` to filter:

```bash
# Per-table filter
makemigrations dump-data users orders --where "users:status='active'" --where "orders:total > 0"

# Global filter (applies to all tables)
makemigrations dump-data users orders --where "active = 1"

# Multiple conditions combined with AND
makemigrations dump-data users --where "users:status='active'" --where "users:created_at > '2025-01-01'"
```

## Limitations

- The `--where` condition is appended to the query as-is. Ensure the condition is valid SQL for your target database.
- `--conflict-key` applies to **all** tables in one invocation. If tables have
  different primary keys, run the command separately for each table.
- Values are stored as plain Go literals (strings, ints, `nil`). SQL quoting
  and escaping happen at migration runtime, not at generation time.
