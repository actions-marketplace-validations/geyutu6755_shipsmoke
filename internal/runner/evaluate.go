package runner

import (
	"fmt"
	"strings"

	"github.com/geyutu6755/shipsmoke/internal/config"
)

// Evaluate applies the configured expectations to an outcome. The returned
// reasons are human-readable and safe to print.
func Evaluate(out *Outcome, expect config.Expect) (pass bool, reasons []string) {
	if out.SpawnErr != nil {
		return false, []string{spawnDiagnosis(out.SpawnErr)}
	}
	if out.TimedOut {
		return false, []string{fmt.Sprintf("timed out after %s (process tree killed)", formatDur(out.Duration))}
	}
	fail := func(r string) {
		reasons = append(reasons, r)
	}

	if out.Signal != "" {
		fail(fmt.Sprintf("process was killed by signal %s (exit code unreliable)", out.Signal))
	} else if out.ExitCode != expect.ExitCode {
		fail(fmt.Sprintf("expected exit code %d, got %d", expect.ExitCode, out.ExitCode))
	}

	stdout := normalize(out.Stdout)
	stderr := normalize(out.Stderr)

	for _, sub := range expect.StdoutContains {
		if !strings.Contains(stdout, sub) {
			fail(fmt.Sprintf("stdout does not contain %q", elide(sub)))
		}
	}
	for _, sub := range expect.StdoutNotContains {
		if strings.Contains(stdout, sub) {
			fail(fmt.Sprintf("stdout must not contain %q but does", elide(sub)))
		}
	}
	for _, sub := range expect.StderrContains {
		if !strings.Contains(stderr, sub) {
			fail(fmt.Sprintf("stderr does not contain %q", elide(sub)))
		}
	}
	for _, sub := range expect.StderrNotContains {
		if strings.Contains(stderr, sub) {
			fail(fmt.Sprintf("stderr must not contain %q but does", elide(sub)))
		}
	}
	if expect.StdoutEquals != "" && stdout != normalize(expect.StdoutEquals) {
		fail(fmt.Sprintf("stdout does not equal %q", elide(expect.StdoutEquals)))
	}
	if expect.StderrEquals != "" && stderr != normalize(expect.StderrEquals) {
		fail(fmt.Sprintf("stderr does not equal %q", elide(expect.StderrEquals)))
	}
	if expect.StdoutRegex != "" {
		re, err := config.CompileExpectRegexes(expect.StdoutRegex)
		if err != nil {
			fail(fmt.Sprintf("internal: stdout_regex failed to compile: %v", err))
		} else if !re.MatchString(stdout) {
			fail(fmt.Sprintf("stdout does not match regex %q", elide(expect.StdoutRegex)))
		}
	}
	if expect.StderrRegex != "" {
		re, err := config.CompileExpectRegexes(expect.StderrRegex)
		if err != nil {
			fail(fmt.Sprintf("internal: stderr_regex failed to compile: %v", err))
		} else if !re.MatchString(stderr) {
			fail(fmt.Sprintf("stderr does not match regex %q", elide(expect.StderrRegex)))
		}
	}

	return len(reasons) == 0, reasons
}

// spawnDiagnosis turns a start failure into something the user can act on.
func spawnDiagnosis(err error) string {
	base := err.Error()
	switch {
	case strings.Contains(base, "no such file") || strings.Contains(base, "cannot find the file") ||
		strings.Contains(base, "The system cannot find"):
		return "could not start the entry binary: file not found (was the artifact extracted? is the binary path right?)"
	case strings.Contains(base, "permission denied") || strings.Contains(base, "Access is denied"):
		return "could not start the entry binary: permission denied (is it executable on this platform?)"
	case strings.Contains(base, "exec format error") || strings.Contains(base, "not a valid Win32 application") ||
		strings.Contains(base, "Bad EXE format") || strings.Contains(base, "inappropriate") ||
		strings.Contains(base, "unknown error"):
		return "could not start the entry binary: format not executable on this platform (wrong OS/arch binary?)"
	default:
		return "could not start the entry binary: " + base
	}
}

// normalize makes output comparison platform-stable: CRLF → LF.
func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func elide(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

func formatDur(d interface{ String() string }) string { return d.String() }
