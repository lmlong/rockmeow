package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if cfg.Agents.MemoryWindow != 50 {
		t.Errorf("Expected MemoryWindow=50, got %d", cfg.Agents.MemoryWindow)
	}

	if cfg.Agents.MaxToolIterations != 20 {
		t.Errorf("Expected MaxToolIterations=20, got %d", cfg.Agents.MaxToolIterations)
	}

	if cfg.Agents.Provider != "openai" {
		t.Errorf("Expected Provider=openai, got %s", cfg.Agents.Provider)
	}

	if cfg.Storage.Type != "file" {
		t.Errorf("Expected Storage.Type=file, got %s", cfg.Storage.Type)
	}

	// 验证默认记忆配置
	if cfg.Agents.MemoryConfig == nil {
		t.Error("Expected MemoryConfig to be non-nil")
	} else {
		if !cfg.Agents.MemoryConfig.Enabled {
			t.Error("Expected MemoryConfig.Enabled=true")
		}
		// 记忆目录固定在 ~/.lingguard/memory/
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("Expected Logging.Level=info, got %s", cfg.Logging.Level)
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "lingguard-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// 创建测试配置
	cfg := DefaultConfig()
	cfg.Providers["test"] = ProviderConfig{
		APIKey:      "test-key",
		APIBase:     "https://api.test.com/v1",
		Model:       "test-model",
		Temperature: 0.7,
		MaxTokens:   4096,
	}
	cfg.Agents.Provider = "test"
	cfg.Agents.SystemPrompt = "Test prompt"
	cfg.Agents.Workspace = "/test/workspace"

	// 保存配置
	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// 加载配置
	loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证加载的配置
	if loadedCfg.Providers["test"].APIKey != "test-key" {
		t.Errorf("Expected APIKey=test-key, got %s", loadedCfg.Providers["test"].APIKey)
	}

	if loadedCfg.Agents.Provider != "test" {
		t.Errorf("Expected Provider=test, got %s", loadedCfg.Agents.Provider)
	}

	if loadedCfg.Agents.SystemPrompt != "Test prompt" {
		t.Errorf("Expected SystemPrompt='Test prompt', got %s", loadedCfg.Agents.SystemPrompt)
	}

	if loadedCfg.Agents.Workspace != "/test/workspace" {
		t.Errorf("Expected Workspace='/test/workspace', got %s", loadedCfg.Agents.Workspace)
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		input   string
		hasHome bool
	}{
		{"~/test", true},
		{"/absolute/path", false},
		{"relative/path", false},
	}

	for _, tt := range tests {
		result := expandPath(tt.input)
		if tt.hasHome && result[0] != '/' {
			t.Errorf("expandPath(%s) should expand ~ to home directory", tt.input)
		}
		if !tt.hasHome && result == tt.input {
			// 路径不变是正常的
		}
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/non/existent/path/config.json")
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
}

func TestFeishuConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Channels.Feishu = &FeishuConfig{
		Enabled:   true,
		AppID:     "cli_test",
		AppSecret: "secret",
		AllowFrom: []string{"user1", "user2"},
	}

	if !cfg.Channels.Feishu.Enabled {
		t.Error("Feishu should be enabled")
	}

	if len(cfg.Channels.Feishu.AllowFrom) != 2 {
		t.Errorf("Expected 2 allowed users, got %d", len(cfg.Channels.Feishu.AllowFrom))
	}
}
