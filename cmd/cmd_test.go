package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spawn-flowise/spawn-flowise/internal/config"
)

func TestEnsureComposeFile(t *testing.T) {
	base := `services:
    flowise:
        image: flowiseai/flowise:latest
        ports:
            - '${HOST_PORT:-${PORT}}:${PORT}'
        networks:
            - flowise_network

networks:
    flowise_network:
        name: ${NETWORK_NAME}
        external: true
`
	dir := t.TempDir()
	basePath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(basePath, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := config.NewInstanceInfo(0)
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureComposeFile(basePath, info); err != nil {
		t.Fatalf("ensureComposeFile failed: %v", err)
	}

	data, err := os.ReadFile(composeFileFor(info))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	if !strings.Contains(got, "flowise-instance-00:") {
		t.Errorf("expected service name flowise-instance-00 in generated compose:\n%s", got)
	}
	if !strings.Contains(got, "'0.0.0.0:${HOST_PORT:-${PORT}}:${PORT}'") {
		t.Errorf("expected 0.0.0.0 binding in generated compose:\n%s", got)
	}
	if strings.Contains(got, "    flowise:") {
		t.Errorf("old service name should be replaced:\n%s", got)
	}
}
