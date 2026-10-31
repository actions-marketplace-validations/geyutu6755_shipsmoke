package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactorFromEnv(t *testing.T) {
	t.Setenv("CI_DEPLOY_TOKEN", "abcdef1234567890")
	t.Setenv("SHORT", "abc") // too short to matter
	t.Setenv("PLAIN", "plainvalue12345")
	red := NewRedactor()
	got := red.Apply("token is abcdef1234567890 and plainvalue12345 stays")
	if strings.Contains(got, "abcdef1234567890") {
		t.Errorf("secret not redacted: %q", got)
	}
	if !strings.Contains(got, "plainvalue12345") {
		t.Errorf("non-secret value must survive: %q", got)
	}
	if !strings.Contains(got, "***REDACTED***") {
		t.Errorf("marker missing: %q", got)
	}
}

func TestRedactorOverlappingLongestFirst(t *testing.T) {
	r := NewRedactor()
	r.Add("short-secret-value")
	r.Add("longer-short-secret-value-extends")
	got := r.Apply("x longer-short-secret-value-extends y")
	if strings.Contains(got, "short-secret-value") {
		t.Errorf("longest secret should be applied first: %q", got)
	}
}

func TestJSONReportWellFormed(t *testing.T) {
	rep := &Report{
		ShipSmokeVersion: "test",
		ConfigPath:       ".shipsmoke.yml",
		HostOS:           "testos",
		HostArch:         "testarch",
		Artifacts: []ArtifactResult{{
			Name: "a", Path: "p.zip", Format: "zip", Decision: "ran",
			Checks: []CheckResult{{Name: "check #1", Status: StatusPass, ExitCode: 0}},
		}},
	}
	rep.ComputeSummary()
	var buf bytes.Buffer
	if err := WriteJSON(&buf, rep, NewRedactor()); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if parsed["summary"].(map[string]any)["overall"] != StatusPass {
		t.Errorf("overall = %v", parsed["summary"])
	}
}

func TestWriteJSONFileCreatesDirs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "deep", "nested", "report.json")
	rep := &Report{ShipSmokeVersion: "t"}
	rep.ComputeSummary()
	if err := WriteJSONFile(p, rep, NewRedactor()); err != nil {
		t.Fatalf("WriteJSONFile: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

func TestComputeSummary(t *testing.T) {
	rep := &Report{Artifacts: []ArtifactResult{
		{Decision: "ran", Checks: []CheckResult{{Status: StatusPass}, {Status: StatusFail}}},
		{Decision: "skipped", Checks: []CheckResult{{Status: StatusSkip}}},
		{Decision: "error"},
	}}
	rep.ComputeSummary()
	s := rep.Summary
	if s.Passed != 1 || s.Failed != 1 || s.Skipped != 2 || s.Errored != 1 || s.Executed != 2 {
		t.Errorf("summary = %+v", s)
	}
	if s.Overall != StatusFail {
		t.Errorf("overall = %s", s.Overall)
	}
}

func TestComputeSummaryNoChecks(t *testing.T) {
	rep := &Report{Artifacts: []ArtifactResult{{Decision: "skipped"}}}
	rep.ComputeSummary()
	if rep.Summary.Overall != StatusNoChecks {
		t.Errorf("overall = %s", rep.Summary.Overall)
	}
}

func TestGitHubSummary(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "summary.md")
	rep := &Report{
		ShipSmokeVersion: "t", ConfigPath: "c", HostOS: "os", HostArch: "arch",
		Artifacts: []ArtifactResult{
			{Decision: "ran", Name: "zip|thing", Checks: []CheckResult{
				{Name: "c1", Status: StatusPass, ExitCode: 0, DurationMS: 5},
				{Name: "c2", Status: StatusFail, ExitCode: 2, DurationMS: 7, Reasons: []string{"expected exit code 0, got 2"}},
			}},
			{Decision: "skipped", Name: "skip", SkipReason: "targets linux/amd64"},
		},
	}
	rep.ComputeSummary()
	if err := WriteGitHubSummary(p, rep); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "## ShipSmoke verification report") {
		t.Error("missing heading")
	}
	if !strings.Contains(s, `zip\|thing`) {
		t.Error("pipe not escaped")
	}
	if !strings.Contains(s, "❌") || !strings.Contains(s, "✅") || !strings.Contains(s, "⏭️") {
		t.Error("status icons missing")
	}
	if !strings.Contains(s, "Reproduce locally") {
		t.Error("repro section missing for failures")
	}
	// Appends, does not truncate:
	if err := WriteGitHubSummary(p, rep); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(p)
	if len(data2) != 2*len(data)/1 && !strings.Contains(string(data2), "ShipSmoke verification report\n\n**Overall") {
		// len(data2) should be roughly double
		t.Logf("append size: %d -> %d", len(data), len(data2))
	}
	if len(data2) <= len(data) {
		t.Errorf("second write did not append: %d vs %d", len(data2), len(data))
	}
}

func TestTerminalReportRender(t *testing.T) {
	rep := &Report{
		ShipSmokeVersion: "1.0.0", ConfigPath: ".shipsmoke.yml", HostOS: "os", HostArch: "arch", DurationMS: 42,
		Artifacts: []ArtifactResult{
			{Decision: "ran", Name: "artifact-1", Format: "zip", DetectedOS: "os", DetectedArch: "arch",
				Checks: []CheckResult{
					{Name: "check #1: tool --version", Status: StatusPass, DurationMS: 3},
					{Name: "check #2: tool bad", Status: StatusFail, DurationMS: 4,
						Reasons: []string{"expected exit code 0, got 2"},
						Stdout:  "some output\nline two", Stderr: "err line",
						Reproduce: "shipsmoke check --config .shipsmoke.yml"},
				}},
			{Decision: "skipped", Name: "artifact-2", SkipReason: "targets linux/amd64 but host is os/arch"},
		},
	}
	rep.ComputeSummary()
	var b bytes.Buffer
	rep.Terminal(&b, false, NewRedactor())
	s := b.String()
	for _, want := range []string{
		"shipsmoke 1.0.0", "[PASS] check #1: tool --version", "[FAIL] check #2",
		"expected exit code 0, got 2", "reproduce:", "[SKIP] artifact-2",
		"1 passed, 1 failed, 1 skipped, 0 errored",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("terminal report missing %q\n---\n%s", want, s)
		}
	}
}

func TestReproduceCommand(t *testing.T) {
	got := ReproduceCommand(".shipsmoke.yml", "my-artifact", []string{"--version", "two words"})
	if !strings.Contains(got, "--artifact my-artifact") {
		t.Errorf("repro = %q", got)
	}
	if !strings.Contains(got, `"two words"`) {
		t.Errorf("args not quoted: %q", got)
	}
}
