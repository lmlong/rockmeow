// Package tools MCP HTTP transport implementation
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

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
)

// MCPHTTPClient MCP HTTP client for Streamable HTTP transport
type MCPHTTPClient struct {
	mu         sync.RWMutex
	serverName string
	config     config.MCPServerConfig
	tools      map[string]*MCPToolDefinition
	client     *http.Client
	baseURL    string
	sessionID  string
	requestID  int64
}

// NewMCPHTTPClient creates a new MCP HTTP client
func NewMCPHTTPClient(serverName string, cfg config.MCPServerConfig) *MCPHTTPClient {
	return &MCPHTTPClient{
		serverName: serverName,
		config:     cfg,
		tools:      make(map[string]*MCPToolDefinition),
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

// Connect connects to the MCP HTTP server
func (c *MCPHTTPClient) Connect(ctx context.Context) error {
	if c.config.URL == "" {
		return fmt.Errorf("MCP server '%s': no URL configured", c.serverName)
	}

	c.baseURL = strings.TrimSuffix(c.config.URL, "/")

	if err := c.initialize(ctx); err != nil {
		return fmt.Errorf("initialize MCP server: %w", err)
	}

	if err := c.listTools(ctx); err != nil {
		return fmt.Errorf("list MCP tools: %w", err)
	}

	logger.Info("MCP HTTP server connected", "server", c.serverName, "tools", len(c.tools))
	return nil
}

// Close closes the MCP HTTP client
func (c *MCPHTTPClient) Close() error {
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
		"clientInfo":      map[string]interface{}{"name": "lingguard", "version": "1.0.0"},
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

	logger.Debug("MCP HTTP server initialized",
		"server", c.serverName,
		"protocol", initResult.ProtocolVersion,
		"serverName", initResult.ServerInfo.Name,
		"serverVersion", initResult.ServerInfo.Version)

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
	result, err := c.sendHTTPRequest(ctx, "tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
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
