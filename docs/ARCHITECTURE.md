# LingGuard - 个人智能助手架构设计文档

## 1. 项目概述

### 1.1 项目名称
**LingGuard** - 一款基于Go语言的超轻量级个人AI智能助手

### 1.2 设计理念
参考 [nanobot](https://github.com/HKUDS/nanobot) 项目的设计思想，打造一个：
- **极简轻量**：核心代码控制在5000行以内
- **高性能**：充分利用Go的并发特性
- **易扩展**：模块化设计，支持插件机制
- **企业友好**：支持飞书、QQ 等即时通讯平台

### 1.3 核心特性

| 特性 | 描述 |
|------|------|
| 渠道接入 | 飞书、QQ（支持WebSocket长连接，无需公网IP） |
| 多LLM支持 | OpenAI, Anthropic, DeepSeek, GLM, Qwen 等 |
| Provider自动匹配 | 根据模型名自动选择合适的 Provider |
| 会话管理 | 内存会话管理，支持历史消息窗口 |
| 技能系统 | 渐进式加载，按需注入技能内容 |
| 记忆系统 | 持久化对话记忆和上下文管理 |
| 定时任务 | Cron 调度，支持消息投递 |
| 子代理系统 | 后台异步执行复杂任务 |
| 安全沙箱 | 工作空间限制和权限控制 |

---

## 2. 与 nanobot 对比

### 2.1 语言与架构对比

| 维度 | LingGuard (Go) | nanobot (Python) |
|------|----------------|------------------|
| **编程语言** | Go 1.23+ | Python 3 |
| **代码量** | ~8,000 行 | ~3,500 行 |
| **部署方式** | 单二进制文件 | pip/uv 安装 |
| **并发模型** | Goroutine | asyncio |
| **内存占用** | ~20MB | ~100MB+ |
| **启动速度** | 毫秒级 | 秒级 |
| **类型安全** | 静态类型 | 动态类型 |

### 2.2 功能对比

| 功能模块 | LingGuard | nanobot | 差异说明 |
|----------|:---------:|:-------:|----------|
| **渠道支持** ||||
| 飞书 (WebSocket) | ✅ | ✅ | 两者都支持，无需公网IP |
| QQ (WebSocket) | ✅ | ✅ | 两者都支持私聊消息 |
| Telegram/Discord/WhatsApp | ❌ | ✅ | nanobot 支持 9+ 渠道 |
| **LLM Provider** ||||
| OpenAI/Anthropic/DeepSeek | ✅ | ✅ | 两者都支持 |
| GLM/Qwen/MiniMax/Moonshot | ✅ | ✅ | 两者都支持 |
| Provider 自动匹配 | ✅ | ✅ | 相同的 Provider Registry 机制 |
| **核心功能** ||||
| Agent Loop | ✅ | ✅ | 相同的循环迭代模式 |
| 会话管理 | ✅ | ✅ | 内存 + 窗口管理 |
| 记忆系统 | ✅ | ✅ | 相同的 MEMORY.md 方案 |
| 工具系统 | ✅ | ✅ | Shell, File 等 |
| 技能系统 | ✅ | ✅ | LingGuard 支持渐进式加载 |
| **高级功能** ||||
| 定时任务 (Cron) | ✅ | ✅ | 相同的调度机制 |
| 子代理 (Subagent) | ✅ | ✅ | 相同的后台任务模式 |
| 流式响应 | ✅ | ✅ | 两者都支持实时输出 |
| Agent Social Network | ✅ | ✅ | 两者都支持 Moltbook 社交网络 |
| 语音转写 | ❌ | ✅ | nanobot 支持 Groq Whisper |
| Docker 支持 | ❌ | ✅ | nanobot 提供镜像 |

### 2.3 实现差异详解

#### 2.3.1 并发模型

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **模型** | Goroutine (CSP) | asyncio (协程) |
| **通信** | Channel | Queue/Event |
| **同步** | sync.Mutex | asyncio.Lock |
| **优势** | 真正并行，多核利用 | 单线程，简单直观 |

```go
// LingGuard: Goroutine + Channel
go func() {
    result := subagent.Run()
    notifyChan <- result
}()

// nanobot: asyncio
async def run_task():
    result = await subagent.run()
    await notify_queue.put(result)
```

#### 2.3.2 子代理系统

| 特性 | LingGuard | nanobot |
|------|-----------|---------|
| **并发模型** | goroutine | asyncio |
| **通知机制** | Channel 轮询 | MessageBus 回调 |
| **工具隔离** | 预配置白名单 | 运行时过滤 |
| **结果获取** | 主动查询 (task_status) | 自动注入会话 |

#### 2.3.3 定时任务

| 特性 | LingGuard | nanobot |
|------|-----------|---------|
| **调度器** | robfig/cron | 自定义 timer |
| **存储格式** | JSON | JSON |
| **Cron 表达式** | ✅ 标准5字段 | ✅ 标准5字段 |
| **消息投递** | Channel Manager | MessageBus |
| **时区支持** | ✅ 支持 | ✅ 支持 |

#### 2.3.4 技能加载

| 特性 | LingGuard | nanobot |
|------|-----------|---------|
| **加载方式** | 渐进式（摘要 → 完整） | 一次性加载 |
| **工具触发** | skill 工具按需加载 | 自动注入 |
| **目录支持** | 多目录（内置 + 用户） | 单目录 |
| **依赖检查** | ✅ 支持 | ✅ 支持 |

#### 2.3.5 流式响应

| 特性 | LingGuard | nanobot |
|------|-----------|---------|
| **飞书更新** | PatchMessage API | 相同 |
| **节流机制** | 500ms 间隔 | 相同 |
| **工具状态** | EventToolStart/End | 相同 |

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
│   Channels    │     │      Agent      │     │     Cron        │
│  (渠道适配层)  │     │   (核心代理)     │     │   (定时任务)     │
│ ┌───────────┐ │     │ ┌─────────────┐ │     │ ┌─────────────┐ │
│ │  Feishu   │ │────▶│ │   Loop      │ │◀────│ │  Scheduler  │ │
│ │ (WebSocket)│ │     │ │   Session   │ │     │ │  Job Store  │ │
│ │AgentAdapter│ │     │ │   Memory    │ │     │ └─────────────┘ │
│ └───────────┘ │     │ │   Tools     │ │     └─────────────────┘
└───────────────┘     │ └─────────────┘ │
                      └─────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Providers Layer                           │
│                      (LLM提供商层)                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  OpenAI  │  │ Anthropic│  │DeepSeek  │  │   GLM    │        │
│  │  Qwen    │  │ MiniMax  │  │Moonshot  │  │OpenRouter│        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
│                                                                  │
│  Provider 自动匹配：根据模型名/API Key/API Base 自动选择         │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Infrastructure                             │
│                        (基础设施层)                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │  Config  │  │  Storage │  │  Logger  │  │ Security │        │
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
                              │  Memory +    │
                              │  Skills)     │
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
                              │ 更新Memory   │
                              └──────────────┘
                                      │
                                      ▼
                              响应 ──▶ Feishu ──▶ 用户
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
│       ├── cron.go         # 定时任务管理命令
│       └── config_cmd.go   # 配置命令
├── internal/
│   ├── agent/              # 核心代理逻辑
│   │   └── agent.go
│   ├── session/            # 会话管理
│   │   └── manager.go
│   ├── tools/              # 内置工具
│   │   ├── registry.go
│   │   ├── registry_manager.go
│   │   ├── shell.go
│   │   ├── file.go
│   │   ├── skill.go
│   │   ├── memory_tool.go
│   │   ├── cron.go
│   │   └── cron_wrapper.go
│   ├── providers/          # LLM提供商
│   │   ├── provider.go
│   │   ├── registry.go
│   │   ├── spec.go         # Provider规范
│   │   ├── openai.go
│   │   └── anthropic.go
│   ├── channels/           # 渠道集成
│   │   ├── channel.go
│   │   ├── manager.go
│   │   ├── feishu.go
│   │   ├── agent_adapter.go
│   │   └── context_adapter.go
│   ├── skills/             # 技能系统
│   │   ├── loader.go
│   │   └── manager.go
│   ├── cron/               # 定时任务
│   │   ├── types.go
│   │   └── service.go
│   ├── subagent/           # 子代理系统
│   │   ├── config.go
│   │   ├── subagent.go
│   │   ├── manager.go
│   │   └── tool.go
│   ├── config/             # 配置管理
│   │   └── config.go
│   ├── bus/                # 消息总线（预留）
│   └── scheduler/          # 调度器（预留）
├── pkg/
│   ├── llm/                # LLM客户端封装
│   │   ├── llm.go
│   │   └── llm_test.go
│   ├── stream/             # 流式响应类型
│   │   └── stream.go
│   ├── memory/             # 记忆系统
│   │   ├── memory.go
│   │   ├── file_store.go
│   │   ├── context_builder.go
│   │   └── file_store_test.go
│   └── logger/             # 日志
│       └── logger.go
├── skills/                 # 技能目录
│   └── builtin/            # 内置技能
│       ├── code-review/    # 代码审查
│       ├── file/           # 文件操作
│       ├── git-workflow/   # Git工作流
│       ├── system/         # 系统操作
│       └── weather/        # 天气查询
├── configs/
│   ├── config.json
│   └── config.example.json
└── docs/
    ├── ARCHITECTURE.md
    └── API.md
```

### 4.2 Provider 自动匹配（参考 nanobot Provider Registry）

#### 4.2.1 与 nanobot 的差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **数据结构** | ProviderSpec struct | dataclass |
| **匹配方法** | 函数式查找 | 类方法 |
| **配置覆盖** | config.json > spec.go | 相同 |
| **API格式判断** | apiBase 包含 /anthropic | 相同 |

#### 4.2.2 核心实现

```go
// internal/providers/spec.go

// ProviderSpec 定义 Provider 的完整规范
// 这是 Provider 匹配和自动配置的单一真实来源
type ProviderSpec struct {
    Name             string   // provider 名称
    Keywords         []string // 模型名关键词
    DisplayName      string   // 显示名称
    APIKeyPrefix     string   // API Key 前缀
    APIBaseKeyword   string   // API Base URL 关键词
    DefaultAPIBase   string   // 默认 API Base
    DefaultModel     string   // 默认模型
    IsAnthropic      bool     // 是否使用 Anthropic 格式
    IsGateway        bool     // 是否是网关类型
    LiteLLMPrefix    string   // 模型前缀
    SkipPrefixes     []string // 跳过已有前缀
}

// 匹配优先级（参考 nanobot）
// 1. "provider/model" 格式 -> 直接匹配 provider
// 2. model 是已注册的 provider 名称 -> 返回该 provider
// 3. 通过关键词匹配（gpt -> openai, claude -> anthropic）
// 4. 通过 API Key 前缀匹配（最长匹配）
// 5. 通过 API Base URL 关键词匹配
// 6. 返回默认 Provider
func (r *Registry) MatchProvider(model string) (Provider, *ProviderSpec)
```

### 4.3 Agent 核心循环

#### 4.3.1 与 nanobot 的差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **循环实现** | for 循环 + break | while True + return |
| **工具执行** | 同步调用 | await 异步 |
| **流式处理** | callback 函数 | async generator |

#### 4.3.2 核心实现

```go
// internal/agent/agent.go

func (a *Agent) runLoop(ctx context.Context, sessionID string, messages []llm.Message) (string, error) {
    iterations := 0
    maxIterations := a.config.MaxToolIterations  // 默认 20

    for iterations < maxIterations {
        iterations++

        // 1. 调用 LLM
        response, err := a.provider.Complete(ctx, req)

        // 2. 检查是否有工具调用
        if len(response.ToolCalls) == 0 {
            // 无工具调用，返回结果
            s.AddMessage("assistant", response.Content)
            return response.Content, nil
        }

        // 3. 添加 assistant 消息
        s.AddMessage("assistant", response.Content, response.ToolCalls)

        // 4. 执行工具
        for _, toolCall := range response.ToolCalls {
            result, err := a.executeTool(ctx, toolCall)
            s.AddMessage("tool", result, nil, toolCall.ID)
        }
    }

    return "", fmt.Errorf("max iterations reached")
}
```

### 4.4 定时任务系统 (Cron)

#### 4.4.1 与 nanobot 的差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **调度器** | robfig/cron 库 | 自定义 timer + croniter |
| **存储** | JSON 文件 | JSON 文件 |
| **执行回调** | 函数闭包 | asyncio 协程 |
| **时区支持** | ✅ time.LoadLocation | ✅ pytz |
| **投递机制** | Channel.SendMessage | MessageBus.publish |

#### 4.4.2 核心实现

```go
// internal/cron/service.go

type Service struct {
    storePath string
    onJob     JobCallback  // 任务执行回调
    store     *CronStore
    timer     *time.Timer
    running   bool
}

// 任务执行回调类型
type JobCallback func(job *CronJob) (string, error)

// 添加任务
func (s *Service) AddJob(name string, schedule CronSchedule, message string, opts ...JobOption) (*CronJob, error)

// 调度类型
type ScheduleKind string
const (
    ScheduleKindAt    ScheduleKind = "at"    // 一次性任务
    ScheduleKindEvery ScheduleKind = "every" // 重复任务
    ScheduleKindCron  ScheduleKind = "cron"  // cron 表达式
)
```

#### 4.4.3 Gateway 集成

```go
// cmd/cli/gateway.go

// 创建任务执行回调
onJob := func(job *cron.CronJob) (string, error) {
    // 执行 Agent 处理消息
    response, err := ag.ProcessMessage(ctx, sessionID, job.Payload.Message)

    // 如果需要投递到渠道
    if job.Payload.Deliver && job.Payload.Channel != "" {
        mgr.SendMessage(job.Payload.Channel, job.Payload.To, response)
    }

    return response, err
}

cronService := cron.NewService(storePath, onJob)
cronService.Start()
```

### 4.5 子代理系统 (Subagent)

#### 4.5.1 与 nanobot 的差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **并发模型** | goroutine | asyncio |
| **通知机制** | Channel 轮询 | MessageBus 回调 |
| **工具隔离** | 预配置白名单 | 运行时过滤 |
| **嵌套防止** | 白名单排除 task | 运行时检查 |

#### 4.5.2 核心实现

```go
// internal/subagent/manager.go

type SubagentManager struct {
    provider     providers.Provider
    toolRegistry *tools.Registry
    config       *SubagentConfig
    mu           sync.RWMutex
    tasks        map[string]*Subagent
    notify       chan *Subagent  // 结果通知通道
}

// 启动后台任务
func (m *SubagentManager) Spawn(task, context string) *Subagent {
    sub := &Subagent{
        id:      generateID(),
        task:    task,
        context: context,
        status:  StatusPending,
    }

    go func() {
        sub.status = StatusRunning
        result := sub.run()  // 执行子代理循环
        sub.result = result
        sub.status = StatusCompleted
        m.notify <- sub  // 通知完成
    }()

    return sub
}

// 默认允许的工具（不包含 task 以防止嵌套）
func DefaultEnabledTools() []string {
    return []string{"shell", "read", "write", "edit", "glob", "grep", "skill"}
}
```

### 4.6 记忆系统（参考 nanobot）

#### 4.6.1 与 nanobot 的差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **存储格式** | Markdown 文件 | Markdown 文件 |
| **长期记忆** | MEMORY.md | MEMORY.md |
| **事件日志** | HISTORY.md | 事件日志 |
| **每日日志** | YYYY-MM-DD.md | 每日笔记 |
| **检索方式** | grep | grep |
| **记忆工具** | memory 工具 | 内置函数 |

#### 4.6.2 文件结构

```
~/.lingguard/memory/
├── MEMORY.md          # 长期记忆
├── HISTORY.md         # 事件日志
└── 2026-02-15.md      # 每日日志
```

**MEMORY.md 结构：**
```markdown
# Memory

## User Preferences
- [2026-02-15 13:03] User prefers dark mode

## Project Context
- [2026-02-15 14:00] Project uses Go 1.23+

## Important Facts
```

### 4.7 技能系统

#### 4.7.1 与 nanobot 的差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **加载方式** | 渐进式（摘要 → 完整） | 一次性加载 |
| **工具触发** | skill 工具按需加载 | 自动注入上下文 |
| **目录支持** | 多目录（内置 + 用户） | 单目录 |
| **格式** | YAML frontmatter + MD | 相同 |

#### 4.7.2 渐进式加载

```go
// 默认只注入摘要
func (l *Loader) GetSummaries() string {
    // 返回所有技能的 name + description
}

// 按需加载完整内容
func (l *Loader) LoadSkill(name string) (*Skill, error) {
    // 返回完整的 SKILL.md 内容
}
```

### 4.8 流式响应系统

#### 4.8.1 与 nanobot 的差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **事件类型** | text/tool_start/tool_end/done/error | 相同 |
| **飞书更新** | PatchMessage API | 相同 |
| **节流机制** | 500ms 间隔 | 相同 |

#### 4.8.2 事件类型

```go
// pkg/stream/stream.go

type StreamEventType string

const (
    EventText      StreamEventType = "text"       // 文本增量
    EventToolStart StreamEventType = "tool_start" // 工具开始
    EventToolEnd   StreamEventType = "tool_end"   // 工具完成
    EventDone      StreamEventType = "done"       // 完成
    EventError     StreamEventType = "error"      // 错误
)

type StreamCallback func(event StreamEvent)
```

---

## 5. 配置管理

### 5.1 配置加载优先级

| 优先级 | 来源 | 路径 |
|--------|------|------|
| 1 | 环境变量 | `$LINGGUARD_CONFIG` |
| 2 | 当前目录 | `./config.json` |
| 3 | 用户目录 | `~/.lingguard/config.json` |

### 5.2 配置覆盖机制

config.json 配置 > spec.go 默认值

| 配置项 | config.json | spec.go |
|--------|-------------|---------|
| apiBase | ✅ 覆盖 | 默认值 |
| model | ✅ 覆盖 | 默认值 |
| IsAnthropic | 根据 apiBase 判断 | 默认值 |

### 5.3 完整配置示例

```json
{
  "providers": {
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic",
      "model": "glm-5"
    },
    "deepseek": {
      "apiKey": "sk-xxx"
    }
  },
  "agents": {
    "workspace": "~/.lingguard/workspace",
    "provider": "glm",
    "maxToolIterations": 20,
    "memoryWindow": 50,
    "systemPrompt": "你是灵侍，一个乐于助人的 AI 助手。",
    "memory": {
      "enabled": true,
      "recentDays": 3
    }
  },
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_xxx",
      "appSecret": "xxx"
    }
  },
  "cron": {
    "enabled": true,
    "storePath": "~/.lingguard/cron/jobs.json"
  },
  "logging": {
    "level": "info",
    "format": "text"
  }
}
```

---

## 6. CLI 命令

```bash
# Agent 交互
lingguard agent              # 交互模式
lingguard agent -m "Hello"   # 单次消息

# Gateway 模式
lingguard gateway            # 启动网关

# 定时任务
lingguard cron add "Name" "every:1h" "Message"
lingguard cron list
lingguard cron remove <id>
lingguard cron run <id> --force

# 状态查看
lingguard status
```

---

## 7. MCP 支持

### 7.1 概述

LingGuard 支持 Model Context Protocol (MCP)，可以连接外部工具服务器扩展能力。

### 7.2 传输方式

| 传输 | 文件 | 说明 |
|------|------|------|
| Stdio | `internal/tools/mcp.go` | 通过子进程启动 MCP 服务器 |
| HTTP | `internal/tools/mcp_http.go` | 连接 HTTP/Streamable HTTP 端点 |

### 7.3 核心组件

```go
// MCPClient - Stdio 传输客户端
type MCPClient struct {
    cmd        *exec.Cmd
    stdin      io.WriteCloser
    stdout     *bufio.Reader
    requestID  int64
    tools      map[string]*MCPToolDefinition
}

// MCPHTTPClient - HTTP 传输客户端
type MCPHTTPClient struct {
    client    *http.Client
    baseURL   string
    sessionID string
    requestID int64
}

// MCPManager - 管理多个 MCP 服务器连接
type MCPManager struct {
    clients map[string]MCPClientInterface
    tools   map[string]Tool
}
```

### 7.4 配置示例

```json
{
  "tools": {
    "mcpServers": {
      "filesystem": {
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"]
      },
      "remote": {
        "url": "http://localhost:8765/mcp"
      }
    }
  }
}
```

### 7.5 与 nanobot 对比

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| Stdio 传输 | ✅ | ✅ |
| HTTP 传输 | ✅ MCPHTTPClient | ✅ streamablehttp |
| SSE 传输 | 预留 SSEClient | ✅ |
| 工具包装 | MCPToolWrapper | 相同 |
| 命名格式 | mcp_{server}_{tool} | 相同 |

---

## 8. 开发路线图

### Phase 1-4: 已完成 ✅

| 功能 | 状态 |
|------|------|
| Provider 自动匹配 | ✅ |
| Agent 核心循环 | ✅ |
| 飞书 WebSocket | ✅ |
| 技能系统 | ✅ |
| 流式响应 | ✅ |
| 文件持久化记忆 | ✅ |
| 子代理系统 | ✅ |
| 定时任务 | ✅ |
| MCP Stdio 传输 | ✅ |
| MCP HTTP 传输 | ✅ |
| Moltbook 技能 | ✅ |

### Phase 5: 计划中

| 功能 | 状态 | 说明 |
|------|------|------|
| 多渠道支持 | ⏳ | Telegram, Discord |
| 向量记忆 | ⏳ | Qdrant 集成 |
| 多模态 | ⏳ | Vision 支持 |
| Docker | ⏳ | 容器化部署 |

---

## 9. 技术选型

| 组件 | 选型 | 说明 |
|------|------|------|
| 语言 | Go 1.23+ | 高性能并发 |
| CLI | Cobra | 成熟框架 |
| Cron | robfig/cron | 标准 cron 库 |
| WebSocket | larksuite/oapi-sdk | 飞书官方 SDK |
| UUID | google/uuid | 唯一 ID 生成 |

---

## 10. Agent Social Network

Agent Social Network 包含两层含义：

### 9.1 Spawn 子代理（Agent-to-Agent 协作）

子代理可以在后台异步执行任务，实现 Agent 之间的协作：

```go
// internal/subagent/manager.go

type SubagentManager struct {
    provider     providers.Provider
    toolRegistry *tools.Registry
    tasks        map[string]*Subagent
    notify       chan *Subagent  // 完成通知
}

// 创建子代理
func (m *SubagentManager) Spawn(task, context string) *Subagent
```

**特点：**
- 后台 goroutine 执行，不阻塞主代理
- 独立的工具白名单（无 message、task_spawn）
- 最多 15 次迭代
- 完成后通过 channel 通知主代理

### 9.2 Moltbook 社交网络（Agent 社交平台）

Moltbook 是一个 AI Agent 专属的社交网络平台，LingGuard 提供完整的技能集成：

**文件结构：**
```
skills/builtin/moltbook/
├── SKILL.md        # 主要 API 文档
├── HEARTBEAT.md    # 定期检查指南
├── MESSAGING.md    # 私信功能
├── RULES.md        # 社区规则
└── package.json    # 元数据
```

**支持的功能：**
| 功能 | API | 说明 |
|------|-----|------|
| 注册认证 | `POST /agents/register` | 创建账号并获取 API Key |
| 发布帖子 | `POST /posts` | 发布内容到社区 |
| 评论回复 | `POST /posts/{id}/comments` | 评论和回复 |
| 投票 | `POST /posts/{id}/upvote` | 点赞/踩 |
| 社区 | `POST /submolts` | 创建和订阅社区 |
| 关注 | `POST /agents/{name}/follow` | 关注其他 Agent |
| 搜索 | `GET /search?q=...` | 语义搜索 |
| 私信 | `POST /agents/dm/request` | 发起私信 |

**使用方式：**
```
# Agent 通过 skill 工具加载 Moltbook 技能
skill --name moltbook

# 然后可以使用 curl 调用 API
curl -X POST https://www.moltbook.com/api/v1/posts \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"submolt": "general", "title": "Hello!", "content": "..."}'
```

### 9.3 与 nanobot 对比

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| Spawn 子代理 | ✅ goroutine | ✅ asyncio |
| Moltbook 技能 | ✅ 完整 | ✅ 完整 |
| 技能加载方式 | 渐进式（按需） | 一次性加载 |

---

## 11. 内置技能

| 技能 | 目录 | 描述 |
|------|------|------|
| weather | `skills/builtin/weather/` | 天气查询 (wttr.in) |
| git-workflow | `skills/builtin/git-workflow/` | Git 工作流自动化 |
| code-review | `skills/builtin/code-review/` | 代码审查指南 |
| file | `skills/builtin/file/` | 文件操作指南 |
| system | `skills/builtin/system/` | 系统操作指南 |
| moltbook | `skills/builtin/moltbook/` | AI Agent 社交网络 |

### Moltbook 技能

Moltbook 是一个 AI Agent 社交网络平台，LingGuard 集成了完整的 Moltbook 技能：

**文件结构：**
```
skills/builtin/moltbook/
├── SKILL.md        # 主要 API 文档
├── HEARTBEAT.md    # 定期检查指南
├── MESSAGING.md    # 私信功能文档
├── RULES.md        # 社区规则
└── package.json    # 技能元数据
```

**功能支持：**
- 注册和认证
- 发布、评论、投票
- 创建和订阅社区 (Submolts)
- 关注其他 Agent
- 语义搜索
- 私信系统
- 心跳集成

---

## 12. 参考资料

- [nanobot](https://github.com/HKUDS/nanobot) - 参考架构设计
- [OpenAI API](https://platform.openai.com/docs/api-reference) - LLM API规范
- [Anthropic API](https://docs.anthropic.com/) - Claude API 规范
- [飞书开放平台](https://open.feishu.cn/document/) - 飞书开发文档
- [MCP 规范](https://modelcontextprotocol.io/) - Model Context Protocol
- [Moltbook](https://www.moltbook.com/) - AI Agent 社交网络
