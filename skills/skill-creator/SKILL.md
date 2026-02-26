---
name: skill-creator
description: 创建、验证和管理自定义 skill。当用户说"创建skill"、"新建skill"、"制作skill"、"开发skill"、"编写skill"、"生成skill"时使用
metadata: {"nanobot":{"emoji":"🛠️","requires":{"bins":["python3"]}}}
---

# Skill Creator

帮助用户创建、验证和管理自定义 skill。

## 触发关键词

- "创建skill"、"新建skill"
- "制作skill"、"开发skill"
- "编写skill"、"生成skill"
- "如何写一个skill"
- "skill怎么写"

## 快速开始

### 1. 创建新 Skill

**重要**：新创建的 skill 存放在 `~/.lingguard/workspace/skills/` 目录

```bash
python3 ~/.lingguard/skills/skill-creator/scripts/create_skill.py \
  --name my-skill \
  --type basic \
  --output ~/.lingguard/workspace/skills/
```

**参数说明：**
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--name, -n` | Skill 名称（必填） | - |
| `--type, -t` | 模板类型: basic/with-script/advanced | basic |
| `--output, -o` | 输出目录 | 当前目录 |
| `--description, -d` | Skill 描述 | - |
| `--force, -f` | 覆盖已存在的文件 | false |

### 2. 验证 Skill 格式

```bash
python3 ~/.lingguard/skills/skill-creator/scripts/validate_skill.py \
  --path ~/.lingguard/workspace/skills/my-skill
```

**参数说明：**
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--path, -p` | Skill 目录路径（必填） | - |
| `--strict, -s` | 严格模式（检查所有推荐项） | false |

### 3. 编辑和测试

创建后直接编辑 `~/.lingguard/workspace/skills/my-skill/SKILL.md`，新会话自动加载。

## 使用流程

```
Step 1: 创建 skill
├── python3 ~/.lingguard/skills/skill-creator/scripts/create_skill.py \
│   -n my-skill -t with-script -o ~/.lingguard/workspace/skills/

Step 2: 编辑 SKILL.md 和脚本
├── 修改描述、触发关键词
├── 编写功能逻辑

Step 3: 验证格式（可选）
├── python3 ~/.lingguard/skills/skill-creator/scripts/validate_skill.py \
│   -p ~/.lingguard/workspace/skills/my-skill

Step 4: 开始新会话
└── 新 skill 自动加载
```

## Skill 类型选择

| 类型 | 适用场景 | 包含文件 |
|------|----------|----------|
| **basic** | 简单说明型 skill | SKILL.md |
| **with-script** | 需要执行脚本 | SKILL.md, scripts/ |
| **advanced** | 复杂多功能 skill | SKILL.md, reference.md, examples.md, scripts/, templates/ |

## 模板示例

### Basic 模板
适用于纯文档说明型 skill，如：
- 代码规范指南
- API 文档查询
- 配置说明

### With-Script 模板
适用于需要执行操作的 skill，如：
- 代码生成
- 文件处理
- 自动化任务

### Advanced 模板
适用于复杂 skill，如：
- 完整的开发工作流
- 多步骤操作
- 需要多种资源文件

## 最佳实践

1. **命名规范**
   - 使用小写字母和连字符: `my-skill`
   - 避免特殊字符和空格
   - 名称应简洁且描述性强

2. **描述编写**
   - 清晰说明功能
   - 包含触发关键词
   - 说明所需依赖

3. **触发关键词**
   - 覆盖用户可能的表达方式
   - 包含中英文关键词
   - 考虑同义词

4. **文档结构**
   - SKILL.md: 主要用法和快速开始
   - reference.md: 详细技术参考
   - examples.md: 使用示例

## 文件说明

| 文件 | 用途 |
|------|------|
| `SKILL.md` | 主文档，必需 |
| `reference.md` | 技术参考文档 |
| `examples.md` | 使用示例 |
| `scripts/*.py` | 可执行脚本 |
| `templates/*.md` | 内部使用的模板 |

## 注意事项

- **创建位置**：新 skill 创建在 `~/.lingguard/workspace/skills/`，新会话自动加载
- 修改 skill 后需要重新开始会话
- Skill 名称冲突时，用户 skill 会覆盖内置 skill
- 使用 `--force` 参数可以覆盖已存在的 skill

## Skill 存放位置

| 位置 | 说明 |
|------|------|
| `~/.lingguard/skills/` | 内置 skill |
| `~/.lingguard/workspace/skills/` | 用户 skill 目录，新 skill 存放这里 |
