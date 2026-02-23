package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TasksBoardConfig 任务看板配置
type TasksBoardConfig struct {
	URL    string `json:"url"`    // 看板 API 地址，如 http://localhost:3000/api/tasks
	APIKey string `json:"apiKey"` // API Key（可选）
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"   // 待办
	TaskStatusRunning   TaskStatus = "running"   // 进行中
	TaskStatusCompleted TaskStatus = "completed" // 已完成
	TaskStatusFailed    TaskStatus = "failed"    // 失败
)

// TaskAssignee 任务分配者
type TaskAssignee string

const (
	TaskAssigneeUser TaskAssignee = "user" // 用户
	TaskAssigneeAI   TaskAssignee = "ai"   // AI
	TaskAssigneeBoth TaskAssignee = "both" // 协作
)

// TaskPriority 任务优先级
type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
)

// Task 任务定义
type Task struct {
	ExternalID  string        `json:"externalId,omitempty"`  // 外部 ID（用于同步）
	Title       string        `json:"title"`                 // 标题
	Description string        `json:"description,omitempty"` // 描述
	Status      TaskStatus    `json:"status"`                // 状态
	Assignee    TaskAssignee  `json:"assignee,omitempty"`    // 分配者
	SessionID   string        `json:"sessionId,omitempty"`   // 会话 ID
	SubagentID  string        `json:"subagentId,omitempty"`  // 子代理 ID
	Priority    TaskPriority  `json:"priority,omitempty"`    // 优先级
	Tags        []string      `json:"tags,omitempty"`        // 标签
	Result      string        `json:"result,omitempty"`      // 结果
	Error       string        `json:"error,omitempty"`       // 错误信息
	StartedAt   *int64        `json:"startedAt,omitempty"`   // 开始时间
	CompletedAt *int64        `json:"completedAt,omitempty"` // 完成时间
	Metadata    *TaskMetadata `json:"metadata,omitempty"`    // 元数据
}

// TaskMetadata 任务元数据
type TaskMetadata struct {
	Source           string `json:"source,omitempty"`           // 来源
	Command          string `json:"command,omitempty"`          // 命令
	WorkingDirectory string `json:"workingDirectory,omitempty"` // 工作目录
}

// TasksBoardTool 任务看板同步工具
type TasksBoardTool struct {
	config     *TasksBoardConfig
	httpClient *http.Client
}

// NewTasksBoardTool 创建任务看板工具
func NewTasksBoardTool(config *TasksBoardConfig) *TasksBoardTool {
	if config == nil {
		config = &TasksBoardConfig{}
	}
	return &TasksBoardTool{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TasksBoardTool) Name() string {
	return "tasks_board"
}

func (t *TasksBoardTool) Description() string {
	return `任务看板同步工具，用于创建、更新和查询任务。

操作类型:
- create: 创建新任务
- update: 更新任务状态
- get: 查询任务列表
- sync: 批量同步任务

任务状态: pending(待办), running(进行中), completed(已完成), failed(失败)
分配者: user(用户), ai(AI助手), both(协作)
优先级: low(低), medium(中), high(高)`
}

func (t *TasksBoardTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"create", "update", "get", "sync"},
				"description": "操作类型: create(创建), update(更新), get(查询), sync(批量同步)",
			},
			"task": map[string]interface{}{
				"type":        "object",
				"description": "任务对象（用于 create/update）",
				"properties": map[string]interface{}{
					"externalId":  map[string]interface{}{"type": "string", "description": "外部唯一标识"},
					"title":       map[string]interface{}{"type": "string", "description": "任务标题"},
					"description": map[string]interface{}{"type": "string", "description": "任务描述"},
					"status":      map[string]interface{}{"type": "string", "enum": []string{"pending", "running", "completed", "failed"}},
					"assignee":    map[string]interface{}{"type": "string", "enum": []string{"user", "ai", "both"}},
					"priority":    map[string]interface{}{"type": "string", "enum": []string{"low", "medium", "high"}},
					"tags":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"result":      map[string]interface{}{"type": "string", "description": "任务结果"},
					"error":       map[string]interface{}{"type": "string", "description": "错误信息"},
				},
			},
			"tasks": map[string]interface{}{
				"type":        "array",
				"description": "任务数组（用于 sync 批量同步）",
				"items":       map[string]interface{}{"$ref": "#/properties/task"},
			},
			"taskId": map[string]interface{}{
				"type":        "string",
				"description": "任务ID（用于 update）",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "筛选状态（用于 get 查询）",
				"enum":        []string{"pending", "running", "completed", "failed"},
			},
		},
		"required": []string{"action"},
	}
}

func (t *TasksBoardTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	if t.config.URL == "" {
		return "", fmt.Errorf("任务看板 URL 未配置，请在配置文件中设置 tools.tasksBoard.url")
	}

	var p struct {
		Action string     `json:"action"`
		Task   *Task      `json:"task"`
		Tasks  []*Task    `json:"tasks"`
		TaskID string     `json:"taskId"`
		Status TaskStatus `json:"status"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}

	switch p.Action {
	case "create":
		return t.createTask(ctx, p.Task)
	case "update":
		return t.updateTask(ctx, p.TaskID, p.Task)
	case "get":
		return t.getTasks(ctx, p.Status)
	case "sync":
		return t.syncTasks(ctx, p.Tasks)
	default:
		return "", fmt.Errorf("未知操作类型: %s", p.Action)
	}
}

func (t *TasksBoardTool) IsDangerous() bool {
	return false
}

// createTask 创建任务
func (t *TasksBoardTool) createTask(ctx context.Context, task *Task) (string, error) {
	if task == nil {
		return "", fmt.Errorf("任务数据不能为空")
	}
	if task.Title == "" {
		return "", fmt.Errorf("任务标题不能为空")
	}

	// 设置默认值
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if task.Assignee == "" {
		task.Assignee = TaskAssigneeAI
	}

	resp, err := t.doRequest(ctx, "POST", task)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return t.readResponse(resp)
}

// updateTask 更新任务
func (t *TasksBoardTool) updateTask(ctx context.Context, taskID string, task *Task) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("任务ID不能为空")
	}
	if task == nil {
		return "", fmt.Errorf("任务数据不能为空")
	}

	// 构建更新请求
	updateReq := map[string]interface{}{
		"id": taskID,
	}
	if task.Status != "" {
		updateReq["status"] = task.Status
	}
	if task.Result != "" {
		updateReq["result"] = task.Result
	}
	if task.Error != "" {
		updateReq["error"] = task.Error
	}

	resp, err := t.doRequest(ctx, "PATCH", updateReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return t.readResponse(resp)
}

// getTasks 获取任务列表
func (t *TasksBoardTool) getTasks(ctx context.Context, status TaskStatus) (string, error) {
	url := t.config.URL
	if status != "" {
		url = fmt.Sprintf("%s?status=%s", url, status)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	t.setHeaders(req)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	return t.readResponse(resp)
}

// syncTasks 批量同步任务
func (t *TasksBoardTool) syncTasks(ctx context.Context, tasks []*Task) (string, error) {
	if len(tasks) == 0 {
		return "", fmt.Errorf("任务列表不能为空")
	}

	syncReq := map[string]interface{}{
		"tasks": tasks,
	}

	resp, err := t.doRequest(ctx, "POST", syncReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return t.readResponse(resp)
}

// doRequest 执行 HTTP 请求
func (t *TasksBoardTool) doRequest(ctx context.Context, method string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, t.config.URL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	t.setHeaders(req)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}

	return resp, nil
}

// setHeaders 设置请求头
func (t *TasksBoardTool) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if t.config.APIKey != "" {
		req.Header.Set("X-API-Key", t.config.APIKey)
	}
}

// readResponse 读取响应
func (t *TasksBoardTool) readResponse(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("请求失败 (%d): %s", resp.StatusCode, string(body))
	}

	// 格式化 JSON 输出
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
		return prettyJSON.String(), nil
	}

	return string(body), nil
}
