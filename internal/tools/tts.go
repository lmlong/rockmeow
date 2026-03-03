// Package tools 工具实现 - 语音合成工具
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	ttspkg "github.com/lingguard/pkg/tts"
)

// TTSTool 语音合成工具
type TTSTool struct {
	config *ttspkg.Config
}

// NewTTSTool 创建语音合成工具
func NewTTSTool(cfg *ttspkg.Config) *TTSTool {
	if cfg == nil {
		cfg = &ttspkg.Config{}
	}

	// 设置默认值
	if cfg.Provider == "" {
		cfg.Provider = "qwen"
	}
	if cfg.Model == "" {
		cfg.Model = "qwen3-tts-flash"
	}
	if cfg.Voice == "" {
		cfg.Voice = "Cherry"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60
	}
	if cfg.OutputDir == "" {
		home, _ := os.UserHomeDir()
		cfg.OutputDir = filepath.Join(home, ".lingguard", "workspace", "generated")
	}

	return &TTSTool{config: cfg}
}

// Name 返回工具名称
func (t *TTSTool) Name() string {
	return "tts"
}

// Description 返回工具描述
func (t *TTSTool) Description() string {
	return `语音合成（TTS）工具。将文本转换为语音。

**参数**：
- action: "synthesize"（目前仅支持合成）
- text: 要合成的文本（必填）
- voice: 音色（可选，默认 Cherry）

**触发场景**：
- "读出来"、"念给我听"
- "转成语音"、"转成音频"
- "朗读这段文字"

详细用法请先加载 tts skill。`
}

// Parameters 返回参数定义
func (t *TTSTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"synthesize"},
				"description": "合成动作",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "要合成的文本内容",
			},
			"voice": map[string]interface{}{
				"type":        "string",
				"description": "音色 (可选，默认 Cherry)",
				"enum":        ttspkg.GetVoices(),
			},
		},
		"required": []string{"action", "text"},
	}
}

// Execute 执行工具
func (t *TTSTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.config.APIKey == "" {
		return "", fmt.Errorf("TTS API key not configured")
	}

	var params struct {
		Action string `json:"action"`
		Text   string `json:"text"`
		Voice  string `json:"voice"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	switch params.Action {
	case "synthesize":
		return t.synthesize(ctx, params.Text, params.Voice)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

// synthesize 合成语音
func (t *TTSTool) synthesize(ctx context.Context, text, voice string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text is required")
	}

	// 创建 TTS 服务
	service, err := ttspkg.NewService(t.config)
	if err != nil {
		return "", fmt.Errorf("create TTS service: %w", err)
	}

	// 合成语音
	var result *ttspkg.SynthesisResult
	if voice != "" {
		result, err = service.SynthesizeWithVoice(ctx, text, voice)
	} else {
		result, err = service.Synthesize(ctx, text)
	}
	if err != nil {
		return "", fmt.Errorf("synthesize: %w", err)
	}

	// 确定使用的音色
	usedVoice := voice
	if usedVoice == "" {
		usedVoice = t.config.Voice
	}

	// 返回特殊格式，让飞书 channel 自动发送音频
	if result.LocalPath != "" {
		return fmt.Sprintf("语音合成成功！\n文本: %s\n音色: %s\n时长: %.1f 秒\n\n[GENERATED_AUDIO:%s]",
			truncateText(text, 100), usedVoice, result.Duration, result.LocalPath), nil
	}

	// 如果没有本地文件，返回 URL
	return fmt.Sprintf("语音合成成功！\n文本: %s\n音色: %s\n时长: %.1f 秒\n音频 URL: %s (24小时有效)",
		truncateText(text, 100), usedVoice, result.Duration, result.AudioURL), nil
}

// truncateText 截断文本
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// IsDangerous 返回是否为危险操作
func (t *TTSTool) IsDangerous() bool {
	return false
}

func (t *TTSTool) ShouldLoadByDefault() bool {
	return false
}

// SetAPIKey 设置 API Key
func (t *TTSTool) SetAPIKey(key string) {
	t.config.APIKey = key
}

// SetVoice 设置默认音色
func (t *TTSTool) SetVoice(voice string) {
	t.config.Voice = voice
}
