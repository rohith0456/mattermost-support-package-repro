package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rohith0456/mattermost-support-package-repro/internal/inference"
	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/internal/parser"
	"github.com/rohith0456/mattermost-support-package-repro/internal/redaction"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

var (
	planSupportPackage string
	planOutputJSON     bool
	planForceDB        string
	planForceSingle    bool
	planForceMulti     bool
	planWithOpenSearch bool
	planWithLDAP       bool
	planWithSAML       bool
	planWithMinIO      bool
	planWithRTCD       bool
	planWithGrafana    bool
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Print the inferred repro plan without generating files",
	Long: `Parse a support package and display the inferred repro plan.
No files are generated unless --output is specified.

Example:
  mm-repro plan --support-package ./customer.zip
  mm-repro plan --support-package ./customer.zip --json`,
	RunE: runPlan,
}

func init() {
	planCmd.Flags().StringVar(&planSupportPackage, "support-package", "", "Path to the support package ZIP (required)")
	planCmd.Flags().BoolVar(&planOutputJSON, "json", false, "Output plan as JSON")
	planCmd.Flags().StringVar(&planForceDB, "db", "", "Force database type: postgres|mysql")
	planCmd.Flags().BoolVar(&planForceSingle, "force-single-node", false, "Force single-node topology")
	planCmd.Flags().BoolVar(&planForceMulti, "force-multi-node", false, "Force multi-node topology")
	planCmd.Flags().BoolVar(&planWithOpenSearch, "with-opensearch", false, "Include OpenSearch in plan")
	planCmd.Flags().BoolVar(&planWithLDAP, "with-ldap", false, "Include LDAP in plan")
	planCmd.Flags().BoolVar(&planWithSAML, "with-saml", false, "Include Keycloak/SAML in plan")
	planCmd.Flags().BoolVar(&planWithMinIO, "with-minio", false, "Include MinIO in plan")
	planCmd.Flags().BoolVar(&planWithRTCD, "with-rtcd", false, "Include RTCD in plan")
	planCmd.Flags().BoolVar(&planWithGrafana, "with-grafana", false, "Include Grafana in plan")

	_ = planCmd.MarkFlagRequired("support-package")
}

func runPlan(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(planSupportPackage); os.IsNotExist(err) {
		return fmt.Errorf("support package not found: %s", planSupportPackage)
	}

	tmpDir := filepath.Join(os.TempDir(), "mm-repro-plan-work")
	ingestor := ingestion.NewIngestor(tmpDir)
	pkg, err := ingestor.Ingest(planSupportPackage)
	if err != nil {
		return fmt.Errorf("ingesting: %w", err)
	}
	defer pkg.Cleanup()

	normalizer := ingestion.NewNormalizer()
	normalized := normalizer.Normalize(pkg)

	redactor := redaction.NewRedactor(false)
	_ = redactor.RedactConfig(normalized.Config, planSupportPackage, "config.json")

	p := parser.NewParser()
	sp := p.Parse(normalized, planSupportPackage)

	flags := models.ReproFlags{
		ForceDB:        planForceDB,
		ForceSingleNode: planForceSingle,
		ForceMultiNode:  planForceMulti,
		WithOpenSearch: planWithOpenSearch,
		WithLDAP:       planWithLDAP,
		WithSAML:       planWithSAML,
		WithMinIO:      planWithMinIO,
		WithRTCD:       planWithRTCD,
		WithGrafana:    planWithGrafana,
	}
	engine := inference.NewEngine(flags)
	plan := engine.Infer(sp, "./generated-repro/preview")

	if planOutputJSON {
		data, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Human-readable plan output
	fmt.Println("")
	fmt.Println("=== Repro Plan ===")
	fmt.Println("")
	fmt.Printf("Mattermost: %s:%s\n", plan.MattermostImage, plan.Services.Mattermost.Tag)
	fmt.Printf("Topology:   %s (%d node(s))\n", plan.Topology, plan.NodeCount)
	fmt.Printf("Database:   %s\n", plan.Services.Database.Type)

	fmt.Println("\nServices:")
	printServiceStatus("Mattermost", true, fmt.Sprintf("port %d", plan.Services.Mattermost.ExposedPort))
	printServiceStatus("Database", plan.Services.Database.Enabled, plan.Services.Database.Type)
	printServiceStatus("OpenSearch", plan.Services.Search.Enabled, plan.Services.Search.Backend)
	printServiceStatus("LDAP", plan.Services.Auth.LDAPEnabled, "OpenLDAP stub")
	printServiceStatus("SAML/OIDC", plan.Services.Auth.KeycloakEnabled, "Keycloak stub")
	printServiceStatus("Object Storage", plan.Services.FileStorage.UseMinIO, "MinIO")
	printServiceStatus("Email", plan.Services.Email.UseMailHog, "MailHog")
	printServiceStatus("Calls/RTCD", plan.Services.Calls.UseRTCD, "local RTCD")
	printServiceStatus("Metrics", plan.Services.Observability.PrometheusEnabled, "Prometheus+Grafana")

	if len(plan.Approximations) > 0 {
		fmt.Printf("\nApproximations (%d):\n", len(plan.Approximations))
		for _, a := range plan.Approximations {
			fmt.Printf("  ~ %s: %s\n", a.Component, a.Description)
		}
	}

	if len(plan.Unsupported) > 0 {
		fmt.Printf("\nUnsupported (%d):\n", len(plan.Unsupported))
		for _, u := range plan.Unsupported {
			fmt.Printf("  ✗ %s: %s\n", u.Component, u.Description)
		}
	}

	fmt.Println()
	return nil
}

func printServiceStatus(name string, enabled bool, detail string) {
	if enabled {
		fmt.Printf("  ✓ %-20s %s\n", name, detail)
	} else {
		fmt.Printf("  - %-20s (disabled)\n", name)
	}
}
