// Package tools MCP HTTP transport implementation
package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
)

// MCPHTTPClient MCP HTTP client for SSE/Streamable HTTP transport
type MCPHTTPClient struct {
	mu         sync.RWMutex
	serverName string
	config     config.MCPServerConfig
	tools      map[string]*MCPToolDefinition
	client     *http.Client
	baseURL    string
	sessionID  string
	requestID  int64
	// SSE connection
	sseResp *http.Response
	sseChan chan json.RawMessage
}

// NewMCPHTTPClient creates a new MCP HTTP client
func NewMCPHTTPClient(serverName string, cfg config.MCPServerConfig) *MCPHTTPClient {
	return &MCPHTTPClient{
		serverName: serverName,
		config:     cfg,
		tools:      make(map[string]*MCPToolDefinition),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		sseChan: make(chan json.RawMessage, 100),
	}
}

// Connect connects to the MCP HTTP server
func (c *MCPHTTPClient) Connect(ctx context.Context) error {
	if c.config.URL == "" {
		return fmt.Errorf("MCP server '%s': no URL configured", c.serverName)
	}

	c.baseURL = strings.TrimSuffix(c.config.URL, "/")

	// Initialize
	if err := c.initialize(ctx); err != nil {
		return fmt.Errorf("initialize MCP server: %w", err)
	}

	// List tools
	if err := c.listTools(ctx); err != nil {
		return fmt.Errorf("list MCP tools: %w", err)
	}

	logger.Info("MCP HTTP server '%s': connected, %d tools registered", c.serverName, len(c.tools))
	return nil
}

// Close closes the MCP HTTP client
func (c *MCPHTTPClient) Close() error {
	if c.sseResp != nil {
		c.sseResp.Body.Close()
	}
	close(c.sseChan)
	return nil
}

// sendHTTPRequest sends a JSON-RPC request via HTTP
func (c *MCPHTTPClient) sendHTTPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
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

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Use baseURL directly (it should already include the full path)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.sessionID != "" {
		httpReq.Header.Set("X-Session-ID", c.sessionID)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Save session ID if provided
	if sid := resp.Header.Get("X-Session-ID"); sid != "" {
		c.sessionID = sid
	}

	var jsonResp jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if jsonResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}

	return jsonResp.Result, nil
}

// initialize sends initialize request to MCP HTTP server
func (c *MCPHTTPClient) initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "lingguard",
			"version": "1.0.0",
		},
	}

	result, err := c.sendHTTPRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

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

	logger.Debug("MCP HTTP server '%s': initialized (protocol=%s, server=%s/%s)",
		c.serverName, initResult.ProtocolVersion, initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	return nil
}

// listTools retrieves available tools from MCP HTTP server
func (c *MCPHTTPClient) listTools(ctx context.Context) error {
	result, err := c.sendHTTPRequest(ctx, "tools/list", map[string]interface{}{})
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

// CallTool calls a tool on the MCP HTTP server
func (c *MCPHTTPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	result, err := c.sendHTTPRequest(ctx, "tools/call", params)
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
func (c *MCPHTTPClient) GetTools() []*MCPToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tools := make([]*MCPToolDefinition, 0, len(c.tools))
	for _, tool := range c.tools {
		tools = append(tools, tool)
	}
	return tools
}

// MCPHTTPToolWrapper wraps an MCP HTTP server tool as a native LingGuard Tool
type MCPHTTPToolWrapper struct {
	serverName   string
	originalName string
	name         string
	description  string
	parameters   map[string]interface{}
	client       *MCPHTTPClient
}

// NewMCPHTTPToolWrapper creates a wrapper for an MCP HTTP tool
func NewMCPHTTPToolWrapper(client *MCPHTTPClient, serverName, originalName, description string, parameters map[string]interface{}) *MCPHTTPToolWrapper {
	return &MCPHTTPToolWrapper{
		serverName:   serverName,
		originalName: originalName,
		name:         fmt.Sprintf("mcp_%s_%s", serverName, originalName),
		description:  description,
		parameters:   parameters,
		client:       client,
	}
}

func (t *MCPHTTPToolWrapper) Name() string {
	return t.name
}

func (t *MCPHTTPToolWrapper) Description() string {
	return t.description
}

func (t *MCPHTTPToolWrapper) Parameters() map[string]interface{} {
	return t.parameters
}

func (t *MCPHTTPToolWrapper) Execute(ctx context.Context, params json.RawMessage) (string, error) {
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

func (t *MCPHTTPToolWrapper) IsDangerous() bool {
	return false
}

// SSE (Server-Sent Events) support for streaming MCP

// SSEClient handles SSE-based MCP connections
type SSEClient struct {
	mu         sync.RWMutex
	serverName string
	config     config.MCPServerConfig
	tools      map[string]*MCPToolDefinition
	client     *http.Client
	baseURL    string
	requestID  int64
	endpoint   string // SSE endpoint
}

// NewSSEClient creates a new SSE-based MCP client
func NewSSEClient(serverName string, cfg config.MCPServerConfig) *SSEClient {
	return &SSEClient{
		serverName: serverName,
		config:     cfg,
		tools:      make(map[string]*MCPToolDefinition),
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Connect connects to the MCP SSE server
func (c *SSEClient) Connect(ctx context.Context) error {
	if c.config.URL == "" {
		return fmt.Errorf("MCP server '%s': no URL configured", c.serverName)
	}

	c.baseURL = strings.TrimSuffix(c.config.URL, "/")

	// First, connect to SSE endpoint to get message endpoint
	if err := c.connectSSE(ctx); err != nil {
		return fmt.Errorf("connect SSE: %w", err)
	}

	// Initialize
	if err := c.initialize(ctx); err != nil {
		return fmt.Errorf("initialize MCP server: %w", err)
	}

	// List tools
	if err := c.listTools(ctx); err != nil {
		return fmt.Errorf("list MCP tools: %w", err)
	}

	logger.Info("MCP SSE server '%s': connected, %d tools registered", c.serverName, len(c.tools))
	return nil
}

// connectSSE establishes SSE connection and gets message endpoint
func (c *SSEClient) connectSSE(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/sse", nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("connect SSE: %w", err)
	}
	defer resp.Body.Close()

	// Read SSE events to get endpoint
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: endpoint") {
			// Next line should be data
			if scanner.Scan() {
				data := scanner.Text()
				if strings.HasPrefix(data, "data: ") {
					c.endpoint = strings.TrimPrefix(data, "data: ")
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to get SSE endpoint")
}

// sendRequest sends a JSON-RPC request via HTTP POST
func (c *SSEClient) sendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
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

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// For SSE, response might be accepted (202), actual response comes via SSE
	// For simplicity, we wait for the response here
	if resp.StatusCode == http.StatusAccepted {
		// TODO: implement SSE response waiting
		return nil, fmt.Errorf("SSE async response not yet implemented")
	}

	var jsonResp jsonRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if jsonResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", jsonResp.Error.Code, jsonResp.Error.Message)
	}

	return jsonResp.Result, nil
}

// initialize sends initialize request
func (c *SSEClient) initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "lingguard",
			"version": "1.0.0",
		},
	}

	result, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

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

	logger.Debug("MCP SSE server '%s': initialized (protocol=%s, server=%s/%s)",
		c.serverName, initResult.ProtocolVersion, initResult.ServerInfo.Name, initResult.ServerInfo.Version)

	return nil
}

// listTools retrieves available tools
func (c *SSEClient) listTools(ctx context.Context) error {
	result, err := c.sendRequest(ctx, "tools/list", map[string]interface{}{})
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
func (c *SSEClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	result, err := c.sendRequest(ctx, "tools/call", params)
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

// GetTools returns all tools
func (c *SSEClient) GetTools() []*MCPToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tools := make([]*MCPToolDefinition, 0, len(c.tools))
	for _, tool := range c.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Close closes the SSE client
func (c *SSEClient) Close() error {
	return nil
}
