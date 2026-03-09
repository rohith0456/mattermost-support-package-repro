// Package cli provides the mm-repro command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/rohith0456/mattermost-support-package-repro/pkg/version"
)

var (
	rootCmd = &cobra.Command{
		Use:   "mm-repro",
		Short: "Generate local Mattermost reproduction environments from support packages",
		Long: `mm-repro parses a Mattermost support package and generates a local Docker
Compose environment that approximates the customer's setup.

IMPORTANT: This tool is for LOCAL DEBUGGING ONLY. It never uses real
production credentials and always generates safe local substitutes.

Documentation: https://github.com/rohith0456/mattermost-support-package-repro`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	verboseFlag bool
)

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		printError(err)
		return err
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Info())
	},
}

func printError(err error) {
	red := color.New(color.FgRed, color.Bold)
	fmt.Fprintf(os.Stderr, "%s %s\n", red.Sprint("ERROR:"), err.Error())
}

func printSuccess(msg string) {
	green := color.New(color.FgGreen, color.Bold)
	fmt.Printf("%s %s\n", green.Sprint("✓"), msg)
}

func printInfo(msg string) {
	cyan := color.New(color.FgCyan)
	fmt.Printf("%s %s\n", cyan.Sprint("→"), msg)
}

func printWarning(msg string) {
	yellow := color.New(color.FgYellow, color.Bold)
	fmt.Printf("%s %s\n", yellow.Sprint("⚠"), msg)
}

func printBanner() {
	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("\n%s\n", cyan.Sprint("mm-repro — Mattermost Support Package Reproducer"))
	fmt.Printf("Version: %s\n\n", version.Short())
	printWarning("This tool is for LOCAL DEBUGGING ONLY.")
	printWarning("All generated credentials are local-only — not from customer environments.")
	fmt.Println()
}
