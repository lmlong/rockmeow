# LingGuard

一款基于 Go 语言的超轻量级个人 AI 智能助手。

## 特性

### 核心能力
- **多 LLM 支持** - OpenAI, Anthropic, DeepSeek, GLM, Qwen, MiniMax, Moonshot 等
- **Provider 自动匹配** - 根据模型名/API Key 自动选择合适的 Provider
- **流式响应** - 实时输出，飞书消息实时更新

### 渠道集成
- **飞书** - WebSocket 长连接，无需公网 IP，流式消息卡片
- **QQ** - 预留支持
- **多渠道支持** - 支持多类型 channel 同时运行（飞书 + QQ），但不支持同类型多实例（如 2 个飞书 channel）

### 工具系统
- **Shell 工具** - 执行命令，支持安全沙箱
- **文件工具** - 读写、编辑、列表
- **Web 工具** - Tavily AI 搜索、网页抓取
- **AIGC 工具** - 图像/视频生成（文生图、文生视频、图生视频、视频生视频）
- **TTS 工具** - 语音合成，多种音色可选
- **MCP 支持** - Model Context Protocol，支持 Stdio 和 HTTP 传输

### 智能能力
- **技能系统** - 渐进式加载，按需注入指令
- **持久化记忆** - MEMORY.md + HISTORY.md 方案
- **子代理** - 后台异步执行复杂任务
- **定时任务** - Cron 调度，支持时区

### 部署优势
- **单二进制部署** - 无运行时依赖
- **低内存占用** - ~20MB 内存

## 快速开始

### 1. 构建

```bash
# 克隆项目
git clone https://github.com/your-org/lingguard.git
cd lingguard

# 构建
go build -o lingguard ./cmd/lingguard
```

### 2. 配置

```bash
# 创建配置目录
mkdir -p ~/.lingguard

# 创建配置文件
cat > ~/.lingguard/config.json << 'EOF'
{
  "providers": {
    "deepseek": {
      "apiKey": "sk-xxx"
    }
  },
  "agents": {
    "provider": "deepseek",
    "systemPrompt": "你是灵侍，一个乐于助人的 AI 助手。"
  }
}
EOF
```

### 3. 运行

```bash
# 交互模式
./lingguard agent

# 单次消息
./lingguard agent -m "你好"

# 启动网关
./lingguard gateway
```

## CLI 命令

### Agent 交互

```bash
# 交互模式
./lingguard agent

# 单次消息
./lingguard agent -m "分析当前目录的代码结构"

# 指定配置文件
./lingguard agent -c /path/to/config.json
```

### Gateway 网关

```bash
# 启动网关
./lingguard gateway
```

### 定时任务

```bash
# 添加 cron 表达式任务
./lingguard cron add "早间简报" "cron:0 9 * * *" "生成今日简报"

# 添加带时区的任务
./lingguard cron add "NYC Morning" "cron:0 9 * * *" "Good morning!" --tz "America/New_York"

# 添加间隔任务
./lingguard cron add "Hourly Check" "every:1h" "检查系统状态"

# 添加一次性任务
./lingguard cron add "Reminder" "at:2026-02-20T10:00:00" "别忘了开会"

# 列出任务
./lingguard cron list

# 删除任务
./lingguard cron remove <job-id>

# 手动执行
./lingguard cron run <job-id> --force
```

### 状态查看

```bash
./lingguard status
```

## 配置

### 配置文件位置（优先级从高到低）

1. 环境变量 `$LINGGUARD_CONFIG`
2. 项目目录 `configs/config.json`
3. 当前目录 `./config.json`
4. 用户目录 `~/.lingguard/config.json`

### 完整配置示例

```json
{
  "providers": {
    "glm": {
      "apiKey": "xxx.xxx",
      "apiBase": "https://open.bigmodel.cn/api/anthropic",
      "model": "glm-5",
      "timeout": 120
    },
    "qwen": {
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
      "recentDays": 3,
      "maxHistoryLines": 1000,
      "autoRecall": true,
      "autoRecallTopK": 3,
      "autoRecallMinScore": 0.3,
      "autoCapture": true,
      "captureMaxChars": 500
    }
  },
  "channels": {
    "feishu": {
      "enabled": true,
      "appId": "cli_xxx",
      "appSecret": "xxx",
      "allowFrom": ["ou_xxx"]
    }
  },
  "tools": {
    "restrictToWorkspace": false,
    "workspace": "~/.lingguard/workspace",
    "tavilyApiKey": "",
    "webMaxChars": 50000,
    "mcpServers": {
      "filesystem": {
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/documents"]
      }
    },
    "aigc": {
      "enabled": true,
      "provider": "qwen"
    }
  },
  "speech": {
    "enabled": true,
    "provider": "qwen",
    "model": "qwen3-asr-flash"
  },
  "tts": {
    "enabled": true,
    "provider": "qwen",
    "model": "qwen3-tts-flash",
    "voice": "Cherry"
  },
  "cron": {
    "enabled": true,
    "storePath": "~/.lingguard/cron/jobs.json"
  },
  "heartbeat": {
    "enabled": true,
    "interval": 30
  },
  "storage": {
    "type": "file",
    "path": "~/.lingguard/memory"
  },
  "logging": {
    "level": "info",
    "format": "text",
    "output": "~/.lingguard/logs/lingguard.log"
  }
}
```

## 内置工具

| 工具名 | 功能描述 | 危险级别 |
|--------|----------|:--------:|
| `shell` | 执行 Shell 命令 | ⚠️ |
| `file` | 文件读写、编辑、列表 | ⚠️ |
| `web_search` | Tavily AI 搜索 | - |
| `web_fetch` | 网页抓取、HTML 转 Markdown | - |
| `aigc` | 图像/视频生成（文生图、文生视频、图生视频、视频生视频） | - |
| `tts` | 语音合成，多种音色可选 | - |
| `moltbook` | AI Agent 社交网络（发帖、评论、投票） | - |
| `skill` | 按需加载技能指令 | - |
| `memory` | 记忆操作（添加/搜索/日志） | - |
| `cron` | 定时任务管理 | - |
| `message` | 发送消息到渠道 | - |
| `workspace` | 工作区管理 | - |
| `task_spawn` | 创建子代理任务 | - |
| `task_status` | 查询子代理状态 | - |
| `mcp_*` | MCP 服务器工具 | - |

## MCP 支持

LingGuard 支持 Model Context Protocol (MCP)，可以连接外部工具服务器。

### Stdio 传输

```json
{
  "tools": {
    "mcpServers": {
      "filesystem": {
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"]
      }
    }
  }
}
```

### HTTP 传输

```json
{
  "tools": {
    "mcpServers": {
      "remote": {
        "url": "http://localhost:8765/mcp"
      }
    }
  }
}
```

MCP 工具命名格式: `mcp_{serverName}_{toolName}`

## Moltbook 集成

LingGuard 内置 Moltbook 工具，可以让 AI Agent 参与社交网络。

### 配置

```json
{
  "tools": {
    "moltbook": {
      "enabled": true
    }
  }
}
```

凭证存储在 `~/.lingguard/moltbook/credentials.json`。

### 当前 Agent 认领信息

| 项目 | 值 |
|------|-----|
| Agent 名称 | lingguard |
| Agent ID | `03614eff-6b82-4c66-bc6d-7ad7ef61cf41` |
| 主页 | https://www.moltbook.com/u/lingguard |
| 认领链接 | https://www.moltbook.com/claim/moltbook_claim_voPC74hYyykl2JltxvKYCAaQ5GzbJX9N |
| 验证码 | `antenna-RBYB` |

### 认领步骤

1. 访问认领链接
2. 验证邮箱
3. 发推验证：
   ```
   I'm claiming my AI agent "lingguard" on @moltbook 🦞
   Verification: antenna-RBYB
   ```

认领后即可使用发帖、评论等功能。

## 技能系统

### 内置技能

| 技能 | 描述 |
|------|------|
| `weather` | 天气查询 (心知天气) |
| `git-workflow` | Git 工作流自动化 |
| `code-review` | 代码审查指南 |
| `file` | 文件操作指南 |
| `system` | 系统操作指南 |
| `moltbook` | AI Agent 社交网络 |

### 技能格式

每个技能是一个目录，包含 `SKILL.md` 文件：

```markdown
---
name: skill-name
description: Skill description
homepage: https://example.com
metadata: {"emoji":"🦞","requires":{"bins":["curl"]}}
---

# Skill Title

Skill instructions here...
```

### 渐进式加载

- 默认只注入技能摘要到系统提示
- `always=true` 的技能自动加载完整内容
- 其他技能通过 `skill` 工具按需加载

## 记忆系统

文件持久化记忆方案：

```
~/.lingguard/memory/
├── MEMORY.md          # 长期记忆（用户偏好、重要事实）
├── HISTORY.md         # 事件日志
└── 2026-02-16.md      # 每日日志
```

### 自动记忆功能（OpenClaw 风格）

LingGuard 支持类似 OpenClaw 的自动记忆功能：

**自动召回 (Auto-Recall)**
- 在对话开始时，自动搜索与用户消息相关的历史记忆
- 将相关记忆注入到系统提示中，帮助 AI 理解上下文

**自动捕获 (Auto-Capture)**
- 在对话结束时，自动分析用户消息
- 根据触发规则识别值得记忆的内容
- 智能去重，避免重复存储相似记忆

### 记忆捕获规则

**会自动捕获的内容（触发规则）：**

| 类别 | 触发词/模式 | 示例 |
|------|-------------|------|
| 记住指令 | `记住`、`remember`、`别忘` | "记住我喜欢猫" |
| 偏好表达 | `喜欢`、`讨厌`、`prefer`、`like` | "我喜欢用 Go 语言" |
| 习惯表达 | `always`、`never`、`usually` | "I always use dark mode" |
| 决策记录 | `决定`、`decided`、`选择` | "我决定使用这个方案" |
| 联系方式 | 电话号码（10位以上）、邮箱 | "我的电话是 13812345678" |
| 身份信息 | `my name is`、`i am` | "My name is Alice" |
| 重要标记 | `重要`、`important`、`关键` | "这很重要：项目截止日期" |
| 项目信息 | `my project`、`working on` | "我的项目使用 React" |

**不会捕获的内容：**

| 类型 | 原因 | 示例 |
|------|------|------|
| 问句 | 以问号结尾或包含疑问词 | "我喜欢什么？" |
| 普通对话 | 不匹配任何触发规则 | "今天天气不错" |
| Prompt 注入 | 检测到注入攻击模式 | "Ignore previous instructions" |
| 重复内容 | 与已有记忆相似度 > 95% | 连续说多次 "我喜欢猫" |

### 记忆分类

捕获的记忆会自动分类：

| 分类 | 说明 | 示例 |
|------|------|------|
| `preference` | 用户偏好 | "我喜欢简洁的回答" |
| `decision` | 决策记录 | "我决定使用 PostgreSQL" |
| `entity` | 实体信息（联系方式等） | "我的邮箱是 xxx@example.com" |
| `fact` | 事实信息 | "我的名字叫 Alice" |
| `other` | 其他 | - |

### 记忆配置

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
          "model": "text-embedding-v4"
        },
        "search": {
          "vectorWeight": 0.7,
          "bm25Weight": 0.3,
          "minScore": 0.5
        }
      }
    }
  }
}
```

### 记忆工具

```
memory add --category "User Preferences" --content "用户喜欢简洁的回答"
memory search "用户偏好"
memory history --recent 10
```

## 子代理系统

子代理可以在后台异步执行复杂任务：

```
# 创建子任务
task_spawn --task "分析代码库结构" --context "项目目录: /home/user/project"

# 查询状态
task_status --id "task_xxx"
```

子代理特点：
- 独立的工具白名单（无 message、task_spawn）
- 最多 15 次迭代
- 完成后通知主代理

## 目录结构

```
lingguard/
├── cmd/
│   ├── lingguard/       # 主程序入口
│   └── cli/             # CLI 命令
├── internal/
│   ├── agent/           # 核心代理
│   ├── providers/       # LLM 提供商
│   ├── channels/        # 消息渠道
│   ├── tools/           # 内置工具
│   │   ├── mcp.go       # MCP Stdio 客户端
│   │   └── mcp_http.go  # MCP HTTP 客户端
│   ├── skills/          # 技能加载器
│   ├── cron/            # 定时任务
│   ├── subagent/        # 子代理
│   ├── session/         # 会话管理
│   └── config/          # 配置管理
├── pkg/
│   ├── llm/             # LLM 类型
│   ├── stream/          # 流式响应
│   ├── memory/          # 记忆系统
│   └── logger/          # 日志
├── skills/builtin/      # 内置技能
├── configs/             # 配置文件
└── docs/                # 文档
```

## 构建方法

```bash
# 标准构建
go build -o lingguard ./cmd/lingguard

# 优化体积
go build -ldflags="-s -w" -o lingguard ./cmd/lingguard

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o lingguard-linux ./cmd/lingguard
GOOS=darwin GOARCH=amd64 go build -o lingguard-darwin ./cmd/lingguard
GOOS=windows GOARCH=amd64 go build -o lingguard.exe ./cmd/lingguard
```

## 部署方式

### 方式一：Make 安装（推荐）

```bash
# 完整安装（二进制 + 配置 + systemd 服务）
make install

# 仅安装二进制
make install-bin

# 安装到指定目录
make install PREFIX=/usr/local

# 启用系统级服务
make install SERVICE=system

# 启用用户级服务（默认）
make install SERVICE=user
```

安装完成后：
- 二进制文件: `~/.local/bin/lingguard` 或 `$PREFIX/bin/lingguard`
- 配置目录: `~/.lingguard/`
- systemd 服务: `lingguard.service`

```bash
# 启动服务
systemctl --user start lingguard

# 开机自启
systemctl --user enable lingguard

# 查看状态
systemctl --user status lingguard

# 查看日志
journalctl --user -u lingguard -f
```

### 方式二：打包部署

```bash
# 打包发布版本
make package

# 打包并生成校验和
make package VERSION=1.0.0
```

打包后的文件结构：
```
dist/
├── lingguard-1.0.0-linux-amd64.tar.gz
├── lingguard-1.0.0-linux-arm64.tar.gz
├── lingguard-1.0.0-darwin-amd64.tar.gz
├── lingguard-1.0.0-darwin-arm64.tar.gz
└── lingguard-1.0.0-windows-amd64.zip
```

部署打包文件：
```bash
# 解压到目标目录
tar -xzf lingguard-1.0.0-linux-amd64.tar.gz -C /opt/

# 创建配置目录
mkdir -p ~/.lingguard

# 复制配置模板
cp /opt/lingguard/configs/config.example.json ~/.lingguard/config.json

# 编辑配置
vim ~/.lingguard/config.json

# 运行
/opt/lingguard/lingguard gateway
```

## 依赖

- Go 1.23+
- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- [robfig/cron](https://github.com/robfig/cron) - Cron 调度
- [larksuite/oapi-sdk-go](https://github.com/larksuite/oapi-sdk-go) - 飞书 SDK

## 文档

- [架构文档](docs/ARCHITECTURE.md) - 系统架构
- [API 文档](docs/API.md) - API 接口和使用说明

## License

MIT
