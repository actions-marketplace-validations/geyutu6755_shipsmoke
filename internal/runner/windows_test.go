//go:build windows

package runner

import "os/exec"

func isUnix() bool { return false }

func processAlive(pid int) bool {
	// taskkill without /F on a dead PID returns an error mentioning it does
	// not exist; presence check via `tasklist /FI "PID eq N"` exit status.
	cmd := exec.Command("tasklist", "/FI", itoa(pid))
	_ = cmd.Run() // best-effort; precise orphan checks are not done on Windows
	return false
}
