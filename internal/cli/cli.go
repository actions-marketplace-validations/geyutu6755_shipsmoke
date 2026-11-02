// Package cli implements the shipsmoke command line interface.
package cli

import (
	"errors"
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
  init       write .shipsmoke.yml; --ci adds a workflow
               --ci --stack go|rust|c   pre-publish gate: build, package, verify
               --ci --published         post-release gate: download, then run
  version    show version information

Run "shipsmoke <command> --help" for the flags of one command.

Exit codes:
  0  all executed checks passed
  1  at least one check failed
  2  usage or configuration error
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

// --- usage helpers ---

// The flag package writes its own error text and usage to this writer; shipsmoke
// renders both itself so the wording stays consistent across subcommands and the
// two writers can be injected in tests. io.Discard directly, not a package
// variable: a shared mutable sink would let one test change another's output.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// renderUsage writes the generated help for one subcommand.
func renderUsage(w io.Writer, name, description string, fs *flag.FlagSet, examples []string) {
	fmt.Fprintf(w, "shipsmoke %s — %s\n\nUsage:\n  shipsmoke %s [flags]\n\nFlags:\n", name, description, name)
	fs.VisitAll(func(f *flag.Flag) {
		typ, _ := flag.UnquoteUsage(f)
		arg := "  --" + f.Name
		if typ != "" {
			arg += " " + typ
		}
		fmt.Fprintf(w, "%s\n", arg)
		// A bool's default is false and reads as noise; the description text
		// already carries any non-obvious default (e.g. "default: exit 3").
		if typ != "" && f.DefValue != "" {
			fmt.Fprintf(w, "      %s (default: %s)\n", f.Usage, f.DefValue)
			return
		}
		fmt.Fprintf(w, "      %s\n", f.Usage)
	})
	if len(examples) > 0 {
		fmt.Fprint(w, "\nExamples:\n")
		for _, e := range examples {
			fmt.Fprintf(w, "  %s\n", e)
		}
	}
	fmt.Fprint(w, "\nExit codes: 0 all executed checks passed, 1 a check failed,\n")
	fmt.Fprint(w, "              2 usage or configuration error, 3 nothing was executed.\n")
}

// parseFailure turns a flag.Parse result into the right output stream and exit
// code. -h/--help is a request, not an error: usage goes to stdout with exit 0
// so it can be piped and read. Anything else is a genuine usage error.
func parseFailure(name string, err error, stdout, stderr io.Writer, usage func(io.Writer)) int {
	if errors.Is(err, flag.ErrHelp) {
		usage(stdout)
		return 0
	}
	fmt.Fprintf(stderr, "shipsmoke %s: %v\n", name, err)
	usage(stderr)
	return engine.ExitUsage
}

// --- check ---

const checkDescription = "run verification (the release quality gate)"

var checkExamples = []string{
	"shipsmoke check",
	"shipsmoke check --config .shipsmoke.yml --artifact linux-tarball",
	"shipsmoke check --quiet --no-color          # failures and the summary only",
	"shipsmoke check --report dist/report.json",
}

func newCheckFlagSet() (*flag.FlagSet, *checkOpts) {
	fs := newFlagSet("check")
	o := &checkOpts{}
	fs.StringVar(&o.ConfigPath, "config", "", "path to .shipsmoke.yml (default: ./.shipsmoke.yml)")
	fs.StringVar(&o.Artifact, "artifact", "", "verify only the artifact with this name")
	fs.StringVar(&o.ReportPath, "report", "", "write the JSON report to this path (overrides config)")
	fs.BoolVar(&o.AllowEmpty, "allow-empty", false, "exit 0 even when no check was executed (default: exit 3)")
	fs.BoolVar(&o.NoColor, "no-color", false, "disable colored terminal output")
	fs.BoolVar(&o.Quiet, "quiet", false, "only print failures, skips and the summary")
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
		return parseFailure("check", err, stdout, stderr, func(w io.Writer) {
			renderUsage(w, "check", checkDescription, fs, checkExamples)
		})
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
		Stderr:          stderr,
	}

	rep, exit := engine.Run(cfg, opts)

	// Terminal report. --quiet prints only non-passing results and the summary;
	// the summary is already computed by engine.Run, so nothing is rendered
	// through the full report path in that mode.
	useColor := !o.NoColor && isTerminal(os.Stdout)
	if o.Quiet {
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

const validateDescription = "validate .shipsmoke.yml without running anything"

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("validate")
	cfgPath := fs.String("config", "", "path to .shipsmoke.yml (default: ./.shipsmoke.yml)")
	if err := fs.Parse(args); err != nil {
		return parseFailure("validate", err, stdout, stderr, func(w io.Writer) {
			renderUsage(w, "validate", validateDescription, fs, []string{
				"shipsmoke validate",
				"shipsmoke validate --config release/.shipsmoke.yml",
			})
		})
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

const initDescription = "write .shipsmoke.yml, optionally with a GitHub Actions workflow"

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("init")
	force := fs.Bool("force", false, "overwrite existing generated files")
	ci := fs.Bool("ci", false, "generate a GitHub Actions workflow (.github/workflows/shipsmoke.yml) plus .shipsmoke.yml")
	published := fs.Bool("published", false, "with --ci: generate the post-release gate (re-download the published asset and run it) instead of the pre-publish build gate")
	stackFlag := fs.String("stack", "", "with --ci: which build stack to generate (go | rust | c) — shipsmoke does not infer build systems")
	nameFlag := fs.String("name", "", "with --ci: your tool's binary name (defaults to the current directory's name)")
	assetLinux := fs.String("asset-linux", "", "with --published: release asset name for linux/amd64 (e.g. mytool-linux-amd64.tar.gz)")
	assetDarwin := fs.String("asset-darwin", "", "with --published: release asset name for darwin/arm64")
	assetWindows := fs.String("asset-windows", "", "with --published: release asset name for windows/amd64")
	if err := fs.Parse(args); err != nil {
		return parseFailure("init", err, stdout, stderr, func(w io.Writer) {
			renderUsage(w, "init", initDescription, fs, []string{
				"shipsmoke init",
				"shipsmoke init --ci --stack go --name mytool",
				"shipsmoke init --ci --published --name mytool \\",
				"    --asset-linux mytool-linux-amd64.tar.gz \\",
				"    --asset-darwin mytool-darwin-arm64.tar.gz \\",
				"    --asset-windows mytool-windows-amd64.zip",
			})
		})
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "shipsmoke init: unexpected argument %q"+"\n", fs.Arg(0))
		return engine.ExitUsage
	}
	if *published && !*ci {
		fmt.Fprintf(stderr, "shipsmoke init: --published only applies with --ci (it selects the post-release gate instead of the pre-publish build gate)"+"\n")
		return engine.ExitUsage
	}
	if *published && *stackFlag != "" {
		// Silently dropping --stack would let a user believe they generated a
		// build gate while they generated a download gate.
		fmt.Fprintf(stderr, "shipsmoke init: --published does not build anything, so --stack %q has no meaning here (drop --stack, or drop --published)"+"\n", *stackFlag)
		return engine.ExitUsage
	}
	if !*published {
		// Not fatal, but a silently ignored flag is how people end up with a
		// config that verifies nothing.
		for _, given := range []struct{ flag, value string }{
			{"--asset-linux", *assetLinux},
			{"--asset-darwin", *assetDarwin},
			{"--asset-windows", *assetWindows},
		} {
			if given.value != "" {
				fmt.Fprintf(stderr, "shipsmoke init: %s is only used with --published (ignored here)"+"\n", given.flag)
			}
		}
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

	// --published mode: the post-release gate. It needs no build stack because
	// it never builds anything — it downloads what is already published.
	if *published {
		pname := *nameFlag
		if pname == "" {
			wd, _ := os.Getwd()
			pname = filepath.Base(wd)
		}
		if err := cigen.ValidateName(pname); err != nil {
			fmt.Fprintf(stderr, "shipsmoke init: %v (pass --name to set the tool's binary name)"+"\n", err)
			return engine.ExitUsage
		}
		return initPublished(pname, *force, *assetLinux, *assetDarwin, *assetWindows, stdout, stderr)
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

// initPublished writes the post-release gate: config plus a release:published
// workflow. It emits no build step, because the gate never builds anything.
func initPublished(name string, force bool, assetLinux, assetDarwin, assetWindows string, stdout, stderr io.Writer) int {
	assets := cigen.PublishedAssets{Linux: assetLinux, Darwin: assetDarwin, Windows: assetWindows}
	cfgYAML, wfYAML, err := cigen.GeneratePublished(name, assets)
	if err != nil {
		fmt.Fprintf(stderr, "shipsmoke init: %v"+"\n", err)
		return engine.ExitUsage
	}
	wfDir := filepath.Join(".github", "workflows")
	wfPath := filepath.Join(wfDir, "shipsmoke-published.yml")
	for _, existing := range []string{config.DefaultConfigName, wfPath} {
		if _, err := os.Stat(existing); err == nil && !force {
			fmt.Fprintf(stderr, "shipsmoke init: %s already exists (use --force to overwrite)"+"\n", existing)
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
		"generated the POST-release gate for tool %q:\n"+
			"  %s                    — one artifact per platform, pointing at your release asset names\n"+
			"  %s — runs on release:published: download the published asset, then run it\n"+
			"\n"+
			"This gate does not build anything and does not block publishing; it detects\n"+
			"a release that is already out and broken. For the pre-publish gate that blocks\n"+
			"a bad release, use: shipsmoke init --ci --stack go|rust|c --name %s\n"+
			"\n"+
			"Next steps:\n"+
			"  1. Commit both files. The gate runs when the next Release is published.\n"+
			"  2. To try it against an existing release without publishing:\n"+
			"       gh workflow run shipsmoke-published.yml --ref <branch> -f tag=<existing-tag>\n"+
			"  3. Adjust \"binary\" in %s if your archive nests the executable in a directory.\n",
		name, config.DefaultConfigName, wfPath, name, config.DefaultConfigName)
	if cigen.HasPlaceholder(assets.Resolve()) {
		fmt.Fprintf(stdout,
			"\nWARNING: at least one asset name is still a placeholder (SET-ME). The generated\n"+
				"workflow refuses to run those entries rather than passing vacuously — set\n"+
				"--asset-linux / --asset-darwin / --asset-windows, or edit the paths in %s.\n",
			config.DefaultConfigName)
	}
	return 0
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
