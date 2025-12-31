package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
	"zhulink/internal/db"
	"zhulink/internal/middleware"
	"zhulink/internal/models"
	"zhulink/internal/services"
	"zhulink/internal/utils"

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
		Select("category").
		Group("category").
		Order("MIN(created_at) ASC").
		Pluck("category", &categories)

	// 如果没有分类，添加默认分类
	if len(categories) == 0 {
		categories = []string{"先放这儿"}
	}

	// 获取当前已订阅总数
	var currentCount int64
	db.DB.Model(&models.UserSubscription{}).Where("user_id = ?", user.ID).Count(&currentCount)

	maxCount := getMaxSubscriptionCount(user.Points)

	Render(c, http.StatusOK, "rss/index.html", gin.H{
		"Title":           "苗圃 - RSS 阅读器",
		"Active":          "rss",
		"Categories":      categories,
		"CurrentRSSCount": int(currentCount),
		"MaxRSSCount":     maxCount,
	})
}

// getMaxSubscriptionCount 根据积分获取订阅上限
func getMaxSubscriptionCount(points int) int {
	if points > 1000 {
		return -1 // 无限制
	}
	if points >= 201 {
		return 100
	}
	if points >= 51 {
		return 30
	}
	if points >= 11 {
		return 10
	}
	return 3
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

	// 按未读数降序排列，未读多的在前面
	sort.Slice(feedsWithCount, func(i, j int) bool {
		return feedsWithCount[i].UnreadCount > feedsWithCount[j].UnreadCount
	})

	// 获取所有分类供编辑时选择
	var allCategories []string
	db.DB.Model(&models.UserSubscription{}).
		Where("user_id = ?", user.ID).
		Select("category").
		Group("category").
		Order("MIN(created_at) ASC").
		Pluck("category", &allCategories)

	c.HTML(http.StatusOK, "rss/feed_list.html", gin.H{
		"Feeds":         feedsWithCount,
		"Category":      category,
		"AllCategories": allCategories,
	})
}

// GetItems HTMX 接口，获取指定订阅源的文章列表（支持游标分页）
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

	// 追加加载参数
	isAppend := c.Query("append") == "true"

	pageSize := 30

	// 获取用户订阅信息
	var subscription models.UserSubscription
	if err := db.DB.Preload("Feed").
		Where("user_id = ? AND feed_id = ?", user.ID, feedID).
		First(&subscription).Error; err != nil {
		c.String(http.StatusNotFound, "未找到订阅")
		return
	}

	// 如果是追加加载，先更新 last_read_anchor 为上一页最后一篇的时间
	// 这样下面的查询就能基于最新的锚点
	if isAppend {
		lastItemIDStr := c.Query("last_item_id")
		if lastItemID, err := strconv.Atoi(lastItemIDStr); err == nil && lastItemID > 0 {
			// 通过 item_id 查询文章的发布时间
			var lastItem models.FeedItem
			if err := db.DB.Select("published_at").First(&lastItem, lastItemID).Error; err == nil {
				// 截断到秒级，避免微秒导致的比较问题
				publishedAt := lastItem.PublishedAt.Truncate(time.Second)
				db.DB.Model(&models.UserSubscription{}).
					Where("user_id = ? AND feed_id = ?", user.ID, feedID).
					Where("last_read_anchor IS NULL OR last_read_anchor < ?", publishedAt).
					Update("last_read_anchor", publishedAt)
				// 更新本地变量
				subscription.LastReadAnchor = &publishedAt
			}
		}
	}

	// 构建查询基础
	query := db.DB.Model(&models.FeedItem{}).Where("feed_id = ?", feedID)

	// 分页逻辑：
	// 1. 首次加载 (isAppend=false):
	//    - 显示未读模式：从 last_read_anchor 之后开始
	//    - 显示全部模式：从最旧的开始
	// 2. 追加加载 (isAppend=true):
	//    - 总是从 last_read_anchor 之后开始（已在上面更新为上一页最后一篇的时间）
	if subscription.LastReadAnchor != nil {
		if !showAll && !isAppend {
			// 首次加载，仅看未读：过滤已读文章
			query = query.Where("published_at > ?", *subscription.LastReadAnchor)
		} else if isAppend {
			// 追加加载（显示全部或仅看未读）：使用锚点分页
			query = query.Where("published_at > ?", *subscription.LastReadAnchor)
		}
	}

	// 获取文章列表，按时间正序排列（旧的在上面，符合阅读流）
	// 多查询1条用于判断是否还有更多
	var items []models.FeedItem
	query.Order("published_at ASC").
		Limit(pageSize + 1).
		Find(&items)

	// 判断是否还有更多
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}

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

	// 获取当前页最后一篇的 ID，作为下一页的游标
	var lastItemID uint
	if len(items) > 0 {
		lastItemID = items[len(items)-1].ID
	}

	// 根据是否追加加载选择模板
	templateName := "rss/item_list.html"
	if isAppend {
		templateName = "rss/item_list_items.html"
	}

	c.HTML(http.StatusOK, templateName, gin.H{
		"Items":        itemsWithStatus,
		"Subscription": subscription,
		"FeedTitle":    subscription.GetDisplayTitle(),
		"Category":     c.Query("category"),
		"HasMore":      hasMore,
		"LastItemID":   lastItemID,
		"ShowAll":      showAll,
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
		// 截断到秒级，避免微秒导致的比较问题
		publishedAt := item.PublishedAt.Truncate(time.Second)
		// 仅当文章发布时间晚于当前已读锚点，或者还从未读过时才更新
		if subscription.LastReadAnchor == nil || publishedAt.After(*subscription.LastReadAnchor) {
			db.DB.Model(&subscription).Update("last_read_anchor", publishedAt)
		}
	}

	c.HTML(http.StatusOK, "rss/reader_content.html", gin.H{
		"Item":        item,
		"ContentHTML": utils.EnhanceHTMLContent(item.Content),
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

	// 校验订阅数量限制
	var currentCount int64
	db.DB.Model(&models.UserSubscription{}).Where("user_id = ?", user.ID).Count(&currentCount)
	maxCount := getMaxSubscriptionCount(user.Points)

	if maxCount != -1 && int(currentCount) >= maxCount {
		payload := map[string]interface{}{
			"show-error": map[string]string{
				"message": "您的竹笋不足以支撑更多订阅",
			},
		}
		if jsonBytes, err := json.Marshal(payload); err == nil {
			c.Header("HX-Trigger", url.PathEscape(string(jsonBytes)))
		}
		c.String(http.StatusForbidden, "")
		return
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
			"message": "订阅成功,文章正在后台加载中...",
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

	// 截断到秒级，避免微秒导致的比较问题
	publishedAt := latestItem.PublishedAt.Truncate(time.Second)
	// 更新锚点为最新文章时间
	db.DB.Model(&models.UserSubscription{}).
		Where("user_id = ? AND feed_id = ?", user.ID, feedID).
		Update("last_read_anchor", publishedAt)

	c.Header("HX-Trigger", "anchor-updated")
	c.String(http.StatusOK, "已标记全部已读")
}

// UpdateReadAnchorBatch 批量更新已读锚点（用于hover标记已读）
func (h *RSSHandler) UpdateReadAnchorBatch(c *gin.Context) {
	user := c.MustGet(middleware.CheckUserKey).(*models.User)
	feedIDStr := c.Param("id")
	feedID, err := strconv.Atoi(feedIDStr)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的订阅源 ID")
		return
	}

	var req struct {
		ItemID uint `json:"item_id"`
	}
	if err := c.BindJSON(&req); err != nil || req.ItemID == 0 {
		c.String(http.StatusBadRequest, "无效的请求")
		return
	}

	// 通过 item_id 查询文章的发布时间
	var item models.FeedItem
	if err := db.DB.Select("published_at").First(&item, req.ItemID).Error; err != nil {
		c.String(http.StatusNotFound, "文章不存在")
		return
	}

	// 截断到秒级，避免微秒导致的比较问题
	publishedAt := item.PublishedAt.Truncate(time.Second)

	// 仅当新时间更晚时才更新
	result := db.DB.Model(&models.UserSubscription{}).
		Where("user_id = ? AND feed_id = ?", user.ID, feedID).
		Where("last_read_anchor IS NULL OR last_read_anchor < ?", publishedAt).
		Update("last_read_anchor", publishedAt)

	if result.Error != nil {
		c.String(http.StatusInternalServerError, "更新失败: "+result.Error.Error())
		return
	}

	c.String(http.StatusOK, "")
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

// PopularFeeds 热门订阅页面（公开，无需登录）
func (h *RSSHandler) PopularFeeds(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize := 20

	// 统计有订阅的 Feed 总数
	var total int64
	db.DB.Model(&models.UserSubscription{}).
		Select("COUNT(DISTINCT feed_id)").
		Scan(&total)

	// 查询每个 Feed 的订阅人数，按人数降序排列
	type FeedWithCount struct {
		models.Feed
		SubscriberCount int `gorm:"column:subscriber_count"`
	}

	var feeds []FeedWithCount
	db.DB.Table("feeds").
		Select("feeds.*, COUNT(user_subscriptions.id) as subscriber_count").
		Joins("LEFT JOIN user_subscriptions ON feeds.id = user_subscriptions.feed_id").
		Group("feeds.id").
		Having("COUNT(user_subscriptions.id) > 0").
		Order("subscriber_count DESC, feeds.created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&feeds)

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	// 获取当前登录用户已订阅的 feed_id 集合
	subscribedMap := make(map[uint]bool)
	if user, exists := c.Get(middleware.CheckUserKey); exists && user != nil {
		currentUser := user.(*models.User)
		var subscribedIDs []uint
		db.DB.Model(&models.UserSubscription{}).
			Where("user_id = ?", currentUser.ID).
			Pluck("feed_id", &subscribedIDs)
		for _, id := range subscribedIDs {
			subscribedMap[id] = true
		}
	}

	Render(c, http.StatusOK, "rss/popular.html", gin.H{
		"Title":         "热门订阅 - RSS 发现",
		"Description":   "发现全站最受欢迎的 RSS 订阅源，按订阅人数排序，一键订阅热门博客、科技媒体和独立创作者的内容。",
		"Keywords":      "热门RSS,RSS订阅,热门博客,科技媒体,独立博客,订阅源推荐",
		"Canonical":     "/rss/popular",
		"Active":        "rss",
		"Feeds":         feeds,
		"CurrentPage":   page,
		"TotalPages":    totalPages,
		"Total":         total,
		"SubscribedMap": subscribedMap,
	})
}
