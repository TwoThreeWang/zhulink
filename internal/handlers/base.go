package handlers

import (
	"net/http"
	"strings"
	"zhulink/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Render helper to inject common variables like 'current user'
func Render(c *gin.Context, code int, name string, obj gin.H) {
	if obj == nil {
		obj = gin.H{}
	}

	// Inject Current User
	if user, exists := c.Get(middleware.CheckUserKey); exists {
		obj["CurrentUser"] = user
		if count, ok := c.Get(middleware.UnreadCountKey); ok {
			obj["UnreadCount"] = int(count.(int64))
		} else {
			obj["UnreadCount"] = 0
		}
	}

	// Inject basic helpers if needed (or do it via template funcs)
	obj["CurrentPath"] = c.Request.URL.Path

	c.HTML(code, name, obj)
}

// HTMX Redirect helper
func HtmxRedirect(c *gin.Context, path string) {
	c.Header("HX-Redirect", path)
	c.Status(http.StatusOK) // HTMX handles the redirect on client side via header
}

// Error helper
func RenderError(c *gin.Context, code int, message string) {
	// Simple error page rendering
	Render(c, code, "error.html", gin.H{"Error": message})
}

func msg(err error) string {
	if err == nil {
		return ""
	}
	// Simple way to return cleaner errors, can be expanded
	return strings.Title(err.Error())
}
