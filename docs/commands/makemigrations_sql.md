# makemigrations_sql Command

The `makemigrations_sql` command generates database migrations from **direct SQL input** rather than YAML schema files. This command provides a bridge between manual SQL development and automated migration generation.

## Overview

The `makemigrations_sql` command allows you to:
- Generate Goose-format migration files from raw SQL
- Create structured migrations from ad-hoc SQL commands
- Convert existing SQL scripts to migration format
- Maintain migration metadata while working with direct SQL
- Bridge between SQL-first and structured migration workflows

**Note:** This is for SQL-based workflows. For YAML-based schema management (the primary use case), use the [`makemigrations`](./makemigrations.md) command instead.

## Usage

```bash
makemigrations makemigrations_sql [flags]
```

## Command Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--sql` | string | "" | Direct SQL statement to convert to migration |
| `--sql-file` | string | "" | Path to SQL file to convert to migration |
| `--name` | string | auto-generated | Migration name (overrides auto-generation) |
| `--up-only` | bool | `false` | Generate only UP migration (no DOWN) |
| `--down-sql` | string | "" | Custom DOWN SQL (auto-generated if not provided) |
| `--dry-run` | bool | `false` | Show what would be generated without creating files |
| `--force` | bool | `false` | Overwrite existing migration files |

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `migrations/makemigrations.config.yaml` | Path to configuration file |

## How It Works

### 1. SQL Processing
- Accepts SQL from command line (`--sql`) or file (`--sql-file`)
- Parses SQL statements to understand structure
- Identifies table operations, constraints, and indexes
- Generates appropriate migration metadata

### 2. DOWN Migration Generation
- Attempts to auto-generate rollback SQL
- For CREATE TABLE: generates DROP TABLE
- For ALTER TABLE: generates reverse operations
- For complex operations: requires manual DOWN SQL

### 3. Migration File Creation
- Creates timestamped migration files
- Uses standard Goose format
- Includes proper statement blocks
- Adds metadata comments

## Examples

### Direct SQL Input

```bash
# Simple table creation
makemigrations makemigrations_sql --sql "CREATE TABLE users (id SERIAL PRIMARY KEY, email VARCHAR(255) NOT NULL);" --name "create_users_table"

# Output
▶ Processing SQL input...
✓ Parsed 1 SQL statement
▶ Generating migration: 20240122134500_create_users_table
✓ Migration generated successfully

# Generated file: migrations/20240122134500_create_users_table.sql
```

### SQL File Input

```bash
# Create SQL file
cat > schema.sql << EOF
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_products_name ON products(name);
EOF

# Generate migration from file
makemigrations makemigrations_sql --sql-file schema.sql --name "create_products"

# Output
▶ Reading SQL file: schema.sql
▶ Processing SQL input...
✓ Parsed 2 SQL statements
▶ Generating migration: 20240122134500_create_products
✓ Migration generated successfully
```

### Custom DOWN Migration

```bash
# Provide custom rollback SQL
makemigrations makemigrations_sql \
  --sql "ALTER TABLE users ADD COLUMN middle_name VARCHAR(100);" \
  --down-sql "ALTER TABLE users DROP COLUMN middle_name;" \
  --name "add_user_middle_name"
```

### UP-Only Migration

```bash
# Generate migration without DOWN SQL
makemigrations makemigrations_sql \
  --sql "INSERT INTO settings (key, value) VALUES ('app_version', '1.0.0');" \
  --up-only \
  --name "add_app_version_setting"
```

### Dry Run Mode

```bash
# Preview migration generation
makemigrations makemigrations_sql \
  --sql "CREATE TABLE orders (id SERIAL PRIMARY KEY, user_id INTEGER NOT NULL);" \
  --dry-run

# Output
▶ DRY RUN MODE - No files will be created
▶ Processing SQL input...
✓ Parsed 1 SQL statement
▶ Would generate migration: 20240122134500_create_orders

--- Generated Migration Preview ---
-- +goose Up
-- +goose StatementBegin
CREATE TABLE orders (
    id SERIAL PRIMARY KEY, 
    user_id INTEGER NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE orders;
-- +goose StatementEnd
```

## Generated Migration Format

### Basic Table Creation

**Input SQL:**
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Generated Migration:**
```sql
-- +goose Up
-- Generated from SQL input on 2024-01-22 13:45:00
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- Auto-generated rollback
-- +goose StatementBegin
DROP TABLE users;
-- +goose StatementEnd
```

### Complex Multi-Statement Migration

**Input SQL:**
```sql
-- Create products table
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category_id INTEGER
);

-- Add foreign key constraint
ALTER TABLE products 
ADD CONSTRAINT fk_products_category 
FOREIGN KEY (category_id) REFERENCES categories(id);

-- Create index
CREATE INDEX idx_products_category ON products(category_id);
```

**Generated Migration:**
```sql
-- +goose Up
-- Generated from SQL input on 2024-01-22 13:45:00
-- +goose StatementBegin

-- Create products table
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category_id INTEGER
);

-- Add foreign key constraint
ALTER TABLE products 
ADD CONSTRAINT fk_products_category 
FOREIGN KEY (category_id) REFERENCES categories(id);

-- Create index
CREATE INDEX idx_products_category ON products(category_id);

-- +goose StatementEnd

-- +goose Down
-- Auto-generated rollback (reverse order)
-- +goose StatementBegin
DROP INDEX idx_products_category;
ALTER TABLE products DROP CONSTRAINT fk_products_category;
DROP TABLE products;
-- +goose StatementEnd
```

## Auto-Generated DOWN Migrations

The command attempts to generate appropriate rollback SQL:

### CREATE Operations
```sql
-- UP
CREATE TABLE users (...);
-- DOWN (auto-generated)
DROP TABLE users;

-- UP
CREATE INDEX idx_name ON table(column);
-- DOWN (auto-generated)  
DROP INDEX idx_name;
```

### ALTER TABLE Operations
```sql
-- UP
ALTER TABLE users ADD COLUMN age INTEGER;
-- DOWN (auto-generated)
ALTER TABLE users DROP COLUMN age;

-- UP
ALTER TABLE users ALTER COLUMN email TYPE TEXT;
-- DOWN (requires manual specification)
-- Cannot auto-reverse type changes safely
```

### Complex Operations
```sql
-- UP
INSERT INTO settings VALUES ('key', 'value');
-- DOWN (auto-generated)
DELETE FROM settings WHERE key = 'key';

-- UP (complex data migration)
UPDATE users SET status = 'active' WHERE created_at > '2024-01-01';
-- DOWN (requires manual specification)
-- Cannot safely auto-reverse data updates
```

## Working with SQL Files

### Single File Processing

```bash
# Process single SQL file
makemigrations makemigrations_sql --sql-file init.sql --name "initial_schema"
```

### Multiple Statements

```sql
-- migration.sql
BEGIN;

CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL
);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category_id INTEGER REFERENCES categories(id)
);

INSERT INTO categories (name, slug) VALUES 
    ('Electronics', 'electronics'),
    ('Books', 'books');

COMMIT;
```

```bash
makemigrations makemigrations_sql --sql-file migration.sql --name "setup_catalog"
```

### Handling Transactions

The command preserves transaction blocks:

```sql
-- +goose Up
-- +goose StatementBegin
BEGIN;

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL
);

INSERT INTO users (email) VALUES ('admin@example.com');

COMMIT;
-- +goose StatementEnd
```

## Advanced Usage

### Custom Migration Names

```bash
# Auto-generated name from SQL content
makemigrations makemigrations_sql --sql "CREATE TABLE orders (...)"
# Generates: 20240122134500_create_orders

# Custom name
makemigrations makemigrations_sql --sql "CREATE TABLE orders (...)" --name "setup_order_system"
# Generates: 20240122134500_setup_order_system

# Descriptive names for complex operations
makemigrations makemigrations_sql \
  --sql-file complex_migration.sql \
  --name "refactor_user_authentication_system"
```

### Handling Different SQL Dialects

#### PostgreSQL
```bash
makemigrations makemigrations_sql \
  --sql "CREATE TABLE users (id UUID PRIMARY KEY DEFAULT gen_random_uuid());"
```

#### MySQL
```bash
makemigrations makemigrations_sql \
  --sql "CREATE TABLE users (id CHAR(36) PRIMARY KEY DEFAULT (UUID()));"
```

#### SQLite
```bash
makemigrations makemigrations_sql \
  --sql "CREATE TABLE users (id TEXT PRIMARY KEY);"
```

### Data Migrations

```bash
# Data-only migration
makemigrations makemigrations_sql \
  --sql "INSERT INTO roles (name, description) VALUES ('admin', 'Administrator role');" \
  --down-sql "DELETE FROM roles WHERE name = 'admin';" \
  --name "add_admin_role"

# Complex data transformation
makemigrations makemigrations_sql \
  --sql-file data_migration.sql \
  --down-sql "-- Manual rollback required" \
  --name "migrate_legacy_user_data"
```

## Error Handling

### SQL Parsing Errors

```bash
$ makemigrations makemigrations_sql --sql "CREATE TALE users (id INT);"
▶ Processing SQL input...
✗ SQL parsing failed: syntax error near "TALE"
  Expected: CREATE TABLE

Suggestion: Check SQL syntax
```

### Unsupported Operations

```bash
$ makemigrations makemigrations_sql --sql "DROP DATABASE old_db;"
▶ Processing SQL input...
⚠️  Warning: Cannot auto-generate safe rollback for DROP DATABASE
  Consider using --down-sql to specify manual rollback
```

### File Not Found

```bash
$ makemigrations makemigrations_sql --sql-file missing.sql
✗ Error: SQL file not found: missing.sql

Suggestion: Check file path and permissions
```

## Integration with Workflow

### Development Workflow

```bash
# 1. Develop SQL interactively
psql -d mydb
# Run experimental SQL...

# 2. Save working SQL to file
echo "CREATE TABLE new_feature (...);" > feature.sql

# 3. Generate migration
makemigrations makemigrations_sql --sql-file feature.sql --name "add_new_feature"

# 4. Review and apply
makemigrations goose up-by-one
```

### Converting Legacy SQL

```bash
# Convert existing SQL scripts to migrations
for sql_file in legacy_sql/*.sql; do
    name=$(basename "$sql_file" .sql)
    makemigrations makemigrations_sql \
      --sql-file "$sql_file" \
      --name "legacy_$name"
done
```

### Team Collaboration

```bash
# Developer creates SQL for feature
vim user_preferences.sql

# Generate migration with team review
makemigrations makemigrations_sql \
  --sql-file user_preferences.sql \
  --name "add_user_preferences" \
  --dry-run

# Review, then generate actual migration
makemigrations makemigrations_sql \
  --sql-file user_preferences.sql \
  --name "add_user_preferences"
```

## Configuration Integration

### Migration Directory

```yaml
# migrations/makemigrations.config.yaml
migration:
  directory: db/migrations          # Custom migration directory
  review_comment_prefix: "-- SQL-REVIEW: "
```

### Database-Specific Settings

```yaml
database:
  type: postgresql                  # Affects DOWN generation logic
  quote_identifiers: true          # Affects SQL formatting
```

## Best Practices

### 1. SQL File Organization

```
sql/
├── schema/
│   ├── users.sql                  # Table definitions
│   ├── products.sql
│   └── orders.sql
├── data/
│   ├── initial_roles.sql          # Reference data
│   └── default_settings.sql
└── migrations/
    ├── 001_complex_refactor.sql   # Complex operations
    └── 002_performance_indexes.sql
```

### 2. DOWN Migration Strategy

```bash
# For reversible operations - let tool auto-generate
makemigrations makemigrations_sql \
  --sql "CREATE TABLE temp_table (...);" \
  --name "add_temp_table"

# For irreversible operations - provide explicit DOWN
makemigrations makemigrations_sql \
  --sql "DROP COLUMN legacy_data FROM users;" \
  --down-sql "-- IRREVERSIBLE: Cannot restore dropped data" \
  --name "remove_legacy_data"

# For data migrations - provide manual DOWN
makemigrations makemigrations_sql \
  --sql "UPDATE users SET status = 'migrated';" \
  --down-sql "UPDATE users SET status = 'pending';" \
  --name "update_user_status"
```

### 3. Testing Migrations

```bash
# Generate and test immediately
makemigrations makemigrations_sql --sql-file test.sql --name "test_migration"
makemigrations goose up-by-one
makemigrations goose down
makemigrations goose up-by-one
```

## Limitations

### Auto-Generated DOWN Limitations

- **Data modifications**: Cannot safely reverse UPDATE/DELETE operations
- **Type changes**: Cannot reverse ALTER COLUMN TYPE changes
- **Complex constraints**: May not properly reverse complex constraint additions
- **Stored procedures**: Cannot reverse function/procedure creations
- **Permission changes**: Cannot reverse GRANT/REVOKE operations

### SQL Parsing Limitations

- **Database-specific syntax**: May not parse all database-specific extensions
- **Complex stored procedures**: Limited support for procedure/function bodies
- **Dynamic SQL**: Cannot process dynamic SQL generation
- **Comments**: May not preserve all comment styles

## See Also

- [init_sql Command](./init_sql.md) - Initialize SQL-based projects
- [Goose Integration](./goose.md) - Apply generated migrations
- [makemigrations Command](./makemigrations.md) - YAML-based migration generation
- [Configuration Guide](../configuration.md) - Configuration options