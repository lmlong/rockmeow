// Package config 配置管理
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 主配置结构
type Config struct {
	Providers map[string]ProviderConfig `json:"providers"`
	Agents    AgentsConfig              `json:"agents"`
	Channels  ChannelsConfig            `json:"channels"`
	Tools     ToolsConfig               `json:"tools"`
	Storage   StorageConfig             `json:"storage"`
	Logging   LoggingConfig             `json:"logging"`
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	APIKey      string  `json:"apiKey"`
	APIBase     string  `json:"apiBase,omitempty"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"maxTokens,omitempty"`
	GroupID     string  `json:"groupId,omitempty"`
	Timeout     int     `json:"timeout,omitempty"` // 请求超时时间（秒），默认 60
}

// AgentsConfig 代理配置
type AgentsConfig struct {
	Workspace         string        `json:"workspace"`
	Provider          string        `json:"provider"`          // 使用的 Provider 名称
	MaxToolIterations int           `json:"maxToolIterations"` // 最大工具迭代次数
	MemoryWindow      int           `json:"memoryWindow"`      // 历史消息窗口大小
	SystemPrompt      string        `json:"systemPrompt"`
	SkillsBuiltinDir  string        `json:"skillsBuiltinDir,omitempty"` // 内置技能目录
	SkillsWorkspace   string        `json:"skillsWorkspace,omitempty"`  // 工作区技能目录
	MemoryConfig      *MemoryConfig `json:"memory,omitempty"`           // 记忆系统配置
	// 注：Temperature 和 MaxTokens 从 Provider 配置中获取，避免重复
}

// MemoryConfig 记忆系统配置（参考 nanobot）
type MemoryConfig struct {
	Enabled         bool   `json:"enabled"`                   // 是否启用持久化记忆
	MemoryDir       string `json:"memoryDir,omitempty"`       // 记忆目录路径，默认 ~/.lingguard/memory
	RecentDays      int    `json:"recentDays,omitempty"`      // 加载最近几天的日志，默认 3
	MaxHistoryLines int    `json:"maxHistoryLines,omitempty"` // 历史记录最大行数，默认 1000
}

// ChannelsConfig 渠道配置
type ChannelsConfig struct {
	Feishu *FeishuConfig `json:"feishu,omitempty"`
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	Enabled           bool     `json:"enabled"`
	AppID             string   `json:"appId"`
	AppSecret         string   `json:"appSecret"`
	EncryptKey        string   `json:"encryptKey,omitempty"`
	VerificationToken string   `json:"verificationToken,omitempty"`
	AllowFrom         []string `json:"allowFrom,omitempty"`
}

// ToolsConfig 工具配置
type ToolsConfig struct {
	RestrictToWorkspace bool   `json:"restrictToWorkspace"`
	Workspace           string `json:"workspace,omitempty"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type        string `json:"type"`
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Database    string `json:"database,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	SSLMode     string `json:"sslmode,omitempty"`
	VectorDbURL string `json:"vectorDbUrl,omitempty"`
	Path        string `json:"path,omitempty"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Providers: make(map[string]ProviderConfig),
		Agents: AgentsConfig{
			Workspace:         "~/.lingguard/workspace",
			Provider:          "openai",
			MaxToolIterations: 20,
			MemoryWindow:      50,
			SystemPrompt:      "You are LingGuard, a helpful AI assistant.",
			MemoryConfig: &MemoryConfig{
				Enabled:         true,
				MemoryDir:       "~/.lingguard/memory",
				RecentDays:      3,
				MaxHistoryLines: 1000,
			},
		},
		Channels: ChannelsConfig{},
		Tools: ToolsConfig{
			RestrictToWorkspace: false,
			Workspace:           "~/.lingguard/workspace",
		},
		Storage: StorageConfig{
			Type: "file", // 改为文件存储
			Path: "~/.lingguard/memory",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// Load 加载配置
func Load(path string) (*Config, error) {
	expandedPath := expandPath(path)
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save 保存配置
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	expandedPath := expandPath(path)
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(expandedPath, data, 0644)
}

// expandPath 展开 ~ 为用户主目录
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
