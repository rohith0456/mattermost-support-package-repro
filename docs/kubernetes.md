# Kubernetes Repro Environments

`mm-repro` can generate Kubernetes manifests instead of Docker Compose when the
customer's Mattermost instance is running on Kubernetes. This creates a local
[kind](https://kind.sigs.k8s.io/) cluster that mirrors the customer's topology.

---

## Prerequisites

| Tool | Purpose | Install |
|------|---------|---------|
| Docker Desktop | Container runtime for kind | [docker.com](https://www.docker.com/products/docker-desktop/) |
| kind | Kubernetes in Docker (local cluster) | [kind.sigs.k8s.io](https://kind.sigs.k8s.io/docs/user/quick-start/) |
| kubectl | Apply manifests & view pods | [kubernetes.io](https://kubernetes.io/docs/tasks/tools/) |

Check all tools are present:

```bash
mm-repro doctor
```

---

## Auto-detection

When mm-repro parses a support package, it looks for Kubernetes signals:

- **Pod name patterns in cluster_info.json** — Deployment pods like
  `mattermost-7d8f4b5c6-2xzpk` or StatefulSet pods like `mattermost-0`
- **SiteURL patterns** — hostnames containing `.cluster.local`, `.svc.`, or `kubernetes`

When these are detected, mm-repro automatically switches to Kubernetes output:

```
Output:   kubernetes
```

You can also force Kubernetes output for any support package:

```bash
mm-repro init --support-package ./sp.zip --with-kubernetes
```

Or override detection and use Docker Compose instead:

```bash
mm-repro init --support-package ./sp.zip --force-docker-compose
```

---

## Generated Output

For a Kubernetes repro, `mm-repro init` creates a `kubernetes/` directory
containing:

```
kubernetes/
  00-namespace.yaml         # mattermost-repro namespace
  01-postgres.yaml          # PostgreSQL Deployment + Service + PVC
  02-mailhog.yaml           # MailHog Deployment + Service
  03-opensearch.yaml        # (optional) OpenSearch
  04-openldap.yaml          # (optional) OpenLDAP + phpLDAPadmin
  05-keycloak.yaml          # (optional) Keycloak OIDC
  06-minio.yaml             # (optional) MinIO S3-compatible storage
  07-mattermost.yaml        # ConfigMap + Deployment/StatefulSet + Services
  kustomization.yaml        # Kustomize resource list
Makefile
scripts/start.sh
scripts/stop.sh
scripts/reset.sh
REPRO_SUMMARY.md
```

### Single-node vs multi-node

| Topology | Kubernetes resource | Mattermost access |
|----------|--------------------|--------------------|
| Single-node | `Deployment` (1 replica) | `NodePort` on `localhost:30065` |
| Multi-node | `StatefulSet` (N replicas) | `NodePort` on `localhost:30065` |

---

## Usage

### Start

```bash
cd generated-repro/<your-repro>
make run
# or
./scripts/start.sh
```

This:
1. Creates a `mm-repro` kind cluster (skipped if already exists)
2. Applies all manifests with `kubectl apply -f kubernetes/`
3. Waits for Mattermost pod to become ready

### Stop (keep cluster + data)

```bash
make stop
# or
./scripts/stop.sh
```

Deletes the Kubernetes resources but preserves the kind cluster and its
persistent volumes.

### Reset (delete everything)

```bash
make reset
# or
./scripts/reset.sh
```

Deletes the entire kind cluster including all data.

### Logs

```bash
make logs
# or
kubectl -n mattermost-repro logs -l app=mattermost -f
```

### Status

```bash
make status
# or
kubectl -n mattermost-repro get pods
```

---

## Service URLs

| Service | URL |
|---------|-----|
| Mattermost | http://localhost:30065 |
| MailHog UI | http://localhost:30025 |
| Keycloak (if enabled) | http://localhost:30080 |
| MinIO console (if enabled) | http://localhost:30901 |

---

## Default Credentials

All services use the same default credentials as Docker Compose repros.
See [REPRO_SUMMARY.md](../README.md#default-users) in the generated project for the full
list of pre-created users.

---

## Troubleshooting

**Pods stuck in `Pending`**

kind uses Docker Desktop's resource limits. Ensure Docker Desktop has at least
4 GB RAM allocated (Preferences → Resources).

**`kind: command not found`**

Install kind from https://kind.sigs.k8s.io/docs/user/quick-start/ and ensure
it is in your PATH.

**Port already in use (30065, 30025, etc.)**

kind NodePorts map to the host. If a port is in use, stop the conflicting
service or recreate the kind cluster with custom port mappings.

**OIDC/Keycloak login not working**

Keycloak's auth endpoint is served at `http://localhost:30080`. Ensure the
Keycloak pod is ready (`make status`) before attempting login.
