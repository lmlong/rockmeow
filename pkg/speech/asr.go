// Package speech 提供语音识别服务
package speech

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lingguard/pkg/logger"
)

// Config 语音识别配置
type Config struct {
	Provider string `json:"provider"`           // 提供商: "qwen" (阿里云通义千问)
	APIKey   string `json:"apiKey"`             // API Key
	APIBase  string `json:"apiBase,omitempty"`  // API 基础 URL
	Model    string `json:"model,omitempty"`    // 模型名称，默认 qwen3-asr-flash
	Format   string `json:"format,omitempty"`   // 音频格式，默认 opus
	Language string `json:"language,omitempty"` // 语言，默认 zh
	Timeout  int    `json:"timeout,omitempty"`  // 超时时间（秒），默认 60
}

// TranscriptionResult 转写结果
type TranscriptionResult struct {
	Text     string  `json:"text"`     // 转写文本
	Language string  `json:"language"` // 检测到的语言
	Duration float64 `json:"duration"` // 音频时长（秒）
}

// Service 语音识别服务接口
type Service interface {
	// Transcribe 转写音频文件
	Transcribe(ctx context.Context, audioPath string) (*TranscriptionResult, error)
	// TranscribeFromBytes 从字节流转写
	TranscribeFromBytes(ctx context.Context, audioData []byte, format string) (*TranscriptionResult, error)
}

// NewService 创建语音识别服务
func NewService(cfg *Config) (Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("speech config is nil")
	}

	switch cfg.Provider {
	case "qwen", "alibaba", "aliyun":
		return NewQwenASR(cfg)
	default:
		return nil, fmt.Errorf("unsupported speech provider: %s", cfg.Provider)
	}
}

// QwenASR 通义千问语音识别服务
type QwenASR struct {
	config  *Config
	client  *http.Client
	apiBase string
}

const (
	defaultQwenAPIBase = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	defaultQwenModel   = "qwen3-asr-flash"
	defaultTimeout     = 60
)

// NewQwenASR 创建通义千问 ASR 服务
func NewQwenASR(cfg *Config) (*QwenASR, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("qwen ASR requires apiKey")
	}

	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = defaultQwenAPIBase
	}

	model := cfg.Model
	if model == "" {
		model = defaultQwenModel
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &QwenASR{
		config: &Config{
			Provider: cfg.Provider,
			APIKey:   cfg.APIKey,
			APIBase:  apiBase,
			Model:    model,
			Format:   cfg.Format,
			Language: cfg.Language,
			Timeout:  timeout,
		},
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		apiBase: apiBase,
	}, nil
}

// Transcribe 转写音频文件
func (a *QwenASR) Transcribe(ctx context.Context, audioPath string) (*TranscriptionResult, error) {
	// 读取音频文件
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("read audio file: %w", err)
	}

	// 检测音频格式
	format := a.detectFormat(audioPath)

	return a.TranscribeFromBytes(ctx, audioData, format)
}

// TranscribeFromBytes 从字节流转写
func (a *QwenASR) TranscribeFromBytes(ctx context.Context, audioData []byte, format string) (*TranscriptionResult, error) {
	// 使用 OpenAI 兼容模式调用
	// 参考: https://help.aliyun.com/zh/model-studio/qwen-asr-api-reference

	// 构建 base64 data URI
	mimeType := a.getMimeType(format)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(audioData))

	// 构建请求体
	requestBody := map[string]any{
		"model": a.config.Model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"data": dataURI,
						},
					},
				},
			},
		},
	}

	// 添加 ASR 选项
	asrOptions := map[string]any{
		"enable_itn": false,
	}
	if a.config.Language != "" {
		asrOptions["language"] = a.config.Language
	}
	requestBody["asr_options"] = asrOptions

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 调用 API
	url := fmt.Sprintf("%s/chat/completions", a.apiBase)
	req, err := http.NewRequestWithContext(ctx, "POST", url, newBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.config.APIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
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

	// 解析响应
	var openaiResp struct {
		Choices []struct {
			Message struct {
				Content     string `json:"content"`
				Annotations []struct {
					Type     string `json:"type"`
					Language string `json:"language"`
				} `json:"annotations"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			Seconds float64 `json:"seconds"`
		} `json:"usage"`
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if openaiResp.Error.Message != "" {
		return nil, fmt.Errorf("api error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no transcription result")
	}

	result := &TranscriptionResult{
		Text:     openaiResp.Choices[0].Message.Content,
		Duration: openaiResp.Usage.Seconds,
	}

	// 获取检测到的语言
	for _, ann := range openaiResp.Choices[0].Message.Annotations {
		if ann.Type == "audio_info" && ann.Language != "" {
			result.Language = ann.Language
			break
		}
	}

	if result.Language == "" {
		result.Language = a.config.Language
	}

	return result, nil
}

// getMimeType 获取音频 MIME 类型
func (a *QwenASR) getMimeType(format string) string {
	switch format {
	case "mp3", "mpeg":
		return "audio/mpeg"
	case "wav":
		return "audio/wav"
	case "ogg":
		return "audio/ogg"
	case "opus":
		return "audio/opus"
	case "m4a":
		return "audio/mp4"
	case "flac":
		return "audio/flac"
	default:
		return "audio/opus"
	}
}

// detectFormat 从文件路径检测音频格式
func (a *QwenASR) detectFormat(path string) string {
	// 如果配置了格式，使用配置的
	if a.config.Format != "" {
		return a.config.Format
	}

	// 从文件扩展名检测
	ext := ""
	if len(path) > 4 {
		ext = path[len(path)-4:]
	}

	switch ext {
	case ".mp3", "mp3":
		return "mp3"
	case ".wav", ".wave", "wav", "wave":
		return "wav"
	case ".m4a", "m4a":
		return "m4a"
	case ".ogg", ".opus", "ogg", "opus":
		return "opus"
	case ".flac", "flac":
		return "flac"
	default:
		return "opus" // 飞书语音默认格式
	}
}

// TranscribeAudio 便捷函数：转写音频文件
func TranscribeAudio(cfg *Config, audioPath string) (string, error) {
	service, err := NewService(cfg)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	result, err := service.Transcribe(ctx, audioPath)
	if err != nil {
		return "", err
	}

	logger.Info("Audio transcribed", "duration", result.Duration, "text_length", len(result.Text))
	return result.Text, nil
}

// newBuffer 创建 io.Reader
func newBuffer(data []byte) *byteBuffer {
	return &byteBuffer{data: data}
}

type byteBuffer struct {
	data []byte
	pos  int
}

func (b *byteBuffer) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}
