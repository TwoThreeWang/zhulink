package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"

	"github.com/gin-gonic/gin"
)

type RSSHandler struct{}

func NewRSSHandler() *RSSHandler {
	return &RSSHandler{}
}

// Index 渲染 RSS 主页框架
func (h *RSSHandler) Index(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)

	// 获取用户的所有分类
	var categories []string
	db.DB.Model(&models.UserSubscription{}).
		Where("user_id = ?", user.ID).
		Distinct("category").
		Pluck("category", &categories)

	// 如果没有分类，添加默认分类
	if len(categories) == 0 {
		categories = []string{"先放这儿"}
	}

	Render(c, http.StatusOK, "rss/index.html", gin.H{
		"Title":      "苗圃 - RSS 阅读器",
		"Active":     "rss",
		"Categories": categories,
	})
}

// GetFeeds HTMX 接口，获取指定分类的订阅源列表
func (h *RSSHandler) GetFeeds(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	category := c.Query("category")
	if category == "" {
		category = "先放这儿"
	}

	// 获取该分类下的订阅
	var subscriptions []models.UserSubscription
	db.DB.Preload("Feed").
		Where("user_id = ? AND category = ?", user.ID, category).
		Order("created_at DESC").
		Find(&subscriptions)

	// 计算每个订阅源的未读数
	type FeedWithCount struct {
		Subscription models.UserSubscription
		UnreadCount  int
	}
	var feedsWithCount []FeedWithCount

	for _, sub := range subscriptions {
		var count int64
		query := db.DB.Model(&models.FeedItem{}).Where("feed_id = ?", sub.FeedID)
		if sub.LastReadAnchor != nil {
			query = query.Where("published_at > ?", *sub.LastReadAnchor)
		}
		query.Count(&count)

		feedsWithCount = append(feedsWithCount, FeedWithCount{
			Subscription: sub,
			UnreadCount:  int(count),
		})
	}

	// 获取所有分类供编辑时选择
	var allCategories []string
	db.DB.Model(&models.UserSubscription{}).
		Where("user_id = ?", user.ID).
		Distinct("category").
		Pluck("category", &allCategories)

	c.HTML(http.StatusOK, "rss/feed_list.html", gin.H{
		"Feeds":         feedsWithCount,
		"Category":      category,
		"AllCategories": allCategories,
	})
}

// GetItems HTMX 接口，获取指定订阅源的文章列表（支持分页）
func (h *RSSHandler) GetItems(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	feedIDStr := c.Query("feed_id")
	feedID, err := strconv.Atoi(feedIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的订阅源 ID")
		return
	}

	// 过滤参数：默认不显示全部（即仅显示未读）
	showAll := c.Query("show_all") == "true"

	// 分页参数
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	pageSize := 30 // 每页 30 条，减少内存占用

	// 获取用户订阅信息（包含已读锚点）
	var subscription models.UserSubscription
	if err := db.DB.Preload("Feed").
		Where("user_id = ? AND feed_id = ?", user.ID, feedID).
		First(&subscription).Error; err != nil {
		c.String(http.StatusNotFound, "未找到订阅")
		return
	}

	// 构建查询基础
	query := db.DB.Model(&models.FeedItem{}).Where("feed_id = ?", feedID)

	// 如果不是显示全部，且有已读锚点，则只查询锚点之后的文章
	if !showAll && subscription.LastReadAnchor != nil {
		query = query.Where("published_at > ?", *subscription.LastReadAnchor)
	}

	// 获取符合过滤条件的文章总数
	var totalItems int64
	query.Count(&totalItems)

	// 计算总页数
	totalPages := int(totalItems) / pageSize
	if int(totalItems)%pageSize > 0 {
		totalPages++
	}
	if totalPages < 1 {
		totalPages = 1
	}

	// 获取文章列表，按时间正序排列（旧的在上面，符合阅读流），分页
	var items []models.FeedItem
	query.Order("published_at ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items)

	// 标记已读状态
	type ItemWithReadStatus struct {
		Item   models.FeedItem
		IsRead bool
	}
	var itemsWithStatus []ItemWithReadStatus

	for _, item := range items {
		isRead := false
		if subscription.LastReadAnchor != nil {
			isRead = !item.PublishedAt.After(*subscription.LastReadAnchor)
		}
		itemsWithStatus = append(itemsWithStatus, ItemWithReadStatus{
			Item:   item,
			IsRead: isRead,
		})
	}

	c.HTML(http.StatusOK, "rss/item_list.html", gin.H{
		"Items":        itemsWithStatus,
		"Subscription": subscription,
		"FeedTitle":    subscription.GetDisplayTitle(),
		"Category":     c.Query("category"),
		"CurrentPage":  page,
		"TotalPages":   totalPages,
		"TotalItems":   int(totalItems),
		"HasMore":      page < totalPages,
		"ShowAll":      showAll, // 回传状态供模板使用
	})
}

// ReadItem HTMX 接口，读取文章内容
func (h *RSSHandler) ReadItem(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	itemIDStr := c.Param("id")
	itemID, err := strconv.Atoi(itemIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的文章 ID")
		return
	}

	// 获取文章
	var item models.FeedItem
	if err := db.DB.First(&item, itemID).Error; err != nil {
		c.String(http.StatusNotFound, "文章不存在")
		return
	}

	// 检查 Content 是否为空，需要实时抓取
	if item.Content == "" {
		crawler := services.GetCrawlerService()
		content, err := crawler.FetchArticleContent(item.Link)
		if err == nil && content != "" {
			// 回填到数据库
			item.Content = content
			db.DB.Model(&item).Update("content", content)
		} else {
			// 抓取失败，使用 Description 作为内容
			item.Content = item.Description
		}
	}

	// 更新已读锚点 (同步执行，确保稳定)
	var subscription models.UserSubscription
	err = db.DB.Where("user_id = ? AND feed_id = ?", user.ID, item.FeedID).First(&subscription).Error
	if err == nil {
		// 仅当文章发布时间晚于当前已读锚点，或者还从未读过时才更新
		if subscription.LastReadAnchor == nil || item.PublishedAt.After(*subscription.LastReadAnchor) {
			db.DB.Model(&subscription).Update("last_read_anchor", item.PublishedAt)
		}
	}

	c.HTML(http.StatusOK, "rss/reader_content.html", gin.H{
		"Item": item,
	})
}

// Subscribe 添加订阅
func (h *RSSHandler) Subscribe(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	rssURL := c.PostForm("url")
	category := c.PostForm("category")
	customTitle := c.PostForm("title")

	if rssURL == "" {
		payload := map[string]interface{}{
			"show-error": map[string]string{
				"message": "请输入 RSS 地址",
			},
		}
		if jsonBytes, err := json.Marshal(payload); err == nil {
			c.Header("HX-Trigger", url.PathEscape(string(jsonBytes)))
		}
		c.String(http.StatusBadRequest, "")
		return
	}

	if category == "" {
		category = "先放这儿"
	}

	// 创建或获取 Feed
	fetcher := services.GetRSSFetcher()
	feed, err := fetcher.CreateOrGetFeed(rssURL)
	if err != nil {
		payload := map[string]interface{}{
			"show-error": map[string]string{
				"message": "无法解析 RSS 地址: " + err.Error(),
			},
		}
		if jsonBytes, err := json.Marshal(payload); err == nil {
			c.Header("HX-Trigger", url.PathEscape(string(jsonBytes)))
		}
		c.String(http.StatusBadRequest, "")
		return
	}

	// 检查是否已经订阅
	var existingSub models.UserSubscription
	if err := db.DB.Where("user_id = ? AND feed_id = ?", user.ID, feed.ID).First(&existingSub).Error; err == nil {
		payload := map[string]interface{}{
			"show-error": map[string]string{
				"message": "您已经订阅了这个源",
			},
		}
		if jsonBytes, err := json.Marshal(payload); err == nil {
			c.Header("HX-Trigger", url.PathEscape(string(jsonBytes)))
		}
		c.String(http.StatusBadRequest, "")
		return
	}

	// 创建订阅关系
	subscription := models.UserSubscription{
		UserID:   user.ID,
		FeedID:   feed.ID,
		Title:    customTitle,
		Category: category,
	}

	if err := db.DB.Create(&subscription).Error; err != nil {
		payload := map[string]interface{}{
			"show-error": map[string]string{
				"message": "订阅失败",
			},
		}
		if jsonBytes, err := json.Marshal(payload); err == nil {
			c.Header("HX-Trigger", url.PathEscape(string(jsonBytes)))
		}
		c.String(http.StatusInternalServerError, "")
		return
	}

	// 返回成功响应，触发Toast和页面刷新
	payload := map[string]interface{}{
		"show-success": map[string]string{
			"message": "订阅成功",
		},
	}
	if jsonBytes, err := json.Marshal(payload); err == nil {
		c.Header("HX-Trigger", url.PathEscape(string(jsonBytes)))
	}
	c.String(http.StatusOK, "")
}

// Unsubscribe 取消订阅
func (h *RSSHandler) Unsubscribe(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	subIDStr := c.Param("id")
	subID, err := strconv.Atoi(subIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的订阅 ID")
		return
	}

	// 删除订阅关系
	result := db.DB.Where("id = ? AND user_id = ?", subID, user.ID).Delete(&models.UserSubscription{})
	if result.RowsAffected == 0 {
		c.String(http.StatusNotFound, "订阅不存在")
		return
	}

	c.Header("HX-Trigger", "subscription-removed")
	c.String(http.StatusOK, "已取消订阅")
}

// RefreshFeed 刷新订阅源
func (h *RSSHandler) RefreshFeed(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	feedIDStr := c.Param("id")
	feedID, err := strconv.Atoi(feedIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的订阅源 ID")
		return
	}

	// 验证用户订阅了该 Feed
	var subscription models.UserSubscription
	if err := db.DB.Preload("Feed").
		Where("user_id = ? AND feed_id = ?", user.ID, feedID).
		First(&subscription).Error; err != nil {
		c.String(http.StatusForbidden, "您未订阅此源")
		return
	}

	// 刷新订阅源
	fetcher := services.GetRSSFetcher()
	if err := fetcher.RefreshFeed(&subscription.Feed); err != nil {
		c.String(http.StatusInternalServerError, "刷新失败: "+err.Error())
		return
	}

	c.Header("HX-Trigger", "feed-refreshed")
	c.String(http.StatusOK, "刷新成功")
}

// ShowAddModal 显示添加订阅的模态框
func (h *RSSHandler) ShowAddModal(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)

	// 获取用户的所有分类供选择
	var categories []string
	db.DB.Model(&models.UserSubscription{}).
		Where("user_id = ?", user.ID).
		Distinct("category").
		Pluck("category", &categories)

	c.HTML(http.StatusOK, "rss/add_modal.html", gin.H{
		"Categories": categories,
	})
}

// UpdateAnchor 更新已读锚点（单独的 API，用于标记全部已读等场景）
func (h *RSSHandler) UpdateAnchor(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	feedIDStr := c.Param("id")
	feedID, err := strconv.Atoi(feedIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的订阅源 ID")
		return
	}

	// 获取该订阅源最新文章的时间
	var latestItem models.FeedItem
	if err := db.DB.Where("feed_id = ?", feedID).
		Order("published_at DESC").
		First(&latestItem).Error; err != nil {
		c.String(http.StatusNotFound, "没有文章")
		return
	}

	// 更新锚点为最新文章时间
	now := time.Now()
	db.DB.Model(&models.UserSubscription{}).
		Where("user_id = ? AND feed_id = ?", user.ID, feedID).
		Update("last_read_anchor", now)

	c.Header("HX-Trigger", "anchor-updated")
	c.String(http.StatusOK, "已标记全部已读")
}

// UpdateSubscription 更新订阅信息（标题、分类）
func (h *RSSHandler) UpdateSubscription(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)

	// 从表单获取 ID
	subIDStr := c.PostForm("id")

	subID, err := strconv.Atoi(subIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的订阅 ID")
		return
	}

	// 获取订阅
	var subscription models.UserSubscription
	if err := db.DB.Where("id = ? AND user_id = ?", subID, user.ID).First(&subscription).Error; err != nil {
		c.String(http.StatusNotFound, "订阅不存在")
		return
	}

	// 更新信息
	title := c.PostForm("title")
	category := c.PostForm("category")

	if category == "" {
		category = "先放这儿"
	}

	subscription.Title = title
	subscription.Category = category

	if err := db.DB.Save(&subscription).Error; err != nil {
		c.String(http.StatusInternalServerError, "更新失败")
		return
	}

	c.Header("HX-Trigger", "subscription-updated")
	c.String(http.StatusOK, "更新成功")
}
