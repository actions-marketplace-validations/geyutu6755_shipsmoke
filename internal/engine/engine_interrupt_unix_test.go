//go:build !windows

package engine

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/geyutu6755/shipsmoke/internal/config"
)

// TestInterruptCancelsRemainingChecks is Unix-only: Windows cannot deliver
// SIGINT to a process, so this path is exercised on the Linux CI runner and
// skipped here rather than faked. It proves the run-scoped handler actually
// aborts the whole run — the pre-fix code cancelled only the check in flight
// and left every later check running.
func TestInterruptCancelsRemainingChecks(t *testing.T) {
	dir := t.TempDir()
	art := zipWith(t, dir)

	// Three long sleeps: if the interrupt is ignored this run takes 90s.
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

	// Register our own handler for the duration of the test so a stray SIGINT
	// can never take the test process down mid-run.
	guard := make(chan os.Signal, 4)
	signal.Notify(guard, syscall.SIGINT)
	defer signal.Stop(guard)

	done := make(chan int, 1)
	go func() {
		_, exit := Run(cfg, Options{ConfigPath: ".shipsmoke.yml", Version: "test"})
		done <- exit
	}()

	// Give Run a moment to install its handler and start the first sleep.
	time.Sleep(300 * time.Millisecond)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Skipf("cannot signal self: %v", err)
	}

	select {
	case exit := <-done:
		if exit == ExitPass {
			t.Errorf("an interrupted run of failing sleeps must not report success (exit=%d)", exit)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("interrupt did not abort the run: later checks kept running")
	}
}
