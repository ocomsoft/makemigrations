# Manual Migration Binary Build Guide

This guide explains how to build and run the compiled migrations binary by hand — useful for CI/CD pipelines, Docker builds, debugging, or any situation where you can't use `makemigrations migrate`.

> **For day-to-day use** prefer `makemigrations migrate <command>` which handles all of this automatically.

---

## Why the standard `go build` can fail

The `migrations/` directory is a separate Go module with its own `go.mod`. Several environment concerns can cause a plain `go build` to fail:

| Problem | Symptom |
|---------|---------|
| Parent `go.work` doesn't list `migrations/` | `main module does not contain package …/migrations` |
| `go.work` has a partial version like `go 1.25` | `go: downloading go1.25 (linux/amd64): toolchain not available` |
| `go.sum` is stale after a `makemigrations` upgrade | `missing go.sum entry for module providing package …` |
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

If you have just upgraded `makemigrations` or pulled a new version, the `migrations/go.sum` may be stale. Run `go mod download` before building:

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

### 4. Using a local development copy of `makemigrations`

If your parent `go.mod` has a `replace` directive pointing to a local copy of `makemigrations`, you can expose it to the `migrations/` build via a temporary `go.work`:

```bash
# Create a temporary workspace that includes both modules
cat > /tmp/migrations.work <<EOF
go 1.25.7

use /absolute/path/to/your-project/migrations
use /absolute/path/to/makemigrations
EOF

GOWORK=/tmp/migrations.work go build -o migrations/migrate ./migrations/
```

> `makemigrations migrate` does all of this automatically by detecting the local replace directive in parent `go.mod` files.

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

## Running the binary

The binary reads database connection details from environment variables set in `migrations/main.go`. A typical invocation:

```bash
export DATABASE_URL="postgresql://user:pass@localhost/mydb"
export DB_TYPE="postgresql"

./migrations/migrate up
./migrations/migrate status
./migrations/migrate fake 0001_initial
./migrations/migrate down --steps 1
```

Or using individual fields instead of `DATABASE_URL`:

```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=secret
export DB_NAME=mydb
export DB_SSLMODE=disable

./migrations/migrate up
```

---

## CI/CD example (GitHub Actions)

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

Or, if `makemigrations` is installed as a tool:

```yaml
- name: Apply database migrations
  env:
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
  run: makemigrations migrate up
```

---

## See Also

- [migrate command](./commands/migrate.md) — full reference for all binary subcommands
- [init command](./commands/init.md) — bootstrap the `migrations/` directory
- [makemigrations command](./commands/makemigrations.md) — generate `.go` migration files
