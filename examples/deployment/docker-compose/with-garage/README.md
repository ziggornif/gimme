# Gimme with Garage

This example deploys Gimme backed by [Garage](https://garagehq.deuxfleurs.fr/), a lightweight S3-compatible object store.

## Prerequisites

- Docker and Docker Compose installed

## Setup

### 1. Generate a secure RPC secret

Edit `garage.toml` and replace the placeholder `rpc_secret` with a real random value:

```bash
openssl rand -hex 32
```

### 2. Start Garage

```bash
docker compose up -d garage
```

### 3. Initialize the Garage cluster (one-time setup)

Garage requires a manual initialization step after the first start.

```bash
# Get the node ID
NODE_ID=$(docker exec $(docker compose ps -q garage) /garage status | grep -oP '[0-9a-f]{16}' | head -1)

# Assign the layout (adjust -c for your available disk space)
docker compose exec garage /garage layout assign -z dc1 -c 10G $NODE_ID
docker compose exec garage /garage layout apply --version 1

# Create a key and bucket for Gimme
docker compose exec garage /garage key create gimme-key
docker compose exec garage /garage bucket create gimme
docker compose exec garage /garage bucket allow --read --write --owner gimme --key gimme-key
```

### 4. Get the credentials

```bash
docker compose exec garage /garage key info gimme-key
```

Copy the **Key ID** and **Secret key** values.

### 5. Configure Gimme

Edit `gimme.yml` and fill in your credentials:

```yaml
secret: <your-random-jwt-secret>
admin:
  user: gimmeadmin
  password: <your-admin-password>
s3:
  url: garage:3900
  key: <Key ID from step 4>
  secret: <Secret key from step 4>
  bucketName: gimme
  location: garage
  ssl: false
```

### 6. Start Gimme

```bash
docker compose up -d
```

Gimme will be available at http://localhost:8080.

## Notes

- Unlike Minio, Garage does **not** auto-create buckets on first use. The initialization steps above are mandatory.
- The S3 region for Garage is `garage` (as set in `garage.toml` under `[s3_api] s3_region`).
- This example uses a single-node Garage setup suitable for development. For production, see the [Garage documentation](https://garagehq.deuxfleurs.fr/documentation/cookbook/real-world/).
