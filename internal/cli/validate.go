package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/internal/parser"
	"github.com/rohith0456/mattermost-support-package-repro/internal/redaction"
)

var validateSupportPackage string

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a support package and report usable signals",
	Long: `Inspect a support package and report what signals are available
for repro generation, without generating any files.

Example:
  mm-repro validate --support-package ./customer.zip`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateSupportPackage, "support-package", "", "Path to the support package ZIP (required)")
	_ = validateCmd.MarkFlagRequired("support-package")
}

func runValidate(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(validateSupportPackage); os.IsNotExist(err) {
		return fmt.Errorf("support package not found: %s", validateSupportPackage)
	}

	printInfo(fmt.Sprintf("Validating: %s", validateSupportPackage))
	fmt.Println()

	tmpDir := filepath.Join(os.TempDir(), "mm-repro-validate-work")
	ingestor := ingestion.NewIngestor(tmpDir)
	pkg, err := ingestor.Ingest(validateSupportPackage)
	if err != nil {
		return fmt.Errorf("ingesting: %w", err)
	}
	defer pkg.Cleanup()

	fmt.Printf("Package format:  %s\n", pkg.Format)
	fmt.Printf("Files extracted: %d\n\n", len(pkg.RawFiles))

	// Check for key files
	type fileCheck struct {
		name     string
		paths    []string
		critical bool
	}
	checks := []fileCheck{
		{"Config JSON", []string{"config.json", "sanitized_config.json"}, true},
		{"Diagnostics", []string{"diagnostic.json", "support_packet.json", "diagnostics.yaml", "diagnostics.json", "metadata.yaml"}, false},
		{"System Info", []string{"system_info.json"}, false},
		{"Cluster Info", []string{"cluster_info.json", "cluster.json"}, false},
		{"Plugin Info", []string{"plugins.json", "plugin_statuses.json"}, false},
		{"Mattermost Log", []string{"mattermost.log"}, false},
	}

	fmt.Println("File presence:")
	for _, check := range checks {
		found := false
		foundPath := ""
		for _, path := range check.paths {
			if p := pkg.FindFile(path); p != "" {
				found = true
				foundPath = path
				break
			}
		}
		if found {
			printSuccess(fmt.Sprintf("%-20s found (%s)", check.name, foundPath))
		} else if check.critical {
			printWarning(fmt.Sprintf("%-20s NOT FOUND (repro quality will be limited)", check.name))
		} else {
			fmt.Printf("  - %-20s not found (optional)\n", check.name)
		}
	}

	// Run normalization and parse
	normalizer := ingestion.NewNormalizer()
	normalized := normalizer.Normalize(pkg)

	redactor := redaction.NewRedactor(false)
	_ = redactor.RedactConfig(normalized.Config, validateSupportPackage, "config.json")

	p := parser.NewParser()
	sp := p.Parse(normalized, validateSupportPackage)

	fmt.Printf("\nExtracted signals:\n")
	fmt.Printf("  Version:  %s (edition: %s)\n", sp.Version.Raw, sp.Version.Edition)
	fmt.Printf("  DB type:  %s\n", sp.Database.Type)
	fmt.Printf("  Topology: nodes=%d, cluster=%v\n", sp.Topology.NodeCount, sp.Topology.IsCluster)
	fmt.Printf("  Storage:  %s\n", sp.FileStorage.Backend)
	fmt.Printf("  Search:   %s\n", sp.Search.Backend)
	fmt.Printf("  Auth:     LDAP=%v, SAML=%v, OIDC=%v\n",
		sp.Auth.HasLDAP, sp.Auth.HasSAML, sp.Auth.HasOIDC)
	fmt.Printf("  Plugins:  %d detected\n", len(sp.Plugins))
	fmt.Printf("  Calls:    %v\n", sp.Integrations.HasCalls)
	fmt.Printf("  Metrics:  %v\n", sp.Observability.MetricsEnabled)

	if len(sp.ParseWarnings) > 0 {
		fmt.Printf("\nWarnings:\n")
		for _, w := range sp.ParseWarnings {
			printWarning(w)
		}
	}

	fmt.Println()
	return nil
}
