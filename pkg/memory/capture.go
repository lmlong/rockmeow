// Package memory - 自动捕获规则（参考 OpenClaw memory-lancedb）
package memory

import (
	"regexp"
	"strings"
)

// MemoryCategory 记忆类别
type MemoryCategory string

const (
	CategoryPreference MemoryCategory = "User Preferences" // 用户偏好
	CategoryFact       MemoryCategory = "Important Facts"  // 事实信息
	CategoryDecision   MemoryCategory = "Decisions"        // 决策记录
	CategoryEntity     MemoryCategory = "Contact Info"     // 实体信息（联系方式等）
	CategoryOther      MemoryCategory = "Other"            // 其他
)

// memoryTriggers 记忆触发规则（参考 OpenClaw）
// 匹配这些规则的内容会被自动捕获
var memoryTriggers = []*regexp.Regexp{
	// 记住/忘记相关
	regexp.MustCompile(`(?i)记住|remember|zapamatuj`),
	regexp.MustCompile(`(?i)别忘|don't forget`),
	// 偏好相关
	regexp.MustCompile(`(?i)我喜欢|我讨厌|我喜欢|我讨厌`),
	regexp.MustCompile(`(?i)prefer|like|hate|favorite|favourite`),
	regexp.MustCompile(`(?i)always|never|usually|often`), // 习惯性表达
	// 决策相关
	regexp.MustCompile(`(?i)决定|decided|will use|using`),
	regexp.MustCompile(`(?i)my choice|选择`),
	// 联系方式
	regexp.MustCompile(`\+?\d{10,}`),           // 电话号码
	regexp.MustCompile(`[\w.-]+@[\w.-]+\.\w+`), // 邮箱
	// 重要标记
	regexp.MustCompile(`(?i)important|重要|关键|核心`),
	regexp.MustCompile(`(?i)my name is|i am|i'm`), // 身份信息
	// 项目/工作相关
	regexp.MustCompile(`(?i)my project|my work|我的项目`),
	regexp.MustCompile(`(?i)working on|developing|building`),
}

// promptInjectionPatterns Prompt 注入检测规则
var promptInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(previous|all|the|prior)`),
	regexp.MustCompile(`(?i)forget\s+(everything|all|previous)`),
	regexp.MustCompile(`(?i)you\s+are\s+now`),
	regexp.MustCompile(`(?i)act\s+as`),
	regexp.MustCompile(`(?i)disregard`),
	regexp.MustCompile(`(?i)override\s+(previous|all)`),
	regexp.MustCompile(`(?i)<\|.*?\|>`), // 特殊 token
}

// ShouldCapture 检查内容是否应该被捕获
func ShouldCapture(content string) bool {
	if len(content) == 0 {
		return false
	}

	// 检测 Prompt 注入，如果检测到则不捕获
	if IsPromptInjection(content) {
		return false
	}

	// 排除问句（以问号结尾或包含"什么"等疑问词）
	if isQuestion(content) {
		return false
	}

	// 检查是否匹配任何触发规则
	for _, pattern := range memoryTriggers {
		if pattern.MatchString(content) {
			return true
		}
	}

	return false
}

// isQuestion 检查内容是否是问句
func isQuestion(content string) bool {
	// 以问号结尾
	if strings.HasSuffix(content, "？") || strings.HasSuffix(content, "?") {
		return true
	}
	// 包含疑问词但不是陈述句
	questionWords := []string{"什么", "为什么", "怎么", "如何", "哪里", "哪个", "几", "多少", "吗", "是否"}
	statementMarkers := []string{"记住", "决定", "选择", "prefer", "like", "decided", "choice"}

	hasQuestionWord := false
	for _, word := range questionWords {
		if strings.Contains(content, word) {
			hasQuestionWord = true
			break
		}
	}

	hasStatementMarker := false
	for _, marker := range statementMarkers {
		if strings.Contains(content, marker) {
			hasStatementMarker = true
			break
		}
	}

	// 如果有疑问词但没有陈述标记，认为是问句
	return hasQuestionWord && !hasStatementMarker
}

// IsPromptInjection 检测是否为 Prompt 注入攻击
func IsPromptInjection(content string) bool {
	for _, pattern := range promptInjectionPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// DetectCategory 自动检测记忆类别
func DetectCategory(text string) MemoryCategory {
	lower := strings.ToLower(text)

	// 偏好检测
	if strings.Contains(lower, "prefer") ||
		strings.Contains(lower, "喜欢") ||
		strings.Contains(lower, "讨厌") ||
		strings.Contains(lower, "favorite") ||
		strings.Contains(lower, "always") ||
		strings.Contains(lower, "never") {
		return CategoryPreference
	}

	// 决策检测
	if strings.Contains(lower, "decided") ||
		strings.Contains(lower, "决定") ||
		strings.Contains(lower, "will use") ||
		strings.Contains(lower, "选择") ||
		strings.Contains(lower, "choice") {
		return CategoryDecision
	}

	// 实体检测（联系方式等）
	if strings.Contains(lower, "@") ||
		regexp.MustCompile(`\d{10,}`).MatchString(text) ||
		strings.Contains(lower, "email") ||
		strings.Contains(lower, "phone") ||
		strings.Contains(lower, "contact") {
		return CategoryEntity
	}

	// 事实检测（身份、项目等）
	if strings.Contains(lower, "my name") ||
		strings.Contains(lower, "i am") ||
		strings.Contains(lower, "i'm") ||
		strings.Contains(lower, "my project") ||
		strings.Contains(lower, "working on") ||
		strings.Contains(lower, "我的项目") {
		return CategoryFact
	}

	return CategoryOther
}

// ExtractCapturableContent 从消息中提取可捕获的内容
// 返回提取的内容和是否应该捕获
func ExtractCapturableContent(userMessage string, maxChars int) (string, bool) {
	if !ShouldCapture(userMessage) {
		return "", false
	}

	// 限制长度
	content := userMessage
	if len(content) > maxChars {
		content = content[:maxChars]
	}

	return content, true
}

// SanitizeContent 清理内容，移除可能的敏感信息
func SanitizeContent(content string) string {
	// 移除可能的 API Key 模式
	apiKeyPattern := regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password)\s*[:=]\s*[\w-]{10,}`)
	content = apiKeyPattern.ReplaceAllString(content, "[REDACTED]")

	return content
}

// CaptureResult 捕获结果
type CaptureResult struct {
	Content  string         // 捕获的内容
	Category MemoryCategory // 检测到的类别
	Captured bool           // 是否被捕获
}

// AnalyzeForCapture 分析消息并返回捕获结果
func AnalyzeForCapture(userMessage string, maxChars int) *CaptureResult {
	result := &CaptureResult{
		Captured: false,
	}

	content, shouldCapture := ExtractCapturableContent(userMessage, maxChars)
	if !shouldCapture {
		return result
	}

	result.Content = SanitizeContent(content)
	result.Category = DetectCategory(content)
	result.Captured = true

	return result
}
