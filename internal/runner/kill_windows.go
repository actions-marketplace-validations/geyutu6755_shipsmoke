//go:build windows

package runner

import (
	"errors"
	"os/exec"
	"syscall"
)

// setProcessGroup is a no-op on Windows; CREATE_NEW_PROCESS_GROUP would only
// affect console ctrl events, and killTree uses taskkill /T instead.
func setProcessGroup(cmd *exec.Cmd) {}

// killTree kills the child and all of its descendants via taskkill /T.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/F", "/T", "/PID", itoa(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = kill.Run()
	// Belt and braces: if taskkill failed (e.g. no console), kill directly.
	_ = cmd.Process.Kill()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func asExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}

func waitStatus(ee *exec.ExitError) (syscall.WaitStatus, bool) {
	ws, ok := ee.Sys().(syscall.WaitStatus)
	return ws, ok
}
