package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// WriteJSON writes the machine-readable report, fully redacted.
func WriteJSON(w io.Writer, r *Report, red *Redactor) error {
	// Redact by round-tripping through a sanitized copy: JSON of the struct
	// with all strings passed through the redactor.
	data, err := json.MarshalIndent(redactReport(r, red), "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// WriteJSONFile writes the report to path (creating parent dirs).
func WriteJSONFile(path string, r *Report, red *Redactor) error {
	if dir := parentDir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("cannot create report directory: %w", err)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create report file: %w", err)
	}
	defer f.Close()
	return WriteJSON(f, r, red)
}

func redactReport(r *Report, red *Redactor) *Report {
	cp := *r
	cp.Artifacts = make([]ArtifactResult, len(r.Artifacts))
	for i, a := range r.Artifacts {
		ca := a
		ca.Error = red.Apply(a.Error)
		ca.SkipReason = red.Apply(a.SkipReason)
		ca.Checks = make([]CheckResult, len(a.Checks))
		for j, c := range a.Checks {
			cc := c
			cc.Name = red.Apply(c.Name)
			cc.Args = redactAll(c.Args, red)
			cc.Stdout = red.Apply(c.Stdout)
			cc.Stderr = red.Apply(c.Stderr)
			cc.Reasons = redactAll(c.Reasons, red)
			cc.Reproduce = red.Apply(c.Reproduce)
			ca.Checks[j] = cc
		}
		cp.Artifacts[i] = ca
	}
	return &cp
}

func redactAll(ss []string, red *Redactor) []string {
	if len(ss) == 0 {
		return nil
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = red.Apply(s)
	}
	return out
}

func parentDir(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i < 0 {
		return ""
	}
	return p[:i]
}

// WriteGitHubSummary appends a markdown job summary to the file at path
// (typically $GITHUB_STEP_SUMMARY). Does nothing when path is empty.
func WriteGitHubSummary(path string, r *Report) error {
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open step summary: %w", err)
	}
	defer f.Close()
	return renderSummary(f, r)
}

func renderSummary(w io.Writer, r *Report) error {
	s := r.Summary
	fmt.Fprintf(w, "## ShipSmoke verification report\n\n")
	fmt.Fprintf(w, "**Overall: %s** — host `%s/%s`, config `%s`, shipsmoke %s\n\n",
		s.Overall, r.HostOS, r.HostArch, r.ConfigPath, r.ShipSmokeVersion)
	fmt.Fprintf(w, "| Artifact | Check | Status | Exit | Duration | Details |\n")
	fmt.Fprintf(w, "|---|---|---|---|---|---|\n")
	for _, a := range r.Artifacts {
		switch a.Decision {
		case "skipped":
			fmt.Fprintf(w, "| %s | — | ⏭️ SKIP | — | — | %s |\n", md(a.Name), md(a.SkipReason))
			continue
		case "error":
			fmt.Fprintf(w, "| %s | — | ❌ ERROR | — | — | %s |\n", md(a.Name), md(a.Error))
			continue
		}
		for _, c := range a.Checks {
			icon := map[string]string{
				StatusPass: "✅", StatusFail: "❌", StatusError: "❌", StatusSkip: "⏭️",
			}[c.Status]
			detail := strings.Join(c.Reasons, "; ")
			if c.TimedOut {
				detail = "timed out, process tree killed; " + detail
			}
			if detail == "" {
				detail = fmt.Sprintf("exit %d as expected", c.ExitCode)
			}
			fmt.Fprintf(w, "| %s | `%s` | %s %s | %d | %dms | %s |\n",
				md(a.Name), md(c.Name), icon, c.Status, c.ExitCode, c.DurationMS, md(detail))
		}
	}
	fmt.Fprintf(w, "\n**Totals:** %d passed, %d failed, %d skipped, %d errored\n",
		s.Passed, s.Failed, s.Skipped, s.Errored)
	// Include repro commands for failures.
	var repros []string
	for _, a := range r.Artifacts {
		for _, c := range a.Checks {
			if c.Status == StatusFail || c.Status == StatusError {
				repros = append(repros, "`"+c.Reproduce+"`")
			}
		}
	}
	if len(repros) > 0 {
		fmt.Fprintf(w, "\n**Reproduce locally:**\n")
		for _, rc := range repros {
			fmt.Fprintf(w, "- %s\n", rc)
		}
	}
	return nil
}

// md escapes pipes so markdown table cells stay intact.
func md(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
