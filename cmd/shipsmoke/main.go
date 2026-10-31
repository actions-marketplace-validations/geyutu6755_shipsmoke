// Command shipsmoke verifies release artifacts by really running them.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/geyutu6755/shipsmoke/internal/cli"
)

// Injected at build time: -ldflags "-X main.version=v1.0.0 -X main.commit=... -X main.date=..."
var (
	version = "0.0.0-dev"
	commit  = "none"
	date    = "unknown"
)

// resolvedVersion prefers the ldflags-injected version, then falls back to
// the module version embedded by `go install ...@vX.Y.Z` so that real
// installs report the truth instead of a placeholder.
func resolvedVersion() string {
	if version != "" && version != "0.0.0-dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "0.0.0-dev"
}

func main() {
	vi := cli.VersionInfo{
		Version: resolvedVersion(),
		Commit:  commit,
		Date:    date,
	}
	if vi.Version == "0.0.0-dev" && vi.Commit == "none" {
		// Built without version metadata (plain go build): say where the
		// version would come from instead of implying it is a release.
		vi.Date = fmt.Sprintf("%s; install a release or use -ldflags for exact version", vi.Date)
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, vi))
}
