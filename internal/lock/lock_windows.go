//go:build windows

package lock

import "os"

// processAlive reports whether pid is a live process on Windows.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(os.Kill) == nil
}
