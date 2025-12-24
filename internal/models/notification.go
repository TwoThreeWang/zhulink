package models

import (
	"time"
)

type NotificationType string

const (
	NotificationTypeCommentPost  NotificationType = "comment_post"
	NotificationTypeReplyComment NotificationType = "reply_comment"
	NotificationTypeSystem       NotificationType = "system"
	NotificationTypeReport       NotificationType = "report" // 举报通知
)

type Notification struct {
	ID        uint             `gorm:"primaryKey" json:"id"`
	UserID    uint             `gorm:"not null;index" json:"user_id"` // Receiver
	User      User             `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user"`
	ActorID   *uint            `gorm:"index" json:"actor_id"` // Sender
	Actor     User             `gorm:"foreignKey:ActorID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"actor"`
	Type      NotificationType `gorm:"type:varchar(20);not null" json:"type"`
	Reason    string           `gorm:"type:text" json:"reason"` // 通知详细内容 (支持 HTML)
	IsRead    bool             `gorm:"default:false;index" json:"is_read"`
	CreatedAt time.Time        `json:"created_at"`
}
