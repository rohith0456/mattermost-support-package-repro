// Package models defines the core data structures for mm-repro.
package models

import "time"

// SupportPackage represents the parsed and normalized contents of a
// Mattermost support package ZIP archive.
type SupportPackage struct {
	// Source metadata
	SourcePath    string    `json:"source_path"`
	ExtractedAt   time.Time `json:"extracted_at"`
	PackageFormat string    `json:"package_format"` // "standard", "cloud", "legacy"

	// Version information
	Version VersionInfo `json:"version"`

	// Detected topology
	Topology TopologyInfo `json:"topology"`

	// Database information
	Database DatabaseInfo `json:"database"`

	// Search backend
	Search SearchInfo `json:"search"`

	// Auth backends
	Auth AuthInfo `json:"auth"`

	// File storage
	FileStorage FileStorageInfo `json:"file_storage"`

	// Plugins
	Plugins []PluginInfo `json:"plugins"`

	// Integrations
	Integrations IntegrationInfo `json:"integrations"`

	// Observability
	Observability ObservabilityInfo `json:"observability"`

	// Raw config snapshot (after redaction)
	RedactedConfig map[string]interface{} `json:"redacted_config,omitempty"`

	// License info (type only, never the license itself)
	License LicenseInfo `json:"license"`

	// Parse warnings and notes
	ParseWarnings []string `json:"parse_warnings,omitempty"`
	ParseNotes    []string `json:"parse_notes,omitempty"`
}

// VersionInfo holds Mattermost version details.
type VersionInfo struct {
	Raw              string `json:"raw"`               // as found in package
	Normalized       string `json:"normalized"`         // normalized semver
	Major            int    `json:"major"`
	Minor            int    `json:"minor"`
	Patch            int    `json:"patch"`
	BuildNumber      string `json:"build_number,omitempty"`
	DockerImageTag   string `json:"docker_image_tag"`   // best tag to use
	ImageTagExact    bool   `json:"image_tag_exact"`    // true if exact match available
	Edition          string `json:"edition"`            // "team", "enterprise", "cloud", "unknown"
}

// TopologyInfo describes the deployment topology.
type TopologyInfo struct {
	NodeCount       int      `json:"node_count"`
	IsCluster       bool     `json:"is_cluster"`
	NodeIDs         []string `json:"node_ids,omitempty"`
	HasReverseProxy bool     `json:"has_reverse_proxy"`
	ProxyType       string   `json:"proxy_type,omitempty"` // "nginx", "apache", "haproxy", "unknown"
	SiteURL         string   `json:"site_url,omitempty"`   // sanitized, local use only
	HasTLS          bool     `json:"has_tls"`
}

// DatabaseInfo describes the database backend.
type DatabaseInfo struct {
	Type          string `json:"type"`           // "postgres", "mysql", "unknown"
	Version       string `json:"version,omitempty"`
	HasReplica    bool   `json:"has_replica"`
	ReplicaCount  int    `json:"replica_count"`
	// Connection details are NEVER stored — only type/version
}

// SearchInfo describes the search backend.
type SearchInfo struct {
	Backend        string `json:"backend"`         // "elasticsearch", "opensearch", "database", "unknown"
	Version        string `json:"version,omitempty"`
	IndexPrefix    string `json:"index_prefix,omitempty"`
	LiveIndexing   bool   `json:"live_indexing"`
	BulkIndexing   bool   `json:"bulk_indexing"`
}

// AuthInfo describes authentication backends.
type AuthInfo struct {
	HasLDAP       bool   `json:"has_ldap"`
	LDAPType      string `json:"ldap_type,omitempty"` // "openldap", "ad", "unknown"
	HasSAML       bool   `json:"has_saml"`
	SAMLProvider  string `json:"saml_provider,omitempty"`
	HasOIDC       bool   `json:"has_oidc"`
	OIDCProvider  string `json:"oidc_provider,omitempty"`
	HasGitLab     bool   `json:"has_gitlab"`
	HasGoogle     bool   `json:"has_google"`
	HasOffice365  bool   `json:"has_office365"`
	MFA           bool   `json:"mfa"`
	GuestAccounts bool   `json:"guest_accounts"`
}

// FileStorageInfo describes the file storage backend.
type FileStorageInfo struct {
	Backend        string `json:"backend"`       // "local", "s3", "azure", "unknown"
	BucketName     string `json:"bucket_name,omitempty"` // sanitized name only
	CDNEnabled     bool   `json:"cdn_enabled"`
	LocalPath      string `json:"local_path,omitempty"`
	// Credentials are NEVER stored
}

// PluginInfo describes a single plugin.
type PluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	Enabled     bool   `json:"enabled"`
	IsBuiltin   bool   `json:"is_builtin"`
	Source      string `json:"source,omitempty"` // "marketplace", "custom", "unknown"
	AutoInstall bool   `json:"auto_install"`     // safe to auto-install locally?
}

// IntegrationInfo describes configured integrations.
type IntegrationInfo struct {
	HasIncomingWebhooks  bool `json:"has_incoming_webhooks"`
	HasOutgoingWebhooks  bool `json:"has_outgoing_webhooks"`
	HasSlashCommands     bool `json:"has_slash_commands"`
	HasBotAccounts       bool `json:"has_bot_accounts"`
	HasOAuthApps         bool `json:"has_oauth_apps"`
	HasSMTP              bool `json:"has_smtp"`
	SMTPProvider         string `json:"smtp_provider,omitempty"`
	HasCalls             bool `json:"has_calls"`
	HasRTCD              bool `json:"has_rtcd"`
	HasPush              bool `json:"has_push"`
	PushProxy            string `json:"push_proxy,omitempty"`
	// Webhook/token values are NEVER stored
}

// ObservabilityInfo describes monitoring configuration.
type ObservabilityInfo struct {
	HasPrometheus    bool   `json:"has_prometheus"`
	MetricsEnabled   bool   `json:"metrics_enabled"`
	MetricsPath      string `json:"metrics_path,omitempty"`
	HasPerformanceMon bool  `json:"has_performance_mon"`
}

// LicenseInfo holds non-sensitive license metadata.
type LicenseInfo struct {
	Type       string `json:"type"`        // "e0", "e10", "e20", "enterprise", "starter", "none", "unknown"
	IsExpired  bool   `json:"is_expired"`
	SkuShortName string `json:"sku_short_name,omitempty"`
	// No license file, no expiry dates, no customer info
}
