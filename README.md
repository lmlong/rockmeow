# LingGuard

一款基于 Go 语言的超轻量级个人 AI 智能助手，参考 [nanobot](https://github.com/HKUDS/nanobot) 设计。

## 特性

- **多 LLM 支持** - OpenAI, Anthropic, DeepSeek, GLM, Qwen, MiniMax 等
- **Provider 自动匹配** - 根据模型名自动选择合适的 Provider
- **多渠道支持** - 飞书、QQ 机器人，WebSocket 长连接，无需公网 IP
- **流式响应** - 实时输出，飞书消息实时更新
- **技能系统** - 渐进式加载，按需注入
- **持久化记忆** - MEMORY.md + HISTORY.md 方案
- **定时任务** - Cron 调度，支持时区
- **子代理** - 后台异步执行复杂任务
- **网页工具** - 搜索和抓取网页内容
- **单二进制部署** - 无运行时依赖

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/your-org/lingguard.git
cd lingguard
```

### 2. 构建

```bash
# 直接构建
go build -o lingguard ./cmd/lingguard

# 或使用 make
make build
```

### 3. 配置

```bash
# 复制示例配置
cp configs/config.example.json ~/.lingguard/config.json

# 编辑配置，填入 API Key
vim ~/.lingguard/config.json
```

最小配置示例：
```json
{
  "providers": {
    "deepseek": {
      "apiKey": "sk-xxx"
    }
  },
  "agents": {
    "provider": "deepseek"
  }
}
```

### 4. 运行

```bash
# 交互模式
./lingguard agent

# 单次消息
./lingguard agent -m "你好"

# 启动网关（飞书）
./lingguard gateway
```

## 构建方法

### 标准构建

```bash
go build -o lingguard ./cmd/lingguard
```

### 优化构建

```bash
# 减小二进制体积
go build -ldflags="-s -w" -o lingguard ./cmd/lingguard

# 交叉编译 Linux
GOOS=linux GOARCH=amd64 go build -o lingguard-linux ./cmd/lingguard

# 交叉编译 macOS
GOOS=darwin GOARCH=amd64 go build -o lingguard-darwin ./cmd/lingguard

# 交叉编译 Windows
GOOS=windows GOARCH=amd64 go build -o lingguard.exe ./cmd/lingguard
```

### 使用 Makefile

```bash
make build          # 标准构建
make build-linux    # Linux 构建
make build-darwin   # macOS 构建
make build-windows  # Windows 构建
make clean          # 清理
```

### 依赖管理

```bash
# 下载依赖
go mod download

# 更新依赖
go mod tidy
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
# 启动网关（连接飞书）
./lingguard gateway
```

### 定时任务

```bash
# 添加任务
./lingguard cron add "早间简报" "cron:0 9 * * *" "生成今日简报"

# 添加带时区的任务
./lingguard cron add "NYC Morning" "cron:0 9 * * *" "Good morning!" --tz "America/New_York"

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
2. 当前目录 `./config.json`
3. 用户目录 `~/.lingguard/config.json`

### 完整配置示例

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
    },
    "qq": {
      "enabled": false,
      "appId": "xxx",
      "secret": "xxx"
    }
  },
  "tools": {
    "braveApiKey": "",
    "webMaxChars": 50000
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

## 目录结构

```
lingguard/
├── cmd/
│   ├── lingguard/       # 主程序入口
│   └── cli/             # CLI 命令
├── internal/
│   ├── agent/           # 核心代理
│   ├── providers/       # LLM 提供商
│   ├── channels/        # 消息渠道（飞书）
│   ├── tools/           # 内置工具
│   ├── skills/          # 技能系统
│   ├── cron/            # 定时任务
│   ├── subagent/        # 子代理
│   ├── session/         # 会话管理
│   ├── config/          # 配置管理
│   ├── bus/             # 消息总线（预留）
│   └── scheduler/       # 调度器（预留）
├── pkg/
│   ├── llm/             # LLM 类型
│   ├── stream/          # 流式响应
│   ├── memory/          # 记忆系统
│   └── logger/          # 日志
├── skills/              # 技能目录
│   └── builtin/         # 内置技能
├── configs/             # 配置文件
└── docs/                # 文档
```

## 文档

- [架构文档](docs/ARCHITECTURE.md) - 系统架构和与 nanobot 的对比
- [API 文档](docs/API.md) - API 接口和使用说明

## 与 nanobot 对比

| 方面 | LingGuard | nanobot |
|------|-----------|---------|
| 语言 | Go | Python |
| 部署 | 单二进制 | pip/uv |
| 内存 | ~20MB | ~100MB+ |
| 渠道 | 飞书、QQ | 9+ 渠道 |
| 定时任务 | ✅ | ✅ |
| 时区支持 | ✅ | ✅ |

## 依赖

- Go 1.23+
- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- [robfig/cron](https://github.com/robfig/cron) - Cron 调度
- [larksuite/oapi-sdk-go](https://github.com/larksuite/oapi-sdk-go) - 飞书 SDK

## License

MIT
