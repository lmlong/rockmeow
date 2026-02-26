---
name: {{SKILL_NAME}}
description: {{SKILL_DESCRIPTION}}。当用户说"{{TRIGGER_WORDS}}"时使用
metadata: {"nanobot":{"emoji":"{{EMOJI}}","requires":{"bins":["python3"]}}}
---

# {{SKILL_TITLE}}

{{SKILL_SUMMARY}}

## 触发关键词

{{TRIGGER_LIST}}

## 快速开始

### 1. 基本命令

```bash
python3 scripts/main.py {{BASIC_ARGS}}
```

### 2. 完整参数

```bash
python3 scripts/main.py {{FULL_ARGS}}
```

## 参数说明

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
{{PARAM_TABLE}}

## 使用示例

### 示例1：{{EXAMPLE1_TITLE}}

```bash
{{EXAMPLE1_COMMAND}}
```

**输出：**
```
{{EXAMPLE1_OUTPUT}}
```

### 示例2：{{EXAMPLE2_TITLE}}

```bash
{{EXAMPLE2_COMMAND}}
```

**输出：**
```
{{EXAMPLE2_OUTPUT}}
```

## 典型工作流

```
Step 1: {{WORKFLOW_STEP1}}
├── {{WORKFLOW_CMD1}}

Step 2: {{WORKFLOW_STEP2}}
├── {{WORKFLOW_CMD2}}

Step 3: {{WORKFLOW_STEP3}}
└── {{WORKFLOW_CMD3}}
```

## 脚本说明

| 脚本 | 用途 |
|------|------|
{{SCRIPT_TABLE}}

## 错误处理

| 错误信息 | 原因 | 解决方案 |
|----------|------|----------|
{{ERROR_TABLE}}

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
{{ENV_TABLE}}

## 注意事项

1. **{{NOTICE1_TITLE}}**
   - {{NOTICE1_CONTENT}}

2. **{{NOTICE2_TITLE}}**
   - {{NOTICE2_CONTENT}}

3. **{{NOTICE3_TITLE}}**
   - {{NOTICE3_CONTENT}}

## 文件说明

- `SKILL.md` - 本文档
- `reference.md` - 技术参考文档
- `scripts/main.py` - 主脚本
- `scripts/utils.py` - 工具函数
