package models

import (
	"time"
)

// Bookmark 收藏模型 - 用户收藏文章
type Bookmark struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index;uniqueIndex:idx_user_post" json:"user_id"`
	PostID    uint      `gorm:"not null;index;uniqueIndex:idx_user_post" json:"post_id"`
	Post      Post      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"post"`
	CreatedAt time.Time `json:"created_at"`
}
