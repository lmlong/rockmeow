// Package tools MCP (Model Context Protocol) client implementation
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
)

// MCPToolWrapper wraps an MCP server tool as a native LingGuard Too
// MCPToolWrapper wraps an MCP server tool as a native LingGuard Tool
type MCPToolWrapper struct {
	serverName   string
	originalName string
	name         string
	description  string
	parameters   map[string]interface{}
	client       MCPToolCaller
}

// MCPToolCaller interface for calling MCP tools
type MCPToolCaller interface {
	CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error)
}

// NewMCPToolWrapper creates a wrapper for an MCP tool
func NewMCPToolWrapper(client MCPToolCaller, serverName, originalName, description string, parameters map[string]interface{}) *MCPToolWrapper {
	return &MCPToolWrapper{
		serverName:   serverName,
		originalName: originalName,
		name:         fmt.Sprintf("mcp_%s_%s", serverName, originalName),
		description:  description,
		parameters:   parameters,
		client:       client,
	}
}

func (t *MCPToolWrapper) Name() string {
	return t.name
}

func (t *MCPToolWrapper) Description() string {
	return t.description
}

func (t *MCPToolWrapper) Parameters() map[string]interface{} {
	return t.parameters
}

func (t *MCPToolWrapper) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	// 验证 JSON 完整性
	if len(params) > 0 && !json.Valid(params) {
		preview := string(params)
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		return "", fmt.Errorf("invalid parameters: incomplete JSON (len=%d, preview=%s)", len(params), preview)
	}

	var args map[string]interface{}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &args); err != nil {
			return "", fmt.Errorf("invalid parameters: %w", err)
		}
	}

	result, err := t.client.CallTool(ctx, t.originalName, args)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (t *MCPToolWrapper) IsDangerous() bool {
	return false
}

// MCPToolDefinition MCP 工具定义
type MCPToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPClient MCP 客户端
type MCPClient struct {
	mu         sync.RWMutex
	cmd        *exec.Cmd
	serverName string
	config     config.MCPServerConfig
	tools      map[string]*MCPToolDefinition
	// JSON-RPC communication
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	requestID int64
	encoder   *json.Encoder
}

// NewMCPClient creates a new MCP client
func NewMCPClient(serverName string, cfg config.MCPServerConfig) *MCPClient {
	return &MCPClient{
		serverName: serverName,
		config:     cfg,
		tools:      make(map[string]*MCPToolDefinition),
	}
}

// expandArgs expands placeholders in args with actual values
func expandArgs(args []string, workspace string) []string {
	home, _ := os.UserHomeDir()
	result := make([]string, len(args))
	for i, arg := range args {
		// Replace ${workspace} placeholder
		if arg == "${workspace}" {
			result[i] = workspace
		} else if arg == "${home}" {
			result[i] = home
		} else if strings.Contains(arg, "${home}") {
			result[i] = strings.ReplaceAll(arg, "${home}", home)
		} else {
			result[i] = arg
		}
	}
	return result
}

// ensureNodeJS ensures Node.js (npm/npx) is installed
// Returns error if installation fails
func ensureNodeJS(ctx context.Context) error {
	// Check if npx exists
	if _, err := exec.LookPath("npx"); err == nil {
		return nil // npx already installed
	}

	logger.Info("Node.js not found, attempting to install...")

	// Detect OS and install Node.js
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		// Try apt first (Debian/Ubuntu)
		if _, err := exec.LookPath("apt"); err == nil {
			cmd = exec.CommandContext(ctx, "sudo", "bash", "-c",
				"curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && apt install -y nodejs")
		} else if _, err := exec.LookPath("yum"); err == nil {
			// RHEL/CentOS/Fedora
			cmd = exec.CommandContext(ctx, "sudo", "bash", "-c",
				"curl -fsSL https://rpm.nodesource.com/setup_20.x | bash - && yum install -y nodejs")
		} else if _, err := exec.LookPath("pacman"); err == nil {
			// Arch Linux
			cmd = exec.CommandContext(ctx, "sudo", "pacman", "-S", "--noconfirm", "nodejs", "npm")
		} else {
			return fmt.Errorf("unsupported Linux distribution - please install Node.js manually")
		}
	case "darwin":
		// macOS - try Homebrew
		if _, err := exec.LookPath("brew"); err == nil {
			cmd = exec.CommandContext(ctx, "brew", "install", "node")
		} else {
			return fmt.Errorf("Homebrew not found - please install Node.js manually")
		}
	default:
		return fmt.Errorf("unsupported OS %s - please install Node.js manually", runtime.GOOS)
	}

	// Run installation
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Node.js: %w", err)
	}

	// Verify installation
	if _, err := exec.LookPath("npx"); err != nil {
		return fmt.Errorf("Node.js installation completed but npx still not found")
	}

	logger.Info("Node.js installed successfully")
	return nil
}

// requiresNodeJS checks if the command requires Node.js (npx/npm)
func requiresNodeJS(command string) bool {
	return command == "npx" || command == "npm"
}

// Connect connects to the MCP server
func (c *MCPClient) Connect(ctx context.Context) error {
	if c.config.Command == "" {
		return fmt.Errorf("MCP server '%s': no command configured", c.serverName)
	}

	// Ensure Node.js is installed if command requires it (npx/npm)
	if requiresNodeJS(c.config.Command) {
		if err := ensureNodeJS(ctx); err != nil {
			return fmt.Errorf("MCP server '%s' requires Node.js: %w", c.serverName, err)
		}
	}

	// Create command
	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	// Set environment
	if c.config.Env != nil {
		c.cmd.Env = os.Environ()
		for k, v := range c.config.Env {
			c.cmd.Env = append(c.cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Set up pipes
	stdin, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("create stdin pipe: %w", err)
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		stdin.Close() // 清理已创建的 pipe
		return fmt.Errorf("create stdout pipe: %w", err)
	}
	// Redirect stderr to parent stderr for logging
	c.cmd.Stderr = os.Stderr

	// Start the process
	if err := c.cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("start MCP server: %w", err)
	}
	// Create JSON-RPC client
	c.stdin = stdin
	c.stdout = bufio.NewReader(stdout)
	c.encoder = json.NewEncoder(stdin)
	c.requestID = 0

	// Initialize
	if err := c.initialize(); err != nil {
		c.Close()
		return fmt.Errorf("initialize MCP server: %w", err)
	}

	// List tools
	if err := c.listTools(); err != nil {
		c.Close()
		return fmt.Errorf("list MCP tools: %w", err)
	}

	logger.Info("MCP server connected", "server", c.serverName, "tools", len(c.tools))
	return nil
}

// Close closes the MCP client with graceful shutdown
func (c *MCPClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		// 使用优雅终止 + 超时机制
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()

		// 先尝试 SIGTERM 优雅终止
		c.cmd.Process.Signal(syscall.SIGTERM)

		select {
		case <-time.After(5 * time.Second):
			// 超时后强制终止
			logger.Warn("MCP server did not stop gracefully, killing", "server", c.serverName)
			c.cmd.Process.Kill()
			<-done // 等待进程结束
		case err := <-done:
			// 进程正常退出
			if err != nil {
				logger.Debug("MCP server exited with error", "server", c.serverName, "error", err)
			}
		}
	}
	return nil
}

// JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// sendRequest sends a JSON-RPC request and waits for response
func (c *MCPClient) sendRequest(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.requestID++
	id := c.requestID
	c.mu.Unlock()

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	logger.Debug("MCP sending request", "method", method, "id", id)

	if err := c.encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Read response, skipping any non-JSON lines (like server startup messages)
	var resp jsonRPCResponse
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse as JSON
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			logger.Debug("MCP skipping non-JSON line", "line", line)
			continue
		}

		// Check if this response matches our request ID
		if resp.ID != id {
			logger.Debug("MCP skipping response with wrong ID", "got", resp.ID, "expected", id)
			continue
		}

		break
	}

	logger.Debug("MCP received response", "id", resp.ID, "error", resp.Error)

	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// initialize sends initialize request to MCP server
func (c *MCPClient) initialize() error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "lingguard",
			"version": "1.0.0",
		},
	}

	result, err := c.sendRequest("initialize", params)
	if err != nil {
		return err
	}

	// Parse result to verify
	var initResult struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	logger.Info("MCP server initialized",
		"server", c.serverName,
		"protocol", initResult.ProtocolVersion,
		"serverName", initResult.ServerInfo.Name,
		"serverVersion", initResult.ServerInfo.Version)

	// Send initialized notification (no ID, no response expected)
	notif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	if err := c.encoder.Encode(notif); err != nil {
		return fmt.Errorf("send initialized notification: %w", err)
	}

	return nil
}

// listTools retrieves available tools from MCP server
func (c *MCPClient) listTools() error {
	result, err := c.sendRequest("tools/list", map[string]interface{}{})
	if err != nil {
		return err
	}

	var listResult struct {
		Tools []MCPToolDefinition `json:"tools"`
	}
	if err := json.Unmarshal(result, &listResult); err != nil {
		return fmt.Errorf("parse tools list: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, tool := range listResult.Tools {
		c.tools[tool.Name] = &tool
	}

	return nil
}

// CallTool calls a tool on the MCP server
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	result, err := c.sendRequest("tools/call", params)
	if err != nil {
		return "", err
	}

	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &callResult); err != nil {
		return "", fmt.Errorf("parse call result: %w", err)
	}

	if callResult.IsError {
		if len(callResult.Content) > 0 {
			return "", fmt.Errorf("tool error: %s", callResult.Content[0].Text)
		}
		return "", fmt.Errorf("tool error (unknown)")
	}

	// Combine text content
	var output string
	for _, content := range callResult.Content {
		if content.Type == "text" {
			if output != "" {
				output += "\n"
			}
			output += content.Text
		}
	}

	if output == "" {
		output = "(no output)"
	}

	return output, nil
}

// GetTools returns all tools from this MCP server
func (c *MCPClient) GetTools() []*MCPToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tools := make([]*MCPToolDefinition, 0, len(c.tools))
	for _, tool := range c.tools {
		tools = append(tools, tool)
	}
	return tools
}

// MCPManager manages multiple MCP server connections
type MCPManager struct {
	mu      sync.RWMutex
	clients map[string]MCPClientInterface
	tools   map[string]Tool
}

// MCPClientInterface interface for MCP clients
type MCPClientInterface interface {
	Connect(ctx context.Context) error
	Close() error
	GetTools() []*MCPToolDefinition
	CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error)
}

// NewMCPManager creates a new MCP manager
func NewMCPManager() *MCPManager {
	return &MCPManager{
		clients: make(map[string]MCPClientInterface),
		tools:   make(map[string]Tool),
	}
}

// ConnectServers connects to all configured MCP servers
// workspace is used to expand ${workspace} placeholder in args
func (m *MCPManager) ConnectServers(ctx context.Context, servers map[string]config.MCPServerConfig, workspace string) error {
	for name, cfg := range servers {
		// Expand ${workspace} placeholder in args
		cfg.Args = expandArgs(cfg.Args, workspace)

		var client MCPClientInterface

		// Determine transport type based on config
		if cfg.URL != "" {
			// HTTP transport (simple POST-based)
			client = NewMCPHTTPClient(name, cfg)
		} else if cfg.Command != "" {
			// Stdio transport
			client = NewMCPClient(name, cfg)
		} else {
			logger.Warn("MCP server has no command or URL configured, skipping", "server", name)
			continue
		}

		if err := client.Connect(ctx); err != nil {
			logger.Error("MCP server failed to connect", "server", name, "error", err)
			continue
		}

		m.mu.Lock()
		m.clients[name] = client

		// Register tools
		for _, toolDef := range client.GetTools() {
			wrapper := NewMCPToolWrapper(client, name, toolDef.Name, toolDef.Description, toolDef.InputSchema)
			m.tools[wrapper.Name()] = wrapper
			logger.Debug("MCP registered tool", "tool", wrapper.Name(), "server", name)
		}
		m.mu.Unlock()
	}
	return nil
}

// GetTools returns all MCP tools
func (m *MCPManager) GetTools() map[string]Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tools := make(map[string]Tool)
	for name, tool := range m.tools {
		tools[name] = tool
	}
	return tools
}

// Close closes all MCP connections
func (m *MCPManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[string]MCPClientInterface)
	m.tools = make(map[string]Tool)
	return nil
}
