// Package logger 日志工具
package logger

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

// Config 日志配置
type Config struct {
	Level      string // 日志级别
	Format     string // 输出格式: text, json
	Output     string // 日志文件路径
	MaxSize    int    // 单个文件最大大小(MB)，默认 10
	MaxAge     int    // 保留旧日志文件的最大天数，默认 7
	MaxBackups int    // 保留的旧日志文件最大数量，默认 5
	Compress   bool   // 是否压缩旧日志文件
	Verbose    bool   // 详细模式，记录完整的请求/响应/结果
}

// Logger 日志器
type Logger struct {
	mu          sync.Mutex
	level       Level
	format      string
	file        *os.File
	writer      *bufio.Writer // 缓冲写入器
	filePath    string
	currentSize int64
	config      Config
	stopSync    chan struct{} // 停止同步信号
}

var defaultLogger *Logger

// Init 初始化日志器
func Init(level, format, output string) error {
	return InitWithConfig(Config{
		Level:  level,
		Format: format,
		Output: output,
	})
}

// InitWithConfig 使用完整配置初始化日志器
func InitWithConfig(cfg Config) error {
	l := &Logger{
		level:    parseLevel(cfg.Level),
		format:   cfg.Format,
		config:   applyDefaults(cfg),
		stopSync: make(chan struct{}),
	}

	if l.config.Output != "" {
		// 展开路径
		l.config.Output = expandPath(l.config.Output)

		// 创建目录
		dir := filepath.Dir(l.config.Output)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// 打开日志文件
		if err := l.openFile(); err != nil {
			return err
		}

		// 启动定期同步 goroutine
		go l.periodicSync()
	}

	defaultLogger = l
	return nil
}

// applyDefaults 应用默认配置
func applyDefaults(cfg Config) Config {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 10 // 10MB
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 7 // 7 days
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 5
	}
	return cfg
}

// openFile 打开日志文件
func (l *Logger) openFile() error {
	f, err := os.OpenFile(l.config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.file = f
	l.writer = bufio.NewWriterSize(f, 4096) // 4KB 缓冲区
	l.filePath = l.config.Output

	// 获取当前文件大小
	info, err := f.Stat()
	if err != nil {
		l.currentSize = 0
	} else {
		l.currentSize = info.Size()
	}

	return nil
}

// GetLogger 获取默认日志器
func GetLogger() *Logger {
	if defaultLogger == nil {
		// 默认输出到 stdout
		defaultLogger = &Logger{
			level:  LevelInfo,
			format: "text",
			config: Config{MaxSize: 10, MaxAge: 7, MaxBackups: 5},
		}
	}
	return defaultLogger
}

// Debug 调试日志
func Debug(msg string, fields ...interface{}) {
	GetLogger().log(LevelDebug, msg, fields...)
}

// Info 信息日志
func Info(msg string, fields ...interface{}) {
	GetLogger().log(LevelInfo, msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...interface{}) {
	GetLogger().log(LevelWarn, msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...interface{}) {
	GetLogger().log(LevelError, msg, fields...)
}

// LLMRequest 记录 LLM 请求
func LLMRequest(provider, model string, req interface{}) {
	fields := []interface{}{
		"provider", provider,
		"model", model,
	}
	// 只在 verbose 模式下记录完整请求
	if GetLogger().config.Verbose {
		fields = append(fields, "request", toJSON(req))
	} else {
		// 简化模式：只记录消息数量
		if m, ok := req.(map[string]interface{}); ok {
			if msgs, ok := m["messages"].([]interface{}); ok {
				fields = append(fields, "messages", len(msgs))
			}
		}
	}
	GetLogger().log(LevelInfo, "[LLM] Request", fields...)
}

// LLMResponse 记录 LLM 响应
func LLMResponse(provider, model string, resp interface{}, duration time.Duration, err error) {
	level := LevelInfo
	msg := "[LLM] Response"
	if err != nil {
		level = LevelError
		msg = "[LLM] Response Error"
	}

	fields := []interface{}{
		"provider", provider,
		"model", model,
		"duration_ms", duration.Milliseconds(),
	}
	// 只在 verbose 模式或错误时记录响应内容
	if err != nil || GetLogger().config.Verbose {
		if resp != nil {
			fields = append(fields, "response", truncate(toJSON(resp), 500))
		}
	}
	if err != nil {
		fields = append(fields, "error", err.Error())
	}

	GetLogger().log(level, msg, fields...)
}

// ToolCall 记录工具调用
func ToolCall(toolName string, params interface{}, result string, duration time.Duration, err error) {
	level := LevelInfo
	msg := "[Tool] Call tool=" + toolName
	if err != nil {
		level = LevelError
		msg = "[Tool] Call Error tool=" + toolName
	}

	fields := []interface{}{
		"params", truncate(toJSON(params), 100),
		"duration_ms", duration.Milliseconds(),
	}
	// 只在 verbose 模式或错误时记录完整结果
	if err != nil || GetLogger().config.Verbose {
		fields = append(fields, "result", truncate(result, 200))
	} else {
		// 简化模式：只记录结果长度
		fields = append(fields, "result_len", len(result))
	}
	if err != nil {
		fields = append(fields, "error", truncate(err.Error(), 200))
	}

	GetLogger().log(level, msg, fields...)
}

// AgentMessage 记录 Agent 消息
func AgentMessage(sessionID, role, content string) {
	GetLogger().log(LevelDebug, "[Agent] Message",
		"session", sessionID,
		"role", role,
		"content", truncate(content, 200),
	)
}

func (l *Logger) log(level Level, msg string, fields ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05.000")
	levelName := levelNames[level]

	var output string
	if l.format == "json" {
		output = l.formatJSON(now, levelName, msg, fields)
	} else {
		output = l.formatText(now, levelName, msg, fields)
	}

	// 输出到文件
	if l.file != nil && l.writer != nil {
		// 检查是否需要轮转
		if l.shouldRotate(len(output)) {
			l.rotate()
		}

		n, _ := l.writer.WriteString(output + "\n")
		l.currentSize += int64(n)
		// 不再每次写入都 Sync，，改用定期同步（periodicSync）
	}

	// 同时输出到 stdout（调试用）
	// fmt.Println(output)
}

// shouldRotate 检查是否需要轮转
func (l *Logger) shouldRotate(newLen int) bool {
	maxBytes := int64(l.config.MaxSize) * 1024 * 1024
	return l.currentSize+int64(newLen) > maxBytes
}

// rotate 执行日志轮转
func (l *Logger) rotate() {
	// 先刷新缓冲区
	if l.writer != nil {
		l.writer.Flush()
	}
	// 关闭当前文件
	if l.file != nil {
		l.file.Close()
	}

	// 重命名当前文件
	backupPath := l.backupPath()
	if err := os.Rename(l.config.Output, backupPath); err != nil {
		// 如果重命名失败（可能文件不存在），直接打开新文件
	}

	// 压缩旧文件
	if l.config.Compress {
		go l.compressFile(backupPath)
	}

	// 清理旧日志
	go l.cleanOldLogs()

	// 打开新文件
	l.openFile()
}

// backupPath 生成备份文件路径
func (l *Logger) backupPath() string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	ext := filepath.Ext(l.config.Output)
	base := strings.TrimSuffix(filepath.Base(l.config.Output), ext)
	dir := filepath.Dir(l.config.Output)

	var backupName string
	if ext == "" {
		backupName = fmt.Sprintf("%s.%s", base, timestamp)
	} else {
		backupName = fmt.Sprintf("%s.%s%s", base, timestamp, ext)
	}

	return filepath.Join(dir, backupName)
}

// compressFile 压缩日志文件
func (l *Logger) compressFile(src string) {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer srcFile.Close()

	dstPath := src + ".gz"
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return
	}
	defer dstFile.Close()

	gzWriter := gzip.NewWriter(dstFile)
	defer gzWriter.Close()

	io.Copy(gzWriter, srcFile)
	gzWriter.Close()
	srcFile.Close()

	// 删除原文件
	os.Remove(src)
}

// cleanOldLogs 清理旧日志文件
func (l *Logger) cleanOldLogs() {
	dir := filepath.Dir(l.config.Output)
	base := filepath.Base(l.config.Output)
	ext := filepath.Ext(l.config.Output)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var backups []os.DirEntry
	prefix := base
	if ext != "" {
		prefix = strings.TrimSuffix(base, ext) + "."
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 匹配备份文件 (包括压缩的)
		if name != base && (strings.HasPrefix(name, prefix) || strings.HasPrefix(name, strings.TrimSuffix(base, ext)+".") && (strings.HasSuffix(name, ext) || strings.HasSuffix(name, ext+".gz"))) {
			backups = append(backups, entry)
		}
	}

	now := time.Now()
	cutoff := now.AddDate(0, 0, -l.config.MaxAge)

	for _, entry := range backups {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 按数量限制清理
		if len(backups) > l.config.MaxBackups {
			os.Remove(filepath.Join(dir, entry.Name()))
			backups = backups[1:] // 简单处理
			continue
		}

		// 按时间清理
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

func (l *Logger) formatText(time, level, msg string, fields []interface{}) string {
	result := fmt.Sprintf("%s [%s] %s", time, level, msg)
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			result += fmt.Sprintf(" %s=%v", fields[i], fields[i+1])
		}
	}
	return result
}

func (l *Logger) formatJSON(time, level, msg string, fields []interface{}) string {
	data := map[string]interface{}{
		"time":    time,
		"level":   level,
		"message": msg,
	}
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			data[fmt.Sprint(fields[i])] = fields[i+1]
		}
	}
	return toJSON(data)
}

// periodicSync 定期同步日志到磁盘
func (l *Logger) periodicSync() {
	ticker := time.NewTicker(5 * time.Second) // 每5秒同步一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			if l.writer != nil {
				l.writer.Flush()
			}
			if l.file != nil {
				l.file.Sync()
			}
			l.mu.Unlock()
		case <-l.stopSync:
			return
		}
	}
}

// Close 关闭日志器
func Close() error {
	if defaultLogger != nil {
		close(defaultLogger.stopSync) // 停止同步 goroutine
		if defaultLogger.writer != nil {
			defaultLogger.writer.Flush()
		}
		if defaultLogger.file != nil {
			return defaultLogger.file.Close()
		}
	}
	return nil
}

func parseLevel(level string) Level {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
