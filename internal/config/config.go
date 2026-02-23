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
	Speech    *SpeechConfig             `json:"speech,omitempty"`    // 语音识别配置
	Cron      *CronConfig               `json:"cron,omitempty"`      // 定时任务配置
	Heartbeat *HeartbeatConfig          `json:"heartbeat,omitempty"` // 心跳服务配置
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	APIKey        string  `json:"apiKey"`
	APIBase       string  `json:"apiBase,omitempty"`
	Model         string  `json:"model,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	MaxTokens     int     `json:"maxTokens,omitempty"`
	GroupID       string  `json:"groupId,omitempty"`
	Timeout       int     `json:"timeout,omitempty"`       // 请求超时时间（秒），默认 60
	SupportsTools *bool   `json:"supportsTools,omitempty"` // 是否支持工具调用，nil 表示自动检测
}

// AgentsConfig 代理配置
type AgentsConfig struct {
	Workspace          string        `json:"workspace"`
	Provider           string        `json:"provider"`                     // 使用的 Provider 名称（文本）
	MultimodalProvider string        `json:"multimodalProvider,omitempty"` // 多模态 Provider 名称（图片/视频），如未设置则使用 Provider
	MaxToolIterations  int           `json:"maxToolIterations"`            // 最大工具迭代次数
	MemoryWindow       int           `json:"memoryWindow"`                 // 历史消息窗口大小
	SystemPrompt       string        `json:"systemPrompt"`
	MemoryConfig       *MemoryConfig `json:"memory,omitempty"` // 记忆系统配置
	// 注：Temperature 和 MaxTokens 从 Provider 配置中获取，避免重复
	// 注：Skills 目录固定在 ~/.lingguard/skills/
}

// MemoryConfig 记忆系统配置（参考 nanobot）
// 记忆文件固定存储在 ~/.lingguard/memory/ 目录下
type MemoryConfig struct {
	Enabled         bool          `json:"enabled"`                   // 是否启用持久化记忆
	RecentDays      int           `json:"recentDays,omitempty"`      // 加载最近几天的日志，默认 3
	MaxHistoryLines int           `json:"maxHistoryLines,omitempty"` // 历史记录最大行数，默认 1000
	Vector          *VectorConfig `json:"vector,omitempty"`          // 向量检索配置
	// 自动召回配置
	AutoRecall         bool    `json:"autoRecall,omitempty"`         // 是否启用自动召回，默认 true
	AutoRecallTopK     int     `json:"autoRecallTopK,omitempty"`     // 自动召回返回数量，默认 3
	AutoRecallMinScore float32 `json:"autoRecallMinScore,omitempty"` // 自动召回最小相似度，默认 0.3
	// 自动捕获配置
	AutoCapture     bool `json:"autoCapture,omitempty"`     // 是否启用自动捕获，默认 true
	CaptureMaxChars int  `json:"captureMaxChars,omitempty"` // 捕获内容最大字符数，默认 500
}

// VectorConfig 向量检索配置
type VectorConfig struct {
	Enabled   bool            `json:"enabled"`             // 是否启用向量检索
	Embedding EmbeddingConfig `json:"embedding,omitempty"` // Embedding 配置
	Search    SearchConfig    `json:"search,omitempty"`    // 搜索配置
	Database  VectorDbConfig  `json:"database,omitempty"`  // 向量数据库配置
}

// EmbeddingConfig Embedding 模型配置
type EmbeddingConfig struct {
	Provider  string `json:"provider"`            // 提供商: "qwen", "openai" 等
	Model     string `json:"model,omitempty"`     // 模型名称，默认 text-embedding-v4
	APIKey    string `json:"apiKey,omitempty"`    // API Key (可从 Provider 配置继承)
	APIBase   string `json:"apiBase,omitempty"`   // API 基础 URL
	Dimension int    `json:"dimension,omitempty"` // 向量维度，默认 1024
}

// SearchConfig 搜索配置
type SearchConfig struct {
	VectorWeight float64       `json:"vectorWeight,omitempty"` // 向量检索权重，默认 0.7
	BM25Weight   float64       `json:"bm25Weight,omitempty"`   // BM25 检索权重，默认 0.3
	TimeDecay    float64       `json:"timeDecay,omitempty"`    // 时间衰减系数，默认 0.1
	DefaultTopK  int           `json:"defaultTopK,omitempty"`  // 默认返回数量，默认 10
	MinScore     float32       `json:"minScore,omitempty"`     // 最小相似度分数，默认 0.5
	Rerank       *RerankConfig `json:"rerank,omitempty"`       // 重排序配置
}

// RerankConfig 重排序配置
type RerankConfig struct {
	Enabled  bool   `json:"enabled"`            // 是否启用重排序
	Provider string `json:"provider,omitempty"` // 提供商: "qwen"
	Model    string `json:"model,omitempty"`    // 模型名称，默认 qwen3-vl-rerank
	APIKey   string `json:"apiKey,omitempty"`   // API Key (可从 Provider 配置继承)
	APIBase  string `json:"apiBase,omitempty"`  // API 基础 URL
}

// VectorDbConfig 向量数据库配置
type VectorDbConfig struct {
	Path      string `json:"path,omitempty"`      // 数据库文件路径，默认 ~/.lingguard/memory/vectors.db
	Dimension int    `json:"dimension,omitempty"` // 向量维度，默认 1024
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
	TavilyAPIKey string `json:"tavilyApiKey,omitempty"` // Tavily Search API Key
	WebMaxChars  int    `json:"webMaxChars,omitempty"`  // 网页抓取最大字符数，默认 50000
	// ClawHub - 技能仓库
	ClawHub *ClawHubConfig `json:"clawhub,omitempty"` // ClawHub 配置
	// MCP (Model Context Protocol) servers
	MCPServers map[string]MCPServerConfig `json:"mcpServers,omitempty"` // MCP 服务器配置
	// OpenCode integration
	OpenCode *OpenCodeConfig `json:"opencode,omitempty"` // OpenCode HTTP API 配置
	// AIGC (AI Generated Content) - 图像/视频生成
	AIGC *AIGCConfig `json:"aigc,omitempty"` // AI 内容生成配置
	// TTS (Text-to-Speech) - 语音合成
	TTS *TTSConfig `json:"tts,omitempty"` // 语音合成配置
	// Moltbook - AI 社交网络
	Moltbook *MoltbookConfig `json:"moltbook,omitempty"` // Moltbook 配置
}

// ClawHubConfig ClawHub 技能仓库配置
type ClawHubConfig struct {
	Enabled  bool   `json:"enabled"`            // 是否启用 ClawHub
	APIToken string `json:"apiToken,omitempty"` // API Token，用于自动登录
}

// MoltbookConfig Moltbook AI 社交网络配置
type MoltbookConfig struct {
	Enabled   bool   `json:"enabled"`             // 是否启用
	APIKey    string `json:"apiKey,omitempty"`    // API Key (可从注册获取并存储到本地)
	AgentName string `json:"agentName,omitempty"` // Agent 名称，默认 LingGuard
	CredPath  string `json:"credPath,omitempty"`  // 凭证文件路径，默认 ~/.lingguard/moltbook/credentials.json
}

// AIGCConfig AI 内容生成配置（图像/视频）
type AIGCConfig struct {
	Enabled              bool   `json:"enabled"`                        // 是否启用
	Provider             string `json:"provider,omitempty"`             // 提供商: "qwen" (通义万相)
	APIKey               string `json:"apiKey,omitempty"`               // API Key (可从 Provider 配置继承)
	TextToImage          string `json:"textToImage,omitempty"`          // 文生图模型，默认 wan2.6-t2i
	TextToVideo          string `json:"textToVideo,omitempty"`          // 文生视频模型，默认 wan2.6-t2v
	ImageToVideo         string `json:"imageToVideo,omitempty"`         // 图生视频模型，默认 wan2.6-i2v-flash
	VideoToVideo         string `json:"videoToVideo,omitempty"`         // 参考生视频模型，默认 wan2.6-r2v
	ImageToVideoDuration int    `json:"imageToVideoDuration,omitempty"` // 图生视频时长（秒），默认 5，最大 15
	OutputDir            string `json:"outputDir,omitempty"`            // 输出目录，默认 ~/.lingguard/workspace/generated
}

// OpenCodeConfig OpenCode HTTP API 配置
type OpenCodeConfig struct {
	Enabled bool   `json:"enabled"`           // 是否启用 OpenCode 工具
	BaseURL string `json:"baseURL,omitempty"` // OpenCode 服务器地址，默认 http://127.0.0.1:4096
	Timeout int    `json:"timeout,omitempty"` // 请求超时时间（秒），默认 300
}

// MCPServerConfig MCP 服务器配置
type MCPServerConfig struct {
	Command string            `json:"command,omitempty"` // Stdio: 命令 (e.g. "npx")
	Args    []string          `json:"args,omitempty"`    // Stdio: 命令参数
	Env     map[string]string `json:"env,omitempty"`     // Stdio: 环境变量
	URL     string            `json:"url,omitempty"`     // HTTP: Streamable HTTP 端点 URL
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
	Level      string `json:"level"`                // 日志级别: debug, info, warn, error
	Format     string `json:"format"`               // 输出格式: text, json
	Output     string `json:"output,omitempty"`     // 日志文件路径
	MaxSize    int    `json:"maxSize,omitempty"`    // 单个文件最大大小(MB)，默认 10
	MaxAge     int    `json:"maxAge,omitempty"`     // 保留旧日志文件的最大天数，默认 7
	MaxBackups int    `json:"maxBackups,omitempty"` // 保留的旧日志文件最大数量，默认 5
	Compress   bool   `json:"compress,omitempty"`   // 是否压缩旧日志文件
}

// CronConfig 定时任务配置
type CronConfig struct {
	Enabled   bool   `json:"enabled"`             // 是否启用定时任务
	StorePath string `json:"storePath,omitempty"` // 任务存储路径，默认 ~/.lingguard/cron/jobs.json
}

// HeartbeatConfig 心跳服务配置
type HeartbeatConfig struct {
	Enabled  bool `json:"enabled"`            // 是否启用心跳服务
	Interval int  `json:"interval,omitempty"` // 心跳间隔（分钟），默认 30
}

// SpeechConfig 语音识别配置
type SpeechConfig struct {
	Enabled  bool   `json:"enabled"`            // 是否启用语音识别
	Provider string `json:"provider,omitempty"` // 提供商: "qwen" (阿里云通义千问 Paraformer)
	APIKey   string `json:"apiKey,omitempty"`   // API Key (可从 Provider 配置继承)
	APIBase  string `json:"apiBase,omitempty"`  // API 基础 URL
	Model    string `json:"model,omitempty"`    // 模型名称，默认 paraformer-realtime-v2
	Format   string `json:"format,omitempty"`   // 音频格式，默认 opus
	Language string `json:"language,omitempty"` // 语言，默认 zh
	Timeout  int    `json:"timeout,omitempty"`  // 超时时间（秒），默认 60
}

// TTSConfig 语音合成配置
type TTSConfig struct {
	Enabled   bool   `json:"enabled"`             // 是否启用语音合成
	Provider  string `json:"provider,omitempty"`  // 提供商: "qwen" (阿里云通义千问)
	APIKey    string `json:"apiKey,omitempty"`    // API Key (可从 Provider 配置继承)
	APIBase   string `json:"apiBase,omitempty"`   // API 基础 URL
	Model     string `json:"model,omitempty"`     // 模型名称，默认 qwen3-tts-flash
	Voice     string `json:"voice,omitempty"`     // 音色，默认 Cherry
	Language  string `json:"language,omitempty"`  // 语言，默认自动检测
	Timeout   int    `json:"timeout,omitempty"`   // 超时时间（秒），默认 60
	OutputDir string `json:"outputDir,omitempty"` // 输出目录，默认 ~/.lingguard/workspace/generated
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
				Enabled:            true,
				RecentDays:         3,
				MaxHistoryLines:    1000,
				AutoRecall:         true,
				AutoRecallTopK:     3,
				AutoRecallMinScore: 0.3,
				AutoCapture:        true,
				CaptureMaxChars:    500,
			},
		},
		Channels: ChannelsConfig{},
		Tools: ToolsConfig{
			RestrictToWorkspace: true,
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
		Heartbeat: &HeartbeatConfig{
			Enabled:  true,
			Interval: 30, // 30 分钟
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
