// TODO(performance): This file contains hardcoded polling intervals (3s, 5s)
// These should be configurable via config.json:
// - tools.aigc.pollInterval (default: 3000ms)
// - tools.aigc.maxPollTime (default: 300000ms)
// Priority: P1 - Estimated effort: 1 day
// Related: #configuration #performance

// Package tools 工具实现 - 图像/视频生成工具
package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // 注册 PNG 解码器
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lingguard/pkg/httpclient"
	"github.com/lingguard/pkg/logger"
)

// AIGCTool 图像/视频生成工具
type AIGCTool struct {
	apiKey               string
	apiBase              string
	textToImage          string // 文生图模型
	textToVideo          string // 文生视频模型
	imageToVideo         string // 图生视频模型
	videoToVideo         string // 参考生视频模型
	imageToVideoDuration int    // 图生视频最大时长（秒）
	outputDir            string
	workspace            string // 工作目录，用于路径验证
	sandboxed            bool   // 是否启用沙箱
}

// AIGCConfig 图像/视频生成配置
type AIGCConfig struct {
	APIKey               string
	APIBase              string
	TextToImage          string // 文生图模型
	TextToVideo          string // 文生视频模型
	ImageToVideo         string // 图生视频模型
	VideoToVideo         string // 参考生视频模型（视频生视频）
	ImageToVideoDuration int    // 图生视频最大时长（秒），默认 5，最大 15
	OutputDir            string
	Workspace            string // 工作目录，用于路径验证
	Sandboxed            bool   // 是否启用沙箱
}

// DefaultAIGCConfig 默认配置
func DefaultAIGCConfig() *AIGCConfig {
	home, _ := os.UserHomeDir()
	return &AIGCConfig{
		APIBase:              "https://dashscope.aliyuncs.com/api/v1/services/aigc",
		TextToImage:          "wan2.6-t2i",
		TextToVideo:          "wan2.6-t2v",
		ImageToVideo:         "wan2.6-i2v-flash",
		VideoToVideo:         "wan2.6-r2v-flash",
		ImageToVideoDuration: 5,
		OutputDir:            filepath.Join(home, ".lingguard", "workspace", "generated"),
	}
}

// NewAIGCTool 创建图像生成工具
func NewAIGCTool(cfg *AIGCConfig) *AIGCTool {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://dashscope.aliyuncs.com/api/v1/services/aigc"
	}
	if cfg.TextToImage == "" {
		cfg.TextToImage = "wan2.6-t2i"
	}
	if cfg.TextToVideo == "" {
		cfg.TextToVideo = "wan2.6-t2v"
	}
	if cfg.ImageToVideo == "" {
		cfg.ImageToVideo = "wan2.6-i2v-flash"
	}
	if cfg.VideoToVideo == "" {
		cfg.VideoToVideo = "wan2.6-r2v"
	}
	if cfg.ImageToVideoDuration <= 0 {
		cfg.ImageToVideoDuration = 5
	}
	if cfg.ImageToVideoDuration > 15 {
		cfg.ImageToVideoDuration = 15
	}
	if cfg.OutputDir == "" {
		home, _ := os.UserHomeDir()
		cfg.OutputDir = filepath.Join(home, ".lingguard", "workspace", "generated")
	}

	return &AIGCTool{
		apiKey:               cfg.APIKey,
		apiBase:              cfg.APIBase,
		textToImage:          cfg.TextToImage,
		textToVideo:          cfg.TextToVideo,
		imageToVideo:         cfg.ImageToVideo,
		videoToVideo:         cfg.VideoToVideo,
		imageToVideoDuration: cfg.ImageToVideoDuration,
		outputDir:            cfg.OutputDir,
		workspace:            cfg.Workspace,
		sandboxed:            cfg.Sandboxed,
	}
}

// Name 返回工具名称
func (t *AIGCTool) Name() string {
	return "aigc"
}

// Description 返回工具描述
func (t *AIGCTool) Description() string {
	return "内容生成"
}

// Parameters 返回参数定义
func (t *AIGCTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"generate_image", "generate_video", "generate_video_from_image", "generate_video_from_video"},
				"description": "生成动作类型",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "图像或视频的文字描述",
			},
			"image_path": map[string]interface{}{
				"type":        "string",
				"description": "图片路径（图生视频时需要）",
			},
			"video_path": map[string]interface{}{
				"type":        "string",
				"description": "参考视频路径（视频生视频时需要）",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "使用的模型（可选，默认 wan2.6-t2i）",
			},
			"size": map[string]interface{}{
				"type":        "string",
				"description": "图片尺寸（如 1024x1024, 720x1280）",
			},
			"duration": map[string]interface{}{
				"type":        "integer",
				"description": "视频时长秒数（默认 4，最大 10）",
			},
			"style": map[string]interface{}{
				"type":        "string",
				"description": "风格预设（可选，如 'anime', 'realistic', '3d'）",
			},
		},
		"required": []string{"action", "prompt"},
	}
}

// Execute 执行工具
func (t *AIGCTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("image generation API key not configured")
	}

	var params struct {
		Action    string `json:"action"`
		Prompt    string `json:"prompt"`
		ImagePath string `json:"image_path"`
		VideoPath string `json:"video_path"`
		Model     string `json:"model"`
		Size      string `json:"size"`
		Duration  int    `json:"duration"`
		Style     string `json:"style"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	switch params.Action {
	case "generate_image":
		return t.generateImage(ctx, params.Prompt, params.Model, params.Size, params.Style)
	case "generate_video":
		return t.generateVideo(ctx, params.Prompt, params.Duration)
	case "generate_video_from_image":
		return t.generateVideoFromImage(ctx, params.ImagePath, params.Prompt, params.Duration)
	case "generate_video_from_video":
		return t.generateVideoFromVideo(ctx, params.VideoPath, params.Prompt, params.Duration)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

// generateImage 生成图片
func (t *AIGCTool) generateImage(ctx context.Context, prompt, model, size, style string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	if model == "" {
		model = t.textToImage
	}

	// 构建 multimodal-generation API 请求格式
	// 参考: https://help.aliyun.com/zh/model-studio/wan-image-generation-api-reference
	reqBody := map[string]interface{}{
		"model": model,
		"input": map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "user",
					"content": []map[string]string{
						{
							"text": prompt,
						},
					},
				},
			},
		},
		"parameters": map[string]interface{}{
			"n": 1,
		},
	}

	// 调用 API
	result, err := t.callImageAPI(ctx, reqBody)
	if err != nil {
		return "", err
	}

	// 提取图片 URL（multimodal-generation 格式）
	var imageURL string
	if len(result.Output.Choices) > 0 && len(result.Output.Choices[0].Message.Content) > 0 {
		imageURL = result.Output.Choices[0].Message.Content[0].Image
	} else if len(result.Output.Results) > 0 {
		// 兼容旧的异步格式
		imageURL = result.Output.Results[0].URL
	}

	if imageURL == "" {
		return "", fmt.Errorf("no image generated")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	// 下载图片
	localPath, err := t.downloadFile(ctx, imageURL, "image", ".png")
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}

	// 返回特殊格式，让飞书 channel 自动发送图片
	return fmt.Sprintf("图片生成成功！\n描述: %s\n\n[GENERATED_IMAGE:%s]", prompt, localPath), nil
}

// generateVideo 生成视频
func (t *AIGCTool) generateVideo(ctx context.Context, prompt string, duration int) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	if duration <= 0 {
		duration = 4
	}
	if duration > 10 {
		duration = 10
	}

	// 构建请求 - 使用配置的文生视频模型
	reqBody := map[string]interface{}{
		"model": t.textToVideo,
		"input": map[string]interface{}{
			"prompt": prompt,
		},
		"parameters": map[string]interface{}{},
	}

	// 调用视频生成 API（异步）
	taskID, err := t.submitVideoTask(ctx, reqBody)
	if err != nil {
		return "", err
	}

	// 等待生成完成
	result, err := t.waitForVideoResult(ctx, taskID)
	if err != nil {
		return "", err
	}

	// 下载视频
	if result.Output.VideoURL == "" {
		return "", fmt.Errorf("no video URL in result")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	localPath, err := t.downloadFile(ctx, result.Output.VideoURL, "video", ".mp4")
	if err != nil {
		return "", fmt.Errorf("download video: %w", err)
	}

	// 返回特殊格式，让飞书 channel 自动发送视频
	return fmt.Sprintf("视频生成成功！\n描述: %s\n时长: %d 秒\n\n[GENERATED_VIDEO:%s]", prompt, duration, localPath), nil
}

// generateVideoFromImage 图生视频
func (t *AIGCTool) generateVideoFromImage(ctx context.Context, imagePath, prompt string, duration int) (string, error) {
	if imagePath == "" {
		return "", fmt.Errorf("image_path is required for image-to-video generation")
	}

	// 路径安全验证
	if err := t.validatePath(imagePath); err != nil {
		return "", err
	}

	// 读取图片文件
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image file: %w", err)
	}

	// 压缩图片以符合 API 限制（base64 最大 61440 字节）
	// 返回带 MIME 类型前缀的完整 base64 字符串
	imgURL, err := t.compressImageForAPI(imageData, 60000)
	if err != nil {
		return "", fmt.Errorf("compress image: %w", err)
	}

	// 使用配置的最大时长
	maxDuration := t.imageToVideoDuration
	if maxDuration <= 0 {
		maxDuration = 5
	}
	if maxDuration > 15 {
		maxDuration = 15
	}

	if duration <= 0 {
		duration = maxDuration
	}
	if duration > maxDuration {
		duration = maxDuration
	}

	// 构建请求 - 使用配置的图生视频模型
	// 注意：字段名是 img_url，不是 image
	reqBody := map[string]interface{}{
		"model": t.imageToVideo,
		"input": map[string]interface{}{
			"img_url": imgURL,
			"prompt":  prompt,
		},
		"parameters": map[string]interface{}{
			"duration": duration,
		},
	}

	// 调用视频生成 API（异步）
	taskID, err := t.submitImageToVideoTask(ctx, reqBody)
	if err != nil {
		return "", err
	}

	// 等待生成完成
	result, err := t.waitForVideoResult(ctx, taskID)
	if err != nil {
		return "", err
	}

	// 下载视频
	if result.Output.VideoURL == "" {
		return "", fmt.Errorf("no video URL in result")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	localPath, err := t.downloadFile(ctx, result.Output.VideoURL, "video", ".mp4")
	if err != nil {
		return "", fmt.Errorf("download video: %w", err)
	}

	// 返回特殊格式，让飞书 channel 自动发送视频
	return fmt.Sprintf("视频生成成功！\n描述: %s\n时长: %d 秒\n\n[GENERATED_VIDEO:%s]", prompt, duration, localPath), nil
}

// generateVideoFromVideo 参考视频生成视频（视频生视频）
// 使用 wan2.6-r2v 模型，保持参考视频中的角色一致性
func (t *AIGCTool) generateVideoFromVideo(ctx context.Context, videoPath, prompt string, duration int) (string, error) {
	if videoPath == "" {
		return "", fmt.Errorf("video_path is required for video-to-video generation")
	}

	// 路径安全验证
	if err := t.validatePath(videoPath); err != nil {
		return "", err
	}

	// 读取视频文件
	videoData, err := os.ReadFile(videoPath)
	if err != nil {
		return "", fmt.Errorf("read video file: %w", err)
	}

	// 先上传视频文件获取临时 URL
	videoURL, err := t.uploadVideoForAPI(ctx, videoData, filepath.Ext(videoPath))
	if err != nil {
		return "", fmt.Errorf("upload video: %w", err)
	}
	logger.Info("Video uploaded for reference", "url", videoURL)

	// 使用配置的最大时长（wan2.6-r2v 支持 5 或 10 秒）
	maxDuration := 10
	if duration <= 0 {
		duration = 5
	}
	if duration > maxDuration {
		duration = maxDuration
	}

	// 构建请求 - 使用配置的参考生视频模型 (wan2.6-r2v)
	// 参考: https://help.aliyun.com/zh/model-studio/wan-video-to-video-api-reference
	reqBody := map[string]interface{}{
		"model": t.videoToVideo,
		"input": map[string]interface{}{
			"prompt":         prompt,
			"reference_urls": []string{videoURL}, // 使用 reference_urls（不是 reference_video_urls）
		},
		"parameters": map[string]interface{}{
			"duration":  duration,
			"size":      "1280*720",
			"shot_type": "single",
		},
	}

	// 调用视频生成 API（异步）
	taskID, err := t.submitVideoToVideoTask(ctx, reqBody)
	if err != nil {
		return "", err
	}

	// 等待生成完成
	result, err := t.waitForVideoResult(ctx, taskID)
	if err != nil {
		return "", err
	}

	// 下载视频
	if result.Output.VideoURL == "" {
		return "", fmt.Errorf("no video URL in result")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	localPath, err := t.downloadFile(ctx, result.Output.VideoURL, "video", ".mp4")
	if err != nil {
		return "", fmt.Errorf("download video: %w", err)
	}

	// 返回特殊格式，让飞书 channel 自动发送视频
	return fmt.Sprintf("视频生成成功！\n参考视频: %s\n描述: %s\n时长: %d 秒\n\n[GENERATED_VIDEO:%s]", videoPath, prompt, duration, localPath), nil
}

// submitVideoToVideoTask 提交参考生视频任务
func (t *AIGCTool) submitVideoToVideoTask(ctx context.Context, reqBody interface{}) (string, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// 使用 video-generation API 端点
	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("X-DashScope-Async", "enable")
	req.Header.Set("X-DashScope-OssResourceResolve", "enable") // 必需：用于解析 oss:// URL

	client := httpclient.ExtraLongTimeout()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result videoAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Output.TaskID, nil
}

// submitImageToVideoTask 提交图生视频任务
func (t *AIGCTool) submitImageToVideoTask(ctx context.Context, reqBody interface{}) (string, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// 使用图生视频 API 端点
	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("X-DashScope-Async", "enable")

	client := httpclient.LongTimeout()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result videoAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Output.TaskID, nil
}

// imageAPIResponse 图片 API 响应 (multimodal-generation)
type imageAPIResponse struct {
	RequestId string `json:"request_id"`
	Output    struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content []struct {
					Type  string `json:"type"`
					Image string `json:"image"`
				} `json:"content"`
				Role string `json:"role"`
			} `json:"message"`
		} `json:"choices"`
		Finished bool `json:"finished"`
		// 兼容旧的异步响应格式
		TaskID     string `json:"task_id,omitempty"`
		TaskStatus string `json:"task_status,omitempty"`
		Results    []struct {
			URL string `json:"url"`
		} `json:"results,omitempty"`
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"output"`
}

// videoAPIResponse 视频 API 响应
type videoAPIResponse struct {
	RequestId string `json:"request_id"`
	Output    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		VideoURL   string `json:"video_url"`
		Code       string `json:"code,omitempty"`
		Message    string `json:"message,omitempty"`
	} `json:"output"`
}

// callImageAPI 调用图片生成 API
func (t *AIGCTool) callImageAPI(ctx context.Context, reqBody interface{}) (*imageAPIResponse, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// wan2.6-t2i 使用 multimodal-generation API（同步）
	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))

	client := httpclient.ExtraLongTimeout()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	// 调试：打印原始响应
	logger.Info("Image API response", "body", string(body))

	var result imageAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// multimodal-generation 是同步的，直接返回结果
	return &result, nil
}

// waitForImageResult 等待图片生成结果
func (t *AIGCTool) waitForImageResult(ctx context.Context, taskID string) (*imageAPIResponse, error) {
	// 阿里云任务查询 URL
	url := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)
	client := httpclient.Default()
	maxAttempts := 60 // 最多等待 5 分钟

	for i := 0; i < maxAttempts; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result imageAPIResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		status := result.Output.TaskStatus
		// 处理可能的状态值（大小写兼容）
		switch {
		case status == "SUCCEEDED" || status == "succeeded":
			return &result, nil
		case status == "FAILED" || status == "failed":
			errMsg := result.Output.Message
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return nil, fmt.Errorf("image generation failed: %s", errMsg)
		case status == "PENDING" || status == "pending" || status == "":
			// 空状态或 PENDING，继续等待
			time.Sleep(3 * time.Second)
		case status == "RUNNING" || status == "running" || status == "SUBMITTED" || status == "submitted":
			time.Sleep(3 * time.Second)
		default:
			// 未知状态，记录但继续等待
			time.Sleep(3 * time.Second)
		}
	}

	return nil, fmt.Errorf("timeout waiting for image generation")
}

// submitVideoTask 提交视频生成任务
func (t *AIGCTool) submitVideoTask(ctx context.Context, reqBody interface{}) (string, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	// 使用 video-generation API 端点
	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("X-DashScope-Async", "enable")

	client := httpclient.LongTimeout()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result videoAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	return result.Output.TaskID, nil
}

// waitForVideoResult 等待视频生成结果
func (t *AIGCTool) waitForVideoResult(ctx context.Context, taskID string) (*videoAPIResponse, error) {
	// 使用统一的任务查询 URL
	url := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)
	client := httpclient.Default()
	maxAttempts := 120 // 最多等待 10 分钟（视频生成较慢）

	for i := 0; i < maxAttempts; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result videoAPIResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}

		switch result.Output.TaskStatus {
		case "SUCCEEDED":
			return &result, nil
		case "FAILED":
			return nil, fmt.Errorf("video generation failed: %s", result.Output.Message)
		case "PENDING", "RUNNING":
			time.Sleep(5 * time.Second)
		default:
			return nil, fmt.Errorf("unknown task status: %s", result.Output.TaskStatus)
		}
	}

	return nil, fmt.Errorf("timeout waiting for video generation")
}

// downloadFile 下载文件
func (t *AIGCTool) downloadFile(ctx context.Context, url, prefix, ext string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := httpclient.ExtraLongTimeout()
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
	filename := fmt.Sprintf("%s-%s%s", prefix, timestamp, ext)
	filepath := filepath.Join(t.outputDir, filename)

	// 创建文件
	file, err := os.Create(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 写入文件
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return filepath, nil
}

// compressImageForAPI 压缩图片以符合 API 大小限制
// 返回带 MIME 类型前缀的完整 data URI 格式
func (t *AIGCTool) compressImageForAPI(imageData []byte, maxSize int) (string, error) {
	// 先检查原始大小
	base64Str := base64.StdEncoding.EncodeToString(imageData)
	dataURI := fmt.Sprintf("data:image/jpeg;base64,%s", base64Str)
	if len(base64Str) <= maxSize {
		return dataURI, nil
	}

	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	// 尝试不同质量级别压缩
	qualities := []int{85, 70, 55, 40, 25}
	for _, quality := range qualities {
		var buf bytes.Buffer
		var err error

		// 根据格式选择编码器
		if format == "png" {
			// PNG 转 JPEG 以获得更好的压缩
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
		} else {
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
		}

		if err != nil {
			continue
		}

		base64Str = base64.StdEncoding.EncodeToString(buf.Bytes())
		if len(base64Str) <= maxSize {
			return fmt.Sprintf("data:image/jpeg;base64,%s", base64Str), nil
		}
	}

	// 如果仍然太大，尝试缩小尺寸
	bounds := img.Bounds()
	maxDimension := 1024
	for maxDimension >= 256 {
		// 计算缩放比例
		ratio := float64(maxDimension) / float64(max(bounds.Dx(), bounds.Dy()))
		if ratio >= 1 {
			maxDimension -= 128
			continue
		}

		newWidth := int(float64(bounds.Dx()) * ratio)
		newHeight := int(float64(bounds.Dy()) * ratio)

		// 使用简单的最近邻缩放
		resized := t.resizeImage(img, newWidth, newHeight)

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 70}); err != nil {
			maxDimension -= 128
			continue
		}

		base64Str = base64.StdEncoding.EncodeToString(buf.Bytes())
		if len(base64Str) <= maxSize {
			return fmt.Sprintf("data:image/jpeg;base64,%s", base64Str), nil
		}

		maxDimension -= 128
	}

	return "", fmt.Errorf("image too large after compression (base64 size: %d bytes, max: %d)", len(base64Str), maxSize)
}

// resizeImage 简单的图片缩放
func (t *AIGCTool) resizeImage(img image.Image, newWidth, newHeight int) image.Image {
	bounds := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	xRatio := float64(bounds.Dx()) / float64(newWidth)
	yRatio := float64(bounds.Dy()) / float64(newHeight)

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) * xRatio)
			srcY := int(float64(y) * yRatio)
			dst.Set(x, y, img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y))
		}
	}

	return dst
}

// uploadVideoForAPI 上传视频文件到 DashScope 获取临时 URL
// 参考: https://help.aliyun.com/zh/model-studio/get-temporary-file-url
// 步骤1: GET 获取上传凭证
// 步骤2: POST 上传文件到 OSS
// 步骤3: 返回 oss:// URL
func (t *AIGCTool) uploadVideoForAPI(ctx context.Context, videoData []byte, ext string) (string, error) {
	// 生成唯一文件名
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("video-%s%s", timestamp, ext)

	// 步骤1: 获取上传凭证
	policyURL := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/uploads?action=getPolicy&model=%s", t.videoToVideo)

	req, err := http.NewRequestWithContext(ctx, "GET", policyURL, nil)
	if err != nil {
		return "", fmt.Errorf("create policy request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := httpclient.Default()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("get upload policy: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("read policy response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("policy API error: status=%d body=%s", resp.StatusCode, string(body))
	}

	// 解析凭证响应
	var policyResp struct {
		Data struct {
			Policy              string `json:"policy"`
			Signature           string `json:"signature"`
			UploadDir           string `json:"upload_dir"`
			UploadHost          string `json:"upload_host"`
			OSSAccessKeyID      string `json:"oss_access_key_id"`
			XOSSObjectAcl       string `json:"x_oss_object_acl"`
			XOSSForbidOverwrite string `json:"x_oss_forbid_overwrite"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &policyResp); err != nil {
		return "", fmt.Errorf("parse policy response: %w (body: %s)", err, string(body))
	}

	data := policyResp.Data
	if data.UploadHost == "" {
		return "", fmt.Errorf("empty upload_host in policy response")
	}

	// 步骤2: 上传文件到 OSS
	key := fmt.Sprintf("%s/%s", data.UploadDir, filename)

	// 构建 multipart/form-data 请求
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加表单字段（顺序不重要，但 file 必须是最后一个）
	if err := writer.WriteField("OSSAccessKeyId", data.OSSAccessKeyID); err != nil {
		return "", fmt.Errorf("write OSSAccessKeyId field: %w", err)
	}
	if err := writer.WriteField("Signature", data.Signature); err != nil {
		return "", fmt.Errorf("write Signature field: %w", err)
	}
	if err := writer.WriteField("policy", data.Policy); err != nil {
		return "", fmt.Errorf("write policy field: %w", err)
	}
	if err := writer.WriteField("key", key); err != nil {
		return "", fmt.Errorf("write key field: %w", err)
	}
	if err := writer.WriteField("x-oss-object-acl", data.XOSSObjectAcl); err != nil {
		return "", fmt.Errorf("write x-oss-object-acl field: %w", err)
	}
	if err := writer.WriteField("x-oss-forbid-overwrite", data.XOSSForbidOverwrite); err != nil {
		return "", fmt.Errorf("write x-oss-forbid-overwrite field: %w", err)
	}
	if err := writer.WriteField("success_action_status", "200"); err != nil {
		return "", fmt.Errorf("write success_action_status field: %w", err)
	}

	// 添加文件（必须是最后一个）
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(videoData); err != nil {
		return "", fmt.Errorf("write file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	// 发送上传请求
	uploadReq, err := http.NewRequestWithContext(ctx, "POST", data.UploadHost, &buf)
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())

	uploadClient := httpclient.ExtraLongTimeout()
	uploadResp, err := uploadClient.Do(uploadReq)
	if err != nil {
		return "", fmt.Errorf("upload to OSS: %w", err)
	}
	defer uploadResp.Body.Close()

	uploadBody, _ := io.ReadAll(uploadResp.Body)
	if uploadResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OSS upload error: status=%d body=%s", uploadResp.StatusCode, string(uploadBody))
	}

	// 步骤3: 返回 oss:// URL
	ossURL := fmt.Sprintf("oss://%s", key)
	logger.Info("Video uploaded successfully", "url", ossURL)

	return ossURL, nil
}

// IsDangerous 返回是否为危险操作
func (t *AIGCTool) IsDangerous() bool {
	return false
}

// SetAPIKey 设置 API Key
func (t *AIGCTool) SetAPIKey(key string) {
	t.apiKey = key
}

// validatePath 验证路径是否在允许的目录内（防止路径遍历攻击）
func (t *AIGCTool) validatePath(path string) error {
	if !t.sandboxed || t.workspace == "" {
		return nil // 未启用沙箱，跳过验证
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// 获取工作区绝对路径
	absWorkspace, err := filepath.Abs(t.workspace)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace: %w", err)
	}

	// 解析符号链接
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// 如果路径不存在，检查父目录
		evalPath, err = t.resolveParentPath(absPath)
		if err != nil {
			return fmt.Errorf("path validation failed: %w", err)
		}
	}

	// 解析工作区的符号链接
	evalWorkspace, err := filepath.EvalSymlinks(absWorkspace)
	if err != nil {
		evalWorkspace = absWorkspace
	}

	// 使用相对路径检查
	rel, err := filepath.Rel(evalWorkspace, evalPath)
	if err != nil {
		return fmt.Errorf("failed to check path relation: %w", err)
	}

	// 检查是否尝试逃逸工作区
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("path outside workspace: %s", path)
	}

	return nil
}

// resolveParentPath 解析父目录路径（用于新文件场景）
func (t *AIGCTool) resolveParentPath(path string) (string, error) {
	dir := filepath.Dir(path)
	for {
		evalDir, err := filepath.EvalSymlinks(dir)
		if err == nil {
			remaining := strings.TrimPrefix(path, dir)
			if remaining != "" {
				return filepath.Join(evalDir, remaining), nil
			}
			return evalDir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot resolve path: %s", path)
		}
		dir = parent
	}
}
