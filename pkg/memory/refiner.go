// Package memory - 记忆提炼器
package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lingguard/internal/config"
	"github.com/lingguard/pkg/logger"
)

// Refiner 记忆提炼器
type Refiner struct {
	fileStore   *FileStore
	hybridStore *HybridStore // 可选，用于语义相似度检测
	config      *config.RefineConfig
}

// NewRefiner 创建记忆提炼器
func NewRefiner(fileStore *FileStore, hybridStore *HybridStore, cfg *config.RefineConfig) *Refiner {
	if cfg == nil {
		cfg = &config.RefineConfig{
			Enabled:               true,
			AutoTrigger:           false,
			Threshold:             50,
			SimilarityThreshold:   0.85,
			KeepBackup:            true,
			MaxEntriesPerCategory: 20,
		}
	}
	return &Refiner{
		fileStore:   fileStore,
		hybridStore: hybridStore,
		config:      cfg,
	}
}

// MemoryEntry 记忆条目
type MemoryEntry struct {
	Category  string    // 分类
	Content   string    // 内容（不含时间戳前缀）
	Timestamp time.Time // 时间戳
	RawLine   string    // 原始行
}

// RefineResult 提炼结果
type RefineResult struct {
	TotalEntries    int            // 原始条目总数
	DuplicateGroups int            // 重复组数
	MergedEntries   int            // 合并后的条目数
	RemovedEntries  int            // 移除的条目数
	Categories      map[string]int // 各分类条目数
	Changes         []string       // 变更摘要
	BackupPath      string         // 备份文件路径（如果有）
}

// Refine 执行记忆提炼
func (r *Refiner) Refine(ctx context.Context) (*RefineResult, error) {
	result := &RefineResult{
		Categories: make(map[string]int),
		Changes:    make([]string, 0),
	}

	// 1. 读取当前 MEMORY.md
	content, err := r.fileStore.GetMemory()
	if err != nil {
		return nil, fmt.Errorf("read memory: %w", err)
	}

	// 2. 解析所有条目
	entries := r.parseEntries(content)
	result.TotalEntries = len(entries)
	logger.Info("Memory refine: parsed entries", "count", len(entries))

	// 3. 按分类分组
	categoryEntries := r.groupByCategory(entries)

	// 4. 去重合并
	refinedEntries := make(map[string][]MemoryEntry)
	for category, cats := range categoryEntries {
		refined := r.deduplicateCategory(ctx, category, cats, result)
		refinedEntries[category] = refined
		result.Categories[category] = len(refined)
	}

	// 5. 计算变更
	result.MergedEntries = 0
	for _, entries := range refinedEntries {
		result.MergedEntries += len(entries)
	}
	result.RemovedEntries = result.TotalEntries - result.MergedEntries

	// 如果没有变化，直接返回
	if result.RemovedEntries == 0 {
		logger.Info("Memory refine: no changes needed")
		return result, nil
	}

	// 6. 备份原始文件
	if r.config.KeepBackup {
		backupPath, err := r.createBackup()
		if err != nil {
			logger.Warn("Failed to create backup", "error", err)
		} else {
			result.BackupPath = backupPath
			result.Changes = append(result.Changes, fmt.Sprintf("备份已保存到: %s", backupPath))
		}
	}

	// 7. 写入新的 MEMORY.md
	newContent := r.buildRefinedContent(refinedEntries)
	memoryPath := filepath.Join(r.fileStore.GetMemoryDir(), "MEMORY.md")
	if err := os.WriteFile(memoryPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write refined memory: %w", err)
	}

	result.Changes = append(result.Changes, fmt.Sprintf("合并了 %d 条重复条目", result.RemovedEntries))
	logger.Info("Memory refine completed", "total", result.TotalEntries, "merged", result.MergedEntries, "removed", result.RemovedEntries)

	return result, nil
}

// parseEntries 解析 MEMORY.md 中的所有条目
func (r *Refiner) parseEntries(content string) []MemoryEntry {
	var entries []MemoryEntry
	var currentCategory string

	lines := strings.Split(content, "\n")
	entryPattern := regexp.MustCompile(`^- \[(\d{4}-\d{2}-\d{2}(?:\s+\d{2}:\d{2})?)\]\s*(.+)$`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检测分类标题
		if strings.HasPrefix(trimmed, "## ") {
			currentCategory = strings.TrimPrefix(trimmed, "## ")
			continue
		}

		// 跳过注释和空行
		if trimmed == "" || strings.HasPrefix(trimmed, "<!--") || strings.HasSuffix(trimmed, "-->") {
			continue
		}

		// 解析条目
		if matches := entryPattern.FindStringSubmatch(trimmed); matches != nil {
			timestampStr := matches[1]
			content := matches[2]

			// 解析时间戳
			timestamp, err := time.Parse("2006-01-02 15:04", timestampStr)
			if err != nil {
				timestamp, _ = time.Parse("2006-01-02", timestampStr)
			}

			entries = append(entries, MemoryEntry{
				Category:  currentCategory,
				Content:   content,
				Timestamp: timestamp,
				RawLine:   trimmed,
			})
		}
	}

	return entries
}

// groupByCategory 按分类分组
func (r *Refiner) groupByCategory(entries []MemoryEntry) map[string][]MemoryEntry {
	groups := make(map[string][]MemoryEntry)
	for _, entry := range entries {
		groups[entry.Category] = append(groups[entry.Category], entry)
	}
	return groups
}

// deduplicateCategory 对单个分类进行去重
func (r *Refiner) deduplicateCategory(ctx context.Context, category string, entries []MemoryEntry, result *RefineResult) []MemoryEntry {
	if len(entries) <= 1 {
		return entries
	}

	// 使用简单的字符串相似度检测
	groups := r.findDuplicateGroups(entries)

	var refined []MemoryEntry
	processed := make(map[int]bool)

	for i, entry := range entries {
		if processed[i] {
			continue
		}

		// 检查是否属于某个重复组
		if group, ok := groups[i]; ok {
			// 选择最新的条目作为代表
			latest := entry
			for _, idx := range group {
				processed[idx] = true
				if entries[idx].Timestamp.After(latest.Timestamp) {
					latest = entries[idx]
				}
			}
			if len(group) > 0 {
				result.DuplicateGroups++
				result.Changes = append(result.Changes, fmt.Sprintf("[%s] 合并 %d 条相似条目: %s",
					category, len(group)+1, truncateString(latest.Content, 50)))
			}
			refined = append(refined, latest)
		} else {
			processed[i] = true
			refined = append(refined, entry)
		}
	}

	// 限制每分类最大条目数
	if r.config.MaxEntriesPerCategory > 0 && len(refined) > r.config.MaxEntriesPerCategory {
		// 按时间排序，保留最新的
		sort.Slice(refined, func(i, j int) bool {
			return refined[i].Timestamp.After(refined[j].Timestamp)
		})
		removed := len(refined) - r.config.MaxEntriesPerCategory
		refined = refined[:r.config.MaxEntriesPerCategory]
		result.Changes = append(result.Changes, fmt.Sprintf("[%s] 移除 %d 条旧条目（超过最大数量限制）", category, removed))
	}

	return refined
}

// findDuplicateGroups 找出重复条目组
func (r *Refiner) findDuplicateGroups(entries []MemoryEntry) map[int][]int {
	groups := make(map[int][]int)
	processed := make(map[int]bool)

	for i := 0; i < len(entries); i++ {
		if processed[i] {
			continue
		}

		var duplicates []int
		for j := i + 1; j < len(entries); j++ {
			if processed[j] {
				continue
			}

			// 计算相似度
			similarity := r.calculateSimilarity(entries[i].Content, entries[j].Content)
			threshold := float32(0.85)
			if r.config.SimilarityThreshold > 0 {
				threshold = r.config.SimilarityThreshold
			}

			if similarity >= threshold {
				duplicates = append(duplicates, j)
				processed[j] = true
			}
		}

		if len(duplicates) > 0 {
			groups[i] = duplicates
		}
	}

	return groups
}

// calculateSimilarity 计算两个字符串的相似度
func (r *Refiner) calculateSimilarity(a, b string) float32 {
	// 1. 关键词相似度
	keywordsA := extractKeywords(a)
	keywordsB := extractKeywords(b)

	keywordSim := float32(0)
	if len(keywordsA) > 0 && len(keywordsB) > 0 {
		intersection := 0
		for k := range keywordsA {
			if keywordsB[k] {
				intersection++
			}
		}
		union := len(keywordsA) + len(keywordsB) - intersection
		keywordSim = float32(intersection) / float32(union)
	}

	// 2. 字符级相似度（对中文更有效）
	runesA := []rune(a)
	runesB := []rune(b)
	charSim := calculateCharSimilarity(runesA, runesB)

	// 3. 包含关系加分
	containBonus := float32(0)
	if strings.Contains(a, b) || strings.Contains(b, a) {
		containBonus = 0.3
	}

	// 综合评分：关键词占 40%，字符级占 60%，包含关系额外加分
	similarity := keywordSim*0.4 + charSim*0.6 + containBonus
	if similarity > 1 {
		similarity = 1
	}

	return similarity
}

// calculateCharSimilarity 计算字符级相似度
func calculateCharSimilarity(a, b []rune) float32 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	// 统计字符出现频率
	countA := make(map[rune]int)
	countB := make(map[rune]int)

	for _, r := range a {
		countA[r]++
	}
	for _, r := range b {
		countB[r]++
	}

	// 计算交集（取较小的计数）
	intersection := 0
	for r, ca := range countA {
		if cb, ok := countB[r]; ok {
			if ca < cb {
				intersection += ca
			} else {
				intersection += cb
			}
		}
	}

	// 计算并集
	union := len(a) + len(b) - intersection
	return float32(intersection) / float32(union)
}

// buildRefinedContent 构建提炼后的内容
func (r *Refiner) buildRefinedContent(entries map[string][]MemoryEntry) string {
	var sb strings.Builder

	sb.WriteString(`# Memory

This file stores long-term memories and important facts.

`)

	// 定义分类顺序
	categoryOrder := []string{
		"User Preferences",
		"Project Context",
		"Important Facts",
		"Decisions",
		"Contact Info",
		"Other",
	}

	// 添加已存在的分类
	for _, category := range categoryOrder {
		cats, ok := entries[category]
		if !ok || len(cats) == 0 {
			// 写入空分类（保留模板）
			sb.WriteString(fmt.Sprintf("## %s\n<!-- %s -->\n\n", category, getCategoryComment(category)))
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n", category))

		// 按时间排序（最新的在前）
		sort.Slice(cats, func(i, j int) bool {
			return cats[i].Timestamp.After(cats[j].Timestamp)
		})

		for _, entry := range cats {
			timestamp := entry.Timestamp.Format("2006-01-02 15:04")
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", timestamp, entry.Content))
		}
		sb.WriteString("\n")
	}

	// 添加其他分类
	for category, cats := range entries {
		found := false
		for _, c := range categoryOrder {
			if c == category {
				found = true
				break
			}
		}
		if found || len(cats) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n", category))
		sort.Slice(cats, func(i, j int) bool {
			return cats[i].Timestamp.After(cats[j].Timestamp)
		})
		for _, entry := range cats {
			timestamp := entry.Timestamp.Format("2006-01-02 15:04")
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", timestamp, entry.Content))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// createBackup 创建备份
func (r *Refiner) createBackup() (string, error) {
	memoryPath := filepath.Join(r.fileStore.GetMemoryDir(), "MEMORY.md")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(r.fileStore.GetMemoryDir(), fmt.Sprintf("MEMORY.backup.%s.md", timestamp))

	data, err := os.ReadFile(memoryPath)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", err
	}

	return backupPath, nil
}

// ShouldTriggerRefine 检查是否应该触发提炼
func (r *Refiner) ShouldTriggerRefine() bool {
	if r.config == nil || !r.config.Enabled || !r.config.AutoTrigger {
		return false
	}

	// 读取当前条目数
	content, err := r.fileStore.GetMemory()
	if err != nil {
		return false
	}

	entries := r.parseEntries(content)
	threshold := r.config.Threshold
	if threshold <= 0 {
		threshold = 50
	}

	return len(entries) >= threshold
}

// ArchiveOld 归档超过滑动窗口的旧记忆到 ARCHIVE.md
// 只对窗口外的旧条目进行去重合并并移动到归档文件，窗口内的条目保留在 MEMORY.md
func (r *Refiner) ArchiveOld(ctx context.Context, recentDays int) (*RefineResult, error) {
	if recentDays <= 0 {
		recentDays = 3 // 默认3天
	}

	result := &RefineResult{
		Categories: make(map[string]int),
		Changes:    make([]string, 0),
	}

	// 1. 读取当前 MEMORY.md
	content, err := r.fileStore.GetMemory()
	if err != nil {
		return nil, fmt.Errorf("read memory: %w", err)
	}

	// 2. 解析所有条目
	entries := r.parseEntries(content)
	result.TotalEntries = len(entries)
	logger.Info("ArchiveOld: parsed entries", "count", len(entries), "recentDays", recentDays)

	// 3. 计算截止时间
	cutoffTime := time.Now().AddDate(0, 0, -recentDays)

	// 4. 分离旧条目和近期条目
	var oldEntries, recentEntries []MemoryEntry
	for _, entry := range entries {
		if entry.Timestamp.Before(cutoffTime) {
			oldEntries = append(oldEntries, entry)
		} else {
			recentEntries = append(recentEntries, entry)
		}
	}

	logger.Info("ArchiveOld: separated entries", "old", len(oldEntries), "recent", len(recentEntries))

	// 5. 如果没有旧条目，直接返回
	if len(oldEntries) == 0 {
		logger.Info("ArchiveOld: no old entries to archive")
		return result, nil
	}

	// 6. 备份原始文件
	if r.config == nil || r.config.KeepBackup {
		backupPath, err := r.createBackup()
		if err != nil {
			logger.Warn("Failed to create backup", "error", err)
		} else {
			result.BackupPath = backupPath
			result.Changes = append(result.Changes, fmt.Sprintf("备份已保存到: %s", backupPath))
		}
	}

	// 7. 对旧条目进行去重合并
	oldByCategory := r.groupByCategory(oldEntries)
	refinedOld := make(map[string][]MemoryEntry)
	for category, cats := range oldByCategory {
		refined := r.deduplicateCategory(ctx, category, cats, result)
		refinedOld[category] = refined
	}

	// 8. 读取现有 ARCHIVE.md 并合并（避免重复归档）
	archivePath := filepath.Join(r.fileStore.GetMemoryDir(), "ARCHIVE.md")
	existingArchive, _ := os.ReadFile(archivePath)
	existingArchiveSlice := r.parseEntries(string(existingArchive))
	existingArchiveEntries := r.groupByCategory(existingArchiveSlice)

	// 合并：现有归档 + 新归档（去重）
	mergedArchive := r.mergeArchiveEntries(existingArchiveEntries, refinedOld)

	// 9. 写入 ARCHIVE.md
	archiveContent := r.buildArchiveContent(mergedArchive)
	if err := os.WriteFile(archivePath, []byte(archiveContent), 0644); err != nil {
		return nil, fmt.Errorf("write archive: %w", err)
	}

	// 10. 写入新的 MEMORY.md（只包含近期条目）
	recentByCategory := r.groupByCategory(recentEntries)
	recentContent := r.buildRefinedContent(recentByCategory)
	memoryPath := filepath.Join(r.fileStore.GetMemoryDir(), "MEMORY.md")
	if err := os.WriteFile(memoryPath, []byte(recentContent), 0644); err != nil {
		return nil, fmt.Errorf("write memory: %w", err)
	}

	// 11. 计算结果
	archivedCount := 0
	for _, entries := range refinedOld {
		archivedCount += len(entries)
	}

	result.MergedEntries = len(recentEntries)
	result.RemovedEntries = len(oldEntries) - archivedCount // 去重减少的数量

	result.Changes = append(result.Changes,
		fmt.Sprintf("归档 %d 条旧记忆到 ARCHIVE.md", archivedCount),
		fmt.Sprintf("保留最近 %d 天的 %d 条条目在 MEMORY.md", recentDays, len(recentEntries)),
	)

	logger.Info("ArchiveOld completed",
		"archivedToArchive", archivedCount,
		"recentInMemory", len(recentEntries),
		"duplicatesRemoved", result.RemovedEntries)

	return result, nil
}

// mergeArchiveEntries 合并现有归档条目和新归档条目（去重）
func (r *Refiner) mergeArchiveEntries(existing, newEntries map[string][]MemoryEntry) map[string][]MemoryEntry {
	merged := make(map[string][]MemoryEntry)

	// 先添加现有归档
	for category, entries := range existing {
		merged[category] = append(merged[category], entries...)
	}

	// 添加新归档（去重）
	for category, newCats := range newEntries {
		existingCats := merged[category]
		if existingCats == nil {
			merged[category] = newCats
			continue
		}

		// 对每个新条目检查是否与现有条目重复
		for _, newEntry := range newCats {
			isDuplicate := false
			for _, existingEntry := range existingCats {
				if r.calculateSimilarity(newEntry.Content, existingEntry.Content) >= 0.9 {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				merged[category] = append(merged[category], newEntry)
			}
		}
	}

	return merged
}

// buildArchiveContent 构建归档文件内容
func (r *Refiner) buildArchiveContent(entries map[string][]MemoryEntry) string {
	var sb strings.Builder

	sb.WriteString(`# Archive

This file stores archived memories (older than the sliding window).
These memories are kept for historical reference but are not loaded into active context.

`)

	// 定义分类顺序
	categoryOrder := []string{
		"User Preferences",
		"Project Context",
		"Important Facts",
		"Decisions",
		"Contact Info",
		"Other",
	}

	// 添加已存在的分类
	for _, category := range categoryOrder {
		cats, ok := entries[category]
		if !ok || len(cats) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n", category))

		// 按时间排序（最新的在前）
		sort.Slice(cats, func(i, j int) bool {
			return cats[i].Timestamp.After(cats[j].Timestamp)
		})

		for _, entry := range cats {
			timestamp := entry.Timestamp.Format("2006-01-02 15:04")
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", timestamp, entry.Content))
		}
		sb.WriteString("\n")
	}

	// 添加其他分类
	for category, cats := range entries {
		found := false
		for _, c := range categoryOrder {
			if c == category {
				found = true
				break
			}
		}
		if found || len(cats) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n", category))
		sort.Slice(cats, func(i, j int) bool {
			return cats[i].Timestamp.After(cats[j].Timestamp)
		})
		for _, entry := range cats {
			timestamp := entry.Timestamp.Format("2006-01-02 15:04")
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", timestamp, entry.Content))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// getCategoryComment 获取分类注释
func getCategoryComment(category string) string {
	comments := map[string]string{
		"User Preferences": "用户偏好设置",
		"Project Context":  "项目上下文信息",
		"Important Facts":  "重要事实记录",
		"Decisions":        "决策记录",
		"Contact Info":     "联系方式等实体信息",
		"Other":            "其他重要信息",
	}
	if comment, ok := comments[category]; ok {
		return comment
	}
	return "自定义分类"
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
