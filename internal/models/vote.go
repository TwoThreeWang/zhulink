package models

import (
	"time"
)

type Vote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	PostID    *uint     `gorm:"index" json:"post_id"`
	CommentID *uint     `gorm:"index" json:"comment_id"`
	Value     int       `gorm:"not null" json:"value"` // 1 or -1
	CreatedAt time.Time `json:"created_at"`
}

// 每人对同一帖子/评论只能投一票：唯一索引 idx_vote_user_post / idx_vote_user_comment 在 db.migrateUniqueConstraints 中创建
