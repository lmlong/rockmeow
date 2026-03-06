# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

LingGuard 是一个基于 Go 语言的超轻量级个人 AI 智能助手。核心特性包括多 LLM 支持、飞书/QQ 集成、工具系统、技能系统（渐进式加载）和记忆系统。单二进制部署，约 20MB 内存。

## 常用命令

```bash
# 构建
make build

# 开发模式运行
make dev

# 运行测试
make test

# 带覆盖率的测试
make test-coverage

# 格式化代码
make fmt

# 静态检查
make lint

# 打包发布
make package            # Linux + macOS
make package-all        # 所有平台

# 安装到系统
make install
```

## 架构

```
lingguard/
├── cmd/lingguard/          # 主程序入口
├── internal/
│   ├── agent/              # 核心代理逻辑（Agent 结构、工具调用循环）
│   ├── providers/          # LLM 提供商实现（OpenAI 兼容 API）
│   ├── channels/           # 消息渠道（飞书 WebSocket、QQ Gateway）
│   ├── tools/              # 内置工具（shell、file、web、aigc、tts 等）
│   ├── skills/             # 技能系统（渐进式加载管理器）
│   ├── session/            # 会话管理
│   ├── cron/               # 定时任务
│   └── subagent/           # 子代理系统
├── pkg/
│   ├── llm/                # LLM 请求/响应类型定义
│   ├── memory/             # 记忆系统（文件存储、向量检索）
│   ├── embedding/          # 向量嵌入
│   └── stream/             # SSE 流式响应处理
├── skills/                 # 内置技能定义（每个子目录一个 SKILL.md）
└── configs/                # 配置文件模板
```

## 核心组件

### Agent (internal/agent/agent.go)

Agent 是核心代理，负责：
- 管理 Provider（主 Provider + 可选的多模态 Provider）
- 工具调用循环（最大迭代次数由 `maxToolIterations` 控制）
- 会话级别动态工具管理（含老化机制，连续 10 次请求未使用则卸载）
- 记忆上下文构建

### Provider Registry (internal/providers/)

Provider 自动匹配机制优先级：
1. 解析 `provider/model` 格式
2. 直接匹配 Provider 名称
3. 关键词匹配（gpt → openai, claude → anthropic）
4. API Key 前缀匹配（sk-or- → openrouter, gsk_ → groq）
5. 默认 Provider

每个 Provider 由 `spec.go` 中的 `ProviderSpec` 定义默认值（apiBase、model 等），config.json 可覆盖。

### 技能系统 (internal/skills/)

渐进式加载策略：
1. 默认只注入技能摘要到系统提示
2. LLM 通过 `skill` 工具按需加载详细指令
3. 技能可声明所需的动态工具（如 `aigc`、`web_search`），加载时动态注入

### 工具系统 (internal/tools/)

工具分为两类：
- **默认工具**：始终加载（skill、memory、message 等）
- **动态工具**：按需加载（aigc、web_search、opencode 等）

### 记忆系统 (pkg/memory/)

- **Auto-Recall**：对话开始时自动召回相关记忆（向量 + BM25 混合检索）
- **Auto-Capture**：对话结束时自动捕获重要内容
- 配置在 `config.json` 的 `agents.memory` 节点

## 配置

配置文件位置（优先级从高到低）：
1. 环境变量 `$LINGGUARD_CONFIG`
2. 用户目录 `~/.lingguard/config.json`

关键配置项：
- `agents.provider`：主 LLM 提供商
- `agents.multimodalProvider`：多模态提供商（可选，用于图片/视频输入）
- `agents.maxToolIterations`：最大工具调用迭代次数
- `agents.memory.enabled`：是否启用持久化记忆

## 开发注意事项

### 添加新 Provider

1. 在 `internal/providers/spec.go` 的 `PROVIDERS` 切片中添加 `ProviderSpec`
2. 在 `config.json` 中配置 `apiKey`

### 添加新工具

1. 在 `internal/tools/` 下创建工具文件
2. 实现 `Execute` 方法
3. 在 `registry.go` 中注册

### 添加新技能

1. 在 `skills/` 下创建目录
2. 创建 `SKILL.md` 文件，包含 `---` 分隔的 YAML frontmatter（name、description、tools）
3. 可选：添加 `reference.md`、`examples.md` 等参考文件

## 文档

- [架构文档](docs/ARCHITECTURE.md) - 详细的架构设计
- [API 文档](docs/API.md) - Provider API、工具 API、渠道 API
- [OpenCode HTTP API](docs/OPENCODE_HTTP_API.md) - OpenCode 集成 API