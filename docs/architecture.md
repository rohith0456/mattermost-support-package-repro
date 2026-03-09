# Architecture

This document describes the internal architecture of `mm-repro`, the data flow from a support package ZIP to a running Docker Compose environment, and the responsibilities of each layer.

---

## High-Level Overview

```
                      ┌────────────────────────────────────────────┐
                      │               mm-repro binary               │
                      │                                             │
  customer.zip ──────►│  Ingestion ──► Normalize ──► Redact        │
                      │                                   │         │
    CLI flags ────────►                                   ▼         │
                      │               Parse ──► Infer ──► Generate  │
                      │                                   │         │
                      └───────────────────────────────────┼─────────┘
                                                          │
                                                          ▼
                                              generated-repro/<ts>/
                                              ├── docker-compose.yml
                                              ├── .env
                                              ├── Makefile
                                              ├── config/
                                              ├── REPRO_SUMMARY.md
                                              ├── REDACTION_REPORT.md
                                              ├── PLUGIN_REPORT.md
                                              └── repro-plan.json
```

---

## Layer Descriptions

### Layer 1: Ingestion

**Package:** `internal/ingestion`

The ingestion layer is responsible for safely reading a support package ZIP file and producing a structured in-memory file index.

**Responsibilities:**
- Open and iterate the ZIP archive
- Sanitize all file paths (defend against path traversal)
- Enforce per-file size limits (500 MB max) to defend against zip bomb attacks
- Skip symlinks (log as warnings)
- Build a `FileIndex` mapping normalized path keys to raw byte content
- Identify the support package format variant (standard, sanitized, cloud)

**Data structures:**
```go
// FileIndex holds all extracted files from a support package.
type FileIndex struct {
    Format   PackageFormat            // standard | sanitized | cloud
    Files    map[string][]byte        // normalized path -> raw bytes
    Size     int64                    // total uncompressed bytes
    Warnings []string                 // non-fatal issues (skipped files, etc.)
}

type PackageFormat string

const (
    FormatStandard   PackageFormat = "standard"
    FormatSanitized  PackageFormat = "sanitized"
    FormatCloud      PackageFormat = "cloud"
    FormatUnknown    PackageFormat = "unknown"
)
```

**Path sanitization detail:**
```
ZIP entry path:  "../../etc/passwd"
After clean:     "etc/passwd"         ← stripped traversal
After prefix check: ERROR             ← rejected, not under extraction root

ZIP entry path:  "mattermost/config/config.json"
After clean:     "mattermost/config/config.json"
After prefix check: OK                ← accepted
```

**Format detection:** The ingestion layer inspects the top-level directory structure of the ZIP to determine the format:

| Signal | Format |
|--------|--------|
| Contains `config/config.json` | standard |
| Contains `sanitized-config.json` | sanitized |
| Contains `cloud-config.json` or `cloud_config.json` | cloud |
| None of the above | unknown (best-effort) |

---

### Layer 2: Normalizer

**Package:** `internal/normalizer`

The normalizer takes the `FileIndex` and extracts well-known files into typed structures, handling format differences between standard, sanitized, and cloud packages.

**Responsibilities:**
- Locate and parse `config.json` (handling format-specific paths)
- Locate and parse `diagnostics.json` / `support-packet.yaml`
- Locate and parse the plugin list (`plugins.json`, `plugin-statuses.json`)
- Locate and parse log samples for topology signals
- Produce a `NormalizedPackage` that downstream layers use uniformly

**Data structures:**
```go
type NormalizedPackage struct {
    Config      *model.Config        // parsed Mattermost config
    Diagnostics *DiagnosticsInfo     // server version, cluster nodes, etc.
    Plugins     []PluginInfo         // installed plugins with status
    LogSamples  []string             // first N lines of mattermost.log
    Warnings    []string             // non-fatal parse issues
}
```

**Format differences handled:**

| Field | Standard | Sanitized | Cloud |
|-------|----------|-----------|-------|
| Config path | `config/config.json` | `sanitized-config.json` | `cloud-config.json` |
| Version source | `diagnostics.json` | `support-packet.yaml` | `diagnostics.json` |
| Plugin list | `plugins/plugin_statuses.json` | `plugins.json` | `plugins.json` |
| Cluster info | `cluster_discovery.json` | `cluster.json` | embedded in diagnostics |

---

### Layer 3: Redaction Engine

**Package:** `internal/redaction`

The redaction engine is the security-critical layer. It scans the parsed `model.Config` and replaces all sensitive values with clearly labeled placeholders before any downstream layer can access them.

**Responsibilities:**
- Apply rule-based redaction by exact config key matching
- Apply pattern-based redaction (heuristic detection for DSNs, PEM blocks, base64 secrets)
- Record every redaction for the `REDACTION_REPORT.md`
- Enforce `--redact-strict` mode (also redacts server addresses and email addresses)
- Return the sanitized config and the redaction report

**Critical invariant:** No code after this layer should ever receive unredacted secrets. This is enforced by:
1. The redaction engine operating on a deep copy of the config
2. The original config being discarded immediately after redaction
3. All downstream layers accepting only `*redaction.RedactedConfig` (a wrapper type that cannot be constructed without going through the redaction engine)

**Data flow:**
```
model.Config (with secrets)
       │
       ▼
redaction.Engine.Redact(cfg, rules, strict)
       ├── deep-copy cfg
       ├── apply high-severity rules
       ├── apply medium-severity rules
       ├── apply pattern heuristics
       ├── (if strict) apply low-severity rules
       ├── record all redactions
       └── discard original cfg
       │
       ▼
redaction.Result{
    Config: *RedactedConfig   // safe for all downstream use
    Report: RedactionReport   // what was redacted
}
```

See [docs/security.md](security.md) for the complete rule set.

---

### Layer 4: Parser Layer

**Package:** `internal/parser`

The parser layer reads the redacted config and normalized package to extract specific signals that the inference engine needs. It focuses on extracting facts, not making decisions.

**Parsers:**

| Parser | Extracts |
|--------|----------|
| `version.Parser` | Mattermost version (major.minor.patch), build hash |
| `topology.Parser` | Single-node vs. cluster, node count, load balancer signals |
| `database.Parser` | DB type (postgres/mysql), driver version if available |
| `auth.Parser` | LDAP enabled/configured, SAML enabled, OAuth providers |
| `storage.Parser` | File storage backend (local, S3, Azure, GCS) |
| `search.Parser` | Search backend (database, elasticsearch, opensearch, bleve) |
| `plugins.Parser` | Plugin IDs, versions, enabled/disabled state |
| `smtp.Parser` | SMTP configured (bool only — credentials are redacted) |
| `calls.Parser` | RTCD configured, standalone vs. built-in |
| `metrics.Parser` | Prometheus/metrics endpoint enabled |

**Data structures:**
```go
type ParsedSignals struct {
    Version    VersionSignal
    Topology   TopologySignal
    Database   DatabaseSignal
    Auth       AuthSignal
    Storage    StorageSignal
    Search     SearchSignal
    Plugins    []PluginSignal
    SMTP       SMTPSignal
    Calls      CallsSignal
    Metrics    MetricsSignal
}

type VersionSignal struct {
    Version     string    // e.g. "9.11.2"
    Major       int
    Minor       int
    Patch       int
    BuildHash   string
    IsEnterprise bool
}

type TopologySignal struct {
    IsCluster    bool
    NodeCount    int       // best-effort from diagnostics
    HasLB        bool
    ClusterName  string    // redacted in strict mode
}
```

---

### Layer 5: Inference Engine

**Package:** `internal/inference`

The inference engine takes parsed signals and CLI flags and produces a `ReproPlan` — a complete, actionable description of what the generated environment should contain.

**Responsibilities:**
- Resolve version to the closest available Docker Hub image tag
- Decide topology (single vs. multi-node) from signals + flags
- Decide which optional services to include (from signals + explicit flags)
- Resolve port assignments (check for conflicts with `doctor` data)
- Determine plugin installation strategy for each detected plugin
- Produce the complete service graph

**Decision logic:**

```
Is --force-single-node set?  ──► use single-node
Is --force-multi-node set?   ──► use multi-node (3 nodes)
TopologySignal.IsCluster == true?  ──► use multi-node
                                        else ──► use single-node

Is --with-opensearch set?   ──► include OpenSearch
OR SearchSignal.Backend == "elasticsearch" || "opensearch"  ──► include OpenSearch

Is --with-ldap set?  ──► include OpenLDAP
OR AuthSignal.LDAPEnabled == true  ──► include OpenLDAP (with warning)
```

**Data structures:**
```go
type ReproPlan struct {
    ProjectName    string
    MattermostImage string       // e.g. "mattermost/mattermost-enterprise-edition:9.11.2"
    Topology       TopologyPlan  // single | multi-node(3)
    Database       DatabasePlan
    Services       []ServicePlan // postgres, opensearch, ldap, keycloak, minio, mailhog, ...
    Plugins        []PluginPlan
    Ports          PortAssignments
    Notes          []PlanNote    // approximations, warnings, manual steps
}

type PlanNote struct {
    Severity string  // info | warning | approximation | manual-step-required
    Message  string
}
```

---

### Layer 6: Generator Layer

**Package:** `internal/generator`

The generator layer takes a `ReproPlan` and renders the complete output project. It uses Go `text/template` for all generated files.

**Generators:**

| Generator | Output file(s) |
|-----------|----------------|
| `compose.Generator` | `docker-compose.yml` |
| `env.Generator` | `.env` |
| `makefile.Generator` | `Makefile` |
| `nginx.Generator` | `config/nginx/nginx.conf` (multi-node only) |
| `prometheus.Generator` | `config/prometheus/prometheus.yml` (if grafana) |
| `readme.Generator` | `README.md` (project-specific) |
| `summary.Generator` | `REPRO_SUMMARY.md` |
| `redaction.Generator` | `REDACTION_REPORT.md` |
| `plugin.Generator` | `PLUGIN_REPORT.md` |
| `plan.Generator` | `repro-plan.json` |
| `scripts.Generator` | `scripts/start.sh`, `scripts/stop.sh`, `scripts/reset.sh` |

**Template approach:**

Templates live in `templates/` and are embedded into the binary using `//go:embed`. Each template receives a typed data struct, preventing accidental nil pointer panics in templates.

```go
//go:embed templates
var templateFS embed.FS

type ComposeData struct {
    Plan        *inference.ReproPlan
    Services    []ServiceConfig
    Networks    []NetworkConfig
    Volumes     []VolumeConfig
}
```

---

### Layer 7: Runtime Layer

**Package:** `internal/runtime`

The runtime layer implements the `run`, `stop`, and `reset` subcommands. It is a thin wrapper around `docker compose` CLI invocations.

**Responsibilities:**
- Locate the generated project directory
- Invoke `docker compose up -d` / `docker compose down` / `docker compose down -v`
- Stream logs for `make logs`
- Perform pre-run health checks (port availability, Docker daemon running)
- Implement `mm-repro doctor` (checks Docker, disk space, ports 8065/5432/9200)

---

### Layer 8: Reporting Layer

**Package:** `internal/reporting`

The reporting layer produces human-readable markdown reports that are written into the generated project. Reports are generated by the generator layer using data from inference, redaction, and plugin resolution.

**Reports produced:**

#### REPRO_SUMMARY.md
Documents what was recreated, what was approximated, and what could not be recreated:
```markdown
## Mattermost Version
Recreated: mattermost-enterprise-edition:9.11.2 (exact match)

## Topology
Recreated: Single node (signals indicated single-node deployment)

## Database
Recreated: PostgreSQL 15 (customer used PostgreSQL; version approximated)

## Approximations
- OpenSearch 2.x used (customer had Elasticsearch 8.x)
- Keycloak used for SAML (customer used Okta)

## Skipped
- Real customer data (privacy)
- Customer LDAP directory (no access)
```

#### REDACTION_REPORT.md
Lists every config key that was redacted. See [security.md](security.md) for format.

#### PLUGIN_REPORT.md
Documents plugin detection results:
```markdown
## Plugin Report

| Plugin ID | Version | Status | Installation Strategy |
|-----------|---------|--------|----------------------|
| com.mattermost.calls | 0.21.1 | enabled | marketplace-auto-install |
| com.mattermost.jira | 4.1.0 | enabled | marketplace-auto-install |
| com.example.custom-plugin | 1.0.0 | enabled | MANUAL - not in marketplace |
```

---

## Package Layout

```
mattermost-support-package-repro/
├── cmd/
│   └── mm-repro/
│       └── main.go              # CLI entry point (cobra commands)
├── internal/
│   ├── ingestion/               # ZIP reading, path sanitization
│   ├── normalizer/              # Format-specific config extraction
│   ├── redaction/               # Secret detection and replacement
│   ├── parser/                  # Signal extraction from redacted config
│   ├── inference/               # ReproPlan construction
│   ├── generator/               # Template-based file generation
│   ├── runtime/                 # docker compose invocation
│   └── reporting/               # Report document generation
├── pkg/
│   ├── model/                   # Mattermost config model types
│   ├── version/                 # Docker Hub image resolution
│   └── marketplace/             # Plugin marketplace API client
├── templates/                   # Embedded Go templates
│   ├── docker-compose.yml.tmpl
│   ├── env.tmpl
│   ├── Makefile.tmpl
│   ├── nginx.conf.tmpl
│   └── ...
├── testdata/
│   ├── fixtures/                # Sample support packages for tests
│   └── golden/                  # Expected output files for golden tests
└── docs/                        # Documentation (this directory)
```

---

## Data Flow Diagram (Detailed)

```
support-package.zip
        │
        ▼
┌───────────────────────────────────────────────────────────────────┐
│ ingestion.Open(zipPath)                                            │
│  ├── iterate ZIP entries                                          │
│  ├── sanitize paths (strip ../)                                   │
│  ├── enforce 500MB per-file limit                                 │
│  ├── skip symlinks                                                │
│  └── return FileIndex{Format, Files map[string][]byte}           │
└──────────────────────────────┬────────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────────┐
│ normalizer.Normalize(fileIndex)                                    │
│  ├── locate config.json (format-specific path)                    │
│  ├── parse JSON into model.Config                                 │
│  ├── locate & parse diagnostics                                   │
│  ├── locate & parse plugin list                                   │
│  └── return NormalizedPackage                                     │
└──────────────────────────────┬────────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────────┐
│ redaction.Engine.Redact(pkg.Config, options)                       │
│  ├── deep copy config                                             │
│  ├── apply high-severity rules (DSN, passwords, keys, certs)     │
│  ├── apply medium-severity rules (webhook secrets, plugin tokens) │
│  ├── apply pattern heuristics (PEM blocks, DSN patterns)         │
│  ├── [if --redact-strict] apply low-severity rules               │
│  ├── record RedactionReport                                       │
│  └── return RedactedConfig + RedactionReport                     │
└──────────────────────────────┬────────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────────┐
│ parser.ParseAll(redactedConfig, normalizedPkg)                     │
│  ├── version.Parse()    → VersionSignal                           │
│  ├── topology.Parse()   → TopologySignal                          │
│  ├── database.Parse()   → DatabaseSignal                          │
│  ├── auth.Parse()       → AuthSignal                              │
│  ├── storage.Parse()    → StorageSignal                           │
│  ├── search.Parse()     → SearchSignal                            │
│  ├── plugins.Parse()    → []PluginSignal                          │
│  └── return ParsedSignals                                         │
└──────────────────────────────┬────────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────────┐
│ inference.Engine.Plan(signals, cliFlags)                           │
│  ├── resolve Mattermost Docker image tag                          │
│  ├── decide topology (single/multi) from signals + flags          │
│  ├── decide optional services from signals + flags               │
│  ├── assign ports (check conflicts)                               │
│  ├── resolve plugin installation strategies                       │
│  └── return ReproPlan                                             │
└──────────────────────────────┬────────────────────────────────────┘
                               │
                               ▼
┌───────────────────────────────────────────────────────────────────┐
│ generator.Generate(plan, outputDir)                                │
│  ├── render docker-compose.yml                                    │
│  ├── render .env (with generated local credentials)              │
│  ├── render Makefile                                              │
│  ├── render config/ (nginx, prometheus)                          │
│  ├── render scripts/ (start.sh, stop.sh, reset.sh)              │
│  ├── render README.md (project-specific)                         │
│  ├── render REPRO_SUMMARY.md                                      │
│  ├── render REDACTION_REPORT.md                                   │
│  ├── render PLUGIN_REPORT.md                                      │
│  └── write repro-plan.json                                        │
└──────────────────────────────┬────────────────────────────────────┘
                               │
                               ▼
                   generated-repro/<timestamp>/
                   (ready to run with make run)
```

---

## Version Resolution

The `pkg/version` package resolves a Mattermost version string to an available Docker Hub image tag.

**Resolution strategy:**

1. Try exact match: `mattermost/mattermost-enterprise-edition:<version>`
2. Try `mattermost/mattermost-team-edition:<version>`
3. Try minor-version fallback: most recent patch of same major.minor
4. Try major-version fallback: most recent stable of same major
5. Warn and use `latest` if no match found

All approximations are recorded as `PlanNote{Severity: "approximation"}` and appear in `REPRO_SUMMARY.md`.

---

## Plugin Resolution

The `pkg/marketplace` package queries the Mattermost plugin marketplace to determine installation strategy for each detected plugin.

**Resolution for each plugin:**

```
Is plugin ID in the known-builtin list?
  → strategy: builtin (no action needed, bundled with Mattermost)

Is plugin ID found in marketplace API response?
  → strategy: marketplace-auto-install
     (use Mattermost's built-in marketplace install API at startup)

Is plugin ID not found in marketplace?
  → strategy: manual
     (documented in PLUGIN_REPORT.md, engineer must install manually)
```

See [docs/plugin-strategy.md](plugin-strategy.md) for details.

---

## Testing Strategy

| Test Type | Coverage | Location |
|-----------|----------|----------|
| Unit tests | Per-layer, mocked dependencies | `internal/*/..._test.go` |
| Golden tests | Generator output vs. expected files | `testdata/golden/` |
| Integration tests | Full pipeline with fixture packages | `testdata/fixtures/` |
| Fuzz tests | Ingestion path sanitization | `internal/ingestion/fuzz_test.go` |

**Golden test approach:**

Golden tests run the full pipeline against a fixture support package and compare the generated output to checked-in expected files. To update golden files after intentional changes:

```bash
make test-update-golden
```

**Fixture packages:**

Fixture packages in `testdata/fixtures/` are synthetic — they contain realistic config structure but no real credentials or customer data. They are generated by `scripts/generate-fixtures.sh`.
