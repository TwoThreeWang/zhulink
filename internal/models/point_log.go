package models

import (
	"time"
)

type PointLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	User      User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user"`
	Amount    int       `gorm:"not null" json:"amount"`          // 正数为增加，负数为扣除
	Action    string    `gorm:"size:100;not null" json:"action"` // 动作描述
	CreatedAt time.Time `json:"created_at"`
}
