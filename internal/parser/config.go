package parser

import (
	"regexp"

	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

// ParseDatabase extracts database configuration signals.
func ParseDatabase(np *ingestion.NormalizedPackage) models.DatabaseInfo {
	info := models.DatabaseInfo{Type: "unknown"}

	dsSettings := getNestedMap(np.Config, "SqlSettings")
	if dsSettings == nil {
		dsSettings = getNestedMap(np.Config, "SqlSettings")
	}
	if dsSettings == nil {
		// Try diagnostics
		dsSettings = getNestedMap(np.Diagnostics, "SqlSettings")
	}

	if dsSettings != nil {
		// Detect type from DriverName
		driver := getNestedString(dsSettings, "DriverName")
		switch driver {
		case "postgres", "postgresql":
			info.Type = "postgres"
		case "mysql":
			info.Type = "mysql"
		}

		// Check replica count (DataSourceReplicas)
		if replicas, ok := dsSettings["DataSourceReplicas"].([]interface{}); ok && len(replicas) > 0 {
			info.HasReplica = true
			info.ReplicaCount = len(replicas)
		}
	}

	// Try to infer from connection string hints (without storing the actual DSN)
	if dsn := getNestedString(np.Config, "SqlSettings", "DataSource"); dsn != "" {
		if info.Type == "unknown" {
			if containsAny(dsn, []string{"postgres://", "postgresql://", "host=", "dbname="}) {
				info.Type = "postgres"
			} else if containsAny(dsn, []string{"mysql://", "tcp(", "charset=utf8"}) {
				info.Type = "mysql"
			}
		}
	}

	return info
}

// ParseFileStorage extracts file storage configuration signals.
func ParseFileStorage(np *ingestion.NormalizedPackage) models.FileStorageInfo {
	info := models.FileStorageInfo{Backend: "local"}

	fsSettings := getNestedMap(np.Config, "FileSettings")
	if fsSettings == nil {
		return info
	}

	driver := getNestedString(fsSettings, "DriverName")
	switch driver {
	case "amazons3", "s3":
		info.Backend = "s3"
		// Store only the bucket name (not credentials)
		info.BucketName = getNestedString(fsSettings, "AmazonS3Bucket")
	case "azureblob", "azure":
		info.Backend = "azure"
	case "local", "":
		info.Backend = "local"
		info.LocalPath = getNestedString(fsSettings, "Directory")
	}

	// CDN
	if cdn := getNestedString(fsSettings, "AmazonS3PathPrefix"); cdn != "" {
		info.CDNEnabled = true
	}
	if cdn := getNestedString(fsSettings, "CdnURL"); cdn != "" {
		info.CDNEnabled = true
	}

	return info
}

// ParseTopology extracts topology and cluster information.
func ParseTopology(np *ingestion.NormalizedPackage) models.TopologyInfo {
	info := models.TopologyInfo{NodeCount: 1}

	// Cluster settings
	clusterSettings := getNestedMap(np.Config, "ClusterSettings")
	if clusterSettings != nil {
		if enabled, ok := clusterSettings["Enable"].(bool); ok && enabled {
			info.IsCluster = true
		}
		if enabled, ok := clusterSettings["Enable"].(string); ok && enabled == "true" {
			info.IsCluster = true
		}
	}

	// Check cluster info JSON
	if np.ClusterInfo != nil {
		if nodes, ok := np.ClusterInfo["Nodes"].([]interface{}); ok {
			info.NodeCount = len(nodes)
			if info.NodeCount > 1 {
				info.IsCluster = true
			}
			for _, n := range nodes {
				if nm, ok := n.(map[string]interface{}); ok {
					if id := getNestedString(nm, "id"); id != "" {
						info.NodeIDs = append(info.NodeIDs, id)
					}
				}
			}
		}
	}

	// Site URL and TLS hints
	siteURL := getNestedString(np.Config, "ServiceSettings", "SiteURL")
	if siteURL != "" {
		if hasPrefix(siteURL, "https://") {
			info.HasTLS = true
		}
		// Don't store the real URL — just the TLS and proxy hints
		if containsAny(siteURL, []string{"/", "."}) {
			info.SiteURL = "http://localhost:8065" // always replace with local
		}
	}

	// Reverse proxy hints from HTTP settings
	if forward := getNestedString(np.Config, "ServiceSettings", "TrustedProxyIPHeader"); forward != "" {
		info.HasReverseProxy = true
	}
	if forward := getNestedString(np.Config, "ServiceSettings", "AllowedUntrustedInternalConnections"); forward != "" {
		info.HasReverseProxy = true
	}

	// Kubernetes detection — check node IDs for k8s pod naming patterns
	// Deployment pods: <name>-<replicaset-hash>-<pod-hash>  e.g. mattermost-7d8f4b5c6-2xzpk
	// StatefulSet pods: <name>-<ordinal>                     e.g. mattermost-0, mattermost-1
	k8sDeploymentPod := regexp.MustCompile(`^[a-z0-9-]+-[a-z0-9]{5,10}-[a-z0-9]{5}$`)
	k8sStatefulPod := regexp.MustCompile(`^[a-z0-9][a-z0-9-]*-\d+$`)
	for _, id := range info.NodeIDs {
		if k8sDeploymentPod.MatchString(id) || k8sStatefulPod.MatchString(id) {
			info.DeploymentPlatform = "kubernetes"
			break
		}
	}
	// Also check SiteURL for k8s ingress hostnames
	if info.DeploymentPlatform == "" && containsAny(siteURL, []string{".cluster.local", ".svc.", "kubernetes"}) {
		info.DeploymentPlatform = "kubernetes"
	}

	return info
}

// ParseIntegrations extracts integration configuration signals.
func ParseIntegrations(np *ingestion.NormalizedPackage) models.IntegrationInfo {
	info := models.IntegrationInfo{}

	// Webhook settings
	if v := getBool(np.Config, "ServiceSettings", "EnableIncomingWebhooks"); v {
		info.HasIncomingWebhooks = true
	}
	if v := getBool(np.Config, "ServiceSettings", "EnableOutgoingWebhooks"); v {
		info.HasOutgoingWebhooks = true
	}
	if v := getBool(np.Config, "ServiceSettings", "EnableCommands"); v {
		info.HasSlashCommands = true
	}
	if v := getBool(np.Config, "ServiceSettings", "EnableBotAccountCreation"); v {
		info.HasBotAccounts = true
	}
	if v := getBool(np.Config, "ServiceSettings", "EnableOAuthServiceProvider"); v {
		info.HasOAuthApps = true
	}

	// SMTP
	emailSettings := getNestedMap(np.Config, "EmailSettings")
	if emailSettings != nil {
		if host := getNestedString(emailSettings, "SMTPServer"); host != "" && host != "localhost" {
			info.HasSMTP = true
		}
	}

	// Calls plugin
	if callsSettings := getNestedMap(np.Config, "PluginSettings", "Plugins", "com.mattermost.calls"); callsSettings != nil {
		info.HasCalls = true
	}

	// Push notifications
	pushSettings := getNestedMap(np.Config, "EmailSettings")
	if pushSettings != nil {
		if server := getNestedString(pushSettings, "PushNotificationServer"); server != "" {
			info.HasPush = true
			if containsAny(server, []string{"mattermost.com", "push-test.mattermost.com"}) {
				info.PushProxy = "mattermost-hosted"
			} else {
				info.PushProxy = "custom"
			}
		}
	}

	return info
}

// ParseObservability extracts observability configuration signals.
func ParseObservability(np *ingestion.NormalizedPackage) models.ObservabilityInfo {
	info := models.ObservabilityInfo{}

	if v := getBool(np.Config, "MetricsSettings", "Enable"); v {
		info.MetricsEnabled = true
		info.HasPrometheus = true
	}
	if path := getNestedString(np.Config, "MetricsSettings", "RouterPath"); path != "" {
		info.MetricsPath = path
	}

	if v := getBool(np.Config, "ServiceSettings", "EnablePerformanceMonitoring"); v {
		info.HasPerformanceMon = true
	}

	return info
}

// Helper: getBool safely reads a boolean from nested map keys.
func getBool(m map[string]interface{}, keys ...string) bool {
	val := getNestedString(m, keys...)
	return val == "true" || val == "1"
}

func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
