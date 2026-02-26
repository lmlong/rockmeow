#!/usr/bin/env python3
"""
Skill Validator - 验证 skill 格式
用途：检查 skill 目录结构和文件格式是否正确
"""

import argparse
import os
import re
import sys
from pathlib import Path
from typing import List, Tuple

class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    BLUE = '\033[0;34m'
    NC = '\033[0m'

class ValidationResult:
    def __init__(self):
        self.errors: List[str] = []
        self.warnings: List[str] = []
        self.infos: List[str] = []
    
    def error(self, msg: str):
        self.errors.append(msg)
        print(f"{Colors.RED}❌ {msg}{Colors.NC}")
    
    def warning(self, msg: str):
        self.warnings.append(msg)
        print(f"{Colors.YELLOW}⚠️ {msg}{Colors.NC}")
    
    def info(self, msg: str):
        self.infos.append(msg)
        print(f"{Colors.GREEN}✅ {msg}{Colors.NC}")
    
    def is_valid(self) -> bool:
        return len(self.errors) == 0

def parse_yaml_front_matter(content: str) -> Tuple[dict, str]:
    if not content.startswith('---'):
        return {}, content
    
    parts = content.split('---', 2)
    if len(parts) < 3:
        return {}, content
    
    yaml_content = parts[1].strip()
    body = parts[2].strip()
    
    metadata = {}
    for line in yaml_content.split('\n'):
        if ':' in line:
            key, value = line.split(':', 1)
            key = key.strip()
            value = value.strip().strip('"\'')
            
            if value.startswith('{'):
                continue
            
            metadata[key] = value
    
    return metadata, body

def validate_name(name: str) -> bool:
    if not name:
        return False
    if not re.match(r'^[a-z][a-z0-9-]*$', name):
        return False
    if name.startswith('-') or name.endswith('-'):
        return False
    if '--' in name:
        return False
    return True

def validate_skill_md(skill_dir: Path, result: ValidationResult) -> dict:
    skill_file = skill_dir / "SKILL.md"
    
    if not skill_file.exists():
        result.error("SKILL.md 文件不存在")
        return {}
    
    result.info("SKILL.md 文件存在")
    
    try:
        content = skill_file.read_text(encoding='utf-8')
    except Exception as e:
        result.error(f"无法读取 SKILL.md: {e}")
        return {}
    
    if not content.strip():
        result.error("SKILL.md 文件为空")
        return {}
    
    metadata, body = parse_yaml_front_matter(content)
    
    if not metadata:
        result.error("SKILL.md 缺少 YAML front matter")
        return {}
    
    result.info("YAML front matter 格式正确")
    
    if 'name' not in metadata:
        result.error("缺少必需的 'name' 字段")
    elif not validate_name(metadata['name']):
        result.error(f"'name' 字段格式错误: {metadata['name']}")
    else:
        result.info(f"name 字段有效: {metadata['name']}")
    
    if 'description' not in metadata:
        result.error("缺少必需的 'description' 字段")
    elif not metadata['description'].strip():
        result.error("'description' 字段为空")
    else:
        result.info("description 字段存在")
    
    if len(body) < 50:
        result.warning("文档内容较短，建议添加更多说明")
    else:
        result.info(f"文档内容长度: {len(body)} 字符")
    
    return metadata

def validate_structure(skill_dir: Path, result: ValidationResult):
    expected_files = ['SKILL.md']
    recommended_files = ['reference.md', 'examples.md']
    
    for f in expected_files:
        path = skill_dir / f
        if path.exists():
            result.info(f"必需文件存在: {f}")
        else:
            result.error(f"缺少必需文件: {f}")
    
    for f in recommended_files:
        path = skill_dir / f
        if path.exists():
            result.info(f"推荐文件存在: {f}")
        else:
            result.warning(f"建议添加: {f}")
    
    scripts_dir = skill_dir / "scripts"
    if scripts_dir.exists() and scripts_dir.is_dir():
        result.info("scripts 目录存在")
        
        scripts = list(scripts_dir.glob("*.py"))
        if scripts:
            result.info(f"发现 {len(scripts)} 个 Python 脚本")
            
            for script in scripts:
                if os.access(script, os.X_OK):
                    result.info(f"脚本有执行权限: {script.name}")
                else:
                    result.warning(f"脚本缺少执行权限: {script.name}")
        else:
            result.warning("scripts 目录为空")
    else:
        result.info("无 scripts 目录（可选）")
    
    templates_dir = skill_dir / "templates"
    if templates_dir.exists() and templates_dir.is_dir():
        templates = list(templates_dir.glob("*"))
        if templates:
            result.info(f"发现 {len(templates)} 个模板文件")
        else:
            result.warning("templates 目录为空")

def validate_content(skill_dir: Path, result: ValidationResult, strict: bool):
    skill_file = skill_dir / "SKILL.md"
    if not skill_file.exists():
        return
    
    content = skill_file.read_text(encoding='utf-8')
    _, body = parse_yaml_front_matter(content)
    
    sections = {
        '触发关键词': ['触发关键词', '触发词', 'Trigger', 'Keywords'],
        '用法': ['用法', 'Usage', '使用方法'],
        '示例': ['示例', 'Example', '例子'],
        '参数': ['参数', 'Parameter', 'Arguments'],
        '注意': ['注意', 'Note', '注意事项', 'Warning'],
    }
    
    for section, keywords in sections.items():
        found = any(kw in body for kw in keywords)
        if found:
            result.info(f"包含 {section} 部分")
        elif strict:
            result.warning(f"建议添加 {section} 部分")
    
    code_blocks = re.findall(r'```(\w+)?', body)
    if code_blocks:
        result.info(f"包含 {len(code_blocks)} 个代码块")
    elif strict:
        result.warning("建议添加代码示例")
    
    tables = re.findall(r'\|.*\|', body)
    if tables:
        result.info(f"包含表格内容")

def validate_skill(path: str, strict: bool) -> bool:
    skill_dir = Path(path)
    
    if not skill_dir.exists():
        print(f"{Colors.RED}❌ 目录不存在: {path}{Colors.NC}")
        return False
    
    if not skill_dir.is_dir():
        print(f"{Colors.RED}❌ 不是目录: {path}{Colors.NC}")
        return False
    
    print(f"{Colors.BLUE}🔍 验证 Skill: {skill_dir.name}{Colors.NC}")
    print(f"{Colors.YELLOW}{'='*50}{Colors.NC}\n")
    
    result = ValidationResult()
    
    print(f"{Colors.BLUE}📄 检查 SKILL.md...{Colors.NC}")
    metadata = validate_skill_md(skill_dir, result)
    print()
    
    print(f"{Colors.BLUE}📁 检查目录结构...{Colors.NC}")
    validate_structure(skill_dir, result)
    print()
    
    print(f"{Colors.BLUE}📝 检查内容...{Colors.NC}")
    validate_content(skill_dir, result, strict)
    print()
    
    print(f"{Colors.YELLOW}{'='*50}{Colors.NC}")
    print(f"{Colors.BLUE}📊 验证结果:{Colors.NC}")
    print(f"  {Colors.GREEN}✅ 通过: {len(result.infos)}{Colors.NC}")
    print(f"  {Colors.YELLOW}⚠️ 警告: {len(result.warnings)}{Colors.NC}")
    print(f"  {Colors.RED}❌ 错误: {len(result.errors)}{Colors.NC}")
    
    if result.is_valid():
        print(f"\n{Colors.GREEN}🎉 Skill 验证通过！{Colors.NC}")
        return True
    else:
        print(f"\n{Colors.RED}❌ Skill 验证失败，请修复上述错误{Colors.NC}")
        return False

def main():
    parser = argparse.ArgumentParser(
        description='验证 skill 格式',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
示例:
  # 验证 skill
  python3 validate_skill.py -p ./my-skill
  
  # 严格模式验证
  python3 validate_skill.py -p ./my-skill --strict
  
  # 验证多个 skill
  python3 validate_skill.py -p ./skill1 ./skill2
'''
    )
    parser.add_argument('--path', '-p', required=True, nargs='+', help='Skill 目录路径')
    parser.add_argument('--strict', '-s', action='store_true', help='严格模式（检查所有推荐项）')
    
    args = parser.parse_args()
    
    all_valid = True
    for path in args.path:
        if not validate_skill(path, args.strict):
            all_valid = False
        print()
    
    sys.exit(0 if all_valid else 1)

if __name__ == '__main__':
    main()
