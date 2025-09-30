# Building makemigrations

This document describes how to build, test, and release makemigrations.

## Prerequisites

- Go 1.21 or later
- Git
- Make (optional, but recommended)

## Quick Start

### Build for current platform
```bash
make build
```

### Run tests
```bash
make test
```

### Development workflow
```bash
make dev  # format, vet, lint, test, build
```

## Available Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build for current platform |
| `make release-build` | Build for all platforms |
| `make test` | Run tests |
| `make test-coverage` | Run tests with coverage report |
| `make clean` | Clean build artifacts |
| `make deps` | Get dependencies |
| `make deps-update` | Update dependencies |
| `make fmt` | Format code |
| `make lint` | Run linter |
| `make vet` | Run go vet |
| `make security` | Run security checks |
| `make install` | Install locally |
| `make dev` | Development workflow |
| `make ci` | CI workflow |
| `make run ARGS="--help"` | Build and run |
| `make version` | Show version info |

## Manual Build

### Single Platform
```bash
go build -ldflags="-s -w" -o makemigrations .
```

### Cross-Platform
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o makemigrations-linux-amd64 .

# macOS
GOOS=darwin GOARCH=amd64 go build -o makemigrations-darwin-amd64 .

# Windows
GOOS=windows GOARCH=amd64 go build -o makemigrations-windows-amd64.exe .
```

### With Version Information
```bash
VERSION=$(git describe --tags --always --dirty)
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(git rev-parse --short HEAD)

go build \
  -ldflags="-s -w \
    -X github.com/ocomsoft/makemigrations/internal/version.Version=$VERSION \
    -X github.com/ocomsoft/makemigrations/internal/version.BuildDate=$BUILD_DATE \
    -X github.com/ocomsoft/makemigrations/internal/version.GitCommit=$GIT_COMMIT" \
  -o makemigrations .
```

## Testing

### Run All Tests
```bash
go test -v ./...
```

### With Coverage
```bash
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Specific Package
```bash
go test -v ./cmd
go test -v ./internal/yaml
```

## Code Quality

### Format Code
```bash
go fmt ./...
```

### Lint
```bash
# Install golangci-lint if not already installed
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

### Vet
```bash
go vet ./...
```

### Security Checks
```bash
# Install security tools
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Run checks
govulncheck ./...
gosec ./...
```

## Release Process

### Automated Release (Recommended)

1. **Create and push a tag:**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GitHub Actions will automatically:**
   - Run tests
   - Build for all platforms
   - Generate checksums
   - Create a GitHub release
   - Upload binaries

### Manual Release

1. **Update version and create tag:**
   ```bash
   # Update version in internal/version/version.go if needed
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **Build release binaries:**
   ```bash
   make release-build
   ```

3. **Create release on GitHub:**
   - Go to GitHub releases page
   - Create new release
   - Upload files from `dist/` directory
   - Include `checksums.txt`

## Supported Platforms

The build system supports the following platforms:

| OS | Architecture | Binary Name |
|----|--------------|-------------|
| Linux | amd64 | `makemigrations-linux-amd64` |
| Linux | arm64 | `makemigrations-linux-arm64` |
| macOS | amd64 (Intel) | `makemigrations-darwin-amd64` |
| macOS | arm64 (Apple Silicon) | `makemigrations-darwin-arm64` |
| Windows | amd64 | `makemigrations-windows-amd64.exe` |
| Windows | arm64 | `makemigrations-windows-arm64.exe` |

## GitHub Actions Workflows

### Build Workflow (`.github/workflows/build.yml`)
- Runs on every push to main/develop
- Runs tests and builds for all platforms
- Includes linting and security checks

### Release Workflow (`.github/workflows/release.yml`)
- Triggers on tag push (v*)
- Builds for all platforms
- Creates GitHub release with binaries
- Generates checksums and release notes

### Security Workflow (`.github/workflows/security.yml`)
- Runs weekly and on pushes
- Vulnerability scanning with govulncheck
- Security analysis with gosec

### Dependency Update (`.github/workflows/dependency-update.yml`)
- Runs weekly
- Updates Go dependencies
- Creates PR with changes

## Version Information

Version information is embedded at build time using Go's ldflags:

- `Version`: Git tag or commit hash
- `BuildDate`: Build timestamp (ISO 8601)
- `GitCommit`: Short git commit hash

Check version:
```bash
./makemigrations version
./makemigrations version --build-info
./makemigrations version --format json
```

## Troubleshooting

### Build Issues

**Module not found:**
```bash
go mod download
go mod tidy
```

**Build fails on older Go versions:**
- Ensure Go 1.21+ is installed
- Check `go.mod` for minimum version

**Cross-compilation issues:**
```bash
# Clear module cache
go clean -modcache

# Rebuild
make clean
make build
```

### Test Issues

**Tests fail:**
```bash
# Run with verbose output
go test -v ./...

# Run specific test
go test -v ./cmd -run TestSpecificFunction
```

**Coverage reports:**
```bash
make test-coverage
# Open coverage.html in browser
```

## Contributing

1. Fork the repository
2. Create feature branch
3. Run tests: `make test`
4. Run CI checks: `make ci`
5. Submit pull request

The CI workflow will automatically run tests and builds for all platforms.