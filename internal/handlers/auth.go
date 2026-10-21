package handlers

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
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

const sessionAbsoluteLifetime = 180 * 24 * time.Hour

func startUserSession(c *gin.Context, userID uint) error {
	now := time.Now()
	session := sessions.Default(c)
	session.Clear()
	session.Set(middleware.SessionUserIDKey, userID)
	session.Set(middleware.SessionIssuedKey, now.Unix())
	session.Set(middleware.SessionRenewedKey, now.Unix())
	session.Set(middleware.SessionExpiresKey, now.Add(sessionAbsoluteLifetime).Unix())
	session.Options(middleware.SessionCookieOptions(middleware.SessionIdleMaxAge))
	return session.Save()
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
// password 为空表示没有密码（Google 注册），库里存空串：bcrypt 校验空串永远失败，无法用密码登录
func (h *AuthHandler) createUser(username, email, password string) (*models.User, error) {
	hash := ""
	if password != "" {
		var err error
		if hash, err = utils.HashPassword(password); err != nil {
			return nil, err
		}
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

const (
	activationCodeTTL = 24 * time.Hour
	resetCodeTTL      = 15 * time.Minute
	maxVerifyAttempts = 5
	// 未激活超过这个时长的账号可被同邮箱重新注册覆盖，防止邮箱被别人抢注占用
	unactivatedReclaimAfter = time.Hour
)

// 全站每小时最多发 200 封激活邮件：被批量注册时保护发信域名的信誉
var registerMailLimiter = middleware.NewLimiter(200, time.Hour)

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// findUserByEmail 按邮箱查用户，不区分大小写（兼容历史上按原样存下的大写邮箱）
func findUserByEmail(email string, user *models.User) error {
	return db.DB.Where("LOWER(email) = ?", normalizeEmail(email)).First(user).Error
}

// issueVerifyCode 生成新验证码，同时重置过期时间和校验次数
func issueVerifyCode(user *models.User, ttl time.Duration) string {
	code := utils.GenerateRandomCode(6)
	expires := time.Now().Add(ttl)
	user.VerifyCode, user.VerifyCodeExpiresAt, user.VerifyAttempts = code, &expires, 0
	db.DB.Model(user).Updates(map[string]interface{}{"verify_code": code, "verify_code_expires_at": expires, "verify_attempts": 0})
	return code
}

func clearVerifyCode(user *models.User) {
	user.VerifyCode, user.VerifyCodeExpiresAt, user.VerifyAttempts = "", nil, 0
}

// checkVerifyCode 校验邮件验证码：空码、过期、校验超过 maxVerifyAttempts 次都不通过。
// 次数在数据库里原子自增，并发请求也没法多猜
func checkVerifyCode(user *models.User, code string) bool {
	if user.VerifyCode == "" || user.VerifyCodeExpiresAt == nil || time.Now().After(*user.VerifyCodeExpiresAt) {
		return false
	}
	var attempts int
	if err := db.DB.Raw("UPDATE users SET verify_attempts = verify_attempts + 1 WHERE id = ? RETURNING verify_attempts", user.ID).Scan(&attempts).Error; err != nil {
		return false
	}
	if attempts > maxVerifyAttempts {
		db.DB.Model(user).Updates(map[string]interface{}{"verify_code": "", "verify_code_expires_at": nil})
		return false
	}
	return subtle.ConstantTimeCompare([]byte(user.VerifyCode), []byte(code)) == 1
}

func (h *AuthHandler) Register(c *gin.Context) {
	email := normalizeEmail(c.PostForm("email"))
	password := c.PostForm("password")
	captchaInput := c.PostForm("captcha")

	session := sessions.Default(c)
	// 出错时换一道新验证码重新渲染注册页
	fail := func(status int, msg string) {
		question, answer := h.captchaService.GenerateMathProblem()
		session.Set("captcha_answer", answer)
		session.Save()
		Render(c, status, "auth/register.html", gin.H{"Error": msg, "Captcha": question})
	}

	// Validate Captcha
	expectedAnswer, ok := session.Get("captcha_answer").(int)
	if !ok || utils.StringToInt(captchaInput) != expectedAnswer {
		fail(http.StatusBadRequest, "验证码错误")
		return
	}
	// Clear captcha after use
	session.Delete("captcha_answer")
	session.Save()

	// Extract username from email
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		fail(http.StatusBadRequest, "邮箱格式不正确")
		return
	}
	username := parts[0]

	if len(password) < 6 {
		fail(http.StatusBadRequest, "密码至少6位")
		return
	}

	// 已激活、或刚注册不到 1 小时的邮箱不能重复注册；
	// 未激活超过 1 小时的可能是别人抢注，也可能是本人放弃了，允许重新注册覆盖
	var existing models.User
	found := findUserByEmail(email, &existing) == nil
	if found && (existing.IsActivated || time.Since(existing.CreatedAt) < unactivatedReclaimAfter) {
		fail(http.StatusConflict, "邮箱已注册")
		return
	}

	if !registerMailLimiter.Allow("all") {
		log.Printf("[Register] 激活邮件已达每小时上限，暂停注册 (email=%s)", email)
		fail(http.StatusServiceUnavailable, "当前注册人数过多，请稍后再试")
		return
	}

	var user *models.User
	if found {
		hash, err := utils.HashPassword(password)
		if err != nil {
			fail(http.StatusInternalServerError, "注册失败，请稍后重试")
			return
		}
		existing.Email = email
		existing.Password = hash
		existing.CreatedAt = time.Now() // 重新计时，避免刚覆盖完又被别人覆盖
		db.DB.Save(&existing)
		user = &existing
	} else {
		var err error
		if user, err = h.createUser(username, email, password); err != nil {
			fail(http.StatusConflict, "邮箱已注册")
			return
		}
	}

	// Send Activation Email
	code := issueVerifyCode(user, activationCodeTTL)
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
	if err := findUserByEmail(email, &user); err != nil {
		Render(c, http.StatusBadRequest, "auth/activate.html", gin.H{"Error": "用户不存在"})
		return
	}

	if user.IsActivated {
		Render(c, http.StatusOK, "auth/login.html", gin.H{"Success": "账号已激活，请登录"})
		return
	}

	if !checkVerifyCode(&user, code) {
		Render(c, http.StatusBadRequest, "auth/activate.html", gin.H{"Error": "激活码错误或已失效，可通过「忘记密码」重新获取验证码完成激活"})
		return
	}

	user.IsActivated = true
	clearVerifyCode(&user)
	db.DB.Save(&user)

	// 激活成功后自动登录
	if err := startUserSession(c, user.ID); err != nil {
		Render(c, http.StatusInternalServerError, "auth/login.html", gin.H{"Error": "登录状态保存失败，请重试"})
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHandler) ShowLogin(c *gin.Context) {
	Render(c, http.StatusOK, "auth/login.html", nil)
}

func (h *AuthHandler) Login(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	var user models.User
	if err := findUserByEmail(email, &user); err != nil {
		Render(c, http.StatusUnauthorized, "auth/login.html", gin.H{"Error": "邮箱或密码错误"})
		return
	}

	if !utils.CheckPasswordHash(password, user.Password) {
		Render(c, http.StatusUnauthorized, "auth/login.html", gin.H{"Error": "邮箱或密码错误"})
		return
	}

	// 检查用户是否被封禁（到期的惩罚先解除）
	middleware.ClearExpiredPunishment(&user)
	if user.Status == 2 {
		Render(c, http.StatusForbidden, "auth/login.html", gin.H{"Error": "您的账号已被封禁,无法登录。"})
		return
	}

	// 检查未激活
	if !user.IsActivated {
		Render(c, http.StatusUnauthorized, "auth/activate.html", gin.H{"Error": "账号未激活，请输入激活码", "Email": email})
		return
	}

	if err := startUserSession(c, user.ID); err != nil {
		Render(c, http.StatusInternalServerError, "auth/login.html", gin.H{"Error": "登录状态保存失败，请重试"})
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	if err := middleware.ClearSession(session); err != nil {
		Render(c, http.StatusInternalServerError, "auth/login.html", gin.H{"Error": "退出登录失败，请重试"})
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHandler) ShowForgotPassword(c *gin.Context) {
	question, answer := h.captchaService.GenerateMathProblem()
	session := sessions.Default(c)
	session.Set("reset_captcha_answer", answer)
	session.Save()
	Render(c, http.StatusOK, "auth/forgot_password.html", gin.H{"Captcha": question})
}

// 同一邮箱每分钟 1 封、每天 5 封重置邮件
var (
	resetMailMinuteLimiter = middleware.NewLimiter(1, time.Minute)
	resetMailDayLimiter    = middleware.NewLimiter(5, 24*time.Hour)
)

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

	// 按邮箱冷却，防止换 IP 轰炸同一个邮箱；在查库之前判断，不暴露邮箱是否存在
	emailKey := normalizeEmail(email)
	if !resetMailMinuteLimiter.Allow(emailKey) || !resetMailDayLimiter.Allow(emailKey) {
		question, answer := h.captchaService.GenerateMathProblem()
		session.Set("reset_captcha_answer", answer)
		session.Save()
		Render(c, http.StatusTooManyRequests, "auth/forgot_password.html", gin.H{"Error": "该邮箱请求过于频繁，请稍后再试", "Captcha": question})
		return
	}
	session.Save()

	var user models.User
	if err := findUserByEmail(email, &user); err != nil {
		// Don't reveal user existence? Or just say sent.
		// For simplicity/UX: "If account exists, email sent."
		// But User wants "Password retrieval... verify code".
		// I'll update ResetCode and send.
		Render(c, http.StatusOK, "auth/reset_password.html", gin.H{"Success": "如果邮箱存在，验证码已发送。请查收并重置。", "Email": email})
		return
	}

	code := issueVerifyCode(&user, resetCodeTTL)
	h.mailService.SendPasswordResetEmail(user.Email, code)

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
	if err := findUserByEmail(email, &user); err != nil {
		Render(c, http.StatusBadRequest, "auth/reset_password.html", gin.H{"Error": "用户不存在", "Email": email})
		return
	}

	// 先查密码长度，输错密码不消耗验证码的校验次数
	if len(newPassword) < 6 {
		Render(c, http.StatusBadRequest, "auth/reset_password.html", gin.H{"Error": "密码至少6位", "Email": email})
		return
	}

	if !checkVerifyCode(&user, code) {
		Render(c, http.StatusBadRequest, "auth/reset_password.html", gin.H{"Error": "验证码错误或已失效，输错 5 次或超过 15 分钟需重新获取", "Email": email})
		return
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		Render(c, http.StatusInternalServerError, "auth/reset_password.html", gin.H{"Error": "重置失败，请稍后重试", "Email": email})
		return
	}
	user.Password = hash
	user.IsActivated = true // 能收到验证码就证明了邮箱归属，未激活账号顺便激活
	clearVerifyCode(&user)
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
