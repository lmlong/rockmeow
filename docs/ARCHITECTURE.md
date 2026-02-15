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
| 多LLM支持 | OpenAI, Anthropic, DeepSeek, GLM, Qwen 等 |
| Provider自动匹配 | 根据模型名自动选择合适的 Provider |
| 会话管理 | 内存会话管理，支持历史消息窗口 |
| 技能系统 | 渐进式加载，按需注入技能内容 |
| 记忆系统 | 持久化对话记忆和上下文管理 |
| 安全沙箱 | 工作空间限制和权限控制 |

---

## 2. 与 nanobot 功能对比

### 2.1 对比概述

| 维度 | LingGuard | nanobot |
|------|-----------|---------|
| **编程语言** | Go 1.23+ | Python 3 |
| **代码量** | ~3,500 行 | ~3,500 行 |
| **部署方式** | 单二进制文件 | pip/uv 安装 |
| **并发模型** | Goroutine | asyncio |
| **内存占用** | 更低 | 较高 |

### 2.2 功能对比详细表

| 功能模块 | LingGuard | nanobot | 说明 |
|----------|:---------:|:-------:|------|
| **渠道支持** ||||
| 飞书 (WebSocket) | ✅ | ✅ | 两者都支持，无需公网IP |
| Telegram | ❌ | ✅ | nanobot 支持 |
| Discord | ❌ | ✅ | nanobot 支持 |
| WhatsApp | ❌ | ✅ | nanobot 支持 |
| Slack | ❌ | ✅ | nanobot 支持 |
| Email (IMAP/SMTP) | ❌ | ✅ | nanobot 支持 |
| QQ | ❌ | ✅ | nanobot 支持 |
| DingTalk (钉钉) | ❌ | ✅ | nanobot 支持 |
| Mochat | ❌ | ✅ | nanobot 支持 |
| **LLM Provider** ||||
| OpenAI | ✅ | ✅ | 通过 OpenAI 兼容 API |
| Anthropic | ✅ | ✅ | 通过 Anthropic 兼容 API |
| DeepSeek | ✅ | ✅ | 两者都支持 |
| GLM (智谱) | ✅ | ✅ | 两者都支持 |
| Qwen (通义) | ✅ | ✅ | 两者都支持 |
| MiniMax | ✅ | ✅ | 两者都支持 |
| Moonshot (Kimi) | ✅ | ✅ | 两者都支持 |
| OpenRouter | ❌ | ✅ | nanobot 支持网关模式 |
| vLLM (本地模型) | ❌ | ✅ | nanobot 支持 |
| Groq | ❌ | ✅ | nanobot 支持语音转写 |
| **核心功能** ||||
| Agent Loop | ✅ | ✅ | 核心处理循环 |
| 会话管理 | ✅ | ✅ | 多会话支持 |
| 记忆系统 | ✅ (文件持久化) | ✅ | 两者都支持 MEMORY.md 方案 |
| 工具系统 | ✅ | ✅ | Shell, File 等 |
| 技能系统 | ✅ (渐进式) | ✅ | LingGuard 支持按需加载 |
| Provider 自动匹配 | ✅ | ✅ | 根据模型名自动选择 |
| **高级功能** ||||
| 定时任务 (Cron) | ❌ | ✅ | nanobot 支持 |
| 子代理 (Subagent) | ✅ | ✅ | 两者都支持后台任务 |
| 流式响应 | ✅ | ✅ | 两者都支持实时输出 |
| Agent Social Network | ❌ | ✅ | nanobot 支持 Moltbook 等 |
| 语音转写 | ❌ | ✅ | nanobot 支持 Groq Whisper |
| Docker 支持 | ❌ | ✅ | nanobot 提供镜像 |
| 多模态 (Vision) | ⏳ | ⏳ | 两者都在规划中 |

### 2.3 LingGuard 独特优势

1. **Go 语言优势**
   - 单二进制部署，无运行时依赖
   - 更低的内存占用和更快的启动速度
   - 原生并发支持，适合高并发场景
   - 静态类型安全

2. **渐进式技能加载**
   - 默认只注入技能摘要，减少上下文占用
   - LLM 可通过 `skill` 工具按需加载完整技能内容
   - 支持 `always=true` 配置始终加载

3. **简洁架构**
   - 清晰的模块划分
   - 最小化依赖
   - 易于理解和修改

### 2.4 未来规划对比

| 功能 | LingGuard 规划 | nanobot 现状 |
|------|---------------|-------------|
| 多渠道支持 | ⏳ 计划中 | ✅ 已实现 9+ 渠道 |
| 定时任务 | ⏳ 计划中 | ✅ 已实现 |
| 持久化存储 | ⏳ 计划中 (PostgreSQL) | ✅ 已实现 |
| 向量记忆 | ⏳ 计划中 (Qdrant) | ✅ 已实现 |
| 子代理 | ✅ 已实现 | ✅ 已实现 |
| 多模态 | ⏳ 计划中 | ⏳ 规划中 |

---

## 3. 系统架构

### 3.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI / Gateway                            │
│                    (命令行 & 网关入口)                            │
└─────────────────────────────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Channels    │     │      Agent      │     │   Scheduler     │
│  (渠道适配层)  │     │   (核心代理)     │     │   (定时任务)     │
│ ┌───────────┐ │     │ ┌─────────────┐ │     │ ┌─────────────┐ │
│ │  Feishu   │ │────▶│ │   Loop      │ │     │ │    Cron     │ │
│ │ (WebSocket)│ │     │ │   Session   │ │     │ │  Heartbeat  │ │
│ │AgentAdapter│ │     │ │   Context   │ │     │ └─────────────┘ │
│ └───────────┘ │     │ │   Memory    │ │     └─────────────────┘
└───────────────┘     │ │   Tools     │ │
                      │ └─────────────┘ │
                      └─────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Providers Layer                           │
│                      (LLM提供商层)                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  OpenAI  │  │ Anthropic│  │DeepSeek  │  │   GLM    │        │
│  │  Qwen    │  │ MiniMax  │  │Moonshot  │  │  vLLM    │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│                                                                  │
│  Provider 自动匹配：根据模型名自动选择 Provider                    │
│  - "gpt-4o" → openai                                            │
│  - "claude-*" → anthropic                                        │
│  - "qwen-*" → qwen                                               │
│  - "glm-*" → glm                                                 │
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

### 3.2 数据流架构

```
用户消息 ──▶ Feishu WebSocket ──▶ AgentAdapter ──▶ Agent Loop
                                      │
                                      ▼
                              ┌──────────────┐
                              │ 构建Context  │
                              │ (System +    │
                              │  Session +   │
                              │  MemoryWindow)│
                              └──────────────┘
                                      │
                                      ▼
                              ┌──────────────┐
                              │   LLM调用    │
                              │ (Provider    │
                              │  自动匹配)   │
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
                    │           │(MaxIter) │           │
                    │           └──────────┘           │
                    │                 │                 │
                    └─────────────────┼─────────────────┘
                                      ▼
                              ┌──────────────┐
                              │ 更新Session  │
                              └──────────────┘
                                      │
                                      ▼
                              响应 ──▶ Feishu ──▶ 用户
```

### 3.3 流式响应架构

```
用户消息 ──▶ Channel ──▶ HandleMessageStream() ──▶ Agent.ProcessMessageStream()
                                                         │
                                                         ▼
                                                  Provider.Stream()
                                                         │
                         ┌───────────────────────────────┼───────────────────┐
                         ▼                               ▼                   ▼
                   StreamEvent(text)            StreamEvent(tool_start)  StreamEvent(done)
                         │                               │                   │
                         └───────────────────────────────┼───────────────────┘
                                                         ▼
                                              StreamCallback(event)
                                                         │
                         ┌───────────────────────────────┼───────────────────┐
                         ▼                               ▼                   ▼
                   CLI: fmt.Print()            飞书: sendReplyAsync()   飞书: updateReply()
```

---

## 4. 核心模块设计

### 4.1 目录结构

```
lingguard/
├── cmd/
│   ├── lingguard/          # 主程序入口
│   │   └── main.go
│   └── cli/                # CLI命令
│       ├── root.go         # 根命令
│       ├── agent.go        # agent交互命令
│       ├── gateway.go      # 网关启动命令
│       └── status.go       # 状态查看
├── internal/
│   ├── agent/              # 核心代理逻辑 ✅
│   │   └── agent.go        # Agent主结构
│   ├── session/            # 会话管理 ✅
│   │   └── manager.go      # 会话管理器
│   ├── tools/              # 内置工具 ✅
│   │   ├── registry.go     # 工具注册中心
│   │   ├── registry_manager.go # 工具管理器
│   │   ├── shell.go        # Shell执行
│   │   ├── file.go         # 文件操作
│   │   └── skill.go        # 技能加载工具
│   ├── providers/          # LLM提供商 ✅
│   │   ├── provider.go     # Provider接口
│   │   ├── registry.go     # 提供商注册
│   │   ├── spec.go         # Provider规范（自动匹配）
│   │   ├── openai.go       # OpenAI兼容
│   │   └── anthropic.go    # Anthropic兼容
│   ├── channels/           # 渠道集成 ✅
│   │   ├── channel.go      # Channel接口定义
│   │   ├── manager.go      # 渠道管理器
│   │   ├── feishu.go       # 飞书WebSocket实现
│   │   └── agent_adapter.go # Agent适配器
│   ├── skills/             # 技能系统 ✅
│   │   ├── loader.go       # 技能加载器（渐进式）
│   │   └── manager.go      # 技能管理器
│   ├── scheduler/          # 定时任务 ⏳
│   │   ├── scheduler.go    # 调度器
│   │   └── cron.go         # Cron解析
│   ├── subagent/           # 子代理系统 ✅
│   │   ├── config.go       # 子代理配置
│   │   ├── subagent.go     # 子代理实现
│   │   ├── manager.go      # 子代理管理器
│   │   └── tool.go         # Task/TaskStatus 工具
│   └── config/             # 配置管理 ✅
│       └── config.go       # 配置结构
├── pkg/
│   ├── llm/                # LLM客户端封装 ✅
│   │   └── llm.go          # 通用类型
│   ├── stream/             # 流式响应类型 ✅
│   │   └── stream.go       # StreamEvent, StreamCallback
│   ├── memory/             # 记忆系统 ✅
│   │   ├── memory.go       # 内存存储（MemoryStore）
│   │   ├── file_store.go   # 文件持久化存储（参考 nanobot）
│   │   └── context_builder.go # 记忆上下文构建器
│   └── logger/             # 日志 ✅
│       └── logger.go
├── skills/                 # 技能目录（支持多路径加载）✅
│   ├── file/               # 文件操作技能
│   │   └── SKILL.md
│   ├── git-workflow/       # Git工作流技能
│   │   ├── SKILL.md
│   │   ├── examples.md
│   │   └── reference.md
│   ├── code-review/        # 代码审查技能
│   │   └── SKILL.md
│   ├── system/             # 系统操作技能
│   │   └── SKILL.md
│   └── weather/            # 天气查询技能
│       └── SKILL.md
├── configs/
│   ├── config.json         # 主配置
│   └── config.example.json # 配置示例
├── docs/
│   ├── ARCHITECTURE.md     # 架构文档
│   └── API.md              # API文档
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 4.2 核心接口定义

#### 4.2.1 Provider接口 (LLM提供商)

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

    // Model 返回当前使用的模型
    Model() string

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
    APIKey      string
    APIBase     string
    Model       string
    Temperature float64
    MaxTokens   int
}
```

#### 4.2.2 Provider 自动匹配

```go
// internal/providers/spec.go

package providers

// ProviderSpec 定义 Provider 的匹配规则
type ProviderSpec struct {
    Name         string   // 配置中的 provider 名称
    Keywords     []string // 模型名关键词（用于自动匹配）
    APIKeyPrefix string   // API Key 前缀
}

// BuiltinSpecs 内置 Provider 规范
var BuiltinSpecs = []ProviderSpec{
    {Name: "openai", Keywords: []string{"gpt", "o1", "o3"}},
    {Name: "anthropic", Keywords: []string{"claude"}},
    {Name: "deepseek", Keywords: []string{"deepseek"}},
    {Name: "qwen", Keywords: []string{"qwen", "tongyi", "dashscope"}},
    {Name: "glm", Keywords: []string{"glm", "chatglm", "codegeex"}},
    {Name: "minimax", Keywords: []string{"minimax"}},
    {Name: "moonshot", Keywords: []string{"moonshot", "kimi"}},
    {Name: "gemini", Keywords: []string{"gemini"}},
    {Name: "groq", Keywords: []string{"llama", "mixtral", "gemma"}, APIKeyPrefix: "gsk_"},
}

// FindSpecByModel 根据模型名查找 Provider 规范
func FindSpecByModel(model string) *ProviderSpec
```

#### 4.2.3 Registry Provider 注册表

```go
// internal/providers/registry.go

// Registry 提供商注册表
type Registry struct {
    providers   map[string]Provider
    defaultName string
}

// MatchProvider 根据模型名自动匹配 Provider
func (r *Registry) MatchProvider(model string) (Provider, bool) {
    // 1. 尝试解析 "provider/model" 格式
    // 2. 检查 model 是否是已注册的 provider 名称
    // 3. 通过关键词匹配
    // 4. 返回默认 Provider
}

// SetDefault 设置默认 Provider
func (r *Registry) SetDefault(name string)
```

#### 4.2.4 会话管理

```go
// internal/session/manager.go

package session

// Session 会话
type Session struct {
    Key       string
    Messages  []*memory.Message
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Manager 会话管理器
type Manager struct {
    store    memory.Store
    sessions map[string]*Session
    window   int // 历史消息窗口大小
}

// GetOrCreate 获取或创建会话
func (m *Manager) GetOrCreate(key string) *Session

// AddMessage 添加消息
func (s *Session) AddMessage(role, content string)

// GetHistory 获取历史消息（限制窗口大小）
func (s *Session) GetHistory(window int) []*memory.Message

// Clear 清空会话
func (s *Session) Clear()
```

#### 4.2.5 Channel 接口 (消息渠道)

```go
// internal/channels/channel.go

package channels

import (
    "context"
    "github.com/lingguard/pkg/stream"
)

// Message 表示从消息平台接收的消息
type Message struct {
    ID        string         // 消息唯一ID
    SessionID string         // 会话ID (格式: "feishu-{open_id}")
    Content   string         // 消息文本内容
    Metadata  map[string]any // 平台特定元数据
}

// MessageHandler 处理消息的接口 (由 Agent 适配器实现)
type MessageHandler interface {
    HandleMessage(ctx context.Context, msg *Message) (string, error)
}

// StreamingMessageHandler 流式处理消息的接口
type StreamingMessageHandler interface {
    MessageHandler
    // HandleMessageStream 流式处理消息
    HandleMessageStream(ctx context.Context, msg *Message, callback stream.StreamCallback) error
}

// Channel 表示一个消息渠道
type Channel interface {
    Name() string
    Start(ctx context.Context) error
    Stop() error
    IsRunning() bool
}
```

#### 4.2.6 配置结构

```go
// internal/config/config.go

// AgentsConfig 代理配置（已更新）
type AgentsConfig struct {
    Workspace         string  `json:"workspace"`
    Model             string  `json:"model"`             // 默认模型/Provider名称
    MaxTokens         int     `json:"maxTokens"`         // 最大输出 tokens
    Temperature       float64 `json:"temperature"`       // 温度参数
    MaxToolIterations int     `json:"maxToolIterations"` // 最大工具迭代次数
    MemoryWindow      int     `json:"memoryWindow"`      // 历史消息窗口大小
    SystemPrompt      string  `json:"systemPrompt"`
}
```

### 4.3 Agent核心实现

```go
// internal/agent/agent.go

package agent

// Agent 核心代理结构
type Agent struct {
    id           string
    provider     providers.Provider
    toolRegistry *tools.Registry
    sessions     *session.Manager  // 会话管理
    config       *config.AgentsConfig
}

// NewAgent 创建新代理
func NewAgent(cfg *config.AgentsConfig, provider providers.Provider) *Agent {
    return &Agent{
        id:           generateID(),
        provider:     provider,
        toolRegistry: tools.NewRegistry(),
        sessions:     session.NewManager(memory.NewMemoryStore(), cfg.MemoryWindow),
        config:       cfg,
    }
}

// ProcessMessage 处理消息
func (a *Agent) ProcessMessage(ctx context.Context, sessionID, userMessage string) (string, error) {
    // 1. 获取或创建会话并添加用户消息
    s := a.sessions.GetOrCreate(sessionID)
    s.AddMessage("user", userMessage)

    // 2. 构建上下文
    messages, err := a.buildContext(sessionID)
    if err != nil {
        return "", fmt.Errorf("failed to build context: %w", err)
    }

    // 3. 执行代理循环
    return a.runLoop(ctx, sessionID, messages)
}

// buildContext 构建上下文
func (a *Agent) buildContext(sessionID string) ([]llm.Message, error) {
    messages := make([]llm.Message, 0)

    // 添加系统提示
    if a.config.SystemPrompt != "" {
        messages = append(messages, llm.Message{
            Role:    "system",
            Content: a.config.SystemPrompt,
        })
    }

    // 获取会话历史消息（使用 MemoryWindow）
    s := a.sessions.GetOrCreate(sessionID)
    for _, msg := range s.GetHistory(a.config.MemoryWindow) {
        messages = append(messages, llm.Message{
            Role:    msg.Role,
            Content: msg.Content,
        })
    }

    return messages, nil
}

// runLoop 代理执行循环
func (a *Agent) runLoop(ctx context.Context, sessionID string, messages []llm.Message) (string, error) {
    iterations := 0
    maxIterations := a.config.MaxToolIterations
    if maxIterations <= 0 {
        maxIterations = 10
    }

    for iterations < maxIterations {
        iterations++
        // ... LLM调用和工具执行逻辑
    }

    return "", fmt.Errorf("max iterations reached")
}

// ProcessMessageStream 流式处理消息
func (a *Agent) ProcessMessageStream(ctx context.Context, sessionID, userMessage string, callback stream.StreamCallback) error {
    // 1. 获取或创建会话并添加用户消息
    s := a.sessions.GetOrCreate(sessionID)
    s.AddMessage("user", userMessage)

    // 2. 构建上下文
    messages, err := a.buildContext(sessionID)
    if err != nil {
        callback(stream.NewErrorEvent(err))
        return err
    }

    // 3. 执行流式代理循环
    return a.runLoopStream(ctx, sessionID, messages, callback)
}
```

### 4.5 流式响应系统

#### 4.5.1 架构概览

```
Before (同步):
Channel -> Handler.HandleMessage() -> Agent.ProcessMessage() -> Provider.Complete() -> string

After (流式):
Channel -> Handler.HandleMessageStream() -> Agent.ProcessMessageStream() -> Provider.Stream()
         |                                                              |
         +------------------ callback <---------------------------------+
```

#### 4.5.2 流式事件类型

```go
// pkg/stream/stream.go

// StreamEventType 流式事件类型
type StreamEventType string

const (
    EventText      StreamEventType = "text"       // 文本增量内容
    EventToolStart StreamEventType = "tool_start" // 工具开始执行
    EventToolEnd   StreamEventType = "tool_end"   // 工具执行完成
    EventDone      StreamEventType = "done"       // 流式响应完成
    EventError     StreamEventType = "error"      // 发生错误
)

// StreamEvent 流式事件
type StreamEvent struct {
    Type       StreamEventType
    Content    string // 增量文本内容 (EventText)
    ToolName   string // 工具名称 (EventToolStart/EventToolEnd)
    ToolResult string // 工具执行结果 (EventToolEnd)
    ToolError  string // 工具错误信息
    Error      error  // 错误信息 (EventError)
}

// StreamCallback 流式响应回调函数
type StreamCallback func(event StreamEvent)
```

#### 4.5.3 流式 LLM 类型

```go
// pkg/llm/llm.go

// Delta 流式增量
type Delta struct {
    Role      string          `json:"role,omitempty"`
    Content   string          `json:"content,omitempty"`
    ToolCalls []DeltaToolCall `json:"tool_calls,omitempty"`
}

// DeltaToolCall 流式增量中的工具调用（包含 index 字段）
type DeltaToolCall struct {
    Index    int           `json:"index"`
    ID       string        `json:"id"`
    Type     string        `json:"type"`
    Function DeltaFunction `json:"function"`
}

// DeltaFunction 流式增量中的函数调用
type DeltaFunction struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"` // 流式时是字符串片段，需要累积
}
```

#### 4.5.4 飞书流式更新

```go
// internal/channels/feishu.go

// handleMessageStream 流式处理消息
func (f *FeishuChannel) handleMessageStream(ctx context.Context, msg *Message, replyTo string) error {
    var contentBuilder strings.Builder
    var messageID string

    return f.streamingHandler.HandleMessageStream(ctx, msg, func(event stream.StreamEvent) {
        switch event.Type {
        case stream.EventText:
            contentBuilder.WriteString(event.Content)
            // 节流更新消息
            if messageID == "" {
                messageID, _ = f.sendReplyAsync(ctx, replyTo, contentBuilder.String())
            } else {
                f.updateReply(ctx, messageID, contentBuilder.String())
            }
        case stream.EventToolStart:
            // 显示工具执行状态
        case stream.EventDone:
            // 最终更新消息
        }
    })
}

// updateReply 更新已发送的消息 (使用 PatchMessage API)
func (f *FeishuChannel) updateReply(ctx context.Context, messageID, content string) error
```

#### 4.5.5 CLI 流式输出

```go
// cmd/cli/agent.go

err := ag.ProcessMessageStream(ctx, sessionID, input, func(event stream.StreamEvent) {
    switch event.Type {
    case stream.EventText:
        fmt.Print(event.Content)
    case stream.EventToolStart:
        fmt.Printf("\n⚙️ 执行工具: %s...\n", event.ToolName)
    case stream.EventDone:
        fmt.Println()
    }
})
```

### 4.6 配置文件加载

#### 4.6.1 加载优先级

```go
// cmd/lingguard/main.go

configPath := os.Getenv("LINGGUARD_CONFIG")
if configPath == "" {
    // 1. 优先从本地 configs 目录加载
    localConfig := filepath.Join("configs", "config.json")
    if _, err := os.Stat(localConfig); err == nil {
        configPath = localConfig
    } else {
        // 2. 如果本地不存在，从用户主目录加载
        home, _ := os.UserHomeDir()
        configPath = filepath.Join(home, ".lingguard", "config.json")
    }
}
```

**配置文件查找顺序：**
1. 环境变量 `LINGGUARD_CONFIG`
2. `./configs/config.json`（本地）
3. `~/.lingguard/config.json`（用户主目录）

### 4.7 技能目录加载

#### 4.7.1 多路径支持

```go
// internal/skills/loader.go

type Loader struct {
    builtinDirs []string // 支持多个内置技能目录
    workspace   string
}

func NewLoader(builtinDirs []string, workspace string) *Loader
```

#### 4.7.2 自动发现技能目录

```go
// cmd/cli/agent.go

// 候选路径（按优先级排序）
candidatePaths := []string{
    // 1. 相对于可执行文件的 skills 目录
    filepath.Join(filepath.Dir(execPath), "skills"),
    // 2. 相对于可执行文件的上级目录
    filepath.Join(filepath.Dir(execPath), "..", "skills"),
    // 3. 用户主目录下的 .lingguard/skills
    filepath.Join(home, ".lingguard", "skills"),
    // 4. 当前工作目录下的 skills
    filepath.Join(cwd, "skills"),
}
```

**技能目录查找顺序：**
1. 可执行文件所在目录的 `skills/`
2. 可执行文件上级目录的 `skills/`
3. `~/.lingguard/skills/`
4. 当前工作目录的 `skills/`

### 4.4 子代理系统 (Subagent)

子代理系统允许主 Agent 在后台启动独立的子任务，异步执行复杂操作。

#### 4.4.1 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                      Main Agent                             │
│                                                             │
│  1. User 请求 → LLM 决定调用 task 工具                      │
│  2. TaskTool.Execute() → SubagentManager.Spawn()            │
│  3. 立即返回 task_id，子代理在 goroutine 中执行              │
│  4. 子代理完成后 → notify channel → 主 Agent 轮询/回调       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Subagent (goroutine)                     │
│                                                             │
│  - 隔离的上下文（task + context 作为初始消息）               │
│  - 限制的工具集（shell, file, skill，无 task）              │
│  - 专注的系统提示                                            │
│  - 最大 15 次迭代                                           │
│  - 完成后发送结果到 notify channel                          │
└─────────────────────────────────────────────────────────────┘
```

#### 4.4.2 核心组件

```go
// internal/subagent/subagent.go

// TaskStatus 任务状态
type TaskStatus string

const (
    StatusPending   TaskStatus = "pending"
    StatusRunning   TaskStatus = "running"
    StatusCompleted TaskStatus = "completed"
    StatusFailed    TaskStatus = "failed"
)

// Subagent 子代理
type Subagent struct {
    id          string
    task        string          // 任务描述
    context     string          // 额外上下文
    status      TaskStatus      // 当前状态
    result      string          // 执行结果
    error       string          // 错误信息
    startedAt   time.Time
    completedAt time.Time

    provider     providers.Provider
    toolRegistry *tools.Registry
    config       *SubagentConfig
}
```

```go
// internal/subagent/manager.go

// SubagentManager 子代理管理器
type SubagentManager struct {
    provider     providers.Provider
    toolRegistry *tools.Registry
    config       *SubagentConfig

    mu    sync.RWMutex
    tasks map[string]*Subagent

    notify chan *Subagent      // 结果通知通道
}
```

#### 4.4.3 工具定义

**Task 工具** - 启动后台任务

```go
// Parameters
{
    "type": "object",
    "properties": {
        "task": {
            "type": "string",
            "description": "Clear description of the task to perform"
        },
        "context": {
            "type": "string",
            "description": "Additional context or background information"
        }
    },
    "required": ["task"]
}

// 返回
{
    "task_id": "abc123",
    "status": "started",
    "message": "Task started in background. Use 'task_status' tool to check progress."
}
```

**TaskStatus 工具** - 查询任务状态

```go
// Parameters
{
    "type": "object",
    "properties": {
        "task_id": {
            "type": "string",
            "description": "The ID of the task to check"
        },
        "list": {
            "type": "boolean",
            "description": "If true, list all tasks instead of checking a specific one"
        }
    }
}

// 返回
{
    "id": "abc123",
    "task": "Analyze code structure",
    "status": "completed",
    "result": "...",
    "summary": {
        "startedAt": "2026-02-14 10:30:00",
        "completedAt": "2026-02-14 10:30:45",
        "duration": "45.2s"
    }
}
```

#### 4.4.4 配置选项

```go
// internal/subagent/config.go

type SubagentConfig struct {
    MaxIterations int      // 最大迭代次数，默认 15
    SystemPrompt  string   // 子代理系统提示模板
    EnabledTools  []string // 允许的工具白名单
}

// 默认允许的工具（不包含 task/task_status 以防止嵌套）
func DefaultEnabledTools() []string {
    return []string{"shell", "read", "write", "edit", "glob", "grep", "skill"}
}
```

#### 4.4.5 使用示例

```
User: 请在后台分析当前目录的代码结构

Agent: 好的，我将在后台启动一个任务来分析代码结构。
[调用 task 工具]
{
    "task_id": "x7k2m9p4",
    "status": "started"
}

任务已启动，ID 为 x7k2m9p4。您可以使用 task_status 工具查询进度。

User: 任务完成了吗？

Agent: [调用 task_status 工具]
{
    "status": "completed",
    "result": "代码结构分析完成..."
}
```

#### 4.4.6 与 nanobot 的差异

| 特性 | nanobot | LingGuard |
|------|---------|-----------|
| 并发模型 | asyncio | goroutine |
| 通知机制 | MessageBus 回调 | 轮询 (初期) |
| 工具隔离 | 运行时过滤 | 预配置白名单 |
| 结果投递 | 自动注入会话 | 主动查询 |

### 4.8 文件持久化记忆系统（参考 nanobot）

LingGuard 实现了与 nanobot 相同的文件持久化方案，使用简单的 Markdown 文件存储记忆。

#### 4.8.1 核心文件

| 文件 | 用途 | 说明 |
|------|------|------|
| `MEMORY.md` | 长期记忆 | 用户偏好、项目上下文、重要事实 |
| `HISTORY.md` | 事件日志 | 时间戳记录的对话和操作历史 |
| `YYYY-MM-DD.md` | 每日日志 | 当天的事件记录 |

#### 4.8.2 文件结构

**MEMORY.md 结构：**
```markdown
# Memory

This file stores long-term memories and important facts.

## User Preferences
<!-- 用户偏好设置 -->
- [2026-02-15 13:03] User prefers dark mode
- [2026-02-15 13:05] User prefers Go over Python

## Project Context
<!-- 项目上下文信息 -->
- [2026-02-15 14:00] Project uses Go 1.23+

## Important Facts
<!-- 重要事实记录 -->
```

**HISTORY.md 结构：**
```markdown
# History

This file records events and conversations in chronological order.

---

### [2026-02-15 13:05:18] Message/user
User greeted and started conversation
- session_id: cli-interactive
- role: user

---
```

#### 4.8.3 配置项

```json
{
  "agents": {
    "memory": {
      "enabled": true,
      "memoryDir": "~/.lingguard/memory",
      "recentDays": 3,
      "maxHistoryLines": 1000
    }
  }
}
```

#### 4.8.4 Memory 工具

Agent 可以使用 `memory` 工具主动记忆和回忆信息：

```json
// 记录长期记忆
{"action": "remember", "category": "User Preferences", "fact": "User prefers dark mode"}

// 搜索记忆
{"action": "recall", "query": "user preferences"}

// 记录事件到每日日志
{"action": "log", "event": "Completed important task"}

// 获取当前上下文
{"action": "context"}
```

#### 4.8.5 检索方式

- 使用 `grep` 命令搜索文本
- **无** 向量数据库
- **无** embedding 模型
- **无** RAG 管道

这与 nanobot 的 "Less is More" 哲学一致，用最简单的方案实现可靠的记忆功能。

#### 4.8.6 与 nanobot 对比

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| 存储格式 | Markdown 文件 | Markdown 文件 |
| 长期记忆 | MEMORY.md | MEMORY.md |
| 事件日志 | HISTORY.md | 事件日志 |
| 每日日志 | YYYY-MM-DD.md | 每日笔记 |
| 检索方式 | grep | grep |
| 记忆工具 | memory 工具 | 内置函数 |

---

## 5. 配置示例

### 5.1 完整配置文件

```json
{
  "providers": {
    "qwen": {
      "apiKey": "sk-xxx",
      "apiBase": "https://dashscope.aliyuncs.com/compatible-mode/v1",
      "model": "qwen3-max-2026-01-23",
      "temperature": 0.7,
      "maxTokens": 4096
    },
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic",
      "model": "glm-5",
      "temperature": 0.7,
      "maxTokens": 4096
    },
    "minimax": {
      "apiKey": "xxx",
      "apiBase": "https://api.minimaxi.com/anthropic",
      "model": "MiniMax-M2.5",
      "temperature": 0.7,
      "maxTokens": 4096
    }
  },
  "agents": {
    "workspace": "~/.lingguard/workspace",
    "provider": "glm",
    "maxToolIterations": 20,
    "memoryWindow": 50,
    "systemPrompt": "你是灵侍，一个乐于助人的 AI 助手。你可以使用工具帮助用户完成各种任务。",
    "memory": {
      "enabled": true,
      "memoryDir": "~/.lingguard/memory",
      "recentDays": 3,
      "maxHistoryLines": 1000
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
    "restrictToWorkspace": false,
    "workspace": "~/.lingguard/workspace"
  },
  "storage": {
    "type": "postgres",
    "host": "localhost",
    "port": 5432,
    "database": "lingguard",
    "username": "postgres",
    "password": "postgres",
    "sslmode": "disable",
    "vectorDbUrl": "http://localhost:6333"
  },
  "logging": {
    "level": "info",
    "format": "text",
    "output": "~/.lingguard/logs/lingguard.log"
  }
}
```

### 5.2 Provider 自动匹配说明

| model 配置值 | 匹配规则 | 使用的 Provider |
|-------------|---------|----------------|
| `"glm"` | 直接匹配 provider 名称 | glm |
| `"glm/glm-4-plus"` | 解析 `provider/model` 格式 | glm |
| `"qwen-max"` | 关键词匹配 `qwen` | qwen |
| `"gpt-4o"` | 关键词匹配 `gpt` | openai |
| `"claude-3-opus"` | 关键词匹配 `claude` | anthropic |
| `"deepseek-chat"` | 关键词匹配 `deepseek` | deepseek |

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
```

### 6.2 状态显示示例

```
LingGuard Status
================
Config: configs/config.json

Providers:
  - glm: glm-5 (configured)
  - qwen: qwen3-max-2026-01-23 (configured)
  - minimax: MiniMax-M2.5 (configured)

Agent:
  Model: glm
  Workspace: ~/.lingguard/workspace
  Max Iterations: 20
  Memory Window: 50

Channels:
  - Feishu: enabled
```

---

## 7. 开发路线图

### Phase 1: 核心功能 ✅ (已完成)

| 功能 | 状态 | 说明 |
|------|------|------|
| 配置结构简化 | ✅ | AgentsConfig 新字段，移除 Mapping |
| Provider 自动匹配 | ✅ | spec.go, MatchProvider() |
| 会话管理 | ✅ | session/manager.go |
| Agent 核心循环 | ✅ | ProcessMessage, runLoop |
| Provider 抽象层 | ✅ | OpenAI/Anthropic 兼容 |
| 基础工具 | ✅ | Shell, File |
| CLI 命令 | ✅ | init, agent, status |
| 内存存储 | ✅ | MemoryStore |

### Phase 2: 渠道集成 ✅ (已完成)

| 功能 | 状态 | 说明 |
|------|------|------|
| Channel 接口定义 | ✅ | channel.go, Message 结构体 |
| Channel 管理器 | ✅ | manager.go, 注册/启动/停止 |
| Agent 适配器 | ✅ | agent_adapter.go, 消息转发 |
| 飞书 WebSocket | ✅ | feishu.go, 使用官方 SDK |
| 消息去重 | ✅ | sync.Map 缓存已处理消息 |
| 表情反应 | ✅ | 收到消息添加 👍 |
| Interactive Card | ✅ | 使用卡片消息格式回复 |
| 权限控制 | ✅ | allowFrom 白名单 |
| Gateway 命令 | ✅ | gateway.go CLI 入口 |

### Phase 3: 技能系统 ✅ (已完成)

| 功能 | 状态 | 说明 |
|------|------|------|
| 技能加载器 | ✅ | loader.go - 加载 SKILL.md 格式技能 |
| 技能管理器 | ✅ | manager.go - 缓存和获取技能 |
| 渐进式加载 | ✅ | 默认注入摘要，按需加载完整内容 |
| 依赖检查 | ✅ | 支持检查二进制和环境变量依赖 |
| Skill 工具 | ✅ | tools/skill.go - 按需加载技能 |
| 内置技能 | ✅ | git-workflow, code-review, file, system, weather |

### Phase 4: 高级功能 ✅ (已完成)

| 功能 | 状态 | 说明 |
|------|------|------|
| 多渠道扩展 | ⏳ | Telegram, Discord, WhatsApp 等 |
| 文件持久化 | ✅ | MEMORY.md + HISTORY.md (参考 nanobot) |
| 向量记忆 | ⏳ | Qdrant 集成 |
| 定时任务 | ⏳ | scheduler/ 模块 (Cron) |
| 多模态支持 | ⏳ | Vision |
| 子代理 | ✅ | Subagent 后台任务 |
| 流式响应 | ✅ | SSE 支持，飞书消息实时更新 |
| Docker 支持 | ⏳ | 容器化部署 |

### Phase 5: 优化与扩展 (待实现)

| 功能 | 状态 | 说明 |
|------|------|------|
| 性能优化 | ⏳ | 缓存、并发优化 |
| 监控指标 | ⏳ | Prometheus |
| Web 管理界面 | ⏳ | 可选 |
| Agent Social Network | ⏳ | Moltbook, ClawdChat 集成 |

---

## 8. 技术选型

| 组件 | 技术选型 | 说明 |
|------|----------|------|
| 语言 | Go 1.23+ | 高性能并发 |
| CLI框架 | Cobra | 成熟的CLI框架 |
| 日志 | Zap | 高性能结构化日志 |
| HTTP客户端 | net/http | 标准库 |
| WebSocket | gorilla/websocket | 飞书长连接 |
| 数据库 | PostgreSQL | 生产级关系型数据库 |
| 向量数据库 | Qdrant | 高性能语义搜索 |

---

## 9. 参考资料

- [nanobot](https://github.com/HKUDS/nanobot) - 参考架构设计
- [OpenAI API](https://platform.openai.com/docs/api-reference) - LLM API规范
- [Anthropic API](https://docs.anthropic.com/) - Claude API
- [飞书开放平台](https://open.feishu.cn/document/) - 飞书开发文档
- [飞书WebSocket长连接](https://open.feishu.cn/document/ukTMukTMukTM/uYjNwUjL2YDM14iN2ATN) - 长连接模式说明

---

## 10. 项目总结

### 10.1 当前功能亮点

LingGuard 作为参考 nanobot 设计的 Go 语言实现，已实现以下核心功能：

1. **完整的 Agent Loop**
   - 支持工具调用循环
   - 可配置最大迭代次数
   - 上下文窗口管理

2. **多 LLM Provider 支持**
   - OpenAI 兼容 API
   - Anthropic 兼容 API
   - 自动 Provider 匹配

3. **渐进式技能系统**
   - YAML frontmatter + Markdown 格式
   - 默认注入摘要，按需加载
   - 支持 always=true 始终加载
   - 依赖检查机制
   - 多目录加载（可执行文件目录 + ~/.lingguard/skills）

4. **流式响应支持**
   - 实时文本输出
   - 工具执行状态显示
   - 飞书消息实时更新（PatchMessage API）
   - CLI 交互模式流式输出

5. **文件持久化记忆系统**（参考 nanobot）
   - MEMORY.md - 长期记忆存储（用户偏好、项目上下文、重要事实）
   - HISTORY.md - 时间戳事件日志
   - YYYY-MM-DD.md - 每日日志文件
   - grep 搜索检索（无需向量数据库）
   - Memory 工具支持 Agent 主动记忆和回忆

6. **飞书 WebSocket 集成**
   - 无需公网 IP
   - 消息去重
   - Interactive Card 响应
   - 权限控制

6. **安全沙箱**
   - 工作空间限制
   - 危险命令检测

7. **配置管理**
   - 多路径配置加载（本地 configs/ + ~/.lingguard/）
   - 环境变量覆盖

### 10.2 与 nanobot 的主要差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **定位** | Go 语言轻量级实现 | Python 轻量级实现 |
| **渠道** | 飞书 | 9+ 渠道 (Telegram, Discord, WhatsApp...) |
| **定时任务** | ⏳ 待实现 | ✅ Cron 支持 |
| **子代理** | ✅ 已实现 | ✅ 后台任务 |
| **流式响应** | ✅ 已实现 | ✅ 支持 |
| **部署** | 单二进制 | pip/uv 安装 |

### 10.3 适用场景

- 需要低内存占用的边缘部署
- 飞书企业环境
- Go 语言技术栈团队
- 需要自定义扩展的开发者

### 10.4 后续开发建议

1. **优先级高**
   - 多渠道支持 (Telegram, Discord)
   - 定时任务 (Cron)
   - 持久化存储

2. **优先级中**
   - 向量记忆
   - 多模态支持 (Vision)

3. **优先级低**
   - Web 管理界面
   - 监控指标
   - Agent Social Network
