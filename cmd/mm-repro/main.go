// mm-repro is a CLI tool that generates a local reproducible Mattermost
// environment from a Mattermost support package.
//
// Usage:
//
//	mm-repro init --support-package ./support-package.zip
//	mm-repro plan --support-package ./support-package.zip
//	mm-repro validate --support-package ./support-package.zip
//	mm-repro doctor
//	mm-repro run --project ./generated-repro/my-repro
//	mm-repro stop --project ./generated-repro/my-repro
//	mm-repro reset --project ./generated-repro/my-repro
//	mm-repro report --project ./generated-repro/my-repro
package main

import (
	"os"

	"github.com/rohith0456/mattermost-support-package-repro/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
