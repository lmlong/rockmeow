package models

// SSE 事件类型常量
const (
	EventConnected  = "connected"
	EventThinking   = "thinking"
	EventToolCall   = "tool_call"
	EventToolResult = "tool_result"
	EventContent    = "content"
	EventCompleted  = "completed"
	EventError      = "error"
	EventPing       = "ping"
	EventTask       = "task"
	EventTrace      = "trace"
)

// SSEEvent SSE 事件
type SSEEvent struct {
	Type string      `json:"event"`
	Data interface{} `json:"data"`
}
