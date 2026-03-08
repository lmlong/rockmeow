// Package sse Server-Sent Events 支持
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DefaultHeartbeatInterval 默认心跳间隔
const DefaultHeartbeatInterval = 30 * time.Second

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

// EventChannel SSE 事件通道类型
type EventChannel <-chan interface{}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Interval     time.Duration // 心跳间隔
	SendPing     bool          // 是否发送 ping 事件
	PingData     interface{}   // ping 事件数据
	OnConnect    func(w *Writer)
	OnDisconnect func()
}

// DefaultHeartbeatConfig 返回默认心跳配置
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Interval: DefaultHeartbeatInterval,
		SendPing: true,
		PingData: map[string]int64{"time": 0},
	}
}

// ServeWithHeartbeat 带心跳的 SSE 服务（通用实现）
// ctx: gin.Context
// eventCh: 事件通道
// eventType: 事件类型名称
// cfg: 心跳配置（可选，nil 使用默认配置）
func ServeWithHeartbeat(ctx *gin.Context, eventCh EventChannel, eventType string, cfg *HeartbeatConfig) {
	SetupHeaders(ctx)
	writer := NewWriter(ctx.Writer)

	if cfg == nil {
		cfg = DefaultHeartbeatConfig()
	}

	// 发送连接事件
	if cfg.OnConnect != nil {
		cfg.OnConnect(writer)
	} else {
		writer.WriteEvent("connected", map[string]string{"message": "connected"})
	}

	// 心跳定时器
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	// 生成 ping 数据的函数
	getPingData := func() interface{} {
		if data, ok := cfg.PingData.(map[string]int64); ok {
			data["time"] = time.Now().Unix()
			return data
		}
		return cfg.PingData
	}

	for {
		select {
		case <-ctx.Request.Context().Done():
			if cfg.OnDisconnect != nil {
				cfg.OnDisconnect()
			}
			return

		case event, ok := <-eventCh:
			if !ok {
				// 通道关闭，结束 SSE
				return
			}
			if err := writer.WriteEvent(eventType, event); err != nil {
				return
			}

		case <-ticker.C:
			if cfg.SendPing {
				if err := writer.WriteEvent("ping", getPingData()); err != nil {
					return
				}
			}
		}
	}
}

// ServeSimple 简单 SSE 服务（带默认心跳）
// 使用默认心跳配置，适用于大多数场景
func ServeSimple(ctx *gin.Context, eventCh EventChannel, eventType string) {
	ServeWithHeartbeat(ctx, eventCh, eventType, nil)
}

// HeartbeatRunner 心跳协程运行器（用于需要自定义事件处理的场景）
// 返回停止函数
func HeartbeatRunner(ctx context.Context, writer *Writer, interval time.Duration) context.CancelFunc {
	heartbeatCtx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if err := writer.WriteEvent("ping", map[string]int64{"time": time.Now().Unix()}); err != nil {
					return
				}
			}
		}
	}()

	return cancel
}
