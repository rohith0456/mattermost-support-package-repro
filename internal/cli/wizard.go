package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

// wizard runs an interactive setup prompt and returns a synthetic SupportPackage
// and ReproFlags that the normal inference + generator pipeline can consume — no
// real support package ZIP required.
type wizard struct {
	reader *bufio.Reader
}

func newWizard() *wizard {
	return &wizard{reader: bufio.NewReader(os.Stdin)}
}

// Run prompts the user for all relevant choices and returns a synthetic
// SupportPackage and ReproFlags that drive the inference + generator pipeline.
func (w *wizard) Run() (*models.SupportPackage, models.ReproFlags, error) {
	cyan := "\033[36m"
	bold := "\033[1m"
	reset := "\033[0m"
	dim := "\033[2m"

	fmt.Printf("\n%s%s🛠  Mattermost Environment Setup Wizard%s\n", bold, cyan, reset)
	fmt.Printf("%sAnswer a few questions and mm-repro will spin up a full local environment.%s\n\n", dim, reset)

	sp := &models.SupportPackage{
		SourcePath:  "wizard",
		ExtractedAt: time.Now(),
		PackageFormat: "wizard",
	}
	var flags models.ReproFlags

	// ── Section 1: Mattermost Version ──────────────────────────────────────

	w.sectionHeader("Mattermost Version")
	versionRaw := w.ask("Version (e.g. 10.5.0, 9.11.0 — or press Enter for latest)", "latest")
	if versionRaw == "latest" || versionRaw == "" {
		sp.Version = models.VersionInfo{
			Raw:           "latest",
			Normalized:    "unknown",
			DockerImageTag: "latest",
			ImageTagExact: false,
			Edition:       "team",
		}
	} else {
		norm, major, minor, patch := wizardParseSemver(versionRaw)
		sp.Version = models.VersionInfo{
			Raw:           versionRaw,
			Normalized:    norm,
			Major:         major,
			Minor:         minor,
			Patch:         patch,
			DockerImageTag: norm,
			ImageTagExact: true,
			Edition:       "team",
		}
	}

	editionIdx := w.choice("Edition", []string{
		"Team (free, open source)",
		"Enterprise (licensed features)",
	}, 0)
	if editionIdx == 1 {
		sp.Version.Edition = "enterprise"
	}

	// ── Section 2: Database ─────────────────────────────────────────────────

	w.sectionHeader("Database")
	dbIdx := w.choice("Database type", []string{
		"PostgreSQL 15 (recommended)",
		"MySQL 8.0",
	}, 0)
	if dbIdx == 1 {
		sp.Database = models.DatabaseInfo{Type: "mysql"}
	} else {
		sp.Database = models.DatabaseInfo{Type: "postgres"}
	}

	// ── Section 3: Topology ─────────────────────────────────────────────────

	w.sectionHeader("Topology")
	topoIdx := w.choice("Deployment topology", []string{
		"Single-node  (1 Mattermost container — fastest, simplest)",
		"Multi-node HA (2 nodes behind nginx load balancer)",
		"Multi-node HA (3 nodes behind nginx load balancer)",
	}, 0)
	switch topoIdx {
	case 0:
		sp.Topology = models.TopologyInfo{IsCluster: false, NodeCount: 1}
	case 1:
		sp.Topology = models.TopologyInfo{IsCluster: true, NodeCount: 2, HasReverseProxy: true}
	case 2:
		sp.Topology = models.TopologyInfo{IsCluster: true, NodeCount: 3, HasReverseProxy: true}
	}

	// Multi-node requires MinIO — note it
	if sp.Topology.NodeCount > 1 {
		fmt.Printf("  %s→ MinIO will be auto-enabled (shared storage required for HA)%s\n", dim, reset)
	}

	// ── Section 4: Output Format ────────────────────────────────────────────

	w.sectionHeader("Output Format")
	fmtIdx := w.choice("How to run it", []string{
		"Docker Compose  (docker compose up — needs Docker Desktop)",
		"Kubernetes (kind)  (kubectl apply — needs kind + kubectl)",
	}, 0)
	if fmtIdx == 1 {
		flags.WithKubernetes = true
	} else {
		flags.ForceDockerCompose = true
	}

	// ── Section 5: Optional Services ───────────────────────────────────────

	w.sectionHeader("Optional Services")
	fmt.Printf("  %sEnable only what you need — each adds a container:%s\n\n", dim, reset)

	if w.yn("OpenSearch  (advanced full-text search)", false) {
		sp.Search = models.SearchInfo{Backend: "opensearch"}
		flags.WithOpenSearch = true
	} else {
		sp.Search = models.SearchInfo{Backend: "database"}
	}

	if w.yn("LDAP authentication  (local OpenLDAP with stub users)", false) {
		sp.Auth.HasLDAP = true
		flags.WithLDAP = true
	}

	if w.yn("SAML / OIDC authentication  (local Keycloak IdP)", false) {
		sp.Auth.HasSAML = true
		flags.WithSAML = true
	}

	if w.yn("MinIO  (local S3-compatible file storage)", sp.Topology.NodeCount > 1) {
		sp.FileStorage = models.FileStorageInfo{Backend: "s3"}
		flags.WithMinIO = true
	} else if sp.Topology.NodeCount == 1 {
		sp.FileStorage = models.FileStorageInfo{Backend: "local"}
	}

	if w.yn("Prometheus + Grafana  (metrics and dashboards)", false) {
		sp.Observability = models.ObservabilityInfo{MetricsEnabled: true}
		flags.WithGrafana = true
	}

	if w.yn("Calls / RTCD  (local video/voice calls container)", false) {
		sp.Integrations.HasRTCD = true
		flags.WithRTCD = true
	}

	if fmtIdx == 0 { // ngrok only makes sense for Compose
		if w.yn("ngrok tunnel  (public HTTPS URL for phone/remote testing)", false) {
			flags.WithNgrok = true
		}
	}

	if w.yn("Private / airgapped registry  (prefix all images with a custom registry URL)", false) {
		flags.ImageRegistry = w.ask("Registry URL (e.g. registry.internal:5000)", "")
	}

	// ── Confirmation ─────────────────────────────────────────────────────────

	w.sectionHeader("Summary")
	w.printSummary(sp, flags)

	fmt.Println()
	ok := w.yn("Generate this environment?", true)
	if !ok {
		return nil, flags, fmt.Errorf("setup cancelled — run 'mm-repro init' again to start over")
	}

	return sp, flags, nil
}

// ─── prompt helpers ───────────────────────────────────────────────────────────

func (w *wizard) sectionHeader(title string) {
	bold := "\033[1m"
	reset := "\033[0m"
	fmt.Printf("\n%s── %s %s\n", bold, title, reset)
}

func (w *wizard) ask(prompt, defaultVal string) string {
	dim := "\033[2m"
	reset := "\033[0m"
	if defaultVal != "" {
		fmt.Printf("  %s %s[%s]%s: ", prompt, dim, defaultVal, reset)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	line, _ := w.reader.ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return defaultVal
	}
	return strings.TrimSpace(line)
}

func (w *wizard) choice(prompt string, options []string, defaultIdx int) int {
	dim := "\033[2m"
	cyan := "\033[36m"
	reset := "\033[0m"
	fmt.Printf("\n  %s:\n", prompt)
	for i, opt := range options {
		marker := fmt.Sprintf("    %s[%d]%s", dim, i+1, reset)
		if i == defaultIdx {
			marker = fmt.Sprintf("  %s▶ [%d]%s", cyan, i+1, reset)
		}
		fmt.Printf("%s %s\n", marker, opt)
	}
	for {
		fmt.Printf("  Choice [%s%d%s]: ", dim, defaultIdx+1, reset)
		line, _ := w.reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" {
			return defaultIdx
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && n >= 1 && n <= len(options) {
			return n - 1
		}
		fmt.Printf("  Please enter a number between 1 and %d.\n", len(options))
	}
}

func (w *wizard) yn(prompt string, defaultYes bool) bool {
	dim := "\033[2m"
	reset := "\033[0m"
	def := "y/N"
	if defaultYes {
		def = "Y/n"
	}
	fmt.Printf("  %s? %s[%s]%s: ", prompt, dim, def, reset)
	line, _ := w.reader.ReadString('\n')
	line = strings.ToLower(strings.TrimRight(line, "\r\n"))
	if strings.TrimSpace(line) == "" {
		return defaultYes
	}
	return line == "y" || line == "yes"
}

// ─── summary ─────────────────────────────────────────────────────────────────

func (w *wizard) printSummary(sp *models.SupportPackage, flags models.ReproFlags) {
	green := "\033[32m"
	dim := "\033[2m"
	reset := "\033[0m"
	tick := green + "✓" + reset

	versionStr := sp.Version.Normalized
	if versionStr == "unknown" {
		versionStr = "latest"
	}
	fmt.Printf("  %s Mattermost:  %s:%s  (%s edition)\n", tick, "mattermost/mattermost-team-edition",
		versionStr, sp.Version.Edition)
	fmt.Printf("  %s Database:    %s\n", tick, sp.Database.Type)

	topoStr := "single-node"
	if sp.Topology.NodeCount > 1 {
		topoStr = fmt.Sprintf("multi-node HA (%d nodes + nginx)", sp.Topology.NodeCount)
	}
	fmt.Printf("  %s Topology:    %s\n", tick, topoStr)

	fmtStr := "Docker Compose"
	if flags.WithKubernetes {
		fmtStr = "Kubernetes (kind)"
	}
	fmt.Printf("  %s Output:      %s\n", tick, fmtStr)

	var extras []string
	if flags.WithOpenSearch {
		extras = append(extras, "OpenSearch")
	}
	if flags.WithLDAP {
		extras = append(extras, "OpenLDAP")
	}
	if flags.WithSAML {
		extras = append(extras, "Keycloak")
	}
	if flags.WithMinIO || sp.Topology.NodeCount > 1 {
		extras = append(extras, "MinIO")
	}
	if flags.WithGrafana {
		extras = append(extras, "Prometheus+Grafana")
	}
	if flags.WithRTCD {
		extras = append(extras, "RTCD/Calls")
	}
	if flags.WithNgrok {
		extras = append(extras, "ngrok tunnel")
	}
	if len(extras) > 0 {
		fmt.Printf("  %s Extras:      %s\n", tick, strings.Join(extras, ", "))
	} else {
		fmt.Printf("  %s Extras:      %snone (bare minimum)%s\n", tick, dim, reset)
	}
	fmt.Printf("  %s Mailpit:     always included — captures all outgoing emails\n", tick)
	if flags.ImageRegistry != "" {
		fmt.Printf("  %s Registry:   %s  (airgapped mode — all images prefixed)\n", tick, flags.ImageRegistry)
	}
}

// ─── version parsing ──────────────────────────────────────────────────────────

func wizardParseSemver(raw string) (normalized string, major, minor, patch int) {
	// Strip any leading "v"
	s := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) == 3 {
		major, _ = strconv.Atoi(parts[0])
		// Patch may have a suffix like "-rc1" — grab the numeric part
		patchStr := strings.SplitN(parts[2], "-", 2)[0]
		minor, _ = strconv.Atoi(parts[1])
		patch, _ = strconv.Atoi(patchStr)
		if major > 0 || minor > 0 || patch > 0 {
			return fmt.Sprintf("%d.%d.%d", major, minor, patch), major, minor, patch
		}
	}
	// Fallback — treat as opaque tag
	return raw, 0, 0, 0
}
