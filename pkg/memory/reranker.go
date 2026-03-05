// Package memory - 重排序器实现
package memory

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
	// Qwen Rerank API 默认配置（可通过配置文件覆盖）
	QwenRerankAPIBase = "https://dashscope.aliyuncs.com/api/v1/services/rerank/text-rerank/text-rerank"
	QwenRerankModel   = "qwen3-rerank"
)

// RerankResult 重排序结果
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float32 `json:"relevance_score"`
	Document       string  `json:"document,omitempty"`
}

// Reranker 重排序器接口
type Reranker interface {
	// Rerank 对文档进行重排序
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankResult, error)
}

// RerankConfig 重排序器配置
type RerankConfig struct {
	Provider string `json:"provider"`          // 提供商: "qwen"
	Model    string `json:"model"`             // 模型名称
	APIKey   string `json:"apiKey"`            // API Key
	APIBase  string `json:"apiBase,omitempty"` // API 基础 URL (可选)
}

// QwenReranker 阿里云 Qwen Reranker 实现
type QwenReranker struct {
	apiKey  string
	apiBase string
	model   string
	client  *http.Client
}

// NewQwenReranker 创建 Qwen Reranker
func NewQwenReranker(cfg *RerankConfig) *QwenReranker {
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = QwenRerankAPIBase
	}

	model := cfg.Model
	if model == "" {
		model = QwenRerankModel
	}

	return &QwenReranker{
		apiKey:  cfg.APIKey,
		apiBase: apiBase,
		model:   model,
		client:  httpclient.Default(),
	}
}

// Rerank 对文档进行重排序
func (r *QwenReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	start := time.Now()

	// 如果 topK <= 0 或大于文档数，使用文档数
	if topK <= 0 || topK > len(documents) {
		topK = len(documents)
	}

	// 构建请求
	reqBody := rerankRequest{
		Model: r.model,
		Input: rerankInput{
			Query:     query,
			Documents: documents,
		},
		Parameters: rerankParams{
			TopN: topK,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 创建请求 (apiBase 应为完整的 rerank 端点 URL)
	req, err := http.NewRequestWithContext(ctx, "POST", r.apiBase, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.apiKey))

	logger.Info("[Reranker] Request", "model", r.model, "apiBase", r.apiBase, "documents", len(documents), "topK", topK, "provider", "qwen")

	// 发送请求
	resp, err := r.client.Do(req)
	if err != nil {
		logger.Error("[Reranker] Request failed", "model", r.model, "error", err, "duration", time.Since(start))
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("[Reranker] API error", "model", r.model, "apiBase", r.apiBase, "status", resp.StatusCode, "body", string(body), "duration", time.Since(start))
		return nil, fmt.Errorf("api error: status=%d body=%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result rerankResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 转换结果
	results := make([]RerankResult, len(result.Output.Results))
	for i, res := range result.Output.Results {
		results[i] = RerankResult{
			Index:          res.Index,
			RelevanceScore: res.RelevanceScore,
			Document:       res.Document,
		}
	}

	logger.Info("[Reranker] Response", "model", r.model, "results", len(results), "tokens", result.Usage.TotalTokens, "duration", time.Since(start))

	return results, nil
}

// rerankRequest 重排序请求
type rerankRequest struct {
	Model      string       `json:"model"`
	Input      rerankInput  `json:"input"`
	Parameters rerankParams `json:"parameters,omitempty"`
}

type rerankInput struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type rerankParams struct {
	TopN int `json:"top_n,omitempty"`
}

// rerankResponse 重排序响应
type rerankResponse struct {
	Output struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float32 `json:"relevance_score"`
			Document       string  `json:"document"`
		} `json:"results"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// 确保 QwenReranker 实现 Reranker 接口
var _ Reranker = (*QwenReranker)(nil)

// NoOpReranker 空操作重排序器（用于禁用重排序时）
type NoOpReranker struct{}

// NewNoOpReranker 创建空操作重排序器
func NewNoOpReranker() *NoOpReranker {
	return &NoOpReranker{}
}

// Rerank 返回原始顺序的结果
func (r *NoOpReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]RerankResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	if topK <= 0 || topK > len(documents) {
		topK = len(documents)
	}

	results := make([]RerankResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = RerankResult{
			Index:          i,
			RelevanceScore: 1.0,
			Document:       documents[i],
		}
	}
	return results, nil
}

// 确保 NoOpReranker 实现 Reranker 接口
var _ Reranker = (*NoOpReranker)(nil)
