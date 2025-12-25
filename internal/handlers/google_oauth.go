package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"zhulink/internal/db"
	"zhulink/internal/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var googleOauthConfig *oauth2.Config

// InitGoogleOAuth 初始化 Google OAuth 配置
func InitGoogleOAuth() {
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:8080"
	}

	googleOauthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  siteURL + "/auth/google/callback",
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
}

// GoogleUserInfo Google 用户信息结构
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

// generateStateToken 生成随机 state token
func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GoogleLogin 发起 Google OAuth 登录
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	state, err := generateStateToken()
	if err != nil {
		c.String(http.StatusInternalServerError, "生成状态令牌失败")
		return
	}

	// 将 state 存储到 session 中,用于验证回调
	session := sessions.Default(c)
	session.Set("oauth_state", state)
	session.Save()

	url := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback 处理 Google OAuth 回调
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	session := sessions.Default(c)
	savedState := session.Get("oauth_state")

	// 验证 state 参数
	if savedState == nil || c.Query("state") != savedState.(string) {
		Render(c, http.StatusBadRequest, "auth/login.html", gin.H{"Error": "无效的状态参数"})
		return
	}

	// 清除 state
	session.Delete("oauth_state")
	session.Save()

	// 获取授权码
	code := c.Query("code")
	if code == "" {
		Render(c, http.StatusBadRequest, "auth/login.html", gin.H{"Error": "未获取到授权码"})
		return
	}

	// 交换 token
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		Render(c, http.StatusInternalServerError, "auth/login.html", gin.H{"Error": "获取访问令牌失败"})
		return
	}

	// 获取用户信息
	userInfo, err := h.getGoogleUserInfo(token.AccessToken)
	if err != nil {
		Render(c, http.StatusInternalServerError, "auth/login.html", gin.H{"Error": "获取用户信息失败"})
		return
	}

	// 检查邮箱是否已验证
	if !userInfo.VerifiedEmail {
		Render(c, http.StatusBadRequest, "auth/login.html", gin.H{"Error": "Google 邮箱未验证"})
		return
	}

	// 查找用户(通过 GoogleID 或 Email)
	var user models.User
	err = db.DB.Where("google_id = ?", userInfo.ID).Or("email = ?", userInfo.Email).First(&user).Error

	if err != nil {
		// 新用户,自动注册
		username := userInfo.GivenName
		if username == "" {
			username = strings.Split(userInfo.Email, "@")[0]
		}

		// 使用 GoogleID 作为初始密码,方便用户后续在设置中修改密码
		newUser, err := h.createUser(username, userInfo.Email, userInfo.ID)
		if err != nil {
			Render(c, http.StatusInternalServerError, "auth/login.html", gin.H{"Error": "创建用户失败"})
			return
		}

		// 绑定 Google 账号
		newUser.GoogleID = userInfo.ID
		newUser.GoogleEmail = userInfo.Email
		db.DB.Save(newUser)

		user = *newUser
	} else {
		// 老用户,如果还没绑定 GoogleID,则绑定
		if user.GoogleID == "" {
			user.GoogleID = userInfo.ID
			user.GoogleEmail = userInfo.Email
			db.DB.Save(&user)
		}

		// 检查用户是否被封禁
		if user.Status == 2 {
			Render(c, http.StatusForbidden, "auth/login.html", gin.H{"Error": "您的账号已被封禁,无法登录。"})
			return
		}
	}

	// 登录
	session.Set("user_id", user.ID)
	session.Save()

	c.Redirect(http.StatusFound, "/")
}

// getGoogleUserInfo 获取 Google 用户信息
func (h *AuthHandler) getGoogleUserInfo(accessToken string) (*GoogleUserInfo, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取用户信息失败: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// BindGoogle 绑定 Google 账号
func (h *AuthHandler) BindGoogle(c *gin.Context) {
	state, err := generateStateToken()
	if err != nil {
		c.String(http.StatusInternalServerError, "生成状态令牌失败")
		return
	}

	// 将 state 和绑定标记存储到 session
	session := sessions.Default(c)
	session.Set("oauth_state", state)
	session.Set("oauth_bind_mode", true) // 标记为绑定模式
	session.Save()

	url := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleBindCallback 处理 Google 账号绑定回调
func (h *AuthHandler) GoogleBindCallback(c *gin.Context) {
	session := sessions.Default(c)
	savedState := session.Get("oauth_state")
	bindMode := session.Get("oauth_bind_mode")

	// 验证 state 参数
	if savedState == nil || c.Query("state") != savedState.(string) {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=invalid_state")
		return
	}

	// 清除 state
	session.Delete("oauth_state")
	session.Delete("oauth_bind_mode")
	session.Save()

	// 检查是否为绑定模式
	if bindMode == nil || !bindMode.(bool) {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=invalid_mode")
		return
	}

	// 获取当前登录用户
	userID := session.Get("user_id")
	if userID == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	var currentUser models.User
	if err := db.DB.First(&currentUser, userID).Error; err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=user_not_found")
		return
	}

	// 获取授权码
	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=no_code")
		return
	}

	// 交换 token
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=token_exchange_failed")
		return
	}

	// 获取用户信息
	userInfo, err := h.getGoogleUserInfo(token.AccessToken)
	if err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=get_userinfo_failed")
		return
	}

	// 检查该 Google 账号是否已被其他用户绑定
	var existingUser models.User
	err = db.DB.Where("google_id = ? AND id != ?", userInfo.ID, currentUser.ID).First(&existingUser).Error
	if err == nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=google_already_bound")
		return
	}

	// 绑定 Google 账号
	currentUser.GoogleID = userInfo.ID
	currentUser.GoogleEmail = userInfo.Email
	if err := db.DB.Save(&currentUser).Error; err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=bind_failed")
		return
	}

	c.Redirect(http.StatusFound, "/dashboard/settings?success=google_bound")
}

// UnbindGoogle 解除 Google 账号绑定
func (h *AuthHandler) UnbindGoogle(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=user_not_found")
		return
	}

	// 解除绑定
	user.GoogleID = ""
	user.GoogleEmail = ""
	if err := db.DB.Save(&user).Error; err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=unbind_failed")
		return
	}

	c.Redirect(http.StatusFound, "/dashboard/settings?success=google_unbound")
}
