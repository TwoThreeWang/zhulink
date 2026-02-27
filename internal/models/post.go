package models

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

type Post struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Pid            string    `gorm:"uniqueIndex;size:20;not null" json:"pid"`
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	User           User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user"`
	NodeID         uint      `gorm:"not null;index;default:1" json:"node_id"`
	Node           Node      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"node"`
	Title          string    `gorm:"not null" json:"title"`
	URL            string    `json:"url"` // Optional
	Content        string    `gorm:"type:text" json:"content"`
	Score          int       `gorm:"default:0" json:"score"`
	Views          int       `gorm:"default:0" json:"views"`           // 浏览/点击量
	SourceType     string    `json:"source_type"`                      // e.g., "rss"
	IsTop          bool      `gorm:"default:false" json:"is_top"`      // 是否置顶
	SEOKeywords    string          `gorm:"type:text" json:"seo_keywords"`    // AI 生成的 SEO 关键词
	SEODescription string          `gorm:"type:text" json:"seo_description"` // AI 生成的 SEO 页面描述
	VectorText     string          `gorm:"type:text" json:"-"`               // 用于生成向量的拼接文本
	Embedding      *pgvector.Vector `gorm:"type:vector(768)" json:"-"`         // 向量数据 (nomic-embed-text 为 768 维)
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// 非数据库字段，用于查询时填充
	CommentCount int `gorm:"-" json:"comment_count"`
}
