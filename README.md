# mm-repro ⚡

> **Point at a Mattermost support package ZIP. Get a running local environment in minutes.**

[![CI](https://github.com/rohith0456/mattermost-support-package-repro/actions/workflows/ci.yml/badge.svg)](https://github.com/rohith0456/mattermost-support-package-repro/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.22+-blue)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## The Problem It Solves

Reproducing a Mattermost issue locally means figuring out the exact server version, database type, auth setup, file storage config, and more — then manually wiring it all up. That's hours of work before you've even started debugging.

mm-repro reads a Mattermost support package ZIP and does it for you — **one command:**

```bash
mm-repro init --support-package ./support-package.zip
```

It figures out the exact setup from the package and generates a ready-to-run environment — Docker Compose **or** Kubernetes, matching the version and config, with all secrets safely replaced. Then:

```bash
cd generated-repro/20250310-143022/   # mm-repro init prints the exact path — just copy it
make run
open http://localhost:8065
```

> 💡 **The output folder is auto-named** `YYYYMMDD-HHMMSS` from when you ran `init` (e.g. `generated-repro/20250310-143022/`). `mm-repro init` prints the exact `cd` command at the end — no guessing needed. Add `--issue MM-1234` and the folder becomes `generated-repro/MM-1234-20250310-143022/`.

That's it. You're reproducing.

---

## Get Running in 5 Minutes

### 1 — Install Docker Desktop

Get it from **[docker.com](https://www.docker.com/products/docker-desktop/)** and make sure it's running.

### 2 — Install mm-repro

**Mac / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/rohith0456/mattermost-support-package-repro/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
winget install golang.go
go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest
```

**Check everything is good:**
```bash
mm-repro doctor
# All green? You're ready.
```

### 3 — Run It

**With a support package** (auto-detects version, DB, topology, services):
```bash
mm-repro init --support-package ~/Downloads/support-package.zip
cd generated-repro/20250310-143022/   # use the path printed by init
make run
make admin   # create sysadmin account — run once after first 'make run'
```

**Without a support package** — interactive wizard picks your setup:
```bash
mm-repro init
cd generated-repro/20250310-143022/   # use the path printed by init
make run
make admin   # create sysadmin account — run once after first 'make run'
```

Open `http://localhost:8065` and sign in:

| Field | Value |
|-------|-------|
| Username | `sysadmin` |
| Password | `Sysadmin1!` |

> **Email/password login always works** — no license, LDAP, or SAML required. After initial login, upload the required license to enable other features.

All emails are captured by Mailpit at `http://localhost:8025` — nothing real gets sent.

### 4 — Seed Test Content

Once Mattermost is running and you've logged in at least once, populate it with posts, threads, reactions, images and file attachments:

```bash
make seed PASS=Sysadmin1!
```

**Create custom channels** and seed posts into a specific channel:

```bash
# Create extra channels (no posts)
make channels NAMES="support,bugs,release-notes" PASS=Sysadmin1!

# Seed posts into a specific channel only
make seed CHANNEL=support PASS=Sysadmin1!

# Create channels AND post into one of them in a single command
make seed CHANNELS="support,bugs" CHANNEL=support PASS=Sysadmin1!
```

Or with the CLI directly for more control:

```bash
mm-repro seed --project . --with-files --posts 30 --password Sysadmin1!

# Create channels + target a specific channel for posts
mm-repro seed --project . --channels "support,bugs" --channel support --password Sysadmin1!
```

This fills `~town-square` and `~off-topic` (or your chosen channel) with varied test content — markdown formatting, code blocks, link unfurling, threaded conversations, emoji reactions, and (with `--with-files`) PNG screenshots and log file attachments.

### 5 — Reproduce, then Clean Up

```bash
make stop    # stop (keeps data)
make reset   # nuke everything and start fresh
```

---

## What Gets Generated

Running `mm-repro init` produces a complete project directory:

**Single-node / standard:**
```
generated-repro/20250310-143022/       ← folder name = YYYYMMDD-HHMMSS of when you ran init
├── docker-compose.yml     ← all services wired up
├── .env                   ← safe local-only credentials
├── Makefile               ← run / stop / reset / logs
├── REPRO_SUMMARY.md       ← what was recreated and what was approximated
├── REDACTION_REPORT.md    ← every credential that was detected and replaced
├── PLUGIN_REPORT.md       ← plugins found and their install status
└── repro-plan.json        ← full machine-readable plan
```

**Multi-node (HA) — extras added automatically:**
```
generated-repro/20250310-143022/
├── docker-compose.yml     ← mattermost-1/2/3 + nginx + minio
├── nginx/nginx.conf       ← load balancer config (auto-generated)
├── config/prometheus.yml  ← scrape config for all nodes (if Grafana enabled)
└── ...same files as above
```

**Kubernetes mode — instead of Compose:**
```
generated-repro/20250310-143022/
└── kubernetes/
    ├── 00-namespace.yaml
    ├── 01-postgres.yaml / 01-mysql.yaml
    ├── 02-mailhog.yaml
    ├── 03-mattermost.yaml
    └── kustomization.yaml
```

No manual YAML editing. No hunting for the right image tag. No credential leaks.

---

## 🧙 No Support Package? Use the Wizard

Don't have a support package yet, or just want to spin up a clean local Mattermost for testing? Run `mm-repro init` with no arguments:

```bash
mm-repro init
```

An interactive wizard walks you through every option:

```
── Mattermost Version
  Version (e.g. 10.5.0 — or press Enter for latest) [latest]:

── Database
  Database type:
  ▶ [1] PostgreSQL 15 (recommended)
    [2] MySQL 8.0

── Topology
  Deployment topology:
  ▶ [1] Single-node  (1 Mattermost container — fastest, simplest)
    [2] Multi-node HA (2 nodes behind nginx load balancer)
    [3] Multi-node HA (3 nodes behind nginx load balancer)

── Output Format
  How to run it:
  ▶ [1] Docker Compose  (docker compose up — needs Docker Desktop)
    [2] Kubernetes (kind)  (kubectl apply — needs kind + kubectl)

── Optional Services
  OpenSearch / Elasticsearch (advanced full-text search)? [y/N]:
  LDAP authentication (local OpenLDAP with stub users)? [y/N]:
  SAML / OIDC authentication (local Keycloak IdP)? [y/N]:
  MinIO (local S3-compatible file storage)? [y/N]:
  Prometheus + Grafana (metrics and dashboards)? [y/N]:
  Calls / RTCD (local video/voice calls container)? [y/N]:
  ngrok tunnel (public HTTPS URL for phone/remote testing)? [y/N]:

── Summary
  ✓ Mattermost:  mattermost/mattermost-team-edition:10.5.0  (team edition)
  ✓ Database:    postgres
  ✓ Topology:    single-node
  ✓ Output:      Docker Compose
  ✓ Extras:      none (bare minimum)
  ✓ Mailpit:     always included — captures all outgoing emails

  Generate this environment? [Y/n]:
```

After confirmation it generates the project and prints the same `cd / make run` next steps.

---

## Add More Services

Need to match a more complex setup? Just add flags:

```bash
mm-repro init --support-package ./support-package.zip \
  --with-ldap          # OpenLDAP (LDAP authentication)
  --with-saml          # Keycloak (SAML / OIDC)
  --with-opensearch      # OpenSearch (search issues)
  --with-elasticsearch   # Elasticsearch (search issues — use when customer used ES)
  --with-minio         # MinIO (S3 file storage)
  --with-grafana       # Prometheus + Grafana (metrics)
  --with-ngrok         # public HTTPS tunnel (test from your phone!)
  --with-kubernetes    # generate kind manifests instead of Compose
  --image-registry registry.internal:5000   # airgapped/private registry (see below)
```

Mix and match whatever the environment needs.

---

## Airgapped / Private Registry

**What it does — one thing only:** prefixes every generated image reference with a registry URL you control. Everything else (config, topology, services, volumes, env vars) is completely unchanged.

### Why it exists

When running in a corporate Kubernetes cluster where Docker Hub is blocked by firewall policy, all images must be pulled from an internal registry (`example.corp.com`) instead.

Without the flag, mm-repro generates:
```yaml
image: postgres:15-alpine
image: mattermost/mattermost-enterprise-edition:10.5.0
```
Docker tries to pull these from Docker Hub at `make run` time — which fails on a network without internet access.

With `--image-registry example.corp.com`, mm-repro generates:
```yaml
image: example.corp.com/postgres:15-alpine
image: example.corp.com/mattermost/mattermost-enterprise-edition:10.5.0
```
Docker pulls from the internal registry instead. That's the entire difference — the compose file, Makefile, env vars, nginx config, and all service configuration are identical in both cases.

### Usage

**CLI flag:**
```bash
mm-repro init --support-package ./support-package.zip \
  --image-registry registry.internal:5000
```

**Interactive wizard** — the last question in "Optional Services":
```
Private / airgapped registry  (prefix all images with a custom registry URL)? [y/N]: y
Registry URL (e.g. registry.internal:5000): registry.internal:5000
```

### Pre-load images onto your private registry

Run this on an internet-connected machine first, then use the registry on the airgapped host:

```bash
REGISTRY=registry.internal:5000

for IMAGE in \
  mattermost/mattermost-enterprise-edition:10.5.0 \
  postgres:15-alpine \
  mysql:8.0 \
  opensearchproject/opensearch:2.11.0 \
  "docker.elastic.co/elasticsearch/elasticsearch:8.11.0" \
  osixia/openldap:1.5.0 \
  osixia/phpldapadmin:0.9.0 \
  "quay.io/keycloak/keycloak:23.0" \
  minio/minio:latest \
  axllent/mailpit:latest \
  nginx:alpine \
  mattermost/rtcd:latest \
  prom/prometheus:latest \
  grafana/grafana:latest \
  ngrok/ngrok:latest; do
    docker pull "$IMAGE"
    docker tag "$IMAGE" "$REGISTRY/$IMAGE"
    docker push "$REGISTRY/$IMAGE"
done
```

> **No flag = no change.** Omitting `--image-registry` keeps all original image references — zero impact on existing workflows.

---

## Services at a Glance

Every service mm-repro can spin up, what it does, and how to reach it:

| Service | What it does | URL | Credentials |
|---------|-------------|-----|-------------|
| **Mattermost** | The collaboration platform itself — what you're testing | http://localhost:8065 | `sysadmin` / `Sysadmin1!` |
| **PostgreSQL** | Relational database that stores all Mattermost data (channels, messages, users) | `localhost:5432` (host port) | user: `mmuser` / pass: `mmuser_password_local_repro_only` / db: `mattermost` |
| **MySQL** | Alternative to PostgreSQL — used when detected in the package or forced with `--db mysql` | `localhost:3306` (host port) | user: `mmuser` / pass: `mmuser_password_local_repro_only` / db: `mattermost` · root pass: `root_password_local_repro_only` |
| **Mailpit** | Catches every outgoing email so nothing reaches real inboxes — view captured emails in its web UI | http://localhost:8025 | No login |
| **nginx** | Load balancer in front of all Mattermost nodes in HA mode — distributes traffic evenly across the cluster | http://localhost:8065 | routes to all nodes |
| **MinIO** | Local S3-compatible object storage — Mattermost stores file attachments here instead of on disk; required for HA so all nodes share one storage backend | http://localhost:9001 | `repro_minio_user` / `minio_password_local_repro_only` |
| **OpenSearch** | Full-text search engine — faster and richer than database search. Use when the support package used OpenSearch (`--with-opensearch`). Auto-detected from support package. | http://localhost:9200 | No login |
| **Elasticsearch** | Full-text search engine — exact match for customers running Elasticsearch. Use when the support package used Elasticsearch (`--with-elasticsearch`). Auto-detected from support package. | http://localhost:9200 | No login |
| **OpenLDAP** | LDAP directory server for testing LDAP auth flows — run `make ldap-users` to load 8 test users after startup | internal | auto-generated |
| **phpLDAPadmin** | Web UI to browse and manage the OpenLDAP directory — view users, groups, and attributes | http://localhost:8089 | `cn=admin,dc=repro,dc=local` / `ldap_admin_local_repro_only` |
| **Keycloak** | Local Azure AD / IdP simulation — OIDC ("Sign in with GitLab", no license needed) + SAML ("Sign in with SAML", needs Enterprise license). Realm auto-imported on startup. Run `make azure-ad` for status. | http://localhost:8080 | `admin` / `keycloak_admin_local_repro_only` |
| **Prometheus** | Scrapes and stores performance metrics from Mattermost every 15 seconds | http://localhost:9090 | No login |
| **Grafana** | Dashboard for visualising Prometheus metrics — API latency, DB queries, memory, active users | http://localhost:3000 | `admin` / `grafana_admin_local_repro_only` |
| **RTCD** | Real-Time Communications Daemon — handles WebRTC signalling for Mattermost Calls (video/voice) | internal | auto-generated |
| **ngrok** | Creates a public HTTPS tunnel so you can open Mattermost on a phone or share it remotely | http://localhost:4040 (inspector) | No login |

> Services listed as **internal** are not exposed on a host port — Mattermost connects to them over Docker's internal network.

**Connect to the database directly** (any SQL client, TablePlus, DBeaver, psql, mysql CLI):

```bash
# PostgreSQL
psql "postgres://mmuser:mmuser_password_local_repro_only@localhost:5432/mattermost"

# MySQL
mysql -h 127.0.0.1 -P 3306 -u mmuser -pmmuser_password_local_repro_only mattermost
# or as root (MySQL only):
mysql -h 127.0.0.1 -P 3306 -u root -proot_password_local_repro_only mattermost
```

---

## 📱 Test From Your Phone — ngrok

Add `--with-ngrok` to get a public HTTPS URL you can open on any device:

```bash
mm-repro init --support-package ./support-package.zip --with-ngrok
cd generated-repro/20250310-143022/   # use the path printed by init
make run

make ngrok-url   # → https://abc123.ngrok.io
make mobile      # alias for the above
```

The ngrok container starts automatically with `make run`. For a stable URL across restarts, drop your free token in `.env`:
```
NGROK_AUTHTOKEN=your_token_here
```
Get one free at [dashboard.ngrok.com](https://dashboard.ngrok.com/get-started/your-authtoken).

---

## 🖥️ Customer on a HA Cluster? Covered.

If the support package came from a **multi-node (High Availability) deployment**, mm-repro auto-detects this and spins up multiple Mattermost containers behind an nginx load balancer — no extra flags needed:

```bash
mm-repro init --support-package ./support-package.zip
# → Topology: multi-node (3 nodes detected)
# → nginx load balancer + MinIO (shared file storage) auto-enabled

cd generated-repro/20250310-143022/   # use the path printed by init
make run
open http://localhost:8065   # nginx routes to any of the nodes
```

**What gets auto-configured for you:**
- 🔁 Multiple `mattermost-1`, `mattermost-2`, `mattermost-3` containers (capped at 3 for local resources)
- ⚖️ **nginx** load balancer in front, same `http://localhost:8065` URL
- 📦 **MinIO** automatically enabled — shared file storage is required for HA (local filesystem can't be shared across nodes)

Or force it explicitly:
```bash
mm-repro init --support-package ./support-package.zip --force-multi-node
# Force single-node even when cluster was detected:
mm-repro init --support-package ./support-package.zip --force-single-node
```

> Note: if the original cluster had more than 3 nodes, it's capped at 3. The `REPRO_SUMMARY.md` will note the approximation.

---

## ☸️ Customer on Kubernetes? Covered.

If the support package came from a Kubernetes deployment, mm-repro **auto-detects** this and generates a local [kind](https://kind.sigs.k8s.io/) cluster instead of Docker Compose:

```bash
mm-repro init --support-package ./support-package.zip
# → Output: kubernetes  (auto-detected from pod naming patterns)

cd generated-repro/20250310-143022/   # use the path printed by init
make run
open http://localhost:30065
```

Or force it explicitly:
```bash
mm-repro init --support-package ./support-package.zip --with-kubernetes
# Force Compose even when k8s is detected:
mm-repro init --support-package ./support-package.zip --force-docker-compose
```

**Prerequisites for k8s mode:** Docker Desktop + [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) + [kubectl](https://kubernetes.io/docs/tasks/tools/)

See [docs/kubernetes.md](docs/kubernetes.md) for the full Kubernetes guide.

---

## 🌱 Seed Test Data

After `make run`, first create the admin account, then seed content:

```bash
make admin              # creates sysadmin / Sysadmin1! (run once)
make seed PASS=Sysadmin1!  # seeds 20 posts, threads, reactions
```

Or with the CLI directly for more control:

```bash
# Quick seed (20 posts, no files)
mm-repro seed --project ./generated-repro/20250310-143022/ --password Sysadmin1!

# Full seed with file attachments
mm-repro seed --project . --with-files --posts 30 --password Sysadmin1!
```

**Custom channels:**

```bash
# Create channels (no posts seeded)
make channels NAMES="support,bugs,release-notes" PASS=Sysadmin1!

# Seed posts into a specific channel only
make seed CHANNEL=support PASS=Sysadmin1!

# Create channels and seed into one in a single command
make seed CHANNELS="support,bugs" CHANNEL=support PASS=Sysadmin1!
```

**What gets created:**

| Content | Details |
|---------|---------|
| 📝 **Text posts** | Markdown, tables, code blocks, blockquotes, lists |
| 🧵 **Threads** | Nested replies to test threaded conversations |
| 😄 **Reactions** | Random emoji reactions on posts |
| 🔗 **Links** | Posts with URLs that trigger link previews |
| 📸 **Images** (`--with-files`) | Coloured PNG screenshots attached inline |
| 📄 **Log files** (`--with-files`) | `.log` text file attachments with preview |

Posts land in both `~town-square` and `~off-topic` by default, or in your chosen `--channel`. Run multiple times to add more content — each run is additive, nothing is overwritten.

> **First run?** If Mattermost's setup wizard hasn't been completed yet, open `http://localhost:8065` first and create your admin account. Then run `mm-repro seed` — it will connect and log in with the credentials you provide.

---

## 👥 LDAP Test Users

If your repro was generated with LDAP enabled (detected automatically from the support package, or via `--with-ldap`), a `ldap/users.ldif` file is created and the `openldap` container is ready to load it:

```bash
make ldap-users
```

This loads 8 test users and 4 groups into the running OpenLDAP container. Re-run safely — existing entries are silently skipped.

| User | Title | Groups |
|------|-------|--------|
| `alice.johnson` | Developer | staff, developers |
| `bob.smith` | Developer | staff, developers |
| `carol.white` | Team Lead | staff, developers, management |
| `dave.brown` | Designer | staff |
| `eve.davis` | QA Engineer | staff, support |
| `frank.miller` | Support Engineer | staff, support |
| `grace.wilson` | Project Manager | staff, management |
| `henry.moore` | System Admin | staff, management |

**Password for all test users:** `Repro1234!`

After loading, trigger a sync via the Makefile (no System Console steps needed — all LDAP settings are already configured):

```bash
make ldap-sync PASS=Sysadmin1!   # requires Enterprise license
```

Or manually in Mattermost: **System Console → Authentication → AD/LDAP → Synchronize Now**. Users can then log in with `uid@repro.local` + `Repro1234!`.

Verify the directory at **http://localhost:8089** (phpLDAPadmin — bind DN: `cn=admin,dc=repro,dc=local`, password: `ldap_admin_local_repro_only`).

---

## 🔑 Azure AD / SAML / OIDC

When your issue involves Azure AD authentication, use `--with-azure-ad` (or it's auto-detected from the support package). This spins up a pre-configured local Keycloak that simulates Azure AD — no real Azure tenant needed.

```bash
mm-repro init --with-azure-ad --support-package ./support-package.zip
cd generated-repro/20250310-143022/
make run && make admin
make azure-ad   # shows OIDC + SAML status + test credentials
```

**Two auth modes, one Keycloak:**

| Mode | License needed | Sign-in button | Works |
|------|---------------|----------------|-------|
| **OIDC** (Entra ID simulation) | No | "Sign in with GitLab" | ✅ Immediately |
| **SAML** (Azure AD SAML 2.0) | Yes (Enterprise) | "Sign in with SAML" | ✅ After license upload |

Both are fully pre-configured — no System Console steps needed. SAML activates automatically when you upload a license.

**Test users** (same 8 as LDAP, password `Repro1234!`):

| User | Role |
|------|------|
| `alice.johnson` | Developer |
| `bob.smith` | Developer |
| `carol.white` | Team Lead |
| `dave.brown` | Designer |
| `eve.davis` | QA Engineer |
| `frank.miller` | Support Engineer |
| `grace.wilson` | Project Manager |
| `henry.moore` | System Admin |

**Keycloak console:** http://localhost:8080 — login `admin` / `keycloak_admin_local_repro_only`, realm `repro`

> ℹ️ Keycloak automatically imports the realm configuration (`keycloak/repro-realm.json`) on first startup via `--import-realm`. No manual setup required.

---

## 📋 Enterprise License

Mattermost Enterprise features (LDAP sync, SAML SSO) require a license.

> **How it works:** All LDAP/SAML settings are already pre-configured in the generated environment via env vars. The license doesn't change any configuration — it just unlocks the features. **But Mattermost must be restarted after the license is loaded** so it re-reads those env vars with the license now present. mm-repro handles the restart for you automatically.

### Option A — Pre-load at init (simplest)

Pass the license file when you generate the environment. Mattermost starts with the license already loaded — everything works from the very first `make run`, no extra steps:

```bash
mm-repro init --support-package ./support-package.zip --license ./your.mattermost-license
cd generated-repro/20250310-143022/
make run
make admin
# ✓ LDAP and SAML work immediately — done
```

### Option B — Upload via command line after startup

```bash
make run
make admin
make upload-license LICENSE=./your.mattermost-license PASS=Sysadmin1!
# ✓ Uploads the license, then automatically restarts Mattermost
# ✓ LDAP and SAML active — no System Console steps needed
```

### Option C — Upload via System Console (browser)

If you uploaded the license manually through **System Console → About → Edition and License → Upload License**, run this one extra command afterward:

```bash
make restart-mattermost
# ✓ Restarts Mattermost so the env var settings take effect
# ✓ LDAP and SAML active — no System Console steps needed
```

> **Why the restart?** Mattermost skips applying licensed-feature settings at startup when no license is present. Once the license is loaded (by any method), a restart tells Mattermost to re-read all its settings with the license now active.

---

## Security: Safe by Default

mm-repro is built so you can't accidentally leak sensitive data:

- 🔐 **Secrets are replaced** — passwords, keys, DSNs, tokens all become safe `*_local_repro_only` placeholders
- 📧 **Email is captured** — Mailpit intercepts everything, nothing reaches real inboxes
- 🚫 **No outbound connections** — zero telemetry, zero calls to external infrastructure
- 📋 **Full audit trail** — `REDACTION_REPORT.md` lists every credential that was detected and replaced
- 🗑️ **Original ZIP untouched** — mm-repro only reads it, never modifies it

See [docs/security.md](docs/security.md) for the complete threat model.

---

## CLI Quick Reference

| Command | What it does |
|---------|-------------|
| `mm-repro doctor` | Check Docker, ports, and optional tools |
| `mm-repro validate --support-package <zip>` | Inspect what's inside a package |
| `mm-repro plan --support-package <zip>` | Preview the repro plan (no files written) |
| `mm-repro init --support-package <zip>` | Generate from a support package |
| `mm-repro init` | Interactive wizard (no support package needed) |
| `mm-repro run --project <dir>` | Start a generated environment |
| `mm-repro stop --project <dir>` | Stop it |
| `mm-repro reset --project <dir>` | Wipe all data and start fresh |
| `mm-repro seed --project <dir> --posts 0 --password Sysadmin1!` | Create sysadmin account only (no posts) |
| `mm-repro seed --project <dir>` | Seed posts, threads, reactions and files |

Full `init` flags:
```
--support-package  ZIP path (required)
--output           Custom output directory
--issue            Issue name for directory labeling
--db               Force postgres or mysql
--force-single-node / --force-multi-node
--with-opensearch / --with-elasticsearch / --with-ldap / --with-saml
--with-minio / --with-rtcd / --with-grafana
--with-kubernetes / --force-docker-compose
--with-ngrok
--redact-strict    Also redact hostnames and email addresses
```

---

## How It Works (for the curious)

```
support-package.zip
      │
      ▼ Ingest       — unzip, sanitize paths
      ▼ Normalize    — find config.json, plugins list, diagnostics
      ▼ Redact       — replace all secrets with safe placeholders
      ▼ Parse        — extract version, DB type, auth, topology, plugins
      ▼ Infer        — build ReproPlan (what to generate + how)
      ▼ Generate     — write docker-compose.yml / k8s manifests + reports
      │
      └─→ ready-to-run project
```

See [docs/architecture.md](docs/architecture.md) for details.

---

## Troubleshooting

**Docker not running** → Start Docker Desktop and wait for it to fully initialize.

**Port already in use** → Edit the relevant port variable in the generated `.env` (e.g. `MM_PORT=8066`, `PROMETHEUS_PORT=9091`, `MAILPIT_PORT=8026`), then `make run` again.

**LDAP / SAML not working after license upload** → Run `make restart-mattermost`. Mattermost must restart after a license is loaded to apply Enterprise settings. If you used `make upload-license`, the restart is automatic. If you uploaded via the System Console (browser), you need to run `make restart-mattermost` manually.

**Keycloak slow to start** → Normal — it takes 2–3 minutes. Watch with `make logs`.

**k8s pods stuck in Pending** → Docker Desktop needs at least **4 GB RAM** (Preferences → Resources).

**Got k8s manifests but wanted Compose** → Add `--force-docker-compose` to your `init` command.

**Support package format unknown** → Run `mm-repro validate` to see what was detected. The tool still does best-effort extraction.

---

## Contributing

PRs welcome! See [CONTRIBUTING.md](CONTRIBUTING.md).

```bash
git clone https://github.com/rohith0456/mattermost-support-package-repro.git
cd mattermost-support-package-repro
make build && make test
```

High-value areas: new support package formats, additional auth providers, better version detection, test fixtures, docs.

---

## License

MIT — see [LICENSE](LICENSE)
