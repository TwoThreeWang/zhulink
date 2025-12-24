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

// Custom hook or unique index enforcement can be done via GORM tags or DB approach.
// To ensure one vote per item per user:
// gorm:"uniqueIndex:idx_post_vote;uniqueIndex:idx_comment_vote" is tricky with nulls in some DBs, but PG handles (user_id, post_id) unique where post_id is not null.
