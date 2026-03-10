package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check local prerequisites for running mm-repro",
	Long: `Check that Docker Desktop, Docker Compose, disk space,
and required ports are available for running repro environments.`,
	RunE: runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("")
	fmt.Println("=== mm-repro Doctor ===")
	fmt.Println("")

	allOK := true

	// Check Docker
	fmt.Println("Checking Docker:")
	if dockerVersion, err := runCommand("docker", "version", "--format", "{{.Server.Version}}"); err == nil {
		printSuccess(fmt.Sprintf("Docker %s is running", strings.TrimSpace(dockerVersion)))
	} else {
		printWarning("Docker is not running or not installed")
		printInfo("Install: https://www.docker.com/products/docker-desktop/")
		allOK = false
	}

	// Check Docker Compose
	fmt.Println("\nChecking Docker Compose:")
	if _, err := runCommand("docker", "compose", "version"); err == nil {
		printSuccess("Docker Compose v2 (plugin) is available")
	} else if _, err := exec.LookPath("docker-compose"); err == nil {
		printSuccess("docker-compose (standalone) is available")
		printWarning("Consider upgrading to Docker Compose v2 (docker compose plugin)")
	} else {
		printWarning("Docker Compose not found")
		allOK = false
	}

	// Check disk space
	fmt.Println("\nChecking disk space:")
	checkDiskSpace(&allOK)

	// Check ports
	fmt.Println("\nChecking default ports:")
	ports := map[int]string{
		8065: "Mattermost",
		5432: "PostgreSQL",
		3306: "MySQL",
		9200: "OpenSearch",
		9000: "MinIO",
		8025: "MailHog UI",
		1025: "MailHog SMTP",
		8080: "Keycloak",
		389:  "OpenLDAP",
		3000: "Grafana",
		9090: "Prometheus",
	}
	for port, service := range ports {
		if isPortFreeSimple(port) {
			printSuccess(fmt.Sprintf("Port %-5d (%s) is available", port, service))
		} else {
			printWarning(fmt.Sprintf("Port %-5d (%s) may be in use", port, service))
		}
	}

	// Optional: ngrok (for --with-ngrok)
	fmt.Println("\nChecking optional tools for mobile/remote access:")
	if _, err := exec.LookPath("ngrok"); err == nil {
		if ver, e := runCommand("ngrok", "version"); e == nil {
			printSuccess(fmt.Sprintf("%s", strings.TrimSpace(ver)))
		} else {
			printSuccess("ngrok CLI is available")
		}
	} else {
		printInfo("[optional] ngrok CLI not found — needed only for Kubernetes --with-ngrok")
		printInfo("           Install: https://ngrok.com/download")
		printInfo("           (Docker Compose --with-ngrok uses the ngrok container instead)")
	}

	// Optional: kubectl + kind (for --with-kubernetes)
	fmt.Println("\nChecking optional Kubernetes tools (required only for --with-kubernetes):")
	if _, err := exec.LookPath("kubectl"); err == nil {
		if ver, e := runCommand("kubectl", "version", "--client", "--short"); e == nil {
			printSuccess(fmt.Sprintf("kubectl %s", strings.TrimSpace(ver)))
		} else {
			printSuccess("kubectl is available")
		}
	} else {
		printInfo("[optional] kubectl not found — needed for Kubernetes repro environments")
		printInfo("           Install: https://kubernetes.io/docs/tasks/tools/")
	}
	if _, err := exec.LookPath("kind"); err == nil {
		if ver, e := runCommand("kind", "version"); e == nil {
			printSuccess(fmt.Sprintf("%s", strings.TrimSpace(ver)))
		} else {
			printSuccess("kind is available")
		}
	} else {
		printInfo("[optional] kind not found — needed for Kubernetes repro environments")
		printInfo("           Install: https://kind.sigs.k8s.io/docs/user/quick-start/")
	}

	// Platform info
	fmt.Printf("\nPlatform: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	fmt.Println()
	if allOK {
		printSuccess("All required prerequisites satisfied. You're ready to run mm-repro!")
	} else {
		printWarning("Some prerequisites are missing. Please resolve the issues above.")
	}
	fmt.Println()
	return nil
}

func runCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func checkDiskSpace(allOK *bool) {
	// Simplified check — real implementation would use syscall
	if runtime.GOOS == "windows" {
		printInfo("Disk space check not available on Windows; ensure you have at least 10GB free")
	} else {
		out, err := runCommand("df", "-h", ".")
		if err == nil {
			lines := strings.Split(strings.TrimSpace(out), "\n")
			if len(lines) > 1 {
				printSuccess(fmt.Sprintf("Disk: %s", lines[1]))
			}
		} else {
			printInfo("Could not determine disk space; ensure at least 10GB is free")
		}
	}
}

func isPortFreeSimple(port int) bool {
	// Use nc or Test-NetConnection to check ports
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-Command",
			fmt.Sprintf("(Test-NetConnection -ComputerName localhost -Port %d -WarningAction SilentlyContinue).TcpTestSucceeded", port)).Output()
		if err != nil {
			return true // assume free if check fails
		}
		return strings.TrimSpace(string(out)) == "False"
	}

	out, err := exec.Command("sh", "-c", fmt.Sprintf("nc -z localhost %d 2>/dev/null; echo $?", port)).Output()
	if err != nil {
		return true
	}
	code, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return code != 0 // exit code 0 means port is in use
}
