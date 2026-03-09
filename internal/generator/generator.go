// Package generator renders the repro project files from a ReproPlan.
package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/version"
)

// Generator renders a repro project directory from a ReproPlan.
type Generator struct {
	plan      *models.ReproPlan
	outputDir string
	tmplDir   string // path to templates directory; empty means use embedded defaults
}

// NewGenerator creates a Generator.
func NewGenerator(plan *models.ReproPlan, outputDir, tmplDir string) *Generator {
	return &Generator{
		plan:      plan,
		outputDir: outputDir,
		tmplDir:   tmplDir,
	}
}

// Generate writes all project files and returns the list of files created.
func (g *Generator) Generate() ([]string, error) {
	if err := os.MkdirAll(g.outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	var created []string

	tasks := []struct {
		name string
		fn   func() (string, error)
	}{
		{"docker-compose.yml", g.generateCompose},
		{".env", g.generateEnv},
		{"Makefile", g.generateMakefile},
		{"REPRO_SUMMARY.md", g.generateReproSummary},
		{"REDACTION_REPORT.md", g.generateRedactionReport},
		{"PLUGIN_REPORT.md", g.generatePluginReport},
		{"repro-plan.json", g.generatePlanJSON},
		{"README.md", g.generateReadme},
		{"scripts/start.sh", g.generateStartScript},
		{"scripts/stop.sh", g.generateStopScript},
		{"scripts/reset.sh", g.generateResetScript},
	}

	for _, task := range tasks {
		path, err := task.fn()
		if err != nil {
			return created, fmt.Errorf("generating %s: %w", task.name, err)
		}
		created = append(created, path)
	}

	// Make scripts executable
	for _, script := range []string{"scripts/start.sh", "scripts/stop.sh", "scripts/reset.sh"} {
		_ = os.Chmod(filepath.Join(g.outputDir, script), 0o750)
	}

	return created, nil
}

// writeFile writes content to a file in the output directory.
func (g *Generator) writeFile(name, content string) (string, error) {
	path := filepath.Join(g.outputDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		return "", err
	}
	return path, nil
}

// renderTemplate renders a named template string with the given data.
func renderTemplate(name, tmpl string, data interface{}) (string, error) {
	t, err := template.New(name).Funcs(template.FuncMap{
		"now": func() string { return time.Now().Format(time.RFC3339) },
		"version": func() string { return version.Short() },
	}).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template %s: %w", name, err)
	}
	var buf []byte
	buf = make([]byte, 0, 4096)
	w := &bytesWriter{buf: &buf}
	if err := t.Execute(w, data); err != nil {
		return "", fmt.Errorf("executing template %s: %w", name, err)
	}
	return string(buf), nil
}

type bytesWriter struct {
	buf *[]byte
}

func (bw *bytesWriter) Write(p []byte) (int, error) {
	*bw.buf = append(*bw.buf, p...)
	return len(p), nil
}

func (g *Generator) generatePlanJSON() (string, error) {
	data, err := json.MarshalIndent(g.plan, "", "  ")
	if err != nil {
		return "", err
	}
	return g.writeFile("repro-plan.json", string(data))
}

func (g *Generator) generateMakefile() (string, error) {
	content := `# Generated Repro Makefile
# Run: make run
# Stop: make stop
# Reset: make reset

COMPOSE := docker compose
COMPOSE_FILE := docker-compose.yml
ENV_FILE := .env

.PHONY: run stop reset logs ps health

## run: Start all services
run:
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) up -d
	@echo "Environment started. Check REPRO_SUMMARY.md for connection details."

## stop: Stop all services
stop:
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) down

## reset: Stop and remove all volumes (WARNING: destroys data)
reset:
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) down -v
	@echo "Environment reset. All data volumes removed."

## logs: Follow logs for all services
logs:
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) logs -f

## ps: Show service status
ps:
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) ps

## health: Check service health
health:
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) ps --format "table {{.Service}}\t{{.Status}}\t{{.Ports}}"
`
	return g.writeFile("Makefile", content)
}

func (g *Generator) generateStartScript() (string, error) {
	content := `#!/usr/bin/env bash
# Generated start script for mm-repro environment
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Starting Mattermost repro environment..."
echo "Project: $PROJECT_DIR"
echo ""

docker compose -f "$PROJECT_DIR/docker-compose.yml" --env-file "$PROJECT_DIR/.env" up -d

echo ""
echo "Environment started. Services:"
docker compose -f "$PROJECT_DIR/docker-compose.yml" --env-file "$PROJECT_DIR/.env" ps
echo ""
echo "Mattermost: http://localhost:8065"
echo "See REPRO_SUMMARY.md for full connection details."
`
	return g.writeFile("scripts/start.sh", content)
}

func (g *Generator) generateStopScript() (string, error) {
	content := `#!/usr/bin/env bash
# Generated stop script for mm-repro environment
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Stopping Mattermost repro environment..."
docker compose -f "$PROJECT_DIR/docker-compose.yml" --env-file "$PROJECT_DIR/.env" down
echo "Done."
`
	return g.writeFile("scripts/stop.sh", content)
}

func (g *Generator) generateResetScript() (string, error) {
	content := `#!/usr/bin/env bash
# Generated reset script for mm-repro environment
# WARNING: This removes all Docker volumes — all data will be lost.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "WARNING: This will destroy all data in the repro environment."
read -r -p "Are you sure? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "Aborted."
    exit 0
fi

echo "Resetting Mattermost repro environment..."
docker compose -f "$PROJECT_DIR/docker-compose.yml" --env-file "$PROJECT_DIR/.env" down -v
echo "Reset complete. Run 'make run' or ./scripts/start.sh to start fresh."
`
	return g.writeFile("scripts/reset.sh", content)
}
