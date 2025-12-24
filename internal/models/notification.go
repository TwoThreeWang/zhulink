package models

import (
	"time"
)

type NotificationType string

const (
	NotificationTypeCommentPost  NotificationType = "comment_post"
	NotificationTypeReplyComment NotificationType = "reply_comment"
)

type Notification struct {
	ID        uint             `gorm:"primaryKey" json:"id"`
	UserID    uint             `gorm:"not null;index" json:"user_id"` // Receiver
	User      User             `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user"`
	ActorID   uint             `gorm:"not null;index" json:"actor_id"` // Sender
	Actor     User             `gorm:"foreignKey:ActorID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"actor"`
	Type      NotificationType `gorm:"type:varchar(20);not null" json:"type"`
	PostID    uint             `gorm:"not null;index" json:"post_id"`
	Post      Post             `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"post"`
	CommentID *uint            `gorm:"index" json:"comment_id"` // Optional, for replies
	Comment   *Comment         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"comment"`
	IsRead    bool             `gorm:"default:false;index" json:"is_read"`
	CreatedAt time.Time        `json:"created_at"`
}
