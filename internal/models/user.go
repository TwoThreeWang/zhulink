package models

import (
	"time"
)

type User struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Username      string     `gorm:"not null" json:"username"` // Username can be modified
	Email         string     `gorm:"uniqueIndex;not null" json:"email"`
	Password      string     `gorm:"not null" json:"-"`                           // Hash
	Avatar        string     `gorm:"default:🌱" json:"avatar"`                     // emoji 头像
	Bio           string     `gorm:"size:200" json:"bio"`                         // 个人简介
	Points        int        `gorm:"default:0" json:"points"`                     // 竹笋积分
	Role          string     `gorm:"size:20;default:'user';not null" json:"role"` // user, admin
	Status        int        `gorm:"default:0" json:"status"`                     // 0:正常, 1:禁言, 2:封禁
	PunishExpires *time.Time `json:"punish_expires"`                              // 惩罚到期时间
	GoogleID      string     `gorm:"index" json:"google_id"`                      // Google OAuth ID
	GoogleEmail   string     `gorm:"index" json:"google_email"`                   // Google 邮箱
	IsActivated   bool       `gorm:"default:false" json:"is_activated"`           // 是否已激活
	VerifyCode    string     `gorm:"size:20" json:"-"`                            // 验证码(激活/重置通用)
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	// No DeletedAt for hard delete
}
