package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/llm"
)

// MockProvider 用于测试的模拟 Provider
type MockProvider struct {
	response *llm.Response
	err      error
}

func (m *MockProvider) Name() string         { return "mock" }
func (m *MockProvider) Model() string        { return "test-model" }
func (m *MockProvider) SupportsTools() bool  { return true }
func (m *MockProvider) SupportsVision() bool { return false }

func (m *MockProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *MockProvider) Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func TestNewAgent(t *testing.T) {
	cfg := &config.AgentsConfig{
		Provider:          "mock",
		SystemPrompt:      "You are a test assistant",
		MemoryWindow:      10,
		MaxToolIterations: 5,
		Workspace:         "~/.lingguard/workspace",
	}

	mockProvider := &MockProvider{}

	agent := NewAgent(cfg, mockProvider, nil)

	if agent == nil {
		t.Fatal("NewAgent returned nil")
	}

	if agent.config.Provider != "mock" {
		t.Errorf("Expected Provider=mock, got %s", agent.config.Provider)
	}
}

func TestAgentRegisterTool(t *testing.T) {
	cfg := &config.AgentsConfig{Provider: "mock"}
	mockProvider := &MockProvider{}

	agent := NewAgent(cfg, mockProvider, nil)

	// 注册工具
	wsMgr := tools.NewWorkspaceManager("", "")
	agent.RegisterTool(tools.NewShellTool(wsMgr, false))

	// 验证工具已注册
	registryTools := agent.toolRegistry.List()
	if len(registryTools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(registryTools))
	}
}

func TestAgentProcessMessage(t *testing.T) {
	cfg := &config.AgentsConfig{
		Provider:          "mock",
		SystemPrompt:      "You are helpful",
		MemoryWindow:      10,
		MaxToolIterations: 5,
	}

	mockProvider := &MockProvider{
		response: &llm.Response{
			ID:    "resp1",
			Model: "test-model",
			Choices: []struct {
				Index        int         `json:"index"`
				Message      llm.Message `json:"message"`
				FinishReason string      `json:"finish_reason"`
			}{
				{
					Index: 0,
					Message: llm.Message{
						Role:    "assistant",
						Content: "Hello! How can I help?",
					},
					FinishReason: "stop",
				},
			},
		},
	}

	agent := NewAgent(cfg, mockProvider, nil)

	ctx := context.Background()
	response, err := agent.ProcessMessage(ctx, "test-session", "Hi")

	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	if response != "Hello! How can I help?" {
		t.Errorf("Unexpected response: %s", response)
	}

	// 验证消息存储
	s := agent.sessions.GetOrCreate("test-session")
	if len(s.Messages) != 2 {
		t.Errorf("Expected 2 stored messages, got %d", len(s.Messages))
	}
}

func TestAgentToolExecution(t *testing.T) {
	cfg := &config.AgentsConfig{
		Provider:          "mock",
		SystemPrompt:      "You are helpful",
		MemoryWindow:      10,
		MaxToolIterations: 5,
	}

	// 第一次响应：调用工具
	toolCallResponse := &llm.Response{
		ID:    "resp1",
		Model: "test-model",
		Choices: []struct {
			Index        int         `json:"index"`
			Message      llm.Message `json:"message"`
			FinishReason string      `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call1",
							Type: "function",
							Function: llm.FunctionCall{
								Name:      "echo_test",
								Arguments: json.RawMessage(`{"text":"hello"}`),
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}

	mockProvider := &MockProvider{response: toolCallResponse}
	agent := NewAgent(cfg, mockProvider, nil)

	// 注册一个简单的测试工具
	agent.RegisterTool(&EchoTool{})

	ctx := context.Background()

	// 第二次响应：最终回复
	mockProvider.response = &llm.Response{
		ID:    "resp2",
		Model: "test-model",
		Choices: []struct {
			Index        int         `json:"index"`
			Message      llm.Message `json:"message"`
			FinishReason string      `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: llm.Message{
					Role:    "assistant",
					Content: "I echoed your message",
				},
				FinishReason: "stop",
			},
		},
	}

	response, err := agent.ProcessMessage(ctx, "test-session", "Echo hello")
	if err != nil {
		t.Fatalf("ProcessMessage failed: %v", err)
	}

	t.Logf("Response: %s", response)
}

// EchoTool 简单的回显工具，用于测试
type EchoTool struct{}

func (t *EchoTool) Name() string        { return "echo_test" }
func (t *EchoTool) Description() string { return "Echo text" }
func (t *EchoTool) IsDangerous() bool   { return false }
func (t *EchoTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"text"},
	}
}
func (t *EchoTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Text string `json:"text"`
	}
	json.Unmarshal(params, &p)
	return "Echo: " + p.Text, nil
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == id2 {
		t.Error("IDs should be unique")
	}

	if len(id1) != 8 {
		t.Errorf("Expected ID length 8, got %d", len(id1))
	}
}
