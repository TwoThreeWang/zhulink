package handlers

import (
	"net/http"
	"strings"
	"zhulink/internal/db"
	"zhulink/internal/models"
	"zhulink/internal/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"zhulink/internal/services"
)

type AuthHandler struct {
	mailService    *services.MailService
	captchaService *services.CaptchaService
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		mailService:    services.NewMailService(),
		captchaService: services.NewCaptchaService(),
	}
}

func (h *AuthHandler) ShowRegister(c *gin.Context) {
	question, answer := h.captchaService.GenerateMathProblem()
	session := sessions.Default(c)
	session.Set("captcha_answer", answer)
	session.Save()
	Render(c, http.StatusOK, "auth/register.html", gin.H{"Captcha": question})
}

// createUser 创建新用户的通用函数
func (h *AuthHandler) createUser(username, email, password string) (*models.User, error) {
	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := models.User{
		Username: username,
		Email:    email,
		Password: hash,
		Avatar:   utils.GetRandomEmoji(), // 随机 emoji 头像
		Points:   0,                      // 默认 0 竹笋
	}

	if err := db.DB.Create(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (h *AuthHandler) Register(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")
	captchaInput := c.PostForm("captcha")

	// Validate Captcha
	session := sessions.Default(c)
	expectedAnswer, ok := session.Get("captcha_answer").(int)
	if !ok || utils.StringToInt(captchaInput) != expectedAnswer {
		question, answer := h.captchaService.GenerateMathProblem()
		session.Set("captcha_answer", answer)
		session.Save()
		Render(c, http.StatusBadRequest, "auth/register.html", gin.H{"Error": "验证码错误", "Captcha": question})
		return
	}
	// Clear captcha after use
	session.Delete("captcha_answer")
	session.Save()

	// Extract username from email
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		question, answer := h.captchaService.GenerateMathProblem()
		session.Set("captcha_answer", answer)
		session.Save()
		Render(c, http.StatusBadRequest, "auth/register.html", gin.H{"Error": "邮箱格式不正确", "Captcha": question})
		return
	}
	username := parts[0]

	if len(password) < 6 {
		question, answer := h.captchaService.GenerateMathProblem()
		session.Set("captcha_answer", answer)
		session.Save()
		Render(c, http.StatusBadRequest, "auth/register.html", gin.H{"Error": "密码至少6位", "Captcha": question})
		return
	}

	user, err := h.createUser(username, email, password)
	if err != nil {
		question, answer := h.captchaService.GenerateMathProblem()
		session.Set("captcha_answer", answer)
		session.Save()
		Render(c, http.StatusConflict, "auth/register.html", gin.H{"Error": "邮箱已注册", "Captcha": question})
		return
	}

	// Send Activation Email
	code := utils.GenerateRandomCode(6)
	user.VerifyCode = code
	db.DB.Save(user)
	h.mailService.SendWelcomeEmail(email, code)

	Render(c, http.StatusOK, "auth/login.html", gin.H{"Success": "注册成功！激活码已发送至您的邮箱，请登录后激活。"})
}

func (h *AuthHandler) ShowActivate(c *gin.Context) {
	Render(c, http.StatusOK, "auth/activate.html", nil)
}

func (h *AuthHandler) Activate(c *gin.Context) {
	code := c.PostForm("code")
	// email := c.PostForm("email") // Optional if we require login first, but simpler to ask email + code?
	// User said: "Login... if !IsActive...". So maybe we require login first?
	// OR prompt email + code. Let's assume user is logged in if they click link? No, user enters code manually.
	// Best UX: Show simple form "Email + Code".
	email := c.PostForm("email")

	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		Render(c, http.StatusBadRequest, "auth/activate.html", gin.H{"Error": "用户不存在"})
		return
	}

	if user.IsActivated {
		Render(c, http.StatusOK, "auth/login.html", gin.H{"Success": "账号已激活，请登录"})
		return
	}

	if user.VerifyCode != code {
		Render(c, http.StatusBadRequest, "auth/activate.html", gin.H{"Error": "激活码错误"})
		return
	}

	user.IsActivated = true
	user.VerifyCode = "" // 清除验证码
	db.DB.Save(&user)

	// 激活成功后自动登录
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
	// 检查用户是否被封禁
	if user.Status == 2 {
		Render(c, http.StatusForbidden, "auth/login.html", gin.H{"Error": "您的账号已被封禁,无法登录。"})
		return
	}

	// 检查未激活
	if !user.IsActivated {
		Render(c, http.StatusUnauthorized, "auth/activate.html", gin.H{"Error": "账号未激活，请输入激活码", "Email": email})
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

func (h *AuthHandler) ShowForgotPassword(c *gin.Context) {
	question, answer := h.captchaService.GenerateMathProblem()
	session := sessions.Default(c)
	session.Set("reset_captcha_answer", answer)
	session.Save()
	Render(c, http.StatusOK, "auth/forgot_password.html", gin.H{"Captcha": question})
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	email := c.PostForm("email")
	captchaInput := c.PostForm("captcha")

	session := sessions.Default(c)
	expectedAnswer, ok := session.Get("reset_captcha_answer").(int)
	if !ok || utils.StringToInt(captchaInput) != expectedAnswer {
		question, answer := h.captchaService.GenerateMathProblem()
		session.Set("reset_captcha_answer", answer)
		session.Save()
		Render(c, http.StatusBadRequest, "auth/forgot_password.html", gin.H{"Error": "验证码错误", "Captcha": question})
		return
	}
	session.Delete("reset_captcha_answer")
	session.Save()

	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// Don't reveal user existence? Or just say sent.
		// For simplicity/UX: "If account exists, email sent."
		// But User wants "Password retrieval... verify code".
		// I'll update ResetCode and send.
		Render(c, http.StatusOK, "auth/reset_password.html", gin.H{"Success": "如果邮箱存在，验证码已发送。请查收并重置。", "Email": email})
		return
	}

	code := utils.GenerateRandomCode(6)
	user.VerifyCode = code
	db.DB.Save(&user)
	h.mailService.SendPasswordResetEmail(email, code)

	Render(c, http.StatusOK, "auth/reset_password.html", gin.H{"Email": email})
}

func (h *AuthHandler) ShowResetPassword(c *gin.Context) {
	email := c.Query("email")
	Render(c, http.StatusOK, "auth/reset_password.html", gin.H{"Email": email})
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	email := c.PostForm("email")
	code := c.PostForm("code")
	newPassword := c.PostForm("password")

	var user models.User
	if err := db.DB.Where("email = ?", email).First(&user).Error; err != nil {
		Render(c, http.StatusBadRequest, "auth/reset_password.html", gin.H{"Error": "用户不存在", "Email": email})
		return
	}

	if user.VerifyCode == "" || user.VerifyCode != code {
		Render(c, http.StatusBadRequest, "auth/reset_password.html", gin.H{"Error": "验证码错误或已失效", "Email": email})
		return
	}

	hash, _ := utils.HashPassword(newPassword)
	user.Password = hash
	user.VerifyCode = "" // Clear code
	db.DB.Save(&user)

	Render(c, http.StatusOK, "auth/login.html", gin.H{"Success": "密码重置成功，请登录"})
}

// RefreshCaptcha 刷新验证码 (AJAX)
func (h *AuthHandler) RefreshCaptcha(c *gin.Context) {
	captchaType := c.Query("type") // "register" or "reset"
	question, answer := h.captchaService.GenerateMathProblem()

	session := sessions.Default(c)
	if captchaType == "reset" {
		session.Set("reset_captcha_answer", answer)
	} else {
		session.Set("captcha_answer", answer)
	}
	session.Save()

	c.JSON(http.StatusOK, gin.H{"captcha": question})
}
