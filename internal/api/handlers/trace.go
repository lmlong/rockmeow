package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/api/sse"
	"github.com/lingguard/internal/trace"
)

// TraceHandler 追踪处理器（Gin 版本）
type TraceHandler struct {
	service *trace.Service
}

// NewTraceHandler 创建追踪处理器
func NewTraceHandler(service *trace.Service) *TraceHandler {
	return &TraceHandler{service: service}
}

// RegisterRoutes 注册路由
func (h *TraceHandler) RegisterRoutes(r *gin.RouterGroup) {
	traces := r.Group("/api")
	{
		traces.GET("/traces", h.ListTraces)
		traces.GET("/traces/stats", h.GetStats)
		traces.GET("/traces/:id", h.GetTrace)
		traces.GET("/traces/:id/spans", h.GetTraceSpans)
		traces.DELETE("/traces/:id", h.DeleteTrace)
		traces.DELETE("/traces/cleanup", h.CleanupTraces)
		traces.GET("/spans/:id", h.GetSpan)
		traces.GET("/trace/events", h.SSE)
	}
}

// ListTraces 列出追踪
func (h *TraceHandler) ListTraces(c *gin.Context) {
	filter := &trace.TraceFilter{}

	if sessionID := c.Query("sessionId"); sessionID != "" {
		filter.SessionID = sessionID
	}
	if statusStr := c.Query("status"); statusStr != "" {
		status := trace.Status(statusStr)
		filter.Status = &status
	}
	if typeStr := c.Query("type"); typeStr != "" {
		traceType := trace.TraceType(typeStr)
		filter.TraceType = &traceType
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	// 设置默认 limit
	if filter.Limit == 0 {
		filter.Limit = 50
	}

	traces, err := h.service.ListTraces(filter)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, traces)
}

// GetTrace 获取追踪详情
func (h *TraceHandler) GetTrace(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "trace id is required"})
		return
	}

	detail, err := h.service.GetTraceDetail(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, detail)
}

// GetTraceSpans 获取追踪的 Spans
func (h *TraceHandler) GetTraceSpans(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "trace id is required"})
		return
	}

	spans, err := h.service.GetSpansByTrace(id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, spans)
}

// GetStats 获取追踪统计
func (h *TraceHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetTraceStats()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, stats)
}

// DeleteTrace 删除追踪
func (h *TraceHandler) DeleteTrace(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "trace id is required"})
		return
	}

	if err := h.service.DeleteTrace(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "trace deleted"})
}

// CleanupTraces 清理旧追踪
func (h *TraceHandler) CleanupTraces(c *gin.Context) {
	days := 7
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	count, err := h.service.CleanupOldTraces(days)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message": "cleanup completed",
		"deleted": count,
	})
}

// GetSpan 获取 Span 详情
func (h *TraceHandler) GetSpan(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "span id is required"})
		return
	}

	span, err := h.service.GetStore().GetSpan(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, span)
}

// SSE Trace SSE 事件流
func (h *TraceHandler) SSE(c *gin.Context) {
	sse.SetupHeaders(c)

	writer := sse.NewWriter(c.Writer)

	// 订阅事件
	eventCh := h.service.Subscribe()

	// 发送初始连接消息
	writer.WriteEvent("connected", gin.H{"message": "connected"})

	// 心跳定时器
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event := <-eventCh:
			writer.WriteEvent("trace", event)
		case <-ticker.C:
			writer.WriteEvent("ping", gin.H{"time": time.Now().Unix()})
		}
	}
}
