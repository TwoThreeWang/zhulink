package handlers

import (
	"log"
	"net/http"
	"regexp"
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
			"PostID":  existingPost.Pid,
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
				log.Printf("[Transplant] AI 判定内容不适宜 (user_id=%d, item_id=%d)", user.ID, itemID)
				go services.AddPoints(user.ID, services.PointsContentViolation, services.ActionContentVioloation)

				c.HTML(http.StatusOK, "rss/transplant_result.html", gin.H{
					"Success": false,
					"Message": "抱歉，AI 认为该文章内容不适宜公开推荐，已扣除 1 竹笋作为惩罚。",
				})
				return
			}
			content = summary
		} else {
			// LLM 调用失败，记录日志并使用降级方案
			log.Printf("[Transplant] LLM 调用失败 (user_id=%d, item_id=%d): %v", user.ID, itemID, err)

			// 清理 Description 作为降级方案
			content = cleanHTMLAndTruncate(item.Description, 500)

			// 如果清理后仍然为空，返回错误提示
			if strings.TrimSpace(content) == "" {
				c.HTML(http.StatusOK, "rss/transplant_result.html", gin.H{
					"Success": false,
					"Message": "AI 摘要生成失败，且文章内容为空，请手动填写推荐语后重试。",
				})
				return
			}
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
		log.Printf("[Transplant] 发布失败 (user_id=%d, item_id=%d): %v", user.ID, itemID, err)
		c.HTML(http.StatusOK, "rss/transplant_result.html", gin.H{
			"Success": false,
			"Message": "发布失败，请稍后重试或手动填写推荐语。",
		})
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
		"PostID":  post.Pid,
	})
}

// autoUpvote 内部自动点赞逻辑
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

// cleanHTMLAndTruncate 清理 HTML 标签并截断文本
func cleanHTMLAndTruncate(html string, maxLength int) string {
	// 移除 HTML 标签
	re := regexp.MustCompile("<[^>]*>")
	text := re.ReplaceAllString(html, "")

	// 解码 HTML 实体
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// 清理多余空白
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// 截断
	if len(text) > maxLength {
		// 尝试在句子边界截断
		text = text[:maxLength]
		if lastPeriod := strings.LastIndexAny(text, "。.!！?？"); lastPeriod > maxLength/2 {
			text = text[:lastPeriod+1]
		} else {
			text = text + "..."
		}
	}

	return text
}
