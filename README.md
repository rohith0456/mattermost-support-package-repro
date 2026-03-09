# mattermost-support-package-repro

> **mm-repro** — Generate local Mattermost reproduction environments from support packages.

[![CI](https://github.com/rohith0456/mattermost-support-package-repro/actions/workflows/ci.yml/badge.svg)](https://github.com/rohith0456/mattermost-support-package-repro/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.22+-blue)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Security](https://img.shields.io/badge/Security-Safe%20by%20Default-green)](docs/security.md)

**FOR LOCAL DEBUGGING ONLY. Never uses real production credentials.**

---

## What This Tool Does

`mm-repro` generates a local reproduction setup from a Mattermost support package.

**One command:**
```bash
mm-repro init --support-package ./customer-support-package.zip
```

**Generates:**
- A ready-to-run Docker Compose environment
- Matching Mattermost version (or closest available)
- Matching database type (PostgreSQL or MySQL)
- Optional services: OpenSearch, LDAP, Keycloak, MinIO, MailHog, Prometheus, Grafana
- Safe local credentials (never reuses customer secrets)
- Detailed reports: what was recreated, approximated, and skipped

Then start with:
```bash
cd generated-repro/<timestamp>/
make run
# open http://localhost:8065
```

---

## Get Started in 5 Minutes

> No YAML editing. No manual config. Just point at the ZIP and go.

### Step 1 — Install Docker Desktop

Download and start **[Docker Desktop](https://www.docker.com/products/docker-desktop/)** for your OS (Mac, Windows, or Linux). Make sure it is running before continuing.

---

### Step 2 — Install mm-repro

**Mac or Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/rohith0456/mattermost-support-package-repro/main/scripts/install.sh | bash
```

**Windows** — open PowerShell and run:
```powershell
winget install golang.go          # installs Go if not already installed
go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest
```
Then add `%USERPROFILE%\go\bin` to your `PATH` if it isn't already.

**Verify the install:**
```bash
mm-repro doctor
```
You should see all green checks. If Docker is flagged, make sure Docker Desktop is running.

---

### Step 3 — Get the Support Package ZIP

Download the customer's support package ZIP from your ticket system (Zendesk, Jira, etc.) to your computer. For example:

```
~/Downloads/customer-support-package.zip
```

> The ZIP is read-only — mm-repro never modifies the original file.

---

### Step 4 — Generate the Environment

Run this one command, pointing at the ZIP you just downloaded:

```bash
mm-repro init --support-package ~/Downloads/customer-support-package.zip
```

mm-repro will:
- Detect the Mattermost version, database type, auth setup, and plugins
- Generate a ready-to-run Docker Compose project in `./generated-repro/<customer-name>/`
- Replace all real credentials with safe local-only ones
- Print a summary of what was detected

**Optional flags** — add any that match the customer's setup:
```bash
mm-repro init --support-package ~/Downloads/customer.zip \
  --with-ldap        # include OpenLDAP (LDAP auth)
  --with-saml        # include Keycloak (SAML / OIDC auth)
  --with-opensearch  # include OpenSearch (search issues)
  --with-minio       # include MinIO (S3 file storage)
  --with-grafana     # include Prometheus + Grafana (metrics)
```

---

### Step 5 — Start the Environment

```bash
cd generated-repro/<customer-name>/
make run
```

Wait about 30–60 seconds for all containers to come up (Keycloak takes the longest). Then open:

```
http://localhost:8065
```

You'll see a fresh Mattermost instance matching the customer's version and configuration.

---

### Step 6 — Read the Reports

Three reports are generated alongside the environment:

| File | What it tells you |
|------|------------------|
| `REPRO_SUMMARY.md` | What was recreated, approximated, and skipped |
| `REDACTION_REPORT.md` | What credentials were detected and replaced |
| `PLUGIN_REPORT.md` | Which plugins were detected and their install status |

---

### Step 7 — Reproduce the Issue

Log in and reproduce the issue in the local environment. All emails are captured by MailHog at `http://localhost:8025` — nothing is sent to real email addresses.

#### Default Test Users

All users below are pre-created in both LDAP and Keycloak (OIDC) when those services are included. Password for all: **`Repro1234!`**

| Username | Full Name | Email | Role |
|----------|-----------|-------|------|
| `alice.johnson` | Alice Johnson | alice.johnson@repro.local | Developer |
| `bob.smith` | Bob Smith | bob.smith@repro.local | Developer |
| `carol.white` | Carol White | carol.white@repro.local | Team Lead |
| `dave.brown` | Dave Brown | dave.brown@repro.local | Designer |
| `eve.davis` | Eve Davis | eve.davis@repro.local | QA Engineer |
| `frank.miller` | Frank Miller | frank.miller@repro.local | Support Engineer |
| `grace.wilson` | Grace Wilson | grace.wilson@repro.local | Project Manager |
| `henry.moore` | Henry Moore | henry.moore@repro.local | System Admin |

#### Default LDAP Groups

| Group | Members |
|-------|---------|
| `staff` | All 8 users |
| `developers` | alice.johnson, bob.smith, carol.white |
| `support` | eve.davis, frank.miller |
| `management` | carol.white, grace.wilson |
| `admins` | henry.moore |

#### Service URLs

| Service | URL | Credentials |
|---------|-----|-------------|
| Mattermost | http://localhost (nginx) or http://localhost:8065 (direct) | Sign in with LDAP users above |
| MailHog (email capture) | http://localhost:8025 | No login required |
| MinIO (file storage) | http://localhost:9001 | `minioadmin` / `minio_local_repro_only` |
| Keycloak (OIDC admin) | http://localhost:8080 | `admin` / `keycloak_admin_local_repro_only` |
| phpLDAPadmin | http://localhost:8089 | `cn=admin,dc=repro,dc=local` / `ldap_admin_local_repro_only` |

---

### Step 8 — Clean Up When Done

```bash
# Stop containers (keeps data):
make stop

# Or fully reset — removes all containers and volumes:
make reset
```

Then delete the generated folder and the support package ZIP from your machine.

---

## Why It Exists

Setting up a local environment to reproduce a Mattermost issue is slow, manual, and easy to get wrong. `mm-repro` automates that entirely — point it at a support package ZIP and get a running local environment in minutes, with no manual config and no risk of leaking real credentials.

---

## What It Can and Cannot Recreate

### Can Recreate (or Approximate)

| Feature | Approach |
|---------|----------|
| Mattermost version | Exact Docker image tag when available |
| Single-node topology | Local Docker container |
| Multi-node HA cluster | Multiple local containers + nginx |
| PostgreSQL database | Local PostgreSQL container |
| MySQL database | Local MySQL container (when detected) |
| Elasticsearch/OpenSearch | Local OpenSearch container |
| LDAP authentication | Local OpenLDAP with stub users |
| SAML/OIDC authentication | Local Keycloak |
| S3 object storage | Local MinIO |
| SMTP email | Local MailHog (captures all mail) |
| Calls/RTCD | Local RTCD container |
| Metrics/Observability | Local Prometheus + Grafana |
| Marketplace plugins | Auto-install from official marketplace |

### Cannot Recreate

| Feature | Reason |
|---------|--------|
| Real customer data | Privacy and security |
| Real customer users/channels | Privacy |
| Live production traffic | Out of scope |
| Custom/proprietary plugins | Not publicly available |
| Real cloud storage | No customer credentials used |
| Real LDAP directory | No customer server access |
| Production network topology | Local environment only |
| Real SSL certificates | Self-signed only |
| Exact production performance | Hardware differences |
| Real email sending | Always intercepted by MailHog |

---

## Security Model

**This tool is designed to be safe by default.** See [docs/security.md](docs/security.md) for the full threat model.

Key guarantees:
1. **Never reuses customer secrets** — all credentials are freshly generated for local use only
2. **Never connects to external services** — no outbound connections to customer infrastructure
3. **Redacts sensitive values** — passwords, keys, tokens, and DSNs are replaced with placeholders
4. **Email is always captured** — MailHog intercepts all outbound email
5. **Reports what was redacted** — `REDACTION_REPORT.md` lists every redacted category
6. **Explicit opt-in for sensitive services** — LDAP, SAML, MinIO are disabled unless requested

---

## Quick Start

### Prerequisites
- Docker Desktop (Mac, Windows, Linux) — [download](https://www.docker.com/products/docker-desktop/)
- Go 1.22+ (for building from source)

### Install

**macOS / Linux (pre-built binary, recommended):**
```bash
curl -fsSL https://raw.githubusercontent.com/rohith0456/mattermost-support-package-repro/main/scripts/install.sh | bash
```

**Go install (builds from source, requires Go 1.22+):**
```bash
go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest
```

**From source:**
```bash
git clone https://github.com/rohith0456/mattermost-support-package-repro.git
cd mattermost-support-package-repro
make build
# Binary: ./bin/mm-repro
```

**Check prerequisites:**
```bash
mm-repro doctor
```

### Example Workflow

```bash
# 1. Download the customer support package
# (from a Jira ticket, Zendesk, etc.)

# 2. Validate the package
mm-repro validate --support-package ./customer.zip

# 3. Preview the repro plan (no files generated)
mm-repro plan --support-package ./customer.zip

# 4. Generate the repro environment
mm-repro init --support-package ./customer.zip

# 5. Review the reports
cat generated-repro/<timestamp>/REPRO_SUMMARY.md
cat generated-repro/<timestamp>/REDACTION_REPORT.md

# 6. Start the environment
cd generated-repro/<timestamp>/
make run

# 7. Open Mattermost
open http://localhost:8065

# 8. When done, stop and clean up
make stop
# or completely remove:
make reset
```

### With Optional Services

```bash
# LDAP authentication
mm-repro init --support-package ./customer.zip --with-ldap

# SAML/OIDC via Keycloak
mm-repro init --support-package ./customer.zip --with-saml

# S3 storage via MinIO
mm-repro init --support-package ./customer.zip --with-minio

# OpenSearch (for search issues)
mm-repro init --support-package ./customer.zip --with-opensearch

# Full stack
mm-repro init --support-package ./customer.zip \
  --with-ldap --with-opensearch --with-minio --with-grafana
```

---

## CLI Reference

### `mm-repro init`
Parse a support package and generate a repro project.
```
Flags:
  --support-package <path>   Support package ZIP (required)
  --output <dir>             Output directory (default: ./generated-repro/<timestamp>)
  --issue <name>             Issue name for directory naming
  --db postgres|mysql        Force database type
  --force-single-node        Force single-node (ignore cluster signals)
  --force-multi-node         Force multi-node
  --with-opensearch          Include OpenSearch
  --with-ldap                Include OpenLDAP
  --with-saml                Include Keycloak (SAML/OIDC)
  --with-minio               Include MinIO (S3 storage)
  --with-rtcd                Include RTCD (Calls)
  --with-grafana             Include Prometheus + Grafana
  --redact-strict            Strict redaction (also redacts server addresses, emails)
```

### `mm-repro plan`
Preview the repro plan without generating files.
```
  --support-package <path>   Required
  --json                     Output as JSON
```

### `mm-repro validate`
Validate support package and show available signals.
```
  --support-package <path>   Required
```

### `mm-repro doctor`
Check Docker, disk space, and port availability.

### `mm-repro run|stop|reset`
```
  --project <path>           Path to generated repro project
```

---

## Architecture Overview

```
mm-repro init --support-package ./customer.zip
       │
       ▼
┌──────────────────┐
│  Ingestion Layer │  Unzip, sanitize paths, build file index
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│   Normalizer     │  Extract config.json, diagnostics, plugins, logs
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Redaction Engine │  Detect + replace secrets with safe placeholders
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│   Parser Layer   │  Extract version, topology, DB, auth, plugins, etc.
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Inference Engine │  Build ReproPlan from signals + flags
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│Generator Layer   │  Render docker-compose.yml, .env, reports, README
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│  Output Project  │  Ready-to-run repro environment
└──────────────────┘
```

See [docs/architecture.md](docs/architecture.md) for details.

---

## Supported Features

- Mattermost versions: 7.x, 8.x, 9.x, 10.x, 11.x
- Support package formats: standard, sanitized, cloud
- Databases: PostgreSQL, MySQL
- Search: OpenSearch/Elasticsearch (approximated with OpenSearch)
- Auth: LDAP (OpenLDAP), SAML/OIDC (Keycloak)
- Storage: local filesystem, S3 (MinIO)
- Email: MailHog
- Calls: RTCD
- Observability: Prometheus, Grafana
- Topology: single-node, multi-node (up to 3 nodes locally)
- Plugins: auto-install from official marketplace

---

## Limitations

- Cannot access or use real customer data
- Cannot exactly replicate production performance characteristics
- Multi-node is limited to 3 nodes for local resource reasons
- Custom plugins require manual installation
- SAML certificates are regenerated locally (never reuses customer certs)
- Cloud storage uses MinIO (not the real S3/Azure service)
- LDAP uses a stub directory with test users only
- Cluster networking is simplified vs. production

---

## Troubleshooting

### "Docker is not running"
Start Docker Desktop and ensure it's fully initialized.

### "Port 8065 is already in use"
Stop any other Mattermost instance or specify a different port:
```bash
# Edit the generated .env file:
MM_PORT=8066  # then re-run make run
```

### "Support package format unknown"
The parser will still attempt best-effort extraction. Run `mm-repro validate` to see what was found.

### "Mattermost image not found"
If the exact version image doesn't exist on Docker Hub, the tool will use a nearby version and document the approximation in REPRO_SUMMARY.md.

### Keycloak takes a long time to start
Keycloak can take 2-3 minutes to initialize. Use `make logs` to monitor.

---

## FAQ

**Q: Will this tool send any data to Mattermost or external servers?**
A: No. mm-repro is entirely local. No telemetry, no outbound connections.

**Q: Can I use this for non-Mattermost issues?**
A: No. It is purpose-built for Mattermost support packages.

**Q: Is it safe to run on an untrusted support package?**
A: The tool sanitizes file paths and limits extraction sizes, but always use reasonable caution with files from external sources.

**Q: What if the support package is from an older Mattermost version?**
A: The tool will attempt to match the closest available Docker image and document any approximations.

**Q: Can I commit the generated repro directory?**
A: No. The `.gitignore` in `generated/` prevents this. The `.env` file contains local credentials that should never be committed even though they are not production credentials.

---

## How Support Engineers Should Use This

1. **Get the support package** from the ticket (Zendesk/Jira attachment)
2. **Run validate** first to understand what's in the package
3. **Run plan** to preview what will be created
4. **Run init** with any needed optional services (--with-ldap, etc.)
5. **Read the reports** — especially REPRO_SUMMARY.md and PLUGIN_REPORT.md
6. **Start the environment** with `make run`
7. **Reproduce the issue** in the local environment
8. **Use `make reset`** when done to clean up volumes
9. **Delete the generated directory** when finished debugging
10. **Never share** the generated `.env` or support package contents

---

## How Generated Projects Work

Each generated project contains:

```
generated-repro/<timestamp>/
├── docker-compose.yml      # All services configured for local use
├── .env                    # Local-only credentials (never production secrets)
├── Makefile                # run / stop / reset / logs shortcuts
├── config/                 # Generated service configs (nginx, prometheus, etc.)
├── scripts/
│   ├── start.sh
│   ├── stop.sh
│   └── reset.sh
├── README.md               # Project-specific quick start
├── REPRO_SUMMARY.md        # What was recreated / approximated / skipped
├── REDACTION_REPORT.md     # What was redacted from the support package
├── PLUGIN_REPORT.md        # Plugin detection and installation status
└── repro-plan.json         # Machine-readable full plan
```

---

## How Redaction Works

The redaction engine scans the parsed config for known sensitive key patterns and replaces their values with labeled placeholders **before** any further processing. Original values are never stored, logged, printed, or committed.

Redacted categories (default):
- Database passwords and connection strings
- LDAP bind passwords
- SAML private keys and certificates
- OAuth client secrets
- SMTP credentials
- Cloud storage access keys and secrets
- Encryption keys and salts
- License file contents
- Webhook secrets
- Plugin API keys and tokens

With `--redact-strict`:
- Server hostnames and addresses
- Admin email addresses

See [docs/security.md](docs/security.md) for the complete redaction model.

---

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Areas where contributions are especially valuable:
- New support package format support
- Additional service modules (more LDAP implementations, more IdPs)
- Improved version detection
- Plugin marketplace integration
- Additional test fixtures
- Documentation improvements

### Development Setup

```bash
git clone https://github.com/rohith0456/mattermost-support-package-repro.git
cd mattermost-support-package-repro
make tidy
make build
make test
```

### Running Tests

```bash
make test               # all tests
make test-short         # skip integration tests
make test-coverage      # with HTML coverage report
```

---

## Safe Handling of Support Packages

Support packages may contain sensitive customer information. Follow these practices:
- **Download** support packages only from official ticket systems (Jira, Zendesk)
- **Never** commit support packages to version control (`.gitignore` is configured for this)
- **Delete** support packages and generated repro directories when debugging is complete
- **Never** share support packages over unencrypted channels
- **Never** upload support packages to untrusted services or AI tools
- **Use** `mm-repro validate` to understand what data is present before processing

---

## License

MIT — see [LICENSE](LICENSE)
