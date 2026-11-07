package engine

import (
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

// TestArchiveNameDoesNotDecideArchitecture pins the core guarantee behind
// the git-graph v0.8.3 finding: an archive whose NAME claims one platform
// but whose payload is a different architecture is judged by the binary
// header, never by the filename.
//
// The fixture archive is always named "tool-linux-amd64.tar.gz" (a name that
// advertises linux/amd64) but contains a real darwin/arm64 binary. The
// expected verdict depends solely on where the test runs:
//
//   - on a linux/amd64 host: the header says darwin/arm64, the host is
//     linux/amd64 -> platform mismatch FAIL (the misleading name must not
//     rescue the artifact)
//   - on a windows/amd64 host: header darwin/arm64 vs host windows/amd64 ->
//     platform mismatch FAIL (same guarantee, second platform pair)
//   - on a darwin/arm64 host: the header matches the host -> the artifact is
//     executed for real and PASSes (the header, not the name, decides)
func TestArchiveNameDoesNotDecideArchitecture(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-compile fixture in short mode")
	}

	dir := t.TempDir()
	foreign := testfixture.BinPath(t, "darwin", "arm64")
	bin, err := os.ReadFile(foreign)
	if err != nil {
		t.Fatal(err)
	}

	// The name advertises linux/amd64; the payload is darwin/arm64.
	archive := filepath.Join(dir, "tool-linux-amd64.tar.gz")
	testfixture.BuildTarGz(t, archive, []testfixture.TarEntry{
		{Name: "tool", Mode: 0o755, Dir: true},
		{Name: "tool/tool", Data: bin, Mode: 0o755},
	})

	host := runtime.GOOS + "/" + runtime.GOARCH
	mismatchHost := host != "darwin/arm64"

	cfg := &config.Config{
		Version: 1,
		Artifacts: []config.Artifact{{
			Name:   "named-linux-amd64",
			Path:   archive,
			Binary: "tool/tool",
			// Declared platforms: an empty list means "attempt on any host",
			// which is exactly how a user would package an archive whose name
			// they trust.
			RunsOn: nil,
			Checks: []config.Check{{
				Args:    []string{"--version"},
				Timeout: config.Duration{Duration: 60 * time.Second},
			}},
		}},
	}
	cfg.ApplyDefaults()

	rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
	if len(rep.Artifacts) != 1 {
		t.Fatalf("artifacts = %d", len(rep.Artifacts))
	}
	art := rep.Artifacts[0]
	checks := art.Checks
	if len(checks) == 0 {
		t.Fatalf("no checks recorded; artifact result: %+v", art)
	}
	gateCheck := checks[0]
	joined := strings.Join(gateCheck.Reasons, " ")

	if mismatchHost {
		// The gate must FAIL with a diagnosis that cites the real header, not
		// the archive name.
		if exit != ExitFail {
			t.Fatalf("host %s: exit = %d, want %d (mismatch must fail)", host, exit, ExitFail)
		}
		if gateCheck.Status != report.StatusFail {
			t.Errorf("host %s: check status = %s, want FAIL", host, gateCheck.Status)
		}
		if !strings.Contains(joined, "mismatch") {
			t.Errorf("host %s: diagnosis should cite the mismatch, got: %s", host, joined)
		}
		if !strings.Contains(art.DetectedOS+"/"+art.DetectedArch, "darwin/arm64") {
			t.Errorf("host %s: detected identity = %s/%s, want darwin/arm64 (header, not name)",
				host, art.DetectedOS, art.DetectedArch)
		}
		t.Logf("host %s: correctly refused — %s", host, joined)
		return
	}

	// darwin/arm64 host: the header matches, so the gate executes the binary
	// for real despite the misleading name.
	if exit != ExitPass {
		t.Fatalf("host %s: exit = %d, want %d; reasons: %v",
			host, exit, ExitPass, gateCheck.Reasons)
	}
	if gateCheck.Status != report.StatusPass {
		t.Errorf("host %s: check status = %s, want PASS", host, gateCheck.Status)
	}
	if art.DetectedOS != "darwin" || art.DetectedArch != "arm64" {
		t.Errorf("host %s: detected identity = %s/%s, want darwin/arm64",
			host, art.DetectedOS, art.DetectedArch)
	}
}
