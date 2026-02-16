package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestOpenCodeClient_Health(t *testing.T) {
	client := NewOpenCodeClient(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthy, version, err := client.Health(ctx)
	if err != nil {
		t.Skipf("OpenCode server not available: %v", err)
		return
	}

	t.Logf("OpenCode server: healthy=%v, version=%s", healthy, version)

	if !healthy {
		t.Error("Server reported unhealthy")
	}
}

func TestOpenCodeClient_CreateSession(t *testing.T) {
	client := NewOpenCodeClient(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check server first
	_, _, err := client.Health(ctx)
	if err != nil {
		t.Skipf("OpenCode server not available: %v", err)
		return
	}

	session, err := client.CreateSession(ctx, "Test Session")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID == "" {
		t.Error("Session ID is empty")
	}

	t.Logf("Session created: ID=%s, Title=%s", session.ID, session.Title)

	// Cleanup
	_ = client.DeleteSession(ctx, session.ID)
}

func TestOpenCodeClient_SendMessage(t *testing.T) {
	client := NewOpenCodeClient(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Check server first
	_, _, err := client.Health(ctx)
	if err != nil {
		t.Skipf("OpenCode server not available: %v", err)
		return
	}

	// Create session
	session, err := client.CreateSession(ctx, "Message Test Session")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	defer client.DeleteSession(ctx, session.ID)

	// Send message
	opts := SendMessageOptions{
		Agent: "build",
		Parts: []MessagePart{
			{Type: "text", Text: "What is 2+2? Answer with just the number."},
		},
	}

	resp, err := client.SendMessage(ctx, session.ID, opts)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	t.Logf("Message sent, response has %d parts", len(resp.Parts))

	// Find text parts
	for _, part := range resp.Parts {
		if part.Type == "text" {
			t.Logf("Response text: %s", part.Text)
		}
		if part.Type == "reasoning" {
			t.Logf("Reasoning: %s", part.Text)
		}
	}
}

func TestOpenCodeTool_Execute(t *testing.T) {
	tool := NewOpenCodeTool(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Check server first
	healthy, _, err := tool.GetClient().Health(ctx)
	if err != nil || !healthy {
		t.Skipf("OpenCode server not available: %v", err)
		return
	}

	// Test prompt action
	params := map[string]interface{}{
		"action": "prompt",
		"task":   "What is 3+3? Answer with just the number.",
		"agent":  "build",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(ctx, paramsJSON)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	t.Logf("Result:\n%s", result)
}

func TestOpenCodeTool_ExecuteShell(t *testing.T) {
	tool := NewOpenCodeTool(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check server first
	healthy, _, err := tool.GetClient().Health(ctx)
	if err != nil || !healthy {
		t.Skipf("OpenCode server not available: %v", err)
		return
	}

	// Get or create session
	session, err := tool.GetClient().GetOrCreateSession(ctx, "Shell Test")
	if err != nil {
		t.Fatalf("GetOrCreateSession failed: %v", err)
	}

	// Test shell action
	shellResult, err := tool.GetClient().ExecuteShell(ctx, session.ID, "echo hello")
	if err != nil {
		t.Fatalf("ExecuteShell failed: %v", err)
	}

	t.Logf("Shell stdout: %s", shellResult.Stdout)
}
