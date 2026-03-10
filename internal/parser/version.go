// Package parser extracts structured information from a normalized support package.
package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

var (
	// semverRe matches versions like 7.8.1, 8.0.0-rc1, 9.11.0-ENTERPRISE
	semverRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)(?:[.\-](\S+))?`)
)

// ParseVersion extracts version information from the normalized package.
func ParseVersion(np *ingestion.NormalizedPackage) models.VersionInfo {
	info := models.VersionInfo{
		Edition: "unknown",
	}

	// Try diagnostics/metadata first — metadata.yaml has an explicit server_version
	// field that is more reliable than config.json's generic "version" field
	// (which is a config schema version, not the MM server version).
	if raw := extractVersionFromDiagnostics(np.Diagnostics); raw != "" {
		info.Raw = raw
	}

	// Try config (only ServiceSettings.Version — skip generic "version" fields
	// that may hold config schema versions rather than the MM server version)
	if info.Raw == "" {
		if raw := extractVersionFromConfig(np.Config); raw != "" {
			info.Raw = raw
		}
	}

	// Try system info
	if info.Raw == "" {
		if raw := extractVersionFromSystemInfo(np.SystemInfo); raw != "" {
			info.Raw = raw
		}
	}

	// Try log lines
	if info.Raw == "" {
		if raw := extractVersionFromLogs(np.LogSnippets); raw != "" {
			info.Raw = raw
		}
	}

	if info.Raw == "" {
		info.Raw = "unknown"
		info.Normalized = "unknown"
		return info
	}

	// Parse semver
	info.Normalized, info.Major, info.Minor, info.Patch = normalizeSemver(info.Raw)

	// Detect edition
	info.Edition = detectEdition(info.Raw, np.Config, np.Diagnostics)

	// Choose Docker image tag
	info.DockerImageTag, info.ImageTagExact = chooseDockerTag(info.Normalized, info.Edition)

	return info
}

func extractVersionFromConfig(config map[string]interface{}) string {
	// Only use ServiceSettings.Version — the top-level "version" / "Version" fields
	// in config.json are config schema versions (e.g. "2.4.0"), NOT the MM server
	// version, so they must not be used here.
	paths := [][]string{
		{"ServiceSettings", "Version"},
	}
	for _, path := range paths {
		if v := getNestedString(config, path...); v != "" {
			return v
		}
	}
	return ""
}

func extractVersionFromDiagnostics(diag map[string]interface{}) string {
	// Only use explicit server version fields. The generic "version" / "Version"
	// keys are intentionally excluded — in metadata.yaml "version" is the format
	// schema integer (e.g. 1), and in support_packet.json it can be a legacy
	// config schema string (e.g. "2.4.0"). Neither represents the MM server version.
	paths := [][]string{
		{"server_version"},    // metadata.yaml: server_version: 10.11.4
		{"server", "version"}, // diagnostics.yaml nested: server.version
		{"ServerVersion"},
		{"mattermost_version"},
	}
	for _, path := range paths {
		if v := getNestedString(diag, path...); v != "" {
			return v
		}
	}
	return ""
}

func extractVersionFromSystemInfo(sysinfo map[string]interface{}) string {
	paths := [][]string{
		{"BuildNumber"},
		{"build_number"},
		{"Version"},
		{"version"},
	}
	for _, path := range paths {
		if v := getNestedString(sysinfo, path...); v != "" {
			return v
		}
	}
	return ""
}

func extractVersionFromLogs(lines []string) string {
	// Look for version strings in log lines like:
	// "Starting Server... version=8.0.1"
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`version[=:][\s]*([\d]+\.[\d]+\.[\d]+[^\s]*)`),
		regexp.MustCompile(`Mattermost\s+([\d]+\.[\d]+\.[\d]+[^\s]*)`),
		regexp.MustCompile(`"version":"([\d]+\.[\d]+\.[\d]+[^"]*)"`),
	}
	for _, line := range lines {
		for _, re := range patterns {
			if m := re.FindStringSubmatch(line); len(m) > 1 {
				return m[1]
			}
		}
	}
	return ""
}

func normalizeSemver(raw string) (normalized string, major, minor, patch int) {
	m := semverRe.FindStringSubmatch(raw)
	if m == nil {
		return raw, 0, 0, 0
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	patch, _ = strconv.Atoi(m[3])
	normalized = fmt.Sprintf("%d.%d.%d", major, minor, patch)
	return
}

func detectEdition(raw string, config, diag map[string]interface{}) string {
	// Check for enterprise indicators
	rawLower := strings.ToLower(raw)
	if strings.Contains(rawLower, "enterprise") {
		return "enterprise"
	}

	// Check license info in config
	if lic := getNestedMap(config, "LicenseSettings"); lic != nil {
		return "enterprise"
	}

	// Check license SKU in diagnostics (e.g. diagnostics.yaml: license.sku_short_name)
	if sku := getNestedString(diag, "license", "sku_short_name"); sku != "" && sku != "team" {
		return "enterprise"
	}

	// Check build tags
	if bt := getNestedString(diag, "build_enterprise_ready"); bt == "true" {
		return "enterprise"
	}
	if bt := getNestedString(config, "BuildEnterpriseReady"); bt == "true" {
		return "enterprise"
	}

	// Check for cloud indicators
	if strings.Contains(rawLower, "cloud") {
		return "cloud"
	}

	return "team"
}

// chooseDockerTag returns the best available Docker Hub tag for a given version.
// It returns (tag, exact) where exact is false if we had to fall back to a minor version.
func chooseDockerTag(normalized, edition string) (string, bool) {
	if normalized == "" || normalized == "unknown" {
		return "latest", false
	}

	// Enterprise gets the -enterprise suffix (or just the version for newer tags)
	// For simplicity we'll use the mattermost/mattermost-team-edition or
	// mattermost/mattermost-enterprise-edition images.
	// The actual image check would require a registry call; here we just form the tag.
	_ = edition // used for future image selection

	tag := normalized
	return tag, true
}

// getNestedString safely traverses a map using dot-separated keys.
func getNestedString(m map[string]interface{}, keys ...string) string {
	current := m
	for i, key := range keys {
		val, ok := current[key]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			if s, ok := val.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", val)
		}
		next, ok := val.(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

func getNestedMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	current := m
	for _, key := range keys {
		val, ok := current[key]
		if !ok {
			return nil
		}
		next, ok := val.(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}
	return current
}
