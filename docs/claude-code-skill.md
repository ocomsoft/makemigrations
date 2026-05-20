# Claude Code Skill

makemigrations ships with a Claude Code skill that teaches Claude how to make database changes in Go projects using the correct schema-first workflow.

## Overview

When installed, the skill **auto-triggers** whenever Claude detects database-related work:

- Adding or modifying tables, fields, indexes, or foreign keys
- Creating migrations
- Working in a project with `schema/schema.yaml` or `migrations/makemigrations.config.yaml`

Claude will follow the enforced workflow rather than writing raw SQL or hand-crafting migration files.

## Installation

### Personal Skill (Recommended)

Copy the skill into your Claude Code personal skills directory so it's available in all Go projects:

```bash
cp -r /path/to/makemigrations/skills/ ~/.claude/skills/go-makemigrations
```

Or symlink if you want it to stay in sync with the repo:

```bash
ln -s /path/to/makemigrations/skills ~/.claude/skills/go-makemigrations
```

### Plugin Installation

The repository includes a `.claude-plugin/plugin.json` for plugin-based installation. If registered as a Claude Code plugin marketplace source, the skill is installed automatically.

## Enforced Workflow

The skill guides Claude through these steps for every database change:

```
1. Check if makemigrations is initialized
   └─ If not, run: makemigrations init --database <type>

2. Edit schema/schema.yaml
   └─ Add/modify/remove tables, fields, indexes, defaults, type mappings

3. Generate migration code
   └─ Run: makemigrations makemigrations --name "description"

4. Review and verify
   └─ Run: makemigrations migrate showsql
   └─ Run: go test ./...

5. Apply when ready
   └─ Run: makemigrations migrate up
```

## Rules the Skill Enforces

1. **Schema-first**: All structural database changes go through `schema/schema.yaml`. Claude will not write raw SQL for structure changes.

2. **Prefer generated code unchanged**: Claude will try to leave generated migration `.go` files as-is. If a modification is genuinely needed (e.g., data migration logic), it will be minimal and careful.

3. **RunSQL is last resort**: Only used for data migrations, complex constraints, or database-specific features that can't be expressed in the schema format. Claude uses `makemigrations empty --name "description"` to create the shell file.

4. **Never skip generation**: Claude will not hand-write migration operations. The tool diffs the schema and generates them.

5. **Descriptive names**: Migrations are always named descriptively (e.g., `--name "add_user_profiles"`).

## Quick References Included

The skill provides Claude with inline quick-reference tables for:

- **Field types**: varchar, text, integer, bigint, float, decimal, boolean, date, timestamp, time, uuid, json, jsonb, serial, text[]
- **Field properties**: primary_key, nullable, default, length, precision, scale, auto_create, auto_update
- **Foreign keys**: type, table, on_delete (CASCADE, RESTRICT, SET_NULL, PROTECT)
- **Many-to-many**: automatic junction table generation
- **Indexes**: unique, method (BTREE, HASH, GIN, GIST, BRIN), partial indexes with `where`
- **Defaults**: per-database default value definitions
- **Type mappings**: per-database SQL type overrides
- **All commands**: init, makemigrations, migrate (up/down/status/showsql/dag), empty, db2schema, struct2schema, dump-data

## Example Interaction

```
You: Add a profiles table with avatar_url and bio fields

Claude (auto-triggers go-makemigrations skill):
1. Checks for migrations/makemigrations.config.yaml ✓
2. Edits schema/schema.yaml to add the profiles table
3. Runs: makemigrations makemigrations --name "add_profiles"
4. Reviews the generated 000N_add_profiles.go
5. Runs: makemigrations migrate showsql to preview SQL
```

## Updating the Skill

If you installed via copy, update by copying the latest version:

```bash
cp -r /path/to/makemigrations/skills/ ~/.claude/skills/go-makemigrations
```

If you installed via symlink, it updates automatically when you pull the repo.

## Further Reading

- [Schema Format Guide](schema-format.md) — complete YAML schema reference
- [Configuration Guide](configuration.md) — config file and environment variables
- [Command Reference](commands/) — all available commands
