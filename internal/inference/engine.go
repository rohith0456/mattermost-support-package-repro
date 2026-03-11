// Package inference determines the repro topology from parsed support package data.
package inference

import (
	"fmt"
	"strings"
	"time"

	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/version"
)

// Engine runs the inference rules and produces a ReproPlan.
type Engine struct {
	flags models.ReproFlags
}

// NewEngine creates an inference Engine with the given flags.
func NewEngine(flags models.ReproFlags) *Engine {
	return &Engine{flags: flags}
}

// Infer builds a ReproPlan from a parsed SupportPackage.
func (e *Engine) Infer(sp *models.SupportPackage, outputDir string) *models.ReproPlan {
	plan := &models.ReproPlan{
		GeneratedAt:   time.Now(),
		ToolVersion:   version.Short(),
		SourcePackage: sp.SourcePath,
		OutputDir:     outputDir,
		Flags:         e.flags,
	}

	e.inferVersion(plan, sp)
	e.inferTopology(plan, sp)
	e.inferOutputFormat(plan, sp)
	e.inferDatabase(plan, sp)
	e.inferSearch(plan, sp)
	e.inferAuth(plan, sp)
	e.inferFileStorage(plan, sp)
	e.inferEmail(plan, sp)
	e.inferCalls(plan, sp)
	e.inferProxy(plan, sp)
	e.inferObservability(plan, sp)
	e.inferTunnel(plan, sp)
	e.inferPlugins(plan, sp)

	return plan
}

func (e *Engine) inferVersion(plan *models.ReproPlan, sp *models.SupportPackage) {
	tag := sp.Version.DockerImageTag
	if tag == "" || tag == "unknown" {
		tag = "latest"
		plan.Approximations = append(plan.Approximations, models.Approximation{
			Component:   "mattermost-version",
			Description: "Could not determine exact version; using 'latest' image tag",
			Reason:      "Version not found in support package",
		})
	} else if !sp.Version.ImageTagExact {
		plan.Approximations = append(plan.Approximations, models.Approximation{
			Component:   "mattermost-version",
			Description: fmt.Sprintf("Using image tag %s (may not be exact)", tag),
			Reason:      "Exact version tag availability not verified against Docker Hub",
		})
	}

	// Choose Docker image based on edition
	image := "mattermost/mattermost-team-edition"
	if sp.Version.Edition == "enterprise" || sp.Version.Edition == "cloud" {
		image = "mattermost/mattermost-enterprise-edition"
	}

	plan.MattermostImage = image
	plan.Services.Mattermost = models.MattermostServicePlan{
		Image:       image,
		Tag:         tag,
		ExposedPort: 8065,
	}
}

func (e *Engine) inferTopology(plan *models.ReproPlan, sp *models.SupportPackage) {
	if e.flags.ForceSingleNode {
		plan.Topology = "single-node"
		plan.NodeCount = 1
		if sp.Topology.IsCluster {
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "topology",
				Description: "Forced single-node despite cluster detected in support package",
				Reason:      "User passed --force-single-node flag",
			})
		}
		plan.Services.Mattermost.Replicas = 1
		return
	}

	// Require actual NodeCount > 1 from cluster_info before going multi-node.
	// IsCluster=true alone (from ClusterSettings.Enable in config) is insufficient —
	// the support package may have been generated from a single active node with
	// cluster mode turned on in config but no peers running.
	clusterConfirmed := sp.Topology.IsCluster && sp.Topology.NodeCount > 1
	if e.flags.ForceMultiNode || (clusterConfirmed && !e.flags.ForceSingleNode) {
		plan.Topology = "multi-node"
		nodeCount := sp.Topology.NodeCount
		if nodeCount < 2 {
			nodeCount = 2
		}
		if nodeCount > 3 {
			nodeCount = 3 // cap at 3 for local repro
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "topology",
				Description: fmt.Sprintf("Capped cluster at 3 nodes (original had %d)", sp.Topology.NodeCount),
				Reason:      "Resource limits for local Docker environment",
			})
		}
		plan.NodeCount = nodeCount
		plan.Services.Mattermost.Replicas = nodeCount
	} else {
		plan.Topology = "single-node"
		plan.NodeCount = 1
		plan.Services.Mattermost.Replicas = 1
	}
}

func (e *Engine) inferOutputFormat(plan *models.ReproPlan, sp *models.SupportPackage) {
	// --force-docker-compose always wins
	if e.flags.ForceDockerCompose {
		plan.OutputFormat = "docker-compose"
		return
	}
	// Explicit flag or auto-detected k8s platform
	if e.flags.WithKubernetes || sp.Topology.DeploymentPlatform == "kubernetes" {
		plan.OutputFormat = "kubernetes"
		if sp.Topology.DeploymentPlatform == "kubernetes" && !e.flags.WithKubernetes {
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "output-format",
				Description: "Kubernetes deployment detected — generating Kubernetes manifests for local kind cluster",
				Reason:      "Node IDs or SiteURL match Kubernetes patterns; use --force-docker-compose to override",
			})
		}
		return
	}
	plan.OutputFormat = "docker-compose"
}

func (e *Engine) inferDatabase(plan *models.ReproPlan, sp *models.SupportPackage) {
	dbType := sp.Database.Type
	if e.flags.ForceDB != "" {
		dbType = e.flags.ForceDB
	}

	switch dbType {
	case "mysql":
		plan.Services.Database = models.DatabaseServicePlan{
			Enabled:     true,
			Type:        "mysql",
			Image:       "mysql",
			Tag:         "8.0",
			ExposedPort: 3306,
			HasReplica:  sp.Database.HasReplica,
		}
	default:
		// Default to PostgreSQL for local repro
		if dbType == "unknown" {
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "database",
				Description: "Database type unknown; defaulting to PostgreSQL",
				Reason:      "Could not determine original DB type from support package",
			})
		}
		plan.Services.Database = models.DatabaseServicePlan{
			Enabled:     true,
			Type:        "postgres",
			Image:       "postgres",
			Tag:         "15-alpine",
			ExposedPort: 5432,
			HasReplica:  sp.Database.HasReplica,
		}
	}

	if sp.Database.HasReplica {
		plan.Approximations = append(plan.Approximations, models.Approximation{
			Component:   "database-replica",
			Description: "Read replica simulated with a second DB container (not a true streaming replica)",
			Reason:      "True PostgreSQL streaming replication requires complex setup not suitable for local repro",
		})
	}
}

func (e *Engine) inferSearch(plan *models.ReproPlan, sp *models.SupportPackage) {
	if e.flags.WithOpenSearch || sp.Search.Backend == "elasticsearch" || sp.Search.Backend == "opensearch" {
		plan.Services.Search = models.SearchServicePlan{
			Enabled:     true,
			Backend:     "opensearch",
			Image:       "opensearchproject/opensearch",
			Tag:         "2.11.0",
			ExposedPort: 9200,
		}
		if sp.Search.Backend == "elasticsearch" {
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "search",
				Description: "Using OpenSearch locally to approximate Elasticsearch",
				Reason:      "OpenSearch is a compatible open-source alternative suitable for local repro",
			})
		}
	} else {
		plan.Services.Search = models.SearchServicePlan{
			Enabled: false,
			Backend: "database",
		}
		if sp.Search.Backend != "database" && sp.Search.Backend != "" && sp.Search.Backend != "unknown" {
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "search",
				Description: fmt.Sprintf("Search backend '%s' not enabled; using database search", sp.Search.Backend),
				Reason:      "Use --with-opensearch flag to enable local OpenSearch container",
			})
		}
	}
}

func (e *Engine) inferAuth(plan *models.ReproPlan, sp *models.SupportPackage) {
	authPlan := models.AuthServicePlan{}

	if e.flags.WithLDAP || sp.Auth.HasLDAP {
		authPlan.LDAPEnabled = true
		authPlan.LDAPImage = "osixia/openldap:1.5.0"
		if !e.flags.WithLDAP {
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "ldap",
				Description: "Local OpenLDAP container configured with stub users and safe test credentials",
				Reason:      "Customer LDAP server is not accessible locally; safe stub used instead",
			})
		}
		plan.Stubbed = append(plan.Stubbed, models.StubbedItem{
			Component: "ldap-users",
			Reason:    "Real customer directory users cannot be replicated; test users generated instead",
			StubType:  "local-mock",
		})
	}

	if e.flags.WithSAML || e.flags.WithAzureAD || sp.Auth.HasSAML || sp.Auth.HasOIDC {
		authPlan.KeycloakEnabled = true
		authPlan.KeycloakImage = "quay.io/keycloak/keycloak:23.0"
		authPlan.SAMLEnabled = true
		plan.Approximations = append(plan.Approximations, models.Approximation{
			Component:   "saml",
			Description: fmt.Sprintf("Local Keycloak used to approximate SAML/OIDC provider '%s'", sp.Auth.SAMLProvider),
			Reason:      "Customer IdP not accessible; Keycloak provides compatible local SAML/OIDC IdP",
		})
		plan.Stubbed = append(plan.Stubbed, models.StubbedItem{
			Component: "saml-certificates",
			Reason:    "Customer SAML certificates cannot be reused; Keycloak generates self-signed certs",
			StubType:  "placeholder",
		})
		if sp.Auth.SAMLProvider == "azure-ad" || e.flags.WithAzureAD {
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "azure-ad",
				Description: "Local Keycloak simulates Azure AD — OIDC works without license, SAML requires Enterprise",
				Reason:      "Azure AD (Entra ID) not accessible locally; Keycloak used as local identity provider",
			})
		}
	}

	// Guest accounts: detected from support package OR always enabled for enterprise edition
	if sp.Auth.GuestAccounts || strings.Contains(strings.ToLower(plan.MattermostImage), "enterprise") {
		authPlan.GuestAccountsEnabled = true
	}

	plan.LicenseProvided = e.flags.LicenseFile != ""
	plan.Services.Auth = authPlan
}

func (e *Engine) inferFileStorage(plan *models.ReproPlan, sp *models.SupportPackage) {
	// Multi-node HA requires shared file storage — auto-enable MinIO even if the
	// original used local storage, since local storage cannot be shared across nodes.
	needsSharedStorage := plan.Topology == "multi-node" && plan.NodeCount > 1
	if needsSharedStorage && sp.FileStorage.Backend == "local" {
		plan.Approximations = append(plan.Approximations, models.Approximation{
			Component:   "file-storage",
			Description: "MinIO auto-enabled for multi-node: shared file storage is required for HA (original used local storage)",
			Reason:      "Local filesystem storage cannot be shared across multiple Mattermost nodes",
		})
	}
	if e.flags.WithMinIO || sp.FileStorage.Backend == "s3" || sp.FileStorage.Backend == "azure" || needsSharedStorage {
		plan.Services.FileStorage = models.FileStorageServicePlan{
			UseMinIO:    true,
			Image:       "minio/minio",
			Tag:         "latest",
			ExposedPort: 9000,
		}
		if sp.FileStorage.Backend != "local" {
			plan.Approximations = append(plan.Approximations, models.Approximation{
				Component:   "file-storage",
				Description: fmt.Sprintf("Local MinIO used to approximate %s storage", sp.FileStorage.Backend),
				Reason:      "Customer cloud storage not accessible; MinIO provides compatible S3 API locally",
			})
		}
		plan.Stubbed = append(plan.Stubbed, models.StubbedItem{
			Component: "cloud-storage-credentials",
			Reason:    "Customer cloud storage credentials are never reused; fresh MinIO credentials generated",
			StubType:  "local-mock",
		})
	} else {
		plan.Services.FileStorage = models.FileStorageServicePlan{
			UseMinIO: false,
		}
	}
}

func (e *Engine) inferEmail(plan *models.ReproPlan, sp *models.SupportPackage) {
	plan.Services.Email = models.EmailServicePlan{
		UseMailHog:  true,
		Image:       "axllent/mailpit",
		ExposedPort: 1025,
		UIPort:      8025,
	}
	if sp.Integrations.HasSMTP {
		plan.Stubbed = append(plan.Stubbed, models.StubbedItem{
			Component: "smtp",
			Reason:    "Customer SMTP server replaced with local Mailpit to prevent accidental email sending",
			StubType:  "local-mock",
		})
	}
}

func (e *Engine) inferCalls(plan *models.ReproPlan, sp *models.SupportPackage) {
	if e.flags.WithRTCD || sp.Integrations.HasRTCD {
		plan.Services.Calls = models.CallsServicePlan{
			UseRTCD:     true,
			Image:       "mattermost/rtcd",
			ExposedPort: 8045,
		}
		plan.Approximations = append(plan.Approximations, models.Approximation{
			Component:   "calls-rtcd",
			Description: "Local RTCD container configured for local-only calls testing",
			Reason:      "Customer RTCD server not accessible; local container approximates the service",
		})
	}
}

func (e *Engine) inferProxy(plan *models.ReproPlan, sp *models.SupportPackage) {
	if plan.Topology == "multi-node" || sp.Topology.HasReverseProxy {
		plan.Services.Proxy = models.ProxyServicePlan{
			Enabled:     true,
			Type:        "nginx",
			ExposedPort: 80,
		}
	}
}

func (e *Engine) inferObservability(plan *models.ReproPlan, sp *models.SupportPackage) {
	obsPlan := models.ObservabilityServicePlan{}

	if e.flags.WithGrafana || sp.Observability.MetricsEnabled {
		obsPlan.PrometheusEnabled = true
		obsPlan.PrometheusPort = 9090
		obsPlan.GrafanaEnabled = true
		obsPlan.GrafanaPort = 3000
	}

	plan.Services.Observability = obsPlan
}

func (e *Engine) inferPlugins(plan *models.ReproPlan, sp *models.SupportPackage) {
	for _, plugin := range sp.Plugins {
		repro := models.PluginRepro{
			ID:      plugin.ID,
			Name:    plugin.Name,
			Version: plugin.Version,
		}

		if plugin.IsBuiltin {
			repro.Action = "skip"
			repro.Reason = "Built-in plugin, no separate installation needed"
		} else if plugin.AutoInstall {
			repro.Action = "install"
			repro.Reason = "Available on official Mattermost marketplace"
		} else {
			repro.Action = "manual"
			repro.Reason = "Custom or proprietary plugin; manual installation required"
			plan.Unsupported = append(plan.Unsupported, models.UnsupportedItem{
				Component:  fmt.Sprintf("plugin:%s", plugin.ID),
				Description: "Custom plugin not available for automatic installation",
				Workaround: "Download plugin manually and place in the plugins directory",
			})
		}

		plan.Plugins = append(plan.Plugins, repro)
	}
}

func (e *Engine) inferTunnel(plan *models.ReproPlan, _ *models.SupportPackage) {
	if !e.flags.WithNgrok {
		return
	}
	plan.Services.Tunnel = models.TunnelServicePlan{
		NgrokEnabled: true,
		NgrokAPIPort: 4040,
	}
}
