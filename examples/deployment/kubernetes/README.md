# Kubernetes configuration example

## Content

- `gimme-deployment.yml` — Namespace, ConfigMap, Deployment (with resources, liveness/readiness probes) and Service (NodePort)
- `gimme-ingress.yml` — Optional Ingress resource for host-based routing (requires an Ingress controller, e.g. nginx)

## Run

```sh
# Deploy the application
kubectl apply -f gimme-deployment.yml

# Optional: expose via Ingress (edit host in gimme-ingress.yml first)
kubectl apply -f gimme-ingress.yml
```

## Health endpoints

| Route      | Type       | Description                                      |
|------------|------------|--------------------------------------------------|
| `GET /healthz` | Liveness  | Returns `200 OK` when the process is running     |
| `GET /readyz`  | Readiness | Returns `200 OK` when the S3 bucket is reachable |

## Scaling options

### HorizontalPodAutoscaler (HPA)

Gimme is stateless (all state lives in S3), so horizontal scaling is safe.
The `resources.requests` defined in the Deployment are required for HPA to work.

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: gimme-hpa
  namespace: gimme
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: gimme-deployment
  minReplicas: 1
  maxReplicas: 5
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

### PodDisruptionBudget (PDB)

When running multiple replicas, a PDB ensures at least one pod stays available during cluster maintenance or rolling updates.

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: gimme-pdb
  namespace: gimme
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: gimme
```