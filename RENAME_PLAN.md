# Plan: Rebrand `makemigrations` → `morphic`

## Goal
Rename the project from **makemigrations** to **morphic** everywhere — code, module
path, binary, CLI commands, config filename, DB history table, docs, and the Claude
Code skill/plugin — while keeping existing user projects working via backward-compat
fallbacks and a new `upgrade` command.

## Locked-in decisions
- **Full rebrand** across code, module path, binary, config, DB table, docs, skill.
- Subcommand `makemigrations` → **`generate`**.
- Module path → **`github.com/ocomsoft/morphic`** (GitHub repo to be renamed too).
- **No separate `upgrade` command.** Instead, keep a `makemigrations` subcommand as a
  legacy shim: it performs the one-time upgrade (rename config file + DB history table)
  and tells the user to switch to `generate` going forward.
- **README**: rework branding + **remove the "Legacy SQL Workflow" section**.
- `makemigrations_sql`: **docs cleanup only** — it is no longer a real subcommand
  (only referenced in `PRD.md` and the README legacy section). Drop the stale
  references; do **not** introduce a `generate_sql` command.

---

## Phase 1 — Module path & imports (mechanical, high volume)
1. `go.mod`: `module github.com/ocomsoft/makemigrations` → `.../morphic`.
2. Sweep every `.go` file: replace import path prefix
   `github.com/ocomsoft/makemigrations` → `github.com/ocomsoft/morphic`
   (~13 internal packages, all `cmd/`, `migrate/`, `internal/`).
3. **yaegi symbol maps** (`migrate/symbols/*.go`): update mangled keys like
   `_github_com_ocomsoft_makemigrations_migrate_Operation` and path strings to match
   the new module path — otherwise the in-process interpreter (`migrate` command)
   breaks. Easy-to-miss critical bit.
4. `go build ./...` to confirm the path sweep compiles before continuing.

## Phase 2 — CLI surface
5. `cmd/root.go`: `Use: "makemigrations"` → `"morphic"`; update the command list in
   `Long` help (list `generate`; mention `makemigrations` only as a deprecated alias).
6. `cmd/go_migrations.go`: rename the generate-migrations command
   `Use: "makemigrations"` → `"generate"` (rename file to `generate.go` for clarity).
   Update help/examples. The freed-up `makemigrations` name is reused by the legacy
   shim in Phase 5.
7. Sweep `makemigrations <subcmd>` usage strings in all `cmd/*.go` help/examples →
   `morphic <subcmd>`.

## Phase 3 — Config filename
8. `internal/config/config.go` `GetConfigPath()` default → `migrations/morphic.config.yaml`,
   **with backward-compat fallback**: if `morphic.config.yaml` is absent but
   `makemigrations.config.yaml` exists, use the old one. Update `config_test.go`.
9. Rename on-disk files: `migrations/makemigrations.config.yaml` and
   `example/migrations/makemigrations.config.yaml` → `morphic.config.yaml`.

## Phase 4 — DB history table
10. Rename `makemigrations_history` → `morphic_history` in `migrate/recorder.go`,
    `migrate/operations.go`, and every provider's `HistoryTableDDL()` (postgresql,
    mysql, sqlite, sqlserver, clickhouse, redshift, tidb, turso, vertica, ydb,
    auroradsql, starrocks) + provider tests.
11. Recorder: on startup, if `morphic_history` is missing but `makemigrations_history`
    exists, emit a clear "run `morphic makemigrations` to upgrade" message (no silent
    dual-write).

## Phase 5 — Legacy `makemigrations` shim (replaces the old `upgrade` command idea)
12. Add `cmd/makemigrations.go` (`Use: "makemigrations"`, registered via
    `rootCmd.AddCommand`) as a **deprecated alias / upgrader**. When run it:
    - Renames `migrations/makemigrations.config.yaml` → `morphic.config.yaml` if present.
    - Renames the DB table via a new provider method
      `RenameHistoryTableSQL(old, new)` (`ALTER TABLE makemigrations_history RENAME TO
      morphic_history`), guarded so it's idempotent (skip if new table already exists).
    - Prints a clear deprecation notice: the project has been upgraded, use
      `morphic generate` from now on.
    - Supports `--dry-run`; prints a summary of actions.
    - Marked `Deprecated`/`Hidden` as appropriate so it stays out of the primary help
      but still works for muscle memory.
13. Add `docs/commands/makemigrations.md` documenting it as the legacy upgrade shim
    (keep the existing filename; repoint content).

## Phase 6 — Build / release / tooling
14. `Makefile`: binary name `makemigrations` → `morphic` (BINARY_NAME, output paths,
    install target).
15. `.github/workflows/*.yml`: artifact names `makemigrations-<os>-<arch>` → `morphic-*`,
    release asset names, any binary refs.
16. `.idea/makemigrations.iml` → `morphic.iml` (+ `modules.xml` reference);
    `.bumpversion.cfg` path stays valid (points at `internal/version`).
17. `example/` and `debug/` references; `migrate/symbols/migrate.go` doc comments.

## Phase 7 — Claude skill / plugin
18. `.claude-plugin/plugin.json`: `name` `go-makemigrations` → `go-morphic`, update
    description.
19. `skills/SKILL.md` frontmatter `name` + body; `skills/references/README.md`;
    `docs/claude-code-skill.md`.

## Phase 8 — Docs & README
20. Rename `docs/commands/makemigrations.md` → `docs/commands/generate.md`; fix links.
21. Sweep all `docs/**/*.md`, `*.md`, `*.vhs` (demo scripts) for `makemigrations` →
    `morphic` (command/binary) and config/table names.
22. **README.md**: title, install/download URLs, command-reference table (rename
    `makemigrations`→`generate`; note `makemigrations` as the legacy upgrade shim), and
    **delete the "Legacy SQL Workflow" section** (~lines 309–319).
23. `PRD.md`: drop the stale `makemigrations_sql` references; rebrand remaining mentions.

## Phase 9 — Verify
24. `go build ./...`, `go test ./...`, `golangci-lint run` (per `.golangci.yml`).
25. Smoke-test the binary: `morphic --help`, `morphic generate --help`,
    `morphic makemigrations --dry-run` (legacy shim/upgrade).
26. Commit to `claude/youthful-sagan-nyhm1` and push.

---

## Notes / risks
- **Repo rename dependency**: imports won't resolve via `go get` until the GitHub repo
  is renamed to `ocomsoft/morphic`. Code builds locally regardless.
- **Legacy SQL / Goose**: only the README section is removed and stale doc references
  are cleaned up; legacy code (if any remains) is just rebranded, not deleted.
- Backward-compat preserved (config fallback + `upgrade` for the table) so existing
  users aren't hard-broken on update.

## Scope summary
- ~158 files reference `makemigrations` (~812 bare occurrences).
- Distinct meanings handled separately: module/import path, binary name, root command,
  `generate` subcommand, config filename, DB history table, skill/plugin name, IDE
  files, docs.
