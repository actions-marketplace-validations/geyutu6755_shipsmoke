// Command shipsmoke verifies release artifacts by really running them.
package main

import (
	"os"

	"github.com/geyutu6755/shipsmoke/internal/cli"
)

// Injected at build time: -ldflags "-X main.version=v1.0.0 -X main.commit=... -X main.date=..."
var (
	version = "0.0.0-dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	vi := cli.VersionInfo{Version: version, Commit: commit, Date: date}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, vi))
}
