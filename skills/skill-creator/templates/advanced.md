---
name: {{SKILL_NAME}}
description: {{SKILL_DESCRIPTION}}。当用户说"{{TRIGGER_WORDS}}"时使用
metadata: {"nanobot":{"emoji":"{{EMOJI}}","requires":{"bins":["python3","git"],"envs":["{{REQUIRED_ENV}}"]}}}
---

# {{SKILL_TITLE}}

{{SKILL_SUMMARY}}

## 触发关键词

{{TRIGGER_LIST}}

## 目录结构

```
{{SKILL_NAME}}/
├── SKILL.md              # 本文档
├── reference.md          # 技术参考
├── examples.md           # 使用示例
├── scripts/              # 脚本目录
│   ├── main.py           # 主脚本
│   ├── {{SCRIPT1}}.py    # {{SCRIPT1_DESC}}
│   └── utils.py          # 工具函数
└── templates/            # 模板目录
    └── {{TEMPLATE1}}     # {{TEMPLATE1_DESC}}
```

## 快速开始

### 安装依赖

```bash
{{INSTALL_COMMAND}}
```

### 基本用法

```bash
python3 scripts/main.py {{BASIC_ARGS}}
```

## 主要功能

### 1. {{FEATURE1_NAME}}

{{FEATURE1_DESC}}

```bash
{{FEATURE1_COMMAND}}
```

### 2. {{FEATURE2_NAME}}

{{FEATURE2_DESC}}

```bash
{{FEATURE2_COMMAND}}
```

### 3. {{FEATURE3_NAME}}

{{FEATURE3_DESC}}

```bash
{{FEATURE3_COMMAND}}
```

## 命令参考

### main.py

```bash
python3 scripts/main.py [command] [options]
```

**可用命令：**

| 命令 | 说明 |
|------|------|
{{MAIN_COMMANDS_TABLE}}

**全局选项：**

| 选项 | 说明 | 默认值 |
|------|------|--------|
{{GLOBAL_OPTIONS_TABLE}}

### {{SCRIPT1}}.py

```bash
python3 scripts/{{SCRIPT1}}.py [options]
```

| 选项 | 说明 | 默认值 |
|------|------|--------|
{{SCRIPT1_OPTIONS_TABLE}}

## 配置

### 环境变量

| 变量 | 必需 | 说明 | 默认值 |
|------|------|------|--------|
{{ENV_TABLE}}

### 配置文件

{{CONFIG_FILE_DESC}}

```json
{{CONFIG_EXAMPLE}}
```

## 使用示例

详细示例请参阅 [examples.md](./examples.md)

### 快速示例

```bash
# {{QUICK_EXAMPLE1_DESC}}
{{QUICK_EXAMPLE1_CMD}}

# {{QUICK_EXAMPLE2_DESC}}
{{QUICK_EXAMPLE2_CMD}}
```

## 工作流程

### {{WORKFLOW1_NAME}}

```
Step 1: {{WORKFLOW1_STEP1}}
├── {{WORKFLOW1_CMD1}}

Step 2: {{WORKFLOW1_STEP2}}
├── {{WORKFLOW1_CMD2}}

Step 3: {{WORKFLOW1_STEP3}}
└── {{WORKFLOW1_CMD3}}
```

### {{WORKFLOW2_NAME}}

```
Step 1: {{WORKFLOW2_STEP1}}
├── {{WORKFLOW2_CMD1}}

Step 2: {{WORKFLOW2_STEP2}}
└── {{WORKFLOW2_CMD2}}
```

## 模板说明

| 模板 | 用途 |
|------|------|
{{TEMPLATE_TABLE}}

## 故障排除

| 问题 | 原因 | 解决方案 |
|------|------|----------|
{{TROUBLESHOOT_TABLE}}

## API 参考

详细的 API 文档请参阅 [reference.md](./reference.md)

## 注意事项

1. **{{NOTICE1_TITLE}}**
   {{NOTICE1_CONTENT}}

2. **{{NOTICE2_TITLE}}**
   {{NOTICE2_CONTENT}}

3. **{{NOTICE3_TITLE}}**
   {{NOTICE3_CONTENT}}

## 更新日志

### v1.0.0 ({{DATE}})
- 初始版本
- {{CHANGELOG1}}
- {{CHANGELOG2}}

## 贡献指南

1. Fork 本项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 许可证

{{LICENSE}}
