//go:build !windows

package engine

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/geyutu6755/shipsmoke/internal/config"
	"github.com/geyutu6755/shipsmoke/internal/report"
)

// TestInterruptCancelsRemainingChecks is Unix-only: Windows cannot deliver
// SIGINT to a process, so this path is exercised on the Linux/macOS CI runners
// and skipped here rather than faked.
//
// What proves the fix is not "the run returned" but that the LATER checks did
// not each run to completion. Three 30s sleeps are configured, so a run that
// only aborts the check in flight (the pre-fix behaviour) takes ~60-90s; a run
// whose context is shared across the whole run finishes in about a second.
func TestInterruptCancelsRemainingChecks(t *testing.T) {
	dir := t.TempDir()
	art := zipWith(t, dir)

	artifact := config.Artifact{
		Name:   "sleeper",
		Path:   art,
		Binary: fixtureName(),
		Checks: []config.Check{
			checkWithArgs("sleep", "30"),
			checkWithArgs("sleep", "30"),
			checkWithArgs("sleep", "30"),
		},
	}
	cfg := buildConfig(t, dir, []config.Artifact{artifact}, "")

	// Register our own handler for the duration of the test so a SIGINT can
	// never take the test process down, whatever the timing works out to be.
	guard := make(chan os.Signal, 4)
	signal.Notify(guard, syscall.SIGINT)
	defer signal.Stop(guard)

	type outcome struct {
		rep   *report.Report
		exit  int
		taken time.Duration
	}
	done := make(chan outcome, 1)
	start := time.Now()
	go func() {
		rep, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
		done <- outcome{rep, exit, time.Since(start)}
	}()

	// Wait for evidence that the first check really launched before signalling,
	// instead of guessing with a fixed sleep that can fire before the child
	// exists on a loaded runner.
	if !waitForFixture(t, 10*time.Second) {
		t.Skip("pgrep unavailable or fixture not visible; cannot time the interrupt reliably")
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Skipf("cannot signal self: %v", err)
	}

	var res outcome
	select {
	case res = <-done:
	case <-time.After(25 * time.Second):
		t.Fatal("interrupt did not abort the run: later checks kept running")
	}

	if res.exit == ExitPass {
		t.Errorf("an interrupted run of three failing sleeps must not report success (exit=%d)", res.exit)
	}
	// Three 30s sleeps were configured. Finishing in a few seconds means checks
	// 2 and 3 did not each get their full timeout.
	if res.taken > 15*time.Second {
		t.Errorf("run took %s after interrupt; later checks appear to have run anyway", res.taken)
	}
	if res.rep == nil || len(res.rep.Artifacts) != 1 {
		t.Fatalf("report missing: %+v", res.rep)
	}
	ran := 0
	for _, c := range res.rep.Artifacts[0].Checks {
		if c.Status == report.StatusPass {
			ran++
		}
	}
	if ran > 0 {
		t.Errorf("a check reported PASS after being interrupted: %d passed of %d", ran, len(res.rep.Artifacts[0].Checks))
	}
	t.Logf("interrupted run: exit=%d elapsed=%s checks=%d", res.exit, res.taken, len(res.rep.Artifacts[0].Checks))
}

// waitForFixture polls until a process named like the fixture binary exists, so
// the signal is delivered while a check is genuinely in flight.
func waitForFixture(t *testing.T, limit time.Duration) bool {
	t.Helper()
	bin := fixtureName()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		out, err := exec.Command("pgrep", "-x", bin).Output()
		if err == nil && len(out) > 0 {
			time.Sleep(100 * time.Millisecond) // let it settle into the sleep
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
