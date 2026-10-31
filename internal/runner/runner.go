// Package runner executes smoke-check commands against binaries and captures
// their behavior. It never uses a shell: commands are launched as argv
// arrays.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// maxCapture caps captured stdout/stderr per stream. Flooded processes are
// still killed by their timeout; we simply stop storing beyond the cap.
const maxCapture = 1 << 20 // 1 MiB

// EnvMode selects the child process environment.
type EnvMode string

const (
	// EnvMinimal passes a small, functional environment (plus any
	// EnvExtra). Secret-shaped variables are never passed.
	EnvMinimal EnvMode = "minimal"
	// EnvInherit passes the parent environment minus secret-shaped
	// variables. A warning is emitted by the engine when this is used.
	EnvInherit EnvMode = "inherit"
)

// Spec describes one check to run.
type Spec struct {
	BinaryPath string        // absolute path to the entry binary
	Args       []string      // argv, never a shell string
	Timeout    time.Duration // must be > 0
	WorkDir    string        // child cwd (extraction dir)
	Env        EnvMode
	EnvExtra   map[string]string
}

// Outcome is what happened when the check ran.
type Outcome struct {
	SpawnErr  error // process could not be started
	ExitCode  int   // observed exit code (0 if signaled or spawn error)
	Signal    string
	Stdout    string
	Stderr    string
	TimedOut  bool
	Killed    bool
	Duration  time.Duration
	Truncated bool
}

// secretNameRe matches environment variable names whose values must not leak
// into child processes or reports.
var secretNameRe = regexp.MustCompile(`(?i)(token|secret|password|passwd|credential|apikey|api_key|private_?key|_key$|_key_)`)

// IsSecretVar reports whether an env var name looks like it carries a secret.
func IsSecretVar(name string) bool {
	return secretNameRe.MatchString(name)
}

// BuildEnv constructs the child environment for the given mode.
func BuildEnv(mode EnvMode, extra map[string]string, dir string) []string {
	var env []string
	switch mode {
	case EnvInherit:
		for _, kv := range os.Environ() {
			name := kv
			if i := strings.IndexByte(kv, '='); i >= 0 {
				name = kv[:i]
			}
			if IsSecretVar(name) {
				continue
			}
			env = append(env, kv)
		}
	default: // minimal
		env = minimalEnv(dir)
	}
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// minimalEnv returns a small environment sufficient for typical CLI tools.
// The extraction dir is prepended to PATH so adjacent DLLs (Windows) and
// relative lookups resolve.
func minimalEnv(dir string) []string {
	var env []string
	pathListSep := string(os.PathListSeparator)
	newPath := dir
	if cur := os.Getenv("PATH"); cur != "" {
		newPath += pathListSep + cur
	}
	env = append(env, "PATH="+newPath)
	for _, k := range []string{"HOME", "LANG", "LC_ALL", "TZ", "TMPDIR"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	if os.Getenv("TMPDIR") == "" {
		env = append(env, "TMPDIR="+os.TempDir())
	}
	// Windows runtime essentials.
	for _, k := range []string{"SystemRoot", "SystemDrive", "COMPUTERNAME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH", "PATHEXT", "APPDATA", "LOCALAPPDATA", "ProgramData"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	if os.Getenv("TEMP") == "" && os.Getenv("TMP") != "" {
		env = append(env, "TMP="+os.Getenv("TMP"))
	} else if os.Getenv("TEMP") != "" {
		env = append(env, "TEMP="+os.Getenv("TEMP"))
	}
	return env
}

// Run executes the spec and returns the outcome. It always returns a
// non-nil Outcome, even on spawn failure.
func Run(ctx context.Context, spec Spec) *Outcome {
	out := &Outcome{}
	if spec.Timeout <= 0 {
		spec.Timeout = 60 * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, spec.BinaryPath, spec.Args...)
	cmd.Dir = spec.WorkDir
	cmd.Env = BuildEnv(spec.Env, spec.EnvExtra, spec.WorkDir)
	setProcessGroup(cmd) // platform-specific (setpgid on Unix)

	var stdout, stderr cappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Start(); err != nil {
		out.SpawnErr = fmt.Errorf("%w", err)
		out.Duration = time.Since(start)
		return out
	}

	waitErr := cmd.Wait()
	out.Duration = time.Since(start)

	// Decide why Wait returned.
	select {
	case <-cmdCtx.Done():
		out.TimedOut = true
	default:
	}
	if out.TimedOut {
		killTree(cmd)
		// Reap to avoid zombies; ignore the resulting error code.
		_ = cmd.Wait()
		out.Killed = true
	} else if waitErr != nil {
		var ee *exec.ExitError
		if asExitError(waitErr, &ee) {
			out.ExitCode = ee.ExitCode()
			if ws, ok := waitStatus(ee); ok && ws.Signaled() {
				if sig := ws.Signal(); sig >= 0 {
					out.Signal = sig.String()
				}
				out.ExitCode = 0
			}
		}
	}
	out.Stdout = stdout.String()
	out.Stderr = stderr.String()
	out.Truncated = stdout.truncated || stderr.truncated
	return out
}

// cappedBuffer captures up to maxCapture bytes, discarding beyond with a
// marker flag.
type cappedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	truncated bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.buf.Len() >= maxCapture {
		c.truncated = true
		return len(p), nil // pretend success; process keeps running
	}
	room := maxCapture - c.buf.Len()
	if len(p) > room {
		c.buf.Write(p[:room])
		c.truncated = true
		return len(p), nil
	}
	return c.buf.Write(p)
}

func (c *cappedBuffer) String() string {
	s := c.buf.String()
	if c.truncated {
		s += "\n[output truncated at 1 MiB]"
	}
	return s
}
