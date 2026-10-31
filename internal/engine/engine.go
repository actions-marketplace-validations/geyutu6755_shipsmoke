// Package engine orchestrates a full verification run: config → artifacts →
// checks → report.
package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/geyutu6755/shipsmoke/internal/artifact"
	"github.com/geyutu6755/shipsmoke/internal/config"
	"github.com/geyutu6755/shipsmoke/internal/report"
	"github.com/geyutu6755/shipsmoke/internal/runner"
)

// withSignalContext returns a context cancelled on Ctrl+C / SIGTERM so a
// running check is killed and reports still render.
func withSignalContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		defer signal.Stop(ch)
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}

// Options controls one verification run.
type Options struct {
	ConfigPath string
	Version    string
	// ArtifactFilter, when non-empty, runs only the named artifact.
	ArtifactFilter string
	// AllowEmpty prevents exit code 3 when nothing was executed.
	AllowEmpty bool
	// Env (override for tests): environment of the shipsmoke process.
	Env []string
	// GITHUB_STEP_SUMMARY path, when non-empty, receives the markdown summary.
	StepSummaryPath string
}

// Exit codes (documented in README and docs/SYSTEM_DESIGN.md).
const (
	ExitPass     = 0
	ExitFail     = 1
	ExitUsage    = 2
	ExitNoChecks = 3
)

// Run executes every configured artifact check and returns the report and
// the process exit code.
func Run(cfg *config.Config, opts Options) (*report.Report, int) {
	red := report.NewRedactor()
	rep := &report.Report{
		ShipSmokeVersion: opts.Version,
		ConfigPath:       opts.ConfigPath,
		StartedAt:        time.Now().UTC().Format(time.RFC3339),
		HostOS:           runtime.GOOS,
		HostArch:         runtime.GOARCH,
	}
	start := time.Now()

	for _, art := range cfg.Artifacts {
		if opts.ArtifactFilter != "" && art.Name != opts.ArtifactFilter {
			continue
		}
		rep.Artifacts = append(rep.Artifacts, verifyArtifact(art, cfg, opts, red))
	}
	rep.DurationMS = time.Since(start).Milliseconds()
	rep.ComputeSummary()

	exit := ExitPass
	switch rep.Summary.Overall {
	case report.StatusFail:
		exit = ExitFail
	case report.StatusNoChecks:
		exit = ExitNoChecks
		if opts.AllowEmpty {
			exit = ExitPass
		}
	}
	rep.ExitCode = exit
	return rep, exit
}

func verifyArtifact(art config.Artifact, cfg *config.Config, opts Options, red *report.Redactor) report.ArtifactResult {
	res := report.ArtifactResult{
		Name:   art.Name,
		Path:   art.Path,
		Format: art.Format,
		RunsOn: art.RunsOn,
	}

	// 1. Platform gate: run only where this artifact claims to run.
	if !artifact.RunsOnHost(art.RunsOn) {
		res.Decision = "skipped"
		res.SkipReason = fmt.Sprintf("artifact targets [%s] but host is %s/%s",
			artifact.FormatRunsOn(art.RunsOn), runtime.GOOS, runtime.GOARCH)
		return res
	}

	// 2. Artifact exists?
	if _, err := os.Stat(art.Path); err != nil {
		res.Decision = "error"
		res.Error = fmt.Sprintf("artifact file not found: %s", art.Path)
		return res
	}

	// 3. Classify container format by magic bytes.
	format, err := artifact.Classify(art.Path)
	if err != nil {
		res.Decision = "error"
		res.Error = err.Error()
		return res
	}
	res.Format = string(format)
	if format == artifact.FormatTar {
		res.Decision = "error"
		res.Error = "plain (uncompressed) tar is not supported: use tar.gz, zip, or a standalone binary"
		return res
	}
	if art.Format != "auto" && art.Format != string(format) {
		res.Decision = "error"
		res.Error = fmt.Sprintf("format mismatch: config says %q but the file is actually %q", art.Format, format)
		return res
	}

	// 4. Extract safely into a temp working directory.
	workRoot, err := os.MkdirTemp("", "shipsmoke-work-")
	if err != nil {
		res.Decision = "error"
		res.Error = fmt.Sprintf("cannot create temp directory: %v", err)
		return res
	}
	defer os.RemoveAll(workRoot)

	var extractDir string
	var files int
	if format == artifact.FormatBinary {
		extractDir = workRoot
		// For a standalone binary, copy it into the work dir so PATH setup
		// and cwd behave the same as archives.
		bin, err := os.ReadFile(art.Path)
		if err != nil {
			res.Decision = "error"
			res.Error = fmt.Sprintf("cannot read standalone binary: %v", err)
			return res
		}
		binName := filepath.Base(art.Path)
		binPath := filepath.Join(workRoot, binName)
		mode := os.FileMode(0o755)
		if runtime.GOOS == "windows" {
			mode = 0o644
		}
		if err := os.WriteFile(binPath, bin, mode); err != nil {
			res.Decision = "error"
			res.Error = fmt.Sprintf("cannot stage standalone binary: %v", err)
			return res
		}
		files = 1
	} else {
		var er *artifact.Result
		var err error
		if format == artifact.FormatZip {
			er, err = artifact.ExtractZip(art.Path, workRoot, limitsFrom(cfg))
		} else {
			er, err = artifact.ExtractTarGz(art.Path, workRoot, limitsFrom(cfg))
		}
		if err != nil {
			res.Decision = "error"
			res.Error = classifyExtractError(err)
			return res
		}
		extractDir = workRoot
		files = er.Files
	}
	res.ExtractedFiles = files

	// 5. Locate the entry binary.
	entryRel := art.Binary
	if format == artifact.FormatBinary && filepath.Base(art.Path) == art.Binary {
		entryRel = filepath.Base(art.Path) // standalone: the file itself
	}
	entryPath, err := artifact.FindEntry(extractDir, entryRel, art.Path, format)
	if err != nil {
		if format == artifact.FormatBinary {
			err = fmt.Errorf("%w (for a standalone binary, set binary to the artifact's own file name, e.g. %q)", err, filepath.Base(art.Path))
		}
		res.Decision = "ran" // the artifact was fine; the check failed
		res.Checks = append(res.Checks, report.CheckResult{
			Name:      "locate entry binary",
			Status:    report.StatusFail,
			Reasons:   []string{err.Error()},
			Reproduce: report.ReproduceCommand(opts.ConfigPath, art.Name, nil),
		})
		return res
	}

	// 6. Identify the binary from its real header.
	bi := artifact.DetectBinary(entryPath)
	res.DetectedOS = bi.OS
	res.DetectedArch = bi.Arch
	res.DetectedDetail = bi.Detail

	// 7. Platform mismatch diagnosis: declared target vs reality vs host.
	if msg := platformDiagnosis(art, bi); msg != "" {
		res.Decision = "ran"
		res.Checks = append(res.Checks, report.CheckResult{
			Name:      "platform identity",
			Status:    report.StatusFail,
			Reasons:   []string{msg},
			Reproduce: report.ReproduceCommand(opts.ConfigPath, art.Name, nil),
		})
		return res
	}

	// 8. Run every check.
	if art.Env == "inherit" {
		fmt.Fprintln(os.Stderr, "warning: artifact \""+art.Name+"\" uses env: inherit; the child process will see the parent environment (secret-shaped variables are still removed)")
	}
	for i, chk := range art.Checks {
		res.Checks = append(res.Checks, runCheck(art, chk, i, entryPath, extractDir, opts, red))
	}
	res.Decision = "ran"
	return res
}

// platformDiagnosis returns a failure message when declared target, binary
// header reality, and host platform disagree. Empty means "consistent".
func platformDiagnosis(art config.Artifact, bi artifact.BinaryInfo) string {
	host := artifact.HostDescription()
	// Binary header vs host.
	if bi.Known() && (bi.OS != runtime.GOOS || bi.Arch != runtime.GOARCH) {
		return fmt.Sprintf("platform mismatch: this artifact runs %s binaries but the binary header says %s; host is %s. The check cannot run here — verify this artifact on the right runner.",
			host, bi.OS+"/"+bi.Arch, host)
	}
	// Declared runs_on vs binary reality.
	if bi.Known() && len(art.RunsOn) > 0 {
		for _, spec := range art.RunsOn {
			if spec == "any" {
				continue
			}
			parts := strings.SplitN(spec, "/", 2)
			if len(parts) == 2 && (parts[0] != bi.OS || parts[1] != bi.Arch) {
				return fmt.Sprintf("declaration mismatch: runs_on says %q but the binary header identifies %s", spec, bi.OS+"/"+bi.Arch)
			}
		}
	}
	return ""
}

func classifyExtractError(err error) string {
	switch {
	case errors.Is(err, artifact.ErrLimitHit):
		return "archive rejected by safety limits: " + err.Error()
	case errors.Is(err, artifact.ErrUnsafeEntry):
		return "archive contains an unsafe entry: " + err.Error()
	default:
		return "archive could not be extracted safely: " + err.Error()
	}
}

func limitsFrom(cfg *config.Config) artifact.Limits {
	return artifact.Limits{
		MaxFiles:            cfg.Safety.MaxFiles,
		MaxTotalSize:        cfg.Safety.MaxTotalSize,
		MaxFileSize:         cfg.Safety.MaxFileSize,
		MaxCompressionRatio: cfg.Safety.MaxCompressionRatio,
	}
}

func runCheck(art config.Artifact, chk config.Check, idx int, entryPath, workDir string, opts Options, red *report.Redactor) report.CheckResult {
	name := fmt.Sprintf("check #%d: %s %s", idx+1, filepath.Base(entryPath), strings.Join(chk.Args, " "))
	out := runner.Run(withSignalContext(), runner.Spec{
		BinaryPath: entryPath,
		Args:       chk.Args,
		Timeout:    chk.Timeout.Duration,
		WorkDir:    workDir,
		Env:        runner.EnvMode(art.Env),
		EnvExtra:   art.EnvExtra,
	})

	cr := report.CheckResult{
		Index:        idx + 1,
		Name:         name,
		Args:         chk.Args,
		ExpectedExit: chk.Expect.ExitCode,
		ExitCode:     out.ExitCode,
		Signal:       out.Signal,
		TimedOut:     out.TimedOut,
		Stdout:       out.Stdout,
		Stderr:       out.Stderr,
		DurationMS:   out.Duration.Milliseconds(),
		Truncated:    out.Truncated,
	}
	cr.Reproduce = report.ReproduceCommand(opts.ConfigPath, art.Name, chk.Args)

	pass, reasons := runner.Evaluate(out, chk.Expect)
	cr.Status = report.StatusPass
	if !pass {
		cr.Status = report.StatusFail
	}
	cr.Reasons = reasons
	return cr
}
