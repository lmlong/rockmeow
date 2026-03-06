// Package memory 用户档案管理
package memory

import "time"

// UserProfile 用户档案
type UserProfile struct {
	UserID         string    `json:"userId"`         // 用户唯一标识（渠道提供的 ID）
	Channel        string    `json:"channel"`        // 渠道名称（feishu, qq 等）
	FirstSeenAt    time.Time `json:"firstSeenAt"`    // 首次交互时间
	SoulDefined    bool      `json:"soulDefined"`    // 是否已定义 Soul
	SoulDefinition string    `json:"soulDefinition"` // Soul 定义内容
	SoulDefinedAt  time.Time `json:"soulDefinedAt,omitempty"` // Soul 定义时间
	UpdatedAt      time.Time `json:"updatedAt"`      // 最后更新时间
}