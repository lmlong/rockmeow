package taskboard

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lingguard/internal/tools"
)

// TaskBoardTool 任务看板工具
type TaskBoardTool struct {
	service *Service
}

// NewTaskBoardTool 创建任务看板工具
func NewTaskBoardTool(service *Service) *TaskBoardTool {
	return &TaskBoardTool{service: service}
}

// Name 工具名称
func (t *TaskBoardTool) Name() string {
	return "taskboard"
}

// Description 工具描述
func (t *TaskBoardTool) Description() string {
	return `任务看板管理工具，用于创建、查询、更新和管理任务。

操作类型:
- list: 列出任务（可按状态、来源等过滤）
- get: 获取单个任务详情
- create: 创建新任务
- update: 更新任务信息
- delete: 删除任务
- status: 更新任务状态
- move: 移动任务到指定列
- assign: 分配任务
- stats: 获取统计信息
- board: 获取看板视图`
}

// Parameters 参数定义
func (t *TaskBoardTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "操作类型: list, get, create, update, delete, status, move, assign, stats, board",
				"enum":        []string{"list", "get", "create", "update", "delete", "status", "move", "assign", "stats", "board"},
			},
			"taskId": map[string]interface{}{
				"type":        "string",
				"description": "任务 ID（用于 get, update, delete, status, move, assign 操作）",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "任务标题（用于 create 操作）",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "任务描述（用于 create, update 操作）",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "任务状态: pending, running, completed, failed, cancelled",
				"enum":        []string{"pending", "running", "completed", "failed", "cancelled"},
			},
			"column": map[string]interface{}{
				"type":        "string",
				"description": "看板列: backlog, todo, in_progress, done",
				"enum":        []string{"backlog", "todo", "in_progress", "done"},
			},
			"priority": map[string]interface{}{
				"type":        "string",
				"description": "优先级: low, medium, high, urgent",
				"enum":        []string{"low", "medium", "high", "urgent"},
			},
			"assignee": map[string]interface{}{
				"type":        "string",
				"description": "分配对象（用于 assign 操作）",
			},
			"filter": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type":        "string",
						"description": "过滤状态",
					},
					"source": map[string]interface{}{
						"type":        "string",
						"description": "过滤来源: user, manual, subagent, cron",
					},
					"column": map[string]interface{}{
						"type":        "string",
						"description": "过滤列",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "返回数量限制",
					},
				},
			},
		},
		"required": []string{"action"},
	}
}

// Execute 执行工具
func (t *TaskBoardTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var args struct {
		Action      string      `json:"action"`
		TaskID      string      `json:"taskId"`
		Title       string      `json:"title"`
		Description string      `json:"description"`
		Status      string      `json:"status"`
		Column      string      `json:"column"`
		Priority    string      `json:"priority"`
		Assignee    string      `json:"assignee"`
		Filter      *TaskFilter `json:"filter"`
		Result      string      `json:"result"`
		Error       string      `json:"error"`
	}

	if err := json.Unmarshal(params, &args); err != nil {
		return "", fmt.Errorf("parse params: %w", err)
	}

	switch args.Action {
	case "list":
		return t.handleList(args.Filter)
	case "get":
		return t.handleGet(args.TaskID)
	case "create":
		return t.handleCreate(args)
	case "update":
		return t.handleUpdate(args)
	case "delete":
		return t.handleDelete(args.TaskID)
	case "status":
		return t.handleStatus(args.TaskID, args.Status)
	case "move":
		return t.handleMove(args.TaskID, args.Column)
	case "assign":
		return t.handleAssign(args.TaskID, args.Assignee)
	case "stats":
		return t.handleStats()
	case "board":
		return t.handleBoard()
	default:
		return "", fmt.Errorf("unknown action: %s", args.Action)
	}
}

// IsDangerous 是否危险操作
func (t *TaskBoardTool) IsDangerous() bool {
	return false
}

// Ensure interface implementation
var _ tools.Tool = (*TaskBoardTool)(nil)

// Handler methods

func (t *TaskBoardTool) handleList(filter *TaskFilter) (string, error) {
	tasks, err := t.service.ListTasks(filter)
	if err != nil {
		return "", err
	}

	if len(tasks) == 0 {
		return "没有找到任务", nil
	}

	result := fmt.Sprintf("找到 %d 个任务:\n\n", len(tasks))
	for i, task := range tasks {
		result += fmt.Sprintf("%d. [%s] %s (状态: %s, 来源: %s)\n",
			i+1, task.ID, task.Title, task.Status, task.Source)
	}

	return result, nil
}

func (t *TaskBoardTool) handleGet(taskID string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskId is required")
	}

	task, err := t.service.GetTask(taskID)
	if err != nil {
		return "", err
	}

	result := fmt.Sprintf("任务详情:\n")
	result += fmt.Sprintf("- ID: %s\n", task.ID)
	result += fmt.Sprintf("- 标题: %s\n", task.Title)
	if task.Description != "" {
		result += fmt.Sprintf("- 描述: %s\n", task.Description)
	}
	result += fmt.Sprintf("- 状态: %s\n", task.Status)
	result += fmt.Sprintf("- 优先级: %s\n", task.Priority)
	result += fmt.Sprintf("- 列: %s\n", task.Column)
	if task.Assignee != "" {
		result += fmt.Sprintf("- 分配给: %s (%s)\n", task.Assignee, task.AssigneeType)
	}
	result += fmt.Sprintf("- 来源: %s\n", task.Source)
	if task.SessionID != "" {
		result += fmt.Sprintf("- 会话 ID: %s\n", task.SessionID)
	}
	result += fmt.Sprintf("- 创建时间: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	if task.StartedAt != nil {
		result += fmt.Sprintf("- 开始时间: %s\n", task.StartedAt.Format("2006-01-02 15:04:05"))
	}
	if task.CompletedAt != nil {
		result += fmt.Sprintf("- 完成时间: %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
	}
	if task.Result != "" {
		result += fmt.Sprintf("- 结果: %s\n", task.Result)
	}
	if task.Error != "" {
		result += fmt.Sprintf("- 错误: %s\n", task.Error)
	}

	return result, nil
}

func (t *TaskBoardTool) handleCreate(args struct {
	Action      string      `json:"action"`
	TaskID      string      `json:"taskId"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      string      `json:"status"`
	Column      string      `json:"column"`
	Priority    string      `json:"priority"`
	Assignee    string      `json:"assignee"`
	Filter      *TaskFilter `json:"filter"`
	Result      string      `json:"result"`
	Error       string      `json:"error"`
}) (string, error) {
	if args.Title == "" {
		return "", fmt.Errorf("title is required")
	}

	task := &Task{
		Title:       args.Title,
		Description: args.Description,
		Source:      TaskSourceManual,
	}

	if args.Status != "" {
		task.Status = TaskStatus(args.Status)
	}
	if args.Column != "" {
		task.Column = Column(args.Column)
	}
	if args.Priority != "" {
		task.Priority = Priority(args.Priority)
	}
	if args.Assignee != "" {
		task.Assignee = args.Assignee
		task.AssigneeType = AssigneeTypeAgent
	}

	if err := t.service.CreateTask(task); err != nil {
		return "", err
	}

	return fmt.Sprintf("任务创建成功: [%s] %s", task.ID, task.Title), nil
}

func (t *TaskBoardTool) handleUpdate(args struct {
	Action      string      `json:"action"`
	TaskID      string      `json:"taskId"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Status      string      `json:"status"`
	Column      string      `json:"column"`
	Priority    string      `json:"priority"`
	Assignee    string      `json:"assignee"`
	Filter      *TaskFilter `json:"filter"`
	Result      string      `json:"result"`
	Error       string      `json:"error"`
}) (string, error) {
	if args.TaskID == "" {
		return "", fmt.Errorf("taskId is required")
	}

	task, err := t.service.GetTask(args.TaskID)
	if err != nil {
		return "", err
	}

	if args.Title != "" {
		task.Title = args.Title
	}
	if args.Description != "" {
		task.Description = args.Description
	}
	if args.Priority != "" {
		task.Priority = Priority(args.Priority)
	}

	if err := t.service.UpdateTask(task); err != nil {
		return "", err
	}

	return fmt.Sprintf("任务更新成功: [%s] %s", task.ID, task.Title), nil
}

func (t *TaskBoardTool) handleDelete(taskID string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskId is required")
	}

	if err := t.service.DeleteTask(taskID); err != nil {
		return "", err
	}

	return fmt.Sprintf("任务已删除: %s", taskID), nil
}

func (t *TaskBoardTool) handleStatus(taskID, status string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskId is required")
	}
	if status == "" {
		return "", fmt.Errorf("status is required")
	}

	switch TaskStatus(status) {
	case TaskStatusRunning:
		if err := t.service.StartTask(taskID); err != nil {
			return "", err
		}
	case TaskStatusCompleted:
		if err := t.service.CompleteTask(taskID, ""); err != nil {
			return "", err
		}
	case TaskStatusFailed:
		if err := t.service.FailTask(taskID, ""); err != nil {
			return "", err
		}
	case TaskStatusCancelled:
		if err := t.service.CancelTask(taskID); err != nil {
			return "", err
		}
	default:
		if err := t.service.UpdateStatus(taskID, TaskStatus(status)); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("任务状态已更新: %s -> %s", taskID, status), nil
}

func (t *TaskBoardTool) handleMove(taskID, column string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskId is required")
	}
	if column == "" {
		return "", fmt.Errorf("column is required")
	}

	if err := t.service.MoveToColumn(taskID, Column(column)); err != nil {
		return "", err
	}

	return fmt.Sprintf("任务已移动: %s -> %s", taskID, column), nil
}

func (t *TaskBoardTool) handleAssign(taskID, assignee string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskId is required")
	}
	if assignee == "" {
		return "", fmt.Errorf("assignee is required")
	}

	if err := t.service.AssignTask(taskID, assignee, AssigneeTypeAgent); err != nil {
		return "", err
	}

	return fmt.Sprintf("任务已分配: %s -> %s", taskID, assignee), nil
}

func (t *TaskBoardTool) handleStats() (string, error) {
	stats, err := t.service.GetStats()
	if err != nil {
		return "", err
	}

	result := "任务看板统计:\n\n"
	result += fmt.Sprintf("- 总任务数: %d\n", stats.Total)

	result += "\n按状态:\n"
	for status, count := range stats.ByStatus {
		result += fmt.Sprintf("  - %s: %d\n", status, count)
	}

	result += "\n按来源:\n"
	for source, count := range stats.BySource {
		result += fmt.Sprintf("  - %s: %d\n", source, count)
	}

	result += "\n按列:\n"
	for column, count := range stats.ByColumn {
		result += fmt.Sprintf("  - %s: %d\n", column, count)
	}

	return result, nil
}

func (t *TaskBoardTool) handleBoard() (string, error) {
	board, err := t.service.GetBoard()
	if err != nil {
		return "", err
	}

	result := "任务看板:\n\n"

	result += fmt.Sprintf("📋 待办 (%d):\n", len(board.Todo))
	for _, task := range board.Todo {
		result += fmt.Sprintf("  - [%s] %s\n", task.ID, task.Title)
	}

	result += fmt.Sprintf("\n🔄 进行中 (%d):\n", len(board.InProgress))
	for _, task := range board.InProgress {
		result += fmt.Sprintf("  - [%s] %s\n", task.ID, task.Title)
	}

	result += fmt.Sprintf("\n✅ 已完成 (%d):\n", len(board.Done))
	for _, task := range board.Done {
		result += fmt.Sprintf("  - [%s] %s\n", task.ID, task.Title)
	}

	result += fmt.Sprintf("\n📦 待定 (%d):\n", len(board.Backlog))
	for _, task := range board.Backlog {
		result += fmt.Sprintf("  - [%s] %s\n", task.ID, task.Title)
	}

	return result, nil
}
