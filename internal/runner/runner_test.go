package runner

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/geyutu6755/shipsmoke/internal/config"
	"github.com/geyutu6755/shipsmoke/internal/testfixture"
)

func runSpec(t *testing.T, spec Spec) *Outcome {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	return Run(ctx, spec)
}

func TestRunExitCodes(t *testing.T) {
	bin := testfixture.HostBinary(t)
	t.Run("exit 0", func(t *testing.T) {
		out := runSpec(t, Spec{BinaryPath: bin, Args: []string{"--version"}, Timeout: 30 * time.Second})
		if out.SpawnErr != nil {
			t.Fatalf("spawn: %v", out.SpawnErr)
		}
		if out.ExitCode != 0 {
			t.Errorf("exit = %d, stderr=%s", out.ExitCode, out.Stderr)
		}
		if !strings.Contains(out.Stdout, "tool version 1.2.3") {
			t.Errorf("stdout = %q", out.Stdout)
		}
	})
	t.Run("exit nonzero", func(t *testing.T) {
		out := runSpec(t, Spec{BinaryPath: bin, Args: []string{"exit", "7"}, Timeout: 30 * time.Second})
		if out.ExitCode != 7 {
			t.Errorf("exit = %d, want 7", out.ExitCode)
		}
	})
	t.Run("crash nonzero", func(t *testing.T) {
		out := runSpec(t, Spec{BinaryPath: bin, Args: []string{"crash"}, Timeout: 30 * time.Second})
		if out.ExitCode == 0 {
			t.Errorf("crash exit = %d, want nonzero", out.ExitCode)
		}
	})
}

func TestRunOutputCapture(t *testing.T) {
	bin := testfixture.HostBinary(t)
	out := runSpec(t, Spec{BinaryPath: bin, Args: []string{"echo", "hello", "world"}, Timeout: 30 * time.Second})
	if strings.TrimSpace(out.Stdout) != "hello world" {
		t.Errorf("stdout = %q", out.Stdout)
	}
	out = runSpec(t, Spec{BinaryPath: bin, Args: []string{"stderr", "boom"}, Timeout: 30 * time.Second})
	if strings.TrimSpace(out.Stderr) != "boom" {
		t.Errorf("stderr = %q", out.Stderr)
	}
}

func TestRunTimeout(t *testing.T) {
	bin := testfixture.HostBinary(t)
	start := time.Now()
	out := runSpec(t, Spec{BinaryPath: bin, Args: []string{"sleep", "60"}, Timeout: 2 * time.Second})
	elapsed := time.Since(start)
	if !out.TimedOut {
		t.Error("TimedOut = false, want true")
	}
	if elapsed > 6*time.Second {
		t.Errorf("kill took %v, far beyond the 2s timeout", elapsed)
	}
}

func TestRunTimeoutKillsTree(t *testing.T) {
	if !isUnix() {
		t.Skip("orphan verification uses Unix process signals")
	}
	bin := testfixture.HostBinary(t)
	out := runSpec(t, Spec{BinaryPath: bin, Args: []string{"spawn-sleep", "60"}, Timeout: 2 * time.Second})
	if !out.TimedOut {
		t.Fatal("TimedOut = false")
	}
	// The child pid was printed as "child pid N"; verify it is gone.
	m := regexp.MustCompile(`child pid (\d+)`).FindStringSubmatch(out.Stdout)
	if m == nil {
		t.Fatalf("no child pid in stdout %q", out.Stdout)
	}
	var pid int
	for _, c := range m[1] {
		pid = pid*10 + int(c-'0')
	}
	// Give the kernel a moment, then check the process no longer exists.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if !processAlive(pid) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("child process %d survived the process-tree kill", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestRunSpawnFailure(t *testing.T) {
	out := runSpec(t, Spec{BinaryPath: "/nonexistent/binary/xyz", Timeout: 5 * time.Second})
	if out.SpawnErr == nil {
		t.Fatal("want spawn error")
	}
	pass, reasons := Evaluate(out, config.Expect{ExitCode: 0})
	if pass || len(reasons) == 0 {
		t.Errorf("Evaluate should fail with diagnosis, got %v %v", pass, reasons)
	}
}

func TestRunOutputFloodCapped(t *testing.T) {
	bin := testfixture.HostBinary(t)
	out := runSpec(t, Spec{BinaryPath: bin, Args: []string{"flood"}, Timeout: 60 * time.Second})
	if out.ExitCode != 0 {
		t.Fatalf("exit = %d", out.ExitCode)
	}
	if len(out.Stdout) > maxCapture+200 {
		t.Errorf("stdout = %d bytes, cap exceeded", len(out.Stdout))
	}
	if !out.Truncated && len(out.Stdout) < maxCapture {
		t.Log("flood ended below cap; acceptable")
	}
}

func TestEvaluateMatchers(t *testing.T) {
	out := &Outcome{
		ExitCode: 3,
		Stdout:   "tool version 1.2.3\r\nbuilt ok\r\n",
		Stderr:   "warning: deprecated\r\n",
	}
	// All pass: CRLF normalized, exit matches.
	pass, reasons := Evaluate(out, config.Expect{
		ExitCode:       3,
		StdoutContains: []string{"tool version"},
		StdoutNotContains: []string{"error"},
		StderrContains: []string{"deprecated"},
		StderrRegex:    "warning.*deprecated",
	})
	if !pass {
		t.Errorf("should pass, got %v", reasons)
	}

	out.ExitCode = 2
	pass, reasons = Evaluate(out, config.Expect{ExitCode: 0})
	if pass || len(reasons) != 1 || !strings.Contains(reasons[0], "expected exit code 0, got 2") {
		t.Errorf("exit mismatch diagnosis = %v", reasons)
	}

	out.ExitCode = 0
	pass, reasons = Evaluate(out, config.Expect{ExitCode: 0, StdoutContains: []string{"absent"}})
	if pass || !strings.Contains(reasons[0], "stdout does not contain") {
		t.Errorf("stdout contains diagnosis = %v", reasons)
	}

	pass, reasons = Evaluate(out, config.Expect{ExitCode: 0, StdoutNotContains: []string{"built ok"}})
	if pass || !strings.Contains(reasons[0], "must not contain") {
		t.Errorf("stdout not-contains diagnosis = %v", reasons)
	}
}

func TestEvaluateEqualsAndRegexFailures(t *testing.T) {
	out := &Outcome{ExitCode: 0, Stdout: "aaa\nbbb\n"}
	pass, reasons := Evaluate(out, config.Expect{ExitCode: 0, StdoutEquals: "aaa\nbbb"})
	if pass || !strings.Contains(reasons[0], "does not equal") {
		t.Errorf("equals diagnosis = %v", reasons)
	}
	pass, _ = Evaluate(out, config.Expect{ExitCode: 0, StdoutEquals: "aaa\nbbb\n"})
	if !pass {
		t.Error("exact match should pass")
	}
	pass, _ = Evaluate(out, config.Expect{ExitCode: 0, StdoutRegex: "aaa.*bbb"})
	if !pass {
		t.Error("dot-matches-newline regex should pass")
	}
}

func TestBuildEnvMinimalFiltersAndAdds(t *testing.T) {
	t.Setenv("SHIPS_SMOKE_TOKEN", "leak-me")
	t.Setenv("MY_TEST_VAR", "keep-me")
	env := BuildEnv(EnvMinimal, map[string]string{"EXTRA": "1"}, "C:\\work\\dir")
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "SHIPS_SMOKE_TOKEN") {
		t.Error("secret-shaped var must not be in minimal env")
	}
	if strings.Contains(joined, "MY_TEST_VAR") {
		t.Error("minimal env must not include unrelated parent vars")
	}
	if !strings.Contains(joined, "EXTRA=1") {
		t.Error("env_extra must be passed")
	}
	if !strings.Contains(joined, "PATH=") {
		t.Error("PATH missing from minimal env")
	}
}

func TestBuildEnvInheritDropsSecrets(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "leak-me")
	t.Setenv("MY_PLAIN_VAR", "keep-me")
	env := BuildEnv(EnvInherit, nil, ".")
	joined := strings.Join(env, "\n")
	if strings.Contains(joined, "GITHUB_TOKEN") {
		t.Error("inherit env must drop secret-shaped vars")
	}
	if !strings.Contains(joined, "MY_PLAIN_VAR=keep-me") {
		t.Error("inherit env should keep plain vars")
	}
}

func TestIsSecretVar(t *testing.T) {
	secrets := []string{"GITHUB_TOKEN", "MY_SECRET", "DB_PASSWORD", "API_KEY", "apikey", "aws_secret_access_key", "MY_PRIVATE_KEY"}
	for _, s := range secrets {
		if !IsSecretVar(s) {
			t.Errorf("IsSecretVar(%q) = false", s)
		}
	}
	plain := []string{"PATH", "HOME", "MY_KEYBOARD_LAYOUT"}
	for _, s := range plain {
		if IsSecretVar(s) {
			t.Errorf("IsSecretVar(%q) = true", s)
		}
	}
}
