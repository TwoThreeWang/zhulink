package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"
	"zhulink/internal/utils"

	"github.com/gin-gonic/gin"
)

type BookmarkHandler struct{}

func NewBookmarkHandler() *BookmarkHandler {
	return &BookmarkHandler{}
}

// Toggle 切换收藏状态 - 收藏/取消收藏
func (h *BookmarkHandler) Toggle(c *gin.Context) {
	user, exists := c.Get(middleware.CheckUserKey)
	if !exists {
		c.Header("HX-Redirect", "/login")
		c.Status(http.StatusOK)
		return
	}
	currentUser := user.(*models.User)

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	postID := uint(id)

	// 检查文章是否存在
	var post models.Post
	if err := db.DB.First(&post, postID).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// 检查是否已收藏
	var existing models.Bookmark
	if err := db.DB.Where("user_id = ? AND post_id = ?", currentUser.ID, postID).First(&existing).Error; err == nil {
		// 已收藏，取消收藏；并发重复请求只有真正删掉的那次才扣分
		if db.DB.Delete(&existing).RowsAffected > 0 && post.UserID != currentUser.ID {
			services.AddPointsAsync(post.UserID, services.PointsPostUnbookmark, services.ActionPostUnbookmark)
		}
	} else {
		// 未收藏，添加收藏
		bookmark := models.Bookmark{
			UserID: currentUser.ID,
			PostID: postID,
		}
		// 并发重复请求会撞唯一索引失败，只有插入成功才给作者加分
		if db.DB.Create(&bookmark).Error == nil && post.UserID != currentUser.ID {
			services.AddPointsAsync(post.UserID, services.PointsPostBookmarked, services.ActionPostBookmarked)
		}
	}

	// 主动失效详情页缓存
	utils.GetCache().Delete(fmt.Sprintf("story:detail:shared:%s", post.Pid))

	// 异步更新帖子 Score
	services.GetRankingService().ScheduleUpdate(postID)

	// 获取当前收藏数
	var count int64
	db.DB.Model(&models.Bookmark{}).Where("post_id = ?", postID).Count(&count)

	// 检查当前用户是否已收藏（用于前端高亮）
	var isBookmarked bool
	var check models.Bookmark
	if err := db.DB.Where("user_id = ? AND post_id = ?", currentUser.ID, postID).First(&check).Error; err == nil {
		isBookmarked = true
	}

	// 返回更新后的HTML片段 - 匹配前端 #bookmark-content-{id} 的内容结构
	if isBookmarked {
		c.String(http.StatusOK, fmt.Sprintf(`
			<svg class="icon w-4 h-4 text-amber-500 fill-amber-500" aria-hidden="true"><use href="#i-bookmark"/></svg>
			<span>收藏 %d</span>`, count))
	} else {
		c.String(http.StatusOK, fmt.Sprintf(`
			<svg class="icon w-4 h-4 text-stone-400 group-hover:text-amber-500 transition-colors" aria-hidden="true"><use href="#i-bookmark"/></svg>
			<span>收藏 %d</span>`, count))
	}
}

// GetBookmarkCount 获取文章收藏数
func GetBookmarkCount(postID uint) int64 {
	var count int64
	db.DB.Model(&models.Bookmark{}).Where("post_id = ?", postID).Count(&count)
	return count
}

// IsBookmarked 检查用户是否已收藏某文章
func IsBookmarked(userID, postID uint) bool {
	var bookmark models.Bookmark
	if err := db.DB.Where("user_id = ? AND post_id = ?", userID, postID).First(&bookmark).Error; err == nil {
		return true
	}
	return false
}
