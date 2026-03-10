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
cd generated-repro/<timestamp>/
make run
open http://localhost:8065
```

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
cd generated-repro/<timestamp>/
make run
```

**Without a support package** — interactive wizard picks your setup:
```bash
mm-repro init
# → answers a few questions, then generates and runs
cd generated-repro/<timestamp>/
make run
```

Open `http://localhost:8065` — Mattermost is running. All emails go to MailHog at `http://localhost:8025`, nothing real gets sent.

### 4 — Seed Test Content

Once Mattermost is running, populate it with posts, threads, reactions, images and file attachments in one command:

```bash
make seed PASS=YourAdminPassword
```

Or with the CLI directly for more control:

```bash
mm-repro seed --project . --with-files --posts 30 --password YourAdminPassword
```

This fills `~town-square` and `~off-topic` with varied test content — markdown formatting, code blocks, link unfurling, threaded conversations, emoji reactions, and (with `--with-files`) PNG screenshots and log file attachments.

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
generated-repro/<timestamp>/
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
generated-repro/<timestamp>/
├── docker-compose.yml     ← mattermost-1/2/3 + nginx + minio
├── nginx/nginx.conf       ← load balancer config (auto-generated)
├── config/prometheus.yml  ← scrape config for all nodes (if Grafana enabled)
└── ...same files as above
```

**Kubernetes mode — instead of Compose:**
```
generated-repro/<timestamp>/
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
  OpenSearch (advanced full-text search)? [y/N]:
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
  ✓ MailHog:     always included — captures all outgoing emails

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
  --with-opensearch    # OpenSearch (search issues)
  --with-minio         # MinIO (S3 file storage)
  --with-grafana       # Prometheus + Grafana (metrics)
  --with-ngrok         # public HTTPS tunnel (test from your phone!)
  --with-kubernetes    # generate kind manifests instead of Compose
```

Mix and match whatever the environment needs.

---

## Default Test Users

All environments come with 8 pre-built users. Password for all: **`Repro1234!`**

| Username | Role |
|----------|------|
| `alice.johnson` | Developer |
| `bob.smith` | Developer |
| `carol.white` | Team Lead |
| `dave.brown` | Designer |
| `eve.davis` | QA Engineer |
| `frank.miller` | Support Engineer |
| `grace.wilson` | Project Manager |
| `henry.moore` | System Admin |

When `--with-ldap` is included, all users are pre-loaded in OpenLDAP too.

---

## Service URLs at a Glance

| Service | URL | Credentials |
|---------|-----|-------------|
| **Mattermost** | http://localhost:8065 | Users above |
| **nginx** (multi-node load balancer) | http://localhost:8065 | — routes to all nodes |
| **MailHog** (email capture) | http://localhost:8025 | No login |
| **MinIO** (S3 storage) | http://localhost:9001 | `minioadmin` / `minio_local_repro_only` |
| **Keycloak** (SAML/OIDC) | http://localhost:8080 | `admin` / `keycloak_admin_local_repro_only` |
| **phpLDAPadmin** | http://localhost:8089 | `cn=admin,dc=repro,dc=local` / `ldap_admin_local_repro_only` |
| **Grafana** | http://localhost:3000 | `admin` / `admin` |
| **ngrok inspector** | http://localhost:4040 | No login |

---

## 📱 Test From Your Phone — ngrok

Add `--with-ngrok` to get a public HTTPS URL you can open on any device:

```bash
mm-repro init --support-package ./support-package.zip --with-ngrok
cd generated-repro/<timestamp>/
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

cd generated-repro/<timestamp>/
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

cd generated-repro/<timestamp>/
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

After the environment is running, `mm-repro seed` fills it with realistic test content so you can start testing immediately — no manual post creation needed.

```bash
# Quick seed (20 posts, no files)
mm-repro seed --project ./generated-repro/<timestamp>/ --password YourAdminPassword

# Full seed with file attachments
mm-repro seed --project . --with-files --posts 30 --password YourAdminPassword

# Or use the Makefile shortcut from the generated project directory
make seed PASS=YourAdminPassword
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

Posts land in both `~town-square` and `~off-topic`. Run multiple times to add more content — each run is additive, nothing is overwritten.

> **First run?** If Mattermost's setup wizard hasn't been completed yet, open `http://localhost:8065` first and create your admin account. Then run `mm-repro seed` — it will connect and log in with the credentials you provide.

---

## Security: Safe by Default

mm-repro is built so you can't accidentally leak sensitive data:

- 🔐 **Secrets are replaced** — passwords, keys, DSNs, tokens all become safe `*_local_repro_only` placeholders
- 📧 **Email is captured** — MailHog intercepts everything, nothing reaches real inboxes
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
| `mm-repro seed --project <dir>` | Seed posts, threads, reactions and files |

Full `init` flags:
```
--support-package  ZIP path (required)
--output           Custom output directory
--issue            Issue name for directory labeling
--db               Force postgres or mysql
--force-single-node / --force-multi-node
--with-opensearch / --with-ldap / --with-saml
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

**Port 8065 in use** → Edit `MM_PORT=8066` in the generated `.env`, then `make run` again.

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
