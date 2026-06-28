// Package utils provides shared helpers, filesystem scanning, and secure sudo
// wrappers for spawn-flowise.
package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ExpandPath resolves a path that may start with ~ to an absolute path.
func ExpandPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return filepath.Abs(p)
}

// PathExists reports whether p exists on disk.
func PathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// IsSymlink reports whether p is a symlink.
func IsSymlink(p string) (bool, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return fi.Mode()&os.ModeSymlink != 0, nil
}

// ValidatePath returns an error if p is a symlink or cannot be resolved.
// Use before destructive or move operations to protect against symlink attacks.
func ValidatePath(p string) error {
	abs, err := ExpandPath(p)
	if err != nil {
		return err
	}
	isLink, err := IsSymlink(abs)
	if err != nil {
		return fmt.Errorf("cannot stat %s: %w", abs, err)
	}
	if isLink {
		return fmt.Errorf("refusing to operate on symlink: %s", abs)
	}
	return nil
}

// SecureRename moves src to dst using os.Rename after validating both paths.
func SecureRename(src, dst string) error {
	for _, p := range []string{src, dst} {
		if err := ValidatePath(p); err != nil {
			return err
		}
	}
	return os.Rename(src, dst)
}

// SudoMove moves src to dst using sudo mv with argument separators.
func SudoMove(ctx context.Context, src, dst string) error {
	for _, p := range []string{src, dst} {
		if err := ValidatePath(p); err != nil {
			return err
		}
	}
	cmd := exec.CommandContext(ctx, "sudo", "--", "mv", "--", src, dst)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo mv failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SudoRemove removes path using sudo rm -rf with argument separators.
func SudoRemove(ctx context.Context, path string) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "sudo", "--", "rm", "-rf", "--", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo rm failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// instanceDirPattern matches ~/.flowiseNN and ~/.bkpflowiseNN directories.
var instanceDirPattern = regexp.MustCompile(`^\.(bkp)?flowise(\d+)$`)

// InstanceDir represents a discovered instance data directory.
type InstanceDir struct {
	Number int
	Path   string
	Held   bool
}

// ListInstanceDirs scans home for ~/.flowiseNN and ~/.bkpflowiseNN directories.
func ListInstanceDirs(home string) ([]InstanceDir, error) {
	entries, err := os.ReadDir(home)
	if err != nil {
		return nil, fmt.Errorf("reading home directory: %w", err)
	}
	var dirs []InstanceDir
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := instanceDirPattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		dirs = append(dirs, InstanceDir{
			Number: n,
			Path:   filepath.Join(home, e.Name()),
			Held:   m[1] == "bkp",
		})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Number < dirs[j].Number })
	return dirs, nil
}

// ArchiveToTarGz creates a gzip-compressed tar archive at destPath containing all
// provided source directories. It uses `sudo tar` so permissions are preserved.
func ArchiveToTarGz(ctx context.Context, sources []string, destPath string) error {
	if len(sources) == 0 {
		return fmt.Errorf("no sources provided for archive")
	}
	for _, src := range sources {
		if err := ValidatePath(src); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating backup directory: %w", err)
	}

	args := []string{"--", "tar", "-czf", destPath}
	args = append(args, "--")
	args = append(args, sources...)
	cmd := exec.CommandContext(ctx, "sudo", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo tar failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureDir creates dir if it does not exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// FirstNonEmpty returns the first non-empty string.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
