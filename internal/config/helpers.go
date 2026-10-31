package config

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
)

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

// envKeyRe matches legal environment variable names (POSIX-ish, plus letters).
var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// platformSpecRe matches "os/arch" platform specs used by runs_on.
var platformSpecRe = regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+$`)

// regexpCompile is an indirection so tests can rely on the same options as
// validation.
func regexpCompile(expr string) (*regexp.Regexp, error) {
	// (?s) so "." spans newlines, which is what people expect when matching
	// whole captured output.
	return regexp.Compile("(?s)" + expr)
}

// CompileExpectRegexes is used by the engine to obtain pre-compiled matchers.
func CompileExpectRegexes(expr string) (*regexp.Regexp, error) {
	return regexpCompile(expr)
}

func baseName(p string) string {
	b := filepath.Base(p)
	// Strip stacked suffixes such as ".tar.gz".
	for ext := filepath.Ext(b); ext != "" && ext != "."; ext = filepath.Ext(b) {
		b = strings.TrimSuffix(b, ext)
	}
	if b == "" || b == "." || b == string(filepath.Separator) {
		return filepath.Base(p)
	}
	return b
}

// ExampleYAML is the file written by `shipsmoke init`.
const ExampleYAML = `# .shipsmoke.yml — ShipSmoke configuration
# Verify that release artifacts actually extract and run. Docs:
# https://github.com/geyutu6755/shipsmoke/blob/main/docs/CONFIGURATION.md

version: 1

# Optional extraction safety limits (defaults shown).
safety:
  max_files: 20000              # max entries per archive
  max_total_size: 1073741824    # 1 GiB total uncompressed
  max_file_size: 536870912      # 512 MiB per file
  max_compression_ratio: 1000   # reject archives that expand absurdly

# Optional: write a machine-readable JSON report to this path.
report:
  json: ""

artifacts:
  - name: mytool-linux-amd64
    path: dist/mytool-linux-amd64.tar.gz
    format: auto                # auto | zip | tar.gz | binary
    binary: mytool              # entry program, relative to the archive root
    runs_on: [linux/amd64]      # verify only where this artifact actually runs
    env: minimal                # minimal (default) | inherit
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
          stdout_contains: ["mytool version"]
        timeout: 30s
      - args: ["--help"]
        expect:
          exit_code: 0
        timeout: 30s

  # Standalone binary example:
  # - name: mytool-windows-amd64
  #   path: dist/mytool-windows-amd64.exe
  #   binary: mytool-windows-amd64.exe   # for standalone files, the file itself
  #   runs_on: [windows/amd64]
  #   checks:
  #     - args: ["--version"]
  #       expect:
  #         exit_code: 0
`
