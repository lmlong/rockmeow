// Package agent Soul 人格管理
package agent

import (
	"strings"
)

// soulQuestionPatterns "你是谁?"相关问题的匹配模式
var soulQuestionPatterns = []string{
	"你是谁",
	"你叫什么",
	"你的名字",
	"介绍一下你自己",
	"自我介绍",
	"who are you",
	"what is your name",
	"tell me about yourself",
}

// IsSoulQuestion 检测消息是否为"你是谁?"相关问题
func IsSoulQuestion(content string) bool {
	content = strings.ToLower(content)
	for _, pattern := range soulQuestionPatterns {
		if strings.Contains(content, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// GetDefaultGuideMessage 获取默认的 Soul 引导消息
func GetDefaultGuideMessage() string {
	return `你好！我是你的 AI 助手。

在开始之前，我想了解一下你希望我成为什么样的助手。

你可以告诉我：
- 我的性格特点（如：温柔、幽默、专业...）
- 我应该如何称呼你
- 你希望我的回复风格（简洁/详细、活泼/稳重...）
- 其他任何你期望的特质

请直接回复你的想法，或者回复"跳过"使用默认设置。`
}

// GetDefaultSoul 获取默认的 Soul 定义
func GetDefaultSoul() string {
	return "我是一个友好、专业的 AI 助手，致力于帮助用户解决问题。"
}