# Goose Integration

The `makemigrations goose` subcommand provides direct access to [Goose](https://github.com/pressly/goose) database migration commands using the same configuration as makemigrations.

## Overview

Goose is a database migration tool that supports sequential and versioned migrations. This integration allows you to use all goose commands through makemigrations while leveraging the same database configuration and migrations directory.

## Configuration

The goose subcommands use the makemigrations configuration system, including:

- **Database type** from `makemigrations.config.yaml`
- **Migrations directory** from configuration
- **Database connection** from environment variables

### Database Connection Environment Variables

Set these environment variables to configure database connections:

#### PostgreSQL
```bash
MAKEMIGRATIONS_DB_HOST=localhost        # Default: localhost
MAKEMIGRATIONS_DB_PORT=5432             # Default: 5432
MAKEMIGRATIONS_DB_USER=postgres         # Default: postgres
MAKEMIGRATIONS_DB_PASSWORD=mypassword   # Optional
MAKEMIGRATIONS_DB_NAME=mydb             # Default: postgres
MAKEMIGRATIONS_DB_SSLMODE=disable       # Default: disable
```

#### MySQL
```bash
MAKEMIGRATIONS_DB_HOST=localhost        # Default: localhost
MAKEMIGRATIONS_DB_PORT=3306             # Default: 3306
MAKEMIGRATIONS_DB_USER=root             # Default: root
MAKEMIGRATIONS_DB_PASSWORD=mypassword   # Optional
MAKEMIGRATIONS_DB_NAME=mydb             # Default: mysql
```

#### SQLite
```bash
MAKEMIGRATIONS_DB_PATH=./database.db    # Default: database.db
```

#### SQL Server
```bash
MAKEMIGRATIONS_DB_HOST=localhost        # Default: localhost
MAKEMIGRATIONS_DB_PORT=1433             # Default: 1433
MAKEMIGRATIONS_DB_USER=sa               # Default: sa
MAKEMIGRATIONS_DB_PASSWORD=mypassword   # Required
MAKEMIGRATIONS_DB_NAME=mydb             # Default: master
```

## Available Commands

### Migration Commands

#### `makemigrations goose up`
Migrate the database to the most recent version available.

```bash
makemigrations goose up
```

#### `makemigrations goose up-by-one`
Migrate the database up by exactly one version.

```bash
makemigrations goose up-by-one
```

#### `makemigrations goose up-to VERSION`
Migrate the database to a specific version.

```bash
makemigrations goose up-to 20240101120000
```

#### `makemigrations goose down`
Roll back the database by one version.

```bash
makemigrations goose down
```

#### `makemigrations goose down-to VERSION`
Roll back the database to a specific version.

```bash
makemigrations goose down-to 20240101120000
```

#### `makemigrations goose redo`
Re-run the latest migration (down then up).

```bash
makemigrations goose redo
```

#### `makemigrations goose reset`
Roll back all migrations (reset to initial state).

```bash
makemigrations goose reset
```

### Information Commands

#### `makemigrations goose status`
Print the status of all migrations.

```bash
makemigrations goose status
```

Example output:
```
▶ Running goose status...
    Applied At                  Migration
    =======================================
    Mon Jan  1 12:00:00 2024 -- 20240101120000_initial.sql
    Pending                  -- 20240102120000_add_users.sql
    Pending                  -- 20240103120000_add_posts.sql
✓ goose status completed successfully
```

#### `makemigrations goose version`
Print the current version of the database.

```bash
makemigrations goose version
```

### Management Commands

#### `makemigrations goose create NAME`
Create a new migration file.

```bash
makemigrations goose create add_user_table
```

This creates a new file like `migrations/20240101120000_add_user_table.sql` with the goose template:

```sql
-- +goose Up
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
```

#### `makemigrations goose fix`
Apply sequential ordering to migrations (fixes numbering issues).

```bash
makemigrations goose fix
```

## Usage Examples

### Basic Workflow

1. **Check current status:**
   ```bash
   makemigrations goose status
   ```

2. **Apply all pending migrations:**
   ```bash
   makemigrations goose up
   ```

3. **Roll back the last migration:**
   ```bash
   makemigrations goose down
   ```

4. **Create a new migration:**
   ```bash
   makemigrations goose create add_user_preferences
   ```

### Development Workflow

1. **Create and edit a migration:**
   ```bash
   makemigrations goose create add_email_index
   # Edit the generated SQL file
   makemigrations goose up-by-one
   ```

2. **Test rollback:**
   ```bash
   makemigrations goose down
   makemigrations goose up
   ```

### Production Workflow

1. **Check status before deployment:**
   ```bash
   makemigrations goose status
   ```

2. **Apply migrations one by one:**
   ```bash
   makemigrations goose up-by-one
   makemigrations goose status
   # Repeat as needed
   ```

3. **Or apply all at once:**
   ```bash
   makemigrations goose up
   ```

## Integration with Makemigrations

The goose subcommand complements the main makemigrations functionality:

1. **Use `makemigrations` to generate migrations** from YAML schemas
2. **Use `makemigrations goose` to apply migrations** to the database

### Example Workflow

```bash
# Generate migration from schema changes
makemigrations --dry-run
makemigrations

# Check what migrations are pending
makemigrations goose status

# Apply the new migration
makemigrations goose up

# Verify current database version
makemigrations goose version
```

## Migration File Format

Goose uses a specific format for migration files:

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

### Key Points:
- `-- +goose Up` section contains forward migration SQL
- `-- +goose Down` section contains rollback migration SQL
- `StatementBegin` and `StatementEnd` wrap complex statements
- Multiple statement blocks are supported

## Error Handling

The goose subcommands provide clear error messages:

```bash
$ makemigrations goose up
▶ Running goose up...
goose up failed: dial tcp 127.0.0.1:5432: connect: connection refused
```

Common issues:
- **Database connection failed**: Check environment variables and database availability
- **Migration syntax error**: Review SQL syntax in migration files
- **Version conflicts**: Use `makemigrations goose fix` to resolve numbering issues

## Best Practices

1. **Always check status before operations:**
   ```bash
   makemigrations goose status
   ```

2. **Test migrations in development:**
   ```bash
   makemigrations goose up-by-one
   makemigrations goose down
   makemigrations goose up-by-one
   ```

3. **Use version-specific operations in production:**
   ```bash
   makemigrations goose up-to 20240101120000
   ```

4. **Keep migration files simple and focused:**
   - One logical change per migration
   - Always provide rollback (Down) operations
   - Test both up and down migrations

5. **Coordinate with team:**
   - Use `makemigrations goose status` to check state
   - Communicate before running `reset` or major rollbacks

## Supported Databases

The goose integration supports all databases that makemigrations supports:

- **PostgreSQL** (`postgresql`)
- **MySQL** (`mysql`)
- **SQLite** (`sqlite`)
- **SQL Server** (`sqlserver`)

Database-specific features and SQL syntax are handled by the underlying goose library.

## Troubleshooting

### Connection Issues

If you see connection errors:

1. **Verify environment variables:**
   ```bash
   echo $MAKEMIGRATIONS_DB_HOST
   echo $MAKEMIGRATIONS_DB_USER
   # etc.
   ```

2. **Test database connectivity:**
   ```bash
   # PostgreSQL example
   psql -h $MAKEMIGRATIONS_DB_HOST -U $MAKEMIGRATIONS_DB_USER -d $MAKEMIGRATIONS_DB_NAME
   ```

3. **Check makemigrations config:**
   ```bash
   cat migrations/makemigrations.config.yaml
   ```

### Migration Issues

If migrations fail:

1. **Check migration syntax:**
   - Review the SQL in the failing migration file
   - Test SQL statements manually in database client

2. **Verify migration order:**
   ```bash
   makemigrations goose fix
   ```

3. **Check database state:**
   ```bash
   makemigrations goose status
   makemigrations goose version
   ```

### Performance Considerations

- Large migrations may take time; monitor progress
- Consider breaking large changes into smaller migrations
- Test migrations on similar-sized datasets in development

## See Also

- [Goose Documentation](https://github.com/pressly/goose)
- [Makemigrations Configuration](./config.md)
- [Migration Best Practices](./migrations.md)