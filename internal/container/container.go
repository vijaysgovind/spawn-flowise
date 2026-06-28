// Package container abstracts Docker/Podman engines and compose operations for
// spawn-flowise.
package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// EngineType identifies the container engine.
type EngineType string

const (
	Docker EngineType = "docker"
	Podman EngineType = "podman"
)

// Engine wraps a container engine client and its compose counterpart.
// Compose holds the program and subcommand(s), e.g. ["docker", "compose"]
// or ["podman", "compose"], so the modern CLI plugin syntax is used.
type Engine struct {
	Type      EngineType
	ClientCmd string   // docker or podman
	Compose   []string // e.g. ["docker", "compose"]
}

// New validates and returns an Engine for the requested engine name.
func New(name string) (*Engine, error) {
	switch strings.ToLower(name) {
	case "docker":
		return &Engine{Type: Docker, ClientCmd: "docker", Compose: []string{"docker", "compose"}}, nil
	case "podman":
		return &Engine{Type: Podman, ClientCmd: "podman", Compose: []string{"podman", "compose"}}, nil
	default:
		return nil, fmt.Errorf("unsupported container engine %q: use docker or podman", name)
	}
}

// Detect looks for an available engine, preferring docker.
func Detect() (*Engine, error) {
	for _, name := range []string{"docker", "podman"} {
		e, err := New(name)
		if err != nil {
			continue
		}
		if _, err := exec.LookPath(e.ClientCmd); err == nil {
			return e, nil
		}
	}
	return nil, fmt.Errorf("no container engine (docker/podman) found in PATH")
}

// SocketPath returns the default socket path for the current platform.
func (e *Engine) SocketPath() string {
	if v := os.Getenv("DOCKER_HOST"); v != "" && e.Type == Docker {
		return v
	}
	if v := os.Getenv("PODMAN_SOCKET"); v != "" && e.Type == Podman {
		return v
	}
	switch runtime.GOOS {
	case "windows":
		if e.Type == Docker {
			return "npipe:////./pipe/docker_engine"
		}
		return ""
	case "darwin":
		home, _ := os.UserHomeDir()
		if e.Type == Docker {
			p := filepath.Join(home, ".docker", "run", "docker.sock")
			if _, err := os.Stat(p); err == nil {
				return "unix://" + p
			}
			return "unix:///var/run/docker.sock"
		}
		return "unix:///var/run/podman/podman.sock"
	default:
		if e.Type == Docker {
			return "unix:///var/run/docker.sock"
		}
		uid := os.Getuid()
		p := fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
		if _, err := os.Stat(p); err == nil {
			return "unix://" + p
		}
		return "unix:///run/podman/podman.sock"
	}
}

// env returns a copy of the current environment with engine-specific socket vars.
func (e *Engine) env() []string {
	env := os.Environ()
	if e.Type == Docker {
		if os.Getenv("DOCKER_HOST") == "" {
			sp := e.SocketPath()
			if sp != "" {
				env = append(env, "DOCKER_HOST="+sp)
			}
		}
	}
	return env
}

// Check verifies the engine is reachable by running `version`.
func (e *Engine) Check(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, e.ClientCmd, "version")
	cmd.Env = e.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s unreachable: %w: %s", e.Type, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// NetworkExists reports whether a network already exists.
func (e *Engine) NetworkExists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, e.ClientCmd, "network", "inspect", "--", name)
	cmd.Env = e.env()
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	exitErr, ok := err.(*exec.ExitError)
	if ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("checking network %s: %w", name, err)
}

// NetworkCreate creates a network if it does not already exist.
func (e *Engine) NetworkCreate(ctx context.Context, name string) error {
	exists, err := e.NetworkExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	cmd := exec.CommandContext(ctx, e.ClientCmd, "network", "create", "--", name)
	cmd.Env = e.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating network %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// NetworkRemove removes a network by name.
func (e *Engine) NetworkRemove(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, e.ClientCmd, "network", "rm", "--", name)
	cmd.Env = e.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("removing network %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListNetworks returns network names matching the given prefix.
func (e *Engine) ListNetworks(ctx context.Context, prefix string) ([]string, error) {
	cmd := exec.CommandContext(ctx, e.ClientCmd, "network", "ls", "--format", "{{.Name}}")
	cmd.Env = e.env()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing networks: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			names = append(names, line)
		}
	}
	return names, nil
}

// ComposeUp starts services defined by composeFile with the given project name and env file.
func (e *Engine) ComposeUp(ctx context.Context, project, composeFile, envFile string) error {
	args := []string{"-p", project, "-f", composeFile}
	if envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "up", "-d", "--remove-orphans")
	cmd := exec.CommandContext(ctx, e.Compose[0], append(e.Compose[1:], args...)...)
	cmd.Env = e.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up failed for %s: %w: %s", project, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ComposeDown stops and removes services for a project.
func (e *Engine) ComposeDown(ctx context.Context, project, composeFile, envFile string) error {
	args := []string{"-p", project, "-f", composeFile}
	if envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "down", "--volumes")
	cmd := exec.CommandContext(ctx, e.Compose[0], append(e.Compose[1:], args...)...)
	cmd.Env = e.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose down failed for %s: %w: %s", project, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// StopContainer stops a container by name or ID.
func (e *Engine) StopContainer(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, e.ClientCmd, "stop", "--time", "30", "--", name)
	cmd.Env = e.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stopping container %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveContainer removes a container by name or ID.
func (e *Engine) RemoveContainer(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, e.ClientCmd, "rm", "--force", "--", name)
	cmd.Env = e.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("removing container %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ListProjectContainers returns container names matching the project prefix.
func (e *Engine) ListProjectContainers(ctx context.Context, prefix string) ([]string, error) {
	cmd := exec.CommandContext(ctx, e.ClientCmd, "ps", "-a", "--format", "{{.Names}}")
	cmd.Env = e.env()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			names = append(names, line)
		}
	}
	return names, nil
}

// ParseMemoryLimit converts a Docker-style memory limit to megabytes.
// Supports M/G suffixes. Returns 0 for empty/unparseable values.
func ParseMemoryLimit(s string) uint64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0
	}
	mult := uint64(1)
	if strings.HasSuffix(s, "G") {
		mult = 1024
		s = strings.TrimSuffix(s, "G")
	} else if strings.HasSuffix(s, "M") {
		s = strings.TrimSuffix(s, "M")
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v * mult
}
