package handlers

import (
	"net/http"
	"strconv"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct{}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

// AdminRequired middleware helper
func (h *AdminHandler) checkAdmin(c *gin.Context) *models.User {
	u, exists := c.Get(middleware.CheckUserKey)
	if !exists {
		return nil
	}
	user := u.(*models.User)
	if user.Role != "admin" {
		return nil
	}
	return user
}

// ToggleTop 置顶/取消置顶
func (h *AdminHandler) ToggleTop(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Status(http.StatusForbidden)
		return
	}

	pid := c.Param("pid")
	var post models.Post
	if err := db.DB.Where("pid = ?", pid).First(&post).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	post.IsTop = !post.IsTop
	db.DB.Model(&post).Update("is_top", post.IsTop)

	// HTMX: 返回按钮新状态
	label := "置顶"
	if post.IsTop {
		label = "取消置顶"
	}
	c.String(http.StatusOK, label)
}

// MoveNode 移动节点
func (h *AdminHandler) MoveNode(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Status(http.StatusForbidden)
		return
	}

	pid := c.Param("pid")
	nodeIDStr := c.PostForm("node_id")
	nodeID, _ := strconv.Atoi(nodeIDStr)

	if err := db.DB.Model(&models.Post{}).Where("pid = ?", pid).Update("node_id", nodeID).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Header("HX-Refresh", "true")
	c.Status(http.StatusOK)
}

// PunishUser 惩罚用户（禁言、封禁）
func (h *AdminHandler) PunishUser(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Status(http.StatusForbidden)
		return
	}

	userIDStr := c.Param("id")
	userID, _ := strconv.Atoi(userIDStr)
	statusStr := c.PostForm("status") // 0: 正常, 1: 禁言, 2: 封禁
	status, _ := strconv.Atoi(statusStr)
	daysStr := c.PostForm("days")
	days, _ := strconv.Atoi(daysStr)

	updates := map[string]interface{}{
		"status": status,
	}

	if status != 0 && days > 0 {
		expires := time.Now().AddDate(0, 0, days)
		updates["punish_expires"] = &expires
	} else {
		updates["punish_expires"] = nil
	}

	if err := db.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Header("HX-Refresh", "true")
	c.Status(http.StatusOK)
}

// AdminDeletePost 管理员删除帖子
func (h *AdminHandler) AdminDeletePost(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Status(http.StatusForbidden)
		return
	}

	pid := c.Param("pid")
	var post models.Post
	if err := db.DB.Where("pid = ?", pid).First(&post).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// 1. 扣除原作者积分 (-10分)
	services.AddPointsAsync(post.UserID, services.PointsPostDeleted, "文章被管理员删除")

	// 2. 发送系统通知给作者
	notification := models.Notification{
		UserID: post.UserID,
		Type:   models.NotificationTypeSystem,
		Reason: "很抱歉，您的文章《" + post.Title + "》因违规已被管理员删除。如有疑问请联系管理。",
	}
	db.DB.Create(&notification)

	// 3. 彻底删除帖子
	db.DB.Unscoped().Delete(&post)

	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}

// ListReports 举报列表
func (h *AdminHandler) ListReports(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	var reports []models.Report
	db.DB.Preload("User").Order("created_at DESC").Find(&reports)

	// 手动关联被举报内容（由于 Report 表目前没有直接关联结构，需要逻辑处理或 Preload）
	// 这里简单处理，Preload 已经在 models 里定义了 User
	// 为了方便展示标题和内容，我们需要根据 ItemType 去查询

	Render(c, http.StatusOK, "admin/reports.html", gin.H{
		"Title":       "举报管理",
		"Reports":     reports,
		"CurrentUser": h.checkAdmin(c),
	})
}

// HandleReport 处理/忽略举报
func (h *AdminHandler) HandleReport(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Status(http.StatusForbidden)
		return
	}

	id := c.Param("id")
	// 这里可以实现删除举报记录或者标记已处理
	db.DB.Delete(&models.Report{}, id)

	c.Status(http.StatusOK)
}
