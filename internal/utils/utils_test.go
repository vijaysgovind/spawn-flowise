package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListInstanceDirs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{".flowise00", ".flowise01", ".bkpflowise02", "otherdir", "flowise03"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ListInstanceDirs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 instance dirs, got %d: %+v", len(got), got)
	}

	expected := map[int]bool{0: false, 1: false, 2: true}
	for _, inst := range got {
		held, ok := expected[inst.Number]
		if !ok {
			t.Errorf("unexpected instance number %d", inst.Number)
			continue
		}
		if inst.Held != held {
			t.Errorf("instance %d held=%v, want %v", inst.Number, inst.Held, held)
		}
	}
}
