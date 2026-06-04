# Manual Migration Binary Build Guide (Optional)

> **You almost certainly do not need this.** The default workflow is
> `morphic migrate <command>`, which loads the migration `.go` files
> in-process via the [yaegi](https://github.com/traefik/yaegi) Go interpreter
> and runs them without ever invoking `go build`. There is no compile or
> rebuild step in the day-to-day developer loop.

This guide covers the **optional** standalone-binary path: compiling the
`migrations/` directory into a self-contained binary using `go build`. Reasons
you might want this:

- **Shipping a release artifact** that runs migrations on a host where you don't
  want to install the morphic CLI.
- **Avoiding yaegi entirely** — for example to debug a yaegi-specific
  interpretation issue, or because your migrations import a third-party
  package that you don't want to register in the symbol map.
- **A locked-down CI environment** that already has Go but not morphic.

The migration `.go` files are valid Go source either way: yaegi-interpreted
and `gc`-compiled paths produce the same on-disk schema state.

---

## Why the standard `go build` can fail

The `migrations/` directory is a separate Go module with its own `go.mod`. Several environment concerns can cause a plain `go build` to fail:

| Problem | Symptom |
|---------|---------|
| Parent `go.work` doesn't list `migrations/` | `main module does not contain package …/migrations` |
| `go.work` has a partial version like `go 1.25` | `go: downloading go1.25 (linux/amd64): toolchain not available` |
| `go.sum` is stale after a `morphic` upgrade | `missing go.sum entry for module providing package …` |
| `GOTOOLCHAIN=local` with an older system Go | `go.mod requires go >= 1.24 (running go 1.22.2; GOTOOLCHAIN=local)` |

---

## Correct build procedure

### 1. Disable the parent workspace

When a `go.work` file exists in a parent directory it overrides module resolution for the whole workspace, but `migrations/` is typically not listed in it. Disable it for the build:

```bash
GOWORK=off go build -o migrations/migrate ./migrations/
```

Or `cd` into the directory first:

```bash
cd migrations
GOWORK=off go build -o migrate .
```

### 2. Sync `go.sum` if needed

If you have just upgraded `morphic` or pulled a new version, the `migrations/go.sum` may be stale. Run `go mod download` before building:

```bash
cd migrations
GOWORK=off go mod download
GOWORK=off go build -o migrate .
```

### 3. Match the parent project's Go version

If your parent `go.work` or `go.mod` requires a Go version that isn't installed locally (e.g. `go 1.25`), the Go toolchain may try to download it. Pin the toolchain to the installed binary using `GOTOOLCHAIN`:

```bash
cd migrations

# Use the exact version that is already installed (e.g. go1.25.7)
GOWORK=off GOTOOLCHAIN=$(go env GOVERSION) go build -o migrate .
```

`go env GOVERSION` returns the full patch version (e.g. `go1.25.7`), which Go recognises as a locally-available toolchain and does not attempt to download.

### 4. Using a local development copy of `morphic`

If your parent `go.mod` has a `replace` directive pointing to a local copy of `morphic`, you can expose it to the `migrations/` build via a temporary `go.work`:

```bash
# Create a temporary workspace that includes both modules
cat > /tmp/migrations.work <<EOF
go 1.25.7

use /absolute/path/to/your-project/migrations
use /absolute/path/to/morphic
EOF

GOWORK=/tmp/migrations.work go build -o migrations/migrate ./migrations/
```

> Note: `morphic migrate` (the default, yaegi path) does **not** need any of this — it neither compiles the migrations directory nor reads `go.mod` at runtime. The setup above only matters if you specifically want a standalone binary.

---

## Complete one-liner examples

```bash
# Simple: no workspace, no toolchain issues
cd migrations && GOWORK=off go build -o migrate .

# With go.sum sync
cd migrations && GOWORK=off go mod download && GOWORK=off go build -o migrate .

# Pin toolchain to installed version
cd migrations && GOWORK=off GOTOOLCHAIN=$(go env GOVERSION) go build -o migrate .

# From project root (adjust path as needed)
GOWORK=off go build -o migrations/migrate ./migrations/
```

---

## Running the standalone binary

Once compiled, the standalone binary reads database connection details from environment variables wired up in `migrations/main.go`. The **generated** `main.go` only reads `DB_TYPE` and `DATABASE_URL`:

```bash
export DATABASE_URL="postgresql://user:pass@localhost/mydb"
export DB_TYPE="postgresql"   # optional, defaults to "postgresql"

./migrations/migrate up
./migrations/migrate status
./migrations/migrate fake 0001_initial
./migrations/migrate down --steps 1
```

If you need to support individual `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` fields, edit `migrations/main.go` to populate them:

```go
app := m.NewApp(m.Config{
    DatabaseType: m.EnvOr("DB_TYPE", "postgresql"),
    DatabaseURL:  m.EnvOr("DATABASE_URL", ""),
    DBHost:       m.EnvOr("DB_HOST", "localhost"),
    DBPort:       m.EnvOr("DB_PORT", "5432"),
    DBUser:       m.EnvOr("DB_USER", "postgres"),
    DBPassword:   os.Getenv("DB_PASSWORD"),
    DBName:       m.EnvOr("DB_NAME", "mydb"),
    DBSSLMode:    m.EnvOr("DB_SSLMODE", "disable"),
})
```

`DATABASE_URL` takes priority over the individual fields when both are set.

---

## CI/CD example (GitHub Actions)

The recommended approach — install morphic and use yaegi:

```yaml
- name: Apply database migrations
  env:
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
  run: |
    go install github.com/ocomsoft/morphic@latest
    morphic migrate up
```

Or, if you specifically want the standalone-binary path:

```yaml
- name: Apply database migrations
  env:
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
  run: |
    cd migrations
    GOWORK=off go mod download
    GOWORK=off go build -o migrate .
    ./migrate up
```

---

## See Also

- [migrate command](./commands/migrate.md) — full reference for all binary subcommands
- [init command](./commands/init.md) — bootstrap the `migrations/` directory
- [morphic command](./commands/morphic.md) — generate `.go` migration files
