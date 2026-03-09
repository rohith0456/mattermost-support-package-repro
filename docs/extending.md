# Extending mm-repro

This guide describes how to add new capabilities to `mm-repro`:

- New support package formats
- New service modules (additional Docker Compose services)
- New redaction rules
- New inference rules
- New parser signals

---

## Adding a New Support Package Format

Support packages can vary in structure depending on the Mattermost version, deployment type (on-prem vs. cloud), and any sanitization applied by the customer before sending.

### 1. Define the New Format Constant

In `internal/ingestion/format.go`, add a new constant to the `PackageFormat` type:

```go
const (
    FormatStandard   PackageFormat = "standard"
    FormatSanitized  PackageFormat = "sanitized"
    FormatCloud      PackageFormat = "cloud"
    FormatMyNewFormat PackageFormat = "my-new-format"  // add this
    FormatUnknown    PackageFormat = "unknown"
)
```

### 2. Add Detection Logic

In `internal/ingestion/format_detect.go`, add a case to the `DetectFormat` function:

```go
func DetectFormat(index *FileIndex) PackageFormat {
    switch {
    case index.Has("sanitized-config.json"):
        return FormatSanitized
    case index.Has("cloud-config.json"), index.Has("cloud_config.json"):
        return FormatCloud
    case index.HasPrefix("my-new-format/") && index.Has("my-new-format/config.json"):
        return FormatMyNewFormat  // add this case
    case index.Has("config/config.json"):
        return FormatStandard
    default:
        return FormatUnknown
    }
}
```

The `index.Has(path)` and `index.HasPrefix(prefix)` helpers check the file index without allocating.

### 3. Add a Normalizer for the Format

In `internal/normalizer/`, create a new file `my_new_format.go`:

```go
package normalizer

// normalizeMyNewFormat handles support packages in the my-new-format variant.
func normalizeMyNewFormat(index *ingestion.FileIndex) (*NormalizedPackage, error) {
    pkg := &NormalizedPackage{}
    var err error

    // Locate and parse the config
    configBytes, ok := index.Get("my-new-format/config.json")
    if !ok {
        return nil, fmt.Errorf("my-new-format: config.json not found")
    }
    pkg.Config, err = parseConfig(configBytes)
    if err != nil {
        return nil, fmt.Errorf("my-new-format: parsing config: %w", err)
    }

    // Locate and parse diagnostics (format-specific path)
    diagBytes, ok := index.Get("my-new-format/diagnostics.json")
    if ok {
        pkg.Diagnostics, err = parseDiagnostics(diagBytes)
        if err != nil {
            pkg.Warnings = append(pkg.Warnings, fmt.Sprintf("diagnostics parse error: %v", err))
        }
    }

    // Locate plugins
    pluginBytes, ok := index.Get("my-new-format/plugins.json")
    if ok {
        pkg.Plugins, err = parsePlugins(pluginBytes)
        if err != nil {
            pkg.Warnings = append(pkg.Warnings, fmt.Sprintf("plugins parse error: %v", err))
        }
    }

    return pkg, nil
}
```

### 4. Register the Normalizer

In `internal/normalizer/normalizer.go`, add the new format to the dispatch switch:

```go
func Normalize(index *ingestion.FileIndex) (*NormalizedPackage, error) {
    switch index.Format {
    case ingestion.FormatStandard:
        return normalizeStandard(index)
    case ingestion.FormatSanitized:
        return normalizeSanitized(index)
    case ingestion.FormatCloud:
        return normalizeCloud(index)
    case ingestion.FormatMyNewFormat:
        return normalizeMyNewFormat(index)   // add this
    default:
        return normalizeBestEffort(index)
    }
}
```

### 5. Add a Fixture and Tests

Create a synthetic fixture in `testdata/fixtures/my-new-format/`:

```
testdata/fixtures/my-new-format/
├── my-new-format/
│   ├── config.json        (use testdata/fixtures/base-config.json as a base)
│   ├── diagnostics.json
│   └── plugins.json
```

Compress it:
```bash
cd testdata/fixtures/my-new-format
zip -r ../my-new-format-fixture.zip .
```

Add a test in `internal/normalizer/my_new_format_test.go`:

```go
func TestNormalizeMyNewFormat(t *testing.T) {
    index, err := ingestion.Open("../../testdata/fixtures/my-new-format-fixture.zip")
    require.NoError(t, err)
    assert.Equal(t, ingestion.FormatMyNewFormat, index.Format)

    pkg, err := Normalize(index)
    require.NoError(t, err)
    assert.NotNil(t, pkg.Config)
    assert.Equal(t, "9.11.2", pkg.Diagnostics.Version)
}
```

---

## Adding a New Service Module

Service modules represent optional Docker Compose services that can be included in a generated repro. Examples: OpenSearch, OpenLDAP, Keycloak, MinIO, MailHog.

### 1. Define the Service in the Plan Model

In `internal/inference/plan.go`, add your service to the `ServiceKind` enum:

```go
type ServiceKind string

const (
    ServicePostgres    ServiceKind = "postgres"
    ServiceMySQL       ServiceKind = "mysql"
    ServiceOpenSearch  ServiceKind = "opensearch"
    ServiceOpenLDAP    ServiceKind = "openldap"
    ServiceKeycloak    ServiceKind = "keycloak"
    ServiceMinIO       ServiceKind = "minio"
    ServiceMailHog     ServiceKind = "mailhog"
    ServiceRTCD        ServiceKind = "rtcd"
    ServicePrometheus  ServiceKind = "prometheus"
    ServiceGrafana     ServiceKind = "grafana"
    ServiceMyNewService ServiceKind = "my-new-service"  // add this
)
```

Also add a `ServiceConfig` struct if your service needs non-trivial configuration:

```go
type MyNewServiceConfig struct {
    Port    int
    Version string
    // additional fields
}
```

### 2. Add CLI Flag

In `cmd/mm-repro/init.go`, add the flag to the `init` command:

```go
initCmd.Flags().Bool("with-my-new-service", false, "Include My New Service")
```

### 3. Add Inference Logic

In `internal/inference/engine.go`, add logic to decide when to include the service:

```go
func (e *Engine) shouldIncludeMyNewService(signals *parser.ParsedSignals, flags InitFlags) bool {
    // Always include if explicitly requested
    if flags.WithMyNewService {
        return true
    }
    // Auto-include based on signals (optional)
    // e.g., if signals.SomeSignal.IsMyNewServiceEnabled {
    //     return true
    // }
    return false
}
```

Add it to the `Plan` method:

```go
if e.shouldIncludeMyNewService(signals, flags) {
    plan.Services = append(plan.Services, ServicePlan{
        Kind: ServiceMyNewService,
        Config: MyNewServiceConfig{
            Port: e.ports.Assign("my-new-service", 9999),
        },
    })
}
```

### 4. Add a Docker Compose Template Fragment

Create `templates/services/my-new-service.yml.tmpl`:

```yaml
  my-new-service:
    image: my-new-service/image:{{ .Version }}
    container_name: {{ .ProjectName }}-my-new-service
    restart: unless-stopped
    environment:
      - MY_NEW_SERVICE_SOME_SETTING=value_local_repro_only
    ports:
      - "{{ .Port }}:9999"
    networks:
      - mm-repro-net
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9999/health"]
      interval: 10s
      timeout: 5s
      retries: 5
```

### 5. Register the Template in the Compose Generator

In `internal/generator/compose.go`, add the service to the compose generation logic:

```go
for _, svc := range plan.Services {
    switch svc.Kind {
    case inference.ServiceOpenSearch:
        fragments = append(fragments, renderFragment("services/opensearch.yml.tmpl", svc))
    case inference.ServiceMyNewService:
        fragments = append(fragments, renderFragment("services/my-new-service.yml.tmpl", svc))
    // ...
    }
}
```

### 6. Add Mattermost Config Overrides (if needed)

If your new service requires Mattermost config changes (e.g., setting a server address or enabling a feature), add overrides in the `.env` template or a Mattermost config template fragment.

In `templates/mattermost-env-overrides.tmpl`:
```
{{- if .HasMyNewService }}
MM_MYNEWSERVICESETTINGS_ENABLE=true
MM_MYNEWSERVICESETTINGS_SERVERURL=http://my-new-service:9999
{{- end }}
```

### 7. Document in REPRO_SUMMARY

Add a case in `internal/reporting/summary.go` to describe the service in the summary report:

```go
case inference.ServiceMyNewService:
    lines = append(lines, fmt.Sprintf(
        "| My New Service | Local my-new-service container (port %d) |",
        svc.Config.(inference.MyNewServiceConfig).Port,
    ))
```

### 8. Add Tests

Add a golden test fixture that exercises your new service:

```bash
make test-update-golden FIXTURE=my-new-service-test
```

---

## Adding a New Redaction Rule

### 1. Define the Rule

In `internal/redaction/rules.go`, add a new entry to the `DefaultRules` slice:

```go
var DefaultRules = []Rule{
    // existing rules...
    {
        ID:          "my-new-secret",
        Severity:    SeverityHigh,
        Description: "My New Service API key",
        // KeyPatterns: exact config key names that should be redacted
        KeyPatterns: []string{
            "MyNewServiceAPIKey",
            "MyNewServiceSecret",
        },
        // Placeholder: the value to substitute
        Placeholder: "REDACTED_KEY_SEE_REDACTION_REPORT",
    },
}
```

**Severity levels:**

| Severity | When applied |
|----------|--------------|
| `SeverityHigh` | Always, including plan/dry-run output |
| `SeverityMedium` | Always |
| `SeverityLow` | Only with `--redact-strict` |

### 2. Add a Pattern Rule (Optional)

If the secret might appear under unpredictable key names but has a detectable value pattern, add a `PatternRule`:

```go
var PatternRules = []PatternRule{
    // existing pattern rules...
    {
        ID:          "my-new-service-token",
        Description: "My New Service token (starts with mns_)",
        // ValuePattern: regex applied to the VALUE of any config field
        ValuePattern: regexp.MustCompile(`^mns_[A-Za-z0-9]{32,}$`),
        Placeholder:  "REDACTED_SECRET_SEE_REDACTION_REPORT",
    },
}
```

### 3. Add Tests

In `internal/redaction/rules_test.go`:

```go
func TestMyNewSecretRedaction(t *testing.T) {
    cfg := &model.Config{}
    // Set the field to a test value
    cfg.MyNewServiceSettings.APIKey = "supersecret-api-key-12345"

    result, err := engine.Redact(cfg, DefaultRules, false)
    require.NoError(t, err)
    assert.Equal(t, "REDACTED_KEY_SEE_REDACTION_REPORT",
        result.Config.MyNewServiceSettings.APIKey)
    assert.Len(t, result.Report.Redactions, 1)
    assert.Equal(t, "my-new-secret", result.Report.Redactions[0].RuleID)
}
```

### 4. Update the Redaction Report Documentation

The redaction report is auto-generated from rule metadata. No manual documentation update is needed — the new rule's `ID`, `Description`, and `Placeholder` fields appear automatically in `REDACTION_REPORT.md`.

---

## Adding a New Parser Signal

Parsers extract typed signals from the redacted config. You may need a new parser if you're adding inference logic that depends on config state not currently captured.

### 1. Define the Signal Type

In `internal/parser/signals.go`:

```go
type MyNewSignal struct {
    IsEnabled      bool
    ServerURL      string   // already-redacted value
    SomeIntSetting int
}
```

Add it to `ParsedSignals`:

```go
type ParsedSignals struct {
    Version    VersionSignal
    Topology   TopologySignal
    // ...existing fields...
    MyNew      MyNewSignal    // add this
}
```

### 2. Implement the Parser

Create `internal/parser/mynew.go`:

```go
package parser

// ParseMyNew extracts signals related to My New Service from the config.
func ParseMyNew(cfg *model.Config) MyNewSignal {
    if cfg.MyNewServiceSettings == nil {
        return MyNewSignal{}
    }
    return MyNewSignal{
        IsEnabled:      cfg.MyNewServiceSettings.Enable != nil && *cfg.MyNewServiceSettings.Enable,
        ServerURL:      cfg.MyNewServiceSettings.ServerURL,  // already redacted if sensitive
        SomeIntSetting: cfg.MyNewServiceSettings.SomeSetting,
    }
}
```

### 3. Call the Parser

In `internal/parser/parser.go`, add the call to `ParseAll`:

```go
func ParseAll(cfg *redaction.RedactedConfig, pkg *normalizer.NormalizedPackage) *ParsedSignals {
    return &ParsedSignals{
        Version:  ParseVersion(cfg.Config, pkg.Diagnostics),
        Topology: ParseTopology(cfg.Config, pkg.Diagnostics),
        // ...
        MyNew:    ParseMyNew(cfg.Config),  // add this
    }
}
```

### 4. Add Tests

In `internal/parser/mynew_test.go`:

```go
func TestParseMyNew_Enabled(t *testing.T) {
    enabled := true
    cfg := &model.Config{
        MyNewServiceSettings: &model.MyNewServiceSettings{
            Enable:    &enabled,
            ServerURL: "REDACTED_SERVER_ADDRESS", // already redacted
        },
    }
    sig := ParseMyNew(cfg)
    assert.True(t, sig.IsEnabled)
}
```

---

## Adding a New Inference Rule

Inference rules determine what goes into the `ReproPlan`. Rules can use any combination of parsed signals and CLI flags.

### 1. Locate the Inference Engine

In `internal/inference/engine.go`, find the `Plan` method. Rules are implemented as private methods and called from `Plan`.

### 2. Add a Decision Method

```go
// shouldEnableMyNewFeature returns true if the plan should include My New Feature.
func (e *Engine) shouldEnableMyNewFeature(signals *parser.ParsedSignals, flags InitFlags) (bool, string) {
    if flags.WithMyNewService {
        return true, "explicitly requested via --with-my-new-service"
    }
    if signals.MyNew.IsEnabled {
        return true, "detected in support package config"
    }
    return false, ""
}
```

### 3. Call the Rule in Plan()

```go
func (e *Engine) Plan(signals *parser.ParsedSignals, flags InitFlags) (*ReproPlan, error) {
    plan := &ReproPlan{}

    // ...existing logic...

    if include, reason := e.shouldEnableMyNewFeature(signals, flags); include {
        plan.Services = append(plan.Services, ServicePlan{
            Kind:   ServiceMyNewService,
            Reason: reason,
            Config: MyNewServiceConfig{
                Port: e.ports.Assign("my-new-service", 9999),
            },
        })
        plan.Notes = append(plan.Notes, PlanNote{
            Severity: "info",
            Message:  fmt.Sprintf("My New Service included: %s", reason),
        })
    }

    return plan, nil
}
```

The `Reason` field in `ServicePlan` is included in `REPRO_SUMMARY.md` to explain why a service was included.

### 4. Add Tests

In `internal/inference/engine_test.go`:

```go
func TestPlan_IncludesMyNewServiceWhenDetected(t *testing.T) {
    signals := &parser.ParsedSignals{
        MyNew: parser.MyNewSignal{IsEnabled: true},
    }
    plan, err := engine.Plan(signals, InitFlags{})
    require.NoError(t, err)
    assert.True(t, planHasService(plan, inference.ServiceMyNewService))
}

func TestPlan_IncludesMyNewServiceWhenFlagSet(t *testing.T) {
    signals := &parser.ParsedSignals{}  // no signals
    plan, err := engine.Plan(signals, InitFlags{WithMyNewService: true})
    require.NoError(t, err)
    assert.True(t, planHasService(plan, inference.ServiceMyNewService))
}

func TestPlan_ExcludesMyNewServiceByDefault(t *testing.T) {
    signals := &parser.ParsedSignals{}
    plan, err := engine.Plan(signals, InitFlags{})
    require.NoError(t, err)
    assert.False(t, planHasService(plan, inference.ServiceMyNewService))
}
```

---

## Running Tests After Your Changes

```bash
# Run all tests
make test

# Run only the layers you changed
go test ./internal/ingestion/...
go test ./internal/normalizer/...
go test ./internal/redaction/...
go test ./internal/parser/...
go test ./internal/inference/...
go test ./internal/generator/...

# Update golden files if generator output changed intentionally
make test-update-golden

# Run with coverage
make test-coverage
# Open coverage report: open coverage.html
```

---

## Checklist for New Contributions

Before opening a pull request, verify:

- [ ] New format: detection logic added, normalizer implemented, fixture created, tests pass
- [ ] New service: CLI flag added, inference logic added, template created, compose generator updated, REPRO_SUMMARY reporter updated
- [ ] New redaction rule: rule defined, tests added (must verify value is redacted, not just that no error occurs)
- [ ] New inference rule: decision method added, positive and negative test cases added
- [ ] Golden tests updated if generator output changed (`make test-update-golden`)
- [ ] No real customer data in test fixtures
- [ ] `make test` passes
- [ ] `make build` succeeds
