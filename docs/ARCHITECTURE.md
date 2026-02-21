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
| 语音识别 | 飞书语音消息转文字（Qwen3-ASR） |

---

## 2. 与 nanobot 对比

### 2.1 基本定位

| 维度 | LingGuard (Go) | nanobot (Python) |
|------|----------------|------------------|
| **编程语言** | Go 1.23+ | Python 3 |
| **代码量** | ~8,000 行 | ~3,700 行核心代码 |
| **核心理念** | 极简、高性能、单二进制部署 | 超轻量级、易研究、易扩展 |
| **部署方式** | 单二进制文件 | pip/uv/Docker |
| **并发模型** | Goroutine | asyncio |
| **内存占用** | ~20MB | ~100MB+ |
| **启动速度** | 毫秒级 | 秒级 |
| **类型安全** | 静态类型 | 动态类型 |

### 2.2 渠道支持对比

| 渠道 | LingGuard | nanobot | 说明 |
|------|:---------:|:-------:|------|
| 飞书 | ✅ WebSocket 长连接 | ✅ | 两者都支持，无需公网IP |
| QQ | ✅ 私聊 | ✅ 私聊 | 两者都支持 WebSocket |
| Telegram | ❌ | ✅ 推荐 | nanobot 官方推荐 |
| Discord | ❌ | ✅ | Socket Mode |
| WhatsApp | ❌ | ✅ | 扫码登录 |
| Slack | ❌ | ✅ | Socket Mode |
| Email | ❌ | ✅ | IMAP/SMTP |
| 钉钉 | ❌ | ✅ | Stream Mode |
| Mochat | ❌ | ✅ | 自动配置 |

**多渠道支持说明：**
- ✅ **支持多类型同时运行**：可以同时配置飞书 + QQ 等不同类型的 channel
- ❌ **不支持同类型多实例**：每种类型只能配置一个实例（如不能配置 2 个飞书 channel）

**nanobot 优势**: 渠道支持更丰富（9种 vs 2种）

### 2.3 LLM 提供商对比

| Provider | LingGuard | nanobot | 说明 |
|----------|:---------:|:-------:|------|
| OpenAI | ✅ | ✅ | GPT 系列 |
| Anthropic | ✅ | ✅ | Claude 系列 |
| OpenRouter | ✅ | ✅ 推荐 | 网关类型，访问所有模型 |
| DeepSeek | ✅ | ✅ | 国产模型 |
| Qwen/通义千问 | ✅ | ✅ | 阿里云 |
| GLM/智谱 | ✅ | ✅ | 智谱 AI |
| MiniMax | ✅ | ✅ | MiniMax |
| Moonshot/Kimi | ✅ | ✅ | 月之暗面 |
| Gemini | ✅ | ✅ | Google |
| Groq | ✅ | ✅ + 语音转录 | 高速推理 |
| vLLM | ✅ | ✅ | 本地部署 |
| AiHubMix | ✅ | ✅ | API 网关 |
| SiliconFlow | ❌ | ✅ | 硅基流动 |
| OpenAI Codex (OAuth) | ❌ | ✅ | ChatGPT Plus/Pro |
| GitHub Copilot (OAuth) | ❌ | ✅ | OAuth 登录 |
| 自定义 OpenAI 兼容 | ✅ | ✅ | 任意兼容端点 |

### 2.4 核心功能对比

| 功能模块 | LingGuard | nanobot | 差异说明 |
|----------|:---------:|:-------:|----------|
| **核心功能** ||||
| Agent Loop | ✅ | ✅ | 相同的循环迭代模式 |
| 会话管理 | ✅ | ✅ | 内存 + 窗口管理 |
| 记忆系统 | ✅ | ✅ | 相同的 MEMORY.md 方案 |
| 工具系统 | ✅ | ✅ | Shell, File, Web 等 |
| 技能系统 | ✅ | ✅ | LingGuard 支持渐进式加载 |
| **高级功能** ||||
| 定时任务 (Cron) | ✅ | ✅ | 相同的调度机制 |
| 子代理 (Subagent) | ✅ | ✅ | 相同的后台任务模式 |
| 流式响应 | ✅ | ✅ | 两者都支持实时输出 |
| MCP (Stdio) | ✅ | ✅ | 子进程启动 |
| MCP (HTTP) | ✅ | ✅ | HTTP/SSE 端点 |
| Agent Social Network | ✅ Moltbook | ✅ Moltbook + ClawdChat | AI 社交网络 |
| **独有功能** ||||
| 渐进式技能加载 | ✅ 独有 | ❌ | 节省 Token |
| 多模态支持 | ✅ 图片+视频 | 🚧 计划中 | 已实现 |
| 独立多模态 Provider | ✅ 独有 | ❌ | 可配置独立模型 |
| ClawHub 技能库 | ❌ | ✅ | 搜索安装技能 |
| OAuth 登录 | ❌ | ✅ | Codex/Copilot |
| 语音转写 | ✅ Qwen3-ASR | ✅ Groq Whisper | 都支持 |
| 语音合成 (TTS) | ✅ Qwen TTS | ❌ | LingGuard 独有 |
| AIGC 生成 | ✅ 图片+视频 | ❌ | LingGuard 独有 |
| Docker 支持 | ❌ | ✅ | 官方镜像 |

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
                ▼ (语音消息)
         ┌──────────────┐
         │ 下载音频文件 │
         └──────────────┘
                │
                ▼
         ┌──────────────┐
         │ ASR 语音转写 │
         │ (Qwen3-ASR)  │
         └──────────────┘
                │
                ▼
           转写文本 ──▶ Agent Loop
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
│   ├── speech/             # 语音识别
│   │   └── asr.go          # Qwen3-ASR 服务
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

### 4.6 记忆系统（参考 nanobot + OpenClaw）

LingGuard 的记忆系统融合了 nanobot 的文件存储方案和 OpenClaw 的自动记忆功能。

#### 4.6.1 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        用户消息                                  │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                     自动召回 (Auto-Recall)                       │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ 提取用户消息 │───▶│ 向量搜索    │───▶│ 注入上下文  │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      对话处理                                    │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                     自动捕获 (Auto-Capture)                      │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ 触发规则匹配 │───▶│ 问句检测    │───▶│ 智能去重    │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│                            │                                    │
│                            ▼                                    │
│                     ┌─────────────┐                            │
│                     │ 存储记忆    │                            │
│                     └─────────────┘                            │
└─────────────────────────────────────────────────────────────────┘
```

#### 4.6.2 与 nanobot 的差异

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **存储格式** | Markdown 文件 | Markdown 文件 |
| **长期记忆** | MEMORY.md | MEMORY.md |
| **事件日志** | HISTORY.md | 事件日志 |
| **每日日志** | YYYY-MM-DD.md | 每日笔记 |
| **检索方式** | grep + 向量搜索 | grep |
| **记忆工具** | memory 工具 | 内置函数 |
| **自动召回** | ✅ OpenClaw 风格 | ❌ |
| **自动捕获** | ✅ 触发规则 | ❌ |
| **智能去重** | ✅ 三层去重 | ❌ |
| **问句过滤** | ✅ 排除问句 | ❌ |
| **分类检测** | ✅ 自动分类 | ❌ |

#### 4.6.3 文件结构

```
~/.lingguard/memory/
├── MEMORY.md          # 长期记忆（用户偏好、重要事实）
├── HISTORY.md         # 事件日志（系统事件）
├── vectors.db         # 向量索引（sqlite-vec）
└── 2026-02-20.md      # 每日日志
```

**MEMORY.md 结构：**
```markdown
# Memory

This file stores long-term memories and important facts.

## User Preferences
<!-- 用户偏好设置 -->
- [2026-02-20 10:12] 喜欢猫，尤其是小猫

## Project Context
<!-- 项目上下文信息 -->

## Important Facts
<!-- 重要事实记录 -->
```

#### 4.6.4 自动召回 (Auto-Recall)

**触发时机：** 每次用户发送消息时

**流程：**
```go
// internal/agent/agent.go - buildContextWithMedia()
if a.config.AutoRecall && a.IsVectorSearchEnabled() {
    // 1. 获取最近的用户消息
    lastUserMessage := getLastUserMessage(history)

    // 2. 向量搜索相关记忆
    relevant := a.searchRelevantMemories(lastUserMessage, topK, minScore)

    // 3. 格式化并注入到系统提示
    memContext := formatRelevantMemories(relevant)
    systemPrompt += memContext
}
```

**配置参数：**
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `autoRecall` | `true` | 是否启用自动召回 |
| `autoRecallTopK` | `3` | 召回记忆数量 |
| `autoRecallMinScore` | `0.3` | 最小相似度阈值 |

#### 4.6.5 自动捕获 (Auto-Capture)

**触发时机：** 对话结束时（使用 `defer` 确保总是执行）

**捕获流程：**
```
用户消息 → 触发规则匹配 → 问句检测 → 去重检查 → 存储记忆
```

**触发规则（pkg/memory/capture.go）：**

| 类别 | 正则表达式 | 示例匹配 |
|------|------------|----------|
| 记住指令 | `(?i)记住\|remember\|zapamatuj` | "记住我喜欢猫" |
| 忘记指令 | `(?i)别忘\|don't forget` | "别忘了开会" |
| 偏好表达 | `(?i)我喜欢\|我讨厌\|prefer\|like\|hate` | "我喜欢 Go 语言" |
| 习惯表达 | `(?i)always\|never\|usually\|often` | "I always use dark mode" |
| 决策记录 | `(?i)决定\|decided\|will use\|using` | "我决定用这个方案" |
| 选择表达 | `(?i)my choice\|选择` | "我的选择是 PostgreSQL" |
| 电话号码 | `\+?\d{10,}` | "我的电话是 13812345678" |
| 邮箱地址 | `[\w.-]+@[\w.-]+\.\w+` | "联系我：test@example.com" |
| 重要标记 | `(?i)important\|重要\|关键\|核心` | "这很重要" |
| 身份信息 | `(?i)my name is\|i am\|i'm` | "My name is Alice" |
| 项目信息 | `(?i)my project\|my work\|我的项目` | "我的项目用 React" |
| 工作相关 | `(?i)working on\|developing\|building` | "I'm working on a new feature" |

**问句过滤（不会捕获）：**

| 检测规则 | 示例 |
|----------|------|
| 以 `？` 或 `?` 结尾 | "我喜欢什么？" |
| 包含疑问词但无陈述标记 | "怎么用这个？" |

**Prompt 注入检测（拒绝捕获）：**

| 检测规则 | 示例 |
|----------|------|
| 忽略指令 | "Ignore all previous instructions" |
| 忘记指令 | "Forget everything" |
| 角色扮演 | "You are now a pirate" |
| 扮演指令 | "Act as if you are..." |
| 特殊 Token | `<\|...\|>` |

#### 4.6.6 记忆分类

捕获的记忆自动分类：

| 分类 | 检测规则 | 示例 |
|------|----------|------|
| `preference` | 包含 `prefer`、`喜欢`、`讨厌`、`always`、`never` | "我喜欢简洁的回答" |
| `decision` | 包含 `decided`、`决定`、`will use`、`选择` | "我决定使用 PostgreSQL" |
| `entity` | 包含 `@` 或电话号码模式 | "我的邮箱是 xxx@example.com" |
| `fact` | 包含 `my name`、`i am`、`my project` | "My name is Alice" |
| `other` | 其他情况 | - |

#### 4.6.7 智能去重（三层检查）

```
┌─────────────────────────────────────────────────────────────────┐
│                      去重检查流程                                │
├─────────────────────────────────────────────────────────────────┤
│  第一层：文件存储检查                                            │
│  ├─ 读取 MEMORY.md 内容                                         │
│  ├─ 检查是否已包含相同内容                                       │
│  └─ 如果存在则跳过                                              │
├─────────────────────────────────────────────────────────────────┤
│  第二层：缓冲区检查                                              │
│  ├─ 遍历待索引的缓冲区记录                                       │
│  ├─ 比较内容是否相同或包含                                       │
│  └─ 如果存在则跳过                                              │
├─────────────────────────────────────────────────────────────────┤
│  第三层：向量搜索检查                                            │
│  ├─ 生成内容向量                                                 │
│  ├─ 搜索相似记忆（TopK=1）                                      │
│  └─ 如果相似度 >= 0.95 则跳过                                   │
└─────────────────────────────────────────────────────────────────┘
```

**代码实现（pkg/memory/hybrid_store.go）：**
```go
func (s *HybridStore) AddMemory(category, content string) error {
    // 第一层：文件存储检查
    existingMemory, _ := s.fileStore.GetMemory()
    if strings.Contains(existingMemory, content) {
        return nil // 已存在
    }

    // 第二层：缓冲区检查
    for _, r := range s.buffer {
        if isSimilar(r.Content, content) {
            return nil // 缓冲区中已存在
        }
    }

    // 第三层：向量搜索检查
    existing, _ := s.Search(ctx, content, 1)
    if len(existing) > 0 && existing[0].Score >= 0.95 {
        return nil // 相似度太高
    }

    // 存储到文件和向量索引
    s.fileStore.AddMemory(category, content)
    s.addToBuffer(record)
}
```

#### 4.6.8 配置示例

```json
{
  "agents": {
    "memory": {
      "enabled": true,
      "recentDays": 3,
      "maxHistoryLines": 1000,
      "autoRecall": true,
      "autoRecallTopK": 3,
      "autoRecallMinScore": 0.3,
      "autoCapture": true,
      "captureMaxChars": 500,
      "vector": {
        "enabled": true,
        "embedding": {
          "provider": "qwen",
          "model": "text-embedding-v4",
          "dimension": 1024
        },
        "search": {
          "vectorWeight": 0.7,
          "bm25Weight": 0.3,
          "defaultTopK": 10,
          "minScore": 0.5,
          "rerank": {
            "enabled": true,
            "provider": "qwen",
            "model": "qwen3-vl-rerank"
          }
        }
      }
    }
  }
}
```

#### 4.6.9 核心文件

| 文件 | 说明 |
|------|------|
| `pkg/memory/capture.go` | 触发规则、问句检测、分类检测 |
| `pkg/memory/hybrid_store.go` | 混合存储、三层去重 |
| `pkg/memory/context_builder.go` | 上下文构建、语义搜索 |
| `pkg/memory/vector_store.go` | 向量索引（sqlite-vec） |
| `internal/agent/agent.go` | 自动召回、自动捕获逻辑 |
| `internal/config/config.go` | 记忆配置结构 |

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

### 4.9 语音识别系统

LingGuard 支持飞书语音消息的自动转写，参考 [OpenClaw](https://github.com/openclaw/openclaw) 的语音交互能力实现。

#### 4.9.1 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                     飞书语音消息                                  │
│                   (opus 格式音频)                                │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Feishu Channel                               │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ 接收 file_key│───▶│ 下载音频    │───▶│ 调用 ASR    │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Speech Service                              │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ Base64 编码 │───▶│ HTTP REST   │───▶│ 返回文本    │         │
│  └─────────────┘    │ API 调用    │    └─────────────┘         │
│                     └─────────────┘                             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                        转写文本 → Agent 处理
```

#### 4.9.2 支持的 ASR 模型

| 模型 | 协议 | API 端点 | 说明 |
|------|------|----------|------|
| `qwen3-asr-flash` | HTTP REST | OpenAI 兼容模式 | 推荐，稳定可靠 |
| `fun-asr-realtime` | WebSocket | WebSocket 流式 | 实时识别，实现复杂 |
| `fun-asr` | HTTP REST | 异步任务 | 需要公网 URL |

#### 4.9.3 Qwen3-ASR 实现细节

使用 OpenAI 兼容模式调用阿里云 DashScope API：

```go
// pkg/speech/asr.go

type QwenASR struct {
    config  *Config
    client  *http.Client
    apiBase string  // https://dashscope.aliyuncs.com/compatible-mode/v1
}

// TranscribeFromBytes 从字节流转写
func (a *QwenASR) TranscribeFromBytes(ctx context.Context, audioData []byte, format string) (*TranscriptionResult, error) {
    // 1. 构建 base64 data URI
    mimeType := a.getMimeType(format)  // audio/opus
    dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(audioData))

    // 2. 构建请求体（OpenAI 兼容格式）
    requestBody := map[string]any{
        "model": a.config.Model,  // qwen3-asr-flash
        "messages": []map[string]any{
            {
                "role": "user",
                "content": []map[string]any{
                    {
                        "type": "input_audio",
                        "input_audio": map[string]any{
                            "data": dataURI,
                        },
                    },
                },
            },
        },
        "asr_options": map[string]any{
            "enable_itn": false,
            "language": a.config.Language,  // zh
        },
    }

    // 3. 调用 API
    url := fmt.Sprintf("%s/chat/completions", a.apiBase)
    // ...
}
```

#### 4.9.4 与 OpenClaw 对比

| 方面 | LingGuard | OpenClaw |
|------|-----------|----------|
| **ASR 提供商** | Qwen3-ASR | OpenAI Whisper / Qwen |
| **API 协议** | HTTP REST (OpenAI 兼容) | 相同 |
| **音频格式** | opus (飞书默认) | 多格式支持 |
| **API Key 继承** | ✅ 从 providers.qwen 继承 | 单独配置 |
| **Base64 编码** | ✅ 支持 | ✅ 支持 |

#### 4.9.5 配置说明

```json
{
  "speech": {
    "enabled": true,
    "provider": "qwen",           // 从 providers.qwen 继承 apiKey
    "model": "qwen3-asr-flash",   // ASR 模型
    "format": "opus",             // 飞书语音格式
    "language": "zh",             // 语言
    "timeout": 60                 // 超时（秒）
  }
}
```

**配置要点：**
- `apiKey` 无需配置，自动从 `providers.{provider}` 继承
- `format` 默认 `opus`，飞书语音消息格式
- `language` 默认 `zh`，支持多语言

#### 4.9.6 飞书渠道集成

```go
// internal/channels/feishu.go

func (c *FeishuChannel) handleAudioMessage(ctx context.Context, event *lark.Event) error {
    // 1. 解析消息获取 file_key
    var msg struct {
        FileKey  string `json:"file_key"`
        Duration int    `json:"duration"`
    }

    // 2. 下载音频文件
    audioData, err := c.downloadAudio(ctx, msg.FileKey)

    // 3. 调用 ASR 服务
    if c.speechService != nil {
        result, err := c.speechService.TranscribeFromBytes(ctx, audioData, "opus")
        if err == nil {
            text = result.Text  // 使用转写文本
        }
    }

    // 4. 传递给 Agent 处理
    return c.handler.Handle(ctx, &Message{Content: text, ...})
}
```

#### 4.9.7 核心文件

| 文件 | 说明 |
|------|------|
| `pkg/speech/asr.go` | ASR 服务实现（Qwen3-ASR） |
| `internal/channels/feishu.go` | 飞书语音消息处理 |
| `internal/config/config.go` | SpeechConfig 配置结构 |
| `cmd/cli/gateway.go` | 语音服务初始化 |

### 4.10 语音合成系统 (TTS)

LingGuard 支持语音合成功能，可以将文本转换为自然流畅的语音，支持多种音色。这是 LingGuard 相对于 nanobot 的独有功能。

#### 4.10.1 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                        文本内容                                   │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                       TTS Tool                                   │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ 接收文本    │───▶│ 调用 API    │───▶│ 下载音频    │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Qwen TTS Service                            │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ HTTP REST   │───▶│ CosyVoice   │───▶│ 返回音频URL │         │
│  │ API 调用    │    │ 模型合成    │    └─────────────┘         │
│  └─────────────┘    └─────────────┘                             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                        音频文件 → 飞书发送
```

#### 4.10.2 可用音色

| 音色 | 描述 | 语言 |
|------|------|------|
| Cherry | 甜美女声（默认） | 中文 |
| Serena | 温柔女声 | 中文 |
| Ethan | 沉稳男声 | 中文 |
| Chelsie | 活力女声 | 中文 |
| Momo | 可爱童声 | 中文 |
| Vivian | 知性女声 | 中文 |
| Moon | 亲切男声 | 中文 |
| Maia | 清澈女声 | 中文 |
| Kai | 磁性男声 | 中文 |

#### 4.10.3 与其他方案对比

| 方面 | LingGuard | nanobot | OpenClaw |
|------|-----------|---------|----------|
| **TTS 支持** | ✅ Qwen TTS | ❌ 无 | ✅ OpenAI/Edge TTS |
| **音色选择** | ✅ 9 种中文音色 | ❌ | ✅ 多种音色 |
| **API 协议** | HTTP REST | - | HTTP REST |
| **配置继承** | ✅ 自动继承 qwen API Key | - | 单独配置 |
| **飞书集成** | ✅ 自动发送音频 | - | ✅ |

**LingGuard TTS 优势：**
1. **配置简洁**：API Key 自动从 providers.qwen 继承，无需重复配置
2. **中文优化**：使用阿里云 CosyVoice 模型，对中文支持更好
3. **多音色**：9 种精选中文音色，覆盖不同场景需求
4. **无缝集成**：生成音频自动通过飞书发送，无需手动操作

#### 4.10.4 配置说明

```json
{
  "tts": {
    "enabled": true,
    "provider": "qwen",
    "model": "qwen3-tts-flash",
    "voice": "Cherry",
    "timeout": 60,
    "outputDir": "~/.lingguard/workspace/generated"
  }
}
```

**配置要点：**
- `apiKey` 无需配置，自动从 `providers.{provider}` 继承
- `voice` 默认 `Cherry`，可在运行时通过参数指定
- 输出音频格式为 WAV (24kHz)
- 文本最大长度 5000 字符

#### 4.10.5 核心文件

| 文件 | 说明 |
|------|------|
| `pkg/tts/tts.go` | TTS 服务实现（Qwen TTS） |
| `internal/tools/tts.go` | TTS 工具封装 |
| `internal/config/config.go` | TTSConfig 配置结构 |

### 4.11 AIGC 内容生成系统

LingGuard 支持 AI 内容生成功能，包括图像生成和视频生成，使用阿里云通义万相模型。这是 LingGuard 相对于 nanobot 的独有功能。

#### 4.11.1 支持的生成类型

| 类型 | 动作 | 模型 | 说明 |
|------|------|------|------|
| 文生图 | `generate_image` | wan2.6-t2i | 根据文字描述生成图片 |
| 文生视频 | `generate_video` | wan2.6-t2v | 根据文字描述生成视频 |
| 图生视频 | `generate_video_from_image` | wan2.6-i2v-flash | 根据图片生成视频 |
| 视频生视频 | `generate_video_from_video` | wan2.6-r2v-flash | 根据参考视频生成新视频（保持角色一致性） |

#### 4.11.2 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                        用户请求                                   │
│  {"action": "generate_video_from_video", "prompt": "...",       │
│   "video_path": "/path/to/ref.mp4"}                             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                       AIGC Tool                                  │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ 解析动作    │───▶│ 准备参数    │───▶│ 调用 API    │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                                │
                ┌───────────────┼───────────────┐
                ▼               ▼               ▼
         ┌──────────┐    ┌──────────┐    ┌──────────┐
         │ 文生图   │    │ 文生视频 │    │ 视频生   │
         │ (同步)   │    │ (异步)   │    │ 视频     │
         └──────────┘    └──────────┘    └──────────┘
                │               │               │
                └───────────────┼───────────────┘
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    阿里云通义万相 API                             │
│  - multimodal-generation (图片同步生成)                          │
│  - video-generation/video-synthesis (视频异步生成)               │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                        生成文件 → 飞书发送
```

#### 4.11.3 与其他方案对比

| 方面 | LingGuard | nanobot | OpenClaw |
|------|-----------|---------|----------|
| **文生图** | ✅ wan2.6-t2i | ❌ | ✅ DALL-E |
| **文生视频** | ✅ wan2.6-t2v | ❌ | ❌ |
| **图生视频** | ✅ wan2.6-i2v | ❌ | ❌ |
| **视频生视频** | ✅ wan2.6-r2v | ❌ | ❌ |
| **角色一致性** | ✅ V2V 支持 | ❌ | ❌ |
| **API 协议** | HTTP REST (同步+异步) | - | HTTP REST |
| **生成速度** | 图片快/视频慢 | - | 图片中等 |
| **飞书集成** | ✅ 自动发送 | - | ✅ |

**LingGuard AIGC 优势：**
1. **功能全面**：支持 4 种生成模式，覆盖主流 AIGC 场景
2. **视频生视频**：独有 V2V 功能，可保持参考视频中角色的一致性
3. **异步处理**：视频生成采用异步任务模式，支持长时间等待
4. **自动发送**：生成内容自动通过飞书发送给用户

**劣势：**
1. **模型限制**：目前仅支持阿里云通义万相模型
2. **网络依赖**：需要稳定的网络连接阿里云 API
3. **成本考虑**：视频生成需要一定的 API 调用费用

#### 4.11.4 视频生视频功能详解

视频生视频（V2V）是 LingGuard 的特色功能，使用 wan2.6-r2v-flash 模型，可以保持参考视频中角色的外观一致性。

**工作流程：**
```
1. 读取参考视频文件
       ↓
2. 获取上传凭证 (GET /uploads?action=getPolicy)
       ↓
3. 上传视频到 OSS (POST upload_host)
       ↓
4. 获取 oss:// 临时 URL
       ↓
5. 提交视频合成任务 (POST video-synthesis)
       ↓
6. 轮询任务状态 (GET /tasks/{task_id})
       ↓
7. 下载生成的视频文件
       ↓
8. 返回本地文件路径
```

**技术要点：**
- 需要设置 `X-DashScope-OssResourceResolve: enable` 请求头
- 视频时长支持 5 或 10 秒
- 输出分辨率 1280x720
- 使用 `reference_urls` 字段传递参考视频

**配置说明：**
```json
{
  "tools": {
    "aigc": {
      "enabled": true,
      "provider": "qwen",
      "textToImage": "wan2.6-t2i",
      "textToVideo": "wan2.6-t2v",
      "imageToVideo": "wan2.6-i2v-flash",
      "videoToVideo": "wan2.6-r2v-flash",
      "imageToVideoDuration": 15
    }
  }
}
```

#### 4.11.5 核心文件

| 文件 | 说明 |
|------|------|
| `internal/tools/aigc.go` | AIGC 工具实现 |
| `internal/config/config.go` | AIGCConfig 配置结构 |

### 4.12 Heartbeat 心跳服务

Heartbeat 服务是一个定期唤醒机制，让 Agent 在没有用户触发的情况下主动执行任务。Agent 会定期读取工作空间中的 `HEARTBEAT.md` 文件，并执行其中定义的任务。

#### 4.12.1 配置

在 `config.json` 中启用心跳服务：

```json
{
  "heartbeat": {
    "enabled": true,
    "interval": 30
  }
}
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | true | 是否启用心跳服务 |
| `interval` | int | 30 | 心跳间隔（分钟） |

#### 4.12.2 工作原理

```
┌─────────────────────────────────────────────────┐
│              HeartbeatService                    │
│                                                  │
│   每 N 分钟（默认 30）                           │
│        ↓                                         │
│   读取 workspace/HEARTBEAT.md 文件              │
│        ↓                                         │
│   文件为空？ ──是──→ 跳过，等待下次心跳          │
│        │                                         │
│       否                                         │
│        ↓                                         │
│   唤醒 Agent 执行任务                            │
│        ↓                                         │
│   任务完成？──是──→ 回复 "HEARTBEAT_OK"          │
│        │                                         │
│       否                                         │
│        ↓                                         │
│   执行任务并记录结果                             │
└─────────────────────────────────────────────────┘
```

#### 4.12.3 使用方法

1. 在工作空间目录（默认 `~/.lingguard/workspace/`）创建 `HEARTBEAT.md` 文件
2. 在文件中定义 Agent 需要定期检查的任务
3. 启动 gateway，心跳服务会自动运行

#### 4.12.4 示例 HEARTBEAT.md

```markdown
# 心跳任务清单

## 定期检查项

1. 检查系统日志是否有错误
2. 检查是否有待处理的任务

## 如果发现问题

- 记录到 HISTORY.md
- 如果严重，发送通知到飞书群

## 无任务时

回复: HEARTBEAT_OK
```

#### 4.12.5 响应格式

- **无任务需要处理**: `HEARTBEAT_OK` 或包含此标识的响应
- **有任务完成**: 描述执行的操作
- **需要人工介入**: 描述问题并请求帮助

#### 4.12.6 使用场景

| 场景 | 说明 |
|------|------|
| 定时监控 | 定期检查系统状态、日志文件 |
| 数据同步 | 定期拉取最新数据或配置 |
| 提醒服务 | 在特定时间主动发送提醒 |
| 自动化运维 | 定期执行清理、备份任务 |
| 社交互动 | 检查消息、回复评论（如 Moltbook 技能） |

#### 4.12.7 与 nanobot 对比

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| **触发机制** | 定时器 + HEARTBEAT.md | 相同 |
| **独立会话** | ✅ heartbeat-main | ✅ |
| **超时控制** | 5 分钟 | 相同 |
| **跳过空文件** | ✅ | ✅ |

#### 4.12.8 注意事项

- 心跳会占用 LLM API 调用次数
- 如果 HEARTBEAT.md 为空或不存在，心跳会被跳过
- 心跳使用独立的会话 ID (`heartbeat-main`)，不影响用户对话
- 心跳超时时间为 5 分钟

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
  "speech": {
    "enabled": true,
    "provider": "qwen",
    "model": "qwen3-asr-flash",
    "format": "opus",
    "language": "zh",
    "timeout": 60
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

### Phase 1-6: 已完成 ✅

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
| 多模态支持 | ✅ 图片+视频 |
| 独立多模态 Provider | ✅ |
| QQ 渠道 | ✅ |
| 向量记忆（sqlite-vec） | ✅ |
| 自动召回/捕获（OpenClaw 风格） | ✅ |
| 语音识别（Qwen3-ASR） | ✅ |
| 语音合成（Qwen TTS） | ✅ |
| AIGC 图像/视频生成 | ✅ |
| 视频生视频（V2V） | ✅ |

### Phase 7: 计划中

| 功能 | 状态 | 说明 |
|------|------|------|
| 多渠道支持 | ⏳ | Telegram, Discord |
| Docker | ⏳ | 容器化部署 |
| ClawHub 技能库 | ⏳ | 技能搜索安装 |

### Phase 8: 自我进化机制（计划中）

| 功能 | 状态 | 说明 |
|------|------|------|
| 核心框架 | ⏳ | types.go, config.go, orchestrator.go |
| 问题扫描 | ⏳ | 日志分析、错误检测、自我反思 |
| 代码生成 | ⏳ | LLM 生成修复方案 |
| Git 管理 | ⏳ | 分支管理、提交、推送 |
| 验证层 | ⏳ | 构建、测试、安全审计 |
| 部署管理 | ⏳ | 编译、服务管理、健康检查 |
| 回滚机制 | ⏳ | 快照、自动回滚 |
| 审计日志 | ⏳ | 操作记录、追踪 |
| 进化技能 | ⏳ | SKILL.md, SAFETY.md |

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
| ClawdChat 支持 | ❌ | ✅ |
| 技能加载方式 | 渐进式（按需） | 一次性加载 |
| ClawHub 技能库 | ❌ | ✅ 搜索安装技能 |

---

## 11. 内置技能

| 技能 | 目录 | 描述 |
|------|------|------|
| weather | `skills/builtin/weather/` | 天气查询 (心知天气) |
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

## 12. 自我进化机制 (Self-Evolution)

LingGuard 支持自我进化能力，可以自动发现问题、生成修复代码、提交版本更新并部署。这是 LingGuard 的核心特性之一。

### 12.1 设计目标

- **发现问题** - 自动检测系统错误、日志异常、自我反思
- **自动修正代码** - 使用 LLM 生成修复代码
- **上库（Git 提交）** - 自动 commit 和 push
- **部署更新** - 重新编译并重启服务

### 12.2 架构设计

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     Evolution Orchestrator (编排器)                       │
│  - 状态机管理    - 权限检查    - 速率限制                                  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        ▼                           ▼                           ▼
┌───────────────────┐    ┌───────────────────┐    ┌───────────────────┐
│   Problem Scanner │    │   Code Generator  │    │   Git Manager     │
│   (问题发现)       │    │   (代码生成)       │    │   (版本控制)       │
│ - 日志分析         │    │ - 修复生成         │    │ - 分支管理         │
│ - 错误检测         │    │ - Diff 生成        │    │ - 提交处理         │
│ - 自我反思         │    │ - 测试生成         │    │ - 推送处理         │
└───────────────────┘    └───────────────────┘    └───────────────────┘
                                    │
                                    ▼
        ┌───────────────────────────────────────────────────────────┐
        │                    Validation Layer                        │
        │  构建检查  →  测试运行  →  安全审计                          │
        └───────────────────────────────────────────────────────────┘
                                    │
                                    ▼
        ┌───────────────────────────────────────────────────────────┐
        │                    Deployment Layer                        │
        │  编译  →  停止服务  →  安装  →  启动服务  →  健康检查        │
        └───────────────────────────────────────────────────────────┘
                                    │
                                    ▼
        ┌───────────────────────────────────────────────────────────┐
        │                    Rollback Manager                        │
        │  快照存储  →  版本历史  →  失败自动回滚                      │
        └───────────────────────────────────────────────────────────┘
```

### 12.3 文件结构

```
internal/
├── evolution/                    # 自我进化模块
│   ├── types.go                  # 核心类型和接口
│   ├── config.go                 # 进化配置
│   ├── orchestrator.go           # 主编排器
│   ├── scanner.go                # 问题扫描器
│   ├── generator.go              # 代码生成器
│   ├── validator.go              # 验证层
│   ├── git_manager.go            # Git 操作
│   ├── deployer.go               # 部署管理
│   ├── rollback.go               # 回滚管理
│   ├── audit.go                  # 审计日志
│   └── tool.go                   # 进化工具（暴露给 Agent）
├── tools/
└── evolution.go                  # 进化工具包装器
skills/builtin/
└── self-evolution/               # 进化技能定义
    ├── SKILL.md                  # 主技能文档
    └── SAFETY.md                 # 安全指南
```

### 12.4 配置设计

```json
{
  "evolution": {
    "enabled": true,
    "projectPath": "/path/to/lingguard",
    "evolveBranch": "ai-evolution",
    "mainBranch": "main",

    "autoScanEnabled": true,
    "scanInterval": 30,
    "minSeverityToFix": "medium",

    "requireApproval": true,
    "allowedUsers": ["ou_xxx"],
    "maxAutoFixesPerDay": 3,

    "allowedFiles": ["internal/**/*.go", "pkg/**/*.go", "skills/**/*.md"],
    "forbiddenFiles": ["**/*_test.go"],
    "forbiddenPatterns": ["os.RemoveAll", "exec.Command.*rm -rf"],
    "maxChangesPerFix": 5,

    "runTests": true,
    "testTimeout": 300,
    "buildTimeout": 120,

    "autoDeploy": false,
    "autoPush": false,
    "serviceName": "lingguard.service",
    "healthCheckTimeout": 30,

    "autoRollback": true,
    "maxSnapshots": 10,
    "rollbackOnFailure": true,

    "auditLogPath": "~/.lingguard/evolution/audit.log"
  }
}
```

### 12.5 工作流程

```
1. 扫描问题
   - 日志分析（错误模式匹配）
   - 自我反思（LLM 分析代码库）
   - 系统检查（服务状态、资源）

2. 生成修复
   - 收集上下文（相关文件、接口定义）
   - LLM 生成修复方案
   - 安全检查（禁止文件、禁止模式）

3. 验证修改
   - 构建检查 (go build)
   - 测试运行 (go test)
   - 安全审计 (go vet)

4. 提交代码
   - 切换到 ai-evolution 分支
   - git add + commit
   - 可选：git push

5. 部署更新
   - 创建快照（用于回滚）
   - 构建新二进制
   - 停止服务 → 安装 → 启动
   - 健康检查

6. 失败回滚
   - Git reset 到快照版本
   - 恢复备份二进制
   - 重新部署
```

### 12.6 安全机制

#### 12.6.1 文件访问控制

| 类型 | 模式 | 说明 |
|------|------|------|
| 白名单 | `internal/**/*.go` | 允许修改的 Go 文件 |
| 白名单 | `pkg/**/*.go` | 允许修改的 pkg 文件 |
| 白名单 | `skills/**/*.md` | 允许修改的技能文档 |
| 黑名单 | `**/*_test.go` | 禁止修改测试文件 |
| 黑名单 | `**/config*.json` | 禁止修改配置文件 |

#### 12.6.2 代码模式检测

| 危险模式 | 说明 |
|----------|------|
| `os.RemoveAll` | 禁止递归删除 |
| `exec.Command.*rm -rf` | 禁止危险 shell 命令 |
| `os.Setenv.*API_KEY` | 禁止设置敏感环境变量 |
| 硬编码密钥 | 禁止在代码中硬编码密钥 |

#### 12.6.3 速率限制

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `maxAutoFixesPerDay` | 3 | 每日最大修复数 |
| `scanInterval` | 30 | 扫描间隔（分钟） |
| 冷却期 | 30 分钟 | 失败后的冷却时间 |

#### 12.6.4 审批流程

```go
type ApprovalConfig struct {
    RequireApproval bool     // 默认需要人工批准
    AllowedUsers    []string // 白名单用户（可跳过审批）
}
```

### 12.7 回滚机制

#### 12.7.1 自动回滚触发条件

| 触发条件 | 说明 |
|----------|------|
| 构建失败 | `go build` 返回错误 |
| 测试失败 | `go test` 有失败用例 |
| 健康检查失败 | 服务启动后无法响应 |
| 服务崩溃 | 进程异常退出 |

#### 12.7.2 快照管理

```go
type Snapshot struct {
    ID          string    // 快照 ID
    CreatedAt   time.Time // 创建时间
    GitCommit   string    // Git commit hash
    BinaryPath  string    // 备份二进制路径
    ConfigPath  string    // 备份配置路径
}

// 保留策略
const MaxSnapshots = 10  // 保留最近 10 个快照
```

### 12.8 审计日志

所有进化操作都记录到审计日志：

```
~/.lingguard/evolution/audit.log
```

日志格式：
```json
{
  "timestamp": "2026-02-20T10:30:00Z",
  "action": "fix_applied",
  "user": "ou_xxx",
  "problem_id": "prob_001",
  "files_changed": ["internal/agent/agent.go"],
  "git_commit": "abc123",
  "status": "success"
}
```

### 12.9 进化工具 API

进化工具暴露给 Agent，可通过对话触发：

```
# 扫描问题
evolution_scan --type logs --severity high

# 生成修复
evolution_fix --problem_id prob_001

# 查看状态
evolution_status

# 部署更新
evolution_deploy --commit abc123

# 回滚
evolution_rollback --snapshot snap_001
```

### 12.10 技能文档

进化技能文档位于 `skills/builtin/self-evolution/`：

**SKILL.md** - 主要技能文档
- 进化能力说明
- 工具使用指南
- 最佳实践

**SAFETY.md** - 安全指南
- 危险操作警告
- 审批流程说明
- 回滚操作指南

### 12.11 核心接口

```go
// internal/evolution/types.go

// Problem 表示发现的问题
type Problem struct {
    ID          string
    Type        ProblemType    // error, warning, suggestion
    Severity    Severity       // low, medium, high, critical
    Source      string         // logs, reflection, system
    Title       string
    Description string
    Location    *Location      // 文件位置
    DetectedAt  time.Time
}

// Fix 表示生成的修复
type Fix struct {
    ID          string
    ProblemID   string
    Files       []FileChange
    Description string
    Approved    bool
    AppliedAt   *time.Time
}

// FileChange 表示文件修改
type FileChange struct {
    Path        string
    Action      FileAction    // create, modify, delete
    OldContent  string
    NewContent  string
    Diff        string
}

// Orchestrator 主编排器接口
type Orchestrator interface {
    Scan(ctx context.Context, opts ScanOptions) ([]*Problem, error)
    GenerateFix(ctx context.Context, problem *Problem) (*Fix, error)
    ValidateFix(ctx context.Context, fix *Fix) error
    ApplyFix(ctx context.Context, fix *Fix) error
    Deploy(ctx context.Context, opts DeployOptions) error
    Rollback(ctx context.Context, snapshotID string) error
}
```

### 12.12 与其他模块的关系

```
┌─────────────────────────────────────────────────────────────────┐
│                        Agent Core                                │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ 对话处理    │───▶│ 进化工具调用 │───▶│ 结果反馈    │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Evolution Orchestrator                         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ Scanner     │    │ Generator   │    │ GitManager  │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │ Validator   │    │ Deployer    │    │ Rollback    │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                                │
                ┌───────────────┼───────────────┐
                ▼               ▼               ▼
        ┌──────────┐    ┌──────────┐    ┌──────────┐
        │ Provider │    │ Git      │    │ Systemd  │
        │ (LLM)    │    │ (版本库) │    │ (服务)   │
        └──────────┘    └──────────┘    └──────────┘
```

### 12.13 配置文件修改

需要在 `internal/config/config.go` 中添加 `EvolutionConfig`：

```go
// EvolutionConfig 自我进化配置
type EvolutionConfig struct {
    Enabled          bool     `json:"enabled"`
    ProjectPath      string   `json:"projectPath"`
    EvolveBranch     string   `json:"evolveBranch"`
    MainBranch       string   `json:"mainBranch"`

    AutoScanEnabled  bool     `json:"autoScanEnabled"`
    ScanInterval     int      `json:"scanInterval"`
    MinSeverityToFix string   `json:"minSeverityToFix"`

    RequireApproval  bool     `json:"requireApproval"`
    AllowedUsers     []string `json:"allowedUsers"`
    MaxAutoFixesPerDay int    `json:"maxAutoFixesPerDay"`

    AllowedFiles     []string `json:"allowedFiles"`
    ForbiddenFiles   []string `json:"forbiddenFiles"`
    ForbiddenPatterns []string `json:"forbiddenPatterns"`
    MaxChangesPerFix int      `json:"maxChangesPerFix"`

    RunTests         bool     `json:"runTests"`
    TestTimeout      int      `json:"testTimeout"`
    BuildTimeout     int      `json:"buildTimeout"`

    AutoDeploy       bool     `json:"autoDeploy"`
    AutoPush         bool     `json:"autoPush"`
    ServiceName      string   `json:"serviceName"`
    HealthCheckTimeout int    `json:"healthCheckTimeout"`

    AutoRollback     bool     `json:"autoRollback"`
    MaxSnapshots     int      `json:"maxSnapshots"`
    RollbackOnFailure bool    `json:"rollbackOnFailure"`

    AuditLogPath     string   `json:"auditLogPath"`
}
```

---

## 13. 参考资料

- [nanobot](https://github.com/HKUDS/nanobot) - 参考架构设计
- [OpenClaw](https://github.com/openclaw/openclaw) - 语音交互能力参考
- [OpenAI API](https://platform.openai.com/docs/api-reference) - LLM API规范
- [Anthropic API](https://docs.anthropic.com/) - Claude API 规范
- [飞书开放平台](https://open.feishu.cn/document/) - 飞书开发文档
- [MCP 规范](https://modelcontextprotocol.io/) - Model Context Protocol
- [Moltbook](https://www.moltbook.com/) - AI Agent 社交网络
- [Qwen ASR API](https://help.aliyun.com/zh/model-studio/qwen-asr-api-reference) - 阿里云语音识别
