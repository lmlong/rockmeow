// Package tools 工具系统
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lingguard/internal/skills"
	"github.com/lingguard/pkg/logger"
)

// SkillTool 技能加载工具
type SkillTool struct {
	skillsMgr *skills.Manager
}

// NewSkillTool 创建技能工具
func NewSkillTool(mgr *skills.Manager) *SkillTool {
	return &SkillTool{
		skillsMgr: mgr,
	}
}

// Name 返回工具名称
func (t *SkillTool) Name() string {
	return "skill"
}

// Description 返回工具描述
func (t *SkillTool) Description() string {
	return `加载指定技能的完整指令。

## 触发条件

- **代码分析/优化** → coding（使用 opencode 工具）
- **git 下载/上传** → git-sync（使用 shell 工具执行脚本）
- 图像/视频生成 → aigc
- 网络搜索 → web
- 代码审查 → code-review
- 天气查询 → weather
- 文件操作 → file
- 系统命令 → system
- 定时任务 → cron

## 多任务流程

"下载代码，分析优化，并上库"：
1. skill git-sync → shell 执行下载脚本
2. skill coding → opencode 分析优化代码
3. skill git-sync → shell 执行上传脚本

调用方式：skill --name <技能名>`
}

// Parameters 返回工具参数定义
func (t *SkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the skill to load (e.g., 'git-sync', 'code-review')",
			},
		},
		"required": []string{"name"},
	}
}

// Execute 执行工具
func (t *SkillTool) Execute(ctx context.Context, argsJSON json.RawMessage) (string, error) {
	if t.skillsMgr == nil {
		return "", fmt.Errorf("skills manager not initialized")
	}

	// 解析参数
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Name == "" {
		return "", fmt.Errorf("skill name is required")
	}

	logger.Info("Loading skill", "name", args.Name)

	// 获取技能指令
	instruction, err := t.skillsMgr.GetSkillInstruction(args.Name)
	if err != nil {
		logger.Error("Failed to load skill", "name", args.Name, "error", err)
		return "", fmt.Errorf("failed to load skill '%s': %w", args.Name, err)
	}

	logger.Info("Skill loaded successfully", "name", args.Name, "length", len(instruction))

	// 在指令前添加执行提示
	executorPrompt := `## ⚠️ 必须立即执行

加载此 skill 后，你必须立即调用相应的工具执行操作！

**禁止行为**：
- ❌ 只返回文本说明
- ❌ 不执行任何工具调用

---

`

	return executorPrompt + instruction, nil
}

// IsDangerous 返回是否为危险操作
func (t *SkillTool) IsDangerous() bool {
	return false
}

// ShouldLoadByDefault 返回是否默认加载（元工具，必须加载）
func (t *SkillTool) ShouldLoadByDefault() bool {
	return true
}
