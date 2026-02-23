package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTasksBoardTool_Name(t *testing.T) {
	tool := NewTasksBoardTool(&TasksBoardConfig{URL: "http://localhost:3000/api/tasks"})
	if tool.Name() != "tasks_board" {
		t.Errorf("Expected name 'tasks_board', got '%s'", tool.Name())
	}
}

func TestTasksBoardTool_Description(t *testing.T) {
	tool := NewTasksBoardTool(&TasksBoardConfig{URL: "http://localhost:3000/api/tasks"})
	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestTasksBoardTool_Parameters(t *testing.T) {
	tool := NewTasksBoardTool(&TasksBoardConfig{URL: "http://localhost:3000/api/tasks"})
	params := tool.Parameters()
	if params == nil {
		t.Error("Parameters should not be nil")
	}

	// Check that action is required
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Error("Parameters should have properties")
	}
	if _, exists := props["action"]; !exists {
		t.Error("Parameters should have 'action' property")
	}
}

func TestTasksBoardTool_IsDangerous(t *testing.T) {
	tool := NewTasksBoardTool(&TasksBoardConfig{URL: "http://localhost:3000/api/tasks"})
	if tool.IsDangerous() {
		t.Error("TasksBoardTool should not be dangerous")
	}
}

func TestTasksBoardTool_Execute_NoURL(t *testing.T) {
	tool := NewTasksBoardTool(nil)
	params := json.RawMessage(`{"action": "get"}`)
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("Expected error when URL is not configured")
	}
}

func TestTasksBoardTool_Execute_InvalidAction(t *testing.T) {
	tool := NewTasksBoardTool(&TasksBoardConfig{URL: "http://localhost:3000/api/tasks"})
	params := json.RawMessage(`{"action": "invalid"}`)
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("Expected error for invalid action")
	}
}

func TestTasksBoardTool_Execute_CreateMissingTitle(t *testing.T) {
	tool := NewTasksBoardTool(&TasksBoardConfig{URL: "http://localhost:3000/api/tasks"})
	params := json.RawMessage(`{"action": "create", "task": {}}`)
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("Expected error when title is missing")
	}
}

func TestTasksBoardTool_Execute_UpdateMissingID(t *testing.T) {
	tool := NewTasksBoardTool(&TasksBoardConfig{URL: "http://localhost:3000/api/tasks"})
	params := json.RawMessage(`{"action": "update", "task": {"status": "completed"}}`)
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("Expected error when taskId is missing")
	}
}

func TestTasksBoardTool_Execute_SyncEmptyTasks(t *testing.T) {
	tool := NewTasksBoardTool(&TasksBoardConfig{URL: "http://localhost:3000/api/tasks"})
	params := json.RawMessage(`{"action": "sync", "tasks": []}`)
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Error("Expected error when tasks list is empty")
	}
}
