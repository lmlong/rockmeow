// Package tools OpenCode HTTP API integration
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lingguard/pkg/logger"
)

// OpenCodeClient OpenCode HTTP API client
type OpenCodeClient struct {
	mu        sync.RWMutex
	baseURL   string
	client    *http.Client
	sessions  map[string]*OpenCodeSession
	sessionID string
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
	BaseURL string
	Timeout time.Duration
}

// DefaultOpenCodeConfig returns default configuration
func DefaultOpenCodeConfig() *OpenCodeConfig {
	return &OpenCodeConfig{
		BaseURL: "http://127.0.0.1:4096",
		Timeout: 300 * time.Second,
	}
}

// NewOpenCodeClient creates a new OpenCode client
func NewOpenCodeClient(cfg *OpenCodeConfig) *OpenCodeClient {
	if cfg == nil {
		cfg = DefaultOpenCodeConfig()
	}
	return &OpenCodeClient{
		baseURL:  strings.TrimSuffix(cfg.BaseURL, "/"),
		client:   &http.Client{Timeout: cfg.Timeout},
		sessions: make(map[string]*OpenCodeSession),
	}
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

	logger.Info("OpenCode session created: %s", session.ID)
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

DO NOT use the 'file' tool for writing code. Use THIS tool instead.

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

	// Check server health first
	healthy, version, err := t.client.Health(ctx)
	if err != nil {
		return "", fmt.Errorf("OpenCode server not available: %w", err)
	}
	if !healthy {
		return "", fmt.Errorf("OpenCode server unhealthy")
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

	var result string

	switch p.Action {
	case "prompt":
		opts := SendMessageOptions{
			Agent: p.Agent,
			Model: p.Model,
			Parts: []MessagePart{
				{Type: "text", Text: p.Task},
			},
		}
		resp, err := t.client.SendMessage(ctx, sessionID, opts)
		if err != nil {
			return "", fmt.Errorf("send message: %w", err)
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
			return "", fmt.Errorf("execute command: %w", err)
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
			return "", fmt.Errorf("execute shell: %w", err)
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

	// Prepend session info
	return fmt.Sprintf("[OpenCode v%s, Session: %s]\n\n%s", version, sessionID[:12]+"...", result), nil
}

func (t *OpenCodeTool) IsDangerous() bool {
	return false
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
