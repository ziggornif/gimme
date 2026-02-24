# CLAUDE.md - Gimme Project

## Project Overview

**Gimme** is a self-hosted CDN (Content Delivery Network) solution written in Go. It allows uploading packages (ZIP archives) and serving static assets (JS, CSS, images, etc.) via a REST API, backed by any S3-compatible object storage — primarily [Garage HQ](https://garagehq.deuxfleurs.fr/) or [Minio](https://min.io/) via the Minio Go SDK.

- **Module**: `github.com/gimme-cdn/gimme`
- **Go version**: 1.26+
- **Docker image**: `ziggornif/gimme`

---

## Architecture

```
gimme/
├── cmd/server/main.go          # Entrypoint
├── api/                        # HTTP controllers (Gin)
│   ├── root.go                 # GET /
│   ├── auth-controller.go      # POST /create-token
│   ├── health-controller.go    # GET /healthz, GET /readyz
│   └── package-controller.go   # GET|POST|DELETE /packages, GET /gimme/...
├── internal/
│   ├── application/            # App bootstrap (config, modules, HTTP server)
│   ├── auth/                   # JWT token management + Gin middleware
│   ├── content/                # Business logic: create/get/delete packages
│   ├── storage/                # S3 client and manager (Minio SDK)
│   ├── archive_validator/      # ZIP file validation
│   └── errors/                 # Custom GimmeError type
├── configs/                    # Config loading via Viper (gimme.yml)
├── pkg/
│   └── file-utils/             # Utility: file content-type detection
├── templates/                  # HTML templates (Gin, .tmpl)
├── docs/                       # Static docs (swagger.json) served at /docs
└── examples/                   # Deployment examples (Docker Compose, K8s, monitoring)
    ├── deployment/
    │   ├── docker-compose/
    │   │   ├── with-garage/        # Self-provisioning stack with Garage HQ
    │   │   ├── with-local-s3/      # Local dev stack with Minio
    │   │   └── with-managed-s3/    # External/managed S3 provider
    │   ├── kubernetes/             # Namespace, Deployment, Service, Ingress
    │   └── systemd/                # Linux systemd unit
    └── monitoring/                 # Prometheus config + Grafana dashboard
```

### Key Data Flow

1. **Upload**: `POST /packages` (Bearer JWT) → `archive_validator` → `content.CreatePackage` → unzip → `storage.AddObject` (S3, parallel goroutines via `errgroup`)
2. **Serve**: `GET /gimme/<package>@<version>/<file>` → `content.GetFile` → `storage.GetObject` → stream response
3. **Auth**: `POST /create-token` (Basic Auth admin) → `auth.CreateToken` → signed JWT (HS256)
4. **Health**: `GET /healthz` → liveness (process alive) / `GET /readyz` → readiness (S3 bucket reachable)

### Package Naming Convention

Objects are stored in S3 as `<package>@<version>/<file>`, e.g., `awesome-lib@1.0.0/awesome-lib.min.js`.

Semver partial versions are supported (e.g., `awesome-lib@1.0` resolves to the latest `1.0.x`).

---

## Development Commands

```bash
# Build (Linux amd64 binary + dist/)
make build

# Run tests with coverage
make test

# View coverage report (text) — requires make test first
make coverage

# Run tests and open HTML coverage report
make test_coverage_html

# Live reload (requires air)
make watch

# Security audit (requires gosec)
make audit
```

### Running Locally

Requires a running S3-compatible backend (Garage or Minio) and a `gimme.yml` config file.

```bash
# Option 1 — Garage (recommended, auto-provisioned)
cd examples/deployment/docker-compose/with-garage
docker compose up -d
# Gimme available at http://localhost:8080

# Option 2 — Minio (manual bucket + key setup)
cd examples/deployment/docker-compose/with-local-s3
docker compose up -d

# Or run from source with live reload
cp gimme.example.yml gimme.yml
# Edit gimme.yml then:
make watch
```

---

## Configuration

Config is read from `gimme.yml` (local dir or `/config/` for Docker) via **Viper**. Environment variables override file values automatically.

| Key               | Description                              | Default  |
|-------------------|------------------------------------------|----------|
| `secret`          | JWT signing secret                       | required |
| `admin.user`      | Basic auth admin username                | required |
| `admin.password`  | Basic auth admin password                | required |
| `port`            | HTTP server port                         | `8080`   |
| `s3.url`          | S3 / Garage endpoint URL                 | required |
| `s3.key`          | S3 access key                            | required |
| `s3.secret`       | S3 secret key                            | required |
| `s3.bucketName`   | S3 bucket name                           | `gimme`  |
| `s3.location`     | S3 region / Garage zone                  | required |
| `s3.ssl`          | Enable TLS for S3 connection             | `true`   |
| `metrics`         | Enable `/metrics` OpenMetrics endpoint   | `true`   |

---

## API Routes

| Method   | Route                        | Auth          | Description                          |
|----------|------------------------------|---------------|--------------------------------------|
| `GET`    | `/`                          | None          | HTML homepage                        |
| `POST`   | `/create-token`              | Basic Auth    | Create JWT access token              |
| `POST`   | `/packages`                  | Bearer JWT    | Upload a ZIP package                 |
| `DELETE` | `/packages/:package`         | Bearer JWT    | Delete a package (`name@version`)    |
| `GET`    | `/gimme/:package`            | None          | List files in a package (HTML)       |
| `GET`    | `/gimme/:package/*file`      | None          | Serve a specific file from a package |
| `GET`    | `/metrics`                   | None          | OpenMetrics/Prometheus endpoint      |
| `GET`    | `/docs`                      | None          | Static API documentation (ReDoc)     |
| `GET`    | `/healthz`                   | None          | Liveness probe                       |
| `GET`    | `/readyz`                    | None          | Readiness probe (checks S3 bucket)   |

---

## Error Handling

Custom error type `GimmeError` (`internal/errors/business-error.go`) implements the `error` interface and maps domain errors to HTTP codes:

| Kind            | HTTP Code |
|-----------------|-----------|
| `BadRequest`    | 400       |
| `Unauthorized`  | 401       |
| `Conflict`      | 409       |
| `InternalError` | 500       |
| `NotImplemented`| 501       |

---

## Testing

Tests use `github.com/stretchr/testify`. Each package has a `_test.go` file alongside the source.

Unit tests run standalone. Integration tests (tagged `//go:build integration`) require a running S3 backend — see `.github/workflows/build.yml` for the CI setup using Garage.

```bash
# Run unit tests only
go test ./... -coverprofile=coverage.out

# Run integration tests
go test -tags integration ./...

# Or via Makefile
make test
```

---

## CI/CD

- **`.github/workflows/build.yml`**: Runs on all branch pushes/PRs. Starts a Garage instance, runs tests, builds and pushes a Docker image tagged with the branch name (and `latest` for `main`).
- **`.github/workflows/release.yml`**: Triggered on GitHub release. Builds binaries for Linux/Windows/macOS (amd64, arm64, 386) and publishes a tagged Docker image.

Required GitHub secrets: `DOCKER_REPO`, `DOCKER_USER`, `DOCKER_PASS`.

---

## Key Dependencies

| Package                         | Version   | Role                                      |
|---------------------------------|-----------|-------------------------------------------|
| `gin-gonic/gin`                 | v1.11.0   | HTTP framework                            |
| `gin-contrib/cors`              | v1.7.6    | CORS middleware                           |
| `golang-jwt/jwt/v4`             | v4.5.2    | JWT token creation and validation         |
| `minio/minio-go/v7`             | v7.0.98   | S3-compatible object storage client       |
| `spf13/viper`                   | v1.21.0   | Configuration management                  |
| `sirupsen/logrus`               | v1.9.4    | Structured logging                        |
| `prometheus/client_golang`      | v1.23.2   | OpenMetrics/Prometheus metrics            |
| `stretchr/testify`              | v1.11.1   | Test assertions                           |
| `golang.org/x/mod/semver`       | v0.33.0   | Semver parsing and sorting                |
| `golang.org/x/sync/errgroup`    | v0.19.0   | Goroutine error propagation               |

---

## Code Conventions

- **Go package naming**: lowercase; use underscores only when necessary (e.g., `archive_validator`)
- **File naming**: lowercase, hyphenated (e.g., `auth-manager.go`, `content-service.go`, `file-utils/`)
- **Error handling**: always use `*errors.GimmeError` for domain errors; log with `logrus` before returning
- **Error interface**: `GimmeError` implements `Error() string` (standard `error` interface)
- **Interfaces**: defined in `internal/storage` (`ObjectStorageManager`, `ObjectStorageClient`) to allow mocking in tests
- **Concurrency**: ZIP file extraction uses goroutines + `errgroup.Group` in `content.CreatePackage` — errors are propagated
- **Context propagation**: HTTP context is passed down to all storage calls (no `context.Background()` in business logic)
- **Defer**: always defer `Close()` on opened resources (files, S3 objects)
- **Logging prefix**: `[PackageName] FunctionName - message` (e.g., `[AuthManager] CreateToken - ...`)
- **Standard library first**: prefer `slices.Contains` (Go 1.21+) over custom array helpers

---

## Agent Instructions

### One task at a time

When working on this project, **pick exactly one task from `TODO.md` and complete it fully** before moving on to the next. Do not batch multiple tasks in a single session.

### Quality gate — mandatory before every commit

After implementing a task, the following checks **must all pass** before proposing a commit:

```bash
# 1. Format
gofmt -l ./...         # must output nothing (no unformatted files)

# 2. Lint (requires golangci-lint)
golangci-lint run ./...

# 3. Tests
make test              # go test ./... -coverprofile=coverage.out

# 4. Build
make build
```

If any check fails, fix the issues before proceeding.

### Code review

Once the implementation is complete and all quality checks pass, **invoke the `code-reviewer` sub-agent** to review the changes. Only propose a commit after the review has been acknowledged.

### Commit

Propose the commit message but **never commit automatically**. Let the user decide when to commit.
