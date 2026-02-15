# LingGuard API 文档

## 概述

LingGuard 提供统一的 LLM API 接口，兼容 OpenAI API 规范，支持多种 LLM 提供商。

---

## 1. LLM Provider API

### 1.1 统一请求格式

所有 LLM Provider 都遵循 OpenAI 兼容的 API 格式：

```
POST {apiBase}/chat/completions
```

### 1.2 请求头

| Header | 值 | 说明 |
|--------|-----|------|
| `Content-Type` | `application/json` | 内容类型 |
| `Authorization` | `Bearer {apiKey}` | API 密钥 |

### 1.3 请求体

```json
{
  "model": "string",           // 必填：模型名称
  "messages": [                // 必填：消息数组
    {
      "role": "system|user|assistant|tool",
      "content": "string",
      "tool_calls": [],        // 可选：工具调用（assistant 角色）
      "tool_call_id": "string" // 可选：工具调用 ID（tool 角色）
    }
  ],
  "tools": [],                 // 可选：工具定义
  "temperature": 0.7,          // 可选：温度参数 (0-2)
  "max_tokens": 4096,          // 可选：最大 token 数
  "stream": false              // 可选：是否流式响应
}
```

### 1.4 响应格式

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "glm-5",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "响应内容",
        "tool_calls": []
      },
      "finish_reason": "stop|tool_calls|length"
    }
  ],
  "usage": {
    "prompt_tokens": 100,
    "completion_tokens": 50,
    "total_tokens": 150
  }
}
```

### 1.5 流式响应 (SSE)

当 `stream: true` 时，返回 Server-Sent Events：

```
data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"Hello"},"index":0}]}

data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":" world"},"index":0}]}

data: [DONE]
```

---

## 2. 支持的 Provider 端点

| Provider | apiBase | 说明 |
|----------|---------|------|
| OpenRouter | `https://openrouter.ai/api/v1` | 推荐，支持所有模型 |
| Anthropic | `https://api.anthropic.com/v1` | Claude 直连 |
| OpenAI | `https://api.openai.com/v1` | GPT 直连 |
| DeepSeek | `https://api.deepseek.com/v1` | DeepSeek |
| Groq | `https://api.groq.com/openai/v1` | 高速推理 |
| Gemini | `https://generativelanguage.googleapis.com/v1beta` | Google Gemini |
| vLLM | `http://localhost:8000/v1` | 本地模型 |
| GLM | `https://open.bigmodel.cn/api/paas/v4` | 智谱 AI |
| MiniMax | `https://api.minimax.chat/v1` | MiniMax AI |
| Moonshot | `https://api.moonshot.cn/v1` | 月之暗面 Kimi |
| Qwen | `https://dashscope.aliyuncs.com/compatible-mode/v1` | 阿里云通义 |
| AiHubMix | `https://aihubmix.com/v1` | 多模型网关 |

---

## 3. Provider 自动匹配（参考 nanobot Provider Registry）

LingGuard 支持根据模型名自动选择 Provider，使用 ProviderSpec 作为单一真实来源。

### 3.1 匹配规则（优先级从高到低）

1. **解析 `provider/model` 格式**: 支持 `glm/glm-4-plus` 格式，直接使用指定 provider
2. **直接匹配 Provider 名称**: 如果 model 值是已注册的 provider 名称，直接使用
3. **关键词匹配**: 根据模型名中的关键词自动匹配（gpt → openai, claude → anthropic）
4. **API Key 前缀匹配**: 根据 API Key 前缀检测（sk-or- → openrouter, gsk_ → groq）
5. **API Base URL 匹配**: 根据 API Base URL 关键词检测
6. **默认 Provider**: 如果以上都不匹配，使用默认 provider

### 3.2 ProviderSpec 规范

```go
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
```

### 3.3 内置 Provider 规范

| Provider | Keywords | DefaultModel | APIKeyPrefix |
|----------|----------|--------------|--------------|
| openai | gpt, o1, o3, chatgpt | gpt-4o | sk- |
| anthropic | claude | claude-3-5-sonnet-20241022 | sk-ant- |
| deepseek | deepseek | deepseek-chat | sk- |
| openrouter | openrouter | anthropic/claude-3.5-sonnet | sk-or- |
| qwen | qwen, tongyi, dashscope | qwen-max | - |
| glm | glm, chatglm, codegeex, zhipu | glm-4 | - |
| minimax | minimax | abab6.5s-chat | - |
| moonshot | moonshot, kimi | moonshot-v1-8k | - |
| gemini | gemini | gemini-1.5-pro | - |
| groq | groq, llama, mixtral, gemma | llama-3.1-70b-versatile | gsk_ |
| vllm | vllm | (需配置) | - |
| aihubmix | aihubmix | (需配置) | - |

### 3.4 配置覆盖机制

config.json 中的配置会覆盖 spec.go 中的默认值：

| 配置项 | config.json | spec.go |
|--------|-------------|---------|
| apiBase | ✅ 覆盖 | 默认值 |
| model | ✅ 覆盖 | 默认值 |
| IsAnthropic | 根据 apiBase 判断 | 默认值 |

**Provider 类型判断：**
- 如果 config.json 配置了 apiBase，根据 apiBase 是否包含 `/anthropic` 判断
- 否则，使用 spec.go 中的 IsAnthropic

### 3.5 配置示例

**简化配置（使用默认值）：**
```json
{
  "providers": {
    "deepseek": {
      "apiKey": "sk-xxx"
    }
  }
}
```

**覆盖默认值：**
```json
{
  "providers": {
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic",
      "model": "glm-5"
    }
  }
}
```

### 3.6 添加新 Provider

只需 2 步：

**步骤 1**: 在 `internal/providers/spec.go` 的 `PROVIDERS` 中添加：

```go
{
    Name:           "myprovider",
    Keywords:       []string{"mymodel"},
    DisplayName:    "My Provider",
    DefaultAPIBase: "https://api.myprovider.com/v1",
    DefaultModel:   "my-model-v1",
}
```

**步骤 2**: 在 `config.json` 中配置：

```json
"providers": {
    "myprovider": {
        "apiKey": "sk-xxx"
    }
}
```

---

## 4. 工具调用 API

### 4.1 工具定义格式

```json
{
  "type": "function",
  "function": {
    "name": "shell",
    "description": "Execute shell commands",
    "parameters": {
      "type": "object",
      "properties": {
        "command": {
          "type": "string",
          "description": "The shell command to execute"
        },
        "timeout": {
          "type": "integer",
          "description": "Timeout in seconds"
        }
      },
      "required": ["command"]
    }
  }
}
```

### 4.2 工具调用响应

当 LLM 决定调用工具时，响应中包含 `tool_calls`：

```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "shell",
        "arguments": "{\"command\":\"ls -la\"}"
      }
    }
  ]
}
```

### 4.3 工具结果提交

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "total 32\ndrwxr-xr-x 4 user user 4096 ..."
}
```

---

## 5. 内置工具

### 5.1 Shell 工具

执行 shell 命令。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| command | string | 是 | Shell 命令 |
| timeout | integer | 否 | 超时秒数，默认 30 |

### 5.2 文件操作工具

| 工具名 | 说明 | 参数 |
|--------|------|------|
| file_read | 读取文件 | `path`: 文件路径 |
| file_write | 写入文件 | `path`, `content` |
| file_edit | 编辑文件 | `path`, `old_string`, `new_string` |
| file_list | 列出目录 | `path`: 目录路径 |

### 5.3 网页工具

#### 5.3.1 web_search

使用 Brave Search API 搜索网页。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | string | 是 | 搜索关键词 |
| count | integer | 否 | 返回结果数量 (1-10)，默认 5 |

**配置要求**：需要设置 `braveApiKey` 配置或 `BRAVE_API_KEY` 环境变量。

**返回示例**：
```
Results for: Go programming language

1. The Go Programming Language
   https://go.dev/
   Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.

2. GitHub - golang/go
   https://github.com/golang/go
   The Go programming language. Contribute to golang/go development by creating an account on GitHub.
```

#### 5.3.2 web_fetch

抓取网页内容并提取可读文本。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| url | string | 是 | 网页地址 |
| extractMode | string | 否 | 提取模式: `markdown` 或 `text`，默认 `markdown` |
| maxChars | integer | 否 | 最大字符数，默认 50000 |

**返回示例**：
```json
{
  "url": "https://example.com",
  "finalUrl": "https://example.com/",
  "status": 200,
  "extractor": "readability",
  "truncated": false,
  "length": 1234,
  "text": "# Example Domain\n\nThis domain is for use in illustrative examples..."
}
```

### 5.4 Spawn 工具

生成子任务并行处理。

| 参数 | 类型 | 说明 |
|------|------|------|
| tasks | array | 任务列表 |
| tasks[].prompt | string | 任务描述 |
| tasks[].description | string | 任务简述 |

---

## 6. 飞书 Channel API

### 6.1 获取访问令牌

```
POST https://open.feishu.cn/open-api/auth/v3/tenant_access_token/internal/
Content-Type: application/json

{
  "app_id": "cli_xxx",
  "app_secret": "xxx"
}
```

**响应：**

```json
{
  "code": 0,
  "msg": "ok",
  "tenant_access_token": "t-xxx",
  "expire": 7200
}
```

### 6.2 获取 WebSocket 连接地址

```
GET https://open.feishu.cn/open-api/bot/v3/ws
Authorization: Bearer {tenant_access_token}
```

**响应：**

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "url": "wss://ws.feishu.cn/xxx"
  }
}
```

### 6.3 发送消息

```
POST https://open.feishu.cn/open-api/im/v1/messages?receive_id_type=open_id
Authorization: Bearer {tenant_access_token}
Content-Type: application/json

{
  "receive_id": "ou_xxx",
  "msg_type": "text",
  "content": "{\"text\":\"Hello\"}"
}
```

### 6.4 接收消息事件 (WebSocket)

```json
{
  "header": {
    "event_id": "xxx",
    "event_type": "im.message.receive_v1",
    "create_time": "1700000000000"
  },
  "event": {
    "sender": {
      "sender_id": {
        "open_id": "ou_xxx"
      }
    },
    "message": {
      "message_id": "om_xxx",
      "content": "{\"text\":\"Hello\"}",
      "create_time": 1700000000
    }
  }
}
```

---

## 7. QQ Channel API

### 7.1 概述

QQ Channel 使用 WebSocket Gateway 连接，支持私聊消息（C2C）和频道消息。

- **网关地址**: `wss://api.sgroup.qq.com/websocket`
- **无需公网 IP**: 使用 WebSocket 长连接接收事件

### 7.2 配置

```json
{
  "channels": {
    "qq": {
      "enabled": true,
      "appId": "xxx",
      "secret": "xxx",
      "allowFrom": []
    }
  }
}
```

### 7.3 WebSocket Gateway 协议

#### 7.3.1 连接流程

1. 建立 WebSocket 连接到 `wss://api.sgroup.qq.com/websocket`
2. 收到 `OP 10 Hello` 后发送 `OP 2 Identify` 进行身份认证
3. 开始心跳（间隔由服务器指定）
4. 接收 `OP 0 Dispatch` 事件

#### 7.3.2 Opcode 定义

| Op | 名称 | 说明 |
|----|------|------|
| 0 | Dispatch | 服务器推送事件 |
| 1 | Heartbeat | 客户端发送心跳 |
| 2 | Identify | 客户端身份认证 |
| 7 | Reconnect | 服务器要求重连 |
| 9 | Invalid Session | 会话失效 |
| 10 | Hello | 服务器欢迎消息 |
| 11 | Heartbeat ACK | 心跳确认 |

#### 7.3.3 Identify Payload

```json
{
  "op": 2,
  "d": {
    "token": {
      "appId": "xxx",
      "token": "secret"
    },
    "intents": 4096,
    "properties": {
      "$os": "linux",
      "$browser": "lingguard",
      "$device": "lingguard"
    }
  }
}
```

### 7.4 发送私聊消息

```
POST https://api.sgroup.qq.com/v2/users/{openid}/messages
Authorization: Bot {appId}.{secret}
Content-Type: application/json

{
  "content": "Hello!",
  "msg_type": 0
}
```

### 7.5 接收消息事件

```json
{
  "op": 0,
  "s": 1,
  "t": "C2C_MESSAGE_CREATE",
  "d": {
    "id": "xxx",
    "content": "你好",
    "timestamp": "2024-01-01T00:00:00+08:00",
    "author": {
      "id": "user_openid",
      "username": "用户名"
    }
  }
}
```

### 7.6 Intents 说明

| Intent | 值 | 说明 |
|--------|-----|------|
| GUILD_MESSAGES | 1 << 9 | 频道消息 |
| DIRECT_MESSAGE | 1 << 12 | 私聊消息 |
| PUBLIC_MESSAGES | 1 << 30 | 公域消息 |

---

## 8. 错误处理

### 8.1 错误响应格式

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid API key",
    "code": "invalid_api_key"
  }
}
```

### 8.2 常见错误码

| HTTP 状态码 | 错误类型 | 说明 |
|-------------|----------|------|
| 400 | invalid_request_error | 请求参数错误 |
| 401 | authentication_error | 认证失败 |
| 403 | permission_error | 权限不足 |
| 404 | not_found_error | 资源不存在 |
| 429 | rate_limit_error | 请求频率限制 |
| 500 | api_error | 服务器内部错误 |
| 503 | overloaded_error | 服务过载 |

---

## 9. Go SDK 使用示例

### 9.1 使用 net/http 发送请求

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type ChatRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
    Stream   bool      `json:"stream"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ChatResponse struct {
    ID      string `json:"id"`
    Model   string `json:"model"`
    Choices []struct {
        Message Message `json:"message"`
    } `json:"choices"`
}

func main() {
    req := ChatRequest{
        Model: "glm-5",
        Messages: []Message{
            {Role: "user", Content: "Hello!"},
        },
        Stream: false,
    }

    body, _ := json.Marshal(req)

    httpReq, _ := http.NewRequest("POST",
        "https://open.bigmodel.cn/api/anthropic/chat/completions",
        bytes.NewReader(body))
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer xxx.xxx")

    resp, err := http.DefaultClient.Do(httpReq)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(resp.Body)

    var chatResp ChatResponse
    json.Unmarshal(respBody, &chatResp)

    fmt.Println(chatResp.Choices[0].Message.Content)
}
```

### 9.2 使用 Provider Registry 自动匹配

```go
package main

import (
    "context"
    "fmt"

    "github.com/lingguard/internal/config"
    "github.com/lingguard/internal/providers"
    "github.com/lingguard/pkg/llm"
)

func main() {
    // 加载配置
    cfg, _ := config.Load("configs/config.json")

    // 创建 Provider 注册表
    registry := providers.NewRegistry()
    registry.InitFromConfig(cfg)

    // 自动匹配 Provider（返回 Provider 和 Spec）
    provider, spec := registry.MatchProvider("glm")
    if provider == nil {
        panic("provider not found")
    }

    fmt.Printf("Provider: %s (%s)\n", spec.DisplayName, spec.Name)

    // 调用 LLM
    req := &llm.Request{
        Model: provider.Model(),
        Messages: []llm.Message{
            {Role: "user", Content: "Hello!"},
        },
    }

    resp, err := provider.Complete(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.GetContent())
}
```

---

## 10. 配置参考

### 10.1 简化配置（推荐）

使用 ProviderSpec 默认值，只需配置 apiKey：

```json
{
  "providers": {
    "deepseek": {
      "apiKey": "sk-xxx"
    },
    "qwen": {
      "apiKey": "sk-xxx"
    },
    "openrouter": {
      "apiKey": "sk-or-v1-xxx",
      "model": "anthropic/claude-opus-4"
    }
  },
  "agents": {
    "workspace": "~/.lingguard/workspace",
    "provider": "openrouter",
    "maxToolIterations": 20,
    "memoryWindow": 50,
    "systemPrompt": "你是灵侍，一个乐于助人的 AI 助手。",
    "memory": {
      "enabled": true,
      "recentDays": 3
    }
  }
}
```

### 10.2 完整配置（覆盖默认值）

```json
{
  "providers": {
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic",
      "model": "glm-5"
    },
    "minimax": {
      "apiKey": "xxx",
      "apiBase": "https://api.minimaxi.com/anthropic",
      "model": "MiniMax-M2.5"
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
      "recentDays": 3,
      "maxHistoryLines": 1000
    }
  }
}
```

### 10.3 配置字段说明

**Provider 配置：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| apiKey | string | ✅ | API 密钥 |
| apiBase | string | 否 | 覆盖默认 API Base（支持 Anthropic 兼容端点） |
| model | string | 否 | 覆盖默认模型 |

**Agent 配置：**

| 字段 | 类型 | 说明 |
|------|------|------|
| workspace | string | 工作空间目录 |
| provider | string | 默认 Provider 名称 |
| maxToolIterations | int | 最大工具调用迭代次数 |
| memoryWindow | int | 历史消息窗口大小 |
| systemPrompt | string | 系统提示词 |
| memory.enabled | bool | 是否启用持久化记忆 |
| memory.recentDays | int | 加载最近几天的日志 |
| memory.maxHistoryLines | int | 历史记录最大行数 |

### 10.4 配置加载优先级

| 优先级 | 来源 | 路径 |
|--------|------|------|
| 1 | 环境变量 | `$LINGGUARD_CONFIG` |
| 2 | 当前目录 | `./config.json` |
| 3 | 用户目录 | `~/.lingguard/config.json` |

---

## 11. 定时任务 (Cron)

LingGuard 支持定时任务功能，可以按计划自动执行 Agent 任务。

### 11.1 CLI 命令

```bash
# 列出所有任务
lingguard cron list
lingguard cron list --all  # 包含已禁用的任务

# 添加任务
lingguard cron add <name> <schedule> <message>

# 删除任务
lingguard cron remove <job-id>

# 启用/禁用任务
lingguard cron enable <job-id>
lingguard cron disable <job-id>

# 手动执行任务
lingguard cron run <job-id>
lingguard cron run <job-id> --force  # 强制执行已禁用的任务

# 查看服务状态
lingguard cron status
```

### 11.2 调度格式

支持三种调度格式：

| 格式 | 示例 | 说明 |
|------|------|------|
| `every:<duration>` | `every:1h` | 重复执行，间隔 1 小时 |
| `at:<datetime>` | `at:2024-12-25 09:00` | 一次性任务，指定时间执行 |
| `cron:<expr>` | `cron:0 9 * * *` | Cron 表达式，每天 9:00 执行 |

**持续时间格式：**
- `30s` - 30 秒
- `5m` - 5 分钟
- `1h` - 1 小时
- `24h` - 24 小时

**Cron 表达式：**
```
分 时 日 月 周
*  *  *  *  *

示例:
0 9 * * *      # 每天 9:00
*/30 * * * *   # 每 30 分钟
0 18 * * 1-5   # 周一到周五 18:00
```

### 11.3 时区支持

使用 `--tz` 参数指定 cron 任务的时区：

```bash
# 纽约时间每天 9:00
lingguard cron add "NYC Morning" "cron:0 9 * * *" "Good morning!" --tz "America/New_York"

# 东京时间每天 18:00
lingguard cron add "Tokyo Evening" "cron:0 18 * * *" "Good evening!" --tz "Asia/Tokyo"

# 上海时间每周一 9:30
lingguard cron add "周一例会" "cron:30 9 * * 1" "周一例会提醒" --tz "Asia/Shanghai"
```

**常用时区：**

| 时区 | 标识符 |
|------|--------|
| 中国 | `Asia/Shanghai` 或 `Asia/Chongqing` |
| 日本 | `Asia/Tokyo` |
| 纽约 | `America/New_York` |
| 洛杉矶 | `America/Los_Angeles` |
| 伦敦 | `Europe/London` |
| UTC | `UTC` |

### 11.4 任务投递选项

添加任务时可以指定将响应投递到消息渠道：

```bash
# 投递到飞书
lingguard cron add "Daily Report" "cron:0 9 * * *" "Generate daily report" \
  --deliver --channel feishu --to ou_xxx

# 带时区投递
lingguard cron add "NYC Report" "cron:0 9 * * *" "Morning report" \
  --tz "America/New_York" --deliver --channel feishu --to ou_xxx
```

### 11.5 任务存储

任务数据存储在 JSON 文件中：

```
~/.lingguard/cron/jobs.json
```

存储格式：

```json
{
  "version": 1,
  "jobs": [
    {
      "id": "abc123",
      "name": "Daily Report",
      "enabled": true,
      "schedule": {
        "kind": "cron",
        "expr": "0 9 * * *",
        "tz": "America/New_York"
      },
      "payload": {
        "kind": "agent_turn",
        "message": "Generate daily report",
        "deliver": true,
        "channel": "feishu",
        "to": "ou_xxx"
      },
      "state": {
        "nextRunAtMs": 1704067200000,
        "lastRunAtMs": 1703980800000,
        "lastStatus": "ok"
      }
    }
  ]
}
```

### 11.6 配置

在 `config.json` 中配置定时任务：

```json
{
  "cron": {
    "enabled": true,
    "storePath": "~/.lingguard/cron/jobs.json"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| enabled | bool | 是否启用定时任务服务 |
| storePath | string | 任务存储文件路径 |

---

## 12. 参考资料

- [OpenAI API Reference](https://platform.openai.com/docs/api-reference)
- [Anthropic API Reference](https://docs.anthropic.com/en/api)
- [智谱 AI API](https://open.bigmodel.cn/dev/api)
- [飞书开放平台](https://open.feishu.cn/document/)
- [QQ 机器人开放平台](https://bot.q.qq.com/wiki/)
