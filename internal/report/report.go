// Package report renders verification results for humans, machines and
// GitHub Actions. It redacts secret values from everything it emits.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/geyutu6755/shipsmoke/internal/artifact"
)

// Status of a check or an overall run.
const (
	StatusPass     = "PASS"
	StatusFail     = "FAIL"
	StatusSkip     = "SKIP"
	StatusError    = "ERROR"
	StatusNoChecks = "NO-CHECKS"
)

// Redactor replaces known secret values in all report output.
type Redactor struct {
	values []string
}

// NewRedactor builds a redactor from environment variable values whose names
// look secret-shaped and whose values are long enough to be meaningful.
func NewRedactor() *Redactor {
	r := &Redactor{}
	for _, kv := range environ() {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		name, val := kv[:i], kv[i+1:]
		if len(val) < 8 {
			continue
		}
		if isSecretVar(name) {
			r.values = append(r.values, val)
		}
	}
	// Longest first so overlapping secrets redact fully.
	sort.Slice(r.values, func(i, j int) bool { return len(r.values[i]) > len(r.values[j]) })
	return r
}

// Add registers an additional secret value to redact.
func (r *Redactor) Add(secret string) {
	if len(secret) >= 8 {
		r.values = append(r.values, secret)
	}
}

// Apply masks every known secret in s.
func (r *Redactor) Apply(s string) string {
	for _, v := range r.values {
		if v != "" {
			s = strings.ReplaceAll(s, v, "***REDACTED***")
		}
	}
	return s
}

// CheckResult is the fully resolved result of one check.
type CheckResult struct {
	Index        int      `json:"index"`
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	Args         []string `json:"args"`
	ExpectedExit int      `json:"expected_exit_code"`
	ExitCode     int      `json:"exit_code"`
	Signal       string   `json:"signal,omitempty"`
	TimedOut     bool     `json:"timed_out,omitempty"`
	Stdout       string   `json:"stdout,omitempty"`
	Stderr       string   `json:"stderr,omitempty"`
	Reasons      []string `json:"failure_reasons,omitempty"`
	DurationMS   int64    `json:"duration_ms"`
	Reproduce    string   `json:"reproduce_command"`
	Truncated    bool     `json:"output_truncated,omitempty"`
}

// ArtifactResult is the result of verifying one artifact.
type ArtifactResult struct {
	Name           string        `json:"name"`
	Path           string        `json:"path"`
	Format         string        `json:"format"`
	Decision       string        `json:"decision"` // ran | skipped | error
	SkipReason     string        `json:"skip_reason,omitempty"`
	Error          string        `json:"error,omitempty"`
	DetectedOS     string        `json:"detected_os,omitempty"`
	DetectedArch   string        `json:"detected_arch,omitempty"`
	DetectedDetail string        `json:"detected_detail,omitempty"`
	RunsOn         []string      `json:"runs_on,omitempty"`
	ExtractedFiles int           `json:"extracted_files,omitempty"`
	Checks         []CheckResult `json:"checks"`
}

// Summary is the run-level rollup.
type Summary struct {
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
	Skipped  int    `json:"skipped"`
	Errored  int    `json:"errored"`
	Executed int    `json:"executed"`
	Overall  string `json:"overall"`
}

// Report is the complete machine-readable verification report.
type Report struct {
	ShipSmokeVersion string           `json:"shipsmoke_version"`
	ConfigPath       string           `json:"config_path"`
	StartedAt        string           `json:"started_at"`
	DurationMS       int64            `json:"duration_ms"`
	HostOS           string           `json:"host_os"`
	HostArch         string           `json:"host_arch"`
	Artifacts        []ArtifactResult `json:"artifacts"`
	Summary          Summary          `json:"summary"`
	ExitCode         int              `json:"exit_code"`
}

// ComputeSummary fills the Summary field from artifact results.
func (r *Report) ComputeSummary() {
	s := Summary{}
	for _, a := range r.Artifacts {
		switch a.Decision {
		case "error":
			s.Errored++
		case "skipped":
			s.Skipped++
		}
		for _, c := range a.Checks {
			switch c.Status {
			case StatusPass:
				s.Passed++
				s.Executed++
			case StatusFail:
				s.Failed++
				s.Executed++
			case StatusSkip:
				s.Skipped++
			case StatusError:
				s.Errored++
				s.Executed++
			}
		}
	}
	switch {
	case s.Failed > 0 || s.Errored > 0:
		s.Overall = StatusFail
	case s.Executed > 0:
		s.Overall = StatusPass
	default:
		s.Overall = StatusNoChecks
	}
	r.Summary = s
}

// Terminal renders the human-readable report to w. Colors are enabled when
// useColor is true.
func (r *Report) Terminal(w io.Writer, useColor bool, red *Redactor) {
	line := func(s string) { fmt.Fprintln(w, red.Apply(s)) }
	statusStr := func(st string) string {
		if !useColor {
			return st
		}
		switch st {
		case StatusPass:
			return "\033[32m" + st + "\033[0m"
		case StatusFail, StatusError:
			return "\033[31m" + st + "\033[0m"
		case StatusSkip:
			return "\033[33m" + st + "\033[0m"
		default:
			return st
		}
	}

	line("")
	line("shipsmoke " + r.ShipSmokeVersion + " — verification report")
	line(fmt.Sprintf("host: %s/%s   config: %s   duration: %dms",
		r.HostOS, r.HostArch, r.ConfigPath, r.DurationMS))
	line("")
	for _, a := range r.Artifacts {
		switch a.Decision {
		case "skipped":
			line(fmt.Sprintf("  [%s] %s — %s", statusStr(StatusSkip), a.Name, a.SkipReason))
			continue
		case "error":
			line(fmt.Sprintf("  [%s] %s — %s", statusStr(StatusError), a.Name, a.Error))
			continue
		}
		det := ""
		if a.DetectedOS != "" {
			det = fmt.Sprintf(" (detected: %s/%s)", a.DetectedOS, a.DetectedArch)
			if a.DetectedDetail != "" {
				det = fmt.Sprintf(" (detected: %s/%s — %s)", a.DetectedOS, a.DetectedArch, a.DetectedDetail)
			}
		}
		line(fmt.Sprintf("  %s  format=%s%s", a.Name, a.Format, det))
		for _, c := range a.Checks {
			line(fmt.Sprintf("    [%s] %s (%dms)", statusStr(c.Status), c.Name, c.DurationMS))
			for _, rsn := range c.Reasons {
				line("        • " + rsn)
			}
			if c.Status != StatusPass {
				if tail := lastLines(red.Apply(c.Stdout), 6); tail != "" {
					line("        stdout | " + strings.ReplaceAll(tail, "\n", "\n                 "))
				}
				if tail := lastLines(red.Apply(c.Stderr), 6); tail != "" {
					line("        stderr | " + strings.ReplaceAll(tail, "\n", "\n                 "))
				}
				line("        reproduce: " + c.Reproduce)
			}
		}
	}
	s := r.Summary
	line("")
	line(fmt.Sprintf("  summary: %d passed, %d failed, %d skipped, %d errored  →  %s",
		s.Passed, s.Failed, s.Skipped, s.Errored, statusStr(s.Overall)))
	if s.Executed == 0 && s.Overall == StatusNoChecks {
		line("  nothing was executed: every check was skipped. This fails by default")
		line("  so CI cannot go green on a misconfiguration (use --allow-empty to allow).")
	}
	line("")
}

func lastLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// ReproduceCommand builds the exact command a user can run to re-verify.
func ReproduceCommand(configPath, artifactName string, args []string) string {
	var b strings.Builder
	b.WriteString("shipsmoke check --config " + configPath)
	if artifactName != "" {
		b.WriteString(" --artifact " + artifactName)
	}
	if len(args) > 0 {
		b.WriteString("  # check args: " + strings.Join(quoteAll(args), " "))
	}
	return b.String()
}

func quoteAll(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\"") {
			out[i] = "\"" + strings.ReplaceAll(a, "\"", "\\\"") + "\""
		} else {
			out[i] = a
		}
	}
	return out
}

// HostDescription exposes the host platform for summaries.
func HostDescription() string { return artifact.HostDescription() }
