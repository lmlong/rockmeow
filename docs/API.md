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
  "model": "gpt-4o",
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
| GLM | `https://open.bigmodel.cn/api/anthropic` | 智谱 AI |
| MiniMax | `https://api.minimaxi.com/anthropic` | MiniMax AI |
| DashScope | `https://dashscope.aliyuncs.com/compatible-mode/v1` | 阿里云通义 |

---

## 3. 工具调用 API

### 3.1 工具定义格式

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

### 3.2 工具调用响应

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

### 3.3 工具结果提交

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "total 32\ndrwxr-xr-x 4 user user 4096 ..."
}
```

---

## 4. 内置工具

### 4.1 Shell 工具

执行 shell 命令。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| command | string | 是 | Shell 命令 |
| timeout | integer | 否 | 超时秒数，默认 30 |

### 4.2 文件操作工具

| 工具名 | 说明 | 参数 |
|--------|------|------|
| file_read | 读取文件 | `path`: 文件路径 |
| file_write | 写入文件 | `path`, `content` |
| file_edit | 编辑文件 | `path`, `old_string`, `new_string` |
| file_list | 列出目录 | `path`: 目录路径 |

### 4.3 网页工具

| 工具名 | 说明 | 参数 |
|--------|------|------|
| web_fetch | 抓取网页 | `url`: 网页地址 |
| web_search | 搜索网页 | `query`: 搜索关键词 |

### 4.4 Spawn 工具

生成子任务并行处理。

| 参数 | 类型 | 说明 |
|------|------|------|
| tasks | array | 任务列表 |
| tasks[].prompt | string | 任务描述 |
| tasks[].description | string | 任务简述 |

---

## 5. 飞书 Channel API

### 5.1 获取访问令牌

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

### 5.2 获取 WebSocket 连接地址

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

### 5.3 发送消息

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

### 5.4 接收消息事件 (WebSocket)

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

## 6. 错误处理

### 6.1 错误响应格式

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid API key",
    "code": "invalid_api_key"
  }
}
```

### 6.2 常见错误码

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

## 7. Go SDK 使用示例

### 7.1 使用 go-resty 发送请求

```go
package main

import (
    "fmt"
    "github.com/go-resty/resty/v2"
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
    client := resty.New()

    req := ChatRequest{
        Model: "gpt-4o",
        Messages: []Message{
            {Role: "user", Content: "Hello!"},
        },
        Stream: false,
    }

    var resp ChatResponse

    _, err := client.R().
        SetHeader("Content-Type", "application/json").
        SetHeader("Authorization", "Bearer sk-xxx").
        SetBody(req).
        SetResult(&resp).
        Post("https://api.openai.com/v1/chat/completions")

    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Choices[0].Message.Content)
}
```

### 7.2 流式响应处理

```go
client := resty.New()

req := ChatRequest{
    Model:    "gpt-4o",
    Messages: messages,
    Stream:   true,
}

resp, err := client.R().
    SetHeader("Content-Type", "application/json").
    SetHeader("Authorization", "Bearer sk-xxx").
    SetBody(req).
    SetDoNotParseResponse(true).
    Post("https://api.openai.com/v1/chat/completions")

if err != nil {
    return err
}
defer resp.RawBody().Close()

// 处理 SSE 流
scanner := bufio.NewScanner(resp.RawBody())
for scanner.Scan() {
    line := scanner.Text()
    if strings.HasPrefix(line, "data: ") {
        data := strings.TrimPrefix(line, "data: ")
        if data == "[DONE]" {
            break
        }
        // 解析 JSON 数据
        fmt.Println(data)
    }
}
```

---

## 8. 配置参考

### 8.1 Provider 配置

```json
{
  "providers": {
    "openrouter": {
      "apiKey": "sk-or-v1-xxx",
      "apiBase": "https://openrouter.ai/api/v1"
    },
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic"
    }
  }
}
```

### 8.2 Agent 配置

```json
{
  "agents": {
    "defaults": {
      "model": "glm/glm-4-plus",
      "systemPrompt": "You are LingGuard...",
      "maxHistoryMessages": 50,
      "maxToolCalls": 10
    }
  }
}
```

---

## 9. 参考资料

- [OpenAI API Reference](https://platform.openai.com/docs/api-reference)
- [Anthropic API Reference](https://docs.anthropic.com/en/api)
- [飞书开放平台](https://open.feishu.cn/document/)
- [go-resty 文档](https://github.com/go-resty/resty)
