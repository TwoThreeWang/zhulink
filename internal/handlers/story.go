package handlers

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"
	"zhulink/internal/utils"

	"regexp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StoryHandler struct {
	mailService *services.MailService
}

func NewStoryHandler() *StoryHandler {
	return &StoryHandler{
		mailService: services.NewMailService(),
	}
}

// fillCommentCounts 批量填充帖子的评论数量
func fillCommentCounts(posts []models.Post) {
	if len(posts) == 0 {
		return
	}

	// 收集所有帖子ID
	postIDs := make([]uint, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}

	// 批量查询评论数量
	type CountResult struct {
		PostID uint
		Count  int
	}
	var results []CountResult
	db.DB.Model(&models.Comment{}).
		Select("post_id, COUNT(*) as count").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Scan(&results)

	// 建立映射
	countMap := make(map[uint]int)
	for _, r := range results {
		countMap[r.PostID] = r.Count
	}

	// 填充到帖子
	for i := range posts {
		posts[i].CommentCount = countMap[posts[i].ID]
	}
}

// Hacker News Ranking Algorithm: (P-1) / (T+2)^G
// P = points of an item (and -1 is to negate submitters vote)
// T = time since submission (in hours)
// G = Gravity, defaults to 1.8
func calculateRank(score int, createdAt time.Time) float64 {
	p := float64(score)
	t := time.Since(createdAt).Hours()
	g := 1.8
	return (p - 1.0) / math.Pow(t+2.0, g)
}

// In a real app we would compute rank in DB or background worker.
// For this MVP, we fetch top 100 recent stories and sort them in Go, or use simple DB order.
// Requirement: "Top/New".
// "New" is easy: ORDER BY created_at DESC.
// "Top" is Hacker News rank.
// Optimization: For now, let's fetch last 200 posts and sort them in memory, or just Sort by Score for simplicity if User prefers?
// User said: "Ranking Algorithm: 类似 Hacker News ...".
// Let's implement a simple SQL based approximation or fetch & sort.
// SQL approximation: score / power(extract(epoch from age(created_at))/3600 + 2, 1.8)
// PostgreSQL: `score / power((EXTRACT(EPOCH FROM NOW() - created_at)/3600) + 2, 1.8)`
// Note: This prevents using index effectively. But for MVP it's fine.

// extractFirstImage 从 Markdown 内容中提取第一张图片的 URL
func extractFirstImage(content string) string {
	re := regexp.MustCompile(`!\[.*?\]\((.*?)\)`)
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func (h *StoryHandler) ListTop(c *gin.Context) {
	// 分页参数
	page := 1
	if p := c.Query("page"); p != "" {
		if pageNum, err := strconv.Atoi(p); err == nil && pageNum > 0 {
			page = pageNum
		}
	}

	cacheKey := fmt.Sprintf("story:top:page:%d", page)
	if cachedData := utils.GetCache().Get(cacheKey); cachedData != nil {
		if hData, ok := cachedData.(gin.H); ok {
			Render(c, http.StatusOK, "story/list.html", hData)
			return
		}
	}

	perPage := 30
	offset := (page - 1) * perPage

	// 查询总数
	var total int64
	db.DB.Model(&models.Post{}).Count(&total)

	// 计算总页数
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages == 0 {
		totalPages = 1
	}

	var posts []models.Post
	db.DB.Preload("User").Preload("Node").
		Order("is_top DESC, score DESC, created_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&posts)

	fillCommentCounts(posts)

	// 获取节点列表（用于侧边栏导航）
	var nodes []models.Node
	db.DB.Order("id ASC").Find(&nodes)

	// SEO 数据
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://zhulink.vip"
	}
	fullURL := siteURL
	if page > 1 {
		fullURL = fmt.Sprintf("%s?page=%d", siteURL, page)
	}

	renderData := gin.H{
		"Posts":       posts,
		"Nodes":       nodes,
		"Active":      "top",
		"Title":       "热门话题",
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"Description": "汇聚 ZhuLink 社区近期热度最高的科技资讯、深度讨论与 RSS 精选。依靠用户“竹笋”共识筛选，拒绝算法推荐，只看最有价值的内容。",
		"Keywords":    "ZhuLink, 竹林, 技术社区, 热门文章, 高质量内容, 去算法, 技术社区, RSS聚合, Go语言, 独立开发, 深度阅读, ZhuLink",
		"FullURL":     fullURL,
	}

	// 写入缓存，有效期 1 分钟
	utils.GetCache().Set(cacheKey, renderData, 1*time.Minute)

	Render(c, http.StatusOK, "story/list.html", renderData)
}

func (h *StoryHandler) ListNew(c *gin.Context) {
	// 分页参数
	page := 1
	if p := c.Query("page"); p != "" {
		if pageNum, err := strconv.Atoi(p); err == nil && pageNum > 0 {
			page = pageNum
		}
	}

	perPage := 30
	offset := (page - 1) * perPage

	// 查询总数
	var total int64
	db.DB.Model(&models.Post{}).Count(&total)

	// 计算总页数
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages == 0 {
		totalPages = 1
	}

	var posts []models.Post
	db.DB.Preload("User").Preload("Node").
		Order("created_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&posts)

	fillCommentCounts(posts)

	// 获取节点列表（用于侧边栏导航）
	var nodes []models.Node
	db.DB.Order("id ASC").Find(&nodes)

	// SEO 数据
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://zhulink.vip"
	}
	fullURL := fmt.Sprintf("%s/new", siteURL)
	if page > 1 {
		fullURL = fmt.Sprintf("%s/new?page=%d", siteURL, page)
	}

	Render(c, http.StatusOK, "story/list.html", gin.H{
		"Posts":       posts,
		"Nodes":       nodes,
		"Active":      "new",
		"Title":       "最新发布",
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"Description": "实时追踪 ZhuLink 社区的最新动态。第一时间获取 RSS 移栽的科技新闻、独立开发者的新作展示以及新鲜的技术问答。",
		"Keywords":    "ZhuLink, 竹林, 最新文章, 技术分享, 社区动态, 最新资讯, 科技新闻, 实时动态, RSS更新",
		"FullURL":     fullURL,
	})
}

func (h *StoryHandler) ListByNode(c *gin.Context) {
	nodeName := c.Param("name")

	// 查找节点
	var node models.Node
	if err := db.DB.Where("name = ?", nodeName).First(&node).Error; err != nil {
		Render(c, http.StatusNotFound, "error.html", gin.H{"Error": "节点不存在"})
		return
	}

	// 分页参数
	page := 1
	if p := c.Query("page"); p != "" {
		if pageNum, err := strconv.Atoi(p); err == nil && pageNum > 0 {
			page = pageNum
		}
	}

	perPage := 30
	offset := (page - 1) * perPage

	// 查询该节点下的文章总数
	var total int64
	db.DB.Model(&models.Post{}).Where("node_id = ?", node.ID).Count(&total)

	// 计算总页数
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages == 0 {
		totalPages = 1
	}

	// 查询该节点下的文章
	var posts []models.Post
	db.DB.Preload("User").Preload("Node").
		Where("node_id = ?", node.ID).
		Order("created_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&posts)

	fillCommentCounts(posts)

	// 获取节点列表（用于侧边栏导航）
	var nodes []models.Node
	db.DB.Order("id ASC").Find(&nodes)

	// SEO 数据
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://zhulink.vip"
	}
	fullURL := fmt.Sprintf("%s/t/%s", siteURL, node.Name)
	if page > 1 {
		fullURL = fmt.Sprintf("%s/t/%s?page=%d", siteURL, node.Name, page)
	}

	description := fmt.Sprintf("ZhuLink 竹林 - %s节点的所有文章", node.Name)
	if node.Description != "" {
		description = fmt.Sprintf("ZhuLink 竹林 - %s节点: %s", node.Name, node.Description)
	}

	Render(c, http.StatusOK, "story/list.html", gin.H{
		"Posts":       posts,
		"Nodes":       nodes,
		"Active":      "node",
		"Title":       "" + node.Name,
		"Node":        node,
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"Description": description,
		"Keywords":    fmt.Sprintf("ZhuLink, 竹林, %s, 技术分享", node.Name),
		"FullURL":     fullURL,
	})
}

func (h *StoryHandler) Search(c *gin.Context) {
	query := c.Query("q")

	var posts []models.Post

	if query != "" {
		// 搜索标题和内容
		searchPattern := "%" + query + "%"
		db.DB.Preload("User").Preload("Node").
			Where("title ILIKE ? OR content ILIKE ?", searchPattern, searchPattern).
			Order("created_at DESC").
			Limit(50).
			Find(&posts)
	}

	fillCommentCounts(posts)

	// SEO 数据
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://zhulink.vip"
	}
	fullURL := fmt.Sprintf("%s/search?q=%s", siteURL, query)

	description := "在 ZhuLink 竹林搜索优质内容和技术文章"
	if query != "" {
		description = fmt.Sprintf("在 ZhuLink 竹林搜索 '%s' 的结果", query)
	}

	Render(c, http.StatusOK, "search.html", gin.H{
		"Posts":       posts,
		"Query":       query,
		"Active":      "search",
		"Title":       "搜索 - " + query,
		"Description": description,
		"Keywords":    fmt.Sprintf("ZhuLink, 竹林, 搜索, %s", query),
		"FullURL":     fullURL,
	})
}

func (h *StoryHandler) ShowCreate(c *gin.Context) {
	// 获取所有节点供用户选择
	var nodes []models.Node
	db.DB.Order("id ASC").Find(&nodes)

	Render(c, http.StatusOK, "story/create.html", gin.H{
		"Title": "发布",
		"Nodes": nodes,
	})
}

func (h *StoryHandler) Create(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)

	// 检查用户状态
	if user.Status == 2 {
		Render(c, http.StatusForbidden, "error.html", gin.H{"Error": "您的账号已被封禁，无法发布内容。"})
		return
	}
	if user.Status == 1 {
		if user.PunishExpires != nil && time.Now().After(*user.PunishExpires) {
			// 惩罚已过期，恢复状态（实际应该在 middleware 或单独逻辑中处理，这里简单处理）
			db.DB.Model(user).Update("status", 0)
		} else {
			Render(c, http.StatusForbidden, "error.html", gin.H{"Error": "您处于禁言状态，暂时无法发布内容。"})
			return
		}
	}

	title := c.PostForm("title")
	url := c.PostForm("url")
	content := c.PostForm("content")
	nodeIDStr := c.PostForm("node_id")

	if title == "" {
		var nodes []models.Node
		db.DB.Order("id ASC").Find(&nodes)
		Render(c, http.StatusBadRequest, "story/create.html", gin.H{
			"Error": "标题不能为空",
			"Nodes": nodes,
		})
		return
	}

	// 解析节点ID,默认为1(技术)
	nodeID := uint(1)
	if nodeIDStr != "" {
		if id, err := strconv.Atoi(nodeIDStr); err == nil {
			nodeID = uint(id)
		}
	}

	post := models.Post{
		Pid:     utils.RandStringBytesMaskImpr(8),
		UserID:  user.ID,
		NodeID:  nodeID,
		Title:   title,
		URL:     url,
		Content: content, // Helper will handle markdown render in view, here we store raw text/md
		Score:   1,       // Self vote
	}

	if err := db.DB.Create(&post).Error; err != nil {
		var nodes []models.Node
		db.DB.Order("id ASC").Find(&nodes)
		Render(c, http.StatusInternalServerError, "story/create.html", gin.H{
			"Error": "发布失败",
			"Nodes": nodes,
		})
		return
	}

	// 异步增加积分（每天前3篇）
	go func() {
		if services.CanEarnPostPoints(user.ID) {
			services.AddPoints(user.ID, services.PointsPostCreate, services.ActionPostCreate)
		}
	}()

	// 异步生成 SEO 元数据（关键词和描述）
	go func(postID uint, postTitle, postContent string) {
		seoMeta, err := services.GetLLMService().GenerateSEOMetadata(postTitle, postContent)
		if err != nil {
			// SEO 生成失败不影响主流程，仅记录日志
			fmt.Printf("[SEO] 生成 SEO 元数据失败 (postID=%d): %v\n", postID, err)
			return
		}
		// 更新数据库
		if err := db.DB.Model(&models.Post{}).Where("id = ?", postID).Updates(map[string]interface{}{
			"seo_keywords":    seoMeta.Keywords,
			"seo_description": seoMeta.Description,
		}).Error; err != nil {
			fmt.Printf("[SEO] 更新 SEO 元数据失败 (postID=%d): %v\n", postID, err)
			return
		}
		fmt.Printf("[SEO] 已更新帖子 %d 的 SEO 元数据\n", postID)
	}(post.ID, title, content)

	c.Redirect(http.StatusFound, "/p/"+post.Pid)
}

func (h *StoryHandler) Detail(c *gin.Context) {
	pid := c.Param("pid")

	// 获取当前用户 ID 用于实时状态查询
	userID := uint(0)
	if user, exists := c.Get(middleware.CheckUserKey); exists && user != nil {
		userID = user.(*models.User).ID
	}

	// 共享缓存逻辑：不再区分用户
	cacheKey := fmt.Sprintf("story:detail:shared:%s", pid)
	if cachedData := utils.GetCache().Get(cacheKey); cachedData != nil {
		if hData, ok := cachedData.(gin.H); ok {
			// 即使是缓存，也要增加浏览量
			if postData, ok := hData["Post"].(models.Post); ok {
				db.DB.Model(&models.Post{}).Where("id = ?", postData.ID).UpdateColumn("views", gorm.Expr("views + 1"))
				services.GetRankingService().ScheduleUpdate(postData.ID)
			}

			// 实时查询当前用户的私有状态（如是否已收藏）
			isBookmarked := false
			if userID > 0 {
				if postData, ok := hData["Post"].(models.Post); ok {
					var bookmark models.Bookmark
					if err := db.DB.Where("user_id = ? AND post_id = ?", userID, postData.ID).First(&bookmark).Error; err == nil {
						isBookmarked = true
					}
				}
			}
			hData["IsBookmarked"] = isBookmarked

			Render(c, http.StatusOK, "story/detail.html", hData)
			return
		}
	}

	var post models.Post
	if err := db.DB.Preload("User").Preload("Node").Where("pid = ?", pid).First(&post).Error; err != nil {
		Render(c, http.StatusNotFound, "error.html", gin.H{"Error": "文章不存在"})
		return
	}

	// 增加浏览量
	db.DB.Model(&post).UpdateColumn("views", post.Views+1)
	post.Views++

	// 异步更新帖子 Score
	services.GetRankingService().ScheduleUpdate(post.ID)

	// Load comments
	var comments []models.Comment
	db.DB.Preload("User").Where("post_id = ?", post.ID).Order("created_at ASC").Find(&comments)

	type FlatComment struct {
		models.Comment
		ContentHTML template.HTML
		Floor       int
	}

	flatComments := make([]FlatComment, len(comments))
	for i, com := range comments {
		htmlContent := utils.RenderMarkdown(com.Content)
		flatComments[i] = FlatComment{
			Comment:     com,
			ContentHTML: htmlContent,
			Floor:       i + 1,
		}
	}

	postContentHTML := utils.RenderMarkdown(post.Content)

	var bookmarkCount int64
	db.DB.Model(&models.Bookmark{}).Where("post_id = ?", post.ID).Count(&bookmarkCount)

	var upvoteCount int64
	db.DB.Model(&models.Vote{}).Where("post_id = ? AND value = 1", post.ID).Count(&upvoteCount)

	var downvoteCount int64
	db.DB.Model(&models.Vote{}).Where("post_id = ? AND value = -1", post.ID).Count(&downvoteCount)

	// 注意：在存入缓存的 renderData 中不包含 IsBookmarked，因为它随请求变化
	// 查询所有节点
	var nodes []models.Node
	db.DB.Order("id ASC").Find(&nodes)

	keywords := post.SEOKeywords
	if keywords == "" {
		keywords = fmt.Sprintf("%s, ZhuLink, 竹林, 技术分享", post.Node.Name)
	}

	description := post.SEODescription
	if description == "" {
		description = post.Content
		if len(description) > 150 {
			runes := []rune(description)
			if len(runes) > 150 {
				description = string(runes[:150]) + "..."
			}
		}
		description = strings.ReplaceAll(description, "#", "")
		description = strings.ReplaceAll(description, "*", "")
		description = strings.ReplaceAll(description, "`", "")
		description = strings.TrimSpace(description)
	}

	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "https://zhulink.vip"
	}
	fullURL := fmt.Sprintf("%s/p/%s", siteURL, post.Pid)
	author := post.User.Username
	publishedTime := post.CreatedAt.Format(time.RFC3339)
	modifiedTime := post.UpdatedAt.Format(time.RFC3339)

	var prevPost models.Post
	hasPrev := db.DB.Select("pid, title").
		Where("created_at < ?", post.CreatedAt).
		Order("created_at DESC").
		First(&prevPost).Error == nil

	var nextPost models.Post
	hasNext := db.DB.Select("pid, title").
		Where("created_at > ?", post.CreatedAt).
		Order("created_at ASC").
		First(&nextPost).Error == nil

	firstImage := extractFirstImage(post.Content)
	imageURL := firstImage
	if imageURL == "" {
		imageURL = "/static/img/logo.svg"
	}

	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		base := strings.TrimSuffix(siteURL, "/")
		if !strings.HasPrefix(imageURL, "/") {
			imageURL = "/" + imageURL
		}
		imageURL = base + imageURL
	}

	renderData := gin.H{
		"Post":          post,
		"PostContent":   postContentHTML,
		"Comments":      flatComments,
		"Title":         post.Title,
		"BookmarkCount": bookmarkCount,
		"UpvoteCount":   upvoteCount,
		"DownvoteCount": downvoteCount,
		"Nodes":         nodes,
		"Description":   description,
		"Keywords":      keywords,
		"FullURL":       fullURL,
		"ImageURL":      imageURL,
		"Author":        author,
		"PublishedTime": publishedTime,
		"ModifiedTime":  modifiedTime,
		"HasPrev":       hasPrev,
		"PrevPost":      prevPost,
		"HasNext":       hasNext,
		"NextPost":      nextPost,
	}

	// 写入共享缓存，有效期延长至 5 分钟
	utils.GetCache().Set(cacheKey, renderData, 5*time.Minute)

	// 为当前请求注入 IsBookmarked
	isBookmarked := false
	if userID > 0 {
		var bookmark models.Bookmark
		if err := db.DB.Where("user_id = ? AND post_id = ?", userID, post.ID).First(&bookmark).Error; err == nil {
			isBookmarked = true
		}
	}
	renderData["IsBookmarked"] = isBookmarked

	Render(c, http.StatusOK, "story/detail.html", renderData)
}

func (h *StoryHandler) CreateComment(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	pid := c.Param("pid")

	// 检查用户状态
	if user.Status == 2 {
		// 封禁用户无法发布评论
		Render(c, http.StatusForbidden, "error.html", gin.H{"Error": "您的账号已被封禁,无法发布评论。"})
		return
	}
	if user.Status == 1 {
		// 禁言用户,检查是否已过期
		if user.PunishExpires != nil && time.Now().After(*user.PunishExpires) {
			// 禁言已过期,恢复状态
			db.DB.Model(user).Updates(map[string]interface{}{
				"status":         0,
				"punish_expires": nil,
			})
			user.Status = 0
		} else {
			// 仍在禁言期
			Render(c, http.StatusForbidden, "error.html", gin.H{"Error": "您处于禁言状态,暂时无法发布评论。"})
			return
		}
	}

	// 通过Pid查找文章
	var post models.Post
	if err := db.DB.Where("pid = ?", pid).First(&post).Error; err != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	content := c.PostForm("content")
	parentIDStr := c.PostForm("parent_id")
	replyFloor := c.PostForm("reply_floor") // 被回复评论的楼层号

	if content == "" {
		c.Redirect(http.StatusFound, "/p/"+pid)
		return
	}

	// 如果是回复评论，在内容开头拼接回复引用
	var parentID *uint
	if parentIDStr != "" {
		pID, _ := strconv.Atoi(parentIDStr)
		uPID := uint(pID)
		parentID = &uPID

		// 获取被回复评论的信息用于拼接引用
		var parentComment models.Comment
		if err := db.DB.Preload("User").First(&parentComment, uPID).Error; err == nil {
			// 拼接回复引用：↳ 回复 [#楼层](#comment-ID)
			replyPrefix := fmt.Sprintf("↳ 回复 [#%s @%s](#comment-%d)\n\n", replyFloor, parentComment.User.Username, parentComment.ID)
			content = replyPrefix + content
		}
	}

	comment := models.Comment{
		Cid:      utils.RandStringBytesMaskImpr(8),
		PostID:   post.ID,
		UserID:   user.ID,
		Content:  content,
		Score:    1,
		ParentID: parentID,
	}

	if err := db.DB.Create(&comment).Error; err != nil {
		// handle error
	}

	// 主动失效详情页缓存
	utils.GetCache().Delete(fmt.Sprintf("story:detail:shared:%s", post.Pid))

	// 异步更新帖子 Score（新增评论）
	services.GetRankingService().ScheduleUpdate(post.ID)

	// 异步增加积分（每天前3条评论）
	go func() {
		if services.CanEarnCommentPoints(user.ID) {
			services.AddPoints(user.ID, services.PointsCommentCreate, services.ActionCommentCreate)
		}
	}()

	// Create Notifications
	go func() {
		// 如果是回复评论，只通知被回复者
		if comment.ParentID != nil {
			var parentComment models.Comment
			if err := db.DB.Preload("User").First(&parentComment, *comment.ParentID).Error; err == nil {
				// 不要通知自己
				if parentComment.UserID != user.ID {
					notification := models.Notification{
						UserID:  parentComment.UserID,
						ActorID: &user.ID,
						Type:    models.NotificationTypeReplyComment,
						Reason: fmt.Sprintf("在文章 <a href=\"/p/%s#comment-%d\" target=\"_blank\" class=\"text-moss font-medium hover:underline tracking-tight\">《%s》</a> 中回复了您的评论",
							post.Pid, comment.ID, post.Title),
					}
					db.DB.Create(&notification)

					// Send Email Notification
					postLink := fmt.Sprintf("%s/p/%s#comment-%d", os.Getenv("SITE_URL"), post.Pid, comment.ID)
					h.mailService.SendCommentNotification(
						parentComment.User.Email,
						user.Username,
						post.Title,
						content, // This includes "Reply to #X..." prefix which gives context
						parentComment.Content,
						postLink,
					)
				}
			}
		} else {
			// 如果是直接评论文章，通知文章作者
			if post.UserID != user.ID {
				notification := models.Notification{
					UserID:  post.UserID,
					ActorID: &user.ID,
					Type:    models.NotificationTypeCommentPost,
					Reason: fmt.Sprintf("在您的文章 <a href=\"/p/%s#comment-%d\" target=\"_blank\" class=\"text-moss font-medium hover:underline tracking-tight\">《%s》</a> 中发布了新的评论",
						post.Pid, comment.ID, post.Title),
				}
				db.DB.Create(&notification)
			}
		}
	}()

	c.Redirect(http.StatusFound, "/p/"+pid)
}

// DeleteComment 软删除评论（只替换内容，保留用户名）
func (h *StoryHandler) DeleteComment(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	cid := c.Param("cid")

	var comment models.Comment
	if err := db.DB.Where("cid = ?", cid).First(&comment).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// 只允许删除自己的评论
	if comment.UserID != user.ID {
		c.Status(http.StatusForbidden)
		return
	}

	// 软删除：只替换内容
	comment.Content = "该评论已删除。"
	db.DB.Save(&comment)

	// 主动失效详情页缓存
	var post models.Post
	if err := db.DB.First(&post, comment.PostID).Error; err == nil {
		utils.GetCache().Delete(fmt.Sprintf("story:detail:shared:%s", post.Pid))
	}

	// 异步扣除积分
	services.AddPointsAsync(user.ID, services.PointsCommentDeleted, services.ActionCommentDeleted)

	c.Status(http.StatusOK)
}

func (h *StoryHandler) Delete(c *gin.Context) {
	// HTMX delete
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	pid := c.Param("pid")

	var post models.Post
	if err := db.DB.Where("pid = ?", pid).First(&post).Error; err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	if post.UserID != user.ID {
		c.Status(http.StatusForbidden)
		return
	}

	// Hard Delete
	db.DB.Unscoped().Delete(&post)

	// 主动失效详情页等相关缓存
	utils.GetCache().Delete(fmt.Sprintf("story:detail:shared:%s", post.Pid))
	// 列表页第一页也失效
	utils.GetCache().Delete("story:top:page:1")
	utils.GetCache().Delete("story:new:page:1")

	// 异步扣除积分
	services.AddPointsAsync(user.ID, services.PointsPostDeleted, services.ActionPostDeleted)

	// HTMX response: Empty content to remove element if targeting specific element,
	// or redirect.
	// If called from list, remove row.
	// If called from detail, redirect home.

	redirect := c.GetHeader("HX-Current-URL")
	if strings.Contains(redirect, "/p/") && !strings.Contains(redirect, "/new") {
		// We are on detail page
		c.Header("HX-Redirect", "/")
	}
	c.Status(http.StatusOK) // Returns 200 OK, empty body removes the target if used with hx-target
}

func (h *StoryHandler) ShowEdit(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	pid := c.Param("pid")

	var post models.Post
	if err := db.DB.Where("pid = ?", pid).First(&post).Error; err != nil {
		Render(c, http.StatusNotFound, "error.html", gin.H{"Error": "文章不存在"})
		return
	}

	// 验证是否为作者
	if post.UserID != user.ID {
		Render(c, http.StatusForbidden, "error.html", gin.H{"Error": "无权编辑此文章"})
		return
	}

	var nodes []models.Node
	db.DB.Order("id ASC").Find(&nodes)

	Render(c, http.StatusOK, "story/edit.html", gin.H{
		"Title": "编辑文章",
		"Post":  post,
		"Nodes": nodes,
	})
}

func (h *StoryHandler) Update(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	pid := c.Param("pid")

	var post models.Post
	if err := db.DB.Where("pid = ?", pid).First(&post).Error; err != nil {
		Render(c, http.StatusNotFound, "error.html", gin.H{"Error": "文章不存在"})
		return
	}

	// 验证是否为作者
	if post.UserID != user.ID {
		Render(c, http.StatusForbidden, "error.html", gin.H{"Error": "无权编辑此文章"})
		return
	}

	title := c.PostForm("title")
	url := c.PostForm("url")
	content := c.PostForm("content")
	nodeIDStr := c.PostForm("node_id")

	if title == "" {
		var nodes []models.Node
		db.DB.Order("id ASC").Find(&nodes)
		Render(c, http.StatusBadRequest, "story/edit.html", gin.H{
			"Error": "标题不能为空",
			"Post":  post,
			"Nodes": nodes,
		})
		return
	}

	// 解析节点ID
	nodeID := post.NodeID
	if nodeIDStr != "" {
		if id, err := strconv.Atoi(nodeIDStr); err == nil {
			nodeID = uint(id)
		}
	}

	// 更新文章
	post.Title = title
	post.URL = url
	post.Content = content
	post.NodeID = nodeID

	if err := db.DB.Save(&post).Error; err != nil {
		var nodes []models.Node
		db.DB.Order("id ASC").Find(&nodes)
		Render(c, http.StatusInternalServerError, "story/edit.html", gin.H{
			"Error": "保存失败",
			"Post":  post,
			"Nodes": nodes,
		})
		return
	}

	c.Redirect(http.StatusFound, "/p/"+pid)
}
