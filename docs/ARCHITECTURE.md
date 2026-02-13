# LingGuard - 个人智能助手架构设计文档

## 1. 项目概述

### 1.1 项目名称
**LingGuard** - 一款基于Go语言的超轻量级个人AI智能助手

### 1.2 设计理念
参考 [nanobot](https://github.com/HKUDS/nanobot) 项目的设计思想，打造一个：
- **极简轻量**：核心代码控制在5000行以内
- **高性能**：充分利用Go的并发特性
- **易扩展**：模块化设计，支持插件机制
- **企业友好**：支持飞书等企业级即时通讯平台

### 1.3 核心特性
| 特性 | 描述 |
|------|------|
| 渠道接入 | 飞书（支持WebSocket长连接，无需公网IP） |
| 多LLM支持 | OpenAI, Anthropic, DeepSeek, 本地模型等 |
| 技能系统 | 可扩展的工具和技能插件 |
| 记忆系统 | 持久化对话记忆和上下文管理 |
| 定时任务 | Cron风格的定时执行 |
| 安全沙箱 | 工作空间限制和权限控制 |

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI / Gateway                            │
│                    (命令行 & 网关入口)                            │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                           Bus Layer                              │
│                      (消息路由层)                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  Router  │  │ Dispatcher│  │  Queue   │  │  Events  │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└─────────────────────────────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Channels    │     │      Agent      │     │   Scheduler     │
│  (渠道适配层)  │     │   (核心代理)     │     │   (定时任务)     │
│ ┌───────────┐ │     │ ┌─────────────┐ │     │ ┌─────────────┐ │
│ │  Feishu   │ │     │ │   Loop      │ │     │ │    Cron     │ │
│ │ (WebSocket)│ │     │ │   Context   │ │     │ │  Heartbeat  │ │
│ └───────────┘ │     │ │   Memory    │ │     │ └─────────────┘ │
└───────────────┘     │ │   Skills    │ │     └─────────────────┘
                      │ │   Tools     │ │
                      │ └─────────────┘ │
                      └─────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Providers Layer                           │
│                      (LLM提供商层)                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  OpenAI  │  │ Anthropic│  │DeepSeek  │  │  vLLM    │        │
│  │  OpenRouter│ │ Gemini  │  │ Moonshot │  │  Local   │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Infrastructure                             │
│                        (基础设施层)                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  Config  │  │  Storage │  │  Logger  │  │ Security │        │
│  │  Cache   │  │  Vector  │  │ Metrics  │  │ Sandbox  │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流架构

```
用户消息 ──▶ Feishu ──▶ Bus ──▶ Agent Loop
                                      │
                                      ▼
                              ┌──────────────┐
                              │ 构建Context  │
                              │ (System +    │
                              │  Memory +    │
                              │  History)    │
                              └──────────────┘
                                      │
                                      ▼
                              ┌──────────────┐
                              │   LLM调用    │
                              │ (Provider)   │
                              └──────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
              ┌──────────┐      ┌──────────┐     ┌──────────┐
              │ 文本响应 │      │ 工具调用 │     │ 技能触发 │
              └──────────┘      └──────────┘     └──────────┘
                    │                 │                 │
                    │                 ▼                 │
                    │           ┌──────────┐           │
                    │           │ 执行工具 │           │
                    │           └──────────┘           │
                    │                 │                 │
                    └─────────────────┼─────────────────┘
                                      ▼
                              ┌──────────────┐
                              │ 更新Memory   │
                              └──────────────┘
                                      │
                                      ▼
                              响应 ──▶ Feishu ──▶ 用户
```

---

## 3. 核心模块设计

### 3.1 目录结构

```
lingguard/
├── cmd/
│   ├── lingguard/          # 主程序入口
│   │   └── main.go
│   └── cli/                # CLI命令
│       ├── root.go         # 根命令
│       ├── agent.go        # agent交互命令
│       ├── gateway.go      # 网关启动命令
│       ├── cron.go         # 定时任务管理
│       └── status.go       # 状态查看
├── internal/
│   ├── agent/              # 核心代理逻辑
│   │   ├── agent.go        # Agent主结构
│   │   ├── loop.go         # Agent执行循环
│   │   ├── context.go      # 上下文构建
│   │   ├── memory.go       # 记忆管理
│   │   └── subagent.go     # 子代理支持
│   ├── tools/              # 内置工具
│   │   ├── registry.go     # 工具注册中心
│   │   ├── shell.go        # Shell执行
│   │   ├── file.go         # 文件操作
│   │   ├── web.go          # 网页抓取
│   │   ├── search.go       # 搜索工具
│   │   └── spawn.go        # 子任务生成
│   ├── skills/             # 技能系统
│   │   ├── loader.go       # 技能加载器
│   │   ├── manager.go      # 技能管理器
│   │   └── builtin/        # 内置技能
│   │       ├── github/
│   │       ├── weather/
│   │       └── calendar/
│   ├── providers/          # LLM提供商
│   │   ├── provider.go     # Provider接口
│   │   ├── registry.go     # 提供商注册
│   │   ├── openai.go       # OpenAI
│   │   ├── anthropic.go    # Anthropic
│   │   ├── deepseek.go     # DeepSeek
│   │   ├── openrouter.go   # OpenRouter
│   │   └── vllm.go         # 本地vLLM
│   ├── channels/           # 渠道集成
│   │   ├── channel.go      # Channel接口
│   │   ├── manager.go      # 渠道管理
│   │   └── feishu.go       # 飞书
│   ├── bus/                # 消息总线
│   │   ├── bus.go          # 总线核心
│   │   ├── router.go       # 消息路由
│   │   ├── dispatcher.go   # 消息分发
│   │   └── events.go       # 事件系统
│   ├── session/            # 会话管理
│   │   ├── session.go      # 会话结构
│   │   ├── manager.go      # 会话管理
│   │   └── store.go        # 会话存储
│   ├── scheduler/          # 定时任务
│   │   ├── scheduler.go    # 调度器
│   │   ├── cron.go         # Cron解析
│   │   └── job.go          # 任务定义
│   └── config/             # 配置管理
│       ├── config.go       # 配置结构
│       ├── loader.go       # 配置加载
│       └── validator.go    # 配置验证
├── pkg/
│   ├── llm/                # LLM客户端封装
│   │   ├── client.go       # 通用客户端
│   │   ├── message.go      # 消息结构
│   │   ├── stream.go       # 流式响应
│   │   └── tool.go         # 工具定义
│   ├── memory/             # 记忆系统
│   │   ├── store.go        # 存储接口
│   │   ├── sqlite.go       # SQLite实现
│   │   └── vector.go       # 向量存储
│   ├── feishu/             # 飞书SDK封装
│   │   ├── client.go       # 飞书客户端
│   │   ├── websocket.go    # WebSocket连接
│   │   ├── message.go      # 消息处理
│   │   └── crypto.go       # 加密解密
│   ├── sandbox/            # 安全沙箱
│   │   ├── sandbox.go      # 沙箱实现
│   │   └── restrict.go     # 权限限制
│   └── utils/              # 工具函数
│       ├── logger.go       # 日志
│       ├── http.go         # HTTP客户端
│       └── crypto.go       # 加密工具
├── api/
│   └── openapi/            # OpenAPI规范
│       └── spec.yaml
├── configs/
│   └── config.example.json # 配置示例
├── docs/                   # 文档
│   ├── ARCHITECTURE.md     # 架构文档
│   ├── DEVELOPMENT.md      # 开发指南
│   └── DEPLOYMENT.md       # 部署指南
├── scripts/
│   └── build.sh            # 构建脚本
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── README.md
```

### 3.2 核心接口定义

#### 3.2.1 Provider接口 (LLM提供商)

```go
// internal/providers/provider.go

package providers

import (
    "context"
    "github.com/lingguard/pkg/llm"
)

// Provider LLM提供商接口
type Provider interface {
    // Name 返回提供商名称
    Name() string

    // Complete 发送消息并获取完成响应
    Complete(ctx context.Context, req *llm.Request) (*llm.Response, error)

    // Stream 发送消息并获取流式响应
    Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error)

    // SupportsTools 是否支持工具调用
    SupportsTools() bool

    // SupportsVision 是否支持视觉
    SupportsVision() bool
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
    APIKey      string  `json:"apiKey"`
    APIBase     string  `json:"apiBase,omitempty"`
    Model       string  `json:"model,omitempty"`
    Temperature float64 `json:"temperature,omitempty"`
    MaxTokens   int     `json:"maxTokens,omitempty"`
}
```

#### 3.2.2 Channel接口 (渠道适配)

```go
// internal/channels/channel.go

package channels

import (
    "context"
    "github.com/lingguard/internal/bus"
)

// Channel 渠道接口
type Channel interface {
    // Name 返回渠道名称
    Name() string

    // Start 启动渠道
    Start(ctx context.Context) error

    // Stop 停止渠道
    Stop(ctx context.Context) error

    // Send 发送消息
    Send(ctx context.Context, userID string, message string) error

    // IsEnabled 是否启用
    IsEnabled() bool
}

// Message 渠道消息
type Message struct {
    ID        string
    Channel   string
    UserID    string
    Content   string
    Timestamp int64
    Metadata  map[string]interface{}
}

// ChannelConfig 渠道配置
type ChannelConfig struct {
    Enabled    bool     `json:"enabled"`
    AllowFrom  []string `json:"allowFrom,omitempty"`
}
```

#### 3.2.3 Tool接口 (工具系统)

```go
// internal/tools/registry.go

package tools

import (
    "context"
    "encoding/json"
)

// Tool 工具接口
type Tool interface {
    // Name 工具名称
    Name() string

    // Description 工具描述
    Description() string

    // Parameters JSON Schema格式的参数定义
    Parameters() map[string]interface{}

    // Execute 执行工具
    Execute(ctx context.Context, params json.RawMessage) (string, error)

    // IsDangerous 是否为危险操作
    IsDangerous() bool
}

// ToolCall 工具调用
type ToolCall struct {
    ID       string           `json:"id"`
    Name     string           `json:"name"`
    Arguments json.RawMessage `json:"arguments"`
}

// ToolResult 工具执行结果
type ToolResult struct {
    CallID string `json:"callId"`
    Result string `json:"result"`
    Error  string `json:"error,omitempty"`
}
```

#### 3.2.4 Skill接口 (技能系统)

```go
// internal/skills/loader.go

package skills

import (
    "context"
    "github.com/lingguard/internal/tools"
)

// Skill 技能接口
type Skill interface {
    // Name 技能名称
    Name() string

    // Description 技能描述
    Description() string

    // Triggers 触发关键词
    Triggers() []string

    // Tools 返回技能提供的工具
    Tools() []tools.Tool

    // OnLoad 技能加载时调用
    OnLoad(ctx context.Context) error

    // OnUnload 技能卸载时调用
    OnUnload(ctx context.Context) error
}
```

#### 3.2.5 Memory接口 (记忆系统)

```go
// pkg/memory/store.go

package memory

import (
    "context"
    "time"
)

// Message 记忆消息
type Message struct {
    ID        string                 `json:"id"`
    Role      string                 `json:"role"` // user, assistant, system, tool
    Content   string                 `json:"content"`
    Timestamp time.Time              `json:"timestamp"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Store 记忆存储接口
type Store interface {
    // Add 添加消息
    Add(ctx context.Context, sessionID string, msg *Message) error

    // Get 获取消息
    Get(ctx context.Context, sessionID string, limit int) ([]*Message, error)

    // Search 搜索相关记忆
    Search(ctx context.Context, query string, limit int) ([]*Message, error)

    // Clear 清除会话记忆
    Clear(ctx context.Context, sessionID string) error

    // Close 关闭存储
    Close() error
}

// VectorStore 向量存储接口(用于语义搜索)
type VectorStore interface {
    // Store 存储向量
    Store(ctx context.Context, id string, embedding []float64, metadata map[string]interface{}) error

    // Search 相似性搜索
    Search(ctx context.Context, embedding []float64, limit int) ([]SearchResult, error)

    // Delete 删除向量
    Delete(ctx context.Context, id string) error
}

// SearchResult 搜索结果
type SearchResult struct {
    ID       string
    Score    float64
    Content  string
    Metadata map[string]interface{}
}
```

### 3.3 Agent核心实现

```go
// internal/agent/agent.go

package agent

import (
    "context"
    "fmt"

    "github.com/lingguard/internal/providers"
    "github.com/lingguard/internal/tools"
    "github.com/lingguard/internal/skills"
    "github.com/lingguard/pkg/memory"
    "github.com/lingguard/pkg/llm"
)

// Agent 核心代理结构
type Agent struct {
    id           string
    provider     providers.Provider
    toolRegistry *tools.Registry
    skillManager *skills.Manager
    memory       memory.Store
    config       *AgentConfig
}

// AgentConfig 代理配置
type AgentConfig struct {
    Model               string
    SystemPrompt        string
    MaxHistoryMessages  int
    MaxToolCalls        int
    Workspace           string
    RestrictToWorkspace bool
}

// NewAgent 创建新代理
func NewAgent(cfg *AgentConfig, provider providers.Provider, mem memory.Store) *Agent {
    return &Agent{
        id:           generateID(),
        provider:     provider,
        toolRegistry: tools.NewRegistry(),
        skillManager: skills.NewManager(),
        memory:       mem,
        config:       cfg,
    }
}

// ProcessMessage 处理消息
func (a *Agent) ProcessMessage(ctx context.Context, sessionID, userMessage string) (string, error) {
    // 1. 存储用户消息
    userMsg := &memory.Message{
        Role:    "user",
        Content: userMessage,
    }
    if err := a.memory.Add(ctx, sessionID, userMsg); err != nil {
        return "", fmt.Errorf("failed to store user message: %w", err)
    }

    // 2. 构建上下文
    context, err := a.buildContext(ctx, sessionID)
    if err != nil {
        return "", fmt.Errorf("failed to build context: %w", err)
    }

    // 3. 执行代理循环
    return a.runLoop(ctx, sessionID, context)
}

// runLoop 代理执行循环
func (a *Agent) runLoop(ctx context.Context, sessionID string, messages []*llm.Message) (string, error) {
    iterations := 0

    for iterations < a.config.MaxToolCalls {
        iterations++

        // 调用LLM
        req := &llm.Request{
            Model:    a.config.Model,
            Messages: messages,
            Tools:    a.toolRegistry.GetToolDefinitions(),
        }

        resp, err := a.provider.Complete(ctx, req)
        if err != nil {
            return "", fmt.Errorf("LLM call failed: %w", err)
        }

        // 添加助手消息到历史
        if resp.Content != "" {
            assistantMsg := &memory.Message{
                Role:    "assistant",
                Content: resp.Content,
            }
            a.memory.Add(ctx, sessionID, assistantMsg)
        }

        // 检查是否有工具调用
        if len(resp.ToolCalls) == 0 {
            return resp.Content, nil
        }

        // 执行工具调用
        messages = append(messages, resp.ToMessage())

        for _, tc := range resp.ToolCalls {
            result, err := a.executeTool(ctx, tc)

            toolResult := &llm.Message{
                Role:       "tool",
                Content:    result,
                ToolCallID: tc.ID,
            }
            messages = append(messages, toolResult)
        }
    }

    return "", fmt.Errorf("max iterations reached")
}

// executeTool 执行工具
func (a *Agent) executeTool(ctx context.Context, tc *tools.ToolCall) (string, error) {
    tool, exists := a.toolRegistry.Get(tc.Name)
    if !exists {
        return "", fmt.Errorf("unknown tool: %s", tc.Name)
    }

    // 沙箱检查
    if a.config.RestrictToWorkspace && tool.IsDangerous() {
        // 在沙箱中执行
        return a.executeInSandbox(ctx, tool, tc.Arguments)
    }

    return tool.Execute(ctx, tc.Arguments)
}
```

### 3.4 Bus消息路由

```go
// internal/bus/bus.go

package bus

import (
    "context"
    "sync"
)

// Bus 消息总线
type Bus struct {
    router     *Router
    dispatcher *Dispatcher
    agentQueue chan *Event
    mu         sync.RWMutex
    handlers   map[string][]EventHandler
}

// Event 总线事件
type Event struct {
    ID        string
    Type      EventType
    Payload   interface{}
    Source    string
    Timestamp int64
}

// EventType 事件类型
type EventType string

const (
    EventMessage  EventType = "message"
    EventResponse EventType = "response"
    EventToolCall EventType = "tool_call"
    EventError    EventType = "error"
)

// EventHandler 事件处理器
type EventHandler func(ctx context.Context, event *Event) error

// NewBus 创建消息总线
func NewBus() *Bus {
    return &Bus{
        router:     NewRouter(),
        dispatcher: NewDispatcher(),
        agentQueue: make(chan *Event, 1000),
        handlers:   make(map[string][]EventHandler),
    }
}

// Publish 发布事件
func (b *Bus) Publish(ctx context.Context, event *Event) error {
    // 路由到对应的处理器
    handlers := b.router.Route(event)
    return b.dispatcher.Dispatch(ctx, event, handlers)
}

// Subscribe 订阅事件
func (b *Bus) Subscribe(eventType string, handler EventHandler) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.handlers[eventType] = append(b.handlers[eventType], handler)
}
```

### 3.5 配置管理

```go
// internal/config/config.go

package config

import (
    "encoding/json"
    "os"
    "path/filepath"
)

// Config 主配置结构
type Config struct {
    Providers ProvidersConfig `json:"providers"`
    Agents    AgentsConfig    `json:"agents"`
    Channels  ChannelsConfig  `json:"channels"`
    Tools     ToolsConfig     `json:"tools"`
    Storage   StorageConfig   `json:"storage"`
    Logging   LoggingConfig   `json:"logging"`
}

// ProvidersConfig LLM提供商配置
type ProvidersConfig struct {
    OpenRouter *ProviderConfig `json:"openrouter,omitempty"`
    OpenAI     *ProviderConfig `json:"openai,omitempty"`
    Anthropic  *ProviderConfig `json:"anthropic,omitempty"`
    DeepSeek   *ProviderConfig `json:"deepseek,omitempty"`
    Groq       *ProviderConfig `json:"groq,omitempty"`
    Gemini     *ProviderConfig `json:"gemini,omitempty"`
    VLLM       *ProviderConfig `json:"vllm,omitempty"`
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
    APIKey      string  `json:"apiKey"`
    APIBase     string  `json:"apiBase,omitempty"`
    Model       string  `json:"model,omitempty"`
    Temperature float64 `json:"temperature,omitempty"`
    MaxTokens   int     `json:"maxTokens,omitempty"`
}

// AgentsConfig 代理配置
type AgentsConfig struct {
    Defaults AgentConfig `json:"defaults"`
}

// AgentConfig 单个代理配置
type AgentConfig struct {
    Model              string `json:"model"`
    SystemPrompt       string `json:"systemPrompt,omitempty"`
    MaxHistoryMessages int    `json:"maxHistoryMessages,omitempty"`
    MaxToolCalls       int    `json:"maxToolCalls,omitempty"`
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
    Type     string `json:"type"`     // sqlite, postgres, memory
    Path     string `json:"path"`
    VectorDB string `json:"vectorDb,omitempty"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
    Level  string `json:"level"`  // debug, info, warn, error
    Format string `json:"format"` // json, text
    Output string `json:"output,omitempty"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
    return &Config{
        Providers: ProvidersConfig{},
        Agents: AgentsConfig{
            Defaults: AgentConfig{
                Model:              "anthropic/claude-opus-4",
                MaxHistoryMessages: 50,
                MaxToolCalls:       10,
            },
        },
        Channels: ChannelsConfig{},
        Tools: ToolsConfig{
            RestrictToWorkspace: false,
        },
        Storage: StorageConfig{
            Type: "sqlite",
            Path: "~/.lingguard/data.db",
        },
        Logging: LoggingConfig{
            Level:  "info",
            Format: "text",
        },
    }
}

// Load 加载配置
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(expandPath(path))
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

    dir := filepath.Dir(expandPath(path))
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }

    return os.WriteFile(expandPath(path), data, 0644)
}

func expandPath(path string) string {
    if len(path) > 0 && path[0] == '~' {
        home, _ := os.UserHomeDir()
        return filepath.Join(home, path[1:])
    }
    return path
}
```

---

## 4. 详细设计

### 4.1 LLM Provider实现

#### 4.1.1 OpenAI兼容Provider

```go
// internal/providers/openai.go

package providers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"

    "github.com/lingguard/pkg/llm"
)

// OpenAIProvider OpenAI兼容提供商
type OpenAIProvider struct {
    name     string
    apiKey   string
    apiBase  string
    client   *http.Client
    supports struct {
        tools  bool
        vision bool
    }
}

// NewOpenAIProvider 创建OpenAI提供商
func NewOpenAIProvider(name string, cfg *ProviderConfig) *OpenAIProvider {
    apiBase := cfg.APIBase
    if apiBase == "" {
        apiBase = "https://api.openai.com/v1"
    }

    return &OpenAIProvider{
        name:    name,
        apiKey:  cfg.APIKey,
        apiBase: apiBase,
        client:  &http.Client{},
        supports: struct {
            tools  bool
            vision bool
        }{
            tools:  true,
            vision: true,
        },
    }
}

func (p *OpenAIProvider) Name() string {
    return p.name
}

func (p *OpenAIProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
    body, err := json.Marshal(p.buildOpenAIRequest(req))
    if err != nil {
        return nil, err
    }

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        p.apiBase+"/chat/completions", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

    resp, err := p.client.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
    }

    var openaiResp openAIResponse
    if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
        return nil, err
    }

    return p.parseResponse(&openaiResp), nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error) {
    // 实现流式响应
    eventChan := make(chan llm.StreamEvent, 100)

    go func() {
        defer close(eventChan)
        // SSE流式处理实现...
    }()

    return eventChan, nil
}

func (p *OpenAIProvider) SupportsTools() bool { return p.supports.tools }
func (p *OpenAIProvider) SupportsVision() bool { return p.supports.vision }

// 内部结构和方法...
type openAIRequest struct {
    Model    string           `json:"model"`
    Messages []openAIMessage  `json:"messages"`
    Tools    []openAITool     `json:"tools,omitempty"`
    Stream   bool             `json:"stream,omitempty"`
}

type openAIMessage struct {
    Role       string           `json:"role"`
    Content    interface{}      `json:"content"`
    ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
    ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIResponse struct {
    ID      string `json:"id"`
    Choices []struct {
        Message struct {
            Role      string           `json:"role"`
            Content   string           `json:"content"`
            ToolCalls []openAIToolCall `json:"tool_calls"`
        } `json:"message"`
        FinishReason string `json:"finish_reason"`
    } `json:"choices"`
}

type openAIToolCall struct {
    ID       string          `json:"id"`
    Type     string          `json:"type"`
    Function openAIFunction  `json:"function"`
}

type openAIFunction struct {
    Name      string          `json:"name"`
    Arguments json.RawMessage `json:"arguments"`
}
```

### 4.2 飞书Channel实现

飞书使用 **WebSocket长连接** 模式，无需公网IP，适合内网部署。

```go
// internal/channels/feishu.go

package channels

import (
    "context"
    "crypto/aes"
    "crypto/cipher"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "sync"
    "time"

    "github.com/gorilla/websocket"
    "github.com/lingguard/internal/bus"
    "github.com/lingguard/internal/config"
)

// FeishuChannel 飞书渠道
type FeishuChannel struct {
    config      *config.FeishuConfig
    bus         *bus.Bus
    conn        *websocket.Conn
    accessToken string
    allowSet    map[string]bool
    mu          sync.RWMutex
    done        chan struct{}
}

// NewFeishuChannel 创建飞书渠道
func NewFeishuChannel(cfg *config.FeishuConfig, b *bus.Bus) *FeishuChannel {
    allowSet := make(map[string]bool)
    for _, id := range cfg.AllowFrom {
        allowSet[id] = true
    }

    return &FeishuChannel{
        config:   cfg,
        bus:      b,
        allowSet: allowSet,
        done:     make(chan struct{}),
    }
}

func (c *FeishuChannel) Name() string { return "feishu" }

func (c *FeishuChannel) IsEnabled() bool { return c.config.Enabled }

// Start 启动飞书渠道
func (c *FeishuChannel) Start(ctx context.Context) error {
    // 1. 获取access_token
    if err := c.getAccessToken(ctx); err != nil {
        return fmt.Errorf("failed to get access token: %w", err)
    }

    // 2. 获取WebSocket连接地址
    wsURL, err := c.getWebSocketURL(ctx)
    if err != nil {
        return fmt.Errorf("failed to get websocket url: %w", err)
    }

    // 3. 建立WebSocket连接
    if err := c.connectWebSocket(ctx, wsURL); err != nil {
        return fmt.Errorf("failed to connect websocket: %w", err)
    }

    // 4. 启动消息处理循环
    go c.messageLoop(ctx)

    return nil
}

// Stop 停止飞书渠道
func (c *FeishuChannel) Stop(ctx context.Context) error {
    close(c.done)
    if c.conn != nil {
        return c.conn.Close()
    }
    return nil
}

// Send 发送消息
func (c *FeishuChannel) Send(ctx context.Context, userID string, message string) error {
    c.mu.RLock()
    token := c.accessToken
    c.mu.RUnlock()

    // 构建消息体
    msgBody := map[string]interface{}{
        "receive_id": userID,
        "msg_type":   "text",
        "content":    json.RawMessage(fmt.Sprintf(`{"text":"%s"}`, message)),
    }

    body, _ := json.Marshal(msgBody)

    // 发送消息API
    req, _ := http.NewRequestWithContext(ctx, "POST",
        "https://open.feishu.cn/open-api/im/v1/messages?receive_id_type=open_id",
        bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    return nil
}

// getAccessToken 获取访问令牌
func (c *FeishuChannel) getAccessToken(ctx context.Context) error {
    body := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`,
        c.config.AppID, c.config.AppSecret)

    req, _ := http.NewRequestWithContext(ctx, "POST",
        "https://open.feishu.cn/open-api/auth/v3/tenant_access_token/internal/",
        bytes.NewReader([]byte(body)))
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    var result struct {
        Code              int    `json:"code"`
        Msg               string `json:"msg"`
        TenantAccessToken string `json:"tenant_access_token"`
        Expire            int    `json:"expire"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return err
    }

    if result.Code != 0 {
        return fmt.Errorf("feishu api error: %s", result.Msg)
    }

    c.mu.Lock()
    c.accessToken = result.TenantAccessToken
    c.mu.Unlock()

    // 定时刷新token
    go c.refreshTokenLoop(ctx, result.Expire)

    return nil
}

// getWebSocketURL 获取WebSocket连接地址
func (c *FeishuChannel) getWebSocketURL(ctx context.Context) (string, error) {
    c.mu.RLock()
    token := c.accessToken
    c.mu.RUnlock()

    req, _ := http.NewRequestWithContext(ctx, "GET",
        "https://open.feishu.cn/open-api/bot/v3/ws?", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result struct {
        Code int    `json:"code"`
        Msg  string `json:"msg"`
        Data struct {
            URL string `json:"url"`
        } `json:"data"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }

    if result.Code != 0 {
        return "", fmt.Errorf("feishu api error: %s", result.Msg)
    }

    return result.Data.URL, nil
}

// connectWebSocket 建立WebSocket连接
func (c *FeishuChannel) connectWebSocket(ctx context.Context, wsURL string) error {
    u, _ := url.Parse(wsURL)

    dialer := websocket.DefaultDialer
    conn, _, err := dialer.Dial(u.String(), nil)
    if err != nil {
        return err
    }

    c.conn = conn
    return nil
}

// messageLoop 消息处理循环
func (c *FeishuChannel) messageLoop(ctx context.Context) {
    for {
        select {
        case <-c.done:
            return
        case <-ctx.Done():
            return
        default:
            _, message, err := c.conn.ReadMessage()
            if err != nil {
                // 重连逻辑
                c.reconnect(ctx)
                continue
            }

            c.handleMessage(ctx, message)
        }
    }
}

// handleMessage 处理飞书消息
func (c *FeishuChannel) handleMessage(ctx context.Context, data []byte) {
    var event feishuEvent
    if err := json.Unmarshal(data, &event); err != nil {
        return
    }

    // 只处理消息事件
    if event.Header.EventType != "im.message.receive_v1" {
        return
    }

    // 解析消息内容
    var msgContent struct {
        Sender struct {
            SenderID struct {
                OpenID string `json:"open_id"`
            } `json:"sender_id"`
        } `json:"sender"`
        Message struct {
            MessageID string `json:"message_id"`
            Content   string `json:"content"`
            CreateTime int64 `json:"create_time"`
        } `json:"message"`
    }

    if err := json.Unmarshal(event.Event, &msgContent); err != nil {
        return
    }

    userID := msgContent.Sender.SenderID.OpenID

    // 权限检查
    if len(c.allowSet) > 0 && !c.allowSet[userID] {
        return
    }

    // 解密消息内容(如果配置了加密)
    content := msgContent.Message.Content
    if c.config.EncryptKey != "" {
        decrypted, err := c.decryptContent(content)
        if err != nil {
            return
        }
        content = decrypted
    }

    // 解析文本内容
    var textContent struct {
        Text string `json:"text"`
    }
    json.Unmarshal([]byte(content), &textContent)

    // 发布消息到总线
    busEvent := &bus.Event{
        Type: bus.EventMessage,
        Payload: &Message{
            ID:        msgContent.Message.MessageID,
            Channel:   c.Name(),
            UserID:    userID,
            Content:   textContent.Text,
            Timestamp: msgContent.Message.CreateTime,
        },
        Source: c.Name(),
    }

    c.bus.Publish(ctx, busEvent)
}

// decryptContent 解密消息内容
func (c *FeishuChannel) decryptContent(encrypted string) (string, error) {
    // AES-256-CBC解密
    key := sha256.Sum256([]byte(c.config.EncryptKey))

    ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }

    block, err := aes.NewCipher(key[:])
    if err != nil {
        return "", err
    }

    iv := ciphertext[:aes.BlockSize]
    ciphertext = ciphertext[aes.BlockSize:]

    mode := cipher.NewCBCDecrypter(block, iv)
    mode.CryptBlocks(ciphertext, ciphertext)

    // 去除padding
    padding := int(ciphertext[len(ciphertext)-1])
    ciphertext = ciphertext[:len(ciphertext)-padding]

    return string(ciphertext), nil
}

// reconnect 重连
func (c *FeishuChannel) reconnect(ctx context.Context) {
    for i := 0; i < 5; i++ {
        time.Sleep(time.Second * time.Duration(i+1))

        wsURL, err := c.getWebSocketURL(ctx)
        if err != nil {
            continue
        }

        if err := c.connectWebSocket(ctx, wsURL); err != nil {
            continue
        }

        return
    }
}

// refreshTokenLoop 定时刷新token
func (c *FeishuChannel) refreshTokenLoop(ctx context.Context, expire int) {
    ticker := time.NewTicker(time.Duration(expire-300) * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-c.done:
            return
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.getAccessToken(ctx)
        }
    }
}

// feishuEvent 飞书事件结构
type feishuEvent struct {
    Header struct {
        EventID    string `json:"event_id"`
        EventType  string `json:"event_type"`
        CreateTime string `json:"create_time"`
        Token      string `json:"token"`
    } `json:"header"`
    Event json.RawMessage `json:"event"`
}
```

### 4.3 工具实现示例

#### 4.3.1 Shell工具

```go
// internal/tools/shell.go

package tools

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "strings"
    "time"
)

// ShellTool Shell执行工具
type ShellTool struct {
    workspace string
    sandboxed bool
}

// NewShellTool 创建Shell工具
func NewShellTool(workspace string, sandboxed bool) *ShellTool {
    return &ShellTool{
        workspace: workspace,
        sandboxed: sandboxed,
    }
}

func (t *ShellTool) Name() string { return "shell" }

func (t *ShellTool) Description() string {
    return "Execute shell commands. Use with caution."
}

func (t *ShellTool) Parameters() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "command": map[string]interface{}{
                "type":        "string",
                "description": "The shell command to execute",
            },
            "timeout": map[string]interface{}{
                "type":        "integer",
                "description": "Timeout in seconds (default: 30)",
            },
        },
        "required": []string{"command"},
    }
}

func (t *ShellTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
    var p struct {
        Command string `json:"command"`
        Timeout int    `json:"timeout"`
    }

    if err := json.Unmarshal(params, &p); err != nil {
        return "", fmt.Errorf("invalid parameters: %w", err)
    }

    if p.Timeout == 0 {
        p.Timeout = 30
    }

    // 安全检查
    if t.sandboxed {
        if err := t.validateCommand(p.Command); err != nil {
            return "", err
        }
    }

    // 创建带超时的上下文
    ctx, cancel := context.WithTimeout(ctx, time.Duration(p.Timeout)*time.Second)
    defer cancel()

    // 执行命令
    cmd := exec.CommandContext(ctx, "bash", "-c", p.Command)
    if t.workspace != "" {
        cmd.Dir = t.workspace
    }

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()

    result := fmt.Sprintf("stdout:\n%s\nstderr:\n%s",
        stdout.String(), stderr.String())

    if err != nil {
        result += fmt.Sprintf("\nerror: %s", err)
    }

    return result, nil
}

func (t *ShellTool) IsDangerous() bool { return true }

func (t *ShellTool) validateCommand(cmd string) error {
    // 危险命令黑名单
    dangerous := []string{
        "rm -rf /",
        "mkfs",
        "dd if=",
        ":(){ :|:& };:",
    }

    lowerCmd := strings.ToLower(cmd)
    for _, d := range dangerous {
        if strings.Contains(lowerCmd, d) {
            return fmt.Errorf("dangerous command detected: %s", d)
        }
    }

    return nil
}
```

---

## 5. 配置示例

### 5.1 完整配置文件

```json
{
  "providers": {
    "openrouter": {
      "apiKey": "sk-or-v1-xxx",
      "apiBase": "https://openrouter.ai/api/v1"
    },
    "anthropic": {
      "apiKey": "sk-ant-xxx"
    },
    "deepseek": {
      "apiKey": "sk-xxx",
      "apiBase": "https://api.deepseek.com/v1"
    },
    "vllm": {
      "apiKey": "dummy",
      "apiBase": "http://localhost:8000/v1"
    }
  },
  "agents": {
    "defaults": {
      "model": "anthropic/claude-opus-4",
      "systemPrompt": "You are LingGuard, a helpful AI assistant.",
      "maxHistoryMessages": 50,
      "maxToolCalls": 10
    }
  },
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_xxx",
      "appSecret": "xxx",
      "encryptKey": "",
      "verificationToken": "",
      "allowFrom": []
    }
  },
  "tools": {
    "restrictToWorkspace": true,
    "workspace": "~/.lingguard/workspace"
  },
  "storage": {
    "type": "sqlite",
    "path": "~/.lingguard/data.db",
    "vectorDb": ""
  },
  "logging": {
    "level": "info",
    "format": "text",
    "output": "~/.lingguard/logs/lingguard.log"
  }
}
```

### 5.2 飞书配置说明

| 配置项 | 说明 | 获取方式 |
|--------|------|----------|
| `appId` | 应用ID | 飞书开放平台 → 应用凭证 |
| `appSecret` | 应用密钥 | 飞书开放平台 → 应用凭证 |
| `encryptKey` | 加密密钥（可选） | 事件订阅 → 加密策略 |
| `verificationToken` | 验证令牌（可选） | 事件订阅 → 验证令牌 |
| `allowFrom` | 白名单用户（可选） | 留空允许所有用户 |

**飞书应用配置步骤**：
1. 访问 [飞书开放平台](https://open.feishu.cn/)
2. 创建企业自建应用
3. 开启 **机器人** 能力
4. 配置权限：`im:message` (发送消息)
5. 配置事件：`im.message.receive_v1` (接收消息)
   - 选择 **长连接** 模式
6. 发布应用

---

## 6. CLI设计

### 6.1 命令列表

```bash
# 初始化配置
lingguard init

# 与Agent交互
lingguard agent -m "Hello"
lingguard agent  # 交互模式

# 启动网关（连接飞书）
lingguard gateway

# 查看状态
lingguard status

# 定时任务管理
lingguard cron add --name "daily" --message "Good morning!" --cron "0 9 * * *"
lingguard cron list
lingguard cron remove <job_id>
```

### 6.2 CLI实现

```go
// cmd/cli/root.go

package cli

import (
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "lingguard",
    Short: "LingGuard - Personal AI Assistant",
    Long:  `A lightweight personal AI assistant written in Go.`,
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    rootCmd.AddCommand(initCmd)
    rootCmd.AddCommand(agentCmd)
    rootCmd.AddCommand(gatewayCmd)
    rootCmd.AddCommand(statusCmd)
    rootCmd.AddCommand(cronCmd)
}
```

```go
// cmd/cli/agent.go

package cli

import (
    "bufio"
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/spf13/cobra"
)

var message string

var agentCmd = &cobra.Command{
    Use:   "agent",
    Short: "Chat with the agent",
    Run: func(cmd *cobra.Command, args []string) {
        cfg := loadConfig()

        agent := createAgent(cfg)
        ctx := context.Background()

        if message != "" {
            // 单次消息模式
            response, err := agent.ProcessMessage(ctx, "cli-session", message)
            if err != nil {
                fmt.Fprintf(os.Stderr, "Error: %v\n", err)
                os.Exit(1)
            }
            fmt.Println(response)
        } else {
            // 交互模式
            runInteractiveMode(ctx, agent)
        }
    },
}

func init() {
    agentCmd.Flags().StringVarP(&message, "message", "m", "", "Message to send")
}

func runInteractiveMode(ctx context.Context, agent *Agent) {
    reader := bufio.NewReader(os.Stdin)
    sessionID := "cli-interactive"

    fmt.Println("LingGuard Interactive Mode")
    fmt.Println("Type 'exit' or 'quit' to exit.")
    fmt.Println()

    for {
        fmt.Print("> ")
        input, _ := reader.ReadString('\n')
        input = strings.TrimSpace(input)

        if input == "" {
            continue
        }

        if input == "exit" || input == "quit" {
            fmt.Println("Goodbye!")
            break
        }

        response, err := agent.ProcessMessage(ctx, sessionID, input)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            continue
        }

        fmt.Println()
        fmt.Println(response)
        fmt.Println()
    }
}
```

---

## 7. 部署方案

### 7.1 Docker部署

```dockerfile
# Dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o lingguard ./cmd/lingguard

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /app/lingguard .

VOLUME ["/root/.lingguard"]

ENTRYPOINT ["./lingguard"]
CMD ["gateway"]
```

```yaml
# docker-compose.yml
version: '3.8'

services:
  lingguard:
    build: .
    container_name: lingguard
    volumes:
      - ~/.lingguard:/root/.lingguard
    restart: unless-stopped
    environment:
      - TZ=Asia/Shanghai
```

### 7.2 构建和运行

```bash
# 构建
make build

# Docker构建
docker build -t lingguard .

# 初始化配置
docker run -v ~/.lingguard:/root/.lingguard --rm lingguard init

# 运行网关（连接飞书）
docker run -v ~/.lingguard:/root/.lingguard lingguard gateway
```

---

## 8. 开发路线图

### Phase 1: 核心功能 (MVP)
- [x] 基础架构设计
- [ ] Agent核心循环
- [ ] Provider抽象层
- [ ] 基础工具(Shell, File)
- [ ] CLI命令
- [ ] SQLite存储

### Phase 2: 渠道集成
- [ ] 飞书WebSocket长连接
- [ ] 消息收发
- [ ] 权限控制

### Phase 3: 高级功能
- [ ] 技能系统
- [ ] 向量记忆
- [ ] 定时任务
- [ ] 多模态支持
- [ ] 子代理

### Phase 4: 优化与扩展
- [ ] 性能优化
- [ ] 监控指标
- [ ] 更多LLM提供商
- [ ] Web管理界面

---

## 9. 技术选型

| 组件 | 技术选型 | 说明 |
|------|----------|------|
| 语言 | Go 1.23+ | 高性能并发 |
| CLI框架 | Cobra | 成熟的CLI框架 |
| 配置管理 | Viper | 支持多种配置格式 |
| 日志 | Zap | 高性能结构化日志 |
| HTTP客户端 | go-resty | 简洁的HTTP客户端 |
| WebSocket | gorilla/websocket | 飞书长连接 |
| 数据库 | PostgreSQL | 生产级关系型数据库 |
| 向量数据库 | Qdrant | 高性能语义搜索 |

---

## 10. 参考资料

- [nanobot](https://github.com/HKUDS/nanobot) - 参考架构设计
- [OpenAI API](https://platform.openai.com/docs/api-reference) - LLM API规范
- [Anthropic API](https://docs.anthropic.com/) - Claude API
- [飞书开放平台](https://open.feishu.cn/document/) - 飞书开发文档
- [飞书WebSocket长连接](https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN) - 长连接模式说明
