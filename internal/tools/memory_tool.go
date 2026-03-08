// Package tools 工具实现 - 记忆工具
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/memory"
)

// MemoryTool 记忆工具（参考 nanobot）
// 允许 Agent 记录和回忆信息
type MemoryTool struct {
	store       *memory.FileStore
	hybridStore *memory.HybridStore // 可选：支持向量检索
	builder     *memory.ContextBuilder
	refiner     *memory.Refiner // 可选：记忆提炼器
}

// NewMemoryTool 创建记忆工具
func NewMemoryTool(memoryDir string) *MemoryTool {
	// 展开路径
	if len(memoryDir) > 0 && memoryDir[0] == '~' {
		home, _ := os.UserHomeDir()
		memoryDir = filepath.Join(home, memoryDir[1:])
	}

	store := memory.NewFileStore(memoryDir)
	if err := store.Init(); err != nil {
		// 初始化失败时记录日志但继续
		fmt.Printf("Warning: failed to init memory store: %v\n", err)
	}

	return &MemoryTool{
		store:   store,
		builder: memory.NewContextBuilder(store),
	}
}

// NewMemoryToolFromStore 从已有存储创建记忆工具
func NewMemoryToolFromStore(store *memory.FileStore) *MemoryTool {
	return &MemoryTool{
		store:   store,
		builder: memory.NewContextBuilder(store),
	}
}

// NewMemoryToolFromHybridStore 从混合存储创建记忆工具
func NewMemoryToolFromHybridStore(store *memory.HybridStore) *MemoryTool {
	return &MemoryTool{
		store:       store.FileStore(),
		hybridStore: store,
		builder:     memory.NewContextBuilderWithHybrid(store),
		refiner:     memory.NewRefiner(store.FileStore(), store, nil),
	}
}

// SetRefiner 设置提炼器
func (t *MemoryTool) SetRefiner(cfg *config.RefineConfig) {
	t.refiner = memory.NewRefiner(t.store, t.hybridStore, cfg)
}

// Name 返回工具名称
func (t *MemoryTool) Name() string {
	return "memory"
}

// Description 返回工具描述
func (t *MemoryTool) Description() string {
	desc := `Memory tool for storing and retrieving information.

Actions:
- remember: Store a fact in long-term memory
- recall: Search memories for relevant information
- log: Write an event to daily log
- context: Get current memory context
- refine: Deduplicate and consolidate memory entries

Usage:
{"action": "remember", "category": "User Preferences", "fact": "User prefers Go over Python"}
{"action": "recall", "query": "user preferences"}
{"action": "log", "event": "Completed task X"}
{"action": "context"}
{"action": "refine"}`

	if t.hybridStore != nil && t.hybridStore.IsVectorEnabled() {
		desc += `

Note: Vector-based semantic search is enabled for better recall results.`
	}

	return desc
}

// Parameters 返回参数定义
func (t *MemoryTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"remember", "recall", "log", "context", "refine"},
				"description": "The memory action to perform",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Category for remember action (e.g., 'User Preferences', 'Project Context', 'Important Facts')",
			},
			"fact": map[string]interface{}{
				"type":        "string",
				"description": "The fact to remember",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query for recall action",
			},
			"event": map[string]interface{}{
				"type":        "string",
				"description": "Event description for log action",
			},
		},
		"required": []string{"action"},
	}
}

// Execute 执行工具
func (t *MemoryTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action     string `json:"action"`
		Category   string `json:"category"`
		Fact       string `json:"fact"`
		Query      string `json:"query"`
		Event      string `json:"event"`
		ArchiveOld bool   `json:"archiveOld"` // 是否只归档旧记忆（超过滑动窗口）
		RecentDays int    `json:"recentDays"` // 滑动窗口天数（用于 archiveOld）
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	switch params.Action {
	case "remember":
		return t.actionRemember(params.Category, params.Fact)
	case "recall":
		return t.actionRecall(params.Query)
	case "log":
		return t.actionLog(params.Event)
	case "context":
		return t.actionContext()
	case "refine":
		return t.actionRefine(ctx, params.ArchiveOld, params.RecentDays)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

func (t *MemoryTool) actionRemember(category, fact string) (string, error) {
	if category == "" {
		category = "General"
	}
	if fact == "" {
		return "", fmt.Errorf("fact is required for remember action")
	}

	// 优先使用 HybridStore（支持向量索引）
	if t.hybridStore != nil {
		if err := t.hybridStore.AddMemory(category, fact); err != nil {
			return "", fmt.Errorf("failed to remember: %w", err)
		}
	} else {
		// 回退到 FileStore
		if err := t.store.AddMemory(category, fact); err != nil {
			return "", fmt.Errorf("failed to remember: %w", err)
		}
	}

	return fmt.Sprintf("Remembered: [%s] %s", category, fact), nil
}

func (t *MemoryTool) actionRecall(query string) (string, error) {
	if query == "" {
		// 如果没有查询，返回整个记忆上下文
		return t.actionContext()
	}

	// 如果启用向量检索，使用语义搜索
	if t.hybridStore != nil && t.hybridStore.IsVectorEnabled() {
		return t.actionRecallSemantic(query)
	}

	// 回退到关键词搜索
	results, err := t.store.SearchAll(query)
	if err != nil {
		return "", fmt.Errorf("failed to recall: %w", err)
	}

	if len(results) == 0 {
		return "No matching memories found.", nil
	}

	var output strings.Builder
	for file, lines := range results {
		output.WriteString(fmt.Sprintf("From %s:\n", file))
		for _, line := range lines {
			output.WriteString(line + "\n")
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}

// actionRecallSemantic 使用语义搜索召回记忆
func (t *MemoryTool) actionRecallSemantic(query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	records, err := t.hybridStore.Search(ctx, query, 10)
	if err != nil {
		return "", fmt.Errorf("failed to recall (semantic): %w", err)
	}

	if len(records) == 0 {
		return "No matching memories found.", nil
	}

	var output strings.Builder
	output.WriteString("## Relevant Memories (Semantic Search)\n\n")
	for _, record := range records {
		content := record.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		output.WriteString(fmt.Sprintf("- [%.2f] %s\n", record.Score, content))
	}

	return output.String(), nil
}

func (t *MemoryTool) actionLog(event string) (string, error) {
	if event == "" {
		return "", fmt.Errorf("event is required for log action")
	}

	if err := t.store.WriteDailyLog(event); err != nil {
		return "", fmt.Errorf("failed to log: %w", err)
	}

	return fmt.Sprintf("Logged: %s", event), nil
}

func (t *MemoryTool) actionContext() (string, error) {
	recentDays := 3
	ctx, err := t.builder.BuildContext(recentDays)
	if err != nil {
		return "", fmt.Errorf("failed to get context: %w", err)
	}

	if ctx == "" {
		return "No memory context available yet.", nil
	}

	return ctx, nil
}

// actionRefine 执行记忆提炼
// archiveOld: 是否只归档超过滑动窗口的旧记忆
// recentDays: 滑动窗口天数（仅 archiveOld=true 时有效）
func (t *MemoryTool) actionRefine(ctx context.Context, archiveOld bool, recentDays int) (string, error) {
	if t.refiner == nil {
		// 如果没有配置提炼器，创建一个默认的
		t.refiner = memory.NewRefiner(t.store, t.hybridStore, nil)
	}

	var result *memory.RefineResult
	var err error

	if archiveOld {
		// 归档模式：只处理超过滑动窗口的旧记忆
		result, err = t.refiner.ArchiveOld(ctx, recentDays)
	} else {
		// 全量提炼模式
		result, err = t.refiner.Refine(ctx)
	}

	if err != nil {
		return "", fmt.Errorf("failed to refine memory: %w", err)
	}

	var output strings.Builder

	if archiveOld {
		output.WriteString("## Memory Archive Complete\n\n")
	} else {
		output.WriteString("## Memory Refinement Complete\n\n")
	}

	output.WriteString(fmt.Sprintf("- **Total entries**: %d\n", result.TotalEntries))
	output.WriteString(fmt.Sprintf("- **Merged entries**: %d\n", result.MergedEntries))
	output.WriteString(fmt.Sprintf("- **Removed duplicates**: %d\n", result.RemovedEntries))

	if !archiveOld {
		output.WriteString(fmt.Sprintf("- **Duplicate groups found**: %d\n", result.DuplicateGroups))
	}

	if result.BackupPath != "" {
		output.WriteString(fmt.Sprintf("- **Backup saved to**: %s\n", result.BackupPath))
	}

	if len(result.Changes) > 0 {
		output.WriteString("\n### Changes Made\n")
		for _, change := range result.Changes {
			output.WriteString(fmt.Sprintf("- %s\n", change))
		}
	}

	if result.RemovedEntries == 0 {
		if archiveOld {
			output.WriteString("\nNo old entries to archive or no duplicates found.\n")
		} else {
			output.WriteString("\nNo duplicate entries found. Memory is already well-organized.\n")
		}
	}

	return output.String(), nil
}

// GetStore 获取存储实例
func (t *MemoryTool) GetStore() *memory.FileStore {
	return t.store
}

// GetBuilder 获取上下文构建器
func (t *MemoryTool) GetBuilder() *memory.ContextBuilder {
	return t.builder
}

// IsDangerous 返回是否为危险操作
func (t *MemoryTool) IsDangerous() bool {
	return false
}

// ShouldLoadByDefault 返回是否默认加载
func (t *MemoryTool) ShouldLoadByDefault() bool {
	return true
}
