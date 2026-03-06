# Gimme with Garage

This example deploys Gimme backed by [Garage](https://garagehq.deuxfleurs.fr/), a lightweight S3-compatible object store.

## Prerequisites

- Docker and Docker Compose installed

## Quick start

```bash
docker compose up -d
```

That's it. The `init-garage` service automatically:
1. Waits for Garage to be healthy
2. Creates the cluster layout (single-node, 10 GiB)
3. Creates the S3 bucket and key
4. Writes the generated `gimme.yml` into the shared `gimme-config` volume
5. Exits — `gimme` then starts and reads its config from that volume

Gimme will be available at <http://localhost:8080>.

## Configuration

All parameters are passed as environment variables on the `init-garage` service in `docker-compose.yml`.

| Variable | Default | Description |
|---|---|---|
| `GARAGE_ADMIN_TOKEN` | `gimme-init-token-change-me` | Must match `admin_token` in `garage.toml` |
| `GARAGE_CAPACITY_BYTES` | `10737418240` (10 GiB) | Storage capacity assigned to the node |
| `GARAGE_ZONE` | `dc1` | Zone name for the cluster layout |
| `GARAGE_REGION` | `garage` | S3 region (must match `s3_region` in `garage.toml`) |
| `BUCKET_NAME` | `gimme` | S3 bucket name |
| `KEY_NAME` | `gimme-key` | S3 access key name |
| `GIMME_ADMIN_USER` | `gimmeadmin` | Gimme admin username |
| `GIMME_ADMIN_PASSWORD` | `gimmeadmin` | Gimme admin password |
| `GIMME_SECRET` | `change_me_use_a_real_secret` | JWT signing secret |
| `CACHE_ENABLED` | `true` | Enable Redis cache |
| `CACHE_REDIS_URL` | `redis://redis:6379` | Redis connection URL |
| `CACHE_TTL` | `3600` | Cache TTL in seconds |

**For production**, change all secrets before deploying:
- `admin_token` in `garage.toml` (generate with `openssl rand -base64 32`)
- `GARAGE_ADMIN_TOKEN` in `docker-compose.yml` (must match the above)
- `GIMME_ADMIN_PASSWORD` and `GIMME_SECRET` in `docker-compose.yml`

## Architecture

```
                 ┌─────────────────────────────────────────┐
                 │           gimme-config volume           │
                 │           (generated gimme.yml)         │
                 └──────────┬──────────────────────────────┘
                            │ writes              │ reads
                            ▼                     ▼
┌─────────────┐   healthy   ┌──────────────┐   completed   ┌─────────┐   healthy
│   garage    │ ──────────► │ init-garage  │ ────────────► │  gimme  │ ◄────────── redis
│  :3900/3903 │             │  (alpine)    │               │  :8080  │             :6379
└─────────────┘             └──────────────┘               └─────────┘
```

## OIDC authentication (optional)

This stack includes a commented-out Keycloak service you can enable to protect `/admin` and the token management API with OpenID Connect.

### 1. Uncomment Keycloak in `docker-compose.yml`

Uncomment the `keycloak-db`, `keycloak` services and the `keycloak-db` volume, then also uncomment the `keycloak` dependency under `gimme.depends_on`.

### 2. Set the OIDC environment variables on `init-garage`

```yaml
# in docker-compose.yml, under init-garage > environment:
AUTH_MODE: oidc
AUTH_OIDC_ISSUER: http://keycloak:8180/realms/gimme
AUTH_OIDC_CLIENT_ID: gimme
AUTH_OIDC_CLIENT_SECRET: change_me_oidc_secret
AUTH_OIDC_REDIRECT_URL: http://localhost:8080/auth/callback
```

### 3. Start the stack

```bash
docker compose up -d
```

### 4. Configure Keycloak

Once the stack is up, open <http://localhost:8180> and log in with `admin` / `admin_password_change_me`.

1. **Create a realm** named `gimme`
2. **Create a client** named `gimme`:
   - Client authentication: **On**
   - Valid redirect URIs: `http://localhost:8080/auth/callback`
   - Copy the **client secret** from the *Credentials* tab → set it as `AUTH_OIDC_CLIENT_SECRET` and restart the stack
3. **Create a user** in the `gimme` realm → set a password → log in at <http://localhost:8080/auth/login>

> **Note:** The `AUTH_OIDC_ISSUER` must be reachable by the Gimme container at runtime. Inside Docker Compose, use the container hostname (`keycloak:8180`). The browser-facing `redirect_url` uses `localhost:8080`.

## Notes

- The `init-garage` script is idempotent: restarting the stack does not create duplicate keys or buckets.
- Unlike Minio, Garage does **not** auto-create buckets — the `init-garage` service handles this.
- The `gimme.yml` file at the root of this directory is a reference template only; the actual config is generated at runtime into the `gimme-config` Docker volume.
- This example uses a single-node Garage setup suitable for development. For production, see the [Garage documentation](https://garagehq.deuxfleurs.fr/documentation/cookbook/real-world/).
