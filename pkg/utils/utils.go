// Package utils 通用工具函数
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExpandHome 展开 ~ 为用户主目录
func ExpandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// ParseTime 解析时间字符串（支持多种格式）
// 支持格式：
//   - 绝对时间: "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"
//   - ISO 8601: "2006-01-02T15:04:05", "2006-01-02T15:04"
//   - 相对时间: "in 5m", "in 1h30m", "+5m", "+1h"
//   - RFC3339: "2006-01-02T15:04:05Z07:00"
func ParseTime(s string) (time.Time, error) {
	// 先尝试相对时间格式
	if t, err := parseRelativeTime(s); err == nil {
		return t, nil
	}

	// 绝对时间格式
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, s, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}

// parseRelativeTime 解析相对时间格式
func parseRelativeTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	now := time.Now()

	// 格式: "in 5m", "in 1h30m"
	if strings.HasPrefix(strings.ToLower(s), "in ") {
		durationStr := strings.TrimSpace(s[3:])
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(d), nil
	}

	// 格式: "+5m", "+1h"
	if strings.HasPrefix(s, "+") {
		durationStr := strings.TrimSpace(s[1:])
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(d), nil
	}

	return time.Time{}, fmt.Errorf("not a relative time format")
}

// FormatTime 格式化时间戳为字符串
func FormatTime(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
}

// TruncateString 截断字符串
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
