// Package cli implements the shipsmoke command line interface.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/geyutu6755/shipsmoke/internal/cigen"
	"github.com/geyutu6755/shipsmoke/internal/config"
	"github.com/geyutu6755/shipsmoke/internal/engine"
	"github.com/geyutu6755/shipsmoke/internal/report"
)

// VersionInfo is injected at build time via -ldflags.
type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

const usageTmpl = `shipsmoke — test what users actually download.

Verify that release artifacts (zip, tar.gz, standalone binaries) actually
extract and run on this operating system, with configurable checks and
PASS/FAIL reports for humans and CI.

Usage:
  shipsmoke <command> [flags]

Commands:
  check      run verification (the release quality gate)
  validate   validate .shipsmoke.yml without running anything
  init       write an example .shipsmoke.yml
  version    show version information

Run "shipsmoke <command> --help" for details.

Exit codes:
  0  all executed checks passed
  1  at least one check failed
  2  configuration or infrastructure error
  3  nothing was executed (all checks skipped); CI should not go green

Security note: shipsmoke executes programs. It isolates the working
directory and terminates timed-out processes, but it is NOT an operating
system sandbox. Only verify artifacts you trust, on machines without
secrets you care about. See SECURITY.md.

Documentation: https://github.com/geyutu6755/shipsmoke
`

// Run is the CLI entrypoint. It returns the process exit code.
func Run(args []string, stdout, stderr io.Writer, vi VersionInfo) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usageTmpl)
		return 0
	}
	switch args[0] {
	case "check":
		return runCheck(args[1:], stdout, stderr, vi)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "shipsmoke %s (%s/%s)\ncommit: %s\nbuilt: %s\n",
			vi.Version, runtime.GOOS, runtime.GOARCH, vi.Commit, vi.Date)
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usageTmpl)
		return 0
	default:
		fmt.Fprintf(stderr, "shipsmoke: unknown command %q\n\n", args[0])
		fmt.Fprint(stderr, usageTmpl)
		return engine.ExitUsage
	}
}

// --- check ---

func newCheckFlagSet() (*flag.FlagSet, *checkOpts) {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o := &checkOpts{}
	fs.StringVar(&o.ConfigPath, "config", "", "path to .shipsmoke.yml (default: ./.shipsmoke.yml)")
	fs.StringVar(&o.Artifact, "artifact", "", "verify only the artifact with this name")
	fs.StringVar(&o.ReportPath, "report", "", "write the JSON report to this path (overrides config)")
	fs.BoolVar(&o.AllowEmpty, "allow-empty", false, "exit 0 even when no check was executed (default: exit 3)")
	fs.BoolVar(&o.NoColor, "no-color", false, "disable colored terminal output")
	fs.BoolVar(&o.Quiet, "quiet", false, "only print failures and the summary")
	return fs, o
}

type checkOpts struct {
	ConfigPath string
	Artifact   string
	ReportPath string
	AllowEmpty bool
	NoColor    bool
	Quiet      bool
}

func runCheck(args []string, stdout, stderr io.Writer, vi VersionInfo) int {
	fs, o := newCheckFlagSet()
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "check", err)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "shipsmoke check: unexpected argument %q\n", fs.Arg(0))
		return engine.ExitUsage
	}

	cfgPath := o.ConfigPath
	if cfgPath == "" {
		cfgPath = config.DefaultConfigName
	}
	if _, err := os.Stat(cfgPath); err != nil {
		fmt.Fprintf(stderr, "shipsmoke check: %s not found.\nRun \"shipsmoke init\" to create an example, or pass --config.\n", cfgPath)
		return engine.ExitUsage
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "shipsmoke check: %v\n", err)
		return engine.ExitUsage
	}

	opts := engine.Options{
		ConfigPath:      cfgPath,
		Version:         vi.Version,
		ArtifactFilter:  o.Artifact,
		AllowEmpty:      o.AllowEmpty,
		StepSummaryPath: os.Getenv("GITHUB_STEP_SUMMARY"),
	}

	rep, exit := engine.Run(cfg, opts)

	// Terminal report.
	useColor := !o.NoColor && isTerminal(os.Stdout)
	if o.Quiet {
		rep.Terminal(io.Discard, false, nil)
		printQuiet(stdout, rep, useColor, report.NewRedactor())
	} else {
		rep.Terminal(stdout, useColor, report.NewRedactor())
	}

	// JSON report.
	jsonPath := o.ReportPath
	if jsonPath == "" {
		jsonPath = cfg.Report.JSON
	}
	if jsonPath != "" {
		if err := report.WriteJSONFile(jsonPath, rep, report.NewRedactor()); err != nil {
			fmt.Fprintf(stderr, "shipsmoke check: %v\n", err)
			return engine.ExitUsage
		}
		fmt.Fprintf(stdout, "json report: %s\n", jsonPath)
	}

	// GitHub Actions job summary.
	if opts.StepSummaryPath != "" {
		if err := report.WriteGitHubSummary(opts.StepSummaryPath, rep); err != nil {
			fmt.Fprintf(stderr, "shipsmoke check: %v\n", err)
		}
	}
	return exit
}

func printQuiet(w io.Writer, rep *report.Report, useColor bool, red *report.Redactor) {
	for _, a := range rep.Artifacts {
		if a.Decision == "skipped" {
			fmt.Fprintf(w, "[SKIP] %s — %s\n", a.Name, red.Apply(a.SkipReason))
			continue
		}
		if a.Decision == "error" {
			fmt.Fprintf(w, "[ERROR] %s — %s\n", a.Name, red.Apply(a.Error))
			continue
		}
		for _, c := range a.Checks {
			if c.Status != report.StatusPass {
				fmt.Fprintf(w, "[%s] %s: %s\n", c.Status, a.Name, c.Name)
				for _, r := range c.Reasons {
					fmt.Fprintf(w, "  • %s\n", red.Apply(r))
				}
			}
		}
	}
	s := rep.Summary
	fmt.Fprintf(w, "summary: %d passed, %d failed, %d skipped, %d errored → %s\n",
		s.Passed, s.Failed, s.Skipped, s.Errored, s.Overall)
}

// --- validate ---

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	cfgPath := fs.String("config", "", "path to .shipsmoke.yml (default: ./.shipsmoke.yml)")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "validate", err)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "shipsmoke validate: unexpected argument %q\n", fs.Arg(0))
		return engine.ExitUsage
	}
	p := *cfgPath
	if p == "" {
		p = config.DefaultConfigName
	}
	cfg, err := config.Load(p)
	if err != nil {
		fmt.Fprintf(stderr, "shipsmoke validate: FAILED\n%v\n", err)
		return engine.ExitUsage
	}
	total := 0
	for _, a := range cfg.Artifacts {
		total += len(a.Checks)
	}
	fmt.Fprintf(stdout, "shipsmoke validate: OK — %d artifacts, %d checks configured\n", len(cfg.Artifacts), total)
	return 0
}

// --- init ---

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "overwrite existing generated files")
	ci := fs.Bool("ci", false, "generate a GitHub Actions workflow (.github/workflows/shipsmoke.yml) plus .shipsmoke.yml")
	stackFlag := fs.String("stack", "", "with --ci: which build stack to generate (go | rust | c) — shipsmoke does not infer build systems")
	nameFlag := fs.String("name", "", "with --ci: your tool's binary name (defaults to the current directory's name)")
	if err := fs.Parse(args); err != nil {
		return usageError(stderr, "init", err)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "shipsmoke init: unexpected argument %q"+"\n", fs.Arg(0))
		return engine.ExitUsage
	}
	if !*ci {
		path := config.DefaultConfigName
		if _, err := os.Stat(path); err == nil && !*force {
			fmt.Fprintf(stderr, "shipsmoke init: %s already exists (use --force to overwrite)"+"\n", path)
			return engine.ExitUsage
		}
		if err := os.WriteFile(path, []byte(config.ExampleYAML), 0o644); err != nil {
			fmt.Fprintf(stderr, "shipsmoke init: %v"+"\n", err)
			return engine.ExitUsage
		}
		fmt.Fprintf(stdout, "wrote %s — edit the artifacts section to point at your release artifacts."+"\n", path)
		return 0
	}

	// --ci mode: generate a complete, pre-verified CI setup for an explicitly
	// named stack. ShipSmoke does not infer arbitrary build systems.
	stack, err := cigen.ParseStack(*stackFlag)
	if err != nil {
		fmt.Fprintf(stderr, "shipsmoke init: %v"+"\n", err)
		return engine.ExitUsage
	}
	name := *nameFlag
	if name == "" {
		wd, _ := os.Getwd()
		name = filepath.Base(wd)
	}
	if err := cigen.ValidateName(name); err != nil {
		fmt.Fprintf(stderr, "shipsmoke init: %v (pass --name to set the tool's binary name)"+"\n", err)
		return engine.ExitUsage
	}
	cfgYAML, wfYAML, err := cigen.Generate(stack, name)
	if err != nil {
		fmt.Fprintf(stderr, "shipsmoke init: %v"+"\n", err)
		return engine.ExitUsage
	}
	wfDir := filepath.Join(".github", "workflows")
	wfPath := filepath.Join(wfDir, "shipsmoke.yml")
	for _, pair := range []struct {
		path string
		what string
	}{
		{config.DefaultConfigName, "config"},
		{wfPath, "workflow"},
	} {
		if _, err := os.Stat(pair.path); err == nil && !*force {
			fmt.Fprintf(stderr, "shipsmoke init: %s already exists (use --force to overwrite)"+"\n", pair.path)
			return engine.ExitUsage
		}
	}
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "shipsmoke init: %v"+"\n", err)
		return engine.ExitUsage
	}
	if err := os.WriteFile(config.DefaultConfigName, []byte(cfgYAML), 0o644); err != nil {
		fmt.Fprintf(stderr, "shipsmoke init: %v"+"\n", err)
		return engine.ExitUsage
	}
	if err := os.WriteFile(wfPath, []byte(wfYAML), 0o644); err != nil {
		fmt.Fprintf(stderr, "shipsmoke init: %v"+"\n", err)
		return engine.ExitUsage
	}
	fmt.Fprintf(stdout,
		"generated files for the %[1]s stack (tool %[2]q):\n"+
			"  %[3]s          — verification config (expects %[2]s --version to print %[4]s)\n"+
			"  %[5]s — three-platform CI: build, package, shipsmoke gate\n"+
			"\n"+
			"Next steps:\n"+
			"  1. Commit both files and push; the workflow runs on ubuntu-24.04, macos-15 and windows-2025.\n"+
			"  2. The generated checks expect %[6]q to exit 0 — adjust .shipsmoke.yml if your CLI differs.\n"+
			"  3. Extend the build step if your release packages more files.\n"+
			"Supported --ci stacks are exactly: go, rust, c. For other build systems copy\n"+
			".github/workflows/example-*.yml from the shipsmoke repository and adapt it.\n",
		stack, name, config.DefaultConfigName,
		`"`+name+` version"`, wfPath, name+" --version")
	return 0
}

func usageError(stderr io.Writer, cmd string, err error) int {
	fmt.Fprintf(stderr, "shipsmoke %s: %v\n", cmd, err)
	return engine.ExitUsage
}

// isTerminal reports whether f is an interactive terminal (for color).
func isTerminal(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// guessVersion is used when the binary was not built with ldflags.
func guessVersion() string { return "0.0.0-dev" }

// SanitizeVersion replaces an empty injected version with the dev fallback.
func SanitizeVersion(v string) string {
	if strings.TrimSpace(v) == "" {
		return guessVersion()
	}
	return v
}

// ConfigPathHint returns the absolute path of the default config for messages.
func ConfigPathHint() string {
	abs, err := filepath.Abs(config.DefaultConfigName)
	if err != nil {
		return config.DefaultConfigName
	}
	return abs
}
