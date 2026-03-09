// Package version provides build-time version information for mm-repro.
package version

import "fmt"

// These variables are set at build time via ldflags.
var (
	Version   = "v0.1.0-dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Info returns a formatted version string.
func Info() string {
	return fmt.Sprintf("mm-repro %s (commit: %s, built: %s)", Version, Commit, BuildDate)
}

// Short returns just the version string.
func Short() string {
	return Version
}
