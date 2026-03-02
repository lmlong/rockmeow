// Package memory - sqlite-vec 向量存储实现
package memory

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lingguard/pkg/embedding"
	_ "modernc.org/sqlite" // 纯 Go SQLite，无需 CGO
)

// VectorRecord 向量记录
type VectorRecord struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Vector    []float32              `json:"vector,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Score     float32                `json:"score,omitempty"` // 检索时使用
}

// SearchOptions 搜索选项
type SearchOptions struct {
	TopK      int                    `json:"topK"`
	MinScore  float32                `json:"minScore"`
	StartTime time.Time              `json:"startTime,omitempty"`
	EndTime   time.Time              `json:"endTime,omitempty"`
	Filters   map[string]interface{} `json:"filters,omitempty"`
}

// HybridSearchOptions 混合搜索选项
type HybridSearchOptions struct {
	TopK         int       `json:"topK"`
	VectorWeight float64   `json:"vectorWeight"` // 向量检索权重
	BM25Weight   float64   `json:"bm25Weight"`   // BM25 检索权重
	MinScore     float32   `json:"minScore"`
	StartTime    time.Time `json:"startTime,omitempty"`
	EndTime      time.Time `json:"endTime,omitempty"`
}

// VectorStoreConfig 向量存储配置
type VectorStoreConfig struct {
	DatabasePath string `json:"databasePath"`
	Dimension    int    `json:"dimension"`
}

// SQLiteVecStore sqlite-vec 向量存储
type SQLiteVecStore struct {
	db        *sql.DB
	embedding embedding.Model
	reranker  Reranker
	dimension int
	config    *VectorStoreConfig
	mu        sync.RWMutex
}

// NewSQLiteVecStore 创建 sqlite-vec 存储
func NewSQLiteVecStore(cfg *VectorStoreConfig, emb embedding.Model, reranker Reranker) (*SQLiteVecStore, error) {
	// 确保目录存在
	dir := filepath.Dir(cfg.DatabasePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	// 打开数据库
	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 设置连接池（SQLite 优化配置）
	// 虽然写操作是串行的，但读操作可以并发
	db.SetMaxOpenConns(10)                  // 允许多个并发读操作
	db.SetMaxIdleConns(5)                   // 保持 5 个空闲连接
	db.SetConnMaxLifetime(time.Hour)        // 连接最大生命周期 1 小时
	db.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接超时 10 分钟
	store := &SQLiteVecStore{
		db:        db,
		embedding: emb,
		reranker:  reranker,
		dimension: cfg.Dimension,
		config:    cfg,
	}

	// 初始化表
	if err := store.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init tables: %w", err)
	}

	return store, nil
}

// initTables 初始化数据库表
func (s *SQLiteVecStore) initTables() error {
	// 创建元数据表
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_vectors (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			metadata TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_memory_timestamp ON memory_vectors(timestamp);
	`)
	if err != nil {
		return fmt.Errorf("create memory table: %w", err)
	}

	// 创建 FTS 表（用于 BM25 检索）
	_, err = s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			id,
			content,
			content='memory_vectors',
			content_rowid='rowid'
		);

		-- 创建触发器以保持 FTS 索引同步
		CREATE TRIGGER IF NOT EXISTS memory_ai AFTER INSERT ON memory_vectors BEGIN
			INSERT INTO memory_fts(rowid, id, content) VALUES (new.rowid, new.id, new.content);
		END;

		CREATE TRIGGER IF NOT EXISTS memory_ad AFTER DELETE ON memory_vectors BEGIN
			INSERT INTO memory_fts(memory_fts, rowid, id, content) VALUES('delete', old.rowid, old.id, old.content);
		END;

		CREATE TRIGGER IF NOT EXISTS memory_au AFTER UPDATE ON memory_vectors BEGIN
			INSERT INTO memory_fts(memory_fts, rowid, id, content) VALUES('delete', old.rowid, old.id, old.content);
			INSERT INTO memory_fts(rowid, id, content) VALUES (new.rowid, new.id, new.content);
		END;
	`)
	if err != nil {
		// FTS5 可能不可用，尝试使用 LIKE 搜索
		// 非致命错误，继续
	}

	// 尝试加载 sqlite-vec 扩展并创建向量表
	// 如果扩展不可用，向量存储将使用内存向量
	err = s.initVectorTable()
	if err != nil {
		// 向量扩展不可用，使用纯内存向量存储
		// 这不是致命错误，但会降低向量搜索性能
	}

	return nil
}

// initVectorTable 初始化向量表
func (s *SQLiteVecStore) initVectorTable() error {
	// 尝试加载 sqlite-vec 扩展
	// 注意：需要预先编译 sqlite-vec 扩展
	// 这里我们使用纯 Go 实现的向量搜索作为备选方案

	// 创建向量存储表（使用 BLOB 存储）
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_vec_data (
			id TEXT PRIMARY KEY,
			vector BLOB NOT NULL,
			FOREIGN KEY (id) REFERENCES memory_vectors(id) ON DELETE CASCADE
		);
	`)
	return err
}

// Upsert 插入或更新向量记录
func (s *SQLiteVecStore) Upsert(ctx context.Context, records []*VectorRecord) error {
	if len(records) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, record := range records {
		if record.Timestamp.IsZero() {
			record.Timestamp = time.Now()
		}

		// 序列化 metadata
		metadataJSON := "{}"
		if record.Metadata != nil {
			if data, err := serializeMetadata(record.Metadata); err == nil {
				metadataJSON = data
			}
		}

		// 插入或更新元数据
		_, err := tx.Exec(`
			INSERT OR REPLACE INTO memory_vectors (id, content, metadata, timestamp)
			VALUES (?, ?, ?, ?)
		`, record.ID, record.Content, metadataJSON, record.Timestamp)
		if err != nil {
			return fmt.Errorf("upsert metadata: %w", err)
		}

		// 如果有向量，存储向量
		if len(record.Vector) > 0 {
			vectorBlob := floatsToBlob(record.Vector)
			_, err = tx.Exec(`
				INSERT OR REPLACE INTO memory_vec_data (id, vector)
				VALUES (?, ?)
			`, record.ID, vectorBlob)
			if err != nil {
				return fmt.Errorf("upsert vector: %w", err)
			}
		}
	}

	return tx.Commit()
}

// Search 向量相似度搜索
// 优化：使用 SQL LIMIT 限制初始加载数量，避免全量加载
func (s *SQLiteVecStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]*VectorRecord, error) {
	if opts.TopK <= 0 {
		opts.TopK = 10
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 优化：只加载最新的记录，避免全量加载
	// 取 TopK * 10 条记录用于计算相似度，这在大多数情况下足够
	limit := opts.TopK * 10
	if limit > 1000 {
		limit = 1000 // 最大限制 1000 条
	}
	if limit < 50 {
		limit = 50 // 最小 50 条
	}

	// 获取最新的记录和向量
	rows, err := s.db.QueryContext(ctx, `
		SELECT v.id, v.content, v.metadata, v.timestamp, vec.vector
		FROM memory_vectors v
		LEFT JOIN memory_vec_data vec ON v.id = vec.id
		ORDER BY v.timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query records: %w", err)
	}
	defer rows.Close()

	var records []*VectorRecord
	var vectors [][]float32

	for rows.Next() {
		var record VectorRecord
		var metadataJSON string
		var vectorBlob []byte

		err := rows.Scan(&record.ID, &record.Content, &metadataJSON, &record.Timestamp, &vectorBlob)
		if err != nil {
			continue
		}

		// 解析 metadata
		record.Metadata = parseMetadata(metadataJSON)

		// 解析向量
		if len(vectorBlob) > 0 {
			record.Vector = blobToFloats(vectorBlob)
		}

		records = append(records, &record)
		vectors = append(vectors, record.Vector)
	}

	if len(records) == 0 {
		return nil, nil
	}

	// 计算余弦相似度
	results := make([]*VectorRecord, 0, len(records))
	for i, record := range records {
		if len(vectors[i]) == 0 {
			continue
		}
		score := cosineSimilarity(query, vectors[i])
		if score >= opts.MinScore {
			record.Score = score
			results = append(results, record)
		}
	}

	// 按分数排序
	sortRecordsByScore(results)

	// 返回 TopK
	if len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
}

// SearchBM25 BM25 全文检索
func (s *SQLiteVecStore) SearchBM25(ctx context.Context, query string, opts SearchOptions) ([]*VectorRecord, error) {
	if opts.TopK <= 0 {
		opts.TopK = 10
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 尝试使用 FTS5
	var rows *sql.Rows
	var err error

	// 检查 FTS 表是否存在
	var ftsExists int
	err = s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM sqlite_master
		WHERE type='table' AND name='memory_fts'
	`).Scan(&ftsExists)

	if err == nil && ftsExists > 0 {
		// 使用 FTS5 搜索
		rows, err = s.db.QueryContext(ctx, `
			SELECT v.id, v.content, v.metadata, v.timestamp, fts.rank as score
			FROM memory_fts fts
			JOIN memory_vectors v ON fts.id = v.id
			WHERE memory_fts MATCH ?
			ORDER BY score DESC
			LIMIT ?
		`, query, opts.TopK)
	} else {
		// 回退到 LIKE 搜索
		searchPattern := "%" + query + "%"
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, content, metadata, timestamp, 1.0 as score
			FROM memory_vectors
			WHERE content LIKE ?
			ORDER BY timestamp DESC
			LIMIT ?
		`, searchPattern, opts.TopK)
	}

	if err != nil {
		return nil, fmt.Errorf("bm25 search: %w", err)
	}
	defer rows.Close()

	var results []*VectorRecord
	for rows.Next() {
		var record VectorRecord
		var metadataJSON string

		err := rows.Scan(&record.ID, &record.Content, &metadataJSON, &record.Timestamp, &record.Score)
		if err != nil {
			continue
		}

		record.Metadata = parseMetadata(metadataJSON)
		results = append(results, &record)
	}

	return results, nil
}

// HybridSearch 混合检索 (向量 + BM25 + RRF 融合)
func (s *SQLiteVecStore) HybridSearch(ctx context.Context, queryVec []float32, textQuery string, opts HybridSearchOptions) ([]*VectorRecord, error) {
	if opts.TopK <= 0 {
		opts.TopK = 10
	}

	// 并行执行向量检索和 BM25 检索
	var vectorResults, bm25Results []*VectorRecord
	var vectorErr, bm25Err error
	var wg sync.WaitGroup

	wg.Add(2)

	// 向量检索
	go func() {
		defer wg.Done()
		searchOpts := SearchOptions{
			TopK:     opts.TopK * 2, // 获取更多结果用于融合
			MinScore: 0.0,
		}
		vectorResults, vectorErr = s.Search(ctx, queryVec, searchOpts)
	}()

	// BM25 检索
	go func() {
		defer wg.Done()
		searchOpts := SearchOptions{
			TopK: opts.TopK * 2,
		}
		bm25Results, bm25Err = s.SearchBM25(ctx, textQuery, searchOpts)
	}()

	wg.Wait()

	if vectorErr != nil && bm25Err != nil {
		return nil, fmt.Errorf("both searches failed: vector=%v, bm25=%v", vectorErr, bm25Err)
	}

	// RRF (Reciprocal Rank Fusion) 融合
	fusedResults := rrfFusion(vectorResults, bm25Results, opts.VectorWeight, opts.BM25Weight)

	// 应用最小分数过滤
	var filtered []*VectorRecord
	for _, r := range fusedResults {
		if r.Score >= opts.MinScore {
			filtered = append(filtered, r)
		}
	}

	// 如果有重排序器，执行重排序
	if s.reranker != nil && len(filtered) > 0 {
		documents := make([]string, len(filtered))
		for i, r := range filtered {
			documents[i] = r.Content
		}

		rerankResults, err := s.reranker.Rerank(ctx, textQuery, documents, opts.TopK)
		if err == nil && len(rerankResults) > 0 {
			// 重新排列结果
			reranked := make([]*VectorRecord, len(rerankResults))
			for i, rr := range rerankResults {
				if rr.Index < len(filtered) {
					record := filtered[rr.Index]
					record.Score = rr.RelevanceScore
					reranked[i] = record
				}
			}
			filtered = reranked
		}
	}

	// 返回 TopK
	if len(filtered) > opts.TopK {
		filtered = filtered[:opts.TopK]
	}

	return filtered, nil
}

// GetByID 根据 ID 获取记录
func (s *SQLiteVecStore) GetByID(ctx context.Context, id string) (*VectorRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var record VectorRecord
	var metadataJSON string
	var vectorBlob []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT v.id, v.content, v.metadata, v.timestamp, vec.vector
		FROM memory_vectors v
		LEFT JOIN memory_vec_data vec ON v.id = vec.id
		WHERE v.id = ?
	`, id).Scan(&record.ID, &record.Content, &metadataJSON, &record.Timestamp, &vectorBlob)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get by id: %w", err)
	}

	record.Metadata = parseMetadata(metadataJSON)
	if len(vectorBlob) > 0 {
		record.Vector = blobToFloats(vectorBlob)
	}

	return &record, nil
}

// Delete 删除记录
func (s *SQLiteVecStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	// 使用参数化查询，防止 SQL 注入
	query := "DELETE FROM memory_vectors WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err = tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("delete records: %w", err)
	}

	return tx.Commit()
}

// Count 获取记录总数
func (s *SQLiteVecStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_vectors").Scan(&count)
	return count, err
}

// Close 关闭存储
func (s *SQLiteVecStore) Close() error {
	return s.db.Close()
}

// Embedding 获取嵌入模型
func (s *SQLiteVecStore) Embedding() embedding.Model {
	return s.embedding
}

// GenerateEmbedding 生成嵌入向量
func (s *SQLiteVecStore) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if s.embedding == nil {
		return nil, fmt.Errorf("embedding model not configured")
	}
	return s.embedding.Embed(ctx, text)
}

// 辅助函数

// floatsToBlob 将 float32 切片转换为 BLOB
func floatsToBlob(floats []float32) []byte {
	blob := make([]byte, len(floats)*4)
	for i, f := range floats {
		// 使用小端序
		bits := float32bits(f)
		blob[i*4] = byte(bits)
		blob[i*4+1] = byte(bits >> 8)
		blob[i*4+2] = byte(bits >> 16)
		blob[i*4+3] = byte(bits >> 24)
	}
	return blob
}

// blobToFloats 将 BLOB 转换为 float32 切片
func blobToFloats(blob []byte) []float32 {
	if len(blob)%4 != 0 {
		return nil
	}
	floats := make([]float32, len(blob)/4)
	for i := range floats {
		bits := uint32(blob[i*4]) | uint32(blob[i*4+1])<<8 | uint32(blob[i*4+2])<<16 | uint32(blob[i*4+3])<<24
		floats[i] = float32frombits(bits)
	}
	return floats
}

// float32bits 返回 f 的位表示
func float32bits(f float32) uint32 {
	return math.Float32bits(f)
}

// float32frombits 返回位表示对应的 float32
func float32frombits(b uint32) float32 {
	return math.Float32frombits(b)
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt32(normA) * sqrt32(normB))
}

// sqrt32 计算 float32 的平方根
func sqrt32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

// sortRecordsByScore 按分数降序排序
func sortRecordsByScore(records []*VectorRecord) {
	sort.Slice(records, func(i, j int) bool {
		return records[i].Score > records[j].Score
	})
}

// rrfFusion RRF 融合算法
func rrfFusion(vectorResults, bm25Results []*VectorRecord, vectorWeight, bm25Weight float64) []*VectorRecord {
	// RRF 参数
	k := 60.0

	// 构建分数映射
	scores := make(map[string]float64)

	// 向量结果
	for i, r := range vectorResults {
		rrfScore := vectorWeight / (k + float64(i+1))
		scores[r.ID] += rrfScore
	}

	// BM25 结果
	for i, r := range bm25Results {
		rrfScore := bm25Weight / (k + float64(i+1))
		scores[r.ID] += rrfScore
	}

	// 合并去重
	recordMap := make(map[string]*VectorRecord)
	for _, r := range vectorResults {
		recordMap[r.ID] = r
	}
	for _, r := range bm25Results {
		if _, exists := recordMap[r.ID]; !exists {
			recordMap[r.ID] = r
		}
	}

	// 构建结果
	results := make([]*VectorRecord, 0, len(recordMap))
	for id, score := range scores {
		if r, exists := recordMap[id]; exists {
			r.Score = float32(score)
			results = append(results, r)
		}
	}

	// 排序
	sortRecordsByScore(results)

	return results
}

// serializeMetadata 序列化 metadata
func serializeMetadata(m map[string]interface{}) (string, error) {
	if m == nil {
		return "{}", nil
	}
	// 简化实现
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf(`"%s":"%v"`, k, v))
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

// parseMetadata 解析 metadata
func parseMetadata(s string) map[string]interface{} {
	if s == "" || s == "{}" {
		return nil
	}
	// 简化实现，返回原始字符串
	return map[string]interface{}{"raw": s}
}
