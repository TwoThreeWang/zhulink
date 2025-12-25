package handlers

import (
	"net/http"
	"strings"
	"zhulink/internal/db"
	"zhulink/internal/models"
	"zhulink/internal/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

func (h *AuthHandler) ShowRegister(c *gin.Context) {
	Render(c, http.StatusOK, "auth/register.html", nil)
}

func (h *AuthHandler) Register(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	// Extract username from email
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		Render(c, http.StatusBadRequest, "auth/register.html", gin.H{"Error": "邮箱格式不正确"})
		return
	}
	username := parts[0]

	if len(password) < 6 {
		Render(c, http.StatusBadRequest, "auth/register.html", gin.H{"Error": "密码至少6位"})
		return
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		Render(c, http.StatusInternalServerError, "auth/register.html", gin.H{"Error": "系统错误"})
		return
	}

	user := models.User{
		Username: username,
		Email:    email,
		Password: hash,
		Avatar:   utils.GetRandomEmoji(), // 随机 emoji 头像
		Points:   0,                      // 默认 0 竹笋
	}

	if err := db.DB.Create(&user).Error; err != nil {
		Render(c, http.StatusConflict, "auth/register.html", gin.H{"Error": "邮箱已注册"})
		return
	}

	// Auto login
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Save()

	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHandler) ShowLogin(c *gin.Context) {
	Render(c, http.StatusOK, "auth/login.html", nil)
}

func (h *AuthHandler) Login(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		Render(c, http.StatusUnauthorized, "auth/login.html", gin.H{"Error": "邮箱或密码错误"})
		return
	}

	if !utils.CheckPasswordHash(password, user.Password) {
		Render(c, http.StatusUnauthorized, "auth/login.html", gin.H{"Error": "邮箱或密码错误"})
		return
	}

	// 检查用户是否被封禁
	if user.Status == 2 {
		Render(c, http.StatusForbidden, "auth/login.html", gin.H{"Error": "您的账号已被封禁,无法登录。"})
		return
	}

	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Save()

	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/")
}
