// Package memory - 混合存储实现 (FileStore + VectorStore)
package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/embedding"
	"github.com/lingguard/pkg/logger"
)

// HybridStore 混合存储，组合 FileStore + VectorStore
// 实现完全向后兼容，同时提供向量检索能力
type HybridStore struct {
	fileStore   *FileStore
	vectorStore *SQLiteVecStore
	embedding   embedding.Model

	// 配置
	vectorEnabled bool
	searchConfig  *config.SearchConfig

	// 缓冲写入
	buffer     []*VectorRecord
	bufferMu   sync.Mutex
	bufferSize int
	flushTimer *time.Timer

	// 状态
	closed bool
}

// HybridStoreConfig 混合存储配置
type HybridStoreConfig struct {
	MemoryDir    string
	VectorConfig *config.VectorConfig
	Providers    map[string]config.ProviderConfig // 用于获取 API Key
}

// NewHybridStore 创建混合存储
func NewHybridStore(cfg *HybridStoreConfig) (*HybridStore, error) {
	// 创建 FileStore
	fileStore := NewFileStore(cfg.MemoryDir)
	if err := fileStore.Init(); err != nil {
		return nil, fmt.Errorf("init file store: %w", err)
	}

	store := &HybridStore{
		fileStore:     fileStore,
		bufferSize:    10, // 缓冲10条记录
		vectorEnabled: cfg.VectorConfig != nil && cfg.VectorConfig.Enabled,
	}

	// 如果启用向量检索，初始化向量存储
	if store.vectorEnabled {
		// 创建 embedding 模型
		emb, err := createEmbeddingModel(cfg.VectorConfig, cfg.Providers)
		if err != nil {
			// 向量初始化失败不影响基本功能
			store.vectorEnabled = false
		} else {
			store.embedding = emb

			// 创建向量存储
			vectorStoreCfg := &VectorStoreConfig{
				DatabasePath: getVectorDbPath(cfg.VectorConfig, cfg.MemoryDir),
				Dimension:    getEmbeddingDimension(cfg.VectorConfig),
			}

			// 创建重排序器
			var reranker Reranker
			if cfg.VectorConfig.Search.Rerank != nil && cfg.VectorConfig.Search.Rerank.Enabled {
				reranker = createReranker(cfg.VectorConfig, cfg.Providers)
			} else {
				reranker = NewNoOpReranker()
			}

			vectorStore, err := NewSQLiteVecStore(vectorStoreCfg, emb, reranker)
			if err != nil {
				// 向量存储初始化失败不影响基本功能
				store.vectorEnabled = false
			} else {
				store.vectorStore = vectorStore
				store.searchConfig = &cfg.VectorConfig.Search
			}
		}
	}

	return store, nil
}

// Add 添加消息 (实现 Store 接口)
func (s *HybridStore) Add(ctx context.Context, sessionID string, msg *Message) error {
	// 始终写入文件存储
	if err := s.fileStore.Add(ctx, sessionID, msg); err != nil {
		return err
	}

	// 如果启用向量，异步索引
	if s.vectorEnabled && s.embedding != nil && msg.Content != "" {
		record := &VectorRecord{
			ID:      uuid.New().String(),
			Content: msg.Content,
			Metadata: map[string]interface{}{
				"session_id": sessionID,
				"role":       msg.Role,
			},
			Timestamp: time.Now(),
		}

		// 添加到缓冲
		s.addToBuffer(record)
	}

	return nil
}

// Get 获取消息 (实现 Store 接口)
func (s *HybridStore) Get(ctx context.Context, sessionID string, limit int) ([]*Message, error) {
	return s.fileStore.Get(ctx, sessionID, limit)
}

// Clear 清除会话 (实现 Store 接口)
func (s *HybridStore) Clear(ctx context.Context, sessionID string) error {
	return s.fileStore.Clear(ctx, sessionID)
}

// Close 关闭存储
func (s *HybridStore) Close() error {
	s.closed = true

	// 刷新缓冲
	s.flushBuffer()

	// 关闭向量存储
	if s.vectorStore != nil {
		s.vectorStore.Close()
	}

	return s.fileStore.Close()
}

// FileStore 获取底层文件存储
func (s *HybridStore) FileStore() *FileStore {
	return s.fileStore
}

// VectorStore 获取底层向量存储
func (s *HybridStore) VectorStore() *SQLiteVecStore {
	return s.vectorStore
}

// IsVectorEnabled 检查是否启用向量检索
func (s *HybridStore) IsVectorEnabled() bool {
	return s.vectorEnabled && s.vectorStore != nil
}

// Search 执行搜索
// 如果启用向量检索，使用混合检索；否则使用文件搜索
func (s *HybridStore) Search(ctx context.Context, query string, topK int) ([]*VectorRecord, error) {
	start := time.Now()

	if !s.IsVectorEnabled() {
		// 回退到文件搜索
		logger.Debug("Vector search disabled, falling back to file search", "query", query)
		return s.searchFromFile(ctx, query, topK)
	}

	// 生成查询向量
	queryVec, err := s.embedding.Embed(ctx, query)
	if err != nil {
		// 向量生成失败，回退到文件搜索
		logger.Warn("Embedding failed, falling back to file search", "query", query, "error", err)
		return s.searchFromFile(ctx, query, topK)
	}

	// 混合检索
	opts := HybridSearchOptions{
		TopK:         topK,
		VectorWeight: s.searchConfig.VectorWeight,
		BM25Weight:   s.searchConfig.BM25Weight,
		MinScore:     s.searchConfig.MinScore,
	}

	if opts.VectorWeight == 0 {
		opts.VectorWeight = 0.7
	}
	if opts.BM25Weight == 0 {
		opts.BM25Weight = 0.3
	}
	if topK <= 0 {
		opts.TopK = 10
	}

	results, err := s.vectorStore.HybridSearch(ctx, queryVec, query, opts)
	if err != nil {
		logger.Error("Hybrid search failed", "query", query, "error", err, "duration", time.Since(start))
		return nil, err
	}

	// 如果向量搜索返回空结果，回退到文件搜索
	if len(results) == 0 {
		logger.Info("Vector search returned no results, falling back to file search", "query", query, "duration", time.Since(start))
		return s.searchFromFile(ctx, query, topK)
	}

	logger.Info("Vector search completed", "query", query, "results", len(results), "duration", time.Since(start))
	return results, nil
}

// searchFromFile 从文件搜索 (回退方案)
func (s *HybridStore) searchFromFile(ctx context.Context, query string, topK int) ([]*VectorRecord, error) {
	results, err := s.fileStore.SearchAll(query)
	if err != nil {
		return nil, err
	}

	var records []*VectorRecord
	for fileName, lines := range results {
		for _, line := range lines {
			records = append(records, &VectorRecord{
				ID:        fmt.Sprintf("%s-%d", fileName, len(records)),
				Content:   line,
				Timestamp: time.Now(),
				Score:     1.0,
				Metadata: map[string]interface{}{
					"source": fileName,
				},
			})
		}
	}

	if len(records) > topK {
		records = records[:topK]
	}

	return records, nil
}

// AddMemory 添加长期记忆
func (s *HybridStore) AddMemory(category, content string) error {
	// 智能去重：检查是否已存在相似记忆

	// 1. 先检查文件存储中是否已有相同或相似内容（最快最可靠）
	existingMemory, err := s.fileStore.GetMemory()
	if err == nil && existingMemory != "" {
		// 按行检查，提取每条记忆内容进行比较
		lines := strings.Split(existingMemory, "\n")
		for _, line := range lines {
			// 提取记忆内容（格式: "- [时间] 内容"）
			if idx := strings.Index(line, "] "); idx > 0 && idx < len(line)-2 {
				existingContent := line[idx+2:]
				// 检查相似度
				if isSimilarContent(existingContent, content) {
					logger.Info("Skipping duplicate memory in file store", "existing", existingContent, "new", content)
					return nil
				}
			}
		}
	}

	// 2. 检查缓冲区中是否有相似内容
	if s.IsVectorEnabled() {
		s.bufferMu.Lock()
		for _, r := range s.buffer {
			// 检查内容相似度
			bufferContent := r.Content
			// 去掉分类前缀进行比较
			if idx := strings.Index(bufferContent, "] "); idx > 0 {
				bufferContent = bufferContent[idx+2:]
			}
			if isSimilarContent(bufferContent, content) {
				s.bufferMu.Unlock()
				logger.Info("Skipping similar memory in buffer", "content", content[:min(50, len(content))])
				return nil
			}
		}
		s.bufferMu.Unlock()

		// 3. 搜索向量数据库中的相似记忆
		// 注意：这里不持有 bufferMu，避免死锁
		// FIXME(context): AddMemory should accept context parameter for proper cancellation
		// Priority: P2 - Estimated effort: 1 day
		// Related: #context-propagation #api-improvement
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		existing, err := s.Search(ctx, content, 1)
		if err == nil && len(existing) > 0 && existing[0].Score >= 0.95 {
			logger.Info("Skipping duplicate memory in vector store", "score", existing[0].Score, "content", content[:min(50, len(content))])
			return nil
		}
	}

	if err := s.fileStore.AddMemory(category, content); err != nil {
		return err
	}

	// 如果启用向量，索引记忆
	if s.IsVectorEnabled() {
		record := &VectorRecord{
			ID:      uuid.New().String(),
			Content: fmt.Sprintf("[%s] %s", category, content),
			Metadata: map[string]interface{}{
				"type":     "memory",
				"category": category,
			},
			Timestamp: time.Now(),
		}
		s.addToBuffer(record)
	}

	return nil
}

// isSimilarContent 检查两个内容是否相似
// 使用多种策略：完全匹配、子串匹配、关键词重叠
func isSimilarContent(a, b string) bool {
	// 完全相同
	if a == b {
		return true
	}

	// 子串匹配（双向）
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}

	// 关键词重叠检测
	// 提取关键词（简单分词：按空格和标点分割，保留中文连续字符）
	keywordsA := extractKeywords(a)
	keywordsB := extractKeywords(b)

	// 计算重叠度
	if len(keywordsA) > 0 && len(keywordsB) > 0 {
		overlap := 0
		for ka := range keywordsA {
			if keywordsB[ka] {
				overlap++
			}
		}
		// 如果重叠关键词占比超过 50%，认为相似
		ratioA := float64(overlap) / float64(len(keywordsA))
		ratioB := float64(overlap) / float64(len(keywordsB))
		if ratioA >= 0.5 || ratioB >= 0.5 {
			return true
		}
	}

	return false
}

// extractKeywords 提取关键词
func extractKeywords(text string) map[string]bool {
	keywords := make(map[string]bool)

	// 简单分词：提取中文词组和英文单词
	var current strings.Builder
	isChinese := func(r rune) bool {
		return r >= 0x4e00 && r <= 0x9fff
	}

	for _, r := range text {
		if isChinese(r) {
			current.WriteRune(r)
		} else if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			current.WriteRune(r)
		} else {
			// 遇到分隔符，保存当前词
			if current.Len() > 1 {
				keywords[current.String()] = true
			}
			current.Reset()
		}
	}
	// 保存最后一个词
	if current.Len() > 1 {
		keywords[current.String()] = true
	}

	// 对于中文，还要提取双字组合（简单 n-gram）
	runes := []rune(text)
	for i := 0; i < len(runes)-1; i++ {
		if isChinese(runes[i]) && isChinese(runes[i+1]) {
			bigram := string(runes[i : i+2])
			keywords[bigram] = true
		}
	}

	return keywords
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetMemory 获取 MEMORY.md 内容
func (s *HybridStore) GetMemory() (string, error) {
	return s.fileStore.GetMemory()
}

// SearchMemory 搜索记忆
func (s *HybridStore) SearchMemory(ctx context.Context, query string) ([]*VectorRecord, error) {
	if s.IsVectorEnabled() {
		return s.Search(ctx, query, 10)
	}

	// 回退到 grep 搜索
	lines, err := s.fileStore.SearchMemory(query)
	if err != nil {
		return nil, err
	}

	var records []*VectorRecord
	for i, line := range lines {
		records = append(records, &VectorRecord{
			ID:        fmt.Sprintf("memory-%d", i),
			Content:   line,
			Timestamp: time.Now(),
			Score:     1.0,
		})
	}

	return records, nil
}

// AddHistory 添加历史记录
func (s *HybridStore) AddHistory(eventType, summary string, details map[string]string) error {
	return s.fileStore.AddHistory(eventType, summary, details)
}

// GetRecentHistory 获取最近历史
func (s *HybridStore) GetRecentHistory(lines int) ([]string, error) {
	return s.fileStore.GetRecentHistory(lines)
}

// WriteDailyLog 写入每日日志
func (s *HybridStore) WriteDailyLog(content string) error {
	if err := s.fileStore.WriteDailyLog(content); err != nil {
		return err
	}

	// 如果启用向量，索引日志
	if s.IsVectorEnabled() {
		record := &VectorRecord{
			ID:      uuid.New().String(),
			Content: content,
			Metadata: map[string]interface{}{
				"type": "daily_log",
				"date": time.Now().Format("2006-01-02"),
			},
			Timestamp: time.Now(),
		}
		s.addToBuffer(record)
	}

	return nil
}

// GetRecentDailyLogs 获取最近几天的日志
func (s *HybridStore) GetRecentDailyLogs(days int) (map[string]string, error) {
	return s.fileStore.GetRecentDailyLogs(days)
}

// 缓冲管理

// addToBuffer 添加到缓冲
func (s *HybridStore) addToBuffer(record *VectorRecord) {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	s.buffer = append(s.buffer, record)

	// 达到缓冲大小或定时刷新
	if len(s.buffer) >= s.bufferSize {
		go s.flushBuffer()
	} else if s.flushTimer == nil {
		s.flushTimer = time.AfterFunc(5*time.Second, s.flushBuffer)
	}
}

// flushBuffer 刷新缓冲到向量存储
func (s *HybridStore) flushBuffer() {
	s.bufferMu.Lock()
	defer s.bufferMu.Unlock()

	if len(s.buffer) == 0 || s.vectorStore == nil {
		return
	}

	// 取出待处理的记录
	records := s.buffer
	s.buffer = make([]*VectorRecord, 0)

	// 停止定时器
	if s.flushTimer != nil {
		s.flushTimer.Stop()
		s.flushTimer = nil
	}

	// 异步生成向量并存储（添加 panic 恢复）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Vector flush goroutine panic recovered", "error", r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 批量生成向量
		texts := make([]string, len(records))
		for i, r := range records {
			texts[i] = r.Content
		}

		vectors, err := s.embedding.EmbedBatch(ctx, texts)
		if err != nil {
			logger.Warn("Vector flush failed", "error", err)
			return
		}

		// 设置向量
		for i, vec := range vectors {
			if i < len(records) {
				records[i].Vector = vec
			}
		}

		// 存储到向量数据库
		if err := s.vectorStore.Upsert(ctx, records); err != nil {
			logger.Warn("Vector upsert failed", "error", err)
		}
	}()
}

// 辅助函数

// createEmbeddingModel 创建 Embedding 模型
func createEmbeddingModel(cfg *config.VectorConfig, providers map[string]config.ProviderConfig) (embedding.Model, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("vector config not enabled")
	}

	embCfg := &embedding.Config{
		Provider:  cfg.Embedding.Provider,
		Model:     cfg.Embedding.Model,
		Dimension: cfg.Embedding.Dimension,
	}

	// 获取 API Key
	if cfg.Embedding.APIKey != "" {
		embCfg.APIKey = cfg.Embedding.APIKey
	} else if providers != nil {
		// 从 Provider 配置继承
		providerName := cfg.Embedding.Provider
		if providerName == "" {
			providerName = "qwen" // 默认使用 qwen
		}
		if p, ok := providers[providerName]; ok {
			embCfg.APIKey = p.APIKey
			embCfg.APIBase = p.APIBase
		}
	}

	if embCfg.APIKey == "" {
		return nil, fmt.Errorf("embedding API key not configured")
	}

	switch embCfg.Provider {
	case "qwen", "dashscope":
		return embedding.NewQwenEmbedding(embCfg), nil
	default:
		// 默认使用 qwen
		return embedding.NewQwenEmbedding(embCfg), nil
	}
}

// createReranker 创建重排序器
func createReranker(cfg *config.VectorConfig, providers map[string]config.ProviderConfig) Reranker {
	rerankCfg := cfg.Search.Rerank
	if rerankCfg == nil {
		return NewNoOpReranker()
	}

	apiKey := rerankCfg.APIKey
	if apiKey == "" && providers != nil {
		// 从 Provider 配置继承
		providerName := rerankCfg.Provider
		if providerName == "" {
			providerName = "qwen"
		}
		if p, ok := providers[providerName]; ok {
			apiKey = p.APIKey
		}
	}

	if apiKey == "" {
		return NewNoOpReranker()
	}

	return NewQwenReranker(&RerankConfig{
		Provider: rerankCfg.Provider,
		Model:    rerankCfg.Model,
		APIKey:   apiKey,
		APIBase:  rerankCfg.APIBase,
	})
}

// getVectorDbPath 获取向量数据库路径
func getVectorDbPath(cfg *config.VectorConfig, memoryDir string) string {
	if cfg.Database.Path != "" {
		return cfg.Database.Path
	}
	return memoryDir + "/vectors.db"
}

// getEmbeddingDimension 获取 Embedding 维度
func getEmbeddingDimension(cfg *config.VectorConfig) int {
	if cfg.Database.Dimension > 0 {
		return cfg.Database.Dimension
	}
	if cfg.Embedding.Dimension > 0 {
		return cfg.Embedding.Dimension
	}
	return embedding.DefaultDimension
}

// 确保 HybridStore 实现 Store 接口
var _ Store = (*HybridStore)(nil)
