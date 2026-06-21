# version Command

The `version` command displays version information for makemigrations, including the current version number, build date, git commit, and platform details.

## Usage

```
makemigrations version [flags]
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format`, `-f` | string | `text` | Output format: `text` or `json` |
| `--build-info`, `-b` | bool | `false` | Show detailed build information |

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | `migrations/makemigrations.config.yaml` | Path to the configuration file |

---

## Examples

### Show version

```bash
makemigrations version

# Output:
# makemigrations v1.4.2
```

### Show detailed build info

```bash
makemigrations version --build-info

# Output:
# makemigrations v1.4.2
# Build Date: 2026-03-20
# Git Commit: 0a556ef
# Go Version: go1.24
# Platform: linux/amd64
```

### JSON output (useful for CI/CD)

```bash
makemigrations version --format json

# Output:
# {"version":"1.4.2","build_date":"2026-03-20","git_commit":"0a556ef","go_version":"go1.24","platform":"linux/amd64"}
```

---

## See Also

- [init command](./init.md) — Initialise the migrations directory
- [makemigrations command](./makemigrations.md) — Generate migrations from YAML schema changes
