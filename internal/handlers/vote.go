package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type VoteHandler struct{}

func NewVoteHandler() *VoteHandler {
	return &VoteHandler{}
}

// Vote handles upvote logic
func (h *VoteHandler) Vote(c *gin.Context) {
	user, exists := c.Get(middleware.CheckUserKey)
	if !exists {
		// HTMX should handle redirect or show login modal. For now, 401.
		c.Header("HX-Redirect", "/login")
		c.Status(http.StatusOK)
		return
	}
	currentUser := user.(*models.User)

	itemType := c.Param("type") // "post" or "comment"
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	uID := uint(id)

	tx := db.DB.Begin()

	query := tx.Where("user_id = ?", currentUser.ID)

	if itemType == "post" {
		query = query.Where("post_id = ?", uID)
	} else if itemType == "comment" {
		query = query.Where("comment_id = ?", uID)
	} else {
		tx.Rollback()
		c.Status(http.StatusBadRequest)
		return
	}

	// Check if already voted
	var existingVote models.Vote
	if err := query.First(&existingVote).Error; err == nil {
		// Already voted - return current upvote count
		tx.Rollback()
		var upvotes int64
		if itemType == "post" {
			db.DB.Model(&models.Vote{}).Where("post_id = ? AND value = 1", uID).Count(&upvotes)
		} else {
			db.DB.Model(&models.Vote{}).Where("comment_id = ? AND value = 1", uID).Count(&upvotes)
		}
		c.String(http.StatusOK, fmt.Sprintf("%d", upvotes))
		return
	}

	// Create vote
	newVote := models.Vote{
		UserID: currentUser.ID,
		Value:  1,
	}
	if itemType == "post" {
		newVote.PostID = &uID
	} else {
		newVote.CommentID = &uID
	}

	if err := tx.Create(&newVote).Error; err != nil {
		tx.Rollback()
		c.Status(http.StatusInternalServerError)
		return
	}

	// Update score (用于排序算法，Score 仍继续更新)
	if itemType == "post" {
		// Increment Post Score (用于排序算法)
		if err := tx.Model(&models.Post{}).Where("id = ?", uID).UpdateColumn("score", gorm.Expr("score + ?", 1)).Error; err != nil {
			tx.Rollback()
			c.Status(http.StatusInternalServerError)
			return
		}
	} else {
		// Increment Comment Score (用于排序算法)
		if err := tx.Model(&models.Comment{}).Where("id = ?", uID).UpdateColumn("score", gorm.Expr("score + ?", 1)).Error; err != nil {
			tx.Rollback()
			c.Status(http.StatusInternalServerError)
			return
		}
	}

	tx.Commit()

	// 异步更新帖子 Score
	if itemType == "post" {
		services.GetRankingService().ScheduleUpdate(uID)
	}

	// 异步给内容作者增加积分
	go func() {
		var authorID uint
		if itemType == "post" {
			var post models.Post
			if err := db.DB.First(&post, uID).Error; err == nil {
				authorID = post.UserID
			}
			if authorID != 0 && authorID != currentUser.ID {
				services.AddPoints(authorID, services.PointsPostLiked, services.ActionPostLiked)
			}
		} else {
			var comment models.Comment
			if err := db.DB.First(&comment, uID).Error; err == nil {
				authorID = comment.UserID
			}
			if authorID != 0 && authorID != currentUser.ID {
				services.AddPoints(authorID, services.PointsCommentLiked, services.ActionCommentLiked)
			}
		}
	}()

	// 返回点赞数（统计 Vote 表，而非 Post.Score）
	var upvotes int64
	if itemType == "post" {
		db.DB.Model(&models.Vote{}).Where("post_id = ? AND value = 1", uID).Count(&upvotes)
	} else {
		db.DB.Model(&models.Vote{}).Where("comment_id = ? AND value = 1", uID).Count(&upvotes)
	}
	c.String(http.StatusOK, fmt.Sprintf("%d", upvotes))
}

// Downvote 处理点踩逻辑
func (h *VoteHandler) Downvote(c *gin.Context) {
	user, exists := c.Get(middleware.CheckUserKey)
	if !exists {
		c.Header("HX-Redirect", "/login")
		c.Status(http.StatusOK)
		return
	}
	currentUser := user.(*models.User)

	itemType := c.Param("type") // "post" or "comment"
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	uID := uint(id)

	tx := db.DB.Begin()

	query := tx.Where("user_id = ?", currentUser.ID)

	if itemType == "post" {
		query = query.Where("post_id = ?", uID)
	} else if itemType == "comment" {
		query = query.Where("comment_id = ?", uID)
	} else {
		tx.Rollback()
		c.Status(http.StatusBadRequest)
		return
	}

	// 检查是否已投票
	var existingVote models.Vote
	if err := query.First(&existingVote).Error; err == nil {
		// 已投票 - 返回当前踩数
		tx.Rollback()
		var downvotes int64
		if itemType == "post" {
			db.DB.Model(&models.Vote{}).Where("post_id = ? AND value = -1", uID).Count(&downvotes)
		} else {
			db.DB.Model(&models.Vote{}).Where("comment_id = ? AND value = -1", uID).Count(&downvotes)
		}
		c.String(http.StatusOK, fmt.Sprintf("%d", downvotes))
		return
	}

	// 创建踩票
	newVote := models.Vote{
		UserID: currentUser.ID,
		Value:  -1,
	}
	if itemType == "post" {
		newVote.PostID = &uID
	} else {
		newVote.CommentID = &uID
	}

	if err := tx.Create(&newVote).Error; err != nil {
		tx.Rollback()
		c.Status(http.StatusInternalServerError)
		return
	}

	// 更新分数 (减少)
	if itemType == "post" {
		if err := tx.Model(&models.Post{}).Where("id = ?", uID).UpdateColumn("score", gorm.Expr("score - ?", 1)).Error; err != nil {
			tx.Rollback()
			c.Status(http.StatusInternalServerError)
			return
		}
	} else {
		if err := tx.Model(&models.Comment{}).Where("id = ?", uID).UpdateColumn("score", gorm.Expr("score - ?", 1)).Error; err != nil {
			tx.Rollback()
			c.Status(http.StatusInternalServerError)
			return
		}
	}

	tx.Commit()

	// 异步更新帖子 Score
	if itemType == "post" {
		services.GetRankingService().ScheduleUpdate(uID)
	}

	// 异步扣除积分：内容作者-3，点踩者-1
	go func() {
		var authorID uint
		if itemType == "post" {
			var post models.Post
			if err := db.DB.First(&post, uID).Error; err == nil {
				authorID = post.UserID
			}
			if authorID != 0 && authorID != currentUser.ID {
				services.AddPoints(authorID, services.PointsPostDownvoted, services.ActionPostDownvoted)
			}
		} else {
			var comment models.Comment
			if err := db.DB.First(&comment, uID).Error; err == nil {
				authorID = comment.UserID
			}
			if authorID != 0 && authorID != currentUser.ID {
				services.AddPoints(authorID, services.PointsCommentDownvoted, services.ActionCommentDownvoted)
			}
		}
		// 点踩者自己扣分
		services.AddPoints(currentUser.ID, services.PointsDownvoteOther, services.ActionDownvoteOther)
	}()

	var downvotes int64
	if itemType == "post" {
		db.DB.Model(&models.Vote{}).Where("post_id = ? AND value = -1", uID).Count(&downvotes)
	} else {
		db.DB.Model(&models.Vote{}).Where("comment_id = ? AND value = -1", uID).Count(&downvotes)
	}
	c.String(http.StatusOK, fmt.Sprintf("%d", downvotes))
}

// Report 处理举报逻辑
func (h *VoteHandler) Report(c *gin.Context) {
	user, exists := c.Get(middleware.CheckUserKey)
	if !exists {
		c.Header("HX-Redirect", "/login")
		c.Status(http.StatusOK)
		return
	}
	currentUser := user.(*models.User)

	itemType := c.Param("type") // "post" or "comment"
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	uID := uint(id)
	reason := c.PostForm("reason")

	// 查询项目的 Pid (如果是帖子直接用，如果是评论查询所属帖子)
	var itemPid string
	if itemType == "post" {
		var post models.Post
		if err := db.DB.First(&post, uID).Error; err == nil {
			itemPid = post.Pid
		}
	} else {
		var comment models.Comment
		if err := db.DB.Preload("Post").First(&comment, uID).Error; err == nil {
			itemPid = comment.Post.Pid
		}
	}

	report := models.Report{
		UserID:   currentUser.ID,
		ItemType: itemType,
		ItemID:   uID,
		ItemPid:  itemPid,
		Reason:   reason,
	}

	if err := db.DB.Create(&report).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// 异步向所有管理员发送举报通知
	go func() {
		// 查询所有管理员用户
		var admins []models.User
		if err := db.DB.Where("role = ?", "admin").Find(&admins).Error; err != nil {
			return
		}

		// 构建被举报内容的链接和描述
		var contentLink string
		var contentDesc string
		if itemType == "post" {
			contentLink = fmt.Sprintf("/p/%s", itemPid)
			var post models.Post
			if err := db.DB.First(&post, uID).Error; err == nil {
				contentDesc = fmt.Sprintf("文章《%s》", post.Title)
			} else {
				contentDesc = "一篇文章"
			}
		} else {
			contentLink = fmt.Sprintf("/p/%s#comment-%d", itemPid, uID)
			contentDesc = "一条评论"
		}

		// 为每个管理员创建通知
		for _, admin := range admins {
			notification := models.Notification{
				UserID:  admin.ID,
				ActorID: &currentUser.ID,
				Type:    models.NotificationTypeReport,
				Reason: fmt.Sprintf("举报了<a href=\"%s\" target=\"_blank\" class=\"text-moss font-medium hover:underline tracking-tight\">%s</a>,原因: %s",
					contentLink, contentDesc, reason),
			}
			db.DB.Create(&notification)
		}
	}()

	c.String(http.StatusOK, "已举报")
}
