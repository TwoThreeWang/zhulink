package models

import (
	"time"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"not null" json:"username"` // Username can be modified
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`       // Hash
	Avatar    string    `gorm:"default:🌱" json:"avatar"` // emoji 头像
	Bio       string    `gorm:"size:200" json:"bio"`     // 个人简介
	Points    int       `gorm:"default:0" json:"points"` // 竹笋积分
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// No DeletedAt for hard delete
}
