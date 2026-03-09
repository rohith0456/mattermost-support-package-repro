# Plugin Strategy

This document describes how `mm-repro` handles plugin detection, classification, and installation in generated repro environments.

---

## Overview

Mattermost support packages contain a list of installed and enabled plugins. `mm-repro` reads this list and determines how to handle each plugin in the local repro environment. The goal is to have the correct plugins running so that plugin-related issues can be reproduced — while being honest about what cannot be recreated.

---

## Plugin Detection

The normalizer layer extracts plugin information from the support package. Depending on the package format, plugins are found in different locations:

| Format | Plugin list location |
|--------|---------------------|
| standard | `plugins/plugin_statuses.json` |
| sanitized | `plugins.json` |
| cloud | `plugins.json` or embedded in diagnostics |

Each detected plugin entry contains:

```go
type PluginInfo struct {
    PluginID    string      // e.g. "com.mattermost.calls"
    Version     string      // e.g. "0.21.1"
    IsEnabled   bool
    IsBuiltin   bool        // reported by server as prepackaged
    Name        string      // display name (from manifest if available)
}
```

If a plugin manifest (`plugin.json`) is present in the support package (some packages include them), the normalizer also reads it for additional metadata. If not, the plugin ID and version from the status list are used.

---

## Plugin Classification

Each detected plugin is classified into one of three categories by the `pkg/marketplace` package:

### 1. Builtin (Prepackaged)

Builtin plugins are shipped with the Mattermost binary and do not need to be installed separately. They are enabled/disabled through config, not through the marketplace.

**Examples:**
- `com.mattermost.calls` (Calls plugin, bundled since 7.x)
- `com.mattermost.nps` (Net Promoter Score survey)
- `com.mattermost.plugin-todo` (bundled in some versions)

**Strategy:** No installation action required. The generated Mattermost config will enable/disable the plugin to match what was detected.

**How to identify builtin plugins:** The `pkg/marketplace/builtins.go` file contains a version-aware registry of known builtin plugins:

```go
// IsBuiltin returns true if the given plugin ID is known to be a prepackaged
// plugin in the given Mattermost version.
func IsBuiltin(pluginID string, mmVersion string) bool {
    entries, ok := builtinRegistry[pluginID]
    if !ok {
        return false
    }
    for _, e := range entries {
        if e.MinVersion <= mmVersion && (e.MaxVersion == "" || mmVersion <= e.MaxVersion) {
            return true
        }
    }
    return false
}
```

### 2. Marketplace

Marketplace plugins are publicly available on the official Mattermost plugin marketplace. They can be auto-installed into the generated environment using Mattermost's built-in marketplace API.

**Examples:**
- `com.mattermost.jira`
- `com.mattermost.github`
- `com.mattermost.zoom`
- `com.mattermost.servicenow`
- `com.mattermost.msteams-sync`

**Strategy:** Auto-install using the Mattermost marketplace API at container startup.

**How auto-install works:**

The generated `docker-compose.yml` includes an `init` container (or startup script) that calls the Mattermost API to install plugins from the marketplace before Mattermost fully starts. The init container uses the local admin credentials to authenticate.

```yaml
  mm-plugin-installer:
    image: curlimages/curl:latest
    depends_on:
      mattermost:
        condition: service_healthy
    environment:
      - MM_ADMIN_TOKEN=${MM_ADMIN_TOKEN}
      - MM_HOST=http://mattermost:8065
    command: |
      sh -c "
        curl -sf -X POST ${MM_HOST}/api/v4/plugins/marketplace \
          -H 'Authorization: Bearer ${MM_ADMIN_TOKEN}' \
          -d '{\"plugin_id\": \"com.mattermost.jira\", \"version\": \"4.1.0\"}'
      "
    networks:
      - mm-repro-net
```

If the exact version is not available in the marketplace, the latest available version is used and the approximation is noted in `PLUGIN_REPORT.md`.

### 3. Custom / Unknown

Custom plugins are not in the official marketplace. They may be:
- Internal company plugins
- Vendor-specific integrations
- Pre-release or unreleased plugins
- Plugins no longer in the marketplace

**Strategy:** Manual installation only. The `PLUGIN_REPORT.md` lists these plugins with a clear `[MANUAL]` label and instructions for the engineer.

---

## The PLUGIN_REPORT.md

Every generated project includes `PLUGIN_REPORT.md`, which summarizes the plugin resolution results:

```markdown
# Plugin Report

Generated from support package: customer-support-package.zip
Mattermost version: 9.11.2

## Summary
- Builtin (no action required): 3
- Marketplace (auto-install): 4
- Manual installation required: 1

## Plugins

| Plugin ID | Name | Version | Status in Package | Installation Strategy |
|-----------|------|---------|-------------------|----------------------|
| com.mattermost.calls | Calls | 0.21.1 | enabled | builtin — enabled via config |
| com.mattermost.nps | User Satisfaction Surveys | 1.3.5 | enabled | builtin — enabled via config |
| com.mattermost.jira | Jira | 4.1.0 | enabled | marketplace-auto-install |
| com.mattermost.github | GitHub | 2.1.6 | enabled | marketplace-auto-install |
| com.mattermost.zoom | Zoom | 1.6.2 | enabled | marketplace-auto-install (version approximated: 1.6.1) |
| com.example.internal-plugin | Internal Widget | 1.0.0 | enabled | MANUAL — not in marketplace |

## Manual Installation Instructions

### com.example.internal-plugin (Internal Widget v1.0.0)

This plugin was enabled in the customer environment but is not available in the
official Mattermost marketplace. To reproduce issues involving this plugin:

1. Obtain the plugin bundle (`.tar.gz`) from your internal artifact store
2. Navigate to System Console > Plugin Management
3. Click "Upload Plugin" and select the bundle
4. Enable the plugin after upload

Note: If you cannot obtain this plugin, issues related to it cannot be
reproduced in this environment.
```

---

## Auto-Install Mechanism Details

When auto-installing marketplace plugins, the generated environment uses a two-phase approach:

### Phase 1: Marketplace API Install

The init container calls the Mattermost marketplace install endpoint:

```
POST /api/v4/plugins/marketplace
{
  "plugin_id": "com.mattermost.jira",
  "version": "4.1.0"
}
```

This triggers Mattermost to download the plugin bundle from the official marketplace CDN and install it.

**Requirement:** The Docker host must have outbound internet access to `api.integrations.mattermost.com` for this to work.

### Phase 2: Enable the Plugin

After install, the plugin is enabled via:

```
POST /api/v4/plugins/{plugin_id}/enable
```

### Offline Mode

If the Docker host does not have internet access, auto-install will fail gracefully. The Mattermost server will log the failure, and the plugin will not be installed. The PLUGIN_REPORT.md includes a note about this:

```markdown
> NOTE: Auto-install requires outbound internet access to the Mattermost
> marketplace CDN. If your machine is offline, download the plugin bundles
> manually and install them via System Console > Plugin Management.
```

---

## Plugin Configuration

Most marketplace plugins require additional configuration (API tokens, server URLs, etc.) to function. Since these are customer-specific secrets, they are never carried over from the support package.

After installing a plugin, you will need to configure it manually in System Console > Plugin Management > (plugin name) > Settings.

For reproducing issues that involve specific plugin configuration, reconstruct the relevant settings from the ticket description, not from the support package config.

---

## Version Matching

The marketplace resolver attempts to find an exact version match for each plugin. The resolution order is:

1. Exact version match: `com.mattermost.jira 4.1.0`
2. Compatible patch version: most recent `4.1.x`
3. Compatible minor version: most recent `4.x.y`
4. Latest available version

Any approximation is recorded in `PLUGIN_REPORT.md` and `REPRO_SUMMARY.md`.

---

## Adding New Plugin Entries

### Adding a Known Builtin

If a new version of Mattermost bundles a previously marketplace-only plugin, add it to `pkg/marketplace/builtins.go`:

```go
var builtinRegistry = map[string][]BuiltinEntry{
    "com.mattermost.calls": {
        {MinVersion: "7.0.0", MaxVersion: ""},  // bundled since 7.0
    },
    "com.mattermost.my-new-bundled-plugin": {
        {MinVersion: "10.5.0", MaxVersion: ""},  // bundled since 10.5
    },
    // ...
}
```

The `MinVersion` and `MaxVersion` fields use the format `"major.minor.patch"`. An empty `MaxVersion` means "all versions from MinVersion onwards".

### Overriding Marketplace Resolution

For plugins that have unusual behavior (e.g., require a specific minimum server version, have been renamed, or are only available in enterprise editions), you can add an override to `pkg/marketplace/overrides.go`:

```go
var marketplaceOverrides = map[string]MarketplaceOverride{
    "com.mattermost.old-plugin-id": {
        RedirectTo: "com.mattermost.new-plugin-id",
        Note:       "Plugin was renamed in marketplace v2.0",
    },
    "com.example.enterprise-only": {
        RequiresEnterprise: true,
        Note: "Only available with Enterprise Edition license",
    },
}
```

### Adding a Well-Known Custom Plugin

If your organization uses a custom plugin that is commonly seen in support tickets, you can add it to `pkg/marketplace/known_custom.go` with a note that helps engineers find it:

```go
var knownCustomPlugins = map[string]KnownCustomEntry{
    "com.example.our-internal-plugin": {
        Name: "Our Internal Plugin",
        Note: "Internal only — find at artifactory.example.com/mattermost-plugins/",
        Source: "internal",
    },
}
```

These entries appear in `PLUGIN_REPORT.md` with the note attached to the manual installation instructions.

---

## Plugin Detection Debugging

If you suspect a plugin is being misclassified, use `mm-repro validate --verbose`:

```bash
mm-repro validate --support-package ./customer.zip --verbose
```

The verbose output includes plugin detection details:

```
Plugins detected (7):
  com.mattermost.calls v0.21.1 [enabled]
    → classification: builtin (known prepackaged plugin in 9.11.2)
  com.mattermost.jira v4.1.0 [enabled]
    → classification: marketplace (found in marketplace API)
  com.example.custom-plugin v1.0.0 [enabled]
    → classification: custom (not found in builtins or marketplace)
```

You can also inspect the marketplace API response directly:
```bash
curl -s "https://api.integrations.mattermost.com/api/v1/plugins?filter=com.mattermost.jira" | jq .
```
