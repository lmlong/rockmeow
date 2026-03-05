package subagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lingguard/internal/tools"
)

// TaskTool 后台任务启动工具
type TaskTool struct {
	manager *SubagentManager
}

// NewTaskTool 创建任务工具
func NewTaskTool(manager *SubagentManager) *TaskTool {
	return &TaskTool{
		manager: manager,
	}
}

func (t *TaskTool) Name() string { return "task" }

func (t *TaskTool) Description() string {
	return `启动后台子代理执行复杂任务。

## 🔴 严格使用规则

**只有在以下场景才使用 task 工具**：
- 一个请求包含**多个独立的复杂子任务**（如：同时修改多个模块的代码）
- 需要**并行执行**多个长时间任务
- 用户**明确要求后台执行**的任务

## 🚫 禁止使用 task 的场景（直接调用工具）

| 场景 | 正确做法 |
|------|----------|
| 生成图片/视频 | 直接调用 aigc 工具 |
| 语音合成 | 直接调用 tts 工具 |
| 网络搜索 | 直接调用 web_search 工具 |
| 单个代码修改 | 直接使用 shell/file 工具 |
| git 操作 | 加载 skill 后直接执行 shell |
| 简单问答 | 直接回复用户 |

## ⚠️ 重要提示

- 使用 task 会增加开销，简单的任务直接执行更快
- 子代理可以使用所有工具（除了 task，防止无限嵌套）
- 返回 task_id，可用 task_status 查询进度和结果`
}

func (t *TaskTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "Clear description of the task to perform. Be specific about what needs to be done.",
			},
			"context": map[string]interface{}{
				"type":        "string",
				"description": "Additional context or background information for the task. Include any relevant details, constraints, or preferences.",
			},
		},
		"required": []string{"task"},
	}
}

func (t *TaskTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Task    string `json:"task"`
		Context string `json:"context"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if p.Task == "" {
		return "", fmt.Errorf("task is required")
	}

	// 创建并启动子代理
	sub, err := t.manager.Spawn(ctx, p.Task, p.Context)
	if err != nil {
		return "", fmt.Errorf("failed to spawn subagent: %w", err)
	}

	// 返回任务信息
	result := map[string]interface{}{
		"task_id": sub.ID(),
		"status":  "started",
		"message": "Task started in background. Use 'task_status' tool with the task_id to check progress and get results.",
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

func (t *TaskTool) IsDangerous() bool { return false }

func (t *TaskTool) ShouldLoadByDefault() bool { return true }

// TaskStatusTool 任务状态查询工具
type TaskStatusTool struct {
	manager *SubagentManager
}

// NewTaskStatusTool 创建任务状态查询工具
func NewTaskStatusTool(manager *SubagentManager) *TaskStatusTool {
	return &TaskStatusTool{
		manager: manager,
	}
}

func (t *TaskStatusTool) Name() string { return "task_status" }

func (t *TaskStatusTool) Description() string {
	return `Check the status and result of a background task.

Use this tool to check if a previously started task has completed and get its results.
The task must have been started using the 'task' tool.

## ⚠️ 轮询策略

**重要**: 子代理任务通常需要较长时间执行（代码分析、优化等）。

- **不要频繁轮询**: 每次调用 task_status 后，请等待至少 10-15 秒再检查
- **不要阻塞等待**: 在等待子代理完成时，主代理可以并行执行其他独立任务
- **合理预期**: 复杂任务（如代码优化）通常需要 1-3 分钟完成

示例工作流:
1. 启动 task → 获得 task_id
2. 执行其他独立任务（如 git status、查看其他文件等）
3. 等待一段时间后检查 task_status
4. 如果仍在运行，继续其他工作或等待后再检查`
}

func (t *TaskStatusTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the task to check (returned by the task tool)",
			},
			"list": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, list all tasks instead of checking a specific one",
			},
		},
	}
}

func (t *TaskStatusTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		TaskID string `json:"task_id"`
		List   bool   `json:"list"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	// 列出所有任务
	if p.List {
		return t.listTasks()
	}

	// 查询特定任务
	if p.TaskID == "" {
		return "", fmt.Errorf("task_id is required (or use list=true to see all tasks)")
	}

	return t.getTaskStatus(p.TaskID)
}

func (t *TaskStatusTool) listTasks() (string, error) {
	summaries := t.manager.ListSummaries()

	result := map[string]interface{}{
		"count": len(summaries),
		"tasks": summaries,
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

func (t *TaskStatusTool) getTaskStatus(taskID string) (string, error) {
	sub, exists := t.manager.GetStatus(taskID)
	if !exists {
		return "", fmt.Errorf("task not found: %s", taskID)
	}

	result := map[string]interface{}{
		"id":        sub.ID(),
		"task":      sub.Task(),
		"status":    sub.Status(),
		"summary":   sub.GetSummary(),
		"toolCalls": sub.GetToolCalls(), // 添加工具调用历史
	}

	// 如果任务完成，包含结果
	if sub.Status() == StatusCompleted {
		result["result"] = sub.Result()
	}

	// 如果任务失败，包含错误信息
	if sub.Status() == StatusFailed {
		result["error"] = sub.Error()
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return string(resultJSON), nil
}

func (t *TaskStatusTool) IsDangerous() bool { return false }

func (t *TaskStatusTool) ShouldLoadByDefault() bool { return true }

// 确保实现了 Tool 接口
var _ tools.Tool = (*TaskTool)(nil)
var _ tools.Tool = (*TaskStatusTool)(nil)
