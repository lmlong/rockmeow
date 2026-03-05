package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/lingguard/internal/skills"
	"github.com/lingguard/pkg/logger"
)

// SkillTool 技能加载工具
type SkillTool struct {
	skillsMgr *skills.Manager
	registry  *Registry
	mu        sync.RWMutex
}

// NewSkillTool 创建技能工具
func NewSkillTool(mgr *skills.Manager) *SkillTool {
	return &SkillTool{
		skillsMgr: mgr,
	}
}

// SetRegistry 设置工具注册表
func (t *SkillTool) SetRegistry(registry *Registry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.registry = registry
}

func (t *SkillTool) Name() string { return "skill" }

func (t *SkillTool) Description() string {
	return `加载指定技能的完整指令和工具定义。

## 触发条件

- 图像/视频生成 → aigc
- 网络搜索 → web
- 代码分析/优化 → coding
- git 下载/上传 → git-sync
- 文件操作 → file
- 定时任务 → cron

调用方式：skill --name <技能名>`
}

func (t *SkillTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "技能名称 (e.g., 'aigc', 'web', 'coding')",
			},
		},
		"required": []string{"name"},
	}
}

// skillToToolMapping skill 名称到工具名称的映射
var skillToToolMapping = map[string][]string{
	"aigc":        {"aigc"},
	"coding":      {"opencode"},
	"git-sync":    {"shell", "file"},
	"web":         {"web_search", "web_fetch"},
	"code-review": {"opencode"},
	"weather":     {"web_search"},
	"file":        {"file"},
	"system":      {"shell"},
	"cron":        {"cron_add", "cron_list", "cron_remove"},
	"tts":         {"tts"},
}

// skillResponse skill 工具返回格式
type skillResponse struct {
	Content string                   `json:"content"`
	Tools   []map[string]interface{} `json:"tools,omitempty"`
}

func (t *SkillTool) Execute(ctx context.Context, argsJSON json.RawMessage) (string, error) {
	if t.skillsMgr == nil {
		return "", fmt.Errorf("skills manager not initialized")
	}

	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Name == "" {
		return "", fmt.Errorf("skill name is required")
	}

	logger.Info("===== [Skill] Loading skill =====", "name", args.Name)

	// 获取技能指令
	instruction, err := t.skillsMgr.GetSkillInstruction(args.Name)
	if err != nil {
		logger.Error("[Skill] Failed to load skill", "name", args.Name, "error", err)
		return "", fmt.Errorf("failed to load skill '%s': %w", args.Name, err)
	}

	logger.Info("[Skill] Instruction loaded", "name", args.Name, "length", len(instruction))

	// 构建返回结果
	response := skillResponse{
		Content: instruction,
	}

	// 获取关联的工具定义
	t.mu.RLock()
	registry := t.registry
	t.mu.RUnlock()

	if registry != nil {
		if toolNames, ok := skillToToolMapping[args.Name]; ok {
			logger.Info("[Skill] Mapping tools", "skill", args.Name, "tools", toolNames)

			toolDefs := registry.GetToolDefinitionsByNames(toolNames)
			if len(toolDefs) > 0 {
				response.Tools = toolDefs
				logger.Info("===== [Skill] Tools attached =====", "skill", args.Name, "tool_count", len(toolDefs))

				// 记录每个工具的名称
				for _, def := range toolDefs {
					if fn, ok := def["function"].(map[string]interface{}); ok {
						if name, ok := fn["name"].(string); ok {
							logger.Info("[Skill] Tool definition included", "skill", args.Name, "tool", name)
						}
					}
				}
			} else {
				logger.Warn("[Skill] No tool definitions found", "skill", args.Name, "tools", toolNames)
			}
		} else {
			logger.Debug("[Skill] No tool mapping for skill", "name", args.Name)
		}
	} else {
		logger.Warn("[Skill] Registry not set, cannot attach tools")
	}

	// 返回 JSON 格式
	result, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	logger.Info("===== [Skill] Load complete =====", "name", args.Name, "result_length", len(result))

	return string(result), nil
}

func (t *SkillTool) IsDangerous() bool { return false }

func (t *SkillTool) ShouldLoadByDefault() bool { return true }
