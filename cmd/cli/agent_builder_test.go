package cli

import (
	"testing"

	"github.com/lingguard/internal/config"
)

func TestAgentBuilder_OpenCodeTool(t *testing.T) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Skipf("Config not found: %v", err)
		return
	}

	t.Logf("OpenCode config: %+v", cfg.Tools.OpenCode)

	if cfg.Tools.OpenCode == nil {
		t.Error("OpenCode config is nil - tool will NOT be registered!")
		t.Skip("Skipping test - OpenCode not configured")
		return
	}

	if !cfg.Tools.OpenCode.Enabled {
		t.Error("OpenCode is not enabled")
	}

	builder := NewAgentBuilder(cfg)
	builder.InitSkills(false)

	ag, err := builder.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check if opencode tool is registered by checking tool definitions
	tools := ag.GetToolDefinitions()
	var hasOpenCode bool
	for _, tool := range tools {
		name := tool["function"].(map[string]interface{})["name"].(string)
		t.Logf("Tool: %s", name)
		if name == "opencode" {
			hasOpenCode = true
		}
	}

	if !hasOpenCode {
		t.Error("opencode tool was NOT registered!")
	} else {
		t.Log("opencode tool is registered correctly")
	}
}
