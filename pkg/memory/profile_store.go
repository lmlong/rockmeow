// Package memory 用户档案存储管理
package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lingguard/pkg/logger"
)

// ProfileStore 用户档案存储
type ProfileStore struct {
	mu       sync.RWMutex
	profiles map[string]*UserProfile // 内存缓存
	basePath string                  // 存储根目录
}

// NewProfileStore 创建用户档案存储
func NewProfileStore(basePath string) *ProfileStore {
	if basePath == "" {
		home, _ := os.UserHomeDir()
		basePath = filepath.Join(home, ".lingguard", "memory", "profiles")
	}

	store := &ProfileStore{
		profiles: make(map[string]*UserProfile),
		basePath: basePath,
	}

	// 确保目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		logger.Warn("Failed to create profiles directory", "path", basePath, "error", err)
	}

	return store
}

// GetProfile 获取用户档案
func (s *ProfileStore) GetProfile(userID string) (*UserProfile, error) {
	// 先从内存缓存中查找
	s.mu.RLock()
	if profile, ok := s.profiles[userID]; ok {
		s.mu.RUnlock()
		return profile, nil
	}
	s.mu.RUnlock()

	// 从文件加载
	profile, err := s.loadProfile(userID)
	if err != nil {
		return nil, err
	}

	// 缓存到内存
	if profile != nil {
		s.mu.Lock()
		s.profiles[userID] = profile
		s.mu.Unlock()
	}

	return profile, nil
}

// SaveProfile 保存用户档案
func (s *ProfileStore) SaveProfile(profile *UserProfile) error {
	if profile == nil {
		return nil
	}

	// 更新时间戳
	profile.UpdatedAt = time.Now()

	// 保存到文件
	if err := s.saveProfileToFile(profile); err != nil {
		return err
	}

	// 更新内存缓存
	s.mu.Lock()
	s.profiles[profile.UserID] = profile
	s.mu.Unlock()

	return nil
}

// IsFirstInteraction 检查是否首次交互
func (s *ProfileStore) IsFirstInteraction(userID string) bool {
	profile, err := s.GetProfile(userID)
	if err != nil || profile == nil {
		return true
	}
	return false
}

// CreateProfile 创建新用户档案（首次交互时调用）
func (s *ProfileStore) CreateProfile(userID, channel string) (*UserProfile, error) {
	now := time.Now()
	profile := &UserProfile{
		UserID:      userID,
		Channel:     channel,
		FirstSeenAt: now,
		UpdatedAt:   now,
	}

	if err := s.SaveProfile(profile); err != nil {
		return nil, err
	}

	logger.Info("User profile created", "userId", userID, "channel", channel)
	return profile, nil
}

// MarkSoulDefined 标记 Soul 已定义
func (s *ProfileStore) MarkSoulDefined(userID, soulDefinition string) error {
	profile, err := s.GetProfile(userID)
	if err != nil {
		return err
	}

	if profile == nil {
		return nil // 用户档案不存在，忽略
	}

	now := time.Now()
	profile.SoulDefined = true
	profile.SoulDefinition = soulDefinition
	profile.SoulDefinedAt = now
	profile.UpdatedAt = now

	if err := s.SaveProfile(profile); err != nil {
		return err
	}

	logger.Info("Soul defined for user", "userId", userID)
	return nil
}

// UpdateSoulDefinition 更新 Soul 定义（用户重新定义时调用）
func (s *ProfileStore) UpdateSoulDefinition(userID, soulDefinition string) error {
	return s.MarkSoulDefined(userID, soulDefinition)
}

// IsSoulDefined 检查用户是否已定义 Soul
func (s *ProfileStore) IsSoulDefined(userID string) bool {
	profile, err := s.GetProfile(userID)
	if err != nil || profile == nil {
		return false
	}
	return profile.SoulDefined
}

// GetSoulDefinition 获取用户的 Soul 定义
func (s *ProfileStore) GetSoulDefinition(userID string) string {
	profile, err := s.GetProfile(userID)
	if err != nil || profile == nil {
		return ""
	}
	return profile.SoulDefinition
}

// loadProfile 从文件加载用户档案
func (s *ProfileStore) loadProfile(userID string) (*UserProfile, error) {
	filePath := s.getProfilePath(userID)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 文件不存在，返回 nil 表示未找到
		}
		return nil, err
	}

	var profile UserProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// saveProfileToFile 保存用户档案到文件
func (s *ProfileStore) saveProfileToFile(profile *UserProfile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	filePath := s.getProfilePath(profile.UserID)
	return os.WriteFile(filePath, data, 0644)
}

// getProfilePath 获取用户档案文件路径
func (s *ProfileStore) getProfilePath(userID string) string {
	// 使用 userID 的安全版本作为文件名（避免特殊字符）
	safeID := sanitizeFileName(userID)
	return filepath.Join(s.basePath, safeID+".json")
}

// sanitizeFileName 清理文件名中的特殊字符
func sanitizeFileName(name string) string {
	// 简单处理：替换可能危险的字符
	result := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			result += string(c)
		} else {
			result += "_"
		}
	}
	return result
}