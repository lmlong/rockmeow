# LingGuard Agent API 文档

> 版本: 1.0.0
> 基础 URL: `http://localhost:18989`

## 概述

LingGuard Agent API 是一个智能体服务接口，与 OpenAI Chat API 不同，它是**有状态**的，自动管理会话历史、工具执行和记忆系统。

| 特性 | OpenAI Chat API | LingGuard Agent API |
|------|-----------------|---------------------|
| 状态管理 | 无状态（客户端管理历史） | 有状态（Session 自动管理） |
| 工具执行 | 返回工具调用，客户端执行 | 自动执行，返回最终结果 |
| 记忆系统 | 无 | 内置长期记忆 + 向量检索 |

---

## 认证

所有 API 请求需要在 Header 中携带 Token：

```http
Authorization: Bearer <your-token>
```

---

## API 端点

### POST /v1/agent/chat

与智能体进行对话。支持流式和非流式响应。

**请求体**

```json
{
  "message": "帮我分析今天的日程安排",
  "media": ["https://example.com/image.png"],
  "session_id": "user-123-device-456",
  "stream": true,
  "clear_history": false
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `message` | string | 是 | 用户消息内容 |
| `media` | []string | 否 | 多媒体 URL 列表（图片、文件等） |
| `session_id` | string | 否 | 会话 ID，不传则新建 |
| `stream` | bool | 否 | 是否流式响应，默认 `false` |
| `clear_history` | bool | 否 | 是否清空历史后对话，默认 `false` |

**非流式响应**

```json
{
  "id": "resp-a1b2c3d4",
  "session_id": "user-123-device-456",
  "agent_id": "default",
  "content": "根据您的日历，今天有以下安排...",
  "created_at": "2026-03-06T10:30:00Z"
}
```

**流式响应（SSE）**

```
event: connected
data: {"session_id": "user-123-device-456"}

event: content
data: {"delta": "根据您的日历，"}

event: tool_call
data: {"tool": "calendar", "status": "running"}

event: tool_result
data: {"tool": "calendar", "status": "completed", "result": "找到 3 个事件"}

event: completed
data: {"id": "resp-a1b2c3d4"}

event: error
data: {"code": "tool_error", "message": "工具执行失败"}
```

---

## 通信方式对比

LingGuard 提供两种流式通信方式：**HTTP + SSE** 和 **WebSocket**。

### 架构对比

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          API Server (Gin)                               │
├─────────────────────────────────────────────────────────────────────────┤
│   ┌─────────────────────────────┐    ┌─────────────────────────────┐   │
│   │      HTTP + SSE             │    │       WebSocket             │   │
│   │   POST /v1/agent/chat       │    │      GET /ws/chat           │   │
│   │   (stream: true)            │    │      ?session=xxx           │   │
│   └─────────────────────────────┘    └─────────────────────────────┘   │
│                 │                                   │                   │
│                 └───────────────┬───────────────────┘                   │
│                                 ▼                                       │
│                    ┌────────────────────────┐                          │
│                    │      Agent Core        │                          │
│                    │  ProcessMessageStream  │                          │
│                    └────────────────────────┘                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 方式一：HTTP + SSE

**端点**: `POST /v1/agent/chat` (设置 `stream: true`)

**特点**: 基于 HTTP 的单向服务端推送

**请求示例**:
```bash
curl -X POST http://localhost:18989/v1/agent/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "你好", "session_id": "user-123", "stream": true}'
```

**前端使用**:
```javascript
const response = await fetch('/v1/agent/chat', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer ' + token,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    message: '你好',
    session_id: 'user-123',
    stream: true
  })
});

const reader = response.body.getReader();
const decoder = new TextDecoder();

while (true) {
  const { done, value } = await reader.read();
  if (done) break;

  const text = decoder.decode(value);
  // 解析 SSE 格式: "event: xxx\ndata: {...}\n\n"
  const lines = text.split('\n');
  for (const line of lines) {
    if (line.startsWith('data: ')) {
      const data = JSON.parse(line.slice(6));
      console.log(data);
    }
  }
}
```

**SSE 事件类型**:

| Event | Data | 说明 |
|-------|------|------|
| `connected` | `{session_id}` | 连接建立 |
| `content` | `{delta}` | 文本增量 |
| `tool_call` | `{id, tool, status}` | 工具调用开始 |
| `tool_result` | `{id, tool, status, result}` | 工具调用结果 |
| `completed` | `{id, session_id, agent_id}` | 完成 |
| `error` | `{code, message}` | 错误 |

---

### 方式二：WebSocket

**端点**: `GET /ws/chat?session={session_id}`

**特点**: 全双工双向通信，保持长连接

**连接示例**:
```javascript
const ws = new WebSocket('ws://localhost:18989/ws/chat?session=user-123');

ws.onopen = () => {
  console.log('WebSocket connected');
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data);
};

// 发送消息
ws.send(JSON.stringify({
  type: 'chat',
  content: '你好'
}));
```

**消息格式**:

*客户端 → 服务端*:
```json
{"type": "chat", "content": "你好"}
{"type": "ping"}
{"type": "switch", "content": "new-session-id"}
```

*服务端 → 客户端*:
```json
{"type": "connected", "sessionId": "webchat-xxx", "done": true}
{"type": "stream", "content": "增量文本", "sessionId": "webchat-xxx", "done": false}
{"type": "stream_end", "content": "完整响应", "sessionId": "webchat-xxx", "done": true}
{"type": "chat", "content": "完整响应", "sessionId": "webchat-xxx", "done": true}
{"type": "error", "content": "错误信息", "sessionId": "webchat-xxx", "done": true}
{"type": "pong"}
```

**消息类型说明**:

| Type | 方向 | 说明 |
|------|------|------|
| `chat` | 双向 | 完整消息 |
| `stream` | 服务端→客户端 | 流式文本增量 |
| `stream_end` | 服务端→客户端 | 流式结束 |
| `connected` | 服务端→客户端 | 连接确认（含实际 sessionId） |
| `switch` | 客户端→服务端 | 切换会话 |
| `switched` | 服务端→客户端 | 会话切换确认 |
| `ping/pong` | 双向 | 心跳 |

---

### 优缺点对比

| 特性 | HTTP + SSE | WebSocket |
|------|-----------|-----------|
| **通信方向** | 单向（服务端推送） | 双向 |
| **协议** | HTTP/1.1 | WS/WSS |
| **连接状态** | 请求级，每次请求建立 | 连接级，保持长连接 |
| **断线重连** | 需手动处理 | 需手动处理 |
| **浏览器支持** | EventSource API | WebSocket API |
| **代理/防火墙** | 兼容性好（纯 HTTP） | 可能被拦截 |
| **资源消耗** | 每次请求创建连接 | 保持连接，开销低 |
| **适用场景** | API 集成、一次性请求 | 实时聊天、长连接 |
| **认证** | Header 携带 Token | URL 参数或首次消息 |

**HTTP + SSE 优点**:
- 简单易用，基于标准 HTTP
- 天然支持断线重连（浏览器 EventSource 自动重连）
- 穿透代理/防火墙能力强
- 适合偶尔的对话场景

**HTTP + SSE 缺点**:
- 单向通信，客户端无法在连接中发送额外数据
- 每次请求都需要重新建立连接
- 不适合高频交互场景

**WebSocket 优点**:
- 全双工通信，实时性好
- 保持长连接，无需重复握手
- 适合高频、持续的对话场景
- 服务端可主动推送消息

**WebSocket 缺点**:
- 需要维护连接状态
- 某些网络环境可能被拦截
- 断线重连需要自行实现

---

### 移动端支持 (iOS / Android)

| 平台 | HTTP + SSE | WebSocket |
|------|-----------|-----------|
| **iOS** | 无原生支持，需第三方库 | iOS 13+ 原生支持 (URLSessionWebSocketTask) |
| **Android** | 无原生支持，需第三方库 | 无原生支持，需 OkHttp 等库 |

**iOS 示例 (WebSocket)**:
```swift
// iOS 13+ 原生 WebSocket
let url = URL(string: "ws://localhost:18989/ws/chat?session=user-123")!
let task = URLSession.shared.webSocketTask(with: url)
task.resume()

// 发送消息
let message = URLSessionWebSocketTask.Message.string("""
{"type": "chat", "content": "你好"}
""")
task.send(message) { error in
  if let error = error { print("Send error: \(error)") }
}

// 接收消息
task.receive { result in
  switch result {
  case .success(let message):
    // 处理消息
  case .failure(let error):
    print("Receive error: \(error)")
  }
}
```

**Android 示例 (WebSocket)**:
```kotlin
// 使用 OkHttp
val client = OkHttpClient()
val request = Request.Builder()
  .url("ws://localhost:18989/ws/chat?session=user-123")
  .build()

val webSocket = client.newWebSocket(request, object : WebSocketListener() {
  override fun onOpen(webSocket: WebSocket, response: Response) {
    // 连接成功
  }

  override fun onMessage(webSocket: WebSocket, text: String) {
    // 接收消息
    val data = JSONObject(text)
  }

  override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
    // 连接失败
  }
})

// 发送消息
webSocket.send("""{"type": "chat", "content": "你好"}""")
```

**移动端推荐**:
- **聊天 APP**: 推荐 WebSocket（实时性、保持连接）
- **工具类 APP**: 推荐 HTTP + SSE（简单、按需请求）
- **后台限制严格**: 推荐 HTTP + SSE（更省电）

---

### 选择建议

| 场景 | 推荐方式 | 理由 |
|------|---------|------|
| Web 聊天界面 | WebSocket | 实时双向、用户体验好 |
| API 集成 | HTTP + SSE | 简单、标准 HTTP |
| 移动端 APP | WebSocket | 保持连接、低延迟 |
| 一次性请求 | HTTP (非流式) | 最简单 |
| 长时间任务 | HTTP + SSE / Task API | 异步处理 |

---

### 会话 API

#### GET /v1/sessions

获取会话列表。

**查询参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | int | 返回数量，默认 20 |
| `offset` | int | 偏移量，默认 0 |

**响应**

```json
{
  "sessions": [
    {
      "id": "user-123-device-456",
      "title": "日程安排分析",
      "message_count": 12,
      "created_at": "2026-03-05T14:00:00Z",
      "updated_at": "2026-03-06T10:30:00Z"
    }
  ],
  "total": 5
}
```

---

#### GET /v1/sessions/{session_id}

获取会话详情，包含历史消息。

---

#### DELETE /v1/sessions/{session_id}

删除会话及其历史记录。

---

#### POST /v1/sessions/{session_id}/clear

清空会话历史，保留会话本身。

---

### 任务 API

用于长时间运行的异步任务。

**特性**：
- 最大并发：3 个任务同时执行
- 排队机制：超过并发限制的任务进入 FIFO 队列等待
- 自动会话：不传 `session_id` 时自动创建独立会话
- 回调通知：任务完成后 POST 到 `callback_url`

#### POST /v1/tasks

创建异步任务。

**请求体**

```json
{
  "message": "帮我重构整个项目的代码结构",
  "session_id": "session-abc123",
  "callback_url": "https://your-server.com/callback"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `message` | string | 是 | 任务描述 |
| `session_id` | string | 否 | 会话 ID，不传则自动创建 |
| `media` | []string | 否 | 多媒体 URL 列表 |
| `callback_url` | string | 否 | 任务完成回调 URL |

**响应**

```json
{
  "id": "task-abc123",
  "status": "pending",
  "session_id": "session-abc123",
  "message": "帮我重构整个项目的代码结构...",
  "created_at": "2026-03-06T10:00:00Z"
}
```

---

#### GET /v1/tasks/{task_id}

查询任务状态。

**状态值**

| 状态 | 说明 |
|------|------|
| `pending` | 等待执行（排队中） |
| `running` | 执行中 |
| `completed` | 已完成 |
| `failed` | 执行失败 |
| `cancelled` | 已取消 |

---

#### DELETE /v1/tasks/{task_id}

删除/取消任务。如果任务正在运行，会先取消再删除。

---

#### GET /v1/tasks/{task_id}/events

获取任务执行事件的 SSE 流。

---

#### GET /v1/tasks

列出任务。

---

## 实现状态

| API | 端点 | 状态 |
|-----|------|------|
| Chat | `POST /v1/agent/chat` | ✅ |
| Session | `GET /v1/sessions` | ✅ |
| Session | `GET /v1/sessions/{id}` | ✅ |
| Session | `DELETE /v1/sessions/{id}` | ✅ |
| Session | `POST /v1/sessions/{id}/clear` | ✅ |
| Task | `POST /v1/tasks` | ✅ |
| Task | `GET /v1/tasks/{id}` | ✅ |
| Task | `DELETE /v1/tasks/{id}` | ✅ |
| Task | `GET /v1/tasks/{id}/events` | ✅ |
| Task | `GET /v1/tasks` | ✅ |

---

## 错误响应

```json
{
  "error": {
    "code": "session_not_found",
    "message": "会话不存在"
  }
}
```

| HTTP 状态码 | 错误码 | 说明 |
|------------|--------|------|
| 400 | `invalid_request` | 请求参数错误 |
| 401 | `unauthorized` | Token 无效或过期 |
| 404 | `session_not_found` | 会话不存在 |
| 404 | `task_not_found` | 任务不存在 |
| 409 | `session_busy` | 会话正在处理其他请求 |
| 429 | `rate_limit_exceeded` | 请求频率超限 |
| 500 | `internal_error` | 服务器内部错误 |
| 503 | `provider_error` | LLM 服务不可用 |

---

## 最佳实践

### Session ID 设计

推荐格式：`{source}-{user_id}-{device_id}`

```javascript
const sessionId = `app-${userId}-${deviceId}`;
// 结果: app-user123-device456
```

### 错误重试

```javascript
async function chatWithRetry(message, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await chat(message);
    } catch (error) {
      if (!error.isRetryable) throw error;
      await sleep(Math.pow(2, i) * 1000);
    }
  }
}
```

---

## 路由总览

```
/v1/*                              # Agent API (对外，需认证)
├── GET  /health                   # 健康检查
├── POST /agent/chat               # 对话
├── GET  /sessions                 # 会话列表
├── GET  /sessions/:id             # 会话详情
├── DELETE /sessions/:id           # 删除会话
├── POST /sessions/:id/clear       # 清空历史
├── POST /tasks                    # 创建任务
├── GET  /tasks                    # 任务列表
├── GET  /tasks/:id                # 任务状态
├── DELETE /tasks/:id              # 删除任务
└── GET  /tasks/:id/events         # SSE 事件流

/_internal/*                       # 内部 WebUI API (仅供本地访问)
├── /crons/*                       # 定时任务管理
│   ├── GET    /                   # 列出定时任务
│   ├── DELETE /:id                # 删除定时任务
│   ├── GET    /stats              # 统计信息
│   └── GET    /events             # SSE 事件流
├── /traces/*                      # 追踪管理
│   ├── GET    /                   # 列出追踪
│   ├── GET    /stats              # 统计信息
│   ├── GET    /:id                # 追踪详情
│   ├── GET    /:id/spans          # 追踪 Spans
│   ├── DELETE /:id                # 删除追踪
│   ├── DELETE /cleanup            # 清理旧追踪
│   ├── GET    /spans/:id          # Span 详情
│   └── GET    /events             # SSE 事件流
└── /webchat/*                     # WebChat 会话管理
    ├── GET    /sessions           # 会话列表
    ├── POST   /sessions           # 创建会话
    ├── GET    /session            # 获取会话
    └── DELETE /session            # 删除会话

/ws/chat                           # WebSocket
```

---

## 快速测试

```bash
# 配置
export API_URL="http://127.0.0.1:18989"
export TOKEN="your-token"

# 对话
curl -X POST $API_URL/v1/agent/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "你好"}'

# 流式对话
curl -X POST $API_URL/v1/agent/chat \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "你好", "stream": true}'

# 创建任务
curl -X POST $API_URL/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"message": "分析代码"}'

# 查询任务
curl $API_URL/v1/tasks/task-abc123 \
  -H "Authorization: Bearer $TOKEN"
```

---

## 配置说明

### 服务器配置

```json
{
  "server": {
    "enabled": true,
    "host": "127.0.0.1",
    "port": 18989,
    "cors": {
      "allowedOrigins": ["*"]
    },
    "api": {
      "enabled": true,
      "auth": {
        "type": "token",
        "tokens": ["your-secret-token"]
      },
      "rateLimit": {
        "enabled": false,
        "requestsPer": 60,
        "burst": 10
      }
    },
    "webui": {
      "taskboard": {
        "dbPath": "~/.lingguard/webui/taskboard.db",
        "trackUserRequests": true,
        "syncSubagent": true,
        "syncCron": true
      },
      "trace": {
        "enabled": true,
        "dbPath": "~/.lingguard/webui/trace.db"
      },
      "webchat": {
        "maxConnections": 100,
        "maxConnectionsPerIP": 5,
        "readLimitKB": 512,
        "writeTimeoutSec": 10,
        "readTimeoutSec": 60,
        "heartbeatSec": 30
      }
    }
  }
}
```

### 配置字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `server.enabled` | bool | 是否启用服务器 |
| `server.host` | string | 监听地址，默认 127.0.0.1 |
| `server.port` | int | 监听端口，默认 8080 |
| `server.cors` | object | CORS 配置 |
| `server.api` | object | Agent API 配置 (/v1/*) |
| `server.webui` | object | 内部 WebUI 配置 (/_internal/*) |

### API 配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `api.enabled` | bool | 是否启用 Agent API |
| `api.auth.type` | string | 认证类型：token / none |
| `api.auth.tokens` | []string | 有效的 Token 列表 |
| `api.rateLimit.enabled` | bool | 是否启用限流 |
| `api.rateLimit.requestsPer` | int | 每分钟请求数限制 |
| `api.rateLimit.burst` | int | 突发容量 |

### WebUI 配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `webui.taskboard.dbPath` | string | 任务看板数据库路径 |
| `webui.taskboard.trackUserRequests` | bool | 追踪用户请求 |
| `webui.trace.enabled` | bool | 是否启用 LLM 追踪 |
| `webui.trace.dbPath` | string | 追踪数据库路径 |
| `webui.webchat.maxConnections` | int | 最大 WebSocket 连接数 |
| `webui.webchat.maxConnectionsPerIP` | int | 每 IP 最大连接数 |
