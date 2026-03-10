# mm-repro ⚡

> **Point at a Mattermost support package ZIP. Get a running local environment in minutes.**

[![CI](https://github.com/rohith0456/mattermost-support-package-repro/actions/workflows/ci.yml/badge.svg)](https://github.com/rohith0456/mattermost-support-package-repro/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.22+-blue)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## The Problem It Solves

A customer files a ticket. You need to reproduce their Mattermost issue locally.

Normally: **hours of manual setup** — hunting down the right version, wiring up their database type, recreating their auth config.

With mm-repro: **one command.**

```bash
mm-repro init --support-package ./customer-support-package.zip
```

It reads the ZIP, figures out their exact setup, and generates a ready-to-run environment — Docker Compose **or** Kubernetes, matching their version and config, with all secrets safely replaced. Then:

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

```bash
mm-repro init --support-package ~/Downloads/customer.zip
cd generated-repro/<timestamp>/
make run
```

Open `http://localhost:8065` — Mattermost is running, matching the customer's version and config. All emails go to MailHog at `http://localhost:8025`, nothing real gets sent.

### 4 — Reproduce, then Clean Up

```bash
make stop    # stop (keeps data)
make reset   # nuke everything and start fresh
```

---

## What Gets Generated

Running `mm-repro init` produces a complete project directory:

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

No manual YAML editing. No hunting for the right image tag. No credential leaks.

---

## Add More Services

Need to match a more complex customer setup? Just add flags:

```bash
mm-repro init --support-package ./customer.zip \
  --with-ldap          # OpenLDAP (LDAP authentication)
  --with-saml          # Keycloak (SAML / OIDC)
  --with-opensearch    # OpenSearch (search issues)
  --with-minio         # MinIO (S3 file storage)
  --with-grafana       # Prometheus + Grafana (metrics)
  --with-ngrok         # public HTTPS tunnel (test from your phone!)
  --with-kubernetes    # generate kind manifests instead of Compose
```

Mix and match whatever matches the customer's stack.

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
mm-repro init --support-package ./customer.zip --with-ngrok
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

## ☸️ Customer on Kubernetes? Covered.

If the support package came from a Kubernetes deployment, mm-repro **auto-detects** this and generates a local [kind](https://kind.sigs.k8s.io/) cluster instead of Docker Compose:

```bash
mm-repro init --support-package ./customer.zip
# → Output: kubernetes  (auto-detected from pod naming patterns)

cd generated-repro/<timestamp>/
make run
open http://localhost:30065
```

Or force it explicitly:
```bash
mm-repro init --support-package ./customer.zip --with-kubernetes
# Force Compose even when k8s is detected:
mm-repro init --support-package ./customer.zip --force-docker-compose
```

**Prerequisites for k8s mode:** Docker Desktop + [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) + [kubectl](https://kubernetes.io/docs/tasks/tools/)

See [docs/kubernetes.md](docs/kubernetes.md) for the full Kubernetes guide.

---

## Security: Safe by Default

mm-repro is built so you can't accidentally leak customer data:

- 🔐 **Secrets are replaced** — passwords, keys, DSNs, tokens all become safe `*_local_repro_only` placeholders
- 📧 **Email is captured** — MailHog intercepts everything, nothing reaches real inboxes
- 🚫 **No outbound connections** — zero telemetry, zero calls to customer infrastructure
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
| `mm-repro init --support-package <zip>` | Generate the full repro project |
| `mm-repro run --project <dir>` | Start a generated environment |
| `mm-repro stop --project <dir>` | Stop it |
| `mm-repro reset --project <dir>` | Wipe all data and start fresh |

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
