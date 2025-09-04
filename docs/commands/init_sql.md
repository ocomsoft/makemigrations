# init_sql Command

The `init_sql` command initializes a new makemigrations project with **SQL-based** migration management. This is an alternative to the YAML-based approach, providing traditional SQL migration workflows similar to other migration tools.

## Overview

The `init_sql` command creates a project structure optimized for:
- Direct SQL migration files
- Traditional migration workflows
- Manual migration creation
- Database-specific SQL development
- Integration with existing SQL-based projects

**Note:** This command is for SQL-first workflows. For YAML-based schema management (the primary use case), use the [`init`](./init.md) command instead.

## Usage

```bash
makemigrations init_sql [flags]
```

## Command Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--database` | string | `postgresql` | Target database type (postgresql, mysql, sqlite, sqlserver) |
| `--force` | bool | `false` | Overwrite existing files if they exist |
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
│   ├── makemigrations.config.yaml    # Configuration for SQL-based workflows
│   └── README.md                     # SQL migration usage guide
└── .gitignore                        # Git ignore patterns (optional)
```

### Configuration File

Creates `migrations/makemigrations.config.yaml` optimized for SQL workflows:

```yaml
# Makemigrations Configuration File - SQL Mode
# 
# This configuration is optimized for SQL-based migration workflows.
# For YAML-based schema management, use 'makemigrations init' instead.

# Database connection and behavior settings
database:
  type: postgresql                    # postgresql, mysql, sqlserver, sqlite
  default_schema: public              # Default schema name for databases that support schemas
  quote_identifiers: true             # Whether to quote table/column names

# Migration generation and execution settings
migration:
  directory: migrations               # Directory for migration files
  file_prefix: "20060102150405"       # Go timestamp format for YYYYMMDDHHMMSS
  auto_apply: false                   # Whether to auto-apply migrations (dangerous!)
  include_down_sql: true              # Whether to generate DOWN migrations
  review_comment_prefix: "-- REVIEW: " # Prefix for review comments on destructive operations
  rejection_comment_prefix: "-- REJECTED: " # Prefix for rejected destructive operations
  silent: false                       # Whether to skip prompts for destructive operations

# Output formatting and display settings
output:
  verbose: false                      # Enable verbose output
  color_enabled: true                 # Enable colored output
  progress_bar: false                 # Show progress bars
  timestamp_format: "2006-01-02 15:04:05" # Format for timestamps in output

# SQL-specific settings (YAML schema features disabled)
schema:
  search_paths: []                    # Not used in SQL mode
  schema_file_name: ""                # Disabled for SQL mode
  validate_strict: false              # Not applicable to SQL mode
```

### Migration Guide

Creates `migrations/README.md` with SQL migration usage instructions:

```markdown
# SQL-Based Migrations

This project uses SQL-based database migrations. Use the commands below to manage your database schema.

## Creating Migrations

Create new migration files manually or using goose:

```bash
# Create a new migration
makemigrations goose create add_users_table

# Edit the generated file
vim migrations/20240122134500_add_users_table.sql
```

## Migration File Format

Use the standard Goose format:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
```

## Applying Migrations

```bash
# Check migration status
makemigrations goose status

# Apply all pending migrations
makemigrations goose up

# Apply one migration at a time
makemigrations goose up-by-one

# Rollback last migration
makemigrations goose down
```

## Database Connection

Set environment variables for database connection:

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

## See Also

- [Goose Commands](../docs/commands/goose.md) - Migration execution
- [Configuration](../docs/configuration.md) - Configuration options

For YAML-based schema management, consider using `makemigrations init` instead.
```

## Examples

### Basic SQL Initialization

```bash
# Initialize SQL-based project
makemigrations init_sql

# Output
▶ Initializing SQL-based makemigrations project...
✓ Created directory: migrations/
✓ Generated config: migrations/makemigrations.config.yaml
✓ Generated guide: migrations/README.md
✓ SQL project initialized successfully!

Next steps:
1. Set up database connection environment variables
2. Create your first migration: makemigrations goose create initial_schema
3. Edit the migration file with your SQL
4. Apply migrations: makemigrations goose up
```

### Database-Specific Initialization

```bash
# Initialize for MySQL
makemigrations init_sql --database mysql

# Initialize for SQLite
makemigrations init_sql --database sqlite

# Initialize for SQL Server
makemigrations init_sql --database sqlserver
```

### Custom Migration Directory

```bash
# Use custom migration directory
makemigrations init_sql --migrations-dir db/migrations

# Creates:
# db/migrations/makemigrations.config.yaml
# db/migrations/README.md
```

### Force Overwrite

```bash
# Overwrite existing SQL project
makemigrations init_sql --force

# Useful for:
# - Converting YAML project to SQL
# - Resetting to SQL-only configuration
# - Updating SQL templates
```

## Post-Initialization Workflow

### 1. Configure Database Connection

Set environment variables for your target database:

```bash
# PostgreSQL
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=5432
export MAKEMIGRATIONS_DB_USER=postgres
export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
export MAKEMIGRATIONS_DB_NAME=yourdb
export MAKEMIGRATIONS_DB_SSLMODE=disable

# MySQL
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=3306
export MAKEMIGRATIONS_DB_USER=root
export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
export MAKEMIGRATIONS_DB_NAME=yourdb

# SQLite
export MAKEMIGRATIONS_DB_PATH=./database.db

# SQL Server
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=1433
export MAKEMIGRATIONS_DB_USER=sa
export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
export MAKEMIGRATIONS_DB_NAME=yourdb
```

### 2. Create First Migration

```bash
# Create initial schema migration
makemigrations goose create initial_schema

# Edit the generated file
vim migrations/20240122134500_initial_schema.sql
```

Example migration content:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
```

### 3. Apply Migration

```bash
# Check status before applying
makemigrations goose status

# Apply the migration
makemigrations goose up

# Verify application
makemigrations goose version
```

## SQL Migration Best Practices

### 1. Migration File Structure

```sql
-- +goose Up
-- Migration description and purpose
-- Author: Your Name
-- Date: 2024-01-22

-- +goose StatementBegin
-- Forward migration SQL
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- Rollback SQL (should undo everything above)
-- +goose StatementBegin
DROP TABLE products;
-- +goose StatementEnd
```

### 2. Database-Specific SQL

#### PostgreSQL
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metadata JSONB DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_metadata ON users USING GIN(metadata);
-- +goose StatementEnd
```

#### MySQL
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    metadata JSON DEFAULT ('{}'),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_metadata ON users((CAST(metadata AS CHAR(255))));
-- +goose StatementEnd
```

#### SQLite
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    metadata TEXT DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_metadata ON users(metadata);
-- +goose StatementEnd
```

### 3. Complex Migrations

```sql
-- +goose Up
-- +goose StatementBegin

-- Step 1: Create new table
CREATE TABLE user_profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Add foreign key constraint
ALTER TABLE user_profiles 
ADD CONSTRAINT fk_user_profiles_user_id 
FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Step 3: Create indexes
CREATE INDEX idx_user_profiles_user_id ON user_profiles(user_id);
CREATE INDEX idx_user_profiles_name ON user_profiles(first_name, last_name);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_profiles;
-- +goose StatementEnd
```

## Differences from YAML Mode

| Feature | YAML Mode (`init`) | SQL Mode (`init_sql`) |
|---------|-------------------|----------------------|
| Schema Definition | YAML files | Direct SQL |
| Migration Generation | Automatic | Manual |
| Cross-Database | Automatic | Manual per database |
| Schema Validation | Built-in | Manual |
| Relationships | Declarative | Manual SQL |
| Defaults | Database-agnostic | Database-specific |
| Change Detection | Automatic | Manual |

### When to Use SQL Mode

**Use `init_sql` when:**
- Migrating from other SQL migration tools
- Need database-specific SQL features
- Team prefers direct SQL control
- Complex migrations requiring fine-grained SQL
- Existing SQL-based infrastructure

**Use `init` (YAML mode) when:**
- Starting new projects
- Want database-agnostic schemas
- Prefer declarative schema definitions
- Need automatic migration generation
- Want built-in validation and best practices

## Converting Between Modes

### From YAML to SQL

```bash
# Generate SQL from existing YAML schema
makemigrations dump_sql > schema.sql

# Initialize SQL mode
makemigrations init_sql --force

# Create initial migration from dumped SQL
makemigrations goose create from_yaml_schema
# Copy content from schema.sql to the migration file
```

### From SQL to YAML

```bash
# Manual conversion required
# 1. Analyze existing SQL migrations
# 2. Create equivalent YAML schema
# 3. Initialize YAML mode
makemigrations init --force
# 4. Generate migration to match current state
```

## Error Handling

### Common Issues

1. **Directory already exists**
   ```bash
   $ makemigrations init_sql
   Error: migrations directory already exists
   
   # Solutions:
   makemigrations init_sql --force
   rm -rf migrations/
   ```

2. **Permission denied**
   ```bash
   $ makemigrations init_sql
   Error: permission denied creating directory
   
   # Solutions:
   sudo makemigrations init_sql
   mkdir migrations && chmod 755 migrations
   ```

3. **Conflicting with YAML mode**
   ```bash
   $ makemigrations init_sql
   Warning: Existing YAML schema files detected
   This will create SQL-only configuration
   Continue? [y/N]
   ```

## Integration Examples

### Docker Setup

```dockerfile
# Dockerfile for SQL-based migrations
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o makemigrations .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/makemigrations .
COPY --from=builder /app/migrations ./migrations
CMD ["./makemigrations", "goose", "up"]
```

### CI/CD Pipeline

```yaml
# .github/workflows/sql-migrations.yml
name: SQL Migrations
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
    
    - name: Run SQL migrations
      env:
        MAKEMIGRATIONS_DB_HOST: localhost
        MAKEMIGRATIONS_DB_PASSWORD: password
        MAKEMIGRATIONS_DB_NAME: postgres
      run: |
        ./makemigrations goose up
        ./makemigrations goose status
```

### Development Workflow

```bash
#!/bin/bash
# sql-migrate.sh

set -e

echo "Creating new migration..."
read -p "Migration name: " name
makemigrations goose create "$name"

echo "Edit the migration file:"
latest=$(ls migrations/*.sql | tail -1)
${EDITOR:-vim} "$latest"

echo "Review migration:"
cat "$latest"

read -p "Apply migration? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    makemigrations goose up-by-one
    echo "Migration applied successfully!"
    makemigrations goose status
fi
```

## See Also

- [Goose Integration](./goose.md) - Migration execution commands
- [makemigrations_sql Command](./makemigrations_sql.md) - SQL migration generation
- [init Command](./init.md) - YAML-based initialization (recommended)
- [Configuration Guide](../configuration.md) - Configuration options