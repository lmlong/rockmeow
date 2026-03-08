// Package config 配置管理
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config 主配置结构
type Config struct {
	Providers map[string]ProviderConfig `json:"providers"`
	Agents    AgentsConfig              `json:"agents"`
	Channels  ChannelsConfig            `json:"channels"`
	Tools     ToolsConfig               `json:"tools"`
	Logging   LoggingConfig             `json:"logging"`
	Heartbeat *HeartbeatConfig          `json:"heartbeat,omitempty"` // 心跳服务配置
	Server    *ServerConfig             `json:"server,omitempty"`    // HTTP 服务器配置（统一 WebUI 和 API）
	Timeouts  *TimeoutsConfig           `json:"timeouts,omitempty"`  // 超时配置
}

// ServerConfig HTTP 服务器配置（统一管理 WebUI 和 Agent API）
type ServerConfig struct {
	Enabled bool        `json:"enabled"`         // 是否启用服务器
	Host    string      `json:"host,omitempty"`  // 监听地址，默认 127.0.0.1
	Port    int         `json:"port,omitempty"`  // 监听端口，默认 8080
	CORS    *CORSConfig `json:"cors,omitempty"`  // CORS 配置
	API     *APIConfig  `json:"api,omitempty"`   // Agent API 配置 (/v1/*)
	WebUI   *WebUIOpts  `json:"webui,omitempty"` // 内部 WebUI 配置 (/_internal/*)
}

// APIConfig Agent API 配置（对应 /v1/* 路由）
type APIConfig struct {
	Auth      *AuthConfig      `json:"auth,omitempty"`      // 认证配置
	RateLimit *RateLimitConfig `json:"rateLimit,omitempty"` // 限流配置
}

// WebUIOpts 内部 WebUI 配置（对应 /_internal/* 路由）
type WebUIOpts struct {
	TaskBoard *TaskBoardConfig `json:"taskboard,omitempty"` // 任务看板配置
	Trace     *TraceOpts       `json:"trace,omitempty"`     // 追踪配置
	WebChat   *WebChatConfig   `json:"webchat,omitempty"`   // WebChat 配置
}

// AuthConfig 认证配置
type AuthConfig struct {
	Type   string   `json:"type"`             // 认证类型: "token" | "none"
	Tokens []string `json:"tokens,omitempty"` // 有效的 Token 列表
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled     bool `json:"enabled,omitempty"`     // 是否启用限流
	RequestsPer int  `json:"requestsPer,omitempty"` // 每分钟请求数限制
	Burst       int  `json:"burst,omitempty"`       // 突发容量
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
	Workspace          string                 `json:"workspace"`
	Provider           string                 `json:"provider"`                     // 使用的 Provider 名称（文本）
	MultimodalProvider string                 `json:"multimodalProvider,omitempty"` // 多模态 Provider 名称（图片/视频），如未设置则使用 Provider
	MaxToolIterations  int                    `json:"maxToolIterations"`            // 最大工具迭代次数
	MemoryWindow       int                    `json:"memoryWindow"`                 // 历史消息窗口大小
	SystemPrompt       string                 `json:"systemPrompt"`
	MemoryConfig       *MemoryConfig          `json:"memory,omitempty"`             // 记忆系统配置
	SessionLockTimeout int                    `json:"sessionLockTimeout,omitempty"` // 会话锁超时（分钟），默认 10
	Soul               *SoulConfig            `json:"soul,omitempty"`               // Soul 人格引导配置
	SessionCompress    *SessionCompressConfig `json:"sessionCompress,omitempty"`    // 会话压缩配置
	// 注：Temperature 和 MaxTokens 从 Provider 配置中获取，避免重复
	// 注：Skills 目录固定在 ~/.lingguard/skills/
}

// SessionCompressConfig 会话压缩配置
type SessionCompressConfig struct {
	Enabled       bool   `json:"enabled,omitempty"`       // 是否启用会话压缩
	Threshold     int    `json:"threshold,omitempty"`     // 触发压缩的消息数阈值，默认 50
	KeepRecent    int    `json:"keepRecent,omitempty"`    // 保留最近N条原始消息，默认 5
	SummaryMaxLen int    `json:"summaryMaxLen,omitempty"` // 摘要最大字符数，默认 500
	SummaryPrompt string `json:"summaryPrompt,omitempty"` // 自定义摘要提示词
}

// SoulConfig Soul 人格引导配置
type SoulConfig struct {
	Enabled      bool   `json:"enabled"`                // 是否启用 Soul 引导
	DefaultSoul  string `json:"defaultSoul,omitempty"`  // 默认 Soul 定义
	GuideMessage string `json:"guideMessage,omitempty"` // 自定义引导消息
}

// MemoryConfig 记忆系统配置（参考 nanobot）
type MemoryConfig struct {
	Enabled         bool          `json:"enabled"`                   // 是否启用持久化记忆
	Path            string        `json:"path,omitempty"`            // 记忆存储路径，默认 ~/.lingguard/memory
	RecentDays      int           `json:"recentDays,omitempty"`      // 加载最近几天的日志，默认 3
	MaxHistoryLines int           `json:"maxHistoryLines,omitempty"` // 历史记录最大行数，默认 1000
	MaxDailyLogAge  int           `json:"maxDailyLogAge,omitempty"`  // 每日日志保留天数，超过则删除，0 表示永不删除
	Vector          *VectorConfig `json:"vector,omitempty"`          // 向量检索配置
	// 自动召回配置
	AutoRecall         bool    `json:"autoRecall,omitempty"`         // 是否启用自动召回，默认 true
	AutoRecallTopK     int     `json:"autoRecallTopK,omitempty"`     // 自动召回返回数量，默认 3
	AutoRecallMinScore float32 `json:"autoRecallMinScore,omitempty"` // 自动召回最小相似度，默认 0.3
	// 自动捕获配置
	AutoCapture     bool `json:"autoCapture,omitempty"`     // 是否启用自动捕获，默认 true
	CaptureMaxChars int  `json:"captureMaxChars,omitempty"` // 捕获内容最大字符数，默认 500
	// 提炼配置
	Refine *RefineConfig `json:"refine,omitempty"` // 记忆提炼配置
}

// RefineConfig 记忆提炼配置
type RefineConfig struct {
	Enabled               bool    `json:"enabled,omitempty"`               // 是否启用提炼功能
	AutoTrigger           bool    `json:"autoTrigger,omitempty"`           // [已废弃] 自动触发已移至 heartbeat 定时任务
	Threshold             int     `json:"threshold,omitempty"`             // [已废弃] 触发阈值已移至 heartbeat
	SimilarityThreshold   float32 `json:"similarityThreshold,omitempty"`   // 相似度阈值，默认 0.85
	KeepBackup            bool    `json:"keepBackup,omitempty"`            // 是否保留备份，默认 true
	MaxEntriesPerCategory int     `json:"maxEntriesPerCategory,omitempty"` // 每分类最大条目数，默认 20
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
	WebSearch *WebSearchConfig `json:"websearch,omitempty"` // 网页搜索配置
	WebFetch  *WebFetchConfig  `json:"webfetch,omitempty"`  // 网页抓取配置
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
	// Speech (ASR) - 语音识别
	Speech *SpeechConfig `json:"speech,omitempty"` // 语音识别配置
	// Cron - 定时任务
	Cron *CronConfig `json:"cron,omitempty"` // 定时任务配置
	// Moltbook - AI 社交网络
	Moltbook *MoltbookConfig `json:"moltbook,omitempty"` // Moltbook 配置
	// Calendar - CalDAV 日历
	Calendar *CalendarConfig `json:"calendar,omitempty"` // CalDAV 日历配置
}

// WebSearchConfig 网页搜索配置
type WebSearchConfig struct {
	Enabled      bool   `json:"enabled,omitempty"`      // 是否启用，默认 true
	TavilyAPIKey string `json:"tavilyApiKey,omitempty"` // Tavily Search API Key
	BochaAPIKey  string `json:"bochaApiKey,omitempty"`  // 博查AI搜索 API Key
	MaxResults   int    `json:"maxResults,omitempty"`   // 最大返回结果数，默认 5
}

// WebFetchConfig 网页抓取配置
type WebFetchConfig struct {
	Enabled  bool `json:"enabled,omitempty"`  // 是否启用，默认 true
	MaxChars int  `json:"maxChars,omitempty"` // 网页抓取最大字符数，默认 50000
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

// CalendarConfig CalDAV 日历配置
type CalendarConfig struct {
	Enabled  bool              `json:"enabled"`            // 是否启用
	Accounts []CalendarAccount `json:"accounts,omitempty"` // 日历账户列表
	Default  string            `json:"default,omitempty"`  // 默认账户名称
}

// CalendarAccount 日历账户配置
type CalendarAccount struct {
	Name     string `json:"name"`               // 账户名称
	URL      string `json:"url,omitempty"`      // CalDAV 服务器 URL
	Username string `json:"username,omitempty"` // 用户名
	Password string `json:"password,omitempty"` // 密码或应用令牌
	Token    string `json:"token,omitempty"`    // Bearer Token (可选)
	Preset   string `json:"preset,omitempty"`   // 预设模板: "yuxiao" | "apple" | "google"
	Timeout  int    `json:"timeout,omitempty"`  // 请求超时（秒），默认 30
}

// AIGCConfig AI 内容生成配置（图像/视频）
type AIGCConfig struct {
	Enabled              bool   `json:"enabled"`                        // 是否启用
	Provider             string `json:"provider,omitempty"`             // 提供商: "qwen" (通义万相)
	APIKey               string `json:"apiKey,omitempty"`               // API Key (可从 Provider 配置继承)
	APIBase              string `json:"apiBase,omitempty"`              // API 基础 URL
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

// WebChatConfig Web 聊天配置（只要配置存在就默认启用，无需 enabled 字段）
// 存储路径固定为 ~/.lingguard/webui/webchat/sessions.json
type WebChatConfig struct {
	MaxConnections      int `json:"maxConnections,omitempty"`      // 最大连接数，默认 100
	MaxConnectionsPerIP int `json:"maxConnectionsPerIP,omitempty"` // 每 IP 最大连接数，默认 5
	ReadLimitKB         int `json:"readLimitKB,omitempty"`         // 消息大小限制 KB，默认 512
	WriteTimeoutSec     int `json:"writeTimeoutSec,omitempty"`     // 写超时秒数，默认 10
	ReadTimeoutSec      int `json:"readTimeoutSec,omitempty"`      // 读超时秒数，默认 60
	HeartbeatSec        int `json:"heartbeatSec,omitempty"`        // 心跳间隔秒数，默认 30
}

// TaskBoardConfig 任务看板功能配置
type TaskBoardConfig struct {
	DBPath            string `json:"dbPath,omitempty"`            // 数据库路径，默认 ~/.lingguard/webui/taskboard.db
	TrackUserRequests bool   `json:"trackUserRequests,omitempty"` // 追踪用户请求，默认 true
	SyncSubagent      bool   `json:"syncSubagent,omitempty"`      // 同步子代理任务，默认 true
	SyncCron          bool   `json:"syncCron,omitempty"`          // 同步定时任务，默认 true
}

// TraceOpts LLM 追踪配置（内部 WebUI）
type TraceOpts struct {
	DBPath string `json:"dbPath,omitempty"` // 数据库路径，默认 ~/.lingguard/webui/trace.db
}

// CORSConfig CORS 配置
type CORSConfig struct {
	AllowedOrigins   []string `json:"allowedOrigins,omitempty"`   // 允许的源，默认 ["*"]
	AllowedMethods   string   `json:"allowedMethods,omitempty"`   // 允许的方法，默认 "GET, POST, PUT, DELETE, OPTIONS"
	AllowedHeaders   string   `json:"allowedHeaders,omitempty"`   // 允许的头，默认 "Content-Type, Authorization"
	AllowCredentials bool     `json:"allowCredentials,omitempty"` // 是否允许凭证
}

// TimeoutsConfig 超时配置
type TimeoutsConfig struct {
	// HTTP 客户端超时 (秒)
	HTTPDefault   int `json:"httpDefault,omitempty"`   // 默认 HTTP 超时，默认 30
	HTTPLong      int `json:"httpLong,omitempty"`      // 长时间操作 HTTP 超时，默认 60
	HTTPExtraLong int `json:"httpExtraLong,omitempty"` // 超长时间操作 HTTP 超时，默认 120
	// 轮询间隔 (毫秒)
	PollInterval int `json:"pollInterval,omitempty"` // 状态轮询间隔，默认 3000 (3s)
	PollShort    int `json:"pollShort,omitempty"`    // 短轮询间隔，默认 100 (100ms)
	// 重试配置
	RetryInterval int `json:"retryInterval,omitempty"` // 重试间隔 (毫秒)，默认 5000 (5s)
	RetryShort    int `json:"retryShort,omitempty"`    // 短重试间隔 (毫秒)，默认 100 (100ms)
	// 其他超时
	SessionLock int `json:"sessionLock,omitempty"` // 会话锁超时 (分钟)，默认 10
}

// DefaultConfig 默认配
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
				Path:               "~/.lingguard/memory",
				RecentDays:         3,
				MaxHistoryLines:    1000,
				AutoRecall:         true,
				AutoRecallTopK:     3,
				AutoRecallMinScore: 0.3,
				AutoCapture:        true,
				CaptureMaxChars:    500,
			},
			SessionLockTimeout: 10, // 10 分钟
			Soul: &SoulConfig{
				Enabled: false,
			},
		},
		Channels: ChannelsConfig{},
		Tools: ToolsConfig{
			RestrictToWorkspace: true,
			Workspace:           "~/.lingguard/workspace",
			Cron: &CronConfig{
				Enabled:   true,
				StorePath: "~/.lingguard/cron/jobs.json",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Heartbeat: &HeartbeatConfig{
			Enabled:  true,
			Interval: 30, // 30 分钟
		},
		Server: &ServerConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    18989,
			API:     &APIConfig{},
			WebUI: &WebUIOpts{
				TaskBoard: &TaskBoardConfig{
					DBPath:            "~/.lingguard/webui/taskboard.db",
					TrackUserRequests: true,
					SyncSubagent:      true,
					SyncCron:          true,
				},
				Trace: &TraceOpts{
					DBPath: "~/.lingguard/webui/trace.db",
				},
			},
		},
		Timeouts: &TimeoutsConfig{
			HTTPDefault:   30,
			HTTPLong:      60,
			HTTPExtraLong: 120,
			PollInterval:  3000,
			PollShort:     100,
			RetryInterval: 5000,
			RetryShort:    100,
			SessionLock:   10,
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

// Validate 验证配置
func (c *Config) Validate() error {
	var errors []string

	// 验证 Providers
	if len(c.Providers) == 0 {
		errors = append(errors, "至少需要配置一个 provider")
	} else {
		for name, p := range c.Providers {
			if p.APIKey == "" {
				errors = append(errors, fmt.Sprintf("provider '%s' 缺少 apiKey", name))
			}
			if p.Timeout < 0 {
				errors = append(errors, fmt.Sprintf("provider '%s' timeout 不能为负数", name))
			}
		}
	}

	// 验证 Agents
	if c.Agents.Provider == "" {
		errors = append(errors, "agents.provider 不能为空")
	} else if _, ok := c.Providers[c.Agents.Provider]; !ok {
		errors = append(errors, fmt.Sprintf("agents.provider '%s' 未在 providers 中定义", c.Agents.Provider))
	}
	if c.Agents.MultimodalProvider != "" {
		if _, ok := c.Providers[c.Agents.MultimodalProvider]; !ok {
			errors = append(errors, fmt.Sprintf("agents.multimodalProvider '%s' 未在 providers 中定义", c.Agents.MultimodalProvider))
		}
	}
	if c.Agents.MaxToolIterations < 0 {
		errors = append(errors, "agents.maxToolIterations 不能为负数")
	}
	if c.Agents.MemoryWindow < 0 {
		errors = append(errors, "agents.memoryWindow 不能为负数")
	}
	if c.Agents.SessionLockTimeout < 0 {
		errors = append(errors, "agents.sessionLockTimeout 不能为负数")
	}

	// 验证 Memory 配置
	if c.Agents.MemoryConfig != nil && c.Agents.MemoryConfig.Enabled {
		if c.Agents.MemoryConfig.RecentDays < 0 {
			errors = append(errors, "agents.memory.recentDays 不能为负数")
		}
		if c.Agents.MemoryConfig.MaxHistoryLines < 0 {
			errors = append(errors, "agents.memory.maxHistoryLines 不能为负数")
		}
		if c.Agents.MemoryConfig.AutoRecallTopK < 0 {
			errors = append(errors, "agents.memory.autoRecallTopK 不能为负数")
		}
		if c.Agents.MemoryConfig.CaptureMaxChars < 0 {
			errors = append(errors, "agents.memory.captureMaxChars 不能为负数")
		}
	}

	// 验证 Channels
	if c.Channels.Feishu != nil && c.Channels.Feishu.Enabled {
		if c.Channels.Feishu.AppID == "" {
			errors = append(errors, "channels.feishu.appId 不能为空")
		}
		if c.Channels.Feishu.AppSecret == "" {
			errors = append(errors, "channels.feishu.appSecret 不能为空")
		}
	}
	if c.Channels.QQ != nil && c.Channels.QQ.Enabled {
		if c.Channels.QQ.AppID == "" {
			errors = append(errors, "channels.qq.appId 不能为空")
		}
		if c.Channels.QQ.Secret == "" {
			errors = append(errors, "channels.qq.secret 不能为空")
		}
	}

	// 验证 Timeouts
	if c.Timeouts != nil {
		if c.Timeouts.HTTPDefault < 0 {
			errors = append(errors, "timeouts.httpDefault 不能为负数")
		}
		if c.Timeouts.HTTPLong < 0 {
			errors = append(errors, "timeouts.httpLong 不能为负数")
		}
		if c.Timeouts.HTTPExtraLong < 0 {
			errors = append(errors, "timeouts.httpExtraLong 不能为负数")
		}
		if c.Timeouts.PollInterval < 0 {
			errors = append(errors, "timeouts.pollInterval 不能为负数")
		}
		if c.Timeouts.SessionLock < 0 {
			errors = append(errors, "timeouts.sessionLock 不能为负数")
		}
	}

	// 验证 Server
	if c.Server != nil && c.Server.Enabled {
		if c.Server.Port < 1 || c.Server.Port > 65535 {
			errors = append(errors, "server.port 必须在 1-65535 范围内")
		}
	}

	// 验证 Logging
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Logging.Level] {
		errors = append(errors, fmt.Sprintf("logging.level '%s' 无效，必须是 debug/info/warn/error", c.Logging.Level))
	}
	validLogFormats := map[string]bool{"text": true, "json": true}
	if !validLogFormats[c.Logging.Format] {
		errors = append(errors, fmt.Sprintf("logging.format '%s' 无效，必须是 text/json", c.Logging.Format))
	}

	if len(errors) > 0 {
		return fmt.Errorf("配置验证失败:\n  - %s", strings.Join(errors, "\n  - "))
	}
	return nil
}
