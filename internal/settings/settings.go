// Package settings stores user preferences for the app as JSON under
// <UserConfigDir>/icons/settings.json. Loading is fault-tolerant:
// a missing or corrupt file yields defaults instead of an error.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DefaultMCPPort is the MCP HTTP port used when settings carry no valid one.
const DefaultMCPPort = 9393

// Settings holds user preferences.
type Settings struct {
	MCPPort int `json:"mcpPort"`
	// LastSaveDir is the folder of the most recent export, reused as the
	// save dialog's starting folder. Empty (the zero value) lets the system
	// choose, which also covers settings files written before the field
	// existed.
	LastSaveDir string `json:"last_save_dir"`
}

// configDir is the config directory resolver; a package-level variable so
// tests can inject a temp directory.
var configDir = os.UserConfigDir

// path returns the full path of the settings file.
func path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "icons", "settings.json"), nil
}

// Path returns the settings file location, for display in the UI. It is
// empty when the OS config directory cannot be determined.
func Path() string {
	p, err := path()
	if err != nil {
		return ""
	}
	return p
}

func defaults() *Settings { return &Settings{MCPPort: DefaultMCPPort} }

// Load reads the settings file. A missing, unreadable or corrupt file
// returns a Settings with default values and a nil error.
func Load() (*Settings, error) {
	p, err := path()
	if err != nil {
		return defaults(), nil
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return defaults(), nil
	}
	s := defaults()
	if err := json.Unmarshal(raw, s); err != nil {
		return defaults(), nil
	}
	if s.MCPPort <= 0 || s.MCPPort > 65535 {
		s.MCPPort = DefaultMCPPort
	}
	return s, nil
}

// Save writes the settings back, creating the parent directory if needed.
func (s *Settings) Save() error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(raw, '\n'), 0o644)
}
