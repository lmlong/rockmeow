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
	return `**必须使用**：加载指定技能的完整指令。

触发条件（必须先调用此工具）：
- coding 任务：编写、编辑、分析、优化代码
- git 操作：下载代码、上传代码、git clone、git push
- 代码审查：review 代码
- 文件操作：读写文件
- 系统操作：执行系统命令

调用方式：skill --name <技能名>
可用技能：coding, git-workflow, code-review, file, system

返回完整的技能指令，包含具体操作步骤和示例。`
}

// Parameters 返回工具参数定义
func (t *SkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the skill to load (e.g., 'git-workflow', 'code-review')",
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
	return instruction, nil
}

// IsDangerous 返回是否为危险操作
func (t *SkillTool) IsDangerous() bool {
	return false
}
