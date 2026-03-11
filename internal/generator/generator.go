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
		// Config files referenced as volume mounts in docker-compose.yml
		if g.plan.Services.Observability.PrometheusEnabled {
			tasks = append(tasks, struct {
				name string
				fn   func() (string, error)
			}{"config/prometheus.yml", g.generatePrometheusConfig})
		}
		if g.plan.Topology == "multi-node" && g.plan.NodeCount > 1 {
			tasks = append(tasks, struct {
				name string
				fn   func() (string, error)
			}{"nginx/nginx.conf", g.generateNginxConfig})
		}
		if g.plan.Services.Auth.LDAPEnabled {
			tasks = append(tasks, struct {
				name string
				fn   func() (string, error)
			}{"ldap/users.ldif", g.generateLDIF})
		}
		if g.plan.Services.Auth.KeycloakEnabled {
			tasks = append(tasks, struct {
				name string
				fn   func() (string, error)
			}{"keycloak/repro-realm.json", g.generateKeycloakRealm})
		}
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
	ngrokTargets := ""
	if g.plan.Services.Tunnel.NgrokEnabled {
		ngrokTargets = `
## ngrok: Start ngrok CLI tunnel to Mattermost (mobile/remote access)
ngrok:
	@which ngrok > /dev/null 2>&1 || (echo "ngrok CLI not found. Install: https://ngrok.com/download" && exit 1)
	ngrok http localhost:30065

## ngrok-url: Print the current public ngrok URL
ngrok-url:
	@curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"https://[^"]*"' | head -1 | cut -d'"' -f4 || echo "ngrok not running — run 'make ngrok' first"

## mobile: Alias for ngrok-url
mobile: ngrok-url
`
	}

	k8sLdapTargets := ""
	if g.plan.Services.Auth.LDAPEnabled {
		k8sLdapTargets = `
## ldap-users: Load test LDAP users into OpenLDAP (run once after 'make run')
## Re-run safely — existing entries are skipped automatically.
## Users: alice.johnson bob.smith carol.white dave.brown eve.davis frank.miller grace.wilson henry.moore
## Password for all users: Repro1234!
ldap-users:
	@echo "Loading test LDAP users..."
	@kubectl -n $(NS) exec deploy/openldap -- \
	  ldapadd -x \
	  -D "cn=admin,dc=repro,dc=local" \
	  -w "ldap_admin_local_repro_only" \
	  -f /ldap/users.ldif 2>&1 | grep -v "already exists" || true
	@echo "Done. User password: Repro1234!"

## ldap-sync: Trigger immediate LDAP sync in Mattermost (requires Enterprise license)
## Usage: make ldap-sync PASS=Sysadmin1!
ldap-sync:
	@PASS=$${PASS:-Sysadmin1!}; \
	 TOKEN=$$(curl -sf -X POST http://localhost:30065/api/v4/users/login \
	     -H "Content-Type: application/json" \
	     -d "{\"login_id\":\"sysadmin\",\"password\":\"$$PASS\"}" -D - 2>/dev/null \
	   | grep -i '^token:' | awk '{print $$2}' | tr -d '\r'); \
	 test -n "$$TOKEN" || (echo "✗ Auth failed — run 'make admin' first." && exit 1); \
	 curl -sf -X POST http://localhost:30065/api/v4/ldap/sync \
	     -H "Authorization: Bearer $$TOKEN" > /dev/null && \
	 echo "✓ LDAP sync triggered. Users will appear in Mattermost within ~60 seconds." || \
	 echo "✗ Sync failed — check that Enterprise license is uploaded ('make upload-license')."
`
	}

	k8sKeycloakTargets := ""
	if g.plan.Services.Auth.KeycloakEnabled {
		k8sKeycloakTargets = `
## azure-ad: Show Azure AD / Keycloak authentication status and test credentials
azure-ad:
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  Azure AD local simulation via Keycloak"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  Keycloak console : http://localhost:30080 (if port-forwarded)"
	@echo "  Realm            : repro"
	@echo "  Admin login      : admin / keycloak_admin_local_repro_only"
	@echo ""
	@echo "  OIDC (Entra ID simulation) — works NOW, no license needed:"
	@echo "  → Click 'Sign in with GitLab' on the Mattermost login page"
	@echo ""
	@echo "  SAML — requires Enterprise license:"
	@echo "  → run: make upload-license LICENSE=./your.mattermost-license PASS=Sysadmin1!"
	@echo "  → SAML is pre-configured — no System Console steps needed after upload"
	@echo ""
	@echo "  Test users (password: Repro1234!):"
	@echo "    alice.johnson   bob.smith    carol.white  dave.brown"
	@echo "    eve.davis       frank.miller grace.wilson henry.moore"
	@echo ""
`
	}

	k8sPhonyExtra := " upload-license"
	if g.plan.Services.Auth.LDAPEnabled {
		k8sPhonyExtra += " ldap-users ldap-sync"
	}
	if g.plan.Services.Auth.KeycloakEnabled {
		k8sPhonyExtra += " azure-ad"
	}

	content := `# Generated Kubernetes Repro Makefile
# Run: make run   (creates kind cluster + applies manifests)
# Stop: make stop
# Reset: make reset  (WARNING: deletes entire kind cluster)

CLUSTER  := mm-repro
NS       := mattermost-repro
MANIFEST := kubernetes/

.PHONY: run stop reset logs status admin seed channels ngrok ngrok-url mobile` + k8sPhonyExtra + `

## run: Create kind cluster (if needed) and apply all manifests
run:
	kind create cluster --name $(CLUSTER) 2>/dev/null || true
	kubectl apply -f $(MANIFEST)
	@echo "Waiting for Mattermost pod to be ready (up to 3 min)..."
	kubectl -n $(NS) wait --for=condition=ready pod -l app=mattermost --timeout=180s || true
	@echo ""
	@echo "Environment started. Open: http://localhost:30065"
	@echo "Mailpit (email) UI:     http://localhost:30025"
	@echo ""
	@echo "Next: run 'make admin' to create the sysadmin account (first time only)"

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

## admin: Create the default sysadmin account (run once after 'make run')
## Creates sysadmin / Sysadmin1! via REST API — safe to re-run.
admin:
	@which mm-repro > /dev/null 2>&1 || (echo "mm-repro not found. Install: go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest" && exit 1)
	mm-repro seed --url http://localhost:30065 --posts 0 --password Sysadmin1!
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  Login at http://localhost:30065"
	@echo "  Username : sysadmin"
	@echo "  Password : Sysadmin1!"
	@echo "  (Email/password login — works without LDAP/SAML)"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

## seed: Seed test posts, images and reactions (run after 'make admin')
## Usage:   make seed PASS=Sysadmin1!
## Options: CHANNELS="support,bugs"  — also create these channels
##          CHANNEL=support          — post only into this channel
seed:
	@which mm-repro > /dev/null 2>&1 || (echo "mm-repro not found. Install: go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest" && exit 1)
	mm-repro seed --url http://localhost:30065 --project . \
	  $(if $(PASS),--password $(PASS),) \
	  $(if $(CHANNELS),--channels "$(CHANNELS)",) \
	  $(if $(CHANNEL),--channel "$(CHANNEL)",)

## channels: Create one or more channels by name (run after 'make admin')
## Usage: make channels NAMES="support,bugs,release-notes" PASS=Sysadmin1!
channels:
	@which mm-repro > /dev/null 2>&1 || (echo "mm-repro not found. Install: go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest" && exit 1)
	@test -n "$(NAMES)" || (echo "Usage: make channels NAMES=\"chan1,chan2\" PASS=Sysadmin1!" && exit 1)
	mm-repro seed --url http://localhost:30065 --project . --posts 0 --channels "$(NAMES)" $(if $(PASS),--password $(PASS),)
	@echo "Channels created."
` + k8sLdapTargets + k8sKeycloakTargets + `
## upload-license: Upload a Mattermost license after startup to unlock Enterprise features
## Usage: make upload-license LICENSE=./your.mattermost-license PASS=Sysadmin1!
## After upload: SAML and LDAP sync activate automatically (already pre-configured).
upload-license:
	@test -n "$(LICENSE)" || (echo "Usage: make upload-license LICENSE=./path/to.mattermost-license PASS=Sysadmin1!" && exit 1)
	@test -f "$(LICENSE)" || (echo "Error: File not found: $(LICENSE)" && exit 1)
	@PASS=$${PASS:-Sysadmin1!}; \
	 TOKEN=$$(curl -sf -X POST http://localhost:30065/api/v4/users/login \
	     -H "Content-Type: application/json" \
	     -d "{\"login_id\":\"sysadmin\",\"password\":\"$$PASS\"}" -D - 2>/dev/null \
	   | grep -i '^token:' | awk '{print $$2}' | tr -d '\r'); \
	 test -n "$$TOKEN" || (echo "✗ Auth failed — run 'make admin' first." && exit 1); \
	 curl -sf -X POST http://localhost:30065/api/v4/license \
	     -H "Authorization: Bearer $$TOKEN" -F "license=@$(LICENSE)" > /dev/null && \
	 echo "✓ License uploaded. Enterprise features (SAML, LDAP sync) are now active." || \
	 echo "✗ Upload may have failed — check Mattermost logs with 'make logs'."
` + ngrokTargets
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
echo "  Mailpit (email) UI: http://localhost:30025"
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
	ngrokTargets := ""
	if g.plan.Services.Tunnel.NgrokEnabled {
		ngrokTargets = `
## ngrok-url: Print the current public ngrok URL (mobile/remote access)
ngrok-url:
	@curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"https://[^"]*"' | head -1 | cut -d'"' -f4 || echo "ngrok not ready — wait a moment and try again"

## mobile: Alias for ngrok-url
mobile: ngrok-url
`
	}

	ldapTargets := ""
	if g.plan.Services.Auth.LDAPEnabled {
		ldapTargets = `
## ldap-users: Load test LDAP users into OpenLDAP (run once after 'make run')
## Re-run safely — existing entries are skipped automatically.
## Users: alice.johnson bob.smith carol.white dave.brown eve.davis frank.miller grace.wilson henry.moore
## Password for all users: Repro1234!
ldap-users:
	@echo "Loading test LDAP users..."
	@$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec -T openldap \
	  ldapadd -x \
	  -D "cn=admin,dc=repro,dc=local" \
	  -w "ldap_admin_local_repro_only" \
	  -f /ldap/users.ldif 2>&1 | grep -v "already exists" || true
	@echo "Done. Verify at http://localhost:8089  (phpLDAPadmin)"
	@echo "  Bind DN  : cn=admin,dc=repro,dc=local"
	@echo "  Password : ldap_admin_local_repro_only"
	@echo "  User pwd : Repro1234!"

## ldap-sync: Trigger immediate LDAP sync in Mattermost (requires Enterprise license)
## Usage: make ldap-sync PASS=Sysadmin1!
ldap-sync:
	@PASS=$${PASS:-Sysadmin1!}; \
	 TOKEN=$$(curl -sf -X POST http://localhost:8065/api/v4/users/login \
	     -H "Content-Type: application/json" \
	     -d "{\"login_id\":\"sysadmin\",\"password\":\"$$PASS\"}" -D - 2>/dev/null \
	   | grep -i '^token:' | awk '{print $$2}' | tr -d '\r'); \
	 test -n "$$TOKEN" || (echo "✗ Auth failed — run 'make admin' first." && exit 1); \
	 curl -sf -X POST http://localhost:8065/api/v4/ldap/sync \
	     -H "Authorization: Bearer $$TOKEN" > /dev/null && \
	 echo "✓ LDAP sync triggered. Users will appear in Mattermost within ~60 seconds." || \
	 echo "✗ Sync failed — check that Enterprise license is uploaded ('make upload-license')."
`
	}

	keycloakTargets := ""
	if g.plan.Services.Auth.KeycloakEnabled {
		keycloakTargets = `
## azure-ad: Show Azure AD / Keycloak authentication status and test credentials
azure-ad:
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  Azure AD local simulation via Keycloak"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  Keycloak console : http://localhost:8080"
	@echo "  Realm            : repro"
	@echo "  Admin login      : admin / keycloak_admin_local_repro_only"
	@echo ""
	@echo "  OIDC (Entra ID simulation) — works NOW, no license needed:"
	@echo "  → Click 'Sign in with GitLab' on the Mattermost login page"
	@echo ""
	@echo "  SAML — requires Enterprise license:"
	@echo "  → run: make upload-license LICENSE=./your.mattermost-license PASS=Sysadmin1!"
	@echo "  → SAML is pre-configured — no System Console steps needed after upload"
	@echo "  → Click 'Sign in with SAML' after license is active"
	@echo ""
	@echo "  Test users (password: Repro1234!):"
	@echo "    alice.johnson   bob.smith    carol.white  dave.brown"
	@echo "    eve.davis       frank.miller grace.wilson henry.moore"
	@echo ""
`
	}

	phonyExtra := " upload-license"
	if g.plan.Services.Auth.LDAPEnabled {
		phonyExtra += " ldap-users ldap-sync"
	}
	if g.plan.Services.Auth.KeycloakEnabled {
		phonyExtra += " azure-ad"
	}

	content := `# Generated Repro Makefile
# Run: make run
# Stop: make stop
# Reset: make reset

COMPOSE := docker compose
COMPOSE_FILE := docker-compose.yml
ENV_FILE := .env

.PHONY: run pull stop reset logs ps health admin seed channels ngrok-url mobile` + phonyExtra + `

## run: Pull images (ensures platform-correct versions) then start all services
## On Apple Silicon this prevents "no matching manifest" / platform cache errors.
run:
	@echo "Pulling images (ensures platform-correct versions on all architectures)..."
	@$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) pull --quiet --ignore-pull-failures 2>/dev/null || true
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) up -d
	@echo ""
	@echo "Environment started!"
	@echo "  Next: run 'make admin' to create the sysadmin account (first time only)"
	@echo "  Then open http://localhost:8065"
	@echo "  See REPRO_SUMMARY.md for all connection details."

## pull: Pull/refresh all Docker images (also run this if you see platform errors)
pull:
	$(COMPOSE) -f $(COMPOSE_FILE) --env-file $(ENV_FILE) pull

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

## admin: Create the default sysadmin account (run once after 'make run')
## This creates sysadmin / Sysadmin1! via the Mattermost REST API.
## Safe to re-run — silently skips if the account already exists.
admin:
	@which mm-repro > /dev/null 2>&1 || (echo "mm-repro not found. Install: go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest" && exit 1)
	mm-repro seed --project . --posts 0 --password Sysadmin1!
	@echo ""
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  Login at http://localhost:8065"
	@echo "  Username : sysadmin"
	@echo "  Password : Sysadmin1!"
	@echo "  (Email/password login — works without LDAP/SAML)"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

## seed: Seed test posts, images and reactions (run after 'make admin')
## Usage:   make seed PASS=Sysadmin1!
## Options: CHANNELS="support,bugs"  — also create these channels
##          CHANNEL=support          — post only into this channel
## Or:      mm-repro seed --project . --with-files --password Sysadmin1!
seed:
	@which mm-repro > /dev/null 2>&1 || (echo "mm-repro not found. Install: go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest" && exit 1)
	mm-repro seed --project . \
	  $(if $(PASS),--password $(PASS),) \
	  $(if $(CHANNELS),--channels "$(CHANNELS)",) \
	  $(if $(CHANNEL),--channel "$(CHANNEL)",)

## channels: Create one or more channels by name (run after 'make admin')
## Usage: make channels NAMES="support,bugs,release-notes" PASS=Sysadmin1!
channels:
	@which mm-repro > /dev/null 2>&1 || (echo "mm-repro not found. Install: go install github.com/rohith0456/mattermost-support-package-repro/cmd/mm-repro@latest" && exit 1)
	@test -n "$(NAMES)" || (echo "Usage: make channels NAMES=\"chan1,chan2\" PASS=Sysadmin1!" && exit 1)
	mm-repro seed --project . --posts 0 --channels "$(NAMES)" $(if $(PASS),--password $(PASS),)
	@echo "Channels created."
` + ldapTargets + keycloakTargets + `
## upload-license: Upload a Mattermost license after startup to unlock Enterprise features
## Usage: make upload-license LICENSE=./your.mattermost-license PASS=Sysadmin1!
## After upload: SAML and LDAP sync activate automatically (already pre-configured).
upload-license:
	@test -n "$(LICENSE)" || (echo "Usage: make upload-license LICENSE=./path/to.mattermost-license PASS=Sysadmin1!" && exit 1)
	@test -f "$(LICENSE)" || (echo "Error: File not found: $(LICENSE)" && exit 1)
	@PASS=$${PASS:-Sysadmin1!}; \
	 TOKEN=$$(curl -sf -X POST http://localhost:8065/api/v4/users/login \
	     -H "Content-Type: application/json" \
	     -d "{\"login_id\":\"sysadmin\",\"password\":\"$$PASS\"}" -D - 2>/dev/null \
	   | grep -i '^token:' | awk '{print $$2}' | tr -d '\r'); \
	 test -n "$$TOKEN" || (echo "✗ Auth failed — run 'make admin' first." && exit 1); \
	 curl -sf -X POST http://localhost:8065/api/v4/license \
	     -H "Authorization: Bearer $$TOKEN" -F "license=@$(LICENSE)" > /dev/null && \
	 echo "✓ License uploaded. Enterprise features (SAML, LDAP sync) are now active." || \
	 echo "✗ Upload may have failed — check Mattermost logs with 'make logs'."
` + ngrokTargets
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

echo "Pulling images (ensures platform-correct versions on Apple Silicon and x86)..."
docker compose -f "$PROJECT_DIR/docker-compose.yml" --env-file "$PROJECT_DIR/.env" pull --quiet --ignore-pull-failures 2>/dev/null || true

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

func (g *Generator) generatePrometheusConfig() (string, error) {
	p := g.plan
	var targets string
	if p.Topology == "multi-node" && p.NodeCount > 1 {
		t := ""
		for i := 1; i <= p.NodeCount; i++ {
			t += fmt.Sprintf("'mattermost-%d:8067', ", i)
		}
		targets = t[:len(t)-2] // trim trailing comma+space
	} else {
		targets = "'mattermost:8067'"
	}

	content := fmt.Sprintf(`# Prometheus configuration — generated by mm-repro
# Scrapes Mattermost metrics on the internal Docker network.
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'mattermost'
    static_configs:
      - targets: [%s]
    metrics_path: '/metrics'
`, targets)
	return g.writeFile("config/prometheus.yml", content)
}

func (g *Generator) generateNginxConfig() (string, error) {
	p := g.plan
	var upstream string
	for i := 1; i <= p.NodeCount; i++ {
		upstream += fmt.Sprintf("    server mattermost-%d:8065;\n", i)
	}

	content := fmt.Sprintf(`# nginx configuration — generated by mm-repro for %d-node topology
upstream mattermost {
%s}

server {
    listen 80;
    server_name localhost;

    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    location / {
        proxy_pass http://mattermost;
        proxy_read_timeout 90;
    }

    location ~ /api/v[0-9]+/(users/)?websocket$ {
        proxy_pass http://mattermost;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
`, p.NodeCount, upstream)
	return g.writeFile("nginx/nginx.conf", content)
}
