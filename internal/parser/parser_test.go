package parser_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/internal/parser"
)

func makeNormalized(config map[string]interface{}) *ingestion.NormalizedPackage {
	return &ingestion.NormalizedPackage{
		Config:      config,
		Diagnostics: make(map[string]interface{}),
		SystemInfo:  make(map[string]interface{}),
		ClusterInfo: make(map[string]interface{}),
		PluginInfo:  make(map[string]interface{}),
		EnvVars:     make(map[string]string),
	}
}

func TestParseVersion_FromConfig(t *testing.T) {
	tests := []struct {
		name            string
		config          map[string]interface{}
		expectedVersion string
		expectedEdition string
	}{
		{
			name: "standard version in ServiceSettings",
			config: map[string]interface{}{
				"ServiceSettings": map[string]interface{}{
					"Version": "8.1.3",
				},
			},
			expectedVersion: "8.1.3",
			expectedEdition: "team",
		},
		{
			name: "enterprise version with license settings",
			config: map[string]interface{}{
				"ServiceSettings": map[string]interface{}{
					"Version": "9.3.0",
				},
				"LicenseSettings": map[string]interface{}{
					"IsLicensed": true,
				},
			},
			expectedVersion: "9.3.0",
			expectedEdition: "enterprise",
		},
		{
			name:            "empty config returns unknown",
			config:          map[string]interface{}{},
			expectedVersion: "unknown",
			expectedEdition: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			np := makeNormalized(tc.config)
			info := parser.ParseVersion(np)
			assert.Equal(t, tc.expectedVersion, info.Normalized)
			assert.Equal(t, tc.expectedEdition, info.Edition)
		})
	}
}

func TestParseDatabase(t *testing.T) {
	tests := []struct {
		name         string
		config       map[string]interface{}
		expectedType string
		hasReplica   bool
	}{
		{
			name: "postgres driver",
			config: map[string]interface{}{
				"SqlSettings": map[string]interface{}{
					"DriverName": "postgres",
				},
			},
			expectedType: "postgres",
		},
		{
			name: "mysql driver",
			config: map[string]interface{}{
				"SqlSettings": map[string]interface{}{
					"DriverName": "mysql",
				},
			},
			expectedType: "mysql",
		},
		{
			name: "postgres with replicas",
			config: map[string]interface{}{
				"SqlSettings": map[string]interface{}{
					"DriverName": "postgres",
					"DataSourceReplicas": []interface{}{
						"postgres://replica1/mm",
					},
				},
			},
			expectedType: "postgres",
			hasReplica:   true,
		},
		{
			name:         "empty config defaults to unknown",
			config:       map[string]interface{}{},
			expectedType: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			np := makeNormalized(tc.config)
			info := parser.ParseDatabase(np)
			assert.Equal(t, tc.expectedType, info.Type)
			assert.Equal(t, tc.hasReplica, info.HasReplica)
		})
	}
}

func TestParseAuth(t *testing.T) {
	t.Run("LDAP enabled", func(t *testing.T) {
		np := makeNormalized(map[string]interface{}{
			"LdapSettings": map[string]interface{}{
				"Enable": true,
			},
		})
		info := parser.ParseAuth(np)
		assert.True(t, info.HasLDAP)
	})

	t.Run("SAML enabled with Okta", func(t *testing.T) {
		np := makeNormalized(map[string]interface{}{
			"SamlSettings": map[string]interface{}{
				"Enable": true,
				"IdpURL": "https://customer.okta.com/app/saml",
			},
		})
		info := parser.ParseAuth(np)
		assert.True(t, info.HasSAML)
		assert.Equal(t, "okta", info.SAMLProvider)
	})

	t.Run("no auth configured", func(t *testing.T) {
		np := makeNormalized(map[string]interface{}{})
		info := parser.ParseAuth(np)
		assert.False(t, info.HasLDAP)
		assert.False(t, info.HasSAML)
	})
}

func TestParsePlugins(t *testing.T) {
	t.Run("plugins from PluginStates", func(t *testing.T) {
		np := makeNormalized(map[string]interface{}{
			"PluginSettings": map[string]interface{}{
				"PluginStates": map[string]interface{}{
					"playbooks": map[string]interface{}{
						"Enable": true,
					},
					"com.mattermost.calls": map[string]interface{}{
						"Enable": true,
					},
					"com.example.custom": map[string]interface{}{
						"Enable": false,
					},
				},
			},
		})
		plugins := parser.ParsePlugins(np)
		require.Len(t, plugins, 3)

		// Find each plugin
		var customPlugin, playbooksPlugin *struct {
			ID          string
			AutoInstall bool
			Source      string
		}
		for _, p := range plugins {
			if p.ID == "com.example.custom" {
				customPlugin = &struct {
					ID          string
					AutoInstall bool
					Source      string
				}{p.ID, p.AutoInstall, p.Source}
			}
			if p.ID == "playbooks" {
				playbooksPlugin = &struct {
					ID          string
					AutoInstall bool
					Source      string
				}{p.ID, p.AutoInstall, p.Source}
			}
		}

		require.NotNil(t, customPlugin)
		assert.False(t, customPlugin.AutoInstall)
		assert.Equal(t, "custom", customPlugin.Source)

		require.NotNil(t, playbooksPlugin)
		assert.True(t, playbooksPlugin.AutoInstall)
	})
}

func TestParseTopology(t *testing.T) {
	t.Run("cluster enabled", func(t *testing.T) {
		np := makeNormalized(map[string]interface{}{
			"ClusterSettings": map[string]interface{}{
				"Enable": true,
			},
		})
		info := parser.ParseTopology(np)
		assert.True(t, info.IsCluster)
	})

	t.Run("no cluster", func(t *testing.T) {
		np := makeNormalized(map[string]interface{}{})
		info := parser.ParseTopology(np)
		assert.False(t, info.IsCluster)
		assert.Equal(t, 1, info.NodeCount)
	})
}

func TestVersionNormalization(t *testing.T) {
	tests := []struct {
		raw        string
		normalized string
		major      int
		minor      int
		patch      int
	}{
		{"8.1.3", "8.1.3", 8, 1, 3},
		{"9.3.0-ENTERPRISE", "9.3.0", 9, 3, 0},
		{"10.0.0+rc1", "10.0.0", 10, 0, 0},
		{"7.8.1.release-1234", "7.8.1", 7, 8, 1},
	}

	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			np := makeNormalized(map[string]interface{}{
				"ServiceSettings": map[string]interface{}{
					"Version": tc.raw,
				},
			})
			info := parser.ParseVersion(np)
			assert.Equal(t, tc.normalized, info.Normalized, "normalized version")
			assert.Equal(t, tc.major, info.Major)
			assert.Equal(t, tc.minor, info.Minor)
			assert.Equal(t, tc.patch, info.Patch)
		})
	}
}

// TestParseVersionFromJSON uses embedded JSON fixtures.
func TestParseVersionFromJSON(t *testing.T) {
	configJSON := `{
		"ServiceSettings": {
			"Version": "8.1.0",
			"SiteURL": "https://customer.example.com"
		},
		"SqlSettings": {
			"DriverName": "postgres",
			"DataSource": "postgres://user:REDACTED@db:5432/mm"
		},
		"LdapSettings": {
			"Enable": true,
			"LdapServer": "ldap.internal.example.com",
			"BindPassword": "REDACTED_PASSWORD"
		},
		"ClusterSettings": {
			"Enable": false
		}
	}`

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(configJSON), &config))

	np := makeNormalized(config)
	p := parser.NewParser()
	sp := p.Parse(np, "./test.zip")

	assert.Equal(t, "8.1.0", sp.Version.Normalized)
	assert.Equal(t, "postgres", sp.Database.Type)
	assert.True(t, sp.Auth.HasLDAP)
	assert.False(t, sp.Topology.IsCluster)
}
