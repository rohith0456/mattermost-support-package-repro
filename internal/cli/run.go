package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rohith0456/mattermost-support-package-repro/internal/runtime"
)

var runProjectDir string

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a generated repro environment",
	Long: `Start the Docker Compose environment for a generated repro project.

Example:
  mm-repro run --project ./generated-repro/my-repro`,
	RunE: func(cmd *cobra.Command, args []string) error {
		launcher, err := runtime.NewLauncher(runProjectDir)
		if err != nil {
			return err
		}
		printInfo(fmt.Sprintf("Starting environment in: %s", runProjectDir))
		if err := launcher.Up(); err != nil {
			return fmt.Errorf("starting environment: %w", err)
		}
		printSuccess("Environment started. Open http://localhost:8065")
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a generated repro environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		launcher, err := runtime.NewLauncher(stopProjectDir)
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
	Short: "Reset a generated repro environment (removes all volumes)",
	Long: `Stop the environment and remove all Docker volumes.
WARNING: All data will be lost.

Example:
  mm-repro reset --project ./generated-repro/my-repro`,
	RunE: func(cmd *cobra.Command, args []string) error {
		launcher, err := runtime.NewLauncher(resetProjectDir)
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
	Short: "Show the repro report for a generated project",
	RunE: func(cmd *cobra.Command, args []string) error {
		launcher, err := runtime.NewLauncher(reportProjectDir)
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
