package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWebFetchTool_Basic(t *testing.T) {
	tool := NewWebFetchTool(10000)

	// Test tool metadata
	if tool.Name() != "web_fetch" {
		t.Errorf("Expected name 'web_fetch', got '%s'", tool.Name())
	}

	if tool.IsDangerous() {
		t.Error("web_fetch should not be dangerous")
	}

	// Test parameters schema
	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("Expected type 'object', got '%v'", params["type"])
	}
}

func TestWebFetchTool_Execute(t *testing.T) {
	tool := NewWebFetchTool(5000)
	ctx := context.Background()

	// Test fetching example.com (a simple, reliable test site)
	testParams := map[string]interface{}{
		"url":         "https://example.com",
		"extractMode": "text",
		"maxChars":    2000,
	}
	paramsJSON, _ := json.Marshal(testParams)

	result, err := tool.Execute(ctx, paramsJSON)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Parse result
	var resultData map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resultData); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Check for errors
	if errMsg, ok := resultData["error"]; ok && errMsg != "" {
		t.Logf("Note: Request might have failed due to network: %v", errMsg)
		t.Skip("Skipping due to network issue")
	}

	// Verify result structure
	if status, ok := resultData["status"].(float64); ok && status != 200 {
		t.Errorf("Expected status 200, got %v", status)
	}

	if text, ok := resultData["text"].(string); ok {
		if len(text) == 0 {
			t.Error("Expected non-empty text content")
		}
		t.Logf("Fetched %d characters", len(text))
		t.Logf("Preview: %s...", truncateStr(text, 200))
	}
}

func TestWebFetchTool_InvalidURL(t *testing.T) {
	tool := NewWebFetchTool(5000)
	ctx := context.Background()

	testParams := map[string]interface{}{
		"url": "ftp://invalid-protocol.com/file",
	}
	paramsJSON, _ := json.Marshal(testParams)

	result, err := tool.Execute(ctx, paramsJSON)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should return error in JSON
	var resultData map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resultData); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if errMsg, ok := resultData["error"]; ok {
		t.Logf("Got expected error: %v", errMsg)
	} else {
		t.Error("Expected error for invalid URL scheme")
	}
}

func TestWebFetchTool_JSONContent(t *testing.T) {
	tool := NewWebFetchTool(5000)
	ctx := context.Background()

	// Test fetching JSON API
	testParams := map[string]interface{}{
		"url": "https://httpbin.org/json",
	}
	paramsJSON, _ := json.Marshal(testParams)

	result, err := tool.Execute(ctx, paramsJSON)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resultData); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Check for network errors
	if errMsg, ok := resultData["error"]; ok && errMsg != "" {
		t.Logf("Note: Request might have failed due to network: %v", errMsg)
		t.Skip("Skipping due to network issue")
	}

	if extractor, ok := resultData["extractor"].(string); ok {
		if extractor != "json" {
			t.Errorf("Expected extractor 'json', got '%s'", extractor)
		}
	}
}

func TestWebSearchTool_Basic(t *testing.T) {
	tool := NewWebSearchTool("", 5)

	// Test tool metadata
	if tool.Name() != "web_search" {
		t.Errorf("Expected name 'web_search', got '%s'", tool.Name())
	}

	if tool.IsDangerous() {
		t.Error("web_search should not be dangerous")
	}
}

func TestWebSearchTool_NoAPIKey(t *testing.T) {
	tool := NewWebSearchTool("", 5) // No API key
	ctx := context.Background()

	testParams := map[string]interface{}{
		"query": "test query",
	}
	paramsJSON, _ := json.Marshal(testParams)

	result, err := tool.Execute(ctx, paramsJSON)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should return error message about missing API key
	if result == "" {
		t.Error("Expected error message about missing API key")
	}

	t.Logf("Result: %s", result)
}

func TestWebFetchTool_HTMLExtraction(t *testing.T) {
	tool := NewWebFetchTool(10000)
	ctx := context.Background()

	// Test markdown extraction
	testParams := map[string]interface{}{
		"url":         "https://example.com",
		"extractMode": "markdown",
	}
	paramsJSON, _ := json.Marshal(testParams)

	result, err := tool.Execute(ctx, paramsJSON)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var resultData map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resultData); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	// Check for network errors
	if errMsg, ok := resultData["error"]; ok && errMsg != "" {
		t.Logf("Note: Request might have failed due to network: %v", errMsg)
		t.Skip("Skipping due to network issue")
	}

	if extractor, ok := resultData["extractor"].(string); ok {
		t.Logf("Extractor used: %s", extractor)
	}

	if text, ok := resultData["text"].(string); ok {
		// Should contain example domain heading
		t.Logf("Content preview: %s", truncateStr(text, 300))
	}
}

func TestURLValidation(t *testing.T) {
	tool := NewWebFetchTool(1000)

	testCases := []struct {
		url        string
		shouldFail bool
	}{
		{"https://example.com", false},
		{"http://example.com", false},
		{"ftp://example.com", true},
		{"javascript:alert(1)", true},
		{"file:///etc/passwd", true},
		{"", true},
	}

	for _, tc := range testCases {
		err := tool.validateURL(tc.url)
		if tc.shouldFail && err == nil {
			t.Errorf("Expected URL '%s' to fail validation", tc.url)
		}
		if !tc.shouldFail && err != nil {
			t.Errorf("Expected URL '%s' to pass validation, got error: %v", tc.url, err)
		}
	}
}

func TestHTMLStripping(t *testing.T) {
	tool := NewWebFetchTool(1000)

	testCases := []struct {
		input    string
		expected string
	}{
		{"<p>Hello World</p>", "Hello World"},
		{"<b>Bold</b> text", "Bold text"},
		{"<a href='#'>Link</a>", "Link"},
		{"&amp; &lt; &gt;", "& < >"},
	}

	for _, tc := range testCases {
		result := tool.stripTags(tc.input)
		if result != tc.expected {
			t.Errorf("stripTags(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// MockMessageSender 用于测试的模拟消息发送器
type MockMessageSender struct {
	lastChannel string
	lastTo      string
	lastContent string
	err         error
}

func (m *MockMessageSender) SendMessage(channelName string, to string, content string) error {
	m.lastChannel = channelName
	m.lastTo = to
	m.lastContent = content
	return m.err
}

func TestMessageTool_Basic(t *testing.T) {
	mockSender := &MockMessageSender{}
	tool := NewMessageTool(mockSender)

	if tool.Name() != "message" {
		t.Errorf("Expected name 'message', got '%s'", tool.Name())
	}

	if tool.IsDangerous() {
		t.Error("message tool should not be dangerous")
	}
}

func TestMessageTool_NoContext(t *testing.T) {
	mockSender := &MockMessageSender{}
	tool := NewMessageTool(mockSender)
	ctx := context.Background()

	params := json.RawMessage(`{"content":"Hello"}`)
	result, err := tool.Execute(ctx, params)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should return warning when no context
	if !strings.Contains(result, "Warning") {
		t.Errorf("Expected warning about no context, got: %s", result)
	}
}

func TestMessageTool_WithSender(t *testing.T) {
	mockSender := &MockMessageSender{}
	tool := NewMessageTool(mockSender)
	tool.SetContext("feishu", "user123")
	ctx := context.Background()

	params := json.RawMessage(`{"content":"Test message"}`)
	result, err := tool.Execute(ctx, params)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if mockSender.lastChannel != "feishu" {
		t.Errorf("Expected channel 'feishu', got '%s'", mockSender.lastChannel)
	}
	if mockSender.lastTo != "user123" {
		t.Errorf("Expected to 'user123', got '%s'", mockSender.lastTo)
	}
	if mockSender.lastContent != "Test message" {
		t.Errorf("Expected content 'Test message', got '%s'", mockSender.lastContent)
	}

	t.Logf("Result: %s", result)
}
