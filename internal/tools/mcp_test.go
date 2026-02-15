package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lingguard/internal/config"
)

// MockMCPClient is a mock MCP client for testing
type MockMCPClient struct {
	tools    []*MCPToolDefinition
	callErr  error
	callResp string
}

func (m *MockMCPClient) Connect(ctx context.Context) error {
	return nil
}

func (m *MockMCPClient) Close() error {
	return nil
}

func (m *MockMCPClient) GetTools() []*MCPToolDefinition {
	return m.tools
}

func (m *MockMCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	if m.callErr != nil {
		return "", m.callErr
	}
	return m.callResp, nil
}

func TestMCPToolWrapper_Metadata(t *testing.T) {
	client := &MockMCPClient{}
	wrapper := NewMCPToolWrapper(
		client,
		"test_server",
		"test_tool",
		"This is a test tool",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Test input",
				},
			},
		},
	)

	// Test name
	expectedName := "mcp_test_server_test_tool"
	if wrapper.Name() != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, wrapper.Name())
	}

	// Test description
	if wrapper.Description() != "This is a test tool" {
		t.Errorf("Expected description 'This is a test tool', got '%s'", wrapper.Description())
	}

	// Test parameters
	params := wrapper.Parameters()
	if params["type"] != "object" {
		t.Errorf("Expected parameters type 'object', got '%v'", params["type"])
	}

	// Test dangerous flag
	if wrapper.IsDangerous() {
		t.Error("MCP tool wrapper should not be dangerous")
	}
}

func TestMCPToolWrapper_Execute(t *testing.T) {
	client := &MockMCPClient{
		callResp: "tool result",
	}
	wrapper := NewMCPToolWrapper(
		client,
		"server",
		"tool",
		"Test tool",
		map[string]interface{}{"type": "object"},
	)

	result, err := wrapper.Execute(context.Background(), json.RawMessage(`{"input":"test"}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "tool result" {
		t.Errorf("Expected 'tool result', got '%s'", result)
	}
}

func TestMCPManager_EmptyServers(t *testing.T) {
	manager := NewMCPManager()

	// Connect with empty servers should succeed
	err := manager.ConnectServers(context.Background(), nil)
	if err != nil {
		t.Errorf("Expected no error with empty servers, got: %v", err)
	}

	// GetTools should return empty map
	tools := manager.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(tools))
	}

	// Close should succeed
	if err := manager.Close(); err != nil {
		t.Errorf("Expected no error on close, got: %v", err)
	}
}

func TestMCPManager_SkipInvalidServers(t *testing.T) {
	manager := NewMCPManager()

	servers := map[string]config.MCPServerConfig{
		"no_config": {}, // No command or URL
	}

	err := manager.ConnectServers(context.Background(), servers)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should have no tools
	tools := manager.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools, got %d", len(tools))
	}
}

func TestMCPConfigParsing(t *testing.T) {
	// Test that MCP config can be parsed from JSON
	configJSON := `{
		"command": "npx",
		"args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
		"env": {
			"NODE_OPTIONS": "--max-old-space-size=4096"
		}
	}`

	var cfg config.MCPServerConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("Failed to parse MCP config: %v", err)
	}

	if cfg.Command != "npx" {
		t.Errorf("Expected command 'npx', got '%s'", cfg.Command)
	}

	if len(cfg.Args) != 3 {
		t.Errorf("Expected 3 args, got %d", len(cfg.Args))
	}

	if cfg.Args[0] != "-y" {
		t.Errorf("Expected first arg '-y', got '%s'", cfg.Args[0])
	}

	if cfg.Env["NODE_OPTIONS"] != "--max-old-space-size=4096" {
		t.Errorf("Expected NODE_OPTIONS env var, got '%s'", cfg.Env["NODE_OPTIONS"])
	}
}

func TestMCPConfig_HTTP(t *testing.T) {
	// Test HTTP-based MCP config
	configJSON := `{
		"url": "https://mcp.example.com/sse"
	}`

	var cfg config.MCPServerConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("Failed to parse MCP config: %v", err)
	}

	if cfg.URL != "https://mcp.example.com/sse" {
		t.Errorf("Expected URL 'https://mcp.example.com/sse', got '%s'", cfg.URL)
	}

	if cfg.Command != "" {
		t.Errorf("Expected empty command for HTTP config, got '%s'", cfg.Command)
	}
}

// TestMCPClient_NoCommand tests that client fails gracefully without command
func TestMCPClient_NoCommand(t *testing.T) {
	cfg := config.MCPServerConfig{
		Command: "",
	}
	client := NewMCPClient("test", cfg)

	err := client.Connect(context.Background())
	if err == nil {
		t.Error("Expected error when no command configured")
		client.Close()
	}
}

// TestSSEClient_NoURL tests that SSE client fails gracefully without URL
func TestSSEClient_NoURL(t *testing.T) {
	cfg := config.MCPServerConfig{
		URL: "",
	}
	client := NewSSEClient("test", cfg)

	err := client.Connect(context.Background())
	if err == nil {
		t.Error("Expected error when no URL configured")
		client.Close()
	}
}

func TestMCPHTTPClient_NoURL(t *testing.T) {
	cfg := config.MCPServerConfig{
		URL: "",
	}
	client := NewMCPHTTPClient("test", cfg)

	err := client.Connect(context.Background())
	if err == nil {
		t.Error("Expected error when no URL configured")
		client.Close()
	}
}
