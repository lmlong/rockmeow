// Package tts 提供语音合成服务
package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lingguard/pkg/logger"
)

// Config 语音合成配置
type Config struct {
	Provider  string `json:"provider"`            // 提供商: "qwen" (阿里云通义千问)
	APIKey    string `json:"apiKey"`              // API Key
	APIBase   string `json:"apiBase,omitempty"`   // API 基础 URL
	Model     string `json:"model,omitempty"`     // 模型名称，默认 qwen3-tts-flash
	Voice     string `json:"voice,omitempty"`     // 音色，默认 Cherry
	Language  string `json:"language,omitempty"`  // 语言，默认 Chinese
	Timeout   int    `json:"timeout,omitempty"`   // 超时时间（秒），默认 60
	OutputDir string `json:"outputDir,omitempty"` // 输出目录，默认 ~/.lingguard/workspace/generated
}

// SynthesisResult 合成结果
type SynthesisResult struct {
	AudioURL  string  `json:"audioUrl,omitempty"`  // 音频 URL（24小时有效）
	LocalPath string  `json:"localPath,omitempty"` // 本地文件路径
	Duration  float64 `json:"duration"`            // 音频时长（秒）
	Text      string  `json:"text"`                // 合成的文本
}

// Service 语音合成服务接口
type Service interface {
	// Synthesize 合成语音
	Synthesize(ctx context.Context, text string) (*SynthesisResult, error)
	// SynthesizeWithVoice 使用指定音色合成
	SynthesizeWithVoice(ctx context.Context, text, voice string) (*SynthesisResult, error)
}

// NewService 创建语音合成服务
func NewService(cfg *Config) (Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tts config is nil")
	}

	switch cfg.Provider {
	case "qwen", "alibaba", "aliyun":
		return NewQwenTTS(cfg)
	default:
		return nil, fmt.Errorf("unsupported tts provider: %s", cfg.Provider)
	}
}

// QwenTTS 通义千问语音合成服务
type QwenTTS struct {
	config    *Config
	client    *http.Client
	apiBase   string
	outputDir string
}

const (
	defaultQwenTTSAPIBase = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	defaultQwenTTSModel   = "qwen3-tts-flash"
	defaultQwenTTSVoice   = "Cherry"
	defaultQwenTTSTimeout = 60
)

// Available Qwen TTS voices (通义千问 TTS 支持的音色)
var QwenTTSVoices = []string{
	"Cherry",  // 甜美女声 (中文)
	"Serena",  // 温柔女声 (中文)
	"Ethan",   // 沉稳男声 (中文)
	"Chelsie", // 活力女声 (中文)
	"Momo",    // 可爱童声 (中文)
	"Vivian",  // 知性女声 (中文)
	"Moon",    // 亲切男声 (中文)
	"Maia",    // 清澈女声 (中文)
	"Kai",     // 磁性男声 (中文)
	"Stella",  // 英文女声
	"Dylan",   // 英文男声
	"Marcus",  // 英文男声 (深沉)
	"Alice",   // 英文女声 (甜美)
}

// NewQwenTTS 创建通义千问 TTS 服务
func NewQwenTTS(cfg *Config) (*QwenTTS, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("qwen TTS requires apiKey")
	}

	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = defaultQwenTTSAPIBase
	}

	model := cfg.Model
	if model == "" {
		model = defaultQwenTTSModel
	}

	voice := cfg.Voice
	if voice == "" {
		voice = defaultQwenTTSVoice
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultQwenTTSTimeout
	}

	outputDir := cfg.OutputDir
	if outputDir == "" {
		home, _ := os.UserHomeDir()
		outputDir = filepath.Join(home, ".lingguard", "workspace", "generated")
	}

	return &QwenTTS{
		config: &Config{
			Provider:  cfg.Provider,
			APIKey:    cfg.APIKey,
			APIBase:   apiBase,
			Model:     model,
			Voice:     voice,
			Language:  cfg.Language,
			Timeout:   timeout,
			OutputDir: outputDir,
		},
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		apiBase:   apiBase,
		outputDir: outputDir,
	}, nil
}

// Synthesize 合成语音
func (t *QwenTTS) Synthesize(ctx context.Context, text string) (*SynthesisResult, error) {
	return t.SynthesizeWithVoice(ctx, text, t.config.Voice)
}

// SynthesizeWithVoice 使用指定音色合成
func (t *QwenTTS) SynthesizeWithVoice(ctx context.Context, text, voice string) (*SynthesisResult, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	// 限制文本长度（Qwen TTS 单次最大 5000 字符）
	if len(text) > 5000 {
		text = text[:5000]
	}

	// 使用默认音色
	if voice == "" {
		voice = t.config.Voice
	}

	// 检测语言
	language := t.detectLanguage(text)

	// 构建请求体
	// 参考: https://help.aliyun.com/document_detail/2879134.html
	reqBody := map[string]interface{}{
		"model": t.config.Model,
		"input": map[string]interface{}{
			"text":          text,
			"voice":         voice,
			"language_type": language,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 调用 API
	req, err := http.NewRequestWithContext(ctx, "POST", t.apiBase, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.config.APIKey))

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	// 调试：记录原始响应
	logger.Debug("TTS API response", "status", resp.StatusCode, "body", string(body))

	// 解析响应
	// 参考: https://help.aliyun.com/zh/model-studio/developer-reference/tongyi-qianwen-tts
	// 非流式响应格式:
	// {
	//   "request_id": "xxx",
	//   "output": {
	//     "audio": {
	//       "url": "xxx",
	//       "duration": 2000
	//     },
	//     "subtitle": {}
	//   }
	// }
	var ttsResp struct {
		RequestID string `json:"request_id"`
		Output    struct {
			Audio struct {
				URL      string `json:"url"`
				Duration int    `json:"duration"` // 毫秒
			} `json:"audio"`
			Subtitle interface{} `json:"subtitle"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &ttsResp); err != nil {
		return nil, fmt.Errorf("parse response: %w, body=%s", err, string(body))
	}

	if ttsResp.Code != "" && ttsResp.Code != "Success" {
		return nil, fmt.Errorf("api error: %s - %s", ttsResp.Code, ttsResp.Message)
	}

	if ttsResp.Output.Audio.URL == "" {
		return nil, fmt.Errorf("no audio URL in response, body=%s", string(body))
	}

	result := &SynthesisResult{
		AudioURL: ttsResp.Output.Audio.URL,
		Duration: float64(ttsResp.Output.Audio.Duration) / 1000.0, // 转换为秒
		Text:     text,
	}

	// 下载音频到本地
	localPath, err := t.downloadAudio(ctx, ttsResp.Output.Audio.URL)
	if err != nil {
		logger.Warn("Failed to download audio, returning URL only", "error", err)
	} else {
		result.LocalPath = localPath
	}

	logger.Info("TTS synthesis completed", "text_length", len(text), "duration", result.Duration, "voice", voice)

	return result, nil
}

// detectLanguage 检测文本语言
func (t *QwenTTS) detectLanguage(text string) string {
	// 如果配置了语言，使用配置的
	if t.config.Language != "" {
		return t.config.Language
	}

	// 简单的语言检测
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			return "Chinese" // 中文
		}
	}

	// 检查是否包含日文
	for _, r := range text {
		if (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF) {
			return "Japanese"
		}
	}

	// 默认英文
	return "English"
}

// downloadAudio 下载音频文件
func (t *QwenTTS) downloadAudio(ctx context.Context, url string) (string, error) {
	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: status=%d", resp.StatusCode)
	}

	// 生成文件名
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("audio-%s.wav", timestamp)
	filePath := filepath.Join(t.outputDir, filename)

	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 写入文件
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

// SynthesizeText 便捷函数：合成语音
func SynthesizeText(cfg *Config, text string) (*SynthesisResult, error) {
	service, err := NewService(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	return service.Synthesize(ctx, text)
}

// IsValidVoice 检查音色是否有效
func IsValidVoice(voice string) bool {
	for _, v := range QwenTTSVoices {
		if strings.EqualFold(v, voice) {
			return true
		}
	}
	return false
}

// GetVoices 获取可用音色列表
func GetVoices() []string {
	return QwenTTSVoices
}

// SynthesizeBase64 合成语音并返回 base64 编码（用于直接发送）
func (t *QwenTTS) SynthesizeBase64(ctx context.Context, text string) (string, error) {
	result, err := t.Synthesize(ctx, text)
	if err != nil {
		return "", err
	}

	// 如果已有本地文件，读取并编码
	if result.LocalPath != "" {
		data, err := os.ReadFile(result.LocalPath)
		if err != nil {
			return "", fmt.Errorf("read audio file: %w", err)
		}
		return base64.StdEncoding.EncodeToString(data), nil
	}

	// 从 URL 下载并编码
	req, err := http.NewRequestWithContext(ctx, "GET", result.AudioURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}
