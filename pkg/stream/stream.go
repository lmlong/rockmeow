// Package stream 流式响应类型定义
package stream

// StreamEventType 流式事件类型
type StreamEventType string

const (
	// EventText 文本增量内容
	EventText StreamEventType = "text"
	// EventToolStart 工具开始执行
	EventToolStart StreamEventType = "tool_start"
	// EventToolEnd 工具执行完成
	EventToolEnd StreamEventType = "tool_end"
	// EventDone 流式响应完成
	EventDone StreamEventType = "done"
	// EventError 发生错误
	EventError StreamEventType = "error"
)

// StreamEvent 流式事件
type StreamEvent struct {
	// Type 事件类型
	Type StreamEventType
	// Content 增量文本内容 (EventText)
	Content string
	// ToolName 工具名称 (EventToolStart/EventToolEnd)
	ToolName string
	// ToolResult 工具执行结果 (EventToolEnd)
	ToolResult string
	// ToolError 工具错误信息
	ToolError string
	// Error 错误信息 (EventError)
	Error error
}

// StreamCallback 流式响应回调函数
type StreamCallback func(event StreamEvent)

// NewTextEvent 创建文本事件
func NewTextEvent(content string) StreamEvent {
	return StreamEvent{
		Type:    EventText,
		Content: content,
	}
}

// NewToolStartEvent 创建工具开始事件
func NewToolStartEvent(toolName string) StreamEvent {
	return StreamEvent{
		Type:     EventToolStart,
		ToolName: toolName,
	}
}

// NewToolEndEvent 创建工具结束事件
func NewToolEndEvent(toolName, result string, err error) StreamEvent {
	event := StreamEvent{
		Type:       EventToolEnd,
		ToolName:   toolName,
		ToolResult: result,
	}
	if err != nil {
		event.ToolError = err.Error()
	}
	return event
}

// NewDoneEvent 创建完成事件
func NewDoneEvent() StreamEvent {
	return StreamEvent{
		Type: EventDone,
	}
}

// NewErrorEvent 创建错误事件
func NewErrorEvent(err error) StreamEvent {
	return StreamEvent{
		Type:  EventError,
		Error: err,
	}
}
