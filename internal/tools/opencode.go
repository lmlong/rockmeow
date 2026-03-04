// TODO(configuration): This file contains multiple hardcoded timeout and polling values:
// - time.Sleep(2 * time.Second) - server restart delay
// - time.Sleep(500 * time.Millisecond) - server health check interval
// - time.Sleep(100 * time.Millisecond) - SSE polling interval
// - consecutiveErrors backoff delays
// These should be moved to config.json under tools.opencode namespace:
// - tools.opencode.restartDelay
// - tools.opencode.healthCheckInterval
// - tools.opencode.pollInterval
// - tools.opencode.maxRetries
// Priority: P1 - Estimated effort: 1 day
// Related: #configuration #performance #opencode
//
// Package tools OpenCode HTTP API integration
package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lingguard/pkg/httpclient"
	"github.com/lingguard/pkg/logger"
)

// OpenCodeClient OpenCode HTTP API client
type OpenCodeClient struct {
	mu        sync.RWMutex
	baseURL   string
	client    *http.Client
	sessions  map[string]*OpenCodeSession
	sessionID string
	workspace string
}

// SSEEvent SSE event from OpenCode
type SSEEvent struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// OpenCodeSession OpenCode session info
type OpenCodeSession struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// OpenCodeConfig OpenCode client configuration
type OpenCodeConfig struct {
	BaseURL   string
	Timeout   time.Duration
	Workspace string // Working directory for OpenCode operations
	Enabled   bool   // Whether OpenCode server is enabled
}

// DefaultOpenCodeConfig returns default configuration
func DefaultOpenCodeConfig() *OpenCodeConfig {
	return &OpenCodeConfig{
		BaseURL:   "http://127.0.0.1:4096",
		Timeout:   300 * time.Second,
		Workspace: "",
		Enabled:   false,
	}
}

// NewOpenCodeClient creates a new OpenCode client
func NewOpenCodeClient(cfg *OpenCodeConfig) *OpenCodeClient {
	if cfg == nil {
		cfg = DefaultOpenCodeConfig()
	}
	return &OpenCodeClient{
		baseURL:   strings.TrimSuffix(cfg.BaseURL, "/"),
		client:    httpclient.WithCustomTimeout(cfg.Timeout),
		sessions:  make(map[string]*OpenCodeSession),
		workspace: cfg.Workspace,
	}
}

// StartServer starts OpenCode server in workspace directory
func (c *OpenCodeClient) StartServer(ctx context.Context) error {
	// Extract port from baseURL
	port := "4096"
	if parts := strings.Split(c.baseURL, ":"); len(parts) == 3 {
		port = parts[2]
	}

	// Check if already running with correct workspace
	healthy, _, err := c.Health(ctx)
	if err == nil && healthy {
		// Check if workspace matches by creating a test session
		resp, sessErr := c.doRequest(ctx, "POST", "/session", map[string]string{"title": "workspace-check"})
		if sessErr == nil {
			var session struct {
				Directory string `json:"directory"`
			}
			if json.Unmarshal(resp, &session) == nil {
				if session.Directory == c.workspace {
					logger.Debug("OpenCode server already running with correct workspace")
					return nil
				}
				// Workspace mismatch, need to restart
				logger.Warn("OpenCode workspace mismatch, restarting server",
					"expected", c.workspace, "current", session.Directory)
				// Kill existing server
				exec.Command("pkill", "-f", "opencode serve").Run()
				time.Sleep(2 * time.Second)
			}
		}
	}

	// Start opencode serve in workspace directory
	cmd := exec.CommandContext(ctx, "opencode", "serve", "--port", port)
	cmd.Dir = c.workspace
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start opencode server: %w", err)
	}

	logger.Info("OpenCode server starting", "port", port, "workspace", c.workspace)

	// Wait for server to be ready
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		healthy, _, _ = c.Health(ctx)
		if healthy {
			logger.Info("OpenCode server ready")
			return nil
		}
	}

	return fmt.Errorf("OpenCode server failed to start within 15 seconds")
}

// Health checks server health
func (c *OpenCodeClient) Health(ctx context.Context) (bool, string, error) {
	resp, err := c.doRequest(ctx, "GET", "/global/health", nil)
	if err != nil {
		return false, "", err
	}

	var result struct {
		Healthy bool   `json:"healthy"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return false, "", err
	}

	return result.Healthy, result.Version, nil
}

// CreateSession creates a new OpenCode session
func (c *OpenCodeClient) CreateSession(ctx context.Context, title string) (*OpenCodeSession, error) {
	body := map[string]string{"title": title}
	resp, err := c.doRequest(ctx, "POST", "/session", body)
	if err != nil {
		return nil, err
	}

	var session OpenCodeSession
	if err := json.Unmarshal(resp, &session); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.sessions[session.ID] = &session
	c.sessionID = session.ID
	c.mu.Unlock()

	logger.Info("OpenCode session created", "id", session.ID)
	return &session, nil
}

// GetOrCreateSession gets existing session or creates new one
func (c *OpenCodeClient) GetOrCreateSession(ctx context.Context, title string) (*OpenCodeSession, error) {
	c.mu.RLock()
	if c.sessionID != "" {
		if sess, ok := c.sessions[c.sessionID]; ok {
			c.mu.RUnlock()
			return sess, nil
		}
	}
	c.mu.RUnlock()

	return c.CreateSession(ctx, title)
}

// MessagePart message part for OpenCode API
type MessagePart struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	Path       string `json:"path,omitempty"`
	StartLine  int    `json:"startLine,omitempty"`
	EndLine    int    `json:"endLine,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Result     string `json:"result,omitempty"`
	ToolCallID string `json:"toolCallID,omitempty"`
}

// SendMessageOptions options for sending message
type SendMessageOptions struct {
	Model   string        // model ID (e.g., "anthropic/claude-sonnet-4-20250514")
	Agent   string        // agent name (e.g., "build", "plan")
	Parts   []MessagePart // message parts
	NoReply bool          // don't wait for response
	System  string        // custom system prompt
	Tools   []string      // allowed tools
}

// MessageResponse response from sending message
type MessageResponse struct {
	Info struct {
		ID               string          `json:"id"`
		Role             string          `json:"role"`
		ModelID          string          `json:"modelID"`
		ProviderID       string          `json:"providerID"`
		Agent            string          `json:"agent"`
		StructuredOutput json.RawMessage `json:"structured_output,omitempty"`
	} `json:"info"`
	Parts []MessageResponsePart `json:"parts"`
}

// MessageResponsePart part of message response
type MessageResponsePart struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Snapshot string `json:"snapshot,omitempty"`
	Time     struct {
		Start    int64 `json:"start,omitempty"`
		End      int64 `json:"end,omitempty"`
		Duration int64 `json:"duration,omitempty"`
	} `json:"time,omitempty"`
}

// SendMessage sends a message to OpenCode session
func (c *OpenCodeClient) SendMessage(ctx context.Context, sessionID string, opts SendMessageOptions) (*MessageResponse, error) {
	reqBody := map[string]interface{}{
		"parts": opts.Parts,
	}

	if opts.Model != "" {
		parts := strings.SplitN(opts.Model, "/", 2)
		if len(parts) == 2 {
			reqBody["model"] = map[string]string{
				"providerID": parts[0],
				"modelID":    parts[1],
			}
		}
	}

	if opts.Agent != "" {
		reqBody["agent"] = opts.Agent
	}

	if opts.NoReply {
		reqBody["noReply"] = true
	}

	if opts.System != "" {
		reqBody["system"] = opts.System
	}

	if len(opts.Tools) > 0 {
		reqBody["tools"] = opts.Tools
	}

	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/session/%s/message", sessionID), reqBody)
	if err != nil {
		return nil, err
	}

	var result MessageResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ExecuteCommand executes a slash command in OpenCode
func (c *OpenCodeClient) ExecuteCommand(ctx context.Context, sessionID, command string) (*MessageResponse, error) {
	reqBody := map[string]interface{}{
		"command": command,
		"agent":   "build",
	}

	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/session/%s/command", sessionID), reqBody)
	if err != nil {
		return nil, err
	}

	var result MessageResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ShellResult result from shell command
type ShellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// ExecuteShell executes a shell command via OpenCode
func (c *OpenCodeClient) ExecuteShell(ctx context.Context, sessionID, command string) (*ShellResult, error) {
	reqBody := map[string]interface{}{
		"command": command,
		"agent":   "build",
	}

	resp, err := c.doRequest(ctx, "POST", fmt.Sprintf("/session/%s/shell", sessionID), reqBody)
	if err != nil {
		return nil, err
	}

	var result ShellResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// AbortSession aborts a running session
func (c *OpenCodeClient) AbortSession(ctx context.Context, sessionID string) error {
	_, err := c.doRequest(ctx, "POST", fmt.Sprintf("/session/%s/abort", sessionID), nil)
	return err
}

// DeleteSession deletes a session
func (c *OpenCodeClient) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/session/%s", sessionID), nil)

	c.mu.Lock()
	delete(c.sessions, sessionID)
	if c.sessionID == sessionID {
		c.sessionID = ""
	}
	c.mu.Unlock()

	return err
}

// FileContent reads file content via OpenCode
func (c *OpenCodeClient) FileContent(ctx context.Context, path string) (string, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/file/content?path=%s", path), nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	return result.Content, nil
}

// FindFiles searches for files by pattern
func (c *OpenCodeClient) FindFiles(ctx context.Context, query string) ([]string, error) {
	resp, err := c.doRequest(ctx, "GET", fmt.Sprintf("/find/file?query=%s", query), nil)
	if err != nil {
		return nil, err
	}

	var result []string
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Grep searches file contents
func (c *OpenCodeClient) Grep(ctx context.Context, pattern, directory string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("/find?pattern=%s", pattern)
	if directory != "" {
		url += fmt.Sprintf("&directory=%s", directory)
	}

	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// SubscribeEvents subscribes to OpenCode SSE event stream with auto-reconnect
// Returns a channel for events and a cancel function
func (c *OpenCodeClient) SubscribeEvents(ctx context.Context, sessionID string) (<-chan SSEEvent, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(ctx)
	eventChan := make(chan SSEEvent, 100)

	go func() {
		defer close(eventChan)
		defer cancel()

		consecutiveErrors := 0
		maxConsecutiveErrors := 3

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Connect to SSE stream
			connectCtx, connectCancel := context.WithCancel(ctx)

			req, err := http.NewRequestWithContext(connectCtx, "GET", c.baseURL+"/event", nil)
			if err != nil {
				connectCancel()
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					logger.Error("SSE connection failed too many times", "errors", consecutiveErrors)
					return
				}
				time.Sleep(time.Duration(consecutiveErrors) * time.Second)
				continue
			}
			req.Header.Set("Accept", "text/event-stream")

			resp, err := c.client.Do(req)
			if err != nil {
				connectCancel()
				consecutiveErrors++
				logger.Warn("SSE connection failed, retrying", "error", err, "attempt", consecutiveErrors)
				if consecutiveErrors >= maxConsecutiveErrors {
					logger.Error("SSE connection failed too many times", "errors", consecutiveErrors)
					return
				}
				time.Sleep(time.Duration(consecutiveErrors) * time.Second)
				continue
			}

			// Connection successful, reset error counter
			consecutiveErrors = 0
			logger.Debug("SSE connected to OpenCode")

			// Process events with timeout detection
			lastEventTime := time.Now()
			timeout := 60 * time.Second // No event for 60 seconds = reconnect

			// Start timeout checker
			go func() {
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-connectCtx.Done():
						return
					case <-ticker.C:
						if time.Since(lastEventTime) > timeout {
							logger.Warn("SSE timeout, reconnecting", "idle", time.Since(lastEventTime).Seconds())
							connectCancel()
							return
						}
					}
				}
			}()

			// Read events
			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				select {
				case <-connectCtx.Done():
					resp.Body.Close()
					break
				default:
				}

				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					lastEventTime = time.Now()
					data := strings.TrimPrefix(line, "data: ")
					var event SSEEvent
					if err := json.Unmarshal([]byte(data), &event); err == nil {
						// Filter to session-specific events if sessionID provided
						if sessionID != "" {
							// Check sessionID in different locations
							eventSessionID := ""
							if sid, ok := event.Properties["sessionID"].(string); ok {
								eventSessionID = sid
							} else if part, ok := event.Properties["part"].(map[string]interface{}); ok {
								if sid, ok := part["sessionID"].(string); ok {
									eventSessionID = sid
								}
							}
							if eventSessionID != "" && eventSessionID != sessionID {
								continue
							}
						}
						select {
						case eventChan <- event:
						default:
							// Channel full, skip event
						}
					}
				}
			}

			resp.Body.Close()
			connectCancel()

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
				// Reconnect after brief pause
				time.Sleep(1 * time.Second)
				logger.Debug("SSE reconnecting...")
			}
		}
	}()

	return eventChan, cancel, nil
}

// FormatEvent formats an SSE event for display
func FormatEvent(event SSEEvent) string {
	switch event.Type {
	case "message.part.updated":
		// Get part object
		part, ok := event.Properties["part"].(map[string]interface{})
		if !ok {
			return ""
		}

		partType, _ := part["type"].(string)
		switch partType {
		case "tool":
			// Tool execution event
			toolName, _ := part["tool"].(string)
			state, _ := part["state"].(map[string]interface{})
			if state == nil {
				return ""
			}
			status, _ := state["status"].(string)

			switch status {
			case "running":
				var details string
				if input, ok := state["input"].(map[string]interface{}); ok {
					// Try to get description
					if desc, ok := input["description"].(string); ok && desc != "" {
						details = desc
					} else {
						// Extract key parameters based on tool type
						details = extractToolDetails(toolName, input)
					}
				}
				if details != "" {
					return fmt.Sprintf("  ⚙️ %s: %s", toolName, details)
				}
				return fmt.Sprintf("  ⚙️ 执行: %s", toolName)
			case "completed":
				// Try to get result summary
				var resultSummary string
				if output, ok := state["output"].(map[string]interface{}); ok {
					resultSummary = extractResultSummary(toolName, output)
				}
				if resultSummary != "" {
					return fmt.Sprintf("  ✓ 完成: %s (%s)", toolName, resultSummary)
				}
				return fmt.Sprintf("  ✓ 完成: %s", toolName)
			case "error":
				if errMsg, ok := state["error"].(string); ok && errMsg != "" {
					return fmt.Sprintf("  ✗ 错误: %s - %s", toolName, truncateString(errMsg, 100))
				}
				return fmt.Sprintf("  ✗ 错误: %s", toolName)
			}

		case "step-start":
			return "  🔄 开始处理..."

		case "step-finish":
			return "  📝 步骤完成"

		case "text", "reasoning":
			// Skip text/reasoning updates - too noisy
			return ""
		}

	case "session.status":
		status, ok := event.Properties["status"].(map[string]interface{})
		if !ok {
			return ""
		}
		statusType, _ := status["type"].(string)
		if statusType == "busy" {
			return "" // Skip busy status, too noisy
		}

	case "session.idle":
		return "  ✅ 任务完成"

	case "server.connected":
		return "  🔗 已连接 OpenCode"
	}

	return ""
}

// extractToolDetails extracts relevant details from tool input
func extractToolDetails(toolName string, input map[string]interface{}) string {
	switch toolName {
	case "bash":
		if cmd, ok := input["command"].(string); ok {
			return truncateString(cmd, 80)
		}
		if cmds, ok := input["command"].([]interface{}); ok && len(cmds) > 0 {
			if first, ok := cmds[0].(string); ok {
				return truncateString(first, 80)
			}
		}
	case "read":
		// OpenCode uses "filePath"
		if path, ok := input["filePath"].(string); ok {
			return path
		}
		if path, ok := input["file_path"].(string); ok {
			return path
		}
		if path, ok := input["path"].(string); ok {
			return path
		}
	case "write":
		if path, ok := input["filePath"].(string); ok {
			return path
		}
		if path, ok := input["file_path"].(string); ok {
			return path
		}
		if path, ok := input["path"].(string); ok {
			return path
		}
	case "edit":
		if path, ok := input["filePath"].(string); ok {
			return path
		}
		if path, ok := input["file_path"].(string); ok {
			return path
		}
		if path, ok := input["path"].(string); ok {
			return path
		}
	case "glob":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "grep":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "web_search", "webSearch":
		if query, ok := input["query"].(string); ok {
			return query
		}
	default:
		// Generic: try common field names (OpenCode uses camelCase)
		for _, key := range []string{"filePath", "file_path", "path", "command", "query", "pattern", "message"} {
			if val, ok := input[key].(string); ok && val != "" {
				return truncateString(val, 80)
			}
		}
	}
	return ""
}

// extractResultSummary extracts a summary from tool output
func extractResultSummary(toolName string, output map[string]interface{}) string {
	switch toolName {
	case "bash":
		if stdout, ok := output["stdout"].(string); ok && stdout != "" {
			lines := strings.Count(stdout, "\n") + 1
			return fmt.Sprintf("%d行输出", lines)
		}
	case "read":
		if content, ok := output["content"].(string); ok {
			lines := strings.Count(content, "\n") + 1
			return fmt.Sprintf("%d行", lines)
		}
	case "glob":
		if files, ok := output["files"].([]interface{}); ok {
			return fmt.Sprintf("%d个文件", len(files))
		}
	case "grep":
		if matches, ok := output["matches"].([]interface{}); ok {
			return fmt.Sprintf("%d个匹配", len(matches))
		}
	case "write", "edit":
		return "已保存"
	}

	// Check for generic result indicator
	if result, ok := output["result"].(string); ok && result != "" {
		return truncateString(result, 50)
	}

	return ""
}

// truncateString truncates a string to maxLen
func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// doRequest performs HTTP request
func (c *OpenCodeClient) doRequest(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error %s: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// --- Tool Interface Implementation ---

// OpenCodeTool OpenCode tool for LingGuard
type OpenCodeTool struct {
	client *OpenCodeClient
	config *OpenCodeConfig
}

// NewOpenCodeTool creates OpenCode tool
func NewOpenCodeTool(cfg *OpenCodeConfig) *OpenCodeTool {
	return &OpenCodeTool{
		client: NewOpenCodeClient(cfg),
		config: cfg,
	}
}

func (t *OpenCodeTool) Name() string {
	return "opencode"
}

func (t *OpenCodeTool) Description() string {
	return `Delegate ALL coding tasks to OpenCode - a professional AI coding agent.

Use this tool for ANY task involving:
- Writing code in ANY language (Python, Go, JavaScript, TypeScript, Java, etc.)
- Creating new source files (.py, .go, .js, .ts, .java, etc.)
- Editing or modifying existing code
- Refactoring code
- Debugging
- Running tests or build commands
- Code analysis and review

IMPORTANT:
- DO NOT use the 'file' or 'shell' tools after opencode completes
- OpenCode handles everything including verification
- If timeout occurs, tell user to try again or split the task

Actions:
- "prompt": Send a natural language coding task
- "command": Execute slash commands (/init, /review)
- "shell": Run shell commands (go test, npm run, etc.)`
}

func (t *OpenCodeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"prompt", "command", "shell"},
				"description": "Action type: 'prompt' for natural language tasks, 'command' for slash commands (/init, /review), 'shell' for shell commands",
			},
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The task description or command to execute",
			},
			"agent": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"build", "plan"},
				"default":     "build",
				"description": "Agent to use: 'build' for execution, 'plan' for planning only",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Model to use (e.g., 'anthropic/claude-sonnet-4-20250514'). Leave empty for default.",
			},
			"session_id": map[string]interface{}{
				"type":        "string",
				"description": "Existing session ID to continue. Leave empty to use/create default session.",
			},
		},
		"required": []string{"action", "task"},
	}
}

func (t *OpenCodeTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Action    string `json:"action"`
		Task      string `json:"task"`
		Agent     string `json:"agent"`
		Model     string `json:"model"`
		SessionID string `json:"session_id"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("parse parameters: %w", err)
	}

	// Set defaults
	if p.Agent == "" {
		p.Agent = "build"
	}

	// If OpenCode is disabled or not configured, return native tools prompt
	if t.config == nil || !t.config.Enabled {
		logger.Debug("OpenCode is disabled, using native tools")
		return fmt.Sprintf("⚠️ OpenCode 服务已禁用，请使用原生工具完成任务：\n\n**任务**: %s\n\n请使用以下原生工具：\n- file 工具：读取、编辑、写入文件\n- shell 工具：执行命令、编译、测试\n- workspace 工具：查看工作目录\n\n工作目录: %s", p.Task, t.getWorkspace()), nil
	}

	// Check server health, try to start if not available
	healthy, version, err := t.client.Health(ctx)
	if err != nil || !healthy {
		// Try to start OpenCode server
		logger.Info("OpenCode server not available, attempting to start...")
		if startErr := t.client.StartServer(ctx); startErr != nil {
			logger.Warn("OpenCode server failed to start, falling back to native tools", "error", startErr)
			// 返回提示让 LLM 使用原生工具
			return fmt.Sprintf("⚠️ OpenCode 服务不可用，请使用原生工具完成任务：\n\n**任务**: %s\n\n请使用以下原生工具：\n- file 工具：读取、编辑、写入文件\n- shell 工具：执行命令、编译、测试\n- workspace 工具：查看工作目录\n\n工作目录: %s", p.Task, t.config.Workspace), nil
		}
		// Re-check health after starting
		healthy, version, err = t.client.Health(ctx)
		if err != nil || !healthy {
			logger.Warn("OpenCode server unhealthy after start, falling back to native tools")
			return fmt.Sprintf("⚠️ OpenCode 服务不可用，请使用原生工具完成任务：\n\n**任务**: %s\n\n请使用以下原生工具：\n- file 工具：读取、编辑、写入文件\n- shell 工具：执行命令、编译、测试\n- workspace 工具：查看工作目录\n\n工作目录: %s", p.Task, t.config.Workspace), nil
		}
	}

	// Get or create session
	sessionID := p.SessionID
	if sessionID == "" {
		session, err := t.client.GetOrCreateSession(ctx, "LingGuard Coding Session")
		if err != nil {
			return "", fmt.Errorf("create session: %w", err)
		}
		sessionID = session.ID
	}

	// Subscribe to SSE events for real-time logs (no session filter, collect all)
	eventChan, cancelEvents, err := t.client.SubscribeEvents(ctx, "")
	if err != nil {
		logger.Warn("Failed to subscribe OpenCode events", "error", err)
		// Continue without events
	}

	// Collect events in background
	var eventLogs []string
	var eventMu sync.Mutex
	done := make(chan struct{})
	if eventChan != nil {
		go func() {
			defer close(done)
			for event := range eventChan {
				// Filter for session-specific events
				// sessionID can be at top level or in part/session
				eventSessionID := ""
				if sid, ok := event.Properties["sessionID"].(string); ok {
					eventSessionID = sid
				} else if part, ok := event.Properties["part"].(map[string]interface{}); ok {
					if sid, ok := part["sessionID"].(string); ok {
						eventSessionID = sid
					}
				}

				// Skip events from other sessions
				if eventSessionID != "" && eventSessionID != sessionID {
					continue
				}

				if formatted := FormatEvent(event); formatted != "" {
					eventMu.Lock()
					eventLogs = append(eventLogs, formatted)
					eventMu.Unlock()
					logger.Info("OpenCode event", "type", event.Type, "log", formatted)
				}
			}
		}()
	} else {
		// Close done channel if no event subscription
		close(done)
	}

	var result string

	switch p.Action {
	case "prompt":
		// If workspace is configured and different from OpenCode's default, change directory first
		if t.config.Workspace != "" {
			// Send a shell command to change directory
			_, shellErr := t.client.ExecuteShell(ctx, sessionID, fmt.Sprintf("cd %s && pwd", t.config.Workspace))
			if shellErr != nil {
				logger.Warn("Failed to change OpenCode directory", "error", shellErr, "workspace", t.config.Workspace)
			} else {
				logger.Debug("Changed OpenCode working directory", "workspace", t.config.Workspace)
			}
		}

		opts := SendMessageOptions{
			Agent: p.Agent,
			Model: p.Model,
			Parts: []MessagePart{
				{Type: "text", Text: p.Task},
			},
		}
		resp, err := t.client.SendMessage(ctx, sessionID, opts)
		if err != nil {
			if cancelEvents != nil {
				cancelEvents()
			}
			return "", fmt.Errorf("send message: %w", err)
		}

		// Wait a bit for remaining events to be collected
		time.Sleep(100 * time.Millisecond)

		// Stop event collection
		if cancelEvents != nil {
			cancelEvents()
		}

		// Wait for goroutine to finish
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}

		// Extract text from response
		var texts []string
		for _, part := range resp.Parts {
			if part.Type == "text" && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
		result = strings.Join(texts, "\n")

		if result == "" {
			result = "(Task completed, no text output)"
		}

	case "command":
		resp, err := t.client.ExecuteCommand(ctx, sessionID, p.Task)
		if err != nil {
			if cancelEvents != nil {
				cancelEvents()
			}
			return "", fmt.Errorf("execute command: %w", err)
		}

		// Wait for events
		time.Sleep(100 * time.Millisecond)
		if cancelEvents != nil {
			cancelEvents()
		}
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}

		var texts []string
		for _, part := range resp.Parts {
			if part.Type == "text" && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
		result = strings.Join(texts, "\n")

	case "shell":
		resp, err := t.client.ExecuteShell(ctx, sessionID, p.Task)
		if err != nil {
			if cancelEvents != nil {
				cancelEvents()
			}
			return "", fmt.Errorf("execute shell: %w", err)
		}

		// Wait for events
		time.Sleep(100 * time.Millisecond)
		if cancelEvents != nil {
			cancelEvents()
		}
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}

		var sb strings.Builder
		if resp.Stdout != "" {
			sb.WriteString(resp.Stdout)
		}
		if resp.Stderr != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("STDERR:\n")
			sb.WriteString(resp.Stderr)
		}
		result = sb.String()

	default:
		return "", fmt.Errorf("unknown action: %s", p.Action)
	}

	// Build result with event logs
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[OpenCode v%s, Session: %s]\n", version, sessionID[:12]+"..."))

	// Add event logs if any
	eventMu.Lock()
	if len(eventLogs) > 0 {
		sb.WriteString("\n执行过程:\n")
		for _, log := range eventLogs {
			sb.WriteString(log + "\n")
		}
	}
	eventMu.Unlock()

	sb.WriteString("\n")
	sb.WriteString(result)

	return sb.String(), nil
}

func (t *OpenCodeTool) IsDangerous() bool {
	return false
}

func (t *OpenCodeTool) ShouldLoadByDefault() bool {
	// 只有启用时才加载到 LLM 工具列表
	return t.config != nil && t.config.Enabled
}

// SetBaseURL updates the base URL
func (t *OpenCodeTool) SetBaseURL(url string) {
	t.config.BaseURL = url
	t.client.baseURL = strings.TrimSuffix(url, "/")
}

// GetClient returns the underlying OpenCode client
func (t *OpenCodeTool) GetClient() *OpenCodeClient {
	return t.client
}

// getWorkspace safely returns the workspace path
func (t *OpenCodeTool) getWorkspace() string {
	if t.config != nil {
		return t.config.Workspace
	}
	return ""
}
