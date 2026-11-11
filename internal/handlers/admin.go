package handlers

import (
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"
	"zhulink/internal/utils"

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
	if err := db.DB.Model(&post).Update("is_top", post.IsTop).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	invalidatePostCache(post.Pid)

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
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var node models.Node
	if err := db.DB.First(&node, uint(nodeID)).Error; err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if err := db.DB.Model(&models.Post{}).Where("pid = ?", pid).Update("node_id", node.ID).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	invalidatePostCache(pid)

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
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	statusStr := c.PostForm("status") // 0: 正常, 1: 禁言, 2: 封禁
	status, statusErr := strconv.Atoi(statusStr)
	if err != nil || statusErr != nil || status < 0 || status > 2 {
		c.Status(http.StatusBadRequest)
		return
	}
	daysStr := c.PostForm("days")
	days, _ := strconv.Atoi(daysStr)
	reason := c.PostForm("reason") // 惩罚原因

	updates := map[string]interface{}{
		"status": status,
	}

	if status != 0 && days > 0 {
		expires := time.Now().AddDate(0, 0, days)
		updates["punish_expires"] = &expires
	} else {
		updates["punish_expires"] = nil
	}

	if err := db.DB.Model(&models.User{}).Where("id = ?", uint(userID)).Updates(updates).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// 发送通知给被惩罚用户
	go func() {
		var notificationReason string
		if status == 1 {
			// 禁言
			if days > 0 {
				notificationReason = "您已被管理员禁言 " + strconv.Itoa(days) + " 天。"
			} else {
				notificationReason = "您已被管理员禁言。"
			}
		} else if status == 2 {
			// 封禁
			if days > 0 {
				notificationReason = "您的账号已被管理员封禁 " + strconv.Itoa(days) + " 天。"
			} else {
				notificationReason = "您的账号已被管理员永久封禁。"
			}
		} else {
			// 解除惩罚
			notificationReason = "您的账号已恢复正常状态。"
		}

		// 如果有原因,添加到通知中
		if reason != "" {
			notificationReason += " 原因: " + html.EscapeString(reason)
		}

		notification := models.Notification{
			UserID: uint(userID),
			Type:   models.NotificationTypeSystem,
			Reason: notificationReason,
		}
		db.DB.Create(&notification)
	}()

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
		Reason: "很抱歉，您的文章《" + html.EscapeString(post.Title) + "》因违规已被管理员删除。如有疑问请联系管理。",
	}
	db.DB.Create(&notification)

	// 3. 彻底删除帖子
	db.DB.Unscoped().Delete(&post)
	invalidatePostCache(post.Pid)

	c.Header("HX-Redirect", "/")
	c.Status(http.StatusOK)
}

// ShowRestoreAIBlockedPost 恢复确认页。通知里的链接是 GET，这里只展示帖子信息，
// 真正的恢复要点确认按钮发 POST：否则帖子里放一张指向这个地址的图片，管理员一浏览就自动恢复了
func (h *AdminHandler) ShowRestoreAIBlockedPost(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Status(http.StatusForbidden)
		return
	}

	var post models.Post
	if err := db.DB.Unscoped().Preload("User").Where("pid = ?", c.Param("pid")).First(&post).Error; err != nil {
		Render(c, http.StatusNotFound, "error.html", gin.H{"Error": "文章不存在或无法恢复"})
		return
	}

	Render(c, http.StatusOK, "admin/restore_confirm.html", gin.H{
		"Title": "恢复被拦截的文章",
		"Post":  post,
	})
}

// RestoreAIBlockedPost 恢复 AI 误判软删除的帖子，并解除自动禁言。
func (h *AdminHandler) RestoreAIBlockedPost(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Status(http.StatusForbidden)
		return
	}

	pid := c.Param("pid")
	var post models.Post
	if err := db.DB.Unscoped().Preload("User").Where("pid = ?", pid).First(&post).Error; err != nil {
		Render(c, http.StatusNotFound, "error.html", gin.H{"Error": "文章不存在或无法恢复"})
		return
	}

	postUpdates := map[string]interface{}{
		"index_status": models.PostIndexStatusIndexable,
	}
	if post.DeletedAt.Valid {
		postUpdates["deleted_at"] = nil
	}
	if err := db.DB.Unscoped().
		Model(&models.Post{}).
		Where("pid = ?", pid).
		Updates(postUpdates).Error; err != nil {
		Render(c, http.StatusInternalServerError, "error.html", gin.H{"Error": "恢复文章失败"})
		return
	}

	var user models.User
	if err := db.DB.First(&user, post.UserID).Error; err == nil && user.Status == 1 {
		db.DB.Model(&models.User{}).Where("id = ?", post.UserID).Updates(map[string]interface{}{
			"status":         0,
			"punish_expires": nil,
		})
	}

	notification := models.Notification{
		UserID: post.UserID,
		Type:   models.NotificationTypeSystem,
		Reason: "您的文章《" + html.EscapeString(post.Title) + "》经管理员复核后已恢复，账号禁言状态也已解除。<br><a href=\"/p/" + post.Pid + "\" class=\"text-moss font-medium hover:underline tracking-tight\">查看文章</a>",
	}
	db.DB.Create(&notification)

	utils.GetCache().Delete("story:detail:shared:" + post.Pid)

	c.Redirect(http.StatusFound, "/p/"+post.Pid)
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

	// 必须先转成数字：GORM 把字符串主键当成原始 SQL 条件，"1 OR 1=1" 会删光整张表
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	// 这里可以实现删除举报记录或者标记已处理
	if err := db.DB.Delete(&models.Report{}, uint(id)).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

// AdminDeleteComment 管理员删除评论
func (h *AdminHandler) AdminDeleteComment(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Status(http.StatusForbidden)
		return
	}

	cid := c.Param("cid")
	var comment models.Comment
	// 预加载 Post 信息以获取文章链接
	if err := db.DB.Preload("Post").Where("cid = ?", cid).First(&comment).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// 保存原评论内容用于通知
	originalContent := comment.Content
	postLink := "/p/" + comment.Post.Pid

	// 1. 软删除：只替换内容。已经删除过的不再处理，否则每点一次就多扣一次分、多发一条通知
	deleted, err := markCommentDeleted(comment.ID, commentDeletedByAdmin)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	invalidatePostCache(comment.Post.Pid)
	if !deleted {
		c.Status(http.StatusOK)
		return
	}

	// 2. 扣除评论作者积分 (-3分)
	services.AddPointsAsync(comment.UserID, services.PointsCommentDeleted, "评论被管理员删除")

	// 3. 发送系统通知给评论作者，附上原评论内容和文章链接
	safeOriginal := strings.ReplaceAll(html.EscapeString(originalContent), "\n", "<br>")
	notification := models.Notification{
		UserID: comment.UserID,
		Type:   models.NotificationTypeSystem,
		Reason: "很抱歉，您的评论因违规已被管理员删除。如有疑问请联系管理。<br><br>原评论内容：<br>" + safeOriginal + "<br><br>文章链接：<a href='" + postLink + "#comment-" + strconv.FormatUint(uint64(comment.ID), 10) + "'>" + postLink + "</a>",
	}
	db.DB.Create(&notification)

	c.Status(http.StatusOK)
}

// ListUsers 用户列表（管理员）
func (h *AdminHandler) ListUsers(c *gin.Context) {
	if h.checkAdmin(c) == nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 20
	offset := (page - 1) * pageSize

	var users []models.User
	var total int64
	db.DB.Model(&models.User{}).Count(&total)
	db.DB.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users)

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	Render(c, http.StatusOK, "admin/users.html", gin.H{
		"Title":       "用户管理",
		"Users":       users,
		"CurrentUser": h.checkAdmin(c),
		"Page":        page,
		"TotalPages":  totalPages,
		"Total":       total,
		"HasNext":     page < totalPages,
		"HasPrev":     page > 1,
	})
}
