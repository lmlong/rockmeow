# LingGuard Agent API 文档

> 版本: 1.0.0
> 基础 URL: `http://localhost:18989`

## 概述

LingGuard Agent API 是一个智能体服务接口，与 OpenAI Chat API 不同，它是**有状态**的，自动管理会话历史、工具执行和记忆系统。

### 与 OpenAI Chat API 的区别

| 特性 | OpenAI Chat API | LingGuard Agent API |
|------|-----------------|---------------------|
| 状态管理 | 无状态（客户端管理历史） | 有状态（Session 自动管理） |
| 工具执行 | 返回工具调用，客户端执行 | 自动执行，返回最终结果 |
| 记忆系统 | 无 | 内置长期记忆 + 向量检索 |
| 技能系统 | 无 | 渐进式加载，按需注入 |

---

## 认证

所有 API 请求需要在 Header 中携带 Token：

```http
Authorization: Bearer <your-token>
```

Token 在 `config.json` 中配置：

```json
{
  "api": {
    "enabled": true,
    "auth": {
      "type": "token",
      "tokens": ["your-secret-token"]
    }
  }
}
```

---

## API 端点

### 对话 API

#### POST /v1/agents/{agent_id}/chat

与智能体进行对话。支持流式和非流式响应。

**路径参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `agent_id` | string | 智能体标识：`default`（默认）、`coding`、`assistant` |

**请求体**

```json
{
  "message": "帮我分析今天的日程安排",
  "media": ["https://example.com/image.png"],
  "session_id": "user-123-device-456",
  "stream": true,
  "clear_history": false,
  "tools": ["calendar", "web_search"],
  "system_prompt": "你是一个专业的助手"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `message` | string | 是 | 用户消息内容 |
| `media` | []string | 否 | 多媒体 URL 列表（图片、文件等） |
| `session_id` | string | 否 | 会话 ID，不传则新建 |
| `stream` | bool | 否 | 是否流式响应，默认 `false` |
| `clear_history` | bool | 否 | 是否清空历史后对话，默认 `false` |
| `tools` | []string | 否 | 指定可用工具，不传则使用默认集 |
| `system_prompt` | string | 否 | 覆盖默认系统提示词 |

**非流式响应**

```json
{
  "id": "resp-a1b2c3d4",
  "session_id": "user-123-device-456",
  "agent_id": "default",
  "content": "根据您的日历，今天有以下安排：\n\n1. 10:00 - 项目评审会议\n2. 14:00 - 客户电话\n3. 16:00 - 团队周会",
  "tool_calls": [
    {
      "id": "tc-001",
      "tool": "calendar",
      "action": "query",
      "params": {"start": "today", "end": "today"},
      "result": "找到 3 个事件",
      "status": "completed"
    }
  ],
  "usage": {
    "input_tokens": 150,
    "output_tokens": 280,
    "total_tokens": 430
  },
  "created_at": "2026-03-06T10:30:00Z"
}
```

**流式响应（SSE）**

请求设置 `"stream": true`，返回 Server-Sent Events：

```
event: connected
data: {"session_id": "user-123-device-456"}

event: thinking
data: {"content": "让我查一下您的日历..."}

event: tool_call
data: {"id": "tc-001", "tool": "calendar", "action": "query", "params": {"start": "today"}}

event: tool_result
data: {"id": "tc-001", "tool": "calendar", "status": "completed", "result": "找到 3 个事件"}

event: content
data: {"delta": "根据您的日历，"}

event: content
data: {"delta": "今天有以下安排："}

event: content
data: {"delta": "\n\n1. 10:00 - 项目评审会议"}

event: completed
data: {"id": "resp-a1b2c3d4", "usage": {"input_tokens": 150, "output_tokens": 280}}

event: error
data: {"code": "tool_error", "message": "工具执行失败"}
```

---

### 会话 API

#### GET /v1/sessions

获取会话列表。

**查询参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | int | 返回数量，默认 20 |
| `offset` | int | 偏移量，默认 0 |
| `agent_id` | string | 按智能体筛选 |

**响应**

```json
{
  "sessions": [
    {
      "id": "user-123-device-456",
      "title": "日程安排分析",
      "agent_id": "default",
      "message_count": 12,
      "created_at": "2026-03-05T14:00:00Z",
      "updated_at": "2026-03-06T10:30:00Z"
    }
  ],
  "total": 5,
  "limit": 20,
  "offset": 0
}
```

---

#### GET /v1/sessions/{session_id}

获取会话详情，包含历史消息。

**查询参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | int | 返回消息数量，默认 50 |

**响应**

```json
{
  "id": "user-123-device-456",
  "title": "日程安排分析",
  "agent_id": "default",
  "messages": [
    {
      "id": "msg-001",
      "role": "user",
      "content": "帮我分析今天的日程",
      "created_at": "2026-03-06T10:00:00Z"
    },
    {
      "id": "msg-002",
      "role": "assistant",
      "content": "根据您的日历...",
      "tool_calls": [...],
      "created_at": "2026-03-06T10:00:05Z"
    }
  ],
  "message_count": 12,
  "created_at": "2026-03-05T14:00:00Z",
  "updated_at": "2026-03-06T10:30:00Z"
}
```

---

#### DELETE /v1/sessions/{session_id}

删除会话及其历史记录。

**响应**

```json
{
  "message": "session deleted",
  "id": "user-123-device-456"
}
```

---

#### POST /v1/sessions/{session_id}/clear

清空会话历史，保留会话本身。

**响应**

```json
{
  "message": "session cleared",
  "id": "user-123-device-456"
}
```

---

### 任务 API

用于长时间运行的异步任务。

#### POST /v1/tasks

创建异步任务。

**请求体**

```json
{
  "prompt": "帮我重构整个项目的代码结构，优化性能",
  "session_id": "user-123",
  "agent_id": "coding",
  "tools": ["shell", "file", "opencode"],
  "callback_url": "https://your-server.com/callback"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `prompt` | string | 是 | 任务描述 |
| `session_id` | string | 否 | 关联的会话 ID |
| `agent_id` | string | 否 | 智能体 ID，默认 `default` |
| `tools` | []string | 否 | 可用工具列表 |
| `callback_url` | string | 否 | 任务完成回调 URL |

**响应**

```json
{
  "id": "task-abc123",
  "status": "pending",
  "prompt": "帮我重构整个项目的代码结构...",
  "agent_id": "coding",
  "created_at": "2026-03-06T10:00:00Z"
}
```

---

#### GET /v1/tasks/{task_id}

查询任务状态。

**响应**

```json
{
  "id": "task-abc123",
  "status": "running",
  "progress": 45,
  "progress_message": "正在分析代码结构...",
  "result": null,
  "error": null,
  "agent_id": "coding",
  "created_at": "2026-03-06T10:00:00Z",
  "updated_at": "2026-03-06T10:05:00Z"
}
```

**状态值**

| 状态 | 说明 |
|------|------|
| `pending` | 等待执行 |
| `running` | 执行中 |
| `completed` | 已完成 |
| `failed` | 执行失败 |
| `cancelled` | 已取消 |

---

#### POST /v1/tasks/{task_id}/cancel

取消正在执行的任务。

**响应**

```json
{
  "id": "task-abc123",
  "status": "cancelled",
  "message": "task cancelled by user"
}
```

---

#### GET /v1/tasks/{task_id}/events

获取任务执行事件的 SSE 流。

**响应**

```
event: started
data: {"task_id": "task-abc123", "timestamp": "2026-03-06T10:00:00Z"}

event: progress
data: {"progress": 20, "message": "正在读取项目文件..."}

event: tool_call
data: {"tool": "file", "action": "list", "path": "/src"}

event: progress
data: {"progress": 45, "message": "正在分析代码结构..."}

event: content
data: {"delta": "我建议进行以下重构..."}

event: completed
data: {"result": "重构方案已生成，共 5 个优化点", "progress": 100}
```

---

### 工具 API

#### GET /v1/tools

获取可用工具列表。

**响应**

```json
{
  "tools": [
    {
      "name": "shell",
      "description": "执行 shell 命令",
      "dangerous": true,
      "enabled": true
    },
    {
      "name": "file",
      "description": "文件读写操作",
      "dangerous": false,
      "enabled": true
    },
    {
      "name": "calendar",
      "description": "日历管理（飞书/钉钉）",
      "dangerous": false,
      "enabled": true
    },
    {
      "name": "web_search",
      "description": "网页搜索",
      "dangerous": false,
      "enabled": true
    },
    {
      "name": "aigc",
      "description": "AI 图像/视频生成",
      "dangerous": false,
      "enabled": false
    }
  ]
}
```

---

#### POST /v1/tools/{tool_name}/execute

直接执行工具（不经过 Agent）。

**请求体**

```json
{
  "action": "query",
  "params": {
    "start": "today",
    "end": "tomorrow"
  }
}
```

**响应**

```json
{
  "tool": "calendar",
  "action": "query",
  "result": "找到 3 个事件：\n1. 10:00 - 项目评审\n2. 14:00 - 客户电话\n3. 16:00 - 周会",
  "success": true,
  "duration_ms": 320
}
```

---

### 智能体 API

#### GET /v1/agents

获取可用智能体列表。

**响应**

```json
{
  "agents": [
    {
      "id": "default",
      "name": "灵侍",
      "description": "通用智能助手",
      "enabled": true,
      "default": true
    },
    {
      "id": "coding",
      "name": "编程助手",
      "description": "专注于代码开发和调试",
      "enabled": true,
      "default": false
    }
  ]
}
```

---

#### GET /v1/agents/{agent_id}

获取智能体详情。

**响应**

```json
{
  "id": "default",
  "name": "灵侍",
  "description": "通用智能助手",
  "provider": "glm",
  "model": "glm-5",
  "tools": ["shell", "file", "calendar", "web_search", "memory"],
  "skills": ["calendar", "weather", "clawhub"],
  "system_prompt": "你是灵侍，一个乐于助人的 AI 助手..."
}
```

---

## 错误响应

所有错误使用统一格式：

```json
{
  "error": {
    "code": "session_not_found",
    "message": "会话不存在",
    "details": {
      "session_id": "user-xxx"
    }
  }
}
```

**错误码**

| HTTP 状态码 | 错误码 | 说明 |
|------------|--------|------|
| 400 | `invalid_request` | 请求参数错误 |
| 400 | `missing_message` | 缺少消息内容 |
| 401 | `unauthorized` | Token 无效或过期 |
| 403 | `forbidden` | 无权限访问 |
| 404 | `session_not_found` | 会话不存在 |
| 404 | `agent_not_found` | 智能体不存在 |
| 404 | `task_not_found` | 任务不存在 |
| 409 | `session_busy` | 会话正在处理其他请求 |
| 429 | `rate_limit_exceeded` | 请求频率超限 |
| 500 | `internal_error` | 服务器内部错误 |
| 503 | `provider_error` | LLM 服务不可用 |
| 504 | `timeout` | 请求超时 |

---

## SDK 示例

### Swift (iOS)

```swift
import Foundation

class LingGuardClient {
    let baseURL: String
    let token: String

    init(baseURL: String, token: String) {
        self.baseURL = baseURL
        self.token = token
    }

    // 简单对话
    func chat(message: String, sessionId: String? = nil) async throws -> ChatResponse {
        var request = URLRequest(url: URL(string: "\(baseURL)/v1/agents/default/chat")!)
        request.httpMethod = "POST"
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")

        var body: [String: Any] = ["message": message, "stream": false]
        if let sessionId = sessionId {
            body["session_id"] = sessionId
        }
        request.httpBody = try JSONSerialization.data(withJSONObject: body)

        let (data, _) = try await URLSession.shared.data(for: request)
        return try JSONDecoder().decode(ChatResponse.self, from: data)
    }

    // 流式对话
    func streamChat(message: String, sessionId: String? = nil) -> AsyncThrowingStream<StreamEvent, Error> {
        AsyncThrowingStream { continuation in
            Task {
                var request = URLRequest(url: URL(string: "\(baseURL)/v1/agents/default/chat")!)
                request.httpMethod = "POST"
                request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                var body: [String: Any] = ["message": message, "stream": true]
                if let sessionId = sessionId {
                    body["session_id"] = sessionId
                }
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                let (bytes, _) = try await URLSession.shared.bytes(for: request)

                for try await line in bytes.lines {
                    if line.hasPrefix("data: ") {
                        let jsonStr = String(line.dropFirst(6))
                        let event = try parseSSEEvent(jsonStr)
                        continuation.yield(event)

                        if case .completed = event {
                            continuation.finish()
                        }
                    }
                }
            }
        }
    }
}

// 使用示例
let client = LingGuardClient(baseURL: "http://localhost:18989", token: "your-token")

// 非流式
let response = try await client.chat(message: "今天有什么安排？")
print(response.content)

// 流式
for try await event in client.streamChat(message: "帮我搜索最新的 AI 新闻") {
    switch event {
    case .content(let delta):
        print(delta, terminator: "")
    case .completed:
        print("\n完成！")
    default:
        break
    }
}
```

### Kotlin (Android)

```kotlin
class LingGuardClient(
    private val baseURL: String,
    private val token: String
) {
    private val client = OkHttpClient()
    private val json = Json { ignoreUnknownKeys = true }

    suspend fun chat(
        message: String,
        sessionId: String? = null,
        agentId: String = "default"
    ): ChatResponse = withContext(Dispatchers.IO) {
        val body = buildJsonObject {
            put("message", message)
            put("stream", false)
            sessionId?.let { put("session_id", it) }
        }

        val request = Request.Builder()
            .url("$baseURL/v1/agents/$agentId/chat")
            .header("Authorization", "Bearer $token")
            .post(body.toString().toRequestBody("application/json".toMediaType()))
            .build()

        client.newCall(request).execute().use { response ->
            json.decodeFromString(response.body!!.string())
        }
    }

    fun streamChat(
        message: String,
        sessionId: String? = null,
        agentId: String = "default",
        onEvent: (StreamEvent) -> Unit
    ) {
        val body = buildJsonObject {
            put("message", message)
            put("stream", true)
            sessionId?.let { put("session_id", it) }
        }

        val request = Request.Builder()
            .url("$baseURL/v1/agents/$agentId/chat")
            .header("Authorization", "Bearer $token")
            .header("Accept", "text/event-stream")
            .post(body.toString().toRequestBody("application/json".toMediaType()))
            .build()

        client.newCall(request).execute().use { response ->
            response.body!!.source().buffer().use { buffer ->
                while (!buffer.exhausted()) {
                    val line = buffer.readUtf8Line() ?: break
                    if (line.startsWith("data: ")) {
                        val event = parseSSEEvent(line.removePrefix("data: "))
                        onEvent(event)
                    }
                }
            }
        }
    }
}

// 使用示例
val client = LingGuardClient("http://localhost:18989", "your-token")

// 非流式
lifecycleScope.launch {
    val response = client.chat("今天有什么安排？")
    println(response.content)
}

// 流式
client.streamChat("帮我搜索最新的 AI 新闻") { event ->
    when (event) {
        is StreamEvent.Content -> print(event.delta)
        is StreamEvent.Completed -> println("\n完成！")
        else -> {}
    }
}
```

### TypeScript (Web)

```typescript
interface ChatRequest {
  message: string;
  media?: string[];
  session_id?: string;
  stream?: boolean;
  clear_history?: boolean;
  tools?: string[];
}

interface ChatResponse {
  id: string;
  session_id: string;
  agent_id: string;
  content: string;
  tool_calls: ToolCall[];
  usage: Usage;
  created_at: string;
}

class LingGuardClient {
  private baseURL: string;
  private token: string;

  constructor(baseURL: string, token: string) {
    this.baseURL = baseURL;
    this.token = token;
  }

  // 简单对话
  async chat(message: string, sessionId?: string): Promise<ChatResponse> {
    const response = await fetch(`${this.baseURL}/v1/agents/default/chat`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        message,
        session_id: sessionId,
        stream: false,
      }),
    });

    if (!response.ok) {
      throw new Error(`API Error: ${response.status}`);
    }

    return response.json();
  }

  // 流式对话
  async *streamChat(
    message: string,
    sessionId?: string
  ): AsyncGenerator<StreamEvent> {
    const response = await fetch(`${this.baseURL}/v1/agents/default/chat`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.token}`,
        'Content-Type': 'application/json',
        'Accept': 'text/event-stream',
      },
      body: JSON.stringify({
        message,
        session_id: sessionId,
        stream: true,
      }),
    });

    const reader = response.body!.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const event = JSON.parse(line.slice(6));
          yield event;
        }
      }
    }
  }
}

// 使用示例
const client = new LingGuardClient('http://localhost:18989', 'your-token');

// 非流式
const response = await client.chat('今天有什么安排？');
console.log(response.content);

// 流式
for await (const event of client.streamChat('帮我搜索最新的 AI 新闻')) {
  switch (event.event) {
    case 'content':
      process.stdout.write(event.data.delta);
      break;
    case 'completed':
      console.log('\n完成！');
      break;
  }
}
```

---

## 最佳实践

### Session ID 设计

推荐格式：`{source}-{user_id}-{device_id}`

```javascript
// APP 端生成示例
const sessionId = `app-${userId}-${deviceId}`;
// 结果: app-user123-device456
```

### 流式响应处理

```swift
// 推荐：累积内容，最后更新 UI
var fullContent = ""

for try await event in client.streamChat(message: "...") {
    switch event {
    case .content(let delta):
        fullContent += delta
        // 可选：实时更新（节流）
        updateUI(fullContent)
    case .completed:
        // 最终更新
        saveMessage(fullContent)
    case .toolCall(let tool, let action):
        showToolIndicator(tool, action)
    case .error(let message):
        showError(message)
    default:
        break
    }
}
```

### 错误重试

```swift
func chatWithRetry(message: String, maxRetries: Int = 3) async throws -> ChatResponse {
    var lastError: Error?

    for i in 0..<maxRetries {
        do {
            return try await client.chat(message: message)
        } catch let error as APIError where error.isRetryable {
            lastError = error
            try await Task.sleep(nanoseconds: UInt64(pow(2.0, Double(i))) * 1_000_000_000)
        }
    }

    throw lastError!
}
```

---

## 实施计划

### 架构设计

```
cmd/lingguard/
└── main.go                    # 修改 gateway 子命令使用 Gin

internal/api/
├── server.go                  # Gin 统一服务器入口
├── router.go                  # 路由注册
├── middleware/
│   ├── auth.go                # Token 认证中间件
│   ├── ratelimit.go           # 限流中间件
│   ├── cors.go                # CORS 中间件
│   └── requestid.go           # Request ID 中间件
├── handlers/
│   ├── chat.go                # Chat API 处理器
│   ├── session.go             # Session API 处理器
│   ├── task.go                # Task API 处理器
│   ├── tool.go                # Tool API 处理器
│   ├── agent.go               # Agent API 处理器
│   ├── taskboard.go           # TaskBoard API（迁移）
│   └── websocket.go           # WebSocket 处理器（迁移）
├── models/
│   ├── request.go             # 请求结构体
│   ├── response.go            # 响应结构体
│   └── sse.go                 # SSE 事件结构体
├── sse/
│   ├── writer.go              # SSE 写入器
│   └── event.go               # 事件类型定义
└── task/
    ├── manager.go             # 任务管理器
    └── store.go               # 任务存储（内存/文件）

internal/taskboard/
├── server.go                  # 删除（迁移到 internal/api/）
├── handler.go                 # 保留（业务逻辑）
└── web/                       # 静态文件（保留）
```

## 统一路由设计

```
Gin Router (单端口 :8080)
│
├── /v1/*                          # Agent API (新增)
│   ├── POST /agents/:id/chat      # 对话
│   ├── GET  /sessions             # 会话列表
│   ├── GET  /sessions/:id         # 会话详情
│   ├── POST /tasks                # 创建任务
│   ├── GET  /tasks/:id/events     # SSE
│   ├── GET  /tools                # 工具列表
│   └── GET  /agents               # 智能体列表
│
├── /api/*                         # TaskBoard API (迁移)
│   ├── GET  /tasks                # 任务列表
│   ├── POST /tasks                # 创建任务
│   ├── GET  /crons                # 定时任务
│   └── GET  /stats                # 统计
│
├── /ws/*                          # WebSocket (迁移)
│   └── /ws/chat                   # WebChat
│
├── /trace/*                       # Trace API (迁移)
│   └── GET  /sessions             # 追踪会话
│
└── /*                             # 静态文件 (迁移)
    └── /index.html, /static/*     # Web UI
```

### Phase 1: 基础框架 (Day 1-2)

**目标**: 搭建 Gin 服务器骨架，支持基础路由和中间件

#### 1.1 添加依赖

```bash
go get -u github.com/gin-gonic/gin
```

#### 1.2 配置扩展

修改 `internal/config/config.go`:

```go
type APIConfig struct {
    Enabled     bool              `json:"enabled"`
    Port        int               `json:"port,omitempty"`        // 默认 18989
    Host        string            `json:"host,omitempty"`        // 默认 0.0.0.0
    Auth        *AuthConfig       `json:"auth,omitempty"`
    RateLimit   *RateLimitConfig  `json:"rateLimit,omitempty"`
    CORS        *CORSConfig       `json:"cors,omitempty"`
}

type AuthConfig struct {
    Type    string   `json:"type"`              // "token" | "none"
    Tokens  []string `json:"tokens,omitempty"`
}

type RateLimitConfig struct {
    Enabled      bool `json:"enabled"`
    RequestsPer  int  `json:"requestsPer"`       // 每分钟请求数
    Burst        int  `json:"burst"`             // 突发容量
}

type CORSConfig struct {
    Enabled        bool     `json:"enabled"`
    AllowedOrigins []string `json:"allowedOrigins,omitempty"`
}
```

配置示例 (`config.json`):

```json
{
  "api": {
    "enabled": true,
    "port": 18989,
    "auth": {
      "type": "token",
      "tokens": ["your-secret-token"]
    },
    "rateLimit": {
      "enabled": true,
      "requestsPer": 60,
      "burst": 10
    },
    "cors": {
      "enabled": true,
      "allowedOrigins": ["*"]
    }
  }
}
```

#### 1.3 Gin 统一服务器

创建 `internal/api/server.go`，整合现有 taskboard：

```go
package api

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "github.com/lingguard/internal/config"
    "github.com/lingguard/internal/taskboard"
    "github.com/lingguard/internal/taskboard/web"
    "github.com/lingguard/pkg/logger"
)

type Server struct {
    config     *config.Config
    httpServer *http.Server
    router     *gin.Engine

    // Services
    agentService  *agent.Service
    sessionMgr    *session.Manager
    taskMgr       *task.Manager
    taskboardSvc  *taskboard.Service
    traceSvc      *trace.Service

    // WebSocket
    wsHandler     WebSocketHandler
}

type WebSocketHandler interface {
    HandleWebSocket(conn *websocket.Conn, sessionID string)
}

func NewServer(cfg *config.Config, opts ...ServerOption) *Server {
    // 设置 Gin 模式
    if cfg.Logging.Level == "debug" {
        gin.SetMode(gin.DebugMode)
    } else {
        gin.SetMode(gin.ReleaseMode)
    }

    router := gin.New()

    // 内置中间件
    router.Use(gin.Recovery())
    router.Use(RequestIDMiddleware())
    router.Use(LoggerMiddleware())

    // CORS 中间件（全局）
    if cfg.CORS != nil && cfg.CORS.Enabled {
        router.Use(CORSMiddleware(cfg.CORS))
    }

    // Agent API 专用中间件（/v1/* 路由组）
    if cfg.API != nil && cfg.API.Enabled {
        v1 := router.Group("/v1")
        if cfg.API.Auth != nil && cfg.API.Auth.Type == "token" {
            v1.Use(AuthMiddleware(cfg.API.Auth.Tokens))
        }
        if cfg.API.RateLimit != nil && cfg.API.RateLimit.Enabled {
            v1.Use(RateLimitMiddleware(cfg.API.RateLimit))
        }
    }

    s := &Server{
        config: cfg,
        router: router,
    }

    // 应用选项
    for _, opt := range opts {
        opt(s)
    }

    // 注册路由
    s.registerRoutes()

    return s
}

func (s *Server) registerRoutes() {
    // ========== Agent API (/v1/*) ==========
    v1 := s.router.Group("/v1")
    {
        // Health (无需认证)
        v1.GET("/health", func(c *gin.Context) {
            c.JSON(200, gin.H{"status": "ok"})
        })

        // Chat
        v1.POST("/agents/:agent_id/chat", s.handleChat)

        // Sessions
        v1.GET("/sessions", s.handleListSessions)
        v1.GET("/sessions/:session_id", s.handleGetSession)
        v1.DELETE("/sessions/:session_id", s.handleDeleteSession)
        v1.POST("/sessions/:session_id/clear", s.handleClearSession)

        // Tasks (Agent 异步任务)
        v1.POST("/tasks", s.handleCreateTask)
        v1.GET("/tasks/:task_id", s.handleGetTask)
        v1.POST("/tasks/:task_id/cancel", s.handleCancelTask)
        v1.GET("/tasks/:task_id/events", s.handleTaskEvents)

        // Tools
        v1.GET("/tools", s.handleListTools)
        v1.POST("/tools/:tool_name/execute", s.handleExecuteTool)

        // Agents
        v1.GET("/agents", s.handleListAgents)
        v1.GET("/agents/:agent_id", s.handleGetAgent)
    }

    // ========== TaskBoard API (/api/*) ==========
    if s.taskboardSvc != nil {
        api := s.router.Group("/api")
        {
            api.GET("/tasks", s.handleTaskboardList)
            api.POST("/tasks", s.handleTaskboardCreate)
            api.PUT("/tasks/:id", s.handleTaskboardUpdate)
            api.DELETE("/tasks/:id", s.handleTaskboardDelete)
            api.GET("/crons", s.handleTaskboardCrons)
            api.DELETE("/crons/:id", s.handleTaskboardDeleteCron)
            api.GET("/stats", s.handleTaskboardStats)
        }
    }

    // ========== Trace API (/trace/*) ==========
    if s.traceSvc != nil {
        trace := s.router.Group("/trace")
        {
            trace.GET("/sessions", s.handleTraceSessions)
            trace.GET("/sessions/:id", s.handleTraceSession)
            trace.GET("/sessions/:id/events", s.handleTraceEvents)
        }
    }

    // ========== WebSocket (/ws/*) ==========
    if s.wsHandler != nil {
        s.router.GET("/ws/chat", s.handleWebSocket)
    }

    // ========== 静态文件 (Web UI) ==========
    s.registerStaticFiles()
}

// registerStaticFiles 注册静态文件服务
func (s *Server) registerStaticFiles() {
    // 从嵌入的 FS 读取静态文件
    staticFS, err := fs.Sub(web.StaticFiles, "static")
    if err != nil {
        logger.Error("Failed to load static files", "error", err)
        return
    }

    // 首页
    s.router.GET("/", func(c *gin.Context) {
        c.FileFromFS("index.html", http.FS(staticFS))
    })

    // 静态资源
    s.router.StaticFS("/static", http.FS(staticFS))

    // Favicon 等
    s.router.GET("/favicon.ico", func(c *gin.Context) {
        c.FileFromFS("favicon.ico", http.FS(staticFS))
    })
}

// handleWebSocket 处理 WebSocket 连接
func (s *Server) handleWebSocket(c *gin.Context) {
    if s.wsHandler == nil {
        c.JSON(503, gin.H{"error": "WebSocket not enabled"})
        return
    }

    sessionID := c.Query("session")
    if sessionID == "" {
        sessionID = uuid.New().String()
    }

    // 升级 WebSocket
    upgrader := websocket.Upgrader{
        ReadBufferSize:  1024,
        WriteBufferSize: 1024,
        CheckOrigin: func(r *http.Request) bool {
            return true // CORS 由中间件处理
        },
    }

    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        logger.Warn("WebSocket upgrade failed", "error", err)
        return
    }

    logger.Info("WebSocket connected", "sessionId", sessionID)
    s.wsHandler.HandleWebSocket(conn, sessionID)
}

func (s *Server) Start(ctx context.Context) error {
    port := s.config.Gateway.Port
    if s.config.API != nil && s.config.API.Port > 0 {
        port = s.config.API.Port
    }

    addr := fmt.Sprintf("%s:%d", s.config.Gateway.Host, port)
    s.httpServer = &http.Server{
        Addr:         addr,
        Handler:      s.router,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 120 * time.Second,
    }

    logger.Info("Server starting", "addr", addr)
    return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
    if s.httpServer == nil {
        return nil
    }
    logger.Info("Server stopping")
    return s.httpServer.Shutdown(ctx)
}

// ServerOption 服务器选项
type ServerOption func(*Server)

func WithAgentService(svc *agent.Service) ServerOption {
    return func(s *Server) { s.agentService = svc }
}

func WithSessionManager(mgr *session.Manager) ServerOption {
    return func(s *Server) { s.sessionMgr = mgr }
}

func WithTaskManager(mgr *task.Manager) ServerOption {
    return func(s *Server) { s.taskMgr = mgr }
}

func WithTaskboardService(svc *taskboard.Service) ServerOption {
    return func(s *Server) { s.taskboardSvc = svc }
}

func WithTraceService(svc *trace.Service) ServerOption {
    return func(s *Server) { s.traceSvc = svc }
}

func WithWebSocketHandler(h WebSocketHandler) ServerOption {
    return func(s *Server) { s.wsHandler = h }
}
```

#### 1.4 中间件实现

创建 `internal/api/middleware/auth.go`:

```go
package middleware

import (
    "strings"

    "github.com/gin-gonic/gin"
)

func AuthMiddleware(validTokens []string) gin.HandlerFunc {
    tokenSet := make(map[string]bool)
    for _, t := range validTokens {
        tokenSet[t] = true
    }

    return func(c *gin.Context) {
        // 跳过健康检查
        if c.Request.URL.Path == "/v1/health" {
            c.Next()
            return
        }

        auth := c.GetHeader("Authorization")
        if auth == "" {
            c.JSON(401, gin.H{
                "error": gin.H{
                    "code":    "unauthorized",
                    "message": "Missing Authorization header",
                },
            })
            c.Abort()
            return
        }

        token := strings.TrimPrefix(auth, "Bearer ")
        if !tokenSet[token] {
            c.JSON(401, gin.H{
                "error": gin.H{
                    "code":    "unauthorized",
                    "message": "Invalid token",
                },
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

创建 `internal/api/middleware/ratelimit.go`:

```go
package middleware

import (
    "sync"
    "time"

    "github.com/gin-gonic/gin"
)

func RateLimitMiddleware(cfg *config.RateLimitConfig) gin.HandlerFunc {
    type client struct {
        count    int
        lastSeen time.Time
    }

    var mu sync.Mutex
    clients := make(map[string]*client)

    // 清理过期记录
    go func() {
        for range time.Tick(time.Minute) {
            mu.Lock()
            for ip, c := range clients {
                if time.Since(c.lastSeen) > time.Minute {
                    delete(clients, ip)
                }
            }
            mu.Unlock()
        }
    }()

    return func(c *gin.Context) {
        ip := c.ClientIP()

        mu.Lock()
        cl, exists := clients[ip]
        if !exists {
            cl = &client{count: 0, lastSeen: time.Now()}
            clients[ip] = cl
        }

        if cl.count >= cfg.RequestsPer {
            mu.Unlock()
            c.JSON(429, gin.H{
                "error": gin.H{
                    "code":    "rate_limit_exceeded",
                    "message": "Too many requests",
                },
            })
            c.Abort()
            return
        }

        cl.count++
        cl.lastSeen = time.Now()
        mu.Unlock()

        c.Next()
    }
}
```

#### 1.5 中间件实现

创建 `internal/api/middleware/` 目录下的中间件：

**auth.go** - Token 认证：
```go
package middleware

import (
    "strings"
    "github.com/gin-gonic/gin"
)

func AuthMiddleware(validTokens []string) gin.HandlerFunc {
    tokenSet := make(map[string]bool)
    for _, t := range validTokens {
        tokenSet[t] = true
    }

    return func(c *gin.Context) {
        // 跳过健康检查
        if c.Request.URL.Path == "/v1/health" {
            c.Next()
            return
        }

        auth := c.GetHeader("Authorization")
        if auth == "" {
            c.JSON(401, gin.H{"error": gin.H{
                "code":    "unauthorized",
                "message": "Missing Authorization header",
            }})
            c.Abort()
            return
        }

        token := strings.TrimPrefix(auth, "Bearer ")
        if !tokenSet[token] {
            c.JSON(401, gin.H{"error": gin.H{
                "code":    "unauthorized",
                "message": "Invalid token",
            }})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

**cors.go** - CORS 支持：
```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/lingguard/internal/config"
)

func CORSMiddleware(cfg *config.CORSConfig) gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        allowedOrigin := "*"

        if cfg != nil && len(cfg.AllowedOrigins) > 0 {
            for _, o := range cfg.AllowedOrigins {
                if o == "*" || o == origin {
                    allowedOrigin = o
                    if o != "*" {
                        allowedOrigin = origin
                    }
                    break
                }
            }
        }

        c.Header("Access-Control-Allow-Origin", allowedOrigin)
        c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if cfg != nil && cfg.AllowCredentials {
            c.Header("Access-Control-Allow-Credentials", "true")
        }

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}
```

**requestid.go** - Request ID：
```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

func RequestIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        requestID := c.GetHeader("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()[:8]
        }
        c.Header("X-Request-ID", requestID)
        c.Next()
    }
}
```

**logger.go** - 请求日志：
```go
package middleware

import (
    "time"
    "github.com/gin-gonic/gin"
    "github.com/lingguard/pkg/logger"
)

func LoggerMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path

        c.Next()

        latency := time.Since(start)
        logger.Info("HTTP Request",
            "method", c.Request.Method,
            "path", path,
            "status", c.Writer.Status(),
            "latency", latency.String(),
            "ip", c.ClientIP(),
        )
    }
}
```

#### 1.6 TaskBoard Handler 迁移

将现有 `internal/taskboard/http_handler.go` 迁移到 Gin：

**迁移前 (net/http)**:
```go
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("/api/tasks", h.handleTasks)
    mux.HandleFunc("/api/crons", h.handleCrons)
    mux.HandleFunc("/api/stats", h.handleStats)
}

func (h *HTTPHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        // ...
    case http.MethodPost:
        // ...
    }
}
```

**迁移后 (Gin)**:
```go
// internal/api/handlers/taskboard.go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/lingguard/internal/taskboard"
)

type TaskboardHandler struct {
    service *taskboard.Service
}

func NewTaskboardHandler(svc *taskboard.Service) *TaskboardHandler {
    return &TaskboardHandler{service: svc}
}

func (h *TaskboardHandler) List(c *gin.Context) {
    tasks := h.service.ListTasks()
    c.JSON(200, gin.H{"tasks": tasks})
}

func (h *TaskboardHandler) Create(c *gin.Context) {
    var req taskboard.TaskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    task, err := h.service.CreateTask(&req)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(201, task)
}

func (h *TaskboardHandler) Update(c *gin.Context) {
    id := c.Param("id")
    var req taskboard.TaskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    task, err := h.service.UpdateTask(id, &req)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, task)
}

func (h *TaskboardHandler) Delete(c *gin.Context) {
    id := c.Param("id")
    if err := h.service.DeleteTask(id); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, gin.H{"message": "deleted"})
}
```

#### 1.7 CLI 命令集成

修改 `cmd/lingguard/main.go`，统一使用 Gin 服务器：

```go
case "gateway":
    // 创建 Gin 服务器
    server := api.NewServer(cfg,
        api.WithAgentService(agentSvc),
        api.WithSessionManager(sessionMgr),
        api.WithTaskManager(taskMgr),
        api.WithTaskboardService(taskboardSvc),
        api.WithWebSocketHandler(wsHandler),
    )

    // 启动服务
    if err := server.Start(ctx); err != nil {
        return err
    }

    // 等待中断信号
    <-ctx.Done()
    return server.Stop(context.Background())
```

**验收标准**:
- ✅ `./lingguard gateway` 启动统一服务器
- ✅ `GET /v1/health` 返回 200
- ✅ `GET /api/tasks` 返回任务列表（TaskBoard）
- ✅ `GET /` 返回 Web UI 首页
- ✅ `GET /ws/chat` WebSocket 连接成功
- ✅ Token 认证中间件正常工作（/v1/* 路由）
- ✅ 无 Token 访问 /v1/* 返回 401

### Phase 0: 迁移准备 (Day 0)

**目标**: 添加 Gin 依赖，确保现有功能不受影响

#### 0.1 添加依赖

```bash
go get -u github.com/gin-gonic/gin
```

#### 0.2 更新 go.mod

```go
require (
    github.com/gin-gonic/gin v1.9.1
    // ... 其他依赖
)
```

#### 0.3 兼容性测试

1. 添加 Gin 依赖后运行现有测试
2. 确保 `./lingguard gateway` 仍能正常启动
3. 逐步迁移，保持功能可用

---

---

### Phase 2: Chat API (Day 3-4) ✅ 已完成

**目标**: 实现核心对话接口，支持流式和非流式响应

#### 2.1 请求/响应模型

创建 `internal/api/models/request.go`:

```go
package models

type ChatRequest struct {
    Message      string   `json:"message" binding:"required"`
    Media        []string `json:"media,omitempty"`
    SessionID    string   `json:"session_id,omitempty"`
    Stream       bool     `json:"stream,omitempty"`
    ClearHistory bool     `json:"clear_history,omitempty"`
    Tools        []string `json:"tools,omitempty"`
    SystemPrompt string   `json:"system_prompt,omitempty"`
}
```

创建 `internal/api/models/response.go`:

```go
package models

import "time"

type ChatResponse struct {
    ID        string     `json:"id"`
    SessionID string     `json:"session_id"`
    AgentID   string     `json:"agent_id"`
    Content   string     `json:"content"`
    ToolCalls []ToolCall `json:"tool_calls,omitempty"`
    Usage     *Usage     `json:"usage,omitempty"`
    CreatedAt time.Time  `json:"created_at"`
}

type ToolCall struct {
    ID     string                 `json:"id"`
    Tool   string                 `json:"tool"`
    Action string                 `json:"action"`
    Params map[string]interface{} `json:"params"`
    Result string                 `json:"result"`
    Status string                 `json:"status"`
}

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
    TotalTokens  int `json:"total_tokens"`
}

type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}
```

#### 2.2 Chat Handler (Gin 版本)

创建 `internal/api/handlers/chat.go`:

```go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/lingguard/internal/api/models"
    "github.com/lingguard/internal/api/sse"
)

type ChatHandler struct {
    agentService *agent.Service
    sessionMgr   *session.Manager
}

func NewChatHandler(agentService *agent.Service, sessionMgr *session.Manager) *ChatHandler {
    return &ChatHandler{
        agentService: agentService,
        sessionMgr:   sessionMgr,
    }
}

func (h *ChatHandler) Handle(c *gin.Context) {
    agentID := c.Param("agent_id")

    var req models.ChatRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "invalid_request",
                Message: err.Error(),
            },
        })
        return
    }

    // 获取或创建 Session
    sess := h.sessionMgr.GetOrCreate(req.SessionID)

    // 清空历史
    if req.ClearHistory {
        sess.ClearHistory()
    }

    // 流式响应
    if req.Stream {
        h.handleStream(c, agentID, req, sess)
        return
    }

    // 非流式响应
    h.handleNonStream(c, agentID, req, sess)
}

func (h *ChatHandler) handleNonStream(c *gin.Context, agentID string, req models.ChatRequest, sess *session.Session) {
    // 调用 Agent
    result, err := h.agentService.Chat(c.Request.Context(), agentID, req.Message, sess)
    if err != nil {
        c.JSON(500, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "internal_error",
                Message: err.Error(),
            },
        })
        return
    }

    c.JSON(200, models.ChatResponse{
        ID:        uuid.New().String(),
        SessionID: sess.ID,
        AgentID:   agentID,
        Content:   result.Content,
        ToolCalls: result.ToolCalls,
        Usage:     result.Usage,
        CreatedAt: time.Now(),
    })
}

func (h *ChatHandler) handleStream(c *gin.Context, agentID string, req models.ChatRequest, sess *session.Session) {
    // 设置 SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    writer := sse.NewWriter(c.Writer)

    // 发送 connected 事件
    writer.WriteEvent("connected", gin.H{"session_id": sess.ID})

    // 创建流式回调
    callback := &StreamCallback{
        writer: writer,
    }

    // 调用 Agent 流式接口
    err := h.agentService.StreamChat(c.Request.Context(), agentID, req.Message, sess, callback)
    if err != nil {
        writer.WriteEvent("error", gin.H{
            "code":    "internal_error",
            "message": err.Error(),
        })
        return
    }
}
```

#### 2.3 SSE 支持 (Gin 版本)

创建 `internal/api/sse/writer.go`:

```go
package sse

import (
    "encoding/json"
    "fmt"
    "io"

    "github.com/gin-gonic/gin"
)

type Writer struct {
    w    io.Writer
    flusher http.Flusher
}

func NewWriter(w gin.ResponseWriter) *Writer {
    return &Writer{
        w:       w,
        flusher: w,
    }
}

func (s *Writer) WriteEvent(event string, data interface{}) error {
    fmt.Fprintf(s.w, "event: %s\n", event)
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }
    fmt.Fprintf(s.w, "data: %s\n\n", jsonData)
    s.flusher.Flush()
    return nil
}
```

创建 `internal/api/sse/event.go`:

```go
package sse

// 事件类型常量
const (
    EventConnected  = "connected"
    EventThinking   = "thinking"
    EventToolCall   = "tool_call"
    EventToolResult = "tool_result"
    EventContent    = "content"
    EventCompleted  = "completed"
    EventError      = "error"
)

type Event struct {
    Type string      `json:"event"`
    Data interface{} `json:"data"`
}
```

#### 2.4 Agent 集成

修改 `internal/agent/agent.go`，添加回调接口：

```go
type StreamCallback interface {
    OnThinking(content string)
    OnToolCall(id, tool, action string, params map[string]interface{})
    OnToolResult(id, tool, status, result string)
    OnContent(delta string)
    OnCompleted(responseID string, usage *Usage)
    OnError(code, message string)
}

// StreamChat 流式对话
func (a *Agent) StreamChat(ctx context.Context, message string, callback StreamCallback) error {
    // 调用 LLM Provider 的流式接口
    stream, err := a.provider.Stream(ctx, messages)
    if err != nil {
        callback.OnError("provider_error", err.Error())
        return err
    }

    for chunk := range stream {
        switch chunk.Type {
        case "content":
            callback.OnContent(chunk.Delta)
        case "tool_call":
            callback.OnToolCall(chunk.ID, chunk.Tool, chunk.Action, chunk.Params)
            // 执行工具
            result := a.executeTool(chunk.Tool, chunk.Action, chunk.Params)
            callback.OnToolResult(chunk.ID, chunk.Tool, "completed", result)
        }
    }

    callback.OnCompleted(uuid.New().String(), usage)
    return nil
}
```

**验收标准**:
- ✅ `POST /v1/agents/default/chat` 非流式响应正常
- ✅ `POST /v1/agents/default/chat` (stream=true) SSE 事件正确
- ✅ Session 历史自动管理
- ✅ 工具调用事件正确推送

---

### Phase 3: Session API (Day 5) ✅ 已完成

**目标**: 实现会话管理接口

#### 3.1 Session Handler (Gin 版本)

创建 `internal/api/handlers/session.go`:

```go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/lingguard/internal/api/models"
    "strconv"
)

type SessionHandler struct {
    sessionMgr *session.Manager
}

func NewSessionHandler(sessionMgr *session.Manager) *SessionHandler {
    return &SessionHandler{sessionMgr: sessionMgr}
}

func (h *SessionHandler) List(c *gin.Context) {
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
    offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
    agentID := c.Query("agent_id")

    sessions := h.sessionMgr.List(limit, offset, agentID)

    c.JSON(200, gin.H{
        "sessions": sessions,
        "total":    h.sessionMgr.Count(),
        "limit":    limit,
        "offset":   offset,
    })
}

func (h *SessionHandler) Get(c *gin.Context) {
    sessionID := c.Param("session_id")

    sess, err := h.sessionMgr.Get(sessionID)
    if err != nil {
        c.JSON(404, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "session_not_found",
                Message: err.Error(),
                Details: map[string]interface{}{"session_id": sessionID},
            },
        })
        return
    }

    c.JSON(200, sess)
}

func (h *SessionHandler) Delete(c *gin.Context) {
    sessionID := c.Param("session_id")

    if err := h.sessionMgr.Delete(sessionID); err != nil {
        c.JSON(404, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "session_not_found",
                Message: err.Error(),
            },
        })
        return
    }

    c.JSON(200, gin.H{
        "message": "session deleted",
        "id":      sessionID,
    })
}

func (h *SessionHandler) Clear(c *gin.Context) {
    sessionID := c.Param("session_id")

    if err := h.sessionMgr.ClearHistory(sessionID); err != nil {
        c.JSON(404, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "session_not_found",
                Message: err.Error(),
            },
        })
        return
    }

    c.JSON(200, gin.H{
        "message": "session cleared",
        "id":      sessionID,
    })
}
```

#### 3.2 Session Manager 扩展

修改 `pkg/session/manager.go`:

```go
type SessionInfo struct {
    ID           string    `json:"id"`
    Title        string    `json:"title"`
    AgentID      string    `json:"agent_id"`
    MessageCount int       `json:"message_count"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

func (m *Manager) List(limit, offset int, agentID string) []SessionInfo {
    // 实现列表查询
}

func (m *Manager) Get(id string) (*SessionDetail, error) {
    // 返回包含历史消息的详情
}

func (m *Manager) Delete(id string) error {
    // 删除会话及其存储
}

func (m *Manager) ClearHistory(id string) error {
    // 清空历史，保留会话
}
```

**验收标准**:
- ✅ `GET /v1/sessions` 返回会话列表
- ✅ `GET /v1/sessions/{id}` 返回会话详情
- ✅ `DELETE /v1/sessions/{id}` 删除成功
- ✅ `POST /v1/sessions/{id}/clear` 清空成功

---

### Phase 4: Task API (Day 6)

**目标**: 实现异步任务机制

#### 4.1 Task Manager

创建 `internal/api/task/manager.go`:

```go
package task

import (
    "sync"
    "time"

    "github.com/google/uuid"
)

type Task struct {
    ID          string    `json:"id"`
    Status      string    `json:"status"` // pending, running, completed, failed, cancelled
    Progress    int       `json:"progress"`
    ProgressMsg string    `json:"progress_message,omitempty"`
    Prompt      string    `json:"prompt"`
    Result      string    `json:"result,omitempty"`
    Error       string    `json:"error,omitempty"`
    AgentID     string    `json:"agent_id"`
    SessionID   string    `json:"session_id,omitempty"`
    CallbackURL string    `json:"callback_url,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`

    events      chan Event
    cancel      context.CancelFunc
}

type Event struct {
    Type string      `json:"type"`
    Data interface{} `json:"data"`
}

type Manager struct {
    tasks   map[string]*Task
    mu      sync.RWMutex
    agent   *agent.Service
}

func NewManager(agent *agent.Service) *Manager {
    return &Manager{
        tasks: make(map[string]*Task),
        agent: agent,
    }
}

func (m *Manager) Create(prompt string, opts ...Option) (*Task, error) {
    task := &Task{
        ID:        "task-" + uuid.New().String()[:8],
        Status:    "pending",
        Prompt:    prompt,
        AgentID:   "default",
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        events:    make(chan Event, 100),
    }

    // 应用选项
    for _, opt := range opts {
        opt(task)
    }

    m.mu.Lock()
    m.tasks[task.ID] = task
    m.mu.Unlock()

    // 异步执行
    ctx, cancel := context.WithCancel(context.Background())
    task.cancel = cancel
    go m.execute(ctx, task)

    return task, nil
}

func (m *Manager) execute(ctx context.Context, task *Task) {
    task.Status = "running"
    task.emit(Event{Type: "started", Data: gin.H{"task_id": task.ID}})

    // 创建流式回调
    callback := &TaskCallback{task: task}

    // 调用 Agent
    result, err := m.agent.Chat(ctx, task.AgentID, task.Prompt, nil)
    if err != nil {
        task.Status = "failed"
        task.Error = err.Error()
        task.emit(Event{Type: "failed", Data: gin.H{"error": err.Error()}})
        return
    }

    task.Status = "completed"
    task.Result = result.Content
    task.Progress = 100
    task.emit(Event{Type: "completed", Data: gin.H{"result": result.Content, "progress": 100}})

    // 回调通知
    if task.CallbackURL != "" {
        m.sendCallback(task)
    }
}

func (t *Task) emit(event Event) {
    select {
    case t.events <- event:
    default:
        // channel 满，丢弃
    }
    t.UpdatedAt = time.Now()
}

func (m *Manager) Get(id string) (*Task, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    task, ok := m.tasks[id]
    if !ok {
        return nil, fmt.Errorf("task not found")
    }
    return task, nil
}

func (m *Manager) Cancel(id string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    task, ok := m.tasks[id]
    if !ok {
        return fmt.Errorf("task not found")
    }

    if task.cancel != nil {
        task.cancel()
    }
    task.Status = "cancelled"
    task.emit(Event{Type: "cancelled", Data: gin.H{"message": "task cancelled by user"}})

    return nil
}

func (m *Manager) Subscribe(id string) (<-chan Event, error) {
    task, err := m.Get(id)
    if err != nil {
        return nil, err
    }
    return task.events, nil
}
```

#### 4.2 Task Handler (Gin 版本)

创建 `internal/api/handlers/task.go`:

```go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/lingguard/internal/api/models"
    "github.com/lingguard/internal/api/sse"
    "github.com/lingguard/internal/api/task"
)

type TaskHandler struct {
    taskMgr *task.Manager
}

func NewTaskHandler(taskMgr *task.Manager) *TaskHandler {
    return &TaskHandler{taskMgr: taskMgr}
}

type CreateTaskRequest struct {
    Prompt      string   `json:"prompt" binding:"required"`
    SessionID   string   `json:"session_id,omitempty"`
    AgentID     string   `json:"agent_id,omitempty"`
    Tools       []string `json:"tools,omitempty"`
    CallbackURL string   `json:"callback_url,omitempty"`
}

func (h *TaskHandler) Create(c *gin.Context) {
    var req CreateTaskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "invalid_request",
                Message: err.Error(),
            },
        })
        return
    }

    task, err := h.taskMgr.Create(req.Prompt,
        task.WithAgentID(req.AgentID),
        task.WithSessionID(req.SessionID),
        task.WithCallback(req.CallbackURL),
    )
    if err != nil {
        c.JSON(500, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "internal_error",
                Message: err.Error(),
            },
        })
        return
    }

    c.JSON(201, task)
}

func (h *TaskHandler) Get(c *gin.Context) {
    taskID := c.Param("task_id")

    task, err := h.taskMgr.Get(taskID)
    if err != nil {
        c.JSON(404, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "task_not_found",
                Message: err.Error(),
            },
        })
        return
    }

    c.JSON(200, task)
}

func (h *TaskHandler) Cancel(c *gin.Context) {
    taskID := c.Param("task_id")

    if err := h.taskMgr.Cancel(taskID); err != nil {
        c.JSON(404, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "task_not_found",
                Message: err.Error(),
            },
        })
        return
    }

    task, _ := h.taskMgr.Get(taskID)
    c.JSON(200, gin.H{
        "id":      taskID,
        "status":  "cancelled",
        "message": "task cancelled by user",
    })
}

func (h *TaskHandler) Events(c *gin.Context) {
    taskID := c.Param("task_id")

    // 设置 SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    events, err := h.taskMgr.Subscribe(taskID)
    if err != nil {
        c.JSON(404, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "task_not_found",
                Message: err.Error(),
            },
        })
        return
    }

    writer := sse.NewWriter(c.Writer)

    for event := range events {
        writer.WriteEvent(event.Type, event.Data)

        // 终止事件
        if event.Type == "completed" || event.Type == "failed" || event.Type == "cancelled" {
            break
        }
    }
}
```

**验收标准**:
- `POST /v1/tasks` 创建任务
- `GET /v1/tasks/{id}` 查询状态
- `POST /v1/tasks/{id}/cancel` 取消任务
- `GET /v1/tasks/{id}/events` SSE 事件流

---

### Phase 5: Tool & Agent API (Day 7)

**目标**: 实现工具查询和智能体管理接口

#### 5.1 Tool Handler (Gin 版本)

创建 `internal/api/handlers/tool.go`:

```go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/lingguard/internal/api/models"
    "time"
)

type ToolHandler struct {
    registry *tools.Registry
}

func NewToolHandler(registry *tools.Registry) *ToolHandler {
    return &ToolHandler{registry: registry}
}

func (h *ToolHandler) List(c *gin.Context) {
    tools := h.registry.List()
    c.JSON(200, gin.H{"tools": tools})
}

type ExecuteRequest struct {
    Action string                 `json:"action" binding:"required"`
    Params map[string]interface{} `json:"params,omitempty"`
}

func (h *ToolHandler) Execute(c *gin.Context) {
    toolName := c.Param("tool_name")

    var req ExecuteRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "invalid_request",
                Message: err.Error(),
            },
        })
        return
    }

    start := time.Now()
    result, err := h.registry.Execute(toolName, req.Action, req.Params)
    duration := time.Since(start).Milliseconds()

    if err != nil {
        c.JSON(500, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "tool_error",
                Message: err.Error(),
            },
        })
        return
    }

    c.JSON(200, gin.H{
        "tool":        toolName,
        "action":      req.Action,
        "result":      result,
        "success":     true,
        "duration_ms": duration,
    })
}
```

#### 5.2 Agent Handler (Gin 版本)

创建 `internal/api/handlers/agent.go`:

```go
package handlers

import (
    "github.com/gin-gonic/gin"
    "github.com/lingguard/internal/api/models"
)

type AgentHandler struct {
    registry *agent.Registry
}

func NewAgentHandler(registry *agent.Registry) *AgentHandler {
    return &AgentHandler{registry: registry}
}

func (h *AgentHandler) List(c *gin.Context) {
    agents := h.registry.List()
    c.JSON(200, gin.H{"agents": agents})
}

func (h *AgentHandler) Get(c *gin.Context) {
    agentID := c.Param("agent_id")

    agent, err := h.registry.Get(agentID)
    if err != nil {
        c.JSON(404, models.ErrorResponse{
            Error: models.ErrorDetail{
                Code:    "agent_not_found",
                Message: err.Error(),
            },
        })
        return
    }

    c.JSON(200, agent)
}
```

**验收标准**:
- `GET /v1/tools` 返回工具列表
- `POST /v1/tools/{name}/execute` 直接执行工具
- `GET /v1/agents` 返回智能体列表
- `GET /v1/agents/{id}` 返回智能体详情

---

### Phase 6: 测试 & 文档 (Day 8)

#### 6.1 单元测试

```
internal/api/
├── handlers/
│   ├── chat_test.go
│   ├── session_test.go
│   ├── task_test.go
│   └── tool_test.go
└── middleware/
    └── auth_test.go
```

创建 `internal/api/handlers/chat_test.go`:

```go
package handlers

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestChatHandler_NonStream(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // Setup
    router := gin.New()
    handler := NewChatHandler(mockAgentService, mockSessionMgr)
    router.POST("/v1/agents/:agent_id/chat", handler.Handle)

    // Request
    body := `{"message": "hello", "stream": false}`
    req := httptest.NewRequest("POST", "/v1/agents/default/chat", bytes.NewBufferString(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    // Execute
    router.ServeHTTP(w, req)

    // Assert
    assert.Equal(t, 200, w.Code)

    var resp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.Contains(t, resp, "content")
    assert.Contains(t, resp, "session_id")
}
```

#### 6.2 集成测试

创建 `tests/api_integration_test.go`:

```go
package tests

import (
    "testing"
    "time"

    "github.com/lingguard/internal/api"
)

func TestChatAPI(t *testing.T) {
    // 启动测试服务器
    server := api.NewServer(testConfig)
    go server.Start(context.Background())
    defer server.Stop(context.Background())

    time.Sleep(100 * time.Millisecond) // 等待启动

    client := NewTestClient("http://localhost:18989", "test-token")

    // 测试非流式对话
    resp, err := client.Chat("hello", "", false)
    if err != nil {
        t.Fatalf("Chat failed: %v", err)
    }
    if resp.Content == "" {
        t.Error("Expected non-empty content")
    }

    // 测试流式对话
    events, err := client.StreamChat("hello again", resp.SessionID)
    if err != nil {
        t.Fatalf("StreamChat failed: %v", err)
    }

    eventCount := 0
    for range events {
        eventCount++
    }
    if eventCount == 0 {
        t.Error("Expected at least one event")
    }
}
```

#### 6.3 API 文档更新

- 添加 OpenAPI/Swagger 规范
- 添加 Postman Collection

---

### 依赖关系

```
Phase 1 (Gin 框架 + TaskBoard 迁移)
    │
    ├── Phase 2 (Chat API) ─────┬── Phase 3 (Session API)
    │                           │
    │                           └── Phase 4 (Task API)
    │
    └── Phase 5 (Tool/Agent API)
                │
                └── Phase 6 (测试 & 文档)
```

### 关键文件路径

| 文件 | 说明 |
|------|------|
| `go.mod` | 添加 gin 依赖 |
| `internal/config/config.go` | 添加 APIConfig |
| `internal/api/server.go` | Gin 统一服务器 |
| `internal/api/router.go` | 路由注册 |
| `internal/api/middleware/*.go` | 中间件 |
| `internal/api/handlers/chat.go` | Chat API |
| `internal/api/handlers/session.go` | Session API |
| `internal/api/handlers/task.go` | Task API |
| `internal/api/handlers/taskboard.go` | TaskBoard 迁移 |
| `internal/api/handlers/websocket.go` | WebSocket 迁移 |
| `internal/api/sse/writer.go` | SSE 支持 |
| `internal/api/task/manager.go` | 任务管理器 |
| `cmd/lingguard/main.go` | CLI 集成 |
| `internal/taskboard/server.go` | 删除（迁移到 api/） |

### 风险点

1. **Session 并发**: 同一会话并发请求需要加锁
2. **SSE 连接管理**: 需要处理客户端断开、超时
3. **任务持久化**: 重启后任务状态恢复（可选）
4. **内存占用**: 大量 Session 的内存管理

---

## Curl 测试示例

### 基础配置

```bash
# 服务地址
export API_URL="http://127.0.0.1:18989"

# 认证 Token（从 config.json 获取）
export TOKEN="1234567890"
```

### 1. 健康检查

```bash
# 无需认证
curl $API_URL/v1/health
# 响应: {"status":"ok"}
```

### 2. 对话 API

#### 非流式对话

```bash
curl -X POST $API_URL/v1/agents/default/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "message": "你好，介绍一下你自己",
    "session_id": "test-session-1"
  }'
```

**响应示例:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "session_id": "test-session-1",
  "agent_id": "default",
  "content": "你好！我是灵侍，一个乐于助人的 AI 助手...",
  "created_at": "2026-03-06T10:30:00Z"
}
```

#### 流式对话

```bash
curl -X POST $API_URL/v1/agents/default/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "message": "写一首简短的诗",
    "session_id": "test-session-2",
    "stream": true
  }'
```

**响应示例 (SSE):**
```
event: connected
data: {"session_id":"test-session-2"}

event: content
data: {"delta":"春风"}

event: content
data: {"delta":"吹绿"}

event: tool_call
data: {"id":"abc123","tool":"web_search","status":"running"}

event: tool_result
data: {"id":"abc123","tool":"web_search","status":"completed","result":"..."}

event: completed
data: {"id":"xxx","session_id":"test-session-2","agent_id":"default"}
```

#### 带媒体的消息

```bash
curl -X POST $API_URL/v1/agents/default/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "message": "描述这张图片",
    "session_id": "test-session-3",
    "media": ["https://example.com/image.png"]
  }'
```

#### 清空历史后对话

```bash
curl -X POST $API_URL/v1/agents/default/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "message": "新对话开始",
    "session_id": "test-session-1",
    "clear_history": true
  }'
```

### 3. Session API

#### 列出所有会话

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "$API_URL/v1/sessions?limit=20&offset=0"
```

**响应示例:**
```json
{
  "sessions": [
    {
      "id": "test-session-1",
      "title": "你好，介绍一下你自己",
      "agent_id": "default",
      "message_count": 2,
      "created_at": "2026-03-06T10:00:00Z",
      "updated_at": "2026-03-06T10:30:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

#### 获取会话详情

```bash
curl -H "Authorization: Bearer $TOKEN" \
  $API_URL/v1/sessions/test-session-1
```

**响应示例:**
```json
{
  "id": "test-session-1",
  "title": "你好，介绍一下你自己",
  "agent_id": "default",
  "messages": [
    {
      "id": "abc123",
      "role": "user",
      "content": "你好，介绍一下你自己",
      "created_at": "2026-03-06T10:00:00Z"
    },
    {
      "id": "def456",
      "role": "assistant",
      "content": "你好！我是灵侍...",
      "created_at": "2026-03-06T10:00:05Z"
    }
  ],
  "message_count": 2,
  "created_at": "2026-03-06T10:00:00Z",
  "updated_at": "2026-03-06T10:00:05Z"
}
```

#### 清空会话历史

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  $API_URL/v1/sessions/test-session-1/clear
```

**响应示例:**
```json
{
  "message": "session cleared",
  "id": "test-session-1"
}
```

#### 删除会话

```bash
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  $API_URL/v1/sessions/test-session-1
```

**响应示例:**
```json
{
  "message": "session deleted",
  "id": "test-session-1"
}
```

### 4. 错误响应测试

#### 无 Token 访问（返回 401）

```bash
curl $API_URL/v1/agents/default/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "test"}'
```

**响应:**
```json
{
  "error": {
    "code": "unauthorized",
    "message": "missing or invalid authorization token"
  }
}
```

#### 无效请求（返回 400）

```bash
curl -X POST $API_URL/v1/agents/default/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
```

**响应:**
```json
{
  "error": {
    "code": "invalid_request",
    "message": "Key: 'ChatRequest.Message' Error:Field validation for 'Message' failed on the 'required' tag"
  }
}
```

#### 会话不存在（返回 404）

```bash
curl -H "Authorization: Bearer $TOKEN" \
  $API_URL/v1/sessions/non-existent-session
```

**响应:**
```json
{
  "error": {
    "code": "session_not_found",
    "message": "session not found: non-existent-session"
  }
}
```

---

## 变更日志

### v1.0.0 (2026-03-06)

- 初始版本
- 支持 Chat、Session、Task、Tool API
- 支持 SSE 流式响应
- 支持多智能体切换
