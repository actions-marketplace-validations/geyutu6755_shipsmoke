package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geyutu6755/shipsmoke/internal/config"
	"github.com/geyutu6755/shipsmoke/internal/testfixture"
)

// Regression tests for the v1.0.3 audit findings in this package:
//   * a signal handler + goroutine was registered per check and never released
//   * the env: inherit warning bypassed the caller's writer

func zipWith(t *testing.T, dir string) string {
	t.Helper()
	bin, err := os.ReadFile(testfixture.HostBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, "tool.zip") // absolute: the engine resolves paths from the process cwd
	testfixture.BuildZip(t, name, []testfixture.ZipEntry{
		{Name: fixtureName(), Data: bin, Mode: 0o755, SetMode: true},
	})
	return name
}

func checkWithArgs(args ...string) config.Check {
	return config.Check{
		Args:    args,
		Timeout: config.Duration{Duration: 30 * time.Second},
		Expect:  config.Expect{ExitCode: 0},
	}
}

// TestRunDoesNotLeakSignalWatchers pins the fix for the per-check leak: Run
// installs one handler for the whole run and releases it on return. Before the
// fix every check left a blocked goroutine plus a live signal.Notify behind, so
// the goroutine count grew with the number of checks.
func TestRunDoesNotLeakSignalWatchers(t *testing.T) {
	dir := t.TempDir()
	art := zipWith(t, dir)

	// Six checks is enough to show linear growth: the old code leaked exactly
	// one goroutine per check.
	artifact := config.Artifact{
		Name:   "leak-check",
		Path:   art,
		Binary: fixtureName(),
		Checks: []config.Check{
			checkWithArgs("--version"), checkWithArgs("--version"),
			checkWithArgs("--version"), checkWithArgs("--version"),
			checkWithArgs("--version"), checkWithArgs("--version"),
		},
	}
	cfg := buildConfig(t, dir, []config.Artifact{artifact}, "")

	settle := func() int {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		var n int
		for time.Now().Before(deadline) {
			runtime.GC()
			time.Sleep(50 * time.Millisecond)
			n = runtime.NumGoroutine()
			if n <= baseline(t) {
				return n
			}
		}
		return n
	}
	before := settle()

	for i := 0; i < 4; i++ {
		rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
		if exit != ExitPass {
			t.Fatalf("run %d exit = %d, want 0 (%+v)", i, exit, rep.Summary)
		}
	}

	after := settle()
	if slack := after - before; slack > 1 {
		t.Errorf("goroutines grew by %d across 4 runs of a 6-check config (before=%d after=%d): a signal watcher or handler is leaking",
			slack, before, after)
	}
}

// baseline reports the goroutine count at the start of the process, used only
// as a stable reference for the leak assertion above.
func baseline(t *testing.T) int {
	t.Helper()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	return runtime.NumGoroutine()
}

// TestSignalContextStopReleasesWatcher exercises the helper directly: stop must
// return only after the watcher goroutine has exited and unregistered.
func TestSignalContextStopReleasesWatcher(t *testing.T) {
	runtime.GC()
	before := baseline(t)

	ctx, stop := signalContext()
	select {
	case <-ctx.Done():
		t.Fatal("fresh context should not be cancelled")
	default:
	}
	stop()

	select {
	case <-ctx.Done():
		// expected: stop cancels
	default:
		t.Error("stop() must cancel the context so a running check unwinds")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
		if runtime.NumGoroutine() <= before {
			return
		}
	}
	t.Errorf("watcher goroutine still alive after stop(): before=%d now=%d", before, runtime.NumGoroutine())
}

// TestEnvInheritWarningUsesOptionsWriter proves the warning is routable: it must
// land on Options.Stderr, which is what makes it assertable from the CLI tests.
func TestEnvInheritWarningUsesOptionsWriter(t *testing.T) {
	dir := t.TempDir()
	art := zipWith(t, dir)
	artifact := config.Artifact{
		Name:   "inherits",
		Path:   art,
		Binary: fixtureName(),
		Env:    "inherit",
		Checks: []config.Check{checkWithArgs("--version")},
	}
	cfg := buildConfig(t, dir, []config.Artifact{artifact}, "")

	var buf bytes.Buffer
	opts := Options{ConfigPath: ".shipsmoke.yml", Version: "test", Stderr: &buf}
	if _, exit := Run(cfg, opts); exit != ExitPass {
		t.Fatalf("run exit = %d, want 0", exit)
	}
	got := buf.String()
	if !strings.Contains(got, "env: inherit") {
		t.Errorf("warning should reach Options.Stderr, got %q", got)
	}
	if !strings.Contains(got, "inherits") {
		t.Errorf("warning should name the artifact, got %q", got)
	}

	// A run without inherit must stay silent, and nil Stderr must not crash.
	var quiet bytes.Buffer
	artifact.Env = ""
	artifact.Name = "minimal"
	cfg2 := buildConfig(t, dir, []config.Artifact{artifact}, "")
	if _, exit := Run(cfg2, Options{ConfigPath: ".shipsmoke.yml", Stderr: &quiet}); exit != ExitPass {
		t.Fatalf("exit = %d", exit)
	}
	if quiet.Len() != 0 {
		t.Errorf("no warning expected for env: minimal, got %q", quiet.String())
	}
	if _, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml"}); exit != ExitPass {
		t.Errorf("nil Stderr must fall back to os.Stderr without crashing, exit = %d", exit)
	}
}
