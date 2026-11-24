package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempConfigDir points the package at a temp config directory and
// returns it.
func withTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := configDir
	configDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { configDir = old })
	return dir
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	dir := withTempConfigDir(t)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.MCPPort != DefaultMCPPort {
		t.Fatalf("MCPPort = %d, want %d", s.MCPPort, DefaultMCPPort)
	}
	if got, want := Path(), filepath.Join(dir, "icons", "settings.json"); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withTempConfigDir(t)

	want := &Settings{MCPPort: 9001}
	if err := want.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.MCPPort != want.MCPPort {
		t.Fatalf("MCPPort = %d, want %d", got.MCPPort, want.MCPPort)
	}
}

func TestLoadCorruptFallsBackToDefaults(t *testing.T) {
	dir := withTempConfigDir(t)

	file := filepath.Join(dir, "icons", "settings.json")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.MCPPort != DefaultMCPPort {
		t.Fatalf("MCPPort = %d, want %d", s.MCPPort, DefaultMCPPort)
	}
}
