// Package sse Server-Sent Events 支持
package sse

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Writer SSE 写入器
type Writer struct {
	w       gin.ResponseWriter
	flusher http.Flusher
}

// NewWriter 创建 SSE 写入器
func NewWriter(w gin.ResponseWriter) *Writer {
	return &Writer{
		w:       w,
		flusher: w,
	}
}

// WriteEvent 写入 SSE 事件
func (s *Writer) WriteEvent(event string, data interface{}) error {
	fmt.Fprintf(s.w, "event: %s\n", event)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	fmt.Fprintf(s.w, "data: %s\n\n", jsonData)
	s.flusher.Flush()
	return nil
}

// WriteMessage 写入简单消息
func (s *Writer) WriteMessage(message string) error {
	fmt.Fprintf(s.w, "data: %s\n\n", message)
	s.flusher.Flush()
	return nil
}

// SetupHeaders 设置 SSE 响应头
func SetupHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
}
