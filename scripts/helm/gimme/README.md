# Gimme Helm Chart

Helm chart for deploying [Gimme](https://github.com/ziggornif/gimme) — a self-hosted CDN backed by any S3-compatible storage.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.10+
- An S3-compatible backend: [Garage HQ](https://garagehq.deuxfleurs.fr/), [Minio](https://min.io/), or any managed S3

## Install

### From GHCR (OCI)

```sh
helm install gimme oci://ghcr.io/ziggornif/charts/gimme \
  --namespace gimme \
  --create-namespace \
  --set credentials.secret=<jwt-secret> \
  --set credentials.admin.user=<admin-user> \
  --set credentials.admin.password=<admin-password> \
  --set credentials.s3.key=<s3-key> \
  --set credentials.s3.secret=<s3-secret> \
  --set s3.url=<s3-endpoint-url>
```

### From source

```sh
git clone https://github.com/ziggornif/gimme.git
cd gimme/examples/deployment/helm

helm install gimme ./gimme \
  --namespace gimme \
  --create-namespace \
  -f my-values.yaml
```

## Configuration

### Minimal `values.yaml`

```yaml
credentials:
  secret: "your-jwt-secret"
  admin:
    user: "admin"
    password: "strongpassword"
  s3:
    key: "your-s3-access-key"
    secret: "your-s3-secret-key"

s3:
  url: "https://s3.example.com"
  bucketName: gimme
  location: eu-west-1
  ssl: true
```

### Full reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Container image repository | `ziggornif/gimme` |
| `image.tag` | Image tag (defaults to chart `appVersion`) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `config.port` | HTTP server port | `8080` |
| `config.metrics` | Enable `/metrics` Prometheus endpoint | `true` |
| `credentials.secret` | **Required** — JWT signing secret | `""` |
| `credentials.admin.user` | **Required** — Admin username (Basic Auth) | `""` |
| `credentials.admin.password` | **Required** — Admin password | `""` |
| `credentials.s3.key` | **Required** — S3 access key | `""` |
| `credentials.s3.secret` | **Required** — S3 secret key | `""` |
| `s3.url` | **Required** — S3 endpoint URL | `""` |
| `s3.bucketName` | S3 bucket name | `gimme` |
| `s3.location` | S3 region / Garage zone | `us-east-1` |
| `s3.ssl` | Enable TLS for S3 connection | `true` |
| `cache.enabled` | Enable Redis/Valkey internal cache | `false` |
| `cache.type` | Cache backend type (`redis`) | `redis` |
| `cache.ttl` | Cache TTL in seconds | `3600` |
| `cache.redisUrl` | Redis/Valkey URL (required when `cache.enabled=true`) | `""` |
| `auth.mode` | Authentication mode: `basic` or `oidc` | `basic` |
| `auth.oidc.issuer` | OIDC issuer URL (required when `auth.mode=oidc`) | `""` |
| `auth.oidc.clientId` | OIDC client ID (required when `auth.mode=oidc`) | `""` |
| `auth.oidc.redirectUrl` | OIDC redirect URI (required when `auth.mode=oidc`) | `""` |
| `auth.oidc.secureCookies` | Enable secure cookies (set to `false` for local HTTP dev) | `true` |
| `credentials.oidcClientSecret` | OIDC client secret (required when `auth.mode=oidc`) | `""` |
| `tokenStore.mode` | Token store backend: `file`, `redis` or `postgres` | `file` |
| `tokenStore.pgUrl` | PostgreSQL URL (required when `tokenStore.mode=postgres`) | `""` |
| `service.type` | Kubernetes Service type | `ClusterIP` |
| `service.port` | Service port | `80` |
| `ingress.enabled` | Enable Ingress | `false` |
| `ingress.className` | Ingress class name | `nginx` |
| `ingress.host` | Ingress hostname | `gimme.example.com` |
| `ingress.annotations` | Additional Ingress annotations | `{}` |
| `ingress.tls.enabled` | Enable TLS on Ingress | `false` |
| `ingress.tls.secretName` | TLS secret name | `gimme-tls` |
| `hpa.enabled` | Enable HorizontalPodAutoscaler | `false` |
| `hpa.minReplicas` | HPA minimum replicas | `1` |
| `hpa.maxReplicas` | HPA maximum replicas | `5` |
| `hpa.targetCPUUtilizationPercentage` | HPA target CPU utilization | `70` |
| `serviceAccount.create` | Create a ServiceAccount | `true` |
| `serviceAccount.name` | ServiceAccount name override | `""` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `64Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Pod tolerations | `[]` |
| `affinity` | Pod affinity rules | `{}` |
| `imagePullSecrets` | Image pull secrets for private registries | `[]` |
| `serviceAccount.annotations` | Annotations to add to the ServiceAccount | `{}` |
| `podAnnotations` | Annotations to add to the pod | `{}` |
| `podLabels` | Additional labels to add to the pod | `{}` |
| `podSecurityContext.runAsNonRoot` | Run pod as non-root | `true` |
| `podSecurityContext.runAsUser` | Pod user UID | `1000` |
| `podSecurityContext.fsGroup` | Pod filesystem GID | `1000` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.readOnlyRootFilesystem` | Mount root filesystem as read-only | `true` |
| `securityContext.capabilities.drop` | Linux capabilities to drop | `["ALL"]` |

## Examples

### With Ingress and TLS (cert-manager)

```yaml
ingress:
  enabled: true
  className: nginx
  host: cdn.example.com
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  tls:
    enabled: true
    secretName: gimme-tls
```

### With Redis cache

```yaml
cache:
  enabled: true
  type: redis
  ttl: 3600
  redisUrl: redis://my-redis-service:6379
```

### With HPA (auto-scaling)

Horizontal scaling requires a shared token store. Everything else in gimme already
scales — but in the default `file` mode, tokens live in a per-pod volume, so a token
issued by one pod is unknown to the others.

Set `tokenStore.mode` to `redis` or `postgres` before adding replicas:

```yaml
replicaCount: 2

tokenStore:
  mode: redis

cache:
  enabled: true
  type: redis
  redisUrl: redis://my-redis-service:6379

hpa:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
```

`redis` is the natural choice: if you also enable the version-resolution cache, one
dependency serves both.

#### What scales, and what does not

| State | Shared across pods? |
|---|---|
| Token store, `file` mode | **no** — one file per pod |
| Token store, `redis` / `postgres` | yes |
| S3 objects | yes, by definition |
| Version-resolution cache | yes (Redis). With the cache off, each pod resolves independently — slower, still correct |
| OIDC sessions | yes |
| OIDC state / nonce | validated against browser cookies, no server-side state |

OIDC scales without extra configuration because the session signing key is derived
from the shared secret. Every pod reads the same `GIMME_SECRET` from the same
Kubernetes Secret, derives the same key, and can therefore validate a cookie issued
by any other pod.

The token store is the only thing standing between this chart and horizontal scaling.

### Token durability in `file` mode

`file` mode is the zero-dependency option: no Redis, no PostgreSQL. The chart mounts
an `emptyDir` at `/tmp` for it, which is required — `readOnlyRootFilesystem: true`
would otherwise leave the token store nowhere to write.

An `emptyDir` lives and dies with the **pod**, not the container:

| Event | Tokens survive? |
|---|---|
| Container crash and restart in the same pod | **yes** |
| Liveness probe kill | **yes** |
| Container OOM kill | **yes** |
| Pod replaced — `helm upgrade`, image update, node drain, eviction, scale down | **no** |

The last row is the one that matters in practice: **`helm upgrade` replaces pods, so
upgrading gimme discards every issued API token.** Clients must be re-issued tokens
after an upgrade.

Use `redis` or `postgres` if tokens need to outlive a pod.

## Upgrade

```sh
helm upgrade gimme oci://ghcr.io/ziggornif/charts/gimme \
  --namespace gimme \
  -f my-values.yaml
```

## Uninstall

```sh
helm uninstall gimme --namespace gimme
```

## Security

Sensitive values (`credentials.*`) are stored in a Kubernetes `Secret` and injected as environment variables into the container. They are **never** written to the `ConfigMap`.

The pod runs as a non-root user (`uid=1000`) with a read-only root filesystem and dropped Linux capabilities by default.
