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
┌─────────────┐   healthy   ┌──────────────┐   completed   ┌─────────┐
│   garage    │ ──────────► │ init-garage  │ ────────────► │  gimme  │
│  :3900/3903 │             │  (alpine)    │               │  :8080  │
└─────────────┘             └──────────────┘               └─────────┘
```

## Notes

- The `init-garage` script is idempotent: restarting the stack does not create duplicate keys or buckets.
- Unlike Minio, Garage does **not** auto-create buckets — the `init-garage` service handles this.
- The `gimme.yml` file at the root of this directory is a reference template only; the actual config is generated at runtime into the `gimme-config` Docker volume.
- This example uses a single-node Garage setup suitable for development. For production, see the [Garage documentation](https://garagehq.deuxfleurs.fr/documentation/cookbook/real-world/).
