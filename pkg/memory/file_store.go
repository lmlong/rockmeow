// Package memory 记忆系统 - 基于 nanobot 的文件持久化方案
package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FileStore 基于文件的持久化存储（参考 nanobot）
// 使用 MEMORY.md 存储长期记忆
type FileStore struct {
	memoryDir  string // 记忆目录路径
	memoryFile string // MEMORY.md 文件路径
}

// NewFileStore 创建文件存储
func NewFileStore(memoryDir string) *FileStore {
	return &FileStore{
		memoryDir:  memoryDir,
		memoryFile: filepath.Join(memoryDir, "MEMORY.md"),
	}
}

// Init 初始化存储目录和文件
func (s *FileStore) Init() error {
	// 展开路径中的 ~
	expandedDir := expandHome(s.memoryDir)
	if expandedDir != s.memoryDir {
		s.memoryDir = expandedDir
		s.memoryFile = filepath.Join(expandedDir, "MEMORY.md")
	}

	if err := os.MkdirAll(s.memoryDir, 0755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	// 初始化 MEMORY.md（如果不存在）
	if _, err := os.Stat(s.memoryFile); os.IsNotExist(err) {
		initialContent := `# Memory

This file stores long-term memories and important facts.

## User Preferences
<!-- 用户偏好设置 -->

## Project Context
<!-- 项目上下文信息 -->

## Important Facts
<!-- 重要事实记录 -->

## Decisions
<!-- 决策记录 -->

## Contact Info
<!-- 联系方式等实体信息 -->

## Other
<!-- 其他重要信息 -->
`
		if err := os.WriteFile(s.memoryFile, []byte(initialContent), 0644); err != nil {
			return fmt.Errorf("create memory file: %w", err)
		}
	}

	return nil
}

// Memory operations

// AddMemory 添加长期记忆到 MEMORY.md
func (s *FileStore) AddMemory(category, content string) error {
	if err := s.ensureInit(); err != nil {
		return err
	}

	// 读取现有内容
	data, err := os.ReadFile(s.memoryFile)
	if err != nil {
		return fmt.Errorf("read memory file: %w", err)
	}

	contentStr := string(data)
	timestamp := time.Now().Format("2006-01-02 15:04")

	// 查找或创建分类
	categoryHeader := fmt.Sprintf("## %s", category)
	newEntry := fmt.Sprintf("- [%s] %s", timestamp, content)

	if strings.Contains(contentStr, categoryHeader) {
		// 在分类后添加（跳过注释行）
		lines := strings.Split(contentStr, "\n")
		var newLines []string
		inserted := false

		for i := 0; i < len(lines); i++ {
			line := lines[i]
			newLines = append(newLines, line)

			if !inserted && strings.HasPrefix(strings.TrimSpace(line), categoryHeader) {
				// 找到分类标题，跳过后续的注释行
				for i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "<!--") {
					i++
					newLines = append(newLines, lines[i])
				}
				// 插入新条目
				newLines = append(newLines, newEntry)
				inserted = true
			}
		}
		contentStr = strings.Join(newLines, "\n")
	} else {
		// 添加新分类
		contentStr = contentStr + fmt.Sprintf("\n\n## %s\n%s", category, newEntry)
	}

	return os.WriteFile(s.memoryFile, []byte(contentStr), 0644)
}

// GetMemory 获取 MEMORY.md 全部内容
func (s *FileStore) GetMemory() (string, error) {
	if err := s.ensureInit(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(s.memoryFile)
	if err != nil {
		return "", fmt.Errorf("read memory file: %w", err)
	}
	return string(data), nil
}

// SearchMemory 搜索记忆（使用 Go 原生实现，避免命令注入）
func (s *FileStore) SearchMemory(query string) ([]string, error) {
	if err := s.ensureInit(); err != nil {
		return nil, err
	}

	// 读取文件内容
	data, err := os.ReadFile(s.memoryFile)
	if err != nil {
		return nil, fmt.Errorf("read memory file: %w", err)
	}

	return grepSearch(string(data), query)
}

// SearchAll 搜索所有记忆文件
func (s *FileStore) SearchAll(query string) (map[string][]string, error) {
	results := make(map[string][]string)

	// 搜索 MEMORY.md
	if memResults, err := s.SearchMemory(query); err == nil && len(memResults) > 0 {
		results["MEMORY.md"] = memResults
	}

	// 搜索每日日志文件
	entries, err := os.ReadDir(s.memoryDir)
	if err != nil {
		return results, nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fileName := entry.Name()
		// 只搜索 .md 文件，排除 MEMORY.md（已搜索）
		if strings.HasSuffix(fileName, ".md") && fileName != "MEMORY.md" {
			filePath := filepath.Join(s.memoryDir, fileName)
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			if lines, err := grepSearch(string(data), query); err == nil && len(lines) > 0 {
				results[fileName] = lines
			}
		}
	}

	return results, nil
}

// Daily log methods (参考 nanobot 的每日日志)

// WriteDailyLog 写入每日日志
func (s *FileStore) WriteDailyLog(content string) error {
	today := time.Now().Format("2006-01-02")
	dailyFile := filepath.Join(s.memoryDir, today+".md")

	// 检查文件是否存在，如果不存在则创建带日期头的文件
	var f *os.File
	if _, err := os.Stat(dailyFile); os.IsNotExist(err) {
		f, err = os.Create(dailyFile)
		if err != nil {
			return err
		}
		header := fmt.Sprintf("# Daily Log - %s\n\n", today)
		f.WriteString(header)
	} else {
		f, err = os.OpenFile(dailyFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	timestamp := time.Now().Format("15:04:05")
	_, writeErr := f.WriteString(fmt.Sprintf("\n## [%s]\n%s\n", timestamp, content))
	return writeErr
}

// GetRecentDailyLogs 获取最近几天的日志
func (s *FileStore) GetRecentDailyLogs(days int) (map[string]string, error) {
	logs := make(map[string]string)

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		dailyFile := filepath.Join(s.memoryDir, date+".md")

		if data, err := os.ReadFile(dailyFile); err == nil {
			logs[date] = string(data)
		}
	}

	return logs, nil
}

// Helper methods

func (s *FileStore) ensureInit() error {
	if _, err := os.Stat(s.memoryDir); os.IsNotExist(err) {
		return s.Init()
	}
	return nil
}

// GetMemoryDir 获取记忆目录路径
func (s *FileStore) GetMemoryDir() string {
	return s.memoryDir
}

// expandHome 展开 ~ 为用户主目录
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// grepSearch 模拟 grep -i -n 的搜索功能（Go 原生实现，避免命令注入）
func grepSearch(content, query string) ([]string, error) {
	if query == "" {
		return nil, nil
	}

	// 转义正则特殊字符，进行安全的字符串匹配
	pattern := "(?i)" + regexp.QuoteMeta(query)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid search pattern: %w", err)
	}

	var results []string
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if re.MatchString(line) {
			// 格式：行号:匹配内容（类似 grep -n 输出）
			results = append(results, fmt.Sprintf("%d:%s", i+1, line))
		}
	}

	return results, nil
}

// 以下方法保留以兼容旧代码，但标记为废弃

// Add 添加会话消息（已废弃，请使用 SessionStore）
// Deprecated: Use SessionStore instead
func (s *FileStore) Add(ctx context.Context, sessionID string, msg *Message) error {
	// 不再写入 HISTORY.md，直接返回 nil
	return nil
}

// Get 获取会话消息（已废弃，请使用 SessionStore）
// Deprecated: Use SessionStore instead
func (s *FileStore) Get(ctx context.Context, sessionID string, limit int) ([]*Message, error) {
	// 不再从 HISTORY.md 读取，返回空
	return []*Message{}, nil
}

// Clear 清除会话（已废弃，请使用 SessionStore）
// Deprecated: Use SessionStore instead
func (s *FileStore) Clear(ctx context.Context, sessionID string) error {
	return nil
}

// Close 关闭存储
func (s *FileStore) Close() error {
	return nil
}
