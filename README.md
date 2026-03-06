# gimme

<p align="center">
  <img src="assets/gimme.png" width="120" alt="Gimme CDN logo" />
</p>

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ziggornif/gimme)](https://goreportcard.com/report/github.com/ziggornif/gimme)

**A self-hosted CDN solution written in Go.**

Upload ZIP packages and serve static assets (JS, CSS, images, …) via a simple REST API, backed by **any S3-compatible object storage** — AWS S3, Google Cloud Storage, OVH Object Storage, Scaleway Object Storage, Clever Cloud Cellar, [Garage](https://garagehq.deuxfleurs.fr/), [Minio](https://min.io/), and more.

> **💡 A caching layer in front of gimme (e.g. Nginx, Varnish, Cloudflare) is strongly recommended for production use.**

---

## Table of Contents

- [Documentation](#documentation)
- [Architecture](#architecture)
  - [Components](#components)
  - [Upload flow](#upload-flow)
  - [Serve flow](#serve-flow)
- [Quick Start](#quick-start)
  - [With a managed S3 provider](#with-a-managed-s3-provider-aws-r2-scaleway-)
  - [With Garage (self-hosted S3)](#with-garage-self-hosted-s3)
  - [From source](#from-source)
- [Configuration](#configuration)
- [API Usage](#api-usage)
- [Deployment Examples](#deployment-examples)
- [Caching Strategy](#caching-strategy)
  - [Level 1 — HTTP Cache-Control headers](#level-1--http-cache-control-headers-zero-dependency)
  - [Level 2 — Internal Redis cache](#level-2--internal-redis-cache-optional)
- [Monitoring](#monitoring)

---

## Documentation

The Gimme documentation is available at https://ziggornif.github.io/gimme/.

## Architecture

### Components

```mermaid
graph LR
    Client["Client\n(browser / curl)"]
    Gimme["gimme\n:8080"]
    S3["Any S3-compatible storage\n(AWS S3, OVH, Cellar, Garage, …)"]
    Cache["Cache layer\n(Nginx / CDN) — optional"]
    Redis["Redis / Valkey\n(optional — tokens + cache)"]

    Client -->|"GET /gimme/pkg@1.0/file.js"| Cache
    Cache -->|cache miss| Gimme
    Gimme -.->|"version resolution (partial)"| Redis
    Gimme -->|stream object| S3
    Client -->|"POST /packages (Bearer token)"| Gimme
    Gimme -->|store objects| S3
```

### Upload flow

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant API as gimme API
    participant Val as Archive Validator
    participant S3 as Object Storage

    Dev->>API: POST /packages (Bearer token, multipart ZIP)
    API->>Val: Validate ZIP (application/zip or application/octet-stream)
    Val-->>API: OK
    API->>S3: PutObject pkg@version/file (parallel goroutines)
    S3-->>API: 200 OK
    API-->>Dev: 201 Created
```

### Serve flow

```mermaid
sequenceDiagram
    participant Browser
    participant API as gimme API
    participant S3 as Object Storage

    Browser->>API: GET /gimme/awesome-lib@1.0.0/awesome-lib.min.js
    Note over API: Semver partial match: 1.0 → latest 1.0.x
    API->>S3: GetObject awesome-lib@1.0.3/awesome-lib.min.js
    S3-->>API: stream
    API-->>Browser: 200 OK (Content-Type: application/javascript)
```

---

## Quick Start

### With a managed S3 provider (AWS, R2, Scaleway, …)

Gimme works with any S3-compatible provider. Just fill in `gimme.yml` with your credentials:

```yaml
s3:
  url: s3.amazonaws.com          # or s3.fr-par.scw.cloud, s3.gra.io.cloud.ovh.net, cellar-c2.services.clever-cloud.com, …
  key: your-access-key
  secret: your-secret-key
  bucketName: gimme
  location: eu-west-1            # region as defined by your provider
  ssl: true
```

> See [with-managed-s3/](examples/deployment/docker-compose/with-managed-s3/) for a ready-to-use Docker Compose example with monitoring included.

### With Garage (self-hosted S3)

If you also want to self-host the object storage, [Garage](https://garagehq.deuxfleurs.fr/) is a lightweight S3-compatible store that runs alongside gimme. The stack provisions itself automatically — no manual setup needed.

```bash
cd examples/deployment/docker-compose/with-garage
docker compose up -d
```

Gimme will be available at <http://localhost:8080>.  
The `init-garage` service creates the bucket and writes the config automatically.

> See [with-garage/README.md](examples/deployment/docker-compose/with-garage/README.md) for configuration details.

### From source

Requires Go 1.26+ and a running S3-compatible backend.

```bash
cp gimme.example.yml gimme.yml
# Edit gimme.yml with your S3 credentials
make build && ./gimme
```

> `make build` compiles a native binary for your current OS/architecture. Use `make release` to produce a Linux/amd64 binary with `upx` compression (used by Docker and CI).

---

## Configuration

Configuration is read from `gimme.yml` (local directory or `/config/gimme.yml` in Docker).  
Environment variables override file values automatically (via [Viper](https://github.com/spf13/viper)).

```yaml
admin:
  user: gimmeadmin
  password: gimmeadmin
port: 8080
secret: your-secret-at-least-32-chars-long
s3:
  url: your.s3.endpoint
  key: your-access-key
  secret: your-secret-key
  bucketName: gimme
  location: garage          # use region name matching your backend
  ssl: false                # default is true; set false for local backends (Garage, dev)
# metrics: true             # optional — expose /metrics (Prometheus), defaults to true

# Token store: "file" (default, no external dependency), "redis", or "postgres"
# tokenStore:
#   mode: file              # "file" | "redis" | "postgres"
#   pg_url: postgres://gimme:password@localhost:5432/gimme?sslmode=disable

# Redis — required when tokenStore.mode is "redis" or cache.enabled is true
# redis_url: redis://localhost:6379

# cache:
#   enabled: false          # optional version-resolution cache
#   type: redis
#   ttl: 3600
#   file_path: /tmp/gimme-tokens.enc  # used only when tokenStore.mode is "file"
```

| Key               | Description                              | Default  |
|-------------------|------------------------------------------|----------|
| `secret`          | Token signing secret (**min 32 chars**)  | required |
| `admin.user`      | Admin username (Basic Auth)              | required |
| `admin.password`  | Admin password (Basic Auth)              | required |
| `port`            | HTTP server port                         | `8080`   |
| `s3.url`          | S3 / Garage endpoint URL                 | required |
| `s3.key`          | S3 access key                            | required |
| `s3.secret`       | S3 secret key                            | required |
| `s3.bucketName`   | Bucket name                              | `gimme`  |
| `s3.location`     | S3 region / Garage zone                  | required |
| `s3.ssl`          | Enable TLS for S3 connection             | `true`   |
| `metrics`         | Enable `/metrics` OpenMetrics endpoint   | `true`   |
| `cors.allowed_origins` | List of allowed CORS origins. Defaults to all origins (`*`) if empty. | `[]` (all origins) |
| `tokenStore.mode` | Token persistence backend. `file` stores tokens in an encrypted local file (no external dependency). `redis` stores tokens in Redis (requires `redis_url`). `postgres` stores tokens in PostgreSQL (requires `tokenStore.pg_url`). | `file` |
| `tokenStore.pg_url` | PostgreSQL connection URL. Required when `tokenStore.mode` is `postgres`. | `""` |
| `cache.enabled`   | Enable internal Redis cache for version resolution | `false`  |
| `cache.type`      | Cache backend (`redis`)                  | `redis`  |
| `cache.ttl`       | Cache entry TTL in seconds               | `3600`   |
| `cache.file_path` | Path to the encrypted token file (used when `tokenStore.mode` is `file`) | `/tmp/gimme-tokens.enc` |
| `redis_url`       | Redis/Valkey connection URL. Required when `tokenStore.mode` is `redis` or `cache.enabled` is `true`. | `""` |
| `auth.mode`       | Admin auth mode (`basic` or `oidc`)      | `basic`  |
| `auth.oidc.issuer`       | OIDC issuer URL                 | required if `oidc` |
| `auth.oidc.client_id`    | OIDC client ID                  | required if `oidc` |
| `auth.oidc.client_secret`| OIDC client secret              | optional |
| `auth.oidc.redirect_url` | OIDC redirect URI               | required if `oidc` |
| `auth.oidc.secure_cookies` | Use `Secure` flag on session cookies (disable only for local HTTP dev) | `true` |

> **Token store mode.** By default (`tokenStore.mode: file`), tokens are persisted to an encrypted local file — no external dependency needed. Set `tokenStore.mode: redis` and provide `redis_url` to share tokens across multiple instances. Set `tokenStore.mode: postgres` and provide `tokenStore.pg_url` for deployments that already have a PostgreSQL database.

### OIDC authentication (optional)

By default, Gimme uses HTTP Basic Auth to protect `/admin` and the token management API.
You can switch to an external OIDC provider (Keycloak, Dex, Auth0, …) with:

```yaml
auth:
  mode: oidc
  oidc:
    issuer: https://keycloak.example.com/realms/gimme
    client_id: gimme
    client_secret: ""        # leave empty if your client is public
    redirect_url: https://gimme.example.com/auth/callback
```

**How it works:**
- Unauthenticated requests to `/admin`, `POST /tokens`, `DELETE /tokens/:id` are redirected to `GET /auth/login`.
- `GET /auth/login` starts the OAuth2 authorization code flow (CSRF-protected with a state cookie).
- `GET /auth/callback` validates the OIDC ID token, then issues a signed session cookie (HS256 JWT, 8 h TTL).

**Keycloak quick setup:**

1. Create a realm named `gimme`
2. Create a client named `gimme`:
   - Client authentication: **On**
   - Valid redirect URIs: `https://gimme.example.com/auth/callback`
3. Copy the client secret → set it as `auth.oidc.client_secret`
4. Create users in the realm — they will be able to log in to `/admin`

> The Docker Compose `with-garage` example includes a commented-out Keycloak service.
> See `examples/deployment/docker-compose/with-garage/README.md` for step-by-step instructions.

---

## API Usage

### 1. Create an access token

> **Breaking change:** API tokens are now cryptographically random opaque strings (`gim_<hex>`, 68 chars), stored as SHA-256 hashes in the token store (encrypted file or Redis). **JWT tokens issued by previous versions are invalid and must be regenerated via `/admin` or `POST /tokens`.**
>
> The raw token is returned **once** — store it securely. Only its hash is persisted.

In `basic` mode, use your `admin.user` / `admin.password` as HTTP Basic Auth credentials:

```bash
curl -s -X POST http://localhost:8080/tokens \
  -u gimmeadmin:gimmeadmin \
  -H 'Content-Type: application/json' \
  -d '{"name": "my-token", "expirationDate": "2027-12-31"}'
```

In `oidc` mode, authenticate via the admin UI at `/admin` and use the token management interface.

 Response: `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "my-token",
  "token": "gim_4a7b9c2d1e3f...",
  "createdAt": "2026-02-28T10:00:00Z",
  "expiresAt": "2027-12-31T00:00:00Z"
}
```

> The raw token (`gim_<hex>`, 68 chars) is returned **once** — store it securely. Only its SHA-256 hash is persisted.

> If `expirationDate` is omitted, the token expires in **90 days**.

### 2. Upload a package

A package is a ZIP archive. The `name` and `version` fields identify it in the CDN.

```bash
curl -s -X POST http://localhost:8080/packages \
  -H 'Authorization: Bearer <token>' \
  -F 'file=@awesome-lib.zip' \
  -F 'name=awesome-lib' \
  -F 'version=1.0.0'
```

Response: `201 Created`

### 3. Serve a file

Once uploaded, files are served at:

```
GET /gimme/<package>@<version>/<file>
```

```bash
curl http://localhost:8080/gimme/awesome-lib@1.0.0/awesome-lib.min.js
```

**Semver partial versions are supported** — `awesome-lib@1.0` resolves to the latest `1.0.x` available.

> **CORS:** CORS is configurable via `cors.allowed_origins` in `gimme.yml`. If left empty (the default), all origins are allowed (`*`) — suitable for a public CDN. Set it to a list of trusted origins to restrict cross-origin access.

Use it directly in HTML:

```html
<link rel="stylesheet" href="http://localhost:8080/gimme/awesome-lib@1.0.0/awesome-lib.min.css">
<script src="http://localhost:8080/gimme/awesome-lib@1.0.0/awesome-lib.min.js" type="module"></script>
```

### 4. Browse package contents

```
GET /gimme/<package>@<version>
```

Returns an HTML page listing all files in the package.

```bash
curl http://localhost:8080/gimme/awesome-lib@1.0.0
```

### 5. Delete a package

```bash
curl -s -X DELETE http://localhost:8080/packages/awesome-lib@1.0.0 \
  -H 'Authorization: Bearer <token>'
```

Response: `204 No Content`

### API routes summary

| Method   | Route                        | Auth         | Description                          |
|----------|------------------------------|--------------|--------------------------------------|
| `GET`    | `/`                          | —            | HTML homepage                        |
| `GET`    | `/admin`                     | Admin auth   | Admin UI (token management)          |
| `POST`   | `/tokens`                    | Admin auth   | Create an opaque access token        |
| `DELETE` | `/tokens/:id`                | Admin auth   | Revoke an access token               |
| `POST`   | `/packages`                  | Bearer token | Upload a ZIP package                 |
| `DELETE` | `/packages/:package`         | Bearer token | Delete a package (`name@version`)    |
| `GET`    | `/gimme/:package`            | —            | List files in a package (HTML)       |
| `GET`    | `/gimme/:package/*file`      | —            | Serve a file from a package          |
| `GET`    | `/metrics`                   | —            | Prometheus / OpenMetrics endpoint    |
| `GET`    | `/docs`                      | —            | Interactive API documentation        |
| `GET`    | `/healthz`                   | —            | Liveness probe                       |
| `GET`    | `/readyz`                    | —            | Readiness probe (checks S3 bucket)   |

---

## Deployment Examples

The [`examples/deployment`](examples/deployment) directory contains ready-to-use configurations:

| Stack                       | Path                                                         | Description                                              |
|-----------------------------|--------------------------------------------------------------|----------------------------------------------------------|
| Docker Compose + managed S3 | [`with-managed-s3/`](examples/deployment/docker-compose/with-managed-s3/) | gimme + any cloud S3 provider (AWS, OVH, Scaleway, Cellar, …) + monitoring |
| Docker Compose + Garage     | [`with-garage/`](examples/deployment/docker-compose/with-garage/) | Self-provisioning stack with self-hosted Garage + monitoring |
| Kubernetes                  | [`kubernetes/`](examples/deployment/kubernetes/)             | Namespace, Deployment, Service, Ingress                  |
| systemd                     | [`systemd/`](examples/deployment/systemd/)                   | Linux systemd unit file                                  |

### Docker — single container

```bash
docker run -p 8080:8080 \
  -v "$(pwd)/gimme.yml:/config/gimme.yml" \
  ziggornif/gimme:latest
```

> The Docker image reads its config from `/config/gimme.yml`. Mount your local `gimme.yml` to that path as shown above.

---

## Caching Strategy

Gimme implements two independent, composable caching levels:

```
Browser → [Level 1: external proxy / CDN] → [gimme + Level 2: internal Redis cache] → [S3]
```

### Level 1 — HTTP Cache-Control headers (zero dependency)

Gimme automatically emits `Cache-Control` headers on every file response, allowing any HTTP cache (browser, CDN, reverse proxy) to cache assets without any extra configuration.

| Version type | Example | `Cache-Control` header |
|---|---|---|
| Pinned (3-part semver) | `pkg@1.0.0` | `public, max-age=31536000, immutable` |
| Partial | `pkg@1.0` or `pkg@1` | `public, max-age=300` |
| Not found (404) | any | `no-store` |

**Pinned versions** (`pkg@1.0.0`) are immutable by design — the same URL always resolves to exactly the same files. Browsers and proxies can cache them for up to 1 year with no revalidation.

**Partial versions** (`pkg@1.0`) resolve to the latest matching patch at request time, so they are only cached for 5 minutes.

**404 responses** are never cached, to avoid propagating transient misses.

#### Using a reverse proxy or CDN

Any HTTP cache that honours `Cache-Control` headers will work in front of gimme — Nginx, Varnish, Caddy (with the [`cache-handler`](https://github.com/caddyserver/cache-handler) plugin), Cloudflare, Fastly, etc.

Configure your proxy to cache `/gimme/*` responses and pass the `Cache-Control` header through. The headers emitted by gimme are enough to drive the caching policy:

- Pinned versions (`pkg@1.0.0`) — `immutable`, safe to cache for 1 year.
- Partial versions (`pkg@1.0`) — `max-age=300`, revalidated every 5 minutes.
- 404 / errors — `no-store`, never cached.

### Level 2 — Internal Redis cache (optional)

Gimme includes an optional internal cache backed by **Redis / Valkey**. When enabled, it caches the result of partial version resolution (`pkg@1.0` → `pkg@1.0.3`) so that S3 `ListObjects` calls are avoided on repeated requests.

> The file body is always streamed directly from S3 — only the resolved S3 object path is cached.

#### How it works

1. A request arrives for `GET /gimme/pkg@1.0/file.js` (partial version).
2. Gimme looks up the key `pkg@1.0/file.js` in Redis.
3. **Cache hit** → the resolved path (e.g. `pkg@1.0.3/file.js`) is returned immediately; S3 `ListObjects` is skipped.
4. **Cache miss** → gimme resolves the latest version via S3, stores the result in Redis with the configured TTL, then streams the file.
5. When a package is deleted (`DELETE /packages/pkg@1.0.3`), cache entries whose key starts with `pkg@1.0.3` are invalidated. Partial-version entries (e.g. `pkg@1.0/file.js`) are not touched — they will naturally expire via the TTL and resolve to the next available version on the following request.

Pinned versions (`pkg@1.0.0`) are **not** stored in Redis — their path is deterministic and requires no resolution.

#### Configuration

```yaml
redis_url: redis://localhost:6379
cache:
  enabled: true
  type: redis          # only "redis" is supported; "memory" is reserved for future use
  ttl: 3600            # TTL in seconds (default: 3600)
```

| Key               | Description                              | Default                      |
|-------------------|------------------------------------------|------------------------------|
| `cache.enabled`   | Enable the internal cache                | `false`                      |
| `cache.type`      | Cache backend (`redis`)                  | `redis`                      |
| `cache.ttl`       | Entry TTL in seconds                     | `3600`                       |
| `redis_url`       | Redis/Valkey connection URL              | `redis://localhost:6379`     |

#### Docker Compose example

A ready-to-use stack with Garage + Valkey is available in [`examples/deployment/docker-compose/with-garage/`](examples/deployment/docker-compose/with-garage/). Add the following to your `gimme.yml` to enable the cache:

```yaml
redis_url: redis://valkey:6379
cache:
  enabled: true
  type: redis
  ttl: 3600
```

---

## Monitoring

Each gimme instance exposes a `/metrics` endpoint in [OpenMetrics](https://openmetrics.io/) format, compatible with Prometheus.

In addition to the standard Go runtime and process metrics (goroutines, memory, GC, CPU), gimme exposes the following **application-level metrics**:

### HTTP traffic

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gimme_http_requests_total` | Counter | `route`, `method`, `status_code` | Total HTTP requests handled, partitioned by Gin route pattern (e.g. `/gimme/:package/*file`), HTTP method and response status code |

### S3 / object storage latency

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gimme_s3_operation_duration_seconds` | Histogram | `operation` | Duration of S3 operations in seconds. `operation` values: `AddObject`, `GetObject`, `ListObjects`, `ObjectExists`, `RemoveObjects`, `Ping` |

### Internal cache (optional — requires `cache.enabled: true`)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gimme_cache_hits_total` | Counter | — | Cache hits on partial-version resolution (e.g. `pkg@1.0` → resolved path served from cache) |
| `gimme_cache_misses_total` | Counter | — | Cache misses on partial-version resolution |

### Package lifecycle

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gimme_packages_uploaded_total` | Counter | — | Total packages successfully uploaded via `POST /packages` |
| `gimme_packages_deleted_total` | Counter | — | Total packages successfully deleted via `DELETE /packages/:package` |

---

A pre-configured Prometheus + Grafana stack is bundled in both Docker Compose examples (`with-garage/` and `with-managed-s3/`). Each stack includes its own `monitoring/` directory with the Prometheus config and Grafana dashboard.

Once a stack is running:
- Prometheus: <http://localhost:9090>
- Grafana: <http://localhost:3000> (anonymous access enabled)
