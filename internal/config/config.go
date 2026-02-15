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
	Cron      *CronConfig               `json:"cron,omitempty"` // 定时任务配置
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
	MemoryConfig      *MemoryConfig `json:"memory,omitempty"` // 记忆系统配置
	// 注：Temperature 和 MaxTokens 从 Provider 配置中获取，避免重复
	// 注：Skills 目录固定在 ~/.lingguard/skills/
}

// MemoryConfig 记忆系统配置（参考 nanobot）
// 记忆文件固定存储在 ~/.lingguard/memory/ 目录下
type MemoryConfig struct {
	Enabled         bool `json:"enabled"`                   // 是否启用持久化记忆
	RecentDays      int  `json:"recentDays,omitempty"`      // 加载最近几天的日志，默认 3
	MaxHistoryLines int  `json:"maxHistoryLines,omitempty"` // 历史记录最大行数，默认 1000
}

// ChannelsConfig 渠道配置
type ChannelsConfig struct {
	Feishu *FeishuConfig `json:"feishu,omitempty"`
	QQ     *QQConfig     `json:"qq,omitempty"`
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

// QQConfig QQ机器人配置
type QQConfig struct {
	Enabled   bool     `json:"enabled"`
	AppID     string   `json:"appId"`  // QQ机器人 AppID
	Secret    string   `json:"secret"` // QQ机器人 Secret
	AllowFrom []string `json:"allowFrom,omitempty"`
}

// ToolsConfig 工具配置
type ToolsConfig struct {
	RestrictToWorkspace bool   `json:"restrictToWorkspace"`
	Workspace           string `json:"workspace,omitempty"`
	// Web tools
	BraveAPIKey string `json:"braveApiKey,omitempty"` // Brave Search API Key
	WebMaxChars int    `json:"webMaxChars,omitempty"` // 网页抓取最大字符数，默认 50000
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

// CronConfig 定时任务配置
type CronConfig struct {
	Enabled   bool   `json:"enabled"`             // 是否启用定时任务
	StorePath string `json:"storePath,omitempty"` // 任务存储路径，默认 ~/.lingguard/cron/jobs.json
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
		Cron: &CronConfig{
			Enabled:   true,
			StorePath: "~/.lingguard/cron/jobs.json",
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
