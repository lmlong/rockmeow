// Package skills 技能系统
package skills

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lingguard/pkg/logger"
)

// Skill 技能定义
type Skill struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Homepage    string                 `json:"homepage,omitempty"`
	Emoji       string                 `json:"emoji,omitempty"`
	Always      bool                   `json:"always,omitempty"` // 是否始终加载完整内容
	Content     string                 `json:"content,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Requires    *Requirements          `json:"requires,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Available   bool                   `json:"available"`
	Unavailable string                 `json:"unavailable,omitempty"`
}

// Requirements 技能依赖要求
type Requirements struct {
	Bins []string `json:"bins,omitempty"`
	Env  []string `json:"env,omitempty"`
}

// Loader 技能加载器
type Loader struct {
	builtinDirs    []string // 支持多个内置技能目录
	workspace      string
	disabledSkills []string // 禁用的技能列表
}

// NewLoader 创建技能加载器
// builtinDirs 支持传入多个内置技能目录，按顺序优先级加载
// disabledSkills 指定需要禁用的技能名称列表
func NewLoader(builtinDirs []string, workspace string, disabledSkills ...[]string) *Loader {
	var disabled []string
	if len(disabledSkills) > 0 {
		disabled = disabledSkills[0]
	}
	return &Loader{
		builtinDirs:    builtinDirs,
		workspace:      workspace,
		disabledSkills: disabled,
	}
}

// isDisabled 检查技能是否被禁用
func (l *Loader) isDisabled(name string) bool {
	for _, disabled := range l.disabledSkills {
		if disabled == name {
			return true
		}
	}
	return false
}

// ListSkills 列出所有可用技能
func (l *Loader) ListSkills() ([]*Skill, error) {
	skills := make([]*Skill, 0)
	seen := make(map[string]bool) // 用于去重

	// 加载所有内置技能目录
	for _, dir := range l.builtinDirs {
		if dir == "" {
			continue
		}
		builtinSkills, err := l.loadFromDir(dir)
		if err != nil {
			logger.Warn("Failed to load builtin skills", "dir", dir, "error", err)
			continue
		}
		// 去重：只添加未 seen 的技能，并过滤禁用的技能
		for _, s := range builtinSkills {
			if !seen[s.Name] && !l.isDisabled(s.Name) {
				seen[s.Name] = true
				skills = append(skills, s)
			}
		}
	}

	// 加载工作区技能
	if l.workspace != "" {
		workspaceSkills, err := l.loadFromDir(l.workspace)
		if err != nil {
			logger.Warn("failed to load workspace skills", "error", err)
		}
		// 去重：工作区技能可以覆盖内置技能，并过滤禁用的技能
		for _, s := range workspaceSkills {
			if !seen[s.Name] && !l.isDisabled(s.Name) {
				seen[s.Name] = true
				skills = append(skills, s)
			}
		}
	}

	// 如果没有加载到任何技能，记录警告
	if len(skills) == 0 {
		logger.Warn("No skills loaded! Check skill directories configuration",
			"builtinDirs", l.builtinDirs,
			"workspace", l.workspace)
	} else {
		logger.Info("Skills loaded", "count", len(skills))
	}

	return skills, nil
}

// LoadSkill 加载指定技能的完整内容
func (l *Loader) LoadSkill(name string) (*Skill, error) {
	// 先在所有 builtin 目录查找（按顺序）
	for _, dir := range l.builtinDirs {
		if dir == "" {
			continue
		}
		skill, err := l.loadSkillByName(dir, name)
		if err == nil {
			return skill, nil
		}
	}

	// 再在 workspace 目录查找
	if l.workspace != "" {
		skill, err := l.loadSkillByName(l.workspace, name)
		if err == nil {
			return skill, nil
		}
	}

	return nil, fmt.Errorf("skill not found: %s", name)
}

// loadSkillByName 从指定目录加载技能
func (l *Loader) loadSkillByName(dir, name string) (*Skill, error) {
	skillPath := filepath.Join(dir, name, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, err
	}

	skill, err := parseSkill(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill %s: %w", name, err)
	}

	skill.Path = skillPath
	// Content 已在 parseSkill 中设置为去掉 frontmatter 的正文
	skill.Available, skill.Unavailable = l.checkRequirements(skill.Requires)

	return skill, nil
}

// loadFromDir 从目录加载所有技能（仅元数据，不加载内容）
func (l *Loader) loadFromDir(dir string) ([]*Skill, error) {
	skills := make([]*Skill, 0)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		content, err := os.ReadFile(skillPath)
		if err != nil {
			// 记录跳过的目录，帮助调试配置问题
			logger.Debug("skipping directory, no SKILL.md", "path", skillPath)
			continue
		}

		skill, err := parseSkill(content)
		if err != nil {
			logger.Warn("failed to parse skill", "name", entry.Name(), "error", err)
			continue
		}

		skill.Path = skillPath
		skill.Available, skill.Unavailable = l.checkRequirements(skill.Requires)
		skills = append(skills, skill)
	}

	return skills, nil
}

// parseSkill 解析 SKILL.md 文件
func parseSkill(content []byte) (*Skill, error) {
	skill := &Skill{
		Metadata: make(map[string]interface{}),
	}

	// 解析 YAML frontmatter
	text := string(content)

	// 检查是否有 frontmatter
	if !strings.HasPrefix(text, "---") {
		return nil, fmt.Errorf("skill file must start with YAML frontmatter")
	}

	// 找到 frontmatter 结束位置
	endIndex := bytes.Index(content[3:], []byte("---"))
	if endIndex == -1 {
		return nil, fmt.Errorf("invalid frontmatter: missing closing ---")
	}

	frontmatter := content[4 : endIndex+3]
	body := content[endIndex+6:]

	// 解析 frontmatter
	lines := strings.Split(string(frontmatter), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			skill.Name = value
		case "description":
			skill.Description = value
		case "homepage":
			skill.Homepage = value
		case "metadata":
			// 解析 JSON metadata
			var metadata struct {
				Nanobot struct {
					Emoji    string        `json:"emoji"`
					Always   bool          `json:"always"`
					Requires *Requirements `json:"requires"`
				} `json:"nanobot"`
			}
			if err := json.Unmarshal([]byte(value), &metadata); err == nil {
				skill.Emoji = metadata.Nanobot.Emoji
				skill.Always = metadata.Nanobot.Always
				skill.Requires = metadata.Nanobot.Requires
			}
		}
	}

	// 存储完整内容
	skill.Content = string(body)

	return skill, nil
}

// checkRequirements 检查技能依赖是否满足
func (l *Loader) checkRequirements(req *Requirements) (bool, string) {
	if req == nil {
		return true, ""
	}

	// 检查二进制依赖
	for _, bin := range req.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			return false, fmt.Sprintf("missing binary: %s", bin)
		}
	}

	// 检查环境变量依赖
	for _, env := range req.Env {
		if os.Getenv(env) == "" {
			return false, fmt.Sprintf("missing environment variable: %s", env)
		}
	}

	return true, ""
}

// BuildSkillsSummary 构建技能摘要（用于注入到系统提示）
// 这是渐进式加载的默认策略：只注入摘要，LLM 可以通过 skill 工具按需加载完整内容
func (l *Loader) BuildSkillsSummary() string {
	skills, err := l.ListSkills()
	if err != nil || len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<skills>\n")
	sb.WriteString("## 可用技能列表\n\n")
	sb.WriteString("**重要：** 处理相关任务时，必须先使用 `skill --name <技能名>` 加载完整指令！\n\n")

	for _, skill := range skills {
		if !skill.Available {
			continue // 跳过不可用的技能
		}
		emoji := ""
		if skill.Emoji != "" {
			emoji = skill.Emoji + " "
		}
		sb.WriteString(fmt.Sprintf("### %s%s\n", emoji, skill.Name))
		sb.WriteString(fmt.Sprintf("%s\n", skill.Description))
		sb.WriteString(fmt.Sprintf("使用方式: `skill --name %s`\n\n", skill.Name))
	}

	sb.WriteString("</skills>")

	return sb.String()
}

// BuildSkillsContext 构建渐进式技能上下文
// 策略：
// - always=true 的技能：始终注入完整内容
// - 其他技能：只注入摘要，LLM 可通过 skill 工具按需加载
func (l *Loader) BuildSkillsContext() string {
	skills, err := l.ListSkills()
	if err != nil || len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<skills>\n")
	sb.WriteString("<!-- always=true 的技能已加载完整内容，其他技能请使用 skill 工具加载 -->\n\n")

	for _, skill := range skills {
		// 跳过不可用的技能
		if !skill.Available {
			continue
		}

		sb.WriteString(fmt.Sprintf("<skill name=\"%s\"", skill.Name))
		if skill.Emoji != "" {
			sb.WriteString(fmt.Sprintf(" emoji=\"%s\"", skill.Emoji))
		}

		// always=true 的技能加载完整内容
		if skill.Always {
			sb.WriteString(" always=\"true\">\n")
			// 加载完整内容
			fullSkill, err := l.LoadSkill(skill.Name)
			if err == nil && fullSkill.Content != "" {
				sb.WriteString(fullSkill.Content)
			} else {
				sb.WriteString(skill.Description)
			}
			sb.WriteString("\n</skill>\n\n")
		} else {
			// 其他技能只显示摘要
			sb.WriteString(">\n")
			sb.WriteString(fmt.Sprintf("<description>%s</description>\n", skill.Description))
			sb.WriteString("<!-- 使用 skill 工具加载此技能的完整指令 -->\n")
			sb.WriteString("</skill>\n\n")
		}
	}

	sb.WriteString("</skills>")

	return sb.String()
}

// GetAvailableSkills 获取可用的技能列表
func (l *Loader) GetAvailableSkills() ([]*Skill, error) {
	skills, err := l.ListSkills()
	if err != nil {
		return nil, err
	}

	available := make([]*Skill, 0)
	for _, s := range skills {
		if s.Available {
			available = append(available, s)
		}
	}

	return available, nil
}
