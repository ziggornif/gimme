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
│   ├── admin-controller.go     # GET /admin, POST|DELETE /tokens
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
└── docs/                       # Static docs (swagger.json) served at /docs
```

### Key Data Flow

1. **Upload**: `POST /packages` (Bearer JWT) → `archive_validator` → `content.CreatePackage` → unzip → `storage.AddObject` (S3, parallel goroutines via `errgroup`)
2. **Serve**: `GET /gimme/<package>@<version>/<file>` → `content.GetFile` → `storage.GetObject` → stream response
3. **Auth**: `POST /tokens` (admin auth via `authProvider`) → `auth.CreateToken` → signed JWT (HS256)
4. **Health**: `GET /healthz` → liveness (process alive) / `GET /readyz` → readiness (S3 bucket reachable)

### Package Naming Convention

Objects are stored in S3 as `<package>@<version>/<file>`, e.g., `awesome-lib@1.0.0/awesome-lib.min.js`.

Semver partial versions are supported (e.g., `awesome-lib@1.0` resolves to the latest `1.0.x`).

---

## Development Commands

```bash
make build              # Build Linux amd64 binary + dist/
make test               # Start Garage, run all tests (unit + integration), stop Garage
make coverage           # View coverage report (requires make test first)
make watch              # Live reload (requires air)
make audit              # Security audit (requires gosec)
```

---

## Configuration

Config is read from `gimme.yml` (local dir or `/config/` for Docker) via **Viper**. Environment variables override file values automatically.

| Key                   | Description                                             | Default  |
|-----------------------|---------------------------------------------------------|----------|
| `secret`              | JWT signing secret                                      | required |
| `admin.user`          | Basic auth admin username                               | required |
| `admin.password`      | Basic auth admin password                               | required |
| `port`                | HTTP server port                                        | `8080`   |
| `s3.url`              | S3 / Garage endpoint URL                                | required |
| `s3.key`              | S3 access key                                           | required |
| `s3.secret`           | S3 secret key                                           | required |
| `s3.bucketName`       | S3 bucket name                                          | `gimme`  |
| `s3.location`         | S3 region / Garage zone                                 | required |
| `s3.ssl`              | Enable TLS for S3 connection                            | `true`   |
| `metrics`             | Enable `/metrics` OpenMetrics endpoint                  | `true`   |
| `tokenStore.mode`     | Token persistence backend (`file`, `redis`, `postgres`) | `file`   |
| `tokenStore.pg_url`   | PostgreSQL URL (required when mode is `postgres`)       | `""`     |

---

## API Routes

| Method   | Route                        | Auth          | Description                          |
|----------|------------------------------|---------------|--------------------------------------|
| `GET`    | `/`                          | None          | HTML homepage                        |
| `GET`    | `/admin`                     | Admin auth    | Admin UI (token management)          |
| `POST`   | `/tokens`                    | Admin auth    | Create JWT access token              |
| `DELETE` | `/tokens/:id`                | Admin auth    | Revoke an access token               |
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

Integration tests (in `api/`) run unconditionally and require a live S3 backend — `make test` handles starting/stopping Garage automatically.

Unit tests only (no S3 required):
```bash
go test $(go list ./... | grep -v 'github.com/gimme-cdn/gimme/api') -coverprofile=coverage.out
```

---

## Key Dependencies

| Package                         | Role                                      |
|---------------------------------|-------------------------------------------|
| `gin-gonic/gin`                 | HTTP framework                            |
| `golang-jwt/jwt/v4`             | JWT token creation and validation         |
| `minio/minio-go/v7`             | S3-compatible object storage client       |
| `spf13/viper`                   | Configuration management                  |
| `sirupsen/logrus`               | Structured logging                        |
| `prometheus/client_golang`      | OpenMetrics/Prometheus metrics            |
| `stretchr/testify`              | Test assertions                           |
| `golang.org/x/mod/semver`       | Semver parsing and sorting                |
| `golang.org/x/sync/errgroup`    | Goroutine error propagation               |
| `redis/go-redis/v9`             | Redis client                              |
| `jackc/pgx/v5`                  | PostgreSQL driver and connection pool     |

---

## Code Conventions

- **Go package naming**: lowercase; use underscores only when necessary (e.g., `archive_validator`)
- **File naming**: lowercase, hyphenated (e.g., `auth-manager.go`, `content-service.go`, `file-utils/`)
- **Error handling**: always use `*errors.GimmeError` for domain errors; log with `logrus` before returning
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
gofmt -l .              # must output nothing (no unformatted files)
golangci-lint run ./... # lint
make test               # unit + integration tests
make build              # build
```

If any check fails, fix the issues before proceeding.

### Code review

Once the implementation is complete and all quality checks pass, **invoke the `code-reviewer` sub-agent** to review the changes. Only propose a commit after the review has been acknowledged.

### Commit

Propose the commit message but **never commit automatically**. Let the user decide when to commit.
