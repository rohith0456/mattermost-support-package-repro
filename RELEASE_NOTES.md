# Release Notes — v0.1.0

**Released:** 2024-03-01

## mm-repro v0.1.0 — Initial Release

We're excited to release the first version of `mm-repro`, a tool for Mattermost support engineers to generate local reproducible environments from customer support packages.

### What's New

#### Core Features
- **`mm-repro init`** — Parse a Mattermost support package and generate a ready-to-run Docker Compose environment
- **`mm-repro plan`** — Preview the inferred repro plan without generating files
- **`mm-repro validate`** — Validate a support package and report available signals
- **`mm-repro doctor`** — Check local prerequisites (Docker, disk space, ports)
- **`mm-repro run/stop/reset`** — Manage generated repro environments

#### Support Package Parsing
- Version detection from config, diagnostics, system info, and log snippets
- Database type detection (PostgreSQL, MySQL)
- Topology detection (single-node vs. cluster)
- Authentication backend detection (LDAP, SAML, OIDC, OAuth)
- File storage backend detection (local, S3, Azure)
- Search backend detection (Elasticsearch, OpenSearch, database)
- Plugin detection from PluginSettings, plugin_statuses.json, and diagnostics
- Integration detection (webhooks, SMTP, Calls/RTCD, push notifications)
- Observability detection (Prometheus metrics, performance monitoring)

#### Repro Environment Generation
- Docker Compose with health checks and named volumes
- PostgreSQL 15 or MySQL 8.0 (detected from support package)
- Optional OpenSearch 2.11 (`--with-opensearch`)
- Optional OpenLDAP + phpLDAPadmin (`--with-ldap`)
- Optional Keycloak 23 for SAML/OIDC (`--with-saml`)
- Optional MinIO for S3-compatible storage (`--with-minio`)
- Optional RTCD for Calls (`--with-rtcd`)
- Optional Prometheus + Grafana (`--with-grafana`)
- MailHog for email capture (always enabled)
- nginx load balancer for multi-node topologies

#### Safety Features
- Full secret redaction pipeline (passwords, DSNs, API keys, certificates, tokens)
- Redaction report documenting what was redacted (no original values ever stored)
- Local-only credentials with `_local_repro_only` suffix for clarity
- MailHog always intercepts email — no accidental sends
- Path sanitization in ZIP extraction
- Decompression bomb protection (500MB per-file limit)

#### Reports
- `REPRO_SUMMARY.md` — What was recreated, approximated, and skipped
- `REDACTION_REPORT.md` — What was redacted for safety
- `PLUGIN_REPORT.md` — Plugin detection and installation status
- `repro-plan.json` — Machine-readable plan

### Platform Support
- macOS (Intel and Apple Silicon)
- Linux (amd64)
- Windows (amd64, via Docker Desktop)

### Known Limitations
- Multi-node capped at 3 nodes for local resource reasons
- Custom plugins require manual installation
- SAML certificates are regenerated locally (customer certs never reused)
- Cloud storage uses MinIO (not the real S3/Azure service)
- LDAP uses a stub directory with test users

### Installation
```bash
git clone https://github.com/rohith0456/mattermost-support-package-repro.git
cd mattermost-support-package-repro
make build
./bin/mm-repro doctor
```

### Quick Start
```bash
mm-repro init --support-package ./customer.zip
cd generated-repro/<timestamp>/
make run
open http://localhost:8065
```

---

Thank you to the Mattermost support engineering team for feedback and testing.
