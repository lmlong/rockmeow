# Tasks Board

任务看板应用，用于追踪 AI 助手与用户协作的所有任务。

## 技术栈

- Next.js 14 (App Router)
- TypeScript
- Tailwind CSS
- Convex (数据库和实时同步)

## 快速开始

### 1. 安装依赖

```bash
npm install
```

### 2. 配置 Convex

首先需要创建 Convex 账户并设置项目：

```bash
# 登录 Convex
npx convex dev

# 这会自动创建 .env.local 文件并配置 CONVEX_URL
```

### 3. 启动开发服务器

```bash
# 启动 Next.js 开发服务器
npm run dev

# 在另一个终端启动 Convex 开发服务器
npm run convex:dev
```

访问 http://localhost:3000 查看看板。

## API 路由

### 外部系统集成 API

看板提供了 REST API 供 LingGuard 后端调用：

#### 获取任务列表

```
GET /api/tasks
GET /api/tasks?status=pending
GET /api/tasks?sessionId=xxx
```

#### 创建任务

```
POST /api/tasks
Content-Type: application/json
X-API-Key: your-api-key

{
  "title": "任务标题",
  "description": "任务描述",
  "assignee": "ai",  // user | ai | both
  "priority": "high", // low | medium | high
  "sessionId": "可选的会话ID",
  "tags": ["tag1", "tag2"]
}
```

#### 批量同步任务

```
POST /api/tasks
Content-Type: application/json
X-API-Key: your-api-key

{
  "tasks": [
    {
      "externalId": "unique-id",
      "title": "任务标题",
      "status": "pending",
      "assignee": "ai",
      ...
    }
  ]
}
```

#### 更新任务状态

```
PATCH /api/tasks
Content-Type: application/json
X-API-Key: your-api-key

{
  "id": "任务ID",
  "status": "completed", // pending | running | completed | failed
  "result": "任务结果",
  "error": "错误信息（如果失败）"
}
```

#### 删除任务

```
DELETE /api/tasks?id=任务ID
X-API-Key: your-api-key
```

### API Key 配置

在 `.env.local` 中设置 API Key：

```
TASKS_API_KEY=your-secret-key
```

## 数据模型

```typescript
{
  title: string;           // 任务标题
  description?: string;    // 任务描述
  status: "pending" | "running" | "completed" | "failed";
  assignee: "user" | "ai" | "both";
  sessionId?: string;      // 关联的会话ID
  subagentId?: string;     // 子代理ID
  priority?: "low" | "medium" | "high";
  tags?: string[];         // 标签
  result?: string;         // 任务结果
  error?: string;          // 错误信息
  createdAt: number;       // 创建时间戳
  updatedAt: number;       // 更新时间戳
  startedAt?: number;      // 开始时间戳
  completedAt?: number;    // 完成时间戳
  metadata?: {
    source?: string;       // 来源标识
    command?: string;      // 执行的命令
    workingDirectory?: string;
  };
}
```

## 项目结构

```
apps/tasks-board/
├── app/
│   ├── api/tasks/route.ts   # REST API 路由
│   ├── globals.css          # 全局样式
│   ├── layout.tsx           # 根布局
│   └── page.tsx             # 主页面
├── components/
│   ├── board/
│   │   ├── TaskBoard.tsx    # 看板主组件
│   │   ├── TaskCard.tsx     # 任务卡片
│   │   └── TaskColumn.tsx   # 列组件
│   ├── task/
│   │   ├── TaskForm.tsx     # 任务表单
│   │   └── TaskStatusBadge.tsx
│   └── ui/
│       └── Button.tsx       # 按钮组件
├── convex/
│   ├── _generated/          # 自动生成的类型
│   ├── schema.ts            # 数据模型
│   └── tasks.ts             # Convex API
└── lib/
    └── utils.ts             # 工具函数
```

## LingGuard 集成

任务看板已集成到 LingGuard Go 后端。

### 配置

在 `~/.lingguard/config.json` 中添加：

```json
{
  "tools": {
    "tasksBoard": {
      "enabled": true,
      "url": "http://localhost:3000/api/tasks",
      "apiKey": ""
    }
  }
}
```

### 使用方式

AI 助手可以使用 `tasks_board` 工具来同步任务：

**创建任务：**
```json
{
  "action": "create",
  "task": {
    "title": "实现新功能",
    "description": "添加用户认证功能",
    "assignee": "ai",
    "priority": "high",
    "tags": ["后端", "安全"]
  }
}
```

**更新任务状态：**
```json
{
  "action": "update",
  "taskId": "任务ID",
  "task": {
    "status": "completed",
    "result": "功能已实现"
  }
}
```

**查询任务：**
```json
{
  "action": "get",
  "status": "pending"
}
```

**批量同步：**
```json
{
  "action": "sync",
  "tasks": [
    {
      "externalId": "task-001",
      "title": "任务1",
      "status": "pending"
    }
  ]
}
```

### 任务状态

| 状态 | 说明 |
|------|------|
| `pending` | 待办 |
| `running` | 进行中 |
| `completed` | 已完成 |
| `failed` | 失败 |

### 分配者

| 分配者 | 说明 |
|--------|------|
| `ai` | AI 助手 |
| `user` | 用户 |
| `both` | 协作 |

