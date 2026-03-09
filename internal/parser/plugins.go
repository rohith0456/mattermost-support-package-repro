package parser

import (
	"strings"

	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

// knownSafePlugins lists plugin IDs that can be safely auto-installed
// from the official Mattermost marketplace.
var knownSafePlugins = map[string]string{
	"com.mattermost.calls":                     "https://github.com/mattermost/mattermost-plugin-calls/releases",
	"playbooks":                                "https://github.com/mattermost/mattermost-plugin-playbooks/releases",
	"com.mattermost.plugin-channel-export":     "https://github.com/mattermost/mattermost-plugin-channel-export/releases",
	"com.mattermost.nps":                       "https://github.com/mattermost/mattermost-plugin-nps/releases",
	"com.mattermost.plugin-todo":               "https://github.com/mattermost/mattermost-plugin-todo/releases",
	"com.mattermost.welcomebot":                "https://github.com/mattermost/mattermost-plugin-welcomebot/releases",
	"com.github.manland.mattermost-plugin-gitlab": "https://github.com/mattermost/mattermost-plugin-gitlab/releases",
	"com.github.moussetc.mattermost.plugin.giphy": "",
	"com.mattermost.plugin-incident-management": "https://github.com/mattermost/mattermost-plugin-playbooks/releases",
	"com.mattermost.apps":                      "",
	"focalboard":                               "https://github.com/mattermost/focalboard/releases",
	"com.mattermost.msteams-sync":              "https://github.com/mattermost/mattermost-plugin-msteams-sync/releases",
}

// builtinPlugins are shipped with Mattermost and don't need separate installation.
var builtinPlugins = map[string]bool{
	"com.mattermost.nps":    true,
	"com.mattermost.apps":   true,
	"playbooks":             false, // bundled in newer versions
}

// ParsePlugins extracts plugin information from the normalized package.
func ParsePlugins(np *ingestion.NormalizedPackage) []models.PluginInfo {
	var plugins []models.PluginInfo
	seen := make(map[string]bool)

	// Try PluginSettings in config
	plugins = append(plugins, parseFromPluginSettings(np.Config, seen)...)

	// Try plugins.json / plugin_statuses.json
	plugins = append(plugins, parseFromPluginJSON(np.PluginInfo, seen)...)

	// Try diagnostics
	plugins = append(plugins, parseFromDiagnostics(np.Diagnostics, seen)...)

	return plugins
}

func parseFromPluginSettings(config map[string]interface{}, seen map[string]bool) []models.PluginInfo {
	var plugins []models.PluginInfo

	pluginSettings := getNestedMap(config, "PluginSettings")
	if pluginSettings == nil {
		return plugins
	}

	// PluginStates map: pluginID -> {Enable: bool}
	if states, ok := pluginSettings["PluginStates"].(map[string]interface{}); ok {
		for id, state := range states {
			if seen[id] {
				continue
			}
			seen[id] = true

			enabled := false
			if sm, ok := state.(map[string]interface{}); ok {
				if e, ok := sm["Enable"].(bool); ok {
					enabled = e
				}
			}

			plugins = append(plugins, buildPluginInfo(id, "", enabled))
		}
	}

	return plugins
}

func parseFromPluginJSON(pluginData map[string]interface{}, seen map[string]bool) []models.PluginInfo {
	var plugins []models.PluginInfo
	if pluginData == nil {
		return plugins
	}

	// Try array under "plugins" key
	rawList := pluginData["plugins"]
	if rawList == nil {
		rawList = pluginData["plugin_statuses"]
	}
	if rawList == nil {
		// Maybe it IS the list
		rawList = pluginData
	}

	if list, ok := rawList.([]interface{}); ok {
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			id := getNestedString(m, "plugin_id")
			if id == "" {
				id = getNestedString(m, "id")
			}
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			version := getNestedString(m, "version")
			if version == "" {
				version = getNestedString(m, "Version")
			}
			enabled := getNestedString(m, "state") == "running" ||
				getNestedString(m, "State") == "running" ||
				getNestedString(m, "active") == "true"
			p := buildPluginInfo(id, version, enabled)
			plugins = append(plugins, p)
		}
	}

	return plugins
}

func parseFromDiagnostics(diag map[string]interface{}, seen map[string]bool) []models.PluginInfo {
	var plugins []models.PluginInfo
	if diag == nil {
		return plugins
	}

	// Check diagnostics for plugin IDs under various keys
	for _, key := range []string{"active_plugins", "plugins", "installed_plugins"} {
		if rawList, ok := diag[key].([]interface{}); ok {
			for _, item := range rawList {
				var id string
				switch v := item.(type) {
				case string:
					id = v
				case map[string]interface{}:
					id = getNestedString(v, "id")
					if id == "" {
						id = getNestedString(v, "plugin_id")
					}
				}
				if id == "" || seen[id] {
					continue
				}
				seen[id] = true
				plugins = append(plugins, buildPluginInfo(id, "", true))
			}
		}
	}

	return plugins
}

func buildPluginInfo(id, version string, enabled bool) models.PluginInfo {
	p := models.PluginInfo{
		ID:      id,
		Version: version,
		Enabled: enabled,
	}

	// Normalize ID
	p.ID = strings.TrimSpace(p.ID)

	// Detect if builtin
	if builtinPlugins[id] {
		p.IsBuiltin = true
		p.Source = "builtin"
	}

	// Detect if safe to auto-install
	if url, safe := knownSafePlugins[id]; safe {
		p.AutoInstall = true
		p.Source = "marketplace"
		if url != "" {
			// Store reference URL (public GitHub releases, not customer URL)
			_ = url
		}
	}

	if !p.IsBuiltin && !p.AutoInstall {
		p.Source = "custom"
	}

	return p
}
