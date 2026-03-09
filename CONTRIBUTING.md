# Contributing to mattermost-support-package-repro

Thank you for your interest in contributing to `mattermost-support-package-repro` (`mm-repro`). This document covers everything you need to know to get started, write good code, and get your changes merged.

---

## Table of Contents

- [How to Contribute](#how-to-contribute)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Code Style Guidelines](#code-style-guidelines)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Commit Message Format](#commit-message-format)
- [Security Reporting](#security-reporting)
- [Adding New Features](#adding-new-features)
  - [New Service Modules](#new-service-modules)
  - [New Redaction Rules](#new-redaction-rules)
  - [New Package Format Support](#new-package-format-support)

---

## How to Contribute

Contributions are welcome in the following forms:

- **Bug reports** — Open a GitHub issue with a clear description, steps to reproduce, and the relevant support package structure (anonymized). Include the output of `mm-repro version`.
- **Bug fixes** — Fork the repository, create a branch, fix the issue, add a regression test, and open a PR.
- **Feature requests** — Open an issue describing the use case before writing code. This avoids duplicated effort and ensures the feature aligns with the project's goals.
- **Documentation improvements** — Typo fixes, clarifications, and example updates are always welcome.
- **New service modules, redaction rules, or package format support** — See the dedicated sections below.

Before starting significant work, open an issue or comment on an existing one so maintainers can confirm the direction.

---

## Development Setup

### Prerequisites

| Tool | Minimum Version | Notes |
|------|----------------|-------|
| Go | 1.21 | `go version` to check |
| Docker | 24.x | Required for integration tests |
| Docker Compose | 2.x | Bundled with Docker Desktop |
| Make | Any | GNU Make recommended |

### Clone and Build

```bash
git clone https://github.com/rohith0456/mattermost-support-package-repro.git
cd mattermost-support-package-repro

# Download dependencies
go mod download

# Build the binary
make build

# Run all unit tests
make test

# Run linter
make lint
```

### Environment

No special environment variables are required for development. The `.env.example` file documents variables used by generated repro environments — it is not needed for building or testing the tool itself.

### Useful Make Targets

```
make build          Build the mm-repro binary to ./bin/mm-repro
make test           Run all unit tests with race detector
make test-integration  Run integration tests (requires Docker)
make lint           Run golangci-lint
make fmt            Format all Go source files
make vet            Run go vet
make clean          Remove build artifacts
make generate       Re-run go generate (templates, embedded files)
make schema-validate  Validate JSON schema files
```

---

## Project Structure

```
mattermost-support-package-repro/
├── cmd/                    CLI entry points (cobra commands)
├── internal/
│   ├── cli/                Command wiring and flag parsing
│   ├── generator/          Docker Compose and .env file generation
│   ├── inference/          Repro plan inference engine
│   ├── ingestion/          Support package extraction and normalization
│   ├── parser/             Config field parsing (version, DB, auth, etc.)
│   ├── redaction/          Sensitive value detection and redaction
│   └── runtime/            Docker runtime helpers (up/down/logs)
├── pkg/
│   └── models/             Shared types (SupportPackage, ReproPlan, etc.)
├── schema/                 JSON Schema definitions
├── templates/              Go text/template files for generated output
├── testdata/               Sample support packages for tests
├── assets/                 Embedded static assets
├── docs/                   Extended documentation
├── examples/               Example generated repro projects
├── scripts/                CI and release helper scripts
├── docker-compose.base.yml Reference Docker Compose with all services
└── .env.example            Environment variable template
```

Key design principles:

- `internal/` packages are not importable by external tools — keep implementation details here.
- `pkg/models/` contains types shared across packages. Keep them stable.
- The `inference` engine is the core logic: given a parsed support package, it produces a `ReproPlan` with no side effects. Keep it pure and well-tested.
- The `generator` package is the only place that writes files to disk.

---

## Code Style Guidelines

### General

- Follow standard Go idioms and the [Effective Go](https://go.dev/doc/effective_go) guide.
- Exported identifiers must have doc comments.
- Keep functions small and focused. Prefer multiple small functions over one large function.
- Avoid `init()` functions unless absolutely necessary.
- Do not use `panic` in library code — return errors.

### Error Handling

- Always return errors explicitly; do not swallow them silently.
- Wrap errors with context using `fmt.Errorf("doing X: %w", err)`.
- Use sentinel errors (e.g., `var ErrUnknownFormat = errors.New(...)`) for errors callers need to check by type.

### Naming

- Use full words over abbreviations: `normalizedPackage` not `np` in exported APIs (short names are fine inside small functions).
- Boolean fields and variables describing state should read naturally: `IsCluster`, `HasLDAP`, `ImageTagExact`.
- Constructor functions follow the `NewXxx` pattern.

### Formatting and Linting

All code must pass `gofmt` and `golangci-lint` with the project's `.golangci.yml` configuration. The CI pipeline enforces this. Run `make fmt && make lint` before pushing.

Notable enabled linters:

- `errcheck` — all errors must be checked
- `gosec` — basic security checks
- `exhaustive` — exhaustive switch coverage on enums
- `gocritic` — style and correctness checks
- `revive` — idiomatic Go

### Templates

Go `text/template` files live in `templates/`. Template file names use `.tmpl` extensions. Keep template logic minimal — complex logic belongs in Go, not templates.

---

## Testing Requirements

Every change must include appropriate tests. PRs without tests for new logic will be asked to add them before merge.

### Unit Tests

- Place test files alongside the package they test, using the `_test` package suffix (e.g., `parser_test`).
- Use `github.com/stretchr/testify/assert` for assertions and `require` for fatal preconditions.
- Use table-driven tests for any function with multiple input cases.
- Tests must not make network calls or write to disk without using `t.TempDir()`.
- Use the `makeNormalized` / `makeMinimalPackage` helper pattern (see existing test files) to construct test fixtures concisely.

### Integration Tests

Integration tests live in `*_integration_test.go` files and are guarded by the `integration` build tag:

```go
//go:build integration
```

They require Docker to be running and are executed with:

```bash
make test-integration
```

Integration tests may spin up real Docker Compose stacks using `testdata/` fixtures.

### Coverage

Aim for meaningful coverage of the logic, not 100% line coverage. The following packages have higher coverage requirements enforced in CI:

- `internal/parser` — 80% minimum
- `internal/redaction` — 80% minimum
- `internal/inference` — 75% minimum

### Test Fixtures

Sample support packages (ZIP archives or extracted directories) live in `testdata/`. When adding a new test fixture:

1. Ensure it contains no real customer data.
2. Run it through `mm-repro redact` before committing.
3. Add a comment in the test explaining what scenario the fixture represents.

---

## Pull Request Process

1. **Fork** the repository and create a branch from `main`. Branch names should be descriptive: `feat/opensearch-inference`, `fix/version-parse-rc-suffix`, `docs/contributing-guide`.

2. **Make your changes** following the code style and testing guidelines above.

3. **Run the full check suite** locally before pushing:
   ```bash
   make fmt lint vet test
   ```

4. **Open the PR** against the `main` branch. Fill in the PR template completely:
   - What problem does this solve?
   - What approach was taken?
   - How was it tested?
   - Are there any breaking changes?

5. **CI must pass.** All checks (build, lint, test, schema validation) must be green before review.

6. **One approving review** from a maintainer is required to merge. For significant feature changes, two reviews may be requested.

7. **Squash or rebase** to a clean commit history before merging. The maintainer merging the PR will squash if the branch has noisy fixup commits.

8. **Changelog**: Update `CHANGELOG.md` in the same PR for user-visible changes. Internal refactors do not require a changelog entry.

---

## Commit Message Format

This project uses a simplified Conventional Commits format:

```
<type>(<scope>): <short summary>

<optional body>

<optional footer>
```

### Type

| Type | When to use |
|------|-------------|
| `feat` | A new user-visible feature |
| `fix` | A bug fix |
| `refactor` | Code change that does not add a feature or fix a bug |
| `test` | Adding or updating tests only |
| `docs` | Documentation only |
| `chore` | Maintenance tasks (dependency updates, CI changes, etc.) |
| `perf` | Performance improvement |

### Scope

Use the name of the primary package or component affected. Examples:

- `feat(inference): cap cluster node count at 3 for local repro`
- `fix(parser): handle version strings with build metadata suffix`
- `refactor(redaction): extract rule evaluation into separate type`
- `test(inference): add table tests for database type fallback`
- `docs(contributing): add section on new service modules`

### Rules

- Summary line must be 72 characters or fewer.
- Use the imperative mood: "add support for" not "added support for".
- Do not end the summary line with a period.
- Separate the body from the summary with a blank line.
- Use the body to explain _why_, not _what_. The diff explains what changed.
- Reference issues in the footer: `Closes #42` or `Fixes #17`.

### Breaking Changes

If a change breaks the CLI interface, the `ReproPlan` JSON schema, or any other public contract, add a `BREAKING CHANGE:` footer:

```
feat(models): rename ReproPlan.NodeCount to ReproPlan.MattermostNodeCount

BREAKING CHANGE: The JSON key `node_count` in repro-plan output has been
renamed to `mattermost_node_count`. Tools consuming the plan JSON must be
updated.

Closes #88
```

---

## Security Reporting

**Do not open a public GitHub issue for security vulnerabilities.**

If you discover a security issue — particularly anything related to credential leakage, the redaction engine failing to scrub sensitive values, or path traversal in ZIP extraction — please report it privately:

1. Email `security@mattermost.com` with the subject `[mm-repro] Security Issue`.
2. Include a description of the vulnerability, steps to reproduce, and potential impact.
3. Allow up to 72 hours for an initial response before any public disclosure.

For non-critical security improvements (e.g., hardening a default, adding a missing redaction rule), a regular PR or issue is fine.

---

## Adding New Features

### New Service Modules

A "service module" is an optional Docker Compose service that can be included in a generated repro environment (e.g., OpenSearch, MinIO, Keycloak).

To add a new service:

1. **Add the service definition** to `docker-compose.base.yml` under an appropriate `profiles:` entry. Use a lowercase, single-word profile name matching the `--with-<name>` CLI flag convention.

2. **Add environment variables** to `.env.example` under a clearly labeled section with a comment noting the flag that enables it.

3. **Add a CLI flag** in `internal/cli/flags.go`. Follow the `WithXxx bool` pattern on the `ReproFlags` struct in `pkg/models/flags.go`.

4. **Add inference logic** in `internal/inference/engine.go`. The `inferServices` method should check the flag and the parsed package state. Add an `Approximation` entry if the service is being substituted or approximated.

5. **Add template support** in `templates/docker-compose.tmpl` and any associated config templates in `templates/config/`.

6. **Add redaction rules** (if the service configuration may appear in support packages) — see the next section.

7. **Write tests** for the inference logic in `internal/inference/engine_test.go` and a generator integration test in `internal/generator/`.

8. **Update `docker-compose.base.yml`** and **`.env.example`** with the new service and variables.

9. **Document** the new service in `docs/services.md` and update the `--help` text.

Checklist for a new service module PR:
- [ ] `docker-compose.base.yml` entry with `profiles:`
- [ ] `.env.example` section
- [ ] `ReproFlags` field
- [ ] Inference logic with approximation notes where needed
- [ ] Template(s)
- [ ] Unit tests covering the inference paths
- [ ] Documentation update

### New Redaction Rules

Redaction rules live in `internal/redaction/rules.go` as a slice of `Rule` structs returned by `DefaultRules()`. Each rule defines which config keys to match, what to replace them with, and a severity level.

To add a new rule:

1. **Identify the config key(s)** that need redaction. Check existing rules to avoid duplicates.

2. **Choose a severity level**:
   - `high` — credentials, private keys, tokens, DSNs with passwords
   - `medium` — server addresses, internal hostnames, user-identifying data
   - `low` — non-sensitive but potentially internal configuration values (strict mode only)

3. **Choose the correct placeholder constant** from `pkg/models/placeholders.go`. If none fits, add a new one following the `PlaceholderXxx` naming convention.

4. **Add the rule** to `DefaultRules()` in `internal/redaction/rules.go`:
   ```go
   {
       ID:          "rule-id-kebab-case",
       Name:        "Human-readable rule name",
       Patterns:    []string{"ExactConfigKey", "AnotherKey"},
       Replacement: models.PlaceholderPassword,
       Severity:    "high",
       StrictOnly:  false, // set true if only applied in --strict mode
   },
   ```

5. **Write a test** in `internal/redaction/redactor_test.go` that:
   - Confirms the target key is redacted.
   - Confirms that adjacent non-sensitive keys in the same section are NOT redacted (to prevent over-redaction regressions).
   - Confirms the original value never appears in the redaction report.

6. **Check for array-valued keys** (e.g., `DataSourceReplicas`). If the new key may hold a slice of sensitive strings, ensure `RedactConfig` handles it — see the `DataSourceReplicas` handling as a reference.

7. **Run `TestDefaultRules_NotEmpty`** to verify all required fields are populated.

### New Package Format Support

Support packages from different Mattermost deployment types (on-prem, Cloud, Kubernetes operator) may have different ZIP structures or file naming conventions. Format support lives in `internal/ingestion/`.

To add support for a new package format:

1. **Add a format detector** in `internal/ingestion/detect.go`. The detector inspects the ZIP entry names and returns a `PackageFormat` constant. Detectors are tried in order; be specific to avoid false positives.

2. **Implement an `Extractor`** that satisfies the `Extractor` interface defined in `internal/ingestion/extractor.go`. The extractor is responsible for:
   - Locating the main config file within the ZIP.
   - Locating diagnostics, system info, cluster info, and plugin info files.
   - Returning a `NormalizedPackage` with all fields populated.
   - Populating `EnvVars` if the package includes environment variable dumps.

3. **Register the extractor** in `internal/ingestion/registry.go` alongside the format constant.

4. **Add a test fixture** in `testdata/` — a minimal ZIP file containing the files your extractor expects. Keep it small; real data must be fully redacted.

5. **Write unit tests** in `internal/ingestion/` that:
   - Confirm the detector correctly identifies the new format.
   - Confirm the extractor produces a correctly populated `NormalizedPackage`.
   - Include a negative test that confirms the detector does NOT match other known formats.

6. **Document the format** in `docs/package-formats.md`, including how to identify it and any known limitations.

Checklist for a new package format PR:
- [ ] Format constant in `internal/ingestion/formats.go`
- [ ] Detector function in `internal/ingestion/detect.go`
- [ ] `Extractor` implementation
- [ ] Registration in `internal/ingestion/registry.go`
- [ ] Test fixture in `testdata/` (fully redacted)
- [ ] Unit tests for detection and extraction
- [ ] Documentation in `docs/package-formats.md`
