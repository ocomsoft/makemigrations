# init Command

The `init` command initializes a new makemigrations project with YAML-based schema management. This command sets up the necessary directory structure, configuration files, and initial schema templates for a new project.

## Overview

The `init` command creates a complete makemigrations project structure with:
- Migrations directory with configuration
- Initial schema directory and template
- Database-specific configuration
- Schema snapshot tracking
- Example YAML schema structure

## Usage

```bash
makemigrations init [flags]
```

## Command Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--database` | string | `postgresql` | Target database type (postgresql, mysql, sqlite, sqlserver) |
| `--force` | bool | `false` | Overwrite existing files if they exist |
| `--schema-dir` | string | `schema` | Directory name for schema files |
| `--migrations-dir` | string | `migrations` | Directory name for migration files |

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `migrations/makemigrations.config.yaml` | Path to configuration file |

## What Gets Created

### Directory Structure

```
project/
├── migrations/
│   ├── makemigrations.config.yaml    # Main configuration
│   └── .schema_snapshot.yaml         # Schema state tracking (empty initially)
├── schema/
│   └── schema.yaml                   # Main schema definition
└── .gitignore                        # Git ignore patterns (optional)
```

### Configuration File

Creates `migrations/makemigrations.config.yaml` with database-appropriate defaults:

```yaml
# Makemigrations Configuration File
# 
# This file contains configuration for the makemigrations tool.
# All settings can be overridden using environment variables with the prefix MAKEMIGRATIONS_
# For example: MAKEMIGRATIONS_DATABASE_TYPE=mysql

# Database connection and behavior settings
database:
  type: postgresql                    # postgresql, mysql, sqlserver, sqlite
  default_schema: public              # Default schema name for databases that support schemas
  quote_identifiers: true             # Whether to quote table/column names

# Migration generation and execution settings
migration:
  directory: migrations               # Directory for migration files
  file_prefix: "20060102150405"       # Go timestamp format for YYYYMMDDHHMMSS
  snapshot_file: .schema_snapshot.yaml # Name of the schema snapshot file
  auto_apply: false                   # Whether to auto-apply migrations (dangerous!)
  include_down_sql: true              # Whether to generate DOWN migrations
  review_comment_prefix: "-- REVIEW: " # Prefix for review comments on destructive operations
  rejection_comment_prefix: "-- REJECTED: " # Prefix for rejected destructive operations
  silent: false                       # Whether to skip prompts for destructive operations
  destructive_operations:             # List of operation types to mark with review comments
    - table_removed
    - field_removed
    - index_removed
    - table_renamed
    - field_renamed
    - field_modified

# Schema scanning and processing settings
schema:
  search_paths: []                    # Additional paths to search for schema files
  ignore_modules: []                  # Module patterns to ignore
  schema_file_name: schema.yaml       # Name of schema files to look for
  validate_strict: false              # Whether to use strict validation

# Output formatting and display settings
output:
  verbose: false                      # Enable verbose output
  color_enabled: true                 # Enable colored output
  progress_bar: false                 # Show progress bars
  timestamp_format: "2006-01-02 15:04:05" # Format for timestamps in output
```

### Schema Template

Creates `schema/schema.yaml` with a starter template:

```yaml
database:
  name: myapp                         # Change this to your application name
  version: 1.0.0                      # Semantic versioning for your schema

defaults:
  postgresql:
    blank: ''
    now: CURRENT_TIMESTAMP
    new_uuid: gen_random_uuid()
    today: CURRENT_DATE
    zero: "0"
    true: "true"
    false: "false"
    null: "null"

# Define your tables here
tables:
  # Example table - remove or modify as needed
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
      - name: username
        type: varchar
        length: 100
        nullable: false
      - name: created_at
        type: timestamp
        default: now
        auto_create: true
      - name: updated_at
        type: timestamp
        nullable: true
        auto_update: true

  # Add more tables here...
```

## Examples

### Basic Initialization

```bash
# Initialize with PostgreSQL (default)
makemigrations init

# Output
▶ Initializing makemigrations project...
✓ Created directory: migrations/
✓ Created directory: schema/
✓ Generated config: migrations/makemigrations.config.yaml
✓ Generated schema: schema/schema.yaml
✓ Generated snapshot: migrations/.schema_snapshot.yaml
✓ Project initialized successfully!

Next steps:
1. Edit schema/schema.yaml to define your database schema
2. Run 'makemigrations makemigrations' to generate your first migration
3. Run 'makemigrations goose up' to apply migrations to your database
```

### Database-Specific Initialization

```bash
# Initialize for MySQL
makemigrations init --database mysql

# Initialize for SQLite  
makemigrations init --database sqlite

# Initialize for SQL Server
makemigrations init --database sqlserver
```

### Custom Directory Structure

```bash
# Use custom directory names
makemigrations init --schema-dir database --migrations-dir db/migrations

# Creates:
# db/migrations/makemigrations.config.yaml
# database/schema.yaml
```

### Force Overwrite

```bash
# Overwrite existing files
makemigrations init --force

# Useful for:
# - Resetting configuration to defaults
# - Updating to new template formats
# - Starting fresh after corruption
```

## Database-Specific Templates

### PostgreSQL (Default)

```yaml
defaults:
  postgresql:
    blank: ''
    now: CURRENT_TIMESTAMP
    new_uuid: gen_random_uuid()
    today: CURRENT_DATE
    zero: "0"
    true: "true"
    false: "false"
    null: "null"
```

### MySQL

```yaml
defaults:
  mysql:
    blank: ''
    now: CURRENT_TIMESTAMP
    new_uuid: (UUID())
    today: (CURDATE())
    zero: "0"
    true: "1"
    false: "0"
    null: "null"
```

### SQLite

```yaml
defaults:
  sqlite:
    blank: ''
    now: CURRENT_TIMESTAMP
    new_uuid: ''
    today: CURRENT_DATE
    zero: "0"
    true: "1"
    false: "0"
    null: "null"
```

### SQL Server

```yaml
defaults:
  sqlserver:
    blank: ''
    now: GETDATE()
    new_uuid: NEWID()
    today: CAST(GETDATE() AS DATE)
    zero: "0"
    true: "1"
    false: "0"
    null: "null"
```

## Post-Initialization Workflow

### 1. Configure Database Connection

Set environment variables for your database:

```bash
# PostgreSQL
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=5432
export MAKEMIGRATIONS_DB_USER=postgres
export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
export MAKEMIGRATIONS_DB_NAME=yourdb

# MySQL
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=3306
export MAKEMIGRATIONS_DB_USER=root
export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
export MAKEMIGRATIONS_DB_NAME=yourdb

# SQLite
export MAKEMIGRATIONS_DB_PATH=./database.db
```

### 2. Define Your Schema

Edit `schema/schema.yaml` to define your application's database schema:

```yaml
database:
  name: ecommerce_app
  version: 1.0.0

tables:
  - name: products
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid
      - name: name
        type: varchar
        length: 255
        nullable: false
      - name: price
        type: decimal
        precision: 10
        scale: 2
        nullable: false
      - name: created_at
        type: timestamp
        default: now
        auto_create: true

  - name: orders
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid
      - name: product_id
        type: foreign_key
        nullable: false
        foreign_key:
          table: products
          on_delete: CASCADE
      - name: quantity
        type: integer
        default: zero
```

### 3. Generate First Migration

```bash
# Generate migration from schema
makemigrations makemigrations --name "initial_schema"
```

### 4. Apply Migration

```bash
# Apply to database
makemigrations goose up
```

## Configuration Customization

After initialization, you can customize the generated configuration:

### Migration Settings

```yaml
migration:
  review_comment_prefix: "-- MANUAL-REVIEW: "
  silent: true                        # Skip prompts in CI/CD
  include_down_sql: false            # Skip rollback generation
```

### Schema Processing

```yaml
schema:
  search_paths:
    - "modules/*/schema"              # Search in module directories
    - "vendors/*/database"            # Search vendor schemas
  ignore_modules:
    - "test/*"                        # Ignore test schemas
    - "*/internal/*"                  # Ignore internal schemas
```

### Output Formatting

```yaml
output:
  verbose: true                       # Detailed output
  color_enabled: false               # Disable colors for CI/CD
  timestamp_format: "15:04:05"       # Time-only format
```

## Version Control Integration

### Git Integration

The init command can optionally create a `.gitignore` file:

```gitignore
# Database files
*.db
*.sqlite
*.sqlite3

# Environment files
.env
.env.local

# OS generated files
.DS_Store
Thumbs.db

# IDE files
.vscode/
.idea/
*.swp
*.swo

# Temporary files
*.tmp
*.temp
```

### Recommended Git Workflow

```bash
# After initialization
git add migrations/makemigrations.config.yaml
git add schema/schema.yaml
git add .gitignore

# Don't commit the empty snapshot initially
echo "migrations/.schema_snapshot.yaml" >> .gitignore

# After first migration generation
git add migrations/
git rm .gitignore
# Edit .gitignore to remove the snapshot line
git add .gitignore
```

## Error Handling

### Directory Already Exists

```bash
$ makemigrations init
Error: migrations directory already exists

# Solutions:
makemigrations init --force           # Overwrite existing
rm -rf migrations/ schema/            # Remove and retry
cd different/directory && makemigrations init
```

### Permission Issues

```bash
$ makemigrations init
Error: permission denied creating directory

# Solutions:
sudo makemigrations init              # Run with elevated permissions
mkdir migrations && chmod 755 migrations
```

### Invalid Database Type

```bash
$ makemigrations init --database oracle
Error: unsupported database type: oracle

# Supported types:
makemigrations init --database postgresql
makemigrations init --database mysql
makemigrations init --database sqlite
makemigrations init --database sqlserver
```

## Best Practices

### 1. Project Structure

```bash
# Recommended project layout
myapp/
├── cmd/                             # Application entry points
├── internal/                        # Private application code
├── migrations/                      # Generated migrations
│   ├── makemigrations.config.yaml  
│   └── 20240101120000_initial.sql
├── schema/                          # YAML schema definitions
│   ├── schema.yaml                  # Main schema
│   ├── core/                        # Core tables
│   └── modules/                     # Feature-specific tables
├── pkg/                             # Public packages
└── go.mod
```

### 2. Schema Organization

```bash
# For large applications, split schemas
schema/
├── schema.yaml                      # Database metadata
├── core/
│   ├── users.yaml                   # User management
│   └── auth.yaml                    # Authentication
├── ecommerce/
│   ├── products.yaml                # Product catalog
│   ├── orders.yaml                  # Order management
│   └── payments.yaml                # Payment processing
└── reporting/
    └── analytics.yaml               # Analytics tables
```

### 3. Environment Setup

```bash
# Use environment-specific configs
cp migrations/makemigrations.config.yaml migrations/makemigrations.dev.yaml
cp migrations/makemigrations.config.yaml migrations/makemigrations.prod.yaml

# Use with --config flag
makemigrations --config migrations/makemigrations.dev.yaml makemigrations
```

## Integration Examples

### Docker Setup

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o makemigrations .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/makemigrations .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/schema ./schema
CMD ["./makemigrations", "goose", "up"]
```

### Docker Compose

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_DB: myapp
    
  migrate:
    build: .
    depends_on:
      - postgres
    environment:
      MAKEMIGRATIONS_DB_HOST: postgres
      MAKEMIGRATIONS_DB_PASSWORD: password
      MAKEMIGRATIONS_DB_NAME: myapp
    command: ["./makemigrations", "goose", "up"]
```

### CI/CD Integration

```yaml
# .github/workflows/database.yml
name: Database Migration
on: [push, pull_request]

jobs:
  migrate:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: password
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Setup Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.21
    
    - name: Build makemigrations
      run: go build -o makemigrations .
    
    - name: Run migrations
      env:
        MAKEMIGRATIONS_DB_HOST: localhost
        MAKEMIGRATIONS_DB_PASSWORD: password
        MAKEMIGRATIONS_DB_NAME: postgres
      run: |
        ./makemigrations goose up
        ./makemigrations makemigrations --check
```

## See Also

- [makemigrations Command](./makemigrations.md) - Generate migrations from YAML schemas
- [Configuration Guide](../configuration.md) - Detailed configuration options
- [Schema Format Guide](../schema-format.md) - YAML schema syntax reference
- [Installation Guide](../installation.md) - Setup and installation