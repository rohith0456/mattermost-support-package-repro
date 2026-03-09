# Multi-Node / HA Local Reproduction

This document covers how `mm-repro` simulates a Mattermost High Availability (HA) cluster locally, the limitations of this approach, and guidance on when and how to use multi-node repros effectively.

---

## When to Use Multi-Node

Use a multi-node repro when the ticket involves:

- **Cluster synchronization issues** — Config changes not propagating across nodes, plugin state sync failures
- **Session affinity problems** — Users being logged out when requests are routed to a different node
- **Gossip / cluster health** — Nodes not detecting each other, split-brain scenarios
- **High availability failover** — Behavior when one node stops
- **Load balancer behavior** — Request routing, sticky sessions, health check endpoints
- **Race conditions** — Issues that only manifest under concurrent requests hitting multiple nodes
- **Plugin behavior in HA** — Some plugins behave differently in a clustered setup

Use a single-node repro for:

- Database-level issues
- Configuration issues (no HA-specific behavior)
- Plugin issues not related to HA state sync
- Auth/LDAP/SAML issues
- Performance issues (single-node is simpler to profile)
- Anything where HA behavior is not the suspected root cause

---

## Auto-Detection vs. Forcing Topology

### Auto-Detection

The topology parser examines these signals to determine if the customer had a cluster:

| Signal | Location in package | What it indicates |
|--------|---------------------|-------------------|
| `ClusterSettings.Enable` | `config.json` | Cluster mode explicitly enabled |
| Multiple entries in `cluster_discovery.json` | `cluster_discovery.json` | Multiple nodes registered |
| Multiple node entries in `diagnostics.json` | `diagnostics.json` | Multiple nodes reported stats |
| Log lines containing `cluster` or `HA` keywords | `mattermost.log` | Cluster-related activity |

If two or more of these signals are present, the inference engine uses multi-node. If only one signal is present, it uses single-node but logs a warning.

### Forcing Topology

Override the auto-detection with explicit flags:

```bash
# Force multi-node (even if signals suggest single-node)
mm-repro init --support-package ./customer.zip --force-multi-node

# Force single-node (even if signals suggest cluster)
mm-repro init --support-package ./customer.zip --force-single-node
```

When `--force-multi-node` is used without cluster signals present, a note is added to `REPRO_SUMMARY.md`:

```
[info] Multi-node forced via --force-multi-node flag. No cluster signals were
detected in the support package. The customer may have been on a single-node
deployment, or cluster configuration may not have been captured in the package.
```

---

## Generated Architecture

A multi-node repro generates a Docker Compose setup with the following services:

```
                    ┌───────────────────────────────────┐
                    │     nginx (load balancer)          │
                    │     port 8065 (http)               │
                    │     port 8443 (https, self-signed) │
                    └──────────┬──────┬──────┬───────────┘
                               │      │      │
                    ┌──────────┘      │      └──────────┐
                    │                 │                  │
             ┌──────▼──────┐  ┌──────▼──────┐  ┌───────▼──────┐
             │ mm-node-1   │  │ mm-node-2   │  │ mm-node-3    │
             │ :8065       │  │ :8066       │  │ :8067        │
             └──────┬──────┘  └──────┬──────┘  └───────┬──────┘
                    │                 │                  │
                    └────────┬────────┘                  │
                             │           ┌───────────────┘
                             ▼           ▼
                    ┌──────────────────────────┐
                    │    postgres / mysql        │
                    │    port 5432 / 3306        │
                    └──────────────────────────┘
```

All nodes share:
- The same PostgreSQL/MySQL database
- The same `.env` file (all nodes use identical config)
- The same Docker bridge network (`mm-repro-net`)
- A shared `config/` volume (for file sharing in cluster mode)

Each node has:
- Its own container name (`mm-node-1`, `mm-node-2`, `mm-node-3`)
- Its own exposed port (8065, 8066, 8067 for direct access)
- Its own cluster address (using the Docker service name)

---

## The nginx Load Balancer

The generated `config/nginx/nginx.conf` uses an upstream block to distribute requests:

```nginx
upstream mattermost_nodes {
    # Sticky session using cookie (mirrors common production setup)
    ip_hash;

    server mm-node-1:8065;
    server mm-node-2:8065;
    server mm-node-3:8065;
}

server {
    listen 80;

    location / {
        proxy_pass http://mattermost_nodes;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support (required for Mattermost)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 90s;
    }

    # Health check endpoint (bypasses load balancing)
    location /nginx-health {
        access_log off;
        return 200 "healthy\n";
        add_header Content-Type text/plain;
    }
}
```

### Load Balancing Algorithm

The default generated config uses `ip_hash` (sticky sessions by client IP). This is the most common production configuration and is appropriate for most HA repros.

To test round-robin behavior (no sticky sessions), edit `config/nginx/nginx.conf` and remove the `ip_hash;` directive, then restart nginx:

```bash
docker compose restart nginx
```

### Testing Individual Nodes

You can bypass the load balancer and connect directly to individual nodes:

| Node | Direct URL |
|------|-----------|
| nginx (load balancer) | http://localhost:8065 |
| mm-node-1 | http://localhost:8066 |
| mm-node-2 | http://localhost:8067 |
| mm-node-3 | http://localhost:8068 |

This is useful for:
- Checking if an issue is isolated to a specific node
- Verifying that cluster state is synchronized across all nodes
- Testing failover by disabling one node via nginx config

---

## Cluster Configuration

The generated Mattermost config enables cluster mode with local Docker container addresses:

```json
{
  "ClusterSettings": {
    "Enable": true,
    "ClusterName": "repro-cluster",
    "UseIPAddress": true,
    "EnableGossip": true,
    "ReadOnlyConfig": false
  }
}
```

Each node's `ClusterSettings.AdvertiseAddress` is set to its container IP, which Docker resolves via service name. The cluster discovery mechanism uses the database (not file-based or Kubernetes-based discovery).

---

## Simulating Cluster-Related Issues

### Simulating a Node Failure

Stop one node to simulate an unexpected failure:

```bash
# Stop node 2
docker compose stop mm-node-2

# Observe how the cluster handles the missing node
docker compose logs mm-node-1 | grep -i cluster
docker compose logs mm-node-3 | grep -i cluster

# Restart the node
docker compose start mm-node-2
```

### Simulating a Slow Node

Use Docker's built-in network throttling to simulate latency on one node:

```bash
# Add 200ms latency to mm-node-2's outbound traffic
docker exec mm-repro-mm-node-2 tc qdisc add dev eth0 root netem delay 200ms

# Remove the throttle
docker exec mm-repro-mm-node-2 tc qdisc del dev eth0 root
```

Note: `tc` requires the container to have `NET_ADMIN` capability. The generated `docker-compose.yml` does not include this by default. To enable:

```yaml
  mm-node-2:
    cap_add:
      - NET_ADMIN
```

Add this to the node's service definition in `docker-compose.yml` and recreate the container with `docker compose up -d mm-node-2`.

### Simulating a Network Partition

Disconnect one node from the others at the Docker network level:

```bash
# Disconnect mm-node-3 from the cluster network
docker network disconnect mm-repro_mm-repro-net mm-repro-mm-node-3

# Observe split behavior
docker compose logs mm-node-1 | grep -i "cluster\|gossip"

# Reconnect
docker network connect mm-repro_mm-repro-net mm-repro-mm-node-3
```

### Verifying Cluster State

Use the Mattermost System Console to verify cluster state:
`System Console > High Availability > Cluster Status`

Or via API:

```bash
# Get cluster status (requires admin token)
MM_TOKEN=$(docker exec mm-repro-mm-node-1 mmctl auth token --local)

curl -s http://localhost:8065/api/v4/cluster/status \
  -H "Authorization: Bearer $MM_TOKEN" | jq .
```

Expected output when all nodes are healthy:

```json
[
  {
    "id": "node1-...",
    "version": "9.11.2",
    "config_hash": "abc123",
    "ipaddress": "172.18.0.3",
    "hostname": "mm-node-1",
    "alive": true
  },
  ...
]
```

---

## Limitations of Local Multi-Node Repros

### Node Count

The local repro is limited to **3 nodes**. This is intentional to keep resource usage manageable on a developer machine. Production clusters can have many more nodes, but most HA bugs manifest with 2-3 nodes.

To reproduce an issue that specifically requires more than 3 nodes, you would need to edit the generated `docker-compose.yml` manually to add more `mm-node-N` service definitions following the same pattern.

### Resource Requirements

A 3-node repro with a database requires significant resources:

| Component | Approximate memory |
|-----------|-------------------|
| mm-node-1 | 1-2 GB |
| mm-node-2 | 1-2 GB |
| mm-node-3 | 1-2 GB |
| PostgreSQL | 256-512 MB |
| nginx | < 64 MB |
| **Total** | **3-7 GB** |

Ensure Docker Desktop has at least 8 GB of memory allocated:
`Docker Desktop > Settings > Resources > Memory`

### Cluster Networking vs. Production

The local Docker bridge network has significantly lower latency than a real production network (typically < 1ms vs. 1-10ms). This means:
- Gossip failures caused by network latency are difficult to reproduce locally
- Race conditions that require real network delays may not manifest
- For network-latency-related issues, use the `tc netem delay` technique described above

### Shared Filesystem

In production, Mattermost nodes typically use NFS or S3 for shared file storage. The local repro uses a shared Docker volume, which has different performance and consistency characteristics. File-locking issues that depend on NFS semantics will not reproduce accurately locally.

### Session Store

In production HA setups, sessions are often stored in a centralized session store (Redis or the database). The local repro uses the database for session storage, which is the default and covers the most common configuration.

---

## Multi-Node and Optional Services

Optional services work the same way in multi-node as in single-node. All nodes connect to the same optional service containers:

```bash
# Multi-node with all optional services
mm-repro init --support-package ./customer.zip \
  --force-multi-node \
  --with-opensearch \
  --with-ldap \
  --with-minio \
  --with-grafana
```

In the generated environment, all three Mattermost nodes point to the same:
- OpenSearch cluster
- OpenLDAP server
- MinIO instance
- MailHog SMTP server
- Prometheus exporter

The Grafana dashboards include per-node metrics when multiple nodes are running, allowing you to compare behavior across nodes.

---

## Debugging Cluster Issues: Checklist

When investigating a cluster-related issue:

1. **Check cluster status** — Are all nodes registered and showing as `alive: true`?
2. **Check config hash** — Are all nodes showing the same `config_hash`? A mismatch means config is not synchronized.
3. **Check plugin status** — Are all nodes running the same plugin versions and states?
4. **Check node logs** — Look for gossip errors, database connection issues, or config reload failures.
5. **Check nginx access logs** — Are requests being routed as expected?
6. **Reproduce with one node first** — If the issue reproduces on single-node, it may not be HA-specific.
7. **Reproduce with node 2 only** — Use direct node URLs to isolate which node has the issue.
8. **Check the database** — Is the `ClusterDiscovery` table correct?

Useful commands:

```bash
# Check the ClusterDiscovery table directly
make shell-db
# PostgreSQL:
SELECT hostname, alive, last_ping_at FROM clusterdiscovery ORDER BY last_ping_at DESC;

# Tail all node logs simultaneously
docker compose logs -f mm-node-1 mm-node-2 mm-node-3

# Show which node handled each request (nginx access log)
docker compose logs nginx | grep "POST\|GET" | tail -50
```

---

## Multi-Node in repro-plan.json

The `repro-plan.json` file in the generated project documents the topology decision:

```json
{
  "topology": {
    "type": "multi-node",
    "nodeCount": 3,
    "reason": "cluster signals detected: ClusterSettings.Enable=true, 3 nodes in diagnostics.json",
    "loadBalancer": "nginx",
    "ports": {
      "loadBalancer": 8065,
      "node1": 8066,
      "node2": 8067,
      "node3": 8068
    }
  }
}
```

This machine-readable format is useful for scripting or for understanding exactly why a multi-node topology was chosen.
