// Package lock provides portable file-based concurrency control for spawn-flowise.
package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Lock represents an acquired file lock.
type Lock struct {
	path string
}

// Acquire attempts to create a lock file. It returns a release function on success.
// If the lock is already held by a live process, it returns an error.
func Acquire(path string) (*Lock, func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating lock directory: %w", err)
	}

	// Try to create the lock file exclusively.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if !os.IsExist(err) {
			return nil, nil, fmt.Errorf("creating lock file: %w", err)
		}
		// Lock exists: see if it is stale.
		pid, readErr := readPID(path)
		if readErr == nil && pid > 0 && !processAlive(pid) {
			_ = os.Remove(path)
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("lock already held: %s", path)
		}
	}

	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Close()

	lk := &Lock{path: path}
	release := func() { _ = lk.Release() }
	return lk, release, nil
}

// Release removes the lock file.
func (l *Lock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}
	return os.Remove(l.path)
}

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}
