package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/rohith0456/mattermost-support-package-repro/internal/generator"
	"github.com/rohith0456/mattermost-support-package-repro/internal/inference"
	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/internal/parser"
	"github.com/rohith0456/mattermost-support-package-repro/internal/redaction"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

var (
	initSupportPackage    string
	initOutputDir         string
	initForceDB           string
	initForceSingle       bool
	initForceMulti        bool
	initWithOpenSearch    bool
	initWithLDAP          bool
	initWithSAML          bool
	initWithMinIO         bool
	initWithRTCD          bool
	initWithGrafana       bool
	initRedactStrict      bool
	initIssueName         string
	initWithKubernetes    bool
	initForceDockerCompose bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Parse a support package and generate a local repro project",
	Long: `Parse a Mattermost support package and generate a ready-to-run repro
environment (Docker Compose or Kubernetes) in a new directory.

Example:
  mm-repro init --support-package ./customer-support-package.zip
  mm-repro init --support-package ./sp.zip --with-ldap --with-opensearch
  mm-repro init --support-package ./sp.zip --with-kubernetes
  mm-repro init --support-package ./sp.zip --output ./my-repros/issue-1234`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initSupportPackage, "support-package", "", "Path to the support package ZIP (required)")
	initCmd.Flags().StringVar(&initOutputDir, "output", "", "Output directory (default: ./generated-repro/<timestamp>)")
	initCmd.Flags().StringVar(&initIssueName, "issue", "", "Issue or ticket name for output directory naming")
	initCmd.Flags().StringVar(&initForceDB, "db", "", "Force database type: postgres|mysql")
	initCmd.Flags().BoolVar(&initForceSingle, "force-single-node", false, "Force single-node topology")
	initCmd.Flags().BoolVar(&initForceMulti, "force-multi-node", false, "Force multi-node topology")
	initCmd.Flags().BoolVar(&initWithOpenSearch, "with-opensearch", false, "Include OpenSearch service")
	initCmd.Flags().BoolVar(&initWithLDAP, "with-ldap", false, "Include local OpenLDAP service")
	initCmd.Flags().BoolVar(&initWithSAML, "with-saml", false, "Include local Keycloak (SAML/OIDC) service")
	initCmd.Flags().BoolVar(&initWithMinIO, "with-minio", false, "Include local MinIO (S3-compatible storage)")
	initCmd.Flags().BoolVar(&initWithRTCD, "with-rtcd", false, "Include local RTCD (Calls) service")
	initCmd.Flags().BoolVar(&initWithGrafana, "with-grafana", false, "Include Prometheus + Grafana observability stack")
	initCmd.Flags().BoolVar(&initRedactStrict, "redact-strict", false, "Apply strict redaction (also redacts server addresses and emails)")
	initCmd.Flags().BoolVar(&initWithKubernetes, "with-kubernetes", false, "Generate Kubernetes manifests (kind) instead of Docker Compose")
	initCmd.Flags().BoolVar(&initForceDockerCompose, "force-docker-compose", false, "Force Docker Compose output even when a Kubernetes deployment is detected")

	_ = initCmd.MarkFlagRequired("support-package")
}

func runInit(cmd *cobra.Command, args []string) error {
	printBanner()

	// Validate support package path
	if _, err := os.Stat(initSupportPackage); os.IsNotExist(err) {
		return fmt.Errorf("support package not found: %s", initSupportPackage)
	}

	// Determine output directory
	outputDir := initOutputDir
	if outputDir == "" {
		ts := time.Now().Format("20060102-150405")
		dirName := ts
		if initIssueName != "" {
			dirName = initIssueName + "-" + ts
		}
		outputDir = filepath.Join("generated-repro", dirName)
	}

	printInfo(fmt.Sprintf("Support package: %s", initSupportPackage))
	printInfo(fmt.Sprintf("Output directory: %s", outputDir))
	fmt.Println()

	// Step 1: Ingest
	printInfo("Step 1/5: Ingesting support package...")
	tmpDir := filepath.Join(os.TempDir(), "mm-repro-work")
	ingestor := ingestion.NewIngestor(tmpDir)
	pkg, err := ingestor.Ingest(initSupportPackage)
	if err != nil {
		return fmt.Errorf("ingesting support package: %w", err)
	}
	defer pkg.Cleanup()
	printSuccess(fmt.Sprintf("Extracted %d files from package (format: %s)", len(pkg.RawFiles), pkg.Format))

	// Step 2: Normalize
	printInfo("Step 2/5: Normalizing package contents...")
	normalizer := ingestion.NewNormalizer()
	normalized := normalizer.Normalize(pkg)
	printSuccess("Package normalized")
	if len(normalized.Warnings) > 0 {
		for _, w := range normalized.Warnings {
			printWarning(w)
		}
	}

	// Step 3: Redact
	printInfo("Step 3/5: Applying redaction rules...")
	redactor := redaction.NewRedactor(initRedactStrict)
	redactionReport := redactor.RedactConfig(normalized.Config, initSupportPackage, "config.json")
	printSuccess(fmt.Sprintf("Redacted %d sensitive values (%d high-severity)",
		redactionReport.TotalRedacted, redactionReport.HighSeverityCount))

	// Step 4: Parse
	printInfo("Step 4/5: Parsing support package signals...")
	p := parser.NewParser()
	sp := p.Parse(normalized, initSupportPackage)
	printSuccess(fmt.Sprintf("Mattermost version: %s (edition: %s)", sp.Version.Normalized, sp.Version.Edition))
	printSuccess(fmt.Sprintf("Database: %s, Topology: nodes=%d cluster=%v",
		sp.Database.Type, sp.Topology.NodeCount, sp.Topology.IsCluster))
	printSuccess(fmt.Sprintf("Plugins detected: %d", len(sp.Plugins)))

	// Step 5: Infer + Generate
	printInfo("Step 5/5: Building repro plan and generating project...")
	flags := models.ReproFlags{
		ForceDB:            initForceDB,
		ForceSingleNode:    initForceSingle,
		ForceMultiNode:     initForceMulti,
		WithOpenSearch:     initWithOpenSearch,
		WithLDAP:           initWithLDAP,
		WithSAML:           initWithSAML,
		WithMinIO:          initWithMinIO,
		WithRTCD:           initWithRTCD,
		WithGrafana:        initWithGrafana,
		RedactStrict:       initRedactStrict,
		WithKubernetes:     initWithKubernetes,
		ForceDockerCompose: initForceDockerCompose,
	}
	engine := inference.NewEngine(flags)
	plan := engine.Infer(sp, outputDir)

	gen := generator.NewGenerator(plan, outputDir, "")
	created, err := gen.Generate()
	if err != nil {
		return fmt.Errorf("generating repro project: %w", err)
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Generated %d files in: %s", len(created), outputDir))
	fmt.Println()

	// Print summary
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Printf("  Image:    %s:%s\n", plan.MattermostImage, plan.Services.Mattermost.Tag)
	fmt.Printf("  Topology: %s (%d node(s))\n", plan.Topology, plan.NodeCount)
	fmt.Printf("  Database: %s\n", plan.Services.Database.Type)
	fmt.Printf("  Output:   %s\n", plan.OutputFormat)
	if plan.Services.Search.Enabled {
		fmt.Printf("  Search:   %s\n", plan.Services.Search.Backend)
	}
	if len(plan.Approximations) > 0 {
		fmt.Printf("  Approx:   %d items approximated\n", len(plan.Approximations))
	}
	if len(plan.Unsupported) > 0 {
		fmt.Printf("  Skipped:  %d items could not be recreated\n", len(plan.Unsupported))
	}
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Println()

	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", outputDir)
	fmt.Printf("  make run\n")
	if plan.OutputFormat == "kubernetes" {
		fmt.Printf("  open http://localhost:30065\n\n")
		fmt.Printf("Requirements: kind + kubectl must be installed.\n")
		fmt.Printf("  kind:    https://kind.sigs.k8s.io/docs/user/quick-start/\n")
		fmt.Printf("  kubectl: https://kubernetes.io/docs/tasks/tools/\n\n")
	} else {
		fmt.Printf("  open http://localhost:8065\n\n")
	}
	fmt.Printf("Review the reports:\n")
	fmt.Printf("  cat %s/REPRO_SUMMARY.md\n", outputDir)
	fmt.Printf("  cat %s/REDACTION_REPORT.md\n", outputDir)

	return nil
}
