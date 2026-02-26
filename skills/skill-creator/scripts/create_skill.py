#!/usr/bin/env python3
"""
Skill Creator - 创建新的 skill 骨架
用途：根据模板生成新的 skill 目录和文件
"""

import argparse
import os
import sys
import re
from datetime import datetime
from pathlib import Path

SCRIPTS_DIR = Path(__file__).parent
TEMPLATES_DIR = SCRIPTS_DIR.parent / "templates"

class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    BLUE = '\033[0;34m'
    NC = '\033[0m'

TEMPLATE_TYPES = {
    'basic': 'basic.md',
    'with-script': 'with-script.md',
    'advanced': 'advanced.md'
}

DEFAULT_VALUES = {
    'EMOJI': '🔧',
    'CODE_LANGUAGE': 'bash',
    'SKILL_TITLE': 'My Skill',
    'SKILL_SUMMARY': '这是一个自定义 skill。',
    'TRIGGER_LIST': '- 触发词1\n- 触发词2\n- 触发词3',
    'FEATURES': '功能说明待补充。',
    'PARAM_TABLE': '| param | 参数说明 | default |',
    'BASIC_USAGE_EXAMPLE': '# 使用示例',
    'EXAMPLE1_TITLE': '基本示例',
    'EXAMPLE1_DESCRIPTION': '基本用法演示。',
    'EXAMPLE1_CODE': '# 示例代码',
    'EXAMPLE2_TITLE': '高级示例',
    'EXAMPLE2_DESCRIPTION': '高级用法演示。',
    'EXAMPLE2_CODE': '# 高级示例代码',
    'NOTE1': '注意事项1',
    'NOTE2': '注意事项2',
    'NOTE3': '注意事项3',
    'RELATED_DOCS': '无',
    'REFERENCES': '无',
    'BASIC_ARGS': '--help',
    'FULL_ARGS': '--param value --option',
    'EXAMPLE1_COMMAND': 'python3 scripts/main.py --example',
    'EXAMPLE1_OUTPUT': '示例输出',
    'EXAMPLE2_COMMAND': 'python3 scripts/main.py --advanced',
    'EXAMPLE2_OUTPUT': '高级示例输出',
    'WORKFLOW_STEP1': '第一步',
    'WORKFLOW_CMD1': 'command1',
    'WORKFLOW_STEP2': '第二步',
    'WORKFLOW_CMD2': 'command2',
    'WORKFLOW_STEP3': '第三步',
    'WORKFLOW_CMD3': 'command3',
    'SCRIPT_TABLE': '| main.py | 主脚本 |',
    'ERROR_TABLE': "| 错误信息 | 原因 | 解决方案 |",
    'ENV_TABLE': '| ENV_VAR | 环境变量说明 | default |',
    'NOTICE1_TITLE': '注意1',
    'NOTICE1_CONTENT': '- 详细说明',
    'NOTICE2_TITLE': '注意2',
    'NOTICE2_CONTENT': '- 详细说明',
    'NOTICE3_TITLE': '注意3',
    'NOTICE3_CONTENT': '- 详细说明',
    'DATE': datetime.now().strftime('%Y-%m-%d'),
}

def validate_skill_name(name: str) -> bool:
    if not name:
        return False
    if not re.match(r'^[a-z][a-z0-9-]*$', name):
        return False
    if name.startswith('-') or name.endswith('-'):
        return False
    if '--' in name:
        return False
    return True

def load_template(template_type: str) -> str:
    template_file = TEMPLATES_DIR / TEMPLATE_TYPES.get(template_type, 'basic.md')
    if not template_file.exists():
        print(f"{Colors.RED}❌ 模板文件不存在: {template_file}{Colors.NC}")
        sys.exit(1)
    return template_file.read_text(encoding='utf-8')

def replace_placeholders(content: str, values: dict) -> str:
    for key, value in values.items():
        placeholder = f"{{{{{key}}}}}"
        content = content.replace(placeholder, value)
    return content

def create_basic_script(skill_dir: Path, skill_name: str):
    scripts_dir = skill_dir / "scripts"
    scripts_dir.mkdir(exist_ok=True)
    
    main_script = scripts_dir / "main.py"
    script_content = f'''#!/usr/bin/env python3
"""
{skill_name} - 主脚本
"""

import argparse
import sys

class Colors:
    RED = '\\033[0;31m'
    GREEN = '\\033[0;32m'
    YELLOW = '\\033[1;33m'
    NC = '\\033[0m'

def main():
    parser = argparse.ArgumentParser(description='{skill_name} 脚本')
    parser.add_argument('--param', '-p', help='参数说明')
    parser.add_argument('--verbose', '-v', action='store_true', help='详细输出')
    args = parser.parse_args()
    
    print(f"{{Colors.GREEN}}✅ {skill_name} 执行成功{{Colors.NC}}")
    if args.param:
        print(f"参数: {{args.param}}")

if __name__ == '__main__':
    main()
'''
    main_script.write_text(script_content, encoding='utf-8')
    main_script.chmod(0o755)
    print(f"{Colors.GREEN}✅ 创建脚本: {main_script}{Colors.NC}")

def create_advanced_structure(skill_dir: Path, skill_name: str):
    scripts_dir = skill_dir / "scripts"
    scripts_dir.mkdir(exist_ok=True)
    
    create_basic_script(skill_dir, skill_name)
    
    utils_script = scripts_dir / "utils.py"
    utils_content = '''#!/usr/bin/env python3
"""
工具函数
"""

class Colors:
    RED = '\\033[0;31m'
    GREEN = '\\033[0;32m'
    YELLOW = '\\033[1;33m'
    NC = '\\033[0m'

def print_success(msg: str):
    print(f"{Colors.GREEN}✅ {msg}{Colors.NC}")

def print_error(msg: str):
    print(f"{Colors.RED}❌ {msg}{Colors.NC}")

def print_warning(msg: str):
    print(f"{Colors.YELLOW}⚠️ {msg}{Colors.NC}")

def print_info(msg: str):
    print(f"{Colors.YELLOW}📌 {msg}{Colors.NC}")
'''
    utils_script.write_text(utils_content, encoding='utf-8')
    utils_script.chmod(0o755)
    
    reference = skill_dir / "reference.md"
    reference_content = f'''---
name: {skill_name}-reference
description: {skill_name} 技术参考文档
---

# {skill_name} 技术参考

## API 参考

### 主要函数

#### function_name

```python
def function_name(param1: str, param2: int) -> bool:
    """
    函数说明
    
    Args:
        param1: 参数1说明
        param2: 参数2说明
    
    Returns:
        返回值说明
    """
```

## 数据结构

### Config

```python
class Config:
    """配置类"""
    param1: str
    param2: int
```

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1 | 一般错误 |

## 扩展开发

### 添加新功能

1. 在 scripts/ 目录添加新脚本
2. 更新 SKILL.md 文档
3. 运行验证脚本
'''
    reference.write_text(reference_content, encoding='utf-8')
    
    examples = skill_dir / "examples.md"
    examples_content = f'''---
name: {skill_name}-examples
description: {skill_name} 使用示例
---

# {skill_name} 使用示例

## 示例1：基本使用

### 场景
基本使用场景描述。

### 命令
```bash
python3 scripts/main.py --param value
```

### 输出
```
示例输出
```

### 说明
详细说明...

## 示例2：高级使用

### 场景
高级使用场景描述。

### 命令
```bash
python3 scripts/main.py --advanced
```

### 输出
```
高级示例输出
```

### 说明
详细说明...
'''
    examples.write_text(examples_content, encoding='utf-8')
    
    print(f"{Colors.GREEN}✅ 创建参考文档: {reference}{Colors.NC}")
    print(f"{Colors.GREEN}✅ 创建示例文档: {examples}{Colors.NC}")

def create_skill(name: str, skill_type: str, output_dir: str, description: str, force: bool):
    if not validate_skill_name(name):
        print(f"{Colors.RED}❌ Skill 名称格式错误{Colors.NC}")
        print(f"{Colors.YELLOW}命名规则：{Colors.NC}")
        print("  - 小写字母开头")
        print("  - 只能包含小写字母、数字和连字符")
        print("  - 不能以连字符开头或结尾")
        print("  - 不能包含连续的连字符")
        print(f"{Colors.YELLOW}示例：my-skill, api-helper, code-gen{Colors.NC}")
        sys.exit(1)
    
    output_path = Path(output_dir)
    skill_dir = output_path / name
    
    if skill_dir.exists():
        if not force:
            print(f"{Colors.RED}❌ 目录已存在: {skill_dir}{Colors.NC}")
            print(f"{Colors.YELLOW}💡 使用 --force 覆盖已存在的文件{Colors.NC}")
            sys.exit(1)
        print(f"{Colors.YELLOW}⚠️ 目录已存在，将覆盖文件{Colors.NC}")
    
    skill_dir.mkdir(parents=True, exist_ok=True)
    print(f"{Colors.GREEN}✅ 创建目录: {skill_dir}{Colors.NC}")
    
    template_content = load_template(skill_type)
    
    values = DEFAULT_VALUES.copy()
    values.update({
        'SKILL_NAME': name,
        'SKILL_DESCRIPTION': description or f'{name} 自定义 skill',
        'SKILL_TITLE': name.replace('-', ' ').title(),
        'TRIGGER_WORDS': f'{name}、{name.replace("-", " ")}',
    })
    
    final_content = replace_placeholders(template_content, values)
    
    skill_file = skill_dir / "SKILL.md"
    skill_file.write_text(final_content, encoding='utf-8')
    print(f"{Colors.GREEN}✅ 创建文件: {skill_file}{Colors.NC}")
    
    if skill_type in ['with-script', 'advanced']:
        create_basic_script(skill_dir, name)
    
    if skill_type == 'advanced':
        create_advanced_structure(skill_dir, name)
    
    print(f"\n{Colors.BLUE}📦 Skill 创建完成！{Colors.NC}")
    print(f"{Colors.YELLOW}目录：{skill_dir}{Colors.NC}")
    print(f"\n{Colors.YELLOW}下一步：{Colors.NC}")
    print(f"  1. 编辑 {skill_dir}/SKILL.md")
    if skill_type != 'basic':
        print(f"  2. 编辑脚本文件")
    print(f"  {'3' if skill_type != 'basic' else '2'}. 运行验证: python3 scripts/validate_skill.py -p {skill_dir}")
    print(f"  {'4' if skill_type != 'basic' else '3'}. 安装 skill: cp -r {skill_dir} ~/.lingguard/skills/")

def main():
    parser = argparse.ArgumentParser(
        description='创建新的 skill 骨架',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
示例:
  # 创建基础 skill
  python3 create_skill.py -n my-skill -t basic
  
  # 创建带脚本的 skill
  python3 create_skill.py -n my-skill -t with-script -d "我的自定义技能"
  
  # 创建高级 skill
  python3 create_skill.py -n my-workflow -t advanced -o ./output
'''
    )
    parser.add_argument('--name', '-n', required=True, help='Skill 名称（小写字母和连字符）')
    parser.add_argument('--type', '-t', choices=['basic', 'with-script', 'advanced'], 
                        default='basic', help='模板类型（默认: basic）')
    parser.add_argument('--output', '-o', default='.', help='输出目录（默认: 当前目录）')
    parser.add_argument('--description', '-d', default='', help='Skill 描述')
    parser.add_argument('--force', '-f', action='store_true', help='覆盖已存在的文件')
    
    args = parser.parse_args()
    
    create_skill(
        name=args.name,
        skill_type=args.type,
        output_dir=args.output,
        description=args.description,
        force=args.force
    )

if __name__ == '__main__':
    main()
