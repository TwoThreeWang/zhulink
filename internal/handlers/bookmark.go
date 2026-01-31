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
		// 已收藏，取消收藏
		db.DB.Delete(&existing)
		// 异步扣除帖子作者积分
		if post.UserID != currentUser.ID {
			services.AddPointsAsync(post.UserID, services.PointsPostUnbookmark, services.ActionPostUnbookmark)
		}
	} else {
		// 未收藏，添加收藏
		bookmark := models.Bookmark{
			UserID: currentUser.ID,
			PostID: postID,
		}
		db.DB.Create(&bookmark)
		// 异步增加帖子作者积分
		if post.UserID != currentUser.ID {
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
			<div class="p-1.5 rounded-full bg-stone-50 group-hover:bg-white group-hover:shadow-sm transition-all border border-transparent group-hover:border-stone-100">
				<i data-lucide="bookmark" class="w-5 h-5 text-amber-500 fill-amber-500"></i>
			</div>
			<span class="text-xs font-semibold text-amber-600">收藏(%d)</span>`, count))
	} else {
		c.String(http.StatusOK, fmt.Sprintf(`
			<div class="p-1.5 rounded-full bg-stone-50 group-hover:bg-white group-hover:shadow-sm transition-all border border-transparent group-hover:border-stone-100">
				<i data-lucide="bookmark" class="w-5 h-5 text-stone-400 group-hover:text-amber-500 transition-colors"></i>
			</div>
			<span class="text-xs font-semibold text-stone-500 group-hover:text-amber-600">收藏(%d)</span>`, count))
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
