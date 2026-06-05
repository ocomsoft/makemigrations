---
name: go-morphic
description: Use when making database schema changes in Go projects — adding tables, fields, indexes, foreign keys, or modifying columns. Triggers on any database structure work, migration creation, or when a project has schema/schema.yaml or migrations/morphic.config.yaml. Enforces schema-first workflow over raw SQL.
---

# Go Morphic

## Overview

**morphic** is the database migration tool for Go projects at Ocom. It works like Django migrations: you define your schema in YAML, and the tool generates type-safe Go migration code.

**Core principle:** schema.yaml is the single source of truth. All database changes flow through it. RunSQL is a last resort.

## When to Use

Auto-trigger on ANY of these:

- "Add a table/model/field/column/index"
- "Change the database schema"
- "Create a migration"
- "Add a foreign key / relationship"
- "Rename a table/column"
- "Remove/drop a table/column"
- Working in a project that has `schema/schema.yaml`
- Working in a project that has `migrations/morphic.config.yaml`
- Any request that implies database structure changes

## The Workflow

```dot
digraph morphic_workflow {
    rankdir=TB;
    "Database change needed" [shape=doublecircle];
    "Is morphic initialized?" [shape=diamond];
    "Run morphic init" [shape=box];
    "Edit schema/schema.yaml" [shape=box];
    "Run morphic generate" [shape=box];
    "Review generated .go file" [shape=box];
    "Preview SQL with migrate showsql" [shape=box];
    "Run tests" [shape=box];
    "Done" [shape=doublecircle];

    "Database change needed" -> "Is morphic initialized?";
    "Is morphic initialized?" -> "Run morphic init" [label="no"];
    "Is morphic initialized?" -> "Edit schema/schema.yaml" [label="yes"];
    "Run morphic init" -> "Edit schema/schema.yaml";
    "Edit schema/schema.yaml" -> "Run morphic generate";
    "Run morphic generate" -> "Review generated .go file";
    "Review generated .go file" -> "Preview SQL with migrate showsql";
    "Preview SQL with migrate showsql" -> "Run tests";
    "Run tests" -> "Done";
}
```

### Step 1: Check Initialization

Look for `migrations/morphic.config.yaml`. If missing:

```bash
morphic init --database postgresql
```

This creates:
- `migrations/go.mod`
- `migrations/main.go`
- `migrations/morphic.config.yaml`

Then create `schema/schema.yaml` with the initial database definition.

### Step 2: Edit schema/schema.yaml

This is the source of truth. ALL schema changes happen here first.

```yaml
database:
  name: myapp
  version: 1.0.0

defaults:
  postgresql:
    now: CURRENT_TIMESTAMP
    new_uuid: gen_random_uuid()

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
    indexes:
      - name: idx_users_email
        fields: [email]
        unique: true
```

### Step 3: Generate Migration

```bash
morphic generate --name "describe_the_change"
```

Use `--dry-run` to preview without writing files. Use `--check` to verify schema is up to date (CI use).

### Step 4: Review and Verify

```bash
# Preview the SQL that will run
morphic migrate showsql

# Run tests
go test ./...
```

### Step 5: Apply (when ready)

```bash
morphic migrate up
```

## Rules

1. **Schema-first**: Edit `schema/schema.yaml` before anything else. Never write SQL to change structure.
2. **NEVER compile or run migrations with `go build` / `go run`**: The `migrations/` directory contains `.go` files but you must NOT compile or execute them with the Go toolchain. Always use `morphic migrate up`, `morphic migrate showsql`, etc. The yaegi interpreter runs them in-process — no build step needed.
3. **Prefer generated code unchanged**: Only modify generated migration `.go` files if you absolutely must (e.g., adding data migration logic). Try to leave them as-is.
4. **RunSQL is last resort**: Only for data migrations or complex SQL that cannot be expressed in the schema. Use `morphic generate empty --name "description"` to create the shell.
5. **Never skip generation**: Don't hand-write migration operations. Let the tool diff the schema and generate them.
6. **Name migrations descriptively**: `--name "add_user_profiles"` not `--name "update"`.

## Quick Reference: Field Types

| Type | Properties | Example |
|------|-----------|---------|
| `varchar` | `length` | `type: varchar, length: 255` |
| `text` | — | `type: text` |
| `text[]` | — | `type: text[]` (PostgreSQL arrays) |
| `integer` | — | `type: integer` |
| `bigint` | — | `type: bigint` |
| `float` | — | `type: float` |
| `decimal` | `precision`, `scale` | `type: decimal, precision: 10, scale: 2` |
| `boolean` | — | `type: boolean` |
| `date` | — | `type: date` |
| `timestamp` | `auto_create`, `auto_update` | `type: timestamp, auto_create: true` |
| `time` | — | `type: time` |
| `uuid` | — | `type: uuid, default: new_uuid` |
| `json` | — | `type: json` |
| `jsonb` | — | `type: jsonb` |
| `serial` | — | `type: serial` (auto-increment) |

## Quick Reference: Field Properties

| Property | Type | Description |
|----------|------|-------------|
| `name` | string | Column name (required) |
| `type` | string | Field type (required) |
| `primary_key` | bool | Mark as primary key |
| `nullable` | bool | Allow NULL (default: true) |
| `default` | string | Default value — references `defaults` section or literal |
| `length` | int | For varchar/text |
| `precision` | int | For decimal |
| `scale` | int | For decimal |
| `auto_create` | bool | Auto-set on INSERT (timestamps) |
| `auto_update` | bool | Auto-set on UPDATE (timestamps) |

## Quick Reference: Foreign Keys

```yaml
- name: author_id
  type: foreign_key
  foreign_key:
    table: users
    on_delete: CASCADE    # CASCADE, RESTRICT, SET_NULL, PROTECT
```

## Quick Reference: Many-to-Many

```yaml
- name: tags
  type: many_to_many
  many_to_many:
    table: tags
```

Generates a junction table automatically.

## Quick Reference: Indexes

```yaml
indexes:
  - name: idx_users_email
    fields: [email]
    unique: true
  - name: idx_posts_search
    fields: [title, body]
    method: GIN           # PostgreSQL: BTREE, HASH, GIN, GIST, BRIN
    where: "deleted_at IS NULL"   # Partial index (PostgreSQL only)
```

## Quick Reference: Defaults Section

Define reusable default values per database type:

```yaml
defaults:
  postgresql:
    now: CURRENT_TIMESTAMP
    new_uuid: gen_random_uuid()
    uuid: uuid_generate_v4()
  mysql:
    now: CURRENT_TIMESTAMP
    new_uuid: UUID()
```

Reference them in fields with `default: now` or `default: new_uuid`.

## Quick Reference: Type Mappings

Override SQL types per database when the built-in mapping doesn't fit:

```yaml
type_mappings:
  postgresql:
    money: "DECIMAL(19,4)"
    percentage: "DECIMAL(5,2)"
  mysql:
    money: "DECIMAL(19,4)"
```

## Quick Reference: Include (Schema Composition)

Import schemas from other Go modules:

```yaml
include:
  - module: github.com/company/auth-schemas
    path: schemas/auth.yaml
```

## Available Commands

| Command | Purpose |
|---------|---------|
| `morphic init` | Bootstrap migrations directory |
| `morphic generate` | Generate migration from schema diff |
| `morphic migrate up` | Apply pending migrations |
| `morphic migrate down` | Rollback last migration |
| `morphic migrate status` | Show migration status |
| `morphic migrate showsql` | Preview SQL without applying |
| `morphic migrate dag` | View migration dependency graph |
| `morphic generate empty` | Create blank migration (for RunSQL) |
| `morphic db2schema` | Reverse-engineer schema from existing DB |
| `morphic struct2schema` | Convert Go structs to schema YAML |
| `morphic generate dump-data` | Generate data-seeding migration |
| `morphic schema-to-sql` | Convert merged YAML schema to SQL |

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Running `go build` or `go run` on the migrations directory | Use `morphic migrate up` — yaegi interprets them in-process, no compilation needed |
| Writing CREATE TABLE SQL directly | Edit schema.yaml and run `morphic generate` |
| Hand-writing migration operations | Let the tool generate them from the schema diff |
| Forgetting `--name` flag | Always name migrations: `--name "add_user_profiles"` |
| Using RunSQL for structure changes | Express it in schema.yaml instead |
| Editing generated migrations unnecessarily | Only modify if you genuinely must; prefer unchanged |
| Not previewing SQL before applying | Always run `migrate showsql` first |

## When RunSQL IS Appropriate

- Data migrations (backfilling values, transforming data)
- Complex constraints the schema format can't express
- Database-specific features not covered by field types (e.g., triggers, stored procedures)
- One-off fixes that don't map to schema changes

Create the shell with: `morphic generate empty --name "description"`
