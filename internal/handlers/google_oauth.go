package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"
	"zhulink/internal/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

var googleOauthConfig *oauth2.Config

// InitGoogleOAuth 初始化 Google OAuth 配置
func InitGoogleOAuth() {
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:32919"
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

	go clearGoogleIDPasswords()
}

// clearGoogleIDPasswords 旧版 Google 注册把 GoogleID 当密码，GoogleID 可被公开工具查到，
// 启动时把这类账号的密码清空（之后只能用 Google 登录或在设置里设新密码）
func clearGoogleIDPasswords() {
	var users []models.User
	if err := db.DB.Select("id, google_id, password").Where("google_id <> '' AND password <> ''").Find(&users).Error; err != nil {
		log.Printf("清理 GoogleID 密码失败: %v", err)
		return
	}
	cleared := 0
	for _, u := range users {
		if utils.CheckPasswordHash(u.GoogleID, u.Password) {
			db.DB.Model(&models.User{}).Where("id = ?", u.ID).UpdateColumn("password", "")
			cleared++
		}
	}
	if cleared > 0 {
		log.Printf("已清空 %d 个以 GoogleID 为密码的账号密码", cleared)
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

// oauthBindStateKey 绑定流程在会话里记下的 state；和 oauth_state 相同才说明这次回调是绑定而不是登录
const oauthBindStateKey = "oauth_bind_state"

// googleRequestTimeout 向 Google 换令牌、取用户信息的超时
const googleRequestTimeout = 10 * time.Second

// GoogleLogin 发起 Google OAuth 登录
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	h.startGoogleOAuth(c, false)
}

// BindGoogle 绑定 Google 账号
func (h *AuthHandler) BindGoogle(c *gin.Context) {
	h.startGoogleOAuth(c, true)
}

// startGoogleOAuth 生成 state 存进会话并跳转到 Google。
// 登录和绑定共用同一个回调地址（Google 后台只登记了 /auth/google/callback），靠会话里的绑定标记区分
func (h *AuthHandler) startGoogleOAuth(c *gin.Context, bind bool) {
	state, err := generateStateToken()
	if err != nil {
		c.String(http.StatusInternalServerError, "生成状态令牌失败")
		return
	}

	session := sessions.Default(c)
	session.Set("oauth_state", state)
	if bind {
		session.Set(oauthBindStateKey, state)
	} else {
		session.Delete(oauthBindStateKey)
	}
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, "保存状态失败，请重试")
		return
	}

	c.Redirect(http.StatusFound, googleOauthConfig.AuthCodeURL(state))
}

// GoogleCallback 处理 Google OAuth 回调（登录和绑定共用）
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	session := sessions.Default(c)
	savedState, _ := session.Get("oauth_state").(string)
	bindState, _ := session.Get(oauthBindStateKey).(string)
	isBind := bindState != "" && bindState == savedState

	// state 只能用一次
	session.Delete("oauth_state")
	session.Delete(oauthBindStateKey)
	session.Save()

	// 出错时：绑定流程回设置页，登录流程回登录页
	fail := func(status int, settingsCode, loginMsg string) {
		if isBind {
			c.Redirect(http.StatusFound, "/dashboard/settings?error="+settingsCode)
			return
		}
		renderLogin(c, status, gin.H{"Error": loginMsg})
	}

	// 验证 state 参数
	if savedState == "" || subtle.ConstantTimeCompare([]byte(c.Query("state")), []byte(savedState)) != 1 {
		fail(http.StatusBadRequest, "invalid_state", "无效的状态参数")
		return
	}

	// 获取授权码
	code := c.Query("code")
	if code == "" {
		fail(http.StatusBadRequest, "no_code", "未获取到授权码")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), googleRequestTimeout)
	defer cancel()

	// 交换 token
	token, err := googleOauthConfig.Exchange(ctx, code)
	if err != nil {
		fail(http.StatusInternalServerError, "token_exchange_failed", "获取访问令牌失败")
		return
	}

	// 获取用户信息
	userInfo, err := h.getGoogleUserInfo(ctx, token)
	if err != nil || userInfo.ID == "" {
		fail(http.StatusInternalServerError, "get_userinfo_failed", "获取用户信息失败")
		return
	}

	if isBind {
		h.finishGoogleBind(c, userInfo)
		return
	}

	// 检查邮箱是否已验证
	if !userInfo.VerifiedEmail || !services.ValidEmailAddress(normalizeEmail(userInfo.Email)) {
		renderLogin(c, http.StatusBadRequest, gin.H{"Error": "Google 邮箱未验证"})
		return
	}

	// 查找用户：先按 GoogleID（绑定关系最可靠），找不到再按邮箱。
	// 不能写成一条 "google_id = ? OR email = ?"：两个条件各命中一个账号时，会登进 id 较小的那个
	var user models.User
	err = db.DB.Where("google_id = ?", userInfo.ID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = findUserByEmail(userInfo.Email, &user)
	}

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 新用户,自动注册
		username := userInfo.GivenName
		if username == "" {
			username = strings.Split(userInfo.Email, "@")[0]
		}

		// 不设密码：GoogleID 可被公开工具查到，不能当密码；用户可在设置里自行设置密码
		newUser, err := h.createUser(uniqueUsername(utils.SanitizeUsername(username)), normalizeEmail(userInfo.Email), "")
		if err != nil {
			renderLogin(c, http.StatusInternalServerError, gin.H{"Error": "创建用户失败"})
			return
		}

		// 绑定 Google 账号；Google 已验证过邮箱，直接视为已激活
		if err := db.DB.Model(newUser).Updates(map[string]interface{}{
			"google_id": userInfo.ID, "google_email": userInfo.Email, "is_activated": true,
		}).Error; err != nil {
			renderLogin(c, http.StatusInternalServerError, gin.H{"Error": "创建用户失败"})
			return
		}
		user = *newUser

	case err != nil:
		// 数据库出错时不能当成"用户不存在"去注册新账号
		renderLogin(c, http.StatusInternalServerError, gin.H{"Error": "登录失败，请稍后重试"})
		return

	default:
		// 老用户,如果还没绑定 GoogleID,则绑定；Google 已验证过邮箱，未激活的顺便激活
		updates := map[string]interface{}{}
		if user.GoogleID == "" {
			updates["google_id"] = userInfo.ID
			updates["google_email"] = userInfo.Email
		}
		if !user.IsActivated {
			// 未激活账号的密码没人验证过，可能是别人抢注时设的；不清掉的话对方激活后就能用密码登录
			updates["password"] = ""
			updates["is_activated"] = true
		}
		if len(updates) > 0 {
			if err := db.DB.Model(&user).Updates(updates).Error; err != nil {
				renderLogin(c, http.StatusInternalServerError, gin.H{"Error": "登录失败，请稍后重试"})
				return
			}
		}

		// 检查用户是否被封禁（到期的惩罚先解除）
		middleware.ClearExpiredPunishment(&user)
		if user.Status == 2 {
			renderLogin(c, http.StatusForbidden, gin.H{"Error": "您的账号已被封禁,无法登录。"})
			return
		}
	}

	// 登录
	if err := startUserSession(c, user.ID, user.SessionVersion); err != nil {
		renderLogin(c, http.StatusInternalServerError, gin.H{"Error": "登录状态保存失败，请重试"})
		return
	}

	c.Redirect(http.StatusFound, "/")
}

// finishGoogleBind 把 Google 账号绑定到当前登录用户
func (h *AuthHandler) finishGoogleBind(c *gin.Context, userInfo *GoogleUserInfo) {
	u, exists := c.Get(middleware.CheckUserKey)
	if !exists {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	currentUser := u.(*models.User)

	// 检查该 Google 账号是否已被其他用户绑定
	var count int64
	if err := db.DB.Model(&models.User{}).Where("google_id = ? AND id != ?", userInfo.ID, currentUser.ID).Count(&count).Error; err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=bind_failed")
		return
	}
	if count > 0 {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=google_already_bound")
		return
	}

	if err := db.DB.Model(&models.User{}).Where("id = ?", currentUser.ID).
		Updates(map[string]interface{}{"google_id": userInfo.ID, "google_email": userInfo.Email}).Error; err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=bind_failed")
		return
	}

	c.Redirect(http.StatusFound, "/dashboard/settings?success=google_bound")
}

// getGoogleUserInfo 获取 Google 用户信息。令牌放在 Authorization 头里，不拼进 URL（URL 会进日志）
func (h *AuthHandler) getGoogleUserInfo(ctx context.Context, token *oauth2.Token) (*GoogleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := (&http.Client{Timeout: googleRequestTimeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取用户信息失败: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
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

	// 没有密码的账号（Google 注册）解绑后将无法登录，必须先设置密码
	if user.Password == "" {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=set_password_first")
		return
	}

	// 解除绑定
	if err := db.DB.Model(&user).Updates(map[string]interface{}{"google_id": "", "google_email": ""}).Error; err != nil {
		c.Redirect(http.StatusFound, "/dashboard/settings?error=unbind_failed")
		return
	}

	c.Redirect(http.StatusFound, "/dashboard/settings?success=google_unbound")
}
