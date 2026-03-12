package inference_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rohith0456/mattermost-support-package-repro/internal/inference"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

func makeMinimalPackage(version, dbType string, isCluster bool) *models.SupportPackage {
	sp := &models.SupportPackage{
		SourcePath: "./test.zip",
	}
	sp.Version = models.VersionInfo{
		Normalized:     version,
		Raw:            version,
		DockerImageTag: version,
		ImageTagExact:  true,
		Edition:        "team",
	}
	sp.Database = models.DatabaseInfo{
		Type: dbType,
	}
	sp.Topology = models.TopologyInfo{
		NodeCount: 1,
		IsCluster: isCluster,
	}
	sp.FileStorage = models.FileStorageInfo{Backend: "local"}
	sp.Search = models.SearchInfo{Backend: "database"}
	sp.Integrations = models.IntegrationInfo{}
	return sp
}

func TestInferVersion(t *testing.T) {
	tests := []struct {
		version       string
		expectedImage string
		expectedTag   string
	}{
		{
			version:       "8.1.3",
			expectedImage: "mattermost/mattermost-team-edition",
			expectedTag:   "8.1.3",
		},
		{
			version:     "",
			expectedTag: "latest",
		},
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			sp := makeMinimalPackage(tc.version, "postgres", false)
			engine := inference.NewEngine(models.ReproFlags{})
			plan := engine.Infer(sp, "./output")
			assert.Equal(t, tc.expectedTag, plan.Services.Mattermost.Tag)
		})
	}
}

func TestInferTopology_SingleNode(t *testing.T) {
	sp := makeMinimalPackage("8.1.3", "postgres", false)
	engine := inference.NewEngine(models.ReproFlags{})
	plan := engine.Infer(sp, "./output")

	assert.Equal(t, "single-node", plan.Topology)
	assert.Equal(t, 1, plan.NodeCount)
	assert.Equal(t, 1, plan.Services.Mattermost.Replicas)
}

func TestInferTopology_ClusterDetected(t *testing.T) {
	sp := makeMinimalPackage("8.1.3", "postgres", true)
	sp.Topology.NodeCount = 4
	engine := inference.NewEngine(models.ReproFlags{})
	plan := engine.Infer(sp, "./output")

	assert.Equal(t, "multi-node", plan.Topology)
	assert.Equal(t, 3, plan.NodeCount, "should cap at 3 nodes for local repro")

	// Should have approximation note
	hasApprox := false
	for _, a := range plan.Approximations {
		if a.Component == "topology" {
			hasApprox = true
			break
		}
	}
	assert.True(t, hasApprox, "should have topology approximation for capped nodes")
}

func TestInferTopology_ForceFlags(t *testing.T) {
	t.Run("force single node from cluster", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", true)
		sp.Topology.NodeCount = 3
		engine := inference.NewEngine(models.ReproFlags{ForceSingleNode: true})
		plan := engine.Infer(sp, "./output")
		assert.Equal(t, "single-node", plan.Topology)
	})

	t.Run("force multi node", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		engine := inference.NewEngine(models.ReproFlags{ForceMultiNode: true})
		plan := engine.Infer(sp, "./output")
		assert.Equal(t, "multi-node", plan.Topology)
	})
}

func TestInferDatabase(t *testing.T) {
	t.Run("postgres detected", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		plan := inference.NewEngine(models.ReproFlags{}).Infer(sp, "./output")
		assert.Equal(t, "postgres", plan.Services.Database.Type)
	})

	t.Run("mysql detected", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "mysql", false)
		plan := inference.NewEngine(models.ReproFlags{}).Infer(sp, "./output")
		assert.Equal(t, "mysql", plan.Services.Database.Type)
	})

	t.Run("force postgres override", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "mysql", false)
		plan := inference.NewEngine(models.ReproFlags{ForceDB: "postgres"}).Infer(sp, "./output")
		assert.Equal(t, "postgres", plan.Services.Database.Type)
	})

	t.Run("unknown defaults to postgres", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "unknown", false)
		plan := inference.NewEngine(models.ReproFlags{}).Infer(sp, "./output")
		assert.Equal(t, "postgres", plan.Services.Database.Type)

		hasApprox := false
		for _, a := range plan.Approximations {
			if a.Component == "database" {
				hasApprox = true
				break
			}
		}
		assert.True(t, hasApprox, "should note the database type fallback")
	})
}

func TestInferSearch(t *testing.T) {
	t.Run("with-opensearch flag", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		plan := inference.NewEngine(models.ReproFlags{WithOpenSearch: true}).Infer(sp, "./output")
		assert.True(t, plan.Services.Search.Enabled)
		assert.Equal(t, "opensearch", plan.Services.Search.Backend)
	})

	t.Run("elasticsearch detected auto-enables elasticsearch", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		sp.Search = models.SearchInfo{Backend: "elasticsearch"}
		plan := inference.NewEngine(models.ReproFlags{}).Infer(sp, "./output")
		assert.True(t, plan.Services.Search.Enabled)
		assert.Equal(t, "elasticsearch", plan.Services.Search.Backend)
	})

	t.Run("with-elasticsearch flag", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		plan := inference.NewEngine(models.ReproFlags{WithElasticsearch: true}).Infer(sp, "./output")
		assert.True(t, plan.Services.Search.Enabled)
		assert.Equal(t, "elasticsearch", plan.Services.Search.Backend)
	})

	t.Run("no search by default", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		plan := inference.NewEngine(models.ReproFlags{}).Infer(sp, "./output")
		assert.False(t, plan.Services.Search.Enabled)
	})
}

func TestInferEmail(t *testing.T) {
	sp := makeMinimalPackage("8.1.3", "postgres", false)
	plan := inference.NewEngine(models.ReproFlags{}).Infer(sp, "./output")

	// Email (MailHog) should always be enabled
	assert.True(t, plan.Services.Email.UseMailHog)
}

func TestInferAuth(t *testing.T) {
	t.Run("LDAP flag", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		plan := inference.NewEngine(models.ReproFlags{WithLDAP: true}).Infer(sp, "./output")
		assert.True(t, plan.Services.Auth.LDAPEnabled)
	})

	t.Run("LDAP detected from package", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		sp.Auth = models.AuthInfo{HasLDAP: true}
		plan := inference.NewEngine(models.ReproFlags{}).Infer(sp, "./output")
		assert.True(t, plan.Services.Auth.LDAPEnabled)
	})

	t.Run("SAML flag enables Keycloak", func(t *testing.T) {
		sp := makeMinimalPackage("8.1.3", "postgres", false)
		plan := inference.NewEngine(models.ReproFlags{WithSAML: true}).Infer(sp, "./output")
		assert.True(t, plan.Services.Auth.KeycloakEnabled)
	})
}

func TestInferPlugins(t *testing.T) {
	sp := makeMinimalPackage("8.1.3", "postgres", false)
	sp.Plugins = []models.PluginInfo{
		{ID: "playbooks", AutoInstall: true, Source: "marketplace"},
		{ID: "com.example.custom", Source: "custom"},
		{ID: "com.mattermost.nps", IsBuiltin: true, Source: "builtin"},
	}

	plan := inference.NewEngine(models.ReproFlags{}).Infer(sp, "./output")
	require.Len(t, plan.Plugins, 3)

	for _, p := range plan.Plugins {
		switch p.ID {
		case "playbooks":
			assert.Equal(t, "install", p.Action)
		case "com.example.custom":
			assert.Equal(t, "manual", p.Action)
		case "com.mattermost.nps":
			assert.Equal(t, "skip", p.Action)
		}
	}
}
