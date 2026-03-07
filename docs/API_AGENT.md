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
/v1/*                              # Agent API
├── POST /agent/chat              # 对话
├── GET  /sessions                # 会话列表
├── GET  /sessions/:id            # 会话详情
├── DELETE /sessions/:id          # 删除会话
├── POST /sessions/:id/clear      # 清空历史
├── POST /tasks                   # 创建任务
├── GET  /tasks                   # 任务列表
├── GET  /tasks/:id               # 任务状态
├── DELETE /tasks/:id             # 删除任务
└── GET  /tasks/:id/events        # SSE 事件流

/api/*                             # 内部 API
├── /crons/*                       # 定时任务
├── /webchat/*                     # WebChat
└── /traces/*                      # Trace

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
