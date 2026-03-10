package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rohith0456/mattermost-support-package-repro/internal/seeder"
)

var (
	seedProjectDir string
	seedURL        string
	seedUsername   string
	seedPassword   string
	seedPostCount  int
	seedWithFiles  bool
	seedChannels   []string
	seedChannel    string
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed test posts, images and reactions into a running repro environment",
	Long: `Connects to the running Mattermost instance and creates test content:
posts with markdown formatting, code blocks, links, threaded conversations,
emoji reactions, and optional file attachments (images + log files).

Run after 'make run' (or 'mm-repro run') once Mattermost is accessible.

Examples:
  # Basic seed using defaults (20 posts, no files)
  mm-repro seed --project ./generated-repro/my-repro

  # Seed with file attachments
  mm-repro seed --project ./generated-repro/my-repro --with-files

  # Custom post count and explicit credentials
  mm-repro seed --project . --posts 40 --username sysadmin --password MyPass1!

  # Target a custom URL (e.g. behind ngrok)
  mm-repro seed --url https://abc123.ngrok.io --username sysadmin --password MyPass1!`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve URL: flag → .env in project dir → fallback
		url := seedURL
		if url == "" && seedProjectDir != "" {
			url = readSiteURLFromEnv(seedProjectDir)
		}
		if url == "" {
			url = "http://localhost:8065"
		}

		// Prompt for password if not provided
		password := seedPassword
		if password == "" {
			p, err := promptPassword(fmt.Sprintf("Password for %s at %s: ", seedUsername, url))
			if err != nil {
				return fmt.Errorf("reading password: %w", err)
			}
			password = p
		}

		opts := seeder.Options{
			SiteURL:     url,
			Username:    seedUsername,
			Password:    password,
			PostCount:   seedPostCount,
			WithFiles:   seedWithFiles,
			Verbose:     verboseFlag,
			Channels:    seedChannels,
			PostChannel: seedChannel,
		}

		printBanner()
		printInfo(fmt.Sprintf("Seeding test content into %s", url))
		if seedWithFiles {
			printInfo("File attachments: enabled (images + log files)")
		}
		fmt.Printf("  Posts to create: %d\n", seedPostCount)
		fmt.Println()

		s := seeder.New(url)
		if err := s.Run(opts); err != nil {
			return err
		}

		fmt.Println()
		printSuccess("Seeding complete!")
		fmt.Printf("\n  Open %s and explore:\n", url)
		if seedChannel != "" {
			fmt.Printf("  • ~%s  — all seeded posts\n", seedChannel)
		} else {
			fmt.Println("  • ~town-square  — posts, code blocks, tables, threads")
			fmt.Println("  • ~off-topic    — long messages, links, reactions, pins")
		}
		for _, ch := range seedChannels {
			fmt.Printf("  • ~%s  — newly created channel\n", ch)
		}
		if seedWithFiles {
			fmt.Println("  • File previews — images and log file attachments")
		}
		fmt.Println()
		fmt.Println("  Tip: run again with --with-files to add image and log attachments.")
		return nil
	},
}

func init() {
	seedCmd.Flags().StringVar(&seedProjectDir, "project", ".", "Path to generated repro project (reads MM_SITE_URL from .env)")
	seedCmd.Flags().StringVar(&seedURL, "url", "", "Mattermost URL (overrides .env; default: http://localhost:8065)")
	seedCmd.Flags().StringVar(&seedUsername, "username", "sysadmin", "Mattermost admin username")
	seedCmd.Flags().StringVar(&seedPassword, "password", "", "Mattermost admin password (prompted if omitted)")
	seedCmd.Flags().IntVar(&seedPostCount, "posts", 20, "Number of posts to create (max ~40 for varied content)")
	seedCmd.Flags().BoolVar(&seedWithFiles, "with-files", false, "Attach test PNG images and log files to posts")
	seedCmd.Flags().StringSliceVar(&seedChannels, "channels", nil, `Comma-separated channel names to create, e.g. --channels "support,bugs,release-notes"`)
	seedCmd.Flags().StringVar(&seedChannel, "channel", "", "Seed posts only into this channel (must exist or be listed in --channels)")
}

// readSiteURLFromEnv reads MM_SITE_URL from the .env file in the project directory.
func readSiteURLFromEnv(projectDir string) string {
	envPath := filepath.Join(projectDir, ".env")
	f, err := os.Open(envPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "MM_SITE_URL=") {
			return strings.TrimPrefix(line, "MM_SITE_URL=")
		}
	}
	return ""
}

// promptPassword reads a password from the terminal without echoing.
func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// Fallback for non-TTY (e.g. piped input)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
