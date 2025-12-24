package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"zhulink/internal/db"
	"zhulink/internal/models"
	"zhulink/internal/services"
	"zhulink/internal/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserHandler struct{}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

// Profile - 用户主页 /u/:id
func (h *UserHandler) Profile(c *gin.Context) {
	userID := c.Param("id")

	// 查找用户
	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		Render(c, http.StatusNotFound, "error.html", gin.H{"Error": "用户不存在"})
		return
	}

	// 计算用户等级和林龄
	levelName, levelIcon := utils.GetUserLevel(user.Points)
	daysSince := utils.GetDaysSinceJoined(user.CreatedAt)

	// 获取 tab 参数，默认为 posts
	tab := c.DefaultQuery("tab", "posts")

	var posts []models.Post
	var comments []models.Comment
	var bookmarkedPosts []models.Post

	if tab == "posts" {
		// 查询用户发布的文章
		db.DB.Preload("Node").
			Preload("User").
			Where("user_id = ?", user.ID).
			Order("created_at DESC").
			Limit(50).
			Find(&posts)
		fillCommentCounts(posts)
	} else if tab == "comments" {
		// 查询用户的评论
		db.DB.Preload("Post").
			Preload("User").
			Where("user_id = ?", user.ID).
			Order("created_at DESC").
			Limit(50).
			Find(&comments)
	} else if tab == "bookmarks" {
		// 查询用户收藏的文章
		var bookmarks []models.Bookmark
		db.DB.Preload("Post").
			Preload("Post.Node").
			Preload("Post.User").
			Where("user_id = ?", user.ID).
			Order("created_at DESC").
			Limit(50).
			Find(&bookmarks)
		// 提取 Post 列表
		for _, b := range bookmarks {
			bookmarkedPosts = append(bookmarkedPosts, b.Post)
		}
		fillCommentCounts(bookmarkedPosts)
	}

	Render(c, http.StatusOK, "user/public.html", gin.H{
		"Title":           user.Username + " 的主页",
		"User":            user,
		"LevelName":       levelName,
		"LevelIcon":       levelIcon,
		"DaysSince":       daysSince,
		"Posts":           posts,
		"Comments":        comments,
		"BookmarkedPosts": bookmarkedPosts,
		"ActiveTab":       tab,
	})
}

// Dashboard - 个人后台概览
func (h *UserHandler) Dashboard(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// 统计数据
	var postCount, commentCount int64
	db.DB.Model(&models.Post{}).Where("user_id = ?", user.ID).Count(&postCount)
	db.DB.Model(&models.Comment{}).Where("user_id = ?", user.ID).Count(&commentCount)

	levelName, levelIcon := utils.GetUserLevel(user.Points)
	daysSince := utils.GetDaysSinceJoined(user.CreatedAt)

	Render(c, http.StatusOK, "dashboard/overview.html", gin.H{
		"Title":        "个人后台",
		"User":         user,
		"LevelName":    levelName,
		"LevelIcon":    levelIcon,
		"DaysSince":    daysSince,
		"PostCount":    postCount,
		"CommentCount": commentCount,
	})
}

// Notifications - 消息中心
func (h *UserHandler) Notifications(c *gin.Context) {
	Render(c, http.StatusOK, "dashboard/notifications.html", gin.H{
		"Title": "消息中心",
	})
}

// PointLogs - 积分明细
func (h *UserHandler) PointLogs(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")

	var logs []models.PointLog
	db.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(100).
		Find(&logs)

	Render(c, http.StatusOK, "dashboard/points.html", gin.H{
		"Title": "积分明细",
		"Logs":  logs,
	})
}

// CheckIn - 每日签到
func (h *UserHandler) CheckIn(c *gin.Context) {
	session := sessions.Default(c)
	userIDInterface := session.Get("user_id")
	if userIDInterface == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	var userID uint
	switch v := userIDInterface.(type) {
	case uint:
		userID = v
	case int:
		userID = uint(v)
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}

	points, bonus, alreadyCheckedIn, err := services.CheckIn(userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签到失败"})
		return
	}

	if alreadyCheckedIn {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "今日已签到",
		})
		return
	}

	totalPoints := points + bonus
	message := fmt.Sprintf("签到成功！获得 %d 🌿竹笋", totalPoints)
	if bonus > 0 {
		message = fmt.Sprintf("签到成功！获得 %d 🌿竹笋（含 %d 额外奖励！）", totalPoints, bonus)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     message,
		"points":      points,
		"bonus":       bonus,
		"totalPoints": totalPoints,
	})
}

// ShowSettings - 显示设置页面
func (h *UserHandler) ShowSettings(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	Render(c, http.StatusOK, "dashboard/settings.html", gin.H{
		"Title":        "设置",
		"User":         user,
		"CommonEmojis": utils.GetCommonEmojis(),
	})
}

// UpdateSettings - 更新设置
func (h *UserHandler) UpdateSettings(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	// 获取表单数据
	username := c.PostForm("username")
	email := c.PostForm("email")
	avatar := c.PostForm("avatar")
	bio := c.PostForm("bio")
	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")

	// 更新基本信息
	updates := make(map[string]interface{})

	if username != "" && username != user.Username {
		updates["username"] = username
	}

	if email != "" && email != user.Email {
		// 检查邮箱是否已被使用
		var existingUser models.User
		if err := db.DB.Where("email = ? AND id != ?", email, user.ID).First(&existingUser).Error; err == nil {
			Render(c, http.StatusBadRequest, "dashboard/settings.html", gin.H{
				"Error":        "该邮箱已被使用",
				"User":         user,
				"CommonEmojis": utils.GetCommonEmojis(),
			})
			return
		}
		updates["email"] = email
	}

	if avatar != "" {
		updates["avatar"] = avatar
	}

	if bio != user.Bio {
		updates["bio"] = bio
	}

	// 如果要修改密码
	if oldPassword != "" && newPassword != "" {
		if !utils.CheckPasswordHash(oldPassword, user.Password) {
			Render(c, http.StatusBadRequest, "dashboard/settings.html", gin.H{
				"Error":        "原密码错误",
				"User":         user,
				"CommonEmojis": utils.GetCommonEmojis(),
			})
			return
		}

		if len(newPassword) < 6 {
			Render(c, http.StatusBadRequest, "dashboard/settings.html", gin.H{
				"Error":        "新密码至少6位",
				"User":         user,
				"CommonEmojis": utils.GetCommonEmojis(),
			})
			return
		}

		hash, err := utils.HashPassword(newPassword)
		if err != nil {
			Render(c, http.StatusInternalServerError, "dashboard/settings.html", gin.H{
				"Error":        "系统错误",
				"User":         user,
				"CommonEmojis": utils.GetCommonEmojis(),
			})
			return
		}
		updates["password"] = hash
	}

	// 执行更新
	if len(updates) > 0 {
		if err := db.DB.Model(&user).Updates(updates).Error; err != nil {
			Render(c, http.StatusInternalServerError, "dashboard/settings.html", gin.H{
				"Error":        "更新失败",
				"User":         user,
				"CommonEmojis": utils.GetCommonEmojis(),
			})
			return
		}
	}

	c.Redirect(http.StatusFound, "/dashboard/settings?success=1")
}

// AddPointLog 添加积分记录并更新用户积分
func AddPointLog(userID uint, amount int, action string) error {
	// 创建记录
	log := models.PointLog{
		UserID: userID,
		Amount: amount,
		Action: action,
	}
	if err := db.DB.Create(&log).Error; err != nil {
		return err
	}

	// 更新用户积分
	return db.DB.Model(&models.User{}).
		Where("id = ?", userID).
		UpdateColumn("points", gorm.Expr("points + ?", amount)).
		Error
}

// GetUserStats 获取用户统计信息（辅助函数）
func GetUserStats(userID uint) (postCount, commentCount int64) {
	db.DB.Model(&models.Post{}).Where("user_id = ?", userID).Count(&postCount)
	db.DB.Model(&models.Comment{}).Where("user_id = ?", userID).Count(&commentCount)
	return
}

// Helper: 从 session 获取当前用户
func getCurrentUser(c *gin.Context) (*models.User, error) {
	session := sessions.Default(c)
	userIDInterface := session.Get("user_id")
	if userIDInterface == nil {
		return nil, gorm.ErrRecordNotFound
	}

	var userID uint
	switch v := userIDInterface.(type) {
	case uint:
		userID = v
	case int:
		userID = uint(v)
	case string:
		id, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, err
		}
		userID = uint(id)
	default:
		return nil, gorm.ErrRecordNotFound
	}

	var user models.User
	if err := db.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
