// Command cs-lint checks that a repository, its documents and its claims hold
// together.
package main

import (
	"os"

	"github.com/codesweep-ai/lint/internal/cli"
)

func main() { os.Exit(cli.Execute()) }
