package models

import (
	"time"
)

// Feed RSS 订阅源
type Feed struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	URL         string     `gorm:"uniqueIndex;not null" json:"url"` // 订阅源 RSS URL
	Title       string     `gorm:"not null" json:"title"`           // 订阅源标题
	IconURL     string     `json:"icon_url"`                        // 订阅源图标
	LastFetchAt *time.Time `json:"last_fetch_at"`                   // 最后抓取时间
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// UserSubscription 用户订阅关系
type UserSubscription struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"not null;uniqueIndex:idx_user_feed" json:"user_id"`
	User           User       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user"`
	FeedID         uint       `gorm:"not null;uniqueIndex:idx_user_feed" json:"feed_id"`
	Feed           Feed       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"feed"`
	Title          string     `json:"title"`                          // 用户自定义标题（可覆盖 Feed.Title）
	Category       string     `gorm:"default:'先放这儿'" json:"category"` // 分类标签
	LastReadAnchor *time.Time `json:"last_read_anchor"`               // 已读锚点时间戳
	CreatedAt      time.Time  `json:"created_at"`
}

// GetDisplayTitle 获取显示标题（优先用户自定义，否则用 Feed 原标题）
func (s *UserSubscription) GetDisplayTitle() string {
	if s.Title != "" {
		return s.Title
	}
	return s.Feed.Title
}

// FeedItem RSS 文章条目
type FeedItem struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	FeedID      uint      `gorm:"not null;index" json:"feed_id"`
	Feed        Feed      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"feed"`
	GUID        string    `gorm:"uniqueIndex;not null" json:"guid"` // RSS 唯一标识
	Title       string    `gorm:"not null" json:"title"`
	Link        string    `gorm:"not null" json:"link"`               // 原文链接
	Description string    `gorm:"type:text" json:"description"`       // 摘要
	Content     string    `gorm:"type:text" json:"content"`           // 正文 HTML (可为空，实时抓取后回填)
	PublishedAt time.Time `gorm:"not null;index" json:"published_at"` // 发布时间
	CreatedAt   time.Time `json:"created_at"`
}
