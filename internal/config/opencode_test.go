package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOpenCodeConfig(t *testing.T) {
	// Check actual user config
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".lingguard", "config.json")

	if _, err := os.Stat(configPath); err != nil {
		t.Skipf("Config file not found: %s", configPath)
		return
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	t.Logf("Config loaded from: %s", configPath)
	t.Logf("Tools.RestrictToWorkspace: %v", cfg.Tools.RestrictToWorkspace)
	t.Logf("Tools.Workspace: %s", cfg.Tools.Workspace)

	if cfg.Tools.OpenCode == nil {
		t.Error("OpenCode config is nil - NOT CONFIGURED!")
		t.Error("This is why opencode tool is not registered")
	} else {
		t.Logf("OpenCode.Enabled: %v", cfg.Tools.OpenCode.Enabled)
		t.Logf("OpenCode.BaseURL: %s", cfg.Tools.OpenCode.BaseURL)
		t.Logf("OpenCode.Timeout: %d", cfg.Tools.OpenCode.Timeout)

		if !cfg.Tools.OpenCode.Enabled {
			t.Error("OpenCode is not enabled")
		}
	}
}
