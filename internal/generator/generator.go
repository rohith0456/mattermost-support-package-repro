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

	// Choose compose vs kubernetes output
	isKubernetes := g.plan.OutputFormat == "kubernetes"

	tasks := []struct {
		name string
		fn   func() (string, error)
	}{
		{"repro-plan.json", g.generatePlanJSON},
		{"REPRO_SUMMARY.md", g.generateReproSummary},
		{"REDACTION_REPORT.md", g.generateRedactionReport},
		{"PLUGIN_REPORT.md", g.generatePluginReport},
		{"README.md", g.generateReadme},
	}

	if isKubernetes {
		tasks = append(tasks,
			struct {
				name string
				fn   func() (string, error)
			}{"kubernetes/", g.generateKubernetes},
			struct {
				name string
				fn   func() (string, error)
			}{"Makefile", g.generateK8sMakefile},
			struct {
				name string
				fn   func() (string, error)
			}{"scripts/start.sh", g.generateK8sStartScript},
			struct {
				name string
				fn   func() (string, error)
			}{"scripts/stop.sh", g.generateK8sStopScript},
			struct {
				name string
				fn   func() (string, error)
			}{"scripts/reset.sh", g.generateK8sResetScript},
		)
	} else {
		tasks = append(tasks,
			struct {
				name string
				fn   func() (string, error)
			}{"docker-compose.yml", g.generateCompose},
			struct {
				name string
				fn   func() (string, error)
			}{".env", g.generateEnv},
			struct {
				name string
				fn   func() (string, error)
			}{"Makefile", g.generateMakefile},
			struct {
				name string
				fn   func() (string, error)
			}{"scripts/start.sh", g.generateStartScript},
			struct {
				name string
				fn   func() (string, error)
			}{"scripts/stop.sh", g.generateStopScript},
			struct {
				name string
				fn   func() (string, error)
			}{"scripts/reset.sh", g.generateResetScript},
		)
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

func (g *Generator) generateK8sMakefile() (string, error) {
	content := `# Generated Kubernetes Repro Makefile
# Run: make run   (creates kind cluster + applies manifests)
# Stop: make stop
# Reset: make reset  (WARNING: deletes entire kind cluster)

CLUSTER  := mm-repro
NS       := mattermost-repro
MANIFEST := kubernetes/

.PHONY: run stop reset logs status

## run: Create kind cluster (if needed) and apply all manifests
run:
	kind create cluster --name $(CLUSTER) 2>/dev/null || true
	kubectl apply -f $(MANIFEST)
	@echo "Waiting for Mattermost pod to be ready (up to 3 min)..."
	kubectl -n $(NS) wait --for=condition=ready pod -l app=mattermost --timeout=180s || true
	@echo ""
	@echo "Environment started. Open: http://localhost:30065"
	@echo "MailHog UI:             http://localhost:30025"

## stop: Delete manifests but keep cluster and volumes
stop:
	kubectl delete -f $(MANIFEST) --ignore-not-found

## reset: Delete the entire kind cluster (all data lost)
reset:
	kind delete cluster --name $(CLUSTER)
	@echo "Cluster deleted. Run 'make run' to start fresh."

## logs: Follow Mattermost pod logs
logs:
	kubectl -n $(NS) logs -l app=mattermost -f

## status: Show pod status
status:
	kubectl -n $(NS) get pods
`
	return g.writeFile("Makefile", content)
}

func (g *Generator) generateK8sStartScript() (string, error) {
	content := `#!/usr/bin/env bash
# Generated start script for mm-repro Kubernetes environment
set -euo pipefail

CLUSTER="mm-repro"
NS="mattermost-repro"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST_DIR="$(dirname "$SCRIPT_DIR")/kubernetes"

echo "Creating kind cluster '${CLUSTER}' (skipped if already exists)..."
kind create cluster --name "${CLUSTER}" 2>/dev/null || true

echo "Applying manifests from ${MANIFEST_DIR}..."
kubectl apply -f "${MANIFEST_DIR}"

echo ""
echo "Waiting for Mattermost pod..."
kubectl -n "${NS}" wait --for=condition=ready pod -l app=mattermost --timeout=180s || true

echo ""
echo "Environment started."
echo "  Mattermost: http://localhost:30065"
echo "  MailHog UI: http://localhost:30025"
echo ""
echo "See REPRO_SUMMARY.md for full connection details."
`
	return g.writeFile("scripts/start.sh", content)
}

func (g *Generator) generateK8sStopScript() (string, error) {
	content := `#!/usr/bin/env bash
# Generated stop script for mm-repro Kubernetes environment
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST_DIR="$(dirname "$SCRIPT_DIR")/kubernetes"

echo "Deleting Kubernetes manifests (cluster and data preserved)..."
kubectl delete -f "${MANIFEST_DIR}" --ignore-not-found
echo "Done. Run ./scripts/start.sh to restart."
`
	return g.writeFile("scripts/stop.sh", content)
}

func (g *Generator) generateK8sResetScript() (string, error) {
	content := `#!/usr/bin/env bash
# Generated reset script for mm-repro Kubernetes environment
# WARNING: Deletes the entire kind cluster — all data will be lost.
set -euo pipefail

CLUSTER="mm-repro"

echo "WARNING: This will delete the kind cluster '${CLUSTER}' and ALL data."
read -r -p "Are you sure? [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "Aborted."
    exit 0
fi

echo "Deleting kind cluster '${CLUSTER}'..."
kind delete cluster --name "${CLUSTER}"
echo "Reset complete. Run 'make run' or ./scripts/start.sh to start fresh."
`
	return g.writeFile("scripts/reset.sh", content)
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
