# Installation Guide

This guide covers installing and setting up makemigrations for Go-based database migration management.

## Prerequisites

- **Go 1.24 or later** — Required for building and running makemigrations
- **Git** — For cloning the repository
- **CGO_ENABLED=1** — Required only when using SQLite; PostgreSQL and MySQL do not need CGO
- **Database** — One of the supported databases: PostgreSQL, MySQL, or SQLite

## Installation Methods

### Option 1: Go Install (Recommended)

```bash
go install github.com/ocomsoft/makemigrations@latest
```

This places the `makemigrations` binary in `$(go env GOPATH)/bin`. Ensure that directory is on your `PATH`.

### Option 2: Build from Source

```bash
git clone https://github.com/ocomsoft/makemigrations.git
cd makemigrations
go build -o makemigrations .

# Optionally install globally
sudo cp makemigrations /usr/local/bin/
```

### Option 3: Download Pre-Built Binary

Download pre-built binaries from the [releases page](https://github.com/ocomsoft/makemigrations/releases):

```bash
# Linux/macOS
curl -L https://github.com/ocomsoft/makemigrations/releases/latest/download/makemigrations-linux-amd64 -o makemigrations
chmod +x makemigrations

# Windows
curl -L https://github.com/ocomsoft/makemigrations/releases/latest/download/makemigrations-windows-amd64.exe -o makemigrations.exe
```

## Verification

Confirm the installation is working:

```bash
makemigrations --help
makemigrations init --help
```

## Quickstart: Go Migration Workflow

This is the primary workflow. Migrations are generated as Go source files, compiled into a standalone binary, and applied directly against your database.

### 1. Install

```bash
go install github.com/ocomsoft/makemigrations@latest
```

### 2. Initialise Your Project

Run `init` from the root of your Go project. This creates a `migrations/` subdirectory with its own Go module and entry point:

```bash
makemigrations init
```

### 3. Define Your Schema

Edit `schema/schema.yaml` to describe your database tables and fields:

```yaml
database:
  name: myapp
  version: 1.0.0

tables:
  - name: users
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid
      - name: email
        type: varchar
        length: 255
        nullable: false
      - name: created_at
        type: timestamp
        default: now
        auto_create: true
```

### 4. Generate Your First Migration

```bash
makemigrations makemigrations --name "initial"
# Creates migrations/0001_initial.go
```

### 5. Build the Migrations Binary

```bash
cd migrations && go mod tidy && go build -o migrate .
```

### 6. Apply Migrations

```bash
./migrations/migrate up
```

### 7. Ongoing Workflow

After each change to `schema.yaml`, regenerate, rebuild, and apply:

```bash
makemigrations makemigrations
cd migrations && go build -o migrate .
./migrations/migrate up
```

### Project Layout After Init

```
myapp/
├── go.mod
├── schema/
│   └── schema.yaml          # YAML schema definitions
└── migrations/
    ├── go.mod               # separate module for the migrations binary
    ├── main.go              # migrations binary entry point
    └── 0001_initial.go      # generated Go migration (after first makemigrations)
```

## Database Configuration

The `./migrations/migrate` binary reads connection settings from environment variables.

### PostgreSQL

```bash
export MAKEMIGRATIONS_DB_TYPE=postgresql
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=5432
export MAKEMIGRATIONS_DB_USER=postgres
export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
export MAKEMIGRATIONS_DB_NAME=yourdb
export MAKEMIGRATIONS_DB_SSLMODE=disable
```

### MySQL

```bash
export MAKEMIGRATIONS_DB_TYPE=mysql
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=3306
export MAKEMIGRATIONS_DB_USER=root
export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
export MAKEMIGRATIONS_DB_NAME=yourdb
```

### SQLite

SQLite requires CGO. Build with `CGO_ENABLED=1`:

```bash
export MAKEMIGRATIONS_DB_TYPE=sqlite
export MAKEMIGRATIONS_DATABASE_URL=./database.db
```

### Development .env File

Create a `.env` file for local development:

```bash
MAKEMIGRATIONS_DB_TYPE=postgresql
MAKEMIGRATIONS_DB_HOST=localhost
MAKEMIGRATIONS_DB_PORT=5432
MAKEMIGRATIONS_DB_USER=dev_user
MAKEMIGRATIONS_DB_PASSWORD=dev_password
MAKEMIGRATIONS_DB_NAME=myapp_development
```

### Production Environment

```bash
MAKEMIGRATIONS_DB_TYPE=postgresql
MAKEMIGRATIONS_DB_HOST=production-db-host
MAKEMIGRATIONS_DB_USER=app_user
MAKEMIGRATIONS_DB_PASSWORD=secure_password
MAKEMIGRATIONS_DB_NAME=myapp_production
```

## Troubleshooting

### "Command not found"

```bash
# Ensure GOPATH/bin is on your PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Or use the full path
./makemigrations --help
```

### "No schema files found"

```bash
# Check that schema.yaml exists in the expected location
find . -name "schema.yaml" -type f

# Verify directory structure
ls -la schema/
```

### Database Connection Issues

```bash
# Check that environment variables are set
echo $MAKEMIGRATIONS_DB_HOST
echo $MAKEMIGRATIONS_DB_USER

# Test a direct connection
psql -h $MAKEMIGRATIONS_DB_HOST -U $MAKEMIGRATIONS_DB_USER -d $MAKEMIGRATIONS_DB_NAME
```

### YAML Parsing Errors

```bash
# Validate YAML syntax
yamllint schema/schema.yaml
```

### SQLite: CGO Errors

SQLite requires CGO. Rebuild the migrations binary with CGO enabled:

```bash
CGO_ENABLED=1 go build -o migrate .
```

## Legacy SQL Workflow

For projects using the older YAML-to-SQL+Goose workflow, initialise with:

```bash
makemigrations init --sql
```

This generates raw `.sql` migration files managed by Goose instead of Go source files. See [docs/commands/init.md](commands/init.md) for full details.

## Next Steps

1. **Read the [Configuration Guide](configuration.md)** to customise settings
2. **Review [Schema Format Documentation](schema-format.md)** for YAML schema syntax
3. **Explore [Command Documentation](commands/)** for detailed usage
4. **Set up CI/CD integration** using `--check` and `--silent` flags

## Support

- **Documentation**: Browse the `/docs` directory for detailed guides
- **Examples**: Check the `/example` directory for sample schemas
- **Issues**: Report problems on GitHub Issues
- **Discussions**: Join GitHub Discussions for questions

For detailed command usage, see the [Commands Documentation](commands/).
