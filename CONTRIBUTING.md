# Contributing to Gimme

Thank you for your interest in contributing! This guide covers everything you need to get started.

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Go](https://go.dev/dl/) | 1.26+ | Build and test |
| [Docker](https://docs.docker.com/get-docker/) | any recent | Integration tests (Garage S3) |
| [golangci-lint](https://golangci-lint.run/usage/install/) | latest | Linting |
| [gosec](https://github.com/securego/gosec#install) | latest | Security audit |
| [Helm](https://helm.sh/docs/intro/install/) | 3.x | Helm chart tests (optional) |

## Getting Started

```bash
# Clone the repository
git clone https://github.com/gimme-cdn/gimme.git
cd gimme

# Copy the example config
cp gimme.example.yml gimme.yml
# Edit gimme.yml with your local S3/Garage settings

# Download dependencies
go mod download

# Run unit tests (no S3 required)
go test $(go list ./... | grep -v 'github.com/gimme-cdn/gimme/api') -coverprofile=coverage.out

# Run all tests including integration (starts Garage via Docker automatically)
make test

# Build the binary
make build

# Live reload during development (requires air: go install github.com/air-verse/air@latest)
make watch
```

## Project Layout

```
gimme/
├── cmd/server/main.go          # Entrypoint
├── api/                        # HTTP controllers (Gin) + integration tests
├── internal/
│   ├── application/            # App bootstrap
│   ├── auth/                   # JWT + token stores
│   ├── content/                # Business logic
│   ├── persistence/            # Redis / PostgreSQL clients
│   ├── storage/                # S3 client (Minio SDK)
│   ├── archive_validator/      # ZIP validation
│   └── errors/                 # GimmeError type
├── configs/                    # Config loading (Viper)
├── pkg/file-utils/             # MIME type detection
├── templates/                  # HTML templates (.tmpl)
├── scripts/helm/gimme/         # Helm chart
└── docs/                       # Documentation site + Swagger
```

## Development Workflow

1. **Fork** the repository and create a branch from `main`:
   ```bash
   git checkout -b feat/my-feature
   # or
   git checkout -b fix/my-bugfix
   ```

2. **Make your changes**, following the [code conventions](#code-conventions) below.

3. **Run the quality gate** before pushing — all checks must pass:
   ```bash
   gofmt -l .              # must output nothing
   make fmt                # auto-format
   make lint               # golangci-lint
   make test               # unit + integration tests
   make build              # binary build
   ```

4. **Open a Pull Request** against `main` with a clear description of what and why.

## Code Conventions

- **Package names**: lowercase, no hyphens (e.g. `archive_validator`)
- **File names**: lowercase, hyphenated (e.g. `auth-manager.go`)
- **Errors**: always use `*errors.GimmeError` for domain errors; log with `logrus` before returning
- **Logging prefix**: `[PackageName] FunctionName - message`
- **Interfaces**: defined in the relevant `internal/` package to allow mocking in tests
- **Context**: pass `ctx` down to all storage/DB calls — no `context.Background()` in business logic
- **Resources**: always `defer Close()` on opened files and S3 objects
- **Standard library first**: prefer `slices.Contains` (Go 1.21+) over custom helpers

## Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
feat: add Redis cache support
fix: handle missing bucket gracefully
chore: update dependencies
refactor: extract persistence client
docs: update API reference
test: add coverage for token revocation
```

## Testing

- Each package has a `_test.go` file alongside the source.
- **Unit tests** use mocks (see `test/mocks/`) and require no external dependencies.
- **Integration tests** (in `api/`) require a live S3 backend — `make test` handles this automatically with Garage running in Docker.
- New features and bug fixes should include tests.

## Helm Chart

If your change affects the Helm chart (`scripts/helm/gimme/`):

```bash
make helm-lint   # lint the chart
make helm-test   # run unit tests
```

## Reporting Issues

Please open an issue on [GitHub](https://github.com/gimme-cdn/gimme/issues) with:
- A clear description of the problem
- Steps to reproduce
- Expected vs actual behaviour
- Your environment (OS, Go version, deployment type)
