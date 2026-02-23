# CLAUDE.md - Gimme Project

## Project Overview

**Gimme** is a self-hosted CDN (Content Delivery Network) solution written in Go. It allows uploading packages (ZIP archives) and serving static assets (JS, CSS, images, etc.) via a REST API, backed by an S3-compatible object storage (Minio SDK).

- **Module**: `github.com/gimme-cdn/gimme`
- **Go version**: 1.18+
- **Docker image**: `ziggornif/gimme`

---

## Architecture

```
gimme/
├── cmd/server/main.go          # Entrypoint
├── api/                        # HTTP controllers (Gin)
│   ├── root.go                 # GET /
│   ├── auth-controller.go      # POST /create-token
│   └── package-controller.go   # GET|POST|DELETE /packages, GET /gimme/...
├── internal/
│   ├── application/            # App bootstrap (config, modules, HTTP server)
│   ├── auth/                   # JWT token management + Gin middleware
│   ├── content/                # Business logic: create/get/delete packages
│   ├── storage/                # Minio S3 client and manager
│   ├── archive_validator/      # ZIP file validation
│   └── errors/                 # Custom GimmeError type
├── configs/                    # Config loading via Viper (gimme.yml)
├── pkg/
│   ├── array/                  # Utility: array helpers
│   └── file-utils/             # Utility: file content-type detection
├── templates/                  # HTML templates (Gin, .tmpl)
├── docs/                       # Static docs served at /docs
└── examples/                   # Deployment examples (Docker, K8s, monitoring)
```

### Key Data Flow

1. **Upload**: `POST /packages` (Bearer JWT) → `archive_validator` → `content.CreatePackage` → unzip → `storage.AddObject` (Minio)
2. **Serve**: `GET /gimme/<package>@<version>/<file>` → `content.GetFile` → `storage.GetObject` → stream response
3. **Auth**: `POST /create-token` (Basic Auth admin) → `auth.CreateToken` → signed JWT (HS256)

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

# View coverage report (text)
make coverage

# View coverage report (HTML)
make test_coverage_html

# Live reload (requires air)
make watch

# Security audit (requires gosec)
make audit
```

### Running Locally

Requires a running Minio instance and a `gimme.yml` config file.

```bash
# Start local Minio
docker run -p 9000:9000 -p 9001:9001 minio/minio server /data --console-address ":9001"

# Run with live reload
make watch

# Or build and run directly
make build && ./gimme
```

---

## Configuration

Config is read from `gimme.yml` (local dir or `/config/` for Docker) via **Viper**. Environment variables are supported automatically.

| Key               | Description                          | Default  |
|-------------------|--------------------------------------|----------|
| `secret`          | JWT signing secret                   | required |
| `admin.user`      | Basic auth admin username            | required |
| `admin.password`  | Basic auth admin password            | required |
| `port`            | HTTP server port                     | `8080`   |
| `s3.url`          | S3/Minio endpoint URL                | required |
| `s3.key`          | S3 access key                        | required |
| `s3.secret`       | S3 secret key                        | required |
| `s3.bucketName`   | S3 bucket name                       | `gimme`  |
| `s3.location`     | S3 region/location                   | required |
| `s3.ssl`          | Enable SSL for S3 connection         | `true`   |
| `metrics`         | Enable `/metrics` OpenMetrics endpoint | `true` |

---

## API Routes

| Method   | Route                        | Auth          | Description                          |
|----------|------------------------------|---------------|--------------------------------------|
| `GET`    | `/`                          | None          | HTML homepage                        |
| `POST`   | `/create-token`              | Basic Auth    | Create JWT access token              |
| `POST`   | `/packages`                  | Bearer JWT    | Upload a ZIP package                 |
| `DELETE` | `/packages/:package`         | Bearer JWT    | Delete a package (`name@version`)    |
| `GET`    | `/gimme/:package`            | None          | List files in a package              |
| `GET`    | `/gimme/:package/*file`      | None          | Serve a specific file from a package |
| `GET`    | `/metrics`                   | None          | OpenMetrics/Prometheus endpoint      |
| `GET`    | `/docs`                      | None          | Static API documentation             |

---

## Error Handling

Custom error type `GimmeError` (`internal/errors/business-error.go`) wraps business errors with a kind and HTTP code mapping:

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

CI tests require a running Minio instance (see `.github/workflows/build.yml` — Docker Minio is started before tests).

```bash
# Run all tests
go test ./... -coverprofile=coverage.out

# Or via Makefile
make test
```

---

## CI/CD

- **`.github/workflows/build.yml`**: Runs on push/PR to `main`. Starts Minio, runs tests, builds and pushes Docker image (`latest`).
- **`.github/workflows/release.yml`**: Triggered on GitHub release. Builds binaries for Linux/Windows/macOS (amd64, arm64, 386) and publishes a tagged Docker image.

Required GitHub secrets: `DOCKER_REPO`, `DOCKER_USER`, `DOCKER_PASS`.

---

## Key Dependencies

| Package                         | Role                                      |
|---------------------------------|-------------------------------------------|
| `gin-gonic/gin`                 | HTTP framework                            |
| `gin-contrib/cors`              | CORS middleware                           |
| `golang-jwt/jwt/v4`             | JWT token creation and validation         |
| `minio/minio-go/v7`             | S3-compatible object storage client       |
| `spf13/viper`                   | Configuration management                  |
| `sirupsen/logrus`               | Structured logging                        |
| `prometheus/client_golang`      | OpenMetrics/Prometheus metrics            |
| `stretchr/testify`              | Test assertions                           |
| `golang.org/x/mod/semver`       | Semver parsing and sorting                |

---

## Code Conventions

- **Package naming**: lowercase, hyphenated (e.g., `archive_validator`, `file-utils`)
- **File naming**: lowercase, hyphenated (e.g., `auth-manager.go`, `content-service.go`)
- **Error handling**: always use `*errors.GimmeError` for domain errors; log with `logrus` before returning
- **Interfaces**: defined in `internal/storage` (`ObjectStorageManager`, `ObjectStorageClient`) to allow mocking in tests
- **Concurrency**: ZIP file extraction uses goroutines + `sync.WaitGroup` in `content.CreatePackage`
- **Defer**: always defer `Close()` on opened resources (files, Minio objects)
- **Logging prefix**: `[PackageName] FunctionName - message` (e.g., `[AuthManager] CreateToken - ...`)

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
