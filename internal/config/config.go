// Package config centralizes naming conventions, defaults, and environment file
// operations for spawn-flowise.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Defaults for spawn-flowise orchestration.
const (
	BasePort           = 3001
	InternalPort       = 3000
	NetworkPoolSize    = 4
	SpawnDelaySec      = 30
	SpawnMemoryCheckMB = 1024 // per-instance RAM reservation used during spawn checks
	DefaultEngine      = "docker"
	ProjectName        = "spawn-flowise"
	ImageRef           = "flowiseai/flowise:latest"
)

// RuntimeDirs returns the per-user directories used by the CLI.
type RuntimeDirs struct {
	Home      string
	StateDir  string // ~/.flowise-spawn
	EnvDir    string // ~/.flowise-spawn/env
	BackupDir string // ~/flowise_backup
}

// Dirs lazily resolves and returns runtime directories.
func Dirs() (RuntimeDirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return RuntimeDirs{}, fmt.Errorf("unable to resolve user home directory: %w", err)
	}
	state := filepath.Join(home, ".flowise-spawn")
	return RuntimeDirs{
		Home:      home,
		StateDir:  state,
		EnvDir:    filepath.Join(state, "env"),
		BackupDir: filepath.Join(home, "flowise_backup"),
	}, nil
}

// InstanceInfo describes a single Flowise instance.
type InstanceInfo struct {
	Number        int
	DataDir       string // ~/.flowiseNN
	BackupDataDir string // ~/.bkpflowiseNN
	ContainerName string // flowise-instance-NN
	ServiceName   string // flowise-instance-NN
	NetworkName   string // flowise-default-XX
	HostPort      int
	InternalPort  int
	EnvFile       string
}

// NewInstanceInfo builds metadata for instance number n (0-based).
func NewInstanceInfo(n int) (InstanceInfo, error) {
	if n < 0 {
		return InstanceInfo{}, fmt.Errorf("instance number must be >= 0, got %d", n)
	}
	dirs, err := Dirs()
	if err != nil {
		return InstanceInfo{}, err
	}
	nn := fmt.Sprintf("%02d", n)
	networkIndex := (n % NetworkPoolSize) + 1
	return InstanceInfo{
		Number:        n,
		DataDir:       filepath.Join(dirs.Home, ".flowise"+nn),
		BackupDataDir: filepath.Join(dirs.Home, ".bkpflowise"+nn),
		ContainerName: "flowise-instance-" + nn,
		ServiceName:   "flowise-instance-" + nn,
		NetworkName:   fmt.Sprintf("flowise-default-%02d", networkIndex),
		HostPort:      BasePort + n,
		InternalPort:  InternalPort,
		EnvFile:       filepath.Join(dirs.EnvDir, "flowise-instance-"+nn+".env"),
	}, nil
}

// EnvVars returns the key/value pairs to write into the instance .env file.
func (info InstanceInfo) EnvVars() map[string]string {
	return map[string]string{
		"PORT":                                strconv.Itoa(info.InternalPort),
		"HOST_PORT":                           strconv.Itoa(info.HostPort),
		"CONTAINER_NAME":                      info.ContainerName,
		"HOST_PATH":                           info.DataDir,
		"CONTAINER_PATH":                      "/root/.flowise",
		"DATABASE_PATH":                       "/root/.flowise",
		"SECRETKEY_PATH":                      "/root/.flowise",
		"DEBUG":                               "true",
		"LOG_PATH":                            "/root/.flowise/logs",
		"BLOB_STORAGE_PATH":                   "/root/.flowise/storage",
		"JWT_AUTH_TOKEN_SECRET":               "16a76397b57f68471747812757c88af16a71d5f53902245ed76f583d27bbce95",
		"JWT_REFRESH_TOKEN_SECRET":            "4b7782b05056fe4ca996db2a2cb8c4f933742815cdeb8a58ac8ddef45ce49ff3",
		"JWT_ISSUER":                          "ISSUER",
		"JWT_AUDIENCE":                        "AUDIENCE",
		"JWT_TOKEN_EXPIRY_IN_MINUTES":         "360",
		"JWT_REFRESH_TOKEN_EXPIRY_IN_MINUTES": "43200",
		"EXPIRE_AUTH_TOKENS_ON_RESTART":       "false",
		"EXPRESS_SESSION_SECRET":              "flowise",
		"PASSWORD_RESET_TOKEN_EXPIRY_IN_MINS": "30",
		"PASSWORD_SALT_HASH_ROUNDS":           "10",
		"TOKEN_HASH_SECRET":                   "aeed00ba888141d51b55659f4be98bed233dcccc4cd15c18501eaa4efaaca582",
		"NETWORK_NAME":                        info.NetworkName,
	}
}

// WriteEnvFile writes the instance environment file, creating parent dirs if needed.
func (info InstanceInfo) WriteEnvFile() error {
	if err := os.MkdirAll(filepath.Dir(info.EnvFile), 0o755); err != nil {
		return fmt.Errorf("creating env directory: %w", err)
	}
	var b strings.Builder
	for k, v := range info.EnvVars() {
		b.WriteString(fmt.Sprintf("%s=%s\n", k, quoteEnv(v)))
	}
	if err := os.WriteFile(info.EnvFile, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("writing env file %s: %w", info.EnvFile, err)
	}
	return nil
}

// quoteEnv wraps a value in single quotes if it contains whitespace or special chars.
func quoteEnv(v string) string {
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, " \t\n\"'#=") {
		return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
	}
	return v
}

// ParseInstanceCount parses a positional argument as a positive integer.
func ParseInstanceCount(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid instance count %q: must be a positive integer", s)
	}
	return n, nil
}
