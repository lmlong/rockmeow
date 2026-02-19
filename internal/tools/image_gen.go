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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ImageGenTool 图像/视频生成工具
type ImageGenTool struct {
	apiKey       string
	apiBase      string
	textToImage  string // 文生图模型
	textToVideo  string // 文生视频模型
	imageToVideo string // 图生视频模型
	outputDir    string
}

// ImageGenConfig 图像/视频生成配置
type ImageGenConfig struct {
	APIKey       string
	APIBase      string
	TextToImage  string // 文生图模型
	TextToVideo  string // 文生视频模型
	ImageToVideo string // 图生视频模型
	OutputDir    string
}

// DefaultImageGenConfig 默认配置
func DefaultImageGenConfig() *ImageGenConfig {
	home, _ := os.UserHomeDir()
	return &ImageGenConfig{
		APIBase:      "https://dashscope.aliyuncs.com/api/v1/services/aigc",
		TextToImage:  "wan2.6-t2i",
		TextToVideo:  "wan2.6-t2v",
		ImageToVideo: "wan2.6-i2v-flash",
		OutputDir:    filepath.Join(home, ".lingguard", "workspace", "generated"),
	}
}

// NewImageGenTool 创建图像生成工具
func NewImageGenTool(cfg *ImageGenConfig) *ImageGenTool {
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
	if cfg.OutputDir == "" {
		home, _ := os.UserHomeDir()
		cfg.OutputDir = filepath.Join(home, ".lingguard", "workspace", "generated")
	}

	return &ImageGenTool{
		apiKey:       cfg.APIKey,
		apiBase:      cfg.APIBase,
		textToImage:  cfg.TextToImage,
		textToVideo:  cfg.TextToVideo,
		imageToVideo: cfg.ImageToVideo,
		outputDir:    cfg.OutputDir,
	}
}

// Name 返回工具名称
func (t *ImageGenTool) Name() string {
	return "image_gen"
}

// Description 返回工具描述
func (t *ImageGenTool) Description() string {
	return `Image and video generation tool using Alibaba Cloud Tongyi Wanxiang.

Actions:
- generate_image: Generate an image from text description
- generate_video: Generate a video from text description
- generate_video_from_image: Generate a video from an existing image

Usage:
{"action": "generate_image", "prompt": "A cute cat sitting on a chair"}
{"action": "generate_video", "prompt": "A cat walking in a garden", "duration": 5}
{"action": "generate_video_from_image", "prompt": "The cat starts walking", "image_path": "/path/to/image.png"}

Image path for generate_video_from_image:
- Generated images: ~/.lingguard/workspace/generated/
- Downloaded images from chat: ~/.lingguard/media/
- ALWAYS use the actual file path from previous messages or list files first

Available models:
- wan2.6-t2i: Text-to-image (default)
- wan2.6-t2v: Text-to-video
- wan2.6-i2v-flash: Image-to-video

Video generation:
- Default duration: 5 seconds
- Max duration: 10 seconds`
}

// Parameters 返回参数定义
func (t *ImageGenTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"generate_image", "generate_video", "generate_video_from_image"},
				"description": "The generation action to perform",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "Text description of the image or video to generate",
			},
			"image_path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the image file for image-to-video generation",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Model to use (optional, defaults to wanx2.1-t2i-turbo)",
			},
			"size": map[string]interface{}{
				"type":        "string",
				"description": "Image size for generation (e.g., 1024x1024, 720x1280)",
			},
			"duration": map[string]interface{}{
				"type":        "integer",
				"description": "Video duration in seconds (default: 4, max: 10)",
			},
			"style": map[string]interface{}{
				"type":        "string",
				"description": "Style preset (optional, e.g., 'anime', 'realistic', '3d')",
			},
		},
		"required": []string{"action", "prompt"},
	}
}

// Execute 执行工具
func (t *ImageGenTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if t.apiKey == "" {
		return "", fmt.Errorf("image generation API key not configured")
	}

	var params struct {
		Action    string `json:"action"`
		Prompt    string `json:"prompt"`
		ImagePath string `json:"image_path"`
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
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

// generateImage 生成图片
func (t *ImageGenTool) generateImage(ctx context.Context, prompt, model, size, style string) (string, error) {
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	if model == "" {
		model = t.textToImage
	}

	// 构建请求
	parameters := map[string]interface{}{
		"n": 1,
	}

	// 注意: wanx2.1 系列模型不支持 size 参数，使用模型默认尺寸
	// size 参数已弃用，忽略用户传入的 size 值

	if style != "" {
		parameters["style"] = style
	}

	reqBody := map[string]interface{}{
		"model": model,
		"input": map[string]string{
			"prompt": prompt,
		},
		"parameters": parameters,
	}

	// 调用 API
	result, err := t.callImageAPI(ctx, reqBody)
	if err != nil {
		return "", err
	}

	// 下载并保存图片
	if len(result.Output.Results) == 0 {
		return "", fmt.Errorf("no image generated")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	// 下载图片
	imageURL := result.Output.Results[0].URL
	localPath, err := t.downloadFile(ctx, imageURL, "image", ".png")
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}

	// 返回特殊格式，让飞书 channel 自动发送图片
	return fmt.Sprintf("图片生成成功！\n描述: %s\n\n[GENERATED_IMAGE:%s]", prompt, localPath), nil
}

// generateVideo 生成视频
func (t *ImageGenTool) generateVideo(ctx context.Context, prompt string, duration int) (string, error) {
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
func (t *ImageGenTool) generateVideoFromImage(ctx context.Context, imagePath, prompt string, duration int) (string, error) {
	if imagePath == "" {
		return "", fmt.Errorf("image_path is required for image-to-video generation")
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

	if duration <= 0 {
		duration = 5
	}
	if duration > 10 {
		duration = 10
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

// submitImageToVideoTask 提交图生视频任务
func (t *ImageGenTool) submitImageToVideoTask(ctx context.Context, reqBody interface{}) (string, error) {
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

	client := &http.Client{Timeout: 60 * time.Second}
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

// imageAPIResponse 图片 API 响应
type imageAPIResponse struct {
	RequestId string `json:"request_id"`
	Output    struct {
		TaskID     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
		Results    []struct {
			URL string `json:"url"`
		} `json:"results"`
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
func (t *ImageGenTool) callImageAPI(ctx context.Context, reqBody interface{}) (*imageAPIResponse, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/text2image/image-synthesis", t.apiBase)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("X-DashScope-Async", "enable")

	client := &http.Client{Timeout: 60 * time.Second}
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

	var result imageAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 如果是异步任务，等待结果
	if result.Output.TaskID != "" && result.Output.TaskStatus != "SUCCEEDED" {
		return t.waitForImageResult(ctx, result.Output.TaskID)
	}

	return &result, nil
}

// waitForImageResult 等待图片生成结果
func (t *ImageGenTool) waitForImageResult(ctx context.Context, taskID string) (*imageAPIResponse, error) {
	// 阿里云任务查询 URL
	url := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)

	client := &http.Client{Timeout: 30 * time.Second}
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
func (t *ImageGenTool) submitVideoTask(ctx context.Context, reqBody interface{}) (string, error) {
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

	client := &http.Client{Timeout: 60 * time.Second}
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
func (t *ImageGenTool) waitForVideoResult(ctx context.Context, taskID string) (*videoAPIResponse, error) {
	// 使用统一的任务查询 URL
	url := fmt.Sprintf("https://dashscope.aliyuncs.com/api/v1/tasks/%s", taskID)

	client := &http.Client{Timeout: 30 * time.Second}
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
func (t *ImageGenTool) downloadFile(ctx context.Context, url, prefix, ext string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 120 * time.Second}
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
func (t *ImageGenTool) compressImageForAPI(imageData []byte, maxSize int) (string, error) {
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
func (t *ImageGenTool) resizeImage(img image.Image, newWidth, newHeight int) image.Image {
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

// IsDangerous 返回是否为危险操作
func (t *ImageGenTool) IsDangerous() bool {
	return false
}

// SetAPIKey 设置 API Key
func (t *ImageGenTool) SetAPIKey(key string) {
	t.apiKey = key
}
