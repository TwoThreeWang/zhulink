package models

import (
	"time"
)

type Comment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Cid       string    `gorm:"uniqueIndex;size:8;not null" json:"cid"`
	PostID    uint      `gorm:"not null;index" json:"post_id"`
	Post      Post      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"post"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	User      User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user"`
	ParentID  *uint     `gorm:"index" json:"parent_id"` // Nullable for top-level comments
	Parent    *Comment  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"parent"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Score     int       `gorm:"default:0" json:"score"`
	CreatedAt time.Time `json:"created_at"`
	// No UpdatedAt usually for comments in HN style, but good to have generally? Detailed requirement didn't specify, but I'll add if standard. User req said: CreatedAt. (No DeletedAt field). I will skip UpdatedAt to be strictly adhering to schema description unless necessary.
}
