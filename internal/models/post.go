package models

import (
	"time"
)

type Post struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Pid        string    `gorm:"uniqueIndex;size:8;not null" json:"pid"`
	UserID     uint      `gorm:"not null;index" json:"user_id"`
	User       User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user"`
	NodeID     uint      `gorm:"not null;index;default:1" json:"node_id"`
	Node       Node      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"node"`
	Title      string    `gorm:"not null" json:"title"`
	URL        string    `json:"url"` // Optional
	Content    string    `gorm:"type:text" json:"content"`
	Score      int       `gorm:"default:0" json:"score"`
	Views      int       `gorm:"default:0" json:"views"` // 浏览/点击量
	SourceType string    `json:"source_type"`            // e.g., "rss"
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// 非数据库字段，用于查询时填充
	CommentCount int `gorm:"-" json:"comment_count"`
}
