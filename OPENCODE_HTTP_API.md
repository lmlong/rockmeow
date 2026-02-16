# OpenCode HTTP API 完整文档

> 基于 OpenAPI 3.1 规范的 HTTP API 接口文档

## 概述

OpenCode 采用客户端/服务器架构，提供 HTTP API 供外部集成使用。服务器默认监听 `http://127.0.0.1:4096`，可通过 `opencode serve` 命令启动独立服务器。

### 基础信息

| 项目 | 值 |
|------|-----|
| 默认地址 | `http://127.0.0.1:4096` |
| 协议 | HTTP RESTful |
| 认证方式 | HTTP Basic Auth (可选) |
| 实时事件 | Server-Sent Events (SSE) |
| OpenAPI 文档 | `http://localhost:4096/doc` |

### 启动服务器

```bash
# 基本启动
opencode serve

# 指定端口和主机
opencode serve --port 4096 --hostname "0.0.0.0"

# 启用 CORS
opencode serve --cors http://localhost:5173 --cors https://app.example.com

# 启用 mDNS 发现
opencode serve --mdns

# 设置认证
OPENCODE_SERVER_PASSWORD=your-password opencode serve
OPENCODE_SERVER_USERNAME=admin opencode serve
```

---

## 通用类型定义

### MessagePart (消息部件)

```typescript
type MessagePart = 
  | { type: "text"; text: string }
  | { type: "tool_result"; tool: string; result: string; toolCallID?: string }
  | { type: "tool_call"; tool: string; args: Record<string, unknown>; id?: string }
  | { type: "image"; data: string; mimeType?: string }
  | { type: "resource"; path: string }
  | { type: "reference"; path: string; startLine?: number; endLine?: number }
```

### Session (会话)

```typescript
interface Session {
  id: string;
  parentID?: string;
  title?: string;
  createdAt: string;
  updatedAt: string;
  status: "idle" | "busy" | "error";
  error?: string;
  model?: string;
  provider?: string;
  agent?: string;
  shared?: boolean;
  sharedURL?: string;
}
```

### Message (消息)

```typescript
interface Message {
  id: string;
  sessionID: string;
  role: "user" | "assistant" | "system";
  createdAt: string;
  updatedAt: string;
  parts: MessagePart[];
}
```

### Project (项目)

```typescript
interface Project {
  id: string;
  directory: string;
  name: string;
  hasAgents: boolean;
}
```

---

## API 端点总览

### 全局 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/global/health` | 获取服务器健康状态和版本 |
| GET | `/global/event` | 获取全局事件流 (SSE) |

### 项目 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/project` | 获取所有项目列表 |
| GET | `/project/current` | 获取当前项目 |

### 路径与 VCS API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/path` | 获取当前工作目录 |
| GET | `/vcs` | 获取版本控制信息 |

### 实例 API

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/instance/dispose` | 释放当前实例 |

### 配置 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/config` | 获取配置信息 |
| PATCH | `/config` | 更新配置 |
| GET | `/config/providers` | 获取提供商列表和默认模型 |

### 提供商 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/provider` | 获取所有提供商 |
| GET | `/provider/auth` | 获取提供商认证方式 |
| POST | `/provider/{id}/oauth/authorize` | OAuth 授权 |
| POST | `/provider/{id}/oauth/callback` | OAuth 回调处理 |

### 会话 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/session` | 获取所有会话 |
| POST | `/session` | 创建新会话 |
| GET | `/session/status` | 获取所有会话状态 |
| GET | `/session/{id}` | 获取会话详情 |
| DELETE | `/session/{id}` | 删除会话 |
| PATCH | `/session/{id}` | 更新会话属性 |
| GET | `/session/{id}/children` | 获取子会话 |
| GET | `/session/{id}/todo` | 获取会话待办事项 |
| POST | `/session/{id}/init` | 分析项目并创建 AGENTS.md |
| POST | `/session/{id}/fork` | 派生会话 |
| POST | `/session/{id}/abort` | 中止运行中的会话 |
| POST | `/session/{id}/share` | 分享会话 |
| DELETE | `/session/{id}/share` | 取消分享会话 |
| GET | `/session/{id}/diff` | 获取会话变更差异 |
| POST | `/session/{id}/summarize` | 摘要会话 |
| POST | `/session/{id}/revert` | 回滚消息 |
| POST | `/session/{id}/unrevert` | 恢复已回滚的消息 |
| POST | `/session/{id}/permissions/{permissionID}` | 响应权限请求 |

### 消息 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/session/{id}/message` | 获取会话消息列表 |
| POST | `/session/{id}/message` | 发送消息并等待响应 |
| GET | `/session/{id}/message/{messageID}` | 获取消息详情 |
| POST | `/session/{id}/prompt_async` | 异步发送消息 |
| POST | `/session/{id}/command` | 执行命令 |
| POST | `/session/{id}/shell` | 执行 Shell 命令 |

### 命令 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/command` | 获取所有可用命令 |

### 文件 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/find` | 搜索文件内容 |
| GET | `/find/file` | 按名称查找文件 |
| GET | `/find/symbol` | 查找符号 |
| GET | `/file` | 列出文件目录 |
| GET | `/file/content` | 读取文件内容 |
| GET | `/file/status` | 获取文件状态 |

### 工具 API (实验性)

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/experimental/tool/ids` | 获取所有工具 ID |
| GET | `/experimental/tool` | 获取工具列表及 JSON Schema |

### LSP/格式化器/MCP API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/lsp` | 获取 LSP 服务器状态 |
| GET | `/formatter` | 获取格式化器状态 |
| GET | `/mcp` | 获取 MCP 服务器状态 |
| POST | `/mcp` | 动态添加 MCP 服务器 |

### Agent API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/agent` | 获取所有可用 Agent |

### 日志 API

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/log` | 写入日志条目 |

### TUI API

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/tui/append-prompt` | 追加提示文本 |
| POST | `/tui/open-help` | 打开帮助对话框 |
| POST | `/tui/open-sessions` | 打开会话选择器 |
| POST | `/tui/open-themes` | 打开主题选择器 |
| POST | `/tui/open-models` | 打开模型选择器 |
| POST | `/tui/submit-prompt` | 提交当前提示 |
| POST | `/tui/clear-prompt` | 清除提示 |
| POST | `/tui/execute-command` | 执行命令 |
| POST | `/tui/show-toast` | 显示通知 |
| GET | `/tui/control/next` | 等待下一个控制请求 |
| POST | `/tui/control/response` | 响应控制请求 |

### 认证 API

| 方法 | 路径 | 描述 |
|------|------|------|
| PUT | `/auth/{id}` | 设置认证凭据 |

### 事件 API

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/event` | 事件流 (SSE) |

---

## 详细 API 文档

### 1. 全局 API

#### 1.1 健康检查

```http
GET /global/health
```

响应示例：

```json
{
  "healthy": true,
  "version": "1.2.5"
}
```

#### 1.2 全局事件流

```http
GET /global/event
```

返回 Server-Sent Events 流，包含全局事件。

---

### 2. 会话 API

#### 2.1 创建会话

```http
POST /session
```

请求体：

```json
{
  "parentID": "optional-parent-session-id",
  "title": "My Session"
}
```

响应：返回创建的 Session 对象

#### 2.2 发送消息

```http
POST /session/{id}/message
```

请求体：

```json
{
  "messageID": "optional-reply-to-message-id",
  "model": {
    "providerID": "anthropic",
    "modelID": "claude-3-5-sonnet-20241022"
  },
  "agent": "build",
  "noReply": false,
  "system": "optional-system-prompt",
  "tools": ["read", "edit", "bash"],
  "parts": [
    { "type": "text", "text": "Hello, analyze this codebase" },
    { "type": "reference", "path": "src/main.ts", "startLine": 1, "endLine": 10 }
  ]
}
```

**参数说明：**

| 字段 | 类型 | 描述 |
|------|------|------|
| messageID | string? | 回复的消息 ID |
| model | object? | 指定模型 { providerID, modelID } |
| agent | string? | 使用的 Agent (build/plan) |
| noReply | boolean? | 是否不等待 AI 回复 |
| system | string? | 系统提示 |
| tools | string[]? | 允许使用的工具 |
| parts | MessagePart[] | 消息内容 |

**结构化输出：**

可以通过 `outputFormat` 指定结构化输出：

```json
{
  "parts": [{ "type": "text", "text": "Extract company info" }],
  "format": {
    "type": "json_schema",
    "schema": {
      "type": "object",
      "properties": {
        "company": { "type": "string", "description": "Company name" },
        "founded": { "type": "number", "description": "Year founded" }
      },
      "required": ["company"]
    },
    "retryCount": 2
  }
}
```

响应：

```json
{
  "info": {
    "id": "msg-xxx",
    "role": "assistant",
    "structured_output": {
      "company": "Anthropic",
      "founded": 2021
    }
  },
  "parts": [...]
}
```

#### 2.3 执行命令

```http
POST /session/{id}/command
```

请求体：

```json
{
  "messageID": "optional-message-id",
  "agent": "build",
  "command": "/init"
}
```

#### 2.4 执行 Shell 命令

```http
POST /session/{id}/shell
```

请求体：

```json
{
  "agent": "build",
  "model": { "providerID": "anthropic", "modelID": "claude-3-5-sonnet" },
  "command": "ls -la"
}
```

#### 2.5 获取会话差异

```http
GET /session/{id}/diff?messageID={optional-message-id}
```

响应：

```json
[
  {
    "path": "src/main.ts",
    "status": "modified",
    "diff": "@@ -1,5 +1,7 @@\n..."
  }
]
```

#### 2.6 回滚/恢复

回滚到某个消息：

```http
POST /session/{id}/revert
```

请求体：

```json
{
  "messageID": "msg-xxx",
  "partID": "optional-part-id"
}
```

恢复所有回滚的消息：

```http
POST /session/{id}/unrevert
```

---

### 3. 文件 API

#### 3.1 搜索文件内容

```http
GET /find?pattern=<regex-pattern>
```

查询参数：

| 参数 | 类型 | 描述 |
|------|------|------|
| pattern | string | 正则表达式模式 |
| directory | string? | 搜索目录 |
| file | string? | 文件过滤器 |

#### 3.2 查找文件

```http
GET /find/file?query=<query>&type=<file|directory>&directory=<dir>&limit=<number>
```

#### 3.3 查找符号

```http
GET /find/symbol?query=<query>
```

#### 3.4 读取文件

```http
GET /file/content?path=<file-path>
```

响应：

```json
{
  "type": "raw",
  "content": "file content here"
}
```

或

```json
{
  "type": "patch",
  "content": "@@ -1,3 +1,4 @@..."
}
```

#### 3.5 文件状态

```http
GET /file/status?path=<optional-path>
```

---

### 4. 配置 API

#### 4.1 获取配置

```http
GET /config
```

#### 4.2 更新配置

```http
PATCH /config
```

请求体：

```json
{
  "model": "anthropic/claude-3-5-sonnet-20241022",
  "agent": "build"
}
```

#### 4.3 获取提供商

```http
GET /config/providers
```

响应：

```json
{
  "providers": [
    {
      "id": "anthropic",
      "name": "Anthropic",
      "auth": ["api", "oauth"]
    }
  ],
  "default": {
    "small": "openai/gpt-4o-mini",
    "large": "anthropic/claude-sonnet-4-20250514"
  }
}
```

---

### 5. TUI API

#### 5.1 追加提示

```http
POST /tui/append-prompt
```

请求体：

```json
{
  "text": "additional context"
}
```

#### 5.2 显示通知

```http
POST /tui/show-toast
```

请求体：

```json
{
  "title": "Success",
  "message": "Task completed",
  "variant": "success"
}
```

`variant` 可选值：`info`, `success`, `warning`, `error`

#### 5.3 执行命令

```http
POST /tui/execute-command
```

请求体：

```json
{
  "command": "/init"
}
```

---

### 6. 认证 API

#### 6.1 设置认证凭据

```http
PUT /auth/{provider-id}
```

请求体（API Key 方式）：

```json
{
  "type": "api",
  "key": "sk-xxx"
}
```

请求体（Bearer Token 方式）：

```json
{
  "type": "bearer",
  "token": "xxx"
}
```

---

### 7. 事件流

#### 7.1 订阅事件

```http
GET /event
```

返回 SSE 流，事件类型包括：

- `server.connected` - 服务器连接
- `session.created` - 会话创建
- `session.status` - 会话状态变化
- `session.idle` - 会话空闲
- `session.error` - 会话错误
- `message.updated` - 消息更新
- `tool.execute.before` - 工具执行前
- `tool.execute.after` - 工具执行后

---

## 集成示例

### cURL 示例

```bash
# 健康检查
curl http://localhost:4096/global/health

# 创建会话
curl -X POST http://localhost:4096/session \
  -H "Content-Type: application/json" \
  -d '{"title": "My Session"}'

# 发送消息
curl -X POST http://localhost:4096/session/session-id/message \
  -H "Content-Type: application/json" \
  -d '{
    "parts": [{"type": "text", "text": "Hello"}]
  }'

# 读取文件
curl "http://localhost:4096/file/content?path=src/main.ts"

# 搜索文件
curl "http://localhost:4096/find?pattern=function.*main"
```

### JavaScript/TypeScript SDK

```typescript
import { createOpencode } from "@opencode-ai/sdk"

const { client } = await createOpencode()

// 创建会话
const session = await client.session.create({
  body: { title: "My Session" }
})

// 发送消息
const result = await client.session.prompt({
  path: { id: session.id },
  body: {
    parts: [{ type: "text", text: "Analyze this codebase" }]
  }
})

// 读取文件
const content = await client.file.read({
  query: { path: "src/main.ts" }
})
```

### Go 示例

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type OpenCodeClient struct {
    baseURL string
    client  *http.Client
}

func NewClient(baseURL string) *OpenCodeClient {
    return &OpenCodeClient{
        baseURL: baseURL,
        client:  &http.Client{},
    }
}

type Session struct {
    ID        string `json:"id"`
    Title     string `json:"title,omitempty"`
    Status    string `json:"status"`
    CreatedAt string `json:"createdAt"`
}

type MessageRequest struct {
    Parts []map[string]string `json:"parts"`
}

type MessageResponse struct {
    Info  MessageInfo `json:"info"`
    Parts []json.RawMessage `json:"parts"`
}

type MessageInfo struct {
    ID      string `json:"id"`
    Role    string `json:"role"`
    Created string `json:"createdAt"`
}

func (c *OpenCodeClient) CreateSession(title string) (*Session, error) {
    body, _ := json.Marshal(map[string]string{"title": title})
    resp, err := c.client.Post(c.baseURL+"/session", "application/json", bytes.NewBuffer(body))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var session Session
    if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
        return nil, err
    }
    return &session, nil
}

func (c *OpenCodeClient) SendMessage(sessionID, text string) (*MessageResponse, error) {
    req := MessageRequest{
        Parts: []map[string]string{{"type": "text", "text": text}},
    }
    body, _ := json.Marshal(req)
    
    resp, err := c.client.Post(
        c.baseURL+"/session/"+sessionID+"/message",
        "application/json",
        bytes.NewBuffer(body))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var msg MessageResponse
    if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
        return nil, err
    }
    return &msg, nil
}

func (c *OpenCodeClient) ReadFile(path string) (string, error) {
    resp, err := c.client.Get(c.baseURL + "/file/content?path=" + path)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    var result struct {
        Type    string `json:"type"`
        Content string `json:"content"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }
    return result.Content, nil
}

func main() {
    client := NewClient("http://localhost:4096")
    
    // 创建会话
    session, err := client.CreateSession("Go Integration Test")
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Session created: %s\n", session.ID)
    
    // 发送消息
    msg, err := client.SendMessage(session.ID, "Hello, what is 1+1?")
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("Message sent: %s\n", msg.Info.ID)
    
    // 读取文件
    content, err := client.ReadFile("package.json")
    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Printf("File content: %s\n", content)
}
```

### Python 示例

```python
import requests
import json

class OpenCodeClient:
    def __init__(self, base_url="http://localhost:4096"):
        self.base_url = base_url
        self.session = requests.Session()
    
    def health(self):
        resp = self.session.get(f"{self.base_url}/global/health")
        return resp.json()
    
    def create_session(self, title=None):
        body = {"title": title} if title else {}
        resp = self.session.post(f"{self.base_url}/session", json=body)
        return resp.json()
    
    def send_message(self, session_id, text, model=None):
        body = {
            "parts": [{"type": "text", "text": text}]
        }
        if model:
            body["model"] = model
        
        resp = self.session.post(
            f"{self.base_url}/session/{session_id}/message",
            json=body
        )
        return resp.json()
    
    def read_file(self, path):
        resp = self.session.get(
            f"{self.base_url}/file/content",
            params={"path": path}
        )
        return resp.json()
    
    def find(self, pattern, directory=None):
        params = {"pattern": pattern}
        if directory:
            params["directory"] = directory
        resp = self.session.get(f"{self.base_url}/find", params=params)
        return resp.json()

# 使用示例
client = OpenCodeClient()

# 健康检查
print(client.health())

# 创建会话
session = client.create_session("My Session")
print(f"Session: {session['id']}")

# 发送消息
result = client.send_message(session["id"], "Hello!")
print(f"Response: {result}")

# 读取文件
content = client.read_file("src/main.ts")
print(f"Content: {content}")
```

---

## 错误处理

### 错误响应格式

```json
{
  "error": {
    "code": "SESSION_NOT_FOUND",
    "message": "Session not found",
    "details": {}
  }
}
```

### 常见错误码

| 错误码 | 描述 |
|--------|------|
| `SESSION_NOT_FOUND` | 会话不存在 |
| `SESSION_BUSY` | 会话正忙 |
| `PERMISSION_DENIED` | 权限不足 |
| `PROVIDER_NOT_CONFIGURED` | 提供商未配置 |
| `MODEL_NOT_FOUND` | 模型不存在 |
| `TOOL_NOT_FOUND` | 工具不存在 |
| `INVALID_REQUEST` | 请求格式错误 |
| `AUTH_REQUIRED` | 需要认证 |

---

## 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.2.5 | 2026-02-15 | 当前版本 |
| 1.2.0 | 2026-01-20 | 添加结构化输出支持 |
| 1.1.0 | 2025-12-15 | 添加 MCP 支持 |
| 1.0.0 | 2025-10-01 | 初始版本 |
