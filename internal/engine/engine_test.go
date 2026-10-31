package engine

import (
	"archive/tar"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geyutu6755/shipsmoke/internal/config"
	"github.com/geyutu6755/shipsmoke/internal/report"
	"github.com/geyutu6755/shipsmoke/internal/testfixture"
)

// buildConfig writes a .shipsmoke.yml with the given artifacts and returns
// the config object.
func buildConfig(t *testing.T, dir string, artifacts []config.Artifact, reportJSON string) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Version:   1,
		Artifacts: artifacts,
		Report:    config.ReportCfg{JSON: reportJSON},
	}
	cfg.ApplyDefaults()
	return cfg
}

func hostPlatform() string { return runtime.GOOS + "/" + runtime.GOARCH }

// --- positive scenarios ---

func TestEndToEndZipPass(t *testing.T) {
	dir := t.TempDir()
	bin, err := os.ReadFile(testfixture.HostBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "tool.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{
		{Name: "bin/" + fixtureName(), Data: bin, Mode: 0o755, SetMode: true},
		{Name: "README.md", Data: []byte("readme")},
	})
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name:   "zip-artifact",
		Path:   archive,
		Binary: "bin/" + fixtureName(),
		Checks: []config.Check{
			{Args: []string{"--version"}, Expect: config.Expect{
				ExitCode:       0,
				StdoutContains: []string{"tool version 1.2.3"},
			}, Timeout: config.Duration{Duration: 30 * time.Second}},
		},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitPass {
		t.Fatalf("exit = %d, want 0; report: %+v", exit, rep.Summary)
	}
	a := rep.Artifacts[0]
	if a.Decision != "ran" || a.DetectedOS != runtime.GOOS || a.DetectedArch != runtime.GOARCH {
		t.Errorf("artifact result wrong: %+v", a)
	}
	if a.Checks[0].Status != report.StatusPass {
		t.Errorf("check status = %s, reasons=%v", a.Checks[0].Status, a.Checks[0].Reasons)
	}
}

func TestEndToEndTarGzPass(t *testing.T) {
	dir := t.TempDir()
	bin, _ := os.ReadFile(testfixture.HostBinary(t))
	archive := filepath.Join(dir, "tool.tar.gz")
	testfixture.BuildTarGz(t, archive, []testfixture.TarEntry{
		{Name: "tool", Mode: 0o755, Dir: true},
		{Name: "tool/" + fixtureName(), Data: bin, Mode: 0o755},
	})
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name:   "targz-artifact",
		Path:   archive,
		Binary: "tool/" + fixtureName(),
		Checks: []config.Check{
			{Args: []string{"--help"}, Expect: config.Expect{ExitCode: 0, StdoutContains: []string{"fixture tool"}}},
		},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitPass {
		t.Fatalf("exit = %d; reasons: %v", exit, rep.Artifacts[0].Checks[0].Reasons)
	}
}

func TestEndToEndStandaloneBinaryPass(t *testing.T) {
	dir := t.TempDir()
	src := testfixture.HostBinary(t)
	name := fixtureName()
	dest := filepath.Join(dir, name)
	bin, _ := os.ReadFile(src)
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(dest, bin, mode); err != nil {
		t.Fatal(err)
	}
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name:   "binary-artifact",
		Path:   dest,
		Binary: name,
		Checks: []config.Check{{Args: []string{"--version"}}},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitPass {
		t.Fatalf("exit = %d; reasons: %v", exit, rep.Artifacts[0].Checks[0].Reasons)
	}
}

func TestEndToEndMultipleArtifacts(t *testing.T) {
	dir := t.TempDir()
	bin, _ := os.ReadFile(testfixture.HostBinary(t))
	name := fixtureName()
	archive := filepath.Join(dir, "tool.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: name, Data: bin, Mode: 0o755, SetMode: true}})
	standalone := filepath.Join(dir, name)
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(standalone, bin, mode); err != nil {
		t.Fatal(err)
	}
	cfg := buildConfig(t, dir, []config.Artifact{
		{Name: "from-zip", Path: archive, Binary: name, RunsOn: []string{hostPlatform()},
			Checks: []config.Check{{Args: []string{"--version"}}}},
		{Name: "standalone", Path: standalone, Binary: name, RunsOn: []string{hostPlatform()},
			Checks: []config.Check{{Args: []string{"echo", "hi"}}}},
		{Name: "other-platform", Path: archive, Binary: name, RunsOn: []string{"bogusos/bogusarch"},
			Checks: []config.Check{{Args: []string{"--version"}}}},
	}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitPass {
		t.Fatalf("exit = %d, want 0 (skip does not fail)", exit)
	}
	if rep.Artifacts[0].Decision != "ran" || rep.Artifacts[1].Decision != "ran" {
		t.Errorf("first two artifacts should run")
	}
	if rep.Artifacts[2].Decision != "skipped" {
		t.Errorf("third artifact should be skipped, got %s", rep.Artifacts[2].Decision)
	}
	if rep.Summary.Skipped != 1 || rep.Summary.Passed != 2 {
		t.Errorf("summary = %+v", rep.Summary)
	}
}

func TestEndToEndStdoutMatcherAndExpectedNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	src := testfixture.HostBinary(t)
	name := fixtureName()
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(filepath.Join(dir, name), mustRead(t, src), mode); err != nil {
		t.Fatal(err)
	}
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "x", Path: filepath.Join(dir, name), Binary: name,
		Checks: []config.Check{
			{Args: []string{"echo", "hello"}, Expect: config.Expect{
				ExitCode: 0, StdoutContains: []string{"hello"}, StdoutNotContains: []string{"bye"},
			}},
			{Args: []string{"exit", "3"}, Expect: config.Expect{ExitCode: 3}},
		},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitPass {
		t.Fatalf("exit = %d", exit)
	}
	if rep.Artifacts[0].Checks[1].Status != report.StatusPass {
		t.Errorf("expected-exit-3 check should pass, reasons: %v", rep.Artifacts[0].Checks[1].Reasons)
	}
}

// --- failure scenarios ---

func TestEndToEndArtifactMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "gone", Path: filepath.Join(dir, "missing.zip"), Binary: "x",
		Checks: []config.Check{{Args: []string{"--version"}}},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitFail {
		t.Errorf("exit = %d, want 1", exit)
	}
	if rep.Artifacts[0].Decision != "error" || !strings.Contains(rep.Artifacts[0].Error, "not found") {
		t.Errorf("artifact error = %+v", rep.Artifacts[0])
	}
}

func TestEndToEndEntryBinaryMissing(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: "readme.txt", Data: []byte("no binary here")}})
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "no-entry", Path: archive, Binary: "tool",
		Checks: []config.Check{{Args: []string{"--version"}}},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitFail {
		t.Errorf("exit = %d, want 1", exit)
	}
	c := rep.Artifacts[0].Checks[0]
	if c.Status != report.StatusFail || !strings.Contains(strings.Join(c.Reasons, " "), "not found") {
		t.Errorf("entry-missing check = %+v", c)
	}
	if !strings.Contains(strings.Join(c.Reasons, " "), "readme.txt") {
		t.Errorf("diagnosis should list archive contents: %v", c.Reasons)
	}
}

func TestEndToEndPlatformMismatch(t *testing.T) {
	var foreignGOOS, foreignGOARCH string
	switch {
	case runtime.GOOS != "linux":
		foreignGOOS, foreignGOARCH = "linux", "amd64"
	default:
		foreignGOOS, foreignGOARCH = "darwin", "arm64"
	}
	if testing.Short() {
		t.Skip("cross-compile fixture needed")
	}
	dir := t.TempDir()
	foreign := testfixture.BinPath(t, foreignGOOS, foreignGOARCH)
	name := "foreign-" + fixtureName()
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	dest := filepath.Join(dir, name)
	if err := os.WriteFile(dest, mustRead(t, foreign), mode); err != nil {
		t.Fatal(err)
	}
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "mismatch", Path: dest, Binary: name, RunsOn: []string{"any"},
		Checks: []config.Check{{Args: []string{"--version"}}},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitFail {
		t.Errorf("exit = %d, want 1 (foreign binary on host must fail)", exit)
	}
	c := rep.Artifacts[0].Checks[0]
	joined := strings.Join(c.Reasons, " ")
	if c.Status != report.StatusFail || !strings.Contains(joined, "mismatch") {
		t.Errorf("platform mismatch diagnosis missing: %+v", c)
	}
}

func TestEndToEndUnexpectedExitAndOutput(t *testing.T) {
	dir := t.TempDir()
	name := fixtureName()
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(filepath.Join(dir, name), mustRead(t, testfixture.HostBinary(t)), mode); err != nil {
		t.Fatal(err)
	}
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "x", Path: filepath.Join(dir, name), Binary: name,
		Checks: []config.Check{
			{Args: []string{"exit", "9"}}, // expected 0, got 9
			{Args: []string{"echo", "wrong"}, Expect: config.Expect{ExitCode: 0, StdoutContains: []string{"right"}}},
			{Args: []string{"stderr", "oops"}, Expect: config.Expect{ExitCode: 0, StderrNotContains: []string{"oops"}}},
		},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitFail {
		t.Errorf("exit = %d, want 1", exit)
	}
	checks := rep.Artifacts[0].Checks
	if checks[0].Status != report.StatusFail || !strings.Contains(strings.Join(checks[0].Reasons, " "), "expected exit code 0, got 9") {
		t.Errorf("unexpected-exit diagnosis: %+v", checks[0])
	}
	if checks[1].Status != report.StatusFail || !strings.Contains(strings.Join(checks[1].Reasons, " "), "does not contain") {
		t.Errorf("stdout-mismatch diagnosis: %+v", checks[1])
	}
	if checks[2].Status != report.StatusFail || !strings.Contains(strings.Join(checks[2].Reasons, " "), "must not contain") {
		t.Errorf("stderr-not-contains diagnosis: %+v", checks[2])
	}
}

func TestEndToEndTimeout(t *testing.T) {
	dir := t.TempDir()
	name := fixtureName()
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(filepath.Join(dir, name), mustRead(t, testfixture.HostBinary(t)), mode); err != nil {
		t.Fatal(err)
	}
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "slow", Path: filepath.Join(dir, name), Binary: name,
		Checks: []config.Check{{Args: []string{"sleep", "60"}, Timeout: config.Duration{Duration: 2 * time.Second}}},
	}}, "")
	start := time.Now()
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitFail {
		t.Errorf("exit = %d, want 1", exit)
	}
	c := rep.Artifacts[0].Checks[0]
	if c.Status != report.StatusFail || !c.TimedOut {
		t.Errorf("timeout check = %+v", c)
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Errorf("timeout enforcement took %v", elapsed)
	}
}

func TestEndToEndCorruptAndMaliciousArchives(t *testing.T) {
	dir := t.TempDir()

	// Corrupt zip.
	good := filepath.Join(dir, "good.zip")
	testfixture.BuildZip(t, good, []testfixture.ZipEntry{{Name: "a.txt", Data: []byte("x")}})
	data, _ := os.ReadFile(good)
	corrupt := filepath.Join(dir, "corrupt.zip")
	os.WriteFile(corrupt, data[:len(data)/2], 0o644)

	// Traversal zip.
	trav := filepath.Join(dir, "trav.zip")
	testfixture.BuildZip(t, trav, []testfixture.ZipEntry{{Name: "../evil.txt", Data: []byte("x")}})

	// Symlink tar.
	sym := filepath.Join(dir, "sym.tar.gz")
	testfixture.BuildTarGz(t, sym, []testfixture.TarEntry{{Name: "l", Typeflag: tar.TypeSymlink, Link: "/etc/passwd"}})

	// Bomb zip.
	bomb := filepath.Join(dir, "bomb.zip")
	testfixture.BuildZip(t, bomb, []testfixture.ZipEntry{{Name: "zeros.bin", Data: make([]byte, 32<<20)}})

	cfg := buildConfig(t, dir, []config.Artifact{
		{Name: "corrupt", Path: corrupt, Binary: "x", Checks: []config.Check{{Args: []string{"v"}}}},
		{Name: "trav", Path: trav, Binary: "x", Checks: []config.Check{{Args: []string{"v"}}}},
		{Name: "sym", Path: sym, Binary: "x", Checks: []config.Check{{Args: []string{"v"}}}},
		{Name: "bomb", Path: bomb, Binary: "x", Checks: []config.Check{{Args: []string{"v"}}}},
	}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitFail {
		t.Errorf("exit = %d, want 1", exit)
	}
	for i, want := range []string{"zip", "traversal", "symbolic link", "limits"} {
		a := rep.Artifacts[i]
		if a.Decision != "error" {
			t.Errorf("artifact %d: decision = %s, want error (error=%s)", i, a.Decision, a.Error)
			continue
		}
		if !strings.Contains(a.Error, want) {
			t.Errorf("artifact %d error %q does not mention %q", i, a.Error, want)
		}
	}
}

func TestEndToEndJSONReport(t *testing.T) {
	dir := t.TempDir()
	name := fixtureName()
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(filepath.Join(dir, name), mustRead(t, testfixture.HostBinary(t)), mode); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(dir, "out", "report.json")
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "json-test", Path: filepath.Join(dir, name), Binary: name,
		Checks: []config.Check{{Args: []string{"--version"}}},
	}}, jsonPath)
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "9.9.9-test"})
	if exit != ExitPass {
		t.Fatalf("exit = %d", exit)
	}
	if err := report.WriteJSONFile(jsonPath, rep, report.NewRedactor()); err != nil {
		t.Fatalf("write json: %v", err)
	}
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json invalid: %v\n%s", err, raw)
	}
	if parsed["shipsmoke_version"] != "9.9.9-test" {
		t.Errorf("version field = %v", parsed["shipsmoke_version"])
	}
	if parsed["exit_code"].(float64) != 0 {
		t.Errorf("exit_code = %v", parsed["exit_code"])
	}
	arts := parsed["artifacts"].([]any)
	if len(arts) != 1 {
		t.Fatalf("artifacts = %d", len(arts))
	}
	art := arts[0].(map[string]any)
	if art["detected_os"] != runtime.GOOS {
		t.Errorf("detected_os = %v", art["detected_os"])
	}
}

func TestEndToEndAllSkipped(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "tool.zip")
	testfixture.BuildZip(t, archive, []testfixture.ZipEntry{{Name: "a.txt", Data: []byte("x")}})
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "elsewhere", Path: archive, Binary: "a.txt", RunsOn: []string{"nonos/nparch"},
		Checks: []config.Check{{Args: []string{"--version"}}},
	}}, "")
	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if exit != ExitNoChecks {
		t.Errorf("exit = %d, want 3", exit)
	}
	if rep.Summary.Overall != report.StatusNoChecks {
		t.Errorf("overall = %s", rep.Summary.Overall)
	}
	rep2, exit2 := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test", AllowEmpty: true})
	if exit2 != ExitPass {
		t.Errorf("--allow-empty: exit = %d, want 0", exit2)
	}
	_ = rep2
}

func TestEndToEndArtifactFilter(t *testing.T) {
	dir := t.TempDir()
	name := fixtureName()
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.WriteFile(filepath.Join(dir, name), mustRead(t, testfixture.HostBinary(t)), mode); err != nil {
		t.Fatal(err)
	}
	cfg := buildConfig(t, dir, []config.Artifact{
		{Name: "one", Path: filepath.Join(dir, name), Binary: name, Checks: []config.Check{{Args: []string{"--version"}}}},
		{Name: "two", Path: filepath.Join(dir, name), Binary: name, Checks: []config.Check{{Args: []string{"--version"}}}},
	}, "")
	rep, _ := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test", ArtifactFilter: "two"})
	if len(rep.Artifacts) != 1 || rep.Artifacts[0].Name != "two" {
		t.Errorf("filter did not apply: %+v", rep.Artifacts)
	}
}

func TestEndToEndSecretRedaction(t *testing.T) {
	dir := t.TempDir()
	name := fixtureName()
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	secret := "super-secret-token-value-123456"
	t.Setenv("SHIPS_TEST_TOKEN", secret)
	if err := os.WriteFile(filepath.Join(dir, name), mustRead(t, testfixture.HostBinary(t)), mode); err != nil {
		t.Fatal(err)
	}
	cfg := buildConfig(t, dir, []config.Artifact{{
		Name: "redact", Path: filepath.Join(dir, name), Binary: name, Env: "minimal", EnvExtra: map[string]string{"SHIPS_TEST_TOKEN": secret},
		Checks: []config.Check{{Args: []string{"echo", secret}}},
	}}, "")
	rep, _ := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	var b strings.Builder
	rep.Terminal(&b, false, report.NewRedactor())
	if strings.Contains(b.String(), secret) {
		t.Error("secret leaked into terminal report")
	}
	var jb strings.Builder
	report.WriteJSON(&jb, rep, report.NewRedactor())
	if strings.Contains(jb.String(), secret) {
		t.Error("secret leaked into JSON report")
	}
}

// --- helpers ---

func fixtureName() string {
	n := "tool"
	if runtime.GOOS == "windows" {
		n += ".exe"
	}
	return n
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
