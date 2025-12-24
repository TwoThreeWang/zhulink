package models

import (
	"time"
)

type Report struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"` // Reporter
	User      User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user"`
	ItemType  string    `gorm:"size:20;not null" json:"item_type"` // "post", "comment"
	ItemID    uint      `gorm:"not null;index" json:"item_id"`
	ItemPid   string    `gorm:"size:8" json:"item_pid"`
	Reason    string    `gorm:"size:200;not null" json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
