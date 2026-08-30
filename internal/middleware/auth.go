package middleware

import (
	"errors"
	"log"
	"net/http"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	CheckUserKey      = "user"
	UnreadCountKey    = "unread_count"
	SessionUserIDKey  = "user_id"
	SessionIssuedKey  = "issued_at"
	SessionRenewedKey = "renewed_at"
	SessionExpiresKey = "absolute_expires_at"

	SessionIdleMaxAge = int((30 * 24 * time.Hour) / time.Second)
)

const sessionRenewalInterval = 24 * time.Hour

func SessionCookieOptions(maxAge int) sessions.Options {
	return sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
		SameSite: http.SameSiteLaxMode,
	}
}

func ClearSession(session sessions.Session) error {
	session.Clear()
	session.Options(SessionCookieOptions(-1))
	return session.Save()
}

func redirectToLogin(c *gin.Context) {
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", "/login")
		c.Status(http.StatusOK)
	} else {
		c.Redirect(http.StatusFound, "/login")
	}
	c.Abort()
}

// AuthRequired ensures a user is logged in
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get(CheckUserKey); !exists {
			redirectToLogin(c)
			return
		}
		c.Next()
	}
}

func validSessionTimes(now time.Time, issuedAt, renewedAt, absoluteExpiresAt int64) bool {
	nowUnix := now.Unix()
	if issuedAt <= 0 || renewedAt < issuedAt || absoluteExpiresAt <= issuedAt {
		return false
	}
	if issuedAt > nowUnix || renewedAt > nowUnix || nowUnix >= absoluteExpiresAt {
		return false
	}
	return now.Sub(time.Unix(renewedAt, 0)) < time.Duration(SessionIdleMaxAge)*time.Second
}

func renewalMaxAge(now time.Time, renewedAt, absoluteExpiresAt int64) (int, bool) {
	maxAge := SessionIdleMaxAge
	remaining := int(time.Unix(absoluteExpiresAt, 0).Sub(now).Seconds())
	if remaining < maxAge {
		maxAge = remaining
	}
	if now.Sub(time.Unix(renewedAt, 0)) < sessionRenewalInterval {
		return maxAge, false
	}
	return maxAge, maxAge > 0
}

// LoadUser retrieves user from session and sets to context
func LoadUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get(SessionUserIDKey)
		if userID == nil {
			c.Next()
			return
		}

		issuedAt, issuedOK := session.Get(SessionIssuedKey).(int64)
		renewedAt, renewedOK := session.Get(SessionRenewedKey).(int64)
		absoluteExpiresAt, expiresOK := session.Get(SessionExpiresKey).(int64)
		now := time.Now()
		if !issuedOK || !renewedOK || !expiresOK || !validSessionTimes(now, issuedAt, renewedAt, absoluteExpiresAt) {
			if err := ClearSession(session); err != nil {
				log.Printf("clear invalid session failed: %v", err)
			}
			redirectToLogin(c)
			return
		}

		var user models.User
		result := db.DB.First(&user, userID)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := ClearSession(session); err != nil {
					log.Printf("clear missing-user session failed: %v", err)
				}
				redirectToLogin(c)
				return
			}
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		if user.Status == 2 || !user.IsActivated {
			if err := ClearSession(session); err != nil {
				log.Printf("clear disabled-user session failed: %v", err)
			}
			redirectToLogin(c)
			return
		}

		maxAge, renew := renewalMaxAge(now, renewedAt, absoluteExpiresAt)
		session.Options(SessionCookieOptions(maxAge))
		if renew {
			session.Set(SessionRenewedKey, now.Unix())
			if err := session.Save(); err != nil {
				log.Printf("renew session failed: %v", err)
			}
		}

		c.Set(CheckUserKey, &user)

		// Fetch Unread Notification Count
		var count int64
		db.DB.Model(&models.Notification{}).Where("user_id = ? AND is_read = ?", user.ID, false).Count(&count)
		c.Set(UnreadCountKey, count)
		c.Next()
	}
}
