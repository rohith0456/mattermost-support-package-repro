package ingestion

import (
	"encoding/json"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// NormalizedPackage holds the structured, normalized content extracted
// from the various files in a support package.
type NormalizedPackage struct {
	Config        map[string]interface{}
	Diagnostics   map[string]interface{}
	SystemInfo    map[string]interface{}
	ClusterInfo   map[string]interface{}
	PluginInfo    map[string]interface{}
	EnvVars       map[string]string
	LogSnippets   []string
	ExtraJSON     []map[string]interface{}
	Warnings      []string
}

// Normalizer extracts known file types from a PackageInfo.
type Normalizer struct{}

// NewNormalizer creates a Normalizer.
func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

// Normalize extracts all known content from a PackageInfo into a NormalizedPackage.
func (n *Normalizer) Normalize(pkg *PackageInfo) *NormalizedPackage {
	np := &NormalizedPackage{
		Config:      make(map[string]interface{}),
		Diagnostics: make(map[string]interface{}),
		SystemInfo:  make(map[string]interface{}),
		ClusterInfo: make(map[string]interface{}),
		PluginInfo:  make(map[string]interface{}),
		EnvVars:     make(map[string]string),
	}

	// Config JSON - multiple possible names
	for _, name := range []string{"config.json", "config/config.json", "mattermost/config/config.json"} {
		if path := pkg.FindFile(name); path != "" {
			if data, err := readJSON(path); err == nil {
				np.Config = data
				break
			} else {
				np.Warnings = append(np.Warnings, "could not parse config.json: "+err.Error())
			}
		}
	}

	// Sanitized config (preferred if both exist)
	for _, name := range []string{"sanitized_config.json", "config_sanitized.json"} {
		if path := pkg.FindFile(name); path != "" {
			if data, err := readJSON(path); err == nil {
				np.Config = data // prefer sanitized version
				break
			}
		}
	}

	// Diagnostics — JSON and YAML formats
	for _, name := range []string{"diagnostic.json", "diagnostics.json", "support_packet.json"} {
		if path := pkg.FindFile(name); path != "" {
			if data, err := readJSON(path); err == nil {
				np.Diagnostics = data
				break
			}
		}
	}
	// YAML diagnostics (modern support packet format: diagnostics.yaml)
	if len(np.Diagnostics) == 0 {
		for _, name := range []string{"diagnostics.yaml", "diagnostic.yaml", "metadata.yaml"} {
			if path := pkg.FindFile(name); path != "" {
				if data, err := readYAML(path); err == nil {
					np.Diagnostics = data
					break
				}
			}
		}
	}

	// System info
	for _, name := range []string{"system_info.json", "systeminfo.json", "sysinfo.json"} {
		if path := pkg.FindFile(name); path != "" {
			if data, err := readJSON(path); err == nil {
				np.SystemInfo = data
				break
			}
		}
	}

	// Cluster info
	for _, name := range []string{"cluster_info.json", "cluster.json", "ha_info.json"} {
		if path := pkg.FindFile(name); path != "" {
			if data, err := readJSON(path); err == nil {
				np.ClusterInfo = data
				break
			}
		}
	}

	// Plugin info
	for _, name := range []string{"plugins.json", "plugin_statuses.json", "installed_plugins.json"} {
		if path := pkg.FindFile(name); path != "" {
			if data, err := readJSON(path); err == nil {
				np.PluginInfo = data
				break
			}
		}
	}

	// Log snippet (first 500 lines of mattermost.log)
	for _, name := range []string{"mattermost.log", "mattermost/logs/mattermost.log"} {
		if path := pkg.FindFile(name); path != "" {
			if lines, err := readFirstLines(path, 500); err == nil {
				np.LogSnippets = lines
				break
			}
		}
	}

	// Collect any other JSON files as extra hints
	for _, rel := range pkg.RawFiles {
		if strings.HasSuffix(strings.ToLower(rel), ".json") {
			if isKnownFile(rel) {
				continue
			}
			path := pkg.FindFile(rel)
			if path == "" {
				continue
			}
			if data, err := readJSON(path); err == nil {
				np.ExtraJSON = append(np.ExtraJSON, data)
			}
		}
	}

	return np
}

func isKnownFile(rel string) bool {
	known := []string{"config.json", "diagnostic.json", "system_info.json",
		"cluster_info.json", "plugins.json", "sanitized_config.json"}
	base := strings.ToLower(strings.TrimPrefix(rel, "/"))
	for _, k := range known {
		if strings.HasSuffix(base, k) {
			return true
		}
	}
	return false
}

func readYAML(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func readJSON(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func readFirstLines(path string, maxLines int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines, nil
}
