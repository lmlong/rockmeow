# LingGuard

基于 Go 语言的超轻量级个人 AI 智能助手。

## 特性

- **多 LLM 支持** - OpenAI, Anthropic, DeepSeek, GLM, Qwen, MiniMax, Moonshot, Groq, Gemini 等
- **飞书集成** - WebSocket 长连接，流式消息卡片
- **工具系统** - Shell, 文件, Web 搜索, AIGC, TTS, MCP
- **技能系统** - 渐进式加载，按需注入
- **记忆系统** - 长期记忆 + 向量检索 + 会话持久化
- **单二进制部署** - 无运行时依赖，~20MB 内存

## 快速开始

```bash
# 克隆项目
git clone https://github.com/your-org/lingguard.git
cd lingguard

# 构建
make build

# 安装（会自动创建配置文件模板）
make install

# 修改配置文件，填入 API Key
vim ~/.lingguard/config.json

# 重启服务
systemctl --user restart lingguard  # Linux
# 或
launchctl load ~/Library/LaunchAgents/com.lingguard.plist  # macOS
```

## Make 命令

### 构建 & 测试

```bash
make build          # 构建项目
make run            # 构建并运行
make clean          # 清理构建产物
make test           # 运行测试（带 race 检测）
make test-coverage  # 生成覆盖率报告
```

### 打包发布

```bash
make package            # 打包 Linux + macOS
make package-all        # 打包所有平台
make package-linux      # Linux (amd64 + arm64)
make package-darwin     # macOS (Intel + Apple Silicon)
make package-windows    # Windows (amd64 + arm64)
```

单独平台：
```bash
make package-linux-amd64
make package-darwin-arm64
make package-windows-amd64
```

### 安装部署

```bash
make install        # 完整安装（二进制 + 配置 + 服务）
make install-bin    # 仅安装二进制到 ~/.local/bin
make uninstall      # 卸载
```

指定安装路径：
```bash
PREFIX=/usr/local make install
```

### 开发

```bash
make dev            # 开发模式运行
make fmt            # 格式化代码
make lint           # 静态检查
make docker         # 构建 Docker 镜像
```

### 帮助

```bash
make help           # 显示所有命令
```

## 配置

配置文件位置（优先级从高到低）：
1. 环境变量 `$LINGGUARD_CONFIG`
2. 用户目录 `~/.lingguard/config.json`

安装后会自动创建配置文件模板，需要修改后重启服务：

```bash
vim ~/.lingguard/config.json
systemctl --user restart lingguard  # Linux
```

### agents 配置

| 字段 | 说明 |
|------|------|
| `provider` | 主 LLM 提供商，用于文本对话和工具调用 |
| `multimodalProvider` | 多模态提供商，用于处理图片/视频输入。**可选**，如未设置则使用 `provider` |

**调用关系：**
```
用户消息
    │
    ├── 纯文本消息 → provider 处理
    │
    └── 包含图片/视频
            │
            ├── multimodalProvider 已配置？
            │       │
            │       ├── 是 → multimodalProvider 处理
            │       │
            │       └── 否 → provider 处理（需支持视觉）
```

**配置示例：**
```json
{
  "providers": {
    "deepseek": { "apiKey": "sk-xxx" },      // 文本模型，便宜
    "qwen": { "apiKey": "sk-xxx" }           // 支持 vision
  },
  "agents": {
    "provider": "deepseek",                   // 默认用 deepseek
    "multimodalProvider": "qwen"              // 图片消息用 qwen
  }
}
```

### tools.websearch 配置

| 字段 | 说明 |
|------|------|
| `tavilyApiKey` | Tavily Search API Key（国际搜索，质量高） |
| `bochaApiKey` | 博查 AI 搜索 API Key（中文搜索优化） |

**优先调用关系：**
```
web_search 工具调用
    │
    ├── tavilyApiKey 已配置？
    │       │
    │       ├── 是 → 调用 Tavily API
    │       │       │
    │       │       ├── 成功 → 返回结果
    │       │       │
    │       │       └── 失败 → 尝试博查（如有配置）
    │       │
    │       └── 否 ↓
    │
    └── bochaApiKey 已配置？
            │
            ├── 是 → 调用博查 AI API
            │
            └── 否 → 报错：未配置搜索 API Key
```

**配置示例：**
```json
{
  "tools": {
    "websearch": {
      "tavilyApiKey": "tvly-xxx",      // 优先使用
      "bochaApiKey": "bocha-xxx"       // 备用（中文优化）
    }
  }
}
```

### tools.opencode 配置

OpenCode 是一个 HTTP API 服务，用于执行编码任务。LingGuard 通过 HTTP 调用 OpenCode 服务。

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `enabled` | 是否启用 OpenCode 工具 | `false` |
| `baseURL` | OpenCode 服务地址 | `http://127.0.0.1:4096` |
| `timeout` | 请求超时时间（秒） | `300` |

**调用关系：**
```
用户请求编码任务
    │
    └── Agent 调用 opencode 工具
            │
            ├── 检查 OpenCode 服务健康状态
            │       │
            │       ├── 运行中 → 直接使用
            │       │
            │       └── 未运行 → 自动启动 opencode server
            │                   （在工作目录执行 opencode 命令）
            │
            └── 发送 HTTP 请求到 baseURL
                    │
                    ├── POST /sessions        创建会话
                    ├── POST /chat            发送消息
                    └── GET /events (SSE)     接收流式响应
```

**配置示例：**
```json
{
  "tools": {
    "opencode": {
      "enabled": true,
      "baseURL": "http://127.0.0.1:4096",
      "timeout": 300
    }
  }
}
```

**前置条件：**
- 需要安装 `opencode` 命令行工具
- LingGuard 会自动启动/管理 OpenCode 服务

## 内置工具

| 工具 | 功能 |
|------|------|
| `shell` | 执行命令（支持沙箱） |
| `file` | 文件读写、编辑 |
| `web_search` | 网页搜索（Tavily/博查） |
| `web_fetch` | 网页抓取 |
| `aigc` | 图像/视频生成 |
| `tts` | 语音合成 |
| `memory` | 记忆操作 |
| `skill` | 加载技能 |
| `cron` | 定时任务 |
| `opencode` | 编码任务（需配置） |
| `mcp_*` | MCP 工具 |

## 技能系统

| 技能 | 描述 |
|------|------|
| `clawhub` | 技能仓库 |
| `git-sync` | Git 工作流 |
| `coding` | 编码任务 |
| `weather` | 天气查询 |

### 渐进式加载机制

技能和工具采用**渐进式加载**：

1. **默认工具**：始终加载（skill, memory, message 等）
2. **动态工具**：按需加载（aigc, web_search, opencode 等）

**加载流程**：
```
用户: "生成美女图片"
    ↓
LLM 调用 skill(name="aigc")
    ↓
返回: { content: "详细指令...", tools: [aigc工具定义] }
    ↓
动态注入 aigc 工具到当前会话
    ↓
LLM 调用 aigc 工具生成图片
```

### 会话隔离 & 老化机制

- **会话隔离**：每个会话独立管理动态工具，互不影响
- **老化机制**：连续 10 次请求未使用的动态工具自动卸载

### 最佳实践：业务隔离

> ⚠️ **建议每个业务场景使用独立的会话通道**（如独立的飞书群）

**原因**：
- 避免多个业务的动态工具累积，增加 LLM 输入 token 消耗
- 每个会话只加载相关工具，减少 LLM 混淆

**推荐配置**：
```
飞书群 A (AI绘画群) → 只加载 aigc 工具
飞书群 B (搜索群)   → 只加载 web_search 工具
飞书群 C (编程群)   → 只加载 opencode 工具
```

老化机制可以缓解单一通道多业务的 token 浪费问题，但独立通道仍是最佳实践。

## 记忆系统

```
~/.lingguard/memory/
├── MEMORY.md        # 长期记忆
├── sessions/        # 会话持久化（JSON）
├── vectors.db       # 向量索引
└── 2026-03-04.md    # 每日日志
```

**自动功能：**
- **Auto-Recall**: 对话开始时自动召回相关记忆
- **Auto-Capture**: 对话结束时自动捕获重要内容

## CLI 命令

```bash
./lingguard agent                    # 交互模式
./lingguard agent -m "你好"          # 单次消息
./lingguard gateway                  # 启动网关

# 定时任务
./lingguard cron add "日报" "cron:0 9 * * *" "生成日报"
./lingguard cron list
./lingguard status
```

## 目录结构

```
lingguard/
├── cmd/lingguard/      # 主程序
├── internal/
│   ├── agent/          # 核心代理
│   ├── providers/      # LLM 提供商
│   ├── channels/       # 消息渠道
│   └── tools/          # 内置工具
├── pkg/
│   ├── memory/         # 记忆系统
│   └── embedding/      # 向量嵌入
├── skills/             # 内置技能
├── scripts/            # 安装脚本
└── configs/            # 配置文件
```

## 服务管理

### Linux (systemd)

```bash
make install                    # 安装并启动服务
systemctl --user status lingguard
systemctl --user restart lingguard
journalctl --user -u lingguard -f
```

### macOS (launchd)

```bash
make install                    # 安装并启动服务
launchctl list | grep lingguard
launchctl unload ~/Library/LaunchAgents/com.lingguard.plist   # 停止
launchctl load ~/Library/LaunchAgents/com.lingguard.plist     # 启动
```

## 文档

- [架构文档](docs/ARCHITECTURE.md)
- [API 文档](docs/API.md)

## License

MIT
