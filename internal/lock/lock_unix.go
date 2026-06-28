//go:build !windows

package lock

import "syscall"

// processAlive reports whether pid is a live process on Unix-like systems.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}
