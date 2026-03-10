package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rohith0456/mattermost-support-package-repro/internal/runtime"
)

var runProjectDir string

// outputFormatFor reads repro-plan.json from the project directory and returns the output_format.
// Returns "docker-compose" if the file is missing or the field is not set.
func outputFormatFor(projectDir string) string {
	planPath := filepath.Join(projectDir, "repro-plan.json")
	data, err := os.ReadFile(planPath)
	if err != nil {
		return "docker-compose"
	}
	var plan struct {
		OutputFormat string `json:"output_format"`
	}
	if err := json.Unmarshal(data, &plan); err != nil || plan.OutputFormat == "" {
		return "docker-compose"
	}
	return plan.OutputFormat
}

// newLauncherFor returns the appropriate launcher based on the project's output format.
func newLauncherFor(projectDir string) (interface {
	Up() error
	Down() error
	Reset() error
	Status() error
}, error) {
	format := outputFormatFor(projectDir)
	if format == "kubernetes" {
		return runtime.NewK8sLauncher(projectDir)
	}
	return runtime.NewLauncher(projectDir)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a generated repro environment",
	Long: `Start the repro environment for a generated repro project.
Automatically uses Docker Compose or Kubernetes (kind) based on
the output format recorded in repro-plan.json.

Example:
  mm-repro run --project ./generated-repro/my-repro`,
	RunE: func(cmd *cobra.Command, args []string) error {
		launcher, err := newLauncherFor(runProjectDir)
		if err != nil {
			return err
		}
		format := outputFormatFor(runProjectDir)
		printInfo(fmt.Sprintf("Starting %s environment in: %s", format, runProjectDir))
		if err := launcher.Up(); err != nil {
			return fmt.Errorf("starting environment: %w", err)
		}
		if format == "kubernetes" {
			printSuccess("Environment started. Open http://localhost:30065")
		} else {
			printSuccess("Environment started. Open http://localhost:8065")
		}
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a generated repro environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		launcher, err := newLauncherFor(stopProjectDir)
		if err != nil {
			return err
		}
		printInfo("Stopping environment...")
		if err := launcher.Down(); err != nil {
			return fmt.Errorf("stopping environment: %w", err)
		}
		printSuccess("Environment stopped")
		return nil
	},
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset a generated repro environment (removes all data)",
	Long: `Stop the environment and remove all data.
For Docker Compose: removes all Docker volumes.
For Kubernetes: deletes the entire kind cluster.
WARNING: All data will be lost.

Example:
  mm-repro reset --project ./generated-repro/my-repro`,
	RunE: func(cmd *cobra.Command, args []string) error {
		launcher, err := newLauncherFor(resetProjectDir)
		if err != nil {
			return err
		}
		printWarning("This will destroy all data in the repro environment. Ctrl+C to abort.")
		printInfo("Resetting environment...")
		if err := launcher.Reset(); err != nil {
			return fmt.Errorf("resetting environment: %w", err)
		}
		printSuccess("Environment reset. Run 'mm-repro run' to start fresh.")
		return nil
	},
}

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show pod/service status for a generated project",
	RunE: func(cmd *cobra.Command, args []string) error {
		launcher, err := newLauncherFor(reportProjectDir)
		if err != nil {
			return err
		}
		return launcher.Status()
	},
}

var stopProjectDir string
var resetProjectDir string
var reportProjectDir string

func init() {
	runCmd.Flags().StringVar(&runProjectDir, "project", "", "Path to generated repro project (required)")
	stopCmd.Flags().StringVar(&stopProjectDir, "project", "", "Path to generated repro project (required)")
	resetCmd.Flags().StringVar(&resetProjectDir, "project", "", "Path to generated repro project (required)")
	reportCmd.Flags().StringVar(&reportProjectDir, "project", "", "Path to generated repro project (required)")

	_ = runCmd.MarkFlagRequired("project")
	_ = stopCmd.MarkFlagRequired("project")
	_ = resetCmd.MarkFlagRequired("project")
	_ = reportCmd.MarkFlagRequired("project")
}
