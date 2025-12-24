package handlers

import (
	"net/http"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"

	"github.com/gin-gonic/gin"
)

type NotificationHandler struct{}

func NewNotificationHandler() *NotificationHandler {
	return &NotificationHandler{}
}

func (h *NotificationHandler) List(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)

	var notifications []models.Notification
	db.DB.Preload("Actor").Preload("Post").Preload("Comment").
		Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Limit(50).
		Find(&notifications)

	Render(c, http.StatusOK, "notification/list.html", gin.H{
		"Title":         "通知",
		"Notifications": notifications,
		"Active":        "notifications",
	})
}

func (h *NotificationHandler) Read(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	id := c.Param("id")

	var notification models.Notification
	if err := db.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&notification).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	notification.IsRead = true
	db.DB.Save(&notification)

	// HTMX response: Update UI to show read state
	// For simplicity, we can just return 200 OK.
	// Or we can return a small snippet if we want to change the style.
	// Let's just return OK for now, client side can add a class or remove the "unread" indicator.
	c.Status(http.StatusOK)
}

func (h *NotificationHandler) Delete(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	id := c.Param("id")

	var notification models.Notification
	if err := db.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&notification).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	db.DB.Delete(&notification)

	// HTMX: Return empty to remove the element
	c.Status(http.StatusOK)
}

func (h *NotificationHandler) ReadAll(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)

	db.DB.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", user.ID, false).
		Update("is_read", true)

	c.Redirect(http.StatusFound, "/notifications")
}
