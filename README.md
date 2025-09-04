# makemigrations

A **YAML-first** database migration tool that generates [Goose](https://github.com/pressly/goose)-compatible migration files from declarative YAML schema definitions. Build database-agnostic schemas that automatically convert to database-specific SQL migrations.

## ✨ Why YAML Schemas?

**The YAML schema format is the primary interface for makemigrations.** It provides:

- 🗄️ **Database-agnostic**: Write once, deploy to PostgreSQL, MySQL, SQLite, or SQL Server
- 🔧 **Declarative**: Define what you want, not how to build it
- 🤖 **Automatic migration generation**: Changes detected and converted to SQL migrations
- 🔗 **Relationship management**: Foreign keys and many-to-many relationships handled automatically
- ✅ **Built-in validation**: Schema validation with helpful error messages
- 🔄 **Change tracking**: Automatic schema snapshots and diff generation

## 🚀 Quick Start

### 1. Install makemigrations

```bash
# Install from GitHub
go install github.com/ocomsoft/makemigrations@latest

# Or build from source
git clone https://github.com/ocomsoft/makemigrations
cd makemigrations
go build -o makemigrations .
```

### 2. Initialize your project

```bash
# Create project structure with YAML schema support
makemigrations init

# This creates:
# migrations/makemigrations.config.yaml   # Configuration
# migrations/.schema_snapshot.yaml        # State tracking (empty initially)
# schema/schema.yaml                      # Your schema definition
```

### 3. Define your database schema

Edit `schema/schema.yaml`:

```yaml
database:
  name: myapp
  version: 1.0.0

defaults:
  postgresql:
    blank: ''
    now: CURRENT_TIMESTAMP
    new_uuid: gen_random_uuid()
    zero: "0"
    true: "true"
    false: "false"

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

  - name: posts
    fields:
      - name: id
        type: uuid
        primary_key: true
        default: new_uuid
      - name: title
        type: varchar
        length: 200
        nullable: false
      - name: user_id
        type: foreign_key
        nullable: false
        foreign_key:
          table: users
          on_delete: CASCADE
```

### 4. Generate your first migration

```bash
# Generate migration from YAML schema
makemigrations makemigrations --name "initial_schema"

# Output:
# migrations/20240122134500_initial_schema.sql
```

### 5. Set up database connection

```bash
# PostgreSQL
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_PORT=5432
export MAKEMIGRATIONS_DB_USER=postgres
export MAKEMIGRATIONS_DB_PASSWORD=yourpassword
export MAKEMIGRATIONS_DB_NAME=yourdb

# MySQL, SQLite, SQL Server also supported
```

### 6. Apply migrations to your database

```bash
# Apply all pending migrations
makemigrations goose up

# Check migration status
makemigrations goose status
```

## 🏗️ Core Features

### Database-Agnostic Schemas

Write your schema once in YAML, deploy anywhere:

```yaml
# Same YAML schema generates different SQL for each database

# PostgreSQL:
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metadata JSONB DEFAULT '{}'
);

# MySQL:
CREATE TABLE users (
    id CHAR(36) PRIMARY KEY DEFAULT (UUID()),
    metadata JSON DEFAULT ('{}')
);

# SQLite:
CREATE TABLE users (
    id TEXT PRIMARY KEY DEFAULT '',
    metadata TEXT DEFAULT '{}'
);
```

### Automatic Relationship Management

Define relationships declaratively:

```yaml
# Foreign keys
- name: user_id
  type: foreign_key
  foreign_key:
    table: users
    on_delete: CASCADE

# Many-to-many relationships (use explicit junction tables)
# Define the junction table separately:
- name: post_categories  # Junction table
  fields:
    - name: post_id
      type: foreign_key
      foreign_key:
        table: posts
        on_delete: CASCADE
    - name: category_id
      type: foreign_key
      foreign_key:
        table: categories
        on_delete: CASCADE
```

### Smart Change Detection

makemigrations tracks your schema state and generates only the necessary changes:

```bash
# Add a new field to your YAML schema
- name: phone
  type: varchar
  length: 20
  nullable: true

# Run makemigrations
makemigrations makemigrations

# Generates migration with only the new field:
# ALTER TABLE users ADD COLUMN phone VARCHAR(20);
```

### Safety Features

Destructive operations require review:

```sql
-- +goose Up
-- REVIEW: The following operation is destructive and requires manual review
-- +goose StatementBegin
DROP TABLE old_users;
-- +goose StatementEnd
```

## 📖 Documentation

### Essential Guides

- **[Installation Guide](docs/installation.md)** - Complete setup instructions
- **[Schema Format Guide](docs/schema-format.md)** - Complete YAML schema reference
- **[Configuration Guide](docs/configuration.md)** - Configuration options and environment variables

### Command Reference

- **[makemigrations](docs/commands/makemigrations.md)** - Generate migrations from YAML schemas ⭐ **Primary command**
- **[init](docs/commands/init.md)** - Initialize new YAML-based projects
- **[goose](docs/commands/goose.md)** - Apply migrations to database
- **[dump_sql](docs/commands/dump_sql.md)** - Preview generated SQL from schemas

### Additional Tools

- **[struct2schema](docs/commands/struct2schema.md)** - Generate YAML schemas from Go structs ⭐ **New feature**
- **[init_sql](docs/commands/init_sql.md)** - Initialize SQL-based projects (alternative workflow)
- **[makemigrations_sql](docs/commands/makemigrations_sql.md)** - Generate migrations from raw SQL

## 🗄️ Database Support

makemigrations supports 12 different database types with comprehensive provider implementations:

### Core Databases (Original Support)
| Database | Status | Testing | Features |
|----------|--------|---------|----------|
| **PostgreSQL** | ✅ Full support | ✅ **Fully tested** | UUID, JSONB, arrays, advanced types |
| **MySQL** | ✅ Supported | ⚠️ Provider tested only | JSON, AUTO_INCREMENT, InnoDB features |
| **SQLite** | ✅ Supported | ⚠️ Provider tested only | Simplified types, basic constraints |
| **SQL Server** | ✅ Supported | ⚠️ Provider tested only | UNIQUEIDENTIFIER, NVARCHAR, BIT types |
| **Amazon Redshift** | ✅ Provider ready | ⚠️ Provider tested only | SUPER JSON type, IDENTITY sequences, VARCHAR limits |
| **ClickHouse** | ✅ Provider ready | ⚠️ Provider tested only | MergeTree engine, columnar storage, Nullable types |
| **TiDB** | ✅ Provider ready | ⚠️ Provider tested only | MySQL-compatible, distributed, native BOOLEAN |
| **Vertica** | ✅ Provider ready | ⚠️ Provider tested only | Columnar analytics, LONG VARCHAR, CASCADE support |
| **YDB (Yandex)** | ✅ Provider ready | ⚠️ Provider tested only | Distributed SQL, Optional<Type>, native JSON |
| **Turso** | ✅ Provider ready | ⚠️ Provider tested only | Edge SQLite, distributed capabilities |
| **StarRocks** | ✅ Provider ready | ⚠️ Provider tested only | MPP analytics, OLAP engine, STRING types |
| **Aurora DSQL** | ✅ Provider ready | ⚠️ Provider tested only | AWS serverless, PostgreSQL-compatible |

**Note**: Only PostgreSQL has been tested with real database instances. All other providers have comprehensive unit tests and follow database-specific SQL syntax, but may require additional testing and refinement for production use.

## 💻 Command Overview

### Primary YAML Commands

```bash
# Initialize YAML-based project
makemigrations init

# Generate migrations from YAML schemas
makemigrations makemigrations

# Preview changes without creating files
makemigrations makemigrations --dry-run

# Check if migrations are needed (CI/CD)
makemigrations makemigrations --check
```

### Database Operations

```bash
# Apply migrations
makemigrations goose up

# Check migration status
makemigrations goose status

# Rollback last migration
makemigrations goose down

# Create custom migration
makemigrations goose create add_indexes
```

### Utilities

```bash
# Preview SQL without generating migrations
makemigrations dump_sql

# Generate YAML schemas from Go structs
makemigrations struct2schema --input ./models --output schema/schema.yaml

```

## 🏗️ Project Structure

```
myproject/
├── schema/
│   └── schema.yaml              # Your YAML schema definition
├── migrations/
│   ├── makemigrations.config.yaml    # Configuration
│   ├── .schema_snapshot.yaml          # State tracking
│   ├── 20240122134500_initial.sql     # Generated migrations
│   └── 20240123102000_add_posts.sql
├── go.mod
└── main.go
```

## ⚙️ Configuration

### Environment Variables

```bash
# Database connection
export MAKEMIGRATIONS_DB_HOST=localhost
export MAKEMIGRATIONS_DB_USER=postgres
export MAKEMIGRATIONS_DB_PASSWORD=password
export MAKEMIGRATIONS_DB_NAME=myapp

# Tool behavior (12 database types supported)
export MAKEMIGRATIONS_DATABASE_TYPE=postgresql  # postgresql, mysql, sqlite, sqlserver, redshift, clickhouse, tidb, vertica, ydb, turso, starrocks, auroradsql
export MAKEMIGRATIONS_MIGRATION_SILENT=false
export MAKEMIGRATIONS_OUTPUT_VERBOSE=true
```

### Configuration File

`migrations/makemigrations.config.yaml`:

```yaml
database:
  type: postgresql
  quote_identifiers: true

migration:
  directory: migrations
  include_down_sql: true
  review_comment_prefix: "-- REVIEW: "

output:
  verbose: false
  color_enabled: true
```

See the [Configuration Guide](docs/configuration.md) for complete options.

## 🔧 Advanced Features

### Multi-Database Deployment

```bash
# Generate PostgreSQL migrations (fully tested)
makemigrations makemigrations --database postgresql

# Generate MySQL migrations for the same schema
makemigrations makemigrations --database mysql

# Generate migrations for cloud/analytics databases
makemigrations makemigrations --database redshift
makemigrations makemigrations --database clickhouse
makemigrations makemigrations --database tidb

# Same YAML schema, database-specific SQL output
```

### Complex Relationships

```yaml
# Self-referencing foreign keys
- name: parent_id
  type: foreign_key
  nullable: true
  foreign_key:
    table: categories
    on_delete: SET_NULL

# Many-to-many with explicit junction table
- name: post_tags  # Junction table
  fields:
    - name: post_id
      type: foreign_key
      foreign_key:
        table: posts
        on_delete: CASCADE
    - name: tag_id
      type: foreign_key
      foreign_key:
        table: tags
        on_delete: CASCADE
  indexes:
    - name: post_tags_unique
      fields: [post_id, tag_id]
      unique: true
```

### Custom Default Values

```yaml
defaults:
  postgresql:
    custom_uuid: custom_uuid_function()
    app_timestamp: custom_timestamp()
    
tables:
  - name: events
    fields:
      - name: id
        type: uuid
        default: custom_uuid  # Uses your custom function
```

## 🚀 Workflow Examples

### Development Workflow

```bash
# 1. Modify schema/schema.yaml
vim schema/schema.yaml

# 2. Preview changes
makemigrations makemigrations --dry-run

# 3. Generate migration
makemigrations makemigrations --name "add_user_preferences"

# 4. Review generated SQL
cat migrations/20240122134500_add_user_preferences.sql

# 5. Apply to database
makemigrations goose up
```

### Team Development

```bash
# Developer A: Add new feature schema
git pull
vim schema/schema.yaml  # Add new tables
makemigrations makemigrations --name "add_messaging_system"
git add . && git commit -m "Add messaging schema"

# Developer B: Pull and apply
git pull
makemigrations goose up  # Apply new migrations
```

### CI/CD Integration

```yaml
# .github/workflows/migrations.yml
- name: Check for schema changes
  run: |
    makemigrations makemigrations --check
    if [ $? -eq 1 ]; then
      echo "Schema changes detected - migrations needed"
      exit 1
    fi

- name: Apply migrations
  run: makemigrations goose up
```

## 🛡️ Best Practices

### 1. Schema Organization

```yaml
# Good: Organized, clear field definitions
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
```

### 2. Migration Naming

```bash
# Good: Descriptive migration names
makemigrations makemigrations --name "add_user_authentication"
makemigrations makemigrations --name "optimize_product_queries"

# Avoid: Generic names
makemigrations makemigrations --name "changes"
```

### 3. Testing Migrations

```bash
# Always test rollback capability
makemigrations goose up-by-one
makemigrations goose down
makemigrations goose up-by-one
```

## 🔄 Alternative Workflows

While **YAML schemas are the primary and recommended approach**, makemigrations also supports:

### SQL-Based Migrations

For teams preferring direct SQL control:

```bash
# Initialize SQL-based project
makemigrations init_sql

# Generate migration from raw SQL
makemigrations makemigrations_sql --sql "CREATE TABLE test (id SERIAL);"
```

### Go Struct Integration

Generate schemas from Go structs:

```bash
# Convert Go structs to YAML
makemigrations struct2schema --input ./models --output schema/schema.yaml

# Process specific files with custom configuration
makemigrations struct2schema --input models.go --config struct2schema.yaml --database postgresql
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Add tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Documentation**: Browse the [`/docs`](docs/) directory
- **Issues**: Report bugs on [GitHub Issues](https://github.com/ocomsoft/makemigrations/issues)
- **Discussions**: Join [GitHub Discussions](https://github.com/ocomsoft/makemigrations/discussions)

---

**Ready to get started?** Check out the [Installation Guide](docs/installation.md) and [Schema Format Guide](docs/schema-format.md)!
