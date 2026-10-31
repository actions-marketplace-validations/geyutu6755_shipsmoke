//go:build !windows

package runner

import "syscall"

func isUnix() bool { return true }

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
