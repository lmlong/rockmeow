// Package embedding - 阿里云 DashScope Embedding 实现
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lingguard/pkg/httpclient"
	"github.com/lingguard/pkg/logger"
)

const (
	// Qwen API 配置
	QwenAPIBase        = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	QwenEmbeddingModel = "text-embedding-v4"
	QwenDimension      = 1024

	// API 超时
	defaultTimeout = 30 * time.Second
)

// QwenEmbedding 阿里云 DashScope Embedding 实现
type QwenEmbedding struct {
	apiKey    string
	apiBase   string
	model     string
	dimension int
	client    *http.Client
}

// NewQwenEmbedding 创建阿里云 Embedding 客户端
func NewQwenEmbedding(cfg *Config) *QwenEmbedding {
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = QwenAPIBase
	}

	model := cfg.Model
	if model == "" {
		model = QwenEmbeddingModel
	}

	dimension := cfg.Dimension
	if dimension <= 0 {
		dimension = QwenDimension
	}

	return &QwenEmbedding{
		apiKey:    cfg.APIKey,
		apiBase:   apiBase,
		model:     model,
		dimension: dimension,
		client:    httpclient.Default(),
	}
}

// Name 返回模型名称
func (e *QwenEmbedding) Name() string {
	return e.model
}

// Dimension 返回向量维度
func (e *QwenEmbedding) Dimension() int {
	return e.dimension
}

// Embed 生成单个文本的嵌入向量
func (e *QwenEmbedding) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return vectors[0], nil
}

// EmbedBatch 批量生成嵌入向量
func (e *QwenEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	start := time.Now()

	// 构建请求
	reqBody := embeddingRequest{
		Model: e.model,
		Input: texts,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建请求
	url := fmt.Sprintf("%s/embeddings", e.apiBase)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.apiKey))

	logger.Info("[Embedding] Request", "model", e.model, "texts", len(texts), "provider", "qwen")

	// 发送请求
	resp, err := e.client.Do(req)
	if err != nil {
		logger.Error("[Embedding] Request failed", "model", e.model, "error", err, "duration", time.Since(start))
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("[Embedding] API error", "model", e.model, "status", resp.StatusCode, "body", string(body), "duration", time.Since(start))
		return nil, fmt.Errorf("api error: status=%d body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result embeddingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("empty embedding data")
	}

	// 提取向量
	vectors := make([][]float32, len(result.Data))
	for i, item := range result.Data {
		vectors[i] = item.Embedding
	}

	logger.Info("[Embedding] Response", "model", e.model, "vectors", len(vectors), "tokens", result.Usage.TotalTokens, "duration", time.Since(start))

	return vectors, nil
}

// embeddingRequest 嵌入请求
type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResponse 嵌入响应
type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// 确保 QwenEmbedding 实现 Model 接口
var _ Model = (*QwenEmbedding)(nil)
