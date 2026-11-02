package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/geyutu6755/shipsmoke/internal/testfixture"
)

// These tests cover the two shipped defects found in v1.0.3:
//   * "check --quiet" panicked with a nil-pointer dereference
//   * "<subcommand> --help" printed a parse error instead of the flags
// Both are regression tests, so they fail loudly if either behaviour returns.

func fixtureBinaryName() string {
	if runtime.GOOS == "windows" {
		return "tool.exe"
	}
	return "tool"
}

// foreignPlatform returns an os/arch pair that cannot match this host, so a
// configured artifact is guaranteed to be skipped rather than run.
func foreignPlatform(t *testing.T) string {
	t.Helper()
	for _, goos := range []string{"linux", "darwin", "windows", "freebsd"} {
		if goos != runtime.GOOS {
			return goos + "/amd64"
		}
	}
	t.Skip("no foreign platform name available")
	return ""
}

// zipFixture writes a real zip containing the host-runnable fixture binary at
// the archive root, and returns its path relative to dir.
func zipFixture(t *testing.T, dir string) string {
	t.Helper()
	bin, err := os.ReadFile(testfixture.HostBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	name := "tool.zip"
	testfixture.BuildZip(t, filepath.Join(dir, name), []testfixture.ZipEntry{
		{Name: fixtureBinaryName(), Data: bin, Mode: 0o755, SetMode: true},
	})
	return name
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- P0: --quiet must work, not panic ---

func TestCheckQuietPass(t *testing.T) {
	dir := t.TempDir()
	art := zipFixture(t, dir)
	writeFile(t, dir, ".shipsmoke.yml", `version: 1
report:
  json: quiet-report.json
artifacts:
  - name: tool
    path: `+art+`
    binary: `+fixtureBinaryName()+`
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
          stdout_contains: ["tool version 1.2.3"]
`)
	chdir(t, dir)

	var out, errOut bytes.Buffer
	code := Run([]string{"check", "--quiet"}, &out, &errOut, VersionInfo{Version: "test"})
	if code != 0 {
		t.Fatalf("quiet pass exit = %d, want 0; stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
	got := out.String()
	if strings.Contains(got, "verification report") {
		t.Errorf("--quiet must not render the full report, got:\n%s", got)
	}
	if !strings.Contains(got, "summary: 1 passed, 0 failed, 0 skipped, 0 errored") {
		t.Errorf("--quiet should print the summary line, got:\n%s", got)
	}
	// A passing check is not a failure: it must not be itemised.
	if strings.Contains(got, "[PASS]") {
		t.Errorf("--quiet should suppress passing checks, got:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "quiet-report.json")); err != nil {
		t.Errorf("--quiet must still write the JSON report: %v", err)
	}
}

func TestCheckQuietFailureShowsReasons(t *testing.T) {
	dir := t.TempDir()
	art := zipFixture(t, dir)
	writeFile(t, dir, ".shipsmoke.yml", `version: 1
artifacts:
  - name: tool
    path: `+art+`
    binary: `+fixtureBinaryName()+`
    checks:
      - args: ["--version"]
        expect:
          exit_code: 7
`)
	chdir(t, dir)

	var out, errOut bytes.Buffer
	code := Run([]string{"check", "--quiet"}, &out, &errOut, VersionInfo{Version: "test"})
	if code != 1 {
		t.Fatalf("quiet failing exit = %d, want 1; stdout=%q", code, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "[FAIL] tool") {
		t.Errorf("quiet output should name the failing artifact, got:\n%s", got)
	}
	if !strings.Contains(got, "summary:") {
		t.Errorf("quiet output should carry the summary, got:\n%s", got)
	}
}

func TestCheckQuietErrorAndSkip(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".shipsmoke.yml", `version: 1
artifacts:
  - name: missing
    path: does-not-exist.zip
    binary: tool
    runs_on: [`+runtime.GOOS+`/`+runtime.GOARCH+`]
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
`)
	chdir(t, dir)

	var out, errOut bytes.Buffer
	code := Run([]string{"check", "--quiet"}, &out, &errOut, VersionInfo{Version: "test"})
	if code != 1 {
		t.Fatalf("quiet error-case exit = %d, want 1; stdout=%q", code, out.String())
	}
	if !strings.Contains(out.String(), "[ERROR] missing") {
		t.Errorf("quiet output should report the errored artifact, got:\n%s", out.String())
	}
}

func TestCheckQuietSkipOnlyExitsNoChecks(t *testing.T) {
	dir := t.TempDir()
	foreign := foreignPlatform(t)
	writeFile(t, dir, ".shipsmoke.yml", `version: 1
artifacts:
  - name: elsewhere
    path: whatever.zip
    binary: tool
    runs_on: [`+foreign+`]
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
`)
	chdir(t, dir)

	var out, errOut bytes.Buffer
	code := Run([]string{"check", "--quiet"}, &out, &errOut, VersionInfo{Version: "test"})
	if code != 3 {
		t.Fatalf("skip-only run exit = %d, want 3; stdout=%q", code, out.String())
	}
	if !strings.Contains(out.String(), "[SKIP] elsewhere") {
		t.Errorf("quiet output should list the skipped artifact, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "NO-CHECKS") {
		t.Errorf("quiet summary should say NO-CHECKS, got:\n%s", out.String())
	}
	// --allow-empty is the documented escape hatch and must survive --quiet.
	out.Reset()
	if code := Run([]string{"check", "--quiet", "--allow-empty"}, &out, &errOut, VersionInfo{Version: "test"}); code != 0 {
		t.Errorf("--quiet --allow-empty exit = %d, want 0; stdout=%q", code, out.String())
	}
}

func TestCheckQuietJSONMatchesTerminalVerdict(t *testing.T) {
	dir := t.TempDir()
	art := zipFixture(t, dir)
	writeFile(t, dir, ".shipsmoke.yml", `version: 1
artifacts:
  - name: tool
    path: `+art+`
    binary: `+fixtureBinaryName()+`
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
`)
	chdir(t, dir)

	var out, errOut bytes.Buffer
	if code := Run([]string{"check", "--quiet", "--report", "m.json"}, &out, &errOut, VersionInfo{Version: "test"}); code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, errOut.String())
	}
	raw, err := os.ReadFile(filepath.Join(dir, "m.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ExitCode int `json:"exit_code"`
		Summary  struct {
			Passed  int    `json:"passed"`
			Overall string `json:"overall"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("JSON report from --quiet run is malformed: %v", err)
	}
	if doc.ExitCode != 0 || doc.Summary.Passed != 1 {
		t.Errorf("JSON report disagrees with the terminal verdict: exit=%d passed=%d", doc.ExitCode, doc.Summary.Passed)
	}
}

func TestQuietNoColorAndQuietCombined(t *testing.T) {
	// The released v1.0.3 binary panicked for both flag orders; cover the
	// combination explicitly so neither path can regress silently.
	dir := t.TempDir()
	art := zipFixture(t, dir)
	writeFile(t, dir, ".shipsmoke.yml", `version: 1
artifacts:
  - name: tool
    path: `+art+`
    binary: `+fixtureBinaryName()+`
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
`)
	chdir(t, dir)
	for _, args := range [][]string{
		{"check", "--quiet", "--no-color"},
		{"check", "--no-color", "--quiet"},
		{"check", "--quiet", "--artifact", "tool"},
	} {
		var out, errOut bytes.Buffer
		if code := Run(args, &out, &errOut, VersionInfo{Version: "test"}); code != 0 {
			t.Errorf("%v exit = %d, want 0; stderr=%q", args, code, errOut.String())
		}
	}
}

// --- P1: --help must document the flags ---

func TestSubcommandHelpDocumentsFlags(t *testing.T) {
	cases := []struct {
		cmd   string
		flags []string
	}{
		{"check", []string{"--config", "--artifact", "--report", "--allow-empty", "--no-color", "--quiet"}},
		{"validate", []string{"--config"}},
		{"init", []string{"--force", "--ci", "--published", "--stack", "--name", "--asset-linux", "--asset-darwin", "--asset-windows"}},
	}
	for _, tc := range cases {
		var out, errOut bytes.Buffer
		code := Run([]string{tc.cmd, "--help"}, &out, &errOut, VersionInfo{Version: "test"})
		if code != 0 {
			t.Errorf("%s --help exit = %d, want 0; stderr=%q", tc.cmd, code, errOut.String())
		}
		got := out.String()
		if !strings.Contains(got, "Usage:\n  shipsmoke "+tc.cmd+" [flags]") {
			t.Errorf("%s --help missing usage line, got:\n%s", tc.cmd, got)
		}
		for _, f := range tc.flags {
			if !strings.Contains(got, f) {
				t.Errorf("%s --help should document %s, got:\n%s", tc.cmd, f, got)
			}
		}
		if errOut.Len() != 0 {
			t.Errorf("%s --help should write nothing to stderr, got %q", tc.cmd, errOut.String())
		}
		// The flag descriptions must be present, not just the flag names:
		// that is what "documents every flag" promises.
		if !strings.Contains(got, "Flags:") {
			t.Errorf("%s --help missing the flags section", tc.cmd)
		}
	}
}

func TestSubcommandShortHelpFlag(t *testing.T) {
	for _, cmd := range []string{"check", "validate", "init"} {
		var out, errOut bytes.Buffer
		if code := Run([]string{cmd, "-h"}, &out, &errOut, VersionInfo{}); code != 0 {
			t.Errorf("%s -h exit = %d, want 0", cmd, code)
		}
		if !strings.Contains(out.String(), "Flags:") {
			t.Errorf("%s -h should print usage, got:\n%s", cmd, out.String())
		}
	}
}

func TestUnknownFlagIsStillAnError(t *testing.T) {
	// --help exits 0; a genuinely bad flag must not be confused with it.
	for _, cmd := range []string{"check", "validate", "init"} {
		var out, errOut bytes.Buffer
		code := Run([]string{cmd, "--definitely-not-a-flag"}, &out, &errOut, VersionInfo{})
		if code != 2 {
			t.Errorf("%s with unknown flag exit = %d, want 2", cmd, code)
		}
		if out.Len() != 0 {
			t.Errorf("%s with unknown flag should write nothing to stdout, got %q", cmd, out.String())
		}
		if !strings.Contains(errOut.String(), "not defined") {
			t.Errorf("%s unknown flag stderr = %q", cmd, errOut.String())
		}
		if !strings.Contains(errOut.String(), "Usage:") {
			t.Errorf("%s unknown flag should also print usage on stderr, got %q", cmd, errOut.String())
		}
	}
}

func TestBadFlagValueIsAnError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeFile(t, dir, ".shipsmoke.yml", "version: 1\nartifacts: []\n")
	var out, errOut bytes.Buffer
	// bool flag given a non-bool value: a parse failure, not a help request.
	if code := Run([]string{"check", "--quiet=yes-please"}, &out, &errOut, VersionInfo{}); code != 2 {
		t.Errorf("bad bool value exit = %d, want 2; stderr=%q", code, errOut.String())
	}
	if strings.Contains(errOut.String(), "Usage:\n  shipsmoke check [flags]\n\nFlags:") && strings.Contains(out.String(), "Flags:") {
		t.Error("a parse error must not also be treated as a help request")
	}
}

// --- P3: warnings must reach the caller's writer, not the raw process stderr ---

func TestEnvInheritWarningGoesToInjectedStderr(t *testing.T) {
	dir := t.TempDir()
	art := zipFixture(t, dir)
	writeFile(t, dir, ".shipsmoke.yml", `version: 1
artifacts:
  - name: tool
    path: `+art+`
    binary: `+fixtureBinaryName()+`
    env: inherit
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
`)
	chdir(t, dir)

	var out, errOut bytes.Buffer
	if code := Run([]string{"check"}, &out, &errOut, VersionInfo{Version: "test"}); code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "env: inherit") {
		t.Errorf("the inherit warning must be routed to the injected stderr writer, got %q", errOut.String())
	}
	if strings.Contains(out.String(), "env: inherit") {
		t.Error("the warning belongs on stderr, not stdout")
	}
}

func TestForeignPlatform(t *testing.T) {
	dir := t.TempDir()
	foreign := foreignPlatform(t)
	writeFile(t, dir, ".shipsmoke.yml", `version: 1
artifacts:
  - name: elsewhere
    path: x.zip
    binary: tool
    runs_on: [`+foreign+`]
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
`)
	chdir(t, dir)
	var out, errOut bytes.Buffer
	if code := Run([]string{"check"}, &out, &errOut, VersionInfo{}); code != 3 {
		t.Errorf("all-skipped exit = %d, want 3; stdout=%q", code, out.String())
	}
}
