---
name: skill-creator-reference
description: Skill 文件格式和技术参考文档
---

# Skill 技术参考

本文档详细说明 Skill 的文件格式、元数据配置和最佳实践。

## Skill 目录结构

### 基础结构
```
my-skill/
└── SKILL.md              # 必需：主文档
```

### 标准结构
```
my-skill/
├── SKILL.md              # 必需：主文档
├── reference.md          # 可选：技术参考
├── examples.md           # 可选：使用示例
└── scripts/              # 可选：脚本目录
    └── main.py
```

### 完整结构
```
my-skill/
├── SKILL.md              # 主文档
├── reference.md          # 技术参考
├── examples.md           # 使用示例
├── scripts/              # 脚本目录
│   ├── main.py
│   └── utils.py
└── templates/            # 模板目录
    └── template.md
```

## SKILL.md 格式

### YAML Front Matter

```yaml
---
name: skill-name                    # 必需：skill 唯一标识
description: 描述文本               # 必需：用于匹配用户意图
metadata:                           # 可选：元数据配置
  nanobot:
    emoji: "🛠️"                     # 显示图标
    always: false                   # 是否始终加载
    requires:                       # 依赖检查
      bins: ["python3", "git"]      # 需要的命令
      envs: ["API_KEY"]             # 需要的环境变量
      files: ["config.json"]        # 需要的文件
---
```

### 元数据字段说明

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | Skill 唯一标识符，小写字母和连字符 |
| `description` | string | ✅ | 功能描述，用于意图匹配 |
| `metadata.nanobot.emoji` | string | ❌ | 显示图标 |
| `metadata.nanobot.always` | bool | ❌ | 是否始终加载（默认 false） |
| `metadata.nanobot.requires.bins` | []string | ❌ | 需要的可执行命令 |
| `metadata.nanobot.requires.envs` | []string | ❌ | 需要的环境变量 |
| `metadata.nanobot.requires.files` | []string | ❌ | 需要的文件 |

### 文档主体结构

```markdown
# Skill 标题

简短的功能介绍。

## 触发关键词

- 关键词1
- 关键词2

## 用法

### 命令1

```bash
command here
```

### 命令2

```bash
another command
```

## 参数说明

| 参数 | 说明 | 默认值 |
|------|------|--------|
| param1 | 描述 | default |

## 示例

### 示例1

描述...

```bash
example command
```

## 注意事项

- 注意点1
- 注意点2
```

## 触发关键词最佳实践

### 中文关键词
```yaml
description: 创建、验证和管理自定义 skill。当用户说"创建skill"、"新建skill"、"制作skill"时使用
```

### 英文关键词
```yaml
description: Create, validate and manage custom skills. Use when user says "create skill", "new skill", "make skill"
```

### 中英混合
```yaml
description: |
  创建、验证和管理自定义 skill。
  触发词：创建skill、新建skill、制作skill、开发skill
  Triggers: create skill, new skill, make skill
```

## 脚本开发规范

### Python 脚本模板

```python
#!/usr/bin/env python3
"""
脚本用途说明
"""

import argparse
import sys
import os

# 配置
DEFAULT_VALUE = "default"

class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    NC = '\033[0m'

def main():
    parser = argparse.ArgumentParser(description='脚本描述')
    parser.add_argument('--param', '-p', help='参数说明')
    args = parser.parse_args()
    
    # 主要逻辑
    print(f"{Colors.GREEN}✅ 操作成功{Colors.NC}")

if __name__ == '__main__':
    main()
```

### 脚本规范要点

1. **Shebang**: 使用 `#!/usr/bin/env python3`
2. **参数解析**: 使用 `argparse`
3. **错误处理**: 提供清晰的错误信息
4. **退出码**: 成功返回 0，失败返回非 0
5. **输出格式**: 使用颜色区分信息类型

## Skill 加载机制

### 加载顺序
1. 系统内置 skills (`~/.lingguard/skills/builtin/`)
2. 用户自定义 skills (`~/.lingguard/skills/`)
3. ClawHub 安装的 skills

### 名称冲突
- 后加载的 skill 会覆盖先加载的
- 用户 skill 优先级高于内置 skill

### 热重载
- 修改 skill 后需要重新开始会话
- 不支持运行时热重载

## 验证规则

### 必需检查
- [ ] SKILL.md 文件存在
- [ ] YAML front matter 格式正确
- [ ] name 字段存在且格式正确
- [ ] description 字段存在

### 推荐检查
- [ ] description 包含触发关键词
- [ ] 有用法说明部分
- [ ] 有示例代码
- [ ] 脚本有执行权限
- [ ] 脚本有错误处理

### 格式检查
- [ ] Markdown 格式正确
- [ ] 代码块有语言标识
- [ ] 表格格式正确

## 常见问题

### Q: Skill 不生效？
1. 检查文件是否在正确目录
2. 检查 SKILL.md 格式是否正确
3. 重新开始会话

### Q: 触发关键词不匹配？
1. 确保 description 包含关键词
2. 使用多种表达方式
3. 考虑中英文混合

### Q: 脚本执行失败？
1. 检查脚本权限 (`chmod +x`)
2. 检查依赖是否安装
3. 检查路径是否正确

## 发布到 ClawHub

```bash
# 安装 ClawHub CLI
npm install -g clawhub

# 登录
clawhub login

# 发布
clawhub publish ./my-skill
```

### 发布要求
- SKILL.md 格式正确
- 有完整的文档
- 包含使用示例
- 通过格式验证
