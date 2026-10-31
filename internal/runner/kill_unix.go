//go:build !windows

package runner

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so a timeout can
// kill the whole tree (children of children included).
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killTree kills the child's entire process group.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid := -cmd.Process.Pid
	_ = syscall.Kill(pgid, syscall.SIGKILL)
}

func asExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}

func waitStatus(ee *exec.ExitError) (syscall.WaitStatus, bool) {
	ws, ok := ee.Sys().(syscall.WaitStatus)
	return ws, ok
}
