package middleware

import (
	"net/http"
	"zhulink/internal/db"
	"zhulink/internal/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const CheckUserKey = "user"
const UnreadCountKey = "unread_count"

// AuthRequired ensures a user is logged in
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		if userID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Optionally verify if user still exists here, or rely on LoadUser
		if _, exists := c.Get(CheckUserKey); !exists {
			// If LoadUser didn't run or fail, try to load again or redirect
			// For now, assume LoadUser runs before this.
		}

		c.Next()
	}
}

// LoadUser retrieves user from session and sets to context
func LoadUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")

		if userID != nil {
			var user models.User
			result := db.DB.First(&user, userID)
			if result.Error == nil {
				c.Set(CheckUserKey, &user)

				// Fetch Unread Notification Count
				var count int64
				db.DB.Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", user.ID, false).Count(&count)
				c.Set(UnreadCountKey, count)
			}
		}
		c.Next()
	}
}
