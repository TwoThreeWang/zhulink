package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"
	"zhulink/internal/utils"

	"github.com/gin-gonic/gin"
)

type TransplantHandler struct{}

func NewTransplantHandler() *TransplantHandler {
	return &TransplantHandler{}
}

// ShowTransplantModal 检查去重并显示移栽弹窗
func (h *TransplantHandler) ShowTransplantModal(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	itemIDStr := c.Param("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的文章 ID")
		return
	}

	// 获取 RSS 文章
	var item models.FeedItem
	if err := db.DB.First(&item, itemID).Error; err != nil {
		c.String(http.StatusNotFound, "文章不存在")
		return
	}

	// 1. 去重检查：通过 URL
	var existingPost models.Post
	if err := db.DB.Where("url = ?", item.Link).First(&existingPost).Error; err == nil {
		// 已存在：自动点赞并提示
		h.autoUpvoteWithContext(&existingPost, user, c)
		return
	}

	// 2. 获取所有分类 (Nodes)
	var nodes []models.Node
	db.DB.Order("id ASC").Find(&nodes)

	c.HTML(http.StatusOK, "rss/transplant_modal.html", gin.H{
		"Item":  item,
		"Nodes": nodes,
	})
}

// Transplant 提交移栽
func (h *TransplantHandler) Transplant(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	itemIDStr := c.Param("id")
	itemID, _ := strconv.Atoi(itemIDStr)

	title := c.PostForm("title")
	nodeIDStr := c.PostForm("node_id")
	content := c.PostForm("content")

	nodeID, _ := strconv.Atoi(nodeIDStr)

	// 获取文章详情
	var item models.FeedItem
	if err := db.DB.First(&item, itemID).Error; err != nil {
		c.String(http.StatusNotFound, "文章不存在")
		return
	}

	// 0. 去重检查 (再次检查，防止并发)
	var existingPost models.Post
	if err := db.DB.Where("url = ?", item.Link).First(&existingPost).Error; err == nil {
		h.autoUpvote(user.ID, existingPost.ID)
		c.HTML(http.StatusOK, "rss/transplant_result.html", gin.H{
			"Success": true,
			"Message": "该文章已由他人推荐过，已为您自动点赞 +1",
		})
		return
	}

	// LLM 逻辑
	if content == "" {
		// 如果推荐语为空，调用 LLM 生成摘要
		llm := services.GetLLMService()
		summary, err := llm.GenerateSummary(item.Title, item.Description)
		if err == nil {
			if strings.Contains(summary, "CONTENT_UNSUITABLE") {
				// 内容不适宜逻辑
				go services.AddPoints(user.ID, services.PointsContentViolation, services.ActionContentVioloation)

				c.HTML(http.StatusOK, "rss/transplant_result.html", gin.H{
					"Success": false,
					"Message": "抱歉，AI 认为该文章内容不适宜公开推荐，已扣除 1 竹笋作为惩罚。",
				})
				return
			}
			content = summary
		} else {
			content = item.Description // 降级方案
		}
	}

	// 发布逻辑
	post := models.Post{
		Pid:        utils.RandStringBytesMaskImpr(8),
		UserID:     user.ID,
		NodeID:     uint(nodeID),
		Title:      title,
		URL:        item.Link,
		Content:    content,
		Score:      1, // 初始分，后续可触发自动点赞
		SourceType: "rss",
	}

	if err := db.DB.Create(&post).Error; err != nil {
		c.String(http.StatusInternalServerError, "发布失败")
		return
	}

	// 异步加分
	go func() {
		if services.CanEarnPostPoints(user.ID) {
			services.AddPoints(user.ID, services.PointsPostCreate, services.ActionPostCreate)
		}
	}()

	// 4. 返回成功提示（由 hx-swap="innerHTML" 渲染结果页）
	c.HTML(http.StatusOK, "rss/transplant_result.html", gin.H{
		"Success": true,
		"Message": "推荐成功！已为您发布到社区",
	})
}

// autoUpvote 内部自动点赞逻辑 (用于 Transplant 内部调用，不涉及 HTTP 响应)
func (h *TransplantHandler) autoUpvote(userID uint, postID uint) {
	// 检查是否已经投过票
	var existingVote models.Vote
	err := db.DB.Where("user_id = ? AND post_id = ?", userID, postID).First(&existingVote).Error
	if err != nil {
		// 未投票，执行点赞
		vote := models.Vote{
			UserID: userID,
			PostID: &postID,
			Value:  1,
		}
		db.DB.Create(&vote)

		// 更新分数
		var post models.Post
		if err := db.DB.First(&post, postID).Error; err == nil {
			db.DB.Model(&post).UpdateColumn("score", post.Score+1)
			services.GetRankingService().ScheduleUpdate(post.ID)

			// 给作者加分
			go services.AddPoints(post.UserID, services.PointsPostLiked, services.ActionPostLiked)
		}
	}
}

// autoUpvoteWithContext 内部自动点赞逻辑 (用于 ShowTransplantModal，需要 HTTP 响应)
func (h *TransplantHandler) autoUpvoteWithContext(post *models.Post, user *models.User, c *gin.Context) {
	// 检查是否已经投过票
	var existingVote models.Vote
	err := db.DB.Where("user_id = ? AND post_id = ?", user.ID, post.ID).First(&existingVote).Error
	if err != nil {
		// 未投票，执行点赞
		vote := models.Vote{
			UserID: user.ID,
			PostID: &post.ID,
			Value:  1,
		}
		db.DB.Create(&vote)
		// 更新分数
		db.DB.Model(post).UpdateColumn("score", post.Score+1)
		services.GetRankingService().ScheduleUpdate(post.ID)

		// 给作者加分
		go services.AddPoints(post.UserID, services.PointsPostLiked, services.ActionPostLiked)
	}

	c.HTML(http.StatusOK, "rss/transplant_result.html", gin.H{
		"Success": true,
		"Message": "该文章已由他人推荐过，已为您自动点赞 +1",
	})
}
